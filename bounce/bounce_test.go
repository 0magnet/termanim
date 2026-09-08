package bounce

import (
	"math"
	"testing"

	"github.com/0magnet/termanim/canvas"
)

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

// The one thing this animation must never do is lose the logo off an edge.
func TestLogoStaysInsideTheWindow(t *testing.T) {
	b := New(1)
	b.Resize(120, 80)
	s := canvas.NewSurface(120, 80)
	hw, hh := b.halfSize()
	for i := 0; i < 5000; i++ {
		b.Frame(s, 0.016)
		if b.x < hw-1e-6 || b.x > 120-hw+1e-6 {
			t.Fatalf("frame %d: x=%v escaped [%v,%v]", i, b.x, hw, 120-hw)
		}
		if b.y < hh-1e-6 || b.y > 80-hh+1e-6 {
			t.Fatalf("frame %d: y=%v escaped [%v,%v]", i, b.y, hh, 80-hh)
		}
	}
}

// A wall must reverse the logo, not absorb or stop it.
func TestWallsReverseTheHeading(t *testing.T) {
	b := New(0)
	b.Resize(60, 40)
	s := canvas.NewSurface(60, 40)
	// Aim it hard at the right wall.
	b.dx, b.dy = 1, 0
	b.x = 55
	before := b.dx
	for i := 0; i < 200 && b.dx == before; i++ {
		b.Frame(s, 0.016)
	}
	if b.dx >= 0 {
		t.Errorf("dx is %v after hitting the right wall; it should have gone negative", b.dx)
	}
	if math.Abs(b.dx) < 1e-9 {
		t.Error("the logo stopped at the wall instead of bouncing")
	}
}

// The color change on contact is the only feedback the animation gives.
func TestColorChangesOnBounce(t *testing.T) {
	b := New(0)
	b.Resize(60, 40)
	s := canvas.NewSurface(60, 40)
	b.dx, b.dy = 1, 0
	b.x = 55
	hue := b.hue
	for i := 0; i < 200 && b.hue == hue; i++ {
		b.Frame(s, 0.016)
	}
	if b.hue == hue {
		t.Error("the hue did not change on the bounce")
	}
	if b.hue < 0 || b.hue >= 1 {
		t.Errorf("hue left 0..1: %v", b.hue)
	}
}

// Hitting the middle of an edge is not a corner. If this fails the counter is
// meaningless, because it would tick on every ordinary bounce.
func TestAnEdgeHitIsNotACorner(t *testing.T) {
	b := New(0)
	b.Resize(200, 200)
	s := canvas.NewSurface(200, 200)
	// Straight across the middle, perpendicular to the left and right walls:
	// this can never touch a corner.
	b.x, b.y = 100, 100
	b.dx, b.dy = 1, 0
	for i := 0; i < 4000; i++ {
		b.Frame(s, 0.016)
	}
	if b.Corners != 0 {
		t.Errorf("counted %d corners on a path that only ever hits edge midpoints", b.Corners)
	}
}

// And two walls together is one. This is the payoff the whole thing exists
// for, so it must actually fire.
func TestTwoWallsTogetherIsACorner(t *testing.T) {
	b := New(0)
	b.Resize(100, 100)
	s := canvas.NewSurface(100, 100)
	hw, hh := b.halfSize()
	// Put it a hair from the bottom-right corner heading into it.
	b.x, b.y = 100-hw-0.1, 100-hh-0.1
	b.dx, b.dy = 1, 1
	b.Frame(s, 0.05)
	if b.Corners != 1 {
		t.Fatalf("Corners is %d after driving into a corner, want 1", b.Corners)
	}
	if b.flash <= 0 {
		t.Error("the screen did not flash on a corner")
	}
}

// One corner is one corner. Both axes flipping in a single frame must not be
// counted twice.
func TestACornerIsCountedOnce(t *testing.T) {
	b := New(0)
	b.Resize(100, 100)
	s := canvas.NewSurface(100, 100)
	hw, hh := b.halfSize()
	b.x, b.y = 100-hw-0.1, 100-hh-0.1
	b.dx, b.dy = 1, 1
	b.Frame(s, 0.05)
	if b.Corners != 1 {
		t.Errorf("Corners is %d, want exactly 1", b.Corners)
	}
}

// The flash fades rather than latching on.
func TestFlashFades(t *testing.T) {
	b := New(0)
	b.Resize(100, 100)
	s := canvas.NewSurface(100, 100)
	hw, hh := b.halfSize()
	b.x, b.y = 100-hw-0.1, 100-hh-0.1
	b.dx, b.dy = 1, 1
	b.Frame(s, 0.05)
	if b.flash <= 0 {
		t.Fatal("no flash to fade")
	}
	for i := 0; i < 200; i++ {
		b.Frame(s, 0.016)
	}
	if b.flash > 0 {
		t.Errorf("flash still %v after three seconds", b.flash)
	}
}

// There must be a logo, and it must have the slot knocked out of it — that is
// what stops it reading as a featureless blob.
func TestLogoIsDrawnAndHasASlot(t *testing.T) {
	b := New(0)
	b.Resize(120, 80)
	s := canvas.NewSurface(120, 80)
	b.Frame(s, 0)
	if n := lit(s); n < 50 {
		t.Errorf("only %d pixels drawn", n)
	}
	// The center of the mark is inside the slot and must be empty.
	if _, ok := logo(0, 0, 1); ok {
		t.Error("the middle of the logo is filled; the slot is missing")
	}
	// And a point above the slot must be solid.
	if _, ok := logo(0, -0.5, 1); !ok {
		t.Error("the body of the logo is not filled")
	}
}

// A resize must move the logo proportionally rather than leaving it outside
// the new window, which would strand it against an edge for good.
func TestResizeKeepsTheLogoInside(t *testing.T) {
	b := New(3)
	b.Resize(200, 100)
	s := canvas.NewSurface(200, 100)
	for i := 0; i < 300; i++ {
		b.Frame(s, 0.016)
	}
	b.Resize(40, 30)
	hw, hh := b.halfSize()
	if b.x < hw-1e-6 || b.x > 40-hw+1e-6 || b.y < hh-1e-6 || b.y > 30-hh+1e-6 {
		t.Errorf("after shrinking, the logo is at %v,%v outside the new window", b.x, b.y)
	}
}

func TestFrameDoesNotAllocate(t *testing.T) {
	b := New(0)
	b.Resize(120, 60)
	s := canvas.NewSurface(120, 60)
	b.Frame(s, 0.016)
	if n := testing.AllocsPerRun(50, func() { b.Frame(s, 0.016) }); n != 0 {
		t.Errorf("Frame allocated %v times per run", n)
	}
}

// Motion scaled by elapsed time, not by frame: one long step must land where
// several short ones do.
func TestAdvancesByElapsedTimeNotByFrame(t *testing.T) {
	mk := func() (*Bounce, *canvas.Surface) {
		b := New(0)
		b.Resize(400, 400)
		b.x, b.y, b.dx, b.dy = 200, 200, 0.6, 0.8
		return b, canvas.NewSurface(400, 400)
	}
	a, sa := mk()
	a.Frame(sa, 0.4)

	c, sc := mk()
	for i := 0; i < 4; i++ {
		c.Frame(sc, 0.1)
	}
	if math.Abs(a.x-c.x) > 1e-9 || math.Abs(a.y-c.y) > 1e-9 {
		t.Errorf("one 0.4s step gave %v,%v; four 0.1s steps gave %v,%v", a.x, a.y, c.x, c.y)
	}
}

func TestTinyWindowDoesNotPanic(t *testing.T) {
	for _, d := range [][2]int{{1, 1}, {4, 2}, {2, 40}, {40, 2}} {
		b := New(0)
		b.Resize(d[0], d[1])
		s := canvas.NewSurface(d[0], d[1])
		for i := 0; i < 20; i++ {
			b.Frame(s, 0.016)
		}
	}
}
