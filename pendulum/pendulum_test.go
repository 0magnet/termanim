package pendulum

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 96, 64

// dt is one tick of a thirty-a-second loop.
const dt = 1.0 / 30

// energy is the total mechanical energy of one pendulum: the kinetic energy of
// the two masses plus their potential energy. It is conserved exactly by the
// equations of motion, so any change in it is the integrator's doing and
// nothing else — which is what makes it the one number worth watching.
func energy(p *Pendulum, s state) float64 {
	m1, m2, l1, l2, g := p.M1, p.M2, p.L1, p.L2, p.Gravity
	ke := 0.5*m1*l1*l1*s.w1*s.w1 +
		0.5*m2*(l1*l1*s.w1*s.w1+l2*l2*s.w2*s.w2+
			2*l1*l2*s.w1*s.w2*math.Cos(s.t1-s.t2))
	pe := -(m1+m2)*g*l1*math.Cos(s.t1) - m2*g*l2*math.Cos(s.t2)
	return ke + pe
}

// swing runs the animation for roughly the given number of seconds.
func swing(p *Pendulum, secs float64) *canvas.Surface {
	s := canvas.NewSurface(tw, th)
	for t := 0.0; t < secs; t += dt {
		p.Frame(s, dt)
	}
	return s
}

// apart is the widest gap between any pendulum's first rod and the first one's.
func apart(p *Pendulum) float64 {
	var worst float64
	for i := range p.arms {
		if d := math.Abs(p.arms[i].t1 - p.arms[0].t1); d > worst {
			worst = d
		}
	}
	return worst
}

func TestEnergyStaysBounded(t *testing.T) {
	// If the integrator fed the system energy the pendulums would wind
	// themselves up and the fan would open on the integrator's arithmetic
	// rather than on the physics. This is the assertion that stops that being
	// true, and it is the reason for RK4.
	p := New(1)
	p.ReleaseAfter = 0 // a re-release changes the energy on purpose
	p.Resize(tw, th)
	want := make([]float64, len(p.arms))
	for i := range p.arms {
		want[i] = energy(p, p.arms[i])
	}
	for secs := 0; secs < 30; secs++ {
		swing(p, 1)
		for i := range p.arms {
			got := energy(p, p.arms[i])
			if rel := math.Abs(got-want[i]) / math.Abs(want[i]); rel > 1e-5 {
				t.Fatalf("after %ds pendulum %d holds %.9f against the %.9f it started with "+
					"(%.2e relative): the integrator is not conserving energy",
					secs+1, i, got, want[i], rel)
			}
		}
	}
}

func TestNearIdenticalStartsTrackThenDiverge(t *testing.T) {
	// The whole animation in one assertion. Angles a hundred-thousandth of a
	// radian apart have to stay together long enough to read as one object,
	// and then come completely apart with nothing having changed.
	p := New(2)
	p.ReleaseAfter = 0
	p.Resize(tw, th)
	if got := apart(p); got > float64(len(p.arms))*p.Spread+1e-12 {
		t.Fatalf("the fan starts %.3e radians wide, which is more than %d spreads",
			got, len(p.arms))
	}
	swing(p, 1)
	// A pixel is about 1/27 of a rod length here, so a hundredth of a radian
	// is comfortably sub-pixel: after a second they are still one object.
	if got := apart(p); got > 0.01 {
		t.Errorf("after one second the fan is already %.4f radians wide", got)
	}
	swing(p, 11)
	if got := apart(p); got < 0.5 {
		t.Errorf("after twelve seconds the fan is only %.4f radians wide: "+
			"these pendulums are not sensitive to their initial conditions", got)
	}
}

func TestTheDivergenceIsThePhysicsAndNotTheIntegrator(t *testing.T) {
	// The honesty check. Run the identical release at the step used and at a
	// quarter of it: the difference between those two is the integrator's own
	// error. It has to be far below the separation the pendulums were started
	// with, or the fan on screen is an artifact of the arithmetic.
	at := func(h, secs float64) *Pendulum {
		p := New(3)
		p.SubStep = h
		p.ReleaseAfter = 0
		p.Resize(tw, th)
		swing(p, secs)
		return p
	}
	const secs = 2
	coarse := at(1.0/480, secs)
	fine := at(1.0/1920, secs)
	integrator := math.Abs(coarse.arms[0].t1 - fine.arms[0].t1)
	physical := math.Abs(coarse.arms[0].t1 - coarse.arms[1].t1)
	if integrator <= 0 {
		t.Fatal("two different steps gave bit-identical answers; the comparison is not being made")
	}
	if physical < integrator*100 {
		t.Errorf("after %ds the integrator's own error is %.3e radians and the "+
			"separation being demonstrated is %.3e: the fan is the arithmetic, not the physics",
			secs, integrator, physical)
	}
}

func TestALowEnergyReleaseIsNotChaotic(t *testing.T) {
	// The other half of the claim in the doc comment for Release. Below the
	// energy needed to go over the top the double pendulum is nearly two
	// coupled linear oscillators and nearby starts stay near, so a fan
	// released from a small angle must not open. If it did, the divergence at
	// the default release would not be evidence of anything.
	p := New(4)
	p.Release = 0.15
	p.ReleaseAfter = 0
	p.Resize(tw, th)
	swing(p, 20)
	if got := apart(p); got > 0.01 {
		t.Errorf("released from %.2f radians the fan opened to %.4f in twenty seconds",
			p.Release, got)
	}
}

func TestTheRodsKeepTheirLengths(t *testing.T) {
	// A geometry check, which catches a sign or a swapped angle in the
	// equations of motion far more cheaply than looking at the picture would.
	p := New(5)
	p.ReleaseAfter = 0
	p.Resize(tw, th)
	for secs := 0; secs < 8; secs++ {
		swing(p, 1)
		for i := range p.arms {
			jx, jy := p.joint(p.arms[i])
			tx, ty := p.tip(p.arms[i])
			upper := math.Hypot(jx-p.cx, jy-p.cy)
			lower := math.Hypot(tx-jx, ty-jy)
			if math.Abs(upper-p.scale*p.L1) > 1e-9 {
				t.Fatalf("pendulum %d: upper rod is %.9f pixels, want %.9f", i, upper, p.scale*p.L1)
			}
			if math.Abs(lower-p.scale*p.L2) > 1e-9 {
				t.Fatalf("pendulum %d: lower rod is %.9f pixels, want %.9f", i, lower, p.scale*p.L2)
			}
		}
	}
}

func TestTrailsFadeRatherThanFillingTheScreen(t *testing.T) {
	p := New(6)
	p.ReleaseAfter = 0
	p.Resize(tw, th)
	s := swing(p, 8)
	drawn := 0
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				drawn++
			}
		}
	}
	if drawn == 0 {
		t.Fatal("eight seconds of swinging drew nothing")
	}
	if drawn == tw*th {
		t.Fatal("every pixel is lit: the trails never fade")
	}
	// Stop the pendulums and let the trail alone. It has to go away, and it
	// has to go away smoothly rather than in one step.
	p.SubStep = 0
	p.ShowArms = false
	var before float64
	for _, v := range p.glow {
		before += float64(v)
	}
	prev := before
	for i := 0; i < 30; i++ {
		p.Frame(s, dt)
		var now float64
		for _, v := range p.glow {
			now += float64(v)
		}
		if now >= prev && prev > 0 {
			t.Fatalf("frame %d: the trail holds %.3f, up from %.3f", i, now, prev)
		}
		prev = now
	}
	if prev > before*0.6 {
		t.Errorf("a second of fading left %.3f of %.3f: TrailLife is not being honored", prev, before)
	}
}

func TestFanIsReleasedAgain(t *testing.T) {
	// Once the pendulums have separated there is nothing further to see, so
	// the fan is started over.
	p := New(7)
	p.ReleaseAfter = 4
	p.Resize(tw, th)
	was := p.released
	s := canvas.NewSurface(tw, th)
	var opened float64
	for i := 0; i < 200 && p.released == was; i++ {
		opened = apart(p)
		p.Frame(s, dt)
	}
	if p.released == was {
		t.Fatal("no second release inside seven seconds with ReleaseAfter of four")
	}
	if opened < 1e-4 {
		t.Fatalf("the fan had only opened to %.3e before it was released again", opened)
	}
	if got := apart(p); got > float64(len(p.arms))*p.Spread+1e-12 {
		t.Errorf("the fan is %.3e radians wide on the frame it was released, want it closed again", got)
	}
}

func TestStepRateIsFrameRateIndependent(t *testing.T) {
	// Two seconds of wall clock at three frame rates. The integrator takes the
	// same steps in the same order in all of them, so the pendulums end up in
	// bit-identical states — which for a chaotic system is the only tolerance
	// worth asking for, since anything else grows.
	run := func(frames int, step float64) *Pendulum {
		p := New(8)
		p.ReleaseAfter = 0
		p.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			p.Frame(s, step)
		}
		return p
	}
	slow := run(60, 1.0/30)
	if slow.steps != 960 {
		t.Fatalf("two seconds at 30fps took %d steps, want 960", slow.steps)
	}
	for _, p := range []*Pendulum{run(120, 1.0/60), run(240, 1.0/120)} {
		if p.steps != slow.steps {
			t.Fatalf("the same two seconds took %d steps at one frame rate and %d at another",
				slow.steps, p.steps)
		}
		for i := range p.arms {
			if p.arms[i] != slow.arms[i] {
				t.Fatalf("pendulum %d ended at %+v at one frame rate and %+v at another",
					i, slow.arms[i], p.arms[i])
			}
		}
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) []state {
		p := New(seed)
		p.Resize(tw, th)
		swing(p, 5)
		out := make([]state, len(p.arms))
		copy(out, p.arms)
		return out
	}
	a, b := run(0), run(0)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at pendulum %d", i)
		}
	}
	c := run(9)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds released the fan identically")
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {3, 2}, {8, 6}} {
		p := New(10)
		p.Resize(sz[0], sz[1])
		s := canvas.NewSurface(max(sz[0], 1), max(sz[1], 1))
		for i := 0; i < 60; i++ {
			p.Frame(s, dt)
		}
	}
}

func TestFrameDoesNotAllocate(t *testing.T) {
	// A Runge-Kutta stage is a value type and the trail buffers are sized in
	// Resize, so integrating sixteen steps for fourteen pendulums and painting
	// the whole surface costs the allocator nothing.
	p := New(11)
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	p.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { p.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}
