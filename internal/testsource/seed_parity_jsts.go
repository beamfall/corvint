package testsource

import (
	"fmt"
	"strconv"

	"github.com/Beamfall/corvint/internal/appmap"
)

// The JS/TS side of the seed-parity reading (TSA-V0-101): factory calls `create`, `build`,
// `createList` and `buildList`, bare or on one receiver (`factory.create('user', {...})`), read in
// the example body and the beforeEach/beforeAll hooks that apply to it.

// jsSeedCallees are the supported setup callees; the List forms take a literal count.
var jsSeedCallees = map[string]bool{"create": true, "build": true, "createList": true, "buildList": true}

// jsSeed extracts the setup of the example declared on line (TSA-V0-101).
func jsSeed(name, text string, line int) seedExtract {
	x := seedExtract{test: fmt.Sprintf("%s:%d", name, line)}
	s := parseSource(name, text)
	if s.uncertain {
		x.abstain(seedReasonSourceUncertain, x.test)
		return x
	}
	r := &jsReader{s: s, name: name}
	r.read()
	found, same := -1, 0
	for k, d := range r.decls {
		if d.kind == "test" && d.line == line {
			found, same = k, same+1
		}
	}
	if same != 1 || !r.decls[found].hasBody {
		x.abstain(seedReasonExampleNotFound, x.test)
		return x
	}
	e := r.decls[found]
	chain := r.ancestors(e.at)
	scope := [][2]int{e.body}
	other := [][2]int{}
	for i := range s.tokens {
		if !s.ident(i, "") || s.tokens[i].Text != "beforeEach" && s.tokens[i].Text != "beforeAll" {
			continue
		}
		if s.memberAccess(i) && !s.ident(i-2, "test") || !s.punct(i+1, "(") || s.match[i+1] < 0 {
			continue
		}
		hook := [2]int{i + 2, s.match[i+1]}
		if r.applies(i, e, chain) {
			scope = append(scope, hook)
		} else {
			other = append(other, hook)
		}
	}
	in := func(i int, ranges [][2]int) bool {
		for _, rg := range ranges {
			if within(i, rg) {
				return true
			}
		}
		return false
	}
	consumed := map[int]bool{}
	for i, t := range s.tokens {
		if t.Kind != appmap.JSIdent || consumed[i] || !seedLike(t.Text) {
			continue
		}
		called := s.punct(i+1, "(") && !s.ident(i-1, "function")
		switch {
		case r.inExampleBody(i, e.at) || in(i, other):
			continue
		case !in(i, scope):
			if called && !s.methodDefinition(i) {
				x.abstain(seedReasonScopeUnresolved, r.at(t.Line))
			}
			continue
		case s.punct(i+1, ":") && (s.punct(i-1, "{") || s.punct(i-1, ",")):
			continue // an object key
		}
		call, reason := s.jsSeedCall(i, consumed)
		if reason != "" {
			x.abstain(reason, r.at(t.Line))
			continue
		}
		call.at = r.at(t.Line)
		x.calls = append(x.calls, call)
	}
	asserts := map[int]*seedAssert{}
	lines := []int{}
	for k := e.body[0]; k < e.body[1]; k++ {
		if !s.jsAssertion(k) {
			continue
		}
		end := k
		if c, ok := s.readExpect(k); ok {
			end = c.end
		} else {
			j := k
			for s.ident(j, "") && s.punct(j+1, ".") {
				j += 2
			}
			if s.ident(j, "") && s.punct(j+1, "(") && s.match[j+1] > j {
				end = s.match[j+1]
			}
		}
		ln := s.tokens[k].Line
		a, ok := asserts[ln]
		if !ok {
			a = &seedAssert{at: r.at(ln), idents: map[string]bool{}}
			asserts[ln] = a
			lines = append(lines, ln)
		}
		for j := k; j <= end && j < e.body[1]; j++ {
			if s.tokens[j].Kind == appmap.JSIdent {
				a.idents[snake(s.tokens[j].Text)] = true
			}
		}
	}
	for _, ln := range lines {
		x.asserts = append(x.asserts, *asserts[ln])
	}
	return x
}

// methodDefinition reports a class or object method `name(...) {` at token i.
func (s *source) methodDefinition(i int) bool {
	return !s.memberAccess(i) && s.punct(i+1, "(") && s.match[i+1] > i && s.punct(s.match[i+1]+1, "{")
}

// jsSeedCall reads the factory call whose callee or receiver is token i, marking the tokens it
// consumes. Any other seed-like identifier is an unsupported form.
func (s *source) jsSeedCall(i int, consumed map[int]bool) (seedCall, string) {
	callee, start := i, i
	switch {
	case s.punct(i+1, ".") && s.ident(i+2, "") && s.punct(i+3, "(") && !s.memberAccess(i):
		callee = i + 2
	case s.memberAccess(i) && s.ident(i-2, "") && !s.memberAccess(i-2):
		start = i - 2
	case s.memberAccess(i):
		return seedCall{}, seedReasonFormUnsupported
	}
	name := s.tokens[callee].Text
	open := callee + 1
	if !jsSeedCallees[name] || !s.punct(open, "(") || s.match[open] < 0 {
		return seedCall{}, seedReasonFormUnsupported
	}
	consumed[start], consumed[callee] = true, true
	args := s.splitArgs(open, s.match[open])
	if len(args) == 0 || !s.loneString(args[0]) || !s.tokens[args[0][0]].Literal {
		return seedCall{}, seedReasonKindNotLiteral
	}
	call := seedCall{kind: snake(s.tokens[args[0][0]].Text), count: 1, attrs: map[string]string{}}
	next := 1
	if name == "createList" || name == "buildList" {
		if len(args) < 2 || args[1][1]-args[1][0] != 1 || s.tokens[args[1][0]].Kind != appmap.JSNumber {
			return seedCall{}, seedReasonCountNotLiteral
		}
		n, err := strconv.Atoi(s.tokens[args[1][0]].Text)
		if err != nil || n < 0 {
			return seedCall{}, seedReasonCountNotLiteral
		}
		call.count, next = n, 2
	}
	if len(args) > next+1 {
		return seedCall{}, seedReasonAttributesNotRead
	}
	if len(args) == next+1 {
		a := args[next]
		if !s.punct(a[0], "{") || s.match[a[0]] != a[1]-1 {
			return seedCall{}, seedReasonAttributesNotRead
		}
		for _, p := range s.splitArgs(a[0], a[1]-1) {
			k := p[0]
			key := ""
			switch {
			case s.ident(k, "") || s.tokens[k].Kind == appmap.JSString && s.tokens[k].Literal:
				key = s.tokens[k].Text
			default:
				return seedCall{}, seedReasonAttributesNotRead
			}
			if !s.punct(k+1, ":") || k+2 >= p[1] {
				return seedCall{}, seedReasonAttributesNotRead
			}
			call.attrs[snake(key)] = s.jsValue([2]int{k + 2, p[1]})
		}
	}
	call.bound = s.jsBound(start)
	return call, ""
}

// jsValue normalises a literal value like rubyValue; anything else is <expr>.
func (s *source) jsValue(v [2]int) string {
	t := s.tokens[v[0]]
	switch {
	case v[1]-v[0] == 2 && s.punct(v[0], "-") && s.tokens[v[0]+1].Kind == appmap.JSNumber:
		return "-" + s.tokens[v[0]+1].Text
	case v[1]-v[0] != 1:
		return seedExpr
	case (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal:
		return strconv.Quote(t.Text)
	case t.Kind == appmap.JSNumber && seedNumber.MatchString(t.Text):
		return t.Text
	case s.ident(v[0], "true"), s.ident(v[0], "false"):
		return t.Text
	case s.ident(v[0], "null"), s.ident(v[0], "undefined"):
		return "null"
	}
	return seedExpr
}

// jsBound is the name or destructured names the call starting at token start is assigned to.
func (s *source) jsBound(start int) []string {
	j := start - 1
	if s.ident(j, "await") {
		j--
	}
	if !s.punct(j, "=") || s.punct(j-1, "=") || s.punct(j-1, "!") || s.punct(j-1, "<") || s.punct(j-1, ">") {
		return nil
	}
	j--
	if s.ident(j, "") {
		return []string{snake(s.tokens[j].Text)}
	}
	if s.punct(j, "]") && s.match[j] >= 0 {
		out := []string{}
		for k := s.match[j] + 1; k < j; k++ {
			if s.ident(k, "") {
				out = append(out, snake(s.tokens[k].Text))
			}
		}
		return out
	}
	return nil
}
