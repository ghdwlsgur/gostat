// Package probe sends one HTTP request to one edge of a domain and reports
// what came back, including where the time went.
//
// It prints nothing. How a Result is shown is up to the report and dashboard
// packages, which is what makes both of them testable without a terminal.
package probe

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultRange is the range the tool asks for unless told otherwise. Two bytes
// are enough to see the response headers without pulling a whole object.
const DefaultRange = "bytes=0-1"

// Options shape the request sent to every edge.
type Options struct {
	// Host overrides the Host line without changing where the request goes.
	Host string
	// Referer and Authorization are sent as given when not empty.
	Referer       string
	Authorization string
	// Range is sent as given; empty sends no Range header at all.
	Range string
	// Port overrides the scheme default.
	Port int
	// Timeout caps one probe. Zero means DefaultTimeout.
	Timeout time.Duration
}

// Sent is what actually went out on the wire, for a report to echo back.
type Sent struct {
	Host          string
	Referer       string
	Authorization string
	Range         string
	// HostOverridden is true when Host came from an option rather than the URL.
	HostOverridden bool
}

// Result is one request to one edge, measured end to end.
type Result struct {
	// Edge is the address that was dialled.
	Edge string
	// URL is what was requested, as the user wrote it.
	URL string
	// Proto is the protocol the exchange actually used, e.g. "HTTP/2.0".
	Proto      string
	Status     string
	StatusCode int
	Headers    http.Header
	Sent       Sent
	// BodySum is the SHA-256 of the bytes that came back. With the default
	// Range that is the first two bytes, not the whole object.
	BodySum []byte
	// BodyBytes is how many bytes went into BodySum.
	BodyBytes int64
	Trace     Trace
}

// Header returns one response header, or "" when it is absent.
func (r *Result) Header(name string) string {
	return r.Headers.Get(name)
}

// Client sends probes. A zero Client works and uses the defaults.
type Client struct {
	opts Options
}

// New returns a Client that applies opts to every request it sends.
func New(opts Options) *Client {
	return &Client{opts: opts}
}

// ParseURL accepts the one argument the CLI takes and rejects anything that is
// not an http or https URL with a host.
func ParseURL(arg string) (*url.URL, error) {
	scheme, rest, found := strings.Cut(arg, "://")
	if !found {
		return nil, fmt.Errorf("%q is not a valid url: missing \"://\"", arg)
	}

	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported protocol %q: only http and https are supported", scheme)
	}

	u, err := url.Parse(arg)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid url: %w", arg, err)
	}

	if u.Host == "" || rest == "" {
		return nil, fmt.Errorf("%q is not a valid url: missing host", arg)
	}

	return u, nil
}

// portFor decides where to connect: the option wins, then the port the URL
// names, then the scheme default. Skipping the middle one meant
// http://host:8080/ was dialled on 80 - the URL said where to go and the tool
// went somewhere else.
func portFor(u *url.URL, option int) int {
	if option > 0 {
		return option
	}

	if named := u.Port(); named != "" {
		if port, err := strconv.Atoi(named); err == nil && port > 0 {
			return port
		}
	}

	return DefaultPort(u.Scheme)
}

// Do sends one GET for u, dialling edge instead of resolving u.Host. Passing
// an empty edge lets DNS decide, which is what happens with no -t.
func (c *Client) Do(ctx context.Context, u *url.URL, edge string) (*Result, error) {
	port := portFor(u, c.opts.Port)

	timeout := c.opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	transport := pinnedTransport(edge, port)
	client := &http.Client{
		Transport: transport,
		// What this edge answers is the answer. A 301 is a result to show,
		// not something to chase, and following one would mix a second
		// connection's timings into the trace. curl does not follow without
		// -L either.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	sent := c.apply(req)

	tracer := newTracer()
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), tracer.clientTrace()))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The body has to be read for the transfer time to mean anything, and
	// hashing it on the way through costs nothing.
	hasher := sha256.New()
	read, err := io.Copy(hasher, resp.Body)
	if err != nil {
		return nil, err
	}
	trace := tracer.finish(time.Now())

	return &Result{
		Edge:       edge,
		URL:        u.String(),
		Proto:      resp.Proto,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Sent:       sent,
		BodySum:    hasher.Sum(nil),
		BodyBytes:  read,
		Trace:      trace,
	}, nil
}

// apply puts the options on the request and reports what it set.
func (c *Client) apply(req *http.Request) Sent {
	sent := Sent{Host: req.URL.Host}

	if c.opts.Host != "" {
		// net/http builds the Host line from req.Host and drops a "Host" entry
		// left in the header map, so this is the only field that works.
		req.Host = c.opts.Host
		sent.Host = c.opts.Host
		sent.HostOverridden = true
	}

	if c.opts.Range != "" {
		req.Header.Set("Range", c.opts.Range)
		sent.Range = c.opts.Range
	}

	if c.opts.Referer != "" {
		req.Header.Set("Referer", c.opts.Referer)
		sent.Referer = c.opts.Referer
	}

	if c.opts.Authorization != "" {
		req.Header.Set("Authorization", c.opts.Authorization)
		sent.Authorization = c.opts.Authorization
	}

	return sent
}
