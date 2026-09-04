package langton

import (
	"testing"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 64

// dt is one tick of the old thirty-a-second loop, which at the default rate is
// the 250 ant moves a frame used to run.
const dt = 1.0 / 30

// one returns a colony of exactly one ant, parked at a known cell facing a
// known way, so a test can check the two rules directly.
func one(x, y, dir int) *Langton {
	l := New(1)
	l.Ants = 1
	l.Resize(tw, th)
	l.ants[0] = ant{x: x, y: y, dir: dir}
	return l
}

func TestWhiteCellTurnsRightAndIsPainted(t *testing.T) {
	l := one(10, 10, 0) // facing up
	l.step()
	if l.cell[10*tw+10] == 0 {
		t.Error("the ant left a white cell white")
	}
	if l.ants[0].dir != 1 {
		t.Errorf("dir %d, want 1: a white cell is a right turn", l.ants[0].dir)
	}
	if l.ants[0].x != 11 || l.ants[0].y != 10 {
		t.Errorf("ant at %d,%d, want 11,10", l.ants[0].x, l.ants[0].y)
	}
}

func TestBlackCellTurnsLeftAndIsCleared(t *testing.T) {
	l := one(10, 10, 0) // facing up
	l.cell[10*tw+10] = 1
	l.step()
	if l.cell[10*tw+10] != 0 {
		t.Error("the ant left a black cell black")
	}
	if l.ants[0].dir != 3 {
		t.Errorf("dir %d, want 3: a black cell is a left turn", l.ants[0].dir)
	}
	if l.ants[0].x != 9 || l.ants[0].y != 10 {
		t.Errorf("ant at %d,%d, want 9,10", l.ants[0].x, l.ants[0].y)
	}
}

func TestWrapsAtTheEdge(t *testing.T) {
	// Facing down on a white cell at the left edge: turn right is now facing
	// left, and the step takes it off the board.
	l := one(0, 0, 2)
	l.step()
	if l.ants[0].x != tw-1 {
		t.Errorf("x %d, want %d: the grid did not wrap", l.ants[0].x, tw-1)
	}
	l = one(0, 0, 1) // facing right on white turns right, to facing down
	l.step()
	if l.ants[0].y != 1 {
		t.Errorf("y %d, want 1", l.ants[0].y)
	}
}

func TestManyStepsDisturbALargeArea(t *testing.T) {
	l := one(tw/2, th/2, 0)
	for i := 0; i < 20000; i++ {
		l.step()
	}
	var touched, black int
	for i := range l.cell {
		if l.stamp[i] != 0 {
			touched++
		}
		if l.cell[i] != 0 {
			black++
		}
	}
	// Twenty thousand steps is well past the point the ant stops shuffling
	// around its start and builds the highway, so a large fraction of the
	// board has been visited.
	if touched < 500 {
		t.Errorf("only %d cells were ever touched in 20000 steps", touched)
	}
	if black == 0 || black == len(l.cell) {
		t.Errorf("board is uniform: %d black of %d", black, len(l.cell))
	}
}

func TestAntsPaintInTheirOwnColours(t *testing.T) {
	l := New(2)
	l.Ants = 3
	l.Resize(tw, th)
	for i := 0; i < 20000; i++ {
		l.step()
	}
	seen := map[byte]bool{}
	for _, v := range l.cell {
		if v != 0 {
			seen[v] = true
		}
	}
	if len(seen) < 2 {
		t.Fatalf("three ants left %d distinct owners on the board", len(seen))
	}
	if len(l.pals) != 3 {
		t.Fatalf("%d palettes for 3 ants", len(l.pals))
	}
	if l.pals[0][255] == l.pals[1][255] {
		t.Error("two ants share a color")
	}
}

func TestRecentlyFlippedCellsAreBrighter(t *testing.T) {
	l := one(10, 10, 0)
	for i := 0; i < 5000; i++ {
		l.step()
	}
	// The cell the ant is standing on is flipped by the next step, so it is
	// the freshest thing on the board afterwards.
	here := l.ants[0].y*tw + l.ants[0].x
	l.step()
	if fresh := l.intensity(here); fresh != 255 {
		t.Fatalf("a cell flipped this step has intensity %d, want 255", fresh)
	}
	cold := -1
	for i := range l.stamp {
		if l.stamp[i] == 0 {
			cold = i
			break
		}
	}
	if cold < 0 {
		t.Fatal("the ant touched every cell; pick a bigger board")
	}
	if stale := l.intensity(cold); stale >= 255 {
		t.Fatalf("an untouched cell is as bright as a fresh one: %d", stale)
	}
	// Fading has to stop somewhere, or old work would vanish entirely.
	l.now = 1 << 30
	if got := l.intensity(cold); got != l.MinIntensity {
		t.Fatalf("an ancient cell faded to %d, want the floor %d", got, l.MinIntensity)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) []byte {
		l := New(seed)
		l.Resize(tw, th)
		for i := 0; i < 5000; i++ {
			l.step()
		}
		out := make([]byte, len(l.cell))
		copy(out, l.cell)
		return out
	}
	a, b := run(4), run(4)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at index %d", i)
		}
	}
	c := run(5)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced an identical board")
	}
}

// stepsOver drives the ants through Frame and reports how many moves ran.
func stepsOver(frames int, step float64) int {
	l := New(1)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		l.Frame(s, step)
	}
	return int(l.now)
}

func TestStepRateIsFrameRateIndependent(t *testing.T) {
	// Two seconds of wall clock at three frame rates. The ants walk at 7500
	// moves a second in all of them, so the pattern is the same age however
	// often it was drawn.
	slow := stepsOver(60, 1.0/30)
	fast := stepsOver(120, 1.0/60)
	odd := stepsOver(34, 1.0/17)
	if slow < 14900 || slow > 15100 {
		t.Fatalf("two seconds at 30fps ran %d moves, want about 15000", slow)
	}
	for _, got := range []int{fast, odd} {
		if d := got - slow; d < -30 || d > 30 {
			t.Fatalf("same two seconds ran %d moves at one frame rate and %d at another", slow, got)
		}
	}
}

func TestOneFrameAtTheDefaultIntervalIsABurst(t *testing.T) {
	l := New(1)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	l.Frame(s, dt)
	if got := int(l.now); got < 245 || got > 255 {
		t.Fatalf("a frame of %.4fs ran %d moves, want about 250", float64(dt), got)
	}
}

func TestFrameWritesToTheSurface(t *testing.T) {
	l := New(1)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 20; i++ {
		l.Frame(s, dt)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != 0 {
				return
			}
		}
	}
	t.Error("Frame left the surface empty")
}
