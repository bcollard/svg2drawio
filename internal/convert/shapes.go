package convert

import (
	"math"
	"strings"

	"github.com/bcollard/svg2drawio/internal/mxgraph"
	"github.com/bcollard/svg2drawio/internal/stencil"
	"github.com/bcollard/svg2drawio/internal/svgdom"
	"github.com/bcollard/svg2drawio/internal/svgpath"
)

// shapeCells converts one drawable SVG element into cells.
func (c *converter) shapeCells(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	switch n.Tag {
	case "rect":
		return c.rectCell(n, m, st, parentID)
	case "circle", "ellipse":
		return c.ellipseCell(n, m, st, parentID)
	case "line":
		return c.lineCell(n, m, st, parentID)
	case "polyline", "polygon":
		return c.polyCell(n, m, st, parentID)
	case "path":
		p := svgpath.Parse(n.Attr("d"))
		if len(p) == 0 {
			return nil
		}
		if cell := c.pathAsEdge(p, n, m, st, parentID); cell != nil {
			return []*mxgraph.Cell{cell}
		}
		return c.cellsFromPath(p, n, m, st, parentID)
	case "text":
		return c.textCells(n, m, st, parentID)
	case "image":
		return c.imageCell(n, m, st, parentID)
	}
	return nil
}

// rectCell keeps a rectangle as a native draw.io rectangle whenever the
// transform is a rotation and scale; skews and flips fall back to a stencil.
func (c *converter) rectCell(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	x := svgdom.LengthOr(n.Attr("x"), 0)
	y := svgdom.LengthOr(n.Attr("y"), 0)
	w := svgdom.LengthOr(n.Attr("width"), 0)
	h := svgdom.LengthOr(n.Attr("height"), 0)
	if w <= 0 || h <= 0 {
		return nil
	}
	rx := svgdom.LengthOr(n.Attr("rx"), -1)
	ry := svgdom.LengthOr(n.Attr("ry"), -1)
	if rx < 0 {
		rx = ry
	}
	if ry < 0 {
		ry = rx
	}
	if rx < 0 {
		rx, ry = 0, 0
	}

	sx, sy, rot, ok := decompose(m)
	if !ok {
		return c.cellsFromPath(svgpath.FromRect(x, y, w, h, rx, ry), n, m, st, parentID)
	}

	p := c.paintOf(st, m)
	style := &mxgraph.Style{}
	style.Set("rounded", "0")
	style.Set("whiteSpace", "wrap")
	style.Set("html", "1")
	if rx > 0 || ry > 0 {
		style.Set("rounded", "1")
		style.Set("absoluteArcSize", "1")
		style.SetFloat("arcSize", c.round(math.Max(rx*sx, ry*sy)))
	}
	c.applyPaint(style, p)
	if rot != 0 {
		style.SetFloat("rotation", c.round(rot))
	}

	cx, cy := m.Apply(x+w/2, y+h/2)
	ow, oh := w*sx, h*sy
	return []*mxgraph.Cell{{
		ID:     c.id(),
		Style:  style.String(),
		Vertex: true,
		Parent: parentID,
		Geometry: &mxgraph.Geometry{
			X: c.round(cx - ow/2), Y: c.round(cy - oh/2),
			W: c.round(ow), H: c.round(oh),
		},
	}}
}

// ellipseCell keeps circles and ellipses native under rotation and scaling.
func (c *converter) ellipseCell(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	cx := svgdom.LengthOr(n.Attr("cx"), 0)
	cy := svgdom.LengthOr(n.Attr("cy"), 0)

	var rx, ry float64
	if n.Tag == "circle" {
		r := svgdom.LengthOr(n.Attr("r"), 0)
		rx, ry = r, r
	} else {
		rx = svgdom.LengthOr(n.Attr("rx"), 0)
		ry = svgdom.LengthOr(n.Attr("ry"), 0)
	}
	if rx <= 0 || ry <= 0 {
		return nil
	}

	sx, sy, rot, ok := decompose(m)
	if !ok {
		return c.cellsFromPath(svgpath.FromEllipse(cx, cy, rx, ry), n, m, st, parentID)
	}

	p := c.paintOf(st, m)
	style := &mxgraph.Style{}
	style.Set("ellipse", "")
	style.Set("whiteSpace", "wrap")
	style.Set("html", "1")
	c.applyPaint(style, p)
	if rot != 0 {
		style.SetFloat("rotation", c.round(rot))
	}

	ocx, ocy := m.Apply(cx, cy)
	ow, oh := 2*rx*sx, 2*ry*sy
	return []*mxgraph.Cell{{
		ID:     c.id(),
		Style:  style.String(),
		Vertex: true,
		Parent: parentID,
		Geometry: &mxgraph.Geometry{
			X: c.round(ocx - ow/2), Y: c.round(ocy - oh/2),
			W: c.round(ow), H: c.round(oh),
		},
	}}
}

// lineCell maps <line> to a draw.io edge so both endpoints stay draggable.
func (c *converter) lineCell(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	x1 := svgdom.LengthOr(n.Attr("x1"), 0)
	y1 := svgdom.LengthOr(n.Attr("y1"), 0)
	x2 := svgdom.LengthOr(n.Attr("x2"), 0)
	y2 := svgdom.LengthOr(n.Attr("y2"), 0)

	if !c.opt.Edges {
		return c.cellsFromPath(svgpath.FromLine(x1, y1, x2, y2), n, m, st, parentID)
	}
	ax, ay := m.Apply(x1, y1)
	bx, by := m.Apply(x2, y2)
	return []*mxgraph.Cell{c.edgeCell([]mxgraph.Point{{X: ax, Y: ay}, {X: bx, Y: by}}, n, st, m, parentID)}
}

// polyCell maps an open polyline to an edge and everything else to a shape.
func (c *converter) polyCell(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	pts := svgdom.Numbers(n.Attr("points"))
	if len(pts) < 4 {
		return nil
	}
	closed := n.Tag == "polygon"
	p := c.paintOf(st, m)

	if !closed && !p.hasFill() && c.opt.Edges {
		out := make([]mxgraph.Point, 0, len(pts)/2)
		for i := 0; i+1 < len(pts); i += 2 {
			x, y := m.Apply(pts[i], pts[i+1])
			out = append(out, mxgraph.Point{X: x, Y: y})
		}
		return []*mxgraph.Cell{c.edgeCell(out, n, st, m, parentID)}
	}
	return c.cellsFromPath(svgpath.FromPoints(pts, closed), n, m, st, parentID)
}

// pathAsEdge maps a stroked, unfilled path made only of straight segments to a
// draw.io edge, which is how connectors stay editable as connectors.
func (c *converter) pathAsEdge(p svgpath.Path, n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) *mxgraph.Cell {
	if !c.opt.Edges {
		return nil
	}
	if pi := c.paintOf(st, m); pi.hasFill() || !pi.hasStroke() {
		return nil
	}
	pts, ok := straightPoints(p)
	if !ok {
		return nil
	}
	out := make([]mxgraph.Point, 0, len(pts))
	for _, pt := range pts {
		x, y := m.Apply(pt.X, pt.Y)
		out = append(out, mxgraph.Point{X: x, Y: y})
	}
	return c.edgeCell(out, n, st, m, parentID)
}

// straightPoints returns the vertices of a single open polyline, or false when
// the path curves, closes or has several subpaths.
func straightPoints(p svgpath.Path) ([]svgpath.Point, bool) {
	if len(p) < 2 || p[0].Op != svgpath.OpMove {
		return nil, false
	}
	pts := []svgpath.Point{{X: p[0].Args[0], Y: p[0].Args[1]}}
	for _, seg := range p[1:] {
		if seg.Op != svgpath.OpLine {
			return nil, false
		}
		pts = append(pts, svgpath.Point{X: seg.Args[0], Y: seg.Args[1]})
	}
	return pts, true
}

// edgeCell builds an edge through the given absolute points.
func (c *converter) edgeCell(pts []mxgraph.Point, n *svgdom.Node, st svgdom.Style, m svgdom.Matrix, parentID string) *mxgraph.Cell {
	p := c.paintOf(st, m)
	// An unstroked line still has to be visible as an edge.
	if !p.hasStroke() {
		p.stroke = "#000000"
	}

	style := &mxgraph.Style{}
	style.Set("endArrow", "none")
	style.Set("html", "1")
	style.Set("rounded", "0")
	applyMarkers(style, n)
	style.Set("strokeColor", p.stroke)
	style.SetFloat("strokeWidth", c.round(p.strokeW))
	if p.opacity < 1 || p.strokeOpacity < 1 {
		style.SetFloat("opacity", c.round(p.opacity*p.strokeOpacity*100))
	}
	if p.dash != "" {
		style.Set("dashed", "1")
		style.Set("dashPattern", p.dash)
	}

	for i := range pts {
		pts[i].X = c.round(pts[i].X)
		pts[i].Y = c.round(pts[i].Y)
	}
	src, dst := pts[0], pts[len(pts)-1]
	var way []mxgraph.Point
	if len(pts) > 2 {
		way = pts[1 : len(pts)-1]
	}

	return &mxgraph.Cell{
		ID:     c.id(),
		Style:  style.String(),
		Edge:   true,
		Parent: parentID,
		Geometry: &mxgraph.Geometry{
			Relative: true,
			Source:   &src,
			Target:   &dst,
			Points:   way,
		},
	}
}

// cellsFromPath emits a vertex whose shape is an inline stencil holding the
// transformed path, which is what makes arbitrary SVG geometry editable in
// draw.io.
func (c *converter) cellsFromPath(p svgpath.Path, n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	if len(p) == 0 {
		return nil
	}
	tp := p.Transform(m.Apply)
	box := tp.Bounds()
	pi := c.paintOf(st, m)

	minDim := math.Max(1, pi.strokeW)
	w, h := box.W, box.H
	offX, offY := -box.X, -box.Y
	if w < minDim {
		offX += (minDim - w) / 2
		w = minDim
	}
	if h < minDim {
		offY += (minDim - h) / 2
		h = minDim
	}

	sh := &stencil.Shape{
		Name: stencilName(n),
		W:    c.round(w),
		H:    c.round(h),
	}
	if pi.lineJoin != "" {
		sh.Foreground = append(sh.Foreground, stencil.Set("linejoin", "join", pi.lineJoin))
	}
	if pi.lineCap != "" {
		sh.Foreground = append(sh.Foreground, stencil.Set("linecap", "cap", pi.lineCap))
	}
	sh.Foreground = append(sh.Foreground, stencil.PathElem(tp.Translate(offX, offY), c.opt.Decimals))
	if fs, ok := stencil.FillStroke(pi.hasFill(), pi.hasStroke()); ok {
		sh.Foreground = append(sh.Foreground, fs)
	}

	shapeValue, err := sh.StyleValue()
	if err != nil {
		return nil
	}

	style := &mxgraph.Style{}
	style.Set("shape", shapeValue)
	style.Set("html", "1")
	style.Set("whiteSpace", "wrap")
	c.applyPaint(style, pi)

	return []*mxgraph.Cell{{
		ID:     c.id(),
		Style:  style.String(),
		Vertex: true,
		Parent: parentID,
		// The stencil origin sits at (-offX, -offY) in absolute coordinates.
		Geometry: &mxgraph.Geometry{
			X: c.round(-offX),
			Y: c.round(-offY),
			W: c.round(w),
			H: c.round(h),
		},
	}}
}

// imageCell maps <image> to draw.io's native image shape.
func (c *converter) imageCell(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style, parentID string) []*mxgraph.Cell {
	href := svgdom.Href(n)
	if href == "" {
		return nil
	}
	x := svgdom.LengthOr(n.Attr("x"), 0)
	y := svgdom.LengthOr(n.Attr("y"), 0)
	w := svgdom.LengthOr(n.Attr("width"), 0)
	h := svgdom.LengthOr(n.Attr("height"), 0)
	if w <= 0 || h <= 0 {
		return nil
	}

	sx, sy, rot, ok := decompose(m)
	if !ok {
		sx, sy, rot = m.ScaleFactor(), m.ScaleFactor(), 0
	}

	style := &mxgraph.Style{}
	style.Set("shape", "image")
	style.Set("html", "1")
	style.Set("verticalLabelPosition", "bottom")
	style.Set("verticalAlign", "top")
	style.Set("imageAspect", "0")
	style.Set("image", styleDataURI(href))
	if rot != 0 {
		style.SetFloat("rotation", c.round(rot))
	}

	ox, oy := m.Apply(x, y)
	return []*mxgraph.Cell{{
		ID:     c.id(),
		Style:  style.String(),
		Vertex: true,
		Parent: parentID,
		Geometry: &mxgraph.Geometry{
			X: c.round(ox), Y: c.round(oy),
			W: c.round(w * sx), H: c.round(h * sy),
		},
	}}
}

// styleDataURI mirrors draw.io's convertDataUri: the ";base64" segment is
// dropped because a semicolon would end the style entry.
func styleDataURI(uri string) string {
	if !strings.HasPrefix(uri, "data:") {
		return uri
	}
	semi := strings.IndexByte(uri, ';')
	if semi <= 0 {
		return uri
	}
	comma := strings.IndexByte(uri[semi+1:], ',')
	if comma < 0 {
		return uri
	}
	return uri[:semi] + uri[semi+1+comma:]
}

// applyMarkers turns SVG marker references into draw.io arrowheads. The marker
// geometry itself is not reproduced; draw.io's block arrow is the closest
// native equivalent and stays editable.
func applyMarkers(style *mxgraph.Style, n *svgdom.Node) {
	if n == nil {
		return
	}
	if hasMarker(n, "marker-end") {
		style.Set("endArrow", "block")
		style.Set("endFill", "1")
	}
	if hasMarker(n, "marker-start") {
		style.Set("startArrow", "block")
		style.Set("startFill", "1")
	}
}

func hasMarker(n *svgdom.Node, attr string) bool {
	v := strings.TrimSpace(n.Attr(attr))
	if v == "" {
		v = strings.TrimSpace(styleProp(n, attr))
	}
	return v != "" && v != "none"
}

func stencilName(n *svgdom.Node) string {
	if id := n.Attr("id"); id != "" {
		return id
	}
	return n.Tag
}

// decompose splits a matrix into scale and rotation. It reports false when the
// matrix skews or mirrors, in which case the geometry has to be baked into a
// path instead.
func decompose(m svgdom.Matrix) (sx, sy, rotDeg float64, ok bool) {
	det := m.A*m.D - m.B*m.C
	if det <= 1e-12 {
		return 0, 0, 0, false
	}
	sx = math.Hypot(m.A, m.B)
	if sx < 1e-12 {
		return 0, 0, 0, false
	}
	if shear := (m.A*m.C + m.B*m.D) / (sx * sx); math.Abs(shear) > 1e-9 {
		return 0, 0, 0, false
	}
	sy = det / sx
	rotDeg = math.Atan2(m.B, m.A) * 180 / math.Pi
	if math.Abs(rotDeg) < 1e-9 {
		rotDeg = 0
	}
	return sx, sy, rotDeg, true
}
