package svgdom

import (
	"strconv"
	"strings"
)

// Style holds the resolved presentation attributes of an element. Empty string
// and nil pointers mean "not set at this level", which is what makes
// inheritance work.
type Style struct {
	Fill        string // "#rrggbb", "none", or "url(#id)" when unresolved
	Stroke      string
	StrokeWidth *float64
	LineJoin    string
	LineCap     string
	MiterLimit  *float64
	DashArray   string
	Opacity     *float64
	FillOpacity *float64
	StrokeOpa   *float64
	FontFamily  string
	FontSize    *float64
	FontWeight  string
	FontStyle   string
	TextDecor   string
	TextAnchor  string
	Display     string
	Visibility  string
}

// inheritedProps are the SVG properties that flow down to children.
var inheritedProps = map[string]bool{
	"fill": true, "stroke": true, "stroke-width": true, "stroke-linejoin": true,
	"stroke-linecap": true, "stroke-miterlimit": true, "stroke-dasharray": true,
	"fill-opacity": true, "stroke-opacity": true, "font-family": true,
	"font-size": true, "font-weight": true, "font-style": true,
	"text-decoration": true, "text-anchor": true, "visibility": true,
}

// StyleOf reads the presentation attributes and the style="" attribute of a
// single node. The style attribute wins, as CSS specificity dictates.
func StyleOf(n *Node) Style {
	var s Style
	for k, v := range n.Attrs {
		s.set(k, v)
	}
	for k, v := range parseStyleAttr(n.Attr("style")) {
		s.set(k, v)
	}
	return s
}

// Inherit returns the child style resolved against its parent.
func (s Style) Inherit(parent Style) Style {
	out := s
	if out.Fill == "" {
		out.Fill = parent.Fill
	}
	if out.Stroke == "" {
		out.Stroke = parent.Stroke
	}
	if out.StrokeWidth == nil {
		out.StrokeWidth = parent.StrokeWidth
	}
	if out.LineJoin == "" {
		out.LineJoin = parent.LineJoin
	}
	if out.LineCap == "" {
		out.LineCap = parent.LineCap
	}
	if out.MiterLimit == nil {
		out.MiterLimit = parent.MiterLimit
	}
	if out.DashArray == "" {
		out.DashArray = parent.DashArray
	}
	if out.FillOpacity == nil {
		out.FillOpacity = parent.FillOpacity
	}
	if out.StrokeOpa == nil {
		out.StrokeOpa = parent.StrokeOpa
	}
	if out.FontFamily == "" {
		out.FontFamily = parent.FontFamily
	}
	if out.FontSize == nil {
		out.FontSize = parent.FontSize
	}
	if out.FontWeight == "" {
		out.FontWeight = parent.FontWeight
	}
	if out.FontStyle == "" {
		out.FontStyle = parent.FontStyle
	}
	if out.TextDecor == "" {
		out.TextDecor = parent.TextDecor
	}
	if out.TextAnchor == "" {
		out.TextAnchor = parent.TextAnchor
	}
	if out.Visibility == "" {
		out.Visibility = parent.Visibility
	}

	// opacity is not inherited, it compounds on the group as a whole; the
	// flattener multiplies it into the children instead.
	if parent.Opacity != nil {
		v := *parent.Opacity
		if out.Opacity != nil {
			v *= *out.Opacity
		}
		out.Opacity = &v
	}
	return out
}

// Hidden reports whether the element must not be rendered.
func (s Style) Hidden() bool {
	return s.Display == "none" || s.Visibility == "hidden" || s.Visibility == "collapse"
}

func (s *Style) set(name, value string) {
	value = strings.TrimSpace(value)
	if value == "" || value == "inherit" {
		return
	}
	switch name {
	case "fill":
		s.Fill = value
	case "stroke":
		s.Stroke = value
	case "stroke-width":
		s.StrokeWidth = floatPtr(value)
	case "stroke-linejoin":
		s.LineJoin = value
	case "stroke-linecap":
		s.LineCap = value
	case "stroke-miterlimit":
		s.MiterLimit = floatPtr(value)
	case "stroke-dasharray":
		if value != "none" {
			s.DashArray = value
		}
	case "opacity":
		s.Opacity = floatPtr(value)
	case "fill-opacity":
		s.FillOpacity = floatPtr(value)
	case "stroke-opacity":
		s.StrokeOpa = floatPtr(value)
	case "font-family":
		s.FontFamily = cleanFontFamily(value)
	case "font-size":
		s.FontSize = fontSizePtr(value)
	case "font-weight":
		s.FontWeight = value
	case "font-style":
		s.FontStyle = value
	case "text-decoration":
		s.TextDecor = value
	case "text-anchor":
		s.TextAnchor = value
	case "display":
		s.Display = value
	case "visibility":
		s.Visibility = value
	}
}

// Bold reports whether the font weight renders as bold.
func (s Style) Bold() bool {
	switch s.FontWeight {
	case "bold", "bolder", "600", "700", "800", "900":
		return true
	}
	return false
}

// Italic reports whether the font style renders as italic.
func (s Style) Italic() bool {
	return s.FontStyle == "italic" || s.FontStyle == "oblique"
}

// Underline reports whether the text is underlined.
func (s Style) Underline() bool {
	return strings.Contains(s.TextDecor, "underline")
}

func parseStyleAttr(s string) map[string]string {
	out := map[string]string{}
	for _, decl := range strings.Split(s, ";") {
		colon := strings.IndexByte(decl, ':')
		if colon < 0 {
			continue
		}
		k := strings.TrimSpace(decl[:colon])
		v := strings.TrimSpace(decl[colon+1:])
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func floatPtr(v string) *float64 {
	f, ok := Length(v)
	if !ok {
		return nil
	}
	return &f
}

// namedFontSizes are the CSS absolute-size keywords, in pixels.
var namedFontSizes = map[string]float64{
	"xx-small": 9, "x-small": 10, "small": 13, "medium": 16,
	"large": 18, "x-large": 24, "xx-large": 32,
}

func fontSizePtr(v string) *float64 {
	if f, ok := namedFontSizes[strings.ToLower(strings.TrimSpace(v))]; ok {
		return &f
	}
	return floatPtr(v)
}

func cleanFontFamily(v string) string {
	first := strings.Split(v, ",")[0]
	first = strings.TrimSpace(first)
	first = strings.Trim(first, `"'`)
	return first
}

// DashPattern converts an SVG stroke-dasharray to the space separated form
// mxGraph uses, scaled by the given factor.
func DashPattern(v string, scale float64) string {
	nums := Numbers(v)
	if len(nums) == 0 {
		return ""
	}
	parts := make([]string, 0, len(nums))
	for _, n := range nums {
		parts = append(parts, strconv.FormatFloat(round(n*scale, 2), 'f', -1, 64))
	}
	return strings.Join(parts, " ")
}

func round(v float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	return float64(int64(v*pow+copySign(0.5, v))) / pow
}

func copySign(mag, sign float64) float64 {
	if sign < 0 {
		return -mag
	}
	return mag
}
