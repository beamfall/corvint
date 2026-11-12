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
func chFindings(s *source) []testvalidity.SourceFinding {
	fixtures := chFixtures(s)
	out := []testvalidity.SourceFinding{}
	for _, name := range chSortedNames(fixtures) {
		f := fixtures[name]
		if f.line == 0 || f.scope != "test" {
			continue
		}
		params, body, ok := chFunction(s, f.fn)
		if ok {
			out = append(out, chRegistrations(s, fixtures, params, body, "", name)...)
		}
	}
	for _, d := range s.Decls {
		if d.Kind != "test" {
			continue
		}
		params, body, ok := chFunction(s, d.Body)
		if ok {
			out = append(out, chRegistrations(s, fixtures, params, body, fmt.Sprintf("%s:%d", s.Path, d.Line), d.Title)...)
		}
	}
	return append(out, chAfterAll(s, fixtures)...)
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
func chFixtures(s *source) map[string]chFixture {
	fixtures := map[string]chFixture{}
	for name, scope := range chBuiltins {
		fixtures[name] = chFixture{name: name, scope: scope}
	}
	for i := range s.Tokens {
		if !s.Ident(i, "extend") || !s.MemberAccess(i) {
			continue
		}
		open := chSkipTypeArguments(s, i+1)
		if !s.Punct(open, "(") || s.Match[open] < 0 || !s.Punct(open+1, "{") || s.Match[open+1] < 0 {
			continue
		}
		for _, p := range s.SplitArgs(open+1, s.Match[open+1]) {
			if f, ok := chProperty(s, p); ok {
				fixtures[f.name] = f
			}
		}
	}
	return fixtures
}

// chSkipTypeArguments returns the index after a `<...>` type argument list at token i, or i.
func chSkipTypeArguments(s *source, i int) int {
	if !s.Punct(i, "<") {
		return i
	}
	depth := 0
	for j := i; j < len(s.Tokens); j++ {
		switch {
		case s.Punct(j, "<"):
			depth++
		case s.Punct(j, ">") && !s.Arrow(j-1):
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return i
}

// chProperty reads one fixture property: `name: fn` (test scope) or `name: [fn, { scope: X }]`.
func chProperty(s *source, p [2]int) (chFixture, bool) {
	key := p[0]
	if !(s.Ident(key, "") || s.LoneString([2]int{key, key + 1})) || !s.Punct(key+1, ":") {
		return chFixture{}, false
	}
	f := chFixture{name: s.Tokens[key].Text, line: s.Tokens[key].Line, scope: "test"}
	value := [2]int{key + 2, p[1]}
	if s.Callback(value) {
		f.fn = value
		return f, true
	}
	if !s.Punct(value[0], "[") || s.Match[value[0]] != value[1]-1 {
		return chFixture{}, false
	}
	elements := s.SplitArgs(value[0], value[1]-1)
	if len(elements) == 0 || !s.Callback(elements[0]) {
		return chFixture{}, false
	}
	f.fn = elements[0]
	if len(elements) > 1 && s.Punct(elements[1][0], "{") {
		for j := elements[1][0] + 1; j < elements[1][1]; j++ {
			if sourceKey(s, j, "scope") {
				f.scope = ""
				if s.LoneString([2]int{j + 2, j + 3}) && s.Tokens[j+2].Literal {
					f.scope = s.Tokens[j+2].Text
				}
			}
		}
	}
	return f, true
}

// chFunction reads a function or arrow function range into its parameter range and body range.
func chFunction(s *source, r [2]int) (params, body [2]int, ok bool) {
	if r[1] <= r[0] {
		return params, body, false
	}
	i := r[0]
	if s.Ident(i, "async") {
		i++
	}
	if s.Ident(i, "function") {
		i++
		if s.Ident(i, "") {
			i++
		}
	}
	var after int
	switch {
	case s.Punct(i, "(") && s.Match[i] > 0:
		params, after = [2]int{i + 1, s.Match[i]}, s.Match[i]+1
		if !s.Ident(r[0], "function") && !s.Ident(r[0]+1, "function") {
			if !s.Arrow(after) {
				return params, body, false
			}
			after += 2
		}
	case s.Ident(i, "") && s.Arrow(i+1):
		params, after = [2]int{i, i + 1}, i+3
	default:
		return params, body, false
	}
	if s.Punct(after, "{") && s.Match[after] > 0 {
		return params, [2]int{after, s.Match[after] + 1}, true
	}
	return params, [2]int{after, r[1]}, after < r[1]
}

// chBindings maps each local name the first parameter destructures (`{ request, reg: r }`) to the
// fixture it names.
func chBindings(s *source, params [2]int) map[string]string {
	bindings := map[string]string{}
	if !s.Punct(params[0], "{") || s.Match[params[0]] < 0 {
		return bindings
	}
	for _, e := range s.SplitArgs(params[0], s.Match[params[0]]) {
		if !s.Ident(e[0], "") {
			continue
		}
		local := s.Tokens[e[0]].Text
		if s.Punct(e[0]+1, ":") && s.Ident(e[0]+2, "") {
			local = s.Tokens[e[0]+2].Text
		}
		bindings[local] = s.Tokens[e[0]].Text
	}
	return bindings
}

// chHandles returns the locals in body bound to a test-scoped fixture: the destructured ones and
// each `const|let|var NAME = <expression naming one>` (TSA-V0-166). Each maps to the fixture.
func chHandles(s *source, fixtures map[string]chFixture, bindings map[string]string, body [2]int) map[string]string {
	handles := map[string]string{}
	for local, fixture := range bindings {
		if fixtures[fixture].scope == "test" {
			handles[local] = fixture
		}
	}
	for j := body[0]; j < body[1]; j++ {
		if !(s.Ident(j, "const") || s.Ident(j, "let") || s.Ident(j, "var")) || !s.Ident(j+1, "") || !s.Punct(j+2, "=") || s.Arrow(j+2) {
			continue
		}
		if fixture := chNames(s, handles, [2]int{j + 3, chStatementEnd(s, j+3, body[1])}); fixture != "" {
			handles[s.Tokens[j+1].Text] = fixture
		}
	}
	return handles
}

// chStatementEnd returns the end of the expression starting at token i: the first top-level `;`,
// or the first top-level token on a later line.
func chStatementEnd(s *source, i, end int) int {
	line := s.Tokens[min(i, len(s.Tokens)-1)].Line
	for j := i; j < end; j++ {
		if s.Punct(j, ";") || s.Tokens[j].Line != line {
			return j
		}
		if s.Match[j] > j {
			j = s.Match[j]
			line = s.Tokens[j].Line
		}
	}
	return end
}

// chNames returns the fixture of the first root identifier in r that names one of handles.
func chNames(s *source, handles map[string]string, r [2]int) string {
	for j := r[0]; j < r[1]; j++ {
		if s.Ident(j, "") && !s.MemberAccess(j) && !s.Punct(j+1, ":") {
			if fixture, ok := handles[s.Tokens[j].Text]; ok {
				return fixture
			}
		}
	}
	return ""
}

// chRegistrations reports each `R.method(..., closure, ...)` in body where R is bound to a
// worker-scoped fixture and the closure captures a test-scoped handle the closure does not
// rebind as its own parameter.
func chRegistrations(s *source, fixtures map[string]chFixture, params, body [2]int, test, title string) []testvalidity.SourceFinding {
	bindings := chBindings(s, params)
	handles := chHandles(s, fixtures, bindings, body)
	out := []testvalidity.SourceFinding{}
	for j := body[0]; j < body[1]; j++ {
		if !s.Ident(j, "") || s.MemberAccess(j) || !s.Punct(j+1, ".") || !s.Ident(j+2, "") || !s.Punct(j+3, "(") || s.Match[j+3] < 0 {
			continue
		}
		bound := bindings[s.Tokens[j].Text]
		registry, ok := fixtures[bound]
		if bound != "" && !ok {
			// The registry is a fixture declared in another file (TSA-V0-303).
			chAbstainCrossFile(s, handles, j, test, bound)
			continue
		}
		if !ok || registry.scope != "worker" || registry.line == 0 {
			continue
		}
		for _, arg := range s.SplitArgs(j+3, s.Match[j+3]) {
			if !s.Callback(arg) {
				continue
			}
			cbParams, cbBody, ok := chFunction(s, arg)
			if !ok {
				continue
			}
			local := chWithout(handles, s, cbParams)
			chDropDeclared(s, local, cbBody)
			captured := chNames(s, local, cbBody)
			if captured == "" {
				continue
			}
			out = append(out, chFinding(s, fixtures, test, title, s.Tokens[j].Line, fmt.Sprintf("%s:%d", s.Path, registry.line),
				fmt.Sprintf("worker-scoped fixture `%s`", registry.name), captured))
			break
		}
		j = s.Match[j+3]
	}
	return out
}

// chAbstainCrossFile records an abstention for the call `R.method(...)` at token j when R is bound
// to a fixture the file does not declare and one of its closures captures a test-scoped handle:
// the registry's scope is declared elsewhere, so whether it outlives the handle is not decided
// (TSA-V0-303). A closure capturing no handle records nothing.
func chAbstainCrossFile(s *source, handles map[string]string, j int, test, registry string) {
	for _, arg := range s.SplitArgs(j+3, s.Match[j+3]) {
		if !s.Callback(arg) {
			continue
		}
		cbParams, cbBody, ok := chFunction(s, arg)
		if !ok {
			continue
		}
		local := chWithout(handles, s, cbParams)
		chDropDeclared(s, local, cbBody)
		captured := chNames(s, local, cbBody)
		if captured == "" {
			continue
		}
		where := fmt.Sprintf("%s:%d", s.Path, s.Tokens[j].Line)
		if test == "" {
			test = where
		}
		im, found := xfFixtureImport(s)
		module, cause := xfWhere(s, im, found, where)
		xfAbstain(s, testvalidity.RuleCleanupHandleOutlived, testvalidity.AbstainCleanupRegistryCrossFile, test, where, cause,
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
		if s.Ident(j, "") {
			delete(out, s.Tokens[j].Text)
		}
	}
	return out
}

// chDropDeclared removes from handles every name the closure body declares with
// `const|let|var`, since that declaration shadows the outer handle in the closure.
func chDropDeclared(s *source, handles map[string]string, body [2]int) {
	for j := body[0]; j < body[1]; j++ {
		if (s.Ident(j, "const") || s.Ident(j, "let") || s.Ident(j, "var")) && s.Ident(j+1, "") {
			delete(handles, s.Tokens[j+1].Text)
		}
	}
}

func chFinding(s *source, fixtures map[string]chFixture, test, title string, line int, cause, registry, handle string) testvalidity.SourceFinding {
	where := fmt.Sprintf("%s:%d", s.Path, line)
	if test == "" {
		test = where
	}
	declared := "a Playwright built-in"
	if f := fixtures[handle]; f.line > 0 {
		declared = fmt.Sprintf("declared at %s:%d", s.Path, f.line)
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
func chAfterAll(s *source, fixtures map[string]chFixture) []testvalidity.SourceFinding {
	assigned := map[string]string{}
	for _, cb := range chCallbacks(s, "test", "beforeEach") {
		params, body, ok := chFunction(s, cb)
		if !ok {
			continue
		}
		handles := chHandles(s, fixtures, chBindings(s, params), body)
		for j := body[0]; j < body[1]; j++ {
			if !s.Ident(j, "") || s.MemberAccess(j) || !s.Punct(j+1, "=") || s.Arrow(j+1) || s.Punct(j+2, "=") || chDeclared(s, j) {
				continue
			}
			if _, local := handles[s.Tokens[j].Text]; local {
				continue
			}
			if fixture := chNames(s, handles, [2]int{j + 2, chStatementEnd(s, j+2, body[1])}); fixture != "" {
				assigned[s.Tokens[j].Text] = fixture
			}
		}
	}
	out := []testvalidity.SourceFinding{}
	for _, at := range chHooks(s, "afterAll") {
		params, body, ok := chFunction(s, chHookCallback(s, at))
		if !ok {
			continue
		}
		if fixture := chNames(s, chWithout(assigned, s, params), body); fixture != "" {
			line := s.Tokens[at].Line
			out = append(out, chFinding(s, fixtures, "", "afterAll", line, fmt.Sprintf("%s:%d", s.Path, line), "the worker-scoped `afterAll` hook", fixture))
		}
	}
	return out
}

// chDeclared reports a name token that a const, let or var declares.
func chDeclared(s *source, j int) bool {
	return s.Ident(j-1, "const") || s.Ident(j-1, "let") || s.Ident(j-1, "var")
}

// chCallbacks returns the callback ranges of every test declaration and every named hook.
func chCallbacks(s *source, kind string, hooks ...string) [][2]int {
	out := [][2]int{}
	for _, d := range s.Decls {
		if d.Kind == kind {
			out = append(out, d.Body)
		}
	}
	for _, hook := range hooks {
		for _, at := range chHooks(s, hook) {
			out = append(out, chHookCallback(s, at))
		}
	}
	return out
}

// chHooks returns the token index of every `hook(` or `test.hook(` call.
func chHooks(s *source, hook string) []int {
	out := []int{}
	for i := range s.Tokens {
		if s.Ident(i, hook) && s.Punct(i+1, "(") && s.Match[i+1] > 0 && (!s.MemberAccess(i) || s.Ident(i-2, "test")) {
			out = append(out, i)
		}
	}
	return out
}

// chHookCallback returns the last argument of the hook call at token i.
func chHookCallback(s *source, i int) [2]int {
	args := s.SplitArgs(i+1, s.Match[i+1])
	if len(args) == 0 || !s.Callback(args[len(args)-1]) {
		return [2]int{}
	}
	return args[len(args)-1]
}
