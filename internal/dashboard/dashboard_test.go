package dashboard

import (
	"crypto/sha256"
	"net/http"
	"net/url"
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
func renderScreen(t *testing.T, width, height int, edges []string) (tcell.SimulationScreen, func()) {
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

	return screen, func() {
		d.app.Stop()
		if err := <-stopped; err != nil {
			t.Errorf("app.Run: %v", err)
		}
	}
}

// render returns what a terminal of the given size would show.
func render(t *testing.T, width, height int, edges []string) string {
	t.Helper()

	screen, stop := renderScreen(t, width, height, edges)
	defer stop()

	return screenText(screen)
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
	fillResponseTable(table, edges, latest, responseColumns(nil, edges, latest))

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
	fillResponseTable(table, edges, latest, responseColumns(nil, edges, latest))

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
	fillResponseTable(table, edges, latest, responseColumns(nil, edges, latest))

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
	fillResponseTable(table, edges, good, after)
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
