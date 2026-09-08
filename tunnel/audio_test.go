package tunnel

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// fly runs frames of tunnel, feeding it src if there is one. A nil src is the
// case that matters: no Listen call at all, which is every existing user.
func fly(src canvas.AudioSource, frames int, step float64) (*Tunnel, *canvas.Surface) {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		if src != nil {
			tn.Listen(src(step))
		}
		tn.Frame(s, step)
	}
	return tn, s
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

// A silent source has to be indistinguishable from no source, pixel for pixel.
// The rush is a multiply by 1+2*level precisely so that a level of exactly
// zero is a multiply by exactly one; this is the assertion that says so.
func TestSilenceIsIdenticalToNoAudio(t *testing.T) {
	quiet, qs := fly(nil, 200, dt)
	silent, ss := fly(canvas.Steady(canvas.Audio{}), 200, dt)

	if quiet.tDepth != silent.tDepth {
		t.Fatalf("depth %v with no source and %v with a silent one", quiet.tDepth, silent.tDepth)
	}
	if quiet.tSpin != silent.tSpin {
		t.Fatalf("spin %v with no source and %v with a silent one", quiet.tSpin, silent.tSpin)
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

// traveled is the total depth covered over frames of the given sound, summed
// past the wrap at 256 so two runs can be compared as distances.
func traveled(src canvas.AudioSource, frames int, step float64) float64 {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	var total, prev float64
	for i := 0; i < frames; i++ {
		if src != nil {
			tn.Listen(src(step))
		}
		tn.Frame(s, step)
		d := tn.tDepth - prev
		if d < 0 {
			d += 256 // the accumulator wrapped this frame
		}
		total += d
		prev = tn.tDepth
	}
	return total
}

// The claim: loudness drives the speed of travel. At a sustained full level
// the viewer flies at 1+audioRush times the cruising rate, so over five
// seconds the distance covered should be just under that multiple — short only
// by the attack.
func TestLoudAudioFliesFaster(t *testing.T) {
	const secs = 5
	quiet := traveled(nil, secs*30, dt)
	loud := traveled(canvas.Steady(canvas.Audio{Level: 1}), secs*30, dt)

	if d := quiet - baseDepth*secs; d > 1e-9 || d < -1e-9 {
		t.Fatalf("cruising covered %.6f in %d seconds, want %d", quiet, secs, baseDepth*secs)
	}
	ratio := loud / quiet
	if ratio <= audioRush+1-0.1 || ratio > audioRush+1 {
		t.Errorf("five loud seconds covered %.4f times the distance, want just under %d",
			ratio, audioRush+1)
	}

	// Spin is deliberately left alone, so a surge can be told apart from the
	// tube simply turning.
	qt, _ := fly(nil, secs*30, dt)
	lt, _ := fly(canvas.Steady(canvas.Audio{Level: 1}), secs*30, dt)
	if qt.tSpin != lt.tSpin {
		t.Errorf("spin is %v quiet and %v loud: sound should not roll the tube",
			qt.tSpin, lt.tSpin)
	}
}

// The picture has to move differently too, not merely the accumulator.
func TestLoudAudioChangesThePicture(t *testing.T) {
	_, qs := fly(nil, 30, dt)
	_, ls := fly(canvas.Steady(canvas.Audio{Level: 1}), 30, dt)
	var same int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) == ls.At(x, y) {
				same++
			}
		}
	}
	if same == tw*th {
		t.Error("a second of loud sound left the tube exactly where it was")
	}
}

// AudioGain of zero has to be as inert as no source at all.
func TestAudioGainZeroIsSilence(t *testing.T) {
	quiet, qs := fly(nil, 120, dt)

	tn := New(0)
	tn.AudioGain = 0
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Steady(canvas.Audio{Level: 1})
	for i := 0; i < 120; i++ {
		tn.Listen(src(dt))
		tn.Frame(s, dt)
	}
	if tn.tDepth != quiet.tDepth {
		t.Fatalf("depth %v with AudioGain 0, want %v", tn.tDepth, quiet.tDepth)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) != s.At(x, y) {
				t.Fatalf("pixel %d,%d differs with AudioGain 0", x, y)
			}
		}
	}
}

// One loud frame is a click, not a beat. A tunnel that jumped to full speed
// for one frame and back would tear the rings rather than accelerate past
// them, which is why this has the slowest attack of the four.
func TestASpikeDoesNotJerkTheTube(t *testing.T) {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ {
		tn.Frame(s, dt) // settle in silence
	}
	restStep := baseDepth * dt

	before := tn.tDepth
	tn.Listen(canvas.Audio{Level: 1})
	tn.Frame(s, dt)
	spikeStep := tn.tDepth - before
	full := restStep * (1 + audioRush)
	if spikeStep >= full*0.8 {
		t.Errorf("one loud frame traveled %.6f against a full-speed %.6f: that is a jerk",
			spikeStep, full)
	}
	if spikeStep <= restStep {
		t.Errorf("one loud frame traveled %.6f against a cruising %.6f: nothing happened",
			spikeStep, restStep)
	}

	before = tn.tDepth
	tn.Listen(canvas.Audio{})
	tn.Frame(s, dt)
	if tail := tn.tDepth - before; tail < restStep+(spikeStep-restStep)*0.7 {
		t.Errorf("the frame after the spike traveled %.6f, down from %.6f: no coast at all",
			tail, spikeStep)
	}
}

// The same two seconds of the same music must fly the same distance however
// the interval was divided into frames.
//
// Within a tolerance rather than exactly, and for the reason plasma's twin
// test sets out: travel is the integral of a rate that is itself moving, and
// an integral taken one frame at a time carries an error of order dt during
// the attack. What must never happen is a response scaled by frame count,
// which would show as a factor of two rather than a fraction of a percent.
func TestAudioResponseIsFrameRateIndependent(t *testing.T) {
	slow := traveled(canvas.Steady(canvas.Audio{Level: 1}), 60, 1.0/30)  // two seconds
	fast := traveled(canvas.Steady(canvas.Audio{Level: 1}), 120, 1.0/60) // the same two seconds
	if d := (slow - fast) / slow; d > 0.01 || d < -0.01 {
		t.Errorf("two loud seconds traveled %.6f at 30fps and %.6f at 60fps: %.3f%% apart",
			slow, fast, d*100)
	}
	if slow <= baseDepth*2 {
		t.Errorf("two loud seconds traveled %.3f, no further than cruising: nothing was driven", slow)
	}

	loud := canvas.Steady(canvas.Audio{Level: 1})
	_, ss := fly(loud, 60, 1.0/30)
	_, fs := fly(loud, 120, 1.0/60)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if d := colorGap(ss.At(x, y), fs.At(x, y)); d > 24 {
				t.Fatalf("pixel %d,%d is %d channel steps apart between the two frame rates", x, y, d)
			}
		}
	}
}

// A beat should be visible as a beat: the travel rate has to move over a bar
// rather than settling at some average.
//
// The bar is a modest one on purpose. This is the longest decay of the four,
// and at 120bpm a beat arrives every half second — one decay constant — so
// the tunnel deliberately smooths a fast kick into a swell rather than
// answering each hit. A third again between the slowest and fastest stretch is
// what that should look like; a tunnel that lurched per kick would be the bug.
func TestABeatSurgesTheFlight(t *testing.T) {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Beat(120)
	lo, hi := 1e9, 0.0
	for i := 0; i < 120; i++ { // four seconds, eight beats
		tn.Listen(src(dt))
		before := tn.tDepth
		tn.Frame(s, dt)
		if i < 30 {
			continue // let the envelope find its range
		}
		step := tn.tDepth - before
		if step < 0 {
			continue // the accumulator wrapped this frame
		}
		if step < lo {
			lo = step
		}
		if step > hi {
			hi = step
		}
	}
	if hi < lo*1.25 {
		t.Errorf("the travel step ranged only %.6f..%.6f over eight beats: no surge", lo, hi)
	}
}

// The tables exist so that a frame is table reads and additions, and the audio
// path must not have changed that: an envelope is two exponentials and some
// multiplies, none of which belong on the heap.
func TestFrameWithAudioDoesNotAllocate(t *testing.T) {
	tn := New(0)
	tn.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Beat(120)
	if n := testing.AllocsPerRun(50, func() {
		tn.Listen(src(dt))
		tn.Frame(s, dt)
	}); n != 0 {
		t.Errorf("a frame with audio allocated %v times per call", n)
	}
}
