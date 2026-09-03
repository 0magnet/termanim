package matrix

import (
	"strings"
	"testing"
)

// collect returns the lit glyphs of each column, top to bottom.
func collect(m *Matrix) map[int][]rune {
	cols := map[int][]rune{}
	m.Cells(func(x, _ int, c Cell) {
		if c.Rune != 0 && c.Rune != ' ' {
			cols[x] = append(cols[x], c.Rune)
		}
	})
	return cols
}

// TestWordsDrawOnlyTheWord: a word stream must never draw a glyph from
// the alphabet, or the word is not what is falling.
func TestWordsDrawOnlyTheWord(t *testing.T) {
	m := New(3)
	m.Words = []string{"serendipity"}
	m.Glyphs = []rune("!@#$%^&*") // deliberately disjoint from the word
	m.Resize(40, 24)
	m.Advance(200)

	inWord := map[rune]bool{}
	for _, r := range "serendipity" {
		inWord[r] = true
	}
	seen, stray := 0, 0
	for _, col := range collect(m) {
		for _, r := range col {
			seen++
			if !inWord[r] {
				stray++
			}
		}
	}
	if seen == 0 {
		t.Fatal("no glyphs drawn at all")
	}
	if stray > 0 {
		t.Errorf("%d of %d glyphs came from the glyph alphabet, want 0", stray, seen)
	}
}

// TestWordsReadDownward: consecutive lit cells in a column advance
// through the word by one, wrapping. That is what makes it read as a
// word falling rather than as its letters shuffled.
func TestWordsReadDownward(t *testing.T) {
	const word = "abcdefgh"
	m := New(11)
	m.Words = []string{word}
	m.Resize(8, 30)
	m.Advance(60)

	checked := 0
	for x, col := range collect(m) {
		if len(col) < 4 {
			continue
		}
		for i := 1; i < len(col); i++ {
			prev := strings.IndexRune(word, col[i-1])
			cur := strings.IndexRune(word, col[i])
			if prev < 0 || cur < 0 {
				t.Fatalf("column %d has a glyph outside the word: %q", x, col[i])
			}
			if cur != (prev+1)%len(word) {
				t.Errorf("column %d reads %q: %q does not follow %q", x, string(col), col[i], col[i-1])
				break
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no column had enough glyphs to check")
	}
}

// TestWordsEmptyIsUnchanged: the default must still be the glyph rain,
// cell for cell.
func TestWordsEmptyIsUnchanged(t *testing.T) {
	a, b := New(5), New(5)
	a.Resize(30, 20)
	b.Words = nil
	b.Resize(30, 20)
	a.Advance(100)
	b.Advance(100)

	ca, cb := map[[2]int]Cell{}, map[[2]int]Cell{}
	a.Cells(func(x, y int, c Cell) { ca[[2]int{x, y}] = c })
	b.Cells(func(x, y int, c Cell) { cb[[2]int{x, y}] = c })
	if len(ca) != len(cb) {
		t.Fatalf("nil Words changed the number of lit cells: %d vs %d", len(ca), len(cb))
	}
	for k, v := range ca {
		if cb[k] != v {
			t.Fatalf("nil Words changed the rain at %v", k)
		}
	}
}

// TestWordGapSeparatesRepeats: with a gap set, a column must read as the
// word, blanks, the word again — not as the letters run together.
func TestWordGapSeparatesRepeats(t *testing.T) {
	const word = "abcdef"
	m := New(4)
	m.Words = []string{word}
	m.WordGap = 3
	m.Resize(6, 40)
	m.Advance(120)

	checked := 0
	m2 := map[int][]rune{}
	m.Cells(func(x, _ int, c Cell) { m2[x] = append(m2[x], c.Rune) })
	for x, col := range m2 {
		if len(col) < 10 {
			continue
		}
		s := string(col)
		// Every run of letters between blanks must be a contiguous piece of
		// the word, and no run may be longer than the word itself.
		for _, run := range strings.Split(s, " ") {
			if run == "" {
				continue
			}
			if len(run) > len(word) {
				t.Errorf("column %d has a %d-letter run %q, longer than the word", x, len(run), run)
			}
			if !strings.Contains(word+word, run) {
				t.Errorf("column %d run %q is not a piece of the word", x, run)
			}
		}
		if !strings.Contains(s, " ") {
			t.Errorf("column %d has no gap at all: %q", x, s)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no column long enough to check")
	}
}

// TestWordGapZeroIsUnchanged: the default must be the run-together form,
// so adding the field changed nothing for callers that do not set it.
func TestWordGapZeroIsUnchanged(t *testing.T) {
	mk := func(gap int) map[[2]int]Cell {
		m := New(9)
		m.Words = []string{"abcdef"}
		m.WordGap = gap
		m.Resize(20, 20)
		m.Advance(80)
		out := map[[2]int]Cell{}
		m.Cells(func(x, y int, c Cell) { out[[2]int{x, y}] = c })
		return out
	}
	a, b := mk(0), mk(0)
	if len(a) != len(b) {
		t.Fatal("not deterministic")
	}
	for k, v := range a {
		if b[k] != v {
			t.Fatalf("not deterministic at %v", k)
		}
	}
}

// TestFreshWordsDoesNotRepeat: a long column with FreshWords set must
// show more than one distinct word, where the default repeats one.
func TestFreshWordsDoesNotRepeat(t *testing.T) {
	words := []string{"alpha", "bravo", "delta", "gamma", "sigma", "omega", "kappa"}

	distinct := func(fresh bool) int {
		m := New(6)
		m.Words = words
		m.WordGap = 1
		m.FreshWords = fresh
		m.Resize(4, 60)
		m.Advance(300)
		cols := map[int][]rune{}
		m.Cells(func(x, _ int, c Cell) { cols[x] = append(cols[x], c.Rune) })
		best := 0
		for _, col := range cols {
			seen := map[string]bool{}
			for _, run := range strings.Split(string(col), " ") {
				for _, w := range words {
					if run != "" && strings.Contains(w, run) && len(run) >= 3 {
						seen[w] = true
					}
				}
			}
			if len(seen) > best {
				best = len(seen)
			}
		}
		return best
	}

	if got := distinct(true); got < 2 {
		t.Errorf("FreshWords: the busiest column showed %d distinct words, want at least 2", got)
	}
	if got := distinct(false); got != 1 {
		t.Errorf("default: the busiest column showed %d distinct words, want exactly 1", got)
	}
}

// TestFreshWordsOffIsUnchanged pins the zero value.
func TestFreshWordsOffIsUnchanged(t *testing.T) {
	mk := func(fresh bool) map[[2]int]Cell {
		m := New(13)
		m.Words = []string{"abcdef", "ghijkl"}
		m.FreshWords = fresh
		m.Resize(16, 16)
		m.Advance(60)
		out := map[[2]int]Cell{}
		m.Cells(func(x, y int, c Cell) { out[[2]int{x, y}] = c })
		return out
	}
	a, b := mk(false), mk(false)
	for k, v := range a {
		if b[k] != v {
			t.Fatalf("not deterministic at %v", k)
		}
	}
}
