package svgpath

import (
	"math"
	"testing"
)

func TestParseAbsoluteAndRelative(t *testing.T) {
	p := Parse("M10 10 l10 0 L30 30 h10 v10 Z")
	want := []struct {
		op   Op
		args []float64
	}{
		{OpMove, []float64{10, 10}},
		{OpLine, []float64{20, 10}},
		{OpLine, []float64{30, 30}},
		{OpLine, []float64{40, 30}},
		{OpLine, []float64{40, 40}},
		{OpClose, nil},
	}
	if len(p) != len(want) {
		t.Fatalf("got %d segments, want %d: %+v", len(p), len(want), p)
	}
	for i, w := range want {
		if p[i].Op != w.op {
			t.Errorf("segment %d: op %c, want %c", i, p[i].Op, w.op)
		}
		for j, a := range w.args {
			if math.Abs(p[i].Args[j]-a) > 1e-9 {
				t.Errorf("segment %d arg %d: %v, want %v", i, j, p[i].Args[j], a)
			}
		}
	}
}

func TestImplicitLinetoAfterMoveto(t *testing.T) {
	p := Parse("M0 0 10 0 10 10")
	if len(p) != 3 {
		t.Fatalf("got %d segments, want 3", len(p))
	}
	if p[1].Op != OpLine || p[2].Op != OpLine {
		t.Errorf("implicit repeats should be linetos: %+v", p)
	}
}

func TestSmoothCubicReflectsControlPoint(t *testing.T) {
	p := Parse("M0 0 C10 10 20 -10 30 0 S50 10 60 0")
	if len(p) != 3 {
		t.Fatalf("got %d segments, want 3", len(p))
	}
	// The reflection of (20,-10) about (30,0) is (40,10).
	if got := p[2].Args[0]; math.Abs(got-40) > 1e-9 {
		t.Errorf("reflected x1 = %v, want 40", got)
	}
	if got := p[2].Args[1]; math.Abs(got-10) > 1e-9 {
		t.Errorf("reflected y1 = %v, want 10", got)
	}
}

func TestArcBecomesCubicsEndingAtTarget(t *testing.T) {
	p := Parse("M0 50 A50 50 0 1 0 100 50")
	if len(p) < 2 {
		t.Fatalf("arc produced %d segments", len(p))
	}
	last := p[len(p)-1]
	if last.Op != OpCubic {
		t.Fatalf("last segment is %c, want a cubic", last.Op)
	}
	if math.Abs(last.Args[4]-100) > 1e-6 || math.Abs(last.Args[5]-50) > 1e-6 {
		t.Errorf("arc ends at (%v,%v), want (100,50)", last.Args[4], last.Args[5])
	}
	b := p.Bounds()
	// A half circle of radius 50 sweeping downwards.
	if math.Abs(b.X) > 1e-6 || math.Abs(b.W-100) > 1e-6 || math.Abs(b.H-50) > 1e-3 {
		t.Errorf("arc bounds = %+v, want x=0 w=100 h=50", b)
	}
}

func TestArcFlagsWithoutSeparators(t *testing.T) {
	p := Parse("M0 0a5 5 0 1125 25")
	if len(p) < 2 {
		t.Fatalf("compact arc flags not parsed: %+v", p)
	}
	last := p[len(p)-1]
	if math.Abs(last.Args[4]-25) > 1e-6 || math.Abs(last.Args[5]-25) > 1e-6 {
		t.Errorf("arc ends at (%v,%v), want (25,25)", last.Args[4], last.Args[5])
	}
}

func TestBoundsUsesCurveExtrema(t *testing.T) {
	// The curve bulges to y = -7.5 at its midpoint, past both endpoints.
	p := Parse("M0 0 C0 -10 10 -10 10 0")
	b := p.Bounds()
	if math.Abs(b.Y+7.5) > 1e-9 {
		t.Errorf("bounds y = %v, want -7.5", b.Y)
	}
	if math.Abs(b.H-7.5) > 1e-9 {
		t.Errorf("bounds h = %v, want 7.5", b.H)
	}
}

func TestEllipsePathBounds(t *testing.T) {
	p := FromEllipse(50, 50, 30, 20)
	b := p.Bounds()
	if math.Abs(b.X-20) > 1e-9 || math.Abs(b.Y-30) > 1e-9 ||
		math.Abs(b.W-60) > 1e-9 || math.Abs(b.H-40) > 1e-9 {
		t.Errorf("ellipse bounds = %+v, want {20 30 60 40}", b)
	}
}

func TestTransformAppliesToEveryCoordinate(t *testing.T) {
	p := Parse("M0 0 L10 0").Transform(func(x, y float64) (float64, float64) {
		return x*2 + 5, y*2 + 7
	})
	if p[0].Args[0] != 5 || p[0].Args[1] != 7 {
		t.Errorf("move = %v, want [5 7]", p[0].Args)
	}
	if p[1].Args[0] != 25 || p[1].Args[1] != 7 {
		t.Errorf("line = %v, want [25 7]", p[1].Args)
	}
}
