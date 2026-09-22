package report

import (
	"bytes"
	"crypto/sha256"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ghdwlsgur/gostat/internal/probe"
)

func sampleResult() *probe.Result {
	sum := sha256.Sum256([]byte("payload"))

	return &probe.Result{
		Edge:       "1.2.3.4",
		URL:        "https://example.com/asset.txt",
		Proto:      "HTTP/2.0",
		Status:     "206 Partial Content",
		StatusCode: http.StatusPartialContent,
		Headers: http.Header{
			"Server":                      []string{"cdn"},
			"Etag":                        []string{`"abc"`},
			"Cache-Control":               []string{"max-age=60"},
			"Access-Control-Allow-Origin": []string{"*"},
			"Set-Cookie":                  []string{"a=1", "b=2"},
		},
		Sent: probe.Sent{
			Host:           "edge.example.com",
			HostOverridden: true,
			Referer:        "http://ref.example.com",
			Range:          probe.DefaultRange,
		},
		BodySum:   sum[:],
		BodyBytes: 7,
		Trace: probe.Trace{
			DNSLookup:        1 * time.Millisecond,
			TCPConnection:    10 * time.Millisecond,
			TLSHandshake:     100 * time.Millisecond,
			ServerProcessing: 1 * time.Second,
			ContentTransfer:  2 * time.Second,
			Total:            3200 * time.Millisecond,
			TLS:              true,
		},
	}
}

// render returns the report as plain text. fatih/color leaves the escapes out
// when the output is not a terminal, which a test never is.
func render(res *probe.Result, target string) string {
	var buf bytes.Buffer
	NewTerminal(&buf).Result(res, target)
	return buf.String()
}

// lastFieldOf returns the right-hand column of the row starting with label.
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

func TestResultRunningTotalIsTheSumOfThePhases(t *testing.T) {
	out := render(sampleResult(), "example.com")

	// 1ms + 10ms + 100ms + 1s + 2s. go-httpstat's Connect is a running total
	// rather than a phase, and adding it here counted DNS and TCP twice.
	if got, want := lastFieldOf(out, "Content Transfer"), "3.111s"; got != want {
		t.Errorf("running total at the last phase = %q, want %q\n%s", got, want, out)
	}
	if got, want := lastFieldOf(out, "Total"), "3.2s"; got != want {
		t.Errorf("Total = %q, want %q\n%s", got, want, out)
	}
	if lastFieldOf(out, "Connect") != "" {
		t.Errorf("a Connect phase is still being printed:\n%s", out)
	}
}

func TestResultEchoesWhatWasSent(t *testing.T) {
	out := render(sampleResult(), "example.com")

	for label, want := range map[string]string{
		"Host":    "edge.example.com",
		"Referer": "http://ref.example.com",
		"Range":   probe.DefaultRange,
	} {
		if got := lastFieldOf(out, label); got != want {
			t.Errorf("%s = %q, want %q\n%s", label, got, want, out)
		}
	}
}

func TestResultOmitsTheHostRowWhenItWasNotOverridden(t *testing.T) {
	res := sampleResult()
	res.Sent.HostOverridden = false

	if strings.Contains(render(res, "example.com"), "\nHost") {
		t.Error("the Host row is printed even though -H was not given")
	}
}

// Ranging a header map hands back a different order every run, so two runs of
// the same command could not be diffed.
func TestResultSortsTheResponseHeaders(t *testing.T) {
	out := render(sampleResult(), "example.com")

	want := []string{"ACA-Origin", "Cache-Control", "Etag", "Server", "Set-Cookie"}
	var at []int
	for _, name := range want {
		i := strings.Index(out, "\n"+name)
		if i < 0 {
			t.Fatalf("header %q is missing:\n%s", name, out)
		}
		at = append(at, i)
	}

	for i := 1; i < len(at); i++ {
		if at[i] < at[i-1] {
			t.Errorf("%s came out before %s; the headers are not sorted:\n%s", want[i], want[i-1], out)
		}
	}
}

// A header sent more than once used to lose everything after the first value.
func TestResultKeepsRepeatedHeaderValues(t *testing.T) {
	out := render(sampleResult(), "example.com")

	for _, want := range []string{"a=1", "b=2"} {
		if !strings.Contains(out, want) {
			t.Errorf("Set-Cookie value %q is missing:\n%s", want, out)
		}
	}
}

func TestResultReportsTheBodyItHashed(t *testing.T) {
	out := render(sampleResult(), "example.com")

	// The digest covers whatever came back, which with the default Range is
	// two bytes rather than the object, so the report says how many.
	if !strings.Contains(out, "Body 7 bytes, sha256") {
		t.Errorf("the report does not say how much body it hashed:\n%s", out)
	}
}

func TestResultHeading(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "a target different from the edge shows both", target: "example.com", want: "example.com - [1.2.3.4]"},
		{name: "a target equal to the edge shows it once", target: "1.2.3.4", want: "[1.2.3.4]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := render(sampleResult(), tt.target); !strings.Contains(got, tt.want) {
				t.Errorf("heading does not contain %q:\n%s", tt.want, got)
			}
		})
	}
}

// A pooled connection leaves the connection phases at zero, and a zero there
// means "did not happen again", not "took no time".
func TestResultFlagsAReusedConnection(t *testing.T) {
	res := sampleResult()
	res.Trace.Reused = true

	if !strings.Contains(render(res, "example.com"), "connection reused") {
		t.Error("a reused connection is not called out, so its zeros read as measurements")
	}
	if strings.Contains(render(sampleResult(), "example.com"), "connection reused") {
		t.Error("a fresh connection is wrongly reported as reused")
	}
}

func TestAttackProgressStaysOnOneLine(t *testing.T) {
	var buf bytes.Buffer
	NewTerminal(&buf).AttackProgress(206, 42)

	out := buf.String()
	if !strings.HasPrefix(out, "\r") {
		t.Errorf("progress does not return to the start of the line: %q", out)
	}
	if strings.Contains(out, "\n") {
		t.Errorf("progress spans more than one line: %q", out)
	}
	for _, want := range []string{"206", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("progress is missing %q: %q", want, out)
		}
	}
}
