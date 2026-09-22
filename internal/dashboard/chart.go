package dashboard

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

// barRune fills the phase bar; sparkRunes give the recent strip its height.
const barRune = '█'

var sparkRunes = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Column widths inside the chart, in characters.
const (
	labelMaxWidth = 15
	totalWidth    = 10
	minBarWidth   = 8
	minStripWidth = 4

	// maxSamples is how much history is kept per edge. The strip draws as
	// much of it as the terminal has room for, so a wide window shows a
	// longer run rather than the same dozen samples with space to spare.
	maxSamples = 60
)

// phaseStyle gives each phase its colour and the short name the legend uses.
// The order is the order Trace.Phases reports them in.
var phaseStyle = map[string]struct {
	color tcell.Color
	short string
}{
	"DNS Lookup":        {tcell.ColorBlue, "DNS"},
	"TCP Connection":    {tcell.ColorAqua, "TCP"},
	"TLS Handshake":     {tcell.ColorFuchsia, "TLS"},
	"Server Processing": {tcell.ColorYellow, "Srv"},
	"Content Transfer":  {tcell.ColorGreen, "Xfer"},
}

var legendOrder = []string{"DNS Lookup", "TCP Connection", "TLS Handshake", "Server Processing", "Content Transfer"}

// edgeChart draws one row per edge: a strip of the status codes it answered
// with, and a bar whose coloured segments are the phases of its last request,
// scaled against the slowest edge on screen.
//
// It is drawn by hand because tview ships no chart widget, and because the
// thing worth seeing here is which edge is slow and which phase made it slow -
// a number in a table says the first but not the second.
type edgeChart struct {
	*tview.Box

	edges   []string
	samples map[string][]sample
	traces  map[string]probe.Trace
}

// sample is one probe of one edge, reduced to what the strip draws: how long
// it took and how it answered.
type sample struct {
	total      time.Duration
	statusCode int
}

func newEdgeChart(edges []string) *edgeChart {
	c := &edgeChart{
		Box:     tview.NewBox(),
		edges:   edges,
		samples: make(map[string][]sample, len(edges)),
		traces:  make(map[string]probe.Trace, len(edges)),
	}
	c.SetBorder(true).SetTitle(" Latency per edge ")
	c.SetDrawFunc(c.draw)

	return c
}

// record folds one result in, dropping the oldest sample once the strip is
// full.
func (c *edgeChart) record(edge string, statusCode int, trace probe.Trace) {
	taken := append(c.samples[edge], sample{total: trace.Total, statusCode: statusCode})
	if len(taken) > maxSamples {
		taken = taken[len(taken)-maxSamples:]
	}

	c.samples[edge] = taken
	c.traces[edge] = trace
}

// slowest is what the bars and the strips are scaled against. It covers every
// sample still on screen rather than only the newest, so one edge slowing down
// shows up as a taller strip instead of everything being redrawn to fit.
func (c *edgeChart) slowest() time.Duration {
	var max time.Duration
	for _, taken := range c.samples {
		for _, s := range taken {
			if s.total > max {
				max = s.total
			}
		}
	}

	return max
}

func (c *edgeChart) draw(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	ix, iy, iw, ih := x+1, y+1, width-2, height-2
	if iw <= 0 || ih <= 0 {
		return ix, iy, iw, ih
	}

	label := labelWidth(c.edges)
	strip, bar := split(iw - label - totalWidth)

	slowest := c.slowest()
	for row, edge := range c.edges {
		if row >= ih {
			break
		}
		c.drawRow(screen, ix, iy+row, edge, label, strip, bar, slowest)
	}

	// The legend only earns its line once every edge already has one.
	if legend := iy + len(c.edges) + 1; legend < iy+ih {
		c.drawLegend(screen, ix, legend, iw)
	}

	return ix, iy, iw, ih
}

func (c *edgeChart) drawRow(screen tcell.Screen, x, y int, edge string, label, strip, bar int, slowest time.Duration) {
	tview.Print(screen, tview.Escape(truncate(edge, label)), x, y, label, tview.AlignLeft, tcell.ColorWhite)
	at := x + label

	if strip > 0 {
		taken := c.samples[edge]
		// Right-aligned, so the newest sample is always in the same column.
		// Height is how long the request took against the slowest on screen,
		// colour is how it answered: one strip, both signals.
		for i := 0; i < strip; i++ {
			j := len(taken) - strip + i
			if j < 0 {
				at++
				continue
			}
			screen.SetContent(at, y, sparkRune(taken[j].total, slowest), nil,
				tcell.StyleDefault.Foreground(statusColor(taken[j].statusCode)))
			at++
		}
	}

	trace, ok := c.traces[edge]
	if !ok || bar == 0 {
		return
	}

	drawn := 0
	for _, phase := range trace.Phases() {
		style, known := phaseStyle[phase.Name]
		if !known {
			continue
		}

		cells := scaleCells(phase.Duration, slowest, bar)
		for i := 0; i < cells && drawn < bar; i++ {
			screen.SetContent(at+drawn, y, barRune, nil, tcell.StyleDefault.Foreground(style.color))
			drawn++
		}
	}

	// Compact, because a Duration's own string is long enough to be truncated
	// by the column - and half a number is worse than none.
	tview.Print(screen, tview.Escape(" "+compactDuration(trace.Total)), x+label+strip+bar, y, totalWidth, tview.AlignRight, tcell.ColorWhite)
}

func (c *edgeChart) drawLegend(screen tcell.Screen, x, y, width int) {
	at := x
	for _, name := range legendOrder {
		style := phaseStyle[name]
		if at+len(style.short)+2 > x+width {
			return
		}

		screen.SetContent(at, y, barRune, nil, tcell.StyleDefault.Foreground(style.color))
		tview.Print(screen, style.short, at+1, y, len(style.short), tview.AlignLeft, tcell.ColorGray)
		at += len(style.short) + 2
	}
}

// split divides the room left over after the label and the total between the
// history strip and the bar. The bar comes first: one too short to show its
// segments says nothing at all, while the strip simply disappears.
func split(free int) (strip, bar int) {
	if free <= 0 {
		return 0, 0
	}

	if free >= minBarWidth+minStripWidth {
		strip = free - minBarWidth
		if half := free / 2; strip > half {
			strip = half
		}
		if strip > maxSamples {
			strip = maxSamples
		}
	}

	return strip, free - strip
}

// sparkRune picks the block whose height stands for d against the slowest
// sample on screen.
func sparkRune(d, slowest time.Duration) rune {
	if d <= 0 || slowest <= 0 {
		return sparkRunes[0]
	}

	i := int(float64(d) / float64(slowest) * float64(len(sparkRunes)-1))
	if i >= len(sparkRunes) {
		i = len(sparkRunes) - 1
	}

	return sparkRunes[i]
}

// scaleCells turns a duration into a number of cells, rounding up so a phase
// that took measurable time never disappears from the bar.
func scaleCells(d, slowest time.Duration, width int) int {
	if d <= 0 || slowest <= 0 || width <= 0 {
		return 0
	}

	cells := int(float64(d) / float64(slowest) * float64(width))
	if cells == 0 {
		cells = 1
	}

	return cells
}

// compactDuration keeps a duration inside its column. Three significant
// figures is as much as a bar chart can justify; the exact number is in the
// latency table underneath.
func compactDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	case d >= time.Microsecond:
		return fmt.Sprintf("%.0fµs", float64(d)/float64(time.Microsecond))
	default:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
}

func labelWidth(edges []string) int {
	width := 0
	for _, edge := range edges {
		if len(edge) > width {
			width = len(edge)
		}
	}

	if width > labelMaxWidth {
		width = labelMaxWidth
	}

	return width + 1
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}

	return fmt.Sprintf("%s…", s[:width-1])
}
