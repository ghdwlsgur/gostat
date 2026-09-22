package cmd

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ghdwlsgur/gostat/internal"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// edgeHistoryLength is how many samples one edge keeps in its bar chart before
// every chart is wiped and started over.
const edgeHistoryLength = 9

type uniqueBox struct {
	data []string
}

func (s *uniqueBox) Add(v string) {
	for _, value := range s.data {
		if value == v {
			return
		}
	}
	s.data = append(s.data, v)
}

func (s *uniqueBox) Remove(v string) {
	for i, value := range s.data {
		if value == v {
			s.data = append(s.data[:i], s.data[i+1:]...)
			return
		}
	}
}

func (s *uniqueBox) Contain(v string) bool {
	for _, value := range s.data {
		if value == v {
			return true
		}
	}
	return false
}

func (s *uniqueBox) Length() int {
	return len(s.data)
}

func (s *uniqueBox) Get() []string {
	list := make([]string, len(s.data))
	copy(list, s.data)
	return list
}

type drawArgs struct {
	edgeCharts             map[string]*widgets.StackedBarChart
	response               *internal.Response
	ip                     string
	ipListLength           int
	index                  int
	responseTable          *widgets.Table
	statusCodeHistoryTable *widgets.Table
	hashHistoryTable       *widgets.Table
	timeHistoryTable       *widgets.Table
	statusBox              *uniqueBox
	hashBox                *uniqueBox
	timeBox                *uniqueBox
	requestOptions         *internal.ReqOptions
}

func (d drawArgs) rendering() {

	ui.Render(d.edgeCharts[d.ip])
	ui.Render(d.responseTable)
	ui.Render(d.statusCodeHistoryTable)
	ui.Render(d.hashHistoryTable)
	ui.Render(d.timeHistoryTable)

	// statuscode table delay
	<-time.After(500 * time.Millisecond)
}

func (d drawArgs) insertData() {
	d.responseTable.Rows[1][d.index+1] = d.response.GetStatusCode()
	d.responseTable.Rows[2][d.index+1] = d.response.GetServer()
	d.responseTable.Rows[3][d.index+1] = d.response.GetDate()
	d.responseTable.Rows[4][d.index+1] = d.response.GetLastModified()
	d.responseTable.Rows[5][d.index+1] = d.response.GetEtag()
	d.responseTable.Rows[6][d.index+1] = d.response.GetAge()
	d.responseTable.Rows[7][d.index+1] = d.response.GetExpires()
	d.responseTable.Rows[8][d.index+1] = d.response.GetCacheControl()
	d.responseTable.Rows[9][d.index+1] = d.response.GetContentType()
	d.responseTable.Rows[10][d.index+1] = d.response.GetContentLength()
	d.responseTable.Rows[11][d.index+1] = d.response.GetACAOrigin()
	d.responseTable.Rows[12][d.index+1] = d.response.GetVia()
	d.responseTable.Rows[13][d.index+1] = d.response.GetHash()
	d.responseTable.Rows[14][d.index+1] = d.requestOptions.GetRequestCount()
}

func showDashboard(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions, protocol string) error {
	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	statusBox := &uniqueBox{data: []string{"StatusCode"}}
	hashBox := &uniqueBox{data: []string{"Hash"}}
	timeBox := &uniqueBox{data: []string{"Time"}}
	statusCodeHistoryTable := createHistoryTable("statusCode")
	hashHistoryTable := createHistoryTable("hash")
	timeHistoryTable := createHistoryTable("time")
	responseTable := createResponseTable(ips)
	edgeCharts := createEdgeChart(addrInfo.DomainName, ips)
	uiEvents := ui.PollEvents()

	for {
		select {
		case e := <-uiEvents:
			// Returning rather than exiting lets the deferred ui.Close put the
			// terminal back the way it was found.
			if e.Type == ui.KeyboardEvent && (e.ID == "q" || e.ID == "<C-c>") {
				return nil
			}
		default:
			for i, ip := range ips {
				addrInfo.IP = ip

				var response *internal.Response
				switch protocol {
				case "https":
					response = internal.GetStatusCodeOnHTTPS(addrInfo, requestOptions)
				case "http":
					response = internal.GetStatusCodeOnHTTP(addrInfo, requestOptions)
				default:
					return fmt.Errorf("unsupported protocol %q", protocol)
				}

				if response.Error != nil {
					return response.Error
				}

				widgetDraw(&drawArgs{
					edgeCharts:             edgeCharts,
					response:               response,
					ip:                     ip,
					ipListLength:           len(ips) - 1,
					index:                  i,
					responseTable:          responseTable,
					statusCodeHistoryTable: statusCodeHistoryTable,
					hashHistoryTable:       hashHistoryTable,
					timeHistoryTable:       timeHistoryTable,
					statusBox:              statusBox,
					hashBox:                hashBox,
					timeBox:                timeBox,
					requestOptions:         requestOptions,
				})
			}
		}
		requestOptions.IncRequestCount()
	}
}

func createEdgeChart(domain string, ips []string) map[string]*widgets.StackedBarChart {
	edgeCharts := make(map[string]*widgets.StackedBarChart, len(ips))

	for _, ip := range ips {
		sbc := widgets.NewStackedBarChart()
		sbc.Title = fmt.Sprintf("%s %s", "StatusCode per Edge of", domain)
		sbc.TitleStyle.Bg = 0
		sbc.Labels = ips
		// One bar per edge: a domain with more than nine A records used to
		// index past the end of this slice.
		sbc.Data = make([][]float64, len(ips))
		sbc.SetRect(0, 0, 85, 30)
		sbc.BarWidth = 20
		sbc.BorderStyle.Fg = 7
		sbc.BorderStyle.Bg = 0
		sbc.LabelStyles = []ui.Style{
			{Fg: 7, Bg: 0, Modifier: ui.ModifierClear},
		}
		sbc.NumStyles = []ui.Style{
			{Bg: 0, Modifier: ui.ModifierClear},
		}
		edgeCharts[ip] = sbc
	}

	return edgeCharts
}

func createHistoryTable(name string) *widgets.Table {
	historyTable := widgets.NewTable()
	historyTable.Rows = [][]string{
		make([]string, 2),
	}
	historyTable.BorderStyle.Fg = 7
	historyTable.BorderStyle.Bg = 0
	historyTable.TitleStyle.Fg = 7
	historyTable.TitleStyle.Bg = 0
	historyTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	historyTable.TextStyle.Bg = 0

	switch name {
	case "statusCode":
		historyTable.Title = "StatusCode History"
		historyTable.SetRect(85, 31, 180, 34)
	case "time":
		historyTable.Title = "Time History"
		historyTable.SetRect(85, 34, 180, 37)
	case "hash":
		historyTable.Title = "Hash History"
		historyTable.SetRect(85, 37, 180, 40)
	}

	return historyTable
}

func createResponseTable(ips []string) *widgets.Table {
	header := make([]string, len(ips)+1)
	header[0] = "IP"
	copy(header[1:], ips)

	responseTable := widgets.NewTable()
	responseTable.Rows = [][]string{
		header,
		make([]string, len(ips)+1), // statusCode
		make([]string, len(ips)+1), // Server
		make([]string, len(ips)+1), // Date
		make([]string, len(ips)+1), // Last-Modified
		make([]string, len(ips)+1), // Etag
		make([]string, len(ips)+1), // Age
		make([]string, len(ips)+1), // Expires
		make([]string, len(ips)+1), // Cache-Control
		make([]string, len(ips)+1), // Content-Type
		make([]string, len(ips)+1), // Content-Length
		make([]string, len(ips)+1), // Access-Control-Allow-Origin
		make([]string, len(ips)+1), // Via
		make([]string, len(ips)+1), // Hash
		make([]string, len(ips)+1), // RequestCount
	}

	responseTable.Title = "Response"
	responseTable.Rows[1][0] = "StatusCode"
	responseTable.Rows[2][0] = "Server"
	responseTable.Rows[3][0] = "Date"
	responseTable.Rows[4][0] = "Last-Modified"
	responseTable.Rows[5][0] = "ETag"
	responseTable.Rows[6][0] = "Age"
	responseTable.Rows[7][0] = "Expires"
	responseTable.Rows[8][0] = "Cache-Control"
	responseTable.Rows[9][0] = "Content-Type"
	responseTable.Rows[10][0] = "Content-Length"
	responseTable.Rows[11][0] = "ACA-Origin"
	responseTable.Rows[12][0] = "Via"
	responseTable.Rows[13][0] = "Hash"
	responseTable.Rows[14][0] = "RequestCount"
	responseTable.BorderStyle.Fg = 7
	responseTable.BorderStyle.Bg = 0
	responseTable.TitleStyle.Fg = 7
	responseTable.TitleStyle.Bg = 0
	responseTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	responseTable.TextStyle.Bg = 0
	responseTable.SetRect(85, 0, 180, 31)

	return responseTable
}

func widgetDraw(d *drawArgs) {
	ip := d.ip
	i := d.index
	response := d.response
	edgeCharts := d.edgeCharts

	edgeCharts[ip].BarColors = dynamicStatusCodeColor(response.StatusCode, edgeCharts[ip].BarColors)
	if response.EdgeIP == ip {
		edgeCharts[ip].Data[i] = append(edgeCharts[ip].Data[i], float64(response.StatusCode))
	}

	if d.responseTable.Rows[0][i+1] == ip {
		d.insertData()
	}

	before := d.statusBox.Length()
	d.statusBox.Add(response.GetStatusCode())
	d.hashBox.Add(response.GetHash())
	after := d.statusBox.Length()

	if before < after {
		d.timeBox.Add(response.GetDateKst())
	}

	d.statusCodeHistoryTable.Rows[0] = d.statusBox.Get()
	d.hashHistoryTable.Rows[0] = d.hashBox.Get()
	d.timeHistoryTable.Rows[0] = d.timeBox.Get()
	d.rendering()

	if len(edgeCharts[ip].Data[i]) >= edgeHistoryLength && i == d.ipListLength {
		for _, v := range edgeCharts {
			v.Data = make([][]float64, d.ipListLength+1)
		}
	}
}

// parseURL splits the argument into its protocol and the rest of the URL,
// rejecting anything that is not an http or https address.
func parseURL(arg string) (protocol, rest string, err error) {
	protocol, rest, found := strings.Cut(arg, "://")
	if !found {
		return "", "", fmt.Errorf("%q is not a valid url: missing \"://\"", arg)
	}

	if protocol != "http" && protocol != "https" {
		return "", "", fmt.Errorf("unsupported protocol %q: only http and https are supported", protocol)
	}

	if rest == "" || strings.HasPrefix(rest, "/") {
		return "", "", fmt.Errorf("%q is not a valid url: missing host", arg)
	}

	return protocol, rest, nil
}

// resolvePort turns the -p flag into a concrete port. Zero means the flag was
// left alone, so the protocol default applies.
func resolvePort(protocol string, flagPort int) int {
	if flagPort > 0 {
		return flagPort
	}

	if protocol == "https" {
		return internal.DefaultHTTPSPort
	}
	return internal.DefaultHTTPPort
}

// request walks every A record of the target once.
func request(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions, protocol string) error {
	for _, ip := range ips {
		addrInfo.IP = ip

		var err error
		switch protocol {
		case "http":
			err = internal.ResolveHTTP(addrInfo, requestOptions)
		case "https":
			err = internal.ResolveHTTPS(addrInfo, requestOptions)
		default:
			err = fmt.Errorf("unsupported protocol %q", protocol)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func runAttack(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions, protocol string, threads int) {
	if threads < 1 {
		threads = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Every worker gets its own Address: request rewrites IP on each
			// pass, so sharing one struct across threads is a data race.
			local := *addrInfo
			for {
				requestOptions.IncRequestCount()
				if err := request(ips, &local, requestOptions, protocol); err != nil {
					panicRed(err)
				}
			}
		}()
	}
	wg.Wait()
}

func dynamicStatusCodeColor(statusCode int, sbcColor []ui.Color) []ui.Color {
	// ColorBlack   Color = 0
	// ColorRed     Color = 1
	// ColorGreen   Color = 2
	// ColorYellow  Color = 3
	// ColorBlue    Color = 4
	// ColorMagenta Color = 5
	// ColorCyan    Color = 6
	// ColorWhite   Color = 7

	switch statusCode / 100 {
	case 2:
		sbcColor = []ui.Color{2} // Green
	case 3:
		sbcColor = []ui.Color{4} // Blue
	case 4:
		sbcColor = []ui.Color{3} // Yellow
	case 5:
		sbcColor = []ui.Color{1} // Red
	}
	return sbcColor
}

var (
	requestCommand = &cobra.Command{
		Use:   "request",
		Short: "Exec `gostat request https://domain.com -t domain.com`",
		Long:  "Receives the response of the URL to each A record of the target domain to the url using the http or https protocol.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				panicRed(err)
			}

			protocol, rest, err := parseURL(args[0])
			if err != nil {
				panicRed(err)
			}

			domainName := strings.Split(rest, "/")[0]
			target := strings.TrimSpace(viper.GetString("target-domain"))
			if target == "" {
				target = domainName
			}

			ips, err := internal.GetRecordIPv4(target)
			if err != nil {
				panicRed(err)
			}
			if len(ips) == 0 {
				panicRed(fmt.Errorf("no IPv4 address found for %q", target))
			}

			// ! [required] Enter your address information.
			addrInfo := &internal.Address{
				Url:        rest,
				DomainName: domainName,
				Target:     target,
			}

			// [optional] It is additionally saved when entering a header or referrer.
			requestOptions := &internal.ReqOptions{
				Host:          strings.TrimSpace(viper.GetString("host-name")),
				Referer:       strings.TrimSpace(viper.GetString("referer-name")),
				Authorization: strings.TrimSpace(viper.GetString("authorization-name")),
				AttackMode:    viper.GetBool("attack-mode"),
				Port:          resolvePort(protocol, viper.GetInt("port-number")),
			}

			switch {
			case viper.GetBool("dashboard-mode"):
				addrInfo.IP = target
				if err := showDashboard(ips, addrInfo, requestOptions, protocol); err != nil {
					panicRed(err)
				}
			case requestOptions.AttackMode:
				runAttack(ips, addrInfo, requestOptions, protocol, viper.GetInt("thread-count"))
			default:
				if err := request(ips, addrInfo, requestOptions, protocol); err != nil {
					panicRed(err)
				}
			}
		},
	}
)

func init() {
	requestCommand.Flags().StringP("target", "t", "", "[required] Receive responses by proxying the A record of the domain forwarded to the target.")
	requestCommand.Flags().IntP("port", "p", 0, "[optional] Port to connect to (default 80 for http, 443 for https).")
	requestCommand.Flags().IntP("thread", "n", 1, "[optional] choose thread numbers")
	requestCommand.Flags().StringP("host", "H", "", "[optional] The host to put in the request headers.")
	requestCommand.Flags().StringP("authorization", "A", "", "[optional]")
	requestCommand.Flags().StringP("referer", "r", "", "[optional]")
	requestCommand.Flags().BoolP("attack", "a", false, "[optional] enable attack mode")
	requestCommand.Flags().BoolP("dashboard", "d", false, "[optional] enable dashboard")

	viper.BindPFlag("target-domain", requestCommand.Flags().Lookup("target"))
	viper.BindPFlag("port-number", requestCommand.Flags().Lookup("port"))
	viper.BindPFlag("host-name", requestCommand.Flags().Lookup("host"))
	viper.BindPFlag("authorization-name", requestCommand.Flags().Lookup("authorization"))
	viper.BindPFlag("referer-name", requestCommand.Flags().Lookup("referer"))
	viper.BindPFlag("attack-mode", requestCommand.Flags().Lookup("attack"))
	viper.BindPFlag("thread-count", requestCommand.Flags().Lookup("thread"))
	viper.BindPFlag("dashboard-mode", requestCommand.Flags().Lookup("dashboard"))

	rootCmd.AddCommand(requestCommand)
}
