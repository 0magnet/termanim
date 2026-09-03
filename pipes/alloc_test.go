package pipes

import (
	"testing"

	"github.com/0magnet/termanim/internal/simscreen"
)

// TestFrameDoesNotAllocate guards the screen-drawing path.
//
// Screen.SetContent is a shim over Put that builds a rune slice and a string
// on every call, so drawing a cell at a time cost one allocation per cell: at
// 80x48 that was 3,840 a frame before anything moved. canvas.PutRune and
// canvas.Blank hand Put a cached string instead. This is here so the cheap
// call does not quietly turn back into the expensive one.
func TestFrameDoesNotAllocate(t *testing.T) {
	s := simscreen.NewScreen()
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
	// Not zero here, and the residue is the test harness rather than the
	// animation: v3 has no SimulationScreen, so simscreen drives a real
	// terminfo screen through a mock terminal, and that terminal parses the
	// escape stream and answers queries — fmt.Sprintf in vt.PrivateMode.Reply
	// accounts for most of it. screen.Put itself measures zero over 3,840
	// katakana cells. What this guards is the 3,840 a frame SetContent cost,
	// so the allowance sits far below that and above the harness noise.
	if n := testing.AllocsPerRun(50, func() { a.Frame(s, cols, rows, 1.0/30) }); n > 32 {
		t.Errorf("Frame allocated %v times per call, want at most 32", n)
	}
}
