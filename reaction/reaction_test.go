package reaction

import (
	"math"
	"testing"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 48

// dt is one frame of the thirty-a-second loop these constants were tuned at.
const dt = 1.0 / 30

// steps runs n reaction steps directly, without the frame accounting, so a
// test of the chemistry says what it means in the model's own units.
func steps(r *Reaction, n int) {
	for i := 0; i < n; i++ {
		r.step()
	}
}

// bare returns a reaction whose surface is pure substrate: U everywhere, no V
// anywhere, and nothing seeded into it.
func bare(w, h int) *Reaction {
	r := New(1)
	r.Resize(w, h)
	for i := range r.u {
		r.u[i], r.v[i] = 1, 0
	}
	return r
}

// oneSpot is bare with a single clean disc of V at the middle. Clean rather
// than the noisy patch Resize makes, so that a test can talk about its center
// and its rim.
func oneSpot(w, h int, rad float64) *Reaction {
	r := bare(w, h)
	cx, cy := w/2, h/2
	ir := int(rad)
	for dy := -ir; dy <= ir; dy++ {
		for dx := -ir; dx <= ir; dx++ {
			if float64(dx*dx+dy*dy) > rad*rad {
				continue
			}
			i := (cy+dy)*w + cx + dx
			r.u[i], r.v[i] = 0.5, 0.25
		}
	}
	return r
}

// Bare substrate is a steady state of the model: U is fed in as fast as it is
// consumed and there is no V to catalyze anything. If it drifts, one of the
// two rate terms is wrong, and every pattern above it would be wrong with it.
func TestBareSubstrateStaysBare(t *testing.T) {
	r := bare(32, 32)
	steps(r, 500)
	for i := range r.u {
		if math.Abs(float64(r.u[i])-1) > 1e-6 || r.v[i] != 0 {
			t.Fatalf("cell %d drifted to u=%v v=%v on an empty surface", i, r.u[i], r.v[i])
		}
	}
}

// A patch of V must grow at its rim and starve in its middle. That is the
// whole mechanism — U reaches the edge of a patch and not its center — and it
// is what makes spots divide rather than simply swell.
func TestASpotStarvesInTheMiddleAndGrowsAtTheRim(t *testing.T) {
	const w, h = 64, 64
	r := oneSpot(w, h, 6)
	steps(r, 4000)

	center := r.v[(h/2)*w+w/2]
	// The brightest cell anywhere on a ring well outside where the patch
	// started: if the front did not move, this is still bare.
	var rim float32
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := math.Hypot(float64(x-w/2), float64(y-h/2))
			if d > 9 && d < 14 && r.v[y*w+x] > rim {
				rim = r.v[y*w+x]
			}
		}
	}
	if rim < 0.1 {
		t.Fatalf("nothing reached the ring outside the seed: peak V there is %v", rim)
	}
	if center >= rim {
		t.Errorf("V at the middle is %v against %v at the rim: the spot swelled instead of hollowing out",
			center, rim)
	}
}

// The pattern has to keep spreading until it runs out of room. A regime that
// merely holds the seed it was given is a still image.
func TestThePatternSpreadsAcrossTheSurface(t *testing.T) {
	r := oneSpot(48, 48, 5)
	lit := func() int {
		var n int
		for _, v := range r.v {
			if v > 0.15 {
				n++
			}
		}
		return n
	}
	start := lit()
	steps(r, 20000)
	if end := lit(); end < start*4 {
		t.Errorf("the front covered %d cells after twenty thousand steps, from %d: it is not spreading",
			end, start)
	}
}

// The surface is a torus. Without the wrap a growing pattern would pile up
// against four invisible walls, and the fronts that ought to be the most
// interesting part of the picture would be the parts stuck at the edge.
func TestThePatternWrapsAroundTheEdge(t *testing.T) {
	const w, h = 40, 40
	r := bare(w, h)
	// A patch straddling the left edge. If the Laplacian did not wrap, the
	// right-hand column could never learn about it.
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			if dx*dx+dy*dy > 16 {
				continue
			}
			r.u[((h/2+dy)*w+(dx+w)%w)], r.v[((h/2+dy)*w+(dx+w)%w)] = 0.5, 0.25
		}
	}
	steps(r, 3000)
	var far float32
	for y := 0; y < h; y++ {
		if v := r.v[y*w+w-1]; v > far {
			far = v
		}
	}
	if far < 0.05 {
		t.Errorf("the far column peaks at %v: the surface is not a torus", far)
	}
}

// Draining V faster than the reaction can make it must kill the pattern. This
// is the boundary of the regime the defaults sit inside, and it is asserted as
// a comparison against the same run with the default rate because the absolute
// numbers depend on the seed.
func TestTooMuchKillStarvesThePattern(t *testing.T) {
	run := func(kill float64) float64 {
		r := oneSpot(48, 48, 5)
		r.Kill = kill
		steps(r, 6000)
		return total(r.v)
	}
	alive, dead := run(0.057), run(0.075)
	if dead >= alive*0.01 {
		t.Errorf("V totals %.4g at kill 0.075 against %.4g at 0.057: the pattern did not starve", dead, alive)
	}
}

// The explicit step has a hard stability limit rather than a soft one: past it
// the grid-scale mode grows every step and the screen fills with hash in
// seconds. step clamps, so the knob at its maximum must still leave a field
// with concentrations in it.
func TestDiffusionIsClampedToStability(t *testing.T) {
	r := oneSpot(48, 48, 5)
	r.DiffU, r.DiffV = 20, 10 // far past 1.25
	steps(r, 2000)
	for i := range r.v {
		if math.IsNaN(float64(r.v[i])) || math.IsInf(float64(r.v[i]), 0) {
			t.Fatalf("cell %d is %v: the diffusion clamp did not hold", i, r.v[i])
		}
		if r.v[i] > 2 || r.u[i] > 2 {
			t.Fatalf("cell %d reached u=%v v=%v: bounded, but growing without limit", i, r.u[i], r.v[i])
		}
	}
}

// The regime the defaults were chosen for is the one that never finishes.
// Long after the surface has filled, fronts must still be moving through it —
// this is the property that ruled out the rounder mitosis parameters, and it
// is the reason the package looks worth leaving on.
func TestItKeepsChangingLongAfterItFills(t *testing.T) {
	r := New(1)
	r.Resize(48, 48)
	steps(r, 20000) // long enough for the pattern to have filled the surface

	before := make([]float32, len(r.v))
	copy(before, r.v)
	steps(r, 9000) // another thirty seconds at the default rate

	var flipped int
	for i, v := range r.v {
		if (v > 0.15) != (before[i] > 0.15) {
			flipped++
		}
	}
	// A tenth of the surface is far below what the defaults measure and far
	// above the settled regimes, so this catches a change of parameters into
	// one that stops without failing on the run-to-run wobble of one that
	// does not.
	if min := len(r.v) / 10; flipped < min {
		t.Errorf("only %d of %d cells changed state in thirty seconds: the pattern has settled",
			flipped, len(r.v))
	}
}

// The same wall-clock second must run the same number of reaction steps
// however many frames it was divided into. Everything here is driven by whole
// steps, so this is exact rather than approximate.
func TestSameElapsedTimeGivesTheSameChemistry(t *testing.T) {
	run := func(frames int, step float64) []float32 {
		r := New(4)
		r.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			r.Frame(s, step)
		}
		out := make([]float32, len(r.v))
		copy(out, r.v)
		return out
	}
	slow := run(30, 1.0/30)
	fast := run(60, 1.0/60)
	for i := range slow {
		if slow[i] != fast[i] {
			t.Fatalf("cell %d holds %v at 30fps and %v at 60fps: the chemistry follows the frame count",
				i, slow[i], fast[i])
		}
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []float32 {
		r := New(9)
		r.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 60; i++ {
			r.Frame(s, dt)
		}
		out := make([]float32, len(r.v))
		copy(out, r.v)
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at cell %d", i)
		}
	}
}

func TestOddSizesDoNotPanic(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 9}, {9, 2}, {3, 3}, {120, 80}} {
		r := New(1)
		r.Resize(sz[0], sz[1])
		s := canvas.NewSurface(sz[0], sz[1])
		for i := 0; i < 3; i++ {
			r.Frame(s, dt)
		}
	}
	// A Frame before any Resize must be a no-op rather than a nil dereference:
	// callers other than canvas.Run may not follow the contract.
	New(1).Frame(canvas.NewSurface(4, 4), dt)
}

func TestFrameDoesNotAllocate(t *testing.T) {
	r := New(1)
	r.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	r.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { r.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per run; buffers belong in Resize", n)
	}
}
