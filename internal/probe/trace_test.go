package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The phases have to be disjoint spans of one request: a caller adds them up
// and shows a running total, so any overlap is a lie on screen.
func TestTracePhasesAreDisjointAndFitTheTotal(t *testing.T) {
	const serverDelay = 30 * time.Millisecond

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serverDelay)
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	res, err := New(Options{Port: port}).Do(context.Background(), mustParse(t, "https://example.com/"), edge)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	trace := res.Trace
	var sum time.Duration
	for _, phase := range trace.Phases() {
		if phase.Duration < 0 {
			t.Errorf("%s = %v, want a non-negative duration", phase.Name, phase.Duration)
		}
		sum += phase.Duration
	}

	// Total is wall clock, so it covers the phases plus whatever happens
	// between them. It can never be smaller than their sum.
	if sum > trace.Total {
		t.Errorf("phases add up to %v but Total is %v", sum, trace.Total)
	}
	if trace.ServerProcessing < serverDelay {
		t.Errorf("ServerProcessing = %v, want at least the %v the handler slept", trace.ServerProcessing, serverDelay)
	}
	if !trace.TLS || trace.TLSHandshake <= 0 {
		t.Errorf("a TLS request reported TLS=%v handshake=%v", trace.TLS, trace.TLSHandshake)
	}
	if trace.Reused {
		t.Error("a fresh transport reported a reused connection")
	}
}

// Dialling an address does no DNS lookup, and reporting a fabricated one would
// be worse than reporting zero.
func TestTraceReportsNoDNSWhenTheEdgeIsAnAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	res, err := New(Options{Port: port}).Do(context.Background(), mustParse(t, "http://example.com/"), edge)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if res.Trace.DNSLookup != 0 {
		t.Errorf("DNSLookup = %v, want zero when no lookup happened", res.Trace.DNSLookup)
	}
	if res.Trace.TCPConnection <= 0 {
		t.Errorf("TCPConnection = %v, want a measured connection", res.Trace.TCPConnection)
	}
	if res.Trace.TLS {
		t.Error("a plaintext request reported a TLS handshake")
	}
}

func TestTracePhasesSkipTLSForPlaintext(t *testing.T) {
	plain := Trace{TLS: false}
	for _, phase := range plain.Phases() {
		if phase.Name == "TLS Handshake" {
			t.Error("a plaintext trace lists a TLS Handshake phase")
		}
	}

	secure := Trace{TLS: true}
	var names []string
	for _, phase := range secure.Phases() {
		names = append(names, phase.Name)
	}

	want := []string{"DNS Lookup", "TCP Connection", "TLS Handshake", "Server Processing", "Content Transfer"}
	if len(names) != len(want) {
		t.Fatalf("phases = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("phase %d = %q, want %q", i, names[i], want[i])
		}
	}
}

// Content transfer only means something once the body has been read; a probe
// that never reaches the body must not invent a span for it.
func TestTraceContentTransferCoversTheBodyRead(t *testing.T) {
	const bodyDelay = 40 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(bodyDelay)
		_, _ = w.Write([]byte("tail"))
	}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	res, err := New(Options{Port: port}).Do(context.Background(), mustParse(t, "http://example.com/"), edge)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if res.Trace.ContentTransfer < bodyDelay {
		t.Errorf("ContentTransfer = %v, want at least the %v the body was held back", res.Trace.ContentTransfer, bodyDelay)
	}
}

func TestFinishOnAnUntouchedTracerIsSafe(t *testing.T) {
	tracer := newTracer()
	trace := tracer.finish(time.Now())

	if trace.ContentTransfer != 0 {
		t.Errorf("ContentTransfer = %v, want zero when no byte ever arrived", trace.ContentTransfer)
	}
	if trace.Total <= 0 {
		t.Errorf("Total = %v, want the wall clock since the tracer started", trace.Total)
	}
}
