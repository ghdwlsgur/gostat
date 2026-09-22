package dashboard

import (
	"sort"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
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
