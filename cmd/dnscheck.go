package cmd

import (
	"fmt"

	"github.com/ghdwlsgur/gostat/internal"
	"github.com/spf13/cobra"
)

var (
	dnscheckCommand = &cobra.Command{
		Use:   "dnscheck",
		Short: "test",
		Long:  "test",
		Run: func(cmd *cobra.Command, args []string) {
			value, err := internal.QueryDnsRecord()
			if err != nil {
				fmt.Println("check")
			}
			fmt.Println(value)

		},
	}
)

func init() {
	rootCmd.AddCommand(dnscheckCommand)
}
