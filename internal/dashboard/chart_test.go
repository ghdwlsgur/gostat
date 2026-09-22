package dashboard

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

// traceOfTotal builds a trace whose phases add up to the total, the way a real
// measurement does. Scaling only Total while the phases stay put would leave
// two edges with identical bars, since the bar is drawn from the phases.
func traceOfTotal(total time.Duration) probe.Trace {
	return probe.Trace{
		DNSLookup:        total / 10,
		TCPConnection:    total / 10,
		TLSHandshake:     total / 10,
		ServerProcessing: total * 6 / 10,
		ContentTransfer:  total / 10,
		Total:            total,
		TLS:              true,
	}
}

func httpsTrace(total time.Duration) probe.Trace {
	return probe.Trace{
		DNSLookup:        1 * time.Millisecond,
		TCPConnection:    10 * time.Millisecond,
		TLSHandshake:     100 * time.Millisecond,
		ServerProcessing: 1 * time.Second,
		ContentTransfer:  2 * time.Second,
		Total:            total,
		TLS:              true,
	}
}

// The strip keeps the last few codes; without a cap it would grow until it ran
// off the side of the panel.
func TestChartCapsItsHistory(t *testing.T) {
	c := newEdgeChart([]string{"1.1.1.1"})

	for i := 0; i < maxSamples+5; i++ {
		c.record("1.1.1.1", http.StatusOK, httpsTrace(time.Second))
	}

	if got := len(c.samples["1.1.1.1"]); got != maxSamples {
		t.Errorf("kept %d samples, want %d", got, maxSamples)
	}
}

// Every bar is scaled against the slowest edge, so two bars can be compared
// with each other rather than each filling its own row.
func TestChartScalesAgainstTheSlowestEdge(t *testing.T) {
	c := newEdgeChart([]string{"1.1.1.1", "2.2.2.2"})
	c.record("1.1.1.1", http.StatusOK, httpsTrace(1*time.Second))
	c.record("2.2.2.2", http.StatusOK, httpsTrace(4*time.Second))

	if got := c.slowest(); got != 4*time.Second {
		t.Errorf("slowest() = %v, want 4s", got)
	}
}

// The strip is the only place the run's history shows, so a slower request has
// to draw a taller block than a fast one.
func TestSparkRune(t *testing.T) {
	tests := []struct {
		name    string
		d       time.Duration
		slowest time.Duration
		want    rune
	}{
		{"the slowest fills the cell", time.Second, time.Second, '█'},
		{"nothing draws the shortest block", 0, time.Second, '▁'},
		{"no reference draws the shortest block", time.Second, 0, '▁'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sparkRune(tt.d, tt.slowest); got != tt.want {
				t.Errorf("sparkRune(%v, %v) = %q, want %q", tt.d, tt.slowest, got, tt.want)
			}
		})
	}

	// Taller for slower, across the whole range.
	prev := sparkRune(0, time.Second)
	for _, d := range []time.Duration{200, 400, 600, 800, 1000} {
		got := sparkRune(d*time.Millisecond, time.Second)
		if got < prev {
			t.Errorf("sparkRune shrank at %v: %q after %q", d*time.Millisecond, got, prev)
		}
		prev = got
	}
}

// One slow sample has to keep showing after a fast one lands, or the strip
// rescales itself flat and the spike disappears.
func TestChartScalesAgainstEverySampleOnScreen(t *testing.T) {
	c := newEdgeChart([]string{"1.1.1.1"})
	c.record("1.1.1.1", http.StatusOK, httpsTrace(4*time.Second))
	c.record("1.1.1.1", http.StatusOK, httpsTrace(1*time.Second))

	if got := c.slowest(); got != 4*time.Second {
		t.Errorf("slowest() = %v, want the 4s spike still in the window", got)
	}
}

func TestScaleCells(t *testing.T) {
	tests := []struct {
		name    string
		d       time.Duration
		slowest time.Duration
		width   int
		want    int
	}{
		{"half of the slowest fills half the bar", 500 * time.Millisecond, time.Second, 20, 10},
		{"the slowest fills the bar", time.Second, time.Second, 20, 20},
		// A phase that took measurable time has to stay visible, or a fast
		// DNS lookup silently vanishes from the breakdown.
		{"a tiny phase still gets a cell", time.Microsecond, time.Second, 20, 1},
		{"nothing takes no cells", 0, time.Second, 20, 0},
		{"no reference takes no cells", time.Second, 0, 20, 0},
		{"no room takes no cells", time.Second, time.Second, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scaleCells(tt.d, tt.slowest, tt.width); got != tt.want {
				t.Errorf("scaleCells(%v, %v, %d) = %d, want %d", tt.d, tt.slowest, tt.width, got, tt.want)
			}
		})
	}
}

// The bar has to survive a narrow panel even when the strip cannot: a bar too
// short to show its segments says nothing at all.
func TestSplit(t *testing.T) {
	tests := []struct {
		name      string
		free      int
		wantStrip int
		wantBar   int
	}{
		{"nothing to give", 0, 0, 0},
		{"too little for both, the bar takes it", 10, 0, 10},
		{"enough for both, split evenly", 20, 10, 10},
		{"plenty, the strip stops at half", 56, 28, 28},
		{"more than there is history, the strip stops there", 200, maxSamples, 200 - maxSamples},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strip, bar := split(tt.free)
			if strip != tt.wantStrip || bar != tt.wantBar {
				t.Errorf("split(%d) = (%d, %d), want (%d, %d)", tt.free, strip, bar, tt.wantStrip, tt.wantBar)
			}
			if strip+bar != tt.free {
				t.Errorf("split(%d) accounts for %d columns", tt.free, strip+bar)
			}
			if tt.free >= minBarWidth && bar < minBarWidth {
				t.Errorf("split(%d) left the bar %d wide, under the %d minimum", tt.free, bar, minBarWidth)
			}
		})
	}
}

func TestLabelWidth(t *testing.T) {
	if got, want := labelWidth([]string{"1.1.1.1", "255.255.255.255"}), len("255.255.255.255")+1; got != want {
		t.Errorf("labelWidth = %d, want %d", got, want)
	}
	// A long name is capped rather than allowed to crowd out the bar.
	if got := labelWidth([]string{strings.Repeat("a", 40)}); got != labelMaxWidth+1 {
		t.Errorf("labelWidth = %d, want the cap %d", got, labelMaxWidth+1)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"1.1.1.1", 10, "1.1.1.1"},
		{"averylongedgename", 8, "averylo…"},
		{"abc", 0, ""},
		{"abc", 1, "a"},
	}

	for _, tt := range tests {
		if got := truncate(tt.in, tt.width); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}

// Every phase the trace reports has to have a colour, or a segment is silently
// dropped from the bar.
func TestEveryPhaseHasAStyle(t *testing.T) {
	for _, phase := range httpsTrace(time.Second).Phases() {
		if _, ok := phaseStyle[phase.Name]; !ok {
			t.Errorf("phase %q has no colour, so it would not be drawn", phase.Name)
		}
	}

	for _, name := range legendOrder {
		if _, ok := phaseStyle[name]; !ok {
			t.Errorf("legend names %q, which has no colour", name)
		}
	}
}

// The bar is the point of the panel, so it has to survive a narrow terminal
// even when the status strip cannot.
func TestChartDrawsBarsAtEveryWidth(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {80, 24}, {120, 40}, {200, 60}} {
		out := render(t, size[0], size[1], []string{"1.1.1.1"})

		if !strings.ContainsRune(out, barRune) {
			t.Errorf("%dx%d draws no bar:\n%s", size[0], size[1], out)
		}
		if !strings.Contains(out, "Latency per edge") {
			t.Errorf("%dx%d has no chart panel:\n%s", size[0], size[1], out)
		}
	}
}

func TestChartDrawsTheLegendWhenThereIsRoom(t *testing.T) {
	out := render(t, 200, 60, []string{"1.1.1.1"})

	for _, want := range []string{"DNS", "TCP", "TLS", "Srv", "Xfer"} {
		if !strings.Contains(out, want) {
			t.Errorf("the legend is missing %q:\n%s", want, out)
		}
	}
}

// A bar drawn in one colour would say which edge is slow but not why.
func TestChartColoursThePhases(t *testing.T) {
	screen, stop := renderScreen(t, 120, 30, []string{"1.1.1.1"})
	defer stop()

	cells, width, height := screen.GetContents()
	seen := map[tcell.Color]bool{}
	for i := 0; i < width*height; i++ {
		if len(cells[i].Runes) > 0 && cells[i].Runes[0] == barRune {
			fg, _, _ := cells[i].Style.Decompose()
			seen[fg] = true
		}
	}

	for _, name := range []string{"DNS Lookup", "TCP Connection", "TLS Handshake", "Server Processing", "Content Transfer"} {
		if !seen[phaseStyle[name].color] {
			t.Errorf("no bar cell is drawn in the colour for %s", name)
		}
	}
}

// A Duration's own string is long enough to be truncated by the column, and a
// number missing its leading digits reads as a different number.
func TestCompactDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{23782667 * time.Nanosecond, "23.8ms"},
		{1500 * time.Millisecond, "1.50s"},
		{436083 * time.Nanosecond, "436µs"},
		{900 * time.Nanosecond, "900ns"},
		// A phase that did not happen, such as DNS when an address was dialled.
		{0, "0s"},
	}

	for _, tt := range tests {
		got := compactDuration(tt.d)
		if got != tt.want {
			t.Errorf("compactDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
		if len(got)+1 > totalWidth {
			t.Errorf("compactDuration(%v) = %q, too wide for the %d-wide column", tt.d, got, totalWidth)
		}
	}
}

// drawChart renders a chart on its own and returns, per row, how many bar
// cells it drew.
func drawChart(t *testing.T, width, height int, c *edgeChart) []int {
	t.Helper()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(width, height)

	c.SetRect(0, 0, width, height)
	c.Draw(screen)
	screen.Show()

	cells, w, h := screen.GetContents()
	rows := make([]int, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if cell := cells[y*w+x]; len(cell.Runes) > 0 && cell.Runes[0] == barRune {
				rows[y]++
			}
		}
	}

	return rows
}

// The panel exists to answer "which edge is slow", so a slower edge has to get
// a visibly longer bar than a faster one.
func TestChartDrawsALongerBarForASlowerEdge(t *testing.T) {
	c := newEdgeChart([]string{"1.1.1.1", "2.2.2.2"})
	c.record("1.1.1.1", http.StatusOK, traceOfTotal(1*time.Second))
	c.record("2.2.2.2", http.StatusOK, traceOfTotal(4*time.Second))

	rows := drawChart(t, 100, 10, c)
	fast, slow := rows[1], rows[2]

	if slow <= fast {
		t.Fatalf("the 4s edge drew %d bar cells and the 1s edge %d; the slower one must be longer", slow, fast)
	}
	// Four times the latency, so roughly four times the bar. Rounding up keeps
	// small phases visible, which pads the short bar, hence the loose bound.
	if fast == 0 || slow < fast*2 {
		t.Errorf("bars are %d and %d cells for a 4x difference in latency", fast, slow)
	}
}

// The legend row must not be mistaken for an edge row.
func TestChartDrawsOneRowPerEdge(t *testing.T) {
	c := newEdgeChart([]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"})
	for _, edge := range c.edges {
		c.record(edge, http.StatusOK, traceOfTotal(time.Second))
	}

	rows := drawChart(t, 100, 12, c)

	drawn := 0
	for _, cells := range rows {
		if cells > 0 {
			drawn++
		}
	}

	// Three edge rows plus the legend, which is drawn with the same rune.
	if drawn != len(c.edges)+1 {
		t.Errorf("%d rows carry bar cells, want %d edges plus the legend", drawn, len(c.edges)+1)
	}
}

// The strip is coloured by status class and nothing else on screen says so.
// Naming only the classes actually seen keeps the legend short and makes it
// grow exactly when something starts answering differently.
func TestChartLegendNamesTheStatusClassesSeen(t *testing.T) {
	c := newEdgeChart([]string{"1.1.1.1"})
	c.record("1.1.1.1", http.StatusOK, traceOfTotal(time.Second))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(100, 10)
	c.SetRect(0, 0, 100, 10)
	c.Draw(screen)
	screen.Show()

	read := func() string {
		cells, w, h := screen.GetContents()
		var b strings.Builder
		for i := 0; i < w*h; i++ {
			if len(cells[i].Runes) > 0 {
				b.WriteRune(cells[i].Runes[0])
			}
		}
		return b.String()
	}

	text := read()
	if !strings.Contains(text, "2xx") {
		t.Errorf("the legend does not name the 2xx it has seen:\n%s", text)
	}
	if strings.Contains(text, "5xx") {
		t.Errorf("the legend names a class that never happened:\n%s", text)
	}

	c.record("1.1.1.1", http.StatusServiceUnavailable, traceOfTotal(time.Second))
	c.Draw(screen)
	screen.Show()

	if text := read(); !strings.Contains(text, "5xx") {
		t.Errorf("the legend did not pick up the 503:\n%s", text)
	}
}

func TestClassesSeenAreSortedAndDistinct(t *testing.T) {
	c := newEdgeChart([]string{"1.1.1.1", "2.2.2.2"})
	c.record("2.2.2.2", http.StatusServiceUnavailable, traceOfTotal(time.Second))
	c.record("1.1.1.1", http.StatusOK, traceOfTotal(time.Second))
	c.record("1.1.1.1", http.StatusOK, traceOfTotal(time.Second))
	c.record("2.2.2.2", http.StatusNotFound, traceOfTotal(time.Second))

	want := []int{2, 4, 5}
	got := c.classesSeen()
	if len(got) != len(want) {
		t.Fatalf("classesSeen() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("classesSeen()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
