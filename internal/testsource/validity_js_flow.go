package testsource

import (
	"fmt"
	"maps"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The Jest and Vitest readings of STUB-ON-PATH, STUB-NEVER-INVOKED and
// ASSERTED-VALUE-NOT-FROM-SUBJECT (TSA-V0-120..130). Like the RSpec readings they follow values
// inside one example only, never read the code under test, report advisory findings and give no
// finding when a trace does not complete.

var (
	jsStubSetters = map[string]bool{"mockReturnValue": true, "mockReturnValueOnce": true, "mockResolvedValue": true,
		"mockResolvedValueOnce": true, "mockImplementation": true, "mockImplementationOnce": true}
	// jsReachCalls mount, render or run code the test does not own (TSA-V0-127).
	jsReachCalls = map[string]bool{"render": true, "mount": true, "shallowMount": true, "shallow": true, "renderHook": true,
		"createComponent": true, "renderToString": true, "require": true}
	// jsCallMatchers are the call expectations TSA-V0-128 reads; -1 marks an ordered expectation
	// (last or nth call), which a later call can fail, so it gives no finding.
	jsCallMatchers = map[string]int{"toHaveBeenCalled": 0, "toBeCalled": 0, "toHaveBeenCalledWith": 0, "toBeCalledWith": 0,
		"toHaveBeenLastCalledWith": -1, "lastCalledWith": -1, "toHaveBeenNthCalledWith": -1, "nthCalledWith": -1}
	// jsMockResets clear the calls a mock recorded before them.
	jsMockResets = map[string]bool{"mockClear": true, "mockReset": true, "mockRestore": true, "clearAllMocks": true,
		"resetAllMocks": true, "restoreAllMocks": true}
	jsUninformative = map[string]bool{"": true, "true": true, "false": true, "null": true, "undefined": true, "[]": true, "{}": true, `""`: true}
	jsValueWords    = map[string]bool{"true": true, "false": true, "null": true, "undefined": true, "NaN": true, "Infinity": true}
)

// jsTestLibrary reports a test-framework module, whose calls never reach the code under test.
func jsTestLibrary(spec string) bool {
	return strings.HasPrefix(spec, "@testing-library/") || spec == "vitest" || spec == "@jest/globals" || spec == "jest" || spec == "bun:test"
}

// jsFirstParty reports a project module specifier: relative or a project alias root.
func jsFirstParty(spec string) bool {
	return isRelativeSpec(spec) || strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "~/") || strings.HasPrefix(spec, "src/")
}

type jsExpect struct {
	at  int
	c   expectChain
	not bool
}

// jsFlow is one example's view for the TSA-V0-120..130 readings.
type jsFlow struct {
	r       *jsReader
	e       jsDecl
	chain   []int
	blocks  [][2]int        // the example body, its describes' bodies innermost first, then the file
	subject map[string]bool // subject names and the locals computed from them
	asserts []jsExpect
	action  int
	// dom memoizes domOnly: dom[0] is the token, dom[1] is 1 when read and 2 when found.
	dom [2]int
}

func (r *jsReader) flow(e jsDecl, chain []int) *jsFlow {
	s := r.s
	f := &jsFlow{r: r, e: e, chain: chain, blocks: [][2]int{e.body}, subject: map[string]bool{}}
	for _, g := range chain {
		f.blocks = append(f.blocks, r.decls[g].body)
	}
	f.blocks = append(f.blocks, [2]int{-1, len(s.tokens)})
	// The describes' subject locals depend only on the chain, so they are read once per describe.
	key := -1
	if len(chain) > 0 {
		key = chain[0]
	}
	if r.flowSubjects == nil {
		r.flowSubjects = map[int]map[string]bool{}
	}
	if known, ok := r.flowSubjects[key]; ok {
		maps.Copy(f.subject, known)
	} else {
		for n := range r.subjectNames {
			f.subject[n] = true
		}
		f.subjectLocals(f.blocks[1 : len(f.blocks)-1])
		r.flowSubjects[key] = maps.Clone(f.subject)
	}
	f.subjectLocals(f.blocks[:1])
	for k := e.body[0]; k < e.body[1]; k++ {
		if f.action == 0 && f.word(k) && f.subject[s.tokens[k].Text] {
			f.action = s.tokens[k].Line
		}
		if c, ok := s.readExpect(k); ok {
			x := jsExpect{at: k, c: c}
			for j := c.subject[1]; j < c.end; j++ {
				x.not = x.not || s.ident(j, "not") && s.memberAccess(j)
			}
			f.asserts = append(f.asserts, x)
		}
	}
	return f
}

// subjectLocals adds the locals of blocks computed from a subject name; two passes reach a local
// of a local.
func (f *jsFlow) subjectLocals(blocks [][2]int) {
	for range 2 {
		for _, b := range blocks {
			f.r.s.localValues(b, func(name string, v [2]int) {
				if !f.subject[name] && f.names(v) {
					f.subject[name] = true
				}
			})
		}
	}
}

// word reports token k as a bare identifier reference: not a property, an object key or a keyword.
func (f *jsFlow) word(k int) bool {
	s := f.r.s
	if !s.ident(k, "") || s.memberAccess(k) || keywords[s.tokens[k].Text] || jsValueWords[s.tokens[k].Text] {
		return false
	}
	if s.punct(k+1, ":") && (s.punct(k-1, "{") || s.punct(k-1, ",")) {
		return false
	}
	return s.tokens[k].Text != "this"
}

func (f *jsFlow) names(rng [2]int) bool {
	for k := rng[0]; k < rng[1]; k++ {
		if f.word(k) && f.subject[f.r.s.tokens[k].Text] {
			return true
		}
	}
	return false
}

// binding returns the token of name's `const|let|var` declaration and its initializer, from the
// innermost block that declares it.
func (f *jsFlow) binding(name string) (int, [2]int, bool) {
	s := f.r.s
	for _, b := range f.blocks {
		v, ok := s.localValue(b, name)
		if !ok {
			continue
		}
		for k := v[0] - 1; k > b[0]; k-- {
			if s.ident(k, name) && (s.ident(k-1, "const") || s.ident(k-1, "let") || s.ident(k-1, "var")) {
				return k, v, true
			}
		}
	}
	return -1, [2]int{}, false
}

// scope reports token k outside every other example's body.
func (f *jsFlow) scope(k int) bool {
	owner := f.r.owners()[k]
	return owner == -1 || owner == f.e.at
}

// owners maps each token to the declaring token of the one example body that holds it, -1 for
// none and -2 for more than one, so scope reads it in constant time; it is built on first use.
func (r *jsReader) owners() []int {
	if r.flowOwner != nil {
		return r.flowOwner
	}
	r.flowOwner = make([]int, len(r.s.tokens))
	for k := range r.flowOwner {
		r.flowOwner[k] = -1
	}
	for _, d := range r.decls {
		if d.kind != "test" || !d.hasBody {
			continue
		}
		for k := max(d.body[0], 0); k < d.body[1] && k < len(r.flowOwner); k++ {
			if within(k, d.body) {
				if r.flowOwner[k] == -1 {
					r.flowOwner[k] = d.at
				} else {
					r.flowOwner[k] = -2
				}
			}
		}
	}
	return r.flowOwner
}

// localValues calls visit with each variable the block declares with an initializer, first
// declaration only, skipping nested functions as localValue does.
func (s *source) localValues(block [2]int, visit func(name string, v [2]int)) {
	seen := map[string]bool{}
	for k := block[0] + 1; k < block[1]; k++ {
		if end := s.nestedFunctionEnd(k); end >= k {
			k = end
			continue
		}
		if !(s.ident(k, "const") || s.ident(k, "let") || s.ident(k, "var")) || !s.ident(k+1, "") || seen[s.tokens[k+1].Text] {
			continue
		}
		name := s.tokens[k+1].Text
		seen[name] = true
		j := k + 2
		if s.punct(j, ":") {
			j = s.skipType(j + 1)
		}
		if s.punct(j, "=") && !s.punct(j+1, "=") && !s.punct(j+1, ">") {
			visit(name, [2]int{j + 1, s.exprEnd(j+1, block[1])})
		}
	}
}

func (f *jsFlow) line(k int) string { return f.r.at(f.r.s.tokens[k].Line) }

// jsFlowFindings returns example e's STUB-ON-PATH, STUB-NEVER-INVOKED and
// ASSERTED-VALUE-NOT-FROM-SUBJECT findings, in that rule order and each in source order
// (TSA-V0-120).
func (r *jsReader) flowFindings(e jsDecl, test string) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	if !e.hasBody {
		return out
	}
	f := r.flow(e, r.ancestors(e.at))
	finding := func(rule string, location, cause int, message string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: rule, Severity: testvalidity.SeverityAdvisory, Test: test, Title: e.title,
			Location: f.line(location), Cause: f.line(cause), Message: message}
	}
	s := r.s
	never := []testvalidity.SourceFinding{}
	unused := map[int]bool{}
	for k := range s.tokens {
		if !s.ident(k+2, "spyOn") || !r.applies(k, e, f.chain) {
			continue
		}
		if name, bound, ok := f.neverInvoked(k); ok {
			unused[k] = true
			never = append(never, finding(testvalidity.RuleStubNeverInvoked, k, bound, fmt.Sprintf(
				"%s is spied on an object the test builds at %s and never hands to any code, so the action cannot invoke it; static reading only, a run that records calls would confirm it",
				name, f.line(bound))))
		}
	}
	for k := range s.tokens {
		if !jsStubSetters[s.tokens[k].Text] || !(within(k, e.body) || r.applies(k, e, f.chain) && f.beforeHook(k)) {
			continue
		}
		if st, ok := f.stub(k); ok && !unused[st.start] {
			if a, action, ok := f.onPath(st); ok {
				out = append(out, finding(testvalidity.RuleStubOnPath, k, a.at, fmt.Sprintf(
					"the first-party collaborator %s is stubbed, and the assertion at %s checks the stub's return value after the action at %s, so it observes the stub instead of the collaborator's behaviour",
					st.symbol(), f.line(a.at), f.r.at(action))))
			}
		}
	}
	out = append(out, never...)
	for _, a := range f.asserts {
		if cause, message, ok := f.notFromSubject(a); ok {
			out = append(out, finding(testvalidity.RuleAssertedValueNotFromSubject, a.at, cause, message))
		}
	}
	return out
}

// beforeHook reports token k inside a beforeEach or beforeAll callback, whose stubs are set before
// the example's action; a stub elsewhere outside the example body is not read.
func (f *jsFlow) beforeHook(k int) bool {
	s := f.r.s
	for j := k - 1; j > 0; j-- {
		if !s.punct(j, "(") || s.match[j] <= k {
			continue
		}
		switch {
		case s.ident(j-1, "beforeEach") || s.ident(j-1, "beforeAll"):
			return true
		case s.ident(j-1, "afterEach") || s.ident(j-1, "afterAll"):
			return false
		}
	}
	return false
}

// jsStub is one stubbed return value set at a `.mockX(...)` call.
type jsStub struct {
	start        int // the receiver's first token
	root, method string
	value        [2]int
	hasValue     bool
}

func (st jsStub) symbol() string {
	if st.method == "" || st.method == st.root {
		return st.root
	}
	return st.root + "." + st.method
}

// stub reads a first-party stub at token k, the `mockX` member (TSA-V0-121).
func (f *jsFlow) stub(k int) (jsStub, bool) {
	s := f.r.s
	if !s.ident(k, "") || !jsStubSetters[s.tokens[k].Text] || !s.memberAccess(k) || !s.punct(k+1, "(") || s.match[k+1] < 0 {
		return jsStub{}, false
	}
	dot := k - 1
	start := s.receiverStart(dot)
	st := jsStub{start: start}
	chain := [2]int{start, dot}
	if (s.ident(start, "jest") || s.ident(start, "vi")) && s.punct(start+1, ".") && s.punct(start+3, "(") && s.match[start+3] == dot-1 {
		args := s.splitArgs(start+3, dot-1)
		switch {
		case s.ident(start+2, "spyOn") && len(args) >= 2 && s.loneString(args[1]):
			chain, st.method = args[0], s.tokens[args[1][0]].Text
		case s.ident(start+2, "mocked") && len(args) >= 1:
			chain = args[0]
		default:
			return jsStub{}, false
		}
	} else if s.punct(start, "(") && s.match[start] == dot-1 {
		chain = [2]int{start + 1, dot - 1}
		for j := chain[0]; j < chain[1]; j++ {
			if s.ident(j, "as") {
				chain[1] = j
			}
		}
	}
	for j := chain[0]; j < chain[1]; j++ {
		if (j-chain[0])%2 == 0 && !s.ident(j, "") || (j-chain[0])%2 == 1 && !s.punct(j, ".") {
			return jsStub{}, false
		}
	}
	if chain[1] <= chain[0] || (chain[1]-chain[0])%2 == 0 {
		return jsStub{}, false
	}
	st.root = s.tokens[chain[0]].Text
	if st.method == "" {
		st.method = s.tokens[chain[1]-1].Text
	}
	if kind, _ := f.classify(st.root); kind != "first-party" {
		return jsStub{}, false
	}
	args := s.splitArgs(k+1, s.match[k+1])
	if len(args) == 1 {
		st.value, st.hasValue = args[0], true
		if strings.HasPrefix(s.tokens[k].Text, "mockImplementation") {
			body, expr, ok := s.functionBody(args[0][0])
			st.value, st.hasValue = body, ok && expr
		}
	}
	return st, true
}

// classify returns a receiver root's class: "first-party", "subject", "boundary" or
// "unresolved", and whether it is a fresh object the test builds (TSA-V0-121, TSA-V0-123).
func (f *jsFlow) classify(root string) (string, bool) {
	s := f.r.s
	module := func(spec string) string {
		switch {
		case f.r.subjectSpecs[spec]:
			return "subject"
		case boundaryModule(spec) || !jsFirstParty(spec):
			return "boundary"
		}
		return "first-party"
	}
	if f.subject[root] {
		return "subject", false
	}
	if b, ok := f.r.bindings[root]; ok {
		return module(b.spec), false
	}
	if jsBoundaryGlobals[root] || root == "Date" || jsBoundarySpied[root] {
		return "boundary", false
	}
	_, v, ok := f.binding(root)
	switch {
	case !ok:
		return "unresolved", false
	case s.ident(v[0], "new") && s.ident(v[0]+1, ""):
		if b, ok := f.r.bindings[s.tokens[v[0]+1].Text]; ok {
			return module(b.spec), true
		}
		return "unresolved", true
	case s.punct(v[0], "{") && s.match[v[0]] == v[1]-1:
		return "unresolved", true
	}
	return "unresolved", false
}

// onPath returns the first assertion that checks stub st's value on the subject's result, or that
// calls the stubbed function itself, with the action line (TSA-V0-122).
func (f *jsFlow) onPath(st jsStub) (jsExpect, int, bool) {
	s := f.r.s
	value := ""
	if st.hasValue {
		value = s.render(st.value[0], st.value[1])
	}
	ident := st.hasValue && st.value[1]-st.value[0] == 1 && s.ident(st.value[0], "")
	for _, a := range f.asserts {
		if a.not {
			continue
		}
		actual := a.c.subject
		for k := actual[0]; k < actual[1]; k++ {
			if !f.word(k) || s.tokens[k].Text != st.root {
				continue
			}
			if st.method == st.root && s.punct(k+1, "(") || s.punct(k+1, ".") && s.ident(k+2, st.method) && s.punct(k+3, "(") {
				return a, s.tokens[a.at].Line, true
			}
		}
		if jsUninformative[value] || f.action == 0 || !f.names(actual) || a.c.margs[1] <= a.c.margs[0] {
			continue
		}
		if s.render(a.c.margs[0], a.c.margs[1]) == value {
			return a, f.action, true
		}
		for k := a.c.margs[0]; ident && k < a.c.margs[1]; k++ {
			if f.word(k) && s.tokens[k].Text == s.tokens[st.value[0]].Text {
				return a, f.action, true
			}
		}
	}
	return jsExpect{}, 0, false
}

// neverInvoked reads `jest|vi.spyOn(NAME, ...)` at token k on a fresh object no other code
// receives (TSA-V0-124); it returns the spied symbol and the object's declaration token.
func (f *jsFlow) neverInvoked(k int) (string, int, bool) {
	s := f.r.s
	if !(s.ident(k, "jest") || s.ident(k, "vi")) || s.memberAccess(k) || !s.punct(k+1, ".") || !s.ident(k+2, "spyOn") || !s.punct(k+3, "(") || s.match[k+3] < 0 {
		return "", 0, false
	}
	args := s.splitArgs(k+3, s.match[k+3])
	if len(args) < 2 || args[0][1]-args[0][0] != 1 || !s.ident(args[0][0], "") {
		return "", 0, false
	}
	name := s.tokens[args[0][0]].Text
	if kind, fresh := f.classify(name); !fresh || kind == "subject" || kind == "boundary" {
		return "", 0, false
	}
	decl, _, _ := f.binding(name)
	spies, expects := [][2]int{}, [][2]int{}
	for j := range s.tokens {
		if (s.ident(j, "jest") || s.ident(j, "vi")) && s.punct(j+1, ".") && s.ident(j+2, "spyOn") && s.punct(j+3, "(") && s.match[j+3] > j {
			spies = append(spies, [2]int{j, s.match[j+3]})
		}
		if c, ok := s.readExpect(j); ok && f.callFree(c.subject) {
			expects = append(expects, c.subject)
		}
	}
	for j := range s.tokens {
		if j == decl || !f.scope(j) || !f.word(j) || s.tokens[j].Text != name || inAny(j, spies) || inAny(j, expects) {
			continue
		}
		return "", 0, false
	}
	symbol := name
	if s.loneString(args[1]) {
		symbol += "." + s.tokens[args[1][0]].Text
	}
	return symbol, decl, true
}

// callFree reports a range with no call or construction in it, so a reference inside it is not
// handed to other code.
func (f *jsFlow) callFree(rng [2]int) bool {
	s := f.r.s
	for k := rng[0]; k < rng[1]; k++ {
		call := s.punct(k, "(") && k > rng[0] && (s.ident(k-1, "") || s.punct(k-1, ")") || s.punct(k-1, "]"))
		if call || s.ident(k, "new") || s.tokens[k].Kind == appmap.JSTemplate && !s.tokens[k].Literal {
			return false
		}
	}
	return true
}

func inAny(k int, ranges [][2]int) bool {
	for _, r := range ranges {
		if within(k, r) {
			return true
		}
	}
	return false
}

// notFromSubject reads one assertion: a call expectation met by the test's own call
// (TSA-V0-128), an assertion on a DOM the test built with nothing that could reach the subject
// (TSA-V0-127), or an actual value built only from test-built locals (TSA-V0-126).
func (f *jsFlow) notFromSubject(a jsExpect) (int, string, bool) {
	s := f.r.s
	actual := s.unwrap(a.c.subject)
	if a.not || actual[1] <= actual[0] {
		return 0, "", false
	}
	if drop, ok := jsCallMatchers[a.c.matcher]; ok {
		if drop < 0 {
			return 0, "", false
		}
		if call, ok := f.ownCall(actual, a, drop); ok {
			return call, fmt.Sprintf("the call expectation on %s is met by the test's own call at %s, so it passes whatever the subject calls",
				s.render(actual[0], actual[1]), f.line(call)), true
		}
		return 0, "", false
	}
	if s.ident(actual[0], "screen") || s.ident(actual[0], "document") && !s.ident(actual[0]+2, "createElement") {
		if made, ok := f.domOnly(); ok {
			return made, fmt.Sprintf("the asserted DOM is the one the test builds at %s, and nothing in the example renders or calls code under test, so the assertion passes whatever the subject does",
				f.line(made)), true
		}
		return 0, "", false
	}
	if f.r.constantChain(a.c, f.e.body) {
		return 0, "", false // JS-CONSTANT-EXPECTATION reads it (TSA-V0-129)
	}
	roots, dom := map[string][2]int{}, false
	if !f.built(actual, 0, roots, &dom) || len(roots) == 0 {
		return 0, "", false
	}
	if dom {
		if _, ok := f.domOnly(); !ok {
			return 0, "", false
		}
	}
	first := -1
	for name, b := range roots {
		if first < 0 || b[0] < first {
			first = b[0]
		}
		for k := range s.tokens {
			if f.word(k) && s.tokens[k].Text == name && f.scope(k) && !f.allowedUse(k, roots, dom) {
				return 0, "", false
			}
		}
	}
	return first, fmt.Sprintf("the asserted value %s comes only from values the test builds itself (from %s) and no code under test receives them, so the assertion passes whatever the subject does",
		s.render(actual[0], actual[1]), f.line(first)), true
}

// built reports that range holds a value the test builds alone (TSA-V0-126); roots collects each
// bound name's declaration token and initializer, and dom records a test-built DOM node.
func (f *jsFlow) built(rng [2]int, depth int, roots map[string][2]int, dom *bool) bool {
	s := f.r.s
	if depth > maxFlowDepth || rng[1] <= rng[0] {
		return false
	}
	for k := rng[0]; k < rng[1]; k++ {
		t := s.tokens[k]
		switch {
		case t.Kind == appmap.JSTemplate && !t.Literal:
			return false
		case s.arrow(k) || s.ident(k, "function") || s.ident(k, "new") || s.ident(k, "await") || s.ident(k, "this") || s.ident(k, "super"):
			return false
		case s.ident(k, "document") && !s.memberAccess(k):
			switch {
			case s.punct(k+1, ".") && s.ident(k+2, "createElement") && s.punct(k+3, "(") && s.match[k+3] == k+5 &&
				s.tokens[k+4].Kind == appmap.JSString && s.tokens[k+4].Literal && !strings.Contains(s.tokens[k+4].Text, "-"):
				k = k + 5
			case s.punct(k+1, ".") && s.ident(k+2, "createDocumentFragment") && s.punct(k+3, "(") && s.match[k+3] == k+4:
				k = k + 4
			default:
				return false
			}
			*dom = true
		case !f.word(k):
			continue
		case s.punct(k+1, "(") || f.subject[t.Text]:
			return false
		default:
			decl, v, ok := f.binding(t.Text)
			if !ok {
				return false
			}
			if _, seen := roots[t.Text]; !seen {
				roots[t.Text] = [2]int{decl, 0}
				if !f.built(v, depth+1, roots, dom) {
					return false
				}
			}
		}
	}
	return true
}

// allowedUse reports a reference to a root that does not hand it to other code: its declaration,
// an assertion's actual or matcher arguments with no call in them, a member write of a test-built
// value, another root's initializer, or (for a test-built DOM node) the node appended to the
// test's own DOM.
func (f *jsFlow) allowedUse(k int, roots map[string][2]int, dom bool) bool {
	s := f.r.s
	name := s.tokens[k].Text
	if roots[name][0] == k {
		return true
	}
	for _, a := range f.asserts {
		if within(k, a.c.subject) && f.callFree(a.c.subject) || within(k, a.c.margs) && f.callFree(a.c.margs) {
			return true
		}
	}
	if s.punct(k+1, ".") && s.ident(k+2, "") && s.punct(k+3, "=") && !s.punct(k+4, "=") {
		// A member write keeps the root test-built only when the written value is test-built too.
		written, more, wroteDOM := [2]int{k + 4, s.exprEnd(k+4, len(s.tokens))}, map[string][2]int{}, false
		if !f.built(written, 0, more, &wroteDOM) || wroteDOM && !dom {
			return false
		}
		for other := range more {
			if _, ok := roots[other]; !ok {
				return false
			}
		}
		return true
	}
	for other := range roots {
		if _, v, ok := f.binding(other); ok && other != name && within(k, v) {
			return true
		}
	}
	appended := s.punct(k-1, "(") && s.punct(k+1, ")") && (s.ident(k-2, "appendChild") || s.ident(k-2, "append") || s.ident(k-2, "prepend"))
	return dom && appended
}

// domOnly returns the first DOM-building token in the example's scope when nothing in that scope
// could reach code under test (TSA-V0-127).
func (f *jsFlow) domOnly() (int, bool) {
	if f.dom[1] == 0 {
		made, ok := f.readDOM()
		f.dom = [2]int{made, 1}
		if ok {
			f.dom[1] = 2
		}
	}
	return f.dom[0], f.dom[1] == 2
}

func (f *jsFlow) readDOM() (int, bool) {
	s := f.r.s
	made := -1
	defs := s.definitions()
	for k := 0; k < len(s.tokens); k++ {
		if s.ident(k, "import") && !s.memberAccess(k) && !s.punct(k+1, "(") && !s.punct(k+1, ".") {
			if k+1 < len(s.tokens) && s.tokens[k+1].Kind == appmap.JSString {
				return 0, false // a bare import runs the module for its effects
			}
			for k < len(s.tokens) && s.tokens[k].Kind != appmap.JSString && !s.punct(k, ";") {
				k++
			}
			continue
		}
		if !f.scope(k) || !s.ident(k, "") {
			continue
		}
		name := s.tokens[k].Text
		call := s.punct(k+1, "(")
		switch {
		case call && jsReachCalls[name], s.ident(k, "new"), f.word(k) && f.subject[name]:
			return 0, false
		case call && f.word(k) && len(defs[name]) > 0:
			return 0, false
		case f.word(k):
			// Any use of an imported binding, a namespace member call included, can run module code.
			if b, ok := f.r.bindings[name]; ok && !jsTestLibrary(b.spec) {
				return 0, false
			}
		}
		dom := name == "createElement" || name == "insertAdjacentHTML" || name == "innerHTML" && s.punct(k+1, "=") && !s.punct(k+2, "=")
		if made < 0 && dom && s.memberAccess(k) && f.r.applies(k, f.e, f.chain) {
			made = k
		}
	}
	return made, made >= 0
}

// resetBetween reports a mock reset or clear between tokens from and to, which drops the calls
// recorded before it.
func (f *jsFlow) resetBetween(from, to int) bool {
	for k := from; k < to; k++ {
		if f.r.s.ident(k, "") && jsMockResets[f.r.s.tokens[k].Text] {
			return true
		}
	}
	return false
}

// ownCall returns the example's own direct call of the expected callee whose arguments the
// expectation repeats (TSA-V0-128).
func (f *jsFlow) ownCall(callee [2]int, a jsExpect, drop int) (int, bool) {
	s := f.r.s
	n := callee[1] - callee[0]
	for j := callee[0]; j < callee[1]; j++ {
		if (j-callee[0])%2 == 0 && !s.ident(j, "") || (j-callee[0])%2 == 1 && !s.punct(j, ".") {
			return 0, false
		}
	}
	if n%2 == 0 || s.memberAccess(callee[0]) {
		return 0, false
	}
	want, has := "", a.c.margs[1] > a.c.margs[0]
	if has {
		args := s.splitArgs(a.c.margs[0]-1, a.c.margs[1])
		if len(args) < drop {
			return 0, false
		}
		if len(args) > drop {
			want = s.render(args[drop][0], a.c.margs[1])
		}
	}
	for k := f.e.body[0] + 1; k < a.at; k++ {
		if end := s.nestedFunctionEnd(k); end >= k {
			k = end
			continue
		}
		if s.memberAccess(k) || !s.punct(k+n, "(") || s.match[k+n] < 0 {
			continue
		}
		same := true
		for j := 0; j < n && same; j++ {
			same = s.tokens[k+j].Kind == s.tokens[callee[0]+j].Kind && s.tokens[k+j].Text == s.tokens[callee[0]+j].Text
		}
		if !same {
			continue
		}
		if (!has || s.render(k+n+1, s.match[k+n]) == want) && !f.resetBetween(k, a.at) {
			return k, true
		}
	}
	return 0, false
}
