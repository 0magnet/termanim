package fire

import (
	"testing"

	"github.com/0magnet/termanim/canvas"
)

// dt is one frame at the rate these constants were tuned at, so every
// assertion about how far things move over N frames still means what it did.
const dt = 1.0 / 30

const tw, th = 40, 24

func settled(frames int) *Fire {
	f := New(1)
	f.Resize(tw, th)
	for i := 0; i < frames; i++ {
		f.step()
	}
	return f
}

func TestBurns(t *testing.T) {
	f := settled(200)
	var lit int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if f.heat[y][x] > 0 {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Fatal("nothing is lit: the fire never caught")
	}
	if lit == tw*th {
		t.Fatal("everything is lit: the fire is not cooling")
	}
}

func TestHotterAtTheBottom(t *testing.T) {
	f := settled(200)
	mean := func(y int) float64 {
		var s int
		for x := 0; x < tw; x++ {
			s += int(f.heat[y][x])
		}
		return float64(s) / tw
	}
	if b, top := mean(th-1), mean(0); b <= top {
		t.Fatalf("not hotter at the bottom: bottom=%.1f top=%.1f", b, top)
	}
}

func TestFlameHasATop(t *testing.T) {
	f := settled(200)
	var lit int
	for x := 0; x < tw; x++ {
		if f.heat[0][x] > 0 {
			lit++
		}
	}
	if lit == tw {
		t.Error("the top row is fully lit; the decay scaling is wrong")
	}
}

func TestKeepsMoving(t *testing.T) {
	f := settled(200)
	before := make([]byte, tw)
	copy(before, f.heat[th/2])
	f.step()
	for x := 0; x < tw; x++ {
		if f.heat[th/2][x] != before[x] {
			return
		}
	}
	t.Error("a frame changed nothing: the animation is static")
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	a, b := settled(50), settled(50)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.heat[y][x] != b.heat[y][x] {
				t.Fatalf("same seed diverged at %d,%d", x, y)
			}
		}
	}
}

func TestFrameWritesToTheSurface(t *testing.T) {
	f := settled(200)
	s := canvas.NewSurface(tw, th)
	f.Frame(s, dt)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != 0 {
				return
			}
		}
	}
	t.Error("Frame left the surface empty")
}

// The cooling rule is a discrete step, so frame-rate independence means the
// same wall-clock interval runs the same number of steps however it is divided
// into frames.
func TestFrameRateIndependent(t *testing.T) {
	run := func(frames int, step float64) *Fire {
		f := New(1)
		f.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			f.Frame(s, step)
		}
		return f
	}
	slow := run(60, 1.0/30)  // 2 seconds
	fast := run(120, 1.0/60) // the same 2 seconds

	// Same seed and the same number of steps means the grids must match
	// exactly — a stronger check than counting steps, because it also catches
	// a step being run with different inputs.
	var lit int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if slow.heat[y][x] != fast.heat[y][x] {
				t.Fatalf("two seconds gave different fires: at %d,%d %d at 30fps but %d at 60fps",
					x, y, slow.heat[y][x], fast.heat[y][x])
			}
			if slow.heat[y][x] > 0 {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Error("nothing burned, so the comparison proved nothing")
	}
}

// reachOn reports how far up a surface h tall the flame climbs, as a fraction.
func reachOn(t *testing.T, h int) float64 {
	t.Helper()
	f := New(1)
	f.Resize(80, h)
	for i := 0; i < 400; i++ {
		f.step()
	}
	for y := 0; y < h; y++ {
		for x := 0; x < 80; x++ {
			if f.heat[y][x] > 0 {
				return float64(h-y) / float64(h)
			}
		}
	}
	return 0
}

// The flame has to climb most of the surface, at every size it is drawn at.
//
// It used to be tuned as a rate of cooling per row, which fixes the flame's
// height in rows rather than as a share of the view: it filled a short terminal
// and sat in the bottom third of a browser window with a screen's worth of
// black above it.
//
// The bound is a range and not a number because the closed form that sizes the
// cooling gets the shape right but not the constant -- see decayTable. What
// this pins is that the flame tracks the surface and is never the bottom third
// of it.
func TestFlameClimbsMostOfTheSurface(t *testing.T) {
	// Two pixel rows per text row, so these are a small pane, a browser window
	// at a usual font size, and a tall one.
	for _, h := range []int{40, 98, 160, 300} {
		got := reachOn(t, h)
		if got < 0.45 {
			t.Errorf("on a surface %d tall the flame reached only %.0f%% of the way up", h, got*100)
		}
		if got > 0.99 {
			t.Errorf("on a surface %d tall the flame reached %.0f%% — it has no top", h, got*100)
		}
	}
}

// The size it is actually looked at, where it should nearly fill the view.
func TestFlameNearlyFillsABrowserWindow(t *testing.T) {
	if got := reachOn(t, 98); got < 0.7 {
		t.Errorf("the flame reached %.0f%% of a browser-sized surface, want most of it", got*100)
	}
}

// A flame that never burns out is a wall of fire, so cooling has to be at
// least one unit a row however tall the surface is.
func TestDecayIsNeverZero(t *testing.T) {
	for _, h := range []int{1, 40, 500, 4000} {
		for y, d := range decayTable(h, 0) {
			if d < 1 {
				t.Fatalf("h=%d row %d cools by %d, which never burns out", h, y, d)
			}
		}
	}
}
