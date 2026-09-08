package canvas

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/internal/simscreen"
)

// audioDT is one frame at the rate these animations were tuned at, so a count
// of frames here means the same amount of time it does everywhere else.
const audioDT = 1.0 / 30

// The zero Audio is silence, and everything downstream leans on it: an
// animation holds one before a host has connected, and a host that stops
// sending leaves one behind.
func TestZeroAudioIsSilence(t *testing.T) {
	var a Audio
	if a.Level != 0 {
		t.Errorf("zero Audio has level %v", a.Level)
	}
	for i := 0; i < Bands; i++ {
		if v := a.Band(i); v != 0 {
			t.Errorf("zero Audio band %d is %v", i, v)
		}
	}
}

// A host with a VU meter and no analyzer still has to drive the animations
// that ask for bands, or half the effects sit still while the other half move
// and it looks broken rather than partial.
func TestBandFallsBackToTheLevelWithoutASpectrum(t *testing.T) {
	a := Audio{Level: 0.4}
	for i := 0; i < Bands; i++ {
		if v := a.Band(i); v != 0.4 {
			t.Errorf("band %d is %v with no spectrum, want the level 0.4", i, v)
		}
	}
	// One band set is a spectrum, and the silent bands must then stay silent
	// rather than picking the level back up.
	a.Spectrum[2] = 0.9
	if v := a.Band(0); v != 0 {
		t.Errorf("band 0 is %v beside a real spectrum, want 0", v)
	}
	if v := a.Band(2); v != 0.9 {
		t.Errorf("band 2 is %v, want 0.9", v)
	}
}

func TestBandClampsItsIndex(t *testing.T) {
	// Called with a ball number rather than a band number somewhere, one day.
	a := Audio{Level: 0.5}
	a.Spectrum[0], a.Spectrum[Bands-1] = 0.1, 0.7
	if v := a.Band(-3); v != 0.1 {
		t.Errorf("Band(-3) = %v, want the first band", v)
	}
	if v := a.Band(99); v != 0.7 {
		t.Errorf("Band(99) = %v, want the last band", v)
	}
}

// The property the whole design rests on: a silent source must leave the
// envelope at exactly zero, not nearly zero, so that every mapping built on it
// is an exact identity and an animation fed silence draws what it drew before
// any of this existed.
func TestEnvelopeStaysExactlyZeroOnSilence(t *testing.T) {
	var e Envelope
	for i := 0; i < 500; i++ {
		e.Step(Audio{}, audioDT)
	}
	if e.Level() != 0 {
		t.Errorf("after 500 silent frames the level is %v, want exactly 0", e.Level())
	}
	for i := 0; i < Bands; i++ {
		if e.Band(i) != 0 {
			t.Errorf("band %d is %v after silence, want exactly 0", i, e.Band(i))
		}
	}
}

// The point of the envelope. One loud frame in a stream of silence is a click,
// a dropout or a bad sample, and driving the picture with the raw number would
// throw it to full for one frame and back the next — a flash rather than a
// pulse.
func TestASingleFrameSpikeDoesNotSnapToFullAndBack(t *testing.T) {
	var e Envelope
	e.Step(Audio{Level: 1}, audioDT)
	peak := e.Level()
	if peak >= 0.75 {
		t.Errorf("one loud frame took the envelope to %.3f: that is a snap, not an attack", peak)
	}
	if peak <= 0 {
		t.Errorf("one loud frame moved the envelope to %.3f: it is not listening at all", peak)
	}
	e.Step(Audio{}, audioDT)
	if next := e.Level(); next < peak*0.7 {
		t.Errorf("the frame after the spike fell from %.3f to %.3f: nothing is holding the tail",
			peak, next)
	}
	// And it does eventually let go, or every spike would be permanent.
	for i := 0; i < 120; i++ {
		e.Step(Audio{}, audioDT)
	}
	if e.Level() > 0.01 {
		t.Errorf("four seconds after a spike the envelope is still at %.3f", e.Level())
	}
}

// Rising fast and falling slow is what makes a meter read as rhythm rather
// than as a smoothed amplitude. Equal constants would pass every other test
// here and look like nothing.
func TestEnvelopeRisesFasterThanItFalls(t *testing.T) {
	var up Envelope
	for i := 0; i < 3; i++ {
		up.Step(Audio{Level: 1}, audioDT)
	}
	rise := up.Level()

	var down Envelope
	for i := 0; i < 200; i++ {
		down.Step(Audio{Level: 1}, audioDT)
	}
	settled := down.Level()
	for i := 0; i < 3; i++ {
		down.Step(Audio{}, audioDT)
	}
	fall := settled - down.Level()

	if rise <= fall {
		t.Errorf("three frames rose %.3f and fell %.3f: the envelope has no attack", rise, fall)
	}
}

// Like everything else here, the response has to be the same after a given
// interval however that interval was divided into frames — otherwise the
// visualizer looks different on a fast machine, which is the one thing dt was
// introduced to stop.
func TestEnvelopeIsFrameRateIndependent(t *testing.T) {
	run := func(frames int, step float64) float64 {
		var e Envelope
		for i := 0; i < frames; i++ {
			e.Step(Audio{Level: 0.8}, step)
		}
		return e.Level()
	}
	slow := run(30, 1.0/30) // one second
	fast := run(60, 1.0/60) // the same second
	if d := slow - fast; d > 1e-9 || d < -1e-9 {
		t.Errorf("a second of the same sound gave %.12f at 30fps and %.12f at 60fps", slow, fast)
	}
	if slow < 0.5 {
		t.Errorf("a second of sound only reached %.3f, so the comparison proved little", slow)
	}
}

// The exponential form has a closed solution, so this pins the actual curve
// rather than merely its self-consistency: a fixed fraction per frame would
// match at one rate and drift at the other.
func TestEnvelopeFollowsTheExponential(t *testing.T) {
	var e Envelope
	const frames = 20
	for i := 0; i < frames; i++ {
		e.Step(Audio{Level: 1}, audioDT)
	}
	want := 1 - math.Exp(-frames*audioDT/DefaultAttack)
	if d := e.Level() - want; d > 1e-9 || d < -1e-9 {
		t.Errorf("level %.9f after %.3fs, want %.9f", e.Level(), frames*audioDT, want)
	}
}

// A host can hand over a raw peak that overshoots. Every mapping downstream
// multiplies by this, so one bad sample would otherwise become a frame of
// nonsense in four different effects at once.
func TestEnvelopeClampsWildInput(t *testing.T) {
	var e Envelope
	for i := 0; i < 200; i++ {
		e.Step(Audio{Level: 40}, audioDT)
	}
	if e.Level() > 1 {
		t.Errorf("a level of 40 drove the envelope to %v, want at most 1", e.Level())
	}
	for i := 0; i < 200; i++ {
		e.Step(Audio{Level: -5}, audioDT)
	}
	if e.Level() < 0 {
		t.Errorf("a negative level drove the envelope to %v, want at least 0", e.Level())
	}
}

func TestEnvelopeResetReturnsToSilence(t *testing.T) {
	var e Envelope
	for i := 0; i < 100; i++ {
		e.Step(Audio{Level: 1}, audioDT)
	}
	e.Reset()
	if e.Level() != 0 {
		t.Errorf("after Reset the level is %v, want exactly 0", e.Level())
	}
	for i := 0; i < Bands; i++ {
		if e.Band(i) != 0 {
			t.Errorf("after Reset band %d is %v", i, e.Band(i))
		}
	}
}

// Beat exists so the wiring can be exercised and watched on a machine with no
// sound card, and so a test is repeatable: same construction, same sequence.
func TestBeatRepeatsAtTheGivenTempo(t *testing.T) {
	// 120bpm is half a second a beat, so at a thirtieth of a second a frame
	// the peak falls on frames 0, 15, 30...
	src := Beat(120)
	var peaks []int
	prev := 0.0
	for i := 0; i < 60; i++ {
		v := src(audioDT).Level
		if i > 0 && v > prev {
			peaks = append(peaks, i)
		}
		prev = v
	}
	if len(peaks) < 3 {
		t.Fatalf("only %d beats in two seconds at 120bpm, want four", len(peaks))
	}
	for i := 1; i < len(peaks); i++ {
		if d := peaks[i] - peaks[i-1]; d != 15 {
			t.Errorf("beats %d frames apart, want 15 at 120bpm and 30fps", d)
		}
	}
}

// Bands that all carry the same signal are a spectrum in name only, and the
// animation driven by them would look driven by the level.
func TestBeatBandsDifferFromOneAnother(t *testing.T) {
	src := Beat(120)
	// A few frames past the onset, where the different decay rates have had
	// time to separate.
	var a Audio
	for i := 0; i < 4; i++ {
		a = src(audioDT)
	}
	if a.Band(0) <= a.Band(Bands-1) {
		t.Errorf("low band %.4f, high band %.4f: the highs should die first",
			a.Band(0), a.Band(Bands-1))
	}
	if a.Band(0) <= 0 {
		t.Errorf("the low band is %.4f just after a beat", a.Band(0))
	}
}

func TestBeatIsRepeatableFromConstruction(t *testing.T) {
	a, b := Beat(140), Beat(140)
	for i := 0; i < 100; i++ {
		x, y := a(audioDT), b(audioDT)
		if x != y {
			t.Fatalf("two beats at the same tempo diverged at frame %d", i)
		}
	}
}

func TestSteadyReportsTheSameSound(t *testing.T) {
	want := Audio{Level: 0.3}
	want.Spectrum[1] = 0.9
	src := Steady(want)
	for i := 0; i < 10; i++ {
		if got := src(audioDT); got != want {
			t.Fatalf("Steady returned %v at frame %d, want %v", got, i, want)
		}
	}
}

// Sound arrives every frame, so nothing on this path may allocate — the same
// rule the animations are held to.
func TestAudioPathDoesNotAllocate(t *testing.T) {
	src := Beat(120)
	var e Envelope
	if n := testing.AllocsPerRun(200, func() { e.Step(src(audioDT), audioDT) }); n != 0 {
		t.Errorf("a frame of audio allocated %v times", n)
	}
	still := Steady(Audio{Level: 0.5})
	if n := testing.AllocsPerRun(200, func() { e.Step(still(audioDT), audioDT) }); n != 0 {
		t.Errorf("a steady frame of audio allocated %v times", n)
	}
}

// earAnim records the sound it was given and the order it arrived in.
type earAnim struct {
	frames int
	heard  []Audio
	// order is "listen" and "frame" as they happened, so a Listen delivered
	// after the frame it describes is caught rather than being invisible.
	order []string
	// stop is called on the nth frame to end the loop from inside it.
	stop func()
	n    int
}

func (e *earAnim) Resize(w, h int) {}

func (e *earAnim) Frame(s *Surface, dt float64) {
	e.frames++
	e.order = append(e.order, "frame")
	if e.frames == e.n && e.stop != nil {
		e.stop()
	}
}

func (e *earAnim) Listen(a Audio) {
	e.heard = append(e.heard, a)
	e.order = append(e.order, "listen")
}

// deafAnim is every other animation in this repository: it does not implement
// AudioListener and must be entirely unaffected by a source being attached.
type deafAnim struct{ frames int }

func (d *deafAnim) Resize(w, h int)              {}
func (d *deafAnim) Frame(s *Surface, dt float64) { d.frames++ }

func TestAudioTapIsNilWithoutASource(t *testing.T) {
	if tap := audioTap(&earAnim{}, nil); tap != nil {
		t.Error("a listening animation with no source still got a tap")
	}
}

func TestAudioTapIsNilForAnAnimationThatDoesNotListen(t *testing.T) {
	if tap := audioTap(&deafAnim{}, Beat(120)); tap != nil {
		t.Error("an animation with no Listen method got a tap")
	}
}

func TestAudioTapDeliversWhatTheSourceReturned(t *testing.T) {
	want := Audio{Level: 0.6}
	want.Spectrum[3] = 0.2
	a := &earAnim{}
	tap := audioTap(a, Steady(want))
	if tap == nil {
		t.Fatal("a listening animation with a source got no tap")
	}
	tap(audioDT)
	if len(a.heard) != 1 || a.heard[0] != want {
		t.Fatalf("heard %v, want one %v", a.heard, want)
	}
}

// The wiring as Run actually does it: sound is delivered, and it is delivered
// before the frame it belongs to rather than after.
//
// The loop is ended from inside the animation — the event queue is buffered
// and Run reads it on the next pass — so the test needs no sleep, no second
// goroutine and no way to hang.
func TestRunFeedsAudioBeforeEachFrame(t *testing.T) {
	sim := simscreen.NewScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(40, 12)

	a := &earAnim{n: 3}
	a.stop = func() {
		sim.EventQ() <- tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone)
	}
	if err := Run(sim, a, Options{FPS: 200, Audio: Steady(Audio{Level: 0.5})}); err != nil {
		t.Fatal(err)
	}
	if a.frames < 3 {
		t.Fatalf("the loop drew %d frames, want at least 3", a.frames)
	}
	if len(a.heard) != a.frames {
		t.Errorf("%d frames drawn but %d heard: the source is not asked once a frame",
			a.frames, len(a.heard))
	}
	for i := 0; i+1 < len(a.order); i += 2 {
		if a.order[i] != "listen" || a.order[i+1] != "frame" {
			t.Fatalf("frame %d went %q then %q, want listen then frame",
				i/2, a.order[i], a.order[i+1])
		}
	}
	for i, h := range a.heard {
		if h.Level != 0.5 {
			t.Fatalf("frame %d heard level %v, want 0.5", i, h.Level)
		}
	}
}

// A source attached to an animation that does not listen must be inert, not an
// error and not a crash — that is the case for every other effect in this
// repository.
func TestRunIgnoresAudioForAnAnimationThatDoesNotListen(t *testing.T) {
	sim := simscreen.NewScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(40, 12)

	sim.EventQ() <- tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone)
	if err := Run(sim, &deafAnim{}, Options{Audio: Beat(120)}); err != nil {
		t.Fatal(err)
	}
}
