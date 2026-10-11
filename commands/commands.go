// Package commands is the termanim command: one subcommand per animation,
// plus rainhelp, an example CLI with the code rain behind its help.
//
//	termanim fire
//	termanim donut
//	termanim --shape cube
//	termanim list
//
// Press q, Escape or Ctrl-C to stop.
package commands

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/0magnet/calvin/clihelp"
	"github.com/gdamore/tcell/v3"
	"github.com/spf13/cobra"

	"github.com/0magnet/termanim/aquarium"
	"github.com/0magnet/termanim/atom"
	"github.com/0magnet/termanim/aurora"
	"github.com/0magnet/termanim/boids"
	"github.com/0magnet/termanim/bonsai"
	"github.com/0magnet/termanim/bounce"
	"github.com/0magnet/termanim/canvas"
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
	"github.com/0magnet/termanim/magnetosphere"
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
	"atom":          {atom.Run, "electrons on tilted orbits round a glowing nucleus"},
	"aurora":        {aurora.Run, "curtains of light, folding and rippling"},
	"bounce":        {bounce.Run, "the screensaver logo, and the wait for a corner"},
	"flow":          {flow.Run, "particles carried through a curl-noise field"},
	"frost":         {frost.Run, "a crystal growing by diffusion-limited aggregation"},
	"julia":         {noSeed(julia.Run), "a Julia set morphing along the cardioid"},
	"lightning":     {lightning.Run, "a branching discharge, flash and afterglow"},
	"parrot":        {parrot.Run, "the party parrot, rolling and cycling hue"},
	"pendulum":      {pendulum.Run, "double pendulums shearing apart from one another"},
	"physarum":      {physarum.Run, "slime mold building a transport network"},
	"reaction":      {reaction.Run, "Gray-Scott reaction-diffusion, spots into labyrinth"},
	"ripple":        {ripple.Run, "a ripple tank: rain, rings and interference"},
	"wolfram":       {wolfram.Run, "elementary cellular automata scrolling upward"},
	"aquarium":      {aquarium.Run, "fish swimming past swaying seaweed"},
	"boids":         {boids.Run, "flocking by separation, alignment and cohesion"},
	"bonsai":        {bonsai.Run, "a bonsai tree growing branch by branch"},
	"clock":         {noSeed(clock.Run), "an analog clock, after aclock"},
	"cube":          {cube.Run, "a rotating wireframe solid, shaded by depth"},
	"donut":         {donut.Run, "a lit torus, z-buffered"},
	"fire":          {fire.Run, "a heat grid seeded with noise and cooled upward"},
	"fireworks":     {fireworks.Run, "shells that rise, burst and droop"},
	"langton":       {langton.Run, "Langton's ants, chaos then a highway"},
	"lavalamp":      {lavalamp.Run, "wax rising and sinking in a lamp"},
	"life":          {life.Run, "Conway's life, colored by age"},
	"magnetosphere": {magnetosphere.Run, "magnetosphere.net's op-art funnel, counter-scrolling"},
	"matrix":        {matrix.Run, "falling columns of glyphs"},
	"maze":          {maze.Run, "a maze carved, then solved"},
	"metaballs":     {metaballs.Run, "blobs that bulge and merge"},
	"moire":         {noSeed(moire.Run), "two overlapping ripples interfering"},
	"pipes":         {pipes.Run, "pipes growing and turning"},
	"plasma":        {noSeed(plasma.Run), "the drifting colored field of the demoscene"},
	"rain":          {rain.Run, "rain with depth, slant and splashes"},
	"sand":          {sand.Run, "falling sand heaping at its angle of repose"},
	"snow":          {snow.Run, "snow that drifts and settles into banks"},
	"starfield":     {starfield.Run, "stars streaming past the viewer"},
	"tunnel":        {tunnel.Run, "flying down a textured tube"},
}

func names() []string {
	out := make([]string, 0, len(anims))
	for k := range anims {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var shape, list bool

// RootCmd is the termanim command.
var RootCmd = &cobra.Command{
	Use:   "termanim <animation>",
	Short: "play a terminal animation",
	Long: "termanim plays one of the terminal animations in this repository.\n\n" +
		"Press q, Escape or Ctrl-C to stop.",
	Args: cobra.ArbitraryArgs,
	RunE: func(c *cobra.Command, args []string) error {
		if list {
			return c.Help()
		}
		if len(args) == 0 {
			return c.Help()
		}
		return fmt.Errorf("no animation %q", args[0])
	},
}

// play opens the screen and runs one animation until the user stops it.
func play(a anim) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	// Fini restores the terminal. Without it an error leaves the user in a
	// screen with no cursor and no echo.
	defer screen.Fini()

	target := screen
	if shape {
		// Glyph and Braille animations ignore it; they draw characters already.
		target = canvas.ShapeScreen{Screen: screen}
	}
	if err := a.run(target, time.Now().UnixNano()); err != nil {
		screen.Fini()
		return err
	}
	return nil
}

func init() {
	RootCmd.PersistentFlags().BoolVar(&shape, "shape", false,
		"draw pixel animations as ASCII characters chosen by shape rather than in half blocks")
	RootCmd.Flags().BoolVar(&list, "list", false, "list the animations")
	for _, n := range names() {
		a := anims[n]
		RootCmd.AddCommand(&cobra.Command{
			Use:   n,
			Short: a.desc,
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return play(a) },
		})
	}
	RootCmd.AddCommand(rainhelpCmd())
	clihelp.Init(RootCmd, "termanim")
	wireRainhelp()
}

// Execute runs the termanim command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
