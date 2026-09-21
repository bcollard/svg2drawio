package svgpath

import "math"

// Transformer applies an affine transform to a point.
type Transformer func(x, y float64) (float64, float64)

// Transform returns a copy of the path with fn applied to every coordinate.
func (p Path) Transform(fn Transformer) Path {
	out := make(Path, len(p))
	for i, seg := range p {
		args := make([]float64, len(seg.Args))
		for j := 0; j+1 < len(seg.Args); j += 2 {
			args[j], args[j+1] = fn(seg.Args[j], seg.Args[j+1])
		}
		out[i] = Segment{Op: seg.Op, Args: args}
	}
	return out
}

// Translate returns a copy of the path moved by (dx, dy).
func (p Path) Translate(dx, dy float64) Path {
	return p.Transform(func(x, y float64) (float64, float64) { return x + dx, y + dy })
}

// Rect is an axis aligned bounding box.
type Rect struct{ X, Y, W, H float64 }

// Empty reports whether the rectangle has no area in either direction.
func (r Rect) Empty() bool { return r.W <= 0 && r.H <= 0 }

// Union returns the smallest rectangle containing both.
func (r Rect) Union(o Rect) Rect {
	if r.W == 0 && r.H == 0 && r.X == 0 && r.Y == 0 {
		return o
	}
	if o.W == 0 && o.H == 0 && o.X == 0 && o.Y == 0 {
		return r
	}
	x0 := math.Min(r.X, o.X)
	y0 := math.Min(r.Y, o.Y)
	x1 := math.Max(r.X+r.W, o.X+o.W)
	y1 := math.Max(r.Y+r.H, o.Y+o.H)
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Grow expands the rectangle by d on every side.
func (r Rect) Grow(d float64) Rect {
	return Rect{r.X - d, r.Y - d, r.W + 2*d, r.H + 2*d}
}

// Bounds returns the tight bounding box of the path. Curve extrema are solved
// analytically rather than sampled, so the box is exact.
func (p Path) Bounds() Rect {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	seen := false

	add := func(x, y float64) {
		seen = true
		minX, minY = math.Min(minX, x), math.Min(minY, y)
		maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
	}

	var cur, start Point
	for _, seg := range p {
		switch seg.Op {
		case OpMove:
			cur = Point{seg.Args[0], seg.Args[1]}
			start = cur
			add(cur.X, cur.Y)
		case OpLine:
			cur = Point{seg.Args[0], seg.Args[1]}
			add(cur.X, cur.Y)
		case OpQuad:
			c := Point{seg.Args[0], seg.Args[1]}
			e := Point{seg.Args[2], seg.Args[3]}
			add(e.X, e.Y)
			for _, t := range quadExtrema(cur.X, c.X, e.X) {
				add(quadAt(cur.X, c.X, e.X, t), quadAt(cur.Y, c.Y, e.Y, t))
			}
			for _, t := range quadExtrema(cur.Y, c.Y, e.Y) {
				add(quadAt(cur.X, c.X, e.X, t), quadAt(cur.Y, c.Y, e.Y, t))
			}
			cur = e
		case OpCubic:
			c1 := Point{seg.Args[0], seg.Args[1]}
			c2 := Point{seg.Args[2], seg.Args[3]}
			e := Point{seg.Args[4], seg.Args[5]}
			add(e.X, e.Y)
			for _, t := range cubicExtrema(cur.X, c1.X, c2.X, e.X) {
				add(cubicAt(cur.X, c1.X, c2.X, e.X, t), cubicAt(cur.Y, c1.Y, c2.Y, e.Y, t))
			}
			for _, t := range cubicExtrema(cur.Y, c1.Y, c2.Y, e.Y) {
				add(cubicAt(cur.X, c1.X, c2.X, e.X, t), cubicAt(cur.Y, c1.Y, c2.Y, e.Y, t))
			}
			cur = e
		case OpClose:
			cur = start
		}
	}

	if !seen {
		return Rect{}
	}
	return Rect{minX, minY, maxX - minX, maxY - minY}
}

func quadAt(p0, p1, p2, t float64) float64 {
	mt := 1 - t
	return mt*mt*p0 + 2*mt*t*p1 + t*t*p2
}

func cubicAt(p0, p1, p2, p3, t float64) float64 {
	mt := 1 - t
	return mt*mt*mt*p0 + 3*mt*mt*t*p1 + 3*mt*t*t*p2 + t*t*t*p3
}

// quadExtrema returns the in-range roots of the quadratic derivative.
func quadExtrema(p0, p1, p2 float64) []float64 {
	den := p0 - 2*p1 + p2
	if math.Abs(den) < 1e-12 {
		return nil
	}
	t := (p0 - p1) / den
	if t > 0 && t < 1 {
		return []float64{t}
	}
	return nil
}

// cubicExtrema returns the in-range roots of the cubic derivative.
func cubicExtrema(p0, p1, p2, p3 float64) []float64 {
	a := -p0 + 3*p1 - 3*p2 + p3
	b := 2 * (p0 - 2*p1 + p2)
	c := p1 - p0

	var roots []float64
	if math.Abs(a) < 1e-12 {
		if math.Abs(b) > 1e-12 {
			roots = append(roots, -c/b)
		}
	} else {
		disc := b*b - 4*a*c
		if disc >= 0 {
			sq := math.Sqrt(disc)
			roots = append(roots, (-b+sq)/(2*a), (-b-sq)/(2*a))
		}
	}

	out := roots[:0]
	for _, t := range roots {
		if t > 0 && t < 1 {
			out = append(out, t)
		}
	}
	return out
}

// FromRect builds a path for an SVG rectangle, honouring rx/ry rounding.
func FromRect(x, y, w, h, rx, ry float64) Path {
	if rx <= 0 && ry <= 0 {
		return Path{
			{OpMove, []float64{x, y}},
			{OpLine, []float64{x + w, y}},
			{OpLine, []float64{x + w, y + h}},
			{OpLine, []float64{x, y + h}},
			{OpClose, nil},
		}
	}
	if rx <= 0 {
		rx = ry
	}
	if ry <= 0 {
		ry = rx
	}
	rx = math.Min(rx, w/2)
	ry = math.Min(ry, h/2)

	// kappa is the circle-to-bezier constant.
	const kappa = 0.5522847498307933
	cx, cy := rx*kappa, ry*kappa

	return Path{
		{OpMove, []float64{x + rx, y}},
		{OpLine, []float64{x + w - rx, y}},
		{OpCubic, []float64{x + w - rx + cx, y, x + w, y + ry - cy, x + w, y + ry}},
		{OpLine, []float64{x + w, y + h - ry}},
		{OpCubic, []float64{x + w, y + h - ry + cy, x + w - rx + cx, y + h, x + w - rx, y + h}},
		{OpLine, []float64{x + rx, y + h}},
		{OpCubic, []float64{x + rx - cx, y + h, x, y + h - ry + cy, x, y + h - ry}},
		{OpLine, []float64{x, y + ry}},
		{OpCubic, []float64{x, y + ry - cy, x + rx - cx, y, x + rx, y}},
		{OpClose, nil},
	}
}

// FromEllipse builds a path approximating an ellipse with four cubics.
func FromEllipse(cx, cy, rx, ry float64) Path {
	const kappa = 0.5522847498307933
	ox, oy := rx*kappa, ry*kappa
	return Path{
		{OpMove, []float64{cx - rx, cy}},
		{OpCubic, []float64{cx - rx, cy - oy, cx - ox, cy - ry, cx, cy - ry}},
		{OpCubic, []float64{cx + ox, cy - ry, cx + rx, cy - oy, cx + rx, cy}},
		{OpCubic, []float64{cx + rx, cy + oy, cx + ox, cy + ry, cx, cy + ry}},
		{OpCubic, []float64{cx - ox, cy + ry, cx - rx, cy + oy, cx - rx, cy}},
		{OpClose, nil},
	}
}

// FromPoints builds a polyline or polygon path.
func FromPoints(pts []float64, closed bool) Path {
	if len(pts) < 4 {
		return nil
	}
	out := Path{{OpMove, []float64{pts[0], pts[1]}}}
	for i := 2; i+1 < len(pts); i += 2 {
		out = append(out, Segment{OpLine, []float64{pts[i], pts[i+1]}})
	}
	if closed {
		out = append(out, Segment{Op: OpClose})
	}
	return out
}

// FromLine builds a two point path.
func FromLine(x1, y1, x2, y2 float64) Path {
	return Path{
		{OpMove, []float64{x1, y1}},
		{OpLine, []float64{x2, y2}},
	}
}
