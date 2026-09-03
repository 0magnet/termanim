package tunnel

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 48, 32

// dt is one frame at the rate the constants were originally tuned at, so the
// distances these tests assert over a count of frames stay meaningful.
const dt = 1.0 / 30

func lum(c tcell.Color) int {
	r, g, b := c.RGB()
	return int(r + g + b)
}

// The tube fills the view, so nothing should be left showing the terminal
// behind it. A gap would appear as a hole in the wall.
func TestEveryPixelIsWritten(t *testing.T) {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	tn.Frame(s, dt)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) == tcell.ColorDefault {
				t.Fatalf("pixel %d,%d was never written", x, y)
			}
		}
	}
}

// A tunnel that does not move is a target painted on the screen.
func TestPatternMovesBetweenFrames(t *testing.T) {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	tn.Frame(s, dt)

	before := make([]tcell.Color, 0, tw*th)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			before = append(before, s.At(x, y))
		}
	}
	// Several frames, because at one frame apart only the pixels straddling a
	// square boundary change and that is a thin set.
	for i := 0; i < 10; i++ {
		tn.Frame(s, dt)
	}
	changed := 0
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != before[y*tw+x] {
				changed++
			}
		}
	}
	if changed < tw*th/20 {
		t.Errorf("only %d of %d pixels changed over ten frames", changed, tw*th)
	}
}

// The vanishing point is the whole illusion: if it drifts off centre the tube
// stops being a tube and becomes a smear. Measured with the checker turned
// off, so what is left is purely the radial falloff.
func TestVanishingPointIsCentred(t *testing.T) {
	tn := New(0)
	tn.Contrast = 0
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	tn.Frame(s, dt)

	var sx, sy, sw float64
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			// Weight by darkness: the far end of the tube is the dark part.
			w := float64(765 - lum(s.At(x, y)))
			sx += float64(x) * w
			sy += float64(y) * w
			sw += w
		}
	}
	if sw == 0 {
		t.Fatal("the whole frame is white")
	}
	cx, cy := sx/sw, sy/sw
	if d := cx - tw/2; d < -1.5 || d > 1.5 {
		t.Errorf("dark centre at x=%.2f, want %d", cx, tw/2)
	}
	if d := cy - th/2; d < -1.5 || d > 1.5 {
		t.Errorf("dark centre at y=%.2f, want %d", cy, th/2)
	}
}

// Light falls off down the tube. Along any ray from the centre the wall must
// get steadily brighter, which is what makes the flat screen read as depth.
func TestBrightnessGrowsWithRadius(t *testing.T) {
	tn := New(0)
	tn.Contrast = 0
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	tn.Frame(s, dt)

	cx, cy := tw/2, th/2
	rays := []struct{ dx, dy int }{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for _, r := range rays {
		prev := -1
		for i := 0; ; i++ {
			x, y := cx+r.dx*i, cy+r.dy*i
			if x < 0 || y < 0 || x >= tw || y >= th {
				break
			}
			v := lum(s.At(x, y))
			if v < prev {
				t.Fatalf("ray %+v dims at step %d: %d after %d", r, i, v, prev)
			}
			prev = v
		}
		if prev <= 0 {
			t.Errorf("ray %+v never lit up", r)
		}
	}
}

// Forward travel means the rings sweep outward past the viewer. The sign of
// the time offset decides this, and getting it backwards looks like falling
// into the screen rather than flying down the tube.
func TestRingsExpandOutward(t *testing.T) {
	tn := New(0)
	tn.Spin = 0     // a rolling tube would move the boundary for other reasons
	tn.Contrast = 1 // dark squares become pure black, so a boundary is exact
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)

	cx, cy := tw/2, th/2
	// This band sits between two ring boundaries at rest, so exactly one
	// boundary crosses it as the tube goes by.
	const lo, hi = 11, 15
	boundary := func() int {
		first := lum(s.At(cx+lo, cy)) == 0
		for d := lo + 1; d <= hi; d++ {
			if (lum(s.At(cx+d, cy)) == 0) != first {
				return d
			}
		}
		return hi + 1
	}

	// Let a boundary enter the band before watching it.
	for i := 0; i < 10; i++ {
		tn.Frame(s, dt)
	}
	tn.Frame(s, dt)
	start := boundary()
	prev := start
	for i := 0; i < 24; i++ {
		tn.Frame(s, dt)
		got := boundary()
		if got < prev {
			t.Fatalf("frame %d: boundary moved inward, %d -> %d", i, prev, got)
		}
		prev = got
	}
	if prev <= start {
		t.Errorf("boundary stayed at %d over 24 frames", start)
	}
}

// The tables are the whole state; a resize must rebuild them rather than
// leaving the old ones and drawing a tunnel sized for the previous window.
func TestResizeRebuildsTables(t *testing.T) {
	tn := New(0)
	tn.Resize(tw, th)
	tn.Resize(tw*2, th/2)
	if len(tn.angle) != tw*2*(th/2) || len(tn.depth) != tw*2*(th/2) || len(tn.shade) != tw*2*(th/2) {
		t.Fatalf("tables are %d/%d/%d entries after resize, want %d",
			len(tn.angle), len(tn.depth), len(tn.shade), tw*2*(th/2))
	}
	s := canvas.NewSurface(tw*2, th/2)
	tn.Frame(s, dt)
	for y := 0; y < th/2; y++ {
		for x := 0; x < tw*2; x++ {
			if s.At(x, y) == tcell.ColorDefault {
				t.Fatalf("pixel %d,%d unwritten after resize", x, y)
			}
		}
	}
}

// A tunnel drawn before any resize, or in a window with no area, must not
// panic on a nil table.
func TestDegenerateSizeIsSafe(t *testing.T) {
	tn := New(0)
	s := canvas.NewSurface(tw, th)
	tn.Frame(s, dt) // never resized
	tn.Resize(0, 0)
	tn.Frame(s, dt)
}

// The tables exist precisely so that a frame is table reads and additions.
// An allocation here would mean the per-pixel work escaped to the heap.
func TestFrameDoesNotAllocate(t *testing.T) {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	if n := testing.AllocsPerRun(50, func() { tn.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}

// Elapsed time, not frame count, drives the tube: the same wall-clock second
// must produce the same picture whether it was drawn in thirty frames or
// sixty. Asserting on the pixels rather than the accumulators makes the point
// where it can be seen.
func TestFrameRateIndependent(t *testing.T) {
	draw := func(steps int, step float64) (*Tunnel, *canvas.Surface) {
		tn := New(0)
		tn.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < steps; i++ {
			tn.Frame(s, step)
		}
		return tn, s
	}
	slow, a := draw(51, 1.0/30) // 1.7 seconds at thirty frames a second
	fast, b := draw(102, 1.0/60)

	// The offsets must be what the per-second speeds promise, not merely
	// consistent with each other.
	if d := slow.tDepth - baseDepth*1.7; d < -1e-6 || d > 1e-6 {
		t.Errorf("after 1.7s tDepth=%.6f, want %.6f", slow.tDepth, baseDepth*1.7)
	}
	if d := slow.tSpin - fast.tSpin; d < -1e-6 || d > 1e-6 {
		t.Errorf("tSpin diverged: %.6f at 30fps, %.6f at 60fps", slow.tSpin, fast.tSpin)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("pixel %d,%d differs between the two frame rates", x, y)
			}
		}
	}
}
