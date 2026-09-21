package convert

import (
	"math"
	"strings"

	"github.com/bcollard/svg2drawio/internal/mxgraph"
	"github.com/bcollard/svg2drawio/internal/svgdom"
)

// defaultFontSize is the SVG initial value for font-size.
const defaultFontSize float64 = 16

// textRun is one positioned piece of text, a <text> or a <tspan> that carries
// its own coordinates.
type textRun struct {
	text  string
	x, y  float64
	style svgdom.Style
}

// textCells converts a <text> element and its tspans into draw.io text cells.
func (c *converter) textCells(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	runs := c.collectRuns(n, st, 0, 0)
	cells := make([]*mxgraph.Cell, 0, len(runs))
	for _, run := range runs {
		if cell := c.textCell(run, m, parentID); cell != nil {
			cells = append(cells, cell)
		}
	}
	return cells
}

// collectRuns flattens a text element into positioned runs. A tspan without
// coordinates continues its parent's run.
func (c *converter) collectRuns(n *svgdom.Node, parent svgdom.Style, px, py float64) []textRun {
	st := c.doc.StyleOf(n).Inherit(parent)
	x := px
	y := py
	if v, ok := svgdom.Length(n.Attr("x")); ok {
		x = v
	}
	if v, ok := svgdom.Length(n.Attr("y")); ok {
		y = v
	}
	if v, ok := svgdom.Length(n.Attr("dx")); ok {
		x += v
	}
	if v, ok := svgdom.Length(n.Attr("dy")); ok {
		y += v
	}

	var runs []textRun
	own := normalizeText(n.Text)
	if own != "" {
		runs = append(runs, textRun{text: own, x: x, y: y, style: st})
	}
	for _, child := range n.Children {
		if child.Tag != "tspan" && child.Tag != "textPath" && child.Tag != "a" {
			continue
		}
		runs = append(runs, c.collectRuns(child, st, x, y)...)
	}
	return runs
}

func (c *converter) textCell(run textRun, m svgdom.Matrix, parentID string) *mxgraph.Cell {
	if strings.TrimSpace(run.text) == "" {
		return nil
	}

	fontSize := defaultFontSize
	if run.style.FontSize != nil {
		fontSize = *run.style.FontSize
	}
	sx, sy, rot, ok := decompose(m)
	if !ok {
		sx, sy, rot = m.ScaleFactor(), m.ScaleFactor(), 0
	}
	outSize := fontSize * sy

	lines := strings.Split(run.text, "\n")
	longest := 0
	for _, l := range lines {
		if len(l) > longest {
			longest = len(l)
		}
	}
	// draw.io lays text out itself, so the box only has to be large enough to
	// avoid wrapping; 0.62em per character is a safe average.
	w := math.Max(float64(longest)*fontSize*0.62*sx, 10)
	h := math.Max(float64(len(lines))*fontSize*1.25*sy, outSize)

	align := "left"
	switch run.style.TextAnchor {
	case "middle":
		align = "center"
	case "end":
		align = "right"
	}

	ox, oy := m.Apply(run.x, run.y)
	x := ox
	switch align {
	case "center":
		x = ox - w/2
	case "right":
		x = ox - w
	}
	// run.y is the baseline; the box is placed so the baseline lands inside it.
	y := oy - outSize*0.85 - (h-outSize)/2

	fill, _ := c.resolvePaint(run.style.Fill, "#000000")
	if fill == "none" {
		fill = "#000000"
	}

	style := &mxgraph.Style{}
	style.Set("text", "")
	style.Set("html", "1")
	style.Set("strokeColor", "none")
	style.Set("fillColor", "none")
	style.Set("align", align)
	style.Set("verticalAlign", "middle")
	style.Set("whiteSpace", "wrap")
	style.Set("rounded", "0")
	style.Set("fontColor", fill)
	style.SetFloat("fontSize", c.round(outSize))
	if run.style.FontFamily != "" {
		style.Set("fontFamily", sanitizeStyleValue(run.style.FontFamily))
	}
	if fs := fontStyleBits(run.style); fs != 0 {
		style.SetFloat("fontStyle", float64(fs))
	}
	if run.style.Opacity != nil && *run.style.Opacity < 1 {
		style.SetFloat("opacity", c.round(clamp01(*run.style.Opacity)*100))
	}
	if rot != 0 {
		style.SetFloat("rotation", c.round(rot))
	}

	return &mxgraph.Cell{
		ID:     c.id(),
		Value:  htmlEscape(run.text),
		Style:  style.String(),
		Vertex: true,
		Parent: parentID,
		Geometry: &mxgraph.Geometry{
			X: c.round(x), Y: c.round(y),
			W: c.round(w), H: c.round(h),
		},
	}
}

// fontStyleBits builds mxGraph's font style bitmask: 1 bold, 2 italic,
// 4 underline.
func fontStyleBits(st svgdom.Style) int {
	bits := 0
	if st.Bold() {
		bits |= 1
	}
	if st.Italic() {
		bits |= 2
	}
	if st.Underline() {
		bits |= 4
	}
	return bits
}

// normalizeText collapses SVG whitespace the way xml:space="default" does.
func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(collapseSpaces(l))
	}
	var kept []string
	for _, l := range lines {
		if l != "" {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, " ")
}

func collapseSpaces(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' {
			if !space {
				b.WriteRune(r)
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// htmlEscape protects the label from being read as markup, since the cells are
// written with html=1.
func htmlEscape(s string) string {
	s = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
	return strings.ReplaceAll(s, "\n", "<br>")
}

// sanitizeStyleValue removes the characters that would break style parsing.
func sanitizeStyleValue(s string) string {
	return strings.NewReplacer(";", " ", "=", " ").Replace(s)
}
