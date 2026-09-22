package dashboard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"
)

// changesPanel answers whether the answer has been stable: what the edges say
// now, how often that has moved, and when it last did.
//
// Status codes are a small enumeration, so every one that has come back is
// worth listing. Body digests are not - an origin that puts a timestamp in its
// output produces a new one on every request - so that row shows the current
// digest and counts the rest. Listing those would grow without limit and push
// the useful part off the side.
type changesPanel struct {
	*tview.TextView

	codes seen
	// Per edge, because two edges can each be perfectly steady while serving
	// different bodies. Folded into one tracker they looked like a single
	// field flipping on every request, and the count climbed forever.
	status map[string]*tracked
	body   map[string]*tracked
}

func newChangesPanel() *changesPanel {
	p := &changesPanel{
		TextView: tview.NewTextView().SetDynamicColors(true),
		status:   map[string]*tracked{},
		body:     map[string]*tracked{},
	}
	p.SetBorder(true).SetTitle(" Changes ")
	p.sync()

	return p
}

func (p *changesPanel) trackerFor(m map[string]*tracked, edge string) *tracked {
	if t, ok := m[edge]; ok {
		return t
	}

	t := &tracked{}
	m[edge] = t

	return t
}

// record folds one answer in. at is when it arrived, used only when something
// actually moved.
func (p *changesPanel) record(edge string, statusCode int, body, at string) {
	code := strconv.Itoa(statusCode)

	p.codes.add(code)
	p.trackerFor(p.status, edge).record(code, at)
	p.trackerFor(p.body, edge).record(body, at)

	p.sync()
}

// recordFailure notes that an edge stopped answering. That is a change like
// any other, and the panel is where a change belongs.
//
// at is the clock here rather than a Date header, because a request that got
// no response carries none - and "last " with nothing after it is worse than
// no timestamp at all.
func (p *changesPanel) recordFailure(edge, at string) {
	p.codes.add(failedLabel)
	p.trackerFor(p.status, edge).record(failedLabel, at)
	p.sync()
}

// digest describes what the edges are saying now: the shared value when they
// agree, and how many distinct ones there are when they do not.
//
// The count is of values, not of edges. Four edges answering A, A, B, B are
// four edges and two bodies, and saying "2 edges differ" of them named the
// wrong thing with the right number.
func (p *changesPanel) digest(m map[string]*tracked, noun string) string {
	var values seen
	for _, t := range m {
		if t.current != "" {
			values.add(t.current)
		}
	}

	switch list := values.list(); len(list) {
	case 0:
		return ""
	case 1:
		return list[0]
	default:
		return fmt.Sprintf("%d distinct %s", len(list), noun)
	}
}

// moves totals the changes across edges, which is what "has this been stable"
// means once there is more than one of them.
func moves(m map[string]*tracked) tracked {
	var total tracked
	for _, t := range m {
		total.changes += t.changes
		if t.at > total.at {
			total.at = t.at
		}
	}

	return total
}

func (p *changesPanel) sync() {
	p.SetText(strings.Join([]string{
		row("Status", colored(p.codes.list(), statusTag), moves(p.status)),
		row("Body", colored([]string{p.digest(p.body, "bodies")}, func(string) string { return "white" }), moves(p.body)),
	}, "\n"))
}

// row lays out one field: what it says, then how restless it has been.
func row(label, values string, t tracked) string {
	if values == "" {
		return fmt.Sprintf("[white]%-7s[gray]waiting", label)
	}

	return fmt.Sprintf("[white]%-7s%s%s", label, values, t.summary())
}

// colored tags each value with the colour chosen for it, so a 503 among the
// codes is visible without reading the number.
func colored(values []string, tag func(string) string) string {
	painted := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		painted = append(painted, fmt.Sprintf("[%s]%s", tag(value), tview.Escape(value)))
	}

	return strings.Join(painted, "  ")
}

// tracked follows one field across a run: what it says now, how often that has
// moved, and when it last did. It keeps one value rather than every value, so
// a field that changes on every request costs the same room as one that never
// changes.
type tracked struct {
	current string
	changes int
	at      string
}

// record notes a value and reports whether it differs from the last one. The
// first value is not a change: there was nothing for it to differ from.
func (t *tracked) record(value, at string) bool {
	if t.current == value {
		return false
	}

	if t.current != "" {
		t.changes++
		t.at = at
	}
	t.current = value

	return true
}

// summary is what follows the value: nothing while it has held still, and how
// restless it has been once it has not.
func (t tracked) summary() string {
	if t.changes == 0 {
		return ""
	}

	when := t.at
	// The date is in the response table; here the time of day is enough, and
	// it leaves room for the digest.
	if _, clock, found := strings.Cut(when, " "); found {
		when = clock
	}

	return fmt.Sprintf("  [gray]· changed %d×, last %s", t.changes, when)
}

// failedLabel stands in for an edge that did not answer, where a status code
// would otherwise go.
const failedLabel = "failed"

// statusTag is the tview colour tag matching statusColor, so the panel and the
// chart agree on what a status class looks like.
func statusTag(code string) string {
	n, err := strconv.Atoi(code)
	if err != nil {
		if code == failedLabel {
			return "gray"
		}
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
