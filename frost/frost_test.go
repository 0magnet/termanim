package frost

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 96, 72

// dt is one tick of a thirty-a-second loop.
const dt = 1.0 / 30

// grow runs the animation for roughly the given number of seconds.
func grow(f *Frost, secs float64) *canvas.Surface {
	s := canvas.NewSurface(tw, th)
	for t := 0.0; t < secs; t += dt {
		f.Frame(s, dt)
	}
	return s
}

func TestEveryParticleSticksTouchingTheClusterAndNoneInsideIt(t *testing.T) {
	// The two halves of the sticking rule, checked over a whole crystal.
	//
	// Touching: every frozen cell except the seed has ice orthogonally next to
	// it, which is the same as saying the cluster is four-connected. A walker
	// that froze in mid-air would show up as an isolated cell.
	//
	// Never inside: as many cells are ice as particles have frozen. A walker
	// that overwrote a cell already frozen would leave the two out of step.
	f := New(1)
	f.Resize(tw, th)
	grow(f, 6)
	if f.stuck < 40 {
		t.Fatalf("only %d particles froze in six seconds; nothing to test", f.stuck)
	}
	var ice int
	seed := f.cy*f.w + f.cx
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			i := y*f.w + x
			if f.grid[i] == 0 {
				continue
			}
			ice++
			if i == seed {
				continue
			}
			touch := (x > 0 && f.grid[i-1] != 0) ||
				(x+1 < f.w && f.grid[i+1] != 0) ||
				(y > 0 && f.grid[i-f.w] != 0) ||
				(y+1 < f.h && f.grid[i+f.w] != 0)
			if !touch {
				t.Fatalf("ice at %d,%d (arrival %d) touches nothing: it froze in mid-air",
					x, y, f.grid[i])
			}
		}
	}
	if uint32(ice) != f.stuck {
		t.Errorf("%d frozen cells for %d particles: something froze on top of ice", ice, f.stuck)
	}
	if f.grid[seed] != 1 {
		t.Errorf("the seed cell holds arrival %d, want 1", f.grid[seed])
	}
}

func TestAWalkerFreezesWhereItFirstTouches(t *testing.T) {
	// One walker, one seed, an otherwise empty window. Wherever it ends up,
	// it has to be a cell orthogonally next to the seed — not on it, and not
	// a cell away from it.
	f := New(2)
	f.Walkers = 1
	f.SpawnMargin = 3
	f.Resize(tw, th)
	seed := f.cy*f.w + f.cx
	for i := 0; i < 100000 && f.stuck < 2; i++ {
		f.step()
	}
	if f.stuck != 2 {
		t.Fatal("a single walker never reached the seed")
	}
	at := -1
	for i, g := range f.grid {
		if g == 2 {
			at = i
		}
	}
	if at < 0 {
		t.Fatal("the second particle is not on the grid")
	}
	dx := at%f.w - f.cx
	dy := at/f.w - f.cy
	if abs(dx)+abs(dy) != 1 {
		t.Errorf("the particle froze at %+d,%+d from the seed: that is not touching", dx, dy)
	}
	if f.grid[seed] != 1 {
		t.Error("the seed was overwritten")
	}
}

func TestEscapedWalkersAreRecycledOntoTheRing(t *testing.T) {
	// The kill radius is the reason this runs at all. A walker past it is put
	// back on the release circle rather than followed home.
	f := New(3)
	f.Resize(tw, th)
	p := &f.walkers[0]
	p.x, p.y = f.cx+1000, f.cy
	f.step()
	d := math.Hypot(float64(p.x-f.cx), float64(p.y-f.cy))
	// One lattice step of slack, because the walker takes its step before the
	// distance is judged.
	if d > f.spawnRadius()+1.5 {
		t.Errorf("a walker 1000 pixels out is still %.1f pixels out after a step "+
			"with a release radius of %.1f", d, f.spawnRadius())
	}
}

func TestTheClusterGrowsOutwardAndStartsAgainWhenItFillsTheWindow(t *testing.T) {
	f := New(4)
	f.Resize(tw, th)
	var restarts int
	last := f.stuck
	lastR := f.maxR
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 4000; i++ {
		f.Frame(s, dt)
		switch {
		case f.stuck < last:
			// The only way the count goes down is a fresh seed.
			restarts++
			if f.stuck != 1 || f.maxR != 0 {
				t.Fatalf("after a restart there are %d particles at radius %.1f, want 1 at 0",
					f.stuck, f.maxR)
			}
		case f.maxR < lastR:
			t.Fatalf("the cluster shrank from %.1f to %.1f without restarting", lastR, f.maxR)
		}
		last, lastR = f.stuck, f.maxR
	}
	if restarts == 0 {
		t.Error("the crystal never filled the window in a hundred seconds; " +
			"a finished picture is a screenshot, not an animation")
	}
}

func TestNewIceIsBrighterThanOld(t *testing.T) {
	// Arrival order is the color, which is what makes the growth history
	// readable off a still frame.
	lum := func(c tcell.Color) int {
		r, g, b := c.RGB()
		return int(r + g + b)
	}
	f := New(5)
	f.ShowWalkers = false
	f.Resize(tw, th)
	s := grow(f, 5)
	newest, at := 0, -1
	for i, g := range f.grid {
		if int(g) > newest {
			newest, at = int(g), i
		}
	}
	if at < 0 {
		t.Fatal("nothing froze")
	}
	tip := lum(s.At(at%f.w, at/f.w))
	old := lum(s.At(f.cx, f.cy))
	if tip <= old {
		t.Errorf("the newest ice draws at %d and the seed at %d: arrival order is not visible",
			tip, old)
	}
	if old == 0 {
		t.Error("the oldest ice faded to nothing: the buried history is lost")
	}
}

func TestGrowthRateIsFrameRateIndependent(t *testing.T) {
	// Two seconds of wall clock at three frame rates. Walkers take
	// StepsPerSecond steps a second in all of them, so the crystal is the same
	// size however often it was drawn.
	stepsOver := func(frames int, step float64) int {
		f := New(6)
		f.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			f.Frame(s, step)
		}
		return f.steps
	}
	slow := stepsOver(60, 1.0/30)
	fast := stepsOver(120, 1.0/60)
	odd := stepsOver(34, 1.0/17)

	// Derived from the configured rate rather than written out. The number
	// used to be a literal 5200, which was two seconds of the rate this
	// shipped with, so raising the default made a test of frame-rate
	// independence fail for having changed the frame rate of nothing.
	want := int(2 * New(6).StepsPerSecond)
	lo, hi := want-want/50, want+want/50
	if slow < lo || slow > hi {
		t.Fatalf("two seconds at 30fps ran %d steps, want about %d", slow, want)
	}
	for _, got := range []int{fast, odd} {
		if d := got - slow; d < -20 || d > 20 {
			t.Fatalf("same two seconds ran %d steps at one frame rate and %d at another", slow, got)
		}
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) []uint32 {
		f := New(seed)
		f.Resize(tw, th)
		grow(f, 4)
		out := make([]uint32, len(f.grid))
		copy(out, f.grid)
		return out
	}
	a, b := run(0), run(0)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at index %d", i)
		}
	}
	c := run(7)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds grew an identical crystal")
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	// Resize can be handed a size where the cluster has filled the window
	// before it has begun, which puts a restart on the first freeze.
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {3, 3}, {8, 6}} {
		f := New(8)
		f.Resize(sz[0], sz[1])
		s := canvas.NewSurface(max(sz[0], 1), max(sz[1], 1))
		for i := 0; i < 30; i++ {
			f.Frame(s, dt)
		}
	}
}

func TestFrameDoesNotAllocate(t *testing.T) {
	// The grid and the walkers are sized in Resize, and a restart reuses the
	// grid in place rather than allocating a new one.
	f := New(9)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { f.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
