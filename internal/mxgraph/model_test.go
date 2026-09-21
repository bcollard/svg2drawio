package mxgraph

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestStyleRendering(t *testing.T) {
	var s Style
	s.Set("ellipse", "")
	s.Set("fillColor", "#ff0000")
	s.SetFloat("strokeWidth", 1.5)
	if got := s.String(); got != "ellipse;fillColor=#ff0000;strokeWidth=1.5;" {
		t.Errorf("style = %q", got)
	}
}

func TestStyleSetReplacesInPlace(t *testing.T) {
	var s Style
	s.Set("fillColor", "#000000")
	s.Set("strokeColor", "#111111")
	s.Set("fillColor", "#222222")
	if got := s.String(); got != "fillColor=#222222;strokeColor=#111111;" {
		t.Errorf("style = %q, want the first entry updated in place", got)
	}
}

func TestVertexXML(t *testing.T) {
	f := &File{Models: []*Model{{
		Name: "Page-1", PageWidth: 850, PageHeight: 1100,
		Cells: []*Cell{{
			ID: "a", Value: "hi", Style: "ellipse;", Vertex: true, Parent: "1",
			Geometry: &Geometry{X: 1, Y: 2, W: 3, H: 4},
		}},
	}}}
	out := f.XML()

	if err := xml.Unmarshal([]byte(out), new(struct {
		XMLName xml.Name `xml:"mxfile"`
	})); err != nil {
		t.Fatalf("not well formed: %v", err)
	}
	for _, want := range []string{
		`<mxCell id="a" value="hi" style="ellipse;" vertex="1" parent="1">`,
		`<mxGeometry x="1" y="2" width="3" height="4" as="geometry" />`,
		`<mxCell id="0" />`,
		`<mxCell id="1" parent="0" />`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestEdgeXMLCarriesPoints(t *testing.T) {
	src := Point{X: 0, Y: 0}
	dst := Point{X: 10, Y: 10}
	f := &File{Models: []*Model{{Cells: []*Cell{{
		ID: "e", Style: "endArrow=none;", Edge: true, Parent: "1",
		Geometry: &Geometry{Relative: true, Source: &src, Target: &dst,
			Points: []Point{{X: 5, Y: 0}}},
	}}}}}
	out := f.XML()

	for _, want := range []string{
		`edge="1"`,
		`<mxPoint x="0" y="0" as="sourcePoint" />`,
		`<mxPoint x="10" y="10" as="targetPoint" />`,
		`<Array as="points">`,
		`<mxPoint x="5" y="0" />`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestChildCellsAreFlatWithParentReference(t *testing.T) {
	child := &Cell{ID: "c", Vertex: true, Parent: "g", Geometry: &Geometry{W: 1, H: 1}}
	group := &Cell{ID: "g", Style: "group;", Vertex: true, Parent: "1",
		Geometry: &Geometry{W: 2, H: 2}, Children: []*Cell{child}}
	out := (&File{Models: []*Model{{Cells: []*Cell{group}}}}).XML()

	if strings.Index(out, `id="g"`) > strings.Index(out, `id="c"`) {
		t.Error("the group must be written before its children")
	}
	if !strings.Contains(out, `<mxCell id="c" style="" vertex="1" parent="g">`) &&
		!strings.Contains(out, `<mxCell id="c" vertex="1" parent="g">`) {
		t.Errorf("child cell is not parented to the group:\n%s", out)
	}
}

func TestAttributesAreEscaped(t *testing.T) {
	out := (&File{Models: []*Model{{Cells: []*Cell{{
		ID: "a", Value: `a & b "c" <d>`, Vertex: true, Parent: "1",
		Geometry: &Geometry{W: 1, H: 1},
	}}}}}).XML()
	if !strings.Contains(out, `value="a &amp; b &quot;c&quot; &lt;d&gt;"`) {
		t.Errorf("value not escaped:\n%s", out)
	}
}

func TestMultiplePagesAreNumbered(t *testing.T) {
	out := (&File{Models: []*Model{{Name: "one"}, {}}}).XML()
	if !strings.Contains(out, `id="page-1" name="one"`) || !strings.Contains(out, `id="page-2" name="Page-2"`) {
		t.Errorf("pages not numbered:\n%s", out)
	}
}
