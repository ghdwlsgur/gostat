package probe

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"time"
)

// DefaultTimeout caps one probe, connection included.
const DefaultTimeout = 30 * time.Second

const (
	dialTimeout         = 10 * time.Second
	keepAlive           = 30 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
)

// DefaultPort is the port a scheme uses when none was given.
func DefaultPort(scheme string) int {
	if scheme == "https" {
		return 443
	}
	return 80
}

// pinnedTransport dials every connection against edge:port whatever host the
// URL names, which is what `curl --resolve` does. The request itself is
// untouched, so the Host line and the TLS SNI still carry the original name
// and the edge routes it the way it routes real traffic.
//
// Each probe gets a transport of its own, so every sample opens its own
// connection. That is deliberate: a pooled connection reports zero for DNS,
// TCP and TLS, which is the part worth measuring.
func pinnedTransport(edge string, port int) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlive,
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if edge != "" {
				addr = net.JoinHostPort(edge, strconv.Itoa(port))
			}
			return dialer.DialContext(ctx, network, addr)
		},

		TLSClientConfig: &tls.Config{
			// An edge holds no certificate for its own address, and checking
			// the chain is not what this tool is for.
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		},

		TLSHandshakeTimeout: tlsHandshakeTimeout,

		// A custom DialContext switches automatic HTTP/2 off. A CDN edge
		// almost always speaks it, so ask for it back rather than measure a
		// protocol no real client would use.
		ForceAttemptHTTP2: true,
	}
}
