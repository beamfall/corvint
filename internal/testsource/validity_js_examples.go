package testsource

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testdiscovery"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The JS/TS example hygiene and strength facts (TSA-V0-073..079). They reuse the TSA-V0-010
// lexer and the lint model's helpers, so they abstain exactly when ValidityFindings does.

// jsModuleMocks is testdiscovery.JSModuleMocks, kept under its old name.
var jsModuleMocks = testdiscovery.JSModuleMocks

// jsTimerMocks is testdiscovery.JSTimerMocks, kept under its old name.
var jsTimerMocks = testdiscovery.JSTimerMocks

// jsBoundaryModules is testdiscovery.JSBoundaryModules, kept under its old name.
var jsBoundaryModules = testdiscovery.JSBoundaryModules

// jsBoundaryGlobals is testdiscovery.JSBoundaryGlobals, kept under its old name.
var jsBoundaryGlobals = testdiscovery.JSBoundaryGlobals

// jsBoundarySpied is testdiscovery.JSBoundarySpied, kept under its old name.
var jsBoundarySpied = testdiscovery.JSBoundarySpied

// jsDecl is testdiscovery.JSDecl, kept under its old name.
type jsDecl = testdiscovery.JSDecl

type jsMock struct {
	at, line int
	subject  bool
	what     string
}

// jsReader is testdiscovery.JSReader, kept under its old name.
type jsReader = testdiscovery.JSReader

// JSExampleSources reads one JS/TS test file's examples (TSA-V0-073). It returns each example's
// source facts keyed by its declaring line, and the reader's uncertainty reason; when the reason is
// not "" the map is empty. The findings are the ValidityFindings that name the example followed by
// TSA-V0-074..079 in rule order.
func JSExampleSources(name, text string) (map[int]testvalidity.ExampleSource, string) {
	out := map[int]testvalidity.ExampleSource{}
	if !IsTestFile(name) {
		return out, testvalidity.ExampleSourceNotJSTestFile
	}
	base := ValidityFindings(name, text)
	if base.Uncertain {
		return out, base.Reason
	}
	r := &jsReader{S: parseSource(name, text), Name: name}
	r.Read()
	byTest := map[string][]testvalidity.SourceFinding{}
	for _, f := range base.Findings {
		byTest[f.Test] = append(byTest[f.Test], f)
	}
	perLine := map[int]int{}
	for _, d := range r.Decls {
		if d.Kind == "test" {
			perLine[d.Line]++
		}
	}
	for _, e := range r.Decls {
		if e.Kind != "test" {
			continue
		}
		test := jsReaderAt(r, e.Line)
		if reason := r.Abstention(e, perLine[e.Line]); reason != "" {
			// Two declarations on one line cannot be told apart, so neither supplies a title path.
			var path []string
			if reason != testvalidity.HygieneReasonJSLineAmbiguous {
				path = r.TitlePath(e)
			}
			out[e.Line] = testvalidity.ExampleSource{Read: reason, Findings: []testvalidity.SourceFinding{}, EndLine: e.EndLine, TitlePath: path}
			continue
		}
		findings := append([]testvalidity.SourceFinding{}, byTest[test]...)
		facts := exampleFacts(r, e, test)
		findings = append(findings, facts.findings...)
		findings = append(findings, flowFindings(r, e, test)...)
		out[e.Line] = testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: facts.count, Findings: findings, ExpectationAnchors: facts.anchors, EndLine: e.EndLine, TitlePath: r.TitlePath(e)}
	}
	return out, ""
}

// ExampleSources routes one file to the RSpec (TSA-V0-062) or JS/TS (TSA-V0-073) example reader.
func ExampleSources(name, text string) (map[int]testvalidity.ExampleSource, string) {
	if strings.HasSuffix(name, "_spec.rb") {
		return RSpecExampleSources(name, text)
	}
	return JSExampleSources(name, text)
}

func jsReaderAt(r *jsReader, line int) string { return fmt.Sprintf("%s:%d", r.Name, line) }

// applies reports that a statement at token i reaches example e: it is in e's body, or outside
// every example body at file level or in a describe enclosing e (TSA-V0-074, TSA-V0-079).
func applies(r *jsReader, i int, e jsDecl, chain []int) bool {
	if e.HasBody && within(i, e.Body) {
		return true
	}
	if r.InExampleBody(i, -1) {
		return false
	}
	own := r.Ancestors(i)
	return len(own) == 0 || slices.Contains(chain, own[0])
}

type jsFacts struct {
	findings []testvalidity.SourceFinding
	anchors  []string
	count    int
}

// exampleFacts returns the TSA-V0-074..079 findings and the expectation anchors of example e.
func exampleFacts(r *jsReader, e jsDecl, test string) jsFacts {
	finding := func(rule, severity, location, cause, message string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: rule, Severity: severity, Test: test, Title: e.Title, Location: location, Cause: cause, Message: message}
	}
	chain := r.Ancestors(e.At)
	out := jsFacts{findings: []testvalidity.SourceFinding{}, anchors: []string{}}
	skipped, focused := 0, 0
	if e.Root == "xit" || e.Root == "xtest" || e.Members["skip"] || e.Members["fixme"] {
		skipped = e.Line
	}
	if e.Root == "fit" || e.Members["only"] {
		focused = e.Line
	}
	for _, a := range r.Annotations {
		if skipped == 0 && a.Unconditional && e.HasBody && within(a.At, e.Body) {
			skipped = a.Line
		}
	}
	each := -1
	if e.Members["each"] {
		each = len(r.Decls) // marker for e itself
	}
	for _, g := range chain {
		d := r.Decls[g]
		skipped = pick(skipped, d.Root == "xdescribe" || d.Root == "xcontext" || d.Members["skip"] || d.Members["fixme"], d.Line)
		focused = pick(focused, d.Root == "fdescribe" || d.Members["only"], d.Line)
		for _, a := range r.Annotations {
			if a.Unconditional && within(a.At, d.Body) && !r.InExampleBody(a.At, -1) && r.Ancestors(a.At)[0] == g {
				skipped = pick(skipped, true, a.Line)
			}
		}
		if each < 0 && d.Members["each"] {
			each = g
		}
	}
	for _, a := range r.Annotations {
		if a.Unconditional && len(r.Ancestors(a.At)) == 0 && !r.InExampleBody(a.At, -1) {
			skipped = pick(skipped, true, a.Line)
		}
	}
	if skipped != 0 {
		out.findings = append(out.findings, finding(testvalidity.RuleJSSkipped, testvalidity.SeverityInvalid, jsReaderAt(r, e.Line), jsReaderAt(r, skipped),
			"the example is skipped, so its result proves nothing about the code; remove the skip, fixme or x-prefix"))
	}
	if e.Members["todo"] {
		out.findings = append(out.findings, finding(testvalidity.RuleJSTodo, testvalidity.SeverityInvalid, jsReaderAt(r, e.Line), jsReaderAt(r, e.Line),
			"the example is a todo with no body, so it runs nothing; write the example or drop the citation"))
	}
	if focused != 0 {
		out.findings = append(out.findings, finding(testvalidity.RuleJSFocused, testvalidity.SeverityInvalid, jsReaderAt(r, e.Line), jsReaderAt(r, focused),
			"the example or a describe around it is focused, so a run filters the suite to it; remove the only or f-prefix"))
	}
	if each >= 0 {
		d := e
		if each < len(r.Decls) {
			d = r.Decls[each]
		}
		out.findings = append(out.findings, finding(testvalidity.RuleJSEachRowBinding, testvalidity.SeverityAdvisory, jsReaderAt(r, e.Line), jsReaderAt(r, d.Line),
			"the example runs once per table row ("+rows(r, d)+") and every row is bound to this one line, so a cited file:line does not name a row"))
	}
	if e.Members["todo"] {
		return out
	}
	count, anchors, unresolved := jsReaderExpectations(r, e.Body)
	out.count, out.anchors = count, anchors
	if count == 0 && !unresolved {
		out.findings = append(out.findings, finding(testvalidity.RuleJSNoAssertion, testvalidity.SeverityAdvisory, jsReaderAt(r, e.Line), jsReaderAt(r, e.Line),
			"the example body asserts nothing the reader can see, directly or through a same-file helper, so it passes whenever it does not throw"))
	}
	if first, ok := constantOnly(r, e.Body); ok {
		out.findings = append(out.findings, finding(testvalidity.RuleJSConstantExpectation, testvalidity.SeverityAdvisory, jsReaderAt(r, first), jsReaderAt(r, e.Line),
			"every expectation compares constants or a value with itself, so it passes whatever the code does"))
	}
	for _, m := range mocks(r, e, chain) {
		rule, message := testvalidity.RuleJSBoundaryMocked, "a boundary ("+m.what+") is mocked; the code under test still runs against the fake"
		if m.subject {
			rule, message = testvalidity.RuleJSSubjectMocked, "the module or class under test ("+m.what+") is mocked or spied on, so an expectation can observe the mock instead of the code"
		}
		out.findings = append(out.findings, finding(rule, testvalidity.SeverityAdvisory, jsReaderAt(r, m.line), jsReaderAt(r, e.Line), message))
	}
	for _, line := range unawaitedAssertions(r, e) {
		out.findings = append(out.findings, finding(testvalidity.RuleJSAssertionNotAwaited, testvalidity.SeverityAdvisory, jsReaderAt(r, line), jsReaderAt(r, e.Line),
			"an assertion sits in a then, catch or finally callback of a promise the example neither awaits nor returns, so the example can finish before the assertion runs; await or return the promise"))
	}
	out.findings = append(out.findings, writeResponseFindings(r, e, finding)...)
	// Subject mocks before boundary mocks, each in source order.
	slices.SortStableFunc(out.findings, func(a, b testvalidity.SourceFinding) int {
		return jsRuleOrder(a.Rule) - jsRuleOrder(b.Rule)
	})
	return out
}

func jsRuleOrder(rule string) int {
	switch rule {
	case testvalidity.RuleJSSubjectMocked:
		return 1
	case testvalidity.RuleJSBoundaryMocked:
		return 2
	}
	return 0
}

// rows describes a `.each` table's row count (TSA-V0-076).
func rows(r *jsReader, d jsDecl) string {
	s := r.S
	switch d.EachKind {
	case "call":
		args := s.SplitArgs(d.EachTable[0]-1, d.EachTable[1])
		if len(args) == 1 && s.Punct(args[0][0], "[") && s.Match[args[0][0]] == args[0][1]-1 {
			return fmt.Sprintf("%d rows", len(s.SplitArgs(args[0][0], args[0][1]-1)))
		}
	case "template":
		t := s.Tokens[d.EachTable[0]]
		text := t.Text
		if t.Source != "" {
			text = strings.Trim(t.Source, "`")
		}
		lines := 0
		for _, line := range strings.Split(text, "\n") {
			if strings.TrimSpace(line) != "" {
				lines++
			}
		}
		if lines >= 2 && strings.Count(text, "${") == strings.Count(text, "}") {
			return fmt.Sprintf("%d rows", lines-1)
		}
	}
	return "rows not counted"
}

// expectations returns the number of assertion calls in body plus calls of same-file helpers that
// assert, their distinct `path:line` locations, and whether a call names a function imported
// from a relative module, other than a subject module, that the reader cannot follow
// (TSA-V0-077).
func jsReaderExpectations(r *jsReader, body [2]int) (int, []string, bool) {
	s := r.S
	lines := []int{}
	for k := body[0]; k < body[1]; k++ {
		if s.JSAssertion(k) {
			lines = append(lines, s.Tokens[k].Line)
		}
	}
	unresolved := false
	for _, k := range helperCalls(s, body[0], body[1]) {
		if s.JSAssertion(k) {
			continue
		}
		asserts, unknown := helperAsserts(r, s.Tokens[k].Text, !s.MemberAccess(k), 1, map[string]int{})
		if asserts {
			lines = append(lines, s.Tokens[k].Line)
		}
		unresolved = unresolved || unknown
	}
	count := len(lines)
	slices.Sort(lines)
	lines = slices.Compact(lines)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, jsReaderAt(r, line))
	}
	return count, out, unresolved
}

// helperAsserts follows a call of name to the file's own definitions. It reports whether one
// asserts, and whether the name, or a name it calls, is imported from a relative module and not
// defined here. visited holds the shallowest depth each name was followed at, so a name reached
// again only by a shorter path is followed again.
func helperAsserts(r *jsReader, name string, standalone bool, depth int, visited map[string]int) (bool, bool) {
	s := r.S
	defs := definitions(s)[name]
	if len(defs) == 0 {
		b, imported := r.Bindings[name]
		return false, standalone && imported && isRelativeSpec(b.Spec) && !r.SubjectSpecs[b.Spec]
	}
	if seen, ok := visited[name]; depth > MaxHelperDepth || ok && seen <= depth {
		return false, false
	}
	visited[name] = depth
	unresolved := false
	for _, def := range defs {
		for k := def.Body[0]; k < def.Body[1]; k++ {
			if s.JSAssertion(k) {
				return true, false
			}
		}
		for _, k := range helperCalls(s, def.Body[0], def.Body[1]) {
			asserts, unknown := helperAsserts(r, s.Tokens[k].Text, !s.MemberAccess(k), depth+1, visited)
			if asserts {
				return true, false
			}
			unresolved = unresolved || unknown
		}
	}
	return false, unresolved
}

// isRelativeSpec is testdiscovery.IsRelativeSpec, kept under its old name.
func isRelativeSpec(spec string) bool { return testdiscovery.IsRelativeSpec(spec) }

// constantOnly returns the first expectation's line when the body holds at least one assertion,
// every one an `expect(...)` chain and every chain constant (TSA-V0-078).
func constantOnly(r *jsReader, body [2]int) (int, bool) {
	s := r.S
	first := 0
	for k := body[0]; k < body[1]; k++ {
		if !s.JSAssertion(k) {
			continue
		}
		chain, ok := s.ReadExpect(k)
		if !ok || !constantChain(r, chain, body) {
			return 0, false
		}
		if first == 0 {
			first = s.Tokens[k].Line
		}
	}
	for _, k := range helperCalls(s, body[0], body[1]) {
		if asserts, _ := helperAsserts(r, s.Tokens[k].Text, !s.MemberAccess(k), 1, map[string]int{}); asserts && !s.JSAssertion(k) {
			return 0, false
		}
	}
	return first, first != 0
}

func constantChain(r *jsReader, c expectChain, body [2]int) bool {
	s := r.S
	subject := c.Subject
	if subject[1]-subject[0] == 1 {
		if literalToken(s, subject[0]) {
			return true
		}
		if s.Ident(subject[0], "") && constLiteral(r, body, s.Tokens[subject[0]].Text) {
			return true
		}
	}
	switch c.Matcher {
	case "toBe", "toEqual", "toStrictEqual":
		return subject[1] > subject[0] && s.Render(subject[0], subject[1]) == s.Render(c.Margs[0], c.Margs[1])
	}
	return false
}

// literalToken reports a number, literal string, true, false, null or undefined token.
func literalToken(s *source, i int) bool {
	t := s.Tokens[i]
	switch t.Kind {
	case appmap.JSNumber:
		return true
	case appmap.JSString:
		return t.Literal
	case appmap.JSIdent:
		return t.Text == "true" || t.Text == "false" || t.Text == "null" || t.Text == "undefined"
	}
	return false
}

// constLiteral reports `const name = LITERAL` ending its statement in body.
func constLiteral(r *jsReader, body [2]int, name string) bool {
	s := r.S
	for k := body[0]; k+3 < body[1]; k++ {
		if !s.Ident(k, "const") || !s.Ident(k+1, name) || !s.Punct(k+2, "=") || s.Punct(k+3, "=") || s.Punct(k+3, ">") || !literalToken(s, k+3) {
			continue
		}
		next := k + 4
		return next >= body[1] || s.Punct(next, ";") || s.Punct(next, "}") ||
			s.Tokens[next].Line != s.Tokens[k+3].Line && s.Ident(next, "") && !jsContinuationWords[s.Tokens[next].Text]
	}
	return false
}

// jsContinuationWords are identifiers that, starting a new line, continue the expression before
// it, so the line break does not end the statement.
var jsContinuationWords = map[string]bool{"in": true, "instanceof": true, "as": true, "satisfies": true}

// specStem is testdiscovery.SpecStem, kept under its old name.
func specStem(spec string) string { return testdiscovery.SpecStem(spec) }

// boundaryModule is testdiscovery.BoundaryModule, kept under its old name.
func boundaryModule(spec string) bool { return testdiscovery.BoundaryModule(spec) }

// mocks returns the mocks that reach example e, classified as subject or boundary; other mocks
// are dropped. One mock is returned per distinct line.
func mocks(r *jsReader, e jsDecl, chain []int) []jsMock {
	s := r.S
	specs, names := r.SubjectSpecs, r.SubjectNames
	stem := strings.TrimSuffix(path.Base(r.Name), path.Ext(r.Name))
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, ".spec"), ".test")
	callbacks := map[[2]int]bool{}
	for _, d := range r.Decls {
		if d.HasBody {
			callbacks[d.Body] = true
		}
	}
	// A named example or describe callback is a declaration's own body, not an external helper.
	inDefinition := func(i int) bool {
		for _, defs := range definitions(s) {
			for _, d := range defs {
				if within(i, d.Body) && !callbacks[d.Body] {
					return true
				}
			}
		}
		return false
	}
	out, seen := []jsMock{}, map[int]bool{}
	for k := range s.Tokens {
		m, ok := mockAt(r, k, specs, names, stem)
		if !ok || seen[m.line] || inDefinition(k) || !applies(r, k, e, chain) {
			continue
		}
		seen[m.line] = true
		out = append(out, m)
	}
	return out
}

// mockAt classifies the mock starting at token k (TSA-V0-079).
func mockAt(r *jsReader, k int, specs, names map[string]bool, stem string) (jsMock, bool) {
	s := r.S
	m := jsMock{at: k}
	if !s.Ident(k, "") || s.MemberAccess(k) || k >= len(s.Tokens) {
		return m, false
	}
	m.line = s.Tokens[k].Line
	root := s.Tokens[k].Text
	if jsBoundaryGlobals[root] && s.Punct(k+1, ".") && s.Ident(k+2, "fetch") && s.Punct(k+3, "=") && !s.Punct(k+4, "=") {
		m.what = root + ".fetch"
		return m, true
	}
	if (root != "jest" && root != "vi") || !s.Punct(k+1, ".") || !s.Ident(k+2, "") || !s.Punct(k+3, "(") || s.Match[k+3] < 0 {
		return m, false
	}
	member := s.Tokens[k+2].Text
	args := s.SplitArgs(k+3, s.Match[k+3])
	switch {
	case jsTimerMocks[member]:
		m.what = "timers"
		return m, true
	case jsModuleMocks[member] && len(args) > 0 && s.LoneString(args[0]) && s.Tokens[args[0][0]].Literal:
		spec := s.Tokens[args[0][0]].Text
		m.what = spec
		switch {
		case boundaryModule(spec):
			return m, true
		case specs[spec] || specStem(spec) == stem:
			m.subject = true
			return m, true
		}
	case member == "spyOn" && len(args) >= 2 && s.Ident(args[0][0], ""):
		receiver := s.Tokens[args[0][0]].Text
		method := ""
		if s.LoneString(args[1]) {
			method = s.Tokens[args[1][0]].Text
		}
		m.what = receiver
		if method != "" {
			m.what += "." + method
		}
		b, bound := r.Bindings[receiver]
		switch {
		case jsBoundaryGlobals[receiver] && jsBoundarySpied[method], receiver == "Date", jsBoundaryModules[receiver],
			bound && boundaryModule(b.Spec):
			return m, true
		case names[receiver]:
			m.subject = true
			return m, true
		}
	}
	return m, false
}
