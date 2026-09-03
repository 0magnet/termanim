package metaballs

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// dt is one frame at the rate these constants were tuned at, so every
// assertion about how far things move over N frames still means what it did.
const dt = 1.0 / 30

const tw, th = 48, 32

func TestBallsStayOnScreen(t *testing.T) {
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 500; i++ {
		m.Frame(s, dt)
		for j, b := range m.balls {
			if b.x < 0 || b.x > float64(tw) || b.y < 0 || b.y > float64(th) {
				t.Fatalf("frame %d: ball %d escaped to %.1f,%.1f", i, j, b.x, b.y)
			}
		}
	}
}

func TestHasBrightCoresAndEmptyEdges(t *testing.T) {
	// The compression should leave cores bright without saturating the whole
	// field, and the far background should stay unset so blobs float.
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	m.Frame(s, dt)
	var bright, empty int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			c := s.At(x, y)
			if c == tcell.ColorDefault {
				empty++
				continue
			}
			r, g, b := c.RGB()
			if r+g+b > 380 {
				bright++
			}
		}
	}
	if bright == 0 {
		t.Error("no bright core anywhere: the field never gets strong")
	}
	if empty == 0 {
		t.Error("nothing is left unset: the blobs fill the screen")
	}
}

func TestMoves(t *testing.T) {
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	m.Frame(s, dt)
	before := m.balls[0]
	m.Frame(s, dt)
	if m.balls[0].x == before.x && m.balls[0].y == before.y {
		t.Error("a ball did not move between frames")
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []ball {
		m := New(9)
		m.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 40; i++ {
			m.Frame(s, dt)
		}
		return m.balls
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at ball %d", i)
		}
	}
}

// The same wall-clock interval must move the blobs the same distance however
// it is divided into frames.
func TestFrameRateIndependent(t *testing.T) {
	run := func(frames int, step float64) []ball {
		m := New(3)
		m.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			m.Frame(s, step)
		}
		return m.balls
	}
	slow := run(60, 1.0/30)  // 2 seconds
	fast := run(120, 1.0/60) // the same 2 seconds
	for i := range slow {
		// Not exact: a bounce lands on a slightly different sub-pixel when the
		// step differs, and reflects from there. A pixel of tolerance over two
		// seconds still catches a rate that followed the frame count, which
		// would be out by a factor of two.
		if d := slow[i].x - fast[i].x; d > 1 || d < -1 {
			t.Errorf("ball %d drifted %.2f px apart in x over two seconds", i, d)
		}
		if d := slow[i].y - fast[i].y; d > 1 || d < -1 {
			t.Errorf("ball %d drifted %.2f px apart in y over two seconds", i, d)
		}
	}
}
