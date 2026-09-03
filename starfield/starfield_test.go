package starfield

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 48, 32

// dt is one frame at the rate the constants were originally tuned at, so the
// distances these tests assert over a count of frames stay meaningful.
const dt = 1.0 / 30

// The one thing that separates this from random dots drifting about: a star's
// distance from the vanishing point must grow every frame, for every star that
// was not recycled.
func TestStarsMoveOutwardFromCentre(t *testing.T) {
	f := New(1)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)

	radius := func(st star) float64 {
		x, y := f.project(st)
		return math.Hypot(x-float64(tw)/2, y-float64(th)/2)
	}

	for frame := 0; frame < 200; frame++ {
		before := make([]star, len(f.stars))
		copy(before, f.stars)
		f.Frame(s, dt)
		for i, st := range f.stars {
			if st.x != before[i].x || st.y != before[i].y {
				// Recycled this frame; its old radius says nothing.
				continue
			}
			if radius(st) <= radius(before[i]) {
				t.Fatalf("frame %d: star %d moved inward, %.3f -> %.3f",
					frame, i, radius(before[i]), radius(st))
			}
		}
	}
}

// Stars must be recycled rather than escaping forever, or the sky empties out.
func TestStarsAreRecycledNotLost(t *testing.T) {
	f := New(2)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	n := len(f.stars)

	for i := 0; i < 1000; i++ {
		f.Frame(s, dt)
		if len(f.stars) != n {
			t.Fatalf("frame %d: star count changed from %d to %d", i, n, len(f.stars))
		}
	}
	for i, st := range f.stars {
		if st.z <= 0 || st.z > zFar {
			t.Fatalf("star %d left the depth range at z=%.4f", i, st.z)
		}
		if math.Abs(st.x) > 1 || math.Abs(st.y) > 1 {
			t.Fatalf("star %d respawned outside the field at %.3f,%.3f", i, st.x, st.y)
		}
	}
}

// After long enough for the whole initial set to have crossed the range, the
// sky must still be lit — proof the recycling actually puts stars back rather
// than parking them somewhere invisible.
func TestSkyStaysPopulated(t *testing.T) {
	f := New(3)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)

	count := func() int {
		n := 0
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				if s.At(x, y) != tcell.ColorDefault {
					n++
				}
			}
		}
		return n
	}

	f.Frame(s, dt)
	first := count()
	if first == 0 {
		t.Fatal("nothing drawn on the first frame")
	}
	for i := 0; i < 600; i++ {
		f.Frame(s, dt)
	}
	later := count()
	if later < first/2 {
		t.Errorf("sky thinned out: %d lit pixels became %d", first, later)
	}
}

// The field must sit on the terminal's own background, not on a black
// rectangle, and it must not smear: every frame starts from a clear surface.
func TestBackgroundIsLeftAlone(t *testing.T) {
	f := New(4)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.Frame(s, dt)
	lit := 0
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Fatal("no stars drawn")
	}
	if lit > tw*th/2 {
		t.Errorf("%d of %d pixels lit: the sky is not a sky", lit, tw*th)
	}
}

// Nearer stars must be brighter, or there is no depth cue at all.
func TestNearerStarsAreBrighter(t *testing.T) {
	f := New(5)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)

	// Two stars on the same bearing, one twice as far as the other.
	f.stars = f.stars[:2]
	f.stars[0] = star{x: 0.02, y: 0, z: 0.9}
	f.stars[1] = star{x: 0.02, y: 0, z: 0.2}
	f.Frame(s, dt)

	sum := func(st star) int {
		x, y := f.project(st)
		r, g, b := s.At(int(x), int(y)).RGB()
		return int(r + g + b)
	}
	far, near := sum(f.stars[0]), sum(f.stars[1])
	if near <= far {
		t.Errorf("near star brightness %d is not above far star %d", near, far)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) []star {
		f := New(seed)
		f.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 120; i++ {
			f.Frame(s, dt)
		}
		out := make([]star, len(f.stars))
		copy(out, f.stars)
		return out
	}
	a, b := run(0), run(0)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("seed 0 diverged at star %d: %+v vs %+v", i, a[i], b[i])
		}
	}
	if c := run(7); len(c) == len(a) {
		same := true
		for i := range a {
			if a[i] != c[i] {
				same = false
				break
			}
		}
		if same {
			t.Error("a different seed produced an identical field")
		}
	}
}

// Density has to be in pixels, not in stars, or the effect falls apart at
// window sizes other than the one it was tuned at.
func TestCountFollowsSurfaceArea(t *testing.T) {
	small, big := New(1), New(1)
	small.Resize(tw, th)
	big.Resize(tw*2, th*2)
	if len(big.stars) <= len(small.stars) {
		t.Errorf("a four times larger surface got %d stars, not more than %d",
			len(big.stars), len(small.stars))
	}
}

// Thirty frames a second in a browser leaves no room for the garbage
// collector; the star set is allocated in Resize and Frame must touch nothing.
func TestFrameDoesNotAllocate(t *testing.T) {
	f := New(1)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	if n := testing.AllocsPerRun(50, func() { f.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}

// The whole point of taking elapsed seconds rather than counting frames: the
// field must travel the same distance in the same wall-clock time whether the
// machine managed sixty frames or thirty. A star is placed by hand so that
// recycling, which consumes random numbers, cannot make the two runs diverge
// for reasons that have nothing to do with the rate.
func TestFrameRateIndependent(t *testing.T) {
	advance := func(steps int, step float64) float64 {
		f := New(1)
		f.Resize(tw, th)
		f.stars = f.stars[:1]
		f.stars[0] = star{x: 0.1, y: 0.1, z: zFar}
		s := canvas.NewSurface(tw, th)
		for i := 0; i < steps; i++ {
			f.Frame(s, step)
		}
		return f.stars[0].z
	}
	slow := advance(51, 1.0/30) // 1.7 seconds at thirty frames a second
	fast := advance(102, 1.0/60)
	if slow >= zFar {
		t.Fatalf("the star did not move at all: z=%.4f", slow)
	}
	if d := slow - fast; d < -1e-9 || d > 1e-9 {
		t.Errorf("1.7s of travel gave z=%.6f at 30fps and z=%.6f at 60fps", slow, fast)
	}
	// And it must be the distance the speed promises, not merely a consistent
	// one: baseSpeed is depth per second.
	want := zFar - baseSpeed*1.7
	if d := slow - want; d < -1e-6 || d > 1e-6 {
		t.Errorf("after 1.7s z=%.6f, want %.6f", slow, want)
	}
}
