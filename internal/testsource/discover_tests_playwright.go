package testsource

import (
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The Playwright half of the static test discovery (DCX-V1-371..376). It reuses the TSA-V0-073
// example reader (jsReader) for placement, titles and abstention, and its token helpers for
// expectations, calls and imports.

var (
	// jsHookNames are the hooks a describe or the file applies to its examples, with their scope.
	jsHookNames = map[string]string{"beforeEach": "each", "afterEach": "each", "beforeAll": "all", "afterAll": "all"}
	// jsNavigationMembers are the page calls a test navigates, routes or awaits a request with.
	jsNavigationMembers = map[string]bool{"goto": true, "route": true, "waitForResponse": true, "waitForRequest": true, "waitForURL": true}
	// jsRequestMethods are the APIRequestContext helpers.
	jsRequestMethods = map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true, "head": true, "fetch": true}
	// jsRuntimeAnnotations are the modifiers a titleless runtime call such as test.skip() sets.
	jsRuntimeAnnotations = map[string]bool{"skip": true, "fixme": true, "fail": true, "slow": true}
	// jsDeclAnnotations are the declaration members discovery reports as annotations.
	jsDeclAnnotations = []string{"fixme", "only", "skip", "todo"}
	// playwrightFixtures are the fixtures @playwright/test itself provides.
	playwrightFixtures = map[string]bool{"page": true, "context": true, "browser": true, "browserName": true, "request": true,
		"baseURL": true, "playwright": true, "headless": true, "channel": true, "viewport": true, "storageState": true,
		"locale": true, "timezoneId": true, "colorScheme": true, "isMobile": true, "hasTouch": true, "javaScriptEnabled": true,
		"geolocation": true, "permissions": true, "extraHTTPHeaders": true, "httpCredentials": true, "offline": true,
		"userAgent": true, "deviceScaleFactor": true, "launchOptions": true, "contextOptions": true, "testIdAttribute": true}
	jsTitleTag = regexp.MustCompile(`(?:^|\s)(@[A-Za-z0-9_][A-Za-z0-9_:.\-]*)`)
)

// jsDiscoveryHook is one hook call the reader placed.
type jsDiscoveryHook struct {
	at, line, endLine int
	kind, scope       string
	body              [2]int
	hasBody           bool
}

// jsRuntime is one titleless runtime modifier call, such as test.skip(condition, reason).
type jsRuntime struct {
	at, line, endLine int
	kind, description string
	conditional       bool
}

type jsDiscovery struct {
	f       *discoveryFile
	r       *jsReader
	s       *source
	hooks   []jsDiscoveryHook
	runtime []jsRuntime
}

// discoverJS reads one Playwright test file and returns "" or the file's uncertainty reason.
func (f *discoveryFile) discoverJS() string {
	s := parseSource(f.path, f.text)
	if s.uncertain {
		f.unknown("", validitySourceUnparsed, "the lexer could not place some source or a bracket does not balance, so no example is read", 0, 0)
		return validitySourceUnparsed
	}
	r := &jsReader{s: s, name: f.path}
	r.read()
	d := &jsDiscovery{f: f, r: r, s: s}
	d.readCalls()
	runtimeAt := map[int]bool{}
	for _, a := range d.runtime {
		runtimeAt[a.at] = true
	}
	perLine := map[int]int{}
	for _, e := range r.decls {
		if e.kind == "test" && !runtimeAt[e.at] {
			perLine[e.line]++
		}
	}
	for _, e := range r.decls {
		if e.kind != "test" || runtimeAt[e.at] {
			continue
		}
		d.example(e, perLine[e.line])
	}
	return ""
}

// tokenEnd is the last line token i covers.
func (d *jsDiscovery) tokenEnd(i int) int {
	t := d.s.tokens[i]
	raw := t.Source
	if raw == "" {
		raw = t.Text
	}
	return t.Line + strings.Count(raw, "\n")
}

// rangeLines is the first and last line of the token range [start, end).
func (d *jsDiscovery) rangeLines(start, end int) (int, int) {
	if end <= start {
		end = start + 1
	}
	return d.s.tokens[start].Line, d.tokenEnd(end - 1)
}

// readCalls places every hook call and every titleless runtime modifier call.
func (d *jsDiscovery) readCalls() {
	s := d.s
	for i := range s.tokens {
		if !s.ident(i, "") || s.ident(i-1, "function") || !s.punct(i+1, "(") || s.match[i+1] < 0 {
			continue
		}
		name := s.tokens[i].Text
		testMember := s.memberAccess(i) && s.ident(i-2, "") && testRoots[s.tokens[i-2].Text] && !s.memberAccess(i-2)
		args := s.splitArgs(i+1, s.match[i+1])
		if scope, ok := jsHookNames[name]; ok && (testMember || !s.memberAccess(i)) {
			h := jsDiscoveryHook{at: i, line: s.tokens[i].Line, endLine: s.tokens[s.match[i+1]].Line, kind: name, scope: scope}
			for _, a := range args {
				if body, _, ok := s.functionBody(a[0]); ok && body[1] <= a[1] {
					h.body, h.hasBody = body, true
					break
				}
			}
			d.hooks = append(d.hooks, h)
			continue
		}
		if !testMember || !jsRuntimeAnnotations[name] {
			continue
		}
		// test.skip('title', fn) declares an example; every other form is a runtime modifier.
		declares := false
		if len(args) > 1 && s.loneString(args[0]) {
			for _, a := range args[1:] {
				if _, _, ok := s.functionBody(a[0]); ok || s.callback(a) {
					declares = true
				}
			}
		}
		if declares {
			continue
		}
		a := jsRuntime{at: i - 2, line: s.tokens[i-2].Line, endLine: s.tokens[s.match[i+1]].Line, kind: name}
		switch {
		case len(args) == 0:
		case len(args) == 1 && s.loneString(args[0]):
			a.description = s.tokens[args[0][0]].Text
		default:
			a.conditional = !(args[0][1]-args[0][0] == 1 && s.ident(args[0][0], "true"))
			if len(args) > 1 && s.loneString(args[1]) {
				a.description = s.tokens[args[1][0]].Text
			}
		}
		d.runtime = append(d.runtime, a)
	}
}

// reaches reports that a statement at token i applies to example e under chain (its describes,
// innermost first): in e's body, or at file or describe level outside every example body.
func (d *jsDiscovery) reaches(i int, e jsDecl, chain []int) bool {
	if e.hasBody && within(i, e.body) {
		return true
	}
	if d.r.inExampleBody(i, -1) {
		return false
	}
	own := d.r.ancestors(i)
	return len(own) == 0 || slices.Contains(chain, own[0])
}

// declArgs recomputes the declaring call's argument ranges of decl e, as jsReader.read walks it.
func (d *jsDiscovery) declArgs(e jsDecl) [][2]int {
	s := d.s
	j := e.at + 1
	for s.punct(j, ".") && s.ident(j+1, "") {
		member := s.tokens[j+1].Text
		j += 2
		if jsUnsupportedMembers[member] && s.punct(j, "(") && s.match[j] > j && s.punct(s.match[j]+1, "(") {
			j = s.match[j] + 1
		}
		if member == "each" {
			switch {
			case s.punct(j, "(") && s.match[j] > j:
				j = s.match[j] + 1
			case j < len(s.tokens) && s.tokens[j].Kind == appmap.JSTemplate:
				j++
			}
		}
	}
	if !s.punct(j, "(") || s.match[j] < 0 {
		return nil
	}
	return s.splitArgs(j, s.match[j])
}

func (d *jsDiscovery) example(e jsDecl, sameLine int) {
	f, r, s := d.f, d.r, d.s
	out, ok := f.newExample(e.line, max(e.line, e.endLine))
	if !ok {
		return
	}
	chain := r.ancestors(e.at)
	out.Identity = ExampleIdentity{Kind: ExampleIdentityJS, File: f.path, Unknown: discoverReasonProjectNotStatic}
	reason := r.abstention(e, sameLine)
	var titles []string
	if reason != testvalidity.HygieneReasonJSLineAmbiguous {
		titles = r.titlePath(e)
	}
	if titles == nil {
		out.TitleUnknown = discoverReasonTitleNotLiteral
		if reason == testvalidity.HygieneReasonJSLineAmbiguous {
			out.TitleUnknown = reason
		}
		out.Identity.Unknown = out.TitleUnknown
		f.unknown(out.ID, out.TitleUnknown, "a title on the example's path is not one literal string, so neither the title path nor the js-title-path identity is fixed", e.line, e.line)
	} else {
		out.Identity.FullName = strings.Join(titles, " > ")
		decls := append([]int{}, chain...)
		slices.Reverse(decls)
		for k, title := range titles {
			line := e.line
			if k < len(decls) {
				line = r.decls[decls[k]].line
			}
			a, _ := f.anchor(line, line, "static test discovery title")
			out.TitlePath = append(out.TitlePath, DiscoveredTitle{Title: title, Evidence: a})
		}
	}
	// Tags and annotations come from the declarations, outermost describe first.
	for k := len(chain) - 1; k >= 0; k-- {
		d.declFacts(&out, r.decls[chain[k]])
	}
	d.declFacts(&out, e)
	for _, a := range d.runtime {
		if !d.reaches(a.at, e, chain) {
			continue
		}
		anchor, _ := f.anchor(a.line, a.endLine, "static test discovery runtime annotation")
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: a.kind, Description: a.description, Conditional: a.conditional, Evidence: anchor})
	}
	if reason != "" {
		out.Read = reason
		f.unknown(out.ID, reason, "the TSA-V0-073 reader abstains on the example, so its body facts are not read", e.line, e.line)
		f.capFacts(&out)
		f.out.Examples = append(f.out.Examples, out)
		return
	}
	hooks := []jsDiscoveryHook{}
	for _, h := range d.hooks {
		if r.inExampleBody(h.at, -1) || !d.reaches(h.at, e, chain) {
			continue
		}
		hooks = append(hooks, h)
		asserts := false
		if h.hasBody {
			for k := h.body[0]; k < h.body[1]; k++ {
				if s.jsAssertion(k) {
					asserts = true
					break
				}
			}
		}
		anchor, _ := f.anchor(h.line, h.endLine, "static test discovery hook")
		out.Hooks = append(out.Hooks, DiscoveredHook{Kind: h.kind, Scope: h.scope, Asserts: asserts, Evidence: anchor})
		if h.hasBody {
			d.navigations(&out, h.body, "hook")
		} else {
			f.unknown(out.ID, "hook-body-not-literal", "the hook's callback is not a literal function, so its calls are not read", h.line, h.endLine)
		}
	}
	if e.hasBody {
		d.expectations(&out, e.body)
		d.navigations(&out, e.body, "example")
	}
	d.reachesOf(&out, e, hooks)
	f.capFacts(&out)
	f.out.Examples = append(f.out.Examples, out)
}

// declFacts adds decl g's title tags, details tags and annotations, and its modifiers.
func (d *jsDiscovery) declFacts(out *DiscoveredExample, g jsDecl) {
	f, s := d.f, d.s
	// The declaration's head: from its call through the line its callback body opens on.
	head := g.line
	if g.hasBody && g.body[0] >= 0 && g.body[0] < len(s.tokens) {
		head = max(head, s.tokens[g.body[0]].Line)
	}
	anchor, _ := f.anchor(g.line, head, "static test discovery declaration")
	if g.literalTitle {
		for _, m := range jsTitleTag.FindAllStringSubmatch(g.title, -1) {
			out.Tags = append(out.Tags, DiscoveredTag{Name: m[1], Evidence: anchor})
		}
	}
	switch g.root[0] {
	case 'x':
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: "skip", Evidence: anchor})
	case 'f':
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: "only", Evidence: anchor})
	}
	for _, m := range jsDeclAnnotations {
		if g.members[m] {
			out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: m, Evidence: anchor})
		}
	}
	args := d.declArgs(g)
	if len(args) < 3 || !s.punct(args[1][0], "{") || s.match[args[1][0]] != args[1][1]-1 {
		return
	}
	open, close := args[1][0], args[1][1]-1
	for _, p := range s.splitArgs(open, close) {
		if !s.ident(p[0], "") || !s.punct(p[0]+1, ":") {
			continue
		}
		key, value := s.tokens[p[0]].Text, [2]int{p[0] + 2, p[1]}
		if value[0] >= value[1] {
			continue
		}
		at, end := d.rangeLines(p[0], p[1])
		where, _ := f.anchor(at, end, "static test discovery details "+key)
		switch key {
		case "tag":
			for _, v := range d.elements(value) {
				if s.loneString(v) && s.tokens[v[0]].Literal {
					out.Tags = append(out.Tags, DiscoveredTag{Name: s.tokens[v[0]].Text, Evidence: where})
				} else {
					f.unknown(out.ID, discoverReasonMetadata, "tag value "+s.render(v[0], v[1]), at, end)
				}
			}
		case "annotation":
			for _, v := range d.elements(value) {
				typ, desc, ok := d.annotationObject(v)
				if !ok {
					f.unknown(out.ID, discoverReasonMetadata, "annotation "+s.render(v[0], v[1]), at, end)
					continue
				}
				out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: typ, Description: desc, Evidence: where})
			}
		}
	}
}

// elements splits an array literal into its elements, or returns the value itself.
func (d *jsDiscovery) elements(v [2]int) [][2]int {
	s := d.s
	if s.punct(v[0], "[") && s.match[v[0]] == v[1]-1 {
		return s.splitArgs(v[0], v[1]-1)
	}
	return [][2]int{v}
}

// annotationObject reads `{ type: 'x', description: 'y' }` with literal strings.
func (d *jsDiscovery) annotationObject(v [2]int) (string, string, bool) {
	s := d.s
	if !s.punct(v[0], "{") || s.match[v[0]] != v[1]-1 {
		return "", "", false
	}
	typ, desc := "", ""
	for _, p := range s.splitArgs(v[0], v[1]-1) {
		if !s.ident(p[0], "") || !s.punct(p[0]+1, ":") || p[1]-p[0] != 3 || !s.loneString([2]int{p[0] + 2, p[1]}) || !s.tokens[p[0]+2].Literal {
			return "", "", false
		}
		switch s.tokens[p[0]].Text {
		case "type":
			typ = s.tokens[p[0]+2].Text
		case "description":
			desc = s.tokens[p[0]+2].Text
		default:
			return "", "", false
		}
	}
	return typ, desc, typ != ""
}

// expectations adds every assertion in body (DCX-V1-372).
func (d *jsDiscovery) expectations(out *DiscoveredExample, body [2]int) {
	f, s := d.f, d.s
	for k := body[0]; k < body[1]; k++ {
		if s.assertionCount(k) {
			continue
		}
		if c, ok := s.readExpect(k); ok {
			x := DiscoveredExpectation{Matcher: c.matcher, Receiver: boundText(s.render(c.subject[0], c.subject[1]))}
			for j := c.subject[1] + 1; j+1 <= c.end; j++ {
				if s.punct(j, ".") && s.ident(j+1, "not") {
					x.Negated = true
				}
			}
			if x.Matcher == "" {
				x.Unknown = discoverReasonMatcherUnread
			}
			start, end := d.rangeLines(k, c.end+1)
			x.Evidence, _ = f.anchor(start, end, "static test discovery expectation")
			out.Expectations = append(out.Expectations, x)
			continue
		}
		if s.ident(k, "assert") && !s.memberAccess(k) {
			call, matcher := k+1, "assert"
			if s.punct(k+1, ".") && s.ident(k+2, "") {
				call, matcher = k+3, s.tokens[k+2].Text
			}
			if s.punct(call, "(") && s.match[call] > call {
				x := DiscoveredExpectation{Matcher: matcher, Negated: strings.HasPrefix(matcher, "not") || strings.Contains(matcher, "Not")}
				if args := s.splitArgs(call, s.match[call]); len(args) > 0 {
					x.Receiver = boundText(s.render(args[0][0], args[0][1]))
				}
				start, end := d.rangeLines(k, s.match[call]+1)
				x.Evidence, _ = f.anchor(start, end, "static test discovery expectation")
				out.Expectations = append(out.Expectations, x)
				k = s.match[call]
				continue
			}
		}
		if s.jsAssertion(k) {
			line := s.tokens[k].Line
			x := DiscoveredExpectation{Unknown: discoverReasonAssertionShape}
			x.Evidence, _ = f.anchor(line, line, "static test discovery expectation")
			out.Expectations = append(out.Expectations, x)
		}
	}
}

// navigations adds every navigation, request and awaited route in body (DCX-V1-373).
func (d *jsDiscovery) navigations(out *DiscoveredExample, body [2]int, source string) {
	f, s := d.f, d.s
	for k := body[0]; k < body[1]; k++ {
		if !s.ident(k, "") || !s.memberAccess(k) || !s.punct(k+1, "(") || s.match[k+1] < 0 {
			continue
		}
		name := s.tokens[k].Text
		n := DiscoveredNavigation{Kind: name, Source: source}
		switch {
		case jsRequestMethods[name] && s.ident(k-2, "request"):
			n.Kind = "request"
			if name != "fetch" {
				n.Method = strings.ToUpper(name)
			}
		case jsNavigationMembers[name]:
		default:
			continue
		}
		close := s.match[k+1]
		args := s.splitArgs(k+1, close)
		receiver := k - 2
		for receiver > 0 && s.punct(receiver-1, ".") && s.ident(receiver-2, "") {
			receiver -= 2
		}
		n.Expression = boundText(s.render(receiver, close+1))
		switch {
		case len(args) == 0:
			n.Unknown = discoverReasonTargetNotLiteral
		case s.loneString(args[0]) && s.tokens[args[0][0]].Literal:
			n.Target = s.tokens[args[0][0]].Text
		case args[0][1]-args[0][0] == 1 && s.tokens[args[0][0]].Kind == appmap.JSString && strings.HasPrefix(s.tokens[args[0][0]].Source, "/"):
			n.Target = s.tokens[args[0][0]].Source // a regular expression pattern, as written
		default:
			n.Unknown = discoverReasonTargetNotLiteral
		}
		if n.Kind == "request" && n.Method == "" && len(args) > 1 {
			n.Method = d.optionMethod(args[1])
		}
		start, end := d.rangeLines(receiver, close+1)
		n.Evidence, _ = f.anchor(start, end, "static test discovery navigation")
		out.Navigations = append(out.Navigations, n)
	}
}

// optionMethod reads a literal `method: 'POST'` from a request.fetch options object.
func (d *jsDiscovery) optionMethod(v [2]int) string {
	s := d.s
	if !s.punct(v[0], "{") || s.match[v[0]] != v[1]-1 {
		return ""
	}
	for _, p := range s.splitArgs(v[0], v[1]-1) {
		if s.ident(p[0], "method") && s.punct(p[0]+1, ":") && p[1]-p[0] == 3 && s.tokens[p[0]+2].Literal {
			return strings.ToUpper(s.tokens[p[0]+2].Text)
		}
	}
	return ""
}

// reachesOf adds the modules, dynamic loads and fixtures example e and its hooks reach
// (DCX-V1-374).
func (d *jsDiscovery) reachesOf(out *DiscoveredExample, e jsDecl, hooks []jsDiscoveryHook) {
	f, r, s := d.f, d.r, d.s
	ranges := [][2]int{}
	if e.hasBody {
		ranges = append(ranges, e.body)
	}
	for _, h := range hooks {
		if h.hasBody {
			ranges = append(ranges, h.body)
		}
	}
	used := map[string]int{} // name to the first token using it
	for _, rg := range ranges {
		for k := rg[0]; k < rg[1]; k++ {
			if !s.ident(k, "") || s.memberAccess(k) {
				continue
			}
			name := s.tokens[k].Text
			if _, seen := used[name]; !seen {
				used[name] = k
			}
			if (name == "require" || name == "import") && s.punct(k+1, "(") && s.match[k+1] > k+1 {
				args := s.splitArgs(k+1, s.match[k+1])
				start, end := d.rangeLines(k, s.match[k+1]+1)
				where, _ := f.anchor(start, end, "static test discovery dynamic load")
				reach := DiscoveredReach{Kind: "import", Specifier: boundText(s.render(k+2, s.match[k+1])), Evidence: where}
				if len(args) == 1 && s.loneString(args[0]) && s.tokens[args[0][0]].Literal {
					reach.Specifier = s.tokens[args[0][0]].Text
					reach.Target, reach.State, reach.Reason = f.resolver.web(f.path, reach.Specifier)
				} else {
					reach.State, reach.Reason = "unresolved", discoverReasonComputedLookup
				}
				out.Reaches = append(out.Reaches, reach)
			}
		}
	}
	names := make([]string, 0, len(used))
	for name := range used {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		b, ok := r.bindings[name]
		if !ok || jsTestLibrary(b.spec) || b.spec == "@playwright/test" || testRoots[name] || describeRoots[name] {
			continue
		}
		line := s.tokens[used[name]].Line
		where, _ := f.anchor(line, line, "static test discovery use of "+name)
		decl, _ := f.anchor(b.line, b.line, "static test discovery import of "+name)
		reach := DiscoveredReach{Kind: "import", Name: name, Specifier: b.spec, Evidence: where, Declaration: &decl}
		reach.Target, reach.State, reach.Reason = f.resolver.web(f.path, b.spec)
		out.Reaches = append(out.Reaches, reach)
	}
	d.fixtures(out, e)
}

// fixtures adds the fixtures the example's callback destructures.
func (d *jsDiscovery) fixtures(out *DiscoveredExample, e jsDecl) {
	f, r, s := d.f, d.r, d.s
	if !e.hasBody {
		return
	}
	var params [2]int
	found := false
	for _, a := range d.declArgs(e) {
		if body, _, ok := s.functionBody(a[0]); ok && body == e.body {
			v := a[0]
			if s.ident(v, "async") {
				v++
			}
			if s.ident(v, "function") {
				v++
				if s.ident(v, "") {
					v++
				}
			}
			if s.punct(v, "(") && s.match[v] > v {
				params, found = [2]int{v + 1, s.match[v]}, true
			} else if s.ident(v, "") {
				params, found = [2]int{v, v + 1}, true
			}
			break
		}
	}
	if !found || params[0] >= params[1] {
		return
	}
	first := s.splitArgs(params[0]-1, params[1])
	if len(first) == 0 {
		return
	}
	p := first[0]
	line := s.tokens[p[0]].Line
	where, _ := f.anchor(line, s.tokens[p[1]-1].Line, "static test discovery fixture parameter")
	if !s.punct(p[0], "{") || s.match[p[0]] < 0 {
		if s.ident(p[0], "") && p[1]-p[0] == 1 && s.tokens[p[0]].Text != "done" {
			out.Reaches = append(out.Reaches, DiscoveredReach{Kind: "fixture", Name: s.tokens[p[0]].Text, State: "unresolved", Reason: discoverReasonComputedLookup, Evidence: where})
		}
		return
	}
	binding, bound := r.bindings[e.root]
	for _, part := range s.splitArgs(p[0], s.match[p[0]]) {
		if !s.ident(part[0], "") {
			continue
		}
		name := s.tokens[part[0]].Text
		reach := DiscoveredReach{Kind: "fixture", Name: name, Evidence: where}
		switch {
		case playwrightFixtures[name]:
			reach.State, reach.Specifier = "builtin", "@playwright/test"
		case bound && isRelativeSpec(binding.spec):
			reach.Specifier = binding.spec
			reach.Target, reach.State, reach.Reason = f.resolver.web(f.path, binding.spec)
			if reach.State == "repository" {
				reach.Reason = discoverReasonFixtureBody
			}
			decl, _ := f.anchor(binding.line, binding.line, "static test discovery import of "+e.root)
			reach.Declaration = &decl
		default:
			reach.State, reach.Reason = "unresolved", discoverReasonFixtureUnknown
			if bound {
				reach.Specifier = binding.spec
			}
		}
		out.Reaches = append(out.Reaches, reach)
	}
}
