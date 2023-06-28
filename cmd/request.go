package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ghdwlsgur/gostat/internal"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// type set map[interface{}]struct{}

// func (s set) Add(v interface{}) {
// 	s[v] = struct{}{}
// }

// func (s set) Remove(v interface{}) {
// 	delete(s, v)
// }

// func (s set) Contain(v interface{}) bool {
// 	_, ok := s[v]
// 	return ok
// }

// func (s set) Length() int {
// 	return len(s)
// }

// func (s set) Get() []string {
// 	list := make([]string, 0, len(s))
// 	list = append(list, "StatusCode")
// 	for key := range s {
// 		list = append(list, key.(string))
// 	}
// 	return list
// }

type statusBox struct {
	data []string
}

func (s *statusBox) Add(v string) {
	for _, value := range s.data {
		if value == v {
			return
		}
	}
	s.data = append(s.data, v)
}

func (s *statusBox) Remove(v string) {
	for i, value := range s.data {
		if value == v {
			s.data = append(s.data[:i], s.data[i+1:]...)
			return
		}
	}
}

func (s *statusBox) Contain(v string) bool {
	for _, value := range s.data {
		if value == v {
			return true
		}
	}
	return false
}

func (s *statusBox) Length() int {
	return len(s.data)
}

func (s *statusBox) Get() []string {
	list := make([]string, len(s.data))
	copy(list, s.data)
	return list
}

type drawArgs struct {
	edgeCharts     map[string]*widgets.StackedBarChart
	response       *internal.Response
	ip             string
	ipListLength   int
	index          int
	responseTable  *widgets.Table
	historyTable   *widgets.Table
	statusBox      *statusBox
	requestOptions *internal.ReqOptions
}

func (d drawArgs) rendering() {
	ui.Render(d.edgeCharts[d.ip])
	ui.Render(d.responseTable)
	ui.Render(d.historyTable)
	time.Sleep(time.Millisecond * 500)
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

	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	statusBox := &statusBox{}
	historyTable := createHistoryTable(protocol)
	responseTable := createResponseTable(ips)
	edgeCharts := createEdgeChart(addrInfo.DomainName, ips)
	uiEvents := ui.PollEvents()

	statusBox.data = append(statusBox.data, "StatusCode")

delay:
	for {
		select {
		case e := <-uiEvents:
			if e.Type == ui.KeyboardEvent && (e.ID == "q" || e.ID == "<C-c>") {
				os.Exit(0)
				break delay
			}
		default:
			for i, ip := range ips {
				addrInfo.IP = ip
				requestOptions.Port = viper.GetInt("port-number")
				switch protocol {
				case "https":
					response := internal.GetStatusCodeOnHTTPS(addrInfo, requestOptions)
					if response.Error != nil {
						return response.Error
					}

					widgetDraw(&drawArgs{
						edgeCharts:     edgeCharts,
						response:       response,
						ip:             ip,
						ipListLength:   len(ips) - 1,
						index:          i,
						responseTable:  responseTable,
						historyTable:   historyTable,
						statusBox:      statusBox,
						requestOptions: requestOptions,
					})
				case "http":
					response := internal.GetStatusCodeOnHTTP(addrInfo, requestOptions)
					if response.Error != nil {
						return response.Error
					}

					widgetDraw(&drawArgs{
						edgeCharts:     edgeCharts,
						response:       response,
						ip:             ip,
						ipListLength:   len(ips) - 1,
						index:          i,
						responseTable:  responseTable,
						historyTable:   historyTable,
						statusBox:      statusBox,
						requestOptions: requestOptions,
					})
				}
			}
		}
		requestOptions.RequestCount++
	}
	return nil
}

func createEdgeChart(domain string, ips []string) map[string]*widgets.StackedBarChart {
	edgeCharts := make(map[string]*widgets.StackedBarChart, len(ips))

	for _, ip := range ips {
		sbc := widgets.NewStackedBarChart()
		sbc.Title = fmt.Sprintf("%s %s", "StatusCode per Edge of", domain)
		sbc.TitleStyle.Bg = 0
		sbc.Labels = ips
		sbc.Data = make([][]float64, 9)
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

func createHistoryTable(protocol string) *widgets.Table {
	historyTable := widgets.NewTable()
	historyTable.Rows = [][]string{
		make([]string, 2), // StatusCode
	}
	historyTable.Rows[0][0] = "StatusCode"
	historyTable.Title = "History"
	historyTable.BorderStyle.Fg = 7
	historyTable.BorderStyle.Bg = 0
	historyTable.TitleStyle.Fg = 7
	historyTable.TitleStyle.Bg = 0
	historyTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	historyTable.TextStyle.Bg = 0

	if protocol == "https" {
		historyTable.SetRect(85, 31, 180, 41)
	} else {
		historyTable.SetRect(85, 31, 180, 39)
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

	d.statusBox.Add(response.GetStatusCode())
	d.historyTable.Rows[0] = d.statusBox.Get()
	d.rendering()

	if len(edgeCharts[ip].Data[i]) == 9 && i == d.ipListLength {
		for _, v := range edgeCharts {
			v.Data = make([][]float64, 9)
		}
	}
}

func getProtocol(data []string) (string, error) {
	if len(data[0]) > 5 {
		if data[0] != "http" {
			return "", fmt.Errorf("the input format is incorrect")
		}
		if data[0] != "https" {
			return "", fmt.Errorf("the input format is incorrect")
		}
	}
	return data[0], nil
}

func reqHTTP(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions) error {
	for _, ip := range ips {
		addrInfo.IP = ip
		requestOptions.Port = viper.GetInt("port-number")

		err := internal.ResolveHTTP(addrInfo, requestOptions)
		if err != nil {
			return err
		}
	}
	return nil
}

func reqHTTPS(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions) error {
	for _, ip := range ips {
		addrInfo.IP = ip

		err := internal.ResolveHTTPS(addrInfo, requestOptions)
		if err != nil {
			return err
		}
	}
	return nil
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
			var (
				err        error
				url        string
				domainName string
				target     string
			)

			var (
				host          string
				referer       string
				authorization string
			)

			if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
				panicRed(err)
			}

			if len(args) > 1 {
				panicRed(fmt.Errorf("up to one argument can be entered"))
			}
			splitData := strings.Split(args[0], "://")

			// Check the url format.
			protocol, err := getProtocol(splitData)
			if err != nil {
				panicRed(err)
			}

			url = splitData[1]
			host = strings.TrimSpace(viper.GetString("host-name"))
			referer = strings.TrimSpace(viper.GetString("referer-name"))
			authorization = strings.TrimSpace(viper.GetString("authorization-name"))
			mode := viper.GetBool("attack-mode")
			dashboard := viper.GetBool("dashboard-mode")
			domainName = strings.Split(url, "/")[0]
			target = strings.TrimSpace(viper.GetString("target-domain"))
			if target == "" {
				target = domainName
			}

			ips, err := internal.GetRecordIPv4(target)
			if err != nil {
				panicRed(err)
			}

			// ! [required] Enter your address information.
			addrInfo := &internal.Address{
				Url:        url,
				DomainName: domainName,
				Target:     target,
			}

			// [optional] It is additionally saved when entering a header or referrer.
			requestOptions := &internal.ReqOptions{
				Host:          host,
				Referer:       referer,
				Authorization: authorization,
				AttackMode:    mode,
			}

			if dashboard {
				var wg sync.WaitGroup
				for i := 0; i < 1; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for {
							requestOptions.RequestCount++
							addrInfo.IP = target

							err = showDashboard(ips, addrInfo, requestOptions, protocol)
							if err != nil {
								panicRed(err)
							}
						}
					}()
				}
				wg.Wait()
			}

			if mode {
				var wg sync.WaitGroup
				for i := 0; i < viper.GetInt("thread-count"); i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for {
							requestOptions.RequestCount++
							addrInfo.IP = target

							if protocol == "http" {
								err = reqHTTP(ips, addrInfo, requestOptions)
								if err != nil {
									panicRed(err)
								}
							}

							if protocol == "https" {
								err = reqHTTPS(ips, addrInfo, requestOptions)
								if err != nil {
									panicRed(err)
								}
							}
						}
					}()
				}
				wg.Wait()
			} else {

				if protocol == "http" {
					err = reqHTTP(ips, addrInfo, requestOptions)
					if err != nil {
						panicRed(err)
					}
				}

				if protocol == "https" {
					err = reqHTTPS(ips, addrInfo, requestOptions)
					if err != nil {
						panicRed(err)
					}
				}
			}

		},
	}
)

func init() {
	requestCommand.Flags().StringP("target", "t", "", "[required] Receive responses by proxying the A record of the domain forwarded to the target.")
	requestCommand.Flags().IntP("port", "p", 80, "[optional] For http protocol, the default value is 80.")
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
