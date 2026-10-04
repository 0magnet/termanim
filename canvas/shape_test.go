package canvas

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/internal/simscreen"
)

// shapeScreen returns a mock screen of cols by rows and a surface sized for
// drawing it by shape.
func shapeScreen(t testing.TB, cols, rows int) (tcell.Screen, *Surface) {
	t.Helper()
	screen := simscreen.NewScreen()
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(cols, rows)
	return screen, NewSurface(cols*ShapeCellW, rows*ShapeCellH)
}

// A wireframe is the case shape drawing is for: a thin diagonal must come out
// as slashes in its own color, and the empty space around it must stay the
// terminal's background rather than becoming characters.
func TestShapeDrawsDiagonalsAsSlashes(t *testing.T) {
	const n = 12
	white := tcell.NewRGBColor(255, 255, 255)
	for _, c := range []struct {
		want string
		col  func(row int) int
		on   func(x, y float64) bool
	}{
		{`\`, func(r int) int { return r },
			func(x, y float64) bool { return math.Abs(x-y/2) < 0.75 }},
		{`/`, func(r int) int { return n - 1 - r },
			func(x, y float64) bool { return math.Abs((2*n-x)-y/2) < 0.75 }},
	} {
		screen, s := shapeScreen(t, n, n)
		w, h := s.Size()
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if c.on(float64(x)+0.5, float64(y)+0.5) {
					s.Set(x, y, white)
				}
			}
		}
		newShapeRenderer().flush(s, screen)
		for r := 1; r < n-1; r++ {
			str, style, _ := screen.Get(c.col(r), r)
			if str != c.want {
				t.Errorf("%s line, row %d: drew %q", c.want, r, str)
			}
			if style.GetForeground() != white {
				t.Errorf("%s line, row %d: color %v, want white", c.want, r, style.GetForeground())
			}
		}
		// A cell far from the line is blank and unstyled.
		if str, style, _ := screen.Get(c.col(n/2)^4, n/2); str != " " || style != tcell.StyleDefault {
			t.Errorf("%s line: empty cell drew %q in %v", c.want, str, style)
		}
	}
}

// putScreen is a screen that only takes Put calls, so an allocation count
// measures the flush and not the goroutines of a real screen behind it.
type putScreen struct{ tcell.Screen }

func (putScreen) Put(_, _ int, str string, _ tcell.Style) (string, int) { return str, 1 }

// The shape flush is as hot as the half-block one and has to meet the same
// bar: no allocation once the matcher has seen the frame. Measured against
// putScreen, because a mock terminal's own goroutines allocate now and then
// and made this flaky; BenchmarkFlushShape measures the real screen.
func TestShapeFlushDoesNotAllocate(t *testing.T) {
	var screen tcell.Screen = putScreen{}
	s := NewSurface(40*ShapeCellW, 12*ShapeCellH)
	w, h := s.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s.Set(x, y, tcell.NewRGBColor(int32(x*6), int32(y*5), 90))
		}
	}
	r := newShapeRenderer()
	r.flush(s, screen) // fills the matcher's cache
	if n := testing.AllocsPerRun(20, func() { r.flush(s, screen) }); n != 0 {
		t.Errorf("shape flush allocates %v times a frame", n)
	}
}

// ShapeScreen is what tells Run to draw by shape.
func TestShapeScreenAsksForShape(t *testing.T) {
	var screen tcell.Screen = ShapeScreen{}
	if sh, ok := screen.(shaped); !ok || !sh.ShapeASCII() {
		t.Error("ShapeScreen does not ask for shape drawing")
	}
}

// Run with: go test -run xxx -bench FlushShape -benchtime 200x ./canvas/
func BenchmarkFlushShape(b *testing.B) {
	screen, s := shapeScreen(b, 210, 49)
	w, h := s.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s.Set(x, y, tcell.NewRGBColor(int32((x*y)%256), int32(x%256), 90))
		}
	}
	r := newShapeRenderer()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.flush(s, screen)
	}
}
