package dashboard

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

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
	}

	return ix, iy, iw, ih
}
