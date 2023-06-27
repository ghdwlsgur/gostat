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

type set map[interface{}]struct{}

func (s set) Add(v interface{}) {
	s[v] = struct{}{}
}

func (s set) Remove(v interface{}) {
	delete(s, v)
}

func (s set) Contain(v interface{}) bool {
	_, ok := s[v]
	return ok
}

func (s set) Length() int {
	return len(s)
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

func showDashboard(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions, protocol string) error {
	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	header := make([]string, len(ips)+1)
	header[0] = "IP"
	copy(header[1:], ips)

	history := widgets.NewTable()
	history.Rows = [][]string{
		make([]string, 2),
	}
	history.Rows[0][0] = "StatusCode"
	history.Title = "History"
	history.BorderStyle.Fg = 7
	history.BorderStyle.Bg = 0
	history.TitleStyle.Fg = 7
	history.TitleStyle.Bg = 0
	history.TextStyle = ui.NewStyle(ui.ColorWhite)
	history.TextStyle.Bg = 0
	history.SetRect(85, 31, 180, 41)

	table := widgets.NewTable()
	table.Rows = [][]string{
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

	table.Title = "Response"
	table.Rows[1][0] = "StatusCode"
	table.Rows[2][0] = "Server"
	table.Rows[3][0] = "Date"
	table.Rows[4][0] = "Last-Modified"
	table.Rows[5][0] = "ETag"
	table.Rows[6][0] = "Age"
	table.Rows[7][0] = "Expires"
	table.Rows[8][0] = "Cache-Control"
	table.Rows[9][0] = "Content-Type"
	table.Rows[10][0] = "Content-Length"
	table.Rows[11][0] = "ACA-Origin"
	table.Rows[12][0] = "Via"
	table.Rows[13][0] = "Hash"
	table.Rows[14][0] = "RequestCount"

	table.BorderStyle.Fg = 7
	table.BorderStyle.Bg = 0
	table.TitleStyle.Fg = 7
	table.TitleStyle.Bg = 0
	table.TextStyle = ui.NewStyle(ui.ColorWhite)
	table.TextStyle.Bg = 0
	table.SetRect(85, 0, 180, 31)

	edgeCharts := make(map[string]*widgets.StackedBarChart, len(ips))

	for _, ip := range ips {
		sbc := widgets.NewStackedBarChart()
		sbc.Title = fmt.Sprintf("%s %s", "StatusCode per Edge of", addrInfo.DomainName)
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

	uiEvents := ui.PollEvents()

	statusBox := &set{}
	initLength := 1

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
					edgeCharts[ip].BarColors = dynamicStatusCodeColor(response.StatusCode, edgeCharts[ip].BarColors)
					if response.EdgeIP == ip {
						edgeCharts[ip].Data[i] = append(edgeCharts[ip].Data[i], float64(response.StatusCode))
					}

					if table.Rows[0][i+1] == ip {
						table.Rows[1][i+1] = response.GetStatusCode()
						table.Rows[2][i+1] = response.GetServer()
						table.Rows[3][i+1] = response.GetDate()
						table.Rows[4][i+1] = response.GetLastModified()
						table.Rows[5][i+1] = response.GetEtag()
						table.Rows[6][i+1] = response.GetAge()
						table.Rows[7][i+1] = response.GetExpires()
						table.Rows[8][i+1] = response.GetCacheControl()
						table.Rows[9][i+1] = response.GetContentType()
						table.Rows[10][i+1] = response.GetContentLength()
						table.Rows[11][i+1] = response.GetACAOrigin()
						table.Rows[12][i+1] = response.GetVia()
						table.Rows[13][i+1] = response.GetHash()
						table.Rows[14][i+1] = requestOptions.GetRequestCount()
					}

					history.Rows[0][1] = response.GetStatusCode()
					statusBox.Add(response.GetStatusCode())
					if initLength != len(*statusBox) {
						history.Rows[0] = append(history.Rows[0], response.GetStatusCode())
						initLength++
					}

					ui.Render(edgeCharts[ip])
					ui.Render(table)
					ui.Render(history)
					time.Sleep(time.Millisecond * 500)

					if len(edgeCharts[ip].Data[i]) == 9 && i == len(ips)-1 {
						for _, v := range edgeCharts {
							v.Data = make([][]float64, 9)
						}
					}
				case "http":
					response := internal.GetStatusCodeOnHTTP(addrInfo, requestOptions)
					if response.Error != nil {
						return response.Error
					}
					edgeCharts[ip].BarColors = dynamicStatusCodeColor(response.StatusCode, edgeCharts[ip].BarColors)
					if response.EdgeIP == ip {
						edgeCharts[ip].Data[i] = append(edgeCharts[ip].Data[i], float64(response.StatusCode))
					}

					if table.Rows[0][i+1] == ip {
						table.Rows[1][i+1] = response.GetStatusCode()
						table.Rows[2][i+1] = response.GetServer()
						table.Rows[3][i+1] = response.GetDate()
						table.Rows[4][i+1] = response.GetLastModified()
						table.Rows[5][i+1] = response.GetEtag()
						table.Rows[6][i+1] = response.GetAge()
						table.Rows[7][i+1] = response.GetExpires()
						table.Rows[8][i+1] = response.GetCacheControl()
						table.Rows[9][i+1] = response.GetContentType()
						table.Rows[10][i+1] = response.GetContentLength()
						table.Rows[11][i+1] = response.GetACAOrigin()
						table.Rows[12][i+1] = response.GetVia()
						table.Rows[13][i+1] = response.GetHash()
					}

					ui.Render(edgeCharts[ip])
					ui.Render(table)
					time.Sleep(time.Millisecond * 500)

					if len(edgeCharts[ip].Data[i]) == 9 && i == len(ips)-1 {
						for _, v := range edgeCharts {
							v.Data = make([][]float64, 9)
						}
					}
				}
			}
		}
		requestOptions.RequestCount++
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
