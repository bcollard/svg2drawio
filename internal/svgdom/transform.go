package svgdom

import (
	"math"
	"strconv"
	"strings"
)

// Matrix is an SVG affine transform [a b c d e f]:
//
//	| a c e |
//	| b d f |
//	| 0 0 1 |
type Matrix struct{ A, B, C, D, E, F float64 }

// Identity is the neutral transform.
var Identity = Matrix{1, 0, 0, 1, 0, 0}

// Mul returns m*o, i.e. o applied first, then m.
func (m Matrix) Mul(o Matrix) Matrix {
	return Matrix{
		A: m.A*o.A + m.C*o.B,
		B: m.B*o.A + m.D*o.B,
		C: m.A*o.C + m.C*o.D,
		D: m.B*o.C + m.D*o.D,
		E: m.A*o.E + m.C*o.F + m.E,
		F: m.B*o.E + m.D*o.F + m.F,
	}
}

// Apply transforms a point.
func (m Matrix) Apply(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.E, m.B*x + m.D*y + m.F
}

// IsIdentity reports whether the matrix leaves points untouched.
func (m Matrix) IsIdentity() bool {
	return nearly(m.A, 1) && nearly(m.B, 0) && nearly(m.C, 0) &&
		nearly(m.D, 1) && nearly(m.E, 0) && nearly(m.F, 0)
}

// IsAxisAligned reports whether the matrix is a translate/scale only, which is
// the case where rectangles and ellipses survive as native draw.io shapes.
func (m Matrix) IsAxisAligned() bool {
	return nearly(m.B, 0) && nearly(m.C, 0)
}

// ScaleFactor returns the average scaling applied by the matrix, used to keep
// stroke widths proportional.
func (m Matrix) ScaleFactor() float64 {
	sx := math.Hypot(m.A, m.B)
	sy := math.Hypot(m.C, m.D)
	return (sx + sy) / 2
}

// Rotation returns the rotation of the matrix in degrees.
func (m Matrix) Rotation() float64 {
	return math.Atan2(m.B, m.A) * 180 / math.Pi
}

func nearly(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// ParseTransform parses an SVG transform attribute into a single matrix.
func ParseTransform(s string) Matrix {
	m := Identity
	for _, fn := range splitTransforms(s) {
		m = m.Mul(fn)
	}
	return m
}

func splitTransforms(s string) []Matrix {
	var out []Matrix
	rest := s
	for {
		open := strings.IndexByte(rest, '(')
		if open < 0 {
			break
		}
		closing := strings.IndexByte(rest[open:], ')')
		if closing < 0 {
			break
		}
		closing += open

		name := strings.TrimSpace(rest[:open])
		name = strings.Trim(name, ", \t\r\n")
		args := parseFloats(rest[open+1 : closing])
		rest = rest[closing+1:]

		if m, ok := transformMatrix(name, args); ok {
			out = append(out, m)
		}
	}
	return out
}

func transformMatrix(name string, a []float64) (Matrix, bool) {
	switch name {
	case "matrix":
		if len(a) >= 6 {
			return Matrix{a[0], a[1], a[2], a[3], a[4], a[5]}, true
		}
	case "translate":
		switch {
		case len(a) >= 2:
			return Matrix{1, 0, 0, 1, a[0], a[1]}, true
		case len(a) == 1:
			return Matrix{1, 0, 0, 1, a[0], 0}, true
		}
	case "scale":
		switch {
		case len(a) >= 2:
			return Matrix{a[0], 0, 0, a[1], 0, 0}, true
		case len(a) == 1:
			return Matrix{a[0], 0, 0, a[0], 0, 0}, true
		}
	case "rotate":
		if len(a) >= 1 {
			rad := a[0] * math.Pi / 180
			cos, sin := math.Cos(rad), math.Sin(rad)
			r := Matrix{cos, sin, -sin, cos, 0, 0}
			if len(a) >= 3 {
				to := Matrix{1, 0, 0, 1, a[1], a[2]}
				from := Matrix{1, 0, 0, 1, -a[1], -a[2]}
				return to.Mul(r).Mul(from), true
			}
			return r, true
		}
	case "skewX":
		if len(a) >= 1 {
			return Matrix{1, 0, math.Tan(a[0] * math.Pi / 180), 1, 0, 0}, true
		}
	case "skewY":
		if len(a) >= 1 {
			return Matrix{1, math.Tan(a[0] * math.Pi / 180), 0, 1, 0, 0}, true
		}
	}
	return Identity, false
}

func parseFloats(s string) []float64 {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

// Length parses an SVG length, dropping the unit. Percentages return ok=false
// because they need a viewport to resolve.
func Length(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "%") {
		return 0, false
	}
	for _, unit := range []string{"px", "pt", "pc", "mm", "cm", "in", "em", "ex"} {
		if strings.HasSuffix(s, unit) {
			v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, unit)), 64)
			if err != nil {
				return 0, false
			}
			return v * unitScale(unit), true
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// LengthOr parses a length, falling back to def.
func LengthOr(s string, def float64) float64 {
	if v, ok := Length(s); ok {
		return v
	}
	return def
}

func unitScale(unit string) float64 {
	switch unit {
	case "pt":
		return 96.0 / 72.0
	case "pc":
		return 16
	case "mm":
		return 96.0 / 25.4
	case "cm":
		return 96.0 / 2.54
	case "in":
		return 96
	case "em", "ex":
		return 16
	}
	return 1
}

// Numbers parses a whitespace/comma separated list of numbers.
func Numbers(s string) []float64 { return parseFloats(s) }
