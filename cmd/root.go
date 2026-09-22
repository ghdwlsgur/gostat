package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the command tree. It takes no package-level state, so
// a test can build a fresh tree, point it at its own output and run it.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "gostat",
		Version: version,
		Short:   `gostat is an interactive CLI tool that proxies the A record of the input domain as a target and returns the response value received from the input URL.`,
		Long:    `gostat is an interactive CLI tool that proxies the A record of the input domain as a target and returns the response value received from the input URL. It can also be used to check latency or to check whether each option is applied to the URL by adding headers and referrers to the request header.`,

		// A bad flag or argument is the user's mistake, not a crash, and the
		// usage text has already been printed by then.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newRequestCommand())

	return root
}

// Execute runs the command tree and turns any error into an exit status.
func Execute(version string) {
	if err := NewRootCommand(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, color.RedString("[err] %s", err))
		os.Exit(1)
	}
}
