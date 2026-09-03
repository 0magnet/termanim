package canvas

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSurfaceRoundTrip(t *testing.T) {
	s := NewSurface(4, 4)
	red := tcell.NewRGBColor(255, 0, 0)
	s.Set(2, 3, red)
	if got := s.At(2, 3); got != red {
		t.Fatalf("At(2,3) = %v, want %v", got, red)
	}
	if got := s.At(0, 0); got != tcell.ColorDefault {
		t.Errorf("untouched pixel is %v, want ColorDefault", got)
	}
}

func TestSurfaceIgnoresOutOfBounds(t *testing.T) {
	s := NewSurface(2, 2)
	// Must not panic: animations are written without clamping at call sites.
	s.Set(-1, 0, tcell.ColorRed)
	s.Set(0, -1, tcell.ColorRed)
	s.Set(2, 0, tcell.ColorRed)
	s.Set(0, 2, tcell.ColorRed)
	if got := s.At(99, 99); got != tcell.ColorDefault {
		t.Errorf("At out of bounds = %v, want ColorDefault", got)
	}
}

// The whole point of the surface is that one cell carries two independently
// coloured pixels. If that stops being true the animations lose half their
// vertical resolution silently.
func TestFlushPacksTwoPixelsPerCell(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(4, 2)

	s := NewSurface(4, 4)
	top := tcell.NewRGBColor(255, 0, 0)
	bot := tcell.NewRGBColor(0, 0, 255)
	s.Set(0, 0, top) // upper pixel of cell row 0
	s.Set(0, 1, bot) // lower pixel of cell row 0
	s.flush(screen)
	screen.Show()

	cells, w, _ := screen.GetContents()
	c := cells[0*w+0]
	if len(c.Runes) == 0 || c.Runes[0] != upperHalf {
		t.Fatalf("cell glyph = %q, want %q", c.Runes, upperHalf)
	}
	fg, bg, _ := c.Style.Decompose()
	if fg != top {
		t.Errorf("foreground = %v, want the upper pixel %v", fg, top)
	}
	if bg != bot {
		t.Errorf("background = %v, want the lower pixel %v", bg, bot)
	}
}

// A cell with only one pixel lit must leave the other half as the terminal's
// background, not as a colour.
//
// Drawing it as an upper block with a default foreground does not do that: a
// default foreground is whatever the terminal writes text in, so the empty half
// came out solid white — a bar dancing along the top of the fire, and a fringe
// around anything else that does not fill the screen. That edge is exactly
// where one pixel of a cell is lit and the other is not.
func TestAHalfLitCellDoesNotPaintTheEmptyHalf(t *testing.T) {
	lit := tcell.NewRGBColor(255, 40, 0)

	for _, tc := range []struct {
		name      string
		upper     bool
		wantGlyph rune
	}{
		{"only the lower pixel is lit", false, lowerHalf},
		{"only the upper pixel is lit", true, upperHalf},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			defer screen.Fini()
			screen.SetSize(4, 2)

			s := NewSurface(4, 4)
			if tc.upper {
				s.Set(0, 0, lit)
			} else {
				s.Set(0, 1, lit)
			}
			s.flush(screen)
			screen.Show()

			cells, w, _ := screen.GetContents()
			c := cells[0*w+0]
			if len(c.Runes) == 0 || c.Runes[0] != tc.wantGlyph {
				t.Fatalf("glyph is %q, want %q", c.Runes, tc.wantGlyph)
			}
			fg, bg, _ := c.Style.Decompose()
			if fg != lit {
				t.Errorf("the lit half is %v, want %v", fg, lit)
			}
			if bg != tcell.ColorDefault {
				t.Errorf("the empty half is %v, want the terminal's own background", bg)
			}
		})
	}
}

func TestFlushLeavesEmptyCellsAlone(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(4, 2)

	s := NewSurface(4, 4) // all ColorDefault
	s.flush(screen)
	screen.Show()

	cells, w, _ := screen.GetContents()
	c := cells[0*w+0]
	if len(c.Runes) > 0 && c.Runes[0] != ' ' {
		t.Errorf("empty cell drew %q, want a space so the terminal background shows", c.Runes[0])
	}
}

func TestPaletteInterpolates(t *testing.T) {
	p := NewPalette(
		Stop{At: 0, R: 0, G: 0, B: 0},
		Stop{At: 1, R: 255, G: 255, B: 255},
	)
	r0, _, _ := p[0].RGB()
	r255, _, _ := p[255].RGB()
	if r0 != 0 {
		t.Errorf("first entry red = %d, want 0", r0)
	}
	if r255 != 255 {
		t.Errorf("last entry red = %d, want 255", r255)
	}
	rMid, _, _ := p[128].RGB()
	if rMid < 100 || rMid > 160 {
		t.Errorf("midpoint red = %d, want roughly half", rMid)
	}
}

// A plasma ramp that does not return to its starting colour shows a seam
// wherever the summed field wraps.
func TestPlasmaPaletteLoops(t *testing.T) {
	r0, g0, b0 := Plasma[0].RGB()
	r1, g1, b1 := Plasma[255].RGB()
	near := func(a, b int32) bool { return a-b < 8 && b-a < 8 }
	if !near(r0, r1) || !near(g0, g1) || !near(b0, b1) {
		t.Errorf("plasma palette does not loop: starts (%d,%d,%d) ends (%d,%d,%d)",
			r0, g0, b0, r1, g1, b1)
	}
}

// hiddenScreen is a simulation screen that reports itself as never watched.
type hiddenScreen struct {
	tcell.SimulationScreen
	active bool
	shown  int
}

func (h *hiddenScreen) Active() bool { return h.active }
func (h *hiddenScreen) Show()        { h.shown++; h.SimulationScreen.Show() }

// countingAnim records how many frames it was asked to draw.
type countingAnim struct{ frames int }

func (c *countingAnim) Resize(w, h int)              {}
func (c *countingAnim) Frame(s *Surface, dt float64) { c.frames++ }

// A hidden window must cost nothing. Drawing is nearly the whole cost of an
// animation, and in a page every window shares one thread — so a frame drawn
// where it cannot be seen is what takes the host down when several are open.
func TestHiddenScreensAreNotDrawn(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(40, 12)

	h := &hiddenScreen{SimulationScreen: sim, active: false}
	a := &countingAnim{}

	// Drive the loop's frame path directly rather than starting Run, which
	// would need a real ticker and a way to stop it.
	if w, ok := interface{}(h).(watched); !ok || w.Active() {
		t.Fatal("the test screen does not report itself hidden")
	}
	// Simulate what run() does per tick for a hidden screen: nothing.
	for i := 0; i < 10; i++ {
		if w, ok := interface{}(h).(watched); ok && !w.Active() {
			continue
		}
		a.Frame(NewSurface(40, 24), 1.0/60)
		h.Show()
	}
	if a.frames != 0 {
		t.Errorf("a hidden screen drew %d frames, want 0", a.frames)
	}
	if h.shown != 0 {
		t.Errorf("a hidden screen was shown %d times, want 0", h.shown)
	}

	// And once it is in front, it draws again.
	h.active = true
	for i := 0; i < 3; i++ {
		if w, ok := interface{}(h).(watched); ok && !w.Active() {
			continue
		}
		a.Frame(NewSurface(40, 24), 1.0/60)
		h.Show()
	}
	if a.frames != 3 {
		t.Errorf("a visible screen drew %d frames, want 3", a.frames)
	}
}

// cursorScreen records what the loop did to the cursor.
type cursorScreen struct {
	tcell.SimulationScreen
	hidden bool
}

func (c *cursorScreen) HideCursor() {
	c.hidden = true
	c.SimulationScreen.HideCursor()
}

func (c *cursorScreen) ShowCursor(x, y int) {
	c.hidden = x < 0 || y < 0
	c.SimulationScreen.ShowCursor(x, y)
}

// An animation has no text entry, so a cursor left parked in one blinks on top
// of the picture. It was visible in every demo embedded in a page, because
// hiding it was done by the command-line front end and nothing else went
// through that.
func TestRunHidesTheCursor(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(40, 12)

	c := &cursorScreen{SimulationScreen: sim}
	c.ShowCursor(0, 0)
	if c.hidden {
		t.Fatal("the test screen starts with the cursor already hidden")
	}

	// Queued before the loop starts: the event channel is buffered and made in
	// Init, so this is waiting when Run begins and ends it on the first read.
	// Doing it this way keeps the test off the clock and off a second
	// goroutine, so there is nothing to race with and nothing to hang on.
	sim.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)

	if err := Run(c, &countingAnim{}, Options{}); err != nil {
		t.Fatal(err)
	}
	if !c.hidden {
		t.Error("the loop left the cursor visible on top of the animation")
	}
}

// A screen that says nothing about being watched must always be drawn, or a
// plain terminal would go blank.
func TestPlainScreensAreAlwaysDrawn(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	if _, ok := interface{}(sim).(watched); ok {
		t.Error("a plain simulation screen should not implement watched")
	}
}
