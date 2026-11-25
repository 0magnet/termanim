package loop

import (
	"reflect"
	"testing"

	"github.com/0magnet/termanim/matrix"
	"github.com/0magnet/termanim/matrix/backdrop"
)

func cells(r *Rain, t int) []matrix.Cell {
	r.Seek(t)
	cols, rows := r.Size()
	out := make([]matrix.Cell, 0, cols*rows)
	for y := range rows {
		for x := range cols {
			c, _ := r.CellAt(x, y)
			c.Rune = r.GlyphAt(x, y)
			out = append(out, c)
		}
	}
	return out
}

// The rain loops exactly: the frame a period on is the same, cell for cell,
// whatever the seed.
func TestRainIsPeriodic(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		r := New(seed, 80, 40, 240)
		if !reflect.DeepEqual(cells(r, 0), cells(r, 240)) {
			t.Errorf("seed %d: step 240 is not step 0", seed)
		}
		if !reflect.DeepEqual(cells(r, 17), cells(r, 257)) {
			t.Errorf("seed %d: step 257 is not step 17", seed)
		}
		if !reflect.DeepEqual(cells(r, -1), cells(r, 239)) {
			t.Errorf("seed %d: step -1 is not step 239", seed)
		}
		if reflect.DeepEqual(cells(r, 0), cells(r, 1)) {
			t.Errorf("seed %d: step 1 is step 0, the rain is not moving", seed)
		}
	}
}

func TestPeriodIsWholeBeats(t *testing.T) {
	for _, c := range []struct{ in, want int }{{240, 240}, {241, 240}, {2, ChangeEvery}, {0, ChangeEvery}} {
		if got := New(1, 10, 10, c.in).Period(); got != c.want {
			t.Errorf("New(period %d).Period() = %d, want %d", c.in, got, c.want)
		}
	}
}

// A lit cell reports the glyph GlyphAt does, so a mask that fills dark cells
// draws the same alphabet the streams carry.
func TestLitGlyphIsHeldGlyph(t *testing.T) {
	r := New(3, 40, 20, 90)
	lit := 0
	r.Cells(func(x, y int, c matrix.Cell) {
		lit++
		if g := r.GlyphAt(x, y); g != c.Rune {
			t.Fatalf("cell %d,%d: CellAt rune %q, GlyphAt %q", x, y, c.Rune, g)
		}
	})
	if lit == 0 {
		t.Fatal("no lit cells")
	}
}

// A backdrop frame and its mask fill from the periodic rain, and the filled
// frame repeats with it.
func TestFrameFromRainRepeats(t *testing.T) {
	r := New(9, 30, 12, 60)
	mask := &backdrop.Stencil{Rows: []string{"#####", "#####"}, X: 10, Y: 5, Floor: 120}
	grab := func(t int) []backdrop.Cell {
		r.Seek(t)
		f := backdrop.NewFrame(30, 12)
		f.SetMask(mask)
		f.FromRain(r, r.Palette, 256)
		var out []backdrop.Cell
		for y := range 12 {
			for x := range 30 {
				c, _ := f.At(x, y)
				out = append(out, c)
			}
		}
		return out
	}
	if !reflect.DeepEqual(grab(5), grab(65)) {
		t.Error("frame from step 65 is not the frame from step 5")
	}
	for y := 5; y < 7; y++ {
		for x := 10; x < 15; x++ {
			r.Seek(5)
			f := backdrop.NewFrame(30, 12)
			f.SetMask(mask)
			f.FromRain(r, r.Palette, 256)
			if _, ok := f.At(x, y); !ok {
				t.Fatalf("cell %d,%d inside the stencil is not lit", x, y)
			}
		}
	}
}

func BenchmarkSeekAndCells(b *testing.B) {
	r := New(1, 200, 60, 240)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		r.Seek(i)
		r.Cells(func(int, int, matrix.Cell) {})
	}
}
