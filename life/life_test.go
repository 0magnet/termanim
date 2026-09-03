package life

import (
	"testing"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 32, 32

// dt is one tick of the old thirty-a-second loop, which is also exactly the
// default generation interval, so a Frame at this dt runs one generation.
const dt = 1.0 / 30

// blank returns a board with the random soup wiped out, so a test can place a
// known pattern and watch exactly what the rules do to it.
func blank() *Life {
	l := New(1)
	l.Resize(tw, th)
	for i := range l.cur {
		l.cur[i] = 0
	}
	l.pop = -1
	return l
}

func place(l *Life, cells [][2]int) {
	for _, c := range cells {
		l.cur[(c[1]%l.h)*l.w+c[0]%l.w] = 1
	}
}

func aliveSet(l *Life) map[[2]int]bool {
	m := make(map[[2]int]bool)
	for y := 0; y < l.h; y++ {
		for x := 0; x < l.w; x++ {
			if l.cur[y*l.w+x] > 0 {
				m[[2]int{x, y}] = true
			}
		}
	}
	return m
}

func want(t *testing.T, l *Life, cells [][2]int) {
	t.Helper()
	got := aliveSet(l)
	if len(got) != len(cells) {
		t.Fatalf("population %d, want %d: %v", len(got), len(cells), got)
	}
	for _, c := range cells {
		if !got[[2]int{c[0] % l.w, c[1] % l.h}] {
			t.Fatalf("cell %v is dead; alive: %v", c, got)
		}
	}
}

var (
	block   = [][2]int{{4, 4}, {5, 4}, {4, 5}, {5, 5}}
	blinker = [][2]int{{4, 5}, {5, 5}, {6, 5}}
	glider  = [][2]int{{1, 0}, {2, 1}, {0, 2}, {1, 2}, {2, 2}}
)

func shift(cells [][2]int, dx, dy int) [][2]int {
	out := make([][2]int, len(cells))
	for i, c := range cells {
		out[i] = [2]int{c[0] + dx, c[1] + dy}
	}
	return out
}

func TestBlockIsAStillLife(t *testing.T) {
	l := blank()
	place(l, block)
	for i := 0; i < 4; i++ {
		l.step()
	}
	want(t, l, block)
}

func TestBlinkerHasPeriodTwo(t *testing.T) {
	l := blank()
	place(l, blinker)
	l.step()
	want(t, l, [][2]int{{5, 4}, {5, 5}, {5, 6}})
	l.step()
	want(t, l, blinker)
}

func TestGliderMoves(t *testing.T) {
	l := blank()
	place(l, shift(glider, 8, 8))
	for i := 0; i < 4; i++ {
		l.step()
	}
	want(t, l, shift(glider, 9, 9))
}

func TestGliderWrapsAtTheEdge(t *testing.T) {
	l := blank()
	// Started near the corner, four cycles carry the glider off the bottom
	// right and back in at the top left.
	place(l, shift(glider, tw-2, th-2))
	for i := 0; i < 16; i++ {
		l.step()
	}
	want(t, l, shift(glider, tw+2, th+2))
}

func TestPopulationChangesOverTime(t *testing.T) {
	l := New(7)
	l.Resize(tw, th)
	first := len(aliveSet(l))
	same := 0
	for i := 0; i < 40; i++ {
		l.step()
		if len(aliveSet(l)) == first {
			same++
		}
	}
	if same == 40 {
		t.Fatal("population never moved off its starting value")
	}
}

func TestStagnationReseeds(t *testing.T) {
	l := blank()
	place(l, block)
	// A block on an otherwise empty board is as dead as Life gets: it matches
	// the generation two back forever. The cycle detector has to notice.
	for i := 0; i < l.CycleStallGens+4; i++ {
		l.step()
	}
	if l.reseeds == 0 {
		t.Fatal("a still life ran to stagnation without reseeding")
	}
	if len(aliveSet(l)) <= len(block) {
		t.Fatal("reseed added no cells")
	}
}

func TestPopulationStallReseeds(t *testing.T) {
	l := blank()
	// Disable the cycle detector so only the slower population signal is left.
	l.CycleStallGens = 1 << 30
	l.PopStallGens = 6
	place(l, blinker)
	for i := 0; i < 20; i++ {
		l.step()
	}
	if l.reseeds == 0 {
		t.Fatal("a constant population ran forever without reseeding")
	}
}

func TestReseedDoesNotImmediatelyRetrigger(t *testing.T) {
	l := blank()
	place(l, block)
	for i := 0; i < 60; i++ {
		l.step()
	}
	// Sixty generations with a threshold of twelve cannot legitimately reseed
	// more than a handful of times; a poisoned-history bug shows up as one
	// reseed per generation.
	if l.reseeds > 5 {
		t.Fatalf("reseeded %d times in 60 generations", l.reseeds)
	}
}

func TestOlderCellsAreDimmer(t *testing.T) {
	l := New(1)
	if newborn, old := l.intensity(1), l.intensity(30); newborn <= old {
		t.Fatalf("age is not visible: newborn=%d old=%d", newborn, old)
	}
	if got := l.intensity(255); got != l.MinIntensity {
		t.Fatalf("ancient cell faded to %d, want the floor %d", got, l.MinIntensity)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []byte {
		l := New(3)
		l.Resize(tw, th)
		for i := 0; i < 200; i++ {
			l.step()
		}
		out := make([]byte, len(l.cur))
		copy(out, l.cur)
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at index %d", i)
		}
	}
}

// gensOver drives the animation through Frame for the given number of frames
// at the given frame interval and reports how many generations ran.
func gensOver(frames int, step float64) int {
	l := New(2)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		l.Frame(s, step)
	}
	return l.gens
}

func TestGenerationRateIsFrameRateIndependent(t *testing.T) {
	// Two seconds of wall clock at three different frame rates. The board is
	// meant to evolve at thirty generations a second in all of them; before
	// the simulation was driven by elapsed time, doubling the frame rate
	// doubled the speed of everything on screen.
	slow := gensOver(60, 1.0/30)
	fast := gensOver(120, 1.0/60)
	odd := gensOver(34, 1.0/17)
	if slow < 58 || slow > 62 {
		t.Fatalf("two seconds at 30fps ran %d generations, want about 60", slow)
	}
	for _, got := range []int{fast, odd} {
		if got < slow-2 || got > slow+2 {
			t.Fatalf("same two seconds ran %d generations at one frame rate and %d at another", slow, got)
		}
	}
}

func TestOneFrameAtTheDefaultIntervalIsOneGeneration(t *testing.T) {
	l := blank()
	place(l, blinker)
	s := canvas.NewSurface(tw, th)
	l.Frame(s, dt)
	want(t, l, [][2]int{{5, 4}, {5, 5}, {5, 6}})
}

func TestATinyFrameRunsNoGeneration(t *testing.T) {
	// A frame shorter than the generation interval must not run a fractional
	// generation; the time is carried instead.
	l := blank()
	place(l, blinker)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 3; i++ {
		l.Frame(s, dt/4)
	}
	want(t, l, blinker)
	// Once the carried time passes a whole interval the generation runs, and
	// exactly one of them does.
	l.Frame(s, dt/4)
	l.Frame(s, dt/4)
	want(t, l, [][2]int{{5, 4}, {5, 5}, {5, 6}})
}

func TestFrameWritesToTheSurface(t *testing.T) {
	l := New(1)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	l.Frame(s, dt)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != 0 {
				return
			}
		}
	}
	t.Error("Frame left the surface empty")
}
