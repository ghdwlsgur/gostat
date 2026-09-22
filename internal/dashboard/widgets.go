package dashboard

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

const (
	// hashPrefix is how much of the body digest a cell shows. A column has no
	// room for all 44 base64 characters, and 12 of them still pin the content.
	hashPrefix = 12

	// Panel heights, in lines, borders included. Everything else is given a
	// share of whatever is left, so the view fits the terminal it is in
	// rather than demanding a particular size.

	// Five phases, the total, the reused note, and the borders.
	latencyHeight = 9

	// Status, body and the timestamp, plus the borders.
	changesHeight = 5
)

// responseRows drives the response table. Keeping a label next to the value it
// holds is what stops the two drifting apart, which a column of hand-indexed
// assignments could not promise.
var responseRows = []struct {
	label string
	value func(*probe.Result) string
	// color paints the cell when the value is worth spotting rather than
	// reading. A nil one leaves the row in the default colour.
	color func(*probe.Result) tcell.Color
}{
	{
		label: "StatusCode",
		value: func(r *probe.Result) string { return strconv.Itoa(r.StatusCode) },
		color: func(r *probe.Result) tcell.Color { return statusColor(r.StatusCode) },
	},
	{label: "Proto", value: func(r *probe.Result) string { return r.Proto }},
	{label: "Server", value: header("Server")},
	{label: "Date", value: header("Date")},
	{label: "Last-Modified", value: header("Last-Modified")},
	{label: "ETag", value: header("ETag")},
	{label: "Age", value: header("Age")},
	{label: "Expires", value: header("Expires")},
	{label: "Cache-Control", value: header("Cache-Control")},
	{label: "Content-Type", value: header("Content-Type")},
	{label: "Content-Length", value: header("Content-Length")},
	{label: "ACA-Origin", value: header("Access-Control-Allow-Origin")},
	{label: "Via", value: header("Via")},
	{label: "Hash", value: func(r *probe.Result) string { return shortHash(r.BodySum) }},
	{label: "Total", value: func(r *probe.Result) string { return r.Trace.Total.String() }},
}

func header(name string) func(*probe.Result) string {
	return func(r *probe.Result) string { return r.Header(name) }
}

// shortHash renders a body digest short enough for a table cell.
func shortHash(sum []byte) string {
	if len(sum) == 0 {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(sum)
	if len(encoded) > hashPrefix {
		return encoded[:hashPrefix]
	}
	return encoded
}

// statusColor maps a status class onto the colour it is shown in.
func statusColor(statusCode int) tcell.Color {
	switch statusCode / 100 {
	case 2:
		return tcell.ColorGreen
	case 3:
		return tcell.ColorBlue
	case 4:
		return tcell.ColorYellow
	case 5:
		return tcell.ColorRed
	default:
		return tcell.ColorWhite
	}
}

func newTable(title string) *tview.Table {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", title))

	return table
}

func labelCell(text string) *tview.TableCell {
	return tview.NewTableCell(text).
		SetTextColor(tcell.ColorWhite).
		SetAttributes(tcell.AttrBold).
		SetSelectable(false)
}

func valueCell(text string) *tview.TableCell {
	return tview.NewTableCell(text).
		SetTextColor(tcell.ColorDefault).
		SetSelectable(false).
		SetExpansion(1)
}

// newResponseTable has a label column, one column per edge, one row per entry
// in responseRows, and a header and a request counter around them. The header
// row and the label column stay put while the rest scrolls, so a terminal too
// small to show everything still shows what each value is.
func newResponseTable(edges []string) *tview.Table {
	table := newTable("Response")
	table.SetFixed(1, 1)

	table.SetCell(0, 0, labelCell("IP"))
	for i, edge := range edges {
		table.SetCell(0, i+1, labelCell(edge))
	}

	for row, spec := range responseRows {
		table.SetCell(row+1, 0, labelCell(spec.label))
		for i := range edges {
			table.SetCell(row+1, i+1, valueCell(""))
		}
	}

	last := len(responseRows) + 1
	table.SetCell(last, 0, labelCell("RequestCount"))
	for i := range edges {
		table.SetCell(last, i+1, valueCell(""))
	}

	return table
}

// requestCountRow is where newResponseTable put the counter.
func requestCountRow() int {
	return len(responseRows) + 1
}

// layout arranges the panels so every one of them gets a share of the terminal
// rather than a fixed rectangle. The old view was pinned to coordinates that
// needed 180 by 43; anything smaller drew an empty box and no data at all.
func layout(d *Dashboard, subtitle string) tview.Primitive {
	left := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.chart, 0, 1, false).
		AddItem(d.latency, latencyHeight, 0, false)

	right := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.responseTable, 0, 1, false).
		AddItem(d.changes, changesHeight, 0, false)

	body := tview.NewFlex().
		AddItem(left, 0, 2, false).
		AddItem(right, 0, 3, false)

	help := tview.NewTextView().SetDynamicColors(true)
	help.SetText(fmt.Sprintf("[white]%s  [gray]· press q to quit", subtitle))

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, false).
		AddItem(help, 1, 0, false)
}
