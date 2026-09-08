package backdrop

import (
	"strings"
	"testing"

	"github.com/0magnet/termanim/plasma"
)

// Options.Anim exists so that a caller which can only hand over Options — of
// which cobrarain is the one that matters — can still choose the backdrop.
// These check that the field is actually honored everywhere Options travels,
// and that leaving it nil changes nothing at all.

func TestOptionsAnimMatchesRenderAnim(t *testing.T) {
	const text = "usage: something\n  --flag   do the thing\n"
	o := Options{Force: true, Width: 40, Seed: 7, Warm: 0.2}

	viaField := Render(text, Options{
		Force: o.Force, Width: o.Width, Seed: o.Seed, Warm: o.Warm,
		Anim: plasma.New(),
	})
	viaArg := RenderAnim(text, plasma.New(), o)

	if viaField != viaArg {
		t.Error("Render with Options.Anim differs from RenderAnim with the same animation")
	}
}

// The rain is the default and must stay the default: an Options with no Anim
// has to render exactly what it rendered before the field existed.
func TestNoAnimIsStillTheRain(t *testing.T) {
	const text = "usage: something\n"
	o := Options{Force: true, Width: 40, Seed: 7, Steps: 20}

	a := Render(text, o)
	b := Render(text, o)
	if a != b {
		t.Fatal("the rain is not reproducible for a fixed seed; this test cannot say anything")
	}
	// And it must not be what the animation path produces.
	withAnim := Render(text, Options{Force: true, Width: 40, Seed: 7, Anim: plasma.New()})
	if a == withAnim {
		t.Error("setting Anim changed nothing")
	}
}

func TestPainterNewHonorsOptionsAnim(t *testing.T) {
	o := Options{Force: true, Width: 30, Seed: 3, Pad: -1, Anim: plasma.New()}
	p := New(o)
	if p.Animation() == nil {
		t.Fatal("New ignored Options.Anim: the painter has no animation")
	}
	if p.Matrix() != nil {
		t.Error("New built a rain as well as an animation")
	}

	// And it draws: the text has to survive, and something has to be behind it.
	out := p.Frame("hello", 0.1)
	if !strings.Contains(out, "hello") {
		t.Error("the text did not survive the painter")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("nothing was drawn behind the text")
	}
}

func TestPainterWithoutAnimStillBuildsTheRain(t *testing.T) {
	p := New(Options{Force: true, Width: 30, Seed: 3, Pad: -1})
	p.Frame("hello", 0.1)
	if p.Matrix() == nil {
		t.Error("a painter with no Anim did not build the rain")
	}
	if p.Animation() != nil {
		t.Error("a painter with no Anim picked up an animation from somewhere")
	}
}

// Off has to win over Anim, the same way it wins over everything else. A
// caller that turns the backdrop off for a --json mode must get plain text
// whatever animation is configured.
func TestOffBeatsAnim(t *testing.T) {
	const text = "plain\n"
	got := Render(text, Options{Force: true, Off: true, Anim: plasma.New()})
	if got != text {
		t.Errorf("Off did not return the text unchanged: %q", got)
	}
}
