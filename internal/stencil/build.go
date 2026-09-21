package stencil

import "github.com/bcollard/svg2drawio/internal/svgpath"

// PathElem converts parsed SVG path data into a stencil <path> element.
// Coordinates are rounded to the given number of decimals; use -1 to keep the
// full precision.
func PathElem(p svgpath.Path, decimals int) Elem {
	out := Elem{Name: "path"}
	n := func(v float64) string { return Num(Round(v, decimals)) }

	for _, seg := range p {
		switch seg.Op {
		case svgpath.OpMove:
			out.Children = append(out.Children, Elem{Name: "move", Attrs: []Attr{
				{"x", n(seg.Args[0])}, {"y", n(seg.Args[1])},
			}})
		case svgpath.OpLine:
			out.Children = append(out.Children, Elem{Name: "line", Attrs: []Attr{
				{"x", n(seg.Args[0])}, {"y", n(seg.Args[1])},
			}})
		case svgpath.OpQuad:
			out.Children = append(out.Children, Elem{Name: "quad", Attrs: []Attr{
				{"x1", n(seg.Args[0])}, {"y1", n(seg.Args[1])},
				{"x2", n(seg.Args[2])}, {"y2", n(seg.Args[3])},
			}})
		case svgpath.OpCubic:
			out.Children = append(out.Children, Elem{Name: "curve", Attrs: []Attr{
				{"x1", n(seg.Args[0])}, {"y1", n(seg.Args[1])},
				{"x2", n(seg.Args[2])}, {"y2", n(seg.Args[3])},
				{"x3", n(seg.Args[4])}, {"y3", n(seg.Args[5])},
			}})
		case svgpath.OpClose:
			out.Children = append(out.Children, Elem{Name: "close"})
		}
	}
	return out
}

// FillStroke returns the paint element matching which of fill and stroke are
// active, or an empty element name when the shape paints nothing.
func FillStroke(hasFill, hasStroke bool) (Elem, bool) {
	switch {
	case hasFill && hasStroke:
		return Elem{Name: "fillstroke"}, true
	case hasFill:
		return Elem{Name: "fill"}, true
	case hasStroke:
		return Elem{Name: "stroke"}, true
	}
	return Elem{}, false
}

// Set returns a single-attribute style element such as <strokecolor color=..>.
func Set(name, attr, value string) Elem {
	return Elem{Name: name, Attrs: []Attr{{attr, value}}}
}
