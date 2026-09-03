package rain

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 48

// dt is the step these tests advance by: a thirtieth of a second, so a count of
// frames here means the same amount of falling it always did.
const dt = 1.0 / 30

// lum is a rough brightness of a pixel, enough to compare two of them.
func lum(c tcell.Color) int {
	if c == tcell.ColorDefault {
		return 0
	}
	r, g, b := c.RGB()
	return int(r + g + b)
}

func TestDropsFallDownward(t *testing.T) {
	r := New(1)
	r.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	before := make([]float64, len(r.drops))
	for i, d := range r.drops {
		before[i] = d.y
	}
	r.Frame(s, dt)
	for i, d := range r.drops {
		// A drop that recycled this frame is back at the top, which is the only
		// legitimate way for y to decrease.
		if d.y <= before[i] && d.y > 0 {
			t.Fatalf("drop %d went from y=%.2f to y=%.2f: rain does not go up", i, before[i], d.y)
		}
	}
}

func TestDropsRecycleRatherThanEscape(t *testing.T) {
	r := New(2)
	r.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 600; i++ {
		r.Frame(s, dt)
		for j, d := range r.drops {
			if d.y > th+d.length {
				t.Fatalf("frame %d: drop %d fell to %.1f on a %d-pixel surface", i, j, d.y, th)
			}
			if d.x < 0 || d.x >= tw {
				t.Fatalf("frame %d: drop %d drifted to x=%.1f outside a %d-pixel surface", i, j, d.x, tw)
			}
		}
	}
}

func TestNearerDropsFallFasterAndBrighter(t *testing.T) {
	// The parallax is the whole effect: without it this is falling dashes.
	r := New(3)
	r.Resize(tw, th)
	var far, near drop
	for _, d := range r.drops {
		if d.depth < far.depth || far.vy == 0 {
			far = d
		}
		if d.depth > near.depth {
			near = d
		}
	}
	if near.vy <= far.vy {
		t.Errorf("near drop (depth %.2f) falls at %.2f, far one (depth %.2f) at %.2f",
			near.depth, near.vy, far.depth, far.vy)
	}
	if r.brightness(near) <= r.brightness(far) {
		t.Errorf("near drop is not brighter: %.2f against %.2f",
			r.brightness(near), r.brightness(far))
	}
	if near.length <= far.length {
		t.Errorf("near drop streaks %.2f, far one %.2f: the blur does not follow speed",
			near.length, far.length)
	}
}

func TestNearerDropsDrawBrighter(t *testing.T) {
	// The same property, but as it reaches the screen: put one far drop and one
	// near drop in separate halves and compare the brightest pixel of each.
	r := New(4)
	r.Resize(tw, th)
	r.drops = []drop{
		{x: 12, y: 10, vy: 30, slant: 0.2, length: 2, depth: 0.05},
		{x: 44, y: 10, vy: 90, slant: 0.2, length: 5, depth: 0.95},
	}
	s := canvas.NewSurface(tw, th)
	r.Frame(s, dt)

	brightest := func(x0, x1 int) int {
		best := 0
		for y := 0; y < th; y++ {
			for x := x0; x < x1; x++ {
				if v := lum(s.At(x, y)); v > best {
					best = v
				}
			}
		}
		return best
	}
	f, n := brightest(0, tw/2), brightest(tw/2, tw)
	if n <= f {
		t.Errorf("far half peaks at %d, near half at %d: depth is not shading the drops", f, n)
	}
}

func TestStreaksAreLongerThanADot(t *testing.T) {
	// A drop is drawn as its motion blur. If it were a dot the column it fell
	// in would only ever hold one lit pixel.
	r := New(5)
	r.Resize(tw, th)
	r.drops = []drop{{x: 30, y: 24, vy: 120, slant: 0, length: 5, depth: 0.9}}
	s := canvas.NewSurface(tw, th)
	r.Frame(s, dt)
	var lit int
	for y := 0; y < th; y++ {
		if s.At(30, y) != tcell.ColorDefault {
			lit++
		}
	}
	if lit < 3 {
		t.Errorf("the drop lit %d pixels in its column: that is a dot, not a streak", lit)
	}
}

func TestSplashesAppearAtTheBottomAndExpire(t *testing.T) {
	r := New(6)
	r.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	seen := false
	for i := 0; i < 200; i++ {
		r.Frame(s, dt)
		if len(r.splashes) > 0 {
			seen = true
		}
		if len(r.splashes) > cap(r.splashes) {
			t.Fatalf("frame %d: splashes outgrew their allocation", i)
		}
		for _, sp := range r.splashes {
			if sp.age >= r.SplashLife {
				t.Fatalf("frame %d: a splash lived %.3fs, past the %.3fs limit",
					i, sp.age, r.SplashLife)
			}
		}
	}
	if !seen {
		t.Error("no drop ever splashed: the rain does not land anywhere")
	}
}

func TestFrameDrawsSomething(t *testing.T) {
	r := New(7)
	r.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	r.Frame(s, dt)
	var lit int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Fatal("the surface is empty")
	}
	if lit == tw*th {
		t.Fatal("every pixel is lit: this is a wall of water, not rain")
	}
}

func TestDensityScalesWithTheSurface(t *testing.T) {
	// A fixed drop count is a drizzle in a big window and a downpour in a small
	// one; the count has to follow the area.
	small, big := New(8), New(8)
	small.Resize(40, 20)
	big.Resize(160, 80)
	if len(big.drops) <= len(small.drops) {
		t.Errorf("%d drops in a big window against %d in a small one",
			len(big.drops), len(small.drops))
	}
}

func TestFallsAtTheSameRateAtAnyFrameRate(t *testing.T) {
	// The point of taking dt: one second of rain is one second of rain whether
	// it arrives in thirty frames or sixty. Both runs get the same single drop,
	// placed so it neither lands nor recycles inside the second.
	fall := func(step float64, frames int) float64 {
		r := New(1)
		r.Resize(tw, th)
		r.drops = []drop{{x: 32, y: 0, vy: 40, slant: 0.3, length: 2, depth: 0.8}}
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			r.Frame(s, step)
		}
		return r.drops[0].y
	}
	slow, fast := fall(1.0/30, 30), fall(1.0/60, 60)
	if diff := slow - fast; diff > 0.01 || diff < -0.01 {
		t.Errorf("a drop fell to %.4f in thirty frames but %.4f in sixty: "+
			"the speed still depends on the frame rate", slow, fast)
	}
	if slow < 39 || slow > 41 {
		t.Errorf("a drop at forty pixels a second fell %.2f pixels in a second", slow)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []drop {
		r := New(9)
		r.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 80; i++ {
			r.Frame(s, dt)
		}
		return r.drops
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at drop %d", i)
		}
	}
}
