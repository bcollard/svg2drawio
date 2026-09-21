package svgdom

import (
	"strings"
	"testing"
)

func docFrom(t *testing.T, body string) *Document {
	t.Helper()
	doc, err := Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg">` + body + `</svg>`))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func firstTag(doc *Document, tag string) *Node {
	var found *Node
	doc.Root.Walk(func(n *Node) {
		if found == nil && n.Tag == tag {
			found = n
		}
	})
	return found
}

func TestClassSelector(t *testing.T) {
	doc := docFrom(t, `<style>.a { fill: #ff0000; }</style><rect class="a"/>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Fill; got != "#ff0000" {
		t.Errorf("fill = %q", got)
	}
}

func TestSpecificityOrder(t *testing.T) {
	doc := docFrom(t, `<style>
		rect { fill: red; }
		.a { fill: green; }
		#r { fill: blue; }
	</style><rect id="r" class="a"/>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Fill; got != "blue" {
		t.Errorf("fill = %q, want the id rule to win", got)
	}
}

func TestLaterRuleWinsAtEqualSpecificity(t *testing.T) {
	doc := docFrom(t, `<style>.a { fill: red; } .a { fill: green; }</style><rect class="a"/>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Fill; got != "green" {
		t.Errorf("fill = %q, want green", got)
	}
}

func TestGroupedSelectors(t *testing.T) {
	doc := docFrom(t, `<style>.a, .b { stroke: #123456; }</style><rect class="b"/>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Stroke; got != "#123456" {
		t.Errorf("stroke = %q", got)
	}
}

func TestDescendantSelector(t *testing.T) {
	doc := docFrom(t, `<style>g .a { fill: teal; }</style><g><g><rect class="a"/></g></g>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Fill; got != "teal" {
		t.Errorf("fill = %q", got)
	}
}

func TestChildSelectorDoesNotMatchGrandchild(t *testing.T) {
	doc := docFrom(t, `<style>svg > rect { fill: teal; }</style><g><rect/></g>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Fill; got != "" {
		t.Errorf("fill = %q, want no match", got)
	}
}

func TestCompoundTagAndClass(t *testing.T) {
	doc := docFrom(t, `<style>text.t { font-size: 22px; } rect.t { font-size: 9px; }</style><text class="t"/>`)
	st := doc.StyleOf(firstTag(doc, "text"))
	if st.FontSize == nil || *st.FontSize != 22 {
		t.Errorf("font size = %v, want 22", st.FontSize)
	}
}

func TestCommentsAndImportantAreHandled(t *testing.T) {
	doc := docFrom(t, `<style>/* comment { fill: red } */ .a { fill: green !important; }</style><rect class="a"/>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Fill; got != "green" {
		t.Errorf("fill = %q", got)
	}
}

func TestUnsupportedSelectorsAreIgnored(t *testing.T) {
	doc := docFrom(t, `<style>.a:hover { fill: red; } .a { fill: green; }</style><rect class="a"/>`)
	if got := doc.StyleOf(firstTag(doc, "rect")).Fill; got != "green" {
		t.Errorf("fill = %q, want the pseudo class rule skipped", got)
	}
}

func TestCascadeOrderAcrossSources(t *testing.T) {
	doc := docFrom(t, `<style>.a { fill: green; }</style><rect class="a" fill="red" style="stroke:#000"/>`)
	st := doc.StyleOf(firstTag(doc, "rect"))
	if st.Fill != "green" {
		t.Errorf("fill = %q, want the rule to beat the presentation attribute", st.Fill)
	}
	if st.Stroke != "#000" {
		t.Errorf("stroke = %q, want the style attribute applied", st.Stroke)
	}
}
