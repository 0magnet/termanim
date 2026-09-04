package maze

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

// A surface twice as tall as a terminal of 60 by 20, which is what canvas.Run
// hands an animation.
const pw, ph = 61, 40

// dt is the elapsed time handed to Frame in these tests: a thirtieth of a
// second, the rate the animation used to be tied to.
const dt = 1.0 / 30

func newMaze(t *testing.T, seed int64) (*Maze, *canvas.Surface) {
	t.Helper()
	m := New(seed)
	m.Resize(pw, ph)
	if m.cw < 4 || m.ch < 4 {
		t.Fatalf("a %dx%d surface only made a %dx%d maze", pw, ph, m.cw, m.ch)
	}
	return m, canvas.NewSurface(pw, ph)
}

// run frames the maze until it reaches the wanted phase, or fails.
func run(t *testing.T, m *Maze, s *canvas.Surface, want phase) {
	t.Helper()
	for i := 0; i < 20000; i++ {
		if m.phase == want {
			return
		}
		m.Frame(s, dt)
	}
	t.Fatalf("the maze never reached phase %d, it is in phase %d", want, m.phase)
}

func TestGenerationVisitsEveryCellExactlyOnce(t *testing.T) {
	m, s := newMaze(t, 1)
	run(t, m, s, solving)
	cells := m.cw * m.ch
	if m.carved != cells {
		t.Errorf("carved %d cells of %d: some were dug twice or not at all", m.carved, cells)
	}
	for c, st := range m.state {
		if st == unvisited {
			t.Fatalf("cell %d,%d was never visited", c%m.cw, c/m.cw)
		}
	}
}

func TestWallsComeDownOnBothSides(t *testing.T) {
	// A door recorded in one cell and not the other is a wall you can walk
	// through in one direction only.
	m, s := newMaze(t, 2)
	run(t, m, s, solving)
	for c := range m.open {
		for d := 0; d < 4; d++ {
			if m.open[c]&(1<<uint(d)) == 0 {
				continue
			}
			nb, ok := m.neighbor(c, d)
			if !ok {
				t.Fatalf("cell %d has a door in direction %d leading off the grid", c, d)
			}
			if m.open[nb]&(1<<uint(back(d))) == 0 {
				t.Fatalf("cells %d and %d disagree about the wall between them", c, nb)
			}
		}
	}
}

// reach flood fills through open walls and returns how many cells it found.
func reach(m *Maze) int {
	seen := make([]bool, m.cw*m.ch)
	seen[0] = true
	stack := []int{0}
	n := 1
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for d := 0; d < 4; d++ {
			if m.open[c]&(1<<uint(d)) == 0 {
				continue
			}
			nb, ok := m.neighbor(c, d)
			if !ok || seen[nb] {
				continue
			}
			seen[nb] = true
			n++
			stack = append(stack, nb)
		}
	}
	return n
}

func TestEveryCellIsReachable(t *testing.T) {
	// A perfect maze is one region: no cell walled off from the rest.
	for seed := int64(0); seed < 8; seed++ {
		m, s := newMaze(t, seed)
		run(t, m, s, solving)
		if got, want := reach(m), m.cw*m.ch; got != want {
			t.Errorf("seed %d: only %d of %d cells can be walked to", seed, got, want)
		}
	}
}

func TestNoLoops(t *testing.T) {
	// Recursive backtracking never joins two cells that are already connected,
	// so the doors number one fewer than the cells: a spanning tree.
	m, s := newMaze(t, 3)
	run(t, m, s, solving)
	doors := 0
	for _, o := range m.open {
		for d := 0; d < 4; d++ {
			if o&(1<<uint(d)) != 0 {
				doors++
			}
		}
	}
	doors /= 2 // each door is recorded in both cells
	if want := m.cw*m.ch - 1; doors != want {
		t.Errorf("%d doors for %d cells, want %d: the maze has loops or islands", doors, m.cw*m.ch, want)
	}
}

func TestSolverFindsAPath(t *testing.T) {
	m, s := newMaze(t, 4)
	run(t, m, s, tracing)
	if len(m.path) == 0 {
		t.Fatal("the solver found no route at all")
	}
	if m.path[0] != 0 {
		t.Errorf("the route starts at cell %d, not the top left", m.path[0])
	}
	if goal := m.cw*m.ch - 1; m.path[len(m.path)-1] != goal {
		t.Errorf("the route ends at cell %d, not the bottom right %d", m.path[len(m.path)-1], goal)
	}
}

func TestPathIsContiguous(t *testing.T) {
	// Every consecutive pair must be neighbors with the wall between them
	// down; otherwise the drawn route jumps through a wall.
	for seed := int64(10); seed < 16; seed++ {
		m, s := newMaze(t, seed)
		run(t, m, s, tracing)
		for i := 0; i+1 < len(m.path); i++ {
			a, b := m.path[i], m.path[i+1]
			joined := false
			for d := 0; d < 4; d++ {
				if m.open[a]&(1<<uint(d)) == 0 {
					continue
				}
				if nb, ok := m.neighbor(a, d); ok && nb == b {
					joined = true
					break
				}
			}
			if !joined {
				t.Fatalf("seed %d: the route steps from %d,%d to %d,%d without a door",
					seed, a%m.cw, a/m.cw, b%m.cw, b/m.cw)
			}
		}
	}
}

func TestPathIsTheShortestRoute(t *testing.T) {
	// Breadth first should not return a wandering route. In a perfect maze
	// there is exactly one simple route, so the length is forced: check it
	// against an independent search.
	m, s := newMaze(t, 5)
	run(t, m, s, tracing)
	dist := make([]int, m.cw*m.ch)
	for i := range dist {
		dist[i] = -1
	}
	dist[0] = 0
	queue := []int{0}
	for i := 0; i < len(queue); i++ {
		c := queue[i]
		for d := 0; d < 4; d++ {
			if m.open[c]&(1<<uint(d)) == 0 {
				continue
			}
			nb, ok := m.neighbor(c, d)
			if !ok || dist[nb] >= 0 {
				continue
			}
			dist[nb] = dist[c] + 1
			queue = append(queue, nb)
		}
	}
	goal := m.cw*m.ch - 1
	if want := dist[goal] + 1; len(m.path) != want {
		t.Errorf("the route is %d cells long, want %d", len(m.path), want)
	}
}

func TestTheRouteIsDrawnGradually(t *testing.T) {
	m, s := newMaze(t, 6)
	run(t, m, s, tracing)
	seen := 0
	frames := 0
	for m.phase == tracing {
		m.Frame(s, dt)
		frames++
		n := 0
		for _, st := range m.state {
			if st == onPath {
				n++
			}
		}
		if n < seen {
			t.Fatal("the route un-drew itself")
		}
		seen = n
	}
	if frames < 2 {
		t.Errorf("the whole route appeared in %d frame(s): it is not animated", frames)
	}
	if seen != len(m.path) {
		t.Errorf("%d cells were lit but the route is %d long", seen, len(m.path))
	}
}

func TestFrameRateDoesNotChangeTheSpeed(t *testing.T) {
	// The same elapsed time must carve the same amount of maze. Same seed, so
	// equal step counts mean identical mazes.
	carve := func(step float64, frames int) *Maze {
		m, s := newMaze(t, 13)
		for i := 0; i < frames; i++ {
			m.Frame(s, step)
		}
		return m
	}
	slow := carve(1.0/30, 60) // two seconds either way
	fast := carve(1.0/60, 120)
	// Within one step, which carves at most one cell: neither frame length is
	// exact in binary, so the accumulators can land a step apart. A rate tied
	// to the frame rate would be out by a factor of two.
	if d := slow.carved - fast.carved; d < -1 || d > 1 {
		t.Errorf("30fps carved %d cells and 60fps carved %d: the speed follows the frame rate",
			slow.carved, fast.carved)
	}
	if slow.phase != fast.phase {
		t.Errorf("after the same two seconds one is in phase %d and the other in phase %d",
			slow.phase, fast.phase)
	}
	// Two seconds of a five second carve should be well under way and not done.
	if slow.carved < 2 || slow.phase != generating {
		t.Errorf("after two seconds of a %g second carve: %d cells, phase %d",
			slow.GenSeconds, slow.carved, slow.phase)
	}
}

func TestMazeRegenerates(t *testing.T) {
	m, s := newMaze(t, 7)
	m.HoldSeconds = 0.1
	first := m.mazes
	run(t, m, s, holding)
	for i := 0; i < 10 && m.mazes == first; i++ {
		m.Frame(s, dt)
	}
	if m.mazes == first {
		t.Fatal("the solved maze was never replaced")
	}
	if m.phase != generating {
		t.Errorf("after replacing, the maze is in phase %d rather than generating", m.phase)
	}
	if m.carved != 1 {
		t.Errorf("the new maze started with %d cells already carved", m.carved)
	}
}

func TestDrawnWallsAndCorridors(t *testing.T) {
	m, s := newMaze(t, 8)
	run(t, m, s, holding)
	m.Frame(s, dt)

	// The grid is enclosed: its outermost pixel ring is all wall.
	x0, y0 := m.ox, m.oy
	x1, y1 := m.ox+m.cw*pitch, m.oy+m.ch*pitch
	for x := x0; x <= x1; x++ {
		if s.At(x, y0) != m.Wall || s.At(x, y1) != m.Wall {
			t.Fatalf("the maze is open at the top or bottom, column %d", x)
		}
	}
	for y := y0; y <= y1; y++ {
		if s.At(x0, y) != m.Wall || s.At(x1, y) != m.Wall {
			t.Fatalf("the maze is open at the left or right, row %d", y)
		}
	}

	// Every cell's two by two interior is carved, and the route is visible.
	var path int
	for cy := 0; cy < m.ch; cy++ {
		for cx := 0; cx < m.cw; cx++ {
			px, py := m.ox+1+cx*pitch, m.oy+1+cy*pitch
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					c := s.At(px+dx, py+dy)
					if c == m.Wall {
						t.Fatalf("cell %d,%d is still solid rock", cx, cy)
					}
					if c == m.Path {
						path++
					}
				}
			}
		}
	}
	if path < 4*len(m.path)/2 {
		t.Errorf("only %d pixels of route are drawn for a %d cell route", path, len(m.path))
	}
}

func TestDoorwaysJoinTheCellsTheyOpen(t *testing.T) {
	// The pixel between two cells with a door must be carved, and the pixel
	// between two cells without one must not be, or the drawing disagrees with
	// the maze.
	m, s := newMaze(t, 9)
	run(t, m, s, holding)
	m.Frame(s, dt)
	for cy := 0; cy < m.ch; cy++ {
		for cx := 0; cx+1 < m.cw; cx++ {
			c := m.cell(cx, cy)
			x := m.ox + 1 + cx*pitch + 2
			y := m.oy + 1 + cy*pitch
			open := m.open[c]&(1<<east) != 0
			carvedHere := s.At(x, y) != m.Wall
			if open != carvedHere {
				t.Fatalf("east of cell %d,%d: door=%v but the pixel is carved=%v", cx, cy, open, carvedHere)
			}
		}
	}
}

func TestTinySurfaceIsHarmless(t *testing.T) {
	m := New(1)
	m.Resize(3, 3)
	s := canvas.NewSurface(3, 3)
	for i := 0; i < 50; i++ {
		m.Frame(s, dt)
	}
	if s.At(0, 0) != tcell.ColorDefault {
		t.Error("a surface too small for a maze should be left blank")
	}
}
