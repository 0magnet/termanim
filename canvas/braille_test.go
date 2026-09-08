package canvas

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/internal/simscreen"
)

// The dot numbering is the one thing here that cannot be derived, only looked
// up, so it is spelled out again in the test rather than read from the table it
// is meant to check. Dots 1 to 6 run down the left column then down the right,
// which is six-dot Braille; dots 7 and 8, the bottom row, took the two bits
// left over when the cell grew a fourth row, so they are the high bits and out
// of sequence.
var wantDotBits = []struct {
	x, y int
	bit  byte
}{
	{0, 0, 0x01}, // dot 1
	{0, 1, 0x02}, // dot 2
	{0, 2, 0x04}, // dot 3
	{1, 0, 0x08}, // dot 4
	{1, 1, 0x10}, // dot 5
	{1, 2, 0x20}, // dot 6
	{0, 3, 0x40}, // dot 7
	{1, 3, 0x80}, // dot 8
}

func TestBrailleDotNumberingIsNotRasterOrder(t *testing.T) {
	for _, d := range wantDotBits {
		if got := dotBit[(d.y%4)*2+d.x%2]; got != d.bit {
			t.Errorf("subpixel (%d,%d) is bit %#02x, want %#02x", d.x, d.y, got, d.bit)
		}
	}
	// The whole point: raster order would make the bottom-left dot 0x40 and
	// nothing above it would move, so a test that only checked the first six
	// would pass on a wrong table.
	if dotBit[3*2+0] == 0x10 || dotBit[3*2+1] == 0x20 {
		t.Error("the bottom row is in raster order, which is the one thing Braille is not")
	}
}

// Every one of the 256 patterns must survive subpixels -> dot mask -> glyph and
// read back the same. A scrambled dot order still draws something that looks
// like a picture at this size, so it has to be checked exhaustively rather than
// looked at.
func TestBrailleFlushDrawsEveryPatternCorrectly(t *testing.T) {
	screen := simscreen.NewScreen()
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(1, 1)

	s := NewBrailleSurface(2, 4)
	for mask := 0; mask < 256; mask++ {
		s.Clear()
		for _, d := range wantDotBits {
			if byte(mask)&d.bit != 0 {
				s.Set(d.x, d.y, true)
			}
		}
		// The dots read back where they were put.
		for _, d := range wantDotBits {
			want := byte(mask)&d.bit != 0
			if got := s.At(d.x, d.y); got != want {
				t.Fatalf("mask %#02x: At(%d,%d) = %v, want %v", mask, d.x, d.y, got, want)
			}
		}

		s.flush(screen)
		screen.Show()
		cells, _, _ := simscreen.Contents(screen)
		glyph := cells[0].Runes

		if mask == 0 {
			// An empty cell is a space, not U+2800: the empty Braille pattern is
			// a printed character a font may size differently.
			if len(glyph) != 1 || glyph[0] != ' ' {
				t.Fatalf("empty cell drew %q, want a space", glyph)
			}
			continue
		}
		want := BrailleRune(byte(mask))
		if len(glyph) != 1 || glyph[0] != want {
			t.Fatalf("mask %#02x drew %q (%U), want %q (%U)", mask, glyph, glyph, want, want)
		}
	}
}

// Two glyphs whose shape is unmistakable, as an anchor independent of the bit
// table: if the dots were transposed or the rows reversed these would come out
// as some other real Braille character and the exhaustive test above would
// still pass, because it checks the mapping against itself.
func TestBrailleDrawsRecognizableLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		draw func(s *BrailleSurface)
		want rune
	}{
		// The top row of a cell, both dots: dots 1 and 4.
		{"a horizontal line along the top", func(s *BrailleSurface) {
			s.Set(0, 0, true)
			s.Set(1, 0, true)
		}, '⠉'},
		// The left column, all four dots: dots 1, 2, 3 and 7.
		{"a vertical line down the left", func(s *BrailleSurface) {
			for y := 0; y < 4; y++ {
				s.Set(0, y, true)
			}
		}, '⡇'},
		// The bottom row, both dots: dots 7 and 8, the two high bits.
		{"a horizontal line along the bottom", func(s *BrailleSurface) {
			s.Set(0, 3, true)
			s.Set(1, 3, true)
		}, '⣀'},
		// Every dot.
		{"a full cell", func(s *BrailleSurface) { s.Fill(true) }, '⣿'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewBrailleSurface(2, 4)
			tc.draw(s)
			got := s.Glyph(0, 0)
			if got != tc.want {
				t.Errorf("drew %q (%U), want %q (%U)", got, got, tc.want, tc.want)
			}
		})
	}
}

// The whole reason this surface exists: one cell carries eight subpixels, four
// times the vertical resolution of the half-block surface. If that stops being
// true the animations lose it silently.
func TestBrailleCellHoldsEightSubpixels(t *testing.T) {
	s := NewBrailleSurface(8, 8)
	if w, h := s.Size(); w != 8 || h != 8 {
		t.Fatalf("Size() = %d,%d, want 8,8", w, h)
	}
	if cols, rows := s.Cells(); cols != 4 || rows != 2 {
		t.Fatalf("Cells() = %d,%d, want 4,2 — 8x8 subpixels is 4x2 cells", cols, rows)
	}
	// Eight distinct subpixels land in the same cell and in no other.
	for _, d := range wantDotBits {
		s.Set(d.x, d.y, true)
	}
	if s.dots[0] != 0xff {
		t.Errorf("the first cell holds %#02x, want every dot lit", s.dots[0])
	}
	for i := 1; i < len(s.dots); i++ {
		if s.dots[i] != 0 {
			t.Errorf("cell %d holds %#02x, want nothing — the dots leaked out of their cell", i, s.dots[i])
		}
	}
}

func TestBrailleSurfaceRoundTrip(t *testing.T) {
	s := NewBrailleSurface(8, 8)
	s.Set(5, 6, true)
	if !s.At(5, 6) {
		t.Error("At(5,6) is off after setting it")
	}
	if s.At(5, 7) || s.At(4, 6) {
		t.Error("setting one subpixel lit a neighbor")
	}
	s.Set(5, 6, false)
	if s.At(5, 6) {
		t.Error("At(5,6) is still on after clearing it")
	}

	s.Fill(true)
	s.Clear()
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if s.At(x, y) {
				t.Fatalf("Clear left (%d,%d) lit", x, y)
			}
		}
	}
}

func TestBrailleIgnoresOutOfBounds(t *testing.T) {
	s := NewBrailleSurface(4, 4)
	// Must not panic: animations are written without clamping at call sites.
	s.Set(-1, 0, true)
	s.Set(0, -1, true)
	s.Set(4, 0, true)
	s.Set(0, 4, true)
	s.SetColor(-1, -1, tcell.NewRGBColor(255, 0, 0))
	s.SetColor(99, 99, tcell.NewRGBColor(255, 0, 0))
	s.Plot(-3, -3, tcell.NewRGBColor(255, 0, 0))
	if s.At(99, 99) {
		t.Error("At out of bounds reports a lit subpixel")
	}
	if got := s.ColorAt(99, 99); got != tcell.ColorDefault {
		t.Errorf("ColorAt out of bounds = %v, want ColorDefault", got)
	}
	if got := s.Glyph(-1, 99); got != BrailleRune(0) {
		t.Errorf("Glyph out of bounds = %q, want the empty pattern", got)
	}
	for i, m := range s.dots {
		if m != 0 {
			t.Errorf("cell %d holds %#02x after only out-of-bounds writes", i, m)
		}
	}
}

// A subpixel outside the last whole cell still has somewhere to go: the
// dimensions round up to a partial cell rather than dropping coordinates that
// are inside the surface the caller asked for.
func TestBrailleRoundsUpToWholeCells(t *testing.T) {
	s := NewBrailleSurface(3, 5) // 1.5 cells by 1.25 cells
	if cols, rows := s.Cells(); cols != 2 || rows != 2 {
		t.Fatalf("Cells() = %d,%d, want 2,2", cols, rows)
	}
	s.Set(2, 4, true) // in the last cell of both axes
	if !s.At(2, 4) {
		t.Error("a subpixel in the partial cell did not stick")
	}
}

// Color is per cell, not per subpixel — that is the price of the resolution. A
// cell keeps the last color written into it; a caller that wants a particular
// one writes it last.
func TestBrailleColorIsPerCellAndLastWriteWins(t *testing.T) {
	red := tcell.NewRGBColor(255, 0, 0)
	blue := tcell.NewRGBColor(0, 0, 255)

	s := NewBrailleSurface(4, 4)
	s.Plot(0, 0, red)
	s.Plot(1, 3, blue) // same cell, seven dots away
	s.Plot(2, 0, red)  // the next cell along

	if got := s.ColorAt(0, 0); got != blue {
		t.Errorf("the first cell is %v, want the last color written to it, %v", got, blue)
	}
	if got := s.ColorAt(1, 3); got != blue {
		t.Errorf("both subpixels of a cell must report one color: got %v, want %v", got, blue)
	}
	if got := s.ColorAt(2, 0); got != red {
		t.Errorf("the neighboring cell is %v, want %v — color leaked between cells", got, red)
	}
}

func TestBrailleFlushPaintsTheCellColor(t *testing.T) {
	screen := simscreen.NewScreen()
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(2, 1)

	green := tcell.NewRGBColor(0, 255, 0)
	s := NewBrailleSurface(4, 4)
	s.Plot(0, 0, green)
	s.Set(1, 1, true) // same cell, colored by the write above
	s.Set(2, 0, true) // the next cell, never colored
	s.flush(screen)
	screen.Show()

	cells, _, _ := simscreen.Contents(screen)
	if got := cells[0].Style.GetForeground(); got != green {
		t.Errorf("colored cell foreground = %v, want %v", got, green)
	}
	if got := cells[0].Runes; len(got) != 1 || got[0] != BrailleRune(0x01|0x10) {
		t.Errorf("colored cell drew %q, want dots 1 and 5", got)
	}
	if got := cells[1].Style.GetForeground(); got != tcell.ColorDefault {
		t.Errorf("uncolored cell foreground = %v, want the terminal's own, %v", got, tcell.ColorDefault)
	}
}

// A monochrome effect sets Color once and never colors a cell. Unlike the
// half-block surface, a default foreground is the right answer here: an unlit
// dot is the absence of ink rather than a painted half, so nothing gets filled
// in with the terminal's text color.
func TestBrailleSurfaceColorIsTheFallback(t *testing.T) {
	screen := simscreen.NewScreen()
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(2, 1)

	amber := tcell.NewRGBColor(255, 176, 0)
	cyan := tcell.NewRGBColor(0, 255, 255)
	s := NewBrailleSurface(4, 4)
	s.Color = amber
	s.Set(0, 0, true)
	s.Set(2, 0, true)
	s.SetColor(2, 0, cyan) // one cell overrides it
	s.flush(screen)
	screen.Show()

	cells, _, _ := simscreen.Contents(screen)
	if got := cells[0].Style.GetForeground(); got != amber {
		t.Errorf("an uncolored cell is %v, want the surface color %v", got, amber)
	}
	if got := cells[1].Style.GetForeground(); got != cyan {
		t.Errorf("a colored cell is %v, want %v — Color must not override SetColor", got, cyan)
	}

	// And Clear returns the cell to the fallback rather than keeping cyan.
	s.Clear()
	if got := s.ColorAt(2, 0); got != tcell.ColorDefault {
		t.Errorf("Clear left the cell at %v, want ColorDefault so Color applies again", got)
	}
}

func TestBrailleFlushLeavesEmptyCellsAlone(t *testing.T) {
	screen := simscreen.NewScreen()
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(4, 2)

	s := NewBrailleSurface(8, 8)
	s.Color = tcell.NewRGBColor(255, 0, 0)
	s.flush(screen)
	screen.Show()

	cells, _, _ := simscreen.Contents(screen)
	for i, c := range cells {
		if len(c.Runes) > 0 && c.Runes[0] != ' ' {
			t.Fatalf("empty cell %d drew %q, want a space so the terminal background shows", i, c.Runes[0])
		}
	}
}

// countingBraille records the size it was given and how many frames it drew.
type countingBraille struct {
	w, h   int
	frames int
}

func (c *countingBraille) Resize(w, h int)                     { c.w, c.h = w, h }
func (c *countingBraille) Frame(s *BrailleSurface, dt float64) { c.frames++ }

// RunBraille must hand the animation subpixels — twice the columns by four
// times the rows — and hide the cursor, exactly as Run does. It shares Run's
// loop, so this is checking the wiring around it and not the loop again.
func TestRunBrailleSizesTheSurfaceInSubpixels(t *testing.T) {
	sim := simscreen.NewScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()

	c := &cursorScreen{Screen: sim}
	c.ShowCursor(0, 0)
	a := &countingBraille{}

	// Both queued before the loop starts, so they are waiting when RunBraille
	// begins and it ends on the second: no clock, no second goroutine, nothing
	// to race with.
	//
	// The size is injected as an event rather than set with SetSize because
	// SetSize goes out through the mock terminal and comes back as an event
	// whenever it gets there, which is not necessarily before the q — the test
	// then measured the mock's default size instead of the one it asked for.
	sim.EventQ() <- tcell.NewEventResize(40, 12)
	sim.EventQ() <- tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone)

	if err := RunBraille(c, a, Options{}); err != nil {
		t.Fatal(err)
	}
	if a.w != 80 || a.h != 48 {
		t.Errorf("Resize got %dx%d subpixels, want 80x48 for a 40x12 terminal", a.w, a.h)
	}
	if !c.hidden {
		t.Error("the loop left the cursor visible on top of the animation")
	}
}

// A frame must not allocate, for the reasons on BenchmarkFlush: in a browser
// everything shares one thread, so a collection is a dropped frame.
//
// Read the steady-state number, not the first one — tcell segments a grapheme
// cluster the first time a cell is given a string it does not already hold and
// caches the result.
//
// Run with: go test -run xxx -bench BrailleFlush -benchtime 1000x ./canvas/
func BenchmarkBrailleFlush(b *testing.B) {
	screen := simscreen.NewScreen()
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(210, 49)

	s := NewBrailleSurface(420, 196)
	for i := range s.dots {
		s.dots[i] = byte(i%255) + 1
		s.fg[i] = tcell.NewRGBColor(int32(i%256), 40, 90)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.flush(screen)
	}
}
