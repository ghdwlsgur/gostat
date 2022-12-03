package internal

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/tcnksm/go-httpstat"
)

// A structure that has the target URL and response latency from the URL as fields.
type Result struct {
	URL     string
	Latency int
}

// The latency response value is obtained through a channel.
func GatherLatencies(url string, results chan<- Result, doneC <-chan struct{}) {
	resultC := make(chan Result)
	go getLatencies(url, resultC)
	for {
		select {
		case r, ok := <-resultC:
			if !ok {
				close(results)
				return
			}
			results <- r
		case <-doneC:
			return
		}
	}
}

func getLatencies(url string, resultC chan<- Result) error {
	var result *httpstat.Result
	var err error

	result, err = getHTTPLatency(url)
	if err != nil {
		return err
	}

	protocol := strings.Split(url, "://")[0]
	if protocol == "http" {
		printHttpStatus(url, result, resultC)
	}
	if protocol == "https" {
		printHttpsStatus(url, result, resultC)
	}

	close(resultC)
	return nil
}

func getHTTPLatency(url string) (*httpstat.Result, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
			"url":   url,
		}).Error("Failed to create")
		return nil, err
	}

	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx)

	client := new(http.Client)
	defer client.CloseIdleConnections()

	client.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
			"url":   url,
		}).Error("Failed to send a HTTP request")
		return nil, err
	}

	result.End(time.Now())
	defer res.Body.Close()
	return &result, nil
}

func printHttpStatus(url string, result *httpstat.Result, resultC chan<- Result) {
	var latency time.Duration

	fmt.Println(color.HiWhiteString("Latency Status"))
	latency += result.DNSLookup
	printStatusFormat(color.HiWhiteString("DNS Lookup"), color.HiGreenString(result.DNSLookup.String()), color.HiMagentaString(latency.String()))
	latency += result.TCPConnection
	printStatusFormat(color.HiWhiteString("TCP Connection"), color.HiGreenString(result.TCPConnection.String()), color.HiMagentaString(latency.String()))
	latency += result.Connect
	printStatusFormat(color.HiWhiteString("Connect"), color.HiGreenString(result.Connect.String()), color.HiMagentaString(latency.String()))
	latency += result.ServerProcessing
	printStatusFormat(color.HiWhiteString("ServerProcessing"), color.HiGreenString(result.ServerProcessing.String()), color.HiMagentaString(latency.String()))

	resultC <- Result{url, int(latency / time.Millisecond)}
}

func printHttpsStatus(url string, result *httpstat.Result, resultC chan<- Result) {
	var latency time.Duration

	fmt.Println(color.HiWhiteString("Latency Status"))
	latency += result.DNSLookup
	printStatusFormat(color.HiWhiteString("DNS Lookup"), color.HiGreenString(result.DNSLookup.String()), color.HiMagentaString(latency.String()))
	latency += result.TCPConnection
	printStatusFormat(color.HiWhiteString("TCP Connection"), color.HiGreenString(result.TCPConnection.String()), color.HiMagentaString(latency.String()))
	latency += result.TLSHandshake
	printStatusFormat(color.HiWhiteString("TLS Handshake"), color.HiGreenString(result.TLSHandshake.String()), color.HiMagentaString(latency.String()))
	latency += result.Connect
	printStatusFormat(color.HiWhiteString("Connect"), color.HiGreenString(result.Connect.String()), color.HiMagentaString(latency.String()))
	latency += result.ServerProcessing
	printStatusFormat(color.HiWhiteString("ServerProcessing"), color.HiGreenString(result.ServerProcessing.String()), color.HiMagentaString(latency.String()))

	resultC <- Result{url, int(latency / time.Millisecond)}
}

func latencyWrapper(url string) {
	results := make(chan Result)
	doneC := make(chan struct{})

	go GatherLatencies(url, results, doneC)

	for r := range results {
		fmt.Printf("\t%s\t\t\t%dms\n\n", color.HiWhiteString("Total"), r.Latency)
	}
}
