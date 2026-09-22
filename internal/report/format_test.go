package report

import "testing"

func TestShortenHeaderName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a long header keeps only segment initials",
			in:   "Access-Control-Allow-Origin",
			want: "ACA-Origin",
		},
		{
			name: "a short header is left alone",
			in:   "Server",
			want: "Server",
		},
		{
			name: "a name that already fits is untouched",
			in:   "Content-Type",
			want: "Content-Type",
		},
		{
			// A leading hyphen leaves an empty segment; slicing it used to panic.
			name: "empty segments are skipped",
			in:   "-Control-Allow-Origin-Extra",
			want: "CAO-Extra",
		},
		{
			// One long trailing segment cannot be squeezed any further, so the
			// recursion has to stop instead of calling itself unchanged.
			name: "an unshortenable name terminates",
			in:   "Verylongsegment-Verylonglastsegment",
			want: "V-Verylonglastsegment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortenHeaderName(tt.in); got != tt.want {
				t.Errorf("shortenHeaderName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPadCountsRunesNotBytes(t *testing.T) {
	// "270µs" is six bytes and five characters. Padding by bytes puts this
	// column one short of every ASCII neighbour.
	got := pad("270µs", "270µs", 8)

	if want := "270µs   "; got != want {
		t.Errorf("pad = %q, want %q", got, want)
	}
}

func TestPadNeverRunsColumnsTogether(t *testing.T) {
	// A value wider than its column still has to be followed by a separator.
	if got := pad("overlong", "overlong", 4); got != "overlong " {
		t.Errorf("pad = %q, want a trailing space", got)
	}
}

func TestSeoulTime(t *testing.T) {
	tests := []struct {
		name string
		date string
		want string
	}{
		{"a GMT header converts to Seoul", "Mon, 02 Oct 2023 00:00:00 GMT", "2023-10-02 09:00:00"},
		{"an offset header converts too", "Mon, 02 Oct 2023 00:00:00 +0000", "2023-10-02 09:00:00"},
		{"garbage is reported, not panicked on", "not a date", "Failed to parse time"},
		{"an absent header stays safe", "", "Failed to parse time"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SeoulTime(tt.date); got != tt.want {
				t.Errorf("SeoulTime(%q) = %q, want %q", tt.date, got, tt.want)
			}
		})
	}
}
