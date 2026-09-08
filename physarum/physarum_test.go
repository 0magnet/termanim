package physarum

import (
	"math"
	"sort"
	"testing"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 96, 48

// dt is one tick of a thirty-a-second loop, so a count of frames here means
// the same amount of crawling it always did.
const dt = 1.0 / 30

// settled runs the colony for roughly the given number of seconds.
func settled(p *Physarum, secs float64) *canvas.Surface {
	s := canvas.NewSurface(tw, th)
	for t := 0.0; t < secs; t += dt {
		p.Frame(s, dt)
	}
	return s
}

// veininess is the fraction of the brightest tenth of the pixels that have at
// least two of their four neighbors in the brightest tenth as well.
//
// This is the question the animation exists to ask. The same quantity of
// chemical dropped by particles that ignored each other lands in isolated
// specks, and a speck has bright neighbors only by coincidence; chemical that
// has been pulled into veins is bright in connected runs, and almost every
// bright pixel has bright company. Total brightness cannot tell the two apart
// and neither can any histogram of it, because both are the same deposits.
func veininess(p *Physarum) float64 {
	vals := make([]float64, len(p.field))
	for i, v := range p.field {
		vals[i] = float64(v)
	}
	sort.Float64s(vals)
	cut := vals[len(vals)*9/10]
	if cut <= 0 {
		return 0
	}
	bright := func(x, y int) bool {
		x = (x + p.w) % p.w
		y = (y + p.h) % p.h
		return float64(p.field[y*p.w+x]) >= cut
	}
	var lit, social int
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			if !bright(x, y) {
				continue
			}
			lit++
			n := 0
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if bright(x+d[0], y+d[1]) {
					n++
				}
			}
			if n >= 2 {
				social++
			}
		}
	}
	if lit == 0 {
		return 0
	}
	return float64(social) / float64(lit)
}

func TestTrailsConcentrateIntoANetwork(t *testing.T) {
	// Particles that only wandered would leave their deposits scattered. The
	// sensor rule pulls them onto each other's trails, so the chemical ends up
	// in connected veins instead of specks. Measured against the same colony
	// one step in, when the deposits are exactly the scattered case.
	p := New(1)
	p.Resize(tw, th)
	p.step()
	scattered := veininess(p)
	if scattered > 0.25 {
		t.Fatalf("one step in, %.2f of the bright pixels already have bright "+
			"neighbors: the control is not a control", scattered)
	}
	settled(p, 25)
	veins := veininess(p)
	if veins < 0.75 {
		t.Errorf("after 25s only %.2f of the bright pixels have bright neighbors "+
			"(scattered start: %.2f): the chemical is still in specks, "+
			"there is no network", veins, scattered)
	}
}

func TestTurnsTowardTheStrongerSensor(t *testing.T) {
	// The three-line rule, tested directly: one particle, a hand-painted
	// gradient, and nothing else on the surface.
	//
	// Headings are in screen coordinates, where y increases downward, so a
	// heading of 0 faces right and a negative turn swings toward the top of
	// the screen — the left sensor.
	paint := func(p *Physarum, ang float64) {
		a := &p.agents[0]
		x := int(wrapf(a.x+math.Cos(a.dir+ang)*p.SensorDist, p.fw))
		y := int(wrapf(a.y+math.Sin(a.dir+ang)*p.SensorDist, p.fh))
		p.field[y*p.w+x] = 100
	}
	for _, tc := range []struct {
		name string
		at   float64
		want float64
	}{
		{"left", -22.5 * math.Pi / 180, -1},
		{"right", +22.5 * math.Pi / 180, +1},
	} {
		p := New(1)
		p.Density = 1
		p.Diffuse = 0 // keep the painted cell exactly where it was put
		p.Resize(tw, th)
		p.agents = p.agents[:1]
		p.agents[0] = agent{x: 48, y: 24, dir: 0}
		for i := range p.field {
			p.field[i] = 0
		}
		paint(p, tc.at)
		before := p.agents[0].dir
		p.step()
		turn := wrapAngle(p.agents[0].dir-before+math.Pi) - math.Pi
		if turn*tc.want <= 0 {
			t.Errorf("%s sensor was the strongest and the particle turned %+.3f rad",
				tc.name, turn)
		}
		if math.Abs(math.Abs(turn)-p.TurnAngle) > 1e-9 {
			t.Errorf("%s: turned %.4f rad, want the whole TurnAngle of %.4f",
				tc.name, math.Abs(turn), p.TurnAngle)
		}
	}
}

func TestStraightAheadHoldsTheHeading(t *testing.T) {
	// The other half of the rule, and the half that makes a trail worth
	// following: a particle already on one is not steered off it.
	p := New(1)
	p.Density = 1
	p.Diffuse = 0
	p.Resize(tw, th)
	p.agents = p.agents[:1]
	p.agents[0] = agent{x: 48, y: 24, dir: 0}
	for i := range p.field {
		p.field[i] = 0
	}
	p.field[24*p.w+int(48+p.SensorDist)] = 100
	p.step()
	if got := p.agents[0].dir; got != 0 {
		t.Errorf("heading moved to %.4f with the center sensor winning", got)
	}
}

func TestAtMostOneParticlePerPixel(t *testing.T) {
	// Exclusion is what stops the population collapsing into a single wave.
	// It has to hold for every step, not just at the start, and the occupancy
	// grid has to stay in step with where the particles actually are.
	p := New(2)
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < 300; i++ {
		p.Frame(s, dt)
	}
	seen := make(map[int]bool, len(p.agents))
	for j, a := range p.agents {
		c := int(a.y)*p.w + int(a.x)
		if seen[c] {
			t.Fatalf("particle %d shares pixel %d with another", j, c)
		}
		seen[c] = true
		if !p.taken[c] {
			t.Fatalf("particle %d sits on pixel %d, which the grid calls free", j, c)
		}
	}
	var marked int
	for _, v := range p.taken {
		if v {
			marked++
		}
	}
	if marked != len(p.agents) {
		t.Errorf("%d pixels marked taken for %d particles: the grid leaks", marked, len(p.agents))
	}
}

func TestAbandonedTrailsEvaporate(t *testing.T) {
	// Decay is the only thing keeping the picture from filling in. Stop the
	// deposits and what is there has to go away.
	p := New(3)
	p.Resize(tw, th)
	settled(p, 10)
	var before float64
	for _, v := range p.field {
		before += float64(v)
	}
	p.Deposit = 0
	settled(p, 5)
	var after float64
	for _, v := range p.field {
		after += float64(v)
	}
	if before == 0 {
		t.Fatal("nothing was deposited in ten seconds")
	}
	if after > before*0.05 {
		t.Errorf("field fell only from %.0f to %.0f with the deposits off", before, after)
	}
}

func TestStepRateIsFrameRateIndependent(t *testing.T) {
	// Two seconds of wall clock at three frame rates. The colony crawls at
	// StepsPerSecond in all of them, so the network is the same age however
	// often it was drawn.
	stepsOver := func(frames int, step float64) int {
		p := New(4)
		p.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			p.Frame(s, step)
		}
		return p.steps
	}
	slow := stepsOver(60, 1.0/30)
	fast := stepsOver(120, 1.0/60)
	odd := stepsOver(34, 1.0/17)
	if slow < 118 || slow > 122 {
		t.Fatalf("two seconds at 30fps ran %d steps, want about 120", slow)
	}
	for _, got := range []int{fast, odd} {
		if d := got - slow; d < -2 || d > 2 {
			t.Fatalf("same two seconds ran %d steps at one frame rate and %d at another", slow, got)
		}
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) []agent {
		p := New(seed)
		p.Resize(tw, th)
		settled(p, 3)
		out := make([]agent, len(p.agents))
		copy(out, p.agents)
		return out
	}
	a, b := run(0), run(0)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("same seed diverged at particle %d: %+v vs %+v", i, a[i], b[i])
		}
	}
	c := run(5)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced an identical colony")
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	// Resize can be handed a degenerate size before the first frame, and the
	// population cap has to leave a free pixel for the placement loop to find
	// or it spins forever.
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {2, 2}, {5, 3}} {
		p := New(6)
		p.Resize(sz[0], sz[1])
		s := canvas.NewSurface(max(sz[0], 1), max(sz[1], 1))
		for i := 0; i < 20; i++ {
			p.Frame(s, dt)
		}
	}
}

func TestFrameWritesToTheSurface(t *testing.T) {
	p := New(7)
	p.Resize(tw, th)
	s := settled(p, 2)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			if s.At(x, y) != 0 {
				return
			}
		}
	}
	t.Error("Frame left the surface empty")
}

func TestFrameDoesNotAllocate(t *testing.T) {
	// Thirty frames a second in a browser: the field, its blur buffer and the
	// occupancy grid are all sized in Resize precisely so that Frame never has
	// to ask the allocator for anything.
	p := New(8)
	p.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	p.Frame(s, dt)
	if n := testing.AllocsPerRun(20, func() { p.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}
