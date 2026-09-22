package internal

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
)

// Title text is displayed in black letters.
func PrintFunc(field, value string) {
	switch field {
	case "Last-Modified", "Content-Length", "Content-Type", "ETag":
		printHiWhiteString(field, value)
	default:
		printHiBlackString(field, value)
	}
}

func printHiBlackString(field, value string) {
	if len(field) < 8 {
		fmt.Printf("%s\t\t%s\n", color.HiBlackString(field), color.HiBlackString(value))
	} else {
		fmt.Printf("%s\t%s\n", color.HiBlackString(field), color.HiBlackString(value))
	}
}

func printHiWhiteString(field, value string) {
	if len(field) < 8 {
		fmt.Printf("%s\t\t%s\n", color.HiBlackString(field), color.HiWhiteString(value))
	} else {
		fmt.Printf("%s\t%s\n", color.HiBlackString(field), color.HiWhiteString(value))
	}
}

// Column widths of the latency table, in visible characters.
const (
	latencyFieldWidth    = 20
	latencyDurationWidth = 16
)

// printStatusFormat prints one row of the latency table. The padding is worked
// out from the plain text: colour escapes are bytes that fmt counts towards a
// width but the terminal never shows, so %-Ns on an already-coloured string
// lines up differently depending on whether colour is on.
func printStatusFormat(field, duration, elapsed string) {
	fmt.Printf("\t%s%s%s\n",
		padded(color.HiWhiteString(field), field, latencyFieldWidth),
		padded(color.HiGreenString(duration), duration, latencyDurationWidth),
		color.HiMagentaString(elapsed))
}

// printStatusTotal prints the closing row, lined up under the elapsed column.
func printStatusTotal(elapsed string) {
	fmt.Printf("\t%s%s\n\n",
		padded(color.HiWhiteString("Total"), "Total", latencyFieldWidth+latencyDurationWidth),
		color.HiMagentaString(elapsed))
}

// padded right-pads coloured to the visible width that plain would occupy.
// The count is in runes, not bytes: a duration like "270µs" is one byte wider
// than it looks on screen.
func padded(coloured, plain string, width int) string {
	if n := width - utf8.RuneCountInString(plain); n > 0 {
		return coloured + strings.Repeat(" ", n)
	}
	return coloured + " "
}

// stringFormat shortens a long header name by keeping only the first letter of
// every segment but the last, so "Access-Control-Allow-Origin" prints as
// "ACA-Origin" and still fits the column.
func stringFormat(word string) string {
	words := strings.Split(word, "-")
	if len(words) < 2 {
		return word
	}

	var prefixBucket []string
	for _, w := range words[:len(words)-1] {
		if w == "" {
			continue
		}
		prefixBucket = append(prefixBucket, w[:1])
	}
	front := strings.Join(prefixBucket, "")
	wordFormat := strings.Join([]string{front, words[len(words)-1]}, "-")

	// Recurse only while the name is still getting shorter; a single long
	// segment cannot be squeezed any further and would loop forever.
	if len(wordFormat) > 14 && len(wordFormat) < len(word) {
		return stringFormat(wordFormat)
	}
	return wordFormat
}

func printStatusToColor(status string) {

	stat := status[0:1]
	if stat == "5" {
		PrintFunc("Status", color.HiRedString(status))
	} else if stat == "4" {
		PrintFunc("Status", color.HiYellowString(status))
	} else {
		PrintFunc("Status", color.HiGreenString(status))
	}
}
