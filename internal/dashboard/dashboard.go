// Package dashboard draws the live view: what every edge answers, how those
// answers change over time, and where each request spends its time.
package dashboard

import (
	"context"
	"net/url"
	"strconv"
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

	chart         *edgeChart
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
		chart:         newEdgeChart(edges),
		responseTable: newResponseTable(edges),
		latency:       newLatencyPanel(),
		changes:       newChangesPanel(),
	}

	d.app.SetRoot(layout(d, u.String()), true)

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
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}

			d.app.QueueUpdateDraw(func() { d.record(i, edge, res) })

			select {
			case <-time.After(sweepInterval):
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// record folds one result into the widgets.
func (d *Dashboard) record(index int, edge string, res *probe.Result) {
	d.requests++

	d.chart.record(edge, res.StatusCode, res.Trace)

	for row, spec := range responseRows {
		cell := d.responseTable.GetCell(row+1, index+1).SetText(spec.value(res))
		if spec.color != nil {
			cell.SetTextColor(spec.color(res))
		}
	}
	d.responseTable.GetCell(requestCountRow(), index+1).
		SetText(strconv.FormatInt(d.requests, 10))

	d.latency.set(res.Trace)
	d.changes.record(res.StatusCode, shortHash(res.BodySum), report.SeoulTime(res.Header("Date")))
}
