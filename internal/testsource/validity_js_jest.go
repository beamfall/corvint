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

// noParams reports a callback at token i that declares no parameter: `() =>`, `async () =>`,
// `(): T =>`, `function () {` or `function name() {`.
func (s *source) noParams(i int) bool {
	if s.ident(i, "async") {
		i++
	}
	if s.ident(i, "function") {
		i++
		if s.ident(i, "") {
			i++
		}
	}
	return s.punct(i, "(") && s.match[i] == i+1
}

// unawaitedAssertions returns the line of each `.then(`, `.catch(` or `.finally(` call in example
// e's own body whose arguments hold an assertion and whose member chain is a whole statement: not
// awaited, returned, assigned or passed on (TSA-V0-389). The example must have a block body and no
// parameter, and nothing after the call in its body may be awaited; a call inside a nested function
// is not read.
func (r *jsReader) unawaitedAssertions(e jsDecl) []int {
	s := r.s
	out := []int{}
	if !e.hasBody || e.concise || !e.noParams {
		return out
	}
	nested := [][2]int{}
	for t := e.body[0]; t < e.body[1]; t++ {
		if body, _, ok := s.functionBody(t); ok && body[0] > e.body[0] && body[1] <= e.body[1] {
			nested = append(nested, body)
		}
	}
	for k := e.body[0]; k < e.body[1]; k++ {
		if !s.ident(k, "") || !jsPromiseCallbacks[s.tokens[k].Text] || !s.memberAccess(k) || !s.punct(k+1, "(") || s.match[k+1] < 0 {
			continue
		}
		if inAnyRange(k, nested) || !r.statementStart(s.receiverStart(k-1), e.body) {
			continue
		}
		close := s.match[k+1]
		asserts := false
		for j := k + 2; j < close && !asserts; j++ {
			asserts = s.jsAssertion(j)
		}
		awaitedLater := false
		for j := close + 1; j < e.body[1] && !awaitedLater; j++ {
			awaitedLater = s.ident(j, "await")
		}
		if asserts && !awaitedLater {
			out = append(out, s.tokens[k].Line)
		}
	}
	return out
}

// statementStart reports that token i starts a statement in body: the body's first token, a token
// after `;`, `{` or `}`, or the first token of a line whose previous line ends in a value.
func (r *jsReader) statementStart(i int, body [2]int) bool {
	s := r.s
	if i == body[0] {
		return true
	}
	if i <= body[0] || s.ident(i, "await") || s.ident(i, "return") {
		return false
	}
	if s.punct(i-1, ";") || s.punct(i-1, "{") || s.punct(i-1, "}") {
		return true
	}
	if s.tokens[i-1].Line == s.tokens[i].Line {
		return false
	}
	if s.punct(i-1, ")") || s.punct(i-1, "]") {
		return true
	}
	if s.ident(i-1, "") {
		return !jsExpressionKeywords[s.tokens[i-1].Text]
	}
	return s.tokens[i-1].Kind != appmap.JSPunct
}

func inAnyRange(i int, ranges [][2]int) bool {
	for _, r := range ranges {
		if within(i, r) {
			return true
		}
	}
	return false
}
