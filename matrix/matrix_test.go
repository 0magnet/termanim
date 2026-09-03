package matrix

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/internal/simscreen"
)

// dt is one frame at the rate these constants were tuned at, so every
// assertion about how far things move over N frames still means what it did.
const dt = 1.0 / 30

const tw, th = 40, 24

func newScreen(t *testing.T) tcell.Screen {
	t.Helper()
	s := simscreen.NewScreen()
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(tw, th)
	return s
}

func TestColumnsFall(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	m := New(1)
	m.Resize(tw, th)

	// Find a column that is running, then check its head descends.
	var x int = -1
	for i := range m.col {
		if m.col[i].active {
			x = i
			break
		}
	}
	if x < 0 {
		t.Skip("no column active at start with this seed")
	}
	before := m.col[x].head
	for i := 0; i < 10; i++ {
		m.Frame(s, tw, th, dt)
	}
	if m.col[x].active && m.col[x].head <= before {
		t.Errorf("column %d head did not descend: %v then %v", x, before, m.col[x].head)
	}
}

func TestDrawsGlyphsNotBlocks(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	m := New(1)
	m.Resize(tw, th)
	for i := 0; i < 60; i++ {
		m.Frame(s, tw, th, dt)
	}
	s.Show()

	cells, w, _ := simscreen.Contents(s)
	inAlphabet := map[rune]bool{}
	for _, r := range m.Glyphs {
		inAlphabet[r] = true
	}
	var drawn int
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				continue
			}
			if inAlphabet[c.Runes[0]] {
				drawn++
			}
		}
	}
	if drawn == 0 {
		t.Fatal("no glyph from the alphabet was drawn")
	}
}

func TestHeadIsBrightest(t *testing.T) {
	s := newScreen(t)
	defer s.Fini()
	m := New(1)
	m.Resize(tw, th)
	for i := 0; i < 40; i++ {
		m.Frame(s, tw, th, dt)
	}
	s.Show()
	cells, w, _ := simscreen.Contents(s)

	lum := func(c tcell.Color) int32 {
		r, g, b := c.RGB()
		return r + g + b
	}
	for x := 0; x < tw; x++ {
		c := m.col[x]
		head := int(c.head)
		if !c.active || head < 1 || head >= th {
			continue
		}
		hf := cells[head*w+x].Style.GetForeground()
		tf := cells[(head-1)*w+x].Style.GetForeground()
		if lum(hf) < lum(tf) {
			t.Fatalf("column %d: head at row %d is dimmer than the glyph behind it", x, head)
		}
		return
	}
}

func TestAllGlyphsAreSingleWidth(t *testing.T) {
	// Full-width kana would occupy two cells and the columns would not line
	// up. Every glyph in the default alphabet must be one cell wide.
	for _, r := range Glyphs {
		if r > 0xFF00 && r < 0xFF61 {
			t.Errorf("glyph %q is full-width", r)
		}
	}
}

func TestDeterministicForAGivenSeed(t *testing.T) {
	run := func() []column {
		s := simscreen.NewScreen()
		_ = s.Init()
		defer s.Fini()
		s.SetSize(tw, th)
		m := New(7)
		m.Resize(tw, th)
		for i := 0; i < 25; i++ {
			m.Frame(s, tw, th, dt)
		}
		return m.col
	}
	a, b := run(), run()
	for i := range a {
		// reflect.DeepEqual rather than ==: a column carries the word its
		// stream is spelling, and a slice field makes the struct uncomparable.
		if !reflect.DeepEqual(a[i], b[i]) {
			t.Fatalf("same seed diverged at column %d", i)
		}
	}
}

// The columns are stepped at a fixed rate from elapsed time, so the same
// wall-clock interval must run the same number of steps however it is divided
// into frames.
func TestFrameRateIndependent(t *testing.T) {
	run := func(frames int, step float64) []column {
		s := simscreen.NewScreen()
		_ = s.Init()
		defer s.Fini()
		s.SetSize(tw, th)
		m := New(5)
		m.Resize(tw, th)
		for i := 0; i < frames; i++ {
			m.Frame(s, tw, th, step)
		}
		return m.col
	}
	slow := run(60, 1.0/30)  // 2 seconds
	fast := run(120, 1.0/60) // the same 2 seconds
	for i := range slow {
		if !reflect.DeepEqual(slow[i], fast[i]) {
			t.Fatalf("column %d differs after two seconds: %+v at 30fps, %+v at 60fps",
				i, slow[i], fast[i])
		}
	}
}

// Only about one stream in five leads with a highlighted glyph.
//
// Highlighting every head is the obvious reading of "the leading character is
// bright", and it is wrong: it turns the screen into a row of white dots racing
// down it. The film has plain green rain with white heads scattered through it.
func TestOnlySomeStreamsAreHighlighted(t *testing.T) {
	m := New(1)
	m.Resize(400, 40)
	// Run long enough that most columns have been through a cycle.
	for i := 0; i < 500; i++ {
		m.step()
	}

	var live, hot int
	for _, c := range m.col {
		if !c.active {
			continue
		}
		live++
		if c.hot {
			hot++
		}
	}
	if live < 50 {
		t.Fatalf("only %d live streams; not enough to judge the ratio", live)
	}
	frac := float64(hot) / float64(live)
	if frac < 0.05 || frac > 0.40 {
		t.Errorf("%d of %d streams are highlighted (%.0f%%), want roughly one in five",
			hot, live, frac*100)
	}
}

// The alphabet turns over on a shared beat. Between beats the only glyph that
// changes is one a head has just moved onto; everything else holds still.
//
// Independent per-glyph flicker is what this did, and it reads as static rather
// than as text being rewritten — in the film every glyph that changes changes on
// the same frame.
func TestGlyphsChangeOnACommonBeat(t *testing.T) {
	m := New(7)
	m.Resize(tw, th)
	m.ChangeEvery = 4
	m.StammerEvery = 0 // isolate the cadence
	for i := 0; i < 40; i++ {
		m.step()
	}

	headRow := func() map[int]int {
		h := map[int]int{}
		for x, c := range m.col {
			if c.active {
				h[x] = int(c.head)
			}
		}
		return h
	}

	// Step to just before a beat, then take the step that is NOT a beat.
	for m.tick%m.ChangeEvery == m.ChangeEvery-1 {
		m.step()
	}
	before := append([]rune(nil), m.glyph...)
	headsBefore := headRow()
	m.step()
	headsAfter := headRow()

	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			i := y*tw + x
			if before[i] == m.glyph[i] {
				continue
			}
			// A head that moved onto this row is allowed to have written it.
			if headsAfter[x] == y && headsBefore[x] != y {
				continue
			}
			t.Fatalf("glyph at %d,%d changed on a step that is not a beat (tick %d)", x, y, m.tick)
		}
	}
}

// Every so often the highlighted glyphs all hesitate at once, and the streams
// carrying them drop a row behind the ones that did not.
func TestTheHighlightedStreamsStammerTogether(t *testing.T) {
	m := New(3)
	m.Resize(tw, th)
	m.StammerEvery = 6

	for i := 0; i < 400; i++ {
		before := make([]float64, len(m.col))
		wasActive := make([]bool, len(m.col))
		for x, c := range m.col {
			before[x], wasActive[x] = c.head, c.active
		}
		m.step()

		var hotHeld, hotMoved, coldMoved int
		for x, c := range m.col {
			if !wasActive[x] || !c.active {
				continue
			}
			moved := c.head != before[x]
			switch {
			case c.hot && moved:
				hotMoved++
			case c.hot:
				hotHeld++
			case moved:
				coldMoved++
			}
		}
		// A stammer is every highlighted stream holding while others move on.
		if hotHeld > 0 && hotMoved == 0 && coldMoved > 0 {
			return
		}
	}
	t.Error("no stammer in 400 steps: the highlighted streams never fell behind")
}

// The film's alphabet has exactly one letter of the English alphabet in it.
func TestTheAlphabetHasZAndNoOtherLetter(t *testing.T) {
	var letters []rune
	for _, r := range Glyphs {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			letters = append(letters, r)
		}
	}
	if string(letters) != "Z" {
		t.Errorf("the alphabet's Latin letters are %q, want just \"Z\"", string(letters))
	}
}

// A glyph is dimmed on the step it changes.
//
// In the film a change is a crossfade — "during a single frame, the new and old
// glyph occupy the same space, each at 50% opacity". A cell holds one glyph and
// cannot draw two, so the half-lit pair becomes a beat: dim on the step it
// changes, full brightness on the next. It is what makes the trail pulse, and
// what makes a cell changing twice running look like it is wavering between two
// characters.
func TestAChangedGlyphIsDimmedForOneStep(t *testing.T) {
	s := simscreen.NewScreen()
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(tw, th)

	m := New(5)
	m.Resize(tw, th)
	for i := 0; i < 60; i++ {
		m.step()
	}
	m.draw(s)
	s.Show()
	cells, w, _ := simscreen.Contents(s)

	checked := 0
	for x := 0; x < tw; x++ {
		c := m.col[x]
		if !c.active {
			continue
		}
		for y := 0; y < th; y++ {
			d := int(c.head) - y
			if d < 0 || d >= c.length {
				continue
			}
			// What the cell's intensity would be with no blend.
			var i int
			switch {
			case d == 0 && c.hot:
				i = 255
			case d == 0:
				i = 200
			default:
				i = 200 - d*200/c.length
				if i < 0 {
					i = 0
				}
			}
			if m.changed[y*tw+x] == m.tick {
				i = i * m.Blend / 256
			}
			got := cells[y*w+x].Style.GetForeground()
			if got != m.Palette[i] {
				t.Fatalf("cell %d,%d drew %v, want %v (blend %v)",
					x, y, got, m.Palette[i], m.changed[y*tw+x] == m.tick)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no lit cells to check")
	}
}

// Turning the blend off must actually change what is drawn, or the option is
// decoration.
func TestBlendOffDrawsAtFullBrightness(t *testing.T) {
	draw := func(blend int) string {
		s := simscreen.NewScreen()
		if err := s.Init(); err != nil {
			t.Fatal(err)
		}
		defer s.Fini()
		s.SetSize(tw, th)
		m := New(5)
		m.Blend = blend
		m.Resize(tw, th)
		for i := 0; i < 60; i++ {
			m.step()
		}
		m.draw(s)
		s.Show()
		cells, w, _ := simscreen.Contents(s)
		var b strings.Builder
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				fg := cells[y*w+x].Style.GetForeground()
				r, g, bl := fg.RGB()
				b.WriteString(string(rune('0' + (r+g+bl)%10)))
			}
		}
		return b.String()
	}
	if draw(128) == draw(0) {
		t.Error("the blend changed nothing that is drawn")
	}
}
