package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ghdwlsgur/gostat/internal"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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

func reqDashboard(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions, protocol string) error {
	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	table1 := widgets.NewTable()
	table1.Rows = [][]string{
		ips,
		make([]string, len(ips)), // statusCode
		make([]string, len(ips)),
	}

	fmt.Println(table1.Rows[0])
	table1.TextStyle = ui.NewStyle(ui.ColorWhite)
	table1.SetRect(85, 0, 160, 30)

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
					statusCode, edgeIP, err := internal.GetStatusCodeOnHTTPS(addrInfo, requestOptions)
					if err != nil {
						return err
					}
					edgeCharts[ip].BarColors = dynamicStatusCodeColor(statusCode, edgeCharts[ip].BarColors)
					if edgeIP == ip {
						edgeCharts[ip].Data[i] = append(edgeCharts[ip].Data[i], float64(statusCode))
					}

					if table1.Rows[0][i] == ip {
						statusCodeStr := strconv.Itoa(statusCode)
						table1.Rows[1][i] = statusCodeStr
					}

					ui.Render(edgeCharts[ip])
					ui.Render(table1)
					if len(edgeCharts[ip].Data[i]) == 9 && i == len(ips)-1 {
						for _, v := range edgeCharts {
							v.Data = make([][]float64, 9)
						}
					}
				case "http":
					statusCode, edgeIP, err := internal.GetStatusCodeOnHTTP(addrInfo, requestOptions)
					if err != nil {
						return err
					}
					edgeCharts[ip].BarColors = dynamicStatusCodeColor(statusCode, edgeCharts[ip].BarColors)
					if edgeIP == ip {
						edgeCharts[ip].Data[i] = append(edgeCharts[ip].Data[i], float64(statusCode))
					}

					if table1.Rows[0][i] == ip {
						statusCodeStr := strconv.Itoa(statusCode)
						table1.Rows[1][i] = statusCodeStr
					}

					ui.Render(edgeCharts[ip])
					ui.Render(table1)
					if len(edgeCharts[ip].Data[i]) == 9 && i == len(ips)-1 {
						for _, v := range edgeCharts {
							v.Data = make([][]float64, 9)
						}
					}
				}
			}
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

							err = reqDashboard(ips, addrInfo, requestOptions, protocol)
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
