// Package stencil builds mxGraph stencil definitions: the <shape> documents
// draw.io understands both in shape libraries and inline in a cell style via
// shape=stencil(...).
package stencil

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Attr is one XML attribute.
type Attr struct {
	Name  string
	Value string
}

// Elem is a stencil drawing element such as <path>, <rect> or <fillstroke>.
type Elem struct {
	Name     string
	Attrs    []Attr
	Children []Elem
}

// Shape is a single stencil definition.
type Shape struct {
	Name        string
	W, H        float64
	Aspect      string // "variable" (default) or "fixed"
	StrokeWidth string // "inherit" lets the cell style drive the stroke width
	Background  []Elem
	Foreground  []Elem
	Connections []Constraint
}

// Constraint is a fixed connection point on a stencil.
type Constraint struct {
	Name      string
	X, Y      float64
	Perimeter bool
	Decimals  int
}

// Library is a named set of stencils, the format of a draw.io shape library.
type Library struct {
	Name   string
	Shapes []*Shape
}

// XML renders the stencil as a standalone <shape> document.
func (s *Shape) XML(indent bool) string {
	var b strings.Builder
	s.write(&b, indent, 0)
	return b.String()
}

func (s *Shape) write(b *strings.Builder, indent bool, depth int) {
	aspect := s.Aspect
	if aspect == "" {
		aspect = "variable"
	}
	strokeWidth := s.StrokeWidth
	if strokeWidth == "" {
		strokeWidth = "inherit"
	}

	pad(b, indent, depth)
	b.WriteString("<shape")
	if s.Name != "" {
		writeAttr(b, "name", s.Name)
	}
	writeAttr(b, "w", Num(s.W))
	writeAttr(b, "h", Num(s.H))
	writeAttr(b, "aspect", aspect)
	writeAttr(b, "strokewidth", strokeWidth)
	b.WriteString(">")
	nl(b, indent)

	if len(s.Connections) > 0 {
		pad(b, indent, depth+1)
		b.WriteString("<connections>")
		nl(b, indent)
		for _, c := range s.Connections {
			pad(b, indent, depth+2)
			b.WriteString("<constraint")
			if c.Name != "" {
				writeAttr(b, "name", c.Name)
			}
			writeAttr(b, "x", Num(c.X))
			writeAttr(b, "y", Num(c.Y))
			writeAttr(b, "perimeter", boolAttr(c.Perimeter))
			b.WriteString("/>")
			nl(b, indent)
		}
		pad(b, indent, depth+1)
		b.WriteString("</connections>")
		nl(b, indent)
	}

	writeSection(b, "background", s.Background, indent, depth+1)
	writeSection(b, "foreground", s.Foreground, indent, depth+1)

	pad(b, indent, depth)
	b.WriteString("</shape>")
}

func writeSection(b *strings.Builder, name string, elems []Elem, indent bool, depth int) {
	if len(elems) == 0 {
		if name == "background" {
			return
		}
		pad(b, indent, depth)
		b.WriteString("<" + name + "/>")
		nl(b, indent)
		return
	}
	pad(b, indent, depth)
	b.WriteString("<" + name + ">")
	nl(b, indent)
	for _, e := range elems {
		e.write(b, indent, depth+1)
	}
	pad(b, indent, depth)
	b.WriteString("</" + name + ">")
	nl(b, indent)
}

func (e Elem) write(b *strings.Builder, indent bool, depth int) {
	pad(b, indent, depth)
	b.WriteString("<" + e.Name)
	for _, a := range e.Attrs {
		writeAttr(b, a.Name, a.Value)
	}
	if len(e.Children) == 0 {
		b.WriteString("/>")
		nl(b, indent)
		return
	}
	b.WriteString(">")
	nl(b, indent)
	for _, c := range e.Children {
		c.write(b, indent, depth+1)
	}
	pad(b, indent, depth)
	b.WriteString("</" + e.Name + ">")
	nl(b, indent)
}

// XML renders the library as a <shapes> document with an XML declaration.
func (l *Library) XML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<shapes")
	if l.Name != "" {
		writeAttr(&b, "name", l.Name)
	}
	b.WriteString(">\n")
	for _, s := range l.Shapes {
		s.write(&b, true, 1)
		b.WriteString("\n")
	}
	b.WriteString("</shapes>\n")
	return b.String()
}

func pad(b *strings.Builder, indent bool, depth int) {
	if indent {
		b.WriteString(strings.Repeat("  ", depth))
	}
}

func nl(b *strings.Builder, indent bool) {
	if indent {
		b.WriteString("\n")
	}
}

func writeAttr(b *strings.Builder, name, value string) {
	b.WriteString(" " + name + `="` + Escape(value) + `"`)
}

func boolAttr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// Escape encodes a string for use in an XML attribute value.
func Escape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
		"\n", "&#10;",
		"\r", "&#13;",
		"\t", "&#9;",
	)
	return r.Replace(s)
}

// Num formats a coordinate without a trailing ".0" and without scientific
// notation, which mxGraph's Number() parsing would choke on.
func Num(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if strings.Contains(s, "e") {
		s = fmt.Sprintf("%.6f", v)
	}
	return s
}

// Round rounds v to the given number of decimals; decimals < 0 leaves the
// value untouched.
func Round(v float64, decimals int) float64 {
	if decimals < 0 {
		return v
	}
	pow := math.Pow(10, float64(decimals))
	r := math.Round(v*pow) / pow
	if r == 0 {
		return 0 // avoid "-0"
	}
	return r
}
