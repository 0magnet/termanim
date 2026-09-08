package flow

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 48

// dt is one frame of the thirty-a-second loop these constants were tuned at.
const dt = 1.0 / 30

func sized(seed int64) *Flow {
	f := New(seed)
	f.Resize(tw, th)
	return f
}

// The reason for the whole curl construction: a field with sources and sinks
// in it drains every particle into a handful of clots, and the screen ends up
// as a few bright clumps with nothing moving between them.
//
// Measured two ways. On the stencil the velocity is itself built from, the
// four cross terms cancel term by term and the divergence is zero to the last
// bit — that is the identity, and it holds whatever the noise underneath does.
// Sampled at a coarser step it is not exactly zero, because a difference over
// a larger interval carries its own truncation error; what it must be there is
// tiny against the flow it is a divergence of.
func TestTheFieldIsDivergenceFree(t *testing.T) {
	f := sized(1)
	// The pixel distance that is exactly eps in the noise coordinates the
	// stream function is differenced in.
	same := eps / (f.Scale / float64(min(tw, th)))

	check := func(h, tol float64, what string) {
		var worst, scale float64
		for y := 6.0; y < th-6; y += 3.7 {
			for x := 6.0; x < tw-6; x += 4.3 {
				vxp, _ := f.velocity(x+h, y)
				vxm, _ := f.velocity(x-h, y)
				_, vyp := f.velocity(x, y+h)
				_, vym := f.velocity(x, y-h)
				div := (vxp-vxm)/(2*h) + (vyp-vym)/(2*h)
				if a := math.Abs(div); a > worst {
					worst = a
				}
				vx, vy := f.velocity(x, y)
				if m := math.Hypot(vx, vy); m > scale {
					scale = m
				}
			}
		}
		if scale == 0 {
			t.Fatal("the field is everywhere zero; there is nothing to be divergence free about")
		}
		if worst > scale*tol {
			t.Errorf("%s: divergence reaches %.3g against a flow of %.3g", what, worst, scale)
		}
	}
	check(same, 1e-9, "on its own stencil")
	check(0.5, 5e-3, "sampled half a pixel apart")
}

// A flow field is only a flow field if it is smooth. Two points a fraction of
// a pixel apart must give nearly the same velocity, or a particle's path is
// noise rather than a streamline — and the field must not be constant either,
// or there is one direction and no field.
func TestTheFieldIsSmoothButNotUniform(t *testing.T) {
	f := sized(2)
	var maxJump, spread float64
	var first [2]float64
	for y := 2.0; y < th-2; y += 1.1 {
		for x := 2.0; x < tw-2; x += 1.3 {
			ax, ay := f.velocity(x, y)
			bx, by := f.velocity(x+0.25, y)
			if d := math.Hypot(bx-ax, by-ay); d > maxJump {
				maxJump = d
			}
			if first == [2]float64{} {
				first = [2]float64{ax, ay}
			}
			if d := math.Hypot(ax-first[0], ay-first[1]); d > spread {
				spread = d
			}
		}
	}
	if maxJump > spread/8 {
		t.Errorf("a quarter pixel changes the flow by %.4f where the whole field spans %.4f: this is noise, not a field",
			maxJump, spread)
	}
	if spread < 1e-6 {
		t.Error("the field is the same everywhere")
	}
}

// The noise has to come out of the seed and nowhere else: the same seed is the
// same field, and a different seed is a different one rather than the same
// field shifted.
func TestNoiseIsSeededAndReproducible(t *testing.T) {
	a, b, c := New(7), New(7), New(8)
	var same, differ int
	for i := 0; i < 200; i++ {
		x, y := float64(i)*0.37, float64(i)*0.11
		if a.noise(x, y, 0) != b.noise(x, y, 0) {
			t.Fatalf("seed 7 gave two different values at %.2f,%.2f", x, y)
		}
		same++
		if math.Abs(a.noise(x, y, 0)-c.noise(x, y, 0)) > 1e-9 {
			differ++
		}
	}
	if differ < same*9/10 {
		t.Errorf("only %d of %d samples differ between seeds 7 and 8", differ, same)
	}
}

// Value noise must stay in its range and use it. A hash that clustered would
// give a stream function with no gradient in it over most of the plane, and
// the particles would sit still.
func TestNoiseUsesItsWholeRange(t *testing.T) {
	f := New(3)
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := 0; i < 4000; i++ {
		v := f.noise(float64(i)*1.37, float64(i%97)*0.91, float64(i%13)*0.33)
		if v < -1 || v > 1 {
			t.Fatalf("noise returned %v, outside -1..1", v)
		}
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if lo > -0.8 || hi < 0.8 {
		t.Errorf("noise only spans %.3f..%.3f: the lattice values are clustered", lo, hi)
	}
}

// Nothing may leave the surface. A particle that walked off would stop
// depositing and be lost, and the field would thin out from the edges inward.
func TestParticlesStayOnTheSurface(t *testing.T) {
	f := sized(5)
	f.Speed = 400 // far faster than the default, to push at the edges hard
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 400; i++ {
		f.Frame(s, dt)
		for j := range f.parts {
			p := f.parts[j]
			if p.x < 0 || p.x >= tw || p.y < 0 || p.y >= th {
				t.Fatalf("frame %d: particle %d reached %.2f,%.2f on a %dx%d surface", i, j, p.x, p.y, tw, th)
			}
		}
	}
}

// The trail is the picture. It has to decay by elapsed time — a per-frame
// multiplier would hold trails twice as long on a machine drawing twice as
// often — and it has to reach the floor rather than leaving a permanent stain.
func TestTrailsFadeByElapsedTime(t *testing.T) {
	f := sized(6)
	f.Density = 0
	f.Resize(tw, th) // one particle, deliberately, so the rest of the field is left alone
	f.parts = f.parts[:0]
	for i := range f.trail {
		f.trail[i] = 1
	}
	s := canvas.NewSurface(tw, th)
	f.Frame(s, f.TrailLife) // exactly one time constant

	want := float32(math.Exp(-1))
	got := f.trail[0]
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("after one time constant a trail is %.4f, want %.4f", got, want)
	}

	for i := 0; i < 40; i++ {
		f.Frame(s, f.TrailLife)
	}
	if v := float64(f.trail[0]); v > f.Floor {
		t.Errorf("a trail left alone for forty time constants is still %.5f, above the floor %.5f", v, f.Floor)
	}
	if c := s.At(0, 0); c != tcell.ColorDefault {
		t.Errorf("a faded pixel is drawn as %v rather than left as the terminal's background", c)
	}
}

// One particle must draw a line rather than a dot: the trail is what makes a
// smooth field legible at this resolution, and a fade that outran the motion
// would leave nothing behind.
func TestOneParticleLeavesAStreak(t *testing.T) {
	f := sized(7)
	f.MaxAge = 1e9
	f.parts = f.parts[:1]
	f.parts[0] = particle{x: tw / 2, y: th / 2}
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 30; i++ {
		f.Frame(s, dt)
	}
	var lit int
	for _, v := range f.trail {
		if float64(v) > f.Floor {
			lit++
		}
	}
	if lit < 5 {
		t.Errorf("one particle lit %d pixels in a second: that is a dot, not a trail", lit)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []particle {
		f := New(9)
		f.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 120; i++ {
			f.Frame(s, dt)
		}
		out := make([]particle, len(f.parts))
		copy(out, f.parts)
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at particle %d", i)
		}
	}
}

func TestDensityScalesWithTheSurface(t *testing.T) {
	small, big := New(1), New(1)
	small.Resize(40, 20)
	big.Resize(160, 80)
	if len(big.parts) <= len(small.parts) {
		t.Errorf("%d particles in a big window against %d in a small one", len(big.parts), len(small.parts))
	}
}

func TestOddSizesDoNotPanic(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 9}, {9, 2}, {200, 120}} {
		f := New(1)
		f.Resize(sz[0], sz[1])
		s := canvas.NewSurface(sz[0], sz[1])
		for i := 0; i < 5; i++ {
			f.Frame(s, dt)
		}
	}
	// A Frame before any Resize must be a no-op rather than a nil dereference.
	New(1).Frame(canvas.NewSurface(4, 4), dt)
}

func TestFrameDoesNotAllocate(t *testing.T) {
	f := sized(1)
	s := canvas.NewSurface(tw, th)
	f.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { f.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per run; buffers belong in Resize", n)
	}
}

// A particle must go where the field points. This is the only thing tying the
// picture to the physics above it.
//
// The drift is turned off for the comparison: Frame advances the field before
// it moves anything, so with the field in motion the velocity sampled here and
// the one the particle was actually carried by are a frame apart. They agree to
// six places even then, but the point of this test is exactness.
func TestParticlesFollowTheField(t *testing.T) {
	f := sized(4)
	f.MaxAge = 1e9 // nothing may be recycled mid-test
	f.Drift = 0
	p := &f.parts[0]
	x0, y0 := p.x, p.y
	vx, vy := f.velocity(x0, y0)

	s := canvas.NewSurface(tw, th)
	f.Frame(s, dt)

	dx, dy := p.x-x0, p.y-y0
	wantX, wantY := vx*f.Speed*dt, vy*f.Speed*dt
	if math.Hypot(dx-wantX, dy-wantY) > 1e-12 {
		t.Errorf("moved by %.6f,%.6f where the field says %.6f,%.6f", dx, dy, wantX, wantY)
	}
}

// travel runs one particle for the given number of frames and reports how far
// it went and where it ended up.
func travel(speed float64, frames int, step float64) (dist, x, y float64) {
	f := New(8)
	f.MaxAge = 1e9
	f.Speed = speed
	f.Resize(tw, th)
	f.parts = f.parts[:1]
	f.parts[0] = particle{x: 20, y: 20}
	s := canvas.NewSurface(tw, th)
	px, py := 20.0, 20.0
	for i := 0; i < frames; i++ {
		f.Frame(s, step)
		// A wrap looks like a jump across the whole surface; that is not
		// distance traveled.
		if d := math.Hypot(f.parts[0].x-px, f.parts[0].y-py); d < tw/2 {
			dist += d
		}
		px, py = f.parts[0].x, f.parts[0].y
	}
	return dist, px, py
}

// A second of wall clock must carry a particle a second's worth of field,
// however many frames it was divided into.
//
// Measured twice, because there are two different claims here and only one of
// them can be made exactly. Advection is forward Euler on a curved streamline,
// so a coarser step cuts every corner and lands somewhere slightly off the
// path — and since the flow is faster in some places than others, being off
// the path also changes the speed. At a step small enough for the integrator
// not to be the variable, the two frame rates end up in the same place. At the
// default speed they do not, and what has to hold there is only that the same
// three seconds covers about the same ground: a motion that followed the frame
// count instead of the clock would cover twice as much.
func TestSameElapsedTimeCarriesTheSameDistance(t *testing.T) {
	slow, sx, sy := travel(4, 90, 1.0/30)  // three seconds
	fast, fx, fy := travel(4, 180, 1.0/60) // the same three seconds
	if slow == 0 {
		t.Fatal("the particle did not move at all")
	}
	if d := math.Hypot(sx-fx, sy-fy); d > 0.5 {
		t.Errorf("at a small step, three seconds ends %.3f pixels apart at the two frame rates", d)
	}

	slow, _, _ = travel(0, 90, 1.0/30)
	if slow != 0 {
		t.Fatalf("a particle at speed zero traveled %.3f pixels", slow)
	}
	slow, _, _ = travel(34, 90, 1.0/30)
	fast, _, _ = travel(34, 180, 1.0/60)
	if ratio := fast / slow; ratio < 0.75 || ratio > 1.25 {
		t.Errorf("three seconds covers %.2f pixels at 30fps and %.2f at 60fps", slow, fast)
	}
}
