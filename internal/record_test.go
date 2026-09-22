package internal

import "testing"

func TestGetRecordIPv4AcceptsALiteralAddress(t *testing.T) {
	got, err := GetRecordIPv4("127.0.0.1")
	if err != nil {
		t.Fatalf("GetRecordIPv4: %v", err)
	}

	if len(got) != 1 || got[0] != "127.0.0.1" {
		t.Errorf("got %v, want [127.0.0.1]", got)
	}
}

func TestGetRecordIPv4DropsIPv6(t *testing.T) {
	// ::1 has no IPv4 form, so the filter must leave nothing behind rather
	// than pass a bracketed address on to the dialler.
	got, err := GetRecordIPv4("::1")
	if err != nil {
		t.Fatalf("GetRecordIPv4: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got %v, want no IPv4 addresses", got)
	}
}

func TestGetRecordIPv4RejectsAnEmptyName(t *testing.T) {
	if _, err := GetRecordIPv4(""); err == nil {
		t.Error("GetRecordIPv4(\"\") returned nil error")
	}
}
