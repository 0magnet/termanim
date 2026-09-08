package parrot

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// frame runs the animation forward by dt and returns the surface.
func frame(p *Parrot, s *canvas.Surface, dt float64) *canvas.Surface {
	p.Frame(s, dt)
	return s
}

func lit(s *canvas.Surface) int {
	w, h := s.Size()
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if s.At(x, y) != 0 {
				n++
			}
		}
	}
	return n
}

// There has to be a bird. A shape test this loose is worth having because
// every later test assumes something is drawn at all.
func TestDrawsABird(t *testing.T) {
	p := New(0)
	p.Resize(80, 60)
	s := canvas.NewSurface(80, 60)
	if n := lit(frame(p, s, 0)); n < 200 {
		t.Errorf("only %d pixels drawn; the bird is missing or tiny", n)
	}
}

// The whole joke is that the color runs around the hue wheel. If the body
// color at half a period is the same as at the start, it is not a party.
func TestHueTurnsOncePerPeriod(t *testing.T) {
	p := New(0)
	p.Period = 1
	p.Resize(80, 60)
	s := canvas.NewSurface(80, 60)

	sample := func() tcell.Color {
		// The crown, well above the beak and away from the eye, is body hue.
		return s.At(40, 22)
	}
	frame(p, s, 0)
	start := sample()

	frame(p, s, 0.5)
	half := sample()
	if half == start {
		t.Error("the color did not change over half a period")
	}

	// Back to where it began after a full turn. Sampled loosely: the pose
	// returns too, so the same pixel is the same part of the bird.
	frame(p, s, 0.5)
	if end := sample(); end != start {
		t.Errorf("after a full period the color is %v, was %v", end, start)
	}
}

// The bird rolls. Two poses half a period apart must differ, or it is a
// static image with a color cycle on it.
func TestItActuallyMoves(t *testing.T) {
	p := New(0)
	p.Period = 1
	p.Value, p.Saturation = 1, 0 // hold color constant so only pose can differ
	p.Resize(80, 60)
	s := canvas.NewSurface(80, 60)

	frame(p, s, 0)
	a := lit(s)
	occupied := make([]bool, 0, 8)
	w, h := s.Size()
	for y := 0; y < h; y += 8 {
		for x := 0; x < w; x += 8 {
			occupied = append(occupied, s.At(x, y) != 0)
		}
	}

	frame(p, s, 0.25)
	changed := false
	i := 0
	for y := 0; y < h; y += 8 {
		for x := 0; x < w; x += 8 {
			if (s.At(x, y) != 0) != occupied[i] {
				changed = true
			}
			i++
		}
	}
	if !changed {
		t.Errorf("the pose is identical a quarter period later (%d pixels then, %d now)", a, lit(s))
	}
}

// Count is what turns the emoji into the wall, so it must actually multiply
// the birds rather than just widening one.
func TestCountDrawsSeparateBirds(t *testing.T) {
	one := New(0)
	one.Resize(160, 40)
	s1 := canvas.NewSurface(160, 40)
	frame(one, s1, 0)

	many := New(0)
	many.Count = 4
	many.Resize(160, 40)
	s4 := canvas.NewSurface(160, 40)
	frame(many, s4, 0)

	// Four birds in the same window are each smaller, but together they must
	// cover more of it than one bird does.
	if lit(s4) <= lit(s1) {
		t.Errorf("four parrots lit %d pixels, one lit %d", lit(s4), lit(s1))
	}

	// And they must be in four places: check that each quarter has something
	// in it, which a single centered bird would fail.
	for q := 0; q < 4; q++ {
		found := false
		for y := 0; y < 40 && !found; y++ {
			for x := q * 40; x < (q+1)*40; x++ {
				if s4.At(x, y) != 0 {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("quarter %d of the window is empty", q)
		}
	}
}

// Spread is what makes a row do a wave instead of moving as one bird.
func TestSpreadOffsetsNeighbors(t *testing.T) {
	same := New(0)
	same.Count, same.Spread = 2, 0
	same.Resize(160, 40)
	s := canvas.NewSurface(160, 40)
	frame(same, s, 0)

	leftCol, rightCol := columnProfile(s, 0, 80), columnProfile(s, 80, 160)
	if leftCol != rightCol {
		t.Errorf("with Spread 0 the two birds differ: %d vs %d", leftCol, rightCol)
	}

	off := New(0)
	off.Count, off.Spread = 2, 0.25
	off.Resize(160, 40)
	s2 := canvas.NewSurface(160, 40)
	frame(off, s2, 0)
	if columnProfile(s2, 0, 80) == columnProfile(s2, 80, 160) {
		t.Error("with Spread set the two birds are still in the same pose")
	}
}

func columnProfile(s *canvas.Surface, x0, x1 int) int {
	_, h := s.Size()
	n := 0
	for y := 0; y < h; y++ {
		for x := x0; x < x1; x++ {
			if s.At(x, y) != 0 {
				n += x - x0 + y
			}
		}
	}
	return n
}

// Frame runs thirty times a second for as long as someone leaves it up. The
// other animations in this repository hold to no allocation per frame and
// this one has no excuse either — it has no buffers at all.
func TestFrameDoesNotAllocate(t *testing.T) {
	p := New(0)
	p.Count = 3
	p.Resize(120, 60)
	s := canvas.NewSurface(120, 60)
	p.Frame(s, 0.016) // first frame, let anything one-off happen

	if n := testing.AllocsPerRun(50, func() { p.Frame(s, 0.016) }); n != 0 {
		t.Errorf("Frame allocated %v times per run", n)
	}
}

// Left unattended for hours the phase must not drift into the range where a
// float64 stops resolving a frame, and the bird must not wander off screen.
func TestPhaseStaysBounded(t *testing.T) {
	p := New(0)
	p.Resize(60, 40)
	s := canvas.NewSurface(60, 40)
	for i := 0; i < 20000; i++ {
		p.Frame(s, 0.016)
	}
	if p.phase < 0 || p.phase >= 1 {
		t.Errorf("phase escaped 0..1: %v", p.phase)
	}
	if n := lit(s); n < 100 {
		t.Errorf("after a long run only %d pixels are drawn", n)
	}
}

// A window too small to hold the bird must clip it rather than write outside
// the surface — Surface ignores out-of-bounds writes, so the failure this
// guards against is a panic in the bounds arithmetic, not a corrupt buffer.
func TestTinyWindowDoesNotPanic(t *testing.T) {
	for _, d := range [][2]int{{1, 1}, {4, 2}, {2, 40}, {40, 2}} {
		p := New(0)
		p.Count = 3
		p.Resize(d[0], d[1])
		s := canvas.NewSurface(d[0], d[1])
		p.Frame(s, 0.016)
	}
}

// The beak points the way the bird faces. If it ever renders behind the head
// the bird has been mirrored, which is the kind of thing that survives review
// because it still looks like a parrot.
func TestBeakIsInFrontOfTheFace(t *testing.T) {
	// Sampled in local space directly, so the test does not depend on the
	// pose the phase happens to be in.
	body := tcell.NewRGBColor(10, 200, 10)
	crest := tcell.NewRGBColor(10, 150, 10)
	if _, ok := shade(-0.9, -0.1, body, crest); !ok {
		t.Error("nothing drawn where the beak should be, ahead of the face")
	}
	if c, _ := shade(0.9, -0.1, body, crest); c == tcell.NewRGBColor(238, 222, 178) {
		t.Error("beak color found behind the head: the bird is mirrored")
	}
}

// The eye has to be an eye. A dark disc with no highlight reads as a hole.
func TestEyeHasAHighlight(t *testing.T) {
	body := tcell.NewRGBColor(10, 200, 10)
	crest := body
	dark, light := 0, 0
	for dy := -0.25; dy <= 0.25; dy += 0.01 {
		for dx := -0.25; dx <= 0.25; dx += 0.01 {
			c, ok := shade(-0.30+dx, -0.28+dy, body, crest)
			if !ok {
				continue
			}
			r, g, b := c.RGB()
			switch {
			case r > 240 && g > 240 && b > 240:
				light++
			case r < 40 && g < 40 && b < 40:
				dark++
			}
		}
	}
	if dark == 0 {
		t.Error("no dark pupil")
	}
	if light == 0 {
		t.Error("no highlight in the eye")
	}
	if light >= dark {
		t.Errorf("the highlight (%d) is not smaller than the pupil (%d)", light, dark)
	}
}

// hsv is written out by hand here, so it is worth checking it against the
// three corners everyone gets wrong.
func TestHSVPrimaries(t *testing.T) {
	for _, c := range []struct {
		h          float64
		r, g, b    int32
		whatItIs   string
		saturation float64
	}{
		{0, 255, 0, 0, "red", 1},
		{1.0 / 3, 0, 255, 0, "green", 1},
		{2.0 / 3, 0, 0, 255, "blue", 1},
		{0.5, 255, 255, 255, "white at zero saturation", 0},
	} {
		r, g, b := hsv(c.h, c.saturation, 1).RGB()
		if r != c.r || g != c.g || b != c.b {
			t.Errorf("%s: got %d,%d,%d want %d,%d,%d", c.whatItIs, r, g, b, c.r, c.g, c.b)
		}
	}
	// And it must wrap rather than clamp, since phase is added to freely.
	if hsv(1.25, 1, 1) != hsv(0.25, 1, 1) {
		t.Error("hue does not wrap")
	}
}

func TestLightenMovesTowardWhiteAndBlack(t *testing.T) {
	mid := tcell.NewRGBColor(100, 100, 100)
	r, _, _ := lighten(mid, 0.5).RGB()
	if r <= 100 {
		t.Errorf("lighten(+) gave %d, not brighter than 100", r)
	}
	r2, _, _ := lighten(mid, -0.5).RGB()
	if r2 >= 100 {
		t.Errorf("lighten(-) gave %d, not darker than 100", r2)
	}
	// Neither direction may leave the byte range.
	for _, amt := range []float64{5, -5} {
		rr, gg, bb := lighten(tcell.NewRGBColor(200, 10, 250), amt).RGB()
		for _, v := range []int32{rr, gg, bb} {
			if v < 0 || v > 255 {
				t.Errorf("amt %v produced channel %d", amt, v)
			}
		}
	}
}

// Motion is scaled by dt everywhere, which is what lets the frame rate change
// without changing how fast the bird appears to dance. Two small steps must
// land in the same place as one big one.
func TestAdvancesByElapsedTimeNotByFrame(t *testing.T) {
	a := New(0)
	a.Resize(40, 30)
	sa := canvas.NewSurface(40, 30)
	a.Frame(sa, 0.4)

	b := New(0)
	b.Resize(40, 30)
	sb := canvas.NewSurface(40, 30)
	for i := 0; i < 4; i++ {
		b.Frame(sb, 0.1)
	}

	if math.Abs(a.phase-b.phase) > 1e-9 {
		t.Errorf("one step of 0.4s gave phase %v, four of 0.1s gave %v", a.phase, b.phase)
	}
}
