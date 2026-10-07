package appmap

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// rawState is one `.state(...)` declaration as written, before hierarchy resolution.
type rawState struct {
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
}

// parseRouter reads every `.state('name', {...})` and `.state({name: ...})` call of one
// ui-router-states/0 file (AMAP-V0-002). Only literal names, parents, URLs and data are read; a
// call it cannot read literally is reported, never guessed.
func parseRouter(e blobEntry, data []byte) ([]rawState, []Unknown) {
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
		arg := toks[j+2]
		switch {
		case literal(arg) && next(toks, j+3, ","):
			s.name = arg.text
			config, _ = parseValue(toks, j+4)
		case isPunct(arg, "{"):
			config, _ = parseValue(toks, j+2)
			if n, ok := config.get("name"); ok && n.kind == "string" {
				s.name = n.str
			}
		}
		if s.name == "" {
			unknowns = append(unknowns, Unknown{Kind: "state", Ref: fmt.Sprintf("%s:%d", e.path, toks[j].line), Reason: "non-literal-name", Path: e.path, Line: toks[j].line})
			continue
		}
		readConfig(&s, config)
		states = append(states, s)
	}
	return states, unknowns
}

func isPunct(t token, p string) bool { return t.kind == tokPunct && t.text == p }

func readConfig(s *rawState, config jsValue) {
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
			if v.kind != "string" {
				s.reason = "non-literal-value"
			}
			s.parent = v.str
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
	slashes    = regexp.MustCompile(`/{2,}`)
)

// normalizeTemplate turns a ui-router path pattern into the map's template form: `:id`,
// `{id}` and `{id:int}` become `{id}`, duplicate slashes collapse and a trailing slash drops.
func normalizeTemplate(pattern string) (template string, params []string) {
	t := braceParam.ReplaceAllString(pattern, "{$1}")
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
	return t, params
}

// templateKey erases parameter names, so templates that differ only in names compare equal.
func templateKey(template string) string { return keyParam.ReplaceAllString(template, "{}") }

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
		sc := Screen{ID: screenID(app, n), State: n, Status: r.status, Reason: r.reason, Parent: s.parent, Abstract: s.abstract,
			Params: []string{}, Query: []string{}, Permissions: []string{}, Flags: []string{}, Anchor: s.anchor,
			PageObjects: []string{}, Workflows: []string{}, Scenarios: []string{}, Specs: []Attribution{}, Flows: []string{},
			Steps: []string{}, Preconditions: []Requirement{}}
		if s.parent != "" {
			sc.Parent = screenID(app, s.parent)
		}
		if r.status == StatusResolved {
			sc.Template, sc.Params = normalizeTemplate(r.path)
			sc.Key = templateKey(sc.Template)
			sc.Query = uniqueSorted(r.query)
		} else {
			unknowns = append(unknowns, Unknown{Kind: "screen", Ref: sc.ID, Reason: r.reason, Path: s.anchor.Path, Line: s.anchor.Start})
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
	byKey      map[string][]string
	bySegments map[int][]routable
}

type routable struct {
	id       string
	segments []string
}

func newScreenIndex(hashPrefix string, screens []Screen) *screenIndex {
	x := &screenIndex{hashPrefix: hashPrefix, byKey: map[string][]string{}, bySegments: map[int][]routable{}}
	for _, s := range screens {
		if s.Status != StatusResolved || s.Abstract {
			continue
		}
		x.byKey[s.Key] = append(x.byKey[s.Key], s.ID)
		segs := splitSegments(s.Key)
		x.bySegments[len(segs)] = append(x.bySegments[len(segs)], routable{id: s.ID, segments: segs})
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

func (x *screenIndex) urlMatches(raw string) ([]string, string) {
	u := raw
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			u = rest[j:]
		} else {
			u = "/"
		}
	}
	if x.hashPrefix != "" {
		if i := strings.Index(u, x.hashPrefix); i >= 0 && strings.Trim(u[:i], "/") == "" {
			u = u[i+len(x.hashPrefix):]
		}
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	if !strings.HasPrefix(u, "/") {
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
