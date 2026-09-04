package backdrop

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func strip(s string) string { return ansi.ReplaceAllString(s, "") }

const help = "Usage:\n  thing [flags]\n\nFlags:\n  -v, --verbose   say more\n"

// The whole point is that the words survive. A backdrop that eats a line of
// the help has failed at the only thing it must not do.
func TestTextSurvives(t *testing.T) {
	out := strip(Render(help, Options{Force: true, Seed: 1, Width: 70}))
	for _, want := range strings.Split(strings.TrimRight(help, "\n"), "\n") {
		if strings.TrimSpace(want) == "" {
			continue
		}
		if !strings.Contains(out, strings.TrimSpace(want)) {
			t.Errorf("line missing from the render: %q", want)
		}
	}
}

// Every row is the declared width or less. A row that ran long would wrap and
// shear the whole frame.
func TestRowsFitTheWidth(t *testing.T) {
	const w = 70
	for _, line := range strings.Split(Render(help, Options{Force: true, Seed: 3, Width: w}), "\n") {
		if n := len([]rune(strip(line))); n > w {
			t.Errorf("row is %d columns wide, want at most %d", n, w)
		}
	}
}

func TestRowCountIsTextPlusPadding(t *testing.T) {
	out := Render(help, Options{Force: true, Seed: 1, Width: 70, Pad: 3})
	// TrimRight because the render ends in a newline.
	got := len(strings.Split(strings.TrimRight(out, "\n"), "\n"))
	if want := 5 + 2*3; got != want {
		t.Errorf("got %d rows, want %d", got, want)
	}
}

// A pipe gets the text back untouched. `--help | less` and a --help pasted
// into a bug report both want plain text.
func TestNotATerminalIsPassedThrough(t *testing.T) {
	// The test binary's stdout is not a terminal, so this is the real path.
	if got := Render(help, Options{Seed: 1, Width: 70}); got != help {
		t.Errorf("got a rendered frame, want the text unchanged")
	}
}

// Text that already has color keeps it, and its escapes are not counted as
// columns. This is the coloredcobra case.
func TestStyledTextKeepsItsColours(t *testing.T) {
	const styled = "\x1b[1;33mUsage:\x1b[0m\n  thing [flags]\n"
	out := Render(styled, Options{Force: true, Seed: 5, Width: 70})

	if !strings.Contains(out, "\x1b[1;33mUsage:") {
		t.Error("the input's own escape did not survive next to its word")
	}
	// An escape is worth no columns, so coloring a word cannot move
	// anything: the same text with and without its escapes must come out as
	// the same frame. Counted as width, "\x1b[1;33m" would push everything on
	// that row seven columns over and take seven cells of rain with it.
	plain := Render(strip(styled), Options{Force: true, Seed: 5, Width: 70})
	if strip(out) != strip(plain) {
		t.Errorf("colouring a word changed the layout\n got: %q\nwant: %q",
			strip(out), strip(plain))
	}
}

// A tab is worth what the terminal makes it worth. Counting it as one column
// puts every glyph after it in the wrong cell.
func TestTabsAreExpanded(t *testing.T) {
	out := strip(Render("a\tb\n", Options{Force: true, Seed: 1, Width: 40}))
	if !strings.Contains(out, "a       b") {
		t.Errorf("tab was not expanded to the next stop:\n%s", out)
	}
}

// Rain must not land in the gaps between words, which is what makes the help
// unreadable — the eye joins the word to the glyph.
func TestNoRainInsideALine(t *testing.T) {
	const line = "  completion  Generate the script\n"
	out := strip(Render(line, Options{Force: true, Seed: 9, Width: 60}))
	if !strings.Contains(out, "completion  Generate the script") {
		t.Errorf("something was drawn inside the line:\n%s", out)
	}
}

// The seed is what makes it different every time; the same seed is what makes
// a test possible at all.
func TestSeedDecidesTheFrame(t *testing.T) {
	o := Options{Force: true, Seed: 11, Width: 60}
	if Render(help, o) != Render(help, o) {
		t.Error("the same seed gave two different frames")
	}
	o2 := o
	o2.Seed = 12
	if Render(help, o) == Render(help, o2) {
		t.Error("two seeds gave the same frame")
	}
}

// A space inside a colored run must not end the color. A styler colors
// "-h, --help" once, at the dash; resetting at the space left everything
// after it uncolored, which is worse than not coloring at all.
func TestSpaceDoesNotBreakAColouredRun(t *testing.T) {
	const in = "  \x1b[36m-h, --help\x1b[0m   help for it\n"
	out := Render(in, Options{Force: true, Seed: 2, Width: 60})
	if !strings.Contains(out, "\x1b[36m-h, --help") {
		t.Errorf("the coloured run was broken up:\n%q", out)
	}
}

// A line wider than the terminal is written out as it came in. Cutting it at
// the last column would lose a word of the help for the sake of a backdrop,
// and skywire's `cli --help` really does run to 108 columns.
func TestOverWideLineIsNotTruncated(t *testing.T) {
	long := strings.Repeat("x", 100)
	out := Render("short\n"+long+"\n", Options{Force: true, Seed: 1, Width: 60})
	if !strings.Contains(strip(out), long) {
		t.Error("the over-wide line was cut")
	}
	// The rows that do fit still get their backdrop.
	if !strings.Contains(strip(out), "short") {
		t.Error("the line that fits went missing")
	}
}

// The indent gives way before the text does. With room, a line is indented by
// the full pad; with none to spare, the indent shrinks so the line still fits
// rather than the line being cut to make room for it.
func TestIndentShrinksBeforeTheTextDoes(t *testing.T) {
	line := strings.Repeat("x", 58)

	// In columns, not bytes: the rain glyphs before the text are three-byte
	// katakana, so a byte index counts them three times over.
	at := func(width int) int {
		for _, row := range strings.Split(strip(Render(line+"\n", Options{
			Force: true, Seed: 1, Width: width,
		})), "\n") {
			i := strings.Index(row, line)
			if i < 0 {
				continue
			}
			if n := len([]rune(row)); n > width {
				t.Fatalf("at width %d the row came out %d columns", width, n)
			}
			return len([]rune(row[:i]))
		}
		t.Fatalf("line not found at width %d", width)
		return -1
	}

	if got := at(100); got != 2 {
		t.Errorf("with room to spare the indent was %d, want the full pad of 2", got)
	}
	if got := at(60); got != 1 {
		t.Errorf("with two columns to spare the indent was %d, want 1", got)
	}
	if got := at(58); got != 0 {
		t.Errorf("with nothing to spare the indent was %d, want 0", got)
	}
}

// Off is what a docs generator reaches for: the same help, printed into a
// markdown file, must not have rain in it even on a terminal.
func TestOffReturnsTheTextUnchanged(t *testing.T) {
	if got := Render(help, Options{Force: true, Seed: 1, Width: 70, Off: true}); got != help {
		t.Error("Off still rendered a frame")
	}
}

// A line's own leading indentation is part of the line. In a flag list the
// indent is what lines the columns up, and a glyph sitting in it reads as
// content: "  ﾚ --json" looks like a typo rather than like something behind
// the text.
func TestNoRainInALinesIndent(t *testing.T) {
	const line = "      --json        print output as JSON"
	out := strip(Render("Flags:\n"+line+"\n", Options{Force: true, Seed: 6, Width: 80}))
	if !strings.Contains(out, line) {
		t.Errorf("something was drawn in the line's indent:\n%s", out)
	}
}

// GapMin is what lets a full-screen caller use this at all. Its layout pads
// every line out to the full width, so under the default rule the screen would
// be opaque everywhere and no rain would show.
func TestGapMinOpensUpThePadding(t *testing.T) {
	// A line as a layout engine emits it: content padded out inside a border.
	// Trailing padding was always transparent — the opaque run ends at the
	// last word — so the case that matters is padding with something after it.
	line := "|name  value" + strings.Repeat(" ", 30) + "|"

	opaque := strip(Render(line+"\n", Options{Force: true, Seed: 1, Width: 60, Pad: -1}))
	holes := strip(Render(line+"\n", Options{Force: true, Seed: 1, Width: 60, Pad: -1, GapMin: 4}))

	if !strings.Contains(opaque, line) {
		t.Errorf("without GapMin the pane's padding should stay blank, got %q", opaque)
	}
	if strings.Contains(holes, line) {
		t.Errorf("with GapMin the pane's padding should carry rain, got %q", holes)
	}
	// The words, and the gap between them, are untouched either way.
	if !strings.Contains(holes, "name  value") {
		t.Errorf("a two-space gap between words was opened up: %q", holes)
	}
}

// The rain never abuts a word, whichever rule is in force.
func TestGapMinKeepsAClearCellEitherSide(t *testing.T) {
	line := "aa" + strings.Repeat(" ", 20) + "bb"
	out := strip(Render(line+"\n", Options{Force: true, Seed: 3, Width: 40, Pad: -1, GapMin: 4}))
	for _, row := range strings.Split(out, "\n") {
		if !strings.Contains(row, "aa") {
			continue
		}
		if !strings.Contains(row, "aa ") || !strings.Contains(row, " bb") {
			t.Errorf("rain came up against a word: %q", row)
		}
	}
}

// brightest is the highest green channel of any rain color in out, which for
// this palette is how bright the brightest rain glyph on the frame is.
//
// The text's own color is skipped. It is a 255-green too, so counting it would
// make every frame look equally bright whatever the rain was doing.
func brightest(out string) int {
	textStyle := sgr(tcell.NewRGBColor(190, 255, 190), true)
	re := regexp.MustCompile(`\x1b\[0;(?:1;)?38;2;\d+;(\d+);\d+m`)
	max := 0
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		if m[0] == textStyle {
			continue
		}
		var g int
		if _, err := fmt.Sscanf(m[1], "%d", &g); err == nil && g > max {
			max = g
		}
	}
	return max
}

// The rain is not turned down by default. The cell of clear either side of
// every word is what keeps the text readable; dimming a rectangle around the
// whole help as well only made the backdrop muddy.
func TestRainIsNotDimmedByDefault(t *testing.T) {
	out := Render(help, Options{Force: true, Seed: 21, Width: 90})
	if got := brightest(out); got < 150 {
		t.Errorf("brightest rain is %d, want a full-brightness palette (>=150)", got)
	}
}

// Dim still works, and now scales the whole frame rather than a box around the
// text — a caller with a screenful of rain behind a dense layout wants it.
func TestDimScalesTheWholeFrame(t *testing.T) {
	full := brightest(Render(help, Options{Force: true, Seed: 21, Width: 90}))
	dim := brightest(Render(help, Options{Force: true, Seed: 21, Width: 90, Dim: 64}))
	if dim >= full {
		t.Errorf("Dim 64 gave brightest=%d, no darker than the default %d", dim, full)
	}
}

// There is no rectangle any more: what is behind the text is lit the same as
// what is beyond the end of it. With the old panel, everything left of the
// widest line was scaled down and everything right of it was not.
func TestBrightnessDoesNotChangeAcrossTheOldPanelEdge(t *testing.T) {
	// A single short line in a wide terminal: the old panel ended just past
	// it, so the two halves of the row were lit differently.
	const line = "Usage:\n"
	out := Render(line, Options{Force: true, Seed: 7, Width: 120})

	var near, far int
	for _, row := range strings.Split(out, "\n") {
		cols := 0
		for _, piece := range strings.SplitAfter(row, "m") {
			g := brightest(piece)
			if strings.Contains(piece, "\x1b[") && g > 0 {
				if cols < 20 && g > near {
					near = g
				}
				if cols > 60 && g > far {
					far = g
				}
			}
			cols += len([]rune(strip(piece)))
		}
	}
	if near == 0 || far == 0 {
		t.Fatalf("no rain found either side of the old panel edge (near=%d far=%d)", near, far)
	}
	if far > near*2 {
		t.Errorf("rain beyond the text (%d) is much brighter than behind it (%d): "+
			"the panel is still being dimmed", far, near)
	}
}
