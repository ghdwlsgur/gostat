package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

// httpsTrace is an ordinary TLS request, with Content Transfer as the phase
// that took the longest.
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

// serverBoundTrace is a request where the server is what took the time, so
// there is an unambiguous phase to point at.
func serverBoundTrace() probe.Trace {
	return probe.Trace{
		DNSLookup:        10 * time.Millisecond,
		TCPConnection:    10 * time.Millisecond,
		TLSHandshake:     10 * time.Millisecond,
		ServerProcessing: 960 * time.Millisecond,
		ContentTransfer:  10 * time.Millisecond,
		Total:            time.Second,
		TLS:              true,
	}
}

// drawLatency renders the panel on its own and returns the screen contents as
// text, plus how many bar cells each row holds and in which colour.
func drawLatency(t *testing.T, width, height int, trace probe.Trace) (string, []int, []tcell.Color) {
	t.Helper()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(width, height)

	p := newLatencyPanel()
	p.set(trace)
	p.SetRect(0, 0, width, height)
	p.Draw(screen)
	screen.Show()

	cells, w, h := screen.GetContents()
	bars := make([]int, h)
	colors := make([]tcell.Color, h)

	var text strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := cells[y*w+x]
			if len(cell.Runes) == 0 {
				text.WriteRune(' ')
				continue
			}
			text.WriteRune(cell.Runes[0])
			if cell.Runes[0] == barRune {
				bars[y]++
				colors[y], _, _ = cell.Style.Decompose()
			}
		}
		text.WriteByte('\n')
	}

	return text.String(), bars, colors
}

// The bar is the share of the request a phase took, so the phase that
// dominates has to be the widest - that is the whole point of the panel.
func TestLatencyPanelBarsAreSharesOfTheRequest(t *testing.T) {
	text, bars, colors := drawLatency(t, 60, 10, serverBoundTrace())

	// Rows 1..5 are the phases, in Phases order.
	dns, tcp, tls, server, transfer := bars[1], bars[2], bars[3], bars[4], bars[5]
	for name, got := range map[string]int{"DNS": dns, "TCP": tcp, "TLS": tls, "transfer": transfer} {
		if got >= server {
			t.Errorf("%s drew %d cells and server processing %d; the dominant phase must be widest\n%s", name, got, server, text)
		}
		if got == 0 {
			t.Errorf("%s drew no cells, so a phase that took time is invisible\n%s", name, text)
		}
	}

	if colors[4] != phaseStyle["Server Processing"].color {
		t.Errorf("the server processing bar is %v, not the colour the chart legend uses", colors[4])
	}
	if colors[1] != phaseStyle["DNS Lookup"].color {
		t.Errorf("the DNS bar is %v, not the colour the chart legend uses", colors[1])
	}
}

func TestLatencyPanelNamesEveryPhaseAndTheTotal(t *testing.T) {
	text, _, _ := drawLatency(t, 60, 10, httpsTrace(3200*time.Millisecond))

	for _, want := range []string{"DNS Lookup", "TCP Connection", "TLS Handshake", "Server Processing", "Content Transfer", "Total", "3.20s"} {
		if !strings.Contains(text, want) {
			t.Errorf("the panel does not show %q:\n%s", want, text)
		}
	}
}

// A plaintext request has one phase fewer, and the row the handshake used to
// occupy must not keep showing a stale value.
func TestLatencyPanelOmitsTLSForPlaintext(t *testing.T) {
	trace := probe.Trace{
		DNSLookup:        10 * time.Millisecond,
		TCPConnection:    10 * time.Millisecond,
		ServerProcessing: 70 * time.Millisecond,
		ContentTransfer:  10 * time.Millisecond,
		Total:            100 * time.Millisecond,
	}

	text, _, _ := drawLatency(t, 60, 10, trace)

	if strings.Contains(text, "TLS Handshake") {
		t.Errorf("a plaintext request lists a TLS phase:\n%s", text)
	}
	if !strings.Contains(text, "Total") {
		t.Errorf("the total went missing:\n%s", text)
	}
}

// Zeros in the connection phases mean "did not happen again" on a pooled
// connection, and reading them as measurements would be wrong.
func TestLatencyPanelCallsOutAReusedConnection(t *testing.T) {
	reused := httpsTrace(time.Second)
	reused.Reused = true

	text, _, _ := drawLatency(t, 60, 12, reused)
	if !strings.Contains(text, "connection reused") {
		t.Errorf("a reused connection is not called out:\n%s", text)
	}

	text, _, _ = drawLatency(t, 60, 12, httpsTrace(time.Second))
	if strings.Contains(text, "connection reused") {
		t.Errorf("a fresh connection is wrongly called reused:\n%s", text)
	}
}

// Nothing has been measured before the first probe lands, and an empty panel
// is better than a row of zeros pretending to be one.
func TestLatencyPanelDrawsNothingBeforeTheFirstProbe(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 10)

	p := newLatencyPanel()
	p.SetRect(0, 0, 60, 10)
	p.Draw(screen)
	screen.Show()

	cells, w, h := screen.GetContents()
	for i := 0; i < w*h; i++ {
		if len(cells[i].Runes) > 0 && cells[i].Runes[0] == barRune {
			t.Fatal("the panel drew a bar before anything was measured")
		}
	}
}

// A narrow panel still has to name the phases and show the numbers; the bar is
// what gives way.
func TestLatencyPanelSurvivesANarrowColumn(t *testing.T) {
	for _, width := range []int{20, 30, 40, 60} {
		text, _, _ := drawLatency(t, width, 10, httpsTrace(3200*time.Millisecond))

		if !strings.Contains(text, "Total") {
			t.Errorf("%d columns lose the total:\n%s", width, text)
		}
		if !strings.Contains(text, "DNS") {
			t.Errorf("%d columns lose the phase names:\n%s", width, text)
		}
	}
}

// A narrow column falls back to the abbreviations the chart legend already
// uses, rather than truncating "Server Processing" into something unreadable
// and leaving the bar four cells wide.
func TestLatencyPanelUsesShortNamesWhenNarrow(t *testing.T) {
	text, bars, _ := drawLatency(t, 30, 10, serverBoundTrace())

	if strings.Contains(text, "…") {
		t.Errorf("a narrow panel truncates its labels:\n%s", text)
	}
	for _, want := range []string{"Srv", "Xfer", "Total"} {
		if !strings.Contains(text, want) {
			t.Errorf("the short label %q is missing:\n%s", want, text)
		}
	}

	// Row 4 is Server Processing, 96% of this request. The short labels exist
	// to leave the bar room to say so.
	widest := 0
	for _, cells := range bars {
		if cells > widest {
			widest = cells
		}
	}
	if bars[4] != widest || widest < minBarWidth {
		t.Errorf("the dominant phase drew %d cells of a widest %d:\n%s", bars[4], widest, text)
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

func TestScaleCells(t *testing.T) {
	tests := []struct {
		name  string
		d     time.Duration
		of    time.Duration
		width int
		want  int
	}{
		{"half of the whole fills half the bar", 500 * time.Millisecond, time.Second, 20, 10},
		{"the whole fills the bar", time.Second, time.Second, 20, 20},
		// A phase that took measurable time has to stay visible, or a fast
		// DNS lookup silently vanishes from the breakdown.
		{"a tiny phase still gets a cell", time.Microsecond, time.Second, 20, 1},
		{"nothing takes no cells", 0, time.Second, 20, 0},
		{"no reference takes no cells", time.Second, 0, 20, 0},
		{"no room takes no cells", time.Second, time.Second, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scaleCells(tt.d, tt.of, tt.width); got != tt.want {
				t.Errorf("scaleCells(%v, %v, %d) = %d, want %d", tt.d, tt.of, tt.width, got, tt.want)
			}
		})
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
