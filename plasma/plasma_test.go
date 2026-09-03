package plasma

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// dt is one frame at the rate these constants were tuned at, so every
// assertion about how far things move over N frames still means what it did.
const dt = 1.0 / 30

const tw, th = 40, 24

func TestFillsEveryPixel(t *testing.T) {
	p := New()
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	p.Frame(s, dt)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) == tcell.ColorDefault {
				t.Fatalf("pixel %d,%d was never written; plasma should cover the surface", x, y)
			}
		}
	}
}

func TestDrifts(t *testing.T) {
	p := New()
	p.Resize(tw, th)
	a := canvas.NewSurface(tw, th)
	b := canvas.NewSurface(tw, th)
	p.Frame(a, dt)
	for i := 0; i < 30; i++ { // a second of drift
		p.Frame(b, dt)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) != b.At(x, y) {
				return
			}
		}
	}
	t.Error("the field is identical after a second: it is not drifting")
}

func TestUsesTheWholeRamp(t *testing.T) {
	// A field that only ever reaches the middle of its palette looks washed
	// out. Over a few seconds it should visit both ends.
	p := New()
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	seen := map[tcell.Color]bool{}
	for i := 0; i < 120; i++ {
		p.Frame(s, dt)
		for y := 0; y < th; y += 3 {
			for x := 0; x < tw; x += 3 {
				seen[s.At(x, y)] = true
			}
		}
	}
	if len(seen) < 32 {
		t.Errorf("only %d distinct colours in four seconds; the field is too flat", len(seen))
	}
}

func TestSinTableMatchesMathSin(t *testing.T) {
	p := New()
	for _, turns := range []float64{0, 0.25, 0.5, 0.75, 1, 1.25, -0.25} {
		got := p.sin(turns)
		want := sinRef(turns)
		if d := got - want; d > 0.01 || d < -0.01 {
			t.Errorf("sin(%v turns) = %v, want about %v", turns, got, want)
		}
	}
}

// The point of taking elapsed time rather than counting frames: the same
// wall-clock interval must advance the field the same amount however it is
// divided into frames.
func TestFrameRateIndependent(t *testing.T) {
	slow, fast := New(), New()
	slow.Resize(tw, th)
	fast.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ { // 2 seconds at 30fps
		slow.Frame(s, 1.0/30)
	}
	for i := 0; i < 120; i++ { // the same 2 seconds at 60fps
		fast.Frame(s, 1.0/60)
	}
	if d := slow.t - fast.t; d > 1e-9 || d < -1e-9 {
		t.Errorf("two seconds advanced the field differently: %v at 30fps, %v at 60fps", slow.t, fast.t)
	}
	if slow.t == 0 {
		t.Error("the field did not advance at all")
	}
}
