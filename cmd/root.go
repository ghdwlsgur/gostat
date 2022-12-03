package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	// rootCmd represents the base command when called without any sub-commands
	rootCmd = &cobra.Command{
		Use:   "gostat",
		Short: `gostat is an interactive CLI tool that proxies the A record of the input domain as a target and returns the response value received from the input URL.`,
		Long:  `gostat is an interactive CLI tool that proxies the A record of the input domain as a target and returns the response value received from the input URL. It can also be used to check latency or to check whether each option is applied to the URL by adding headers and referrers to the request header.`,
	}
)

// panicRed raises error with text.
func panicRed(err error) {
	fmt.Println(color.RedString("[err] %s", err.Error()))
	os.Exit(1)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		panicRed(err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	args := os.Args[1:]
	_, _, err := rootCmd.Find(args)
	if err != nil {
		panicRed(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}
