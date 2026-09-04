package boids

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 96, 64

// dt is the step every test takes unless it is testing the step itself. It is
// the rate these numbers were tuned at, so the assertions below go on meaning
// what they meant before motion was expressed in seconds.
const dt = 1.0 / 30

func TestBoidsStayOnScreen(t *testing.T) {
	// Wrapping, not clamping: a boid that leaves the right edge has to arrive
	// at the left, and every position has to stay addressable.
	b := New(1)
	b.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 500; i++ {
		b.Frame(s, dt)
		for j := range b.flock {
			p := b.flock[j]
			if p.x < 0 || p.x >= tw || p.y < 0 || p.y >= th {
				t.Fatalf("frame %d: boid %d escaped to %.2f,%.2f", i, j, p.x, p.y)
			}
		}
	}
}

func TestSpeedStaysWithinTheClamp(t *testing.T) {
	// Neither frozen nor runaway: three rules that agree would otherwise
	// accelerate a boid without limit, and three that cancel would stop it.
	b := New(2)
	b.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	const eps = 1e-9
	for i := 0; i < 300; i++ {
		b.Frame(s, dt)
		for j := range b.flock {
			p := b.flock[j]
			sp := math.Hypot(p.vx, p.vy)
			if sp < b.minSpeed-eps || sp > b.maxSpeed+eps {
				t.Fatalf("frame %d: boid %d at speed %.4f, outside [%.4f, %.4f]",
					i, j, sp, b.minSpeed, b.maxSpeed)
			}
		}
	}
}

// order is the length of the mean unit velocity: 1 when every boid flies the
// same way, near 0 when the headings are scattered.
func order(b *Boids) float64 {
	var sx, sy float64
	for i := range b.flock {
		p := b.flock[i]
		ux, uy := unit(p.vx, p.vy)
		sx += ux
		sy += uy
	}
	return math.Hypot(sx, sy) / float64(len(b.flock))
}

// crowding is the average number of neighbours each boid has inside the
// perception radius. It rises as the boids gather, and is not fooled by a flock
// that has split into several groups the way a distance to a global center is.
func crowding(b *Boids) float64 {
	r2 := b.radius * b.radius
	var n int
	for i := range b.flock {
		for j := range b.flock {
			if i == j {
				continue
			}
			dx := wrapDelta(b.flock[j].x-b.flock[i].x, b.w)
			dy := wrapDelta(b.flock[j].y-b.flock[i].y, b.h)
			if dx*dx+dy*dy <= r2 {
				n++
			}
		}
	}
	return float64(n) / float64(len(b.flock))
}

func TestHeadingsAlign(t *testing.T) {
	// Boids start on random headings. If alignment works they end up largely
	// agreeing, which is the difference between a flock and a swarm of gnats.
	b := New(3)
	b.Count = 40
	b.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	b.Frame(s, dt)
	before := order(b)
	for i := 0; i < 600; i++ {
		b.Frame(s, dt)
	}
	after := order(b)
	if after <= before {
		t.Fatalf("headings did not align: order %.3f -> %.3f", before, after)
	}
	if after < 0.5 {
		t.Errorf("flock is still barely aligned after 600 frames: order %.3f", after)
	}
}

func TestBoidsGatherIntoAFlock(t *testing.T) {
	// Cohesion has to beat the uniform scatter it starts from without
	// separation letting the whole flock collapse onto one pixel.
	b := New(4)
	b.Count = 40
	b.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	b.Frame(s, dt)
	before := crowding(b)
	for i := 0; i < 600; i++ {
		b.Frame(s, dt)
	}
	after := crowding(b)
	if after <= before {
		t.Fatalf("boids did not gather: neighbours per boid %.2f -> %.2f", before, after)
	}
}

func TestSeparationKeepsThemApart(t *testing.T) {
	// The other half of the previous test: a flock that has formed must not be
	// a single point. Averaged over the flock, neighbours should sit around the
	// separation distance rather than on top of each other.
	b := New(5)
	b.Count = 40
	b.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 600; i++ {
		b.Frame(s, dt)
	}
	var sum float64
	for i := range b.flock {
		nearest := math.Inf(1)
		for j := range b.flock {
			if i == j {
				continue
			}
			dx := wrapDelta(b.flock[j].x-b.flock[i].x, b.w)
			dy := wrapDelta(b.flock[j].y-b.flock[i].y, b.h)
			if d := math.Hypot(dx, dy); d < nearest {
				nearest = d
			}
		}
		sum += nearest
	}
	avg := sum / float64(len(b.flock))
	if avg < 0.5 {
		t.Errorf("flock collapsed: average nearest neighbour %.3f pixels", avg)
	}
}

func TestDrawsHeadsAndTails(t *testing.T) {
	// A boid is a head and a trail behind it, and the trail is dimmer, which is
	// the only thing that shows which end is the front.
	b := New(6)
	b.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 20; i++ {
		b.Frame(s, dt)
	}
	var lit, dim int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			c := s.At(x, y)
			if c == tcell.ColorDefault {
				continue
			}
			lit++
			r, g, bl := c.RGB()
			if r+g+bl < 200 {
				dim++
			}
		}
	}
	if lit < len(b.flock) {
		t.Errorf("only %d pixels lit for %d boids: tails are missing", lit, len(b.flock))
	}
	if dim == 0 {
		t.Error("nothing is dim: the tails do not fade, so heading is invisible")
	}
	if lit == tw*th {
		t.Error("every pixel is lit: the flock is not floating on the background")
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []boid {
		b := New(9)
		b.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 60; i++ {
			b.Frame(s, dt)
		}
		return b.flock
	}
	a, c := run(), run()
	if len(a) != len(c) {
		t.Fatalf("same seed gave %d boids then %d", len(a), len(c))
	}
	for i := range a {
		if a[i] != c[i] {
			t.Fatalf("same seed diverged at boid %d: %+v vs %+v", i, a[i], c[i])
		}
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	// The grid has fewer than three cells on an axis here, which is the case
	// the wrapping neighbour scan has to special-case, and Resize may be handed
	// a degenerate size before the first frame.
	for _, sz := range [][2]int{{1, 1}, {4, 4}, {8, 6}, {0, 0}} {
		b := New(7)
		b.Resize(sz[0], sz[1])
		s := canvas.NewSurface(max(sz[0], 1), max(sz[1], 1))
		for i := 0; i < 30; i++ {
			b.Frame(s, dt)
		}
	}
}

func TestFrameDoesNotAllocate(t *testing.T) {
	// Thirty frames a second in a browser: the spatial grid and the
	// accelerations are sized in Resize precisely so that Frame does not have
	// to ask the allocator for anything.
	b := New(11)
	b.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	b.Frame(s, dt)
	if n := testing.AllocsPerRun(50, func() { b.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}

func TestFrameRateIndependence(t *testing.T) {
	// Flocking is chaotic, and two runs stepped at different sizes come apart
	// within a few seconds however carefully the rates are converted — they do
	// not even draw the same count of random numbers. So the two things worth
	// asserting are asserted over the windows where each means something: how
	// far the flock flies, checked while the runs are still together, and how
	// flocked it ends up, checked long after they have parted, when it is a
	// statistic rather than a trajectory.
	run := func(step float64, secs int) (aligned, flown float64) {
		b := New(21)
		b.Count = 40
		b.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		prev := make([]boid, len(b.flock))
		var path float64
		for i := 0; i < int(float64(secs)/step); i++ {
			copy(prev, b.flock)
			b.Frame(s, step)
			for j := range b.flock {
				dx := wrapDelta(b.flock[j].x-prev[j].x, b.w)
				dy := wrapDelta(b.flock[j].y-prev[j].y, b.h)
				path += math.Hypot(dx, dy)
			}
		}
		return order(b), path / float64(len(b.flock))
	}

	// Three seconds of flying. This is the searching half: a speed left in
	// per-frame units still reads correctly off the boid, because the speed is
	// the state, while the flock crosses the screen at twice the rate. Only
	// the ground covered per second of simulation catches that.
	_, d30 := run(1.0/30, 3)
	_, d60 := run(1.0/60, 3)
	if math.Abs(d30-d60) > 0.03*d30 {
		t.Errorf("distance flown depends on the frame rate: %.1f px at 30fps, %.1f at 60fps", d30, d60)
	}

	// Thirty seconds, by which time both runs have long since settled into a
	// flock and neither remembers the other.
	a30, _ := run(1.0/30, 30)
	a60, _ := run(1.0/60, 30)
	if math.Abs(a30-a60) > 0.15 {
		t.Errorf("alignment depends on the frame rate: %.3f at 30fps, %.3f at 60fps", a30, a60)
	}
}
