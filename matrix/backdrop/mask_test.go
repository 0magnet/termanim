package backdrop

import (
	"strings"
	"testing"

	"github.com/0magnet/termanim/matrix"
)

func TestStencilZeroValueChangesNothing(t *testing.T) {
	s := &Stencil{Rows: []string{"##", "##"}}
	for _, p := range [][2]int{{0, 0}, {1, 1}, {9, 9}} {
		if got := s.IntensityAt(p[0], p[1], 120); got != 120 {
			t.Errorf("IntensityAt(%d,%d,120) = %d, want 120 for a zero-valued stencil", p[0], p[1], got)
		}
	}
}

func TestStencilInsideOutside(t *testing.T) {
	s := &Stencil{
		Rows:    []string{" # ", "###"},
		X:       10,
		Y:       5,
		Inside:  512,
		Outside: 64,
	}
	cases := []struct {
		x, y, n, want int
	}{
		{11, 5, 100, 200}, // the '#' on the first row: doubled
		{10, 5, 100, 25},  // the space beside it: quartered
		{10, 6, 100, 200}, // second row is solid
		{13, 6, 100, 25},  // past the end of the row
		{10, 4, 100, 25},  // above the block
		{11, 5, 200, 255}, // clamped to the top of the palette
	}
	for _, c := range cases {
		if got := s.IntensityAt(c.x, c.y, c.n); got != c.want {
			t.Errorf("IntensityAt(%d,%d,%d) = %d, want %d", c.x, c.y, c.n, got, c.want)
		}
	}
}

func TestStencilFloorFillsDarkCells(t *testing.T) {
	s := &Stencil{Rows: []string{"# "}, Floor: 60}

	if got := s.IntensityAt(0, 0, 0); got != 60 {
		t.Errorf("a dark cell inside the shape = %d, want the floor 60", got)
	}
	if got := s.IntensityAt(0, 0, 180); got != 180 {
		t.Errorf("a lit cell inside the shape = %d, want its own 180 — the floor is a floor, not a level", got)
	}
	if got := s.IntensityAt(1, 0, 0); got != 0 {
		t.Errorf("a dark cell outside the shape = %d, want 0", got)
	}
}

func TestStencilSize(t *testing.T) {
	s := &Stencil{Rows: []string{"#", "####", "##"}}
	cols, rows := s.Size()
	if cols != 4 || rows != 3 {
		t.Errorf("Size() = %d,%d, want 4,3", cols, rows)
	}
}

// The floor is what makes a shape solid: every cell inside it is lit, whether
// or not a stream happened to be crossing it.
func TestFloorMakesTheShapeSolid(t *testing.T) {
	m := matrix.New(1)
	m.Resize(20, 10)
	m.Advance(50)

	f := NewFrame(20, 10)
	f.SetMask(&Stencil{Rows: fullBlock(6, 4), X: 2, Y: 2, Floor: 60})
	f.FromMatrix(m, 256)

	for y := 2; y < 6; y++ {
		for x := 2; x < 8; x++ {
			c, lit := f.At(x, y)
			if !lit {
				t.Fatalf("cell %d,%d inside the shape is dark", x, y)
			}
			if c.Rune == 0 {
				t.Fatalf("cell %d,%d inside the shape has no glyph", x, y)
			}
		}
	}
}

// A filled cell carries the rain's own glyph at that position, not one made up
// for the shape.
func TestFilledCellsUseTheRainsGlyphs(t *testing.T) {
	m := matrix.New(9)
	m.Resize(12, 8)
	m.Advance(40)

	f := NewFrame(12, 8)
	f.SetMask(&Stencil{Rows: fullBlock(12, 8), Floor: 50})
	f.FromMatrix(m, 256)

	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			c, _ := f.At(x, y)
			if c.Rune != m.GlyphAt(x, y) {
				t.Fatalf("cell %d,%d drew %q, the rain holds %q", x, y, c.Rune, m.GlyphAt(x, y))
			}
		}
	}
}

// Outside a shape with no floor, a mask leaves the rain exactly as it was.
func TestOutsideIsUntouched(t *testing.T) {
	m := matrix.New(5)
	m.Resize(20, 10)
	m.Advance(50)

	masked := NewFrame(20, 10)
	masked.SetMask(&Stencil{Rows: fullBlock(4, 3), X: 0, Y: 0, Floor: 80})
	masked.FromMatrix(m, 256)

	plain := NewFrame(20, 10)
	plain.FromMatrix(m, 256)

	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			if x < 4 && y < 3 {
				continue
			}
			a, litA := plain.At(x, y)
			b, litB := masked.At(x, y)
			if litA != litB || a != b {
				t.Fatalf("cell %d,%d outside the shape changed", x, y)
			}
		}
	}
}

// Dim and a mask compose: the mask acts on the level Dim set.
func TestMaskComposesWithDim(t *testing.T) {
	m := matrix.New(7)
	m.Resize(8, 8)
	m.Advance(40)

	half := NewFrame(8, 8)
	half.FromMatrix(m, 128)

	quarter := NewFrame(8, 8)
	quarter.SetMask(&Stencil{Rows: fullBlock(8, 8), Inside: 128})
	quarter.FromMatrix(m, 128)

	same := true
	for y := 0; y < 8 && same; y++ {
		for x := 0; x < 8; x++ {
			a, _ := half.At(x, y)
			b, _ := quarter.At(x, y)
			if a.Fg != b.Fg {
				same = false
				break
			}
		}
	}
	if same {
		t.Error("a mask of 128 under a dim of 128 produced the same colors as dim alone")
	}
}

// fitRecorder is a Mask that notes the grid size it was fitted to.
type fitRecorder struct{ cols, rows int }

func (f *fitRecorder) IntensityAt(_, _, n int) int { return n }
func (f *fitRecorder) Fit(cols, rows int)          { f.cols, f.rows = cols, rows }

func TestFitterIsToldTheGridSize(t *testing.T) {
	m := matrix.New(3)
	m.Resize(24, 6)
	m.Advance(20)

	r := &fitRecorder{}
	f := NewFrame(24, 6)
	f.SetMask(r)
	f.FromMatrix(m, 256)

	if r.cols != 24 || r.rows != 6 {
		t.Errorf("Fit got %d,%d, want 24,6", r.cols, r.rows)
	}
}

// A plain Mask that does not implement Fitter must still work.
func TestNonFitterMaskIsFine(t *testing.T) {
	m := matrix.New(4)
	m.Resize(10, 4)
	m.Advance(20)

	f := NewFrame(10, 4)
	f.SetMask(&Stencil{Rows: []string{"##"}, Inside: 300})
	f.FromMatrix(m, 256) // must not panic
}

// fullBlock is a stencil block of the given size.
func fullBlock(cols, rows int) []string {
	out := make([]string, rows)
	for i := range out {
		out[i] = strings.Repeat("#", cols)
	}
	return out
}
