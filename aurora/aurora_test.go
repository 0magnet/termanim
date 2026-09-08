package aurora

import (
	"math"
	"sort"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 80, 60

// dt is one frame of the thirty-a-second loop these constants were tuned at.
const dt = 1.0 / 30

// single returns an aurora with one curtain in it, run for a moment so that
// the sky is not in its starting pose. One curtain, because the properties
// below are about the shape of a curtain and two of them overlapping is a sum
// of two shapes.
func single(seed int64) *Aurora {
	a := New(seed)
	a.Count = 1
	a.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 40; i++ {
		a.Frame(s, dt)
	}
	return a
}

// border returns the lowest lit pixel in a column, or -1.
func border(a *Aurora, x int) int {
	for y := a.h - 1; y >= 0; y-- {
		if float64(a.light[y*a.w+x]) > a.Floor {
			return y
		}
	}
	return -1
}

// A curtain is brightest at its lower border and fades from there upward. That
// is the shape of the thing: the electrons are used up as they descend, so
// there is no bright top and no top edge either.
func TestBrightestAtTheBaseAndFadingUpward(t *testing.T) {
	a := single(1)
	var checked int
	for x := 0; x < a.w; x++ {
		b := border(a, x)
		if b < 10 {
			continue // too little column above it to say anything
		}
		checked++
		prev := a.light[b*a.w+x]
		for y := b - 1; y >= 0; y-- {
			v := a.light[y*a.w+x]
			if v > prev {
				t.Fatalf("column %d brightens going up: %.4f at row %d against %.4f at row %d",
					x, v, y, prev, y+1)
			}
			prev = v
		}
	}
	if checked < a.w/4 {
		t.Fatalf("only %d of %d columns had a curtain in them", checked, a.w)
	}
}

// The lower border is a hard edge, not a fade. A curtain that faded out below
// as well as above reads as a cloud; the sharp bottom is most of what makes
// this an aurora.
func TestTheLowerBorderIsSharp(t *testing.T) {
	a := single(2)
	var steps int
	for x := 0; x < a.w; x++ {
		b := border(a, x)
		if b < 0 || b >= a.h-1 {
			continue
		}
		if v := a.light[(b+1)*a.w+x]; v != 0 {
			t.Fatalf("column %d has light %.4f below its border at row %d", x, v, b)
		}
		// And the pixel at the border must be properly lit rather than the
		// tail of a fade that happened to cross the floor there.
		if float64(a.light[b*a.w+x]) > a.Floor*4 {
			steps++
		}
	}
	if steps < a.w/4 {
		t.Errorf("only %d columns step from dark to bright at the border", steps)
	}
}

// The folds are the point. A curtain is a sheet, and where its track turns
// back toward the viewer many samples land in one column: that column looks
// along the sheet instead of through it and comes out several times brighter.
// Nothing draws them, so if the projection were wrong they would simply be
// absent and the curtain would be a flat band.
func TestFoldsAreBrighterThanTheFlatSheet(t *testing.T) {
	a := single(3)
	var w []float64
	for x := 0; x < a.w; x++ {
		if b := border(a, x); b >= 0 {
			w = append(w, float64(a.light[b*a.w+x]))
		}
	}
	if len(w) < 10 {
		t.Fatal("hardly any of the curtain landed on the screen")
	}
	sort.Float64s(w)
	median := w[len(w)/2]
	peak := w[len(w)-1]
	if peak < median*2 {
		t.Errorf("the brightest crease is %.4f against a median of %.4f: the sheet is not folding",
			peak, median)
	}
}

// The curtains have to ripple and fold rather than stand there. A still sky is
// a wallpaper.
func TestTheSkyMoves(t *testing.T) {
	a := New(4)
	a.Resize(tw, th)
	p := canvas.NewSurface(tw, th)
	q := canvas.NewSurface(tw, th)
	for i := 0; i < 30; i++ {
		a.Frame(p, dt)
	}
	for i := 0; i < 90; i++ { // three more seconds
		a.Frame(q, dt)
	}
	var same int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if p.At(x, y) == q.At(x, y) {
				same++
			}
		}
	}
	if same > tw*th*9/10 {
		t.Errorf("%d of %d pixels are unchanged after three seconds", same, tw*th)
	}
}

// The same wall clock must reach the same sky whatever the frame rate: a
// display that ran on frame count would ripple twice as fast on a machine
// drawing twice as often.
func TestTheSkyFollowsTheClockNotTheFrameCount(t *testing.T) {
	run := func(frames int, step float64) *Aurora {
		a := New(5)
		a.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			a.Frame(s, step)
		}
		return a
	}
	slow := run(60, 1.0/30)  // two seconds
	fast := run(120, 1.0/60) // the same two seconds
	if math.Abs(slow.t-fast.t) > 1e-9 {
		t.Fatalf("two seconds reached phase %.6f at 30fps and %.6f at 60fps", slow.t, fast.t)
	}
	if math.Abs(slow.t-2) > 1e-9 {
		t.Errorf("two seconds at Speed 1 advanced the sky by %.6f", slow.t)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() *canvas.Surface {
		a := New(9)
		a.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 50; i++ {
			a.Frame(s, dt)
		}
		return s
	}
	p, q := run(), run()
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if p.At(x, y) != q.At(x, y) {
				t.Fatalf("same seed diverged at %d,%d", x, y)
			}
		}
	}
}

func TestSkyIsNotFullyPainted(t *testing.T) {
	// A curtain sits in a sky. If every pixel comes out lit, the floor is
	// doing nothing and the effect is a wash rather than a shape.
	a := New(6)
	a.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 60; i++ {
		a.Frame(s, dt)
	}
	var lit int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != tcell.ColorDefault {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Fatal("the sky is empty")
	}
	if lit == tw*th {
		t.Fatal("every pixel is lit: this is a wash, not a curtain")
	}
}

func TestOddSizesDoNotPanic(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 9}, {9, 2}, {200, 120}} {
		a := New(1)
		a.Resize(sz[0], sz[1])
		s := canvas.NewSurface(sz[0], sz[1])
		for i := 0; i < 5; i++ {
			a.Frame(s, dt)
		}
	}
	// A Frame before any Resize must be a no-op rather than a nil dereference.
	New(1).Frame(canvas.NewSurface(4, 4), dt)
}

func TestFrameDoesNotAllocate(t *testing.T) {
	a := New(1)
	a.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	a.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { a.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per run; buffers belong in Resize", n)
	}
}

// The ramp itself, which is what the package is a showcase for: green where
// the light is strongest, through cyan, into the violets at the faint end.
// Asserted on the palette rather than on a picture, because it is the claim
// the rest of the coloring rests on.
func TestTheRampRunsGreenThroughCyanIntoViolet(t *testing.T) {
	p := New(1).Palette
	if r, g, b := p[230].RGB(); g <= b || g <= r {
		t.Errorf("the bright end is r=%d g=%d b=%d, not green", r, g, b)
	}
	if r, g, b := p[60].RGB(); b <= g || g <= r {
		t.Errorf("the middle is r=%d g=%d b=%d, not cyan", r, g, b)
	}
	if r, g, b := p[20].RGB(); b <= g || r <= g {
		t.Errorf("the faint end is r=%d g=%d b=%d, not violet", r, g, b)
	}
}

// And the composition of the two: because brightness falls with height and the
// ramp runs green to violet, a curtain must be green at its border and cool
// above it — and having gone cool it must not come back, or the color is not
// tracking height at all.
//
// One curtain, because two of them overlapping in a column is a sum of two
// heights and the brightness there is no longer a statement about either.
func TestColorClimbsFromGreenIntoVioletUpACurtain(t *testing.T) {
	a := single(1)
	s := canvas.NewSurface(tw, th)
	a.Frame(s, dt)

	// The column with the brightest border: the one carrying a fold, which is
	// where the curtain is strong enough to have a green base at all.
	best, bestX := float32(-1), -1
	for x := 0; x < tw; x++ {
		b := border(a, x)
		if b < 30 {
			continue
		}
		if v := a.light[b*tw+x]; v > best {
			best, bestX = v, x
		}
	}
	if bestX < 0 {
		t.Fatal("no column has a curtain tall enough to walk up")
	}

	b := border(a, bestX)
	if _, g, bl := s.At(bestX, b).RGB(); g <= bl {
		t.Errorf("the brightest border pixel is g=%d b=%d, not green", g, bl)
	}

	cool := -1
	for y := b; y >= 0; y-- {
		c := s.At(bestX, y)
		if c == tcell.ColorDefault {
			continue
		}
		_, g, bl := c.RGB()
		switch {
		case bl > g && cool < 0:
			cool = y
		case bl <= g && cool >= 0:
			t.Fatalf("column %d is green again at row %d after turning cool at row %d", bestX, y, cool)
		}
	}
	if cool < 0 {
		t.Errorf("column %d never leaves the green: the color is not following the height", bestX)
	}
}
