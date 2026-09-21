package mxgraph

import (
	"math"
	"strconv"
	"strings"
)

// Num formats a number for a draw.io attribute, avoiding scientific notation.
func Num(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if strings.Contains(s, "e") {
		s = strconv.FormatFloat(v, 'f', 6, 64)
	}
	return s
}
