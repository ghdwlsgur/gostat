package cmd

import (
	"fmt"
	"strings"

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

func returnIP(ips []string) <-chan string {
	out := make(chan string)
	go func() {
		for _, ip := range ips {
			out <- ip
		}
		close(out)
	}()
	return out
}

func resolveHTTPPuller(c <-chan string, reqOpt *internal.ReqOptions, addrInfo *internal.Address) {
	go func() {
		for n := range c {
			addrInfo.IP = n
			fmt.Println(n)
		}

		err := internal.ResolveHttp(addrInfo, reqOpt)
		if err != nil {
			panicRed(err)
		}
	}()
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

			// if protocol == "http" {
			// 	for _, ip := range ips {
			// 		addrInfo.IP = ip
			// 		requestOptions.Port = viper.GetInt("port-number")

			// 		err = internal.ResolveHttp(addrInfo, requestOptions)
			// 		if err != nil {
			// 			panicRed(err)
			// 		}
			// 	}
			// }

			if protocol == "http" {
				// for _, ip := range ips {
				// 	addrInfo.IP = ip
				// 	requestOptions.Port = viper.GetInt("port-number")

				// 	err = internal.ResolveHttp(addrInfo, requestOptions)
				// 	if err != nil {
				// 		panicRed(err)
				// 	}
				// }

				in := returnIP(ips)
				requestOptions.Port = viper.GetInt("port-number")
				resolveHTTPPuller(in, requestOptions, addrInfo)

			}

			if protocol == "https" {
				for _, ip := range ips {
					addrInfo.IP = ip

					err = internal.ResolveHttps(addrInfo, requestOptions)
					if err != nil {
						panicRed(err)
					}
				}
			}

			// c := make(chan string)

			// if mode {
			// 	for {
			// 		go func() {
			// 			for _, ip := range ips {
			// 				c <- ip
			// 			}
			// 		}()
			// 		go func() {
			// 			for {
			// 				addrInfo.IP = <-c
			// 				err = internal.ResolveHttps(addrInfo, requestOptions)
			// 				if err != nil {
			// 					panicRed(err)
			// 				}
			// 			}
			// 		}()
			// 	}
			// }

		},
	}
)

func init() {
	requestCommand.Flags().StringP("target", "t", "", "[required] Receive responses by proxying the A record of the domain forwarded to the target.")
	requestCommand.Flags().IntP("port", "p", 80, "[optional] For http protocol, the default value is 80.")
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

	rootCmd.AddCommand(requestCommand)
}
