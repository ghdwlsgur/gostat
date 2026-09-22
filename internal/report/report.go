package report

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/fatih/color"
	"github.com/ghdwlsgur/gostat/internal/probe"
)

// highlighted response headers are the ones worth spotting at a glance. The
// keys are canonical, because that is the form a header map uses: net/http
// stores "ETag" under "Etag".
var highlighted = map[string]bool{
	http.CanonicalHeaderKey("Last-Modified"):  true,
	http.CanonicalHeaderKey("Content-Length"): true,
	http.CanonicalHeaderKey("Content-Type"):   true,
	http.CanonicalHeaderKey("ETag"):           true,
}

// Terminal writes probe results as the CLI prints them.
type Terminal struct {
	out io.Writer
}

// NewTerminal returns a Terminal writing to out.
func NewTerminal(out io.Writer) *Terminal {
	return &Terminal{out: out}
}

// Result writes the full report for one edge. target is what the user asked
// for, which is only worth repeating when it is not the address that answered.
func (t *Terminal) Result(res *probe.Result, target string) {
	t.heading(res, target)
	t.latency(res.Trace)
	t.requestHeaders(res.Sent)
	t.responseHeaders(res)
}

// AttackProgress rewrites a single line, so a long run does not scroll.
func (t *Terminal) AttackProgress(statusCode int, count int64) {
	fmt.Fprintf(t.out, "\r%s: %d, %s: %d",
		color.HiBlackString("Status Code"), statusCode,
		color.HiBlackString("Request Count"), count)
}

func (t *Terminal) heading(res *probe.Result, target string) {
	edge := res.Edge
	if edge == "" {
		edge = res.Sent.Host
	}

	if target != "" && target != edge {
		fmt.Fprintf(t.out, "\n%s - [%s]\n\n", color.HiYellowString(target), color.HiYellowString(edge))
		return
	}
	fmt.Fprintf(t.out, "\n[%s]\n\n", color.HiYellowString(edge))
}

// latency prints one row per phase with a running total beside it, then the
// measured wall clock. The phases are disjoint, so the running total is the
// sum of everything shown above it.
func (t *Terminal) latency(trace probe.Trace) {
	fmt.Fprintln(t.out, color.HiWhiteString("Latency Status"))

	var elapsed time.Duration
	for _, phase := range trace.Phases() {
		elapsed += phase.Duration
		fmt.Fprintf(t.out, "\t%s%s%s\n",
			pad(color.HiWhiteString(phase.Name), phase.Name, phaseWidth),
			pad(color.HiGreenString(phase.Duration.String()), phase.Duration.String(), durationWidth),
			color.HiMagentaString(elapsed.String()))
	}

	total := trace.Total.String()
	fmt.Fprintf(t.out, "\t%s%s\n",
		pad(color.HiWhiteString("Total"), "Total", phaseWidth+durationWidth),
		color.HiMagentaString(total))

	// A pooled connection skips the phases above, and a zero there means
	// "did not happen again", not "took no time".
	if trace.Reused {
		fmt.Fprintf(t.out, "\t%s\n", color.HiBlackString("connection reused; connection phases did not run"))
	}
	fmt.Fprintln(t.out)
}

func (t *Terminal) requestHeaders(sent probe.Sent) {
	fmt.Fprintf(t.out, "%s\n", color.HiWhiteString("Request Headers"))

	if sent.HostOverridden {
		t.field("Host", sent.Host, false)
	}
	if sent.Referer != "" {
		t.field("Referer", sent.Referer, false)
	}
	if sent.Authorization != "" {
		t.field("Authorization", sent.Authorization, false)
	}
	if sent.Range != "" {
		t.field("Range", sent.Range, false)
	}

	fmt.Fprintln(t.out)
}

func (t *Terminal) responseHeaders(res *probe.Result) {
	fmt.Fprintf(t.out, "%s\n", color.HiWhiteString("Response Headers"))

	t.field("Proto", res.Proto, false)
	t.field("Status", statusColor(res.StatusCode)(res.Status), false)

	// Sorted, because ranging a header map hands back a different order every
	// run and two runs of the same command should be diffable.
	names := make([]string, 0, len(res.Headers))
	for name := range res.Headers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		values := res.Headers[name]
		if len(values) == 0 {
			continue
		}
		t.field(shortenHeaderName(name), values[0], highlighted[name])

		// A header sent more than once, such as Set-Cookie, keeps its own
		// lines instead of being silently dropped.
		for _, extra := range values[1:] {
			t.field("", extra, highlighted[name])
		}
	}

	fmt.Fprintf(t.out, "\n%s %s\n\n",
		color.HiBlackString(fmt.Sprintf("Body %d bytes, sha256", res.BodyBytes)),
		color.HiBlackString(base64.StdEncoding.EncodeToString(res.BodySum)))
}

func (t *Terminal) field(name, value string, highlight bool) {
	paint := color.HiBlackString
	if highlight {
		paint = color.HiWhiteString
	}

	fmt.Fprintf(t.out, "%s%s\n",
		pad(color.HiBlackString(name), name, headerWidth),
		paint(value))
}

// statusColor picks the colour for a status line: green for success, yellow
// for a client error, red for a server error, blue for a redirect.
func statusColor(statusCode int) func(string, ...interface{}) string {
	switch statusCode / 100 {
	case 3:
		return color.HiBlueString
	case 4:
		return color.HiYellowString
	case 5:
		return color.HiRedString
	default:
		return color.HiGreenString
	}
}
