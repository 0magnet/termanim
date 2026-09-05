package bonsai

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/internal/simscreen"
)

const tw, th = 60, 24

// dt is the elapsed time handed to Frame in these tests: a thirtieth of a
// second, the rate the animation used to be tied to.
const dt = 1.0 / 30

func newScreen(t *testing.T) tcell.Screen {
	t.Helper()
	s := simscreen.NewScreen()
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(tw, th)
	return s
}

// grow runs frames until the tree is finished, or fails if it never is.
func grow(t *testing.T, b *Bonsai, s tcell.Screen) int { //nolint:unparam
	t.Helper()
	for i := 0; i < 5000; i++ {
		if b.Done() {
			return i
		}
		b.Frame(s, tw, th, dt)
	}
	t.Fatalf("the tree was still growing after 5000 frames, %d branches live", len(b.live))
	return 0
}

func TestTreeGrowsOverFrames(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	b := New(1)
	b.Resize(tw, th)
	prev := b.occupied
	if prev != 0 {
		t.Fatalf("a freshly planted tree already occupies %d cells", prev)
	}
	grew := 0
	for i := 0; i < 200 && !b.Done(); i++ {
		b.Frame(s, tw, th, dt)
		if b.occupied < prev {
			t.Fatalf("frame %d: the tree shrank, %d cells then %d", i, prev, b.occupied)
		}
		if b.occupied > prev {
			grew++
		}
		prev = b.occupied
	}
	if grew < 10 {
		t.Errorf("the tree only grew on %d frames: it is not being drawn incrementally", grew)
	}
}

func TestTreeStartsAtTheBottom(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	b := New(2)
	b.Resize(tw, th)
	b.Frame(s, tw, th, dt)
	// The first cells drawn must be on the bottom row, near the middle.
	found := -1
	for x := 0; x < tw; x++ {
		if b.buf[(th-1)*tw+x].r != 0 {
			found = x
		}
	}
	if found < 0 {
		t.Fatal("nothing was drawn on the bottom row: the tree is not rooted")
	}
	if found < tw/2-4 || found > tw/2+4 {
		t.Errorf("the trunk started at column %d, nowhere near the middle of %d", found, tw)
	}
}

func TestTreeReachesUpward(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	b := New(3)
	b.Resize(tw, th)
	grow(t, b, s)
	top := th
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if b.buf[y*tw+x].r != 0 {
				if y < top {
					top = y
				}
				break
			}
		}
	}
	// A trunk of half the window plus a canopy should clear the halfway line.
	if top > th/2 {
		t.Errorf("the tree only reached row %d of %d: it is a stump", top, th)
	}
}

func TestBranchesTerminate(t *testing.T) {
	// Lives halve at every level, so the recursion is bounded however the dice
	// fall. Check across many seeds, since it is a probabilistic claim.
	s := newScreen(t)
	defer s.Fini()
	for seed := int64(0); seed < 30; seed++ {
		b := New(seed)
		b.StepsPerSecond = 2000 // finish quickly; the pacing is not what is tested
		b.Resize(tw, th)
		grow(t, b, s)
		if len(b.live) != 0 {
			t.Fatalf("seed %d: %d branches never died", seed, len(b.live))
		}
	}
}

func TestLeavesAppear(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	b := New(4)
	b.Resize(tw, th)
	grow(t, b, s)
	leaves := 0
	for _, c := range b.buf {
		if c.r != 0 && strings.ContainsRune(string(LeafGlyphs), c.r) {
			leaves++
		}
	}
	if leaves < 10 {
		t.Errorf("the finished tree has %d leaf characters: it is bare", leaves)
	}
}

func TestTrunkAndShootsUseDifferentGlyphs(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	b := New(5)
	b.Resize(tw, th)
	grow(t, b, s)
	var heavy, light int
	for _, c := range b.buf {
		switch c.r {
		case TrunkGlyphs.Vertical, TrunkGlyphs.Horizontal, TrunkGlyphs.Left, TrunkGlyphs.Right:
			heavy++
		case ShootGlyphs.Vertical, ShootGlyphs.Horizontal, ShootGlyphs.Left, ShootGlyphs.Right:
			light++
		}
	}
	if heavy == 0 {
		t.Error("no trunk strokes were drawn")
	}
	if light == 0 {
		t.Error("no shoot strokes were drawn: the tree has no branches")
	}
}

// b1step is the most cells one growth step can paint: a stroke, the second
// stroke of a wide trunk, and a leaf cluster if the branch died.
const b1step = 2 + 9

func TestFrameRateDoesNotChangeTheSpeed(t *testing.T) {
	// The same elapsed time must grow the same amount of tree. Same seed, so
	// equal step counts mean identical trees.
	grow := func(step float64, frames int) *Bonsai {
		s := newScreen(t)
		defer s.Fini()
		b := New(12)
		b.Resize(tw, th)
		for i := 0; i < frames; i++ {
			b.Frame(s, tw, th, step)
		}
		return b
	}
	slow := grow(1.0/30, 30) // one second either way
	fast := grow(1.0/60, 60)
	// Within one step, which can draw a stroke and a whole leaf cluster: the
	// two accumulators can land a single step apart because neither frame
	// length is exact in binary. A rate tied to the frame rate would be out by
	// a factor of two.
	if d := slow.occupied - fast.occupied; d < -(b1step) || d > b1step {
		t.Errorf("30fps grew %d cells and 60fps grew %d: the speed follows the frame rate",
			slow.occupied, fast.occupied)
	}
	if d := len(slow.live) - len(fast.live); d < -1 || d > 1 {
		t.Errorf("%d branches live at 30fps but %d at 60fps", len(slow.live), len(fast.live))
	}
	if slow.occupied == 0 {
		t.Error("nothing grew in a second")
	}
}

func TestANewTreeReplacesTheOld(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	b := New(6)
	b.StepsPerSecond = 2000
	b.HoldSeconds = 0.1
	b.Resize(tw, th)
	first := b.trees
	grow(t, b, s)
	full := b.occupied
	b.StepsPerSecond = 30 // so the replanting frame is caught before it grows
	for i := 0; i < 10 && b.trees == first; i++ {
		b.Frame(s, tw, th, dt)
	}
	if b.trees == first {
		t.Fatal("the finished tree was never replaced")
	}
	if b.occupied >= full {
		t.Errorf("the canvas was not cleared for the new tree: %d cells then %d", full, b.occupied)
	}
}

func TestDrawnOnScreen(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	b := New(7)
	b.StepsPerSecond = 2000
	b.Resize(tw, th)
	grow(t, b, s)
	b.Frame(s, tw, th, dt)
	s.Show()
	cells, w, _ := simscreen.Contents(s)
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
		t.Fatal("the screen is empty")
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	s := simscreen.NewScreen()
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(4, 3)
	b := New(8)
	b.Resize(4, 3)
	for i := 0; i < 200; i++ {
		b.Frame(s, 4, 3, dt)
	}
}
