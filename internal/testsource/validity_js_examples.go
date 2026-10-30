package testsource

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The JS/TS example hygiene and strength facts (TSA-V0-073..079). They reuse the TSA-V0-010
// lexer and the lint model's helpers, so they abstain exactly when ValidityFindings does.

var (
	// jsRunMembers are the declaration members the reader reads (TSA-V0-073).
	jsRunMembers = map[string]bool{"only": true, "skip": true, "todo": true, "each": true, "concurrent": true,
		"sequential": true, "fixme": true, "serial": true, "parallel": true}
	// jsUnsupportedMembers make an example or describe a shape the reader does not decide.
	jsUnsupportedMembers = map[string]bool{"failing": true, "fails": true, "fail": true, "skipIf": true, "runIf": true}
	jsModuleMocks        = map[string]bool{"mock": true, "doMock": true, "unstable_mockModule": true}
	jsTimerMocks         = map[string]bool{"useFakeTimers": true, "setSystemTime": true}
	jsBoundaryModules    = map[string]bool{"axios": true, "node-fetch": true, "cross-fetch": true, "got": true, "superagent": true,
		"undici": true, "ky": true, "http": true, "https": true}
	jsBoundaryGlobals = map[string]bool{"global": true, "globalThis": true, "window": true}
	jsBoundarySpied   = map[string]bool{"fetch": true, "setTimeout": true, "setInterval": true}
)

// jsDecl is one example or describe the reader placed.
type jsDecl struct {
	kind        string // "test" or "describe"
	root        string
	at, line    int
	endLine     int // the line of the declaring call's closing parenthesis
	members     map[string]bool
	unsupported bool
	title       string
	// literalTitle marks a title that is one exact string literal on a declaration with no
	// `.each` table, so it names the example or group as the runner does (LPCV-V0-147).
	literalTitle bool
	eachTable    [2]int // the `.each(...)` argument range, or the template token as [i, i+1]
	eachKind     string // "call", "template" or ""
	hasBody      bool
	body         [2]int
	concise      bool // the body is an arrow function's expression, which the example returns
	noParams     bool // the callback declares no parameter, so it takes no `done` (TSA-V0-389)
}

// jsAnnotation is one runtime `test.skip()` or `test.fixme()` call with no title.
type jsAnnotation struct {
	at, line      int
	unconditional bool
}

// jsBinding is one imported identifier and the specifier it comes from.
type jsBinding struct {
	spec string
	line int
	// imported is the exported name an `import { X as Y }` binding takes (X), or the local name.
	imported string
}

type jsMock struct {
	at, line int
	subject  bool
	what     string
}

type jsReader struct {
	s           *source
	name        string
	decls       []jsDecl
	annotations []jsAnnotation
	bindings    map[string]jsBinding
	// subjectSpecs and subjectNames are the TSA-V0-079 subject modules and identifiers.
	subjectSpecs, subjectNames map[string]bool
	// flowOwner maps tokens to their example for TSA-V0-120..130, built on first use.
	flowOwner []int
	// flowSubjects caches each describe's subject locals for TSA-V0-120..130.
	flowSubjects map[int]map[string]bool
}

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
	r := &jsReader{s: parseSource(name, text), name: name}
	r.read()
	byTest := map[string][]testvalidity.SourceFinding{}
	for _, f := range base.Findings {
		byTest[f.Test] = append(byTest[f.Test], f)
	}
	perLine := map[int]int{}
	for _, d := range r.decls {
		if d.kind == "test" {
			perLine[d.line]++
		}
	}
	for _, e := range r.decls {
		if e.kind != "test" {
			continue
		}
		test := r.at(e.line)
		if reason := r.abstention(e, perLine[e.line]); reason != "" {
			// Two declarations on one line cannot be told apart, so neither supplies a title path.
			var path []string
			if reason != testvalidity.HygieneReasonJSLineAmbiguous {
				path = r.titlePath(e)
			}
			out[e.line] = testvalidity.ExampleSource{Read: reason, Findings: []testvalidity.SourceFinding{}, EndLine: e.endLine, TitlePath: path}
			continue
		}
		findings := append([]testvalidity.SourceFinding{}, byTest[test]...)
		facts := r.exampleFacts(e, test)
		findings = append(findings, facts.findings...)
		findings = append(findings, r.flowFindings(e, test)...)
		out[e.line] = testvalidity.ExampleSource{Read: testvalidity.ExampleSourceLocated, Expectations: facts.count, Findings: findings, ExpectationAnchors: facts.anchors, EndLine: e.endLine, TitlePath: r.titlePath(e)}
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

func (r *jsReader) at(line int) string { return fmt.Sprintf("%s:%d", r.name, line) }

// read places every example, describe and runtime annotation, and the file's import bindings.
func (r *jsReader) read() {
	s := r.s
	for i := range s.tokens {
		if !s.ident(i, "") || s.memberAccess(i) || s.ident(i-1, "function") {
			continue
		}
		root := s.tokens[i].Text
		if !testRoots[root] && !describeRoots[root] {
			continue
		}
		d := jsDecl{kind: "test", root: root, at: i, line: s.tokens[i].Line, members: map[string]bool{}}
		if describeRoots[root] {
			d.kind = "describe"
		}
		j, ignore := i+1, false
		for !ignore && s.punct(j, ".") && s.ident(j+1, "") {
			member := s.tokens[j+1].Text
			j += 2
			switch {
			case member == "describe" && testRoots[root]:
				d.kind = "describe"
			case jsRunMembers[member]:
				d.members[member] = true
			case jsUnsupportedMembers[member]:
				d.unsupported = true
				// skipIf(condition)(title, fn): step over the condition call.
				if s.punct(j, "(") && s.match[j] > j && s.punct(s.match[j]+1, "(") {
					j = s.match[j] + 1
				}
			default:
				ignore = true
			}
			if member != "each" || ignore {
				continue
			}
			switch {
			case s.punct(j, "(") && s.match[j] > j:
				d.eachTable, d.eachKind = [2]int{j + 1, s.match[j]}, "call"
				j = s.match[j] + 1
			case j < len(s.tokens) && s.tokens[j].Kind == appmap.JSTemplate:
				d.eachTable, d.eachKind = [2]int{j, j + 1}, "template"
				j++
			default:
				ignore = true
			}
		}
		if ignore || !s.punct(j, "(") || s.match[j] < 0 {
			continue
		}
		d.endLine = s.tokens[s.match[j]].Line
		args := s.splitArgs(j, s.match[j])
		titled := len(args) > 0 && s.loneString(args[0])
		if titled {
			d.title = titleText(s.tokens[args[0][0]])
			d.literalTitle = s.tokens[args[0][0]].Literal && d.eachKind == ""
		}
		for k := 1; k < len(args); k++ {
			// functionBody also takes a typed arrow, `(): void => {…}`, which callback does not.
			body, concise, ok := s.functionBody(args[k][0])
			ok = ok && body[1] <= args[k][1]
			if !ok && !s.callback(args[k]) {
				continue
			}
			if ok {
				d.body, d.hasBody, d.concise, d.noParams = body, true, concise, s.noParams(args[k][0])
			}
			break
		}
		if d.kind == "test" && !titled && !d.hasBody && (d.members["skip"] || d.members["fixme"]) {
			unconditional := len(args) == 0 || args[0][1]-args[0][0] == 1 && s.ident(args[0][0], "true")
			r.annotations = append(r.annotations, jsAnnotation{at: i, line: d.line, unconditional: unconditional})
			continue
		}
		if d.kind == "describe" && !d.hasBody {
			continue
		}
		r.decls = append(r.decls, d)
	}
	r.bindings = s.importBindings()
	r.subjectSpecs, r.subjectNames = r.subjects()
}

// abstention returns why the reader does not decide example e, or "".
func (r *jsReader) abstention(e jsDecl, sameLine int) string {
	if sameLine > 1 {
		return testvalidity.HygieneReasonJSLineAmbiguous
	}
	if e.unsupported {
		return testvalidity.HygieneReasonJSShapeUnsupported
	}
	for _, g := range r.ancestors(e.at) {
		if r.decls[g].unsupported {
			return testvalidity.HygieneReasonJSShapeUnsupported
		}
	}
	if !e.members["todo"] && !e.hasBody {
		return testvalidity.HygieneReasonJSBodyNotLiteral
	}
	return ""
}

// titlePath returns example e's literal title path, outermost describe first and e's own title
// last (LPCV-V0-147), or nil when any title on it is not one literal string.
func (r *jsReader) titlePath(e jsDecl) []string {
	if !e.literalTitle {
		return nil
	}
	path := []string{e.title}
	for _, g := range r.ancestors(e.at) {
		if !r.decls[g].literalTitle {
			return nil
		}
		path = append(path, r.decls[g].title)
	}
	slices.Reverse(path)
	return path
}

// ancestors returns the indexes of the describes whose bodies hold token i, innermost first.
func (r *jsReader) ancestors(i int) []int {
	out := []int{}
	for k, d := range r.decls {
		if d.kind == "describe" && d.at < i && within(i, d.body) {
			out = append(out, k)
		}
	}
	slices.Reverse(out)
	return out
}

// inExampleBody reports token i inside some example's body other than the one at token except.
func (r *jsReader) inExampleBody(i, except int) bool {
	for _, d := range r.decls {
		if d.kind == "test" && d.at != except && d.hasBody && within(i, d.body) {
			return true
		}
	}
	return false
}

// applies reports that a statement at token i reaches example e: it is in e's body, or outside
// every example body at file level or in a describe enclosing e (TSA-V0-074, TSA-V0-079).
func (r *jsReader) applies(i int, e jsDecl, chain []int) bool {
	if e.hasBody && within(i, e.body) {
		return true
	}
	if r.inExampleBody(i, -1) {
		return false
	}
	own := r.ancestors(i)
	return len(own) == 0 || slices.Contains(chain, own[0])
}

type jsFacts struct {
	findings []testvalidity.SourceFinding
	anchors  []string
	count    int
}

// exampleFacts returns the TSA-V0-074..079 findings and the expectation anchors of example e.
func (r *jsReader) exampleFacts(e jsDecl, test string) jsFacts {
	finding := func(rule, severity, location, cause, message string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: rule, Severity: severity, Test: test, Title: e.title, Location: location, Cause: cause, Message: message}
	}
	chain := r.ancestors(e.at)
	out := jsFacts{findings: []testvalidity.SourceFinding{}, anchors: []string{}}
	skipped, focused := 0, 0
	if e.root == "xit" || e.root == "xtest" || e.members["skip"] || e.members["fixme"] {
		skipped = e.line
	}
	if e.root == "fit" || e.members["only"] {
		focused = e.line
	}
	for _, a := range r.annotations {
		if skipped == 0 && a.unconditional && e.hasBody && within(a.at, e.body) {
			skipped = a.line
		}
	}
	each := -1
	if e.members["each"] {
		each = len(r.decls) // marker for e itself
	}
	for _, g := range chain {
		d := r.decls[g]
		skipped = pick(skipped, d.root == "xdescribe" || d.root == "xcontext" || d.members["skip"] || d.members["fixme"], d.line)
		focused = pick(focused, d.root == "fdescribe" || d.members["only"], d.line)
		for _, a := range r.annotations {
			if a.unconditional && within(a.at, d.body) && !r.inExampleBody(a.at, -1) && r.ancestors(a.at)[0] == g {
				skipped = pick(skipped, true, a.line)
			}
		}
		if each < 0 && d.members["each"] {
			each = g
		}
	}
	for _, a := range r.annotations {
		if a.unconditional && len(r.ancestors(a.at)) == 0 && !r.inExampleBody(a.at, -1) {
			skipped = pick(skipped, true, a.line)
		}
	}
	if skipped != 0 {
		out.findings = append(out.findings, finding(testvalidity.RuleJSSkipped, testvalidity.SeverityInvalid, r.at(e.line), r.at(skipped),
			"the example is skipped, so its result proves nothing about the code; remove the skip, fixme or x-prefix"))
	}
	if e.members["todo"] {
		out.findings = append(out.findings, finding(testvalidity.RuleJSTodo, testvalidity.SeverityInvalid, r.at(e.line), r.at(e.line),
			"the example is a todo with no body, so it runs nothing; write the example or drop the citation"))
	}
	if focused != 0 {
		out.findings = append(out.findings, finding(testvalidity.RuleJSFocused, testvalidity.SeverityInvalid, r.at(e.line), r.at(focused),
			"the example or a describe around it is focused, so a run filters the suite to it; remove the only or f-prefix"))
	}
	if each >= 0 {
		d := e
		if each < len(r.decls) {
			d = r.decls[each]
		}
		out.findings = append(out.findings, finding(testvalidity.RuleJSEachRowBinding, testvalidity.SeverityAdvisory, r.at(e.line), r.at(d.line),
			"the example runs once per table row ("+r.rows(d)+") and every row is bound to this one line, so a cited file:line does not name a row"))
	}
	if e.members["todo"] {
		return out
	}
	count, anchors, unresolved := r.expectations(e.body)
	out.count, out.anchors = count, anchors
	if count == 0 && !unresolved {
		out.findings = append(out.findings, finding(testvalidity.RuleJSNoAssertion, testvalidity.SeverityAdvisory, r.at(e.line), r.at(e.line),
			"the example body asserts nothing the reader can see, directly or through a same-file helper, so it passes whenever it does not throw"))
	}
	if first, ok := r.constantOnly(e.body); ok {
		out.findings = append(out.findings, finding(testvalidity.RuleJSConstantExpectation, testvalidity.SeverityAdvisory, r.at(first), r.at(e.line),
			"every expectation compares constants or a value with itself, so it passes whatever the code does"))
	}
	for _, m := range r.mocks(e, chain) {
		rule, message := testvalidity.RuleJSBoundaryMocked, "a boundary ("+m.what+") is mocked; the code under test still runs against the fake"
		if m.subject {
			rule, message = testvalidity.RuleJSSubjectMocked, "the module or class under test ("+m.what+") is mocked or spied on, so an expectation can observe the mock instead of the code"
		}
		out.findings = append(out.findings, finding(rule, testvalidity.SeverityAdvisory, r.at(m.line), r.at(e.line), message))
	}
	for _, line := range r.unawaitedAssertions(e) {
		out.findings = append(out.findings, finding(testvalidity.RuleJSAssertionNotAwaited, testvalidity.SeverityAdvisory, r.at(line), r.at(e.line),
			"an assertion sits in a then, catch or finally callback of a promise the example neither awaits nor returns, so the example can finish before the assertion runs; await or return the promise"))
	}
	out.findings = append(out.findings, r.writeResponseFindings(e, finding)...)
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
func (r *jsReader) rows(d jsDecl) string {
	s := r.s
	switch d.eachKind {
	case "call":
		args := s.splitArgs(d.eachTable[0]-1, d.eachTable[1])
		if len(args) == 1 && s.punct(args[0][0], "[") && s.match[args[0][0]] == args[0][1]-1 {
			return fmt.Sprintf("%d rows", len(s.splitArgs(args[0][0], args[0][1]-1)))
		}
	case "template":
		t := s.tokens[d.eachTable[0]]
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

// jsAssertion reports an assertion at token k: a TSA-V0-013 assertion call, an `assert.` or
// `expect.` member call, or a `.should` member (TSA-V0-077).
func (s *source) jsAssertion(k int) bool {
	if s.assertionCount(k) {
		return false
	}
	if s.isAssertion(k) {
		return true
	}
	if !s.ident(k, "") {
		return false
	}
	switch s.tokens[k].Text {
	case "assert", "expect":
		return !s.memberAccess(k) && s.punct(k+1, ".") && s.ident(k+2, "") && s.punct(k+3, "(")
	case "should":
		return s.memberAccess(k)
	}
	return false
}

// assertionCount reports `expect.assertions(` or `expect.hasAssertions(` at token k, either at
// `expect` or at the member: a declared count, not an assertion (TSA-V0-077).
func (s *source) assertionCount(k int) bool {
	counted := func(i int) bool {
		return s.ident(i, "") && (s.tokens[i].Text == "assertions" || s.tokens[i].Text == "hasAssertions") && s.punct(i+1, "(")
	}
	if s.ident(k, "expect") && !s.memberAccess(k) && s.punct(k+1, ".") && counted(k+2) {
		return true
	}
	return s.memberAccess(k) && s.ident(k-2, "expect") && !s.memberAccess(k-2) && counted(k)
}

// expectations returns the number of assertion calls in body plus calls of same-file helpers that
// assert, their distinct `path:line` locations, and whether a call names a function imported
// from a relative module, other than a subject module, that the reader cannot follow
// (TSA-V0-077).
func (r *jsReader) expectations(body [2]int) (int, []string, bool) {
	s := r.s
	lines := []int{}
	for k := body[0]; k < body[1]; k++ {
		if s.jsAssertion(k) {
			lines = append(lines, s.tokens[k].Line)
		}
	}
	unresolved := false
	for _, k := range s.helperCalls(body[0], body[1]) {
		if s.jsAssertion(k) {
			continue
		}
		asserts, unknown := r.helperAsserts(s.tokens[k].Text, !s.memberAccess(k), 1, map[string]int{})
		if asserts {
			lines = append(lines, s.tokens[k].Line)
		}
		unresolved = unresolved || unknown
	}
	count := len(lines)
	slices.Sort(lines)
	lines = slices.Compact(lines)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, r.at(line))
	}
	return count, out, unresolved
}

// helperAsserts follows a call of name to the file's own definitions. It reports whether one
// asserts, and whether the name, or a name it calls, is imported from a relative module and not
// defined here. visited holds the shallowest depth each name was followed at, so a name reached
// again only by a shorter path is followed again.
func (r *jsReader) helperAsserts(name string, standalone bool, depth int, visited map[string]int) (bool, bool) {
	s := r.s
	defs := s.definitions()[name]
	if len(defs) == 0 {
		b, imported := r.bindings[name]
		return false, standalone && imported && isRelativeSpec(b.spec) && !r.subjectSpecs[b.spec]
	}
	if seen, ok := visited[name]; depth > MaxHelperDepth || ok && seen <= depth {
		return false, false
	}
	visited[name] = depth
	unresolved := false
	for _, def := range defs {
		for k := def.body[0]; k < def.body[1]; k++ {
			if s.jsAssertion(k) {
				return true, false
			}
		}
		for _, k := range s.helperCalls(def.body[0], def.body[1]) {
			asserts, unknown := r.helperAsserts(s.tokens[k].Text, !s.memberAccess(k), depth+1, visited)
			if asserts {
				return true, false
			}
			unresolved = unresolved || unknown
		}
	}
	return false, unresolved
}

func isRelativeSpec(spec string) bool {
	return spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// constantOnly returns the first expectation's line when the body holds at least one assertion,
// every one an `expect(...)` chain and every chain constant (TSA-V0-078).
func (r *jsReader) constantOnly(body [2]int) (int, bool) {
	s := r.s
	first := 0
	for k := body[0]; k < body[1]; k++ {
		if !s.jsAssertion(k) {
			continue
		}
		chain, ok := s.readExpect(k)
		if !ok || !r.constantChain(chain, body) {
			return 0, false
		}
		if first == 0 {
			first = s.tokens[k].Line
		}
	}
	for _, k := range s.helperCalls(body[0], body[1]) {
		if asserts, _ := r.helperAsserts(s.tokens[k].Text, !s.memberAccess(k), 1, map[string]int{}); asserts && !s.jsAssertion(k) {
			return 0, false
		}
	}
	return first, first != 0
}

func (r *jsReader) constantChain(c expectChain, body [2]int) bool {
	s := r.s
	subject := c.subject
	if subject[1]-subject[0] == 1 {
		if s.literalToken(subject[0]) {
			return true
		}
		if s.ident(subject[0], "") && r.constLiteral(body, s.tokens[subject[0]].Text) {
			return true
		}
	}
	switch c.matcher {
	case "toBe", "toEqual", "toStrictEqual":
		return subject[1] > subject[0] && s.render(subject[0], subject[1]) == s.render(c.margs[0], c.margs[1])
	}
	return false
}

// literalToken reports a number, literal string, true, false, null or undefined token.
func (s *source) literalToken(i int) bool {
	t := s.tokens[i]
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
func (r *jsReader) constLiteral(body [2]int, name string) bool {
	s := r.s
	for k := body[0]; k+3 < body[1]; k++ {
		if !s.ident(k, "const") || !s.ident(k+1, name) || !s.punct(k+2, "=") || s.punct(k+3, "=") || s.punct(k+3, ">") || !s.literalToken(k+3) {
			continue
		}
		next := k + 4
		return next >= body[1] || s.punct(next, ";") || s.punct(next, "}") ||
			s.tokens[next].Line != s.tokens[k+3].Line && s.ident(next, "") && !jsContinuationWords[s.tokens[next].Text]
	}
	return false
}

// jsContinuationWords are identifiers that, starting a new line, continue the expression before
// it, so the line break does not end the statement.
var jsContinuationWords = map[string]bool{"in": true, "instanceof": true, "as": true, "satisfies": true}

// importBindings maps each identifier an `import` declaration or a top-level `require` binds to
// its specifier.
func (s *source) importBindings() map[string]jsBinding {
	out := map[string]jsBinding{}
	literal := func(k int) bool {
		return k < len(s.tokens) && s.tokens[k].Kind == appmap.JSString && s.tokens[k].Literal
	}
	for i := range s.tokens {
		switch {
		case s.ident(i, "import") && !s.memberAccess(i) && !s.punct(i+1, "(") && !s.punct(i+1, "."):
			names := []int{}
			for k := i + 1; k < len(s.tokens) && k < i+256; k++ {
				if s.ident(k, "from") && literal(k+1) {
					for _, n := range names {
						imported := s.tokens[n].Text
						if n >= 2 && s.ident(n-1, "as") && s.ident(n-2, "") {
							imported = s.tokens[n-2].Text
						}
						out[s.tokens[n].Text] = jsBinding{spec: s.tokens[k+1].Text, line: s.tokens[i].Line, imported: imported}
					}
					break
				}
				if s.tokens[k].Kind == appmap.JSString || s.punct(k, ";") {
					break
				}
				if s.ident(k, "") && !s.ident(k, "type") && !s.ident(k, "as") && !s.ident(k+1, "as") {
					names = append(names, k)
				}
			}
		case (s.ident(i, "const") || s.ident(i, "let") || s.ident(i, "var")) && !s.memberAccess(i):
			names, j := []int{}, i+1
			switch {
			case s.ident(j, ""):
				names, j = append(names, j), j+1
			case s.punct(j, "{") && s.match[j] > j:
				for k := j + 1; k < s.match[j]; k++ {
					if s.ident(k, "") && !s.punct(k+1, ":") {
						names = append(names, k)
					}
				}
				j = s.match[j] + 1
			}
			if s.punct(j, "=") && s.ident(j+1, "require") && s.punct(j+2, "(") && literal(j+3) && s.punct(j+4, ")") {
				for _, n := range names {
					imported := s.tokens[n].Text
					if s.punct(n-1, ":") && s.ident(n-2, "") {
						// `const { X: Y } = require(...)` binds Y to the exported X.
						imported = s.tokens[n-2].Text
					}
					out[s.tokens[n].Text] = jsBinding{spec: s.tokens[j+3].Text, line: s.tokens[i].Line, imported: imported}
				}
			}
		}
	}
	return out
}

// specStem is a module specifier's base name without its extension, or its directory's name for
// an index module.
func specStem(spec string) string {
	base := path.Base(spec)
	if ext := path.Ext(base); webExtensions[ext] {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "index" {
		return path.Base(path.Dir(spec))
	}
	return base
}

func boundaryModule(spec string) bool {
	return jsBoundaryModules[strings.TrimPrefix(spec, "node:")]
}

// subjectWord folds a stem or identifier for the TSA-V0-079 (c) comparison: lower case, without
// `-`, `_` or `.`, so `user-service` and `UserService` agree.
func subjectWord(name string) string {
	return strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(name))
}

// subjects returns the subject module specifiers and the identifiers that name the subject
// (TSA-V0-079).
func (r *jsReader) subjects() (map[string]bool, map[string]bool) {
	stem := strings.TrimSuffix(path.Base(r.name), path.Ext(r.name))
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, ".spec"), ".test")
	specs, names := map[string]bool{}, map[string]bool{}
	for _, b := range r.bindings {
		switch {
		case boundaryModule(b.spec):
		case specStem(b.spec) == stem:
			specs[b.spec] = true
		case isRelativeSpec(b.spec) && b.imported != "" && subjectWord(b.imported) == subjectWord(stem):
			// (c) a relative module, such as a barrel, that exports the stem's name (V1-1862).
			specs[b.spec] = true
		}
	}
	locals := make([]string, 0, len(r.bindings))
	for local := range r.bindings {
		locals = append(locals, local)
	}
	slices.Sort(locals)
	// importedAs finds the first local, in name order, whose exported name is title, for
	// `import { X as Y }`.
	importedAs := func(title string) (string, jsBinding, bool) {
		for _, local := range locals {
			if b := r.bindings[local]; b.imported == title {
				return local, b, true
			}
		}
		return "", jsBinding{}, false
	}
	s := r.s
	for _, d := range r.decls {
		if d.kind != "describe" || len(r.ancestors(d.at)) != 0 {
			continue
		}
		name := d.title
		if first := d.at + 1; name == "" {
			for !s.punct(first, "(") && first < len(s.tokens) {
				first++
			}
			if s.ident(first+1, "") && (s.punct(first+2, ",") || s.punct(first+2, ")")) {
				name = s.tokens[first+1].Text
			}
		}
		b, ok := r.bindings[name]
		// Only a string title can name an exported X; an identifier argument is a local binding.
		if !ok && d.title != "" {
			var local string
			if local, b, ok = importedAs(name); ok {
				names[local] = true
			}
		}
		if ok && !boundaryModule(b.spec) {
			specs[b.spec] = true
			names[name] = true
		}
	}
	for n, b := range r.bindings {
		if specs[b.spec] {
			names[n] = true
		}
	}
	return specs, names
}

// mocks returns the mocks that reach example e, classified as subject or boundary; other mocks
// are dropped. One mock is returned per distinct line.
func (r *jsReader) mocks(e jsDecl, chain []int) []jsMock {
	s := r.s
	specs, names := r.subjectSpecs, r.subjectNames
	stem := strings.TrimSuffix(path.Base(r.name), path.Ext(r.name))
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, ".spec"), ".test")
	callbacks := map[[2]int]bool{}
	for _, d := range r.decls {
		if d.hasBody {
			callbacks[d.body] = true
		}
	}
	// A named example or describe callback is a declaration's own body, not an external helper.
	inDefinition := func(i int) bool {
		for _, defs := range s.definitions() {
			for _, d := range defs {
				if within(i, d.body) && !callbacks[d.body] {
					return true
				}
			}
		}
		return false
	}
	out, seen := []jsMock{}, map[int]bool{}
	for k := range s.tokens {
		m, ok := r.mockAt(k, specs, names, stem)
		if !ok || seen[m.line] || inDefinition(k) || !r.applies(k, e, chain) {
			continue
		}
		seen[m.line] = true
		out = append(out, m)
	}
	return out
}

// mockAt classifies the mock starting at token k (TSA-V0-079).
func (r *jsReader) mockAt(k int, specs, names map[string]bool, stem string) (jsMock, bool) {
	s := r.s
	m := jsMock{at: k}
	if !s.ident(k, "") || s.memberAccess(k) || k >= len(s.tokens) {
		return m, false
	}
	m.line = s.tokens[k].Line
	root := s.tokens[k].Text
	if jsBoundaryGlobals[root] && s.punct(k+1, ".") && s.ident(k+2, "fetch") && s.punct(k+3, "=") && !s.punct(k+4, "=") {
		m.what = root + ".fetch"
		return m, true
	}
	if (root != "jest" && root != "vi") || !s.punct(k+1, ".") || !s.ident(k+2, "") || !s.punct(k+3, "(") || s.match[k+3] < 0 {
		return m, false
	}
	member := s.tokens[k+2].Text
	args := s.splitArgs(k+3, s.match[k+3])
	switch {
	case jsTimerMocks[member]:
		m.what = "timers"
		return m, true
	case jsModuleMocks[member] && len(args) > 0 && s.loneString(args[0]) && s.tokens[args[0][0]].Literal:
		spec := s.tokens[args[0][0]].Text
		m.what = spec
		switch {
		case boundaryModule(spec):
			return m, true
		case specs[spec] || specStem(spec) == stem:
			m.subject = true
			return m, true
		}
	case member == "spyOn" && len(args) >= 2 && s.ident(args[0][0], ""):
		receiver := s.tokens[args[0][0]].Text
		method := ""
		if s.loneString(args[1]) {
			method = s.tokens[args[1][0]].Text
		}
		m.what = receiver
		if method != "" {
			m.what += "." + method
		}
		b, bound := r.bindings[receiver]
		switch {
		case jsBoundaryGlobals[receiver] && jsBoundarySpied[method], receiver == "Date", jsBoundaryModules[receiver],
			bound && boundaryModule(b.spec):
			return m, true
		case names[receiver]:
			m.subject = true
			return m, true
		}
	}
	return m, false
}
