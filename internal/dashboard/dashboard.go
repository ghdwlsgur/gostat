// Package dashboard draws the live view: what every edge answers, how those
// answers change over time, and where each request spends its time.
package dashboard

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
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
	latencyTable  *tview.Table
	statusSeen    *seen
	changedAt     *seen
	hashSeen      *seen
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
		latencyTable:  newLatencyTable(),
		statusSeen:    newSeen("StatusCode"),
		changedAt:     newSeen("Time"),
		hashSeen:      newSeen("Hash"),
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
		d.responseTable.GetCell(row+1, index+1).SetText(spec.value(res))
	}
	d.responseTable.GetCell(requestCountRow(), index+1).
		SetText(strconv.FormatInt(d.requests, 10))

	d.fillLatency(res.Trace)

	// A status not seen before is worth a timestamp; the same one repeating
	// is not.
	if d.statusSeen.add(strconv.Itoa(res.StatusCode)) {
		d.changedAt.add(report.SeoulTime(res.Header("Date")))
	}
	d.hashSeen.add(shortHash(res.BodySum))
}

// fillLatency rewrites the latency table for one request, clearing the row a
// plaintext request leaves unused rather than letting a stale TLS handshake
// sit there.
func (d *Dashboard) fillLatency(trace probe.Trace) {
	d.latencyTable.Clear()

	phases := trace.Phases()
	for i, phase := range phases {
		d.latencyTable.SetCell(i, 0, labelCell(phase.Name))
		d.latencyTable.SetCell(i, 1, valueCell(phase.Duration.String()))
	}

	d.latencyTable.SetCell(len(phases), 0, labelCell("Total"))
	d.latencyTable.SetCell(len(phases), 1, valueCell(trace.Total.String()))

	if trace.Reused {
		// A pooled connection skips the phases above, and a zero there means
		// "did not happen again", not "took no time".
		d.latencyTable.SetCell(len(phases)+1, 0, labelCell(""))
		d.latencyTable.SetCell(len(phases)+1, 1, valueCell("connection reused"))
	}
}

// seen is the ordered set of distinct values a field has taken, shown as a
// one-line history strip.
type seen struct {
	title  string
	values []string
	view   *tview.TextView
}

func newSeen(title string) *seen {
	s := &seen{title: title, view: newHistoryView(title)}
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
	if len(s.values) == 0 {
		s.view.SetText(fmt.Sprintf("[gray]no %s yet", strings.ToLower(s.title)))
		return
	}

	s.view.SetText(strings.Join(s.values, "   "))
}

// list returns the values recorded so far, as a copy.
func (s *seen) list() []string {
	out := make([]string, len(s.values))
	copy(out, s.values)

	return out
}
