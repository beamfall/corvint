package testsource

import (
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// CLEANUP-HANDLE-OUTLIVED reads (TSA-V0-165..167): a cleanup closure registered on a longer-lived
// (worker-scoped) registry that captures a shorter-lived (test-scoped) fixture handle, which is
// already disposed when the registry runs at worker shutdown.

// chFixture is one Playwright fixture the file declares through `.extend({...})`, or a built-in.
type chFixture struct {
	name  string
	line  int    // declaration line; 0 for a built-in
	scope string // "test" or "worker"; "" when the declared scope is not a literal
	fn    [2]int // the fixture function's token range; empty for a built-in
}

// chBuiltins are Playwright's built-in fixtures and their scopes.
var chBuiltins = map[string]string{"page": "test", "context": "test", "request": "test", "browser": "worker", "playwright": "worker", "browserName": "worker"}

// chFindings reports every cleanup registration whose closure captures a handle that its registry
// outlives (TSA-V0-165, TSA-V0-166). A registry or handle whose scope the file does not state is
// not reported (TSA-V0-167).
func (s *source) chFindings() []testvalidity.SourceFinding {
	fixtures := s.chFixtures()
	out := []testvalidity.SourceFinding{}
	for _, name := range chSortedNames(fixtures) {
		f := fixtures[name]
		if f.line == 0 || f.scope != "test" {
			continue
		}
		params, body, ok := s.chFunction(f.fn)
		if ok {
			out = append(out, s.chRegistrations(fixtures, params, body, "", name)...)
		}
	}
	for _, d := range s.decls {
		if d.kind != "test" {
			continue
		}
		params, body, ok := s.chFunction(d.body)
		if ok {
			out = append(out, s.chRegistrations(fixtures, params, body, fmt.Sprintf("%s:%d", s.path, d.line), d.title)...)
		}
	}
	return append(out, s.chAfterAll(fixtures)...)
}

func chSortedNames(fixtures map[string]chFixture) []string {
	names := []string{}
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Slice(names, func(a, b int) bool { return fixtures[names[a]].line < fixtures[names[b]].line })
	return names
}

// chFixtures reads every `X.extend<...>({ name: fn | [fn, { scope: 'worker' }] })` call, over the
// built-ins; a later declaration of a name replaces an earlier one.
func (s *source) chFixtures() map[string]chFixture {
	fixtures := map[string]chFixture{}
	for name, scope := range chBuiltins {
		fixtures[name] = chFixture{name: name, scope: scope}
	}
	for i := range s.tokens {
		if !s.ident(i, "extend") || !s.memberAccess(i) {
			continue
		}
		open := s.chSkipTypeArguments(i + 1)
		if !s.punct(open, "(") || s.match[open] < 0 || !s.punct(open+1, "{") || s.match[open+1] < 0 {
			continue
		}
		for _, p := range s.splitArgs(open+1, s.match[open+1]) {
			if f, ok := s.chProperty(p); ok {
				fixtures[f.name] = f
			}
		}
	}
	return fixtures
}

// chSkipTypeArguments returns the index after a `<...>` type argument list at token i, or i.
func (s *source) chSkipTypeArguments(i int) int {
	if !s.punct(i, "<") {
		return i
	}
	depth := 0
	for j := i; j < len(s.tokens); j++ {
		switch {
		case s.punct(j, "<"):
			depth++
		case s.punct(j, ">") && !s.arrow(j-1):
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return i
}

// chProperty reads one fixture property: `name: fn` (test scope) or `name: [fn, { scope: X }]`.
func (s *source) chProperty(p [2]int) (chFixture, bool) {
	key := p[0]
	if !(s.ident(key, "") || s.loneString([2]int{key, key + 1})) || !s.punct(key+1, ":") {
		return chFixture{}, false
	}
	f := chFixture{name: s.tokens[key].Text, line: s.tokens[key].Line, scope: "test"}
	value := [2]int{key + 2, p[1]}
	if s.callback(value) {
		f.fn = value
		return f, true
	}
	if !s.punct(value[0], "[") || s.match[value[0]] != value[1]-1 {
		return chFixture{}, false
	}
	elements := s.splitArgs(value[0], value[1]-1)
	if len(elements) == 0 || !s.callback(elements[0]) {
		return chFixture{}, false
	}
	f.fn = elements[0]
	if len(elements) > 1 && s.punct(elements[1][0], "{") {
		for j := elements[1][0] + 1; j < elements[1][1]; j++ {
			if s.key(j, "scope") {
				f.scope = ""
				if s.loneString([2]int{j + 2, j + 3}) && s.tokens[j+2].Literal {
					f.scope = s.tokens[j+2].Text
				}
			}
		}
	}
	return f, true
}

// chFunction reads a function or arrow function range into its parameter range and body range.
func (s *source) chFunction(r [2]int) (params, body [2]int, ok bool) {
	if r[1] <= r[0] {
		return params, body, false
	}
	i := r[0]
	if s.ident(i, "async") {
		i++
	}
	if s.ident(i, "function") {
		i++
		if s.ident(i, "") {
			i++
		}
	}
	var after int
	switch {
	case s.punct(i, "(") && s.match[i] > 0:
		params, after = [2]int{i + 1, s.match[i]}, s.match[i]+1
		if !s.ident(r[0], "function") && !s.ident(r[0]+1, "function") {
			if !s.arrow(after) {
				return params, body, false
			}
			after += 2
		}
	case s.ident(i, "") && s.arrow(i+1):
		params, after = [2]int{i, i + 1}, i+3
	default:
		return params, body, false
	}
	if s.punct(after, "{") && s.match[after] > 0 {
		return params, [2]int{after, s.match[after] + 1}, true
	}
	return params, [2]int{after, r[1]}, after < r[1]
}

// chBindings maps each local name the first parameter destructures (`{ request, reg: r }`) to the
// fixture it names.
func (s *source) chBindings(params [2]int) map[string]string {
	bindings := map[string]string{}
	if !s.punct(params[0], "{") || s.match[params[0]] < 0 {
		return bindings
	}
	for _, e := range s.splitArgs(params[0], s.match[params[0]]) {
		if !s.ident(e[0], "") {
			continue
		}
		local := s.tokens[e[0]].Text
		if s.punct(e[0]+1, ":") && s.ident(e[0]+2, "") {
			local = s.tokens[e[0]+2].Text
		}
		bindings[local] = s.tokens[e[0]].Text
	}
	return bindings
}

// chHandles returns the locals in body bound to a test-scoped fixture: the destructured ones and
// each `const|let|var NAME = <expression naming one>` (TSA-V0-166). Each maps to the fixture.
func (s *source) chHandles(fixtures map[string]chFixture, bindings map[string]string, body [2]int) map[string]string {
	handles := map[string]string{}
	for local, fixture := range bindings {
		if fixtures[fixture].scope == "test" {
			handles[local] = fixture
		}
	}
	for j := body[0]; j < body[1]; j++ {
		if !(s.ident(j, "const") || s.ident(j, "let") || s.ident(j, "var")) || !s.ident(j+1, "") || !s.punct(j+2, "=") || s.arrow(j+2) {
			continue
		}
		if fixture := s.chNames(handles, [2]int{j + 3, s.chStatementEnd(j+3, body[1])}); fixture != "" {
			handles[s.tokens[j+1].Text] = fixture
		}
	}
	return handles
}

// chStatementEnd returns the end of the expression starting at token i: the first top-level `;`,
// or the first top-level token on a later line.
func (s *source) chStatementEnd(i, end int) int {
	line := s.tokens[min(i, len(s.tokens)-1)].Line
	for j := i; j < end; j++ {
		if s.punct(j, ";") || s.tokens[j].Line != line {
			return j
		}
		if s.match[j] > j {
			j = s.match[j]
			line = s.tokens[j].Line
		}
	}
	return end
}

// chNames returns the fixture of the first root identifier in r that names one of handles.
func (s *source) chNames(handles map[string]string, r [2]int) string {
	for j := r[0]; j < r[1]; j++ {
		if s.ident(j, "") && !s.memberAccess(j) && !s.punct(j+1, ":") {
			if fixture, ok := handles[s.tokens[j].Text]; ok {
				return fixture
			}
		}
	}
	return ""
}

// chRegistrations reports each `R.method(..., closure, ...)` in body where R is bound to a
// worker-scoped fixture and the closure captures a test-scoped handle the closure does not
// rebind as its own parameter.
func (s *source) chRegistrations(fixtures map[string]chFixture, params, body [2]int, test, title string) []testvalidity.SourceFinding {
	bindings := s.chBindings(params)
	handles := s.chHandles(fixtures, bindings, body)
	out := []testvalidity.SourceFinding{}
	for j := body[0]; j < body[1]; j++ {
		if !s.ident(j, "") || s.memberAccess(j) || !s.punct(j+1, ".") || !s.ident(j+2, "") || !s.punct(j+3, "(") || s.match[j+3] < 0 {
			continue
		}
		bound := bindings[s.tokens[j].Text]
		registry, ok := fixtures[bound]
		if bound != "" && !ok {
			// The registry is a fixture declared in another file (TSA-V0-303).
			s.chAbstainCrossFile(handles, j, test, bound)
			continue
		}
		if !ok || registry.scope != "worker" || registry.line == 0 {
			continue
		}
		for _, arg := range s.splitArgs(j+3, s.match[j+3]) {
			if !s.callback(arg) {
				continue
			}
			cbParams, cbBody, ok := s.chFunction(arg)
			if !ok {
				continue
			}
			local := chWithout(handles, s, cbParams)
			s.chDropDeclared(local, cbBody)
			captured := s.chNames(local, cbBody)
			if captured == "" {
				continue
			}
			out = append(out, s.chFinding(fixtures, test, title, s.tokens[j].Line, fmt.Sprintf("%s:%d", s.path, registry.line),
				fmt.Sprintf("worker-scoped fixture `%s`", registry.name), captured))
			break
		}
		j = s.match[j+3]
	}
	return out
}

// chAbstainCrossFile records an abstention for the call `R.method(...)` at token j when R is bound
// to a fixture the file does not declare and one of its closures captures a test-scoped handle:
// the registry's scope is declared elsewhere, so whether it outlives the handle is not decided
// (TSA-V0-303). A closure capturing no handle records nothing.
func (s *source) chAbstainCrossFile(handles map[string]string, j int, test, registry string) {
	for _, arg := range s.splitArgs(j+3, s.match[j+3]) {
		if !s.callback(arg) {
			continue
		}
		cbParams, cbBody, ok := s.chFunction(arg)
		if !ok {
			continue
		}
		local := chWithout(handles, s, cbParams)
		s.chDropDeclared(local, cbBody)
		captured := s.chNames(local, cbBody)
		if captured == "" {
			continue
		}
		where := fmt.Sprintf("%s:%d", s.path, s.tokens[j].Line)
		if test == "" {
			test = where
		}
		im, found := s.xfFixtureImport()
		module, cause := s.xfWhere(im, found, where)
		s.xfAbstain(testvalidity.RuleCleanupHandleOutlived, testvalidity.AbstainCleanupRegistryCrossFile, test, where, cause,
			fmt.Sprintf("cleanup registered on `%s` captures the test-scoped fixture `%s`, but `%s` is not declared in this file, so its scope is declared in %s, which this reading does not follow; CLEANUP-HANDLE-OUTLIVED is not decided for this registration", registry, captured, registry, module))
		return
	}
}

// chWithout returns handles less every name the parameter range declares.
func chWithout(handles map[string]string, s *source, params [2]int) map[string]string {
	out := map[string]string{}
	for k, v := range handles {
		out[k] = v
	}
	for j := params[0]; j < params[1]; j++ {
		if s.ident(j, "") {
			delete(out, s.tokens[j].Text)
		}
	}
	return out
}

// chDropDeclared removes from handles every name the closure body declares with
// `const|let|var`, since that declaration shadows the outer handle in the closure.
func (s *source) chDropDeclared(handles map[string]string, body [2]int) {
	for j := body[0]; j < body[1]; j++ {
		if (s.ident(j, "const") || s.ident(j, "let") || s.ident(j, "var")) && s.ident(j+1, "") {
			delete(handles, s.tokens[j+1].Text)
		}
	}
}

func (s *source) chFinding(fixtures map[string]chFixture, test, title string, line int, cause, registry, handle string) testvalidity.SourceFinding {
	where := fmt.Sprintf("%s:%d", s.path, line)
	if test == "" {
		test = where
	}
	declared := "a Playwright built-in"
	if f := fixtures[handle]; f.line > 0 {
		declared = fmt.Sprintf("declared at %s:%d", s.path, f.line)
	}
	return testvalidity.SourceFinding{
		Rule: testvalidity.RuleCleanupHandleOutlived, Severity: testvalidity.SeverityAdvisory,
		Test: test, Title: title, Location: where, Cause: cause,
		Message: fmt.Sprintf("cleanup registered on %s (%s) captures the test-scoped fixture `%s` (%s), which is disposed when its test ends, before the registry runs at worker shutdown, so the cleanup fails and leaves data behind; create a dedicated context inside the cleanup (for example `await playwright.request.newContext()`) or run the cleanup before `%s` tears down",
			registry, cause, handle, declared, handle),
	}
}

// chAfterAll reports an `afterAll` hook that reads an outer variable a test or beforeEach body
// assigned from a test-scoped fixture (TSA-V0-166): the hook runs once per worker, after every
// test's handles are disposed.
func (s *source) chAfterAll(fixtures map[string]chFixture) []testvalidity.SourceFinding {
	assigned := map[string]string{}
	for _, cb := range s.chCallbacks("test", "beforeEach") {
		params, body, ok := s.chFunction(cb)
		if !ok {
			continue
		}
		handles := s.chHandles(fixtures, s.chBindings(params), body)
		for j := body[0]; j < body[1]; j++ {
			if !s.ident(j, "") || s.memberAccess(j) || !s.punct(j+1, "=") || s.arrow(j+1) || s.punct(j+2, "=") || s.chDeclared(j) {
				continue
			}
			if _, local := handles[s.tokens[j].Text]; local {
				continue
			}
			if fixture := s.chNames(handles, [2]int{j + 2, s.chStatementEnd(j+2, body[1])}); fixture != "" {
				assigned[s.tokens[j].Text] = fixture
			}
		}
	}
	out := []testvalidity.SourceFinding{}
	for _, at := range s.chHooks("afterAll") {
		params, body, ok := s.chFunction(s.chHookCallback(at))
		if !ok {
			continue
		}
		if fixture := s.chNames(chWithout(assigned, s, params), body); fixture != "" {
			line := s.tokens[at].Line
			out = append(out, s.chFinding(fixtures, "", "afterAll", line, fmt.Sprintf("%s:%d", s.path, line), "the worker-scoped `afterAll` hook", fixture))
		}
	}
	return out
}

// chDeclared reports a name token that a const, let or var declares.
func (s *source) chDeclared(j int) bool {
	return s.ident(j-1, "const") || s.ident(j-1, "let") || s.ident(j-1, "var")
}

// chCallbacks returns the callback ranges of every test declaration and every named hook.
func (s *source) chCallbacks(kind string, hooks ...string) [][2]int {
	out := [][2]int{}
	for _, d := range s.decls {
		if d.kind == kind {
			out = append(out, d.body)
		}
	}
	for _, hook := range hooks {
		for _, at := range s.chHooks(hook) {
			out = append(out, s.chHookCallback(at))
		}
	}
	return out
}

// chHooks returns the token index of every `hook(` or `test.hook(` call.
func (s *source) chHooks(hook string) []int {
	out := []int{}
	for i := range s.tokens {
		if s.ident(i, hook) && s.punct(i+1, "(") && s.match[i+1] > 0 && (!s.memberAccess(i) || s.ident(i-2, "test")) {
			out = append(out, i)
		}
	}
	return out
}

// chHookCallback returns the last argument of the hook call at token i.
func (s *source) chHookCallback(i int) [2]int {
	args := s.splitArgs(i+1, s.match[i+1])
	if len(args) == 0 || !s.callback(args[len(args)-1]) {
		return [2]int{}
	}
	return args[len(args)-1]
}
