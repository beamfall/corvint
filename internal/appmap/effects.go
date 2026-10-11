package appmap

import (
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Step effects (ANL-V0-035, ANL-V0-036): each test-sequence edge is classified from the spec
// source between the two page-object constructions it joins, following page-object method calls.
// Only a step whose every call is provably navigation, a wait or a read is navigation-only.
const (
	EffectNavigation = "navigation-only"
	EffectState      = "state-effect"
	EffectUnknown    = "unknown"

	maxEffectAnchors = 64
)

var (
	effectNavigationCalls = wordSet("goto reload goBack goForward waitForURL waitForLoadState waitForSelector " +
		"waitForTimeout waitFor waitForResponse waitForRequest getByTestId getByRole getByLabel getByPlaceholder " +
		"getByAltText getByTitle getByText locator frameLocator first last nth filter and or textContent innerText " +
		"innerHTML inputValue getAttribute isVisible isHidden isEnabled isDisabled isChecked isEditable count all " +
		"allTextContents allInnerTexts boundingBox url title hover focus scrollIntoViewIfNeeded")
	effectStateCalls = wordSet("fill type pressSequentially press check uncheck setChecked selectOption " +
		"setInputFiles clear dragTo dragAndDrop submit setItem removeItem addCookies clearCookies fetch")
	effectHTTPVerbs = wordSet("get post put patch delete head fetch")
	effectClicks    = wordSet("click dblclick tap")
	// effectPureMembers admits only ROOT.member calls that cannot mutate application state; any
	// other member of these roots (Object.assign, Object.defineProperty, ...) is unknown.
	effectPureMembers = map[string]map[string]bool{
		"JSON":    wordSet("stringify parse"),
		"Math":    wordSet("abs ceil floor round trunc max min pow sqrt random sign"),
		"Number":  wordSet("isNaN isFinite isInteger parseFloat parseInt"),
		"String":  wordSet("fromCharCode raw"),
		"Object":  wordSet("keys values entries fromEntries is"),
		"Array":   wordSet("isArray from of"),
		"Promise": wordSet("all allSettled race any resolve reject"),
		"Date":    wordSet("now parse UTC"),
		"console": wordSet("log info warn error debug"),
	}
	effectPureBare    = wordSet("expect test it describe String Number Boolean parseInt parseFloat encodeURIComponent decodeURIComponent encodeURI Symbol")
	effectKeywords    = wordSet("if for while switch catch return typeof function await async super with void yield in of do else")
	effectPureClasses = wordSet("Date URL URLSearchParams RegExp Error Map Set Array Promise")
)

// assignmentAt reports whether the '=' at toks[q] assigns (plain or compound) rather than compares
// or starts an arrow, and returns the index of the assigned target's last token.
func assignmentAt(toks []token, q int) (int, bool) {
	if q <= 0 || !isPunct(toks[q], "=") {
		return 0, false
	}
	if q+1 < len(toks) && toks[q+1].glued && (toks[q+1].text == "=" || toks[q+1].text == ">") {
		return 0, false
	}
	if toks[q].glued && strings.Contains("=!<>", toks[q-1].text) {
		return 0, false
	}
	k := q - 1
	for k > 0 && toks[k].kind == tokPunct && toks[k+1].glued && strings.Contains("+-*/%&|^?", toks[k].text) {
		k--
	}
	return k, true
}

// memberTarget reports whether an assignment target ending at toks[k] is a member (`a.b`, `a[k]`).
func memberTarget(toks []token, k int) bool {
	return isPunct(toks[k], "]") || toks[k].kind == tokIdent && k > 0 && isPunct(toks[k-1], ".")
}

// targetRoot walks a member target ending at toks[k] back to its root identifier, or -1.
func targetRoot(toks []token, k int) int {
	for k >= 0 && toks[k].kind == tokIdent && k > 1 && isPunct(toks[k-1], ".") {
		k -= 2
	}
	if k < 0 || toks[k].kind != tokIdent || (k > 0 && isPunct(toks[k-1], ".")) {
		return -1
	}
	return k
}

func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(s) {
		out[w] = true
	}
	return out
}

// stepEffect is one classification with the call that decided it and the page-object method
// spans read to reach it.
type stepEffect struct {
	kind, ref, call string
	from            []Anchor
}

func effectRank(kind string) int {
	switch kind {
	case EffectState:
		return 2
	case EffectUnknown:
		return 1
	}
	return 0
}

// join keeps the stronger classification (state-effect over unknown over navigation-only), the
// first call that decided it, and every method span read.
func (e *stepEffect) join(o stepEffect) {
	if effectRank(o.kind) > effectRank(e.kind) {
		e.kind, e.ref, e.call = o.kind, o.ref, o.call
	}
	for _, a := range o.from {
		dup := false
		for _, b := range e.from {
			if a.Repo == b.Repo && a.Path == b.Path && a.Start == b.Start {
				dup = true
				break
			}
		}
		if !dup {
			e.from = append(e.from, a)
		}
	}
	if len(e.from) > maxEffectAnchors && e.kind != EffectState {
		e.kind, e.call = EffectUnknown, "effect-bound"
	}
}

func effectAt(kind, path string, line int, call string) stepEffect {
	if secretscreen.MatchString(call) {
		call = ""
	}
	return stepEffect{kind: kind, ref: refAt(path, line), call: call}
}

// effectReader classifies spans of test source, memoizing page-object methods across specs.
type effectReader struct {
	b      *builder
	tokens map[string][]token
	memo   map[string]*stepEffect
}

func newEffectReader(b *builder) *effectReader {
	return &effectReader{b: b, tokens: map[string][]token{}, memo: map[string]*stepEffect{}}
}

func (r *effectReader) toks(path string) []token {
	if t, ok := r.tokens[path]; ok {
		return t
	}
	t, _ := lexJS(string(r.b.sources[path]))
	r.tokens[path] = t
	return t
}

// whole anchors the complete test file: an effect also depends on the file's imports, instance
// bindings and other members, so any change to it leaves the effect not fresh.
func (r *effectReader) whole(path string) Anchor {
	a := r.b.files[path].Anchor
	data := r.b.sources[path]
	return spanOf(blobEntry{repo: a.Repo, path: path, oid: a.Blob}, data, 1, strings.Count(string(data), "\n")+1)
}

// instances binds each `NAME = new CLASS(` in a spec to the test file CLASS resolves to; a name
// bound to two files maps to "" (ambiguous).
func instances(toks []token, classes map[string]string) map[string]string {
	out := map[string]string{}
	for k := 0; k+3 < len(toks); k++ {
		n := k + 1
		if next(toks, n, ":") && n+2 < len(toks) && toks[n+1].kind == tokIdent {
			n += 2 // NAME: Type = new ...
		}
		if toks[k].kind != tokIdent || !next(toks, n, "=") || n+2 >= len(toks) || toks[n+1].text != "new" || toks[n+1].kind != tokIdent || toks[n+2].kind != tokIdent {
			continue
		}
		target := classes[toks[n+2].text]
		if cur, seen := out[toks[k].text]; seen && cur != target {
			target = ""
		}
		out[toks[k].text] = target
	}
	// Any other assignment to a bound name or one of its members, such as `home = other` or
	// `home.open = fn`, makes the binding ambiguous.
	for q := range toks {
		k, ok := assignmentAt(toks, q)
		if !ok {
			continue
		}
		root := targetRoot(toks, k)
		if root < 0 {
			continue
		}
		if _, bound := out[toks[root].text]; !bound {
			continue
		}
		construct := root == k && q+2 < len(toks) && toks[q+1].text == "new" && classes[toks[q+2].text] == out[toks[root].text]
		if !construct {
			out[toks[root].text] = ""
		}
	}
	return out
}

// span classifies every call between token indexes from and to (exclusive). self is the file whose
// class `this` names, inst the spec's page-object instances.
func (r *effectReader) span(path string, toks []token, from, to int, inst map[string]string, classes map[string]string, self string, depth int) stepEffect {
	out := stepEffect{kind: EffectNavigation}
	if len(toks) > 0 && toks[0].unsure {
		return effectAt(EffectUnknown, path, 1, "unbounded-source")
	}
	for i := from + 1; i < to && i < len(toks); i++ {
		t := toks[i]
		if t.kind == tokTemplate && t.subst && (t.unsure || strings.Contains(t.code, "(")) {
			out.join(effectAt(EffectUnknown, path, t.line, "template-substitution"))
			continue
		}
		if k, ok := assignmentAt(toks, i); ok && memberTarget(toks, k) {
			out.join(effectAt(EffectUnknown, path, t.line, "member-assignment"))
			continue
		}
		if t.kind == tokIdent && t.text == "delete" && !property(toks, i) {
			out.join(effectAt(EffectUnknown, path, t.line, "delete"))
			continue
		}
		// A computed or optional invocation (`a[k](...)`, `f?.(...)`, `g()(...)`) is not read.
		if isPunct(t, "(") && i > 0 && (isPunct(toks[i-1], "]") || isPunct(toks[i-1], ".") || isPunct(toks[i-1], ")")) {
			out.join(effectAt(EffectUnknown, path, t.line, "computed-call"))
			continue
		}
		if t.kind != tokIdent || !next(toks, i+1, "(") {
			continue
		}
		if i > 0 && toks[i-1].kind == tokIdent && toks[i-1].text == "new" {
			if !effectPureClasses[t.text] && classes[t.text] == "" {
				out.join(effectAt(EffectUnknown, path, t.line, t.text))
			}
			continue
		}
		out.join(r.call(path, toks, i, inst, self, depth))
	}
	return out
}

type chainSeg struct {
	name string
	call bool
	at   int
}

// chainOf reads the member chain ending at the call toks[i] back to its root identifier; a
// computed, optional or parenthesized receiver is not read.
func chainOf(toks []token, i int) ([]chainSeg, bool) {
	segs := []chainSeg{{toks[i].text, true, i}}
	for j := i - 1; next(toks, j, "."); {
		k, call := j-1, false
		if next(toks, k, ")") {
			k, call = openParen(toks, k)-1, true
		}
		if k < 0 || toks[k].kind != tokIdent {
			return nil, false
		}
		segs = append(segs, chainSeg{toks[k].text, call, k})
		j = k - 1
	}
	for a, z := 0, len(segs)-1; a < z; a, z = a+1, z-1 {
		segs[a], segs[z] = segs[z], segs[a]
	}
	return segs, true
}

// openParen returns the index of the '(' matching the ')' at toks[i], or -1.
func openParen(toks []token, i int) int {
	depth := 0
	for j := i; j >= 0; j-- {
		if toks[j].kind != tokPunct {
			continue
		}
		switch toks[j].text {
		case ")", "]", "}":
			depth++
		case "(", "[", "{":
			depth--
			if depth == 0 {
				if toks[j].text != "(" {
					return -1
				}
				return j
			}
		}
	}
	return -1
}

// linkTarget reports whether a click's receiver is a getByRole('link', ...) locator, possibly
// narrowed by first, last or nth.
func linkTarget(toks []token, segs []chainSeg) bool {
	k := len(segs) - 2
	for k > 0 && segs[k].call && (segs[k].name == "first" || segs[k].name == "last" || segs[k].name == "nth") {
		k--
	}
	if k < 1 || !segs[k].call || segs[k].name != "getByRole" {
		return false
	}
	at := segs[k].at
	return at+2 < len(toks) && literal(toks[at+2]) && toks[at+2].text == "link"
}

func (r *effectReader) call(path string, toks []token, i int, inst map[string]string, self string, depth int) stepEffect {
	m, line := toks[i].text, toks[i].line
	nav := stepEffect{kind: EffectNavigation}
	if i == 0 || !next(toks, i-1, ".") {
		switch {
		case effectKeywords[m] || effectPureBare[m]:
			return nav
		case m == "fetch":
			return effectAt(EffectState, path, line, m)
		}
		return effectAt(EffectUnknown, path, line, m)
	}
	segs, ok := chainOf(toks, i)
	if !ok {
		return effectAt(EffectUnknown, path, line, m)
	}
	root := segs[0]
	if len(segs) == 2 && !root.call {
		if root.name == "this" && self != "" {
			return r.method(self, m, depth+1, path, line)
		}
		if target, bound := inst[root.name]; bound {
			if target == "" {
				return effectAt(EffectUnknown, path, line, m)
			}
			return r.method(target, m, depth+1, path, line)
		}
	}
	for _, s := range segs[:len(segs)-1] {
		if !s.call && s.name == "request" && effectHTTPVerbs[m] {
			return effectAt(EffectState, path, line, m)
		}
	}
	switch {
	case root.name == "expect" && (m == "soft" || m == "poll" || (len(m) > 2 && strings.HasPrefix(m, "to") && m[2] >= 'A' && m[2] <= 'Z')):
		return nav
	case !root.call && len(segs) == 2 && effectPureMembers[root.name][m]:
		return nav
	case root.name == "test" && m == "step" && len(segs) == 2:
		return nav
	case effectStateCalls[m]:
		return effectAt(EffectState, path, line, m)
	case effectClicks[m]:
		if linkTarget(toks, segs) {
			return nav
		}
		return effectAt(EffectUnknown, path, line, m)
	case effectNavigationCalls[m]:
		return nav
	}
	return effectAt(EffectUnknown, path, line, m)
}

// method classifies the body of the one class member named name in the test file at path,
// following `this.m()` calls up to maxImportHops deep; a missing, duplicated or recursive member
// is unknown.
func (r *effectReader) method(path, name string, depth int, callPath string, callLine int) stepEffect {
	if depth > maxImportHops {
		return effectAt(EffectUnknown, callPath, callLine, name)
	}
	key := path + "#" + name
	if e, ok := r.memo[key]; ok {
		if e == nil {
			return effectAt(EffectUnknown, callPath, callLine, name)
		}
		return *e
	}
	var found *rawMethod
	count := 0
	facts := r.b.facts[path]
	for k := range facts.methods {
		if facts.methods[k].name == name {
			count++
			found = &facts.methods[k]
		}
	}
	if count != 1 || !found.member || r.b.files[path] == nil {
		return effectAt(EffectUnknown, callPath, callLine, name)
	}
	r.memo[key] = nil
	toks := r.toks(path)
	from, to := len(toks), len(toks)
	for k, t := range toks {
		if t.line > found.start && from == len(toks) {
			from = k - 1
		}
		if t.line > found.end {
			to = k
			break
		}
	}
	e := r.span(path, toks, from, to, nil, nil, path, depth)
	e.join(stepEffect{kind: EffectNavigation, from: []Anchor{r.whole(path)}})
	r.memo[key] = &e
	return e
}
