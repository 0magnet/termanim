package lavalamp

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 80

// dt is the step every test takes unless it is testing the step itself: the
// rate the lamp's rates were tuned at.
const dt = 1.0 / 30

func TestBlobsStayInsideTheVessel(t *testing.T) {
	// The vessel, not the surface, is the boundary: a blob that wandered into
	// the corner of the window would be clipped away by the drawing and the
	// lamp would appear to lose its wax.
	l := New(1)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 1000; i++ {
		l.Frame(s, dt)
		for j := range l.blobs {
			b := l.blobs[j]
			if b.y < 0 || b.y > th {
				t.Fatalf("frame %d: blob %d left the vessel vertically at y=%.2f", i, j, b.y)
			}
			if lim := l.limit(b.y); math.Abs(b.x-l.cx) > lim+1e-9 {
				t.Fatalf("frame %d: blob %d is %.2f from the axis, limit %.2f at y=%.2f",
					i, j, math.Abs(b.x-l.cx), lim, b.y)
			}
		}
	}
}

func TestNothingIsDrawnOutsideTheVessel(t *testing.T) {
	// The silhouette is what makes this a lamp rather than metaballs in a box.
	l := New(2)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 50; i++ {
		l.Frame(s, dt)
	}
	for y := 0; y < th; y++ {
		hw := l.halfWidth(float64(y) + 0.5)
		for x := 0; x < tw; x++ {
			inside := float64(x)+0.5 >= l.cx-hw-1 && float64(x)+0.5 <= l.cx+hw+1
			if !inside && s.At(x, y) != tcell.ColorDefault {
				t.Fatalf("pixel %d,%d is lit but lies outside the vessel", x, y)
			}
		}
	}
}

func TestTheVesselIsNarrowerAtTheTop(t *testing.T) {
	l := New(3)
	l.Resize(tw, th)
	neck := l.halfWidth(0.15 * th)
	body := l.halfWidth(0.85 * th)
	if neck >= body {
		t.Fatalf("neck %.2f is not narrower than the body %.2f", neck, body)
	}
	if l.halfWidth(0) > 0.2*l.maxHalf || l.halfWidth(th) > 0.2*l.maxHalf {
		t.Error("the rim and the base are not rounded off")
	}
}

func TestTheBackgroundIsNotAllLit(t *testing.T) {
	// An inverse-square field never reaches zero. Without the cutoff every
	// pixel in the vessel comes out faintly lit and the wax stops floating.
	l := New(4)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	l.Frame(s, dt)
	var inside, unlit, bright int
	for y := 0; y < th; y++ {
		hw := l.halfWidth(float64(y) + 0.5)
		for x := 0; x < tw; x++ {
			fx := float64(x) + 0.5
			if fx < l.cx-hw+1 || fx > l.cx+hw-1 {
				continue // the glass wall itself, which is meant to be lit
			}
			inside++
			c := s.At(x, y)
			if c == tcell.ColorDefault {
				unlit++
				continue
			}
			if r, g, b := c.RGB(); r+g+b > 380 {
				bright++
			}
		}
	}
	if bright == 0 {
		t.Error("no bright core anywhere: the field never gets strong")
	}
	if unlit*4 < inside {
		t.Errorf("only %d of %d interior pixels are unlit: the cutoff is not working",
			unlit, inside)
	}
}

func TestWaxAtTheBaseHeatsAndRises(t *testing.T) {
	l := New(5)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	// One blob only, parked on the element at neutral temperature, so nothing
	// but the heating can explain what happens to it.
	l.blobs = l.blobs[:1]
	b := &l.blobs[0]
	b.x, b.y = l.cx, 0.97*th
	b.vx, b.vy = 0, 0
	b.temp = neutral
	for i := 0; i < 60; i++ {
		l.Frame(s, dt)
	}
	if b.temp <= neutral {
		t.Errorf("wax on the element did not heat: %.3f", b.temp)
	}
	if b.vy >= 0 {
		t.Errorf("hot wax is not rising: vy %.4f", b.vy)
	}
	if b.r <= b.r0 {
		t.Errorf("hot wax did not expand: radius %.2f, neutral %.2f", b.r, b.r0)
	}
}

func TestWaxAtTheNeckCoolsAndSinks(t *testing.T) {
	l := New(6)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	l.blobs = l.blobs[:1]
	b := &l.blobs[0]
	b.x, b.y = l.cx, 0.03*th
	b.vx, b.vy = 0, 0
	b.temp = 1
	for i := 0; i < 200; i++ {
		l.Frame(s, dt)
	}
	if b.temp >= 1 {
		t.Errorf("wax at the cold glass did not cool: %.3f", b.temp)
	}
	if b.vy <= 0 {
		t.Errorf("cold wax is not sinking: vy %.4f", b.vy)
	}
	if b.r >= b.r0 {
		t.Errorf("cold wax did not contract: radius %.2f, neutral %.2f", b.r, b.r0)
	}
}

func TestWaxCirculates(t *testing.T) {
	// The point of the whole thing: over a long run a blob should visit both
	// ends of the lamp rather than settling anywhere.
	l := New(7)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	l.blobs = l.blobs[:1]
	b := &l.blobs[0]
	b.y, b.temp, b.vy = 0.5*th, neutral, 0
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := 0; i < 3000; i++ {
		l.Frame(s, dt)
		lo = math.Min(lo, b.y)
		hi = math.Max(hi, b.y)
	}
	if hi-lo < 0.5*th {
		t.Errorf("wax travelled only %.1f of %d rows: it is not circulating", hi-lo, th)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []blob {
		l := New(9)
		l.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 100; i++ {
			l.Frame(s, dt)
		}
		return l.blobs
	}
	a, c := run(), run()
	if len(a) != len(c) {
		t.Fatalf("same seed gave %d blobs then %d", len(a), len(c))
	}
	for i := range a {
		if a[i] != c[i] {
			t.Fatalf("same seed diverged at blob %d: %+v vs %+v", i, a[i], c[i])
		}
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {4, 4}, {8, 6}, {0, 0}} {
		l := New(8)
		l.Resize(sz[0], sz[1])
		w, h := sz[0], sz[1]
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		s := canvas.NewSurface(w, h)
		for i := 0; i < 30; i++ {
			l.Frame(s, dt)
		}
	}
}

func TestFrameDoesNotAllocate(t *testing.T) {
	// Everything Frame needs is sized in Resize, so a frame costs the allocator
	// nothing.
	l := New(11)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	l.Frame(s, dt)
	if n := testing.AllocsPerRun(50, func() { l.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}

func TestFrameRateIndependence(t *testing.T) {
	// One blob released cold at the base, so it heats, lifts and is caught
	// mid-climb rather than resting against either end of the lamp, where a
	// wrong rate would be hidden by the confinement.
	run := func(step float64, steps int) float64 {
		l := New(21)
		l.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		l.blobs = l.blobs[:1]
		b := &l.blobs[0]
		b.x, b.y = l.cx, 0.95*th
		b.vx, b.vy = 0, 0
		b.temp = neutral
		for i := 0; i < steps; i++ {
			l.Frame(s, step)
		}
		return b.y
	}
	// Eight seconds of lamp, at thirty steps a second and at sixty.
	a := run(1.0/30, 240)
	c := run(1.0/60, 480)
	if a > 0.85*th || a < 0.1*th {
		t.Fatalf("the wax is at %.1f of %d rows: the test is not catching it mid-climb", a, th)
	}
	if math.Abs(a-c) > 0.05*th {
		t.Errorf("wax height depends on the frame rate: %.1f at 30fps, %.1f at 60fps", a, c)
	}
}
