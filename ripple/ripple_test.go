package ripple

import (
	"math"
	"testing"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 48

// dt is one frame of the thirty-a-second loop these constants were tuned at,
// so a count of frames here means the same amount of water as it always did.
const dt = 1.0 / 30

// still returns a tank with the rain turned off, so a test can place one drop
// and watch exactly what the medium does with it.
func still(w, h int) *Ripple {
	r := New(1)
	r.DropRate = 0
	r.Resize(w, h)
	return r
}

// A drop must travel. A field that keeps its dent where it was made is a
// diffusion, not a wave, and would sit there as a static dimple.
func TestADropRadiates(t *testing.T) {
	r := still(64, 64)
	r.drop(32, 32, 3, -1)

	before := math.Abs(float64(r.height(38, 32)))
	for i := 0; i < 20; i++ {
		r.step()
	}
	after := math.Abs(float64(r.height(38, 32)))

	if after <= before {
		t.Errorf("no wave reached six cells out: %.5f before, %.5f after", before, after)
	}
}

// Damping below 1 must actually settle the tank. A medium that rings forever
// silts up with standing waves and no individual drop can be made out in it.
func TestDampingSettlesTheTank(t *testing.T) {
	r := still(48, 48)
	r.Damping = 0.9
	r.drop(24, 24, 3, -1)

	start := r.energy()
	for i := 0; i < 400; i++ {
		r.step()
	}
	if end := r.energy(); end >= start*0.01 {
		t.Errorf("energy %.6g did not decay from %.6g", end, start)
	}
}

// The CFL limit is a cliff rather than a slope: past it the integration does
// not degrade, it goes to NaN and the screen goes black. step clamps, so the
// knob at its maximum must still leave a finite field.
func TestSpeedIsClampedToStability(t *testing.T) {
	r := still(48, 48)
	r.Speed = 10 // far past sqrt(0.5)
	r.Damping = 1
	r.drop(24, 24, 2, -1)

	for i := 0; i < 500; i++ {
		r.step()
	}
	for i, v := range r.cur {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("cell %d is %v: the speed clamp did not hold", i, v)
		}
	}
	if e := r.energy(); e > 1e6 {
		t.Errorf("energy %.6g — bounded, but growing without limit", e)
	}
}

// Reflect is the knob the tank exists for: walls keep the rings and open water
// lets them go. Asserted as a comparison rather than against a threshold,
// because the absolute energy depends on the drop and the damping while the
// ordering does not.
func TestReflectControlsWhetherWavesReturn(t *testing.T) {
	run := func(reflect float64) float64 {
		r := still(64, 64)
		r.Reflect = reflect
		r.Damping = 1 // isolate the boundary from the decay
		r.Spread = 0
		r.drop(32, 32, 3, -1)
		// Long enough for the front to reach a wall and, if it can, return.
		for i := 0; i < 200; i++ {
			r.step()
		}
		return r.energy()
	}
	open, wall := run(0), run(1)
	if open >= wall {
		t.Errorf("absorbing walls kept %.6g and reflecting ones %.6g: the wave never left", open, wall)
	}
	if open > wall*0.5 {
		t.Errorf("absorbing walls kept %.6g of %.6g: more than half came back", open, wall)
	}
}

// Spread is what separates a liquid from a drum head: it should take the
// finest ripples out and leave the coarse ones alone.
func TestSpreadRemovesTheFinestRipples(t *testing.T) {
	fine := func(spread float64) float64 {
		const n = 64
		r := still(n, n)
		r.Spread = spread
		r.Damping = 1
		// Alternating cells: the shortest wavelength the grid can hold.
		for y := 1; y < n-1; y++ {
			for x := 1; x < n-1; x++ {
				if (x+y)%2 == 0 {
					r.cur[y*n+x] = 1
				} else {
					r.cur[y*n+x] = -1
				}
			}
		}
		for i := 0; i < 12; i++ {
			r.step()
		}
		return r.energy()
	}
	if with, without := fine(0.5), fine(0); with >= without {
		t.Errorf("spread kept the grid-scale ripple: %.4g with it, %.4g without", with, without)
	}
}

// Still water is the middle of the ramp and covers the whole surface, so an
// untouched tank reads as water rather than as an empty screen.
func TestStillWaterIsFlatAndCoversTheSurface(t *testing.T) {
	r := still(tw, th)
	s := canvas.NewSurface(tw, th)
	r.Frame(s, 0) // no elapsed time, so no step and no rain
	want := r.Palette[128]
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if got := s.At(x, y); got != want {
				t.Fatalf("still water at %d,%d is %v, want the middle of the ramp %v", x, y, got, want)
			}
		}
	}
}

// The rain has to fall by itself: this is a tank left out in the weather, not
// one waiting for a pointer.
func TestRainFallsWithoutBeingAsked(t *testing.T) {
	r := New(2)
	r.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 120; i++ { // four seconds
		r.Frame(s, dt)
	}
	if r.energy() == 0 {
		t.Fatal("four seconds passed and the surface is still flat")
	}

	// And it must keep falling rather than being a single opening splash:
	// with damping this strong, a tank left alone for another ten seconds
	// would be nearly flat again.
	before := r.energy()
	for i := 0; i < 300; i++ {
		r.Frame(s, dt)
	}
	if r.energy() < before*0.1 {
		t.Errorf("energy fell from %.4g to %.4g: the rain stopped", before, r.energy())
	}
}

// More surface catches more rain. A rate quoted per second whatever the window
// is a downpour in a small one and a drizzle in a large one.
func TestRainScalesWithTheSurface(t *testing.T) {
	count := func(w, h int) int {
		r := New(3)
		r.Resize(w, h)
		var n int
		for i := 0; i < 300; i++ {
			before := r.drops
			r.rain(1.0 / 60)
			if r.drops < before {
				n++
			}
		}
		return n
	}
	small, big := count(40, 24), count(200, 120)
	if big <= small {
		t.Errorf("%d drops on a big surface against %d on a small one", big, small)
	}
}

// The same wall-clock second must leave the same water however many frames it
// was divided into. Everything here is driven by whole simulation steps, so
// this is exact rather than approximate.
func TestSameElapsedTimeGivesTheSameWater(t *testing.T) {
	run := func(frames int, step float64) []float32 {
		r := New(4)
		r.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			r.Frame(s, step)
		}
		out := make([]float32, len(r.cur))
		copy(out, r.cur)
		return out
	}
	slow := run(60, 1.0/30)  // two seconds
	fast := run(120, 1.0/60) // the same two seconds
	for i := range slow {
		if slow[i] != fast[i] {
			t.Fatalf("cell %d is %v at 30fps and %v at 60fps: the water follows the frame count",
				i, slow[i], fast[i])
		}
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []float32 {
		r := New(9)
		r.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 200; i++ {
			r.Frame(s, dt)
		}
		out := make([]float32, len(r.cur))
		copy(out, r.cur)
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
	for _, sz := range [][2]int{{1, 1}, {2, 9}, {9, 2}, {5, 5}, {200, 120}} {
		r := New(1)
		r.Resize(sz[0], sz[1])
		s := canvas.NewSurface(sz[0], sz[1])
		for i := 0; i < 5; i++ {
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
