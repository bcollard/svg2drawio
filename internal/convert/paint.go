package convert

import (
	"math"
	"strings"

	"github.com/bcollard/svg2drawio/internal/svgdom"
)

// gradientStop is one colour stop of an SVG gradient.
type gradientStop struct {
	offset  float64
	color   string
	opacity float64
}

// gradient is a linear or radial SVG gradient reduced to what draw.io can
// express: a start colour, an end colour and a direction.
type gradient struct {
	stops  []gradientStop
	x1, y1 float64
	x2, y2 float64
	radial bool
}

// start returns the first stop colour.
func (g *gradient) start() string {
	if len(g.stops) == 0 {
		return ""
	}
	return g.stops[0].color
}

// end returns the last stop colour.
func (g *gradient) end() string {
	if len(g.stops) == 0 {
		return ""
	}
	return g.stops[len(g.stops)-1].color
}

// direction maps the gradient vector onto the four directions draw.io knows.
func (g *gradient) direction() string {
	if g.radial {
		return "radial"
	}
	dx, dy := g.x2-g.x1, g.y2-g.y1
	if math.Abs(dx) >= math.Abs(dy) {
		if dx >= 0 {
			return "east"
		}
		return "west"
	}
	if dy >= 0 {
		return "south"
	}
	return "north"
}

// average collapses the gradient to a single colour, used where draw.io cannot
// carry the gradient itself.
func (g *gradient) average() string {
	colors := make([]string, 0, len(g.stops))
	for _, s := range g.stops {
		colors = append(colors, s.color)
	}
	return svgdom.MixColors(colors)
}

// gradients indexes the gradient definitions of a document.
type gradients struct {
	doc   *svgdom.Document
	cache map[string]*gradient
}

func newGradients(doc *svgdom.Document) *gradients {
	return &gradients{doc: doc, cache: map[string]*gradient{}}
}

// lookup resolves a url(#id) paint reference to a gradient, following
// href chains for inherited stops.
func (g *gradients) lookup(paint string) *gradient {
	id := refFromURL(paint)
	if id == "" {
		return nil
	}
	if cached, ok := g.cache[id]; ok {
		return cached
	}
	// Guard against reference cycles while the chain is being resolved.
	g.cache[id] = nil

	node := g.doc.ByID(id)
	if node == nil {
		return nil
	}
	res := g.build(node, 0)
	g.cache[id] = res
	return res
}

const maxGradientDepth = 8

func (g *gradients) build(n *svgdom.Node, depth int) *gradient {
	if depth > maxGradientDepth {
		return nil
	}
	switch n.Tag {
	case "linearGradient", "radialGradient":
	default:
		return nil
	}

	out := &gradient{radial: n.Tag == "radialGradient"}

	// Default vector of a linear gradient is left to right.
	out.x1, out.y1, out.x2, out.y2 = 0, 0, 1, 0
	if v, ok := ratio(n.Attr("x1")); ok {
		out.x1 = v
	}
	if v, ok := ratio(n.Attr("y1")); ok {
		out.y1 = v
	}
	if v, ok := ratio(n.Attr("x2")); ok {
		out.x2 = v
	}
	if v, ok := ratio(n.Attr("y2")); ok {
		out.y2 = v
	}

	out.stops = collectStops(n)
	if len(out.stops) == 0 {
		if ref := svgdom.RefID(n); ref != "" {
			if parent := g.doc.ByID(ref); parent != nil {
				if inherited := g.build(parent, depth+1); inherited != nil {
					out.stops = inherited.stops
				}
			}
		}
	}
	if len(out.stops) == 0 {
		return nil
	}
	return out
}

func collectStops(n *svgdom.Node) []gradientStop {
	var stops []gradientStop
	for _, child := range n.Children {
		if child.Tag != "stop" {
			continue
		}
		st := gradientStop{opacity: 1}
		if v, ok := ratio(child.Attr("offset")); ok {
			st.offset = v
		}
		style := svgdom.StyleOf(child)
		color := firstNonEmpty(child.Attr("stop-color"), styleProp(child, "stop-color"))
		if c, ok := svgdom.NormalizeColor(color); ok && c != "none" {
			st.color = c
		} else if style.Fill != "" {
			if c, ok := svgdom.NormalizeColor(style.Fill); ok && c != "none" {
				st.color = c
			}
		}
		if st.color == "" {
			st.color = "#000000"
		}
		op := firstNonEmpty(child.Attr("stop-opacity"), styleProp(child, "stop-opacity"))
		if v, ok := svgdom.Length(op); ok {
			st.opacity = v
		}
		stops = append(stops, st)
	}
	return stops
}

func styleProp(n *svgdom.Node, name string) string {
	for _, decl := range strings.Split(n.Attr("style"), ";") {
		colon := strings.IndexByte(decl, ':')
		if colon < 0 {
			continue
		}
		if strings.TrimSpace(decl[:colon]) == name {
			return strings.TrimSpace(decl[colon+1:])
		}
	}
	return ""
}

func refFromURL(paint string) string {
	p := strings.TrimSpace(paint)
	if !strings.HasPrefix(p, "url(") {
		return ""
	}
	p = strings.TrimSuffix(strings.TrimPrefix(p, "url("), ")")
	p = strings.Trim(p, `"' `)
	return strings.TrimPrefix(p, "#")
}

func ratio(v string) (float64, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if strings.HasSuffix(v, "%") {
		f, ok := svgdom.Length(strings.TrimSuffix(v, "%"))
		return f / 100, ok
	}
	return svgdom.Length(v)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
