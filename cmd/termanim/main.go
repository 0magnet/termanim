// Command termanim plays one of the terminal animations in this repository.
//
//	termanim fire
//	termanim donut
//	termanim -list
//
// Press q, Escape or Ctrl-C to stop.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v3"

	"github.com/0magnet/termanim/aquarium"
	"github.com/0magnet/termanim/atom"
	"github.com/0magnet/termanim/aurora"
	"github.com/0magnet/termanim/boids"
	"github.com/0magnet/termanim/bonsai"
	"github.com/0magnet/termanim/bounce"
	"github.com/0magnet/termanim/clock"
	"github.com/0magnet/termanim/cube"
	"github.com/0magnet/termanim/donut"
	"github.com/0magnet/termanim/fire"
	"github.com/0magnet/termanim/fireworks"
	"github.com/0magnet/termanim/flow"
	"github.com/0magnet/termanim/frost"
	"github.com/0magnet/termanim/julia"
	"github.com/0magnet/termanim/langton"
	"github.com/0magnet/termanim/lavalamp"
	"github.com/0magnet/termanim/life"
	"github.com/0magnet/termanim/lightning"
	"github.com/0magnet/termanim/logo"
	"github.com/0magnet/termanim/matrix"
	"github.com/0magnet/termanim/maze"
	"github.com/0magnet/termanim/metaballs"
	"github.com/0magnet/termanim/moire"
	"github.com/0magnet/termanim/parrot"
	"github.com/0magnet/termanim/pendulum"
	"github.com/0magnet/termanim/physarum"
	"github.com/0magnet/termanim/pipes"
	"github.com/0magnet/termanim/plasma"
	"github.com/0magnet/termanim/rain"
	"github.com/0magnet/termanim/reaction"
	"github.com/0magnet/termanim/ripple"
	"github.com/0magnet/termanim/sand"
	"github.com/0magnet/termanim/snow"
	"github.com/0magnet/termanim/starfield"
	"github.com/0magnet/termanim/tunnel"
	"github.com/0magnet/termanim/wolfram"
)

// anim is one entry in the catalog. Every package exposes the same Run, so
// adding an animation is a line here and an import above.
type anim struct {
	run  func(tcell.Screen, int64) error
	desc string
}

// noSeed adapts the two animations that are a pure function of time and have
// nothing to randomize.
func noSeed(f func(tcell.Screen) error) func(tcell.Screen, int64) error {
	return func(s tcell.Screen, _ int64) error { return f(s) }
}

var anims = map[string]anim{
	"atom":      {atom.Run, "electrons on tilted orbits round a glowing nucleus"},
	"aurora":    {aurora.Run, "curtains of light, folding and rippling"},
	"bounce":    {bounce.Run, "the screensaver logo, and the wait for a corner"},
	"flow":      {flow.Run, "particles carried through a curl-noise field"},
	"frost":     {frost.Run, "a crystal growing by diffusion-limited aggregation"},
	"julia":     {noSeed(julia.Run), "a Julia set morphing along the cardioid"},
	"lightning": {lightning.Run, "a branching discharge, flash and afterglow"},
	"parrot":    {parrot.Run, "the party parrot, rolling and cycling hue"},
	"pendulum":  {pendulum.Run, "double pendulums shearing apart from one another"},
	"physarum":  {physarum.Run, "slime mold building a transport network"},
	"reaction":  {reaction.Run, "Gray-Scott reaction-diffusion, spots into labyrinth"},
	"ripple":    {ripple.Run, "a ripple tank: rain, rings and interference"},
	"wolfram":   {wolfram.Run, "elementary cellular automata scrolling upward"},
	"aquarium":  {aquarium.Run, "fish swimming past swaying seaweed"},
	"boids":     {boids.Run, "flocking by separation, alignment and cohesion"},
	"bonsai":    {bonsai.Run, "a bonsai tree growing branch by branch"},
	"clock":     {noSeed(clock.Run), "an analog clock, after aclock"},
	"cube":      {cube.Run, "a rotating wireframe solid, shaded by depth"},
	"donut":     {donut.Run, "a lit torus, z-buffered"},
	"fire":      {fire.Run, "a heat grid seeded with noise and cooled upward"},
	"fireworks": {fireworks.Run, "shells that rise, burst and droop"},
	"langton":   {langton.Run, "Langton's ants, chaos then a highway"},
	"lavalamp":  {lavalamp.Run, "wax rising and sinking in a lamp"},
	"logo":      {logo.Run, "magnetosphere.net's op-art funnel, counter-scrolling"},
	"life":      {life.Run, "Conway's life, colored by age"},
	"matrix":    {matrix.Run, "falling columns of glyphs"},
	"maze":      {maze.Run, "a maze carved, then solved"},
	"metaballs": {metaballs.Run, "blobs that bulge and merge"},
	"moire":     {noSeed(moire.Run), "two overlapping ripples interfering"},
	"pipes":     {pipes.Run, "pipes growing and turning"},
	"plasma":    {noSeed(plasma.Run), "the drifting colored field of the demoscene"},
	"rain":      {rain.Run, "rain with depth, slant and splashes"},
	"sand":      {sand.Run, "falling sand heaping at its angle of repose"},
	"snow":      {snow.Run, "snow that drifts and settles into banks"},
	"starfield": {starfield.Run, "stars streaming past the viewer"},
	"tunnel":    {tunnel.Run, "flying down a textured tube"},
}

func names() []string {
	out := make([]string, 0, len(anims))
	for k := range anims {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func usage(w *os.File) {
	fmt.Fprintf(w, "usage: termanim <animation>\n\n") //nolint:errcheck,gosec
	for _, n := range names() {
		fmt.Fprintf(w, "  %-10s %s\n", n, anims[n].desc) //nolint:errcheck,gosec
	}
	fmt.Fprintf(w, "\nPress q, Escape or Ctrl-C to stop.\n") //nolint:errcheck,gosec
}

func main() {
	if len(os.Args) != 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "-h", "--help", "-list", "--list":
		usage(os.Stdout)
		return
	}
	a, ok := anims[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "termanim: no animation %q\n\navailable: %s\n",
			os.Args[1], strings.Join(names(), " "))
		os.Exit(2)
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintln(os.Stderr, "termanim:", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "termanim:", err)
		os.Exit(1)
	}
	// Fini restores the terminal. Without it an error leaves the user in a
	// screen with no cursor and no echo.
	defer screen.Fini()

	if err := a.run(screen, time.Now().UnixNano()); err != nil {
		screen.Fini()
		fmt.Fprintln(os.Stderr, "termanim:", err)
		os.Exit(1)
	}
}
