// Package mxgraph builds .drawio files: an mxfile wrapping an mxGraphModel of
// mxCells.
package mxgraph

import (
	"fmt"
	"strings"
)

// Style is an ordered list of draw.io style entries. Order matters because the
// leading entry (shape name) is what draw.io reads first.
type Style struct {
	entries []entry
}

type entry struct{ key, value string }

// Set appends or replaces a style entry. An empty value writes a bare key,
// which is how draw.io spells shape names such as "ellipse".
func (s *Style) Set(key, value string) {
	for i := range s.entries {
		if s.entries[i].key == key {
			s.entries[i].value = value
			return
		}
	}
	s.entries = append(s.entries, entry{key, value})
}

// SetFloat appends a numeric entry.
func (s *Style) SetFloat(key string, v float64) { s.Set(key, Num(v)) }

// Has reports whether the key is present.
func (s *Style) Has(key string) bool {
	for _, e := range s.entries {
		if e.key == key {
			return true
		}
	}
	return false
}

// String renders the style in draw.io syntax.
func (s *Style) String() string {
	var b strings.Builder
	for _, e := range s.entries {
		if e.value == "" {
			b.WriteString(e.key)
		} else {
			b.WriteString(e.key + "=" + e.value)
		}
		b.WriteString(";")
	}
	return b.String()
}

// Point is a coordinate inside a geometry.
type Point struct{ X, Y float64 }

// Geometry is an mxGeometry. Source, Target and Points are only used by edges,
// where they carry the floating endpoints and the waypoints.
type Geometry struct {
	X, Y, W, H float64
	Relative   bool
	Source     *Point
	Target     *Point
	Points     []Point
}

// Cell is an mxCell. Only the vertex form is produced by the converter.
type Cell struct {
	ID       string
	Value    string
	Style    string
	Parent   string
	Vertex   bool
	Edge     bool
	Geometry *Geometry
	Children []*Cell
}

// Model is one diagram page.
type Model struct {
	Name       string
	PageWidth  float64
	PageHeight float64
	Background string
	Cells      []*Cell
}

// File is a complete .drawio document.
type File struct {
	Host   string
	Agent  string
	Models []*Model
}

// XML renders the .drawio file.
func (f *File) XML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<mxfile")
	attr(&b, "host", orDefault(f.Host, "go-svg2drawio"))
	attr(&b, "agent", orDefault(f.Agent, "go-svg2drawio"))
	attr(&b, "type", "device")
	b.WriteString(">\n")

	for i, m := range f.Models {
		name := m.Name
		if name == "" {
			name = fmt.Sprintf("Page-%d", i+1)
		}
		b.WriteString("  <diagram")
		attr(&b, "id", fmt.Sprintf("page-%d", i+1))
		attr(&b, "name", name)
		b.WriteString(">\n")
		m.write(&b, 2)
		b.WriteString("  </diagram>\n")
	}

	b.WriteString("</mxfile>\n")
	return b.String()
}

func (m *Model) write(b *strings.Builder, depth int) {
	pad(b, depth)
	b.WriteString("<mxGraphModel")
	attr(b, "dx", "1422")
	attr(b, "dy", "798")
	attr(b, "grid", "1")
	attr(b, "gridSize", "10")
	attr(b, "guides", "1")
	attr(b, "tooltips", "1")
	attr(b, "connect", "1")
	attr(b, "arrows", "1")
	attr(b, "fold", "1")
	attr(b, "page", "1")
	attr(b, "pageScale", "1")
	attr(b, "pageWidth", Num(m.PageWidth))
	attr(b, "pageHeight", Num(m.PageHeight))
	if m.Background != "" {
		attr(b, "background", m.Background)
	}
	attr(b, "math", "0")
	attr(b, "shadow", "0")
	b.WriteString(">\n")

	pad(b, depth+1)
	b.WriteString("<root>\n")
	pad(b, depth+2)
	b.WriteString(`<mxCell id="0" />` + "\n")
	pad(b, depth+2)
	b.WriteString(`<mxCell id="1" parent="0" />` + "\n")
	for _, c := range m.Cells {
		c.write(b, depth+2)
	}
	pad(b, depth+1)
	b.WriteString("</root>\n")
	pad(b, depth)
	b.WriteString("</mxGraphModel>\n")
}

func (c *Cell) write(b *strings.Builder, depth int) {
	pad(b, depth)
	b.WriteString("<mxCell")
	attr(b, "id", c.ID)
	if c.Value != "" {
		attr(b, "value", c.Value)
	}
	if c.Style != "" {
		attr(b, "style", c.Style)
	}
	if c.Vertex {
		attr(b, "vertex", "1")
	}
	if c.Edge {
		attr(b, "edge", "1")
	}
	attr(b, "parent", c.Parent)

	if c.Geometry == nil {
		b.WriteString(" />\n")
	} else {
		b.WriteString(">\n")
		c.Geometry.write(b, depth+1)
		pad(b, depth)
		b.WriteString("</mxCell>\n")
	}

	for _, child := range c.Children {
		child.write(b, depth)
	}
}

func (g *Geometry) write(b *strings.Builder, depth int) {
	pad(b, depth)
	b.WriteString("<mxGeometry")
	if g.X != 0 || g.Source == nil {
		attr(b, "x", Num(g.X))
	}
	if g.Y != 0 || g.Source == nil {
		attr(b, "y", Num(g.Y))
	}
	if g.W != 0 || g.Source == nil {
		attr(b, "width", Num(g.W))
	}
	if g.H != 0 || g.Source == nil {
		attr(b, "height", Num(g.H))
	}
	if g.Relative {
		attr(b, "relative", "1")
	}
	attr(b, "as", "geometry")

	if g.Source == nil && g.Target == nil && len(g.Points) == 0 {
		b.WriteString(" />\n")
		return
	}

	b.WriteString(">\n")
	if g.Source != nil {
		writePoint(b, depth+1, g.Source.X, g.Source.Y, "sourcePoint")
	}
	if g.Target != nil {
		writePoint(b, depth+1, g.Target.X, g.Target.Y, "targetPoint")
	}
	if len(g.Points) > 0 {
		pad(b, depth+1)
		b.WriteString(`<Array as="points">` + "\n")
		for _, p := range g.Points {
			writePoint(b, depth+2, p.X, p.Y, "")
		}
		pad(b, depth+1)
		b.WriteString("</Array>\n")
	}
	pad(b, depth)
	b.WriteString("</mxGeometry>\n")
}

func writePoint(b *strings.Builder, depth int, x, y float64, as string) {
	pad(b, depth)
	b.WriteString("<mxPoint")
	attr(b, "x", Num(x))
	attr(b, "y", Num(y))
	if as != "" {
		attr(b, "as", as)
	}
	b.WriteString(" />\n")
}

func attr(b *strings.Builder, name, value string) {
	b.WriteString(" " + name + `="` + Escape(value) + `"`)
}

func pad(b *strings.Builder, depth int) { b.WriteString(strings.Repeat("  ", depth)) }

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Escape encodes a string for an XML attribute value.
func Escape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
		"\n", "&#10;",
		"\r", "&#13;",
	).Replace(s)
}
