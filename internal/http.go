package internal

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/tcnksm/go-httpstat"
)

type Stat struct {
	DnsLookup         string
	TcpConnection     string
	TlsHandshake      string
	ServerProcessing  string
	ContentTransfer   string
	Total             string
	CumulativeTcp     string
	CumulativeTls     string
	CumulativeServer  string
	CumulativeContent string
	Ipv4              string
}

type Request struct {
	hostField    string
	rangeField   string
	refererField string
}

type Response struct {
	respStatus string
}

func (r Response) getRespStatus() string {
	return r.respStatus
}

func (r Request) getHost() string {
	return r.hostField
}

func (r Request) getRange() string {
	return r.rangeField
}

func (r Request) getReferer() string {
	return r.refererField
}

func RequestResolveHTTP(ip string, domain, domainHost string, target string, port int, host string, referer string) error {

	netUrl := url.URL{}
	ref := fmt.Sprintf("http://%s:%v@%s:%v", domainHost, port, ip, port)
	url_proxy, err := netUrl.Parse(ref)
	if err != nil {
		return err
	}

	transport := http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	transport.Proxy = http.ProxyURL(url_proxy)

	client := &http.Client{
		Transport: &transport,
	}

	urlDomain := fmt.Sprintf("http://%s", domain)
	req, err := http.NewRequest("GET", urlDomain, nil)
	if err != nil {
		panic(err)
	}

	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	req.Header.Add("Range", "bytes=0-1")
	if host != "" {
		req.Header.Add("Host", host)
	}
	if referer != "" {
		req.Header.Add("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("\n\t%s%s [%s]%s\n\n", color.HiYellowString("=============="), color.HiYellowString(target), color.HiYellowString(ip), color.HiYellowString("=============="))
	latencyWrapper(urlDomain)

	fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
	reqDirective := &Request{}
	if len(resp.Request.Header.Values("host")) > 0 {
		reqDirective.hostField = resp.Request.Header.Values("host")[0] // [optional]
		PrintFunc("Host", reqDirective.getHost())

	}
	if len(resp.Request.Header.Values("referer")) > 0 {
		reqDirective.refererField = resp.Request.Header.Values("referer")[0] // [optional]
		PrintFunc("Referer", reqDirective.getReferer())
	}
	reqDirective.rangeField = resp.Request.Header.Values("range")[0] // [required]
	PrintFunc("Range", reqDirective.getRange())
	fmt.Println()

	res := &Response{}
	res.respStatus = resp.Status

	fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
	if res.getRespStatus()[0:1] == "5" {
		PrintFunc("Status", color.HiRedString(res.getRespStatus()))
	} else if res.getRespStatus()[0:1] == "4" {
		PrintFunc("Status", color.HiYellowString(res.getRespStatus()))
	} else {
		PrintFunc("Status", color.HiGreenString(res.getRespStatus()))
	}

	for directive, value := range resp.Header {
		length := len(directive)

		if length > 14 {
			var prefixBucket []string
			words := strings.Split(directive, "-")
			for i, word := range words {
				if i != len(words)-1 {
					prefixBucket = append(prefixBucket, word[:1])
				}
			}

			front := strings.Join(prefixBucket, "")
			directiveFormat := strings.Join([]string{front, words[len(words)-1]}, "-")

			PrintFunc(directiveFormat, value[0])
		} else if length < 8 {
			PrintFunc(directive, value[0])
		} else {
			PrintFunc(directive, value[0])
		}
	}
	fmt.Println()

	return nil
}

func RequestResolveHTTPS(ip string, domain, domainHost string, target string, host string, referer string) error {

	ref := fmt.Sprintf("https://%s:443@%s:443", domainHost, ip)
	netUrl := url.URL{}

	url_proxy, err := netUrl.Parse(ref)
	if err != nil {
		return err
	}

	transport := http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	transport.Proxy = http.ProxyURL(url_proxy)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	dialAddr := fmt.Sprintf("%s:443", domainHost)
	conn, err := tls.Dial("tcp", dialAddr, transport.TLSClientConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := &http.Client{
		Transport: &transport,
	}

	urlDomain := fmt.Sprintf("http://%s", domain)
	req, err := http.NewRequest("GET", urlDomain, nil)
	if err != nil {
		panic(err)
	}
	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	req.Header.Add("Range", "bytes=0-1")
	if host != "" {
		req.Header.Add("Host", host)
	}
	if referer != "" {
		req.Header.Add("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("\n\t%s%s [%s]%s\n\n", color.HiYellowString("=============="), color.HiYellowString(target), color.HiYellowString(ip), color.HiYellowString("=============="))
	latencyWrapper(urlDomain)

	fmt.Printf("%s\n", color.HiWhiteString("Request Headers"))
	reqDirective := &Request{}
	if len(resp.Request.Header.Values("host")) > 0 {
		reqDirective.hostField = resp.Request.Header.Values("host")[0] // [optional]
		PrintFunc("Host", reqDirective.getHost())
	}
	if len(resp.Request.Header.Values("referer")) > 0 {
		reqDirective.refererField = resp.Request.Header.Values("referer")[0] // [optional]
		PrintFunc("Referer", reqDirective.getReferer())
	}
	reqDirective.rangeField = resp.Request.Header.Values("range")[0] // [required]
	PrintFunc("Range", reqDirective.getRange())
	fmt.Println()

	res := &Response{}
	res.respStatus = resp.Status

	fmt.Printf("%s\n", color.HiWhiteString("Response Headers"))
	if res.getRespStatus()[0:1] == "5" {
		PrintFunc("Status", color.HiRedString(res.getRespStatus()))
	} else if res.getRespStatus()[0:1] == "4" {
		PrintFunc("Status", color.HiYellowString(res.getRespStatus()))
	} else {
		PrintFunc("Status", color.HiGreenString(res.getRespStatus()))
	}

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

	return nil

}
