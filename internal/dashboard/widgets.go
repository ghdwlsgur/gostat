package dashboard

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

const (
	// hashPrefix is how much of the body digest a cell shows. A column has no
	// room for all 44 base64 characters, and 12 of them still pin the content.
	hashPrefix = 12

	// latencyHeight is five phases, the total, the reused note, the legend,
	// and the borders. Everything else is given a share of whatever is left,
	// so the view fits the terminal it is in rather than demanding a size.
	latencyHeight = 10
)

// responseFields are the columns the response table can carry. Keeping a label
// next to the value it holds is what stops the two drifting apart, which a
// column of hand-indexed assignments could not promise.
//
// The order is what is worth seeing first: the table is wider than most
// terminals, and everything past the right edge may as well not be there.
var responseFields = []struct {
	label string
	value func(*probe.Result) string
	// color paints the cell when the value is worth spotting rather than
	// reading. A nil one leaves it in the default colour.
	color func(*probe.Result) tcell.Color
}{
	{
		label: "StatusCode",
		value: func(r *probe.Result) string { return strconv.Itoa(r.StatusCode) },
		color: func(r *probe.Result) tcell.Color { return statusColor(r.StatusCode) },
	},
	{label: "Total", value: func(r *probe.Result) string { return compactDuration(r.Trace.Total) }},
	{label: "Proto", value: func(r *probe.Result) string { return r.Proto }},
	{label: "Server", value: header("Server")},
	{label: "Cache-Control", value: header("Cache-Control")},
	{label: "Age", value: header("Age")},
	{label: "ETag", value: header("ETag")},
	{label: "Last-Modified", value: header("Last-Modified")},
	{label: "Content-Length", value: header("Content-Length")},
	{label: "Content-Type", value: header("Content-Type")},
	{label: "Hash", value: func(r *probe.Result) string { return shortHash(r.BodySum) }},
	{label: "Date", value: header("Date")},
	{label: "Expires", value: header("Expires")},
	{label: "ACA-Origin", value: header("Access-Control-Allow-Origin")},
	{label: "Via", value: header("Via")},
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
		SetSelectable(false)
}

// newResponseTable is filled in by fillResponseTable once there is something
// to show. The header row and the address column stay put while the rest
// scrolls, so a table wider than the terminal still says what each value is.
func newResponseTable() *tview.Table {
	table := newTable("Response")
	table.SetFixed(1, 1)

	return table
}

// responseHeight is the room the table needs: a header, a row per edge, and
// the borders.
func responseHeight(edges int) int {
	return edges + 3
}

// responseColumns returns the field indexes worth a column, keeping the ones
// already there.
//
// A field earns its column the first time any edge answers with it, and keeps
// it afterwards. Deciding from only the newest answers would drop the
// Cache-Control and ETag columns the moment an edge returned a 503, and put
// them back on the next 200: a table reshaping itself under the eye is worse
// than a column of blanks. A header no edge ever sends still gets nothing.
func responseColumns(shown []int, edges []string, latest map[string]*probe.Result) []int {
	for i, field := range responseFields {
		if contains(shown, i) {
			continue
		}

		for _, edge := range edges {
			if res := latest[edge]; res != nil && field.value(res) != "" {
				shown = append(shown, i)
				break
			}
		}
	}
	sort.Ints(shown)

	return shown
}

// fillResponseTable rebuilds the table from the latest answer of each edge:
// one row per address, one column per field in shown.
func fillResponseTable(table *tview.Table, edges []string, latest map[string]*probe.Result, shown []int) {
	table.Clear()

	table.SetCell(0, 0, labelCell("IP"))
	for column, i := range shown {
		table.SetCell(0, column+1, labelCell(responseFields[i].label))
	}

	for row, edge := range edges {
		table.SetCell(row+1, 0, labelCell(edge))

		res := latest[edge]
		for column, i := range shown {
			field := responseFields[i]
			cell := valueCell("")
			if res != nil {
				cell = valueCell(field.value(res))
				if field.color != nil {
					cell.SetTextColor(field.color(res))
				}
			}
			table.SetCell(row+1, column+1, cell)
		}
	}
}

// layout stacks the panels so each gets a share of the terminal rather than a
// fixed rectangle. The old view was pinned to coordinates that needed 180 by
// 43; anything smaller drew an empty box and no data at all.
//
// The response table spans the full width because it is the widest thing on
// screen. It is exactly as tall as its rows, and whatever is left over is
// shared between the panels above and below rather than left to pool in one of
// them, which reads as an abandoned box.
func layout(d *Dashboard, edges int, subtitle string) tview.Primitive {
	top := tview.NewFlex().
		AddItem(d.chart, 0, 1, false).
		AddItem(d.latency, 0, 1, false)

	help := tview.NewTextView().SetDynamicColors(true)
	help.SetText(fmt.Sprintf("[white]%s  [gray]· press q to quit", subtitle))

	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(top, 0, 3, false).
		AddItem(d.responseTable, responseHeight(edges), 0, false).
		AddItem(d.changes, 0, 1, false).
		AddItem(help, 1, 0, false)
}
