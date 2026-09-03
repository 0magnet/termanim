package pipes

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

const tw, th = 60, 20

// dt is the elapsed time handed to Frame in these tests: a thirtieth of a
// second, the rate the animation used to be tied to.
const dt = 1.0 / 30

func newScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(tw, th)
	return s
}

// connects reports the sides a glyph joins, which is the only thing that
// matters about it: two glyphs meet cleanly when the side one leaves by is the
// side the next is entered through.
func connects(s Set, r rune) (uint8, bool) {
	switch r {
	case s.Horizontal:
		return sides(Left, Right), true
	case s.Vertical:
		return sides(Up, Down), true
	case s.UpRight:
		return sides(Up, Right), true
	case s.UpLeft:
		return sides(Up, Left), true
	case s.DownRight:
		return sides(Down, Right), true
	case s.DownLeft:
		return sides(Down, Left), true
	}
	return 0, false
}

func TestGlyphJoinsTheSidesItShould(t *testing.T) {
	// For every way through a cell, the glyph drawn must connect exactly the
	// side the pipe came in through and the side it left by. This is the whole
	// correctness of the effect.
	for _, set := range []Set{Light, Heavy} {
		for in := Up; in <= Left; in++ {
			for out := Up; out <= Left; out++ {
				if out == in.opposite() {
					continue // a 180 degree turn is never generated
				}
				r := set.glyph(in, out)
				got, ok := connects(set, r)
				if !ok {
					t.Fatalf("in=%d out=%d drew %q, which is not in the set", in, out, r)
				}
				want := sides(in.opposite(), out)
				if got != want {
					t.Errorf("in=%d out=%d drew %q joining %04b, want %04b", in, out, r, got, want)
				}
			}
		}
	}
}

func TestTurnsDrawCornersAndStraightsDrawStraights(t *testing.T) {
	for _, set := range []Set{Light, Heavy} {
		for d := Up; d <= Left; d++ {
			straight := set.glyph(d, d)
			if straight != set.Horizontal && straight != set.Vertical {
				t.Errorf("going straight in direction %d drew %q, not a straight piece", d, straight)
			}
			for _, cw := range []bool{true, false} {
				out := d.turn(cw)
				corner := set.glyph(d, out)
				if corner == set.Horizontal || corner == set.Vertical {
					t.Errorf("turning from %d to %d drew the straight piece %q", d, out, corner)
				}
			}
		}
	}
}

func TestSpecificCorners(t *testing.T) {
	// Spelled out so a future rearrangement of the table cannot quietly rotate
	// every elbow by ninety degrees and still pass the symmetric tests above.
	cases := []struct {
		in, out Dir
		want    rune
	}{
		{Down, Right, '└'}, // came from above, leaves right
		{Down, Left, '┘'},
		{Up, Right, '┌'}, // came from below, leaves right
		{Up, Left, '┐'},
		{Right, Up, '┘'}, // came from the left, leaves upward
		{Right, Down, '┐'},
		{Left, Up, '└'},
		{Left, Down, '┌'},
	}
	for _, c := range cases {
		if got := Light.glyph(c.in, c.out); got != c.want {
			t.Errorf("glyph(in=%d,out=%d) = %q, want %q", c.in, c.out, got, c.want)
		}
	}
}

func TestPipeAdvancesOneCellPerStep(t *testing.T) {
	p := New(1)
	p.Resize(tw, th)
	q := &p.pipe[0]
	for i := 0; i < 200; i++ {
		x, y := q.x, q.y
		p.step(q)
		// A respawn teleports; skip those and check the rest.
		dx, dy := q.x-x, q.y-y
		if dx*dx+dy*dy > 1 {
			continue
		}
		if dx*dx+dy*dy != 1 {
			t.Fatalf("step %d: pipe did not move: %d,%d to %d,%d", i, x, y, q.x, q.y)
		}
	}
}

func TestTrailIsContinuous(t *testing.T) {
	// Walk a single pipe and check every pair of consecutive glyphs meets: the
	// side one leaves by has to be the side the next is entered through. This
	// is what "no broken joints" means when it is written down.
	p := New(7)
	p.Count = 1
	p.Resize(tw, th)
	q := &p.pipe[0]
	prevDir := q.dir
	prevX, prevY := q.x, q.y
	for i := 0; i < 500; i++ {
		p.step(q)
		if q.x-prevX > 1 || prevX-q.x > 1 || q.y-prevY > 1 || prevY-q.y > 1 {
			// Respawned at an edge; start a new run.
			prevDir, prevX, prevY = q.dir, q.x, q.y
			continue
		}
		got, ok := connects(q.set, p.buf[prevY*p.cols+prevX].r)
		if !ok {
			t.Fatalf("step %d: cell %d,%d holds a glyph outside the set", i, prevX, prevY)
		}
		// The glyph just drawn must join the side the pipe entered by and the
		// side it has now left by.
		want := sides(prevDir.opposite(), q.dir)
		if got != want {
			t.Fatalf("step %d at %d,%d: glyph joins %04b, want %04b", i, prevX, prevY, got, want)
		}
		prevDir, prevX, prevY = q.dir, q.x, q.y
	}
}

func TestFrameRateDoesNotChangeTheSpeed(t *testing.T) {
	// Twice as many frames of half the length must grow the same amount of
	// pipe. Same seed, so equal step counts mean identical screens.
	grow := func(step float64, frames int) *Pipes {
		s := newScreen(t)
		defer s.Fini()
		p := New(9)
		p.Resize(tw, th)
		for i := 0; i < frames; i++ {
			p.Frame(s, tw, th, step)
		}
		return p
	}
	slow := grow(1.0/30, 90) // three seconds either way
	fast := grow(1.0/60, 180)
	// Within one step: neither a thirtieth nor a sixtieth of a second is exact
	// in binary, so the accumulators can land a single step apart over three
	// seconds. A rate that followed the frame rate would be out by a factor of
	// two, not by one step.
	if d := slow.filled - fast.filled; d < -len(slow.pipe) || d > len(slow.pipe) {
		t.Errorf("30fps painted %d cells and 60fps painted %d: the speed follows the frame rate",
			slow.filled, fast.filled)
	}
	// And it is actually growing, or the comparison proves nothing.
	if want := int(3 * slow.StepsPerSecond); slow.filled < want/2 {
		t.Errorf("only %d cells painted in three seconds at %g cells a second", slow.filled, slow.StepsPerSecond)
	}
}

func TestScreenClearsWhenFull(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	p := New(3)
	p.Resize(tw, th)
	peak := 0
	for i := 0; i < 4000 && p.cleared == 0; i++ {
		if p.filled > peak {
			peak = p.filled
		}
		p.Frame(s, tw, th, dt)
	}
	if p.cleared == 0 {
		t.Fatalf("the screen never filled: reached %d of %d cells", peak, tw*th)
	}
	if p.filled > peak {
		t.Fatalf("filled count %d did not drop after the wipe", p.filled)
	}
	if p.filled >= int(p.FillFraction*float64(tw*th)) {
		t.Errorf("after the wipe %d cells are still painted", p.filled)
	}
}

func TestDeterministicForASeed(t *testing.T) {
	read := func(seed int64) []rune {
		s := newScreen(t)
		defer s.Fini()
		p := New(seed)
		p.Resize(tw, th)
		for i := 0; i < 50; i++ {
			p.Frame(s, tw, th, dt)
		}
		s.Show()
		cells, w, _ := s.GetContents()
		out := make([]rune, 0, tw*th)
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				c := cells[y*w+x]
				if len(c.Runes) == 0 {
					out = append(out, ' ')
					continue
				}
				out = append(out, c.Runes[0])
			}
		}
		return out
	}
	a, b := read(11), read(11)
	if string(a) != string(b) {
		t.Error("the same seed drew two different screens")
	}
	if string(a) == string(read(12)) {
		t.Error("two different seeds drew the same screen")
	}
}

func TestPipesStayOnScreen(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	p := New(5)
	p.Resize(tw, th)
	for i := 0; i < 2000; i++ {
		p.Frame(s, tw, th, dt)
		for j, q := range p.pipe {
			if q.x < 0 || q.y < 0 || q.x >= tw || q.y >= th {
				t.Fatalf("frame %d: pipe %d is at %d,%d, outside %dx%d", i, j, q.x, q.y, tw, th)
			}
		}
	}
}
