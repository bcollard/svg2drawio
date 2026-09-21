# svg2drawio

Go CLI that converts SVG files into a `.drawio` diagram of native draw.io
cells.

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/svg2drawio/` | `main`, build-info vars stamped by ldflags |
| `internal/cli/` | cobra commands: root/convert/version/skill |
| `internal/svgdom/` | SVG parse, `use`/`symbol` expansion, transforms, colours, CSS cascade |
| `internal/svgpath/` | path `d` parsing, arcs → cubics, transforms, exact bounds |
| `internal/stencil/` | inline stencil XML + the `shape=stencil(...)` encoding |
| `internal/mxgraph/` | `.drawio` file model (mxfile / mxGraphModel / mxCell) |
| `internal/convert/` | the SVG → draw.io mapping |
| `skill.go` + `SKILL.md` | the Agent Skill, embedded into the binary |
| `website/` | static site for svg2drawio.runlocal.dev |

## Rules that matter

- **The stencil encoding is not negotiable.** draw.io decodes
  `shape=stencil(<data>)` with `Graph.decompress`: base64 → raw inflate →
  `decodeURIComponent`. `internal/stencil.Compress` implements exactly that,
  and its round-trip test guards it. Verified against the drawio source
  (`js/grapheditor/Graph.js`, `mxgraph/src/shape/mxStencil.js`).
- **Native first, stencil second.** A shape only becomes an inline stencil when
  draw.io has no native equivalent. Rect/ellipse survive rotation and scaling
  (`decompose` in `internal/convert/shapes.go`); skew and mirroring fall back to
  a baked path.
- **Style resolution goes through the document**, not the node: use
  `doc.StyleOf(n)` so `<style>` rules and `class` attributes are applied in
  cascade order. `svgdom.StyleOf` alone skips the stylesheet.
- **Data URIs in styles drop `;base64`** — a semicolon would end the style
  entry. draw.io does the same in `EditorUi.convertDataUri`.
- **Coordinates are baked.** Stencils have no transform; every transform is
  applied to the points before output.

## Checks

`make check` = gofmt + go vet + go test. `make snapshot` dry-runs the release.
The drawio source clone at `../drawio` is the reference for anything about
draw.io's own behaviour — read it rather than guessing.
