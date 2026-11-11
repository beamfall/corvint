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
	if s.Uncertain {
		x.abstain(seedReasonSourceUncertain, x.test)
		return x
	}
	r := &jsReader{S: s, Name: name}
	r.Read()
	found, same := -1, 0
	for k, d := range r.Decls {
		if d.Kind == "test" && d.Line == line {
			found, same = k, same+1
		}
	}
	if same != 1 || !r.Decls[found].HasBody {
		x.abstain(seedReasonExampleNotFound, x.test)
		return x
	}
	e := r.Decls[found]
	chain := r.Ancestors(e.At)
	scope := [][2]int{e.Body}
	// exec holds the function bodies that run for the example (TSA-V0-101).
	exec := [][2]int{e.Body}
	other := [][2]int{}
	for i := range s.Tokens {
		if !s.Ident(i, "") || s.Tokens[i].Text != "beforeEach" && s.Tokens[i].Text != "beforeAll" {
			continue
		}
		if s.MemberAccess(i) && !s.Ident(i-2, "test") || !s.Punct(i+1, "(") || s.Match[i+1] < 0 {
			continue
		}
		hook := [2]int{i + 2, s.Match[i+1]}
		if applies(r, i, e, chain) {
			scope = append(scope, hook)
			if body, _, ok := s.FunctionBody(hook[0]); ok && body[1] <= hook[1] {
				exec = append(exec, body)
			}
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
	for i, t := range s.Tokens {
		if t.Kind != appmap.JSIdent || consumed[i] || !seedLike(t.Text) {
			continue
		}
		called := s.Punct(i+1, "(") && !s.Ident(i-1, "function")
		switch {
		case r.InExampleBody(i, e.At) || in(i, other):
			continue
		case !in(i, scope):
			if called && !methodDefinition(s, i) {
				x.abstain(seedReasonScopeUnresolved, jsReaderAt(r, t.Line))
			}
			continue
		case s.Punct(i+1, ":") && (s.Punct(i-1, "{") || s.Punct(i-1, ",")):
			continue // an object key
		}
		call, start, reason := jsSeedCall(s, i, consumed)
		if reason == "" && !jsExecutes(s, start, exec) {
			reason = seedReasonExecutionUnresolved
		}
		if reason != "" {
			x.abstain(reason, jsReaderAt(r, t.Line))
			continue
		}
		call.at = jsReaderAt(r, t.Line)
		x.calls = append(x.calls, call)
	}
	// A call at a statement head to a function that is neither a factory callee nor known test
	// vocabulary may seed records the reading cannot see, so the side abstains naming it; the JS/TS
	// reading does not follow helpers (TSA-V0-644).
	for i, t := range s.Tokens {
		if t.Kind != appmap.JSIdent || !in(i, scope) || !s.Punct(i+1, "(") || s.MemberAccess(i) || !jsHelperCandidate(t.Text) {
			continue
		}
		if s.Ident(i-1, "function") || s.Ident(i-1, "new") || s.Ident(i-1, "async") && methodDefinition(s, i) || !jsStatementHead(s, i) {
			continue
		}
		x.abstainHelper(t.Text, jsReaderAt(r, t.Line))
		break
	}
	asserts := map[int]*seedAssert{}
	lines := []int{}
	for k := e.Body[0]; k < e.Body[1]; k++ {
		if !s.JSAssertion(k) {
			continue
		}
		end := k
		if c, ok := s.ReadExpect(k); ok {
			end = c.End
		} else {
			j := k
			for s.Ident(j, "") && s.Punct(j+1, ".") {
				j += 2
			}
			if s.Ident(j, "") && s.Punct(j+1, "(") && s.Match[j+1] > j {
				end = s.Match[j+1]
			}
		}
		// A `.should` chain asserts on its receiver: `users.reduce(f).should` reads users.
		begin := k
		for s.MemberAccess(begin) {
			p := begin - 2
			if (s.Punct(p, ")") || s.Punct(p, "]")) && s.Match[p] >= 0 && s.Match[p] < p {
				p = s.Match[p] - 1
			}
			if !s.Ident(p, "") || p < e.Body[0] {
				break
			}
			begin = p
		}
		ln := s.Tokens[k].Line
		a, ok := asserts[ln]
		if !ok {
			a = &seedAssert{at: jsReaderAt(r, ln), idents: map[string]bool{}}
			asserts[ln] = a
			lines = append(lines, ln)
		}
		for j := begin; j <= end && j < e.Body[1]; j++ {
			if s.Tokens[j].Kind == appmap.JSIdent {
				a.idents[snake(s.Tokens[j].Text)] = true
			}
		}
	}
	for _, ln := range lines {
		x.asserts = append(x.asserts, *asserts[ln])
	}
	return x
}

// methodDefinition reports a class or object method `name(...) {` at token i.
func methodDefinition(s *source, i int) bool {
	return !s.MemberAccess(i) && s.Punct(i+1, "(") && s.Match[i+1] > i && s.Punct(s.Match[i+1]+1, "{")
}

// jsControl names the keywords whose bodies or operands may not run.
var jsControl = map[string]bool{"if": true, "else": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "function": true, "try": true, "catch": true, "finally": true}

// jsExecutes reports that the call starting at token start runs whenever its enclosing function
// body in exec runs: it is not nested in a block, a callback, a condition or a short-circuit
// operand of the statement it belongs to (TSA-V0-101).
func jsExecutes(s *source, start int, exec [][2]int) bool {
	body := [2]int{-1, -1}
	for _, b := range exec {
		if within(start, b) && b[0] > body[0] {
			body = b
		}
	}
	if body[0] < 0 {
		return false
	}
	block := s.Punct(body[0], "{")
	for j := start - 1; j >= body[0]; j-- {
		t, next := s.Tokens[j], s.Tokens[j+1]
		if t.Line < next.Line && next.Kind == appmap.JSIdent && !s.Punct(j, ".") &&
			(t.Kind == appmap.JSIdent || t.Kind == appmap.JSNumber || t.Kind == appmap.JSString || s.Punct(j, ")") || s.Punct(j, "]")) {
			return true // a statement boundary without a semicolon
		}
		switch {
		case s.Punct(j, ";") || s.Punct(j, "}"):
			return true
		case (s.Punct(j, ")") || s.Punct(j, "]")) && s.Match[j] >= 0 && s.Match[j] < j:
			j = s.Match[j]
		case s.Punct(j, "{"):
			return block && j == body[0]
		case s.Punct(j, ">") && s.Arrow(j-1), s.Punct(j, "&"), s.Punct(j, "|"), s.Punct(j, "?"):
			return false
		case t.Kind == appmap.JSIdent && jsControl[t.Text]:
			return false
		}
	}
	return true
}

// jsSeedCall reads the factory call whose callee or receiver is token i, marking the tokens it
// consumes, and returns the token the call starts at. Any other seed-like identifier is an
// unsupported form.
func jsSeedCall(s *source, i int, consumed map[int]bool) (seedCall, int, string) {
	callee, start := i, i
	switch {
	case s.Punct(i+1, ".") && s.Ident(i+2, "") && s.Punct(i+3, "(") && !s.MemberAccess(i):
		callee = i + 2
	case s.MemberAccess(i) && s.Ident(i-2, "") && !s.MemberAccess(i-2):
		start = i - 2
	case s.MemberAccess(i):
		return seedCall{}, start, seedReasonFormUnsupported
	}
	name := s.Tokens[callee].Text
	open := callee + 1
	if !jsSeedCallees[name] || !s.Punct(open, "(") || s.Match[open] < 0 {
		return seedCall{}, start, seedReasonFormUnsupported
	}
	consumed[start], consumed[callee] = true, true
	args := s.SplitArgs(open, s.Match[open])
	if len(args) == 0 || !s.LoneString(args[0]) || !s.Tokens[args[0][0]].Literal {
		return seedCall{}, start, seedReasonKindNotLiteral
	}
	call := seedCall{kind: snake(s.Tokens[args[0][0]].Text), count: 1, attrs: map[string]string{}}
	next := 1
	if name == "createList" || name == "buildList" {
		if len(args) < 2 || args[1][1]-args[1][0] != 1 || s.Tokens[args[1][0]].Kind != appmap.JSNumber {
			return seedCall{}, start, seedReasonCountNotLiteral
		}
		n, err := strconv.Atoi(s.Tokens[args[1][0]].Text)
		if err != nil || n < 0 {
			return seedCall{}, start, seedReasonCountNotLiteral
		}
		call.count, next = n, 2
	}
	if len(args) > next+1 {
		return seedCall{}, start, seedReasonAttributesNotRead
	}
	if len(args) == next+1 {
		a := args[next]
		if !s.Punct(a[0], "{") || s.Match[a[0]] != a[1]-1 {
			return seedCall{}, start, seedReasonAttributesNotRead
		}
		for _, p := range s.SplitArgs(a[0], a[1]-1) {
			k := p[0]
			key := ""
			switch {
			case s.Ident(k, "") || s.Tokens[k].Kind == appmap.JSString && s.Tokens[k].Literal:
				key = s.Tokens[k].Text
			default:
				return seedCall{}, start, seedReasonAttributesNotRead
			}
			if !s.Punct(k+1, ":") || k+2 >= p[1] {
				return seedCall{}, start, seedReasonAttributesNotRead
			}
			call.attrs[snake(key)] = jsValue(s, [2]int{k + 2, p[1]})
		}
	}
	call.bound = jsBound(s, start)
	return call, start, ""
}

// jsValue normalises a literal value like rubyValue; anything else is <expr>.
func jsValue(s *source, v [2]int) string {
	t := s.Tokens[v[0]]
	switch {
	case v[1]-v[0] == 2 && s.Punct(v[0], "-") && s.Tokens[v[0]+1].Kind == appmap.JSNumber:
		return "-" + s.Tokens[v[0]+1].Text
	case v[1]-v[0] != 1:
		return seedExpr
	case (t.Kind == appmap.JSString || t.Kind == appmap.JSTemplate) && t.Literal:
		return strconv.Quote(t.Text)
	case t.Kind == appmap.JSNumber && seedNumber.MatchString(t.Text):
		return t.Text
	case s.Ident(v[0], "true"), s.Ident(v[0], "false"):
		return t.Text
	case s.Ident(v[0], "null"), s.Ident(v[0], "undefined"):
		return "null"
	}
	return seedExpr
}

// jsBound is the name or destructured names the call starting at token start is assigned to.
func jsBound(s *source, start int) []string {
	j := start - 1
	if s.Ident(j, "await") {
		j--
	}
	if !s.Punct(j, "=") || s.Punct(j-1, "=") || s.Punct(j-1, "!") || s.Punct(j-1, "<") || s.Punct(j-1, ">") {
		return nil
	}
	j--
	if s.Ident(j, "") {
		return []string{snake(s.Tokens[j].Text)}
	}
	if s.Punct(j, "]") && s.Match[j] >= 0 {
		out := []string{}
		for k := s.Match[j] + 1; k < j; k++ {
			if s.Ident(k, "") {
				out = append(out, snake(s.Tokens[k].Text))
			}
		}
		return out
	}
	return nil
}

// jsStatementHead reports that token i starts a statement or an expression statement's value: it
// follows `;`, a brace, an assignment, an arrow, `await` or `return`, or a line break.
func jsStatementHead(s *source, i int) bool {
	j := i - 1
	switch {
	case j < 0, s.Tokens[j].Line < s.Tokens[i].Line:
		return true
	case s.Punct(j, ";"), s.Punct(j, "{"), s.Punct(j, "}"), s.Punct(j, "=") && !s.Punct(j-1, "="), s.Ident(j, "await"), s.Ident(j, "return"):
		return true
	case s.Punct(j, ">") && s.Arrow(j-1):
		return true
	}
	return false
}

// jsHelperCandidate reports a lowercase callee that is not a factory callee, a keyword or known
// JS/TS test vocabulary (TSA-V0-644).
func jsHelperCandidate(name string) bool {
	if name == "" || !(name[0] >= 'a' && name[0] <= 'z' || name[0] == '_') || seedLike(name) {
		return false
	}
	return !jsCallVocabulary[name]
}

// jsCallVocabulary are the keywords and test-framework names a setup statement calls that never
// seed records the reading misses (TSA-V0-644).
var jsCallVocabulary = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "typeof": true,
	"await": true, "function": true, "async": true, "super": true, "import": true, "yield": true, "void": true,
	"delete": true, "in": true, "of": true, "instanceof": true, "do": true, "with": true,
	"expect": true, "describe": true, "it": true, "test": true, "beforeEach": true, "beforeAll": true,
	"afterEach": true, "afterAll": true, "xit": true, "fit": true, "xtest": true, "fail": true, "pending": true,
	"done": true, "require": true, "setTimeout": true, "clearTimeout": true, "setInterval": true,
	"clearInterval": true, "setImmediate": true, "queueMicrotask": true, "structuredClone": true,
	"parseInt": true, "parseFloat": true, "isNaN": true, "render": true, "act": true, "waitFor": true,
	"mount": true, "shallow": true, "cleanup": true, "assert": true,
}
