package testdiscovery

import (
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// HelperDef is one function, method or arrow-function property definition.
type HelperDef struct {
	Src  *Source
	Line int
	Body [2]int // a block's [`{`, `}`] or an arrow expression's [start, end)
	Expr bool
}

// BlockAfterParams returns the `{...}` body after the parameter list opened at open, skipping a
// return-type annotation.
func (s *Source) BlockAfterParams(open int) ([2]int, bool) {
	if s.Match[open] < 0 {
		return [2]int{}, false
	}
	j := s.Match[open] + 1
	if s.Punct(j, ":") {
		j = s.SkipType(j + 1)
	}
	if s.Punct(j, "{") && s.Match[j] > j {
		return [2]int{j, s.Match[j]}, true
	}
	return [2]int{}, false
}

// FunctionBody reads `[async] function (...) {}`, `[async] (...) [: T] => body` or
// `[async] x => body` at token v.
func (s *Source) FunctionBody(v int) ([2]int, bool, bool) {
	if s.Ident(v, "async") {
		v++
	}
	if s.Ident(v, "function") {
		if s.Ident(v+1, "") {
			v++
		}
		if s.Punct(v+1, "(") {
			body, ok := s.BlockAfterParams(v + 1)
			return body, false, ok
		}
		return [2]int{}, false, false
	}
	after := -1
	switch {
	case s.Punct(v, "(") && s.Match[v] > v:
		after = s.Match[v] + 1
	case s.Ident(v, ""):
		after = v + 1
	default:
		return [2]int{}, false, false
	}
	if s.Punct(after, ":") {
		after = s.SkipType(after + 1)
	}
	if !s.Arrow(after) {
		return [2]int{}, false, false
	}
	start := after + 2
	if s.Punct(start, "{") && s.Match[start] > start {
		return [2]int{start, s.Match[start]}, false, true
	}
	return [2]int{start, s.ExprEnd(start, len(s.Tokens))}, true, true
}

// SkipType walks a type annotation from j to the first top-level `{`, `=` or `;`.
func (s *Source) SkipType(j int) int {
	angle := 0
	for limit := j + 64; j < len(s.Tokens) && j < limit; j++ {
		switch {
		case s.Punct(j, "<"):
			angle++
		case s.Punct(j, ">") && angle > 0 && !s.Arrow(j-1):
			angle--
		case angle == 0 && (s.Punct(j, "{") || s.Punct(j, "=") || s.Punct(j, ";")):
			return j
		case (s.Punct(j, "(") || s.Punct(j, "[") || s.Punct(j, "{")) && s.Match[j] > j:
			j = s.Match[j]
		}
	}
	return j
}

// ExprEnd returns the end of the expression starting at start: a top-level `;` or `,`, an
// unmatched closer, or a line break that does not continue the expression.
func (s *Source) ExprEnd(start, limit int) int {
	for j := start; j < limit && j < len(s.Tokens); j++ {
		if j > start && s.Tokens[j].Line != s.Tokens[j-1].Line && !s.continues(j) {
			return j
		}
		if s.Tokens[j].Kind != appmap.JSPunct {
			continue
		}
		switch s.Tokens[j].Text {
		case ";", ",", ")", "]", "}":
			return j
		case "(", "[", "{":
			if s.Match[j] <= j {
				return j
			}
			j = s.Match[j]
		}
	}
	return min(limit, len(s.Tokens))
}

// continues reports that token j, first on its line, continues the previous line's expression.
func (s *Source) continues(j int) bool {
	if t := s.Tokens[j]; t.Kind == appmap.JSPunct && strings.Contains(".?:&|+-*/<>=", t.Text) {
		return true
	}
	if p := s.Tokens[j-1]; p.Kind == appmap.JSPunct && strings.Contains("=&|+-*/<>?:.!(", p.Text) {
		return true
	}
	return s.Ident(j-1, "await") || s.Ident(j-1, "return") || s.Ident(j-1, "new") || s.Ident(j-1, "typeof")
}
