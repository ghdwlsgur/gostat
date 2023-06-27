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
	"time"

	"github.com/fatih/color"
	"github.com/tcnksm/go-httpstat"
)

// A structure with fields required for request options, range is fixed as byte=0-1 by default.
type ReqOptions struct {
	Host          string `json:"domain-host"`
	Authorization string `json:"authorization"`
	Referer       string `json:"referer"`
	ByteRange     string `json:"range"`
	Port          int    `json:"port"`
	Transport     http.Transport
	AttackMode    bool `json:"attack-mode"`
	RequestCount  int
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

// Structure with response status code as field.
type ResolveResponse struct {
	respStatus string
}

func (rr ResolveResponse) getRespStatus() string {
	return rr.respStatus
}

func (ro ReqOptions) getAuthorization() string {
	return ro.Authorization
}

func (ro ReqOptions) getHost() string {
	return ro.Host
}

func (ro ReqOptions) getReferer() string {
	return ro.Referer
}

func (ro ReqOptions) getRange() string {
	return ro.ByteRange
}

func (ro ReqOptions) getPort() int {
	return ro.Port
}

func (ro ReqOptions) getAttackMode() bool {
	return ro.AttackMode
}

func (ro ReqOptions) getTransport() http.Transport {
	return ro.Transport
}

func (ro *ReqOptions) getRequestCount() int {
	return ro.RequestCount
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

// Applied when using HTTP protocol.
func ResolveHTTP(addr *Address, opt *ReqOptions) error {

	netURL := url.URL{}
	ref := fmt.Sprintf("http://%s:%v@%s:%v", addr.getDomainName(), opt.getPort(), addr.getIP(), opt.getPort())
	urlProxy, err := netURL.Parse(ref)
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
			Proxy:               http.ProxyURL(urlProxy),
		},
	}

	urlDomain := fmt.Sprintf("http://%s", addr.Url)
	req, err := http.NewRequest("GET", urlDomain, nil)
	if err != nil {
		panic(err)
	}

	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	addRequestHeader(req, opt.getHost(), opt.getReferer(), opt.getAuthorization(), opt.getAttackMode())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !opt.getAttackMode() {
		if addr.getTarget() != addr.getIP() {
			fmt.Printf("\n%s - [%s]\n\n", color.HiYellowString(addr.getTarget()), color.HiYellowString(addr.getIP()))
		} else {
			fmt.Printf("\n[%s]\n\n", color.HiYellowString(addr.getTarget()))
		}

		// ips, _ := GetRecordIPv4(urlDomain)
		latencyWrapper(urlDomain)

		fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
		setRequestHeader(resp)

		res := &ResolveResponse{
			respStatus: resp.Status,
		}

		fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
		printStatusToColor(res.getRespStatus())

		printResponse(resp)
	} else {
		fmt.Printf("\r%s: %v, %s: %d",
			color.HiBlackString("Status Code"),
			resp.StatusCode,
			color.HiBlackString("Reqeust Count"),
			opt.getRequestCount())
	}

	return nil
}

// Applied when using HTTPS protocol.
func ResolveHTTPS(addr *Address, opt *ReqOptions) error {

	transport := SetTransport(addr.getUrl(), addr.getIP())
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", addr.getDomainName()), transport.TLSClientConfig)

	if err != nil {
		return err
	}
	defer conn.Close()

	client := &http.Client{Transport: &transport}

	url := fmt.Sprintf("https://%s", addr.getUrl())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	// request
	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	addRequestHeader(req, opt.getHost(), opt.getReferer(), opt.getAuthorization(), opt.getAttackMode())

	// response
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !opt.getAttackMode() {
		if addr.getTarget() != addr.getIP() {
			fmt.Printf("\n%s - [%s]\n\n", color.HiYellowString(addr.getTarget()), color.HiYellowString(addr.getIP()))
		} else {
			fmt.Printf("\n[%s]\n\n", color.HiYellowString(addr.getTarget()))
		}
		latencyWrapper(url)

		fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
		setRequestHeader(resp)

		res := &ResolveResponse{
			respStatus: resp.Status,
		}

		fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
		printStatusToColor(res.getRespStatus())
		printResponse(resp)
	} else {
		fmt.Printf("\r%s: %v, %s: %d",
			color.HiBlackString("Status Code"),
			resp.StatusCode,
			color.HiBlackString("Reqeust Count"),
			opt.getRequestCount(),
		)
		if resp.StatusCode/100 != 2 {
			fmt.Printf(", %s: %s\n", color.HiBlackString("IP"), color.HiYellowString(addr.getIP()))
		}

	}

	return nil
}

func SetTransport(domainName, ip string) http.Transport {

	transport := http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == fmt.Sprintf("%s:443", domainName) {
			addr = fmt.Sprintf("%s:443", ip)
		} else if ip != "" {
			addr = fmt.Sprintf("%s:443", ip)
		}

		return dialer.DialContext(ctx, network, addr)
	}

	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS11,
		MaxVersion:         tls.VersionTLS13,
	}

	r := &ReqOptions{
		Transport: transport,
	}

	return r.getTransport()
}

func addRequestHeader(req *http.Request, host, referer, authorization string, attack bool) {

	if !attack {
		req.Header.Add("Range", "bytes=0-1")
	}

	if host != "" {
		req.Header.Add("Host", host)
	}

	if referer != "" {
		req.Header.Add("Referer", referer)
	}

	if authorization != "" {
		req.Header.Add("Authorization", authorization)
	}
}

func setRequestHeader(resp *http.Response) {
	req := &ReqOptions{}

	// optional [Host]
	if len(resp.Request.Header.Values("Host")) > 0 {
		req.Host = resp.Request.Header.Values("host")[0]
		PrintFunc("Host", req.getHost())
	}

	// optional [Referer]
	if len(resp.Request.Header.Values("referer")) > 0 {
		req.Referer = resp.Request.Header.Values("referer")[0]
		PrintFunc("Referer", req.getReferer())
	}

	// optional [Authorization]
	if len(resp.Request.Header.Values("Authorization")) > 0 {
		req.Authorization = resp.Request.Header.Values("Authorization")[0]
		PrintFunc("Authorization", req.getAuthorization())
	}

	// required [Range]
	if len(resp.Request.Header.Values("Range")) > 0 {
		req.ByteRange = resp.Request.Header.Values("range")[0]
		PrintFunc("Range", req.getRange())
	}
	fmt.Println()
}

func printResponse(resp *http.Response) {
	for directive, value := range resp.Header {
		length := len(directive)
		if length > 14 {
			word := stringFormat(directive)
			PrintFunc(word, value[0])
		} else if length < 8 {
			PrintFunc(directive, value[0])
		} else {
			PrintFunc(directive, value[0])
		}
	}
	fmt.Println()
}

func GetStatusCodeOnHTTPS(addr *Address, opt *ReqOptions) *Response {

	response := &Response{}
	transport := SetTransport(addr.getUrl(), addr.getIP())
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", addr.getDomainName()), transport.TLSClientConfig)
	if err != nil {
		response.Error = err
		return response
	}
	defer conn.Close()

	client := &http.Client{Transport: &transport}
	url := fmt.Sprintf("https://%s", addr.getUrl())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		response.Error = err
		return response
	}

	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)
	addRequestHeader(req, opt.getHost(), opt.getReferer(), opt.getAuthorization(), opt.getAttackMode())

	latencyTermuiWrapper(url)

	resp, err := client.Do(req)
	if err != nil {
		response.Error = err
		return response
	}
	defer resp.Body.Close()

	// Get Contents Hash
	hasher := sha256.New()
	if _, err := io.Copy(hasher, resp.Body); err != nil {
		response.Error = err
		return response
	}
	sum := hasher.Sum(nil)

	response = &Response{
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
		EdgeIP:        addr.getIP(),
		Hash:          sum,
		Error:         nil,
	}

	return response
}

func GetStatusCodeOnHTTP(addr *Address, opt *ReqOptions) *Response {
	response := &Response{}

	netURL := url.URL{}
	ref := fmt.Sprintf("http://%s:%v@%s:%v", addr.getDomainName(), opt.getPort(), addr.getIP(), opt.getPort())
	urlProxy, err := netURL.Parse(ref)
	if err != nil {
		response.Error = err
		return response
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
			Proxy:               http.ProxyURL(urlProxy),
		},
	}

	urlDomain := fmt.Sprintf("http://%s", addr.Url)
	req, err := http.NewRequest("GET", urlDomain, nil)
	if err != nil {
		response.Error = err
		return response
	}

	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	addRequestHeader(req, opt.getHost(), opt.getReferer(), opt.getAuthorization(), opt.getAttackMode())

	resp, err := client.Do(req)
	if err != nil {
		response.Error = err
		return response
	}
	defer resp.Body.Close()

	latencyTermuiWrapper(urlDomain)

	// Get Contents Hash
	hasher := sha256.New()
	if _, err := io.Copy(hasher, resp.Body); err != nil {
		response.Error = err
		return response
	}
	sum := hasher.Sum(nil)

	response = &Response{
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
		EdgeIP:        addr.getIP(),
		Hash:          sum,
		Error:         nil,
	}

	return response
}
