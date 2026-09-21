package stencil

import (
	"strings"
	"testing"

	"github.com/bcollard/svg2drawio/internal/svgpath"
)

func TestShapeXML(t *testing.T) {
	sh := &Shape{
		Name: "demo", W: 100, H: 50,
		Foreground: []Elem{
			PathElem(svgpath.Parse("M0 0 L10 10 Z"), 2),
			{Name: "fillstroke"},
		},
	}
	got := sh.XML(false)
	for _, want := range []string{
		`<shape name="demo" w="100" h="50" aspect="variable" strokewidth="inherit">`,
		`<move x="0" y="0"/>`,
		`<line x="10" y="10"/>`,
		`<close/>`,
		`<fillstroke/>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stencil XML missing %q:\n%s", want, got)
		}
	}
}

func TestStyleValueRoundTrip(t *testing.T) {
	sh := &Shape{Name: "round trip & <escape>", W: 10, H: 10,
		Foreground: []Elem{PathElem(svgpath.Parse("M0 0 L5 5"), 2), {Name: "stroke"}}}

	value, err := sh.StyleValue()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(value, "stencil(") || !strings.HasSuffix(value, ")") {
		t.Fatalf("style value has the wrong shape: %q", value)
	}
	if strings.Contains(value, ";") {
		t.Error("a semicolon in the style value would terminate the style entry")
	}

	data := strings.TrimSuffix(strings.TrimPrefix(value, "stencil("), ")")
	back, err := Decompress(data)
	if err != nil {
		t.Fatal(err)
	}
	if back != sh.XML(false) {
		t.Errorf("round trip mismatch:\n got %s\nwant %s", back, sh.XML(false))
	}
}

func TestCompressIsURIEncodedBeforeDeflate(t *testing.T) {
	// draw.io runs decodeURIComponent after inflating, so the deflated payload
	// has to be percent encoded.
	data, err := Compress(`<shape name="é"/>`)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Decompress(data)
	if err != nil {
		t.Fatal(err)
	}
	if raw != `<shape name="é"/>` {
		t.Errorf("round trip = %q", raw)
	}
}

func TestNumAvoidsScientificNotation(t *testing.T) {
	if got := Num(0.0000001); strings.Contains(got, "e") {
		t.Errorf("Num = %q, should not use scientific notation", got)
	}
	if got := Num(12); got != "12" {
		t.Errorf("Num(12) = %q, want \"12\"", got)
	}
}

func TestEscapeAttributes(t *testing.T) {
	if got := Escape(`a & b < c "d"`); got != `a &amp; b &lt; c &quot;d&quot;` {
		t.Errorf("Escape = %q", got)
	}
}
