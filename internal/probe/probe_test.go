package probe

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"
)

// recorder keeps what a test server actually received, so an assertion can
// look at the wire rather than at the client's own idea of it.
type recorder struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (rec *recorder) handler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.requests = append(rec.requests, r.Clone(r.Context()))
		rec.mu.Unlock()
		fn(w, r)
	}
}

func (rec *recorder) count() int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return len(rec.requests)
}

func (rec *recorder) first(t *testing.T) *http.Request {
	t.Helper()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) == 0 {
		t.Fatal("the server received no request")
	}
	return rec.requests[0]
}

// edgeOf splits an httptest URL into the address and port to pin to.
func edgeOf(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parsing the port of %q: %v", rawURL, err)
	}
	return u.Hostname(), port
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()

	u, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL(%q): %v", raw, err)
	}
	return u
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name       string
		arg        string
		wantScheme string
		wantHost   string
		wantErr    bool
	}{
		{name: "https url", arg: "https://www.naver.com/index.html", wantScheme: "https", wantHost: "www.naver.com"},
		{name: "http url", arg: "http://example.com", wantScheme: "http", wantHost: "example.com"},
		{name: "host with a port", arg: "http://example.com:8080/x", wantScheme: "http", wantHost: "example.com"},
		// This used to slip past the check and then panic on splitData[1].
		{name: "a bare domain is rejected, not panicked on", arg: "www.naver.com", wantErr: true},
		{name: "other protocols are rejected", arg: "ftp://example.com", wantErr: true},
		{name: "a protocol with no host is rejected", arg: "https://", wantErr: true},
		{name: "a path with no host is rejected", arg: "https:///index.html", wantErr: true},
		{name: "an empty argument is rejected", arg: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ParseURL(tt.arg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseURL(%q) = %v, want an error", tt.arg, u)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseURL(%q): %v", tt.arg, err)
			}
			if u.Scheme != tt.wantScheme {
				t.Errorf("scheme = %q, want %q", u.Scheme, tt.wantScheme)
			}
			if u.Hostname() != tt.wantHost {
				t.Errorf("hostname = %q, want %q", u.Hostname(), tt.wantHost)
			}
		})
	}
}

func TestDefaultPort(t *testing.T) {
	if got := DefaultPort("https"); got != 443 {
		t.Errorf(`DefaultPort("https") = %d, want 443`, got)
	}
	if got := DefaultPort("http"); got != 80 {
		t.Errorf(`DefaultPort("http") = %d, want 80`, got)
	}
}

func TestDoPinsTheEdgeAndSendsTheOptions(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "test-origin")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	client := New(Options{
		Port:          port,
		Range:         DefaultRange,
		Host:          "override.example.com",
		Referer:       "http://ref.example.com",
		Authorization: "Bearer token",
	})

	res, err := client.Do(context.Background(), mustParse(t, "http://example.com/asset.txt"), edge)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	got := rec.first(t)
	// net/http builds the Host line from req.Host; a "Host" entry left in the
	// header map never reaches the server.
	if got.Host != "override.example.com" {
		t.Errorf("server saw Host %q, want the -H override", got.Host)
	}
	// Origin-form, the way curl --resolve sends it. The old proxy trick put
	// an absolute URI on the request line instead.
	if got.RequestURI != "/asset.txt" {
		t.Errorf("request line carried %q, want %q", got.RequestURI, "/asset.txt")
	}
	if v := got.Header.Get("Range"); v != DefaultRange {
		t.Errorf("Range = %q, want %q", v, DefaultRange)
	}
	if v := got.Header.Get("Referer"); v != "http://ref.example.com" {
		t.Errorf("Referer = %q", v)
	}
	if v := got.Header.Get("Authorization"); v != "Bearer token" {
		t.Errorf("Authorization = %q", v)
	}

	if res.StatusCode != http.StatusPartialContent {
		t.Errorf("StatusCode = %d, want 206", res.StatusCode)
	}
	if res.Header("Server") != "test-origin" {
		t.Errorf("Server header = %q", res.Header("Server"))
	}
	if res.Edge != edge {
		t.Errorf("Edge = %q, want %q", res.Edge, edge)
	}

	want := sha256.Sum256([]byte("payload"))
	if string(res.BodySum) != string(want[:]) {
		t.Error("BodySum is not the digest of the body that came back")
	}
	if res.BodyBytes != int64(len("payload")) {
		t.Errorf("BodyBytes = %d, want %d", res.BodyBytes, len("payload"))
	}
	if rec.count() != 1 {
		t.Errorf("the edge received %d requests, want exactly 1", rec.count())
	}
}

func TestDoSendsNoRangeWhenTheOptionIsEmpty(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	if _, err := New(Options{Port: port}).Do(context.Background(), mustParse(t, "http://example.com/"), edge); err != nil {
		t.Fatalf("Do: %v", err)
	}

	if v := rec.first(t).Header.Get("Range"); v != "" {
		t.Errorf("Range = %q, want no Range header at all", v)
	}
}

func TestDoKeepsSNIAndTheHostWhenPinned(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewTLSServer(rec.handler(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	if _, err := New(Options{Port: port}).Do(context.Background(), mustParse(t, "https://example.com/asset.txt"), edge); err != nil {
		t.Fatalf("Do: %v", err)
	}

	got := rec.first(t)
	if got.Host != "example.com" {
		t.Errorf("server saw Host %q, want %q", got.Host, "example.com")
	}
	if got.TLS == nil || got.TLS.ServerName != "example.com" {
		t.Errorf("SNI was not preserved: %+v", got.TLS)
	}
}

// A redirect is an answer from this edge, not something to chase: following it
// would report another host's headers and mix a second connection's timings
// into the trace.
func TestDoDoesNotFollowRedirects(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec.handler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/elsewhere", http.StatusMovedPermanently)
	}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	res, err := New(Options{Port: port}).Do(context.Background(), mustParse(t, "http://example.com/"), edge)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if res.StatusCode != http.StatusMovedPermanently {
		t.Errorf("StatusCode = %d, want the 301 itself", res.StatusCode)
	}
	if res.Header("Location") != "/elsewhere" {
		t.Errorf("Location = %q", res.Header("Location"))
	}
	if rec.count() != 1 {
		t.Errorf("the edge received %d requests, want exactly 1", rec.count())
	}
}

func TestDoHonoursACancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if _, err := New(Options{Port: port}).Do(ctx, mustParse(t, "http://example.com/"), edge); err == nil {
		t.Error("Do returned nil for a cancelled request")
	}
}

func TestDoTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	edge, port := edgeOf(t, srv.URL)
	client := New(Options{Port: port, Timeout: 100 * time.Millisecond})

	start := time.Now()
	if _, err := client.Do(context.Background(), mustParse(t, "http://example.com/"), edge); err == nil {
		t.Fatal("Do returned nil for a server that never answers")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Do took %v, so the timeout was not applied", elapsed)
	}
}

func TestDoReportsADialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	edge, port := edgeOf(t, srv.URL)
	srv.Close() // nothing is listening any more

	if _, err := New(Options{Port: port}).Do(context.Background(), mustParse(t, "http://example.com/"), edge); err == nil {
		t.Error("Do returned nil for a closed server")
	}
}

func TestDoWithoutAnEdgeFallsBackToDNS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	u, err := ParseURL(srv.URL)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parsing the port of %q: %v", u, err)
	}

	// An empty edge means "resolve the url", which is what happens with no -t.
	if _, err := New(Options{Port: port}).Do(context.Background(), u, ""); err != nil {
		t.Errorf("Do: %v", err)
	}
}
