package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// versionString is the one-line build signature: version, commit and build
// date as stamped by the release build.
func versionString() string {
	return fmt.Sprintf("%s (%s, %s)", buildVersion, buildCommit, buildDate)
}

func newVersionCmd() *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the svg2drawio version and build signature",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if short {
				fmt.Fprintln(out, buildVersion)
				return nil
			}
			fmt.Fprintf(out, "svg2drawio %s\n", versionString())
			fmt.Fprintf(out, "  go:       %s\n", runtime.Version())
			fmt.Fprintf(out, "  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "Print just the version number")
	return cmd
}
