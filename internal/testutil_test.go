package internal

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"strconv"
	"testing"
)

// captureStdout swaps os.Stdout for a pipe while fn runs. The package prints
// its report with fmt.Printf, so this is the only way to assert on it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	collected := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		collected <- buf.String()
	}()

	fn()

	os.Stdout = orig
	_ = w.Close()
	return <-collected
}

// splitHostPort pulls the host and port out of an httptest server URL.
func splitHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parsing port of %q: %v", rawURL, err)
	}

	return u.Hostname(), port
}
