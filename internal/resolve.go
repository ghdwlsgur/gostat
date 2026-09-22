package internal

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

// Port numbers used when -p is not given.
const (
	DefaultHTTPPort  = 80
	DefaultHTTPSPort = 443
)

const (
	defaultByteRange    = "bytes=0-1"
	dialTimeout         = 10 * time.Second
	keepAlive           = 30 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
)

// A structure with fields required for request options, range is fixed as byte=0-1 by default.
type ReqOptions struct {
	Host          string `json:"domain-host"`
	Authorization string `json:"authorization"`
	Referer       string `json:"referer"`
	Port          int    `json:"port"`
	AttackMode    bool   `json:"attack-mode"`

	// requestCount is shared by every attack-mode worker, so it is only ever
	// touched through atomic operations.
	requestCount atomic.Int64
}

type Response struct {
	StatusCode    int    `json:"Status"`
	Server        string `json:"Server"`
	Date          string `json:"Date"`
	LastModified  string `json:"Last-Modified"`
	Etag          string `json:"Etag"`
	Age           string `json:"Age"`
	Expires       string `json:"Expires"`
	CacheControl  string `json:"Cache-Control"`
	ContentType   string `json:"Content-Type"`
	ContentLength string `json:"Content-Length"`
	ACAOrigin     string `json:"Access-Control-Allow-Origin"`
	Via           string `json:"Via"`
	EdgeIP        string
	Hash          []byte
	Error         error
}

func (r Response) GetStatusCode() string {
	return strconv.Itoa(r.StatusCode)
}

func (r Response) GetServer() string {
	return r.Server
}

func (r Response) GetDateKst() string {
	layout := "Mon, 02 Jan 2006 15:04:05 MST"
	t, err := time.Parse(layout, r.Date)
	if err != nil {
		return "Failed to parse time"
	}

	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return "Failed to load location"
	}

	krTime := t.In(loc)
	krLayout := "2006-01-02 15:04:05"
	krStr := krTime.Format(krLayout)

	return krStr
}

func (r Response) GetDate() string {
	return r.Date
}

func (r Response) GetLastModified() string {
	return r.LastModified
}

func (r Response) GetEtag() string {
	return r.Etag
}

func (r Response) GetAge() string {
	return r.Age
}

func (r Response) GetExpires() string {
	return r.Expires
}

func (r Response) GetCacheControl() string {
	return r.CacheControl
}

func (r Response) GetContentType() string {
	return r.ContentType
}

func (r Response) GetContentLength() string {
	return r.ContentLength
}

func (r Response) GetACAOrigin() string {
	return r.ACAOrigin
}

func (r Response) GetVia() string {
	return r.Via
}

func (r Response) GetHash() string {
	return base64.StdEncoding.EncodeToString(r.Hash)
}

// Structure with fields for address information.
type Address struct {
	IP         string `json:"ip"`
	Url        string `json:"url"`
	DomainName string `json:"domainName"`
	Target     string `json:"target"`
}

func (ro *ReqOptions) getAuthorization() string {
	return ro.Authorization
}

func (ro *ReqOptions) getHost() string {
	return ro.Host
}

func (ro *ReqOptions) getReferer() string {
	return ro.Referer
}

func (ro *ReqOptions) getPort() int {
	return ro.Port
}

func (ro *ReqOptions) getAttackMode() bool {
	return ro.AttackMode
}

func (ro *ReqOptions) getRequestCount() int64 {
	return ro.requestCount.Load()
}

// IncRequestCount bumps the shared counter by one and returns the new value.
func (ro *ReqOptions) IncRequestCount() int64 {
	return ro.requestCount.Add(1)
}

func (ro *ReqOptions) GetRequestCount() string {
	return strconv.FormatInt(ro.requestCount.Load(), 10)
}

func (addr Address) getIP() string {
	return addr.IP
}

func (addr Address) getUrl() string {
	return addr.Url
}

func (addr Address) getDomainName() string {
	return addr.DomainName
}

func (addr Address) getTarget() string {
	return addr.Target
}

// newHTTPTransport pins plain HTTP traffic to a single edge by proxying every
// request through ip:port instead of whatever DNS hands back for the domain.
func newHTTPTransport(domainName, ip string, port int) (*http.Transport, error) {
	if port == 0 {
		port = DefaultHTTPPort
	}

	proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d@%s:%d", domainName, port, ip, port))
	if err != nil {
		return nil, err
	}

	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: keepAlive,
		}).DialContext,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		Proxy:               http.ProxyURL(proxyURL),
	}, nil
}

// SetTransport pins TLS traffic to a single edge: the request keeps the
// original host name so SNI and the Host line stay intact, while every
// connection is dialled against ip:port.
func SetTransport(ip string, port int) *http.Transport {
	if port == 0 {
		port = DefaultHTTPSPort
	}

	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: keepAlive,
		DualStack: true,
	}

	return &http.Transport{
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if ip != "" {
				addr = net.JoinHostPort(ip, strconv.Itoa(port))
			}
			return dialer.DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{
			// The point of the tool is to talk to an edge that does not serve
			// a certificate for its own address, so verification stays off.
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
		},
	}
}

// Applied when using HTTP protocol.
func ResolveHTTP(addr *Address, opt *ReqOptions) error {
	transport, err := newHTTPTransport(addr.getDomainName(), addr.getIP(), opt.getPort())
	if err != nil {
		return err
	}
	client := &http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	requestURL := fmt.Sprintf("http://%s", addr.getUrl())
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	addRequestHeader(req, opt)
	req, measured := traceRequest(req)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if opt.getAttackMode() {
		printAttackProgress(resp.StatusCode, opt)
		return nil
	}

	if err := measured.readBody(resp, io.Discard); err != nil {
		return err
	}

	printExchange(addr, resp, requestURL, measured, "http")
	return nil
}

// Applied when using HTTPS protocol.
func ResolveHTTPS(addr *Address, opt *ReqOptions) error {
	transport := SetTransport(addr.getIP(), opt.getPort())
	client := &http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	requestURL := fmt.Sprintf("https://%s", addr.getUrl())
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	addRequestHeader(req, opt)
	req, measured := traceRequest(req)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if opt.getAttackMode() {
		printAttackProgress(resp.StatusCode, opt)
		return nil
	}

	if err := measured.readBody(resp, io.Discard); err != nil {
		return err
	}

	printExchange(addr, resp, requestURL, measured, "https")
	return nil
}

// printExchange writes the one-edge report: which address answered, how long
// each stage took, and the headers that went out and came back.
func printExchange(addr *Address, resp *http.Response, requestURL string, measured *timing, protocol string) {
	if addr.getTarget() != addr.getIP() {
		fmt.Printf("\n%s - [%s]\n\n", color.HiYellowString(addr.getTarget()), color.HiYellowString(addr.getIP()))
	} else {
		fmt.Printf("\n[%s]\n\n", color.HiYellowString(addr.getTarget()))
	}

	printLatency(requestURL, measured, protocol)

	fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
	setRequestHeader(resp)

	fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
	printStatusToColor(resp.Status)
	printResponse(resp)
}

func printAttackProgress(statusCode int, opt *ReqOptions) {
	fmt.Printf("\r%s: %v, %s: %d",
		color.HiBlackString("Status Code"),
		statusCode,
		color.HiBlackString("Request Count"),
		opt.getRequestCount())
}

// addRequestHeader mirrors the flags onto the outgoing request. Host has to go
// on the request struct rather than the header map: net/http builds the Host
// line from req.Host and drops any "Host" entry left in the map.
func addRequestHeader(req *http.Request, opt *ReqOptions) {
	if !opt.getAttackMode() {
		req.Header.Set("Range", defaultByteRange)
	}

	if host := opt.getHost(); host != "" {
		req.Host = host
	}

	if referer := opt.getReferer(); referer != "" {
		req.Header.Set("Referer", referer)
	}

	if authorization := opt.getAuthorization(); authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
}

func setRequestHeader(resp *http.Response) {
	req := resp.Request

	// optional [Host] - only worth printing when it was overridden.
	if req.Host != "" && req.Host != req.URL.Host {
		PrintFunc("Host", req.Host)
	}

	// optional [Referer]
	if v := req.Header.Get("Referer"); v != "" {
		PrintFunc("Referer", v)
	}

	// optional [Authorization]
	if v := req.Header.Get("Authorization"); v != "" {
		PrintFunc("Authorization", v)
	}

	// required [Range]
	if v := req.Header.Get("Range"); v != "" {
		PrintFunc("Range", v)
	}
	fmt.Println()
}

func printResponse(resp *http.Response) {
	for directive, value := range resp.Header {
		if len(value) == 0 {
			continue
		}
		if len(directive) > 14 {
			PrintFunc(stringFormat(directive), value[0])
		} else {
			PrintFunc(directive, value[0])
		}
	}
	fmt.Println()
}

// newResponse snapshots the headers the dashboard compares. sum is the digest
// of the body, which is what tells two edges serving different bytes for the
// same URL apart.
func newResponse(resp *http.Response, edgeIP string, sum []byte) *Response {
	return &Response{
		StatusCode:    resp.StatusCode,
		Server:        resp.Header.Get("Server"),
		Date:          resp.Header.Get("Date"),
		LastModified:  resp.Header.Get("Last-Modified"),
		Etag:          resp.Header.Get("Etag"),
		Age:           resp.Header.Get("Age"),
		Expires:       resp.Header.Get("Expires"),
		CacheControl:  resp.Header.Get("Cache-Control"),
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.Header.Get("Content-Length"),
		ACAOrigin:     resp.Header.Get("Access-Control-Allow-Origin"),
		Via:           resp.Header.Get("Via"),
		EdgeIP:        edgeIP,
		Hash:          sum,
	}
}

func GetStatusCodeOnHTTPS(addr *Address, opt *ReqOptions) *Response {
	transport := SetTransport(addr.getIP(), opt.getPort())
	client := &http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	requestURL := fmt.Sprintf("https://%s", addr.getUrl())
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return &Response{Error: err}
	}
	addRequestHeader(req, opt)
	req, measured := traceRequest(req)

	resp, err := client.Do(req)
	if err != nil {
		return &Response{Error: err}
	}
	defer resp.Body.Close()

	hasher := sha256.New()
	if err := measured.readBody(resp, hasher); err != nil {
		return &Response{Error: err}
	}

	showLatencyDashBoard(measured, "https")
	return newResponse(resp, addr.getIP(), hasher.Sum(nil))
}

func GetStatusCodeOnHTTP(addr *Address, opt *ReqOptions) *Response {
	transport, err := newHTTPTransport(addr.getDomainName(), addr.getIP(), opt.getPort())
	if err != nil {
		return &Response{Error: err}
	}
	client := &http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	requestURL := fmt.Sprintf("http://%s", addr.getUrl())
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return &Response{Error: err}
	}
	addRequestHeader(req, opt)
	req, measured := traceRequest(req)

	resp, err := client.Do(req)
	if err != nil {
		return &Response{Error: err}
	}
	defer resp.Body.Close()

	hasher := sha256.New()
	if err := measured.readBody(resp, hasher); err != nil {
		return &Response{Error: err}
	}

	showLatencyDashBoard(measured, "http")
	return newResponse(resp, addr.getIP(), hasher.Sum(nil))
}
