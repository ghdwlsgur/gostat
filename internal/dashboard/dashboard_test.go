package dashboard

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

func sampleResult(status int) *probe.Result {
	sum := sha256.Sum256([]byte("payload"))

	return &probe.Result{
		Edge:       "1.2.3.4",
		Proto:      "HTTP/2.0",
		StatusCode: status,
		Headers: http.Header{
			"Server":                      []string{"cdn"},
			"Date":                        []string{"Mon, 02 Oct 2023 00:00:00 GMT"},
			"Access-Control-Allow-Origin": []string{"*"},
		},
		BodySum: sum[:],
		Trace: probe.Trace{
			DNSLookup:        1 * time.Millisecond,
			TCPConnection:    10 * time.Millisecond,
			TLSHandshake:     100 * time.Millisecond,
			ServerProcessing: 1 * time.Second,
			ContentTransfer:  2 * time.Second,
			Total:            3200 * time.Millisecond,
			TLS:              true,
		},
	}
}

// edgeOf splits an httptest URL into the address and port to pin to.
func edgeOf(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parsing the port of %q: %v", rawURL, err)
	}

	return u.Hostname(), port
}

func testDashboard(t *testing.T, edges []string) *Dashboard {
	t.Helper()

	u, err := url.Parse("https://example.com/asset.txt")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}

	return newDashboard(nil, u, edges)
}

// settle runs fn on the application goroutine and waits for the redraw that
// follows, so the simulation screen can be read straight afterwards.
func settle(app *tview.Application, fn func()) {
	if fn != nil {
		app.QueueUpdateDraw(fn)
	}

	done := make(chan struct{})
	app.QueueUpdateDraw(func() { close(done) })
	<-done
}

// offsetOf reads the table's scroll position on the application's own
// goroutine. tview does not guard the field, so reading it from the test
// races with the input handler writing it - and returns whichever value the
// race happens to produce.
func offsetOf(app *tview.Application, table *tview.Table) (int, int) {
	var row, column int

	done := make(chan struct{})
	app.QueueUpdate(func() {
		row, column = table.GetOffset()
		close(done)
	})
	<-done

	return row, column
}

// screenNow reads the simulation screen on the application's own goroutine.
// GetContents hands back the live cell buffer, so reading it from the test
// races with the next draw.
func screenNow(app *tview.Application, screen tcell.SimulationScreen) string {
	var text string

	done := make(chan struct{})
	app.QueueUpdate(func() {
		text = screenText(screen)
		close(done)
	})
	<-done

	return text
}

func screenText(screen tcell.SimulationScreen) string {
	cells, width, height := screen.GetContents()

	var b strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := cells[y*width+x]
			if len(cell.Runes) > 0 {
				b.WriteRune(cell.Runes[0])
			} else {
				b.WriteRune(' ')
			}
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// renderScreen draws the dashboard onto a simulation screen of the given size
// and hands the screen back, along with the call that shuts the application
// down again.
func renderScreen(t *testing.T, width, height int, edges []string) (tcell.SimulationScreen, *tview.Application, func()) {
	t.Helper()

	screen := tcell.NewSimulationScreen("UTF-8")
	d := testDashboard(t, edges)
	d.app.SetScreen(screen)

	stopped := make(chan error, 1)
	go func() { stopped <- d.app.Run() }()

	settle(d.app, nil)
	screen.SetSize(width, height)
	settle(d.app, func() {
		for i, edge := range edges {
			d.record(i, edge, sampleResult(http.StatusPartialContent))
		}
	})

	return screen, d.app, func() {
		d.app.Stop()
		if err := <-stopped; err != nil {
			t.Errorf("app.Run: %v", err)
		}
	}
}

// render returns what a terminal of the given size would show.
func render(t *testing.T, width, height int, edges []string) string {
	t.Helper()

	screen, app, stop := renderScreen(t, width, height, edges)
	defer stop()

	return screenNow(app, screen)
}

// The view this replaced was pinned to coordinates that needed 180 by 43. On
// an ordinary terminal it drew one empty box and no data whatsoever.
func TestRendersOnAnOrdinaryTerminal(t *testing.T) {
	out := render(t, 80, 24, []string{"1.1.1.1"})

	for _, want := range []string{"Response", "StatusCode", "206", "Latency", "Total"} {
		if !strings.Contains(out, want) {
			t.Errorf("an 80x24 terminal does not show %q:\n%s", want, out)
		}
	}
}

func TestRendersOnALargeTerminal(t *testing.T) {
	out := render(t, 200, 60, []string{"1.1.1.1", "2.2.2.2"})

	for _, want := range []string{
		"Response", "Status per edge", "Latency", "Changes",
		"1.1.1.1", "2.2.2.2", "206", "cdn", "HTTP/2.0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("a 200x60 terminal does not show %q:\n%s", want, out)
		}
	}
}

// Every panel has to keep a share of the screen at any size, so none of them
// can be squeezed out of existence the way fixed rectangles allowed.
func TestRendersAtSeveralSizes(t *testing.T) {
	sizes := [][2]int{{60, 20}, {80, 24}, {120, 40}, {200, 60}}

	for _, size := range sizes {
		out := render(t, size[0], size[1], []string{"1.1.1.1"})
		if !strings.Contains(out, "206") {
			t.Errorf("%dx%d shows no status code:\n%s", size[0], size[1], out)
		}
	}
}

// The table is an edge per row and a header per column, and only the headers
// that carry something get one.
func TestResponseTableIsOneRowPerEdge(t *testing.T) {
	edges := []string{"1.1.1.1", "2.2.2.2"}
	table := newResponseTable()

	latest := map[string]*probe.Result{
		"1.1.1.1": sampleResult(http.StatusPartialContent),
		"2.2.2.2": sampleResult(http.StatusServiceUnavailable),
	}
	fillResponseTable(table, edges, latest, nil, responseColumns(nil, edges, latest))

	if got := table.GetRowCount(); got != len(edges)+1 {
		t.Fatalf("table has %d rows, want a header plus %d edges", got, len(edges))
	}
	if got := table.GetCell(0, 0).Text; got != "IP" {
		t.Errorf("the corner cell is %q, want IP", got)
	}
	for i, edge := range edges {
		if got := table.GetCell(i+1, 0).Text; got != edge {
			t.Errorf("row %d is labelled %q, want %q", i+1, got, edge)
		}
	}
}

// A CDN that sends no Age, Expires or Via would otherwise get three columns of
// nothing, pushing what it does send off the side of the screen.
func TestResponseTableOnlyColumnsWhatCameBack(t *testing.T) {
	edges := []string{"1.1.1.1"}
	latest := map[string]*probe.Result{"1.1.1.1": sampleResult(http.StatusPartialContent)}

	table := newResponseTable()
	fillResponseTable(table, edges, latest, nil, responseColumns(nil, edges, latest))

	headers := map[string]bool{}
	for column := 0; column < table.GetColumnCount(); column++ {
		headers[table.GetCell(0, column).Text] = true
	}

	// The sample carries these.
	for _, want := range []string{"StatusCode", "Server", "Date", "ACA-Origin", "Hash", "Total"} {
		if !headers[want] {
			t.Errorf("%q carries a value but has no column: %v", want, headers)
		}
	}
	// And not these.
	for _, absent := range []string{"Age", "Expires", "Via", "Last-Modified"} {
		if headers[absent] {
			t.Errorf("%q is empty everywhere but still has a column", absent)
		}
	}
}

// A field one edge answers and another does not still earns its column, or the
// difference between the two edges would be invisible.
func TestResponseTableKeepsAColumnAnyEdgeFilled(t *testing.T) {
	withAge := sampleResult(http.StatusOK)
	withAge.Headers.Set("Age", "42")

	edges := []string{"1.1.1.1", "2.2.2.2"}
	latest := map[string]*probe.Result{
		"1.1.1.1": sampleResult(http.StatusOK),
		"2.2.2.2": withAge,
	}

	table := newResponseTable()
	fillResponseTable(table, edges, latest, nil, responseColumns(nil, edges, latest))

	column := -1
	for i := 0; i < table.GetColumnCount(); i++ {
		if table.GetCell(0, i).Text == "Age" {
			column = i
		}
	}
	if column < 0 {
		t.Fatal("no Age column, though one edge sent one")
	}
	if got := table.GetCell(1, column).Text; got != "" {
		t.Errorf("the edge without an Age shows %q, want a blank", got)
	}
	if got := table.GetCell(2, column).Text; got != "42" {
		t.Errorf("the edge with an Age shows %q, want 42", got)
	}
}

// A 503 carries fewer headers than a 200. Recomputing the columns from only
// the newest answers would drop Cache-Control and ETag the moment one came
// back, and put them back on the next 200 - a table reshaping itself under the
// eye is worse than a column of blanks.
func TestResponseColumnsKeepWhatTheyHaveEarned(t *testing.T) {
	edges := []string{"1.1.1.1"}

	good := map[string]*probe.Result{"1.1.1.1": sampleResult(http.StatusOK)}
	shown := responseColumns(nil, edges, good)

	bare := sampleResult(http.StatusServiceUnavailable)
	bare.Headers = http.Header{}
	after := responseColumns(shown, edges, map[string]*probe.Result{"1.1.1.1": bare})

	if len(after) != len(shown) {
		t.Errorf("columns went from %d to %d when an error came back", len(shown), len(after))
	}

	// A header no edge has ever sent still earns nothing.
	table := newResponseTable()
	fillResponseTable(table, edges, good, nil, after)
	for column := 0; column < table.GetColumnCount(); column++ {
		if table.GetCell(0, column).Text == "Via" {
			t.Error("Via has a column though nothing ever sent one")
		}
	}
}

func TestRecordFillsTheTableAndCountsTheRequests(t *testing.T) {
	d := testDashboard(t, []string{"1.1.1.1"})
	d.record(0, "1.1.1.1", sampleResult(http.StatusPartialContent))
	d.record(0, "1.1.1.1", sampleResult(http.StatusPartialContent))

	column := -1
	for i := 0; i < d.responseTable.GetColumnCount(); i++ {
		if d.responseTable.GetCell(0, i).Text == "StatusCode" {
			column = i
		}
	}
	if column < 0 {
		t.Fatal("no StatusCode column")
	}
	if got := d.responseTable.GetCell(1, column).Text; got != "206" {
		t.Errorf("StatusCode cell = %q, want 206", got)
	}

	// The counter moved to the title: as a column it would repeat one number
	// down every row.
	if got := d.responseTable.GetTitle(); !strings.Contains(got, "2 requests") {
		t.Errorf("title = %q, want it to carry the request count", got)
	}
}

func TestShortHash(t *testing.T) {
	sum := sha256.Sum256([]byte("payload"))

	if got := shortHash(sum[:]); len(got) != hashPrefix {
		t.Errorf("shortHash returned %d characters, want %d", len(got), hashPrefix)
	}
	if got := shortHash(nil); got != "" {
		t.Errorf("shortHash(nil) = %q, want an empty cell", got)
	}

	other := sha256.Sum256([]byte("different"))
	if shortHash(sum[:]) == shortHash(other[:]) {
		t.Error("two different bodies produced the same short hash")
	}
}

func TestStatusColor(t *testing.T) {
	tests := []struct {
		statusCode int
		want       tcell.Color
	}{
		{200, tcell.ColorGreen},
		{301, tcell.ColorBlue},
		{404, tcell.ColorYellow},
		{503, tcell.ColorRed},
		{100, tcell.ColorWhite},
	}

	for _, tt := range tests {
		if got := statusColor(tt.statusCode); got != tt.want {
			t.Errorf("statusColor(%d) = %v, want %v", tt.statusCode, got, tt.want)
		}
	}
}

// waitForOffset drives the application until the table's column offset
// satisfies want, or gives up.
//
// Queueing an update does not guarantee that keys injected before it have been
// handled: the event and the update reach the same loop by different routes.
// Asserting after a fixed number of settles therefore passes or fails
// depending on which arrived first, which is how this test came to block a
// release on a busy machine.
func waitForOffset(t *testing.T, app *tview.Application, table *tview.Table, want func(int) bool) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		_, column := offsetOf(app, table)
		if want(column) {
			return column
		}
		if time.Now().After(deadline) {
			t.Fatalf("the table settled at column %d, which is not what this was waiting for", column)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The guard is what stops tview scrolling past the start, and it is a pure
// function of the offset, so it is worth testing without an event loop at all.
func TestLeftArrowStopsAtTheStart(t *testing.T) {
	table := newResponseTable()
	fillResponseTable(table, []string{"1.1.1.1"}, map[string]*probe.Result{
		"1.1.1.1": sampleResult(http.StatusOK),
	}, nil, []int{0, 1, 2})

	capture := table.GetInputCapture()
	left := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)

	// At the start the key is swallowed: tview would decrement the offset with
	// no floor, leaving it at -1 and making the next right press look ignored.
	table.SetOffset(0, 0)
	if capture(left) != nil {
		t.Error("a left press at the start was passed through to tview")
	}

	// Away from it the key goes through and tview does the scrolling.
	table.SetOffset(0, 2)
	if capture(left) == nil {
		t.Error("a left press away from the start was swallowed")
	}
}

// The table can be wider than the screen, so the arrow keys have to scroll it.
// Asserting that focus was set would only restate the code; this drives the
// key and checks what is on screen.
func TestArrowKeysScrollTheResponseTable(t *testing.T) {
	edges := []string{"1.1.1.1"}

	// Every header filled, so the table is unambiguously wider than the
	// screen. With a sparse response it nearly fits and tview simply clamps
	// the offset back, which is correct but tests nothing.
	wide := sampleResult(http.StatusPartialContent)
	for _, h := range []string{"Cache-Control", "Age", "ETag", "Last-Modified", "Content-Length", "Content-Type", "Expires", "Via"} {
		wide.Headers.Set(h, "0123456789abcdef")
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	d := testDashboard(t, edges)
	d.app.SetScreen(screen)

	stopped := make(chan error, 1)
	go func() { stopped <- d.app.Run() }()
	defer func() {
		d.app.Stop()
		if err := <-stopped; err != nil {
			t.Errorf("app.Run: %v", err)
		}
	}()

	settle(d.app, nil)
	// Narrow enough that the later columns cannot fit.
	screen.SetSize(60, 12)
	settle(d.app, func() { d.record(0, "1.1.1.1", wide) })

	before := screenNow(d.app, screen)

	for i := 0; i < 6; i++ {
		screen.InjectKey(tcell.KeyRight, 0, tcell.ModNone)
	}
	far := waitForOffset(t, d.app, d.responseTable, func(column int) bool { return column > 0 })

	after := screenNow(d.app, screen)
	if before == after {
		t.Error("the table moved but the screen did not")
	}
	if !strings.Contains(after, "IP") || !strings.Contains(after, "1.1.1.1") {
		t.Errorf("the fixed header and address column scrolled away with the rest:\n%s", after)
	}

	// tview stops at the last column rather than scrolling into blank space.
	for i := 0; i < 40; i++ {
		screen.InjectKey(tcell.KeyRight, 0, tcell.ModNone)
	}
	end := waitForOffset(t, d.app, d.responseTable, func(column int) bool { return column >= far })
	if end < far {
		t.Errorf("scrolling further moved the table backwards, from %d to %d", far, end)
	}

	// And left brings it home and stops there.
	for i := 0; i < 60; i++ {
		screen.InjectKey(tcell.KeyLeft, 0, tcell.ModNone)
	}
	waitForOffset(t, d.app, d.responseTable, func(column int) bool { return column == 0 })
}

// A panel given a share of the leftover can be squeezed until a row falls off
// the bottom, silently. That is how the request counter went missing once, and
// how the body digest went missing from a 20-row terminal.
func TestEveryPanelKeepsItsRowsOnAShortTerminal(t *testing.T) {
	for _, height := range []int{20, 22, 24, 30} {
		out := render(t, 150, height, []string{"1.1.1.1", "2.2.2.2"})

		for _, want := range []string{"Status", "Body"} {
			if !strings.Contains(out, want) {
				t.Errorf("%d rows lose the Changes %q row:\n%s", height, want, out)
			}
		}
		// Both edges, and the counter that lives in the title.
		for _, want := range []string{"1.1.1.1", "2.2.2.2", "requests"} {
			if !strings.Contains(out, want) {
				t.Errorf("%d rows lose %q:\n%s", height, want, out)
			}
		}
	}
}

// An edge that stops answering is the thing this view exists to show. Ending
// the run on it would close the window at the moment it became worth
// watching, and take the other edges with it.
func TestAFailingEdgeDoesNotEndTheRun(t *testing.T) {
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer live.Close()

	_, port := edgeOf(t, live.URL)

	// The port comes from the URL, so the edges have to differ by address.
	// httptest binds 127.0.0.1, which leaves 127.0.0.2 refusing on the same
	// port - one edge up, one edge down, nothing else different.
	u, err := url.Parse("http://example.com:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}

	d := newDashboard(probe.New(probe.Options{Timeout: 2 * time.Second}), u, []string{"127.0.0.1", "127.0.0.2"})
	screen := tcell.NewSimulationScreen("UTF-8")
	d.app.SetScreen(screen)

	stopped := make(chan error, 1)
	go func() { stopped <- d.app.Run() }()

	ctx, cancel := context.WithCancel(context.Background())
	loop := make(chan error, 1)
	go func() { loop <- d.probeLoop(ctx) }()

	deadline := time.Now().Add(15 * time.Second)
	for {
		var answered, failed bool
		settle(d.app, func() {
			answered = len(d.latest) > 0
			failed = len(d.failed) > 0
		})
		if answered && failed {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-loop
			d.app.Stop()
			<-stopped
			t.Fatal("the loop never recorded both a success and a failure")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	if err := <-loop; err != nil {
		t.Errorf("probeLoop returned %v; a dead edge must not end the run", err)
	}

	d.app.Stop()
	<-stopped
}

// The edge that went away keeps the last thing it said, greyed, with the
// failure in its status cell - throwing the row away would lose the
// comparison that made the failure worth looking at.
func TestAFailedEdgeKeepsItsLastAnswer(t *testing.T) {
	edges := []string{"1.1.1.1"}
	latest := map[string]*probe.Result{"1.1.1.1": sampleResult(http.StatusOK)}
	shown := responseColumns(nil, edges, latest)

	table := newResponseTable()
	fillResponseTable(table, edges, latest, map[string]error{
		"1.1.1.1": errors.New("dial tcp: connection refused"),
	}, shown)

	var status, server string
	for column := 0; column < table.GetColumnCount(); column++ {
		switch table.GetCell(0, column).Text {
		case "StatusCode":
			status = table.GetCell(1, column).Text
		case "Server":
			server = table.GetCell(1, column).Text
		}
	}

	if status != "refused" {
		t.Errorf("StatusCode cell = %q, want the failure", status)
	}
	if server != "cdn" {
		t.Errorf("Server cell = %q, want the last answer kept", server)
	}
}

func TestFailureText(t *testing.T) {
	tests := map[string]error{
		"timeout": context.DeadlineExceeded,
		"refused": errors.New("dial tcp 1.2.3.4:80: connect: connection refused"),
		"no host": errors.New("lookup nope: no such host"),
		"failed":  errors.New("something else entirely"),
	}

	for want, err := range tests {
		if got := failureText(err); got != want {
			t.Errorf("failureText(%v) = %q, want %q", err, got, want)
		}
	}
}
