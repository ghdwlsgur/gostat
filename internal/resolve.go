package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/fatih/color"
	"github.com/tcnksm/go-httpstat"
)

// A structure with fields required for request options, range is fixed as byte=0-1 by default.
type ReqOptions struct {
	Host      string `json:"domain-host"`
	Referer   string `json:"referer"`
	ByteRange string `json:"range"`
	Port      int    `json:"port"`
	Transport http.Transport
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

func (ro ReqOptions) getTransport() http.Transport {
	return ro.Transport
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
func ResolveHttp(addr *Address, opt *ReqOptions) error {

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

	addRequestHeader(req, opt.getHost(), opt.getReferer())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("\n%s [%s]\n\n", color.HiYellowString(addr.getTarget()), color.HiYellowString(addr.getIP()))
	latencyWrapper(urlDomain)

	fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
	setRequestHeader(resp)

	res := &ResolveResponse{
		respStatus: resp.Status,
	}

	fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
	printStatusToColor(res.getRespStatus())

	printResponse(resp)

	return nil
}

// Applied when using HTTPS protocol.
func ResolveHttps(addr *Address, opt *ReqOptions) error {

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

	addRequestHeader(req, opt.getHost(), opt.getReferer())

	// response
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("\n%s [%s]\n\n", color.HiYellowString(addr.getTarget()), color.HiYellowString(addr.getIP()))
	latencyWrapper(url)

	fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
	setRequestHeader(resp)

	res := &ResolveResponse{
		respStatus: resp.Status,
	}

	fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
	printStatusToColor(res.getRespStatus())

	printResponse(resp)

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

func addRequestHeader(req *http.Request, host, referer string) {
	req.Header.Add("Range", "bytes=0-1")
	if host != "" {
		// req.Header.Add("Host", host)
		req.Host = host
	}

	if referer != "" {
		req.Header.Add("Referer", referer)
	}
}

func setRequestHeader(resp *http.Response) {
	req := &ReqOptions{}

	// optional
	if len(resp.Request.Header.Values("host")) > 0 {
		req.Host = resp.Request.Header.Values("host")[0]
		PrintFunc("Host", req.getHost())
	}

	// optional
	if len(resp.Request.Header.Values("referer")) > 0 {
		req.Referer = resp.Request.Header.Values("referer")[0]
		PrintFunc("Referer", req.getReferer())
	}

	// required
	req.ByteRange = resp.Request.Header.Values("range")[0]
	PrintFunc("Range", req.getRange())
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
