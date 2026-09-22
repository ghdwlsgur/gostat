package dashboard

import (
	"fmt"

	"github.com/rivo/tview"
)

// latencyHeight is five phases, the total, the reused note, the legend, and
// the borders. Every other panel is given a share of whatever is left, so the
// view fits the terminal it is in rather than demanding a particular size.
const (
	latencyHeight = 10

	// changesHeight is the two rows the panel always has, and its borders.
	// Sharing the leftover with it let a short terminal give it one row and
	// drop the body silently, which is the same way the request counter once
	// went missing.
	changesHeight = 4
)

// layout stacks the panels so each gets a share of the terminal rather than a
// fixed rectangle. The view this replaced was pinned to coordinates that
// needed 180 by 43; anything smaller drew an empty box and no data at all.
//
// The response table spans the full width because it is the widest thing on
// screen. It and the changes panel are exactly as tall as their rows; whatever
// is left over goes to the top, which is the only part that grows - one line
// per edge, and a longer history strip the wider the terminal is.
func layout(d *Dashboard, edges int, subtitle string) tview.Primitive {
	top := tview.NewFlex().
		AddItem(d.chart, 0, 1, false).
		AddItem(d.latency, 0, 1, false)

	help := tview.NewTextView().SetDynamicColors(true)
	help.SetText(fmt.Sprintf("[white]%s  [gray]· ←/→ scroll the table · q to quit", subtitle))

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(top, 0, 3, false).
		AddItem(d.responseTable, responseHeight(edges), 0, false).
		AddItem(d.changes, changesHeight, 0, false).
		AddItem(help, 1, 0, false)
}
