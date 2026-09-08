package magnetosphere

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 120, 68

// dt is the step the assertions below are calibrated to.
const dt = 1.0 / 30

func render(frames int) (*Magnetosphere, *canvas.Surface) {
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		m.Frame(s, dt)
	}
	return m, s
}

// lum is the pixel's brightness, 0 to 1.
func lum(s *canvas.Surface, x, y int) float64 {
	c := s.At(x, y)
	if c == tcell.ColorDefault {
		return 0
	}
	r, g, b := c.RGB()
	return float64(r+g+b) / (3 * 255)
}

func TestDrawsBothColors(t *testing.T) {
	_, s := render(1)
	var light, dark int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if lum(s, x, y) > 0.6 {
				light++
			} else if lum(s, x, y) < 0.2 {
				dark++
			}
		}
	}
	// Roughly half and half. All of one is a blank frame; a handful of the
	// other is a pattern that has collapsed to a single band.
	if light < tw*th/8 || dark < tw*th/8 {
		t.Fatalf("light %d dark %d of %d; the pattern has collapsed", light, dark, tw*th)
	}
}

// TestSilhouetteIsTheHyperbola ties the outline to the geometry.
//
// The discriminant of the ray test decides which side of the funnel's edge a
// point is on, and the whole design rests on that working out to the hyperbola
// u² = (v*Slope)² + Waist². If the two ever disagree the picture grows an edge
// where nothing is.
func TestSilhouetteIsTheHyperbola(t *testing.T) {
	m, _ := render(1)
	for i := 0; i <= 40; i++ {
		v := -1 + 2*float64(i)/40
		want := math.Hypot(v*m.Slope, m.Waist)
		// Just inside and just outside the predicted edge.
		if m.disc(want*0.98, v) < 0 {
			t.Errorf("v=%.2f: no funnel just inside the predicted edge %.4f", v, want)
		}
		if m.disc(want*1.02, v) >= 0 {
			t.Errorf("v=%.2f: funnel just outside the predicted edge %.4f", v, want)
		}
	}
}

// TestWaistIsNarrowest is what makes it an hourglass rather than a cone.
func TestWaistIsNarrowest(t *testing.T) {
	m, _ := render(1)
	edge := func(v float64) float64 {
		// Walk out until the ray stops hitting.
		x := 0.0
		for ; x < 8; x += 0.001 {
			if m.disc(x, v) < 0 {
				break
			}
		}
		return x
	}
	at0 := edge(0)
	if at0 <= 0 {
		t.Fatal("the funnel has no width at the waist")
	}
	for _, v := range []float64{0.2, 0.5, 0.8, 1.0} {
		if edge(v) <= at0 {
			t.Errorf("half-width at v=%.1f is %.3f, not wider than %.3f at the waist",
				v, edge(v), at0)
		}
		if edge(-v) <= at0 {
			t.Errorf("half-width at v=%.1f is %.3f, not wider than %.3f at the waist",
				-v, edge(-v), at0)
		}
	}
}

// bestShift returns how far the pattern moved between two columns of pixels,
// in pixels, positive downward.
//
// The negation is not a fudge: matching a[i] against b[i-sh] is best at
// sh = -d when the pattern has moved down by d, because the offset that lines
// the samples up runs against the direction the pattern went.
func bestShift(a, b []float64) int {
	best, bestErr := 0, math.Inf(1)
	for sh := -8; sh <= 8; sh++ {
		e, n := 0.0, 0
		for i := range a {
			j := i - sh
			if j < 0 || j >= len(b) {
				continue
			}
			d := a[i] - b[j]
			e += d * d
			n++
		}
		if n < len(a)/2 {
			continue
		}
		if e /= float64(n); e < bestErr {
			best, bestErr = sh, e
		}
	}
	return -best
}

// TestCounterScroll is the animation the logo was asked for: the funnel goes
// up while the field goes down. Both directions are asserted from the pixels,
// not from the phase variables, because the sign of a phase is an
// implementation detail and the direction on screen is not.
func TestCounterScroll(t *testing.T) {
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)

	column := func(x int) []float64 {
		out := make([]float64, th)
		for y := range out {
			out[y] = lum(s, x, y)
		}
		return out
	}

	m.Frame(s, dt)
	// Just off the middle: the waist row itself is the one place the funnel
	// does not move, being where every band coordinate passes through zero.
	funnelBefore := column(tw / 2)
	fieldBefore := column(1)

	// Half a second, which at the default rates is a third of a band — far
	// enough to see and not so far that the match could have wrapped.
	for i := 0; i < 15; i++ {
		m.Frame(s, dt)
	}
	funnelShift := bestShift(funnelBefore, column(tw/2))
	fieldShift := bestShift(fieldBefore, column(1))

	if funnelShift >= 0 {
		t.Errorf("the funnel moved %d pixels; it should be going up (negative)", funnelShift)
	}
	if fieldShift <= 0 {
		t.Errorf("the field moved %d pixels; it should be going down (positive)", fieldShift)
	}
}

// TestPitchHeldAcrossSizes is the regression for measuring the bands in pixels
// rather than counting them over the height.
//
// A count fixed in screen-independent units gives a thirty-row terminal bands
// under three pixels tall, which is a moire and not a picture. Holding the
// pitch means a small window shows fewer bands, each still legible.
func TestPitchHeldAcrossSizes(t *testing.T) {
	for _, h := range []int{48, 68, 140, 300} {
		m := New(1)
		w := h * 2
		m.Resize(w, h)
		s := canvas.NewSurface(w, h)
		m.Frame(s, dt)

		// Count light-to-dark crossings down the outermost column, where the
		// field's stripes are straight and FieldPitch is meant to hold.
		edges := 0
		for y := 1; y < h; y++ {
			if (lum(s, 0, y-1) > 0.5) != (lum(s, 0, y) > 0.5) {
				edges++
			}
		}
		if edges < 2 {
			t.Errorf("h=%d: only %d band edges down the left column", h, edges)
			continue
		}
		// Two edges to a band.
		got := float64(h) / (float64(edges) / 2)
		if got < m.FieldPitch*0.6 || got > m.FieldPitch*1.7 {
			t.Errorf("h=%d: band pitch %.1f px, want near FieldPitch %.1f",
				h, got, m.FieldPitch)
		}
	}
}

// TestDegenerateSizes covers the sizes a host really does hand over while a
// pane is being laid out.
func TestDegenerateSizes(t *testing.T) {
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {0, 20}, {20, 0}, {3, 2}} {
		m := New(1)
		m.Resize(sz[0], sz[1])
		s := canvas.NewSurface(max(sz[0], 1), max(sz[1], 1))
		for i := 0; i < 5; i++ {
			m.Frame(s, dt)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
