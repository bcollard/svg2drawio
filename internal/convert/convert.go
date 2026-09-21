// Package convert turns an SVG document into a draw.io diagram of native
// mxCells.
package convert

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/bcollard/svg2drawio/internal/mxgraph"
	"github.com/bcollard/svg2drawio/internal/stencil"
	"github.com/bcollard/svg2drawio/internal/svgdom"
	"github.com/bcollard/svg2drawio/internal/svgpath"
)

// Options controls the conversion.
type Options struct {
	// Decimals is the number of decimals coordinates are rounded to; -1 keeps
	// full precision.
	Decimals int
	// Scale multiplies every coordinate.
	Scale float64
	// Groups keeps SVG <g> elements as draw.io groups.
	Groups bool
	// Edges maps <line> and open <polyline> to draw.io edges instead of
	// shapes, so their endpoints stay draggable.
	Edges bool
	// PageName is the name of the generated diagram page.
	PageName string
}

// DefaultOptions returns the conversion defaults.
func DefaultOptions() Options {
	return Options{Decimals: 2, Scale: 1, Groups: true, Edges: true}
}

// converter holds the state of one document conversion.
type converter struct {
	doc  *svgdom.Document
	opt  Options
	grad *gradients
	seq  int
}

// Diagram converts an SVG stream into a .drawio file.
func Diagram(r io.Reader, name string, opt Options) (*mxgraph.File, error) {
	c, root, err := newConverter(r, opt)
	if err != nil {
		return nil, err
	}
	w, h, m := c.viewport(root)

	cells := c.walk(root, m, c.rootStyle(root), "1")

	pageName := opt.PageName
	if pageName == "" {
		pageName = displayName(name)
	}
	return &mxgraph.File{
		Models: []*mxgraph.Model{{
			Name:       pageName,
			PageWidth:  math.Max(w, 850),
			PageHeight: math.Max(h, 1100),
			Cells:      cells,
		}},
	}, nil
}

func newConverter(r io.Reader, opt Options) (*converter, *svgdom.Node, error) {
	doc, err := svgdom.Parse(r)
	if err != nil {
		return nil, nil, err
	}
	doc.Resolve()
	if opt.Scale == 0 {
		opt.Scale = 1
	}
	return &converter{doc: doc, opt: opt, grad: newGradients(doc)}, doc.Root, nil
}

// rootStyle seeds the inheritance chain with the SVG defaults draw.io needs to
// see explicitly: black fill, no stroke.
func (c *converter) rootStyle(root *svgdom.Node) svgdom.Style {
	s := c.doc.StyleOf(root)
	if s.Fill == "" {
		s.Fill = "#000000"
	}
	if s.Stroke == "" {
		s.Stroke = "none"
	}
	return s
}

// viewport returns the output size and the matrix mapping SVG user units onto
// draw.io coordinates.
func (c *converter) viewport(root *svgdom.Node) (float64, float64, svgdom.Matrix) {
	vb := svgdom.Numbers(root.Attr("viewBox"))
	width, hasW := svgdom.Length(root.Attr("width"))
	height, hasH := svgdom.Length(root.Attr("height"))

	var vbX, vbY, vbW, vbH float64
	hasVB := len(vb) >= 4 && vb[2] > 0 && vb[3] > 0
	if hasVB {
		vbX, vbY, vbW, vbH = vb[0], vb[1], vb[2], vb[3]
	}

	switch {
	case hasVB && hasW && hasH:
		sx, sy := width/vbW, height/vbH
		m := svgdom.Matrix{A: sx * c.opt.Scale, D: sy * c.opt.Scale}
		m = m.Mul(svgdom.Matrix{A: 1, D: 1, E: -vbX, F: -vbY})
		return width * c.opt.Scale, height * c.opt.Scale, m
	case hasVB:
		s := c.opt.Scale
		m := svgdom.Matrix{A: s, D: s}.Mul(svgdom.Matrix{A: 1, D: 1, E: -vbX, F: -vbY})
		return vbW * s, vbH * s, m
	case hasW && hasH:
		s := c.opt.Scale
		return width * s, height * s, svgdom.Matrix{A: s, D: s}
	}

	s := c.opt.Scale
	return 0, 0, svgdom.Matrix{A: s, D: s}
}

// nonRendering lists the elements whose content is never drawn in place.
var nonRendering = map[string]bool{
	"defs": true, "symbol": true, "marker": true, "clipPath": true,
	"mask": true, "pattern": true, "style": true, "title": true,
	"desc": true, "metadata": true, "filter": true, "script": true,
	"linearGradient": true, "radialGradient": true, "animate": true,
	"animateTransform": true, "animateMotion": true, "foreignObject": true,
}

// walk converts the children of n into cells, in document order so that the
// z-order of the SVG is preserved.
func (c *converter) walk(n *svgdom.Node, ctm svgdom.Matrix, parent svgdom.Style, parentID string) []*mxgraph.Cell {
	var out []*mxgraph.Cell

	for _, child := range n.Children {
		if nonRendering[child.Tag] {
			continue
		}
		st := c.doc.StyleOf(child).Inherit(parent)
		if st.Hidden() {
			continue
		}
		m := ctm.Mul(svgdom.ParseTransform(child.Attr("transform")))

		switch child.Tag {
		case "g", "a", "switch", "svg":
			inner := c.walk(child, m, st, parentID)
			if len(inner) == 0 {
				continue
			}
			if c.opt.Groups && len(inner) > 1 {
				out = append(out, c.group(child, inner, parentID))
				continue
			}
			out = append(out, inner...)
		default:
			out = append(out, c.shapeCells(child, m, st, parentID)...)
		}
	}
	return out
}

// group wraps cells in a draw.io group, rebasing their geometry on the group
// origin as the model expects.
func (c *converter) group(n *svgdom.Node, children []*mxgraph.Cell, parentID string) *mxgraph.Cell {
	box := cellsBounds(children)
	id := c.id()

	for _, child := range children {
		child.Parent = id
		shiftCell(child, -box.X, -box.Y)
	}

	style := &mxgraph.Style{}
	style.Set("group", "")
	return &mxgraph.Cell{
		ID:       id,
		Value:    n.Attr("id"),
		Style:    style.String(),
		Vertex:   true,
		Parent:   parentID,
		Geometry: &mxgraph.Geometry{X: box.X, Y: box.Y, W: box.W, H: box.H},
		Children: children,
	}
}

func cellsBounds(cells []*mxgraph.Cell) svgpath.Rect {
	var box svgpath.Rect
	first := true
	for _, c := range cells {
		g := c.Geometry
		if g == nil {
			continue
		}
		r := svgpath.Rect{X: g.X, Y: g.Y, W: g.W, H: g.H}
		if g.Source != nil || g.Target != nil || len(g.Points) > 0 {
			r = edgeBounds(g)
		}
		if first {
			box, first = r, false
			continue
		}
		box = box.Union(r)
	}
	return box
}

func edgeBounds(g *mxgraph.Geometry) svgpath.Rect {
	pts := append([]mxgraph.Point{}, g.Points...)
	if g.Source != nil {
		pts = append(pts, *g.Source)
	}
	if g.Target != nil {
		pts = append(pts, *g.Target)
	}
	if len(pts) == 0 {
		return svgpath.Rect{}
	}
	minX, minY := pts[0].X, pts[0].Y
	maxX, maxY := minX, minY
	for _, p := range pts[1:] {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	return svgpath.Rect{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}
}

// shiftCell moves a cell's own geometry; nested children are already relative
// to it and stay untouched.
func shiftCell(c *mxgraph.Cell, dx, dy float64) {
	g := c.Geometry
	if g == nil {
		return
	}
	if g.Source != nil || g.Target != nil || len(g.Points) > 0 {
		if g.Source != nil {
			g.Source.X += dx
			g.Source.Y += dy
		}
		if g.Target != nil {
			g.Target.X += dx
			g.Target.Y += dy
		}
		for i := range g.Points {
			g.Points[i].X += dx
			g.Points[i].Y += dy
		}
		return
	}
	g.X += dx
	g.Y += dy
}

func (c *converter) id() string {
	c.seq++
	return fmt.Sprintf("s2x-%d", c.seq)
}

func (c *converter) round(v float64) float64 { return stencil.Round(v, c.opt.Decimals) }

func displayName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "svg"
	}
	return base
}
