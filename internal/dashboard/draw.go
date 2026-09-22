package dashboard

import (
	"fmt"
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
