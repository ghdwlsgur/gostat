package cmd

import (
	"fmt"
	"net"
	"strings"

	"github.com/ghdwlsgur/gostat/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	requestCommand = &cobra.Command{
		Use:   "request",
		Short: "Exec `gostat request`",
		Long:  "Receives the response of the url to each A record of the target domain to the url using the http or https protocol.",
		Run: func(_ *cobra.Command, args []string) {
			var (
				err     error
				url     string
				urlHost string
				target  string
			)

			var (
				port    int
				host    string
				referer string
			)

			uri := args[0]
			splitData := strings.Split(uri, "://")

			protocol := splitData[0]
			url = splitData[1]

			if len(args) > 1 {
				panicRed(fmt.Errorf("up to one argument can be entered"))
			}

			urlHost = strings.Split(url, "/")[0]

			target = strings.TrimSpace(viper.GetString("stat-target-domain"))
			if target == "" {
				panicRed(fmt.Errorf("please enter your target. ex) gostat stat -t naver.com"))
			}

			host = strings.TrimSpace(viper.GetString("host-name"))
			referer = strings.TrimSpace(viper.GetString("referer-name"))

			ips, err := internal.GetRecord(target)
			if err != nil {
				panicRed(err)
			}

			if protocol == "http" {
				port = viper.GetInt("port-number")

				for _, ip := range ips {

					err = internal.RequestResolveHTTP(ip.String(), url, urlHost, target, port, host, referer)
					if err != nil {
						panicRed(err)
					}
				}
			}

			if protocol == "https" {
				for _, ip := range ips {
					if net.ParseIP(ip.String()).To4() != nil {
						err = internal.RequestResolveHTTPS(ip.String(), url, urlHost, target, host, referer)
						if err != nil {
							panicRed(err)
						}
					}
				}
			}

		},
	}
)

func init() {
	requestCommand.Flags().StringP("target", "t", "", "[required] Receive responses by proxying the A record of the domain forwarded to the target.")
	requestCommand.Flags().IntP("port", "p", 80, "[optional] For http protocol, the default value is 80.")
	requestCommand.Flags().StringP("http-host", "H", "", "[optional] The host to put in the request headers.")
	requestCommand.Flags().StringP("referer", "r", "", "[optional]")

	viper.BindPFlag("stat-target-domain", requestCommand.Flags().Lookup("target"))
	viper.BindPFlag("port-number", requestCommand.Flags().Lookup("port"))
	viper.BindPFlag("host-name", requestCommand.Flags().Lookup("http-host"))
	viper.BindPFlag("referer-name", requestCommand.Flags().Lookup("referer"))

	rootCmd.AddCommand(requestCommand)
}
