package convert

import (
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/bcollard/svg2drawio/internal/mxgraph"
	"github.com/bcollard/svg2drawio/internal/stencil"
)

func diagramFrom(t *testing.T, svg string) *mxgraph.File {
	t.Helper()
	f, err := Diagram(strings.NewReader(svg), "test.svg", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func cellsOf(t *testing.T, svg string) []*mxgraph.Cell {
	t.Helper()
	return diagramFrom(t, svg).Models[0].Cells
}

const wrap = `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">%s</svg>`

func svgWith(body string) string { return strings.Replace(wrap, "%s", body, 1) }

func TestRectBecomesNativeRectangle(t *testing.T) {
	cells := cellsOf(t, svgWith(`<rect x="10" y="20" width="30" height="40" fill="#ff0000" stroke="#000"/>`))
	if len(cells) != 1 {
		t.Fatalf("got %d cells, want 1", len(cells))
	}
	c := cells[0]
	if strings.Contains(c.Style, "shape=stencil") {
		t.Errorf("a plain rectangle should stay native: %s", c.Style)
	}
	if !strings.Contains(c.Style, "fillColor=#ff0000") || !strings.Contains(c.Style, "rounded=0") {
		t.Errorf("style = %s", c.Style)
	}
	g := c.Geometry
	if g.X != 10 || g.Y != 20 || g.W != 30 || g.H != 40 {
		t.Errorf("geometry = %+v, want {10 20 30 40}", g)
	}
}

func TestRoundedRectUsesAbsoluteArcSize(t *testing.T) {
	cells := cellsOf(t, svgWith(`<rect x="0" y="0" width="50" height="20" rx="6"/>`))
	style := cells[0].Style
	if !strings.Contains(style, "rounded=1") || !strings.Contains(style, "absoluteArcSize=1") ||
		!strings.Contains(style, "arcSize=6") {
		t.Errorf("style = %s", style)
	}
}

func TestCircleBecomesNativeEllipse(t *testing.T) {
	cells := cellsOf(t, svgWith(`<circle cx="50" cy="50" r="20" fill="none" stroke="#00ff00" stroke-width="4"/>`))
	c := cells[0]
	if !strings.HasPrefix(c.Style, "ellipse;") {
		t.Errorf("style = %s", c.Style)
	}
	if !strings.Contains(c.Style, "fillColor=none") || !strings.Contains(c.Style, "strokeWidth=4") {
		t.Errorf("style = %s", c.Style)
	}
	g := c.Geometry
	if g.X != 30 || g.Y != 30 || g.W != 40 || g.H != 40 {
		t.Errorf("geometry = %+v, want {30 30 40 40}", g)
	}
}

func TestRotatedRectKeepsNativeShapeWithRotation(t *testing.T) {
	cells := cellsOf(t, svgWith(`<g transform="rotate(45,50,50)"><rect x="40" y="45" width="20" height="10"/></g>`))
	c := cells[0]
	if strings.Contains(c.Style, "shape=stencil") {
		t.Errorf("a rotated rectangle should stay native: %s", c.Style)
	}
	if !strings.Contains(c.Style, "rotation=45") {
		t.Errorf("style = %s", c.Style)
	}
	// The centre is on the rotation origin, so the geometry is unchanged.
	if math.Abs(c.Geometry.X-40) > 0.01 || math.Abs(c.Geometry.Y-45) > 0.01 {
		t.Errorf("geometry = %+v, want x=40 y=45", c.Geometry)
	}
}

func TestSkewedRectFallsBackToStencil(t *testing.T) {
	cells := cellsOf(t, svgWith(`<rect x="0" y="0" width="20" height="10" transform="skewX(20)"/>`))
	if !strings.Contains(cells[0].Style, "shape=stencil(") {
		t.Errorf("a skewed rectangle cannot stay native: %s", cells[0].Style)
	}
}

func TestPathBecomesInlineStencilAtTheRightPlace(t *testing.T) {
	cells := cellsOf(t, svgWith(`<path d="M20,30 L60,30 L60,70 Z" fill="#123456"/>`))
	c := cells[0]
	if !strings.Contains(c.Style, "shape=stencil(") {
		t.Fatalf("style = %s", c.Style)
	}
	g := c.Geometry
	if g.X != 20 || g.Y != 30 || g.W != 40 || g.H != 40 {
		t.Errorf("geometry = %+v, want {20 30 40 40}", g)
	}

	xml := decodeStencil(t, c.Style)
	if !strings.Contains(xml, `w="40" h="40"`) {
		t.Errorf("stencil size does not match the geometry: %s", xml)
	}
	// Stencil coordinates are local to the cell, so the path starts at 0,0.
	if !strings.Contains(xml, `<move x="0" y="0"/>`) {
		t.Errorf("stencil path is not rebased on the cell origin: %s", xml)
	}
	if !strings.Contains(xml, "<fill/>") {
		t.Errorf("a filled path without a stroke should only fill: %s", xml)
	}
}

func TestStraightStrokedPathBecomesEdge(t *testing.T) {
	cells := cellsOf(t, svgWith(`<path d="M10,10 L50,10 L50,40" fill="none" stroke="#000" stroke-width="2"/>`))
	c := cells[0]
	if !c.Edge {
		t.Fatalf("a straight connector path should become an edge: %s", c.Style)
	}
	if c.Geometry.Source.X != 10 || c.Geometry.Target.Y != 40 {
		t.Errorf("endpoints = %+v / %+v", c.Geometry.Source, c.Geometry.Target)
	}
	if len(c.Geometry.Points) != 1 {
		t.Errorf("waypoints = %+v, want 1", c.Geometry.Points)
	}
}

func TestCurvedStrokedPathStaysAShape(t *testing.T) {
	cells := cellsOf(t, svgWith(`<path d="M10,10 C20,0 40,20 50,10" fill="none" stroke="#000"/>`))
	if cells[0].Edge {
		t.Error("a curved path cannot be expressed as an edge")
	}
}

func TestThinPathKeepsASelectableBox(t *testing.T) {
	opt := DefaultOptions()
	opt.Edges = false
	f, err := Diagram(strings.NewReader(svgWith(`<path d="M10,10 L90,10" fill="none" stroke="#000" stroke-width="2"/>`)), "t.svg", opt)
	if err != nil {
		t.Fatal(err)
	}
	g := f.Models[0].Cells[0].Geometry
	if g.H < 2 {
		t.Errorf("height = %v, want at least the stroke width", g.H)
	}
	if g.W != 80 {
		t.Errorf("width = %v, want 80", g.W)
	}
}

func TestMarkersBecomeArrowheads(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<defs><marker id="a"><path d="M0 0 L10 5 L0 10 z"/></marker></defs>
		<path d="M10,10 L90,10" fill="none" stroke="#000" marker-end="url(#a)"/>`))
	style := cells[0].Style
	if !strings.Contains(style, "endArrow=block") || !strings.Contains(style, "endFill=1") {
		t.Errorf("style = %s", style)
	}
}

func TestCSSClassesAreApplied(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<style>.box { fill: #ff0000; stroke: #00ff00; stroke-width: 3; }</style>
		<rect class="box" x="0" y="0" width="10" height="10"/>`))
	style := cells[0].Style
	if !strings.Contains(style, "fillColor=#ff0000") || !strings.Contains(style, "strokeColor=#00ff00") ||
		!strings.Contains(style, "strokeWidth=3") {
		t.Errorf("style = %s", style)
	}
}

func TestStyleAttributeBeatsCSSRule(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<style>.box { fill: #ff0000; }</style>
		<rect class="box" style="fill:#0000ff" x="0" y="0" width="10" height="10"/>`))
	if !strings.Contains(cells[0].Style, "fillColor=#0000ff") {
		t.Errorf("style = %s", cells[0].Style)
	}
}

func TestCSSRuleBeatsPresentationAttribute(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<style>.box { fill: #ff0000; }</style>
		<rect class="box" fill="#00ff00" x="0" y="0" width="10" height="10"/>`))
	if !strings.Contains(cells[0].Style, "fillColor=#ff0000") {
		t.Errorf("style = %s", cells[0].Style)
	}
}

func TestCSSTextRulesReachTextCells(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<style>text.t { font-size: 22px; font-weight: 700; fill: #0f172a; }</style>
		<text class="t" x="10" y="20">Title</text>`))
	style := cells[0].Style
	if !strings.Contains(style, "fontSize=22") || !strings.Contains(style, "fontStyle=1") ||
		!strings.Contains(style, "fontColor=#0f172a") {
		t.Errorf("style = %s", style)
	}
}

func TestLineBecomesEdge(t *testing.T) {
	cells := cellsOf(t, svgWith(`<line x1="10" y1="10" x2="90" y2="50" stroke="#000"/>`))
	c := cells[0]
	if !c.Edge {
		t.Fatalf("a line should become an edge: %+v", c)
	}
	if c.Geometry.Source.X != 10 || c.Geometry.Target.Y != 50 {
		t.Errorf("endpoints = %+v / %+v", c.Geometry.Source, c.Geometry.Target)
	}
	if !strings.Contains(c.Style, "endArrow=none") {
		t.Errorf("style = %s", c.Style)
	}
}

func TestPolylineBecomesEdgeWithWaypoints(t *testing.T) {
	cells := cellsOf(t, svgWith(`<polyline points="0,0 10,10 20,0 30,10" fill="none" stroke="#000"/>`))
	c := cells[0]
	if !c.Edge {
		t.Fatalf("an open polyline should become an edge")
	}
	if len(c.Geometry.Points) != 2 {
		t.Errorf("waypoints = %+v, want 2", c.Geometry.Points)
	}
}

func TestFilledPolygonStaysAShape(t *testing.T) {
	cells := cellsOf(t, svgWith(`<polygon points="0,0 10,0 5,10" fill="#abc"/>`))
	if cells[0].Edge {
		t.Error("a filled polygon must not become an edge")
	}
	if !strings.Contains(cells[0].Style, "shape=stencil(") {
		t.Errorf("style = %s", cells[0].Style)
	}
}

func TestGradientFillKeepsBothStops(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<defs><linearGradient id="g" x1="0" y1="0" x2="0" y2="1">
			<stop offset="0" stop-color="#ffffff"/><stop offset="1" stop-color="#000000"/>
		</linearGradient></defs>
		<rect x="0" y="0" width="10" height="10" fill="url(#g)"/>`))
	style := cells[0].Style
	if !strings.Contains(style, "fillColor=#ffffff") ||
		!strings.Contains(style, "gradientColor=#000000") ||
		!strings.Contains(style, "gradientDirection=south") {
		t.Errorf("style = %s", style)
	}
}

func TestGradientStopsInheritedThroughHref(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<defs>
			<linearGradient id="base"><stop offset="0" stop-color="#112233"/><stop offset="1" stop-color="#445566"/></linearGradient>
			<linearGradient id="derived" xlink:href="#base" x1="0" y1="0" x2="1" y2="0"/>
		</defs>
		<rect x="0" y="0" width="10" height="10" fill="url(#derived)"/>`))
	style := cells[0].Style
	if !strings.Contains(style, "fillColor=#112233") || !strings.Contains(style, "gradientDirection=east") {
		t.Errorf("style = %s", style)
	}
}

func TestStyleInheritanceFromGroup(t *testing.T) {
	cells := cellsOf(t, svgWith(`<g fill="#00ff00" stroke="#ff0000"><rect x="0" y="0" width="10" height="10"/></g>`))
	if !strings.Contains(cells[0].Style, "fillColor=#00ff00") ||
		!strings.Contains(cells[0].Style, "strokeColor=#ff0000") {
		t.Errorf("style = %s", cells[0].Style)
	}
}

func TestGroupsBecomeDrawioGroups(t *testing.T) {
	cells := cellsOf(t, svgWith(`<g id="pair">
		<rect x="10" y="10" width="20" height="20"/>
		<rect x="40" y="10" width="20" height="20"/></g>`))
	if len(cells) != 1 {
		t.Fatalf("got %d top level cells, want 1", len(cells))
	}
	group := cells[0]
	if !strings.HasPrefix(group.Style, "group;") {
		t.Fatalf("style = %s", group.Style)
	}
	g := group.Geometry
	if g.X != 10 || g.Y != 10 || g.W != 50 || g.H != 20 {
		t.Errorf("group geometry = %+v, want {10 10 50 20}", g)
	}
	if len(group.Children) != 2 {
		t.Fatalf("got %d children", len(group.Children))
	}
	// Children are positioned relative to the group origin.
	if group.Children[0].Geometry.X != 0 || group.Children[1].Geometry.X != 30 {
		t.Errorf("children not rebased: %+v %+v", group.Children[0].Geometry, group.Children[1].Geometry)
	}
	for _, child := range group.Children {
		if child.Parent != group.ID {
			t.Errorf("child parent = %q, want %q", child.Parent, group.ID)
		}
	}
}

func TestGroupsCanBeFlattened(t *testing.T) {
	opt := DefaultOptions()
	opt.Groups = false
	f, err := Diagram(strings.NewReader(svgWith(`<g><rect x="0" y="0" width="5" height="5"/><rect x="9" y="0" width="5" height="5"/></g>`)), "t.svg", opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Models[0].Cells) != 2 {
		t.Errorf("got %d cells, want 2 flattened", len(f.Models[0].Cells))
	}
}

func TestHiddenElementsAreSkipped(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<rect x="0" y="0" width="5" height="5" display="none"/>
		<rect x="0" y="0" width="5" height="5" visibility="hidden"/>
		<defs><rect x="0" y="0" width="5" height="5"/></defs>`))
	if len(cells) != 0 {
		t.Errorf("got %d cells, want none", len(cells))
	}
}

func TestTextCellCarriesFontAndLabel(t *testing.T) {
	cells := cellsOf(t, svgWith(`<text x="50" y="50" text-anchor="middle" font-size="20" font-weight="bold" fill="#333333">Hi &amp; bye</text>`))
	c := cells[0]
	if c.Value != "Hi &amp; bye" {
		t.Errorf("value = %q", c.Value)
	}
	if !strings.Contains(c.Style, "fontSize=20") || !strings.Contains(c.Style, "fontStyle=1") ||
		!strings.Contains(c.Style, "align=center") || !strings.Contains(c.Style, "fontColor=#333333") {
		t.Errorf("style = %s", c.Style)
	}
}

func TestTspanWithOwnCoordinatesBecomesItsOwnCell(t *testing.T) {
	cells := cellsOf(t, svgWith(`<text x="10" y="10">first<tspan x="10" y="30">second</tspan></text>`))
	if len(cells) != 2 {
		t.Fatalf("got %d cells, want 2", len(cells))
	}
	if cells[0].Value != "first" || cells[1].Value != "second" {
		t.Errorf("values = %q, %q", cells[0].Value, cells[1].Value)
	}
	if cells[1].Geometry.Y <= cells[0].Geometry.Y {
		t.Error("the tspan should sit below its parent text")
	}
}

func TestImageStripsBase64FromDataURI(t *testing.T) {
	cells := cellsOf(t, svgWith(`<image x="0" y="0" width="10" height="10" xlink:href="data:image/png;base64,AAAA"/>`))
	style := cells[0].Style
	if !strings.Contains(style, "image=data:image/png,AAAA") {
		t.Errorf("style = %s", style)
	}
	if strings.Contains(style, "base64") {
		t.Errorf("the semicolon before base64 would break the style: %s", style)
	}
}

func TestViewBoxScalesToWidthAndHeight(t *testing.T) {
	f := diagramFrom(t, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 100 100">
		<rect x="10" y="10" width="20" height="20"/></svg>`)
	g := f.Models[0].Cells[0].Geometry
	if g.X != 20 || g.W != 40 {
		t.Errorf("geometry = %+v, want the viewBox scaled by 2", g)
	}
}

func TestViewBoxOffsetIsRemoved(t *testing.T) {
	f := diagramFrom(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-50 -50 100 100">
		<rect x="-50" y="-50" width="20" height="20"/></svg>`)
	g := f.Models[0].Cells[0].Geometry
	if g.X != 0 || g.Y != 0 {
		t.Errorf("geometry = %+v, want the viewBox origin at 0,0", g)
	}
}

func TestScaleOption(t *testing.T) {
	opt := DefaultOptions()
	opt.Scale = 2
	f, err := Diagram(strings.NewReader(svgWith(`<rect x="10" y="10" width="20" height="20"/>`)), "t.svg", opt)
	if err != nil {
		t.Fatal(err)
	}
	g := f.Models[0].Cells[0].Geometry
	if g.W != 40 || g.X != 20 {
		t.Errorf("geometry = %+v, want everything doubled", g)
	}
}

func TestUseIsResolvedIntoRealCells(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<defs><rect id="r" x="0" y="0" width="10" height="10" fill="#0000ff"/></defs>
		<use xlink:href="#r" x="50" y="60"/>`))
	if len(cells) != 1 {
		t.Fatalf("got %d cells, want 1", len(cells))
	}
	g := cells[0].Geometry
	if g.X != 50 || g.Y != 60 {
		t.Errorf("geometry = %+v, want the use offset applied", g)
	}
	if !strings.Contains(cells[0].Style, "fillColor=#0000ff") {
		t.Errorf("style = %s", cells[0].Style)
	}
}

func TestStencilModeProducesOneShape(t *testing.T) {
	sh, err := StencilShape(strings.NewReader(svgWith(`
		<rect x="10" y="10" width="20" height="20" fill="#ff0000"/>
		<circle cx="50" cy="50" r="10" fill="none" stroke="#000"/>`)), "icon.svg", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if sh.W != 200 || sh.H != 200 {
		t.Errorf("stencil size = %vx%v, want the viewport", sh.W, sh.H)
	}
	xml := sh.XML(false)
	for _, want := range []string{
		`name="icon"`, `<fillcolor color="#ff0000"/>`, `<fill/>`,
		`<strokecolor color="#000000"/>`, `<stroke/>`, `<save/>`, `<restore/>`,
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("stencil XML missing %q:\n%s", want, xml)
		}
	}
}

func TestDocumentOrderIsPreserved(t *testing.T) {
	cells := cellsOf(t, svgWith(`
		<rect id="a" x="0" y="0" width="5" height="5"/>
		<circle id="b" cx="5" cy="5" r="2"/>
		<path id="c" d="M0 0 L5 5"/>`))
	if len(cells) != 3 {
		t.Fatalf("got %d cells", len(cells))
	}
	if !strings.HasPrefix(cells[1].Style, "ellipse;") {
		t.Errorf("z-order changed: %s", cells[1].Style)
	}
}

var stencilRe = regexp.MustCompile(`stencil\(([^)]+)\)`)

func decodeStencil(t *testing.T, style string) string {
	t.Helper()
	m := stencilRe.FindStringSubmatch(style)
	if m == nil {
		t.Fatalf("no inline stencil in style: %s", style)
	}
	xml, err := stencil.Decompress(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return xml
}
