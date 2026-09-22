package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the command tree. Nothing lives in a package
// variable, so a test can build a tree of its own and point it at its own
// output.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "gostat",
		Version: version,
		Short:   `gostat is an interactive CLI tool that proxies the A record of the input domain as a target and returns the response value received from the input URL.`,
		Long:    `gostat is an interactive CLI tool that proxies the A record of the input domain as a target and returns the response value received from the input URL. It can also be used to check latency or to check whether each option is applied to the URL by adding headers and referrers to the request header.`,

		// A bad flag or argument is the user's mistake, not a crash, and
		// cobra has already printed the usage by the time the error surfaces.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newRequestCommand())

	return root
}

// Execute runs the command tree and turns an error into an exit status.
//
// Ctrl-c cancels the context rather than killing the process, so an in-flight
// request unwinds and the terminal is handed back the way it was found.
func Execute(version string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := NewRootCommand(version).ExecuteContext(ctx)
	switch {
	case err == nil, errors.Is(err, context.Canceled):
		return
	default:
		fmt.Fprintln(os.Stderr, color.RedString("[err] %s", err))
		os.Exit(1)
	}
}
