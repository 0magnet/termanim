package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0magnet/termanim/matrix/backdrop"
	"github.com/0magnet/termanim/matrix/backdrop/cobrarain"
)

// rainhelpCmd is an example command whose help screen has the code rain behind
// it, generated fresh on every run. It does nothing else worth doing; the part
// to copy is the single cobrarain call in wireRainhelp. The flags that tune
// the look are there so it can be settled on before it is copied:
//
//	termanim rainhelp --dim 128 --help    # rain behind the text, brighter
//	termanim rainhelp --pad 6 --help      # more rain around the text
//	termanim rainhelp --seed 1 --help     # the same frame every time
//	termanim rainhelp --help | cat        # plain, as a pipe should be
var rainOpts = struct {
	dim, pad, width int
	seed            int64
	force           bool
}{}

var rainCmd = &cobra.Command{
	Use:   "rainhelp",
	Short: "a cobra CLI with the code rain behind its help",
	Long: "rainhelp is an example, not a tool.\n\n" +
		"Its help screen is printed over a still frame of the Matrix code rain,\n" +
		"seeded from the clock, so it is different every time you ask for it. The\n" +
		"rain is a still: nothing takes over the terminal, nothing waits for a\n" +
		"key, and the help scrolls up the scrollback the way help does.\n\n" +
		"Piping or redirecting turns it off, so --help | less and a --help pasted\n" +
		"into a bug report are both plain text.",
	// With no arguments, show the thing the command exists to show.
	RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
}

func rainhelpCmd() *cobra.Command {
	f := rainCmd.PersistentFlags()
	f.IntVar(&rainOpts.dim, "dim", 0, "brightness of the rain behind the text, out of 256")
	f.IntVar(&rainOpts.pad, "pad", 0, "rows of rain above and below, and the text indent")
	f.Int64Var(&rainOpts.seed, "seed", 0, "seed for the rain; 0 takes one from the clock")
	f.BoolVar(&rainOpts.force, "force", false, "draw the rain even when output is not a terminal")
	f.IntVar(&rainOpts.width, "width", 0, "columns to render; 0 asks the terminal")

	rainCmd.AddCommand(&cobra.Command{
		Use:   "play <animation>",
		Short: "pretend to play an animation",
		Long: "play does not play anything; termanim <animation> does.\n\n" +
			"It is here so the help has a command list on it and so there is a\n" +
			"subcommand whose own help can be asked for, which is the case worth\n" +
			"checking: cobra looks the help function up on the parent, so wiring the\n" +
			"parent wires everything under it.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}, &cobra.Command{
		Use:   "version",
		Short: "print a version that is not real",
		Run:   func(*cobra.Command, []string) { fmt.Println("rainhelp (example)") },
	})
	return rainCmd
}

// wireRainhelp puts the rain behind rainhelp's help. It must run after
// clihelp.Init: cobrarain captures the help function in effect when it is
// called and draws the rain behind its output, so wiring it earlier would put
// the rain behind the uncolored help.
func wireRainhelp() {
	cobrarain.OnFunc(rainCmd, func(*cobra.Command) backdrop.Options {
		return backdrop.Options{Dim: rainOpts.dim, Pad: rainOpts.pad, Seed: rainOpts.seed, Force: rainOpts.force, Width: rainOpts.width}
	})
}
