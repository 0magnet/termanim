package backdrop

import (
	"strings"
	"testing"
)

// The point of a painter over Render: the rain moves between frames.
func TestFrameAdvancesTheRain(t *testing.T) {
	p := New(Options{Force: true, Seed: 1, Width: 60})
	first := p.Frame(help, 0)
	second := p.Frame(help, 1.0/30)
	if first == second {
		t.Error("a tick did not move the rain")
	}
}

// A redraw that is not a tick — a resize, a keypress — asks for the frame as
// it stands. Advancing on every redraw would make the rain run at the speed
// the user types.
func TestZeroDtRedrawsWithoutMoving(t *testing.T) {
	p := New(Options{Force: true, Seed: 1, Width: 60})
	first := p.Frame(help, 0)
	if again := p.Frame(help, 0); again != first {
		t.Error("a zero-dt redraw moved the rain")
	}
}

// The text has to survive an animated frame exactly as it survives a still.
func TestFrameKeepsTheText(t *testing.T) {
	p := New(Options{Force: true, Seed: 2, Width: 70})
	out := strip(p.Frame(help, 0.1))
	for _, want := range strings.Split(strings.TrimRight(help, "\n"), "\n") {
		if strings.TrimSpace(want) == "" {
			continue
		}
		if !strings.Contains(out, strings.TrimSpace(want)) {
			t.Errorf("line missing from the frame: %q", want)
		}
	}
}

// A caller that has composed a whole screen wants the text exactly where it
// put it. Zero cannot ask for that, being the zero value, so negative does.
func TestNegativePadMeansNone(t *testing.T) {
	const screen = "top\nmiddle\nbottom"

	p := New(Options{Force: true, Seed: 3, Width: 40, Pad: -1})
	rows := strings.Split(strings.TrimRight(p.Frame(screen, 0), "\n"), "\n")
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want the 3 that were handed over", len(rows))
	}
	if !strings.HasPrefix(strip(rows[0]), "top") {
		t.Errorf("first row was indented or padded: %q", strip(rows[0]))
	}
}

// A resized terminal gets a simulation the new size. The per-column state has
// no meaningful way to be stretched, so it is rebuilt.
func TestResizeRebuildsTheSimulation(t *testing.T) {
	p := New(Options{Force: true, Seed: 4, Width: 60})
	p.Frame(help, 0)
	if c, _ := p.Matrix().Size(); c != 60 {
		t.Fatalf("simulation is %d columns, want 60", c)
	}

	p.o.Width = 100
	out := p.Frame(help, 0)
	if c, _ := p.Matrix().Size(); c != 100 {
		t.Errorf("after a resize the simulation is %d columns, want 100", c)
	}
	for _, row := range strings.Split(out, "\n") {
		if n := len([]rune(strip(row))); n > 100 {
			t.Errorf("row is %d columns after the resize, want at most 100", n)
		}
	}
}

// Off means off here too: a painter is handed to a program that may be told to
// keep quiet.
func TestPainterOffReturnsTextUnchanged(t *testing.T) {
	p := New(Options{Force: true, Seed: 1, Width: 60, Off: true})
	if got := p.Frame(help, 0.5); got != help {
		t.Error("Off still painted a frame")
	}
}

// A caller that is told its width must be able to say so. Left to the default
// the painter asks the terminal, and when output is not the terminal that is
// 80 — at which point every row of a screen composed at the real width is
// wider than the painter thinks the screen is, and comes back untouched.
func TestSetWidthIsUsed(t *testing.T) {
	row := "left" + strings.Repeat(" ", 90) + "right"
	p := New(Options{Force: true, Seed: 1, Pad: -1, GapMin: 4})

	if got := p.Frame(row+"\n", 0); strings.TrimRight(got, "\n") != row {
		t.Error("without a width the 99-column row should have been passed through")
	}
	p.SetWidth(99)
	if got := p.Frame(row+"\n", 0); strings.TrimRight(got, "\n") == row {
		t.Error("after SetWidth the row should have been painted")
	}
	if c, _ := p.Matrix().Size(); c != 99 {
		t.Errorf("simulation is %d columns, want 99", c)
	}
}
