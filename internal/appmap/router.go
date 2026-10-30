package appmap

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// rawState is one literal router declaration, before shared hierarchy resolution.
type rawState struct {
	entry              *EntryDeclaration
	splatNonEmpty      bool
	name               string
	parent             string
	parentSet          bool
	url                string
	urlSet             bool
	abstract           bool
	perms, flags       []string
	permsSet, flagsSet bool
	permsBad, flagsBad bool
	reason             string
	anchor             Anchor
	duplicate          bool
	// redirect marks a React route whose element is only a <Navigate> (AMAP-V0-194).
	redirect bool
	// nameFrom and parentFrom anchor a name or parent read through a constant table (AMAP-V0-016).
	nameFrom, parentFrom []Anchor
	// parentDetail names why an `X.Y` parent did not resolve (AMAP-V0-202).
	parentDetail string
}

// constLookup resolves a member expression `X.Y`, read at token index tok, to the string literal a
// constant table declares for it, with the declaration's anchor and, for an imported table, the
// import binding's (AMAP-V0-016), or the injection's (AMAP-V0-022); ok is false when it cannot,
// and detail then names an injected read's refusal, or is "" (AMAP-V0-202).
type constLookup func(ref string, tok int) (value string, at []Anchor, detail string, ok bool)

// literalName reads a name or parent: a string literal, or a member expression a constant table
// resolves. Anything else reads "", with the lookup's refusal detail when it names one.
func literalName(v jsValue, lookup constLookup) (string, []Anchor, string) {
	switch {
	case v.kind == "string":
		return v.str, nil, ""
	case v.kind == "member" && lookup != nil:
		s, at, detail, ok := lookup(v.str, v.tok)
		if ok {
			return s, at, ""
		}
		return "", nil, detail
	}
	return "", nil, ""
}

// parseRouter reads every `.state('name', {...})` and `.state({name: ...})` call of one
// ui-router-states/0 file (AMAP-V0-002). Only literal names, parents, URLs and data are read; a
// call it cannot read literally is reported, never guessed. A name or parent written `X.Y` is read
// through lookup (AMAP-V0-016); a nil lookup resolves none.
func parseRouter(e blobEntry, data []byte, lookup constLookup) ([]rawState, []Unknown) {
	toks, _ := lexJS(string(data))
	states, unknowns := []rawState{}, []Unknown{}
	for j := 1; j+2 < len(toks); j++ {
		if toks[j].kind != tokIdent || toks[j].text != "state" || !isPunct(toks[j-1], ".") || !isPunct(toks[j+1], "(") {
			continue
		}
		end := closeParen(toks, j+1)
		anchor := spanOf(e, data, toks[j].line, toks[end].line)
		s := rawState{anchor: anchor}
		var config jsValue
		detail := ""
		arg := toks[j+2]
		switch {
		case literal(arg) && next(toks, j+3, ","):
			s.name = arg.text
			config = wholeArg(toks, j+4, end)
		case isPunct(arg, "{"):
			config = wholeArg(toks, j+2, end)
			if n, ok := config.get("name"); ok {
				s.name, s.nameFrom, detail = literalName(n, lookup)
			}
		default:
			if v, after := parseValue(toks, j+2); v.kind == "member" && next(toks, after, ",") {
				s.name, s.nameFrom, detail = literalName(v, lookup)
				config = wholeArg(toks, after+1, end)
			}
		}
		if s.name == "" && detail == "" {
			detail = nameForm(toks, j+2, config)
		}
		if s.name == "" {
			unknowns = append(unknowns, Unknown{Kind: "state", Ref: fmt.Sprintf("%s:%d", e.path, toks[j].line), Reason: "non-literal-name", Path: e.path, Line: toks[j].line, Detail: detail})
			continue
		}
		readConfig(&s, config, lookup)
		s.entry = readEntryDeclaration(config)
		states = append(states, s)
	}
	return states, unknowns
}

// nameForm names, for a state unknown's detail, the form of a `.state()` name the reader did not
// read (AMAP-V0-205): `name-missing` (an object argument without `name`), or
// `name-expression:<form>` for the argument at toks[i] -- `identifier`, `call`, `template`,
// `optional-chain`, `member-<token>` (a member read followed by `as`, `!`, `+`, ... instead of the
// argument's end), `member-unread` (a member read no table resolved and no lookup named) or
// `expression`.
func nameForm(toks []token, i int, config jsValue) string {
	if i >= len(toks) {
		return "name-expression:expression"
	}
	t := toks[i]
	if isPunct(t, "{") {
		n, ok := config.get("name")
		if !ok {
			return "name-missing"
		}
		if n.kind == "member" {
			return "name-expression:member-unread"
		}
		if n.subst {
			return "name-expression:template"
		}
		return "name-expression:expression"
	}
	form := "expression"
	switch {
	case t.kind == tokTemplate:
		form = "template"
	case t.kind != tokIdent:
	case next(toks, i+1, "("):
		form = "call"
	case next(toks, i+1, "?") && next(toks, i+2, "."):
		form = "optional-chain"
	case next(toks, i+1, ".") && i+3 < len(toks) && toks[i+2].kind == tokIdent:
		switch a := toks[i+3]; {
		case isPunct(a, "."), isPunct(a, "["):
			form = "member-chain"
		case isPunct(a, ","):
			form = "member-unread"
		case a.kind == tokIdent || a.kind == tokPunct:
			form = "member-" + a.text
		}
	case next(toks, i+1, "!") && next(toks, i+2, "."):
		form = "non-null"
	case next(toks, i+1, ","), next(toks, i+1, ")"):
		form = "identifier"
	}
	return "name-expression:" + form
}

// wholeArg parses the last argument of the call closing at toks[end]. A literal that is only a
// prefix of the argument ({...} && config) is not what the program passes, so it reads "other".
func wholeArg(toks []token, i, end int) jsValue {
	if i >= len(toks) || i > end {
		return jsValue{kind: "other"} // a truncated call has no argument to read
	}
	v, after := parseValue(toks, i)
	if after == end || (after+1 == end && isPunct(toks[after], ",")) {
		return v
	}
	return jsValue{kind: "other", line: toks[i].line}
}

func isPunct(t token, p string) bool { return t.kind == tokPunct && t.text == p }

func readConfig(s *rawState, config jsValue, lookup constLookup) {
	if config.kind != "object" {
		s.reason = "non-literal-value"
		return
	}
	for _, p := range config.obj {
		v := p.value
		switch p.key {
		case "":
			// a spread or computed member could set any field
			s.reason = "non-literal-value"
		case "parent":
			s.parentSet = true
			s.parent, s.parentFrom, s.parentDetail = literalName(v, lookup)
			if v.kind != "string" && s.parentFrom == nil {
				s.reason = "non-literal-value"
			}
		case "url":
			s.urlSet = true
			if v.kind != "string" {
				s.reason = "non-literal-url"
			}
			s.url = v.str
		case "abstract":
			s.abstract = v.kind == "bool" && v.str == "true"
			if v.kind != "bool" {
				s.reason = "non-literal-value"
			}
		case "data":
			if v.kind != "object" {
				s.permsSet, s.permsBad, s.flagsSet, s.flagsBad = true, true, true, true
				continue
			}
			if pv, ok := v.get("permissions"); ok {
				s.permsSet = true
				s.perms, ok = pv.stringList()
				s.permsBad = !ok
			}
			if fv, ok := v.get("flags"); ok {
				s.flagsSet = true
				s.flags, ok = fv.stringList()
				s.flagsBad = !ok
			}
			for _, dp := range v.obj {
				if dp.key == "" {
					s.permsSet, s.permsBad, s.flagsSet, s.flagsBad = true, true, true, true
				}
			}
		}
	}
	if !s.parentSet {
		if i := strings.LastIndexByte(s.name, '.'); i > 0 {
			s.parent = s.name[:i]
		}
	}
}

var (
	braceParam = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(?::[^{}]*)?\}`)
	colonParam = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)
	keyParam   = regexp.MustCompile(`\{[A-Za-z_][A-Za-z0-9_]*\}`)
	splatParam = regexp.MustCompile(`\*([A-Za-z_][A-Za-z0-9_]*)?$`)
	splatKey   = regexp.MustCompile(`\{\*[A-Za-z_][A-Za-z0-9_]*\}`)
	slashes    = regexp.MustCompile(`/{2,}`)
)

// normalizeTemplate shares parameter and terminal-splat spelling across router dialects:
// `:id`, `{id}` and `{id:int}` become `{id}`; `*name` becomes `{*name}`. Duplicate
// slashes collapse and a trailing slash drops.
func normalizeTemplate(pattern string) (template string, params []string) {
	t := splatParam.ReplaceAllStringFunc(pattern, func(s string) string {
		if s == "*" {
			s = "*splat"
		}
		return "{" + s + "}"
	})
	t = braceParam.ReplaceAllString(t, "{$1}")
	t = colonParam.ReplaceAllString(t, "{$1}")
	if !strings.HasPrefix(t, "/") {
		t = "/" + t
	}
	t = slashes.ReplaceAllString(t, "/")
	if len(t) > 1 {
		t = strings.TrimSuffix(t, "/")
	}
	params = []string{}
	for _, m := range keyParam.FindAllString(t, -1) {
		params = append(params, m[1:len(m)-1])
	}
	for _, m := range splatKey.FindAllString(t, -1) {
		params = append(params, m[2:len(m)-1])
	}
	return t, params
}

// templateKey erases parameter names, so templates that differ only in names compare equal.
func templateKey(template string) string {
	return splatKey.ReplaceAllString(keyParam.ReplaceAllString(template, "{}"), "{*}")
}

// splitQuery separates a ui-router url's `?a&b` query declaration from its path.
func splitQuery(url string) (string, []string) {
	p, q, ok := strings.Cut(url, "?")
	if !ok {
		return p, nil
	}
	names := []string{}
	for _, n := range strings.Split(q, "&") {
		if n = strings.TrimSpace(n); n != "" {
			n, _, _ = strings.Cut(n, ":")
			names = append(names, strings.Trim(n, "{}"))
		}
	}
	return p, names
}

type resolved struct {
	path    string
	query   []string
	status  string
	reason  string
	visited bool
	done    bool
}

// resolveScreens resolves every declared state's hierarchy and template (AMAP-V0-002,
// AMAP-V0-003). A duplicate name, a missing or cyclic parent, or a non-literal value makes that
// state and all of its descendants UNKNOWN.
func resolveScreens(app string, raws []rawState) ([]Screen, []Unknown) {
	byName := map[string]*rawState{}
	for i := range raws {
		r := &raws[i]
		if prior, dup := byName[r.name]; dup {
			prior.duplicate = true
			continue
		}
		byName[r.name] = r
	}
	memo := map[string]*resolved{}
	var compute func(name string) *resolved
	compute = func(name string) *resolved {
		if m := memo[name]; m != nil {
			if m.visited && !m.done {
				return &resolved{status: StatusUnknown, reason: "parent-cycle"}
			}
			return m
		}
		m := &resolved{visited: true}
		memo[name] = m
		s := byName[name]
		ownPath, ownQuery := splitQuery(s.url)
		parentPath, parentQuery := "", []string(nil)
		switch {
		case s.duplicate:
			m.status, m.reason = StatusUnknown, "duplicate-state"
		case s.reason != "":
			m.status, m.reason = StatusUnknown, s.reason
		case s.parent != "" && byName[s.parent] == nil:
			m.status, m.reason = StatusUnknown, "missing-parent"
		case s.parent != "":
			p := compute(s.parent)
			if p.status != StatusResolved {
				m.status, m.reason = StatusUnknown, "unresolved-parent"
				if p.reason == "parent-cycle" {
					m.reason = "parent-cycle"
				}
			}
			parentPath, parentQuery = p.path, p.query
		}
		if m.status == "" {
			m.status = StatusResolved
			if strings.HasPrefix(ownPath, "^") {
				m.path = ownPath[1:]
			} else {
				m.path = parentPath + ownPath
			}
			m.query = append(append([]string{}, parentQuery...), ownQuery...)
		}
		m.done = true
		return m
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	screens, unknowns := []Screen{}, []Unknown{}
	index := map[string]int{}
	for _, n := range names {
		s, r := byName[n], compute(n)
		sc := Screen{Entry: s.entry, ID: screenID(app, n), State: n, Status: r.status, Reason: r.reason, Parent: s.parent, Abstract: s.abstract, SplatNonEmpty: s.splatNonEmpty,
			Params: []string{}, Query: []string{}, Permissions: []string{}, Flags: []string{}, Anchor: s.anchor,
			PageObjects: []string{}, Workflows: []string{}, Scenarios: []string{}, Specs: []Attribution{}, Flows: []string{},
			Steps: []string{}, Preconditions: []Requirement{}}
		if s.parent != "" {
			sc.Parent = screenID(app, s.parent)
		}
		sc.NameFrom, sc.ParentFrom = s.nameFrom, s.parentFrom
		if r.status == StatusResolved {
			sc.Template, sc.Params = normalizeTemplate(r.path)
			sc.Key = templateKey(sc.Template)
			sc.Query = uniqueSorted(r.query)
		} else {
			u := Unknown{Kind: "screen", Ref: sc.ID, Reason: r.reason, Path: s.anchor.Path, Line: s.anchor.Start}
			if r.reason == s.reason && r.reason == "non-literal-value" {
				u.Detail = s.parentDetail // an injected parent's refusal (AMAP-V0-202)
			}
			unknowns = append(unknowns, u)
		}
		index[n] = len(screens)
		screens = append(screens, sc)
	}
	// Permissions and flags inherit from the nearest ancestor that declares them.
	inherit := func(name string, set func(*rawState) (bool, bool, []string)) ([]string, string, bool) {
		seen := map[string]bool{}
		for cur := name; cur != "" && byName[cur] != nil && !seen[cur]; cur = byName[cur].parent {
			seen[cur] = true
			declared, bad, list := set(byName[cur])
			if declared {
				return uniqueSorted(list), cur, !bad
			}
		}
		return []string{}, "", true
	}
	for _, n := range names {
		sc := &screens[index[n]]
		if sc.Status != StatusResolved {
			continue
		}
		perms, from, ok := inherit(n, func(s *rawState) (bool, bool, []string) { return s.permsSet, s.permsBad, s.perms })
		sc.Permissions = perms
		if from != n && from != "" {
			sc.PermissionsFrom = screenID(app, from)
		}
		if !ok {
			unknowns = append(unknowns, Unknown{Kind: "screen-permissions", Ref: sc.ID, Reason: "non-literal-value", Path: byName[from].anchor.Path, Line: byName[from].anchor.Start})
		}
		flags, from, ok := inherit(n, func(s *rawState) (bool, bool, []string) { return s.flagsSet, s.flagsBad, s.flags })
		sc.Flags = flags
		if from != n && from != "" {
			sc.FlagsFrom = screenID(app, from)
		}
		if !ok {
			unknowns = append(unknowns, Unknown{Kind: "screen-flags", Ref: sc.ID, Reason: "non-literal-value", Path: byName[from].anchor.Path, Line: byName[from].anchor.Start})
		}
	}
	return screens, unknowns
}

func screenID(app, state string) string { return "screen:" + app + ":" + state }

func uniqueSorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	w := 0
	for i, s := range out {
		if i == 0 || s != out[w-1] {
			out[w] = s
			w++
		}
	}
	return out[:w]
}

// screenIndex answers template and URL lookups over the routable screens: resolved and not
// abstract. A lookup that matches no screen, or more than one, is UNKNOWN (AMAP-V0-003); the map
// never prefers one candidate over another.
type screenIndex struct {
	hashPrefix string
	basePath   string
	byKey      map[string][]string
	bySegments map[int][]routable
	splats     []routable
}

type routable struct {
	nonempty bool
	id       string
	segments []string
}

func newScreenIndex(hashPrefix, basePath string, screens []Screen) *screenIndex {
	x := &screenIndex{hashPrefix: hashPrefix, basePath: basePath, byKey: map[string][]string{}, bySegments: map[int][]routable{}}
	for _, s := range screens {
		if s.Status != StatusResolved || s.Abstract {
			continue
		}
		x.byKey[s.Key] = append(x.byKey[s.Key], s.ID)
		segs := splitSegments(s.Key)
		if len(segs) > 0 && segs[len(segs)-1] == "{*}" {
			x.splats = append(x.splats, routable{id: s.ID, segments: segs[:len(segs)-1], nonempty: s.SplatNonEmpty})
		} else {
			x.bySegments[len(segs)] = append(x.bySegments[len(segs)], routable{id: s.ID, segments: segs})
		}
	}
	return x
}

func splitSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return []string{}
	}
	return strings.Split(p, "/")
}

// ambiguousKeys lists every template key more than one routable screen shares.
func (x *screenIndex) ambiguousKeys() []string {
	out := []string{}
	for k, ids := range x.byKey {
		if len(ids) > 1 {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// byTemplate resolves a route template (as an AFU-V1 navigation step declares it) to one screen.
func (x *screenIndex) byTemplate(template string) (string, string) {
	return pick(x.templateMatches(template))
}

func (x *screenIndex) templateMatches(template string) ([]string, string) {
	path, _ := splitQuery(template)
	t, _ := normalizeTemplate(path)
	return x.byKey[templateKey(t)], ""
}

// pick returns the single match, or the reason none is chosen.
func pick(ids []string, reason string) (string, string) {
	switch {
	case reason != "":
		return "", reason
	case len(ids) == 0:
		return "", "no-matching-screen"
	case len(ids) == 1:
		return ids[0], ""
	}
	return "", "ambiguous-template"
}

// byURL resolves a literal URL from test source. substMark segments come from a template literal's
// whole-segment ${} substitution and match only parameter segments; a partial substitution
// cannot be placed and is UNKNOWN.
func (x *screenIndex) byURL(raw string) (string, string) {
	return pick(x.urlMatches(raw))
}

var urlScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// urlPath is the application path a test URL names: authority, router base path (AMAP-V0-191),
// hash-routing prefix, query and fragment removed. It is false when the result does not start
// with `/`, or when a base path is declared and the URL path lies outside it.
func (x *screenIndex) urlPath(raw string) (string, bool) {
	u, outside := x.basePathStripped(raw)
	if outside {
		return "", false
	}
	if x.hashPrefix != "" {
		if i := strings.Index(u, x.hashPrefix); i >= 0 && strings.Trim(u[:i], "/") == "" {
			u = u[i+len(x.hashPrefix):]
		}
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return u, strings.HasPrefix(u, "/")
}

// basePathStripped removes a leading scheme and authority and then the declared base path. A
// path-absolute URL that neither equals the base path nor continues it with `/`, `?` or `#` is
// outside the application's router and reads outside; a relative URL is returned unchanged.
func (x *screenIndex) basePathStripped(raw string) (string, bool) {
	u := raw
	if loc := urlScheme.FindStringIndex(u); loc != nil {
		// Only a leading scheme starts an authority; "://" inside a query or fragment is data.
		rest := u[loc[1]:]
		// The authority ends at the first '/', '?' or '#': https://host?next=/home is the root.
		switch j := strings.IndexAny(rest, "/?#"); {
		case j < 0:
			u = "/"
		case rest[j] == '/':
			u = rest[j:]
		default:
			u = "/" + rest[j:]
		}
	}
	if x.basePath == "" || !strings.HasPrefix(u, "/") {
		return u, false
	}
	rest, ok := strings.CutPrefix(u, x.basePath)
	switch {
	case !ok || rest != "" && !strings.ContainsRune("/?#", rune(rest[0])):
		return "", true
	case rest == "" || rest[0] != '/':
		rest = "/" + rest
	}
	return rest, false
}

func (x *screenIndex) urlMatches(raw string) ([]string, string) {
	if _, outside := x.basePathStripped(raw); outside {
		return nil, "outside-base-path"
	}
	u, ok := x.urlPath(raw)
	if !ok {
		return nil, "non-literal-url"
	}
	segs := splitSegments(slashes.ReplaceAllString(u, "/"))
	for _, s := range segs {
		if strings.Contains(s, substMark) && s != substMark {
			return nil, "partial-segment-substitution"
		}
	}
	matches := []string{}
	for _, r := range x.bySegments[len(segs)] {
		if segmentsMatch(r.segments, segs) {
			matches = append(matches, r.id)
		}
	}
	for _, r := range x.splats {
		if len(segs) >= len(r.segments) && (!r.nonempty || len(segs) > len(r.segments)) && segmentsMatch(r.segments, segs[:len(r.segments)]) {
			matches = append(matches, r.id)
		}
	}
	sort.Strings(matches)
	return matches, ""
}

func segmentsMatch(template, concrete []string) bool {
	for i, t := range template {
		c := concrete[i]
		switch {
		case c == substMark:
			if t != "{}" {
				return false
			}
		case t == "{}":
			if c == "" {
				return false
			}
		case strings.Contains(t, "{}"):
			if !wildcardMatch(t, c) {
				return false
			}
		case t != c:
			return false
		}
	}
	return true
}

// wildcardMatch matches a segment such as `{}.pdf` against a concrete segment.
func wildcardMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "{}")
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i, p := range parts[1:] {
		if i == len(parts)-2 {
			return len(s) > len(p) && strings.HasSuffix(s, p)
		}
		// Each placeholder consumes at least one byte; a middle part needs literal text to
		// delimit it, and an exhausted subject cannot match.
		if s == "" || p == "" {
			return false
		}
		k := strings.Index(s[1:], p)
		if k < 0 {
			return false
		}
		s = s[1+k+len(p):]
	}
	return true
}
