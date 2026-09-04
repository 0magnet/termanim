package backdrop

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
	"github.com/0magnet/termanim/matrix"
)

// A cell nobody drew into is not a black cell. The rain is mostly gaps, and a
// gap has to leave the terminal's own background showing or the whole backdrop
// becomes a rectangle.
func TestUnsetCellsAreUnlit(t *testing.T) {
	f := NewFrame(4, 3)
	if _, ok := f.At(2, 1); ok {
		t.Error("a fresh frame reports a cell as lit")
	}
	f.Set(2, 1, Cell{Rune: 'x', Fg: tcell.NewRGBColor(255, 0, 0)})
	if c, ok := f.At(2, 1); !ok || c.Rune != 'x' {
		t.Errorf("At after Set gave %q, %v", c.Rune, ok)
	}
	f.Clear()
	if _, ok := f.At(2, 1); ok {
		t.Error("Clear left a cell lit")
	}
}

// Out of bounds is dropped rather than panicking, so a source can be written
// without clamping at every call site.
func TestFrameBoundsAreDropped(t *testing.T) {
	f := NewFrame(2, 2)
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {2, 0}, {0, 2}} {
		f.Set(p[0], p[1], Cell{Rune: 'x'})
		if _, ok := f.At(p[0], p[1]); ok {
			t.Errorf("cell at %v is lit", p)
		}
	}
}

// Resize keeps the buffer when it can, which is the whole reason a Painter
// holds a Frame rather than making one per redraw.
func TestResizeReusesTheBuffer(t *testing.T) {
	f := NewFrame(80, 25)
	first := &f.cells[0]
	f.Resize(40, 20)
	if &f.cells[0] != first {
		t.Error("shrinking reallocated")
	}
	if c, r := f.Size(); c != 40 || r != 20 {
		t.Errorf("Size is %dx%d, want 40x20", c, r)
	}
	// And it clears: a shrink that reused the buffer would otherwise show the
	// old frame's cells through the new one.
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			if _, ok := f.At(x, y); ok {
				t.Fatalf("cell %d,%d survived the resize", x, y)
			}
		}
	}
}

// The surface adapter has to make the same choice canvas.flush makes about
// which half block to use, and for the same reason: a cell with one pixel lit
// must leave the other half to the terminal. Drawing it as an upper block with
// a default foreground paints that half in whatever color the terminal writes
// text in, which is a solid bar.
func TestSurfaceHalfBlocks(t *testing.T) {
	red, blue := tcell.NewRGBColor(255, 0, 0), tcell.NewRGBColor(0, 0, 255)
	s := canvas.NewSurface(4, 8)
	s.Set(0, 0, red)  // upper pixel only
	s.Set(1, 3, blue) // lower pixel only
	s.Set(2, 4, red)  // both
	s.Set(2, 5, blue)

	f := NewFrame(4, 4)
	f.FromSurface(s, 256)

	for _, tc := range []struct {
		x, y   int
		r      rune
		fg, bg tcell.Color
	}{
		{0, 0, canvas.UpperHalf, red, tcell.ColorDefault},
		{1, 1, canvas.LowerHalf, blue, tcell.ColorDefault},
		{2, 2, canvas.UpperHalf, red, blue},
	} {
		c, ok := f.At(tc.x, tc.y)
		if !ok {
			t.Errorf("cell %d,%d is unlit", tc.x, tc.y)
			continue
		}
		if c.Rune != tc.r || c.Fg != tc.fg || c.Bg != tc.bg {
			t.Errorf("cell %d,%d is %q fg=%v bg=%v, want %q fg=%v bg=%v",
				tc.x, tc.y, c.Rune, c.Fg, c.Bg, tc.r, tc.fg, tc.bg)
		}
	}
	// Everything else is untouched surface and must stay unlit.
	if _, ok := f.At(3, 3); ok {
		t.Error("an empty part of the surface came back lit")
	}
}

// Dimming a surface scales its colors. There is no ramp to walk down: a
// surface is already the picture.
func TestSurfaceDimScalesTheColor(t *testing.T) {
	s := canvas.NewSurface(1, 2)
	s.Set(0, 0, tcell.NewRGBColor(200, 100, 40))
	f := NewFrame(1, 1)
	f.FromSurface(s, 128)
	c, _ := f.At(0, 0)
	r, g, b := c.Fg.RGB()
	if r != 100 || g != 50 || b != 20 {
		t.Errorf("dimmed to %d,%d,%d, want 100,50,20", r, g, b)
	}
}

// ColorDefault is the absence of a color and has nothing to scale. Resolving
// it would turn every transparent half of every cell solid.
func TestDimLeavesDefaultAlone(t *testing.T) {
	if got := dimColor(tcell.ColorDefault, 32); got != tcell.ColorDefault {
		t.Errorf("dimming ColorDefault gave %v", got)
	}
}

// The rain dims along its palette rather than by scaling the color it arrives
// at. The ramp runs from a near-black green to a white head and is not linear
// in any channel, so the two disagree — and walking down it is the one that
// stays green.
func TestMatrixDimWalksThePalette(t *testing.T) {
	m := matrix.New(5)
	m.Resize(20, 10)
	m.Advance(60)

	f := NewFrame(20, 10)
	f.FromMatrix(m, 128)

	var checked int
	m.Cells(func(x, y int, mc matrix.Cell) {
		c, ok := f.At(x, y)
		if !ok {
			t.Fatalf("cell %d,%d is unlit but the rain lit it", x, y)
		}
		if want := m.Palette[mc.Intensity*128/256]; c.Fg != want {
			t.Errorf("cell %d,%d is %v, want the palette at half intensity %v",
				x, y, c.Fg, want)
		}
		checked++
	})
	if checked == 0 {
		t.Fatal("the rain lit nothing")
	}
}

// The palette is the matrix's own, so a caller that tuned it — which is what
// Painter.Matrix is for — sees the change behind its text too.
func TestMatrixUsesItsOwnPalette(t *testing.T) {
	m := matrix.New(5)
	m.Palette = canvas.Fire
	m.Resize(20, 10)
	m.Advance(60)

	f := NewFrame(20, 10)
	f.FromMatrix(m, 256)

	var seen bool
	m.Cells(func(x, y int, mc matrix.Cell) {
		c, _ := f.At(x, y)
		if c.Fg != canvas.Fire[mc.Intensity] {
			t.Fatalf("cell %d,%d ignored the palette it was given", x, y)
		}
		seen = true
	})
	if !seen {
		t.Fatal("the rain lit nothing")
	}
}

// A cell with no background emits exactly what the old foreground-only style
// did. That is every cell of the rain, and it is why the rain's output did not
// change when the compositor stopped knowing what the rain was.
func TestCellStyleMatchesSgrWithoutABackground(t *testing.T) {
	c := tcell.NewRGBColor(0, 160, 40)
	for _, bold := range []bool{false, true} {
		if got, want := cellStyle(Cell{Fg: c, Bold: bold}), sgr(c, bold); got != want {
			t.Errorf("bold=%v: cellStyle gave %q, sgr gave %q", bold, got, want)
		}
	}
}

// A background is emitted alongside the foreground, which is what a half block
// needs and what the rain never had.
func TestCellStyleEmitsABackground(t *testing.T) {
	got := cellStyle(Cell{
		Rune: canvas.UpperHalf,
		Fg:   tcell.NewRGBColor(255, 0, 0),
		Bg:   tcell.NewRGBColor(0, 0, 255),
	})
	if want := "\x1b[0;38;2;255;0;0;48;2;0;0;255m"; got != want {
		t.Errorf("cellStyle gave %q, want %q", got, want)
	}
}

var sgrRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// RenderAnim is the point of all of this: a pixel animation behind a help
// screen, which before this could only be the rain.
func TestRenderAnimDrawsBehindTheText(t *testing.T) {
	out := RenderAnim(help, fill{}, Options{Force: true, Width: 60, Warm: -1})
	for _, want := range strings.Split(strings.TrimRight(help, "\n"), "\n") {
		if strings.TrimSpace(want) == "" {
			continue
		}
		if !strings.Contains(strip(out), strings.TrimSpace(want)) {
			t.Errorf("line missing from the render: %q", want)
		}
	}
	if !strings.Contains(out, "48;2;") {
		t.Error("no background color anywhere: the half blocks did not survive")
	}
	if !strings.ContainsRune(out, canvas.UpperHalf) {
		t.Error("no half block anywhere")
	}
}

// Every row still fits the declared width. A row that ran long would wrap and
// shear the whole frame, and a backdrop made of half blocks fills far more of
// each row than the rain does.
func TestRenderAnimRowsFitTheWidth(t *testing.T) {
	const w = 60
	for _, line := range strings.Split(RenderAnim(help, fill{}, Options{Force: true, Width: w, Warm: -1}), "\n") {
		if n := len([]rune(sgrRe.ReplaceAllString(line, ""))); n > w {
			t.Errorf("row is %d columns wide, want at most %d", n, w)
		}
	}
}

// A painter built for an animation drives it, and hands back the surface so a
// caller can draw on the backdrop itself.
func TestPainterForAnimation(t *testing.T) {
	a := &counter{}
	p := NewFor(a, Options{Force: true, Width: 40, Warm: -1})
	if p.Matrix() != nil {
		t.Error("an animation painter has a matrix")
	}
	out := p.Frame(help, 0)
	if p.Surface() == nil {
		t.Fatal("no surface after the first frame")
	}
	if a.resized != 1 {
		t.Errorf("Resize called %d times, want 1", a.resized)
	}
	if p.Animation() != a {
		t.Error("Animation did not give back what was passed in")
	}
	if !strings.ContainsRune(out, canvas.UpperHalf) {
		t.Error("nothing was drawn")
	}

	// A dt of zero redraws without moving; a positive one advances. The
	// warm-up already drew, so what matters is the change from here.
	base := a.frames
	if base == 0 {
		t.Fatal("the warm-up drew nothing")
	}
	p.Frame(help, 0)
	if a.frames != base {
		t.Errorf("a zero dt advanced the animation %d times", a.frames-base)
	}
	p.Frame(help, 0.1)
	if a.frames != base+1 {
		t.Errorf("advanced %d times, want 1", a.frames-base)
	}
}

// The rain painter is unchanged: it still has a matrix and no surface.
func TestPainterForRainIsUnchanged(t *testing.T) {
	p := New(Options{Force: true, Width: 40, Seed: 3})
	p.Frame(help, 0)
	if p.Matrix() == nil {
		t.Error("no matrix")
	}
	if p.Surface() != nil || p.Animation() != nil {
		t.Error("the rain painter grew a surface or an animation")
	}
}

// Warm hands the time over in small steps rather than one lump, because these
// animations integrate: a single step of a second would send everything that
// moves a second's travel in a straight line.
func TestWarmUpStepsSmall(t *testing.T) {
	a := &counter{}
	warmUp(a, canvas.NewSurface(4, 4), 1)
	if a.frames < 55 || a.frames > 65 {
		t.Errorf("one second of warm-up took %d steps, want about 60", a.frames)
	}
	if a.maxDt > 0.02 {
		t.Errorf("largest step was %v, want sixtieths", a.maxDt)
	}
}

// fill lights every pixel, so a frame taken from it is entirely half blocks.
type fill struct{}

func (f fill) Resize(w, h int) {}
func (f fill) Frame(s *canvas.Surface, dt float64) {
	w, h := s.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s.Set(x, y, tcell.NewRGBColor(int32(x*3%256), 64, int32(y*5%256)))
		}
	}
}

// counter records what it was asked to do.
type counter struct {
	resized, frames int
	maxDt           float64
}

func (c *counter) Resize(w, h int) { c.resized++ }
func (c *counter) Frame(s *canvas.Surface, dt float64) {
	c.frames++
	if dt > c.maxDt {
		c.maxDt = dt
	}
	s.Set(0, 0, tcell.NewRGBColor(1, 2, 3))
}
