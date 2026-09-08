package plasma

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// drift runs frames of plasma, feeding it src if there is one. A nil src is
// the case that matters: no Listen call at all, which is every existing user.
func drift(src canvas.AudioSource, frames int, step float64) (*Plasma, *canvas.Surface) {
	p := New()
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		if src != nil {
			p.Listen(src(step))
		}
		p.Frame(s, step)
	}
	return p, s
}

// colorGap is the largest per-channel difference between two pixels. Zero
// means identical.
func colorGap(a, b tcell.Color) int {
	if a == b {
		return 0
	}
	ar, ag, ab := a.RGB()
	br, bg, bb := b.RGB()
	d := 0
	for _, v := range [...]int32{ar - br, ag - bg, ab - bb} {
		if v < 0 {
			v = -v
		}
		if int(v) > d {
			d = int(v)
		}
	}
	return d
}

// A silent source has to be indistinguishable from no source at all, pixel for
// pixel. The surge is written as a multiply by 1+3*level precisely so that a
// level of exactly zero is a multiply by exactly one; this is the assertion
// that says so.
func TestSilenceIsIdenticalToNoAudio(t *testing.T) {
	quiet, qs := drift(nil, 300, dt)
	silent, ss := drift(canvas.Steady(canvas.Audio{}), 300, dt)

	if quiet.t != silent.t {
		t.Fatalf("phase %v with no source and %v with a silent one", quiet.t, silent.t)
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

// The claim: loudness drives the rate of phase advance, so the field surges
// rather than drifting evenly. At a sustained full level the rate reaches
// 1+audioSurge times its resting value, so over five seconds the total advance
// should be just under that multiple — short only by the attack.
func TestLoudAudioSurgesTheField(t *testing.T) {
	const secs = 5
	quiet, qs := drift(nil, secs*30, dt)
	loud, ls := drift(canvas.Steady(canvas.Audio{Level: 1}), secs*30, dt)

	if quiet.t <= 0 {
		t.Fatal("the quiet field did not advance at all")
	}
	ratio := loud.t / quiet.t
	if ratio <= audioSurge+1-0.1 || ratio > audioSurge+1 {
		t.Errorf("five loud seconds advanced the field %.4f times as far, want just under %d",
			ratio, audioSurge+1)
	}

	var same int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) == ls.At(x, y) {
				same++
			}
		}
	}
	if same == tw*th {
		t.Error("the surge changed nothing on the screen")
	}
}

// AudioGain of zero has to be as inert as no source at all, or it is not the
// off switch it claims to be.
func TestAudioGainZeroIsSilence(t *testing.T) {
	quiet, qs := drift(nil, 120, dt)

	p := New()
	p.AudioGain = 0
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Steady(canvas.Audio{Level: 1})
	for i := 0; i < 120; i++ {
		p.Listen(src(dt))
		p.Frame(s, dt)
	}
	if p.t != quiet.t {
		t.Fatalf("phase %v with AudioGain 0, want %v", p.t, quiet.t)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) != s.At(x, y) {
				t.Fatalf("pixel %d,%d differs with AudioGain 0", x, y)
			}
		}
	}
}

// One loud frame is a click or a dropout, not a beat. It must not slam the
// field to full speed for a frame and back, which reads as a tear rather than
// a surge.
func TestASpikeDoesNotSlamTheField(t *testing.T) {
	p := New()
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ {
		p.Frame(s, dt) // settle in silence
	}
	rest := p.t
	p.Frame(s, dt)
	restStep := p.t - rest

	p.Listen(canvas.Audio{Level: 1})
	before := p.t
	p.Frame(s, dt)
	spikeStep := p.t - before
	full := restStep * (1 + audioSurge)
	if spikeStep >= full*0.8 {
		t.Errorf("one loud frame advanced %.6f against a full-speed %.6f: that is a snap",
			spikeStep, full)
	}
	if spikeStep <= restStep {
		t.Errorf("one loud frame advanced %.6f against a resting %.6f: nothing happened",
			spikeStep, restStep)
	}

	p.Listen(canvas.Audio{})
	before = p.t
	p.Frame(s, dt)
	if tail := p.t - before; tail < restStep+(spikeStep-restStep)*0.7 {
		t.Errorf("the frame after the spike advanced %.6f, down from %.6f: no tail at all",
			tail, spikeStep)
	}
}

// The same second of the same music must advance the field by the same amount
// however it was divided into frames.
//
// Within a tolerance, unlike the silent case above, and the reason is worth
// stating because it is not something to be fixed later. The phase is the
// integral of a rate that is itself changing, and any such integral taken one
// frame at a time carries an error of order dt: a quarter of a percent over
// the two seconds below, nearly all of it accumulated during the 50 ms attack
// where the rate moves fastest. The envelope's own value at a given elapsed
// time is exact — canvas asserts that — so once the level settles the two runs
// advance in step.
//
// Exactness was never on offer in any case: a real source is sampled once a
// frame, so a run at 60fps hears twice as much of the music as one at 30 and
// cannot integrate the same signal. What must never happen is a response
// scaled by frame count rather than by time, which would show up here as a
// factor of two rather than a quarter of a percent.
func TestAudioResponseIsFrameRateIndependent(t *testing.T) {
	loud := canvas.Steady(canvas.Audio{Level: 1})
	slow, ss := drift(loud, 60, 1.0/30)  // two seconds
	fast, fs := drift(loud, 120, 1.0/60) // the same two seconds

	if d := (slow.t - fast.t) / slow.t; d > 0.01 || d < -0.01 {
		t.Errorf("two loud seconds advanced the field to %v at 30fps and %v at 60fps: %.3f%% apart",
			slow.t, fast.t, d*100)
	}
	// And it is the same picture. The ramp runs the whole spectrum in 256
	// entries, so neighboring entries are tens of units apart and this bounds
	// the difference to a couple of them.
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if d := colorGap(ss.At(x, y), fs.At(x, y)); d > 24 {
				t.Fatalf("pixel %d,%d is %d channel steps apart between the two frame rates", x, y, d)
			}
		}
	}
}

// A beat should be visible as a beat: the rate has to move over a bar rather
// than settling at some average.
func TestABeatMakesTheFieldPulse(t *testing.T) {
	p := New()
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Beat(120)
	lo, hi := 1e9, 0.0
	for i := 0; i < 120; i++ { // four seconds, eight beats
		p.Listen(src(dt))
		before := p.t
		p.Frame(s, dt)
		if i < 30 {
			continue // let the envelope find its range
		}
		step := p.t - before
		if step < lo {
			lo = step
		}
		if step > hi {
			hi = step
		}
	}
	if hi < lo*1.4 {
		t.Errorf("the phase step ranged only %.6f..%.6f over eight beats: no pulse", lo, hi)
	}
}
