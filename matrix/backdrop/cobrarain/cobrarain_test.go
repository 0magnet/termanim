package cobrarain

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/0magnet/termanim/matrix/backdrop"
	"github.com/0magnet/termanim/plasma"
)

func helpOf(t *testing.T, o backdrop.Options) string {
	t.Helper()
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "a command that exists to have its help printed",
		// Runnable, because cobra prints only the Short description for a
		// command it cannot run — no usage line and no flags — and then there
		// is nothing here worth asserting about.
		Run: func(*cobra.Command, []string) {},
	}
	cmd.Flags().String("flag", "", "do the thing")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	On(cmd, o)
	cmd.HelpFunc()(cmd, nil)
	return buf.String()
}

// The help itself is cobra's and must survive whatever is put behind it. If
// this fails, nothing else here matters.
func TestHelpTextSurvives(t *testing.T) {
	for name, o := range map[string]backdrop.Options{
		"rain": {Force: true, Width: 60, Seed: 1},
		"anim": {Force: true, Width: 60, Seed: 1, Anim: plasma.New()},
	} {
		got := helpOf(t, o)
		for _, want := range []string{"demo", "--flag", "do the thing"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: help is missing %q", name, want)
			}
		}
	}
}

// The point of putting the animation in Options rather than in a second entry
// point: cobrarain was not changed at all, and a CLI can now put a plasma
// behind its help by setting one field.
func TestOptionsAnimReachesThroughCobra(t *testing.T) {
	rain := helpOf(t, backdrop.Options{Force: true, Width: 60, Seed: 1, Steps: 20})
	anim := helpOf(t, backdrop.Options{Force: true, Width: 60, Seed: 1, Anim: plasma.New()})

	if rain == anim {
		t.Fatal("the help looks identical with and without an animation; Anim did not reach Render")
	}
	if !strings.Contains(anim, "\x1b[") {
		t.Error("nothing was drawn behind the help")
	}
}

// Off has to survive the trip too — this is the --json case, where a caller
// wants its help plain even on a terminal.
func TestOffPassesThroughUnpainted(t *testing.T) {
	got := helpOf(t, backdrop.Options{Force: true, Off: true, Anim: plasma.New()})
	if strings.Contains(got, "\x1b[") {
		t.Error("Off still painted something behind the help")
	}
	if !strings.Contains(got, "--flag") {
		t.Error("Off lost the help text")
	}
}

// Usage is what a command prints when it is called wrongly. Scripts capture it
// and people grep it, so it must stay plain however the help is decorated.
func TestUsageIsNotPainted(t *testing.T) {
	cmd := &cobra.Command{Use: "demo", Run: func(*cobra.Command, []string) {}}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	On(cmd, backdrop.Options{Force: true, Width: 60, Anim: plasma.New()})
	if err := cmd.UsageFunc()(cmd); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Error("usage was painted; it is meant to stay plain")
	}
}
