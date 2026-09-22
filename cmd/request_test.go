package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// run executes the command tree with args and returns everything it wrote.
func run(t *testing.T, ctx context.Context, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := NewRootCommand("test")
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	err := root.ExecuteContext(ctx)

	return out.String(), err
}

// edgeFlags turns an httptest URL into the -t and -p a test needs.
func edgeFlags(t *testing.T, rawURL string) (string, string) {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}
	if _, err := strconv.Atoi(u.Port()); err != nil {
		t.Fatalf("parsing the port of %q: %v", rawURL, err)
	}

	return u.Hostname(), u.Port()
}

func TestRequestReportsWhatTheEdgeAnswered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "edge.example.com" {
			t.Errorf("server saw Host %q, want the -H override", r.Host)
		}
		w.Header().Set("Server", "test-origin")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	edge, port := edgeFlags(t, srv.URL)
	out, err := run(t, context.Background(),
		"request", "http://example.com/asset.txt",
		"-t", edge, "-p", port, "-H", "edge.example.com")
	if err != nil {
		t.Fatalf("request: %v\n%s", err, out)
	}

	for _, want := range []string{"206 Partial Content", "test-origin", "Latency Status", "Total", edge} {
		if !strings.Contains(out, want) {
			t.Errorf("the report is missing %q:\n%s", want, out)
		}
	}
}

func TestRequestProbesEveryEdge(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	// 127.0.0.1 resolves to exactly one address, so one sweep is one request.
	edge, port := edgeFlags(t, srv.URL)
	if _, err := run(t, context.Background(), "request", "http://example.com/", "-t", edge, "-p", port); err != nil {
		t.Fatalf("request: %v", err)
	}

	if got := hits.Load(); got != 1 {
		t.Errorf("the edge was hit %d times, want once per sweep", got)
	}
}

func TestRequestRejectsBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			// This used to reach splitData[1] and panic.
			name: "a bare domain",
			args: []string{"request", "www.naver.com"},
			want: `missing "://"`,
		},
		{
			name: "an unsupported protocol",
			args: []string{"request", "ftp://example.com"},
			want: "unsupported protocol",
		},
		{
			name: "no argument",
			args: []string{"request"},
			want: "accepts 1 arg",
		},
		{
			name: "more than one argument",
			args: []string{"request", "http://a.com", "http://b.com"},
			want: "accepts 1 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(t, context.Background(), tt.args...)
			if err == nil {
				t.Fatalf("%v returned a nil error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

// Ctrl-c cancels the context; that is a clean stop, not a failure to report.
func TestRequestTreatsCancellationAsACleanStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	edge, port := edgeFlags(t, srv.URL)
	if _, err := run(t, ctx, "request", "http://example.com/", "-t", edge, "-p", port); err != nil {
		t.Errorf("a cancelled run reported %v, want a clean stop", err)
	}
}

func TestAttackModeStopsWhenCancelled(t *testing.T) {
	// The handler runs on a goroutine per request, so the counters are atomic.
	var hits, ranged atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			ranged.Add(1)
		}
		hits.Add(1)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	edge, port := edgeFlags(t, srv.URL)
	out, err := run(t, ctx, "request", "http://example.com/", "-t", edge, "-p", port, "-a", "-n", "2")
	if err != nil {
		t.Fatalf("attack: %v\n%s", err, out)
	}

	if hits.Load() == 0 {
		t.Error("attack mode sent no request")
	}
	if !strings.Contains(out, "Request Count") {
		t.Errorf("attack mode printed no progress:\n%s", out)
	}
	// Attack mode is about load, so it asks for the object rather than two bytes.
	if got := ranged.Load(); got != 0 {
		t.Errorf("attack mode sent a Range header on %d requests", got)
	}
}
