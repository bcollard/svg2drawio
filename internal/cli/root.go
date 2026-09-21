// Package cli implements the svg2drawio command line interface.
package cli

import (
	"github.com/spf13/cobra"
)

// Build information, injected from main via SetBuildInfo.
var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

// skillMD holds the Agent Skill definition embedded in the binary.
var skillMD string

// SetBuildInfo records the values stamped into the binary at link time.
func SetBuildInfo(version, commit, date string) {
	buildVersion, buildCommit, buildDate = version, commit, date
}

// SetSkill records the embedded SKILL.md content.
func SetSkill(skill string) { skillMD = skill }

const rootLong = `svg2drawio converts SVG files into draw.io (mxGraph) XML.

The default output is a .drawio diagram whose cells are native draw.io shapes:
rectangles stay rectangles, circles stay ellipses, connectors become edges with
draggable endpoints, and arbitrary paths become inline stencils that draw.io
renders as first class shapes.`

const rootExample = `  # A .drawio file next to the input
  svg2drawio diagram.svg

  # A whole tree of icons into one output directory
  svg2drawio -o out/ icons/

  # Several SVGs as pages of a single file
  svg2drawio -o all.drawio a.svg b.svg c.svg

  # Straight to the clipboard, ready for Extras > Edit Diagram
  svg2drawio -o - diagram.svg | pbcopy`

// NewRootCmd builds the root command. Running it with SVG arguments converts
// them, which keeps the common case a single word.
func NewRootCmd() *cobra.Command {
	opts := &convertOptions{}

	root := &cobra.Command{
		Use:           "svg2drawio [flags] <input.svg|directory>...",
		Short:         "Convert SVG files to draw.io (mxGraph) XML",
		Long:          rootLong,
		Example:       rootExample,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return opts.runDiagram(cmd, args)
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.Version = versionString()
	opts.addFlags(root)

	root.AddCommand(
		newConvertCmd(),
		newVersionCmd(),
		newSkillCmd(),
	)
	return root
}

// Execute runs the root command, returning the process exit code.
func Execute() int {
	if err := NewRootCmd().Execute(); err != nil {
		return 1
	}
	return 0
}
