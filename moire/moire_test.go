package moire

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/0magnet/termanim/canvas"
)

// dt is one frame at the rate these constants were tuned at, so every
// assertion about how far things move over N frames still means what it did.
const dt = 1.0 / 30

const tw, th = 48, 32

func TestFillsEveryPixel(t *testing.T) {
	m := New()
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	m.Frame(s, dt)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) == tcell.ColorDefault {
				t.Fatalf("pixel %d,%d unset; the pattern should cover the surface", x, y)
			}
		}
	}
}

func TestInterferes(t *testing.T) {
	// Two ripples summed must produce both cancellation and reinforcement, or
	// there is no moire — just rings.
	m := New()
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	m.Frame(s, dt)
	var lo, hi int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			r, g, b := s.At(x, y).RGB()
			switch v := r + g + b; {
			case v < 200:
				lo++
			case v > 400:
				hi++
			}
		}
	}
	if lo == 0 || hi == 0 {
		t.Errorf("no interference: %d dark and %d bright pixels", lo, hi)
	}
}

func TestPatternSweeps(t *testing.T) {
	m := New()
	m.Resize(tw, th)
	a := canvas.NewSurface(tw, th)
	b := canvas.NewSurface(tw, th)
	m.Frame(a, dt)
	for i := 0; i < 15; i++ {
		m.Frame(b, dt)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) != b.At(x, y) {
				return
			}
		}
	}
	t.Error("the pattern is identical after half a second: it is not moving")
}

// The same wall-clock interval must move the centres the same amount however
// it is divided into frames.
func TestFrameRateIndependent(t *testing.T) {
	slow, fast := New(), New()
	slow.Resize(tw, th)
	fast.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ {
		slow.Frame(s, 1.0/30)
	}
	for i := 0; i < 120; i++ {
		fast.Frame(s, 1.0/60)
	}
	if d := slow.t - fast.t; d > 1e-9 || d < -1e-9 {
		t.Errorf("two seconds advanced differently: %v at 30fps, %v at 60fps", slow.t, fast.t)
	}
	if slow.t == 0 {
		t.Error("the pattern did not advance at all")
	}
}
