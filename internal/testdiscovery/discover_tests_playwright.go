package testdiscovery

import (
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

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
	r       *JSReader
	s       *Source
	hooks   []jsDiscoveryHook
	runtime []jsRuntime
}

// discoverJS reads one Playwright test file and returns "" or the file's uncertainty reason.
func (f *discoveryFile) discoverJS() string {
	s := ParseSource(f.path, f.text)
	if s.Uncertain {
		f.unknown("", ValiditySourceUnparsed, "the lexer could not place some source or a bracket does not balance, so no example is read", 0, 0)
		return ValiditySourceUnparsed
	}
	r := &JSReader{S: s, Name: f.path}
	r.Read()
	d := &jsDiscovery{f: f, r: r, s: s}
	d.readCalls()
	runtimeAt := map[int]bool{}
	for _, a := range d.runtime {
		runtimeAt[a.at] = true
	}
	perLine := map[int]int{}
	for _, e := range r.Decls {
		if e.Kind == "test" && !runtimeAt[e.At] {
			perLine[e.Line]++
		}
	}
	for _, e := range r.Decls {
		if e.Kind != "test" || runtimeAt[e.At] {
			continue
		}
		d.example(e, perLine[e.Line])
	}
	return ""
}

// tokenEnd is the last line token i covers.
func (d *jsDiscovery) tokenEnd(i int) int {
	t := d.s.Tokens[i]
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
	return d.s.Tokens[start].Line, d.tokenEnd(end - 1)
}

// readCalls places every hook call and every titleless runtime modifier call.
func (d *jsDiscovery) readCalls() {
	s := d.s
	for i := range s.Tokens {
		if !s.Ident(i, "") || s.Ident(i-1, "function") || !s.Punct(i+1, "(") || s.Match[i+1] < 0 {
			continue
		}
		name := s.Tokens[i].Text
		testMember := s.MemberAccess(i) && s.Ident(i-2, "") && testRoots[s.Tokens[i-2].Text] && !s.MemberAccess(i-2)
		args := s.SplitArgs(i+1, s.Match[i+1])
		if scope, ok := jsHookNames[name]; ok && (testMember || !s.MemberAccess(i)) {
			h := jsDiscoveryHook{at: i, line: s.Tokens[i].Line, endLine: s.Tokens[s.Match[i+1]].Line, kind: name, scope: scope}
			for _, a := range args {
				if body, _, ok := s.FunctionBody(a[0]); ok && body[1] <= a[1] {
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
		if len(args) > 1 && s.LoneString(args[0]) {
			for _, a := range args[1:] {
				if _, _, ok := s.FunctionBody(a[0]); ok || s.Callback(a) {
					declares = true
				}
			}
		}
		if declares {
			continue
		}
		a := jsRuntime{at: i - 2, line: s.Tokens[i-2].Line, endLine: s.Tokens[s.Match[i+1]].Line, kind: name}
		switch {
		case len(args) == 0:
		case len(args) == 1 && s.LoneString(args[0]):
			a.description = s.Tokens[args[0][0]].Text
		default:
			a.conditional = !(args[0][1]-args[0][0] == 1 && s.Ident(args[0][0], "true"))
			if len(args) > 1 && s.LoneString(args[1]) {
				a.description = s.Tokens[args[1][0]].Text
			}
		}
		d.runtime = append(d.runtime, a)
	}
}

// reaches reports that a statement at token i applies to example e under chain (its describes,
// innermost first): in e's body, or at file or describe level outside every example body.
func (d *jsDiscovery) reaches(i int, e JSDecl, chain []int) bool {
	if e.HasBody && Within(i, e.Body) {
		return true
	}
	if d.r.InExampleBody(i, -1) {
		return false
	}
	own := d.r.Ancestors(i)
	return len(own) == 0 || slices.Contains(chain, own[0])
}

// declArgs recomputes the declaring call's argument ranges of decl e, as jsReader.read walks it.
func (d *jsDiscovery) declArgs(e JSDecl) [][2]int {
	s := d.s
	j := e.At + 1
	for s.Punct(j, ".") && s.Ident(j+1, "") {
		member := s.Tokens[j+1].Text
		j += 2
		if jsUnsupportedMembers[member] && s.Punct(j, "(") && s.Match[j] > j && s.Punct(s.Match[j]+1, "(") {
			j = s.Match[j] + 1
		}
		if member == "each" {
			switch {
			case s.Punct(j, "(") && s.Match[j] > j:
				j = s.Match[j] + 1
			case j < len(s.Tokens) && s.Tokens[j].Kind == appmap.JSTemplate:
				j++
			}
		}
	}
	if !s.Punct(j, "(") || s.Match[j] < 0 {
		return nil
	}
	return s.SplitArgs(j, s.Match[j])
}

func (d *jsDiscovery) example(e JSDecl, sameLine int) {
	f, r, s := d.f, d.r, d.s
	out, ok := f.newExample(e.Line, max(e.Line, e.EndLine))
	if !ok {
		return
	}
	chain := r.Ancestors(e.At)
	out.Identity = ExampleIdentity{Kind: ExampleIdentityJS, File: f.path, Unknown: DiscoverReasonProjectNotStatic}
	reason := r.Abstention(e, sameLine)
	var titles []string
	if reason != testvalidity.HygieneReasonJSLineAmbiguous {
		titles = r.TitlePath(e)
	}
	if titles == nil {
		out.TitleUnknown = DiscoverReasonTitleNotLiteral
		if reason == testvalidity.HygieneReasonJSLineAmbiguous {
			out.TitleUnknown = reason
		}
		out.Identity.Unknown = out.TitleUnknown
		f.unknown(out.ID, out.TitleUnknown, "a title on the example's path is not one literal string, so neither the title path nor the js-title-path identity is fixed", e.Line, e.Line)
	} else {
		out.Identity.FullName = strings.Join(titles, " > ")
		decls := append([]int{}, chain...)
		slices.Reverse(decls)
		for k, title := range titles {
			line := e.Line
			if k < len(decls) {
				line = r.Decls[decls[k]].Line
			}
			a, _ := f.anchor(line, line, "static test discovery title")
			out.TitlePath = append(out.TitlePath, DiscoveredTitle{Title: title, Evidence: a})
		}
	}
	// Tags and annotations come from the declarations, outermost describe first.
	for k := len(chain) - 1; k >= 0; k-- {
		d.declFacts(&out, r.Decls[chain[k]])
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
		f.unknown(out.ID, reason, "the TSA-V0-073 reader abstains on the example, so its body facts are not read", e.Line, e.Line)
		f.capFacts(&out)
		f.out.Examples = append(f.out.Examples, out)
		return
	}
	hooks := []jsDiscoveryHook{}
	for _, h := range d.hooks {
		if r.InExampleBody(h.at, -1) || !d.reaches(h.at, e, chain) {
			continue
		}
		hooks = append(hooks, h)
		asserts := false
		if h.hasBody {
			for k := h.body[0]; k < h.body[1]; k++ {
				if s.JSAssertion(k) {
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
	if e.HasBody {
		d.expectations(&out, e.Body)
		d.navigations(&out, e.Body, "example")
	}
	d.reachesOf(&out, e, hooks)
	f.capFacts(&out)
	f.out.Examples = append(f.out.Examples, out)
}

// declFacts adds decl g's title tags, details tags and annotations, and its modifiers.
func (d *jsDiscovery) declFacts(out *DiscoveredExample, g JSDecl) {
	f, s := d.f, d.s
	// The declaration's head: from its call through the line its callback body opens on.
	head := g.Line
	if g.HasBody && g.Body[0] >= 0 && g.Body[0] < len(s.Tokens) {
		head = max(head, s.Tokens[g.Body[0]].Line)
	}
	anchor, _ := f.anchor(g.Line, head, "static test discovery declaration")
	if g.literalTitle {
		for _, m := range jsTitleTag.FindAllStringSubmatch(g.Title, -1) {
			out.Tags = append(out.Tags, DiscoveredTag{Name: m[1], Evidence: anchor})
		}
	}
	switch g.Root[0] {
	case 'x':
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: "skip", Evidence: anchor})
	case 'f':
		out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: "only", Evidence: anchor})
	}
	for _, m := range jsDeclAnnotations {
		if g.Members[m] {
			out.Annotations = append(out.Annotations, DiscoveredAnnotation{Type: m, Evidence: anchor})
		}
	}
	args := d.declArgs(g)
	if len(args) < 3 || !s.Punct(args[1][0], "{") || s.Match[args[1][0]] != args[1][1]-1 {
		return
	}
	open, close := args[1][0], args[1][1]-1
	for _, p := range s.SplitArgs(open, close) {
		if !s.Ident(p[0], "") || !s.Punct(p[0]+1, ":") {
			continue
		}
		key, value := s.Tokens[p[0]].Text, [2]int{p[0] + 2, p[1]}
		if value[0] >= value[1] {
			continue
		}
		at, end := d.rangeLines(p[0], p[1])
		where, _ := f.anchor(at, end, "static test discovery details "+key)
		switch key {
		case "tag":
			for _, v := range d.elements(value) {
				if s.LoneString(v) && s.Tokens[v[0]].Literal {
					out.Tags = append(out.Tags, DiscoveredTag{Name: s.Tokens[v[0]].Text, Evidence: where})
				} else {
					f.unknown(out.ID, DiscoverReasonMetadata, "tag value "+s.Render(v[0], v[1]), at, end)
				}
			}
		case "annotation":
			for _, v := range d.elements(value) {
				typ, desc, ok := d.annotationObject(v)
				if !ok {
					f.unknown(out.ID, DiscoverReasonMetadata, "annotation "+s.Render(v[0], v[1]), at, end)
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
	if s.Punct(v[0], "[") && s.Match[v[0]] == v[1]-1 {
		return s.SplitArgs(v[0], v[1]-1)
	}
	return [][2]int{v}
}

// annotationObject reads `{ type: 'x', description: 'y' }` with literal strings.
func (d *jsDiscovery) annotationObject(v [2]int) (string, string, bool) {
	s := d.s
	if !s.Punct(v[0], "{") || s.Match[v[0]] != v[1]-1 {
		return "", "", false
	}
	typ, desc := "", ""
	for _, p := range s.SplitArgs(v[0], v[1]-1) {
		if !s.Ident(p[0], "") || !s.Punct(p[0]+1, ":") || p[1]-p[0] != 3 || !s.LoneString([2]int{p[0] + 2, p[1]}) || !s.Tokens[p[0]+2].Literal {
			return "", "", false
		}
		switch s.Tokens[p[0]].Text {
		case "type":
			typ = s.Tokens[p[0]+2].Text
		case "description":
			desc = s.Tokens[p[0]+2].Text
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
		if s.AssertionCount(k) {
			continue
		}
		if c, ok := s.ReadExpect(k); ok {
			x := DiscoveredExpectation{Matcher: c.Matcher, Receiver: boundText(s.Render(c.Subject[0], c.Subject[1]))}
			for j := c.Subject[1] + 1; j+1 <= c.End; j++ {
				if s.Punct(j, ".") && s.Ident(j+1, "not") {
					x.Negated = true
				}
			}
			if x.Matcher == "" {
				x.Unknown = DiscoverReasonMatcherUnread
			}
			start, end := d.rangeLines(k, c.End+1)
			x.Evidence, _ = f.anchor(start, end, "static test discovery expectation")
			out.Expectations = append(out.Expectations, x)
			continue
		}
		if s.Ident(k, "assert") && !s.MemberAccess(k) {
			call, matcher := k+1, "assert"
			if s.Punct(k+1, ".") && s.Ident(k+2, "") {
				call, matcher = k+3, s.Tokens[k+2].Text
			}
			if s.Punct(call, "(") && s.Match[call] > call {
				x := DiscoveredExpectation{Matcher: matcher, Negated: strings.HasPrefix(matcher, "not") || strings.Contains(matcher, "Not")}
				if args := s.SplitArgs(call, s.Match[call]); len(args) > 0 {
					x.Receiver = boundText(s.Render(args[0][0], args[0][1]))
				}
				start, end := d.rangeLines(k, s.Match[call]+1)
				x.Evidence, _ = f.anchor(start, end, "static test discovery expectation")
				out.Expectations = append(out.Expectations, x)
				k = s.Match[call]
				continue
			}
		}
		if s.JSAssertion(k) {
			line := s.Tokens[k].Line
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
		if !s.Ident(k, "") || !s.MemberAccess(k) || !s.Punct(k+1, "(") || s.Match[k+1] < 0 {
			continue
		}
		name := s.Tokens[k].Text
		n := DiscoveredNavigation{Kind: name, Source: source}
		switch {
		case jsRequestMethods[name] && s.Ident(k-2, "request"):
			n.Kind = "request"
			if name != "fetch" {
				n.Method = strings.ToUpper(name)
			}
		case jsNavigationMembers[name]:
		default:
			continue
		}
		close := s.Match[k+1]
		args := s.SplitArgs(k+1, close)
		receiver := k - 2
		for receiver > 0 && s.Punct(receiver-1, ".") && s.Ident(receiver-2, "") {
			receiver -= 2
		}
		n.Expression = boundText(s.Render(receiver, close+1))
		switch {
		case len(args) == 0:
			n.Unknown = DiscoverReasonTargetNotLiteral
		case s.LoneString(args[0]) && s.Tokens[args[0][0]].Literal:
			n.Target = s.Tokens[args[0][0]].Text
		case args[0][1]-args[0][0] == 1 && s.Tokens[args[0][0]].Kind == appmap.JSString && strings.HasPrefix(s.Tokens[args[0][0]].Source, "/"):
			n.Target = s.Tokens[args[0][0]].Source // a regular expression pattern, as written
		default:
			n.Unknown = DiscoverReasonTargetNotLiteral
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
	if !s.Punct(v[0], "{") || s.Match[v[0]] != v[1]-1 {
		return ""
	}
	for _, p := range s.SplitArgs(v[0], v[1]-1) {
		if s.Ident(p[0], "method") && s.Punct(p[0]+1, ":") && p[1]-p[0] == 3 && s.Tokens[p[0]+2].Literal {
			return strings.ToUpper(s.Tokens[p[0]+2].Text)
		}
	}
	return ""
}

// reachesOf adds the modules, dynamic loads and fixtures example e and its hooks reach
// (DCX-V1-374).
func (d *jsDiscovery) reachesOf(out *DiscoveredExample, e JSDecl, hooks []jsDiscoveryHook) {
	f, r, s := d.f, d.r, d.s
	ranges := [][2]int{}
	if e.HasBody {
		ranges = append(ranges, e.Body)
	}
	for _, h := range hooks {
		if h.hasBody {
			ranges = append(ranges, h.body)
		}
	}
	used := map[string]int{} // name to the first token using it
	for _, rg := range ranges {
		for k := rg[0]; k < rg[1]; k++ {
			if !s.Ident(k, "") || s.MemberAccess(k) {
				continue
			}
			name := s.Tokens[k].Text
			if _, seen := used[name]; !seen {
				used[name] = k
			}
			if (name == "require" || name == "import") && s.Punct(k+1, "(") && s.Match[k+1] > k+1 {
				args := s.SplitArgs(k+1, s.Match[k+1])
				start, end := d.rangeLines(k, s.Match[k+1]+1)
				where, _ := f.anchor(start, end, "static test discovery dynamic load")
				reach := DiscoveredReach{Kind: "import", Specifier: boundText(s.Render(k+2, s.Match[k+1])), Evidence: where}
				if len(args) == 1 && s.LoneString(args[0]) && s.Tokens[args[0][0]].Literal {
					reach.Specifier = s.Tokens[args[0][0]].Text
					reach.Target, reach.State, reach.Reason = f.resolver.web(f.path, reach.Specifier)
				} else {
					reach.State, reach.Reason = "unresolved", DiscoverReasonComputedLookup
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
		b, ok := r.Bindings[name]
		if !ok || JSTestLibrary(b.Spec) || b.Spec == "@playwright/test" || testRoots[name] || describeRoots[name] {
			continue
		}
		line := s.Tokens[used[name]].Line
		where, _ := f.anchor(line, line, "static test discovery use of "+name)
		decl, _ := f.anchor(b.line, b.line, "static test discovery import of "+name)
		reach := DiscoveredReach{Kind: "import", Name: name, Specifier: b.Spec, Evidence: where, Declaration: &decl}
		reach.Target, reach.State, reach.Reason = f.resolver.web(f.path, b.Spec)
		out.Reaches = append(out.Reaches, reach)
	}
	d.fixtures(out, e)
}

// fixtures adds the fixtures the example's callback destructures.
func (d *jsDiscovery) fixtures(out *DiscoveredExample, e JSDecl) {
	f, r, s := d.f, d.r, d.s
	if !e.HasBody {
		return
	}
	var params [2]int
	found := false
	for _, a := range d.declArgs(e) {
		if body, _, ok := s.FunctionBody(a[0]); ok && body == e.Body {
			v := a[0]
			if s.Ident(v, "async") {
				v++
			}
			if s.Ident(v, "function") {
				v++
				if s.Ident(v, "") {
					v++
				}
			}
			if s.Punct(v, "(") && s.Match[v] > v {
				params, found = [2]int{v + 1, s.Match[v]}, true
			} else if s.Ident(v, "") {
				params, found = [2]int{v, v + 1}, true
			}
			break
		}
	}
	if !found || params[0] >= params[1] {
		return
	}
	first := s.SplitArgs(params[0]-1, params[1])
	if len(first) == 0 {
		return
	}
	p := first[0]
	line := s.Tokens[p[0]].Line
	where, _ := f.anchor(line, s.Tokens[p[1]-1].Line, "static test discovery fixture parameter")
	if !s.Punct(p[0], "{") || s.Match[p[0]] < 0 {
		if s.Ident(p[0], "") && p[1]-p[0] == 1 && s.Tokens[p[0]].Text != "done" {
			out.Reaches = append(out.Reaches, DiscoveredReach{Kind: "fixture", Name: s.Tokens[p[0]].Text, State: "unresolved", Reason: DiscoverReasonComputedLookup, Evidence: where})
		}
		return
	}
	binding, bound := r.Bindings[e.Root]
	for _, part := range s.SplitArgs(p[0], s.Match[p[0]]) {
		if !s.Ident(part[0], "") {
			continue
		}
		name := s.Tokens[part[0]].Text
		reach := DiscoveredReach{Kind: "fixture", Name: name, Evidence: where}
		switch {
		case playwrightFixtures[name]:
			reach.State, reach.Specifier = "builtin", "@playwright/test"
		case bound && IsRelativeSpec(binding.Spec):
			reach.Specifier = binding.Spec
			reach.Target, reach.State, reach.Reason = f.resolver.web(f.path, binding.Spec)
			if reach.State == "repository" {
				reach.Reason = DiscoverReasonFixtureBody
			}
			decl, _ := f.anchor(binding.line, binding.line, "static test discovery import of "+e.Root)
			reach.Declaration = &decl
		default:
			reach.State, reach.Reason = "unresolved", discoverReasonFixtureUnknown
			if bound {
				reach.Specifier = binding.Spec
			}
		}
		out.Reaches = append(out.Reaches, reach)
	}
}
