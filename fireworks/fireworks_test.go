package fireworks

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 48

// dt is the step these tests advance by: a thirtieth of a second, so a count of
// frames here means the same amount of flight it always did.
const dt = 1.0 / 30

// quiet returns a display that never launches on its own, so a test can fire
// one shell and watch only that.
func quiet(seed int64) *Fireworks {
	f := New(seed)
	f.LaunchRate = 0
	return f
}

func TestShellRisesThenBursts(t *testing.T) {
	f := quiet(1)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.launch()
	start := f.shells[0].y

	var top float64 = th
	burst := -1
	for i := 0; i < 400 && burst < 0; i++ {
		f.Frame(s, dt)
		if len(f.shells) > 0 {
			if f.shells[0].y > top {
				t.Fatalf("frame %d: the shell sank from %.1f to %.1f before bursting",
					i, top, f.shells[0].y)
			}
			top = f.shells[0].y
		} else {
			burst = i
		}
	}
	if burst < 0 {
		t.Fatal("the shell never burst")
	}
	if top >= start {
		t.Errorf("the shell reached %.1f having started at %.1f: it never rose", top, start)
	}
	if top > th*0.6 {
		t.Errorf("the shell burst at %.1f, still in the bottom of a %d-pixel surface", top, th)
	}
}

func TestParticleCountJumpsAtTheBurst(t *testing.T) {
	f := quiet(2)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.launch()

	// While climbing there is only the trail, a few embers at a time.
	var climbing int
	for len(f.shells) > 0 {
		f.Frame(s, dt)
		if len(f.shells) > 0 {
			climbing = len(f.particles)
		}
	}
	after := len(f.particles)
	if after < climbing+f.burst/2 {
		t.Errorf("particles went from %d while climbing to %d after the burst, "+
			"with a burst size of %d: the shell fizzled", climbing, after, f.burst)
	}
}

func TestBurstIsRadial(t *testing.T) {
	// Every direction should be represented, or the burst is a spray.
	f := quiet(3)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.launch()
	for len(f.shells) > 0 {
		f.Frame(s, dt)
	}
	var quadrant [4]int
	for _, p := range f.particles {
		if p.life < 0.9 {
			continue // a trail ember, not part of the burst
		}
		q := 0
		if p.vx < 0 {
			q |= 1
		}
		if p.vy < 0 {
			q |= 2
		}
		quadrant[q]++
	}
	for i, n := range quadrant {
		if n == 0 {
			t.Errorf("no particle flew into quadrant %d: the burst is not radial", i)
		}
	}
}

func TestOneBurstIsOneColour(t *testing.T) {
	f := quiet(4)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.launch()
	want := f.shells[0].colour
	for len(f.shells) > 0 {
		f.Frame(s, dt)
	}
	for i, p := range f.particles {
		if p.colour != want {
			t.Fatalf("particle %d is %v, but the shell was %v: the burst is confetti",
				i, p.colour, want)
		}
	}
}

func TestParticlesFallUnderGravity(t *testing.T) {
	f := quiet(5)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.launch()
	for len(f.shells) > 0 {
		f.Frame(s, dt)
	}
	// Take the particle thrown straight up hardest and follow only it. The
	// live slice is compacted every frame, so an index into it is not a handle
	// on one particle; keeping it alone keeps index zero meaning what it did.
	pick := f.particles[0]
	for _, p := range f.particles {
		if p.vy < pick.vy {
			pick = p
		}
	}
	if pick.vy >= 0 {
		t.Fatal("no particle was thrown upwards at all")
	}
	f.particles = append(f.particles[:0], pick)

	before := pick.vy
	var rose, fell bool
	// Long enough for the hardest-thrown particle to run out of climb: the
	// burst speed is up to 0.6 of the height a second against gravity of 0.54
	// of it a second squared, so the turn comes about a second in.
	for i := 0; i < 60 && !fell; i++ {
		f.Frame(s, dt)
		if len(f.particles) == 0 {
			t.Fatal("the particle expired too soon to watch it fall")
		}
		p := f.particles[0]
		if p.vy <= before {
			t.Fatalf("frame %d: vertical velocity went from %.4f to %.4f, which is not gravity",
				i, before, p.vy)
		}
		if p.vy < 0 {
			rose = true
		} else {
			fell = true
		}
		before = p.vy
	}
	if !rose || !fell {
		t.Errorf("the particle only ever went one way (rose=%v fell=%v): it does not arc", rose, fell)
	}
}

func TestParticlesExpireRatherThanAccumulate(t *testing.T) {
	f := quiet(6)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.launch()
	for len(f.shells) > 0 {
		f.Frame(s, dt)
	}
	peak := len(f.particles)
	for i := 0; i < 400; i++ {
		f.Frame(s, dt)
	}
	if len(f.particles) != 0 {
		t.Errorf("%d of %d particles are still alive four hundred frames on", len(f.particles), peak)
	}
}

func TestSlicesNeverOutgrowTheirAllocation(t *testing.T) {
	// Frame must not allocate: it trims a burst that will not fit instead.
	f := New(7)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	pc, sc, fc := cap(f.particles), cap(f.shells), cap(f.flashes)
	for i := 0; i < 2000; i++ {
		f.Frame(s, dt)
		if cap(f.particles) != pc || cap(f.shells) != sc || cap(f.flashes) != fc {
			t.Fatalf("frame %d: a slice was reallocated mid-animation", i)
		}
		if len(f.shells) > f.Rockets {
			t.Fatalf("frame %d: %d shells in flight, more than the %d allowed",
				i, len(f.shells), f.Rockets)
		}
	}
}

func TestBurstFlashes(t *testing.T) {
	f := quiet(8)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	f.launch()
	for len(f.shells) > 0 {
		f.Frame(s, dt)
	}
	if len(f.flashes) == 0 {
		t.Fatal("the burst produced no flash")
	}
	fl := f.flashes[0]
	// The flash has to be brighter than the sparks around it, or it does not
	// read as the light of the explosion.
	var best int32
	cx, cy := int(fl.x), int(fl.y)
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			c := s.At(cx+dx, cy+dy)
			if c == tcell.ColorDefault {
				continue
			}
			r, g, b := c.RGB()
			if r+g+b > best {
				best = r + g + b
			}
		}
	}
	if best < 600 {
		t.Errorf("the brightest pixel at the burst is %d: there is no flash", best)
	}
	for i := 0; i < 8; i++ {
		f.Frame(s, dt)
	}
	if len(f.flashes) != 0 {
		t.Error("the flash is still burning eight frames later")
	}
}

func TestSeveralFireworksAreInFlightAtOnce(t *testing.T) {
	f := New(9)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	var most int
	for i := 0; i < 1200; i++ {
		f.Frame(s, dt)
		if n := len(f.shells); n > most {
			most = n
		}
	}
	if most < 2 {
		t.Errorf("at most %d shell was ever in the air: they are launching one at a time", most)
	}
}

func TestBurstScalesWithTheSurface(t *testing.T) {
	small, big := New(10), New(10)
	small.Resize(40, 24)
	big.Resize(200, 100)
	if big.burst <= small.burst {
		t.Errorf("%d particles per burst in a big window against %d in a small one",
			big.burst, small.burst)
	}
}

func TestFadeDimsTowardsBlack(t *testing.T) {
	c := tcell.NewRGBColor(200, 100, 50)
	r, g, b := fade(c, 0.5).RGB()
	if r != 100 || g != 50 || b != 25 {
		t.Errorf("fade(0.5) gave %d,%d,%d, want 100,50,25", r, g, b)
	}
	if fade(c, 0) != tcell.ColorDefault {
		t.Error("a dead particle should leave the pixel unset, not black")
	}
}

func TestBurstsAtTheSameHeightAtAnyFrameRate(t *testing.T) {
	// The one that a naive conversion gets wrong. Stepping velocity and then
	// position under constant acceleration overshoots by half a step of extra
	// speed each frame, so a coarser step throws the shell higher; the midpoint
	// form is exact at any step, and the burst height is where that shows.
	// The flash is left exactly where the shell burst, so it is the height
	// itself rather than the last position seen before the shell was dropped.
	apex := func(step float64) float64 {
		f := quiet(1)
		f.Resize(tw, th)
		f.launch()
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 2000 && len(f.flashes) == 0; i++ {
			f.Frame(s, step)
		}
		if len(f.flashes) == 0 {
			t.Fatal("the shell never burst")
		}
		return f.flashes[0].y
	}
	slow, fast := apex(1.0/30), apex(1.0/60)
	if diff := slow - fast; diff > 0.5 || diff < -0.5 {
		t.Errorf("the shell burst at %.2f at thirty frames and %.2f at sixty: "+
			"the arc depends on the step size", slow, fast)
	}
	// And it is the height the launch aimed at, not merely a consistent one.
	f := quiet(1)
	f.Resize(tw, th)
	f.launch()
	sh := f.shells[0]
	want := sh.y - (sh.vy*sh.vy)/(2*f.Gravity*f.h)
	if diff := slow - want; diff > 0.5 || diff < -0.5 {
		t.Errorf("the shell peaked at %.2f where v²/2g puts it at %.2f", slow, want)
	}
}

func TestParticlesFlyTheSameDistanceAtAnyFrameRate(t *testing.T) {
	// A spark is the harder case: gravity, drag and a fading life all at once,
	// and drag compounds rather than adding up.
	fly := func(step float64, frames int) (float64, float64, float64) {
		f := quiet(2)
		f.Resize(tw, th)
		f.particles = append(f.particles[:0], particle{
			x: 32, y: 24, vx: 20, vy: -10,
			life: 1, decay: 0.5, colour: color.White,
		})
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			f.Frame(s, step)
		}
		if len(f.particles) == 0 {
			t.Fatal("the particle expired inside the second under test")
		}
		p := f.particles[0]
		return p.x, p.y, p.life
	}
	sx, sy, sl := fly(1.0/30, 30)
	fx, fy, fl := fly(1.0/60, 60)
	if diff := sy - fy; diff > 0.05 || diff < -0.05 {
		t.Errorf("a spark fell to %.4f in thirty frames but %.4f in sixty", sy, fy)
	}
	if diff := sx - fx; diff > 0.01 || diff < -0.01 {
		t.Errorf("a spark drifted to %.4f in thirty frames but %.4f in sixty: "+
			"drag is being applied per frame, not per second", sx, fx)
	}
	if diff := sl - fl; diff > 0.001 || diff < -0.001 {
		t.Errorf("a spark faded to %.4f in thirty frames but %.4f in sixty: "+
			"the life is a per-frame decay", sl, fl)
	}
	if sl < 0.49 || sl > 0.51 {
		t.Errorf("half a life a second left %.4f after a second", sl)
	}
}

func TestLaunchRateIsPerSecond(t *testing.T) {
	// A per-frame chance would put twice as many shells up at sixty frames.
	count := func(step float64, frames int) int {
		f := New(5)
		// Room for many times the launches expected, so the count measures the
		// rate and not the ceiling on shells in flight. Not wildly more: Resize
		// sizes the particle pool from this.
		f.Rockets = 500
		// A shell's climb lasts sqrt(1.2/Gravity) seconds whatever the surface
		// size, so this weak a pull keeps every shell in the air for a minute
		// and none of them bursts inside the window. Shells then only ever
		// arrive, and the number in the sky at the end is exactly the number
		// launched — no counting of deltas that a simultaneous burst would
		// quietly swallow.
		f.Gravity = 0.0003
		f.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			f.Frame(s, step)
		}
		return len(f.shells)
	}
	// A minute, which at 1.2 a second is about seventy launches. The two runs
	// draw different random numbers so they will not agree exactly; over that
	// many the counts cannot differ by half unless the rate itself did.
	slow, fast := count(1.0/30, 1800), count(1.0/60, 3600)
	if fast*2 > slow*3 || slow*2 > fast*3 {
		t.Errorf("a minute gave %d launches at thirty frames and %d at sixty",
			slow, fast)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []particle {
		f := New(11)
		f.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 300; i++ {
			f.Frame(s, dt)
		}
		return f.particles
	}
	a, b := run(), run()
	if len(a) != len(b) {
		t.Fatalf("same seed gave %d particles then %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at particle %d", i)
		}
	}
}

func TestShellAimsIntoTheUpperSurface(t *testing.T) {
	// v = sqrt(2*g*rise) should just run out of climb in the top half.
	f := quiet(12)
	f.Resize(tw, th)
	g := f.Gravity * f.h
	for i := 0; i < 50; i++ {
		f.shells = f.shells[:0]
		f.launch()
		sh := f.shells[0]
		apex := sh.y - (sh.vy*sh.vy)/(2*g)
		if apex < 0 || apex > th*0.6 {
			t.Fatalf("shell %d would peak at %.1f on a %d-pixel surface", i, apex, th)
		}
		if math.Abs(sh.vx) > f.h*0.13 {
			t.Fatalf("shell %d drifts sideways at %.3f: that is a mortar, not a rocket", i, sh.vx)
		}
	}
}
