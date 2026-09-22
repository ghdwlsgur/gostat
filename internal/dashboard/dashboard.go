// Package dashboard draws the live view: what every edge answers, how those
// answers change over time, and where each request spends its time.
package dashboard

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ghdwlsgur/gostat/internal/probe"
	"github.com/ghdwlsgur/gostat/internal/report"
)

// sweepInterval paces the probes so the view does not flicker faster than it
// can be read.
const sweepInterval = 500 * time.Millisecond

// Dashboard owns the widgets and the history behind them. Everything here is
// touched from the application's own goroutine, never from the prober.
type Dashboard struct {
	app    *tview.Application
	client *probe.Client
	url    *url.URL
	edges  []string

	requests int64
	// latest is each edge's most recent answer, which the response table is
	// rebuilt from, and columns is which fields have earned a place in it.
	latest  map[string]*probe.Result
	failed  map[string]error
	columns []int

	chart         *statusChart
	responseTable *tview.Table
	latency       *latencyPanel
	changes       *changesPanel
}

// Run takes over the terminal until q or ctrl-c, probing every edge in turn.
func Run(ctx context.Context, client *probe.Client, u *url.URL, edges []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	d := newDashboard(client, u, edges)

	d.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC || event.Rune() == 'q' {
			// Cancelling rather than stopping outright lets the prober unwind
			// first, so nothing queues a redraw against an application that
			// has already torn the screen down.
			cancel()
			return nil
		}
		return event
	})

	failed := make(chan error, 1)
	go func() {
		failed <- d.probeLoop(ctx)
		d.app.Stop()
	}()

	if err := d.app.Run(); err != nil {
		return err
	}

	return <-failed
}

func newDashboard(client *probe.Client, u *url.URL, edges []string) *Dashboard {
	d := &Dashboard{
		app:           tview.NewApplication(),
		client:        client,
		url:           u,
		edges:         edges,
		latest:        make(map[string]*probe.Result, len(edges)),
		failed:        make(map[string]error, len(edges)),
		chart:         newStatusChart(edges),
		responseTable: newResponseTable(),
		latency:       newLatencyPanel(),
		changes:       newChangesPanel(),
	}

	d.app.SetRoot(layout(d, len(edges), u.String()), true)
	// The response table is the one panel that can be wider than the screen,
	// so it takes the focus and the arrow keys scroll it. Its header row and
	// address column are fixed, so a column scrolled into view still says
	// what it is and which edge it belongs to.
	d.app.SetFocus(d.responseTable)

	return d
}

// probeLoop walks the edges until the run is cancelled or a probe fails. It
// runs on its own goroutine and touches no widget directly: every change goes
// through the application queue.
func (d *Dashboard) probeLoop(ctx context.Context) error {
	for {
		for i, edge := range d.edges {
			if ctx.Err() != nil {
				return nil
			}

			res, err := d.client.Do(ctx, d.url, edge)
			if ctx.Err() != nil {
				return nil
			}

			// An edge that stops answering is the thing this view exists to
			// show. Ending the run on it would close the window at the moment
			// it became worth watching, and take the other edges with it.
			d.app.QueueUpdateDraw(func() {
				if err != nil {
					d.recordFailure(i, edge, err)
					return
				}
				d.record(i, edge, res)
			})

			select {
			case <-time.After(sweepInterval):
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// recordFailure notes that an edge did not answer. The response table keeps
// the last thing it did say, so what changed stays visible beside the failure.
func (d *Dashboard) recordFailure(index int, edge string, err error) {
	d.requests++
	d.failed[edge] = err

	d.chart.record(edge, failedStatus)
	d.refreshResponse()
	d.changes.recordFailure(edge, report.Now())
}

// record folds one result into the widgets.
func (d *Dashboard) record(index int, edge string, res *probe.Result) {
	d.requests++
	d.latest[edge] = res
	delete(d.failed, edge)

	d.chart.record(edge, res.StatusCode)

	d.refreshResponse()
	d.latency.set(res.Trace)
	d.changes.record(edge, res.StatusCode, shortHash(res.BodySum), report.SeoulTime(res.Header("Date")))
}

func (d *Dashboard) refreshResponse() {
	d.columns = responseColumns(d.columns, d.edges, d.latest)
	fillResponseTable(d.responseTable, d.edges, d.latest, d.failed, d.columns)
	// The counter belongs in the title now that the table has a row per edge
	// rather than a column: as a column it would repeat one number down every
	// row.
	d.responseTable.SetTitle(fmt.Sprintf(" Response · %d requests ", d.requests))
}
