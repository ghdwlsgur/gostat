package dashboard

import (
	"fmt"
	"strings"
	"testing"
)

func TestSeenKeepsDistinctValuesInOrder(t *testing.T) {
	var s seen

	if !s.add("200") {
		t.Error("the first value was not reported as new")
	}
	if s.add("200") {
		t.Error("a repeated value was reported as new")
	}
	s.add("404")

	want := []string{"200", "404"}
	got := s.list()
	if len(got) != len(want) {
		t.Fatalf("list = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("list[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSeenHandsOutACopy(t *testing.T) {
	var s seen
	s.add("abc")

	s.list()[0] = "tampered"

	if s.list()[0] != "abc" {
		t.Error("list shares its backing array with the set")
	}
}

// A body digest is unbounded - an origin that puts a timestamp in its output
// produces a new one every request - so the panel keeps one value and counts
// the rest rather than listing them.
func TestTrackedKeepsOneValueHoweverOftenItMoves(t *testing.T) {
	var body tracked

	for i := 0; i < 50; i++ {
		body.record(fmt.Sprintf("digest-%d", i), "2026-09-22 19:44:41")
	}

	if body.current != "digest-49" {
		t.Errorf("current = %q, want the newest", body.current)
	}
	// 50 values is 49 changes: the first had nothing to differ from.
	if body.changes != 49 {
		t.Errorf("changes = %d, want 49", body.changes)
	}
	if n := len(body.summary()); n > 60 {
		t.Errorf("summary is %d characters after 50 values; it must not grow with them", n)
	}
}

func TestTrackedReportsOnlyRealChanges(t *testing.T) {
	var s tracked

	if !s.record("206", "2026-09-22 19:00:00") {
		t.Error("the first value was not reported as a change")
	}
	if s.record("206", "2026-09-22 19:00:01") {
		t.Error("the same value again was reported as a change")
	}
	// The first value is not a change: there was nothing for it to differ from.
	if s.changes != 0 {
		t.Errorf("changes = %d after one value, want 0", s.changes)
	}

	s.record("503", "2026-09-22 19:00:02")
	if s.changes != 1 {
		t.Errorf("changes = %d after a real change, want 1", s.changes)
	}
	if s.at != "2026-09-22 19:00:02" {
		t.Errorf("at = %q, want when it changed", s.at)
	}
}

// Nothing follows a value that has held still, so a steady run stays quiet.
func TestTrackedSaysNothingWhileItHoldsStill(t *testing.T) {
	var s tracked
	s.record("206", "2026-09-22 19:00:00")

	if got := s.summary(); got != "" {
		t.Errorf("summary = %q, want nothing while it has not moved", got)
	}

	s.record("503", "2026-09-22 19:00:02")
	summary := s.summary()
	for _, want := range []string{"changed 1", "19:00:02"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary = %q, want it to mention %q", summary, want)
		}
	}
	// The date is in the response table; repeating it here only costs room.
	if strings.Contains(summary, "2026-09-22") {
		t.Errorf("summary = %q, want the time of day only", summary)
	}
}

func TestChangesPanelShowsTheCodesAndTheCurrentBody(t *testing.T) {
	p := newChangesPanel()

	p.record("1.1.1.1", 200, "aaa", "2026-09-22 10:00:00")
	p.record("1.1.1.1", 503, "bbb", "2026-09-22 10:00:10")
	p.record("1.1.1.1", 200, "ccc", "2026-09-22 10:00:20")

	text := p.GetText(true)

	// Every code that came back is listed: there are few of them by nature.
	for _, want := range []string{"200", "503"} {
		if !strings.Contains(text, want) {
			t.Errorf("the panel does not list the code %q:\n%s", want, text)
		}
	}
	// Only the newest digest, with the rest counted.
	if !strings.Contains(text, "ccc") {
		t.Errorf("the panel does not show the current body:\n%s", text)
	}
	for _, gone := range []string{"aaa", "bbb"} {
		if strings.Contains(text, gone) {
			t.Errorf("the panel still lists the old digest %q:\n%s", gone, text)
		}
	}
	if !strings.Contains(text, "changed 2") {
		t.Errorf("the panel does not count the body changes:\n%s", text)
	}
}

// A 503 among the codes has to be visible without reading the number.
func TestChangesPanelColorsTheStatusCodes(t *testing.T) {
	p := newChangesPanel()
	p.record("1.1.1.1", 200, "aaa", "2026-09-22 10:00:00")
	p.record("1.1.1.1", 503, "bbb", "2026-09-22 10:00:10")

	// GetText(false) keeps the colour tags in.
	text := p.GetText(false)
	for _, want := range []string{"[green]200", "[red]503"} {
		if !strings.Contains(text, want) {
			t.Errorf("the panel does not tag %q:\n%s", want, text)
		}
	}
}

func TestChangesPanelWaitsBeforeAnythingLands(t *testing.T) {
	text := newChangesPanel().GetText(true)

	for _, want := range []string{"Status", "Body", "waiting"} {
		if !strings.Contains(text, want) {
			t.Errorf("the empty panel does not show %q:\n%s", want, text)
		}
	}
}

func TestStatusTag(t *testing.T) {
	tests := map[string]string{
		"200":  "green",
		"301":  "blue",
		"404":  "yellow",
		"503":  "red",
		"100":  "white",
		"what": "white",
	}

	for code, want := range tests {
		if got := statusTag(code); got != want {
			t.Errorf("statusTag(%q) = %q, want %q", code, got, want)
		}
	}
}

// Two edges can each be perfectly steady while serving different bodies.
// Folded into one tracker they looked like a single field flipping on every
// request, and the change count climbed forever.
func TestChangesDoesNotCountSteadyEdgesAsChanging(t *testing.T) {
	p := newChangesPanel()

	for i := 0; i < 10; i++ {
		p.record("1.1.1.1", 200, "aaa", "2026-09-22 10:00:00")
		p.record("2.2.2.2", 200, "bbb", "2026-09-22 10:00:00")
	}

	for edge, tracker := range p.body {
		if tracker.changes != 0 {
			t.Errorf("edge %s held one body throughout but counted %d changes", edge, tracker.changes)
		}
	}

	// They disagree, and the panel says so rather than picking one.
	text := p.GetText(true)
	if !strings.Contains(text, "2 edges differ") {
		t.Errorf("the panel does not report the disagreement:\n%s", text)
	}
	for _, digest := range []string{"aaa", "bbb"} {
		if strings.Contains(text, digest) {
			t.Errorf("the panel picked %q as if it were the answer:\n%s", digest, text)
		}
	}
}

func TestChangesShowsTheSharedDigestWhenTheEdgesAgree(t *testing.T) {
	p := newChangesPanel()
	p.record("1.1.1.1", 200, "same", "2026-09-22 10:00:00")
	p.record("2.2.2.2", 200, "same", "2026-09-22 10:00:00")

	if text := p.GetText(true); !strings.Contains(text, "same") {
		t.Errorf("the panel hides a digest the edges agree on:\n%s", text)
	}
}

// An edge that stops answering is a change, and the panel is where a change
// belongs.
func TestChangesRecordsAFailure(t *testing.T) {
	p := newChangesPanel()
	p.record("1.1.1.1", 200, "aaa", "2026-09-22 10:00:00")
	p.recordFailure("1.1.1.1", "2026-09-22 10:00:10")

	text := p.GetText(true)
	if !strings.Contains(text, failedLabel) {
		t.Errorf("the panel does not note the failure:\n%s", text)
	}
	if got := p.status["1.1.1.1"].changes; got != 1 {
		t.Errorf("the failure counted as %d changes, want 1", got)
	}
}
