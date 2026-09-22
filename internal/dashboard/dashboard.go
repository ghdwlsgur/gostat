// Package dashboard draws the live termui view: what every edge answers, how
// those answers change over time, and where each request spends its time.
package dashboard

import (
	"context"
	"net/url"
	"strconv"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/ghdwlsgur/gostat/internal/probe"
	"github.com/ghdwlsgur/gostat/internal/report"
)

// redrawInterval keeps a sweep from spinning faster than the eye can follow.
const redrawInterval = 500 * time.Millisecond

// chartHistory is how many samples one edge keeps before its chart restarts.
const chartHistory = 9

// Dashboard holds the widgets and the running history behind them.
type Dashboard struct {
	client *probe.Client
	url    *url.URL
	edges  []string

	requests int64

	charts        map[string]*widgets.StackedBarChart
	responseTable *widgets.Table
	latencyTable  *widgets.Table
	statusSeen    *seen
	hashSeen      *seen
	changedAt     *seen
}

// Run takes over the terminal until q or ctrl-c, probing every edge in turn.
func Run(ctx context.Context, client *probe.Client, u *url.URL, edges []string) error {
	if err := ui.Init(); err != nil {
		return err
	}
	// Returning rather than exiting is what lets this run and hand the
	// terminal back in the state it was found.
	defer ui.Close()

	d := &Dashboard{
		client:        client,
		url:           u,
		edges:         edges,
		charts:        newEdgeChart(u.Hostname(), edges),
		responseTable: newResponseTable(edges),
		latencyTable:  newLatencyTable(),
		statusSeen:    newSeen("StatusCode"),
		hashSeen:      newSeen("Hash"),
		changedAt:     newSeen("Time"),
	}

	return d.loop(ctx)
}

func (d *Dashboard) loop(ctx context.Context) error {
	events := ui.PollEvents()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-events:
			if e.Type == ui.KeyboardEvent && (e.ID == "q" || e.ID == "<C-c>") {
				return nil
			}
		default:
			if err := d.sweep(ctx); err != nil {
				return err
			}
		}
	}
}

// sweep probes every edge once, redrawing after each one.
func (d *Dashboard) sweep(ctx context.Context) error {
	for i, edge := range d.edges {
		res, err := d.client.Do(ctx, d.url, edge)
		if err != nil {
			return err
		}

		d.requests++
		d.record(i, edge, res)
		d.render(edge)

		<-time.After(redrawInterval)
	}

	return nil
}

// record folds one result into the widgets.
func (d *Dashboard) record(index int, edge string, res *probe.Result) {
	chart := d.charts[edge]
	chart.BarColors = barColor(res.StatusCode)
	chart.Data[index] = append(chart.Data[index], float64(res.StatusCode))

	for row, spec := range responseRows {
		d.responseTable.Rows[row+1][index+1] = spec.value(res)
	}
	d.responseTable.Rows[len(d.responseTable.Rows)-1][index+1] = strconv.FormatInt(d.requests, 10)

	fillLatencyTable(d.latencyTable, res.Trace)

	// A new status is worth a timestamp; the same status repeating is not.
	statusChanged := d.statusSeen.add(strconv.Itoa(res.StatusCode))
	d.hashSeen.add(shortHash(res.BodySum))
	if statusChanged {
		d.changedAt.add(report.SeoulTime(res.Header("Date")))
	}

	// Every chart restarts together, so the bars stay comparable.
	if len(chart.Data[index]) >= chartHistory && index == len(d.edges)-1 {
		for _, c := range d.charts {
			c.Data = make([][]float64, len(d.edges))
		}
	}
}

func (d *Dashboard) render(edge string) {
	ui.Render(
		d.charts[edge],
		d.responseTable,
		d.latencyTable,
		d.statusSeen.table,
		d.changedAt.table,
		d.hashSeen.table,
	)
}

// fillLatencyTable writes the phases of one request, blanking the rows a
// plaintext request does not use.
func fillLatencyTable(table *widgets.Table, trace probe.Trace) {
	phases := trace.Phases()

	for i := range table.Rows {
		table.Rows[i][0] = ""
		table.Rows[i][1] = ""
	}

	for i, phase := range phases {
		table.Rows[i][0] = phase.Name
		table.Rows[i][1] = phase.Duration.String()
	}

	total := len(phases)
	table.Rows[total][0] = "Total"
	table.Rows[total][1] = trace.Total.String()
}

// seen is an ordered set of the distinct values a field has taken, rendered as
// a one-row history table.
type seen struct {
	values []string
	table  *widgets.Table
}

var historyRects = map[string][2]int{
	"StatusCode": {31, 34},
	"Time":       {34, 37},
	"Hash":       {37, 40},
}

func newSeen(title string) *seen {
	rect := historyRects[title]

	s := &seen{
		values: []string{title},
		table:  newHistoryTable(title+" History", rect[0], rect[1]),
	}
	s.sync()

	return s
}

// add records value and reports whether it had not been seen before.
func (s *seen) add(value string) bool {
	for _, existing := range s.values {
		if existing == value {
			return false
		}
	}

	s.values = append(s.values, value)
	s.sync()

	return true
}

func (s *seen) sync() {
	row := make([]string, len(s.values))
	copy(row, s.values)
	s.table.Rows = [][]string{row}
}
