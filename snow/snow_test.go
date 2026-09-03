package snow

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 64, 48

// dt is the step these tests advance by: a thirtieth of a second, so a count of
// frames here means the same amount of falling and settling it always did.
const dt = 1.0 / 30

func total(g []int) int {
	n := 0
	for _, v := range g {
		n += v
	}
	return n
}

func TestFlakesDriftRatherThanFall(t *testing.T) {
	// Rain crosses the surface in well under a second: its slowest drop falls
	// six tenths of the height a second. Snow has to be far slower than that or
	// it is only rain with a different colour.
	const rainSlowest = 0.6
	s := New(1)
	s.Resize(tw, th)
	for i, f := range s.flakes {
		if f.vy/th >= rainSlowest/2 {
			t.Fatalf("flake %d falls %.3f of the height a second, as fast as rain", i, f.vy/th)
		}
		if f.vy <= 0 {
			t.Fatalf("flake %d is not falling at all: %.3f", i, f.vy)
		}
	}
}

func TestFlakesSway(t *testing.T) {
	// A flake that only moves down is a raindrop. The horizontal position has
	// to reverse, which is what a sine has and a slant does not.
	s := New(2)
	s.Resize(tw, th)
	f := s.flakes[0]
	prev := f.at()
	var up, down bool
	for i := 0; i < 200; i++ {
		f.phase += f.swayV * dt
		x := f.at()
		if x > prev {
			up = true
		} else if x < prev {
			down = true
		}
		prev = x
	}
	if !up || !down {
		t.Error("the flake's x never reversed: it is not swaying")
	}
}

func TestFlakesAreNotInLockstep(t *testing.T) {
	s := New(3)
	s.Resize(tw, th)
	first := s.flakes[0]
	for _, f := range s.flakes[1:] {
		if f.phase != first.phase && f.swayV != first.swayV {
			return
		}
	}
	t.Error("every flake shares a phase and a rate: the fall will move as one body")
}

func TestSnowAccumulates(t *testing.T) {
	s := New(4)
	s.Resize(tw, th)
	surf := canvas.NewSurface(tw, th)
	for i := 0; i < 200; i++ {
		s.Frame(surf, dt)
	}
	early := total(s.ground)
	if early == 0 {
		t.Fatal("nothing settled in two hundred frames")
	}
	for i := 0; i < 600; i++ {
		s.Frame(surf, dt)
	}
	if late := total(s.ground); late <= early {
		t.Errorf("the bank went from %d pixels to %d: snow is not building up", early, late)
	}
}

func TestBankStaysBelowTheCeiling(t *testing.T) {
	// Left unbounded the bank buries the screen. Melting and the clamp together
	// have to hold it under MaxDepth however long it snows.
	s := New(5)
	s.Resize(tw, th)
	surf := canvas.NewSurface(tw, th)
	maxD := s.maxDepth()
	for i := 0; i < 3000; i++ {
		s.Frame(surf, dt)
		for x, d := range s.ground {
			if d > maxD {
				t.Fatalf("frame %d: column %d is %d deep, over the %d ceiling", i, x, d, maxD)
			}
			if d >= th {
				t.Fatalf("frame %d: column %d reaches the top of the surface", i, x)
			}
		}
	}
}

func TestTheBankIsDrawnAtTheBottom(t *testing.T) {
	s := New(6)
	s.Resize(tw, th)
	surf := canvas.NewSurface(tw, th)
	for i := 0; i < 400; i++ {
		s.Frame(surf, dt)
	}
	x := -1
	for i, d := range s.ground {
		if d > 0 {
			x = i
			break
		}
	}
	if x < 0 {
		t.Fatal("no snow settled at all")
	}
	if surf.At(x, th-1) == tcell.ColorDefault {
		t.Errorf("column %d holds %d pixels of snow but its bottom row is unlit", x, s.ground[x])
	}
}

func TestDriftsDoNotStandAsCliffs(t *testing.T) {
	// Settling is what rounds the bank off. A column left two or more pixels
	// above its neighbour for long is snow behaving like masonry.
	s := New(7)
	s.Resize(tw, th)
	surf := canvas.NewSurface(tw, th)
	for i := 0; i < 1500; i++ {
		s.Frame(surf, dt)
	}
	var cliffs int
	for x := 1; x < len(s.ground); x++ {
		d := s.ground[x] - s.ground[x-1]
		if d > 2 || d < -2 {
			cliffs++
		}
	}
	if cliffs > len(s.ground)/8 {
		t.Errorf("%d of %d column joins are cliffs: the bank is not settling",
			cliffs, len(s.ground))
	}
}

func TestFlakesScaleWithTheSurface(t *testing.T) {
	small, big := New(8), New(8)
	small.Resize(40, 20)
	big.Resize(160, 80)
	if len(big.flakes) <= len(small.flakes) {
		t.Errorf("%d flakes in a big window against %d in a small one",
			len(big.flakes), len(small.flakes))
	}
}

func TestFallsAndSwaysAtTheSameRateAtAnyFrameRate(t *testing.T) {
	// One second of snow is one second of snow however it is sliced. Both runs
	// get the same single flake, high enough that it cannot reach the ground.
	one := flake{x: 32, y: 0, vy: 6, phase: 0, swayW: 3, swayV: 2, depth: 0.7}
	drift := func(step float64, frames int) (float64, float64) {
		s := New(1)
		s.Resize(tw, th)
		s.flakes = []flake{one}
		surf := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			s.Frame(surf, step)
		}
		return s.flakes[0].y, s.flakes[0].at()
	}
	slowY, slowX := drift(1.0/30, 30)
	fastY, fastX := drift(1.0/60, 60)
	if diff := slowY - fastY; diff > 0.01 || diff < -0.01 {
		t.Errorf("a flake fell to %.4f in thirty frames but %.4f in sixty", slowY, fastY)
	}
	if diff := slowX - fastX; diff > 0.01 || diff < -0.01 {
		t.Errorf("a flake swayed to %.4f in thirty frames but %.4f in sixty: "+
			"the sway still depends on the frame rate", slowX, fastX)
	}
	if slowY < 5.9 || slowY > 6.1 {
		t.Errorf("a flake at six pixels a second fell %.2f pixels in a second", slowY)
	}
}

func TestAccumulatesAtTheSameRateAtAnyFrameRate(t *testing.T) {
	// The bank is built by landings and worn down by melting, both of which are
	// rates. Ten seconds of snow should leave about the same depth whether it
	// was drawn at thirty frames a second or sixty.
	build := func(step float64, frames int) int {
		s := New(3)
		s.Resize(tw, th)
		surf := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			s.Frame(surf, step)
		}
		return total(s.ground)
	}
	slow, fast := build(1.0/30, 300), build(1.0/60, 600)
	// Not identical: the flakes are randomly placed and land in a different
	// order. Within a fifth is the claim — that the rate is the same, not that
	// the two runs are the same run.
	if d := slow - fast; d*5 > slow || -d*5 > slow {
		t.Errorf("ten seconds of snow left %d pixels at thirty frames and %d at sixty",
			slow, fast)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() ([]flake, []int) {
		s := New(9)
		s.Resize(tw, th)
		surf := canvas.NewSurface(tw, th)
		for i := 0; i < 300; i++ {
			s.Frame(surf, dt)
		}
		return s.flakes, s.ground
	}
	fa, ga := run()
	fb, gb := run()
	for i := range fa {
		if fa[i] != fb[i] {
			t.Fatalf("same seed diverged at flake %d", i)
		}
	}
	for i := range ga {
		if ga[i] != gb[i] {
			t.Fatalf("same seed diverged at ground column %d", i)
		}
	}
}
