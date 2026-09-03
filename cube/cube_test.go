package cube

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 78, 46

// dt is the step the assertions below are calibrated to: a thirtieth of a
// second is what a frame used to be worth, so "N frames" still moves the pose
// as far as it always did.
const dt = 1.0 / 30

func render(seed int64, frames int) (*Cube, *canvas.Surface) {
	c := New(seed)
	c.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < frames; i++ {
		c.Frame(s, dt)
	}
	return c, s
}

// litPixels returns how many pixels are drawn and their mean luminance.
func litPixels(s *canvas.Surface) (n int, meanLum float64) {
	w, h := s.Size()
	total := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := s.At(x, y)
			if c == tcell.ColorDefault {
				continue
			}
			r, g, b := c.RGB()
			n++
			total += int(r) + int(g) + int(b)
		}
	}
	if n == 0 {
		return 0, 0
	}
	return n, float64(total) / float64(n)
}

func TestDefaultSolidIsACube(t *testing.T) {
	s := NewCube()
	if len(s.Verts) != 8 {
		t.Errorf("got %d corners, want 8", len(s.Verts))
	}
	if len(s.Edges) != 12 {
		t.Errorf("got %d edges, want 12", len(s.Edges))
	}
	// Every corner must be on exactly three edges, or the enumeration has
	// produced something that is not a cube.
	deg := make([]int, len(s.Verts))
	for _, e := range s.Edges {
		deg[e[0]]++
		deg[e[1]]++
	}
	for i, d := range deg {
		if d != 3 {
			t.Errorf("corner %d is on %d edges, want 3", i, d)
		}
	}
}

// TestEveryEdgeDraws renders each edge on its own, so that an edge which never
// reaches the surface cannot hide behind the eleven that do.
func TestEveryEdgeDraws(t *testing.T) {
	full := NewCube()
	for frame := 1; frame <= 30; frame += 7 {
		for i, e := range full.Edges {
			c := New(1)
			// Same corners, so the scale and depth range are unchanged; only
			// this one edge is joined up.
			c.Solid = Solid{Name: "edge", Verts: full.Verts, Edges: [][2]int{e}}
			c.Resize(tw, th)
			s := canvas.NewSurface(tw, th)
			for f := 0; f < frame; f++ {
				c.Frame(s, dt)
			}
			if n, _ := litPixels(s); n == 0 {
				t.Errorf("frame %d: edge %d (%v) drew nothing", frame, i, e)
			}
		}
	}
}

func TestProjectionStaysInBounds(t *testing.T) {
	c := New(1)
	c.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 600; i++ {
		c.Frame(s, dt)
		for v := range c.px {
			if c.px[v] < 0 || c.px[v] >= tw || c.py[v] < 0 || c.py[v] >= th {
				t.Fatalf("frame %d: corner %d projected to %.1f,%.1f, outside %dx%d",
					i, v, c.px[v], c.py[v], tw, th)
			}
		}
	}
}

func TestRotationChangesTheImage(t *testing.T) {
	c := New(1)
	c.Resize(tw, th)
	a := canvas.NewSurface(tw, th)
	b := canvas.NewSurface(tw, th)
	c.Frame(a, dt)
	c.Frame(b, dt)
	diff := 0
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) != b.At(x, y) {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Error("consecutive frames are identical: the solid is not turning")
	}
}

// TestNearEdgesAreBrighterThanFarOnes draws each edge alone so that the
// comparison is not confused by edges painting over one another.
func TestNearEdgesAreBrighterThanFarOnes(t *testing.T) {
	ref, _ := render(1, 12)
	full := ref.Solid

	var nearest, farthest int
	var nearOoz, farOoz float64
	for i, e := range full.Edges {
		mid := (ref.ooz[e[0]] + ref.ooz[e[1]]) / 2
		if i == 0 || mid > nearOoz {
			nearest, nearOoz = i, mid
		}
		if i == 0 || mid < farOoz {
			farthest, farOoz = i, mid
		}
	}
	if nearOoz <= farOoz {
		t.Fatal("all edges are at the same depth; nothing to compare")
	}

	lum := func(edge int) float64 {
		c := New(1)
		c.Solid = Solid{Verts: full.Verts, Edges: [][2]int{full.Edges[edge]}}
		c.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for f := 0; f < 12; f++ {
			c.Frame(s, dt)
		}
		_, m := litPixels(s)
		return m
	}
	near, far := lum(nearest), lum(farthest)
	if near <= far {
		t.Errorf("nearest edge %d shades to %.0f but farthest edge %d shades to %.0f; "+
			"the wireframe has no depth cue", nearest, near, farthest, far)
	}
}

func TestShadeRisesWithNearness(t *testing.T) {
	c, _ := render(1, 1)
	prev := -1
	for i := 0; i <= 20; i++ {
		ooz := c.oozFar + (c.oozNear-c.oozFar)*float64(i)/20
		v := c.shade(ooz)
		if v < prev {
			t.Fatalf("shade fell from %d to %d as the point came nearer", prev, v)
		}
		prev = v
	}
	if c.shade(c.oozFar) >= c.shade(c.oozNear) {
		t.Error("the near and far ends of the depth range shade the same")
	}
	if c.shade(c.oozFar) == 0 {
		t.Error("the farthest edge shades to palette index 0 and will vanish into the background")
	}
}

func TestAnotherSolidCanBeSubstituted(t *testing.T) {
	c := New(1)
	c.Solid = NewTetrahedron()
	c.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	c.Frame(s, dt)
	if n, _ := litPixels(s); n == 0 {
		t.Fatal("the tetrahedron drew nothing")
	}
	for i := 0; i < 200; i++ {
		c.Frame(s, dt)
		for v := range c.px {
			if c.px[v] < 0 || c.px[v] >= tw || c.py[v] < 0 || c.py[v] >= th {
				t.Fatalf("frame %d: tetrahedron corner %d left the surface", i, v)
			}
		}
	}
}

// TestLinesAreUnbroken looks at what actually landed on the surface. A DDA
// whose step along the dominant axis exceeds one pixel comes out dashed, and
// the gaps are invisible in a picture where twelve edges overlap, so each edge
// is drawn on its own and every row or column it spans must be occupied.
func TestLinesAreUnbroken(t *testing.T) {
	full := NewCube()
	for frame := 1; frame <= 40; frame += 3 {
		for i, e := range full.Edges {
			c := New(1)
			c.Solid = Solid{Verts: full.Verts, Edges: [][2]int{e}}
			c.Resize(tw, th)
			s := canvas.NewSurface(tw, th)
			for f := 0; f < frame; f++ {
				c.Frame(s, dt)
			}

			minX, maxX, minY, maxY := tw, -1, th, -1
			for y := 0; y < th; y++ {
				for x := 0; x < tw; x++ {
					if s.At(x, y) == tcell.ColorDefault {
						continue
					}
					if x < minX {
						minX = x
					}
					if x > maxX {
						maxX = x
					}
					if y < minY {
						minY = y
					}
					if y > maxY {
						maxY = y
					}
				}
			}
			if maxX < 0 {
				t.Fatalf("frame %d: edge %d drew nothing", frame, i)
			}

			// Along the longer axis the line touches every step; along the
			// shorter one it may legitimately skip.
			if maxX-minX >= maxY-minY {
				for x := minX; x <= maxX; x++ {
					if !columnUsed(s, x, minY, maxY) {
						t.Fatalf("frame %d: edge %d is dashed, column %d is empty", frame, i, x)
					}
				}
			} else {
				for y := minY; y <= maxY; y++ {
					if !rowUsed(s, y, minX, maxX) {
						t.Fatalf("frame %d: edge %d is dashed, row %d is empty", frame, i, y)
					}
				}
			}
		}
	}
}

func columnUsed(s *canvas.Surface, x, y0, y1 int) bool {
	for y := y0; y <= y1; y++ {
		if s.At(x, y) != tcell.ColorDefault {
			return true
		}
	}
	return false
}

func rowUsed(s *canvas.Surface, y, x0, x1 int) bool {
	for x := x0; x <= x1; x++ {
		if s.At(x, y) != tcell.ColorDefault {
			return true
		}
	}
	return false
}

// TestPoseDependsOnElapsedTimeNotFrameCount is the whole point of taking dt:
// a run at half the frame rate must reach the same attitude in the same number
// of seconds, so a slow terminal drops frames instead of running in slow
// motion.
func TestPoseDependsOnElapsedTimeNotFrameCount(t *testing.T) {
	const seconds = 4.0

	slow := New(2)
	slow.Resize(tw, th)
	ss := canvas.NewSurface(tw, th)
	for i := 0; i < 120; i++ {
		slow.Frame(ss, seconds/120)
	}

	fast := New(2)
	fast.Resize(tw, th)
	fs := canvas.NewSurface(tw, th)
	for i := 0; i < 240; i++ {
		fast.Frame(fs, seconds/240)
	}

	// Exact equality is not on offer: the two runs accumulate a different
	// number of floating point additions. A thousandth of a radian is far below
	// anything the eye could see and far below one frame's worth of turn.
	for i, pair := range [3][2]float64{
		{slow.ax, fast.ax}, {slow.ay, fast.ay}, {slow.az, fast.az},
	} {
		if math.Abs(pair[0]-pair[1]) > 1e-3 {
			t.Errorf("axis %d after %g seconds: 30fps reached %.4f but 60fps reached %.4f",
				i, seconds, pair[0], pair[1])
		}
	}

	// And the rate must actually be per second: four seconds of the slowest
	// axis has to be a visible amount of turn, or the comparison above would
	// pass on a solid that never moved.
	if slow.az < 0.5 {
		t.Errorf("four seconds turned the slowest axis by only %.3f radians; "+
			"the rates are not per second", slow.az)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	_, a := render(5, 30)
	_, b := render(5, 30)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("same seed diverged at %d,%d", x, y)
			}
		}
	}
}

func TestOddSizesAndEmptySolidDoNotPanic(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {2, 9}, {9, 2}, {17, 5}, {200, 120}} {
		c := New(1)
		c.Resize(sz[0], sz[1])
		s := canvas.NewSurface(sz[0], sz[1])
		c.Frame(s, dt)
	}
	// Frame before Resize, and a solid with no corners at all: both must be
	// no-ops rather than nil dereferences.
	New(1).Frame(canvas.NewSurface(4, 4), dt)
	empty := New(1)
	empty.Solid = Solid{}
	empty.Resize(tw, th)
	empty.Frame(canvas.NewSurface(tw, th), dt)
}

func TestFrameDoesNotAllocate(t *testing.T) {
	c := New(1)
	c.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	c.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { c.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per run; buffers belong in Resize", n)
	}
}
