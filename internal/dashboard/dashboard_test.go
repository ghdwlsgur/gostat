package dashboard

import (
	"crypto/sha256"
	"net/http"
	"testing"
	"time"

	"github.com/ghdwlsgur/gostat/internal/probe"
	ui "github.com/gizak/termui/v3"
)

func sampleResult(status int) *probe.Result {
	sum := sha256.Sum256([]byte("payload"))

	return &probe.Result{
		Edge:       "1.2.3.4",
		StatusCode: status,
		Headers: http.Header{
			"Server":                      []string{"cdn"},
			"Date":                        []string{"Mon, 02 Oct 2023 00:00:00 GMT"},
			"Access-Control-Allow-Origin": []string{"*"},
		},
		BodySum: sum[:],
		Trace:   probe.Trace{Total: 1500 * time.Millisecond},
	}
}

// Every response row has to fit the table the widgets build, or a value lands
// under the wrong label.
func TestResponseTableMatchesTheRowSpecs(t *testing.T) {
	edges := []string{"1.1.1.1", "2.2.2.2"}
	table := newResponseTable(edges)

	// One header row, one row per spec, one request counter.
	if want := len(responseRows) + 2; len(table.Rows) != want {
		t.Fatalf("table has %d rows, want %d", len(table.Rows), want)
	}
	for i, row := range table.Rows {
		if len(row) != len(edges)+1 {
			t.Errorf("row %d has %d cells, want %d", i, len(row), len(edges)+1)
		}
	}

	if table.Rows[0][0] != "IP" || table.Rows[0][1] != "1.1.1.1" || table.Rows[0][2] != "2.2.2.2" {
		t.Errorf("header row = %v", table.Rows[0])
	}
	for i, spec := range responseRows {
		if table.Rows[i+1][0] != spec.label {
			t.Errorf("row %d label = %q, want %q", i+1, table.Rows[i+1][0], spec.label)
		}
	}
	if last := table.Rows[len(table.Rows)-1][0]; last != "RequestCount" {
		t.Errorf("last row label = %q, want RequestCount", last)
	}
}

func TestResponseRowsReadTheResult(t *testing.T) {
	res := sampleResult(http.StatusOK)

	values := map[string]string{}
	for _, spec := range responseRows {
		values[spec.label] = spec.value(res)
	}

	for label, want := range map[string]string{
		"StatusCode": "200",
		"Server":     "cdn",
		"ACA-Origin": "*",
		"Total":      "1.5s",
	} {
		if values[label] != want {
			t.Errorf("%s = %q, want %q", label, values[label], want)
		}
	}

	// An absent header is blank rather than missing, so the column stays aligned.
	if values["Via"] != "" {
		t.Errorf("Via = %q, want an empty cell", values["Via"])
	}
}

// A CDN domain routinely answers with more than nine A records; the chart used
// to hold a fixed nine slots while being indexed by edge.
func TestEdgeChartHasOneSlotPerEdge(t *testing.T) {
	edges := make([]string, 12)
	for i := range edges {
		edges[i] = "10.0.0." + string(rune('a'+i))
	}

	charts := newEdgeChart("example.com", edges)
	if len(charts) != len(edges) {
		t.Fatalf("got %d charts, want %d", len(charts), len(edges))
	}
	for edge, chart := range charts {
		if len(chart.Data) != len(edges) {
			t.Fatalf("chart for %s has %d slots, want %d", edge, len(chart.Data), len(edges))
		}
	}
}

func TestBarColor(t *testing.T) {
	tests := []struct {
		statusCode int
		want       ui.Color
	}{
		{200, ui.ColorGreen},
		{301, ui.ColorBlue},
		{404, ui.ColorYellow},
		{503, ui.ColorRed},
		{100, ui.ColorWhite},
	}

	for _, tt := range tests {
		got := barColor(tt.statusCode)
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("barColor(%d) = %v, want [%v]", tt.statusCode, got, tt.want)
		}
	}
}

func TestShortHash(t *testing.T) {
	sum := sha256.Sum256([]byte("payload"))

	if got := shortHash(sum[:]); len(got) != hashPrefix {
		t.Errorf("shortHash returned %d characters, want %d", len(got), hashPrefix)
	}
	if got := shortHash(nil); got != "" {
		t.Errorf("shortHash(nil) = %q, want an empty cell", got)
	}

	// Different bodies have to stay distinguishable after truncation.
	other := sha256.Sum256([]byte("different"))
	if shortHash(sum[:]) == shortHash(other[:]) {
		t.Error("two different bodies produced the same short hash")
	}
}

func TestSeenKeepsDistinctValuesInOrder(t *testing.T) {
	s := newSeen("StatusCode")

	if added := s.add("200"); !added {
		t.Error("the first value was not reported as new")
	}
	if added := s.add("200"); added {
		t.Error("a repeated value was reported as new")
	}
	s.add("404")

	want := []string{"StatusCode", "200", "404"}
	if len(s.table.Rows[0]) != len(want) {
		t.Fatalf("history row = %v, want %v", s.table.Rows[0], want)
	}
	for i := range want {
		if s.table.Rows[0][i] != want[i] {
			t.Errorf("history[%d] = %q, want %q", i, s.table.Rows[0][i], want[i])
		}
	}
}

// The row the caller mutates must not be the slice the set keeps.
func TestSeenHandsOutACopy(t *testing.T) {
	s := newSeen("Hash")
	s.add("abc")

	s.table.Rows[0][1] = "tampered"
	s.add("def")

	if s.table.Rows[0][1] != "abc" {
		t.Errorf("the history table shares its backing array with the set: %v", s.table.Rows[0])
	}
}

func TestFillLatencyTable(t *testing.T) {
	table := newLatencyTable()

	https := probe.Trace{
		DNSLookup:        1 * time.Millisecond,
		TCPConnection:    10 * time.Millisecond,
		TLSHandshake:     100 * time.Millisecond,
		ServerProcessing: 1 * time.Second,
		ContentTransfer:  2 * time.Second,
		Total:            3200 * time.Millisecond,
		TLS:              true,
	}
	fillLatencyTable(table, https)

	want := [][2]string{
		{"DNS Lookup", "1ms"},
		{"TCP Connection", "10ms"},
		{"TLS Handshake", "100ms"},
		{"Server Processing", "1s"},
		{"Content Transfer", "2s"},
		{"Total", "3.2s"},
	}
	for i, row := range want {
		if table.Rows[i][0] != row[0] || table.Rows[i][1] != row[1] {
			t.Errorf("row %d = %v, want %v", i, table.Rows[i], row)
		}
	}

	// A plaintext request has one phase fewer, and the row the TLS handshake
	// used to occupy has to be cleared rather than left stale.
	fillLatencyTable(table, probe.Trace{Total: time.Second})
	if table.Rows[4][0] != "Total" {
		t.Errorf("total row = %v, want it to move up for a plaintext request", table.Rows[4])
	}
	if table.Rows[5][0] != "" || table.Rows[5][1] != "" {
		t.Errorf("row 5 = %v, want it blanked", table.Rows[5])
	}
}
