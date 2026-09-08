package atom

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 96, 64

// dt is the step the assertions below are calibrated to.
const dt = 1.0 / 30

func render(seed int64, frames int) *canvas.Surface {
	a := New(seed)
	a.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		a.Frame(s, dt)
	}
	return s
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
	s := render(1, 1)
	// Three orbit ellipses nearly as wide as the window, plus a nucleus:
	// hundreds of pixels. A handful would mean the projection or the scale had
	// collapsed.
	if n := setPixels(s); n < 200 {
		t.Fatalf("only %d pixels drawn out of %d; the atom is missing or tiny", n, tw*th)
	}
}

// TestNucleusIsWarmAndOrbitsAreCool is the thing that makes the picture
// readable as an atom rather than as three ellipses: one warm object in the
// middle, cool scaffolding around it.
func TestNucleusIsWarmAndOrbitsAreCool(t *testing.T) {
	s := render(1, 1)
	var warm, cool int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			c := s.At(x, y)
			if c == tcell.ColorDefault {
				continue
			}
			r, _, b := c.RGB()
			near := math.Hypot(float64(x)-tw/2, float64(y)-th/2) < 6
			switch {
			case near && r > b:
				warm++
			case !near && b > r:
				cool++
			}
		}
	}
	if warm == 0 {
		t.Error("no warm pixels in the middle; the nucleus is missing")
	}
	if cool == 0 {
		t.Error("no cool pixels away from the middle; the orbits are missing")
	}
}

// TestEdgeOnOrbitDoesNotBlowOut is the regression for the sample weighting in
// drawOrbits.
//
// A ring is sampled at a fixed number of angles whatever angle it is seen
// from, so foreshortening does not thin the samples out — it piles them into
// fewer pixels. Light adds here, so without weighting each sample by how far
// it moved on screen, the same orbit is a faint ellipse face on and a
// saturated white bar edge on. The two should be within a small factor of one
// another instead.
func TestEdgeOnOrbitDoesNotBlowOut(t *testing.T) {
	identity := func(x, y, z float64) [3]float64 { return [3]float64{x, y, z} }

	peak := func(s shell) float64 {
		a := New(1)
		a.shells = []shell{s}
		a.Resize(tw, th)
		a.drawOrbits(identity)
		max := 0.0
		for _, v := range a.acc {
			if v > max {
				max = v
			}
		}
		return max
	}

	// A circle in the plane of the screen, and the same circle turned a
	// quarter turn so it projects to a line.
	faceOn := peak(shell{ux: 1, vy: 1, radius: 1})
	edgeOn := peak(shell{ux: 1, vz: 1, radius: 1})

	if faceOn <= 0 || edgeOn <= 0 {
		t.Fatalf("nothing drawn: face on %v, edge on %v", faceOn, edgeOn)
	}
	// Edge on is legitimately brighter — more of the thread lies under each
	// pixel — but by a little, not by the sample count.
	if edgeOn > 4*faceOn {
		t.Errorf("edge-on orbit peaks at %v against %v face on; the samples are piling up",
			edgeOn, faceOn)
	}
}

// TestSameSeedSameFrame keeps the animation reproducible: the demo pages hand
// a seed in, and a seed that did not determine the picture would make every
// screenshot and every bug report a one-off.
func TestSameSeedSameFrame(t *testing.T) {
	a := render(7, 40)
	b := render(7, 40)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("seed 7 diverged at (%d,%d): %v vs %v", x, y, a.At(x, y), b.At(x, y))
			}
		}
	}
}

// TestDegenerateSizes covers the sizes a host really does hand over while a
// pane is being laid out.
func TestDegenerateSizes(t *testing.T) {
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {0, 20}, {20, 0}, {3, 2}} {
		a := New(1)
		a.Resize(sz[0], sz[1])
		s := canvas.NewSurface(maxInt(sz[0], 1), maxInt(sz[1], 1))
		for i := 0; i < 5; i++ {
			a.Frame(s, dt)
		}
	}
}

// TestElectronsMove is what separates this from a still diagram.
func TestElectronsMove(t *testing.T) {
	a := New(1)
	a.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	a.Frame(s, dt)
	before := append([]float64(nil), a.shells[0].phase...)
	for i := 0; i < 15; i++ {
		a.Frame(s, dt)
	}
	if math.Abs(a.shells[0].phase[0]-before[0]) < 1e-6 {
		t.Error("the electron on the first shell did not move in half a second")
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
