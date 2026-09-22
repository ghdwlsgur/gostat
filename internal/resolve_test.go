package internal

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recorder collects what a test server actually received, so the assertions
// can look at the bytes on the wire instead of at the client's own state.
type recorder struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (rec *recorder) add(r *http.Request) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.requests = append(rec.requests, r.Clone(r.Context()))
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

func TestAddRequestHeaderPutsHostOnTheRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/asset.txt", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	addRequestHeader(req, &ReqOptions{
		Host:          "edge.example.com",
		Referer:       "http://referer.example.com",
		Authorization: "Bearer token",
	})

	// net/http writes the Host line from req.Host and ignores the header map,
	// so anything left only in the map never reaches the server.
	if req.Host != "edge.example.com" {
		t.Errorf("req.Host = %q, want %q", req.Host, "edge.example.com")
	}
	if got := req.Header.Get("Referer"); got != "http://referer.example.com" {
		t.Errorf("Referer = %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer token" {
		t.Errorf("Authorization = %q", got)
	}
	if got := req.Header.Get("Range"); got != defaultByteRange {
		t.Errorf("Range = %q, want %q", got, defaultByteRange)
	}
}

func TestAddRequestHeaderSkipsRangeInAttackMode(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	addRequestHeader(req, &ReqOptions{AttackMode: true})

	if got := req.Header.Get("Range"); got != "" {
		t.Errorf("Range = %q, want it to be absent in attack mode", got)
	}
}

func TestResolveHTTPPinsTheEdgeAndSendsHeaders(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		w.Header().Set("Server", "test-origin")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "payload")
	}))
	defer srv.Close()

	ip, port := splitHostPort(t, srv.URL)
	addr := &Address{
		Url:        "example.com/asset.txt",
		DomainName: "example.com",
		Target:     "example.com",
		IP:         ip,
	}

	out := captureStdout(t, func() {
		if err := ResolveHTTP(addr, &ReqOptions{Port: port, Host: "override.example.com"}); err != nil {
			t.Errorf("ResolveHTTP: %v", err)
		}
	})

	got := rec.first(t)
	if got.Host != "override.example.com" {
		t.Errorf("server saw Host %q, want the -H override %q", got.Host, "override.example.com")
	}
	if got.URL.Path != "/asset.txt" {
		t.Errorf("server saw path %q, want %q", got.URL.Path, "/asset.txt")
	}
	if v := got.Header.Get("Range"); v != defaultByteRange {
		t.Errorf("server saw Range %q, want %q", v, defaultByteRange)
	}
	if !strings.Contains(out, "test-origin") {
		t.Errorf("report does not mention the Server header:\n%s", out)
	}
	if !strings.Contains(out, "Content Transfer") {
		t.Errorf("report does not include the latency breakdown:\n%s", out)
	}
	// The timings come from this request, so no second, header-less probe is
	// fired at the edge any more.
	if n := rec.count(); n != 1 {
		t.Errorf("the edge received %d requests, want exactly 1", n)
	}
}

func TestResolveHTTPSPinsTheEdgeAndKeepsSNI(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ip, port := splitHostPort(t, srv.URL)
	addr := &Address{
		Url:        "example.com/asset.txt",
		DomainName: "example.com",
		Target:     "example.com",
		IP:         ip,
	}

	captureStdout(t, func() {
		if err := ResolveHTTPS(addr, &ReqOptions{Port: port}); err != nil {
			t.Errorf("ResolveHTTPS: %v", err)
		}
	})

	got := rec.first(t)
	if got.Host != "example.com" {
		t.Errorf("server saw Host %q, want %q", got.Host, "example.com")
	}
	if got.TLS == nil || got.TLS.ServerName != "example.com" {
		t.Errorf("SNI was not preserved: %+v", got.TLS)
	}
	if n := rec.count(); n != 1 {
		t.Errorf("the edge received %d requests, want exactly 1", n)
	}
}

func TestResolveHTTPSReportsDialFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ip, port := splitHostPort(t, srv.URL)
	srv.Close() // nothing is listening any more

	addr := &Address{Url: "example.com/", DomainName: "example.com", Target: "example.com", IP: ip}

	captureStdout(t, func() {
		if err := ResolveHTTPS(addr, &ReqOptions{Port: port}); err == nil {
			t.Error("ResolveHTTPS returned nil for a closed server")
		}
	})
}

func TestReadBodyHashesAndTimesTheTransfer(t *testing.T) {
	body := "hello edge"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}

	measured := &timing{start: time.Now()}
	hasher := sha256.New()
	if err := measured.readBody(resp, hasher); err != nil {
		t.Fatalf("readBody: %v", err)
	}

	want := sha256.Sum256([]byte(body))
	if !bytes.Equal(hasher.Sum(nil), want[:]) {
		t.Error("the body was not hashed as it was read")
	}
	if measured.total <= 0 {
		t.Errorf("total = %v, want a positive duration", measured.total)
	}
	if measured.contentTransfer < 0 {
		t.Errorf("contentTransfer = %v, want a non-negative duration", measured.contentTransfer)
	}
}

func TestNewResponseMapsTheHeadersTheDashboardCompares(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusPartialContent,
		Header: http.Header{
			"Server":                      []string{"cdn"},
			"Etag":                        []string{`"abc"`},
			"Cache-Control":               []string{"max-age=60"},
			"Access-Control-Allow-Origin": []string{"*"},
			"Via":                         []string{"1.1 edge"},
		},
	}

	sum := sha256.Sum256([]byte("hello edge"))
	got := newResponse(resp, "1.2.3.4", sum[:])

	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"StatusCode", got.GetStatusCode(), "206"},
		{"Server", got.GetServer(), "cdn"},
		{"ETag", got.GetEtag(), `"abc"`},
		{"Cache-Control", got.GetCacheControl(), "max-age=60"},
		{"ACA-Origin", got.GetACAOrigin(), "*"},
		{"Via", got.GetVia(), "1.1 edge"},
		{"EdgeIP", got.EdgeIP, "1.2.3.4"},
		{"Hash", got.GetHash(), base64.StdEncoding.EncodeToString(sum[:])},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.field, c.got, c.want)
		}
	}
}

func TestGetDateKst(t *testing.T) {
	tests := []struct {
		name string
		date string
		want string
	}{
		{"utc header converts to seoul", "Mon, 02 Oct 2023 00:00:00 GMT", "2023-10-02 09:00:00"},
		{"garbage is reported, not panicked on", "not a date", "Failed to parse time"},
		{"empty stays empty-safe", "", "Failed to parse time"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Response{Date: tt.date}).GetDateKst(); got != tt.want {
				t.Errorf("GetDateKst() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestCountIsRaceFree(t *testing.T) {
	opt := &ReqOptions{}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				opt.IncRequestCount()
			}
		}()
	}
	wg.Wait()

	if got := opt.GetRequestCount(); got != "800" {
		t.Errorf("GetRequestCount() = %q, want %q", got, "800")
	}
}

func TestSetTransportPinsEveryAddress(t *testing.T) {
	transport := SetTransport("9.9.9.9", 0)

	if transport.TLSClientConfig.MinVersion != 0x0303 { // tls.VersionTLS12
		t.Errorf("MinVersion = %#x, want TLS 1.2", transport.TLSClientConfig.MinVersion)
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify is off; the tool cannot reach an edge by IP without it")
	}
	if transport.DialContext == nil {
		t.Fatal("DialContext is nil, so traffic would follow DNS instead of the target")
	}
}
