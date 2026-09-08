package lightning

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 96, 64

// dt is one tick of a thirty-a-second loop.
const dt = 1.0 / 30

// light is the total light in the air, which is what the flash multiplies and
// the afterglow eats.
func light(l *Lightning) float64 {
	var sum float64
	for _, v := range l.glow {
		sum += float64(v)
	}
	return sum
}

// deepest is the lowest row holding any light at all. It reads the field
// rather than the surface, because a pixel that has decayed below Cutoff is
// no longer drawn but the stroke still got there.
func deepest(l *Lightning) int {
	for y := l.h - 1; y >= 0; y-- {
		for x := 0; x < l.w; x++ {
			if l.glow[y*l.w+x] > 0 {
				return y
			}
		}
	}
	return -1
}

func TestTheLeaderDescendsBeforeItFlashes(t *testing.T) {
	// The order of events is the whole effect. If the stroke appeared whole
	// and then flashed, this would be a blinking picture of lightning rather
	// than a discharge.
	l := New(1)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	var half int
	last := -1
	for l.strikes == 0 {
		l.Frame(s, dt)
		d := deepest(l)
		if d < last {
			t.Fatalf("the stroke reached row %d and then only row %d: it is not descending", last, d)
		}
		last = d
		if half == 0 && l.progress >= 0.5 {
			half = d
		}
	}
	if half <= 0 {
		t.Fatal("nothing was lit by the time the leader was halfway down")
	}
	if half > th*9/10 {
		t.Errorf("halfway down the descent the stroke already reaches row %d of %d: "+
			"it is not descending, it is appearing", half, th)
	}
	if last < th-2 {
		t.Errorf("the stroke stopped at row %d of %d: it never reached the ground", last, th)
	}
}

func TestTheReturnStrokeIsFarBrighterThanTheLeader(t *testing.T) {
	l := New(2)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	var before float64
	for l.strikes == 0 {
		before = light(l)
		l.Frame(s, dt)
	}
	after := light(l)
	if after < before*2 {
		t.Errorf("the air held %.0f before the flash and %.0f after: "+
			"the return stroke is not a flash", before, after)
	}
	if l.ambient <= 0 {
		t.Error("the strike threw no light on the rest of the surface")
	}
}

func TestTheAfterglowDecaysAndTheAirGoesDarkAgain(t *testing.T) {
	// An afterglow is a decay, not a second animation: the field has to fall
	// every frame, and it has to get out of the way before the next stroke.
	l := New(3)
	// One long pause, so the whole decay is observed with nothing added to it.
	l.MinPause, l.MaxPause = 6, 6
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for l.strikes == 0 {
		l.Frame(s, dt)
	}
	peak := light(l)
	prev := peak
	for i := 0; i < 60; i++ {
		l.Frame(s, dt)
		if got := light(l); got >= prev {
			t.Fatalf("frame %d after the flash the air holds %.2f, up from %.2f", i, got, prev)
		} else {
			prev = got
		}
	}
	if prev > peak*0.02 {
		t.Errorf("two seconds after the flash the air still holds %.2f of its %.2f peak", prev, peak)
	}
	// And the screen is actually blank, not merely dim: Cutoff is what stops
	// the surface settling into a permanent faint wash.
	lit := 0
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				lit++
			}
		}
	}
	if lit > tw*th/20 {
		t.Errorf("%d of %d pixels are still lit two seconds after the flash", lit, tw*th)
	}
}

func TestTheChannelIsJaggedAtEveryScale(t *testing.T) {
	// What midpoint displacement buys over a wobbly line. Two things have to
	// hold: the path is much longer than the straight line it spans, and the
	// displacement at each round of subdivision is bounded by half the round
	// before, which is what makes the jaggedness self-similar rather than all
	// at one frequency.
	l := New(4)
	l.Resize(tw, th)
	n := len(l.px)
	var path float64
	for i := 0; i+1 < n; i++ {
		path += math.Hypot(l.px[i+1]-l.px[i], l.py[i+1]-l.py[i])
	}
	straight := math.Hypot(l.px[n-1]-l.px[0], l.py[n-1]-l.py[0])
	if path < straight*1.15 {
		t.Errorf("the channel is %.1f pixels long across a %.1f pixel span: that is a wire",
			path, straight)
	}

	// Each level's midpoints must lie within that level's displacement of the
	// chord they were placed on.
	bound := l.Roughness * l.fw
	for step := n - 1; step > 1; step /= 2 {
		half := step / 2
		var worst float64
		for i := half; i < n; i += step {
			d := math.Abs(l.px[i] - (l.px[i-half]+l.px[i+half])/2)
			if d > worst {
				worst = d
			}
		}
		if worst > bound+1e-9 {
			t.Errorf("at interval %d a midpoint sits %.2f off its chord, past the %.2f budget",
				step, worst, bound)
		}
		bound *= 0.55
	}
}

func TestBranchesLeaveTheChannelAfterTheLeaderPasses(t *testing.T) {
	l := New(5)
	l.Resize(tw, th)
	n := len(l.px)
	channel := n - 1
	if len(l.segs) != channel+l.Branches*l.BranchSegments {
		t.Fatalf("%d segments for a %d-piece channel and %d branches of %d",
			len(l.segs), channel, l.Branches, l.BranchSegments)
	}
	for b := 0; b < l.Branches; b++ {
		root := l.segs[channel+b*l.BranchSegments]
		if root.main {
			t.Fatalf("branch %d is marked as part of the main channel", b)
		}
		// The root has to sit on a point of the channel, and light up after
		// the leader has been there.
		at := -1
		for i := 0; i < n; i++ {
			if l.px[i] == root.x0 && l.py[i] == root.y0 {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatalf("branch %d starts at %.2f,%.2f, which is not on the channel",
				b, root.x0, root.y0)
		}
		if want := float64(at) / float64(n-1); root.order < want {
			t.Errorf("branch %d lights at %.4f but the leader only reaches its root at %.4f",
				b, root.order, want)
		}
		// And it must be dimmer than the channel it came off, or the eye
		// cannot tell which way the discharge went.
		if root.amp >= l.segs[0].amp {
			t.Errorf("branch %d is drawn at %.2f against the channel's %.2f", b, root.amp, l.segs[0].amp)
		}
	}
}

func TestStrikesKeepComingAndAreSpacedOut(t *testing.T) {
	l := New(6)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	seen, gap, worst := 0, 0.0, math.Inf(1)
	for i := 0; i < 30*30; i++ { // thirty seconds
		before := l.strikes
		l.Frame(s, dt)
		gap += dt
		if l.strikes > before {
			if seen > 0 && gap < worst {
				worst = gap
			}
			seen++
			gap = 0
		}
	}
	if seen < 8 {
		t.Errorf("only %d strikes in thirty seconds", seen)
	}
	// Every gap is a whole descent plus at least the minimum pause. Half a
	// frame of slack for the time carried across the phase change.
	if floor := l.DescendTime + l.MinPause - dt; worst < floor {
		t.Errorf("two strikes came %.3fs apart, inside the %.3fs floor", worst, floor)
	}
}

func TestStrikeRateIsFrameRateIndependent(t *testing.T) {
	// Twenty seconds of wall clock at three frame rates. Each phase carries
	// its leftover time into the next, so the same strikes happen at the same
	// moments however often the screen was drawn.
	strikesOver := func(frames int, step float64) int {
		l := New(7)
		l.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			l.Frame(s, step)
		}
		return l.strikes
	}
	slow := strikesOver(600, 1.0/30)
	fast := strikesOver(1200, 1.0/60)
	odd := strikesOver(340, 1.0/17)
	if slow < 5 {
		t.Fatalf("only %d strikes in twenty seconds", slow)
	}
	for _, got := range []int{fast, odd} {
		if got != slow {
			t.Errorf("twenty seconds gave %d strikes at one frame rate and %d at another", slow, got)
		}
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) []seg {
		l := New(seed)
		l.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 200; i++ {
			l.Frame(s, dt)
		}
		out := make([]seg, len(l.segs))
		copy(out, l.segs)
		return out
	}
	a, b := run(0), run(0)
	if len(a) != len(b) {
		t.Fatalf("same seed gave %d segments and then %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at segment %d", i)
		}
	}
	c := run(9)
	same := len(a) == len(c)
	for i := range a {
		if !same || a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds struck in exactly the same place")
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {3, 2}, {8, 6}} {
		l := New(8)
		l.Resize(sz[0], sz[1])
		s := canvas.NewSurface(max(sz[0], 1), max(sz[1], 1))
		for i := 0; i < 200; i++ {
			l.Frame(s, dt)
		}
	}
}

func TestFrameDoesNotAllocate(t *testing.T) {
	// Building a whole new stroke happens inside Frame, so the scratch the
	// midpoint displacement works in and the segment list it fills are both
	// sized once in Resize and reused for every strike thereafter.
	l := New(10)
	l.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 40; i++ {
		l.Frame(s, dt)
	}
	if n := testing.AllocsPerRun(200, func() { l.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}
