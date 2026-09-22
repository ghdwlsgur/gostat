package internal

import "testing"

func TestStringFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "long header keeps only segment initials",
			in:   "Access-Control-Allow-Origin",
			want: "ACA-Origin",
		},
		{
			name: "a name without a hyphen is left alone",
			in:   "Server",
			want: "Server",
		},
		{
			name: "two segments collapse to one initial",
			in:   "Content-Length",
			want: "C-Length",
		},
		{
			// A leading hyphen leaves an empty segment; slicing it used to panic.
			name: "empty segments are skipped",
			in:   "-Control-Allow-Origin",
			want: "CA-Origin",
		},
		{
			name: "double hyphen is survivable",
			in:   "X--Forwarded--For-Something",
			want: "XFF-Something",
		},
		{
			// One long trailing segment cannot be shortened any further; the
			// recursion has to stop instead of calling itself with the same
			// string over and over.
			name: "an unshortenable name terminates",
			in:   "Verylongsegment-Verylonglastsegment",
			want: "V-Verylonglastsegment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringFormat(tt.in); got != tt.want {
				t.Errorf("stringFormat(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
