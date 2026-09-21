package convert

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"

	"github.com/bcollard/svg2drawio/internal/stencil"
)

// TestSampleFile converts the bundled sample end to end and checks that the
// result is well formed XML whose inline stencils all decode.
func TestSampleFile(t *testing.T) {
	f, err := os.Open("../../testdata/sample.svg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	file, err := Diagram(f, "sample.svg", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out := file.XML()

	if err := xml.Unmarshal([]byte(out), new(struct {
		XMLName xml.Name `xml:"mxfile"`
	})); err != nil {
		t.Fatalf("output is not well formed: %v", err)
	}

	matches := stencilRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		t.Fatal("no inline stencils were produced")
	}
	for _, m := range matches {
		decoded, err := stencil.Decompress(m[1])
		if err != nil {
			t.Fatalf("stencil does not decode: %v", err)
		}
		if !strings.HasPrefix(decoded, "<shape ") {
			t.Errorf("decoded stencil is not a shape: %s", decoded)
		}
		if err := xml.Unmarshal([]byte(decoded), new(struct {
			XMLName xml.Name `xml:"shape"`
		})); err != nil {
			t.Errorf("stencil XML is not well formed: %v", err)
		}
	}

	cells := file.Models[0].Cells
	if len(cells) < 10 {
		t.Errorf("got %d top level cells, expected the whole drawing", len(cells))
	}
	if file.Models[0].Name != "sample" {
		t.Errorf("page name = %q", file.Models[0].Name)
	}
}

func TestSampleAsStencilLibrary(t *testing.T) {
	f, err := os.Open("../../testdata/sample.svg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sh, err := StencilShape(f, "sample.svg", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	lib := &stencil.Library{Name: "samples", Shapes: []*stencil.Shape{sh}}
	if err := xml.Unmarshal([]byte(lib.XML()), new(struct {
		XMLName xml.Name `xml:"shapes"`
	})); err != nil {
		t.Fatalf("library is not well formed: %v", err)
	}
}
