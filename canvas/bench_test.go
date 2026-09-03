package canvas

import (
	"testing"

	"github.com/gdamore/tcell/v2"
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
// Run with: go test -run xxx -bench Flush -benchtime 20x ./canvas/
func BenchmarkFlush(b *testing.B) {
	screen := tcell.NewSimulationScreen("UTF-8")
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
