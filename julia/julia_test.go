package julia

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 48

// dt is one frame of the thirty-a-second loop these constants were tuned at.
const dt = 1.0 / 30

func at(seconds float64) (*Julia, *canvas.Surface) {
	j := New()
	j.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	j.t = seconds
	j.Frame(s, dt)
	return j, s
}

// inMandelbrot reports whether c is in the Mandelbrot set, which is to say
// whether the Julia set of c is connected. Written out longhand here rather
// than shared with the animation: it is the independent statement the tests
// about the path are made against.
func inMandelbrot(re, im float64) bool {
	var x, y float64
	for i := 0; i < 2000; i++ {
		x2, y2 := x*x, y*y
		if x2+y2 > 4 {
			return false
		}
		x, y = x2-y2+re, 2*x*y+im
	}
	return true
}

// z² + c is even in z, so a point and its negation escape in the same number
// of steps and the picture must be symmetric under a half turn. The animation
// computes half the pixels and copies the rest on the strength of that; if the
// centering were half a pixel out the copy would be of the wrong point, and
// the set would be subtly doubled.
func TestPictureIsSymmetricUnderAHalfTurn(t *testing.T) {
	for _, seconds := range []float64{0, 7, 23, 61} {
		_, s := at(seconds)
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				if a, b := s.At(x, y), s.At(tw-1-x, th-1-y); a != b {
					t.Fatalf("t=%.0f: %d,%d is %v but its opposite %d,%d is %v",
						seconds, x, y, a, tw-1-x, th-1-y, b)
				}
			}
		}
	}
}

// The path has to close, or the animation is a walk that never comes home and
// the parameter eventually wanders somewhere dull.
func TestTheParameterPathIsClosed(t *testing.T) {
	j := New()
	j.Resize(tw, th)

	j.t = 0
	re0, im0 := j.param()
	j.t = j.Period()
	re1, im1 := j.param()
	if math.Hypot(re1-re0, im1-im0) > 1e-9 {
		t.Errorf("one period ends at %+.9f%+.9fi, having started at %+.9f%+.9fi", re1, im1, re0, im0)
	}

	// And it must go somewhere in between rather than sitting still.
	var far float64
	for k := 1; k < 12; k++ {
		j.t = j.Period() * float64(k) / 12
		re, im := j.param()
		if d := math.Hypot(re-re0, im-im0); d > far {
			far = d
		}
	}
	if far < 0.5 {
		t.Errorf("the parameter never gets further than %.4f from where it started", far)
	}
}

// A Julia set that fills the screen or leaves it empty is not a Julia set that
// anyone can see. The interior is the silhouette and the exterior is what the
// bands are drawn in, so both have to be there at every point of the walk.
func TestTheSetHasBothAnInteriorAndAnExterior(t *testing.T) {
	j := New()
	for k := 0; k < 16; k++ {
		jj, s := at(j.Period() * float64(k) / 16)
		var inside int
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				if s.At(x, y) == tcell.ColorDefault {
					inside++
				}
			}
		}
		f := float64(inside) / float64(tw*th)
		if f < 0.05 || f > 0.6 {
			re, im := jj.param()
			t.Errorf("at t=%.1f, c=%+.4f%+.4fi, the interior covers %.0f%% of the screen", jj.t, re, im, f*100)
		}
	}
}

// The exterior has to be a gradient rather than four flat plateaus. Most of it
// escapes within three steps, so this is only true because the escape count is
// corrected by how far past the bailout the point landed.
func TestTheExteriorIsAGradientAndNotAHandfulOfBands(t *testing.T) {
	_, s := at(25)
	seen := make(map[tcell.Color]bool)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if c := s.At(x, y); c != tcell.ColorDefault {
				seen[c] = true
			}
		}
	}
	if len(seen) < 40 {
		t.Errorf("the exterior uses %d colors: the escape count is being taken as an integer", len(seen))
	}
}

// The iteration limit is what keeps a frame inside its budget on a terminal of
// any size, so it has to answer to how long frames are actually taking.
func TestTheIterationLimitFollowsTheFrameBudget(t *testing.T) {
	j := New()
	j.Resize(tw, th)
	s := canvas.NewSurface(tw, th)

	// Frames arriving far apart mean the loop is not keeping up.
	for i := 0; i < 200; i++ {
		j.Frame(s, j.FrameBudget*3)
	}
	if int(j.iter) != j.MinIter {
		t.Errorf("after two hundred slow frames the limit is %.0f, not the floor of %d", j.iter, j.MinIter)
	}

	// And frames arriving comfortably inside it mean there is room for detail.
	for i := 0; i < 400; i++ {
		j.Frame(s, dt)
	}
	if int(j.iter) != j.MaxIter {
		t.Errorf("after four hundred fast frames the limit is %.0f, not the ceiling of %d", j.iter, j.MaxIter)
	}
}

// And it has to start from the size of the surface, because the first frame on
// a large terminal has no history to steer by and is the one most likely to
// stall.
func TestABiggerSurfaceStartsWithFewerIterations(t *testing.T) {
	small, big := New(), New()
	small.Resize(60, 40)
	big.Resize(400, 240)
	if big.iter >= small.iter {
		t.Errorf("a 400x240 surface starts at %.0f iterations and a 60x40 one at %.0f", big.iter, small.iter)
	}
	if big.iter < float64(big.MinIter) {
		t.Errorf("a large surface started below the floor, at %.0f", big.iter)
	}
	// The budget is pixels times iterations, so that is the thing that must
	// hold whatever the window.
	if cost := big.iter * 400 * 240; cost > big.PixelBudget*1.01 {
		t.Errorf("the first frame of a 400x240 surface costs %.0f pixel-iterations against a budget of %.0f",
			cost, big.PixelBudget)
	}
}

// Cycling the palette must move the colors without moving the set: the bands
// are level sets of the escape count, and rotating the ramp makes them appear
// to flow outward from a boundary that has not changed.
func TestThePaletteCyclesWithoutMovingTheSet(t *testing.T) {
	j := New()
	j.Speed = 0 // hold c still
	j.Resize(tw, th)
	p := canvas.NewSurface(tw, th)
	q := canvas.NewSurface(tw, th)
	j.Frame(p, dt)
	for i := 0; i < 90; i++ { // three seconds of cycling
		j.Frame(q, dt)
	}

	var moved, changed int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			a, b := p.At(x, y), q.At(x, y)
			if (a == tcell.ColorDefault) != (b == tcell.ColorDefault) {
				moved++
			}
			if a != b {
				changed++
			}
		}
	}
	if moved != 0 {
		t.Errorf("%d pixels crossed the boundary of the set while c was held still", moved)
	}
	if changed < tw*th/4 {
		t.Errorf("only %d of %d pixels changed color in three seconds of cycling", changed, tw*th)
	}
}

// And the shape itself must morph, which is the reason c walks at all.
func TestTheShapeMorphs(t *testing.T) {
	j := New()
	j.CycleRate = 0 // isolate the shape from the color
	j.Resize(tw, th)
	p := canvas.NewSurface(tw, th)
	q := canvas.NewSurface(tw, th)
	j.Frame(p, dt)
	for i := 0; i < 30*20; i++ { // twenty seconds of walking
		j.Frame(q, dt)
	}
	var moved int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if (p.At(x, y) == tcell.ColorDefault) != (q.At(x, y) == tcell.ColorDefault) {
				moved++
			}
		}
	}
	if moved < tw*th/20 {
		t.Errorf("only %d of %d pixels changed sides in twenty seconds: the set is barely moving",
			moved, tw*th)
	}
}

// The walk must follow the clock and not the frame count.
func TestTheWalkFollowsTheClock(t *testing.T) {
	run := func(frames int, step float64) *Julia {
		j := New()
		j.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			j.Frame(s, step)
		}
		return j
	}
	slow := run(60, 1.0/30)  // two seconds
	fast := run(120, 1.0/60) // the same two seconds
	sre, sim := slow.param()
	fre, fim := fast.param()
	if math.Hypot(sre-fre, sim-fim) > 1e-9 {
		t.Errorf("two seconds reached %+.9f%+.9fi at 30fps and %+.9f%+.9fi at 60fps", sre, sim, fre, fim)
	}
	if math.Abs(slow.t-2) > 1e-9 {
		t.Errorf("two seconds of frames advanced the walk by %.9f seconds", slow.t)
	}
}

func TestTheSameFramesGiveTheSamePicture(t *testing.T) {
	run := func() *canvas.Surface {
		j := New()
		j.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 90; i++ {
			j.Frame(s, dt)
		}
		return s
	}
	p, q := run(), run()
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if p.At(x, y) != q.At(x, y) {
				t.Fatalf("two identical runs diverged at %d,%d", x, y)
			}
		}
	}
}

func TestOddSizesDoNotPanic(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 9}, {9, 2}, {200, 120}} {
		j := New()
		j.Resize(sz[0], sz[1])
		s := canvas.NewSurface(sz[0], sz[1])
		for i := 0; i < 3; i++ {
			j.Frame(s, dt)
		}
	}
	// A Frame before any Resize must be a no-op rather than a nil dereference.
	New().Frame(canvas.NewSurface(4, 4), dt)
}

func TestFrameDoesNotAllocate(t *testing.T) {
	j := New()
	j.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	j.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { j.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per run; buffers belong in Resize", n)
	}
}

// The path must run just INSIDE the boundary of the Mandelbrot set: inside,
// because outside it the Julia set is a dust with no shape to it, and just
// inside, because that is where the shapes are.
//
// Two claims, and only the first is exact. Every point of the path is in the
// set, so the Julia set on screen is connected in every frame. And the path is
// near the edge, which is checked by stepping three percent further out from
// the cardioid it is an inset copy of and finding that most of the time that
// leaves the set — most and not all, because the cardioid has bulbs budding
// off it, and where the path passes one, a step outward lands in the bulb.
// That is still the boundary; it is just not the cardioid's own.
func TestTheParameterRunsJustInsideTheBoundary(t *testing.T) {
	j := New()
	j.Resize(tw, th)
	const samples = 120
	var left int
	for k := 0; k < samples; k++ {
		j.t = j.Period() * float64(k) / samples
		re, im := j.param()
		if !inMandelbrot(re, im) {
			t.Fatalf("at %.0f%% round the path, c=%+.5f%+.5fi is outside the set: the Julia set there is dust",
				100*float64(k)/samples, re, im)
		}
		rho := 1 - j.Inset + j.Wobble*math.Sin(3*j.t*baseWalk*j.Speed)
		if out := 1.03 / rho; !inMandelbrot(re*out, im*out) {
			left++
		}
	}
	if left < samples*7/10 {
		t.Errorf("a three percent step outward left the set at only %d of %d points on the path: "+
			"the walk is not hugging the boundary", left, samples)
	}
}
