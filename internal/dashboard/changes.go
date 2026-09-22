package dashboard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"
)

// changesPanel keeps the distinct values the run has turned up: which status
// codes came back, which bodies, and when the status last changed.
//
// Three values in one frame, rather than three frames holding one short token
// each - which is what the same information used to cost.
type changesPanel struct {
	*tview.TextView

	status seen
	body   seen
	since  seen
}

func newChangesPanel() *changesPanel {
	p := &changesPanel{TextView: tview.NewTextView().SetDynamicColors(true)}
	p.SetBorder(true).SetTitle(" Changes ")
	p.sync()

	return p
}

// record folds one result in. The timestamp is only worth keeping when the
// status changed: the same code repeating is not an event.
func (p *changesPanel) record(statusCode int, body, at string) {
	if p.status.add(strconv.Itoa(statusCode)) {
		p.since.add(at)
	}
	p.body.add(body)

	p.sync()
}

func (p *changesPanel) sync() {
	p.SetText(strings.Join([]string{
		line("Status", colored(p.status.list(), statusTag)),
		line("Body", colored(p.body.list(), func(string) string { return "white" })),
		line("Since", colored(p.since.list(), func(string) string { return "gray" })),
	}, "\n"))
}

func line(label, values string) string {
	if values == "" {
		values = "[gray]waiting"
	}

	return fmt.Sprintf("[white]%-7s%s", label, values)
}

// colored tags each value with the colour tag chosen for it, so a 503 among
// the codes is visible without reading the number.
func colored(values []string, tag func(string) string) string {
	painted := make([]string, len(values))
	for i, value := range values {
		painted[i] = fmt.Sprintf("[%s]%s", tag(value), tview.Escape(value))
	}

	return strings.Join(painted, "  ")
}

// statusTag is the tview colour tag matching statusColor, so the panel and the
// chart agree on what a status class looks like.
func statusTag(code string) string {
	n, err := strconv.Atoi(code)
	if err != nil {
		return "white"
	}

	switch n / 100 {
	case 2:
		return "green"
	case 3:
		return "blue"
	case 4:
		return "yellow"
	case 5:
		return "red"
	default:
		return "white"
	}
}

// seen is the ordered set of distinct values a field has taken.
type seen struct {
	values []string
}

// add records value and reports whether it had not been seen before.
func (s *seen) add(value string) bool {
	for _, existing := range s.values {
		if existing == value {
			return false
		}
	}
	s.values = append(s.values, value)

	return true
}

// list returns the values recorded so far, as a copy.
func (s *seen) list() []string {
	out := make([]string, len(s.values))
	copy(out, s.values)

	return out
}
