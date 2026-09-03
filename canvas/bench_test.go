package canvas

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/internal/simscreen"
)

// A frame must not allocate.
//
// This is the hottest loop these animations have — every cell of the surface,
// sixty times a second — and where it runs the collector is not free. In a
// browser everything shares the one thread, so a collection is a dropped frame;
// under TinyGo's conservative collector the heap does not come back down at all.
//
// It reached zero by giving the screen a string it already has rather than a
// rune it has to build one from. SetContent takes a rune and is deprecated for
// it: internally it does string(append([]rune{r}, combining...)), so a
// full-screen frame cost about twenty thousand allocations, which measured as
// three quarters of the cost of drawing one. Put takes the string, and the two
// this draws with are constants.
//
// Read the steady-state number, not the first one. tcell segments a grapheme
// cluster the first time a cell is given a string it does not already hold, and
// caches the result — so the first frame allocates about twice per cell and
// every frame after it allocates nothing. At a low iteration count that
// one-time cost is divided over too few frames and reads as if drawing
// allocates when it does not.
//
// Run with: go test -run xxx -bench Flush -benchtime 1000x ./canvas/
func BenchmarkFlush(b *testing.B) {
	screen := simscreen.NewScreen()
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(210, 49)

	s := NewSurface(210, 98)
	for i := range s.px {
		s.px[i] = tcell.NewRGBColor(int32(i%256), 40, 90)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.flush(screen)
	}
}
