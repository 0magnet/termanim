package aquarium

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestFrameDoesNotAllocate guards the screen-drawing path.
//
// Screen.SetContent is a shim over Put that builds a rune slice and a string
// on every call, so drawing a cell at a time cost one allocation per cell: at
// 80x48 that was 3,840 a frame before anything moved. canvas.PutRune and
// canvas.Blank hand Put a cached string instead. This is here so the cheap
// call does not quietly turn back into the expensive one.
func TestFrameDoesNotAllocate(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	const cols, rows = 80, 48
	s.SetSize(cols, rows)

	a := New(1)
	a.Resize(cols, rows)
	for i := 0; i < 5; i++ {
		a.Frame(s, cols, rows, 1.0/30)
	}
	if n := testing.AllocsPerRun(50, func() { a.Frame(s, cols, rows, 1.0/30) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}
