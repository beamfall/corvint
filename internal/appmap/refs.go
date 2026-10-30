package appmap

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Reference check (AMAP-V0-090..099): the routes, selectors and mocked endpoint segments that
// tests name but the application source does not contain. It is name-level only.
const RefsSchema = "application-map-refs/0"

// RefsLimits is printed in every report (AMAP-V0-096).
const RefsLimits = "name-level only: a selector or endpoint name is present when it occurs as a name in the declared source, " +
	"a route when it matches a resolved screen template; rendering, reachability, attribute placement and runtime request matching are not checked"

const (
	refRoute    = "route"
	refSelector = "selector"
	refEndpoint = "endpoint"

	matchExact  = "exact"
	matchPrefix = "prefix"

	classCatchAll     = "catch-all-route"
	classRegex        = "regular-expression"
	classAppTemplated = "app-templated-name"

	maxRefSources   = 16
	maxRefValue     = 256
	refLocationsMax = 3
)

// refValue is one reference value read from test source: an exact name, a prefix, or the reason it
// is not checked.
type refValue struct {
	value, mode, class, reason string
}

type testRef struct {
	kind string
	v    refValue
	line int
}

// scanRefs reads the route, selector and endpoint references of one test source (AMAP-V0-091).
func scanRefs(text string) []testRef {
	toks, _ := lexJS(text)
	out := []testRef{}
	for i := 1; i+1 < len(toks); i++ {
		t := toks[i]
		if t.kind != tokIdent || !isPunct(toks[i+1], "(") || !isPunct(toks[i-1], ".") {
			continue
		}
		var arg refValue
		if i+2 < len(toks) {
			arg = refArg(toks, i+2)
		} else {
			arg = refValue{reason: "non-literal-value"}
		}
		add := func(kind string, v refValue) { out = append(out, testRef{kind: kind, v: v, line: t.line}) }
		switch {
		case t.text == "goto" || t.text == "waitForURL" || t.text == "toHaveURL":
			// A string's `*` is a URL glob; a regular expression's escaped `*` is a literal and keeps
			// its label (AMAP-V0-092/095).
			if g := strings.IndexByte(arg.value, '*'); arg.reason == "" && arg.class != classRegex && g >= 0 {
				arg = prefixValue(arg.value[:g]) // a URL glob is a prefix of its literal start
				if arg.reason != "" {
					arg.reason = "glob-without-literal"
				}
			}
			add(refRoute, arg)
		case t.text == "route":
			for _, v := range endpointSegments(arg) {
				add(refEndpoint, v)
			}
		case selectorKinds[t.text].kind != "":
			for _, v := range selectorNames(t.text, arg) {
				add(refSelector, v)
			}
		}
	}
	return out
}

// refArg classifies one argument (AMAP-V0-092): a whole literal is exact; a template with a
// substitution, or a literal joined by `+`, is a prefix up to the first dynamic part; a regular
// expression contributes its leading literal text as a labelled prefix; anything else is unchecked.
func refArg(toks []token, j int) refValue {
	t := toks[j]
	after := func(p string) bool { return j+1 < len(toks) && isPunct(toks[j+1], p) }
	var v refValue
	switch {
	case t.regex != "":
		if lit := regexLiteral(t.regex); lit != "" {
			v = refValue{value: lit, mode: matchPrefix, class: classRegex}
		} else {
			v = refValue{reason: "regex-without-literal"}
		}
	case t.inexact || (t.kind != tokString && t.kind != tokTemplate):
		v = refValue{reason: "non-literal-value"}
	case t.kind == tokTemplate && t.subst:
		text := t.text
		if strings.HasPrefix(text, substMark+"/") { // a leading substitution before `/` is an origin
			text = text[len(substMark):]
		}
		prefix, _, _ := strings.Cut(text, substMark)
		v = prefixValue(prefix)
	case after(")") || after(","):
		v = refValue{value: t.text, mode: matchExact}
	case after("+"):
		v = prefixValue(t.text)
	default:
		v = refValue{reason: "non-literal-value"}
	}
	switch {
	case v.reason != "":
	case secretscreen.MatchString(v.value):
		v = refValue{reason: "secret-shaped"}
	case len(v.value) > maxRefValue:
		v = refValue{reason: "value-too-long"}
	}
	return v
}

func prefixValue(p string) refValue {
	if p == "" {
		return refValue{reason: "dynamic-value"}
	}
	return refValue{value: p, mode: matchPrefix}
}

// regexLiteral is the literal text a regular-expression literal starts with: a leading `^` is
// dropped, `\/`-style escapes of punctuation read as the character, and the text stops at the first
// metacharacter, class escape or optional character.
func regexLiteral(src string) string {
	end := strings.LastIndexByte(src, '/')
	if end <= 0 {
		return ""
	}
	body := strings.TrimPrefix(src[1:end], "^")
	var b strings.Builder
	for k := 0; k < len(body); {
		c, n := body[k], 1
		if c == '\\' {
			if k+1 >= len(body) || !strings.ContainsRune(`/.-_:=#&?*+()[]{}|^$`, rune(body[k+1])) {
				break
			}
			c, n = body[k+1], 2
		} else if strings.IndexByte(`.*+?()[]{}|^$`, c) >= 0 {
			break
		}
		if k+n < len(body) && strings.IndexByte("?*{", body[k+n]) >= 0 {
			break // the character is optional or repeated zero times
		}
		b.WriteByte(c)
		if k+n < len(body) && body[k+n] == '+' {
			break
		}
		k += n
	}
	return b.String()
}

// selectorNames reads the checkable names of one selector call (AMAP-V0-091): a test id, and the
// id, class and `data-*` names of a CSS locator. Text-level kinds are counted, never checked.
func selectorNames(method string, v refValue) []refValue {
	if v.reason != "" {
		return []refValue{v}
	}
	switch method {
	case "getByTestId":
		return []refValue{v}
	case "locator":
	default:
		return []refValue{{reason: "text-level-selector"}}
	}
	s := v.value
	if v.mode == matchExact {
		if m := testIDAttribute.FindStringSubmatch(s); m != nil {
			return []refValue{{value: m[1], mode: matchExact}}
		}
	}
	switch {
	case strings.HasPrefix(s, "//") || strings.HasPrefix(s, "xpath="):
		return []refValue{{reason: "xpath-selector"}}
	case strings.HasPrefix(s, "text=") || strings.HasPrefix(s, "role=") || strings.HasPrefix(s, "internal:"):
		return []refValue{{reason: "text-level-selector"}}
	}
	names := cssNames(strings.TrimPrefix(s, "css="), v.mode == matchPrefix)
	for i := range names {
		names[i].class = v.class
	}
	if len(names) == 0 {
		return []refValue{{reason: "no-checkable-name"}}
	}
	return names
}

func nameByte(c byte) bool {
	return c == '_' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// cssNames extracts `#id`, `.class` and `[data-*]` / `[id]` names from a CSS selector. In a
// prefix the name that runs to the end of the text is itself a prefix.
func cssNames(s string, prefix bool) []refValue {
	out := []refValue{}
	add := func(name string, atEnd bool) {
		if name == "" {
			return
		}
		mode := matchExact
		if prefix && atEnd {
			mode = matchPrefix
		}
		out = append(out, refValue{value: name, mode: mode})
	}
	run := func(k int) int {
		for k < len(s) && nameByte(s[k]) {
			k++
		}
		return k
	}
	for k := 0; k < len(s); k++ {
		switch s[k] {
		case '#', '.':
			e := run(k + 1)
			if e > k+1 && (s[k+1] < '0' || s[k+1] > '9') { // `.1x` is not a CSS class or id
				add(s[k+1:e], e == len(s))
			}
			k = e - 1
		case ':':
			k = run(k+1) - 1 // a pseudo-class name is CSS, not the application's
		case '"', '\'':
			e := strings.IndexByte(s[k+1:], s[k])
			if e < 0 {
				return out
			}
			k += e + 1
		case '[':
			space := func(e int) int {
				for e < len(s) && strings.IndexByte(" \t\n\r\f", s[e]) >= 0 {
					e++
				}
				return e
			}
			a := space(k + 1)
			e := run(a)
			attr := s[a:e]
			e = space(e)
			op := ""
			for e < len(s) && strings.IndexByte("~|^$*=", s[e]) >= 0 {
				op += s[e : e+1]
				e++
			}
			e = space(e)
			value, atEnd := "", false
			if e < len(s) && (s[e] == '"' || s[e] == '\'') {
				q := strings.IndexByte(s[e+1:], s[e])
				if q < 0 {
					value, atEnd, e = s[e+1:], true, len(s)
				} else {
					value, e = s[e+1:e+1+q], e+q+2
				}
			} else {
				v := e
				for e < len(s) && s[e] != ']' {
					e++
				}
				value, atEnd = strings.TrimSpace(s[v:e]), e == len(s)
			}
			if strings.HasPrefix(attr, "data-") {
				add(attr, false)
			}
			if strings.HasPrefix(attr, "data-") || attr == "id" {
				switch op {
				case "=":
					add(value, atEnd)
				case "^=":
					if value != "" {
						out = append(out, refValue{value: value, mode: matchPrefix})
					}
				}
			}
			k = e
		}
	}
	return out
}

// endpointSegments splits a mocked endpoint (`route(` first argument) into its path segments
// (AMAP-V0-094): scheme, authority, query and fragment removed; a segment holding a glob character
// is a prefix of its literal start; empty, wildcard-only and digit-only segments are not checked.
func endpointSegments(v refValue) []refValue {
	if v.reason != "" {
		return []refValue{v}
	}
	s := v.value
	if loc := urlScheme.FindStringIndex(s); loc != nil {
		rest := s[loc[1]:]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			s = rest[j:]
		} else {
			s = ""
		}
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, "/")
	if len(parts) > 1 && !strings.HasPrefix(s, "*") && strings.ContainsAny(parts[0], ".:") {
		parts = parts[1:] // host[:port] written without a scheme
	}
	out := []refValue{}
	for i, p := range parts {
		cut := strings.IndexAny(p, "*?[{")
		lit, mode := p, matchExact
		if cut >= 0 {
			lit, mode = p[:cut], matchPrefix
		}
		if v.mode == matchPrefix && i == len(parts)-1 {
			mode = matchPrefix
		}
		if lit == "" || strings.Trim(lit, "0123456789") == "" {
			continue
		}
		out = append(out, refValue{value: lit, mode: mode, class: v.class})
	}
	if len(out) == 0 {
		return []refValue{{reason: "no-checkable-name"}}
	}
	return out
}

// nameIndex is the name-level view of the declared application source (AMAP-V0-094).
type nameIndex struct {
	names []string
	files [][]byte
	cache map[string]bool
}

func newNameIndex(files [][]byte) *nameIndex {
	set := map[string]bool{}
	for _, f := range files {
		for k := 0; k < len(f); {
			if !nameByte(f[k]) {
				k++
				continue
			}
			e := k
			for e < len(f) && nameByte(f[e]) {
				e++
			}
			set[string(f[k:e])] = true
			k = e
		}
	}
	x := &nameIndex{files: files, cache: map[string]bool{}}
	for n := range set {
		x.names = append(x.names, n)
	}
	sort.Strings(x.names)
	return x
}

// has reports whether name occurs as a whole name (exact) or starts a name (prefix). A name with
// other characters is looked up as a substring of the source.
func (x *nameIndex) has(name, mode string) bool {
	key := mode + "\x00" + name
	if v, ok := x.cache[key]; ok {
		return v
	}
	found := false
	plain := true
	for k := 0; k < len(name); k++ {
		plain = plain && nameByte(name[k])
	}
	switch {
	case !plain:
		for _, f := range x.files {
			if bytes.Contains(f, []byte(name)) {
				found = true
				break
			}
		}
	default:
		i := sort.SearchStrings(x.names, name)
		found = i < len(x.names) && (x.names[i] == name || mode == matchPrefix && strings.HasPrefix(x.names[i], name))
	}
	x.cache[key] = found
	return found
}

// templated reports whether a proper prefix of name ending in `-` or `_` is itself a source name,
// the shape an application builds a name from at run time (AMAP-V0-095).
func (x *nameIndex) templated(name string) bool {
	for k := len(name) - 2; k > 0; k-- {
		if (name[k] == '-' || name[k] == '_') && x.has(name[:k+1], matchExact) {
			return true
		}
	}
	return false
}

// routeCheck matches one route reference against the resolved screens (AMAP-V0-093). It returns
// whether the reference misses, its false-positive class, or why it is not checked.
func routeCheck(x *screenIndex, v refValue) (miss bool, class, reason string) {
	path, ok := x.urlPath(v.value)
	if !ok && v.mode == matchPrefix && path == "" {
		path, ok = "/", true // a prefix that ends inside the hash-routing prefix names the root
	}
	if !ok {
		return false, "", "relative-url"
	}
	path = slashes.ReplaceAllString(path, "/")
	type candidate struct {
		r     routable
		splat bool
	}
	routes := []candidate{}
	for _, r := range x.splats {
		routes = append(routes, candidate{r, true})
	}
	for _, rs := range x.bySegments {
		for _, r := range rs {
			routes = append(routes, candidate{r, false})
		}
	}
	var concrete []string
	partial := ""
	if v.mode == matchPrefix {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		concrete, partial = parts[:len(parts)-1], parts[len(parts)-1]
	} else {
		concrete = splitSegments(path)
	}
	matched, catchAll := false, false
	for _, c := range routes {
		r, splat := c.r, c.splat
		var hit bool
		switch {
		case v.mode == matchExact && !splat:
			hit = len(r.segments) == len(concrete) && segmentsMatch(r.segments, concrete)
		case v.mode == matchExact:
			hit = len(concrete) >= len(r.segments) && (!r.nonempty || len(concrete) > len(r.segments)) && segmentsMatch(r.segments, concrete[:len(r.segments)])
		default:
			hit = prefixMatch(r.segments, concrete, partial, splat)
		}
		if !hit {
			continue
		}
		if splat && len(r.segments) == 0 {
			catchAll = true
		} else {
			matched = true
		}
	}
	switch {
	case matched:
		return false, "", ""
	case catchAll:
		return true, classCatchAll, ""
	}
	return true, "", ""
}

// prefixMatch reports whether some URL starting with the concrete segments and then partial can
// reach the template: a splat template is reached once its literal segments are.
func prefixMatch(template, concrete []string, partial string, splat bool) bool {
	if splat && len(concrete) >= len(template) {
		return segmentsMatch(template, concrete[:len(template)])
	}
	if len(template) < len(concrete) || !segmentsMatch(template[:len(concrete)], concrete) {
		return false
	}
	if partial == "" || len(template) == len(concrete) {
		return partial == "" || splat
	}
	t := template[len(concrete)]
	if before, _, ok := strings.Cut(t, "{}"); ok {
		return strings.HasPrefix(partial, before) || strings.HasPrefix(before, partial)
	}
	return strings.HasPrefix(t, partial)
}

type refMiss struct {
	Kind        string   `json:"kind"`
	Value       string   `json:"value"`
	Match       string   `json:"match"`
	Class       string   `json:"class,omitempty"`
	Occurrences int      `json:"occurrences"`
	Tests       []string `json:"tests"`
}

type refUnchecked struct {
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
	Count   int    `json:"count"`
	Example string `json:"example"`
}

// RefsReport is the reference check's capped document and its count of unlabelled misses, which
// decides the exit status (AMAP-V0-098).
type RefsReport struct {
	Document   []byte
	Unlabelled int
}

// CheckRefs runs the reference check (AMAP-V0-090..099). It reads the map's test files and the
// declared source directories from --root at the map's revision, and from each aliased root bound
// with Options.Repos at the map's pin of it (SKH-V0-189); it writes nothing.
func CheckRefs(ctx context.Context, m *Map, sources []string, o Options) (*RefsReport, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	query := func(msg string) error { return &gokernel.Error{Code: "appmap-invalid-query", Message: msg} }
	if len(sources) == 0 || len(sources) > maxRefSources {
		return nil, query(fmt.Sprintf("refs requires 1 to %d --source directories", maxRefSources))
	}
	roots, err := openSources(ctx, m, sources, o)
	if err != nil {
		return nil, err
	}
	r, _, err := mapRoot(ctx, m, "", o)
	if err != nil {
		return nil, err
	}
	tests, unread, err := readMapTests(r, m.Revision, m, "")
	if err != nil {
		return nil, err
	}
	// A map test file of an aliased root is read at the map's pin of that root, so that root must
	// be bound with --repo or --workspace; an unbound one refuses the check instead of counting
	// its files unread and passing with no references (SKH-V0-189, AMAP-V0-350).
	testRepo := map[string]string{}
	for p := range tests {
		testRepo[p] = ""
	}
	if err := unboundTestRoot(m, o); err != nil {
		return nil, err
	}
	for _, pin := range m.Roots {
		if o.Repos[pin.Repo] == "" {
			continue
		}
		ar, arev, err := mapRoot(ctx, m, pin.Repo, o)
		if err != nil {
			return nil, err
		}
		more, n, err := readMapTests(ar, arev, m, pin.Repo)
		if err != nil {
			return nil, err
		}
		unread += n
		for p, b := range more {
			if prev, ok := testRepo[p]; ok {
				return nil, query(fmt.Sprintf("map test file %s is read from two roots (%s and %s)", p, rootName(prev), rootName(pin.Repo)))
			}
			testRepo[p], tests[p] = pin.Repo, b
		}
	}
	entries, err := sourceTree(roots, maxTestFiles)
	if err != nil {
		return nil, err
	}
	// Every map test path of a read root stays out of the corpus, read or not (AMAP-V0-097).
	testPaths := map[string]bool{}
	for _, f := range m.Files {
		testPaths[f.Anchor.Repo+"\x00"+f.Path] = true
	}
	corpus, skipped, corpusBytes := []blobEntry{}, 0, 0
	for _, e := range entries {
		switch {
		case testPaths[e.repo+"\x00"+e.path]:
		case e.size > maxTestFileBytes:
			skipped++
		default:
			corpus = append(corpus, e)
			corpusBytes += e.size
		}
	}
	// Each path is scanned, so the total counts every path even when blobs repeat (AMAP-V0-097).
	if corpusBytes > maxTestBytes {
		return nil, bound(fmt.Sprintf("declared source files exceed %d bytes", maxTestBytes))
	}
	data := map[string][]byte{}
	for _, sr := range roots {
		mine := []blobEntry{}
		for _, e := range corpus {
			if e.repo == sr.alias {
				mine = append(mine, e)
			}
		}
		got, err := sr.r.blobs(mine, maxTestFileBytes, maxTestBytes)
		if err != nil {
			return nil, err
		}
		for oid, b := range got {
			data[oid] = b
		}
	}
	files := [][]byte{}
	for _, e := range corpus {
		if b := data[e.oid]; bytes.IndexByte(b, 0) < 0 {
			files = append(files, b)
		} else {
			skipped++
		}
	}
	names := newNameIndex(files)
	idx := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
	type missKey struct{ kind, value, match, class string }
	type uncheckedKey struct{ kind, reason string }
	misses, unchecked := map[missKey]*refMiss{}, map[uncheckedKey]*refUnchecked{}
	references, checked := 0, 0
	paths := make([]string, 0, len(tests))
	for p := range tests {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		for _, ref := range scanRefs(string(tests[p])) {
			references++
			at := p + ":" + strconv.Itoa(ref.line)
			v, miss, class, reason := ref.v, false, "", ref.v.reason
			if reason == "" && ref.kind == refRoute {
				miss, class, reason = routeCheck(idx, v)
			} else if reason == "" {
				miss = !names.has(v.value, v.mode)
				if miss && v.mode == matchExact && names.templated(v.value) {
					class = classAppTemplated
				}
			}
			if reason != "" {
				k := uncheckedKey{ref.kind, reason}
				if unchecked[k] == nil {
					unchecked[k] = &refUnchecked{Kind: ref.kind, Reason: reason, Example: at}
				}
				unchecked[k].Count++
				continue
			}
			checked++
			if !miss {
				continue
			}
			if class == "" {
				class = v.class
			}
			k := missKey{ref.kind, v.value, v.mode, class}
			if misses[k] == nil {
				misses[k] = &refMiss{Kind: ref.kind, Value: v.value, Match: v.mode, Class: class, Tests: []string{}}
			}
			misses[k].Occurrences++
			if len(misses[k].Tests) < refLocationsMax {
				misses[k].Tests = append(misses[k].Tests, at)
			}
		}
	}
	missRows := make([]*refMiss, 0, len(misses))
	unlabelled, labelled := 0, 0
	for _, mm := range misses {
		missRows = append(missRows, mm)
		if mm.Class == "" {
			unlabelled++
		} else {
			labelled++
		}
	}
	sort.Slice(missRows, func(i, j int) bool {
		a, b := missRows[i], missRows[j]
		if (a.Class == "") != (b.Class == "") {
			return a.Class == ""
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Value != b.Value {
			return a.Value < b.Value
		}
		if a.Match != b.Match {
			return a.Match < b.Match
		}
		return a.Class < b.Class
	})
	uncheckedRows := make([]*refUnchecked, 0, len(unchecked))
	uncheckedTotal := 0
	for _, u := range unchecked {
		uncheckedRows = append(uncheckedRows, u)
		uncheckedTotal += u.Count
	}
	sort.Slice(uncheckedRows, func(i, j int) bool {
		a, b := uncheckedRows[i], uncheckedRows[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Reason < b.Reason
	})
	screensUnknown := 0
	for _, s := range m.Screens {
		if s.Status != StatusResolved {
			screensUnknown++
		}
	}
	head := []field{{"schema", RefsSchema}, {"app", m.App}, {"map_revision", m.Revision}, {"map_digest", m.Digest},
		{"budget", budget}, {"full", o.Full}, {"check", "name-level"}, {"limits", RefsLimits}, {"sources", sources},
		{"source_files", len(files)}, {"source_files_skipped", skipped}, {"test_files_read", len(tests)},
		{"test_files_unread", unread}, {"screens_unknown", screensUnknown}, {"references", references},
		{"checked", checked}, {"unchecked_total", uncheckedTotal}, {"misses_unlabelled", unlabelled}, {"misses_labelled", labelled}}
	missItems := make([]any, len(missRows))
	for i, mm := range missRows {
		missItems[i] = mm
	}
	uncheckedItems := make([]any, len(uncheckedRows))
	for i, u := range uncheckedRows {
		uncheckedItems[i] = u
	}
	doc, err := render(head, []section{{"misses", missItems}, {"unchecked", uncheckedItems}}, budget)
	if err != nil {
		return nil, err
	}
	return &RefsReport{Document: doc, Unlabelled: unlabelled}, nil
}

// unboundTestRoot refuses when a map test file lives in an aliased root that neither --repo nor
// --workspace binds, naming the first such alias, its file count and the flag (AMAP-V0-350).
func unboundTestRoot(m *Map, o Options) error {
	count := map[string]int{}
	for _, f := range m.Files {
		if f.Anchor.Repo != "" && o.Repos[f.Anchor.Repo] == "" {
			count[f.Anchor.Repo]++
		}
	}
	if len(count) == 0 {
		return nil
	}
	aliases := make([]string, 0, len(count))
	for a := range count {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	a := aliases[0]
	return rootUnavailable(a, fmt.Sprintf("holds %d map test files the check must read but is not bound; pass --repo %s=DIR or a --workspace that declares it", count[a], a))
}

// readTestFiles reads every map test file from --root at the map revision through one batch
// lookup; a file read from an aliased root, or one whose blob differs from the map's, is unread.
func readTestFiles(r repo, m *Map) (map[string][]byte, int, error) {
	out, unread, err := readMapTests(r, m.Revision, m, "")
	for _, f := range m.Files {
		if f.Anchor.Repo != "" {
			unread++
		}
	}
	return out, unread, err
}

// readMapTests reads the map test files of one root (alias, "" for --root) at rev; a file whose
// blob differs from the map's is unread. Files of other roots are not counted.
func readMapTests(r repo, rev string, m *Map, alias string) (map[string][]byte, int, error) {
	unread := 0
	want := []TestFile{}
	var in bytes.Buffer
	for _, f := range m.Files {
		if f.Anchor.Repo != alias {
			continue
		}
		if strings.ContainsAny(f.Path, "\n\r") {
			unread++
			continue
		}
		want = append(want, f)
		in.WriteString(rev + ":" + f.Path + "\n")
	}
	out := map[string][]byte{}
	if len(want) == 0 {
		return out, unread, nil
	}
	raw, err := r.git(len(want)*128+1024, in.Bytes(), "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	if err != nil {
		return nil, 0, err
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != len(want) {
		return nil, 0, &gokernel.Error{Code: "appmap-invalid-map", Message: "the map's test files are unreadable at its revision"}
	}
	entries := []blobEntry{}
	for i, f := range want {
		fields := strings.Fields(lines[i])
		size := 0
		if len(fields) == 3 {
			size, _ = strconv.Atoi(fields[2])
		}
		if len(fields) != 3 || fields[1] != "blob" || fields[0] != f.Anchor.Blob || size > maxTestFileBytes {
			unread++
			continue
		}
		entries = append(entries, blobEntry{path: f.Path, oid: fields[0], size: size})
	}
	data, err := r.blobs(entries, maxTestFileBytes, maxTestBytes)
	if err != nil {
		return nil, 0, err
	}
	for _, e := range entries {
		out[e.path] = data[e.oid]
	}
	return out, unread, nil
}
