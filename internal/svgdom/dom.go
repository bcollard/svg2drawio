// Package svgdom parses SVG documents into a simple mutable tree and resolves
// the parts of SVG that have no equivalent in mxGraph stencils: <defs>, <use>,
// <symbol> and nested transforms.
package svgdom

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Node is an element of the SVG tree.
type Node struct {
	Tag      string
	Attrs    map[string]string
	Text     string
	Children []*Node
	Parent   *Node
}

// Attr returns the attribute value, or "" when absent.
func (n *Node) Attr(name string) string { return n.Attrs[name] }

// HasAttr reports whether the attribute is present.
func (n *Node) HasAttr(name string) bool { _, ok := n.Attrs[name]; return ok }

// SetAttr sets an attribute value.
func (n *Node) SetAttr(name, value string) { n.Attrs[name] = value }

// Clone returns a deep copy of the node, detached from its parent.
func (n *Node) Clone() *Node {
	c := &Node{Tag: n.Tag, Text: n.Text, Attrs: make(map[string]string, len(n.Attrs))}
	for k, v := range n.Attrs {
		c.Attrs[k] = v
	}
	for _, child := range n.Children {
		cc := child.Clone()
		cc.Parent = c
		c.Children = append(c.Children, cc)
	}
	return c
}

// Append adds a child and sets its parent.
func (n *Node) Append(child *Node) {
	child.Parent = n
	n.Children = append(n.Children, child)
}

// Walk calls fn for n and every descendant, depth first, document order.
func (n *Node) Walk(fn func(*Node)) {
	fn(n)
	for _, c := range n.Children {
		c.Walk(fn)
	}
}

// Document is a parsed SVG file.
type Document struct {
	Root *Node
	// ids maps every id attribute in the document to its node.
	ids map[string]*Node
	// sheet holds the rules of the document's <style> elements.
	sheet *Stylesheet
}

// ByID returns the node carrying the given id.
func (d *Document) ByID(id string) *Node { return d.ids[id] }

// StyleOf resolves the style of a node in CSS cascade order: presentation
// attributes first, then the document's style rules by specificity, then the
// element's own style attribute.
func (d *Document) StyleOf(n *Node) Style {
	var s Style
	for k, v := range n.Attrs {
		if k == "style" {
			continue
		}
		s.set(k, v)
	}
	for _, decls := range d.sheet.Match(n) {
		for k, v := range decls {
			s.set(k, v)
		}
	}
	for k, v := range parseStyleAttr(n.Attr("style")) {
		s.set(k, v)
	}
	return s
}

// Parse reads an SVG document. Namespace prefixes are dropped: only local
// names are kept, which is what every stencil-relevant element needs.
func Parse(r io.Reader) (*Document, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	var root *Node
	var stack []*Node

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse svg: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			n := &Node{Tag: t.Name.Local, Attrs: make(map[string]string, len(t.Attr))}
			for _, a := range t.Attr {
				name := a.Name.Local
				if a.Name.Space == "xmlns" || name == "xmlns" {
					continue
				}
				n.Attrs[name] = a.Value
			}
			if len(stack) > 0 {
				stack[len(stack)-1].Append(n)
			} else if root == nil {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}

	if root == nil {
		return nil, fmt.Errorf("no root element found")
	}
	if root.Tag != "svg" {
		return nil, fmt.Errorf("root element is <%s>, not <svg>", root.Tag)
	}

	doc := &Document{Root: root, ids: map[string]*Node{}}
	var css strings.Builder
	root.Walk(func(n *Node) {
		if id := n.Attr("id"); id != "" {
			if _, seen := doc.ids[id]; !seen {
				doc.ids[id] = n
			}
		}
		if n.Tag == "style" {
			css.WriteString(n.Text)
			css.WriteString("\n")
		}
	})
	doc.sheet = ParseStylesheet(css.String())
	return doc, nil
}

// Resolve expands <use> references and inlines <symbol> content so that the
// tree can be walked as plain geometry.
func (d *Document) Resolve() {
	d.expandUses(d.Root, 0)
}

const maxUseDepth = 16

func (d *Document) expandUses(n *Node, depth int) {
	if depth > maxUseDepth {
		return
	}
	for i, child := range n.Children {
		if child.Tag != "use" {
			d.expandUses(child, depth)
			continue
		}
		ref := refID(child)
		target := d.ids[ref]
		if target == nil {
			continue
		}

		// A <use> becomes a group carrying the reference's own attributes plus
		// the x/y offset, which SVG defines as an extra translate.
		g := &Node{Tag: "g", Attrs: map[string]string{}}
		for k, v := range child.Attrs {
			if k == "href" || k == "x" || k == "y" || k == "width" || k == "height" {
				continue
			}
			g.Attrs[k] = v
		}
		x, y := child.Attr("x"), child.Attr("y")
		if x != "" || y != "" {
			translate := fmt.Sprintf("translate(%s,%s)", orZero(x), orZero(y))
			if t := g.Attr("transform"); t != "" {
				g.SetAttr("transform", t+" "+translate)
			} else {
				g.SetAttr("transform", translate)
			}
		}

		clone := target.Clone()
		if clone.Tag == "symbol" || clone.Tag == "svg" {
			// <symbol> is not rendered on its own; its children are.
			clone.Tag = "g"
			delete(clone.Attrs, "id")
		}
		delete(clone.Attrs, "id")
		g.Append(clone)

		g.Parent = n
		n.Children[i] = g
		d.expandUses(g, depth+1)
	}
}

func orZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}

func refID(n *Node) string {
	href := n.Attr("href")
	if href == "" {
		href = n.Attr("xlink:href")
	}
	return strings.TrimPrefix(strings.TrimSpace(href), "#")
}

// Href returns the (possibly external) reference of a node.
func Href(n *Node) string {
	href := n.Attr("href")
	if href == "" {
		href = n.Attr("xlink:href")
	}
	return strings.TrimSpace(href)
}

// RefID returns the local id referenced by href/xlink:href.
func RefID(n *Node) string { return refID(n) }
