// Package report renders a probe result for a terminal.
package report

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Column widths, in visible characters.
const (
	phaseWidth    = 20
	durationWidth = 16
	headerWidth   = 18
)

// maxHeaderName is the longest response header name printed in full.
const maxHeaderName = 14

// pad right-pads coloured to the width plain would occupy on screen.
//
// The count is in runes, not bytes: colour escapes and a duration like "270µs"
// both make a string wider in bytes than it looks, so %-Ns lines up
// differently depending on whether colour is on.
func pad(coloured, plain string, width int) string {
	if n := width - utf8.RuneCountInString(plain); n > 0 {
		return coloured + strings.Repeat(" ", n)
	}
	return coloured + " "
}

// shortenHeaderName keeps only the first letter of every segment but the last,
// so "Access-Control-Allow-Origin" prints as "ACA-Origin" and still fits the
// column.
func shortenHeaderName(name string) string {
	if len(name) <= maxHeaderName {
		return name
	}

	segments := strings.Split(name, "-")
	if len(segments) < 2 {
		return name
	}

	var initials []string
	for _, segment := range segments[:len(segments)-1] {
		if segment == "" {
			continue
		}
		initials = append(initials, segment[:1])
	}

	shortened := strings.Join(initials, "") + "-" + segments[len(segments)-1]

	// Recurse only while the name is still getting shorter. One long trailing
	// segment cannot be squeezed any further and would otherwise loop forever.
	if len(shortened) > maxHeaderName && len(shortened) < len(name) {
		return shortenHeaderName(shortened)
	}
	return shortened
}

// Now is the local clock in the same shape SeoulTime renders a header in, for
// the moments there is no header to read - a request that got no response.
func Now() string {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.Now().Format("2006-01-02 15:04:05")
	}

	return time.Now().In(seoul).Format("2006-01-02 15:04:05")
}

// SeoulTime reformats an HTTP date header in Asia/Seoul, or explains why it
// could not. A response with no usable Date is not an error worth aborting on.
func SeoulTime(httpDate string) string {
	t, err := time.Parse(time.RFC1123, httpDate)
	if err != nil {
		if t, err = time.Parse(time.RFC1123Z, httpDate); err != nil {
			return "Failed to parse time"
		}
	}

	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return "Failed to load location"
	}

	return t.In(seoul).Format("2006-01-02 15:04:05")
}
