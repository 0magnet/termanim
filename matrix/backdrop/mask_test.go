package backdrop

import (
	"strings"
	"testing"

	"github.com/0magnet/termanim/matrix"
)

func TestStencilZeroValueChangesNothing(t *testing.T) {
	s := &Stencil{Rows: []string{"##", "##"}}
	for _, p := range [][2]int{{0, 0}, {1, 1}, {9, 9}} {
		if got := s.ScaleAt(p[0], p[1]); got != 256 {
			t.Errorf("ScaleAt(%d,%d) = %d, want 256 for a zero-valued stencil", p[0], p[1], got)
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
		x, y int
		want int
	}{
		{11, 5, 512}, // the '#' on the first row
		{10, 5, 64},  // the space beside it
		{10, 6, 512}, // second row is solid
		{12, 6, 512},
		{13, 6, 64}, // past the end of the row
		{10, 4, 64}, // above the block
		{0, 0, 64},  // nowhere near it
	}
	for _, c := range cases {
		if got := s.ScaleAt(c.x, c.y); got != c.want {
			t.Errorf("ScaleAt(%d,%d) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

func TestStencilSize(t *testing.T) {
	s := &Stencil{Rows: []string{"#", "####", "##"}}
	cols, rows := s.Size()
	if cols != 4 || rows != 3 {
		t.Errorf("Size() = %d,%d, want 4,3", cols, rows)
	}
}

// A mask brightens the rain that is there and cannot invent rain where there
// is none — the property that keeps a masked shape looking like weather.
func TestMaskCannotLightAnEmptyCell(t *testing.T) {
	m := matrix.New(1)
	m.Resize(20, 10)
	m.Advance(50)

	f := NewFrame(20, 10)
	f.SetMask(&Stencil{Rows: fullBlock(20, 10), Inside: 4096})
	f.FromMatrix(m, 256)

	plain := NewFrame(20, 10)
	plain.FromMatrix(m, 256)

	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			_, litPlain := plain.At(x, y)
			_, litMasked := f.At(x, y)
			if litPlain != litMasked {
				t.Fatalf("mask changed which cells are lit at %d,%d", x, y)
			}
		}
	}
}

// Dim and a mask compose: the mask varies the level Dim set.
func TestMaskComposesWithDim(t *testing.T) {
	m := matrix.New(7)
	m.Resize(8, 8)
	m.Advance(40)

	half := NewFrame(8, 8)
	half.FromMatrix(m, 128)

	dimmedTwice := NewFrame(8, 8)
	dimmedTwice.SetMask(&Stencil{Rows: fullBlock(8, 8), Inside: 128})
	dimmedTwice.FromMatrix(m, 128)

	same := true
	for y := 0; y < 8 && same; y++ {
		for x := 0; x < 8; x++ {
			a, _ := half.At(x, y)
			b, _ := dimmedTwice.At(x, y)
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

// fullBlock is a stencil block covering the whole grid.
func fullBlock(cols, rows int) []string {
	out := make([]string, rows)
	for i := range out {
		out[i] = strings.Repeat("#", cols)
	}
	return out
}

// fitRecorder is a Mask that notes the grid size it was fitted to.
type fitRecorder struct{ cols, rows int }

func (f *fitRecorder) ScaleAt(int, int) int { return 256 }
func (f *fitRecorder) Fit(cols, rows int)   { f.cols, f.rows = cols, rows }

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
