package internal

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fatih/color"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/sirupsen/logrus"
	"github.com/tcnksm/go-httpstat"
)

// A structure that has the target URL and response latency from the URL as fields.
type Result struct {
	URL     string
	Latency int
}

// Terminal ================================================================

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
		latency := fmt.Sprintf("%dms", r.Latency)
		fmt.Printf("\t%s\t\t\t\t\t\t%s\n\n", color.HiWhiteString("Total"), color.HiMagentaString(latency))
	}
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

// DashBoard ================================================================

func latencyTermuiWrapper(url string) {
	results := make(chan Result)
	doneC := make(chan struct{})

	go GatherLatenciesOnDashBoard(url, results, doneC)
}

func GatherLatenciesOnDashBoard(url string, results chan<- Result, doneC <-chan struct{}) {
	resultC := make(chan Result)
	go getLatenciesOnDashBoard(url, resultC)
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

func getLatenciesOnDashBoard(url string, resultC chan<- Result) error {
	var result *httpstat.Result
	var err error

	result, err = getHTTPLatency(url)
	if err != nil {
		return err
	}

	protocol := strings.Split(url, "://")[0]
	if protocol == "http" {
		showStatusOnDashBoardHTTP(url, result, resultC)
	}
	if protocol == "https" {
		showStatusOnDashBoardHTTPS(url, result, resultC)
	}

	close(resultC)
	return nil
}

func showStatusOnDashBoardHTTP(url string, result *httpstat.Result, resultC chan<- Result) {

	t := widgets.NewTable()
	t.Rows = [][]string{
		make([]string, 2),
		make([]string, 2),
		make([]string, 2),
		make([]string, 2),
	}

	t.Title = "Latency"
	t.BorderStyle.Fg = 7
	t.BorderStyle.Bg = 0
	t.TitleStyle.Fg = 7
	t.TitleStyle.Bg = 0
	t.TextStyle = ui.NewStyle(ui.ColorWhite)
	t.TextStyle.Bg = 0
	t.SetRect(0, 30, 85, 39)

	t.Rows[0][0] = "DNS Lookup"
	t.Rows[1][0] = "TCP Connection"
	t.Rows[2][0] = "Server Processing"
	t.Rows[3][0] = "Content Transfer"

	dnsChan := make(chan string)
	tcpChan := make(chan string)
	serverChan := make(chan string)
	rttChan := make(chan string)

	go func() {
		time.Sleep(time.Millisecond * 50)
		dnsChan <- result.DNSLookup.String()
	}()

	go func() {
		time.Sleep(time.Millisecond * 50)
		tcpChan <- result.TCPConnection.String()
	}()

	go func() {
		time.Sleep(time.Millisecond * 50)
		serverChan <- result.ServerProcessing.String()
	}()

	go func() {
		time.Sleep(time.Millisecond * 50)
		rttChan <- result.Total(time.Now()).String()
	}()

	t.Rows[0][1] = <-dnsChan
	t.Rows[1][1] = <-tcpChan
	t.Rows[2][1] = <-serverChan
	total := result.DNSLookup + result.TCPConnection + result.ServerProcessing
	t.Rows[3][1] = total.String()

	ui.Render(t)
}

func showStatusOnDashBoardHTTPS(url string, result *httpstat.Result, resultC chan<- Result) {

	t := widgets.NewTable()
	t.Rows = [][]string{
		make([]string, 2),
		make([]string, 2),
		make([]string, 2),
		make([]string, 2),
		make([]string, 2),
	}
	t.Title = "Latency"
	t.BorderStyle.Fg = 7
	t.BorderStyle.Bg = 0
	t.TitleStyle.Fg = 7
	t.TitleStyle.Bg = 0
	t.TextStyle = ui.NewStyle(ui.ColorWhite)
	t.TextStyle.Bg = 0
	t.SetRect(0, 30, 85, 41)

	t.Rows[0][0] = "DNS Lookup"
	t.Rows[1][0] = "TCP Connection"
	t.Rows[2][0] = "TLS Handshake"
	t.Rows[3][0] = "Server Processing"
	t.Rows[4][0] = "Content Transfer"

	dnsChan := make(chan string)
	tcpChan := make(chan string)
	tlsChan := make(chan string)
	serverChan := make(chan string)
	rttChan := make(chan string)

	go func() {
		time.Sleep(time.Millisecond * 50)
		dnsChan <- result.DNSLookup.String()
	}()

	go func() {
		time.Sleep(time.Millisecond * 50)
		tcpChan <- result.TCPConnection.String()
	}()

	go func() {
		time.Sleep(time.Millisecond * 50)
		tlsChan <- result.TLSHandshake.String()
	}()

	go func() {
		time.Sleep(time.Millisecond * 50)
		serverChan <- result.ServerProcessing.String()
	}()

	go func() {
		time.Sleep(time.Millisecond * 50)
		rttChan <- result.Total(time.Now()).String()
	}()

	t.Rows[0][1] = <-dnsChan
	t.Rows[1][1] = <-tcpChan
	t.Rows[2][1] = <-tlsChan
	t.Rows[3][1] = <-serverChan
	total := result.DNSLookup + result.TCPConnection + result.TLSHandshake + result.ServerProcessing
	t.Rows[4][1] = total.String()

	ui.Render(t)
}
