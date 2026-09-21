// Package svgpath parses SVG path data into absolute segments that can be
// transformed, measured and written out as mxGraph stencil path commands.
package svgpath

import (
	"math"
	"strconv"
	"strings"
)

// Op identifies a segment type. Every segment is absolute; H, V, S and T are
// resolved at parse time and arcs are converted to cubic curves so that a
// single affine transform can be applied to all of them.
type Op byte

const (
	OpMove  Op = 'M'
	OpLine  Op = 'L'
	OpCubic Op = 'C'
	OpQuad  Op = 'Q'
	OpClose Op = 'Z'
)

// Segment is one absolute path command. Args holds the coordinate pairs:
// 2 values for move/line, 4 for quad, 6 for cubic, none for close.
type Segment struct {
	Op   Op
	Args []float64
}

// Path is a sequence of absolute segments.
type Path []Segment

type parser struct {
	d          string
	i          int
	cur        Point
	start      Point
	lastCtrl   Point
	lastOp     byte
	out        Path
	hasLastCtl bool
}

// Point is a 2D coordinate.
type Point struct{ X, Y float64 }

// Parse converts SVG path data into absolute segments.
func Parse(d string) Path {
	p := &parser{d: d}
	p.run()
	return p.out
}

func (p *parser) run() {
	var op byte
	for {
		p.skipSep()
		if p.i >= len(p.d) {
			return
		}
		c := p.d[p.i]
		if isCommand(c) {
			op = c
			p.i++
		} else if op == 0 {
			return // data before any command letter
		} else {
			// An implicit repeat: after moveto the repeat is lineto.
			switch op {
			case 'm':
				op = 'l'
			case 'M':
				op = 'L'
			}
		}
		if !p.emit(op) {
			return
		}
	}
}

func (p *parser) emit(op byte) bool {
	rel := op >= 'a'
	switch upper(op) {
	case 'M':
		x, y, ok := p.coord(rel, p.cur)
		if !ok {
			return false
		}
		p.cur = Point{x, y}
		p.start = p.cur
		p.add(OpMove, x, y)
	case 'L':
		x, y, ok := p.coord(rel, p.cur)
		if !ok {
			return false
		}
		p.cur = Point{x, y}
		p.add(OpLine, x, y)
	case 'H':
		v, ok := p.number()
		if !ok {
			return false
		}
		if rel {
			v += p.cur.X
		}
		p.cur.X = v
		p.add(OpLine, p.cur.X, p.cur.Y)
	case 'V':
		v, ok := p.number()
		if !ok {
			return false
		}
		if rel {
			v += p.cur.Y
		}
		p.cur.Y = v
		p.add(OpLine, p.cur.X, p.cur.Y)
	case 'C':
		x1, y1, ok1 := p.coord(rel, p.cur)
		x2, y2, ok2 := p.coord(rel, p.cur)
		x, y, ok3 := p.coord(rel, p.cur)
		if !ok1 || !ok2 || !ok3 {
			return false
		}
		p.cubic(x1, y1, x2, y2, x, y)
	case 'S':
		x2, y2, ok1 := p.coord(rel, p.cur)
		x, y, ok2 := p.coord(rel, p.cur)
		if !ok1 || !ok2 {
			return false
		}
		c1 := p.reflectedCubicCtrl()
		p.cubic(c1.X, c1.Y, x2, y2, x, y)
	case 'Q':
		x1, y1, ok1 := p.coord(rel, p.cur)
		x, y, ok2 := p.coord(rel, p.cur)
		if !ok1 || !ok2 {
			return false
		}
		p.quad(x1, y1, x, y)
	case 'T':
		x, y, ok := p.coord(rel, p.cur)
		if !ok {
			return false
		}
		c := p.reflectedQuadCtrl()
		p.quad(c.X, c.Y, x, y)
	case 'A':
		rx, ok1 := p.number()
		ry, ok2 := p.number()
		rot, ok3 := p.number()
		laf, ok4 := p.flag()
		sf, ok5 := p.flag()
		x, y, ok6 := p.coord(rel, p.cur)
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
			return false
		}
		p.arc(rx, ry, rot, laf, sf, x, y)
	case 'Z':
		p.out = append(p.out, Segment{Op: OpClose})
		p.cur = p.start
		p.hasLastCtl = false
		p.lastOp = 'Z'
	default:
		return false
	}
	return true
}

func (p *parser) add(op Op, args ...float64) {
	p.out = append(p.out, Segment{Op: op, Args: args})
	p.hasLastCtl = false
	p.lastOp = byte(op)
}

func (p *parser) cubic(x1, y1, x2, y2, x, y float64) {
	p.out = append(p.out, Segment{Op: OpCubic, Args: []float64{x1, y1, x2, y2, x, y}})
	p.cur = Point{x, y}
	p.lastCtrl = Point{x2, y2}
	p.hasLastCtl = true
	p.lastOp = 'C'
}

func (p *parser) quad(x1, y1, x, y float64) {
	p.out = append(p.out, Segment{Op: OpQuad, Args: []float64{x1, y1, x, y}})
	p.cur = Point{x, y}
	p.lastCtrl = Point{x1, y1}
	p.hasLastCtl = true
	p.lastOp = 'Q'
}

func (p *parser) reflectedCubicCtrl() Point {
	if p.hasLastCtl && p.lastOp == 'C' {
		return Point{2*p.cur.X - p.lastCtrl.X, 2*p.cur.Y - p.lastCtrl.Y}
	}
	return p.cur
}

func (p *parser) reflectedQuadCtrl() Point {
	if p.hasLastCtl && p.lastOp == 'Q' {
		return Point{2*p.cur.X - p.lastCtrl.X, 2*p.cur.Y - p.lastCtrl.Y}
	}
	return p.cur
}

func (p *parser) coord(rel bool, base Point) (float64, float64, bool) {
	x, ok1 := p.number()
	y, ok2 := p.number()
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	if rel {
		x += base.X
		y += base.Y
	}
	return x, y, true
}

func (p *parser) skipSep() {
	for p.i < len(p.d) {
		switch p.d[p.i] {
		case ' ', '\t', '\r', '\n', ',':
			p.i++
		default:
			return
		}
	}
}

func (p *parser) number() (float64, bool) {
	p.skipSep()
	start := p.i
	if p.i < len(p.d) && (p.d[p.i] == '+' || p.d[p.i] == '-') {
		p.i++
	}
	for p.i < len(p.d) && (isDigit(p.d[p.i]) || p.d[p.i] == '.') {
		p.i++
	}
	if p.i < len(p.d) && (p.d[p.i] == 'e' || p.d[p.i] == 'E') {
		p.i++
		if p.i < len(p.d) && (p.d[p.i] == '+' || p.d[p.i] == '-') {
			p.i++
		}
		for p.i < len(p.d) && isDigit(p.d[p.i]) {
			p.i++
		}
	}
	if start == p.i {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(p.d[start:p.i], "."), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// flag reads an arc flag, which may be written without any separator, as in
// "a5 5 0 0125 25".
func (p *parser) flag() (bool, bool) {
	p.skipSep()
	if p.i < len(p.d) && (p.d[p.i] == '0' || p.d[p.i] == '1') {
		v := p.d[p.i] == '1'
		p.i++
		return v, true
	}
	v, ok := p.number()
	return v != 0, ok
}

func isCommand(c byte) bool {
	switch upper(c) {
	case 'M', 'L', 'H', 'V', 'C', 'S', 'Q', 'T', 'A', 'Z':
		return true
	}
	return false
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// arc converts an elliptical arc to cubic segments using the endpoint to
// center parameterization from the SVG specification.
func (p *parser) arc(rx, ry, rotDeg float64, largeArc, sweep bool, x, y float64) {
	x0, y0 := p.cur.X, p.cur.Y
	if x0 == x && y0 == y {
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		p.cur = Point{x, y}
		p.add(OpLine, x, y)
		return
	}

	phi := rotDeg * math.Pi / 180
	cosPhi, sinPhi := math.Cos(phi), math.Sin(phi)

	dx2, dy2 := (x0-x)/2, (y0-y)/2
	x1p := cosPhi*dx2 + sinPhi*dy2
	y1p := -sinPhi*dx2 + cosPhi*dy2

	// Scale the radii up if they are too small to span the endpoints.
	lambda := (x1p*x1p)/(rx*rx) + (y1p*y1p)/(ry*ry)
	if lambda > 1 {
		s := math.Sqrt(lambda)
		rx *= s
		ry *= s
	}

	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	factor := 0.0
	if den != 0 && num > 0 {
		factor = math.Sqrt(num / den)
	}
	if largeArc == sweep {
		factor = -factor
	}
	cxp := factor * rx * y1p / ry
	cyp := -factor * ry * x1p / rx

	cx := cosPhi*cxp - sinPhi*cyp + (x0+x)/2
	cy := sinPhi*cxp + cosPhi*cyp + (y0+y)/2

	theta1 := angle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	dTheta := angle((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && dTheta > 0 {
		dTheta -= 2 * math.Pi
	} else if sweep && dTheta < 0 {
		dTheta += 2 * math.Pi
	}

	segments := int(math.Ceil(math.Abs(dTheta) / (math.Pi / 2)))
	if segments == 0 {
		segments = 1
	}
	delta := dTheta / float64(segments)
	t := 4.0 / 3.0 * math.Tan(delta/4)

	theta := theta1
	for i := 0; i < segments; i++ {
		cosT1, sinT1 := math.Cos(theta), math.Sin(theta)
		theta2 := theta + delta
		cosT2, sinT2 := math.Cos(theta2), math.Sin(theta2)

		ex1, ey1 := ellipsePoint(cx, cy, rx, ry, cosPhi, sinPhi, cosT1, sinT1)
		ex2, ey2 := ellipsePoint(cx, cy, rx, ry, cosPhi, sinPhi, cosT2, sinT2)
		dx1, dy1 := ellipseDeriv(rx, ry, cosPhi, sinPhi, cosT1, sinT1)
		dx2b, dy2b := ellipseDeriv(rx, ry, cosPhi, sinPhi, cosT2, sinT2)

		p.cubic(ex1+t*dx1, ey1+t*dy1, ex2-t*dx2b, ey2-t*dy2b, ex2, ey2)
		theta = theta2
	}
	// The final point is set exactly to avoid drift from the approximation.
	last := &p.out[len(p.out)-1]
	last.Args[4], last.Args[5] = x, y
	p.cur = Point{x, y}
}

func ellipsePoint(cx, cy, rx, ry, cosPhi, sinPhi, cosT, sinT float64) (float64, float64) {
	return cx + rx*cosT*cosPhi - ry*sinT*sinPhi,
		cy + rx*cosT*sinPhi + ry*sinT*cosPhi
}

func ellipseDeriv(rx, ry, cosPhi, sinPhi, cosT, sinT float64) (float64, float64) {
	return -rx*sinT*cosPhi - ry*cosT*sinPhi,
		-rx*sinT*sinPhi + ry*cosT*cosPhi
}

func angle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	lens := math.Hypot(ux, uy) * math.Hypot(vx, vy)
	if lens == 0 {
		return 0
	}
	a := math.Acos(math.Max(-1, math.Min(1, dot/lens)))
	if ux*vy-uy*vx < 0 {
		return -a
	}
	return a
}
