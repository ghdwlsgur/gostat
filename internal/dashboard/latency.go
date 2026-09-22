package dashboard

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

// phaseStyle gives each phase its colour and the short name a narrow column
// falls back to. The order is the order Trace.Phases reports them in.
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

// scaleCells turns a duration into a number of cells, rounding up so a phase
// that took measurable time never disappears from the bar.
func scaleCells(d, of time.Duration, width int) int {
	if d <= 0 || of <= 0 || width <= 0 {
		return 0
	}

	cells := int(float64(d) / float64(of) * float64(width))
	if cells == 0 {
		cells = 1
	}

	return cells
}

// compactDuration keeps a duration inside its column. Three significant
// figures is as much as a bar can justify; the exact number is in the
// response table.
func compactDuration(d time.Duration) string {
	switch {
	case d <= 0:
		// A phase that did not happen reads better as 0s than as 0ns.
		return "0s"
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

const (
	// latencyLabelWidth fits the longest phase name, "Server Processing".
	latencyLabelWidth = 18

	// latencyShortLabelWidth fits the abbreviations the chart legend uses,
	// which a narrow column falls back to rather than truncating names into
	// "Server Process…".
	latencyShortLabelWidth = 6
)

// latencyPanel breaks the last request down phase by phase: each bar is as
// wide as the share of the request that phase took, in the colours the
// per-edge chart uses, so the two panels say the same thing at two scales.
//
// A table of durations answers "how long"; this answers "where did it go",
// which is the question someone opens the dashboard with.
type latencyPanel struct {
	*tview.Box

	trace    probe.Trace
	measured bool
}

func newLatencyPanel() *latencyPanel {
	p := &latencyPanel{Box: tview.NewBox()}
	p.SetBorder(true).SetTitle(" Latency ")
	p.SetDrawFunc(p.draw)

	return p
}

func (p *latencyPanel) set(trace probe.Trace) {
	p.trace = trace
	p.measured = true
}

func (p *latencyPanel) draw(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	ix, iy, iw, ih := x+1, y+1, width-2, height-2
	if iw <= 0 || ih <= 0 || !p.measured {
		return ix, iy, iw, ih
	}

	label, short := latencyLabelWidth, false
	if iw-label-totalWidth < minBarWidth {
		label, short = latencyShortLabelWidth, true
	}

	bar := iw - label - totalWidth
	if bar < 0 {
		bar = 0
	}

	row := 0
	for _, phase := range p.trace.Phases() {
		if row >= ih {
			return ix, iy, iw, ih
		}

		style := phaseStyle[phase.Name]
		name := phase.Name
		if short {
			name = style.short
		}
		tview.Print(screen, tview.Escape(truncate(name, label)), ix, iy+row, label, tview.AlignLeft, tcell.ColorWhite)

		// Scaled against the whole request, so the bars are shares of it and
		// the widest one is the phase worth looking at.
		for i := 0; i < scaleCells(phase.Duration, p.trace.Total, bar) && i < bar; i++ {
			screen.SetContent(ix+label+i, iy+row, barRune, nil, tcell.StyleDefault.Foreground(style.color))
		}

		tview.Print(screen, tview.Escape(compactDuration(phase.Duration)), ix+label+bar, iy+row, totalWidth, tview.AlignRight, tcell.ColorWhite)
		row++
	}

	if row < ih {
		tview.Print(screen, "Total", ix, iy+row, label, tview.AlignLeft, tcell.ColorWhite)
		tview.Print(screen, tview.Escape(compactDuration(p.trace.Total)), ix+label+bar, iy+row, totalWidth, tview.AlignRight, tcell.ColorWhite)
		row++
	}

	// A pooled connection skips the phases above, and a zero there means "did
	// not happen again", not "took no time".
	if p.trace.Reused && row < ih {
		tview.Print(screen, "connection reused", ix, iy+row, iw, tview.AlignLeft, tcell.ColorGray)
		row++
	}

	// The bars are the only thing naming these colours, so the legend goes
	// with them rather than beside the status strip.
	if row < ih {
		at := ix
		for _, name := range legendOrder {
			if at = legendEntry(screen, at, iy+row, ix+iw, phaseStyle[name].color, phaseStyle[name].short); at < 0 {
				break
			}
		}
	}

	return ix, iy, iw, ih
}
