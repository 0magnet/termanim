// Command rainhelp is a cobra CLI whose help screen has the code rain behind
// it, generated fresh on every run.
//
//	go run ./cmd/rainhelp --help
//	go run ./cmd/rainhelp play --help
//
// It does nothing else worth doing. The commands and flags below are there to
// give the help screen enough on it to be worth looking at; the part to copy is
// the single cobrarain call in main, and the flags that tune the look are so
// that the look can be settled on before it is copied:
//
//	go run ./cmd/rainhelp --dim 128 --help    # rain behind the text, brighter
//	go run ./cmd/rainhelp --pad 6 --help      # more rain around the text
//	go run ./cmd/rainhelp --seed 1 --help     # the same frame every time
//	go run ./cmd/rainhelp --help | cat        # plain, as a pipe should be
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/0magnet/termanim/matrix/backdrop"
	"github.com/0magnet/termanim/matrix/backdrop/cobrarain"
)

func main() {
	var (
		dim   int
		pad   int
		seed  int64
		force bool
		width int
	)

	root := &cobra.Command{
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

	f := root.PersistentFlags()
	f.IntVar(&dim, "dim", 0, "brightness of the rain behind the text, out of 256")
	f.IntVar(&pad, "pad", 0, "rows of rain above and below, and the text indent")
	f.Int64Var(&seed, "seed", 0, "seed for the rain; 0 takes one from the clock")
	f.BoolVar(&force, "force", false, "draw the rain even when output is not a terminal")
	f.IntVar(&width, "width", 0, "columns to render; 0 asks the terminal")

	root.AddCommand(&cobra.Command{
		Use:   "play <animation>",
		Short: "pretend to play an animation",
		Long: "play does not play anything. termanim(1) does.\n\n" +
			"It is here so the root help has a command list on it and so there is a\n" +
			"subcommand whose own help can be asked for, which is the case worth\n" +
			"checking: cobra looks the help function up on the parent, so wiring the\n" +
			"root wires everything under it.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	})

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "print a version that is not real",
		Run:   func(*cobra.Command, []string) { fmt.Println("rainhelp (example)") },
	})

	// This is the whole of it. Everything above is a CLI to hang it on.
	cobrarain.OnFunc(root, func(*cobra.Command) backdrop.Options {
		return backdrop.Options{Dim: dim, Pad: pad, Seed: seed, Force: force, Width: width}
	})

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
