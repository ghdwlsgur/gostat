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

// render draws the dashboard onto a simulation screen of the given size and
// returns what a terminal would show.
func render(t *testing.T, width, height int, edges []string) string {
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

	out := screenText(screen)

	d.app.Stop()
	if err := <-stopped; err != nil {
		t.Fatalf("app.Run: %v", err)
	}

	return out
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
		"Response", "Status per edge", "Latency",
		"StatusCode History", "Time History", "Hash History",
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

func TestResponseTableMatchesTheRowSpecs(t *testing.T) {
	edges := []string{"1.1.1.1", "2.2.2.2"}
	table := newResponseTable(edges)

	// One header row, one row per spec, one request counter.
	if want := len(responseRows) + 2; table.GetRowCount() != want {
		t.Fatalf("table has %d rows, want %d", table.GetRowCount(), want)
	}
	if want := len(edges) + 1; table.GetColumnCount() != want {
		t.Fatalf("table has %d columns, want %d", table.GetColumnCount(), want)
	}

	if got := table.GetCell(0, 0).Text; got != "IP" {
		t.Errorf("header label = %q, want IP", got)
	}
	for i, edge := range edges {
		if got := table.GetCell(0, i+1).Text; got != edge {
			t.Errorf("header column %d = %q, want %q", i+1, got, edge)
		}
	}
	for i, spec := range responseRows {
		if got := table.GetCell(i+1, 0).Text; got != spec.label {
			t.Errorf("row %d label = %q, want %q", i+1, got, spec.label)
		}
	}
	if got := table.GetCell(requestCountRow(), 0).Text; got != "RequestCount" {
		t.Errorf("counter row label = %q, want RequestCount", got)
	}
}

func TestResponseRowsReadTheResult(t *testing.T) {
	res := sampleResult(http.StatusOK)

	values := map[string]string{}
	for _, spec := range responseRows {
		values[spec.label] = spec.value(res)
	}

	for label, want := range map[string]string{
		"StatusCode": "200",
		"Proto":      "HTTP/2.0",
		"Server":     "cdn",
		"ACA-Origin": "*",
		"Total":      "3.2s",
	} {
		if values[label] != want {
			t.Errorf("%s = %q, want %q", label, values[label], want)
		}
	}

	// An absent header is blank rather than missing, so the column stays put.
	if values["Via"] != "" {
		t.Errorf("Via = %q, want an empty cell", values["Via"])
	}
}

func TestRecordFillsTheRowAndTheCounter(t *testing.T) {
	d := testDashboard(t, []string{"1.1.1.1"})
	d.record(0, "1.1.1.1", sampleResult(http.StatusPartialContent))
	d.record(0, "1.1.1.1", sampleResult(http.StatusPartialContent))

	if got := d.responseTable.GetCell(1, 1).Text; got != "206" {
		t.Errorf("StatusCode cell = %q, want 206", got)
	}
	if got := d.responseTable.GetCell(requestCountRow(), 1).Text; got != "2" {
		t.Errorf("RequestCount cell = %q, want 2", got)
	}
}

// The strip keeps the last few codes; without a cap it would grow until it ran
// off the side of the panel.
func TestRecentStatusesAreCapped(t *testing.T) {
	d := testDashboard(t, []string{"1.1.1.1"})

	for i := 0; i < recentStatuses+5; i++ {
		d.recordStatus(0, "1.1.1.1", http.StatusOK)
	}

	if got := len(d.recent["1.1.1.1"]); got != recentStatuses {
		t.Errorf("kept %d statuses, want %d", got, recentStatuses)
	}
	if got := d.statusTable.GetColumnCount(); got != recentStatuses+1 {
		t.Errorf("strip is %d columns wide, want %d", got, recentStatuses+1)
	}
}

func TestRecentStatusesAreColouredByClass(t *testing.T) {
	d := testDashboard(t, []string{"1.1.1.1"})
	d.recordStatus(0, "1.1.1.1", http.StatusServiceUnavailable)

	// SetTextColor writes into Style; Color is the legacy field and stays
	// at its default.
	fg, _, _ := d.statusTable.GetCell(0, 1).Style.Decompose()
	if fg != tcell.ColorRed {
		t.Errorf("a 503 is drawn in %v, want red", fg)
	}
}

func TestFillLatency(t *testing.T) {
	d := testDashboard(t, []string{"1.1.1.1"})
	d.fillLatency(sampleResult(http.StatusOK).Trace)

	want := [][2]string{
		{"DNS Lookup", "1ms"},
		{"TCP Connection", "10ms"},
		{"TLS Handshake", "100ms"},
		{"Server Processing", "1s"},
		{"Content Transfer", "2s"},
		{"Total", "3.2s"},
	}
	for i, row := range want {
		if got := [2]string{d.latencyTable.GetCell(i, 0).Text, d.latencyTable.GetCell(i, 1).Text}; got != row {
			t.Errorf("row %d = %v, want %v", i, got, row)
		}
	}

	// A plaintext request has one phase fewer - DNS, TCP, Server Processing,
	// Content Transfer - so Total moves up a row and the handshake row must
	// not keep showing a stale value.
	d.fillLatency(probe.Trace{Total: time.Second})
	if got := d.latencyTable.GetRowCount(); got != 5 {
		t.Errorf("a plaintext trace left %d rows, want 5", got)
	}
	if got := d.latencyTable.GetCell(4, 0).Text; got != "Total" {
		t.Errorf("total row = %q, want it to move up", got)
	}
	for i := 0; i < 5; i++ {
		if d.latencyTable.GetCell(i, 0).Text == "TLS Handshake" {
			t.Error("the TLS row survived a plaintext request")
		}
	}
}

func TestFillLatencyCallsOutAReusedConnection(t *testing.T) {
	d := testDashboard(t, []string{"1.1.1.1"})

	d.fillLatency(probe.Trace{Total: time.Second, Reused: true})
	if !strings.Contains(d.latencyTable.GetCell(5, 1).Text, "reused") {
		t.Error("a reused connection is not called out, so its zeros read as measurements")
	}

	d.fillLatency(probe.Trace{Total: time.Second})
	if got := d.latencyTable.GetRowCount(); got != 5 {
		t.Errorf("the reused note was left behind: %d rows", got)
	}
}

func TestSeenKeepsDistinctValuesInOrder(t *testing.T) {
	s := newSeen("StatusCode")

	if !s.add("200") {
		t.Error("the first value was not reported as new")
	}
	if s.add("200") {
		t.Error("a repeated value was reported as new")
	}
	s.add("404")

	want := []string{"200", "404"}
	got := s.list()
	if len(got) != len(want) {
		t.Fatalf("history = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("history[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if !strings.Contains(s.view.GetText(true), "404") {
		t.Errorf("the strip does not show the new value: %q", s.view.GetText(true))
	}
}

func TestSeenHandsOutACopy(t *testing.T) {
	s := newSeen("Hash")
	s.add("abc")

	s.list()[0] = "tampered"

	if s.list()[0] != "abc" {
		t.Error("list shares its backing array with the set")
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
