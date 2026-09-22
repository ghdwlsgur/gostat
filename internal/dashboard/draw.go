package dashboard

import (
	"encoding/base64"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// barRune is the block every bar and every status cell is drawn from.
const barRune = '█'

// Column widths shared by the drawn panels, in characters.
const (
	labelMaxWidth = 15
	totalWidth    = 10
	minBarWidth   = 8

	// hashPrefix is how much of the body digest a cell shows. A column has no
	// room for all 44 base64 characters, and 12 of them still pin the content.
	hashPrefix = 12
)

// legendEntry draws one swatch and its label, returning where the next entry
// starts or -1 when there was no room for this one.
func legendEntry(screen tcell.Screen, at, y, limit int, color tcell.Color, label string) int {
	if at+len(label)+2 > limit {
		return -1
	}

	screen.SetContent(at, y, barRune, nil, tcell.StyleDefault.Foreground(color))
	tview.Print(screen, label, at+1, y, len(label), tview.AlignLeft, tcell.ColorGray)

	return at + len(label) + 2
}

// labelWidth is what the edge column needs, capped so one long name cannot
// crowd out everything drawn beside it.
func labelWidth(edges []string) int {
	width := 0
	for _, edge := range edges {
		if n := utf8.RuneCountInString(edge); n > width {
			width = n
		}
	}

	if width > labelMaxWidth {
		width = labelMaxWidth
	}

	return width + 1
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	if width <= 1 {
		return string([]rune(s)[:width])
	}

	return fmt.Sprintf("%s…", string([]rune(s)[:width-1]))
}

// statusColor maps a status class onto the colour it is shown in. The chart,
// the response table and the changes panel all use it, so a 503 looks the same
// wherever it turns up.
func statusColor(statusCode int) tcell.Color {
	switch statusCode / 100 {
	case 2:
		return tcell.ColorGreen
	case 3:
		return tcell.ColorBlue
	case 4:
		return tcell.ColorYellow
	case 5:
		return tcell.ColorRed
	default:
		return tcell.ColorWhite
	}
}

// shortHash renders a body digest short enough for a table cell.
func shortHash(sum []byte) string {
	if len(sum) == 0 {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(sum)
	if len(encoded) > hashPrefix {
		return encoded[:hashPrefix]
	}

	return encoded
}

// compactDuration keeps a duration inside its column. Three significant
// figures is as much as a bar or a table cell can justify.
func compactDuration(d time.Duration) string {
	switch {
	case d <= 0:
		// A phase that did not happen reads better as 0s than as 0ns.
		return "0s"
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	case d >= time.Microsecond:
		return fmt.Sprintf("%.0fµs", float64(d)/float64(time.Microsecond))
	default:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
}

func contains(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}

func newTable(title string) *tview.Table {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", title))

	return table
}

func labelCell(text string) *tview.TableCell {
	return tview.NewTableCell(text).
		SetTextColor(tcell.ColorWhite).
		SetAttributes(tcell.AttrBold).
		SetSelectable(false)
}

func valueCell(text string) *tview.TableCell {
	return tview.NewTableCell(text).
		SetTextColor(tcell.ColorDefault).
		SetSelectable(false)
}
