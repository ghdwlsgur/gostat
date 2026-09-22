package dashboard

import (
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

// The timestamp marks when the status changed, so a status repeating must not
// add one - otherwise the line fills with the same moment over and over.
func TestChangesRecordsATimestampOnlyWhenTheStatusChanges(t *testing.T) {
	p := newChangesPanel()

	p.record(200, "aaa", "2026-09-22 10:00:00")
	p.record(200, "aaa", "2026-09-22 10:00:05")
	p.record(503, "bbb", "2026-09-22 10:00:10")

	if got := len(p.since.list()); got != 2 {
		t.Errorf("kept %d timestamps for 2 status changes: %v", got, p.since.list())
	}
	if got := len(p.status.list()); got != 2 {
		t.Errorf("kept %d statuses, want 200 and 503: %v", got, p.status.list())
	}
	if got := len(p.body.list()); got != 2 {
		t.Errorf("kept %d bodies, want two distinct ones: %v", got, p.body.list())
	}
}

// A 503 among the codes has to be visible without reading the number.
func TestChangesColorsTheStatusCodes(t *testing.T) {
	p := newChangesPanel()
	p.record(200, "aaa", "2026-09-22 10:00:00")
	p.record(503, "bbb", "2026-09-22 10:00:10")

	// GetText(false) keeps the colour tags in.
	text := p.GetText(false)
	for _, want := range []string{"[green]200", "[red]503"} {
		if !strings.Contains(text, want) {
			t.Errorf("the panel does not tag %q:\n%s", want, text)
		}
	}
}

func TestChangesShowsAllThreeFieldsBeforeAnythingLands(t *testing.T) {
	text := newChangesPanel().GetText(true)

	for _, want := range []string{"Status", "Body", "Since", "waiting"} {
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
