package probe

import "testing"

func TestLookupIPv4AcceptsALiteralAddress(t *testing.T) {
	got, err := LookupIPv4("127.0.0.1")
	if err != nil {
		t.Fatalf("LookupIPv4: %v", err)
	}

	if len(got) != 1 || got[0] != "127.0.0.1" {
		t.Errorf("got %v, want [127.0.0.1]", got)
	}
}

// An address with no IPv4 form has to come back empty rather than be handed on
// to a dialler that would join it to a port without brackets.
func TestLookupIPv4DropsIPv6(t *testing.T) {
	got, err := LookupIPv4("::1")
	if err != nil {
		t.Fatalf("LookupIPv4: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got %v, want no IPv4 addresses", got)
	}
}

func TestLookupIPv4RejectsAnEmptyName(t *testing.T) {
	if _, err := LookupIPv4(""); err == nil {
		t.Error(`LookupIPv4("") returned a nil error`)
	}
}
