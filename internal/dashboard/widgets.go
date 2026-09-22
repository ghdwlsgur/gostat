package dashboard

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/ghdwlsgur/gostat/internal/probe"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

// hashPrefix is how much of the body digest a table cell shows. A column has
// no room for all 44 base64 characters, and 12 of them still pin the content.
const hashPrefix = 12

// Layout. The chart and the latency table share the left column, the response
// table and the histories the right one.
const (
	leftWidth   = 85
	totalWidth  = 180
	chartBottom = 30
	historyRows = 1
)

// tableHeight is the room termui needs for a table: two lines per row and a
// closing border. Deriving it is what keeps a row from being drawn outside its
// own panel, which is how the request counter went missing when a row was
// added to the response table.
func tableHeight(rows int) int {
	return rows*2 + 1
}

// responseRows drives the response table. Keeping the label next to the value
// it holds is what stops the two drifting apart, which fourteen hand-indexed
// assignments could not promise.
var responseRows = []struct {
	label string
	value func(*probe.Result) string
}{
	{"StatusCode", func(r *probe.Result) string { return strconv.Itoa(r.StatusCode) }},
	{"Server", header("Server")},
	{"Date", header("Date")},
	{"Last-Modified", header("Last-Modified")},
	{"ETag", header("ETag")},
	{"Age", header("Age")},
	{"Expires", header("Expires")},
	{"Cache-Control", header("Cache-Control")},
	{"Content-Type", header("Content-Type")},
	{"Content-Length", header("Content-Length")},
	{"ACA-Origin", header("Access-Control-Allow-Origin")},
	{"Via", header("Via")},
	{"Hash", func(r *probe.Result) string { return shortHash(r.BodySum) }},
	{"Total", func(r *probe.Result) string { return r.Trace.Total.String() }},
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

// style applies the shared look: white on the terminal's own background.
func style(block *ui.Block) {
	block.BorderStyle.Fg = ui.ColorWhite
	block.BorderStyle.Bg = ui.ColorClear
	block.TitleStyle.Fg = ui.ColorWhite
	block.TitleStyle.Bg = ui.ColorClear
}

func newTable(title string, rows int, columns int) *widgets.Table {
	table := widgets.NewTable()
	table.Title = title
	table.Rows = make([][]string, rows)
	for i := range table.Rows {
		table.Rows[i] = make([]string, columns)
	}
	table.TextStyle = ui.NewStyle(ui.ColorWhite)
	table.TextStyle.Bg = ui.ColorClear
	style(&table.Block)

	return table
}

// newResponseTable has one column per edge plus the label column, and one row
// per entry in responseRows plus the header and the request counter.
func newResponseTable(edges []string) *widgets.Table {
	table := newTable("Response", len(responseRows)+2, len(edges)+1)

	table.Rows[0][0] = "IP"
	copy(table.Rows[0][1:], edges)
	for i, row := range responseRows {
		table.Rows[i+1][0] = row.label
	}
	table.Rows[len(table.Rows)-1][0] = "RequestCount"
	table.SetRect(leftWidth, 0, totalWidth, responseBottom())

	return table
}

// responseBottom is where the response table ends and the histories begin.
func responseBottom() int {
	return tableHeight(len(responseRows) + 2)
}

// historyTop is where the nth history table starts, counting from zero.
func historyTop(n int) int {
	return responseBottom() + n*tableHeight(historyRows)
}

func newHistoryTable(title string, n int) *widgets.Table {
	table := newTable(title, historyRows, 2)
	table.SetRect(leftWidth, historyTop(n), totalWidth, historyTop(n+1))

	return table
}

func newLatencyTable() *widgets.Table {
	// One row per phase, plus Total. TLS only shows for https, so the table is
	// sized for the widest case and the unused row stays blank.
	const rows = 6

	table := newTable("Latency", rows, 2)
	table.SetRect(0, chartBottom, leftWidth, chartBottom+tableHeight(rows))

	return table
}

func newEdgeChart(domain string, edges []string) map[string]*widgets.StackedBarChart {
	charts := make(map[string]*widgets.StackedBarChart, len(edges))

	for _, edge := range edges {
		chart := widgets.NewStackedBarChart()
		chart.Title = fmt.Sprintf("StatusCode per Edge of %s", domain)
		// One bar per edge. Sizing this to a fixed nine used to panic on a
		// domain with a tenth A record.
		chart.Data = make([][]float64, len(edges))
		chart.Labels = edges
		chart.BarWidth = 20
		chart.SetRect(0, 0, leftWidth, chartBottom)
		chart.LabelStyles = []ui.Style{{Fg: ui.ColorWhite, Bg: ui.ColorClear, Modifier: ui.ModifierClear}}
		chart.NumStyles = []ui.Style{{Bg: ui.ColorClear, Modifier: ui.ModifierClear}}
		style(&chart.Block)

		charts[edge] = chart
	}

	return charts
}

// barColor maps a status class onto the colour its bar is drawn in.
func barColor(statusCode int) []ui.Color {
	switch statusCode / 100 {
	case 2:
		return []ui.Color{ui.ColorGreen}
	case 3:
		return []ui.Color{ui.ColorBlue}
	case 4:
		return []ui.Color{ui.ColorYellow}
	case 5:
		return []ui.Color{ui.ColorRed}
	default:
		return []ui.Color{ui.ColorWhite}
	}
}
