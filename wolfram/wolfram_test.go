package wolfram

import (
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/canvas"
)

const tw, th = 128, 64

// dt is one tick of a thirty-a-second loop.
const dt = 1.0 / 30

// only returns an automaton pinned to a single rule that never cycles, so a
// test can count rows without a reseed landing in the middle of them.
func only(rule uint8, random bool) *Wolfram {
	a := New(1)
	a.Rules = []RuleSpec{{Number: rule, Random: random}}
	a.RowsPerRule = 1 << 30
	a.Resize(tw, th)
	return a
}

// row returns the automaton's state age rows before the newest one.
func row(a *Wolfram, age int) []byte {
	r := (a.head - age + a.h*2) % a.h
	return a.cells[r*a.w : (r+1)*a.w]
}

func TestRule90DrawsSierpinski(t *testing.T) {
	// Rule 90 is the exclusive or of the two neighbors, so from a single lit
	// cell row n is row n of Pascal's triangle reduced mod 2 — the Sierpinski
	// triangle. Kummer's theorem gives the parity without computing the
	// binomial: C(n,k) is odd exactly when k has no bit that n-k also has.
	a := only(90, false)
	c := tw / 2
	for n := 1; n < tw/2-1; n++ {
		a.step()
		got := row(a, 0)
		for x := 0; x < tw; x++ {
			j := x - c
			var want byte
			if d := j; d <= n && -d <= n && (n-j)%2 == 0 {
				k := (n + j) / 2
				if k&(n-k) == 0 {
					want = 1
				}
			}
			if got[x] != want {
				t.Fatalf("row %d cell %+d is %d, want %d: this is not Sierpinski",
					n, j, got[x], want)
			}
		}
	}
}

func TestTheRuleNumberIsTheTable(t *testing.T) {
	// The definition, checked against itself for all eight neighborhoods of
	// several rules: a cell whose left, center and right read as the binary
	// number n becomes bit n of the rule.
	for _, rule := range []uint8{0, 30, 90, 110, 150, 255} {
		for n := 0; n < 8; n++ {
			a := only(rule, false)
			cur := row(a, 0)
			for i := range cur {
				cur[i] = 0
			}
			cur[1] = byte(n>>2) & 1
			cur[2] = byte(n>>1) & 1
			cur[3] = byte(n) & 1
			a.step()
			want := (rule >> uint(n)) & 1
			if got := row(a, 0)[2]; got != want {
				t.Errorf("rule %d, neighborhood %03b: got %d, want bit %d of the rule (%d)",
					rule, n, got, n, want)
			}
		}
	}
}

func TestRule110FromNoiseKeepsGoing(t *testing.T) {
	// The rule the whole package is arguably for. A rule that dies out or
	// saturates has nothing to compute with; 110 has to stay somewhere in
	// between forever, which is where the gliders live.
	a := only(110, true)
	for n := 0; n < 2000; n++ {
		a.step()
		var live int
		for _, c := range row(a, 0) {
			live += int(c)
		}
		if live == 0 {
			t.Fatalf("rule 110 died out at row %d", n)
		}
		if live == tw {
			t.Fatalf("rule 110 saturated at row %d", n)
		}
	}
}

func TestDeadAndFullRulesDoWhatTheyMustNot(t *testing.T) {
	// The two degenerate ends, as a check that the table is read the right way
	// round: rule 0 answers dead to everything and rule 255 answers alive.
	dead := only(0, true)
	dead.step()
	for x, c := range row(dead, 0) {
		if c != 0 {
			t.Fatalf("rule 0 left cell %d alive", x)
		}
	}
	full := only(255, false)
	full.step()
	for x, c := range row(full, 0) {
		if c != 1 {
			t.Fatalf("rule 255 left cell %d dead", x)
		}
	}
}

func TestNewRowsArriveAtTheBottomAndScrollOff(t *testing.T) {
	// Time runs up the screen. The newest row must be the bottom one, and a
	// row must leave the top after exactly a screenful.
	a := only(255, false)
	s := canvas.NewSurface(tw, th)
	// The seed row is one lit cell in the middle, and rule 255 fills
	// everything from the row after it, so the seed row is identifiable for
	// as long as it is on the surface.
	isSeed := func(r []byte) bool {
		for x, c := range r {
			if (c == 1) != (x == tw/2) {
				return false
			}
		}
		return true
	}
	if !isSeed(row(a, 0)) {
		t.Fatal("the seed row is not the newest row before any step")
	}
	a.Frame(s, 0)
	if lit := s.At(tw/2, th-1); lit == tcell.ColorDefault {
		t.Error("the newest row was not drawn along the bottom edge")
	}
	for n := 1; n < th; n++ {
		a.step()
		if !isSeed(row(a, n)) {
			t.Fatalf("after %d rows the seed is no longer %d rows back", n, n)
		}
	}
	a.step()
	for age := 0; age < th; age++ {
		if isSeed(row(a, age)) {
			t.Fatalf("the seed row is still on a %d-row surface after %d rows", th, th+1)
		}
	}
}

func TestOlderRowsAreDimmerAndKeepTheirRuleColor(t *testing.T) {
	lum := func(c tcell.Color) int {
		if c == tcell.ColorDefault {
			return 0
		}
		r, g, b := c.RGB()
		return int(r + g + b)
	}
	// Rule 255 fills the surface, so every pixel is lit and only the age
	// shading is left to compare.
	a := only(255, false)
	s := canvas.NewSurface(tw, th)
	for i := 0; i < th*2; i++ {
		a.step()
	}
	a.Frame(s, 0)
	if newest, oldest := lum(s.At(0, th-1)), lum(s.At(0, 0)); newest <= oldest {
		t.Errorf("bottom row is %d and top row %d: history is not fading", newest, oldest)
	}

	// Two rules running one after the other: rows made by the first keep the
	// first rule's color after the second has taken over.
	b := New(2)
	b.Rules = []RuleSpec{{Number: 255}, {Number: 255}}
	b.RowsPerRule = 8
	b.Resize(tw, th)
	for i := 0; i < 12; i++ {
		b.step()
	}
	if b.specIdx != 1 {
		t.Fatalf("still on spec %d after 12 rows with 8 rows a rule", b.specIdx)
	}
	if b.pal[b.head] == b.pal[(b.head-6+b.h)%b.h] {
		t.Error("rows from either side of a rule change share a color")
	}
}

func TestRowRateIsFrameRateIndependent(t *testing.T) {
	// Two seconds of wall clock at three frame rates. The picture scrolls at
	// RowsPerSecond in all of them.
	rowsOver := func(frames int, step float64) int {
		a := only(30, false)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < frames; i++ {
			a.Frame(s, step)
		}
		return a.rows
	}
	slow := rowsOver(60, 1.0/30)
	fast := rowsOver(120, 1.0/60)
	odd := rowsOver(34, 1.0/17)
	if slow < 50 || slow > 54 {
		t.Fatalf("two seconds at 30fps emitted %d rows, want about 52", slow)
	}
	for _, got := range []int{fast, odd} {
		if d := got - slow; d < -2 || d > 2 {
			t.Fatalf("same two seconds emitted %d rows at one frame rate and %d at another", slow, got)
		}
	}
}

func TestCyclesThroughTheRules(t *testing.T) {
	a := New(3)
	a.RowsPerRule = 5
	a.Resize(tw, th)
	seen := map[uint8]bool{a.Rule: true}
	for i := 0; i < 5*len(a.Rules); i++ {
		a.step()
		seen[a.Rule] = true
	}
	for _, spec := range a.Rules {
		if !seen[spec.Number] {
			t.Errorf("rule %d (%s) never ran", spec.Number, spec.Name)
		}
	}
	// A whole turn of the cycle brings the first rule back round.
	if a.Rule != Interesting[0].Number {
		t.Errorf("a full turn of the cycle ended on rule %d rather than wrapping to %d",
			a.Rule, Interesting[0].Number)
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) []byte {
		a := New(seed)
		a.RowsPerRule = 20
		a.Resize(tw, th)
		s := canvas.NewSurface(tw, th)
		for i := 0; i < 300; i++ {
			a.Frame(s, dt)
		}
		out := make([]byte, len(a.cells))
		copy(out, a.cells)
		return out
	}
	x, y := run(0), run(0)
	for i := range x {
		if x[i] != y[i] {
			t.Fatalf("same seed diverged at index %d", i)
		}
	}
	// Only the random-start rules depend on the seed, but the cycle reaches
	// them, so two seeds must not give the same picture.
	z := run(11)
	same := true
	for i := range x {
		if x[i] != z[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds produced an identical picture")
	}
}

func TestSurvivesATinyWindow(t *testing.T) {
	// A one-row ring would have the row being read and the row being written
	// share memory, and Resize can be handed a degenerate size.
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {2, 2}, {4, 3}} {
		a := New(4)
		a.Resize(sz[0], sz[1])
		s := canvas.NewSurface(max(sz[0], 1), max(sz[1], 1))
		for i := 0; i < 20; i++ {
			a.Frame(s, dt)
		}
	}
}

func TestFrameDoesNotAllocate(t *testing.T) {
	// The ring is why: scrolling by moving an index rather than copying rows
	// is what keeps this free of the allocator.
	a := New(5)
	a.Resize(tw, th)
	s := canvas.NewSurface(tw, th)
	a.Frame(s, dt)
	if n := testing.AllocsPerRun(50, func() { a.Frame(s, dt) }); n != 0 {
		t.Errorf("Frame allocated %v times per call", n)
	}
}
