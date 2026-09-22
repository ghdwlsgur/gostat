package probe

import (
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"
)

// Trace is where one request spent its time.
//
// Every phase but ContentTransfer happens before the first response byte, and
// none of it happens again on a connection taken from the pool. Reused says
// when that was the case, because reading a zero DNSLookup as "DNS was
// instant" would be wrong.
type Trace struct {
	DNSLookup        time.Duration
	TCPConnection    time.Duration
	TLSHandshake     time.Duration
	ServerProcessing time.Duration
	ContentTransfer  time.Duration

	// Total is wall clock for the whole exchange, from just before the
	// request was handed to the transport until the last body byte. It is at
	// least the sum of the phases: it also covers connection pool waits and
	// the gaps httptrace does not report.
	Total time.Duration

	// Reused is true when the connection came out of the pool, which leaves
	// DNSLookup, TCPConnection and TLSHandshake at zero.
	Reused bool

	// TLS is true when a handshake took place on this request.
	TLS bool
}

// Phases returns the breakdown in display order, skipping the TLS row for a
// plaintext request. The durations are disjoint, so a running total over them
// is meaningful.
func (t Trace) Phases() []Phase {
	phases := []Phase{
		{"DNS Lookup", t.DNSLookup},
		{"TCP Connection", t.TCPConnection},
	}

	if t.TLS {
		phases = append(phases, Phase{"TLS Handshake", t.TLSHandshake})
	}

	return append(phases,
		Phase{"Server Processing", t.ServerProcessing},
		Phase{"Content Transfer", t.ContentTransfer},
	)
}

// Phase is one named span of a request.
type Phase struct {
	Name     string
	Duration time.Duration
}

// tracer collects a Trace while a request is in flight. httptrace runs its
// callbacks on whichever goroutine drives the connection, so the fields are
// guarded.
type tracer struct {
	mu sync.Mutex

	start        time.Time
	dnsStart     time.Time
	connectStart time.Time
	tlsStart     time.Time
	requestSent  time.Time
	firstByte    time.Time

	trace Trace
}

func newTracer() *tracer {
	return &tracer{start: time.Now()}
}

func (t *tracer) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.dnsStart = time.Now()
		},

		DNSDone: func(httptrace.DNSDoneInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.dnsStart.IsZero() {
				t.trace.DNSLookup = time.Since(t.dnsStart)
			}
		},

		// Dialling several addresses at once is allowed, so these two can fire
		// more than once. The first attempt starts the clock and the first one
		// to succeed stops it.
		ConnectStart: func(string, string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if t.connectStart.IsZero() {
				t.connectStart = time.Now()
			}
		},

		ConnectDone: func(_, _ string, err error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if err != nil || t.trace.TCPConnection != 0 || t.connectStart.IsZero() {
				return
			}
			t.trace.TCPConnection = time.Since(t.connectStart)
		},

		TLSHandshakeStart: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.tlsStart = time.Now()
		},

		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if err != nil || t.tlsStart.IsZero() {
				return
			}
			t.trace.TLS = true
			t.trace.TLSHandshake = time.Since(t.tlsStart)
		},

		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.trace.Reused = info.Reused
		},

		WroteRequest: func(httptrace.WroteRequestInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.requestSent = time.Now()
		},

		GotFirstResponseByte: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.firstByte = time.Now()
			if !t.requestSent.IsZero() {
				t.trace.ServerProcessing = t.firstByte.Sub(t.requestSent)
			}
		},
	}
}

// finish closes the measurement. end must be the moment the response body was
// fully read: the transfer is not over until the last byte lands.
func (t *tracer) finish(end time.Time) Trace {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.firstByte.IsZero() {
		t.trace.ContentTransfer = end.Sub(t.firstByte)
	}
	t.trace.Total = end.Sub(t.start)

	return t.trace
}
