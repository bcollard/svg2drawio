package convert

import (
	"github.com/bcollard/svg2drawio/internal/stencil"
	"github.com/bcollard/svg2drawio/internal/svgdom"
	"github.com/bcollard/svg2drawio/internal/svgpath"
)

// stencilElems renders the whole drawing into the element list of a single
// stencil, which is the shape-library form the original Java tool produced.
// Each element is wrapped in save/restore so its style cannot leak into the
// next one.
func (c *converter) stencilElems(n *svgdom.Node, ctm svgdom.Matrix, parent svgdom.Style) []stencil.Elem {
	var out []stencil.Elem

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
			out = append(out, c.stencilElems(child, m, st)...)
		case "text":
			out = append(out, c.stencilText(child, m, st)...)
		case "image":
			out = append(out, c.stencilImage(child, m)...)
		default:
			out = append(out, c.stencilGeometry(child, m, st)...)
		}
	}
	return out
}

func (c *converter) stencilGeometry(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style) []stencil.Elem {
	p := pathOf(n)
	if len(p) == 0 {
		return nil
	}
	pi := c.paintOf(st, m)
	if !pi.hasFill() && !pi.hasStroke() {
		return nil
	}

	body := c.stencilPaintSetters(pi)
	body = append(body, stencil.PathElem(p.Transform(m.Apply), c.opt.Decimals))
	if fs, ok := stencil.FillStroke(pi.hasFill(), pi.hasStroke()); ok {
		body = append(body, fs)
	}
	return wrapSaveRestore(body)
}

// pathOf turns any geometric SVG element into path data.
func pathOf(n *svgdom.Node) svgpath.Path {
	switch n.Tag {
	case "path":
		return svgpath.Parse(n.Attr("d"))
	case "rect":
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
		return svgpath.FromRect(svgdom.LengthOr(n.Attr("x"), 0), svgdom.LengthOr(n.Attr("y"), 0), w, h, rx, ry)
	case "circle":
		r := svgdom.LengthOr(n.Attr("r"), 0)
		if r <= 0 {
			return nil
		}
		return svgpath.FromEllipse(svgdom.LengthOr(n.Attr("cx"), 0), svgdom.LengthOr(n.Attr("cy"), 0), r, r)
	case "ellipse":
		rx := svgdom.LengthOr(n.Attr("rx"), 0)
		ry := svgdom.LengthOr(n.Attr("ry"), 0)
		if rx <= 0 || ry <= 0 {
			return nil
		}
		return svgpath.FromEllipse(svgdom.LengthOr(n.Attr("cx"), 0), svgdom.LengthOr(n.Attr("cy"), 0), rx, ry)
	case "line":
		return svgpath.FromLine(
			svgdom.LengthOr(n.Attr("x1"), 0), svgdom.LengthOr(n.Attr("y1"), 0),
			svgdom.LengthOr(n.Attr("x2"), 0), svgdom.LengthOr(n.Attr("y2"), 0))
	case "polyline":
		return svgpath.FromPoints(svgdom.Numbers(n.Attr("points")), false)
	case "polygon":
		return svgpath.FromPoints(svgdom.Numbers(n.Attr("points")), true)
	}
	return nil
}

func (c *converter) stencilPaintSetters(pi paint) []stencil.Elem {
	var out []stencil.Elem
	if pi.hasFill() {
		out = append(out, stencil.Set("fillcolor", "color", pi.fill))
	}
	if pi.hasStroke() {
		out = append(out, stencil.Set("strokecolor", "color", pi.stroke))
		out = append(out, stencil.Set("strokewidth", "width", stencil.Num(c.round(pi.strokeW))))
	}
	if pi.opacity < 1 {
		out = append(out, stencil.Set("alpha", "alpha", stencil.Num(c.round(pi.opacity))))
	}
	if pi.fillOpacity < 1 {
		out = append(out, stencil.Set("fillalpha", "alpha", stencil.Num(c.round(pi.fillOpacity))))
	}
	if pi.strokeOpacity < 1 {
		out = append(out, stencil.Set("strokealpha", "alpha", stencil.Num(c.round(pi.strokeOpacity))))
	}
	if pi.dash != "" {
		out = append(out, stencil.Set("dashed", "dashed", "1"))
		out = append(out, stencil.Set("dashpattern", "pattern", pi.dash))
	}
	if pi.lineJoin != "" {
		out = append(out, stencil.Set("linejoin", "join", pi.lineJoin))
	}
	if pi.lineCap != "" {
		out = append(out, stencil.Set("linecap", "cap", pi.lineCap))
	}
	return out
}

func (c *converter) stencilText(n *svgdom.Node, m svgdom.Matrix, st svgdom.Style) []stencil.Elem {
	var out []stencil.Elem
	for _, run := range c.collectRuns(n, st, 0, 0) {
		if run.text == "" {
			continue
		}
		fontSize := defaultFontSize
		if run.style.FontSize != nil {
			fontSize = *run.style.FontSize
		}
		color, _ := c.resolvePaint(run.style.Fill, "#000000")
		if color == "none" {
			color = "#000000"
		}

		body := []stencil.Elem{
			stencil.Set("fontcolor", "color", color),
			stencil.Set("fontsize", "size", stencil.Num(c.round(fontSize*m.ScaleFactor()))),
		}
		if run.style.FontFamily != "" {
			body = append(body, stencil.Set("fontfamily", "family", run.style.FontFamily))
		}
		if bits := fontStyleBits(run.style); bits != 0 {
			body = append(body, stencil.Set("fontstyle", "style", stencil.Num(float64(bits))))
		}

		align := "left"
		switch run.style.TextAnchor {
		case "middle":
			align = "center"
		case "end":
			align = "right"
		}
		x, y := m.Apply(run.x, run.y)
		body = append(body, stencil.Text(run.text, c.round(x), c.round(y), align, "bottom", c.opt.Decimals))
		out = append(out, wrapSaveRestore(body)...)
	}
	return out
}

func (c *converter) stencilImage(n *svgdom.Node, m svgdom.Matrix) []stencil.Elem {
	href := svgdom.Href(n)
	w := svgdom.LengthOr(n.Attr("width"), 0)
	h := svgdom.LengthOr(n.Attr("height"), 0)
	if href == "" || w <= 0 || h <= 0 {
		return nil
	}
	x, y := m.Apply(svgdom.LengthOr(n.Attr("x"), 0), svgdom.LengthOr(n.Attr("y"), 0))
	sx, sy, _, ok := decompose(m)
	if !ok {
		sx, sy = m.ScaleFactor(), m.ScaleFactor()
	}
	return []stencil.Elem{stencil.Image(c.round(x), c.round(y), c.round(w*sx), c.round(h*sy), href, c.opt.Decimals)}
}

func wrapSaveRestore(body []stencil.Elem) []stencil.Elem {
	if len(body) == 0 {
		return nil
	}
	out := make([]stencil.Elem, 0, len(body)+2)
	out = append(out, stencil.Elem{Name: "save"})
	out = append(out, body...)
	out = append(out, stencil.Elem{Name: "restore"})
	return out
}
