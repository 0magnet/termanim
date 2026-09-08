package fire

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// burn runs frames of fire, feeding it src if there is one. A nil src is the
// case that matters most: no Listen call at all, which is every existing user
// of this package.
func burn(src canvas.AudioSource, frames int, step float64) (*Fire, *canvas.Surface) {
	f := New(1)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		if src != nil {
			f.Listen(src(step))
		}
		f.Frame(s, step)
	}
	return f, s
}

// The property this whole seam is built around. A silent source has to be
// indistinguishable from no source, pixel for pixel, or adding audio to a
// library other things already draw with is not safe.
//
// Asserted on the surface rather than on the heat grid, because the surface is
// what anyone actually sees; the grid is checked too, since a difference there
// would eventually reach the screen even if it has not yet.
func TestSilenceIsIdenticalToNoAudio(t *testing.T) {
	quiet, qs := burn(nil, 200, dt)
	silent, ss := burn(canvas.Steady(canvas.Audio{}), 200, dt)

	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) != ss.At(x, y) {
				t.Fatalf("pixel %d,%d is %v with no source and %v with a silent one",
					x, y, qs.At(x, y), ss.At(x, y))
			}
			if quiet.heat[y][x] != silent.heat[y][x] {
				t.Fatalf("heat at %d,%d is %d with no source and %d with a silent one",
					x, y, quiet.heat[y][x], silent.heat[y][x])
			}
		}
	}
}

// heatStats is the mean heat of the grid and the topmost row that is lit at
// all, which is how tall the flame stands.
func heatStats(f *Fire) (mean float64, top int) {
	top = th
	var sum int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if h := f.heat[y][x]; h > 0 {
				sum += int(h)
				if y < top {
					top = y
				}
			}
		}
	}
	return float64(sum) / float64(tw*th), top
}

// The claim: loudness fills the gaps in the fuel row, so the flame burns
// hotter and stands taller. Both halves are checked, because filling the gaps
// without the flame growing would mean the heat never reached the visible
// rows.
func TestLoudAudioMakesTheFlameLeap(t *testing.T) {
	quiet, qs := burn(nil, 200, dt)
	loud, ls := burn(canvas.Steady(canvas.Audio{Level: 1}), 200, dt)

	qm, qt := heatStats(quiet)
	lm, lt := heatStats(loud)
	if lm <= qm*1.2 {
		t.Errorf("mean heat %.1f quiet against %.1f loud: the fuel gaps are not filling", qm, lm)
	}
	if lt >= qt {
		t.Errorf("the flame topped out at row %d quiet and %d loud: it did not leap", qt, lt)
	}

	// And it reaches the screen: more of the surface is lit.
	lit := func(s *canvas.Surface) int {
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
	if lit(ls) <= lit(qs) {
		t.Errorf("%d pixels lit quiet and %d loud", lit(qs), lit(ls))
	}
}

// AudioGain of zero has to be as inert as no source at all, or it is not the
// off switch it claims to be.
func TestAudioGainZeroIsSilence(t *testing.T) {
	quiet, qs := burn(nil, 120, dt)

	f := New(1)
	f.AudioGain = 0
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Steady(canvas.Audio{Level: 1})
	for i := 0; i < 120; i++ {
		f.Listen(src(dt))
		f.Frame(s, dt)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if qs.At(x, y) != s.At(x, y) {
				t.Fatalf("pixel %d,%d differs with AudioGain 0", x, y)
			}
			if quiet.heat[y][x] != f.heat[y][x] {
				t.Fatalf("heat at %d,%d differs with AudioGain 0", x, y)
			}
		}
	}
}

// One loud frame is a click or a dropout, not a beat. The fuel must not go
// solid for a frame and empty the next, which on a fire reads as a flashbulb.
func TestASpikeDoesNotFlashTheFuel(t *testing.T) {
	f := New(1)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ {
		f.Frame(s, dt) // settle in silence
	}
	f.Listen(canvas.Audio{Level: 1})
	f.Frame(s, dt)
	peak := f.fuelGap
	if peak == 255 {
		t.Error("one loud frame took the fuel gaps straight to full")
	}
	if peak == 0 {
		t.Error("one loud frame moved the fuel gaps not at all")
	}
	f.Listen(canvas.Audio{})
	f.Frame(s, dt)
	if f.fuelGap < peak*7/10 {
		t.Errorf("the fuel gaps fell from %d to %d in one frame: no tail at all", peak, f.fuelGap)
	}
}

// The same second of the same music must burn the same fire whether it was
// drawn in thirty frames or sixty. The envelope is scaled by dt like
// everything else, and the simulation steps fall at the same elapsed times, so
// the two runs seed the same fuel with the same heat.
//
// Not asserted bit for bit, unlike the silent case above and unlike the
// existing frame-rate test. The envelope is an exponential composed once per
// frame, so the same elapsed second differs in the last bits of a float
// depending on how it was divided — and the fuel heat is a byte, so a value
// sitting on a rounding boundary lands one unit apart. That is a rounding
// artifact and not a rate dependence: a response that followed the frame count
// instead of the clock would be out by a factor of two, which a tolerance of
// one heat unit still catches with room to spare.
func TestAudioResponseIsFrameRateIndependent(t *testing.T) {
	loud := canvas.Audio{Level: 1}
	slow, ss := burn(canvas.Steady(loud), 60, 1.0/30)  // two seconds
	fast, fs := burn(canvas.Steady(loud), 120, 1.0/60) // the same two seconds

	var lit, differing int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			a, b := int(slow.heat[y][x]), int(fast.heat[y][x])
			if d := a - b; d > 1 || d < -1 {
				t.Fatalf("two loud seconds gave different fires: at %d,%d %d at 30fps but %d at 60fps",
					x, y, a, b)
			}
			if d := colorGap(ss.At(x, y), fs.At(x, y)); d > 0 {
				differing++
				if d > 8 {
					t.Fatalf("pixel %d,%d is %d channel steps apart between the two frame rates", x, y, d)
				}
			}
			if a > 0 {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Error("nothing burned, so the comparison proved nothing")
	}
	// Some pixels do differ, by a single entry of a 256-entry ramp; the check
	// in the loop bounds how far apart they can be. This bounds how many, well
	// above the sixth or so that boundary rounding accounts for and well below
	// what a flame standing somewhere else would produce.
	if differing > tw*th/3 {
		t.Errorf("%d of %d pixels differ between the two frame rates", differing, tw*th)
	}
	sm, st := heatStats(slow)
	fm, ft := heatStats(fast)
	if st != ft {
		t.Errorf("the flame topped out at row %d at 30fps and %d at 60fps", st, ft)
	}
	if d := sm - fm; d > sm/100 || d < -sm/100 {
		t.Errorf("mean heat %.2f at 30fps against %.2f at 60fps", sm, fm)
	}
}

// A beat should be visible as a beat: the fuel gaps have to move over a bar,
// not sit at some average.
func TestABeatPulsesTheFuel(t *testing.T) {
	f := New(1)
	f.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	src := canvas.Beat(120)
	var lo, hi byte = 255, 0
	for i := 0; i < 120; i++ { // four seconds, eight beats
		f.Listen(src(dt))
		f.Frame(s, dt)
		if i < 30 {
			continue // let the envelope find its range
		}
		if f.fuelGap < lo {
			lo = f.fuelGap
		}
		if f.fuelGap > hi {
			hi = f.fuelGap
		}
	}
	if hi-lo < 40 {
		t.Errorf("the fuel gaps ranged only %d..%d over eight beats: the flame is not pulsing", lo, hi)
	}
}

// colorGap is the largest per-channel difference between two pixels, which is
// how far apart two entries of the same ramp are. Zero means identical.
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
