package svgdom

import (
	"math"
	"strings"
	"testing"
)

func TestParseKeepsLocalNamesAndTree(t *testing.T) {
	doc, err := Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" width="10">
		<g id="g1"><rect x="1"/></g></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Root.Attr("width") != "10" {
		t.Errorf("root width = %q", doc.Root.Attr("width"))
	}
	g := doc.ByID("g1")
	if g == nil || g.Tag != "g" || len(g.Children) != 1 || g.Children[0].Tag != "rect" {
		t.Fatalf("group not parsed: %+v", g)
	}
}

func TestParseRejectsNonSVGRoot(t *testing.T) {
	if _, err := Parse(strings.NewReader(`<html></html>`)); err == nil {
		t.Error("expected an error for a non-svg root")
	}
}

func TestResolveExpandsUseWithOffset(t *testing.T) {
	doc, err := Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg">
		<defs><circle id="c" cx="0" cy="0" r="5"/></defs>
		<use xlink:href="#c" x="10" y="20"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	doc.Resolve()

	var found *Node
	doc.Root.Walk(func(n *Node) {
		if n.Tag == "g" && strings.Contains(n.Attr("transform"), "translate") {
			found = n
		}
	})
	if found == nil {
		t.Fatal("use was not expanded into a translated group")
	}
	m := ParseTransform(found.Attr("transform"))
	if m.E != 10 || m.F != 20 {
		t.Errorf("use offset = (%v,%v), want (10,20)", m.E, m.F)
	}
	if len(found.Children) != 1 || found.Children[0].Tag != "circle" {
		t.Errorf("expanded content = %+v", found.Children)
	}
}

func TestParseTransformComposesInOrder(t *testing.T) {
	m := ParseTransform("translate(10,20) scale(2)")
	x, y := m.Apply(3, 4)
	if x != 16 || y != 28 {
		t.Errorf("point = (%v,%v), want (16,28)", x, y)
	}
}

func TestRotateAboutPoint(t *testing.T) {
	m := ParseTransform("rotate(90,10,10)")
	x, y := m.Apply(10, 0)
	if math.Abs(x-20) > 1e-9 || math.Abs(y-10) > 1e-9 {
		t.Errorf("point = (%v,%v), want (20,10)", x, y)
	}
}

func TestLengthUnits(t *testing.T) {
	cases := map[string]float64{"10": 10, "10px": 10, "1in": 96, "72pt": 96, "": 0}
	for in, want := range cases {
		got, ok := Length(in)
		if in == "" {
			if ok {
				t.Errorf("empty length should not parse")
			}
			continue
		}
		if !ok || math.Abs(got-want) > 1e-9 {
			t.Errorf("Length(%q) = %v, want %v", in, got, want)
		}
	}
	if _, ok := Length("50%"); ok {
		t.Error("percentages need a viewport and should not parse")
	}
}

func TestNormalizeColor(t *testing.T) {
	cases := map[string]string{
		"#abc":            "#aabbcc",
		"#AABBCC":         "#AABBCC",
		"rgb(255, 0, 10)": "#ff000a",
		"rgb(100%,0%,0%)": "#ff0000",
		"red":             "#ff0000",
		"none":            "none",
	}
	for in, want := range cases {
		got, ok := NormalizeColor(in)
		if !ok || !strings.EqualFold(got, want) {
			t.Errorf("NormalizeColor(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := NormalizeColor("url(#grad)"); ok {
		t.Error("url() paints must be reported as unresolved")
	}
}

func TestStyleAttributeBeatsPresentationAttribute(t *testing.T) {
	doc, _ := Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg">
		<rect fill="red" style="fill:blue;stroke-width:3"/></svg>`))
	st := StyleOf(doc.Root.Children[0])
	if st.Fill != "blue" {
		t.Errorf("fill = %q, want blue", st.Fill)
	}
	if st.StrokeWidth == nil || *st.StrokeWidth != 3 {
		t.Errorf("stroke width not read from the style attribute")
	}
}

func TestInheritFillsGapsAndCompoundsOpacity(t *testing.T) {
	half := 0.5
	parent := Style{Fill: "#ff0000", Opacity: &half}
	child := Style{Stroke: "#00ff00", Opacity: &half}
	got := child.Inherit(parent)
	if got.Fill != "#ff0000" {
		t.Errorf("fill = %q, want inherited #ff0000", got.Fill)
	}
	if got.Opacity == nil || math.Abs(*got.Opacity-0.25) > 1e-9 {
		t.Errorf("opacity = %v, want 0.25", got.Opacity)
	}
}

func TestDashPatternIsScaled(t *testing.T) {
	if got := DashPattern("8 4", 0.5); got != "4 2" {
		t.Errorf("DashPattern = %q, want \"4 2\"", got)
	}
}
