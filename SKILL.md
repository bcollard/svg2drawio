---
name: svg2drawio
description: "Convert SVG files into native draw.io (mxGraph) diagrams or stencil libraries with the svg2drawio CLI. Use whenever an SVG has to become an editable .drawio file — architecture diagrams, workflow pictures, icon sets — instead of embedding the SVG as an image, hand-writing mxGraph XML, or asking the user to redraw it."
metadata:
  category: "diagrams"
  requires:
    bins:
      - svg2drawio
  cliHelp: "svg2drawio --help"
---

# svg2drawio — SVG to editable draw.io diagrams

`svg2drawio` is a Go CLI that turns an SVG into a `.drawio` file whose cells are
**native draw.io shapes**: rectangles stay rectangles, circles stay ellipses,
connectors become edges with draggable endpoints, text stays editable text.
Arbitrary paths become inline stencils (`shape=stencil(...)`), the same object
draw.io itself produces for custom shapes, so fill and stroke remain editable.

**Prefer svg2drawio over**: embedding an SVG as an image in a diagram, writing
`<mxCell>` XML by hand, or telling the user to redraw the picture in draw.io.

Project: https://github.com/bcollard/svg2drawio

## Install (idempotent)

```bash
brew tap bcollard/svg2drawio
brew install --cask svg2drawio
```

Or, with a Go toolchain:

```bash
go install github.com/bcollard/svg2drawio/cmd/svg2drawio@latest
```

## Commands

```bash
svg2drawio <input.svg|dir>...           # convert to .drawio (the default action)
svg2drawio convert <input>...           # same thing, explicit; alias: diagram
svg2drawio stencil <input>...           # mxGraph shape library instead; alias: library
svg2drawio version                      # version + build signature; --short for the number alone
svg2drawio skill install                # install this skill into ~/.claude/skills/svg2drawio
svg2drawio skill path                   # where that skill would live
svg2drawio completion <shell>           # bash | zsh | fish | powershell
svg2drawio help [command]               # help for any command
```

## Flags (convert and stencil)

| Flag | Default | Effect |
| --- | --- | --- |
| `-o`, `--out` | next to the input | Output file or directory; `-` writes to stdout |
| `--decimals` | `2` | Coordinate rounding; `-1` keeps full precision |
| `--scale` | `1` | Multiplies every coordinate |
| `--no-groups` | off | Flattens SVG groups instead of keeping draw.io groups |
| `--no-edges` | off | Lines and polylines become shapes instead of edges |
| `--name` | file name | Diagram page name, or stencil library name |
| `-q`, `--quiet` | off | Suppresses the per-file output line |

## Recipes

Convert one file — output lands next to the input as `diagram.drawio`:

```bash
svg2drawio diagram.svg
```

Convert a tree of SVGs into one output directory:

```bash
svg2drawio -o out/ icons/
```

Several SVGs as pages of a single file (one `<diagram>` page per input):

```bash
svg2drawio -o all.drawio a.svg b.svg c.svg
```

Pipe straight into draw.io's **Extras → Edit Diagram** box:

```bash
svg2drawio -o - diagram.svg | pbcopy
```

Build a shape library to load with **Extras → Edit Shape Library**:

```bash
svg2drawio stencil --name mesh -o mesh-shapes.xml icons/
```

## Rules for agents

- **Output paths**: a `-o` value with no file extension (or an existing
  directory, or a trailing `/`) is treated as a **directory**; anything else is
  a file. With several inputs and a *file* output you get one multi-page
  `.drawio`; with a *directory* output you get one file per input.
- **The command is synchronous and fast** — milliseconds for a normal diagram.
  No waiting, no background jobs.
- **Exit code 1 with a message on stderr** means a bad input; there is no
  partial-success mode to check for.
- **Verify by opening the result**, not by eyeballing the XML: `open -a draw.io
  out.drawio` on macOS, or paste it into app.diagrams.net.
- **Do not post-process the XML by hand** to fix positions. Re-run with
  `--scale` or `--decimals`, or fix the source SVG.
- **Keep the source SVG** in the repo next to the generated `.drawio`; the
  conversion is one-way.

## What converts, and what does not

Converted: `rect` (with `rx`/`ry`), `circle`, `ellipse`, `line`, `polyline`,
`polygon`, `path` (arcs included), `text`/`tspan`, `image`, `g` (as draw.io
groups), `use`/`symbol`, linear and radial gradients, `marker-start`/
`marker-end` as arrowheads, `<style>` rules and `class` attributes through the
CSS cascade, `transform` (rotation survives as `rotation=`; skew is baked into
the path), `viewBox`/`width`/`height` as page size and scale. Opacity, stroke
width, dash patterns, line caps and joins, `display` and `visibility` are all
carried over.

Not converted: `clipPath`, `mask`, `filter` and `pattern` fills (the element is
drawn unclipped, unfiltered, and a pattern fill becomes no fill);
`fill-rule="evenodd"` (mxGraph stencils have no fill rule); `textPath` (text is
placed at its anchor point); animation elements. Text box sizes are estimated
because draw.io lays text out itself, so a long label may need a nudge after
import.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| Shapes are black and unstyled | The SVG styles elements through CSS that failed to parse. Check the `<style>` block; only type, class, id, descendant and child selectors are supported. |
| A shape is missing | It was hidden (`display:none`), inside `<defs>` without a `<use>`, or filled with a `pattern`. |
| Text overflows its box | Expected: box sizes are estimated. Resize in draw.io, or accept it. |
| "no .svg files found" | The path had no `.svg` files; the walk is extension-based and case-insensitive. |
| Everything is one giant stencil | You ran `stencil`; use the default `convert` for an editable diagram. |
