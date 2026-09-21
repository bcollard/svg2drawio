package svgdom

import (
	"sort"
	"strings"
)

// Stylesheet holds the rules of the <style> elements of a document. Only the
// selector forms SVG files realistically use are supported: type, class and id
// selectors, compounds of those, and descendant or child combinators.
type Stylesheet struct {
	rules []rule
}

type rule struct {
	selector    selector
	decls       map[string]string
	specificity int
	order       int
}

// selector is a list of compounds read right to left, e.g. "g .box rect" is
// [rect, .box, g].
type selector struct {
	compounds []compound
	// child[i] is true when compound i must be the direct parent of i-1.
	child []bool
}

type compound struct {
	tag     string
	id      string
	classes []string
}

// ParseStylesheet reads CSS text into a stylesheet. Anything it cannot parse
// is skipped rather than failing the conversion.
func ParseStylesheet(css string) *Stylesheet {
	css = stripComments(css)
	sheet := &Stylesheet{}
	order := 0

	for len(css) > 0 {
		open := strings.IndexByte(css, '{')
		if open < 0 {
			break
		}
		close := matchBrace(css, open)
		if close < 0 {
			break
		}
		selectorText := strings.TrimSpace(css[:open])
		body := css[open+1 : close]
		css = css[close+1:]

		// At-rules such as @media wrap further rules; parse their body and
		// drop the condition, which is close enough for static documents.
		if strings.HasPrefix(selectorText, "@") {
			if strings.Contains(body, "{") {
				inner := ParseStylesheet(body)
				for _, r := range inner.rules {
					r.order = order
					order++
					sheet.rules = append(sheet.rules, r)
				}
			}
			continue
		}

		decls := parseDeclarations(body)
		if len(decls) == 0 {
			continue
		}
		for _, sel := range strings.Split(selectorText, ",") {
			s, spec, ok := parseSelector(sel)
			if !ok {
				continue
			}
			sheet.rules = append(sheet.rules, rule{
				selector: s, decls: decls, specificity: spec, order: order,
			})
			order++
		}
	}
	return sheet
}

// Match returns the declaration blocks that apply to a node, ordered from
// weakest to strongest so the caller can apply them in sequence.
func (s *Stylesheet) Match(n *Node) []map[string]string {
	if s == nil || len(s.rules) == 0 {
		return nil
	}
	var matched []rule
	for _, r := range s.rules {
		if r.selector.matches(n) {
			matched = append(matched, r)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].specificity != matched[j].specificity {
			return matched[i].specificity < matched[j].specificity
		}
		return matched[i].order < matched[j].order
	})

	out := make([]map[string]string, 0, len(matched))
	for _, r := range matched {
		out = append(out, r.decls)
	}
	return out
}

func (s selector) matches(n *Node) bool {
	if len(s.compounds) == 0 {
		return false
	}
	if !s.compounds[0].matches(n) {
		return false
	}
	node := n
	for i := 1; i < len(s.compounds); i++ {
		if s.child[i-1] {
			node = node.Parent
			if node == nil || !s.compounds[i].matches(node) {
				return false
			}
			continue
		}
		// Descendant: walk up until an ancestor matches.
		found := false
		for p := node.Parent; p != nil; p = p.Parent {
			if s.compounds[i].matches(p) {
				node = p
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (c compound) matches(n *Node) bool {
	if c.tag != "" && c.tag != "*" && c.tag != n.Tag {
		return false
	}
	if c.id != "" && c.id != n.Attr("id") {
		return false
	}
	if len(c.classes) > 0 {
		have := n.Classes()
		for _, want := range c.classes {
			if !contains(have, want) {
				return false
			}
		}
	}
	return true
}

// Classes returns the class list of the node.
func (n *Node) Classes() []string {
	return strings.Fields(n.Attr("class"))
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func parseSelector(sel string) (selector, int, bool) {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return selector{}, 0, false
	}
	// Pseudo classes and attribute selectors are out of scope.
	if strings.ContainsAny(sel, ":[]+~") {
		return selector{}, 0, false
	}

	sel = strings.ReplaceAll(sel, ">", " > ")
	parts := strings.Fields(sel)

	var compounds []compound
	var child []bool
	spec := 0
	nextIsChild := false

	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if p == ">" {
			nextIsChild = true
			continue
		}
		c, s, ok := parseCompound(p)
		if !ok {
			return selector{}, 0, false
		}
		if len(compounds) > 0 {
			child = append(child, nextIsChild)
		}
		nextIsChild = false
		compounds = append(compounds, c)
		spec += s
	}
	if len(compounds) == 0 {
		return selector{}, 0, false
	}
	return selector{compounds: compounds, child: child}, spec, true
}

func parseCompound(s string) (compound, int, bool) {
	var c compound
	spec := 0
	i := 0

	// A leading name, if any, is the element type.
	for i < len(s) && s[i] != '.' && s[i] != '#' {
		i++
	}
	if i > 0 {
		c.tag = s[:i]
		if c.tag != "*" {
			spec++
		}
	}

	for i < len(s) {
		sep := s[i]
		i++
		start := i
		for i < len(s) && s[i] != '.' && s[i] != '#' {
			i++
		}
		name := s[start:i]
		if name == "" {
			return compound{}, 0, false
		}
		switch sep {
		case '.':
			c.classes = append(c.classes, name)
			spec += 10
		case '#':
			c.id = name
			spec += 100
		}
	}
	return c, spec, true
}

func parseDeclarations(body string) map[string]string {
	out := map[string]string{}
	for _, decl := range strings.Split(body, ";") {
		colon := strings.IndexByte(decl, ':')
		if colon < 0 {
			continue
		}
		k := strings.TrimSpace(decl[:colon])
		v := strings.TrimSpace(decl[colon+1:])
		v = strings.TrimSuffix(v, "!important")
		v = strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func stripComments(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return b.String()
		}
		s = s[i+2+j+2:]
	}
}

func matchBrace(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
