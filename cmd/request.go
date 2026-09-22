package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/ghdwlsgur/gostat/internal/dashboard"
	"github.com/ghdwlsgur/gostat/internal/probe"
	"github.com/ghdwlsgur/gostat/internal/report"
)

// requestFlags is what the request command accepts on the command line.
type requestFlags struct {
	target        string
	port          int
	threads       int
	host          string
	referer       string
	authorization string
	attack        bool
	dashboard     bool
}

func newRequestCommand() *cobra.Command {
	flags := &requestFlags{}

	cmd := &cobra.Command{
		Use:   "request <url>",
		Short: "Exec `gostat request https://domain.com -t domain.com`",
		Long:  "Receives the response of the URL to each A record of the target domain to the url using the http or https protocol.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRequest(cmd.Context(), cmd.OutOrStdout(), args[0], flags)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&flags.target, "target", "t", "", "Address or domain to send the request to instead of resolving the url. Every A record behind it is probed in turn.")
	f.IntVarP(&flags.port, "port", "p", 0, "Port to connect to (default 80 for http, 443 for https)")
	f.StringVarP(&flags.host, "host", "H", "", "Host header to send, without changing where the request goes")
	f.StringVarP(&flags.referer, "referer", "r", "", "Referer header to send")
	f.StringVarP(&flags.authorization, "authorization", "A", "", "Authorization header to send")
	f.BoolVarP(&flags.dashboard, "dashboard", "d", false, "Draw the live dashboard instead of printing once")
	f.BoolVarP(&flags.attack, "attack", "a", false, "Keep requesting in a loop, printing only the status code")
	f.IntVarP(&flags.threads, "thread", "n", 1, "How many workers attack mode runs")

	return cmd
}

func runRequest(ctx context.Context, out io.Writer, arg string, flags *requestFlags) error {
	u, err := probe.ParseURL(arg)
	if err != nil {
		return err
	}

	target := strings.TrimSpace(flags.target)
	if target == "" {
		target = u.Hostname()
	}

	edges, err := probe.LookupIPv4(target)
	if err != nil {
		return err
	}
	if len(edges) == 0 {
		return fmt.Errorf("no IPv4 address found for %q", target)
	}

	opts := probe.Options{
		Host:          strings.TrimSpace(flags.host),
		Referer:       strings.TrimSpace(flags.referer),
		Authorization: strings.TrimSpace(flags.authorization),
		Port:          flags.port,
		Range:         probe.DefaultRange,
	}
	if flags.attack {
		// Attack mode is about load rather than about reading one response,
		// so it asks for whatever a real client would get.
		opts.Range = ""
	}

	client := probe.New(opts)

	switch {
	case flags.dashboard:
		return dashboard.Run(ctx, client, u, edges)
	case flags.attack:
		return attack(ctx, out, client, u, edges, flags.threads)
	default:
		return sweep(ctx, out, client, u, target, edges)
	}
}

// sweep probes every edge once and prints a report for each.
func sweep(ctx context.Context, out io.Writer, client *probe.Client, u *url.URL, target string, edges []string) error {
	terminal := report.NewTerminal(out)

	for _, edge := range edges {
		res, err := client.Do(ctx, u, edge)
		if err != nil {
			return stopped(ctx, err)
		}
		terminal.Result(res, target)
	}

	return nil
}

// attack keeps every worker requesting until one fails or the run is
// cancelled. Only the counter goroutine writes to out, so the workers cannot
// interleave halfway through a line.
func attack(parent context.Context, out io.Writer, client *probe.Client, u *url.URL, edges []string, threads int) error {
	if threads < 1 {
		threads = 1
	}

	// The workers share a context this function cancels on the first failure,
	// so whether the run was stopped from outside has to be asked of the
	// parent. Asking the derived one would answer "cancelled" every time
	// something broke, and report a dead edge as a clean exit.
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	statuses := make(chan int, threads)
	printed := make(chan struct{})
	go func() {
		defer close(printed)

		terminal := report.NewTerminal(out)
		var count int64
		for status := range statuses {
			count++
			terminal.AttackProgress(status, count)
		}
	}()

	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
	)

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				for _, edge := range edges {
					if ctx.Err() != nil {
						return
					}

					res, err := client.Do(ctx, u, edge)
					if err != nil {
						// The first failure ends the run; the rest of the
						// workers see the cancelled context and stop between
						// requests instead of being killed mid-flight.
						once.Do(func() {
							first = err
							cancel()
						})
						return
					}
					statuses <- res.StatusCode
				}
			}
		}()
	}

	wg.Wait()
	close(statuses)
	<-printed
	fmt.Fprintln(out)

	return stopped(parent, first)
}

// stopped reports err unless the run was cancelled from outside, which is what
// ctrl-c looks like from down here. ctx must be the context handed in, not one
// this package cancelled itself.
func stopped(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return nil
	}

	return err
}
