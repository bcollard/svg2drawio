package convert

import (
	"math"
	"strings"

	"github.com/bcollard/svg2drawio/internal/mxgraph"
	"github.com/bcollard/svg2drawio/internal/svgdom"
)

// paint is the resolved painting state of one element.
type paint struct {
	fill     string // "none" or "#rrggbb"
	fillGrad *gradient
	stroke   string
	strokeW  float64

	opacity       float64 // 0..1
	fillOpacity   float64 // 0..1
	strokeOpacity float64 // 0..1

	dash     string
	lineCap  string
	lineJoin string
}

func (p paint) hasFill() bool   { return p.fill != "" && p.fill != "none" }
func (p paint) hasStroke() bool { return p.stroke != "" && p.stroke != "none" }

// paintOf resolves colours, widths and opacities for an element under the
// given transform.
func (c *converter) paintOf(st svgdom.Style, m svgdom.Matrix) paint {
	p := paint{opacity: 1, fillOpacity: 1, strokeOpacity: 1, strokeW: 1}

	p.fill, p.fillGrad = c.resolvePaint(st.Fill, "#000000")
	p.stroke, _ = c.resolvePaint(st.Stroke, "none")

	scale := m.ScaleFactor()
	if st.StrokeWidth != nil {
		p.strokeW = *st.StrokeWidth
	}
	p.strokeW *= scale
	if p.strokeW <= 0 {
		p.stroke = "none"
	}

	if st.Opacity != nil {
		p.opacity = clamp01(*st.Opacity)
	}
	if st.FillOpacity != nil {
		p.fillOpacity = clamp01(*st.FillOpacity)
	}
	if st.StrokeOpa != nil {
		p.strokeOpacity = clamp01(*st.StrokeOpa)
	}

	if st.DashArray != "" {
		p.dash = svgdom.DashPattern(st.DashArray, scale/math.Max(p.strokeW, 0.01))
	}
	p.lineCap = st.LineCap
	p.lineJoin = st.LineJoin
	return p
}

// resolvePaint turns an SVG paint value into a draw.io colour. Gradients are
// returned alongside so the caller can keep them where draw.io supports them.
func (c *converter) resolvePaint(value, fallback string) (string, *gradient) {
	v := strings.TrimSpace(value)
	if v == "" {
		return fallback, nil
	}
	if strings.HasPrefix(v, "url(") {
		g := c.grad.lookup(v)
		if g == nil {
			return "none", nil
		}
		return g.start(), g
	}
	if col, ok := svgdom.NormalizeColor(v); ok {
		return col, nil
	}
	if v == "currentcolor" || v == "currentColor" {
		return "#000000", nil
	}
	return fallback, nil
}

// applyPaint writes the paint onto a cell style.
func (c *converter) applyPaint(s *mxgraph.Style, p paint) {
	if p.hasFill() {
		s.Set("fillColor", p.fill)
		if p.fillGrad != nil && p.fillGrad.end() != p.fillGrad.start() {
			s.Set("gradientColor", p.fillGrad.end())
			s.Set("gradientDirection", p.fillGrad.direction())
		}
	} else {
		s.Set("fillColor", "none")
	}

	if p.hasStroke() {
		s.Set("strokeColor", p.stroke)
		s.SetFloat("strokeWidth", c.round(p.strokeW))
	} else {
		s.Set("strokeColor", "none")
	}

	if p.opacity < 1 {
		s.SetFloat("opacity", c.round(p.opacity*100))
	}
	if p.fillOpacity < 1 && p.hasFill() {
		s.SetFloat("fillOpacity", c.round(p.fillOpacity*100))
	}
	if p.strokeOpacity < 1 && p.hasStroke() {
		s.SetFloat("strokeOpacity", c.round(p.strokeOpacity*100))
	}
	if p.dash != "" {
		s.Set("dashed", "1")
		s.Set("dashPattern", p.dash)
	}
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }
