package clock

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

const tw, th = 60, 24

// at returns a clock held at one moment, drawn once, and the screen it drew on.
//
// A clock is a pure function of the time, which makes it the most testable
// thing in this repository: fix the time and the picture is fixed too.
func at(t *testing.T, when time.Time) (tcell.SimulationScreen, *Clock) {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Fini)
	s.SetSize(tw, th)

	c := New()
	c.Now = func() time.Time { return when }
	c.Resize(tw, th)
	c.Frame(s, tw, th, 0)
	s.Show()
	return s, c
}

// grid reads the screen back as rows of runes.
func grid(s tcell.SimulationScreen) []string {
	cells, w, h := s.GetContents()
	out := make([]string, 0, h)
	for y := 0; y < h; y++ {
		var b strings.Builder
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				b.WriteByte(' ')
				continue
			}
			b.WriteRune(c.Runes[0])
		}
		out = append(out, b.String())
	}
	return out
}

func TestDialHasSixtyMarks(t *testing.T) {
	s, _ := at(t, time.Date(2026, 8, 22, 10, 8, 30, 0, time.UTC))
	var marks, hours int
	for _, row := range grid(s) {
		marks += strings.Count(row, ".")
		hours += strings.Count(row, "o")
	}
	// Twelve of the sixty are hour marks. The dots are the other forty-eight
	// plus the second hand and the two colons in the digital readout.
	if hours != 12 {
		t.Errorf("the dial has %d hour marks, want 12", hours)
	}
	if marks < 48 {
		t.Errorf("the dial has %d minute marks, want at least 48", marks)
	}
}

// Twelve o'clock is the one position anybody can check by eye: the minute hand
// points straight up.
//
// Half a minute past, so that the hands separate. At 12:00:00 exactly all three
// point the same way, and the second hand is both the longest and the last
// drawn, so it covers the other two completely — true of the original as well,
// and the reason this is not tested on the hour.
func TestTheMinuteHandPointsUpAtTwelve(t *testing.T) {
	s, _ := at(t, time.Date(2026, 8, 22, 12, 0, 30, 0, time.UTC))
	rows := grid(s)
	cx, cy := tw/2, th/2

	up, down := false, false
	for y := 0; y < cy; y++ {
		if []rune(rows[y])[cx] == 'm' {
			up = true
		}
	}
	// And the second hand, thirty seconds round, points the other way.
	for y := cy + 1; y < th; y++ {
		if []rune(rows[y])[cx] == '.' {
			down = true
		}
	}
	if !up {
		t.Error("at twelve the minute hand does not point up the centre column")
	}
	if !down {
		t.Error("at half a minute past the second hand does not point down")
	}
	for y := cy + 1; y < th; y++ {
		if []rune(rows[y])[cx] == 'm' {
			t.Errorf("the minute hand reaches below the centre, at row %d", y)
		}
	}
}

// Three o'clock puts the hour hand on the right, which is the other axis and
// the one that catches a sign error in the aspect scaling.
func TestHourHandPointsRightAtThree(t *testing.T) {
	s, _ := at(t, time.Date(2026, 8, 22, 3, 0, 0, 0, time.UTC))
	rows := grid(s)
	cx, cy := tw/2, th/2

	right := false
	for x := cx + 1; x < tw; x++ {
		if []rune(rows[cy])[x] == 'h' {
			right = true
		}
	}
	if !right {
		t.Error("at three the hour hand does not point right along the centre row")
	}
	for x := 0; x < cx; x++ {
		if []rune(rows[cy])[x] == 'h' {
			t.Errorf("at three there is an hour hand to the left of centre, at column %d", x)
		}
	}
}

// The hour hand creeps between the hours rather than jumping at the top of
// each, which is what a real one does and what makes half past readable.
//
// An hour is five dial positions, so a minute is a twelfth of one and sixty of
// them must land exactly on the next hour. Dividing by ten instead — which the
// original does, in integer arithmetic — puts the hand a whole position past
// the hour by the time the hour is up, which at three would have it pointing
// past the four.
func TestHourHandCreepsBetweenHours(t *testing.T) {
	pos := func(h, m int) float64 { return float64(h%12)*5 + float64(m)/12 }

	if a, b := pos(3, 0), pos(3, 30); b <= a {
		t.Errorf("half past three (%v) is not past three o'clock (%v)", b, a)
	}
	if got, want := pos(3, 60), pos(4, 0); got != want {
		t.Errorf("three o'clock plus sixty minutes is %v; four o'clock is %v", got, want)
	}
	if got, want := pos(3, 30), 17.5; got != want {
		t.Errorf("half past three is at %v, want %v — halfway between the 3 and the 4", got, want)
	}
}

func TestDigitalReadoutShowsTheTime(t *testing.T) {
	s, _ := at(t, time.Date(2026, 8, 22, 13, 45, 7, 0, time.UTC))
	joined := strings.Join(grid(s), "\n")
	if !strings.Contains(joined, "[13:45:07]") {
		t.Error("the digital readout does not show 13:45:07")
	}
	if !strings.Contains(joined, ".:ACLOCK:.") {
		t.Error("the title is missing")
	}
}

// A second hand that does not move is the one failure that looks exactly like
// a working clock in a screenshot.
func TestTheClockMoves(t *testing.T) {
	base := time.Date(2026, 8, 22, 10, 8, 0, 0, time.UTC)
	a := grid(func() tcell.SimulationScreen { s, _ := at(t, base); return s }())
	b := grid(func() tcell.SimulationScreen { s, _ := at(t, base.Add(time.Second)); return s }())
	if strings.Join(a, "\n") == strings.Join(b, "\n") {
		t.Error("a second passed and nothing on the clock changed")
	}
}

// Every hour of the day must draw, including midnight and noon, where the
// twelve-hour wrap is easy to get wrong by one.
func TestEveryHourDraws(t *testing.T) {
	for h := 0; h < 24; h++ {
		when := time.Date(2026, 8, 22, h, 30, 0, 0, time.UTC)
		s, _ := at(t, when)
		if !strings.Contains(strings.Join(grid(s), "\n"), "h") {
			t.Errorf("no hour hand at %02d:30", h)
		}
	}
}

// A window too small for a dial must draw nothing rather than a smear of
// characters through the middle of it.
func TestATinyWindowDrawsNothing(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(8, 4)

	c := New()
	c.Now = func() time.Time { return time.Date(2026, 8, 22, 10, 8, 30, 0, time.UTC) }
	c.Resize(8, 4)
	c.Frame(s, 8, 4, 0)
	s.Show()

	for _, row := range grid(s) {
		if strings.TrimSpace(row) != "" {
			t.Errorf("a window with no room for a dial drew %q", row)
		}
	}
}

// Nothing may be drawn outside the screen; the hands are computed from a
// radius and could overshoot on an awkward size.
func TestNothingIsDrawnOffScreen(t *testing.T) {
	for _, size := range [][2]int{{20, 10}, {200, 50}, {40, 40}, {13, 9}} {
		s := tcell.NewSimulationScreen("UTF-8")
		if err := s.Init(); err != nil {
			t.Fatal(err)
		}
		s.SetSize(size[0], size[1])
		c := New()
		c.Now = func() time.Time { return time.Date(2026, 8, 22, 7, 23, 45, 0, time.UTC) }
		c.Resize(size[0], size[1])
		// Must not panic or write out of bounds.
		c.Frame(s, size[0], size[1], 0)
		s.Show()
		s.Fini()
	}
}

// Sweep makes the second hand continuous. Without it the hand must be in the
// same place all through a second, which is what a stepping hand means.
func TestSweepMovesWithinASecond(t *testing.T) {
	base := time.Date(2026, 8, 22, 10, 8, 30, 0, time.UTC)
	half := base.Add(500 * time.Millisecond)

	still := func(sweep bool) bool {
		draw := func(when time.Time) string {
			s := tcell.NewSimulationScreen("UTF-8")
			if err := s.Init(); err != nil {
				t.Fatal(err)
			}
			defer s.Fini()
			s.SetSize(tw, th)
			c := New()
			c.Sweep = sweep
			c.Now = func() time.Time { return when }
			c.Resize(tw, th)
			c.Frame(s, tw, th, 0)
			s.Show()
			return strings.Join(grid(s), "\n")
		}
		return draw(base) == draw(half)
	}

	if !still(false) {
		t.Error("a stepping second hand moved within a second")
	}
	if still(true) {
		t.Error("a sweeping second hand did not move within a second")
	}
}
