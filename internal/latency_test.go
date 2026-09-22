package internal

import (
	"strings"
	"testing"
	"time"
)

// stubTiming is a finished measurement built without touching the network.
func stubTiming() *timing {
	t := &timing{}
	t.stat.DNSLookup = 1 * time.Millisecond
	t.stat.TCPConnection = 10 * time.Millisecond
	t.stat.TLSHandshake = 100 * time.Millisecond
	t.stat.ServerProcessing = 1 * time.Second
	// The running totals go-httpstat also reports. Nothing may add these to a
	// sum: Connect already contains DNSLookup and TCPConnection.
	t.stat.NameLookup = 1 * time.Millisecond
	t.stat.Connect = 11 * time.Millisecond
	t.stat.Pretransfer = 111 * time.Millisecond
	t.stat.StartTransfer = 1111 * time.Millisecond
	t.contentTransfer = 2 * time.Second
	t.total = 3200 * time.Millisecond
	return t
}

// lastFieldOf returns the right-hand column of the row whose first column is
// label, or "" when there is no such row.
func lastFieldOf(out, label string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(strings.Join(fields, " "), label+" ") {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func TestPrintLatencyDoesNotDoubleCountTheHandshake(t *testing.T) {
	measured := stubTiming()

	var got Result
	out := captureStdout(t, func() {
		got = printLatency("https://example.com/", measured, "https")
	})

	// 1ms + 10ms + 100ms + 1s + 2s. The old code added Connect as if it were
	// a phase of its own, which counted DNS and TCP a second time.
	if want := "3.111s"; lastFieldOf(out, "Content Transfer") != want {
		t.Errorf("running total at the last stage = %q, want %q\n%s",
			lastFieldOf(out, "Content Transfer"), want, out)
	}

	if want := "3200ms"; lastFieldOf(out, "Total") != want {
		t.Errorf("Total = %q, want %q\n%s", lastFieldOf(out, "Total"), want, out)
	}

	if got.Latency != 3200 {
		t.Errorf("Result.Latency = %d, want 3200", got.Latency)
	}
	if got.URL != "https://example.com/" {
		t.Errorf("Result.URL = %q", got.URL)
	}
}

func TestPrintLatencyStagesPerProtocol(t *testing.T) {
	tests := []struct {
		protocol    string
		wantStages  []string
		absentStage string
		wantRunning string
	}{
		{
			protocol:    "https",
			wantStages:  []string{"DNS Lookup", "TCP Connection", "TLS Handshake", "Server Processing", "Content Transfer"},
			absentStage: "Connect",
			wantRunning: "3.111s",
		},
		{
			protocol:    "http",
			wantStages:  []string{"DNS Lookup", "TCP Connection", "Server Processing", "Content Transfer"},
			absentStage: "TLS Handshake",
			wantRunning: "3.011s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			out := captureStdout(t, func() {
				printLatency("x://example.com/", stubTiming(), tt.protocol)
			})

			for _, stage := range tt.wantStages {
				if lastFieldOf(out, stage) == "" {
					t.Errorf("stage %q is missing:\n%s", stage, out)
				}
			}
			if lastFieldOf(out, tt.absentStage) != "" {
				t.Errorf("stage %q should not be reported for %s:\n%s", tt.absentStage, tt.protocol, out)
			}
			if got := lastFieldOf(out, "Content Transfer"); got != tt.wantRunning {
				t.Errorf("running total = %q, want %q", got, tt.wantRunning)
			}
		})
	}
}

func TestCreateLatencyTable(t *testing.T) {
	tests := []struct {
		protocol string
		want     []string
	}{
		{"http", []string{"DNS Lookup", "TCP Connection", "Server Processing", "Total"}},
		{"https", []string{"DNS Lookup", "TCP Connection", "TLS Handshake", "Server Processing", "Total"}},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			table := createLatencyTable(tt.protocol)
			if table == nil {
				t.Fatalf("createLatencyTable(%q) = nil", tt.protocol)
			}
			if len(table.Rows) != len(tt.want) {
				t.Fatalf("got %d rows, want %d", len(table.Rows), len(tt.want))
			}
			for i, label := range tt.want {
				if table.Rows[i][0] != label {
					t.Errorf("row %d label = %q, want %q", i, table.Rows[i][0], label)
				}
			}
		})
	}

	if createLatencyTable("ftp") != nil {
		t.Error(`createLatencyTable("ftp") should be nil rather than an empty table`)
	}
}

func TestGetLatencyDataFillsEveryRow(t *testing.T) {
	for _, protocol := range []string{"http", "https"} {
		t.Run(protocol, func(t *testing.T) {
			table := getLatencyData(stubTiming(), createLatencyTable(protocol), protocol)

			for i, row := range table.Rows {
				if row[1] == "" {
					t.Errorf("row %d (%s) has no value", i, row[0])
				}
			}

			last := table.Rows[len(table.Rows)-1]
			// The bottom row used to say "Content Transfer" while holding the
			// sum of every phase.
			if last[0] != "Total" || last[1] != "3.2s" {
				t.Errorf("bottom row = %v, want [Total 3.2s]", last)
			}
		})
	}
}
