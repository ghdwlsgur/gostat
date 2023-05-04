package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ghdwlsgur/gostat/internal"
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

		err := internal.ResolveHttp(addrInfo, requestOptions)
		if err != nil {
			return err
		}
	}
	return nil
}

func reqHTTPS(ips []string, addrInfo *internal.Address, requestOptions *internal.ReqOptions) error {
	for _, ip := range ips {
		addrInfo.IP = ip

		err := internal.ResolveHttps(addrInfo, requestOptions)
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	requestCommand = &cobra.Command{
		Use:   "request",
		Short: "Exec `gostat request https://domain.com -t domain.com`",
		Long:  "Receives the response of the URL to each A record of the target domain to the url using the http or https protocol.",
		Run: func(_ *cobra.Command, args []string) {
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
	requestCommand.Flags().IntP("thread", "n", 1, "[optional] thread")
	requestCommand.Flags().StringP("host", "H", "", "[optional] The host to put in the request headers.")
	requestCommand.Flags().StringP("authorization", "A", "", "[optional]")
	requestCommand.Flags().StringP("referer", "r", "", "[optional]")
	requestCommand.Flags().BoolP("attack", "a", false, "[optional] enable attack mode")

	viper.BindPFlag("target-domain", requestCommand.Flags().Lookup("target"))
	viper.BindPFlag("port-number", requestCommand.Flags().Lookup("port"))
	viper.BindPFlag("host-name", requestCommand.Flags().Lookup("host"))
	viper.BindPFlag("authorization-name", requestCommand.Flags().Lookup("authorization"))
	viper.BindPFlag("referer-name", requestCommand.Flags().Lookup("referer"))
	viper.BindPFlag("attack-mode", requestCommand.Flags().Lookup("attack"))
	viper.BindPFlag("thread-count", requestCommand.Flags().Lookup("thread"))

	rootCmd.AddCommand(requestCommand)
}
