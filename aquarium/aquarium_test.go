package aquarium

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// dt is one frame at the rate these constants were tuned at, so every
// assertion about how far things move over N frames still means what it did.
const dt = 1.0 / 30

const tw, th = 60, 20

func newScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(tw, th)
	return s
}

func TestMirrorFlipsHandedGlyphs(t *testing.T) {
	// A mirrored fish whose fins still point the old way reads as broken
	// rather than reversed.
	m := mirror(newSprite(`>< >`))
	if got := m.rows[0]; got != `< ><` {
		t.Errorf("mirror(%q) = %q, want %q", `>< >`, got, `< ><`)
	}
	m = mirror(newSprite(` /\`, `>< >`, ` \/`))
	if !strings.Contains(m.rows[0], `\`) || !strings.Contains(m.rows[2], `/`) {
		t.Errorf("mirror did not swap the slashes: %q", m.rows)
	}
}

func TestMirrorPreservesWidth(t *testing.T) {
	// Ragged sprites must be padded before mirroring or their rows shift
	// relative to one another.
	s := newSprite(`  \`, `><_>`, `  /`)
	m := mirror(s)
	if m.w != s.w {
		t.Errorf("width changed under mirror: %d then %d", s.w, m.w)
	}
	for i, r := range m.rows {
		if len([]rune(r)) != s.w {
			t.Errorf("row %d is %d wide, want %d", i, len([]rune(r)), s.w)
		}
	}
}

func TestFishSwim(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	a := New(2)
	a.Resize(tw, th)
	before := make([]float64, len(a.fish))
	for i, f := range a.fish {
		before[i] = f.x
	}
	for i := 0; i < 20; i++ {
		a.Frame(s, tw, th, dt)
	}
	for i, f := range a.fish {
		if f.x != before[i] {
			return
		}
	}
	t.Error("no fish moved in twenty frames")
}

func TestFishGetDrawn(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	a := New(2)
	a.Resize(tw, th)
	for i := 0; i < 10; i++ {
		a.Frame(s, tw, th, dt)
	}
	s.Show()
	cells, w, _ := s.GetContents()
	var nonBlank int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			c := cells[y*w+x]
			if len(c.Runes) > 0 && c.Runes[0] != ' ' {
				nonBlank++
			}
		}
	}
	if nonBlank == 0 {
		t.Fatal("the tank is empty")
	}
}

func TestSeaweedSways(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	a := New(2)
	a.Resize(tw, th)
	// Find a column with weed.
	x := -1
	for i, h := range a.weed {
		if h > 0 {
			x = i
			break
		}
	}
	if x < 0 {
		t.Skip("no seaweed with this seed")
	}
	read := func() rune {
		s.Show()
		cells, w, _ := s.GetContents()
		c := cells[(th-1)*w+x]
		if len(c.Runes) == 0 {
			return ' '
		}
		return c.Runes[0]
	}
	a.Frame(s, tw, th, dt)
	first := read()
	for i := 0; i < 12; i++ { // the sway advances every 8 frames
		a.Frame(s, tw, th, dt)
	}
	if read() == first {
		t.Error("the seaweed never changed: it is not swaying")
	}
}

func TestFishStayInTheTank(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	a := New(4)
	a.Resize(tw, th)
	for i := 0; i < 400; i++ {
		a.Frame(s, tw, th, dt)
		for j, f := range a.fish {
			if int(f.y) < 0 || int(f.y)+f.s.h > th {
				t.Fatalf("frame %d: fish %d at row %.0f is outside a %d-row tank", i, j, f.y, th)
			}
		}
	}
}

func TestFishBodiesOccludeTheBackground(t *testing.T) {
	// A fish drawn over seaweed must hide it. Interior spaces are opaque; only
	// the spaces outside the drawn part of a row are transparent.
	s := newScreen(t)
	defer s.Fini()
	a := New(21)
	a.Resize(tw, th)
	// Force a deep-bodied fish over a weeded column at the bottom.
	x := -1
	for i, h := range a.weed {
		if h > 3 {
			x = i
			break
		}
	}
	if x < 0 {
		t.Skip("no tall seaweed with this seed")
	}
	deep := fishRight[4] // the one with a hollow body
	if deep.h > th {
		t.Skip("tank too short for the deep-bodied fish")
	}
	a.fish = []fish{{
		s: deep, x: float64(x - 2), y: float64(th - deep.h),
		speed: 0.3, colour: tcell.ColorWhite,
	}}
	a.Frame(s, tw, th, dt)
	s.Show()

	cells, w, _ := s.GetContents()
	for dy, row := range deep.rows {
		y := th - deep.h + dy
		rs := []rune(row)
		first, last := -1, -1
		for i, r := range rs {
			if r != ' ' {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		if first < 0 {
			continue
		}
		for dx := first; dx <= last; dx++ {
			cx := x - 2 + dx
			if cx < 0 || cx >= tw || y < 0 || y >= th {
				continue
			}
			got := cells[y*w+cx]
			if len(got.Runes) == 0 {
				continue
			}
			if got.Runes[0] != rs[dx] {
				t.Fatalf("at %d,%d the fish drew %q but the cell holds %q: something showed through the body",
					cx, y, rs[dx], got.Runes[0])
			}
		}
	}
}

func TestTallFishAreNotPickedForShortTanks(t *testing.T) {
	a := New(1)
	a.Resize(30, 5) // shorter than the angelfish
	for _, s := range a.fits() {
		if s.h > 4 {
			t.Errorf("a %d-row fish is offered for a 5-row tank", s.h)
		}
	}
}

// The tank is stepped at a fixed rate from elapsed time, so the same
// wall-clock interval must swim the fish the same distance however it is
// divided into frames.
func TestFrameRateIndependent(t *testing.T) {
	run := func(frames int, step float64) []fish {
		s := tcell.NewSimulationScreen("UTF-8")
		_ = s.Init()
		defer s.Fini()
		s.SetSize(tw, th)
		a := New(6)
		a.Resize(tw, th)
		for i := 0; i < frames; i++ {
			a.Frame(s, tw, th, step)
		}
		return a.fish
	}
	slow := run(60, 1.0/30)  // 2 seconds
	fast := run(120, 1.0/60) // the same 2 seconds
	if len(slow) != len(fast) {
		t.Fatalf("different fish counts: %d at 30fps, %d at 60fps", len(slow), len(fast))
	}
	// Tolerance is one step's travel. Neither 1/30 nor 1/60 is exact in
	// binary, so the two accumulators can land a single step apart over two
	// seconds. A rate that still followed the frame count would be out by a
	// factor of two — around forty steps — which this still catches.
	const maxStepTravel = 0.6 // the fastest a fish swims in one step
	for i := range slow {
		d := slow[i].x - fast[i].x
		if d < 0 {
			d = -d
		}
		if d > maxStepTravel {
			t.Errorf("fish %d swam %.3f columns apart over two seconds, more than one step", i, d)
		}
	}
}

// TestFishInFishRightActuallyFaceRight is the test that was missing.
//
// The sprites live in fishRight and newFish mirrors them when a fish swims
// left, so a sprite drawn facing the wrong way makes every fish of that design
// swim backwards. Checking that the fins mirrored correctly did not catch it,
// because a backwards fish is internally consistent — it is only wrong
// relative to its direction of travel.
//
// A fish's eye is near its head, so the eye being in the right-hand half is
// what "faces right" means here.
func TestFishInFishRightActuallyFaceRight(t *testing.T) {
	for i, s := range fishRight {
		eye := -1
		for _, row := range s.rows {
			for c, r := range []rune(row) {
				if r == 'o' {
					eye = c
				}
			}
		}
		if eye < 0 {
			continue // the small fish have no drawn eye
		}
		if eye < s.w/2 {
			t.Errorf("fish %d has its eye at column %d of %d — it faces left, but it is in fishRight",
				i, eye, s.w)
		}
	}
}

// And mirroring one must put the eye on the other side.
func TestMirroringMovesTheEye(t *testing.T) {
	for i, s := range fishRight {
		eye := -1
		for _, row := range s.rows {
			for c, r := range []rune(row) {
				if r == 'o' {
					eye = c
				}
			}
		}
		if eye < 0 {
			continue
		}
		m := mirror(s)
		meye := -1
		for _, row := range m.rows {
			for c, r := range []rune(row) {
				if r == 'o' {
					meye = c
				}
			}
		}
		if meye < 0 {
			t.Errorf("fish %d lost its eye when mirrored", i)
			continue
		}
		if meye >= m.w/2 {
			t.Errorf("fish %d mirrored still has its eye at column %d of %d — it did not turn around",
				i, meye, m.w)
		}
	}
}
