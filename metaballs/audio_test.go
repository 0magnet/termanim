package metaballs

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// swell runs frames of metaballs, feeding it src if there is one. A nil src is
// the case that matters: no Listen call at all, which is every existing user.
func swell(src canvas.AudioSource, frames int, step float64) (*Metaballs, *canvas.Surface) {
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		if src != nil {
			m.Listen(src(step))
		}
		m.Frame(s, step)
	}
	return m, s
}

// lit counts the pixels the field reached, which is how big the blobs are.
func lit(s *canvas.Surface) int {
	var n int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				n++
			}
		}
	}
	return n
}

// A silent source has to be indistinguishable from no source, pixel for pixel.
// The radius is recomputed every frame as r0*(1+0.6*band), so this is the
// assertion that a band of exactly zero leaves it bit for bit the radius
// Resize chose.
func TestSilenceIsIdenticalToNoAudio(t *testing.T) {
	quiet, qs := swell(nil, 200, dt)
	silent, ss := swell(canvas.Steady(canvas.Audio{}), 200, dt)

	for i := range quiet.balls {
		if quiet.balls[i] != silent.balls[i] {
			t.Fatalf("ball %d is %+v with no source and %+v with a silent one",
				i, quiet.balls[i], silent.balls[i])
		}
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) != ss.At(x, y) {
				t.Fatalf("pixel %d,%d is %v with no source and %v with a silent one",
					x, y, qs.At(x, y), ss.At(x, y))
			}
		}
	}
}

// The claim: loudness swells the balls, so the blobs bulge and cover more of
// the screen. The radius is the only thing sound touches — the balls must
// travel exactly as they did, or the effect is not a bulge but a different
// animation.
func TestLoudAudioSwellsTheBlobs(t *testing.T) {
	quiet, qs := swell(nil, 120, dt)
	loud, ls := swell(canvas.Steady(canvas.Audio{Level: 1}), 120, dt)

	for i := range quiet.balls {
		if quiet.balls[i].x != loud.balls[i].x || quiet.balls[i].y != loud.balls[i].y {
			t.Errorf("ball %d moved differently under sound: %.3f,%.3f against %.3f,%.3f",
				i, quiet.balls[i].x, quiet.balls[i].y, loud.balls[i].x, loud.balls[i].y)
		}
		if loud.balls[i].r <= quiet.balls[i].r {
			t.Errorf("ball %d has radius %.2f loud and %.2f quiet: it did not swell",
				i, loud.balls[i].r, quiet.balls[i].r)
		}
		// A full band is 1+audioBulge times the resting radius, less a little
		// for the attack over the first frames.
		want := quiet.balls[i].r0 * (1 + audioBulge)
		if d := loud.balls[i].r - want; d > 0 || d < -want*0.05 {
			t.Errorf("ball %d swelled to %.3f, want just under %.3f", i, loud.balls[i].r, want)
		}
	}
	if lit(ls) <= lit(qs) {
		t.Errorf("%d pixels lit quiet and %d loud: the swell never reached the screen",
			lit(qs), lit(ls))
	}
}

// The reason metaballs takes bands rather than the level: different
// frequencies must bulge different blobs, or the spectrum is decoration.
func TestBandsBulgeIndividualBalls(t *testing.T) {
	var a canvas.Audio
	a.Spectrum[0] = 1 // only the lowest band is playing
	m, _ := swell(canvas.Steady(a), 120, dt)

	if m.balls[0].r <= m.balls[0].r0 {
		t.Errorf("ball 0 is %.3f against a resting %.3f: its own band did not reach it",
			m.balls[0].r, m.balls[0].r0)
	}
	for i := 1; i < len(m.balls); i++ {
		if m.balls[i].r != m.balls[i].r0 {
			t.Errorf("ball %d is %.3f against a resting %.3f, but its band is silent",
				i, m.balls[i].r, m.balls[i].r0)
		}
	}

	// And the other way round, so the wiring is not simply "ball 0 gets
	// everything": a high band moves a different ball and leaves the first
	// alone.
	var b canvas.Audio
	b.Spectrum[3] = 1
	n, _ := swell(canvas.Steady(b), 120, dt)
	if n.balls[3].r <= n.balls[3].r0 {
		t.Errorf("ball 3 did not answer band 3")
	}
	if n.balls[0].r != n.balls[0].r0 {
		t.Errorf("ball 0 moved on band 3")
	}
}

// AudioGain of zero has to be as inert as no source at all.
func TestAudioGainZeroIsSilence(t *testing.T) {
	quiet, qs := swell(nil, 120, dt)

	m := New(1)
	m.AudioGain = 0
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Steady(canvas.Audio{Level: 1})
	for i := 0; i < 120; i++ {
		m.Listen(src(dt))
		m.Frame(s, dt)
	}
	for i := range quiet.balls {
		if quiet.balls[i] != m.balls[i] {
			t.Fatalf("ball %d differs with AudioGain 0", i)
		}
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) != s.At(x, y) {
				t.Fatalf("pixel %d,%d differs with AudioGain 0", x, y)
			}
		}
	}
}

// A radius is read straight off the envelope with no integral to hide a jitter
// in, so this is where a missing attack would show worst: one loud frame must
// not inflate a blob to full size and deflate it the next.
func TestASpikeDoesNotPopTheBlobs(t *testing.T) {
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ {
		m.Frame(s, dt) // settle in silence
	}
	rest := m.balls[0].r

	m.Listen(canvas.Audio{Level: 1})
	m.Frame(s, dt)
	peak := m.balls[0].r
	full := m.balls[0].r0 * (1 + audioBulge)
	if peak >= rest+(full-rest)*0.8 {
		t.Errorf("one loud frame took the radius to %.3f of a full %.3f: that is a pop", peak, full)
	}
	if peak <= rest {
		t.Errorf("one loud frame moved the radius from %.3f to %.3f", rest, peak)
	}

	m.Listen(canvas.Audio{})
	m.Frame(s, dt)
	if next := m.balls[0].r; next < rest+(peak-rest)*0.7 {
		t.Errorf("the radius fell from %.3f to %.3f in one frame: no tail at all", peak, next)
	}
}

// The same two seconds of the same sound must leave the blobs the same size
// however the interval was divided into frames.
//
// Exact here, unlike plasma and tunnel: a radius is the envelope's current
// value rather than an integral of it, and the envelope's value at a given
// elapsed time is the same at both rates to the last few bits. The tolerance
// below is the existing one for the balls' positions, which drift by a
// sub-pixel because a bounce lands on a slightly different fraction.
func TestAudioResponseIsFrameRateIndependent(t *testing.T) {
	loud := canvas.Steady(canvas.Audio{Level: 1})
	slow, _ := swell(loud, 60, 1.0/30)  // two seconds
	fast, _ := swell(loud, 120, 1.0/60) // the same two seconds

	for i := range slow.balls {
		if d := slow.balls[i].r - fast.balls[i].r; d > 1e-9 || d < -1e-9 {
			t.Errorf("ball %d has radius %.9f at 30fps and %.9f at 60fps",
				i, slow.balls[i].r, fast.balls[i].r)
		}
		if slow.balls[i].r <= slow.balls[i].r0 {
			t.Errorf("ball %d never swelled, so the comparison proved nothing", i)
		}
	}
}

// A beat should be visible as a beat: the radii have to move over a bar rather
// than settling at some average.
func TestABeatPulsesTheBlobs(t *testing.T) {
	m := New(1)
	m.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Beat(120)
	lo, hi := 1e9, 0.0
	for i := 0; i < 120; i++ { // four seconds, eight beats
		m.Listen(src(dt))
		m.Frame(s, dt)
		if i < 30 {
			continue // let the envelope find its range
		}
		f := m.balls[0].r / m.balls[0].r0
		if f < lo {
			lo = f
		}
		if f > hi {
			hi = f
		}
	}
	if hi-lo < 0.1 {
		t.Errorf("ball 0 ranged only %.3f..%.3f of its resting size over eight beats", lo, hi)
	}
}
