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

type (
	ReqOptions struct {
		host      string
		referer   string
		byteRange string
		transport http.Transport
	}

	ResolveResponse struct {
		respStatus string
	}
)

func (rr ResolveResponse) getRespStatus() string {
	return rr.respStatus
}

func (ro ReqOptions) getHost() string {
	return ro.host
}

func (ro ReqOptions) getReferer() string {
	return ro.referer
}

func (ro ReqOptions) getRange() string {
	return ro.byteRange
}

func ResolveHttp(ip string, domain, domainHost string, target string, port int, host string, referer string) error {

	netUrl := url.URL{}
	ref := fmt.Sprintf("http://%s:%v@%s:%v", domainHost, port, ip, port)
	urlProxy, err := netUrl.Parse(ref)
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

	urlDomain := fmt.Sprintf("http://%s", domain)
	req, err := http.NewRequest("GET", urlDomain, nil)
	if err != nil {
		panic(err)
	}

	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	addRequestHeader(req, host, referer)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("\n\t%s%s [%s]%s\n\n", color.HiYellowString("=============="), color.HiYellowString(target), color.HiYellowString(ip), color.HiYellowString("=============="))
	latencyWrapper(urlDomain)

	fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
	setRequestHeader(resp)

	res := &ResolveResponse{}
	res.respStatus = resp.Status

	fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
	printStatusToColor(res.getRespStatus())

	printResponse(resp)

	return nil
}

func ResolveHttps(ip string, domain, domainHost string, target string, host string, referer string) error {

	transport := SetTransport(domain, ip)
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:443", domainHost), transport.TLSClientConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := &http.Client{Transport: &transport}

	url := fmt.Sprintf("https://%s", domain)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	// request
	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	addRequestHeader(req, host, referer)

	// response
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("\n\t%s%s [%s]%s\n\n", color.HiYellowString("=============="), color.HiYellowString(target), color.HiYellowString(ip), color.HiYellowString("=============="))
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
		transport: transport,
	}

	return r.transport
}

func addRequestHeader(req *http.Request, host, referer string) {
	req.Header.Add("Range", "bytes=0-1")
	if host != "" {
		req.Header.Add("Host", host)
	}

	if referer != "" {
		req.Header.Add("Referer", referer)
	}
}

func setRequestHeader(resp *http.Response) {
	req := &ReqOptions{}

	// optional
	if len(resp.Request.Header.Values("host")) > 0 {
		req.host = resp.Request.Header.Values("host")[0]
		PrintFunc("Host", req.getHost())
	}

	// optional
	if len(resp.Request.Header.Values("referer")) > 0 {
		req.referer = resp.Request.Header.Values("referer")[0]
		PrintFunc("Referer", req.getReferer())
	}

	// required
	req.byteRange = resp.Request.Header.Values("range")[0]
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
