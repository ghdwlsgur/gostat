package dashboard

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// drawStatus renders the chart on its own and returns the screen as text
// alongside the colour of every block it drew, row by row.
func drawStatus(t *testing.T, width, height int, c *statusChart) (string, [][]tcell.Color) {
	t.Helper()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(width, height)

	c.SetRect(0, 0, width, height)
	c.Draw(screen)
	screen.Show()

	cells, w, h := screen.GetContents()
	blocks := make([][]tcell.Color, h)

	var text strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := cells[y*w+x]
			if len(cell.Runes) == 0 {
				text.WriteRune(' ')
				continue
			}
			text.WriteRune(cell.Runes[0])
			if cell.Runes[0] == barRune {
				fg, _, _ := cell.Style.Decompose()
				blocks[y] = append(blocks[y], fg)
			}
		}
		text.WriteByte('\n')
	}

	return text.String(), blocks
}

// One edge answering differently from the rest has to be visible without
// reading anything, which is the whole point of the strip.
func TestStatusChartColorsEachEdgeByClass(t *testing.T) {
	c := newStatusChart([]string{"1.1.1.1", "2.2.2.2"})
	c.record("1.1.1.1", http.StatusOK)
	c.record("2.2.2.2", http.StatusServiceUnavailable)

	text, blocks := drawStatus(t, 60, 8, c)

	// Row 1 is the first edge, row 2 the second.
	if len(blocks[1]) == 0 || len(blocks[2]) == 0 {
		t.Fatalf("an edge drew no blocks:\n%s", text)
	}
	if got := blocks[1][len(blocks[1])-1]; got != tcell.ColorGreen {
		t.Errorf("the 200 edge ends in %v, want green:\n%s", got, text)
	}
	if got := blocks[2][len(blocks[2])-1]; got != tcell.ColorRed {
		t.Errorf("the 503 edge ends in %v, want red:\n%s", got, text)
	}
}

// A run that starts answering differently has to show as a colour change part
// way along the strip, not as a strip redrawn in one colour.
func TestStatusChartKeepsTheHistory(t *testing.T) {
	c := newStatusChart([]string{"1.1.1.1"})
	for i := 0; i < 3; i++ {
		c.record("1.1.1.1", http.StatusOK)
	}
	c.record("1.1.1.1", http.StatusServiceUnavailable)

	_, blocks := drawStatus(t, 60, 8, c)
	row := blocks[1]

	if len(row) < 4 {
		t.Fatalf("drew %d blocks for 4 samples", len(row))
	}
	if row[len(row)-1] != tcell.ColorRed {
		t.Errorf("the newest sample is %v, want the 503 in red", row[len(row)-1])
	}
	if row[len(row)-2] != tcell.ColorGreen {
		t.Errorf("the sample before it is %v, want a 200 in green", row[len(row)-2])
	}
}

func TestStatusChartShowsTheLatestCode(t *testing.T) {
	c := newStatusChart([]string{"1.1.1.1"})
	c.record("1.1.1.1", http.StatusOK)
	c.record("1.1.1.1", http.StatusNotFound)

	text, _ := drawStatus(t, 60, 8, c)

	if !strings.Contains(text, "404") {
		t.Errorf("the latest code is not shown:\n%s", text)
	}
	if c.latest("1.1.1.1") != http.StatusNotFound {
		t.Errorf("latest() = %d, want 404", c.latest("1.1.1.1"))
	}
	if c.latest("9.9.9.9") != 0 {
		t.Errorf("latest() = %d for an edge with no samples, want 0", c.latest("9.9.9.9"))
	}
}

// The legend names only what happened, so it growing is the signal.
func TestStatusChartLegendNamesTheClassesSeen(t *testing.T) {
	c := newStatusChart([]string{"1.1.1.1"})
	c.record("1.1.1.1", http.StatusOK)

	text, _ := drawStatus(t, 60, 8, c)
	if !strings.Contains(text, "2xx") {
		t.Errorf("the legend does not name the 2xx it has seen:\n%s", text)
	}
	if strings.Contains(text, "5xx") {
		t.Errorf("the legend names a class that never happened:\n%s", text)
	}

	c.record("1.1.1.1", http.StatusServiceUnavailable)
	if text, _ := drawStatus(t, 60, 8, c); !strings.Contains(text, "5xx") {
		t.Errorf("the legend did not pick up the 503:\n%s", text)
	}
}

func TestClassesSeenAreSortedAndDistinct(t *testing.T) {
	c := newStatusChart([]string{"1.1.1.1", "2.2.2.2"})
	c.record("2.2.2.2", http.StatusServiceUnavailable)
	c.record("1.1.1.1", http.StatusOK)
	c.record("1.1.1.1", http.StatusOK)
	c.record("2.2.2.2", http.StatusNotFound)

	want := []int{2, 4, 5}
	got := c.classesSeen()
	if len(got) != len(want) {
		t.Fatalf("classesSeen() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("classesSeen()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// Without a cap the history would grow until it ran off the side of the panel.
func TestStatusChartCapsItsHistory(t *testing.T) {
	c := newStatusChart([]string{"1.1.1.1"})
	for i := 0; i < maxSamples+5; i++ {
		c.record("1.1.1.1", http.StatusOK)
	}

	if got := len(c.samples["1.1.1.1"]); got != maxSamples {
		t.Errorf("kept %d samples, want %d", got, maxSamples)
	}
}

// A wide window shows a longer run; a narrow one shows fewer samples rather
// than losing the row.
func TestStatusChartFillsTheWidthItIsGiven(t *testing.T) {
	c := newStatusChart([]string{"1.1.1.1"})
	for i := 0; i < maxSamples; i++ {
		c.record("1.1.1.1", http.StatusOK)
	}

	_, narrow := drawStatus(t, 40, 8, c)
	_, wide := drawStatus(t, 100, 8, c)

	if len(wide[1]) <= len(narrow[1]) {
		t.Errorf("a wide panel drew %d blocks and a narrow one %d", len(wide[1]), len(narrow[1]))
	}
	for _, size := range [][2]int{{30, 8}, {40, 8}, {80, 8}, {200, 8}} {
		_, blocks := drawStatus(t, size[0], size[1], c)
		if len(blocks[1]) == 0 {
			t.Errorf("%d columns drew no blocks at all", size[0])
		}
	}
}

func TestLabelWidth(t *testing.T) {
	if got, want := labelWidth([]string{"1.1.1.1", "255.255.255.255"}), len("255.255.255.255")+1; got != want {
		t.Errorf("labelWidth = %d, want %d", got, want)
	}
	// A long name is capped rather than allowed to crowd out the strip.
	if got := labelWidth([]string{strings.Repeat("a", 40)}); got != labelMaxWidth+1 {
		t.Errorf("labelWidth = %d, want the cap %d", got, labelMaxWidth+1)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"1.1.1.1", 10, "1.1.1.1"},
		{"averylongedgename", 8, "averylo…"},
		{"abc", 0, ""},
		{"abc", 1, "a"},
	}

	for _, tt := range tests {
		if got := truncate(tt.in, tt.width); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.width, got, tt.want)
		}
	}
}
