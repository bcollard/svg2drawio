# svg2drawio

Convert SVG files into draw.io (mxGraph) XML from the command line.

The default output is a `.drawio` diagram in which every SVG element is a native
draw.io cell: rectangles are rectangles, circles are ellipses, connectors are
edges with draggable endpoints, text is a text cell with its own font settings.
Arbitrary paths become inline stencils (`shape=stencil(...)`), which draw.io
renders as first class shapes with editable fill and stroke.

The original [jgraph/svg2xml](https://github.com/jgraph/svg2xml) (Java, Swing
GUI) only produced mxGraph *stencil libraries*. That output is available here
too, under `svg2drawio stencil`.

Website: <https://svg2drawio.runlocal.dev>

## Install

```bash
brew tap bcollard/svg2drawio
brew install --cask svg2drawio
```

Or with a Go toolchain:

```bash
go install github.com/bcollard/svg2drawio/cmd/svg2drawio@latest
```

Or from a clone:

```bash
make build     # ./bin/svg2drawio
```

## Usage

```
svg2drawio [flags] <input.svg|directory>...
```

| Command | Purpose |
| --- | --- |
| `svg2drawio <input>...` | Convert to `.drawio` — the default action |
| `svg2drawio convert <input>...` | The same thing, explicit (alias: `diagram`) |
| `svg2drawio stencil <input>...` | An mxGraph shape library instead (alias: `library`) |
| `svg2drawio version` | Version and build signature; `--short` for the number alone |
| `svg2drawio skill install` | Install the Agent Skill for AI coding tools |
| `svg2drawio skill path` | Where that skill is installed |
| `svg2drawio completion <shell>` | bash, zsh, fish or powershell completions |
| `svg2drawio help [command]` | Help for any command |

Flags for `convert` and `stencil`:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-o`, `--out` | next to the input | Output file or directory; `-` writes to stdout |
| `--decimals` | `2` | Rounding applied to coordinates; `-1` keeps full precision |
| `--scale` | `1` | Multiplies every coordinate |
| `--no-groups` | off | Flattens SVG groups instead of keeping draw.io groups |
| `--no-edges` | off | Converts lines and polylines to shapes instead of edges |
| `--name` | file name | Name of the diagram page or the stencil library |
| `-q`, `--quiet` | off | Suppresses the per-file output line |

Examples:

```bash
svg2drawio diagram.svg                        # writes diagram.drawio next to it
svg2drawio -o out/ icons/                     # converts a tree of SVGs
svg2drawio -o all.drawio a.svg b.svg c.svg    # one file, one page per SVG
svg2drawio stencil -o icons.xml icons/        # a shape library to import
svg2drawio -o - diagram.svg | pbcopy          # paste into Extras > Edit Diagram
```

An `-o` value with no file extension (or an existing directory, or a trailing
`/`) is treated as a directory; anything else is a file.

## Agent Skill

The repo-root `SKILL.md` is embedded in the binary, so AI coding tools can learn
the CLI without a separate download:

```bash
svg2drawio skill install          # → ~/.claude/skills/svg2drawio/SKILL.md
svg2drawio skill install --print  # write it wherever you like
```

## What is converted

| SVG | draw.io |
| --- | --- |
| `rect` | native rectangle, `rounded=1;absoluteArcSize=1;arcSize=<rx>` when `rx`/`ry` are set |
| `circle`, `ellipse` | native ellipse |
| `line`, open `polyline`, straight open `path` | edge with floating endpoints and waypoints |
| `polygon`, curved or filled `path` | vertex with an inline stencil |
| `text`, `tspan` | text cell with font family, size, colour, weight, style and alignment |
| `image` | `shape=image`, data URIs rewritten the way draw.io stores them |
| `g` | draw.io group, with children rebased on the group origin |
| `use`, `symbol` | expanded before conversion |
| `linearGradient`, `radialGradient` | `fillColor` + `gradientColor` + `gradientDirection` |
| `marker-start`, `marker-end` | `startArrow` / `endArrow` block arrowheads |
| `<style>` rules, `class`, `style=""` | resolved through the CSS cascade before conversion |
| `transform` | baked into coordinates; rotations survive as `rotation=`, skews are baked into a stencil path |
| `viewBox`, `width`, `height` | page size and the user-unit scale |

Opacity, stroke width, dash patterns, line caps and joins, `display`,
`visibility` and style inheritance through groups are all carried over.

## Not converted

- `clipPath`, `mask`, `filter`, `pattern` fills — the element is drawn unclipped
  and unfiltered; a pattern fill becomes no fill.
- `fill-rule="evenodd"` — mxGraph stencils have no fill rule.
- `textPath`, text on a curve — the text is placed at its anchor point.
- Animation elements are ignored.
- Text box sizes are estimated from the font metrics, since draw.io lays text
  out itself. Long labels may need a nudge after import.

## How the inline stencils are encoded

draw.io reads `shape=stencil(<data>)` with `Graph.decompress`: base64 decode,
raw inflate, `decodeURIComponent`. This tool writes exactly that — the stencil
XML is URI encoded, deflated raw and base64 encoded — so the shapes are the same
kind of object draw.io produces itself for custom shapes.

## Development

```bash
make check     # gofmt, go vet, go test
make build     # ./bin/svg2drawio
make snapshot  # local goreleaser dry run
```

| Package | Responsibility |
| --- | --- |
| `internal/cli` | cobra commands, file walking, output paths |
| `internal/svgdom` | SVG parsing, `use`/`symbol` expansion, transforms, colours, the CSS cascade |
| `internal/svgpath` | path data parsing, arcs to cubics, transforms, exact bounding boxes |
| `internal/stencil` | mxGraph stencil XML and the draw.io style encoding |
| `internal/mxgraph` | the `.drawio` file model |
| `internal/convert` | the SVG to draw.io mapping |

`website/` holds the static site published to
[svg2drawio.runlocal.dev](https://svg2drawio.runlocal.dev); see
`website/README.md`.

## Releases

Tagging `vX.Y.Z` runs `.github/workflows/release.yml`: goreleaser builds macOS
and Linux binaries (amd64 + arm64) stamped with version, commit and build date,
attaches an SPDX SBOM and third-party licences, and pushes the Homebrew cask to
[bcollard/homebrew-svg2drawio](https://github.com/bcollard/homebrew-svg2drawio).

## License

MIT — see [LICENSE](LICENSE).
