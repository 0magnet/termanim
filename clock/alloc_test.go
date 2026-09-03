package clock

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestFrameBarelyAllocates guards the screen-drawing path, as in the other
// animations: SetContent cost one allocation per cell, 3,840 a frame at 80x48,
// and canvas.PutRune removes them. One remains, from time.Now loading zone
// data, so this allows a single allocation rather than none.
func TestFrameBarelyAllocates(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	const cols, rows = 80, 48
	s.SetSize(cols, rows)

	c := New()
	c.Resize(cols, rows)
	for i := 0; i < 5; i++ {
		c.Frame(s, cols, rows, 1.0/30)
	}
	if n := testing.AllocsPerRun(50, func() { c.Frame(s, cols, rows, 1.0/30) }); n > 1 {
		t.Errorf("Frame allocated %v times per call, want at most 1", n)
	}
}
