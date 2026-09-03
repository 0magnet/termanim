package donut

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 78, 46

// dt is the step the assertions below are calibrated to: a thirtieth of a
// second is what a frame used to be worth, so "N frames" still moves the pose
// as far as it always did.
const dt = 1.0 / 30

func render(seed int64, frames int) (*Donut, *canvas.Surface) {
	d := New(seed)
	d.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		d.Frame(s, dt)
	}
	return d, s
}

func setPixels(s *canvas.Surface) int {
	w, h := s.Size()
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				n++
			}
		}
	}
	return n
}

func TestDrawsSomething(t *testing.T) {
	_, s := render(1, 1)
	n := setPixels(s)
	// The torus fills most of a circle 0.9 of the shorter side across, so it
	// should cover a good fraction of the window. A handful of pixels would
	// mean the sampling or the scale had collapsed.
	if n < tw*th/10 {
		t.Fatalf("only %d pixels drawn out of %d; the torus is missing or tiny", n, tw*th)
	}
}

// TestHasAHole is the difference between a torus and a disc: somewhere in the
// middle of the silhouette there must be a run of untouched pixels with drawn
// pixels on both sides of it.
func TestHasAHole(t *testing.T) {
	_, s := render(1, 1)
	rows := 0
	for y := 0; y < th; y++ {
		first, last := -1, -1
		for x := 0; x < tw; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first < 0 {
			continue
		}
		gap := 0
		for x := first; x <= last; x++ {
			if s.At(x, y) == tcell.ColorDefault {
				gap++
			}
		}
		// One or two stray pixels could be a sampling gap rather than the hole.
		if gap > 3 {
			rows++
		}
	}
	if rows < 4 {
		t.Errorf("only %d rows cross an interior gap: this is a disc, not a torus", rows)
	}
}

// TestZBufferHoldsTheNearestSample checks the buffer against an independent
// pass over the same surface points, written out longhand here rather than
// reusing Frame's loop so that it is a real second opinion.
func TestZBufferHoldsTheNearestSample(t *testing.T) {
	d, _ := render(1, 3)

	bigR, tubeR := d.radii()
	sinA, cosA := math.Sincos(d.a)
	sinB, cosB := math.Sincos(d.b)

	want := make([]float64, tw*th)
	for pi := range d.phiCos {
		for ti := range d.thetaCos {
			sp, cp := d.phiSin[pi], d.phiCos[pi]
			st, ct := d.thetaSin[ti], d.thetaCos[ti]
			ring := bigR + tubeR*ct
			x0, y0, z0 := ring*cp, tubeR*st, ring*sp
			y1 := y0*cosA - z0*sinA
			z1 := y0*sinA + z0*cosA
			x2 := x0*cosB - y1*sinB
			y2 := x0*sinB + y1*cosB
			depth := z1 + camDist
			if depth <= 0.05 {
				continue
			}
			ooz := 1 / depth
			x := int(d.cx + d.scale*x2*ooz)
			y := int(d.cy - d.scale*y2*ooz)
			if x < 0 || y < 0 || x >= tw || y >= th {
				continue
			}
			if ooz > want[y*tw+x] {
				want[y*tw+x] = ooz
			}
		}
	}

	for i := range want {
		if math.Abs(want[i]-d.zbuf[i]) > 1e-12 {
			t.Fatalf("pixel %d,%d: z-buffer holds depth %.6f but a nearer sample at %.6f reached it",
				i%tw, i/tw, 1/d.zbuf[i], 1/want[i])
		}
	}
}

// TestFarSideDoesNotOverwriteNearSide is the same claim seen from the other
// side: if the buffer were removed and the last sample to arrive simply won,
// the picture would be materially different, because the far wall of the tube
// is visited after the near wall for half of the sweep.
func TestFarSideDoesNotOverwriteNearSide(t *testing.T) {
	d, _ := render(1, 3)

	bigR, tubeR := d.radii()
	sinA, cosA := math.Sincos(d.a)
	sinB, cosB := math.Sincos(d.b)

	last := make([]float64, tw*th)
	for pi := range d.phiCos {
		for ti := range d.thetaCos {
			sp, cp := d.phiSin[pi], d.phiCos[pi]
			st, ct := d.thetaSin[ti], d.thetaCos[ti]
			ring := bigR + tubeR*ct
			x0, y0, z0 := ring*cp, tubeR*st, ring*sp
			y1 := y0*cosA - z0*sinA
			z1 := y0*sinA + z0*cosA
			x2 := x0*cosB - y1*sinB
			y2 := x0*sinB + y1*cosB
			depth := z1 + camDist
			if depth <= 0.05 {
				continue
			}
			ooz := 1 / depth
			x := int(d.cx + d.scale*x2*ooz)
			y := int(d.cy - d.scale*y2*ooz)
			if x < 0 || y < 0 || x >= tw || y >= th {
				continue
			}
			last[y*tw+x] = ooz
		}
	}

	var covered, wrong int
	for i := range last {
		if d.zbuf[i] == 0 {
			continue
		}
		covered++
		if last[i] < d.zbuf[i]-1e-9 {
			wrong++
		}
	}
	if covered == 0 {
		t.Fatal("nothing drawn")
	}
	// Enough of the surface overlaps itself that this is not a marginal effect;
	// if the z test were doing nothing this would be zero.
	if wrong*10 < covered {
		t.Errorf("only %d of %d covered pixels would have been taken by a farther sample; "+
			"the z test is not doing any work", wrong, covered)
	}
}

func TestLitAndUnlitRegionsBothExist(t *testing.T) {
	_, s := render(1, 1)
	var bright, dark int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			c := s.At(x, y)
			if c == tcell.ColorDefault {
				continue
			}
			r, g, b := c.RGB()
			switch lum := int(r) + int(g) + int(b); {
			case lum > 500:
				bright++
			case lum < 120:
				dark++
			}
		}
	}
	if bright == 0 {
		t.Error("nothing is brightly lit: the light never faces the surface")
	}
	if dark == 0 {
		t.Error("nothing is in shadow: the surface is being drawn flat")
	}
}

func TestImageChangesBetweenFrames(t *testing.T) {
	d := New(1)
	d.Resize(tw, th)
	a := canvas.NewSurface(tw, th)
	b := canvas.NewSurface(tw, th)
	d.Frame(a, dt)
	d.Frame(b, dt)
	same := 0
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) == b.At(x, y) {
				same++
			}
		}
	}
	if same == tw*th {
		t.Error("consecutive frames are identical: the torus is not turning")
	}
}

// TestPoseDependsOnElapsedTimeNotFrameCount is the whole point of taking dt:
// a run at half the frame rate must reach the same attitude in the same number
// of seconds, so a slow terminal drops frames instead of running in slow
// motion.
func TestPoseDependsOnElapsedTimeNotFrameCount(t *testing.T) {
	const seconds = 4.0

	slow := New(2)
	slow.Resize(tw, th)
	ss := canvas.NewSurface(tw, th)
	for i := 0; i < 120; i++ {
		slow.Frame(ss, seconds/120)
	}

	fast := New(2)
	fast.Resize(tw, th)
	fs := canvas.NewSurface(tw, th)
	for i := 0; i < 240; i++ {
		fast.Frame(fs, seconds/240)
	}

	// Exact equality is not on offer: the two runs accumulate a different
	// number of floating point additions. A thousandth of a radian is far below
	// anything the eye could see and far below one frame's worth of turn.
	if math.Abs(slow.a-fast.a) > 1e-3 || math.Abs(slow.b-fast.b) > 1e-3 {
		t.Errorf("after %g seconds: 30fps reached %.4f,%.4f but 60fps reached %.4f,%.4f",
			seconds, slow.a, slow.b, fast.a, fast.b)
	}

	// And the rate must actually be per second: four seconds of the slower axis
	// has to be a visible amount of turn, or the comparison above would pass on
	// a torus that never moved.
	if math.Abs(slow.b-New(2).b) < 1 {
		t.Error("four seconds barely moved the pose; the rates are not per second")
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	_, a := render(7, 25)
	_, b := render(7, 25)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("same seed diverged at %d,%d", x, y)
			}
		}
	}
}

func TestStaysInsideTheSurface(t *testing.T) {
	// Set never writes out of bounds, so the real risk is the scale factor
	// letting the torus clip against the edges as it turns. Nothing should
	// reach the outermost ring of pixels.
	d := New(3)
	d.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 400; i++ {
		d.Frame(s, dt)
		for x := 0; x < tw; x++ {
			if s.At(x, 0) != tcell.ColorDefault || s.At(x, th-1) != tcell.ColorDefault {
				t.Fatalf("frame %d: the torus reached the top or bottom edge", i)
			}
		}
		for y := 0; y < th; y++ {
			if s.At(0, y) != tcell.ColorDefault || s.At(tw-1, y) != tcell.ColorDefault {
				t.Fatalf("frame %d: the torus reached the left or right edge", i)
			}
		}
	}
}

func TestOddSizesDoNotPanic(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 9}, {9, 2}, {17, 5}, {200, 120}} {
		d := New(1)
		d.Resize(sz[0], sz[1])
		s := canvas.NewSurface(sz[0], sz[1])
		d.Frame(s, dt)
	}
	// A Frame before any Resize must be a no-op rather than a nil dereference,
	// because callers other than canvas.Run may not follow the contract.
	New(1).Frame(canvas.NewSurface(4, 4), dt)
}

func TestFrameDoesNotAllocate(t *testing.T) {
	d := New(1)
	d.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	d.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { d.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per run; buffers belong in Resize", n)
	}
}
