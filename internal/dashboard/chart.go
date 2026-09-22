package dashboard

import (
	"sort"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	// maxSamples is how much history an edge keeps. The strip draws as much
	// of it as the terminal has room for, so a wide window shows a longer run
	// rather than the same handful with space to spare.
	maxSamples = 120

	// codeWidth fits a status code and the space before it.
	codeWidth = 5
)

// statusChart draws one row per edge: a block for each of its recent requests,
// coloured by status class, and the code it answered with last.
//
// It is drawn by hand because tview ships no chart widget. A strip of colour
// is what makes one edge answering differently from the rest visible without
// reading anything, which is the question this panel exists to answer.
type statusChart struct {
	*tview.Box

	edges   []string
	samples map[string][]int
	// total counts every answer an edge has given, which is what decides how
	// far along the current pass is. The samples slice is capped, so its
	// length cannot be used for that.
	total map[string]int
}

func newStatusChart(edges []string) *statusChart {
	c := &statusChart{
		Box:     tview.NewBox(),
		edges:   edges,
		samples: make(map[string][]int, len(edges)),
		total:   make(map[string]int, len(edges)),
	}
	c.SetBorder(true).SetTitle(" Status per edge ")
	c.SetDrawFunc(c.draw)

	return c
}

// record folds one answer in, dropping the oldest once the history is full.
func (c *statusChart) record(edge string, statusCode int) {
	codes := append(c.samples[edge], statusCode)
	if len(codes) > maxSamples {
		codes = codes[len(codes)-maxSamples:]
	}

	c.samples[edge] = codes
	c.total[edge]++
}

// pass returns the answers drawn in the strip's current left-to-right pass,
// oldest first.
func (c *statusChart) pass(edge string, strip int) []int {
	codes := c.samples[edge]
	if strip <= 0 || len(codes) == 0 {
		return nil
	}

	drawn := (c.total[edge]-1)%strip + 1
	if drawn > len(codes) {
		drawn = len(codes)
	}

	return codes[len(codes)-drawn:]
}

// latest is the code an edge answered with last, or zero before it has.
func (c *statusChart) latest(edge string) int {
	codes := c.samples[edge]
	if len(codes) == 0 {
		return 0
	}

	return codes[len(codes)-1]
}

func (c *statusChart) draw(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	ix, iy, iw, ih := x+1, y+1, width-2, height-2
	if iw <= 0 || ih <= 0 {
		return ix, iy, iw, ih
	}

	label := labelWidth(c.edges)
	strip := iw - label - codeWidth
	if strip > maxSamples {
		strip = maxSamples
	}
	if strip < 0 {
		strip = 0
	}

	for row, edge := range c.edges {
		if row >= ih {
			return ix, iy, iw, ih
		}
		c.drawRow(screen, ix, iy+row, edge, label, strip)
	}

	// The legend only earns its line once every edge already has one.
	if legend := iy + len(c.edges) + 1; legend < iy+ih {
		c.drawLegend(screen, ix, legend, iw)
	}

	return ix, iy, iw, ih
}

func (c *statusChart) drawRow(screen tcell.Screen, x, y int, edge string, label, strip int) {
	tview.Print(screen, tview.Escape(truncate(edge, label)), x, y, label, tview.AlignLeft, tcell.ColorWhite)

	// The strip fills from the left and starts over once it reaches the right,
	// so a run reads as a sequence of passes. A window sliding under the eye
	// moves every block on every request, which makes a colour change hard to
	// catch; here only the newest block moves.
	for i, code := range c.pass(edge, strip) {
		screen.SetContent(x+label+i, y, barRune, nil, tcell.StyleDefault.Foreground(statusColor(code)))
	}

	switch latest := c.latest(edge); latest {
	case failedStatus:
		// Nothing to show when the edge has not answered yet either; the
		// strip is empty then too.
		if len(c.samples[edge]) > 0 {
			tview.Print(screen, "----", x+label+strip, y, codeWidth, tview.AlignRight, statusColor(failedStatus))
		}
	default:
		tview.Print(screen, strconv.Itoa(latest), x+label+strip, y, codeWidth, tview.AlignRight, statusColor(latest))
	}
}

// drawLegend names only the classes the run has turned up, so the legend
// growing is itself the signal that something started answering differently.
func (c *statusChart) drawLegend(screen tcell.Screen, x, y, width int) {
	at := x
	for _, class := range c.classesSeen() {
		label := strconv.Itoa(class) + "xx"
		if class == failedStatus {
			label = "failed"
		}

		at = legendEntry(screen, at, y, x+width, statusColor(class*100), label)
		if at < 0 {
			return
		}
	}
}

// classesSeen lists the status classes the run has turned up, in order.
func (c *statusChart) classesSeen() []int {
	var classes []int
	for _, edge := range c.edges {
		for _, code := range c.samples[edge] {
			if class := code / 100; !contains(classes, class) {
				classes = append(classes, class)
			}
		}
	}
	sort.Ints(classes)

	return classes
}
