package internal

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fatih/color"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/tcnksm/go-httpstat"
)

// A structure that has the target URL and response latency from the URL as fields.
type Result struct {
	URL     string
	Latency int
}

// timing measures the one request the tool was asked to make, rather than a
// second plainer request alongside it.
//
// go-httpstat fills in the phases up to the first response byte. Its Total and
// ContentTransfer accessors read timestamps that the go1.8+ tracer never sets,
// so those two are taken from the clock here instead.
type timing struct {
	stat            httpstat.Result
	start           time.Time
	contentTransfer time.Duration
	total           time.Duration
}

// traceRequest attaches the tracer to req and hands back the measurement that
// will collect it.
func traceRequest(req *http.Request) (*http.Request, *timing) {
	t := &timing{start: time.Now()}
	return req.WithContext(httpstat.WithHTTPStat(req.Context(), &t.stat)), t
}

// readBody drains the response into sink and closes the measurement: the
// transfer is not over until the last byte lands. Pass io.Discard when the
// content itself is not needed.
func (t *timing) readBody(resp *http.Response, sink io.Writer) error {
	transferStart := time.Now()
	if _, err := io.Copy(sink, resp.Body); err != nil {
		return err
	}

	t.contentTransfer = time.Since(transferStart)
	t.total = time.Since(t.start)
	t.stat.End(time.Now())

	return nil
}

// stages lists what to show, in order. Every duration here is the length of
// one phase; go-httpstat's Connect, Pretransfer and StartTransfer are running
// totals from the start of the request, and adding those to a sum is what used
// to make the printed total count DNS and TCP twice.
func (t *timing) stages(protocol string) [][2]string {
	stages := [][2]string{
		{"DNS Lookup", t.stat.DNSLookup.String()},
		{"TCP Connection", t.stat.TCPConnection.String()},
	}

	if protocol == "https" {
		stages = append(stages, [2]string{"TLS Handshake", t.stat.TLSHandshake.String()})
	}

	return append(stages, [2]string{"Server Processing", t.stat.ServerProcessing.String()})
}

// Terminal ================================================================

// printLatency writes the stage-by-stage breakdown, with a running total in
// the right-hand column, and returns what it printed.
func printLatency(url string, t *timing, protocol string) Result {
	fmt.Println(color.HiWhiteString("Latency Status"))

	var elapsed time.Duration
	stage := func(name string, d time.Duration) {
		elapsed += d
		printStatusFormat(name, d.String(), elapsed.String())
	}

	stage("DNS Lookup", t.stat.DNSLookup)
	stage("TCP Connection", t.stat.TCPConnection)
	if protocol == "https" {
		stage("TLS Handshake", t.stat.TLSHandshake)
	}
	stage("Server Processing", t.stat.ServerProcessing)
	stage("Content Transfer", t.contentTransfer)

	result := Result{URL: url, Latency: int(t.total / time.Millisecond)}
	printStatusTotal(fmt.Sprintf("%dms", result.Latency))

	return result
}

// DashBoard ================================================================

func showLatencyDashBoard(t *timing, protocol string) {
	latencyTable := createLatencyTable(protocol)
	if latencyTable == nil {
		return
	}

	ui.Render(getLatencyData(t, latencyTable, protocol))
}

func createLatencyTable(protocol string) *widgets.Table {
	var labels []string
	var bottom int

	switch protocol {
	case "http":
		labels = []string{"DNS Lookup", "TCP Connection", "Server Processing", "Total"}
		bottom = 39
	case "https":
		labels = []string{"DNS Lookup", "TCP Connection", "TLS Handshake", "Server Processing", "Total"}
		bottom = 41
	default:
		return nil
	}

	latencyTable := widgets.NewTable()
	latencyTable.Rows = make([][]string, len(labels))
	for i, label := range labels {
		latencyTable.Rows[i] = make([]string, 2)
		latencyTable.Rows[i][0] = label
	}

	latencyTable.Title = "Latency"
	latencyTable.BorderStyle.Fg = 7
	latencyTable.BorderStyle.Bg = 0
	latencyTable.TitleStyle.Fg = 7
	latencyTable.TitleStyle.Bg = 0
	latencyTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	latencyTable.TextStyle.Bg = 0
	latencyTable.SetRect(0, 30, 85, bottom)

	return latencyTable
}

func getLatencyData(t *timing, latencyTable *widgets.Table, protocol string) *widgets.Table {
	for i, stage := range t.stages(protocol) {
		latencyTable.Rows[i][0] = stage[0]
		latencyTable.Rows[i][1] = stage[1]
	}

	// The last row used to be labelled "Content Transfer" while holding the
	// sum of every phase; it is the total, so it says so.
	last := len(latencyTable.Rows) - 1
	latencyTable.Rows[last][0] = "Total"
	latencyTable.Rows[last][1] = t.total.String()

	return latencyTable
}
