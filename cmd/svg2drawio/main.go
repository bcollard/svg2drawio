// Command svg2drawio converts SVG files into draw.io (mxGraph) XML.
package main

import (
	"os"

	"github.com/bcollard/svg2drawio"
	"github.com/bcollard/svg2drawio/internal/cli"
)

// Injected at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetBuildInfo(version, commit, date)
	cli.SetSkill(svg2drawio.SkillMD)
	os.Exit(cli.Execute())
}
