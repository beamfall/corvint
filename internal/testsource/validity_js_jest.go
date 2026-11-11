package testsource

import (
	"github.com/Beamfall/corvint/internal/appmap"
)

// Jest test files and never-executed assertions in test-validity (TSA-V0-388..390). They reuse
// the TSA-V0-010 lexer and the TSA-V0-073 example reader.

// jsPromiseCallbacks are the promise members whose callback runs only when the promise settles.
var jsPromiseCallbacks = map[string]bool{"then": true, "catch": true, "finally": true}

// jsExpressionKeywords take an expression after them, so a token that follows one on a new line
// does not start a statement.
var jsExpressionKeywords = map[string]bool{"await": true, "return": true, "new": true, "typeof": true, "void": true,
	"yield": true, "throw": true, "delete": true, "in": true, "of": true, "instanceof": true, "case": true}

// unawaitedAssertions returns the line of each `.then(`, `.catch(` or `.finally(` call in example
// e's own body whose arguments hold an assertion and whose member chain is a whole statement: not
// awaited, returned, assigned or passed on (TSA-V0-389). The example must have a block body and no
// parameter, and nothing after the call in its body may be awaited; a call inside a nested function
// is not read.
func unawaitedAssertions(r *jsReader, e jsDecl) []int {
	s := r.S
	out := []int{}
	if !e.HasBody || e.Concise || !e.NoParams {
		return out
	}
	nested := [][2]int{}
	for t := e.Body[0]; t < e.Body[1]; t++ {
		if body, _, ok := s.FunctionBody(t); ok && body[0] > e.Body[0] && body[1] <= e.Body[1] {
			nested = append(nested, body)
		}
	}
	for k := e.Body[0]; k < e.Body[1]; k++ {
		if !s.Ident(k, "") || !jsPromiseCallbacks[s.Tokens[k].Text] || !s.MemberAccess(k) || !s.Punct(k+1, "(") || s.Match[k+1] < 0 {
			continue
		}
		if inAnyRange(k, nested) || !statementStart(r, receiverStart(s, k-1), e.Body) {
			continue
		}
		close := s.Match[k+1]
		asserts := false
		for j := k + 2; j < close && !asserts; j++ {
			asserts = s.JSAssertion(j)
		}
		awaitedLater := false
		for j := close + 1; j < e.Body[1] && !awaitedLater; j++ {
			awaitedLater = s.Ident(j, "await")
		}
		if asserts && !awaitedLater {
			out = append(out, s.Tokens[k].Line)
		}
	}
	return out
}

// statementStart reports that token i starts a statement in body: the body's first token, a token
// after `;`, `{` or `}`, or the first token of a line whose previous line ends in a value.
func statementStart(r *jsReader, i int, body [2]int) bool {
	s := r.S
	if i == body[0] {
		return true
	}
	if i <= body[0] || s.Ident(i, "await") || s.Ident(i, "return") {
		return false
	}
	if s.Punct(i-1, ";") || s.Punct(i-1, "{") || s.Punct(i-1, "}") {
		return true
	}
	if s.Tokens[i-1].Line == s.Tokens[i].Line {
		return false
	}
	if s.Punct(i-1, ")") || s.Punct(i-1, "]") {
		return true
	}
	if s.Ident(i-1, "") {
		return !jsExpressionKeywords[s.Tokens[i-1].Text]
	}
	return s.Tokens[i-1].Kind != appmap.JSPunct
}

func inAnyRange(i int, ranges [][2]int) bool {
	for _, r := range ranges {
		if within(i, r) {
			return true
		}
	}
	return false
}
