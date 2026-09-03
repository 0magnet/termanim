package sand

import (
	"testing"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 40, 30

// dt is one tick of the old thirty-a-second loop, which at the default rate is
// exactly the one settle step a frame used to run.
const dt = 1.0 / 30

// still returns a sandbox with the nozzles turned off, so a test can place
// grains by hand and watch only what the falling rules do to them.
func still(seed int64) *Sand {
	s := New(seed)
	s.Streams = 0
	s.Resize(tw, th)
	return s
}

func (s *Sand) at(x, y int) byte { return s.grid[y*s.w+x] }

func (s *Sand) put(x, y int, v byte) { s.grid[y*s.w+x] = v }

func (s *Sand) count() int {
	var n int
	for _, v := range s.grid {
		if v != 0 {
			n++
		}
	}
	return n
}

func TestGrainFalls(t *testing.T) {
	s := still(1)
	s.put(5, 0, 40)
	s.step()
	if s.at(5, 0) != 0 {
		t.Error("the grain did not leave the cell it fell from")
	}
	if s.at(5, 1) != 40 {
		t.Error("the grain did not arrive one cell down")
	}
}

func TestAGrainFallsOnlyOneCellPerFrame(t *testing.T) {
	// The classic bug in this algorithm is a top-down sweep, which drops a
	// grain the whole height of the screen in a single frame. It shows up as
	// nothing ever being visible in mid-air.
	s := still(1)
	s.put(5, 0, 40)
	for y := 1; y < th; y++ {
		s.step()
		if s.at(5, y) != 40 {
			t.Fatalf("after %d frames the grain is not at row %d", y, y)
		}
	}
}

func TestGrainRestsOnASettledGrain(t *testing.T) {
	s := still(1)
	for x := 0; x < tw; x++ {
		s.put(x, th-1, 90) // a full floor, so nothing can slide sideways
	}
	s.put(5, th-3, 40)
	before := s.count()
	s.step()
	s.step()
	s.step()
	if s.at(5, th-2) != 40 {
		t.Error("the grain did not come to rest on top of the floor")
	}
	if s.at(5, th-1) != 90 {
		t.Error("the grain fell through a settled grain")
	}
	if got := s.count(); got != before {
		t.Errorf("%d grains, started with %d", got, before)
	}
}

func TestGrainSlidesOffAPeak(t *testing.T) {
	s := still(1)
	s.put(5, th-1, 90)
	s.put(5, th-2, 40)
	s.step()
	if s.at(5, th-2) != 0 {
		t.Fatal("a grain balanced on a single grain instead of sliding off")
	}
	if s.at(4, th-1) != 40 && s.at(6, th-1) != 40 {
		t.Fatal("the grain went somewhere other than a diagonal")
	}
}

func TestDiagonalChoiceIsNotBiased(t *testing.T) {
	var left, right int
	for seed := int64(0); seed < 60; seed++ {
		s := still(seed)
		s.put(5, th-1, 90)
		s.put(5, th-2, 40)
		s.step()
		if s.at(4, th-1) == 40 {
			left++
		}
		if s.at(6, th-1) == 40 {
			right++
		}
	}
	if left == 0 || right == 0 {
		t.Fatalf("grains only ever slide one way: left=%d right=%d", left, right)
	}
}

func TestNothingEscapesTheGrid(t *testing.T) {
	s := still(9)
	for i := range s.grid {
		if i%7 == 0 {
			s.grid[i] = byte(i%200 + 1)
		}
	}
	before := s.count()
	for i := 0; i < 300; i++ {
		s.step()
	}
	if got := s.count(); got != before {
		t.Fatalf("%d grains after settling, started with %d", got, before)
	}
	// Everything must have ended up in a contiguous stack against the floor:
	// nothing left through a wall or the bottom.
	for x := 0; x < tw; x++ {
		var gap bool
		for y := th - 1; y >= 0; y-- {
			if s.at(x, y) == 0 {
				gap = true
			} else if gap {
				t.Fatalf("column %d has a grain floating above a hole", x)
			}
		}
	}
}

// deepest is the tallest stack resting on the floor: the height of the biggest
// heap, ignoring whatever is still in the air.
func (s *Sand) deepest() int {
	var best int
	for x := 0; x < s.w; x++ {
		var run int
		for y := s.h - 1; y >= 0 && s.at(x, y) != 0; y-- {
			run++
		}
		if run > best {
			best = run
		}
	}
	return best
}

func TestPilesGrow(t *testing.T) {
	s := New(3)
	s.Resize(tw, th)
	for i := 0; i < 40; i++ {
		s.step()
	}
	early := s.deepest()
	for i := 0; i < 300; i++ {
		s.step()
	}
	late := s.deepest()
	if late <= early {
		t.Fatalf("no heap grew: tallest stack was %d grains, now %d", early, late)
	}
	if late < 3 {
		t.Fatalf("tallest stack is %d grains; nothing piled up", late)
	}
}

func TestBandsAreColoured(t *testing.T) {
	s := New(3)
	s.Resize(tw, th)
	for i := 0; i < 400; i++ {
		s.step()
	}
	seen := map[byte]bool{}
	for _, v := range s.grid {
		if v != 0 {
			seen[v] = true
		}
	}
	if len(seen) < 3 {
		t.Fatalf("only %d grain colours in the box; the piles will not be banded", len(seen))
	}
}

func TestAFullBoxKeepsMoving(t *testing.T) {
	s := New(5)
	s.Resize(tw, th)
	for i := 0; i < 4000; i++ {
		s.step()
	}
	if !s.backedUp() {
		t.Skip("the box never filled; nothing to test")
	}
	before := make([]byte, len(s.grid))
	copy(before, s.grid)
	for i := 0; i < 5; i++ {
		s.step()
	}
	for i := range before {
		if before[i] != s.grid[i] {
			return
		}
	}
	t.Fatal("a full box froze: the drain is not running")
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []byte {
		s := New(6)
		s.Resize(tw, th)
		for i := 0; i < 500; i++ {
			s.step()
		}
		out := make([]byte, len(s.grid))
		copy(out, s.grid)
		return out
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at index %d", i)
		}
	}
}

// framesOver drives the sand through Frame and reports the steps and the grain
// count that resulted.
func framesOver(frames int, step float64) (steps, grains int) {
	s := New(8)
	s.Resize(tw, th)
	surf := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		s.Frame(surf, step)
	}
	return s.steps, s.count()
}

func TestFallRateIsFrameRateIndependent(t *testing.T) {
	// Two seconds of wall clock at three frame rates. Sand falls one cell per
	// step at thirty steps a second, so the same amount of sand has fallen the
	// same distance in all three; before this, a faster terminal poured faster.
	slowSteps, slowGrains := framesOver(60, 1.0/30)
	fastSteps, fastGrains := framesOver(120, 1.0/60)
	oddSteps, oddGrains := framesOver(34, 1.0/17)
	if slowSteps < 58 || slowSteps > 62 {
		t.Fatalf("two seconds at 30fps ran %d steps, want about 60", slowSteps)
	}
	for _, got := range []int{fastSteps, oddSteps} {
		if d := got - slowSteps; d < -2 || d > 2 {
			t.Fatalf("same two seconds ran %d steps at one frame rate and %d at another", slowSteps, got)
		}
	}
	// The same number of steps must have poured about the same amount of sand.
	for _, got := range []int{fastGrains, oddGrains} {
		if d := got - slowGrains; d < -12 || d > 12 {
			t.Fatalf("%d grains at one frame rate and %d at another", slowGrains, got)
		}
	}
}

func TestOneFrameAtTheDefaultIntervalIsOneStep(t *testing.T) {
	s := still(1)
	s.put(5, 0, 40)
	surf := canvas.NewSurface(tw, th)
	s.Frame(surf, dt)
	if s.at(5, 1) != 40 {
		t.Fatal("a frame at the default interval did not move the grain one cell")
	}
	s.Frame(surf, dt/8)
	if s.at(5, 1) != 40 {
		t.Fatal("a frame shorter than the step interval moved the grain anyway")
	}
}

func TestFrameWritesToTheSurface(t *testing.T) {
	s := New(1)
	s.Resize(tw, th)
	surf := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ {
		s.Frame(surf, dt)
	}
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if surf.At(x, y) != 0 {
				return
			}
		}
	}
	t.Error("Frame left the surface empty")
}
