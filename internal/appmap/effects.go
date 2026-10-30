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
		"allTextContents allInnerTexts boundingBox url title")
	effectStateCalls = wordSet("fill type pressSequentially press check uncheck setChecked selectOption " +
		"setInputFiles clear dragTo dragAndDrop submit setItem removeItem addCookies clearCookies fetch")
	effectHTTPVerbs = wordSet("get post put patch delete head fetch")
	effectClicks    = wordSet("click dblclick tap")
	// effectCallbackMatchers are the expect matchers that call the function under test.
	effectCallbackMatchers = wordSet("toPass toThrow toThrowError")
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
	effectKeywords    = wordSet("if for while switch catch return typeof function await async super with void yield in of do else constructor")
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
	k, op := q-1, ""
	for k > 0 && toks[k].kind == tokPunct && toks[k+1].glued && strings.Contains("+-*/%&|^?<>=!", toks[k].text) {
		op = toks[k].text + op
		k--
	}
	switch op {
	case "", "+", "-", "*", "/", "%", "**", "&", "|", "^", "&&", "||", "??", "<<", ">>", ">>>":
		return k, true
	}
	return 0, false // a comparison: <=, >=, ==, !=, ===, !==
}

// memberTarget reports whether an assignment target ending at toks[k] is a member (`a.b`, `a[k]`).
func memberTarget(toks []token, k int) bool {
	return isPunct(toks[k], "]") || toks[k].kind == tokIdent && k > 0 && isPunct(toks[k-1], ".")
}

// targetNames returns the identifier tokens an assignment target ending at toks[k] may rebind:
// the root of a member target, or every identifier inside a destructuring pattern or computed key.
// ok is false when the target cannot be bounded.
func targetNames(toks []token, k int) ([]int, bool) {
	if isPunct(toks[k], "]") || isPunct(toks[k], "}") || isPunct(toks[k], ")") {
		o := openBracket(toks, k)
		if o < 0 {
			return nil, false
		}
		out := []int{}
		for j := o + 1; j < k; j++ {
			if toks[j].kind == tokIdent {
				out = append(out, j)
			}
		}
		if isPunct(toks[k], "]") && o > 0 && (toks[o-1].kind == tokIdent || isPunct(toks[o-1], "]") || isPunct(toks[o-1], ")")) {
			more, ok := targetNames(toks, o-1)
			return append(out, more...), ok
		}
		return out, true
	}
	if r := targetRoot(toks, k); r >= 0 {
		return []int{r}, true
	}
	return nil, false
}

// openBracket returns the index of the bracket of any kind that opens the one closing at toks[i],
// or -1.
func openBracket(toks []token, i int) int {
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
				return j
			}
		}
	}
	return -1
}

// substUnsafe reports whether any ${} substitution in a template's code may call, assign, update
// or delete: such a template is not read as a value.
func substUnsafe(code string) bool {
	for i := 0; i+1 < len(code); i++ {
		if code[i] != '$' || code[i+1] != '{' {
			continue
		}
		depth, j := 1, i+2
		for ; j < len(code) && depth > 0; j++ {
			switch code[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if depth != 0 {
			return true
		}
		body := code[i+2 : j-1]
		if strings.ContainsAny(body, "(=`") || strings.Contains(body, "++") || strings.Contains(body, "--") || strings.Contains(body, "delete") {
			return true
		}
		i = j - 1
	}
	return false
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
	pages  map[string]bool
	plain  map[string]string
}

func newEffectReader(b *builder) *effectReader {
	return &effectReader{b: b, tokens: map[string][]token{}, memo: map[string]*stepEffect{}, pages: map[string]bool{}, plain: map[string]string{}}
}

// opaque names why a page-object file's members cannot be read from the file alone: a class that
// extends another inherits members, constructors and accessors this reader does not follow, and
// an accessor runs code on a plain property read or write. It returns "" for a plain file.
func (r *effectReader) opaque(path string) string {
	if why, seen := r.plain[path]; seen {
		return why
	}
	why := ""
	toks := r.toks(path)
	for k, t := range toks {
		if t.kind != tokIdent || property(toks, k) {
			continue
		}
		if t.text == "extends" {
			why = "extends"
			break
		}
		if (t.text == "get" || t.text == "set") && k+2 < len(toks) && toks[k+1].kind == tokIdent && next(toks, k+2, "(") {
			why = "accessor"
			break
		}
	}
	r.plain[path] = why
	return why
}

// pageBound reports whether the file at path leaves `page` and `this.page` as the Playwright page
// it receives: no assignment rebinds `page`, and `this.page` is assigned only `page` itself.
func (r *effectReader) pageBound(path string, toks []token) bool {
	if ok, seen := r.pages[path]; seen {
		return ok
	}
	ok := true
	for q := range toks {
		k, assign := assignmentAt(toks, q)
		if !assign {
			continue
		}
		names, bounded := targetNames(toks, k)
		if !bounded {
			ok = false
			break
		}
		for _, n := range names {
			if toks[n].text == "page" && !property(toks, n) {
				ok = false
			}
		}
		thisPage := toks[k].text == "page" && k > 1 && isPunct(toks[k-1], ".") && toks[k-2].text == "this" && !(k > 2 && isPunct(toks[k-3], "."))
		if thisPage && !(q+1 < len(toks) && toks[q+1].text == "page" && toks[q+1].kind == tokIdent && (q+2 >= len(toks) || next(toks, q+2, ";", "}") || toks[q+2].line > toks[q+1].line)) {
			ok = false
		}
	}
	r.pages[path] = ok
	return ok
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
	// A destructuring or unbounded target makes every name it may rebind ambiguous.
	for q := range toks {
		k, ok := assignmentAt(toks, q)
		if !ok {
			continue
		}
		names, bounded := targetNames(toks, k)
		if !bounded {
			for name := range out {
				out[name] = ""
			}
			continue
		}
		for _, root := range names {
			if _, bound := out[toks[root].text]; !bound || property(toks, root) {
				continue
			}
			construct := root == k && q+2 < len(toks) && toks[q+1].text == "new" && classes[toks[q+2].text] == out[toks[root].text]
			if !construct {
				out[toks[root].text] = ""
			}
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
		if t.kind == tokTemplate && t.subst && (t.unsure || substUnsafe(t.code)) {
			out.join(effectAt(EffectUnknown, path, t.line, "template-substitution"))
			continue
		}
		// A tagged template (`` tag`x` ``) calls its tag.
		if t.kind == tokTemplate && i > 0 && (toks[i-1].kind == tokIdent && !effectKeywords[toks[i-1].text] && toks[i-1].text != "case" && toks[i-1].text != "throw" || isPunct(toks[i-1], ")") || isPunct(toks[i-1], "]")) {
			out.join(effectAt(EffectUnknown, path, t.line, "tagged-template"))
			continue
		}
		// An increment or decrement of a member (`a.b++`, `--a[k]`) mutates it.
		if (isPunct(t, "+") || isPunct(t, "-")) && i+1 < len(toks) && toks[i+1].glued && toks[i+1].text == t.text {
			if (i > 0 && memberTarget(toks, i-1)) || (i+3 < len(toks) && toks[i+2].kind == tokIdent && next(toks, i+3, ".", "[")) {
				out.join(effectAt(EffectUnknown, path, t.line, "member-update"))
			}
			i++
			continue
		}
		if k, ok := assignmentAt(toks, i); ok && memberTarget(toks, k) {
			// A page object may set its own fields (`this.page = page`); a field named like one of
			// its methods could replace that method, and `this.page` is checked by pageBound.
			if self != "" && r.opaque(self) == "" && k > 1 && isPunct(toks[k-1], ".") && toks[k-2].text == "this" && !(k > 2 && isPunct(toks[k-3], ".")) && !r.hasMember(self, toks[k].text) {
				continue
			}
			out.join(effectAt(EffectUnknown, path, t.line, "member-assignment"))
			continue
		}
		if t.kind == tokIdent && t.text == "delete" && !property(toks, i) {
			out.join(effectAt(EffectUnknown, path, t.line, "delete"))
			continue
		}
		// A computed, optional or non-null asserted invocation (`a[k](...)`, `f?.(...)`, `g()(...)`,
		// `f!(...)`) is not read.
		if isPunct(t, "(") && i > 1 && isPunct(toks[i-1], "!") && (toks[i-2].kind == tokIdent && !effectKeywords[toks[i-2].text] || isPunct(toks[i-2], ")") || isPunct(toks[i-2], "]")) {
			out.join(effectAt(EffectUnknown, path, t.line, "computed-call"))
			continue
		}
		if isPunct(t, "(") && i > 0 && (isPunct(toks[i-1], "]") || isPunct(toks[i-1], ".") || isPunct(toks[i-1], ")")) {
			out.join(effectAt(EffectUnknown, path, t.line, "computed-call"))
			continue
		}
		// A call with type arguments (`f<T>(...)`) is not read; `=> (` is an arrow body.
		if isPunct(t, "(") && i > 1 && isPunct(toks[i-1], ">") && !(toks[i-1].glued && isPunct(toks[i-2], "=")) {
			out.join(effectAt(EffectUnknown, path, t.line, "generic-call"))
			continue
		}
		if t.kind != tokIdent || !next(toks, i+1, "(") {
			continue
		}
		if i > 0 && toks[i-1].kind == tokIdent && toks[i-1].text == "new" {
			if target := classes[t.text]; target != "" {
				out.join(r.construct(target, depth+1, path, t.line))
			} else if !effectPureClasses[t.text] {
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
	return at+3 < len(toks) && literal(toks[at+2]) && toks[at+2].text == "link" && next(toks, at+3, ",", ")")
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
	// test.step, expect.poll and the toPass / toThrow matchers run a callback; it is read only when
	// it is written in place, so its body lies in the classified span.
	switch {
	case root.name == "test" && m == "step" && len(segs) == 2:
		if inlineCallback(toks, i+1, 1) {
			return nav
		}
		return effectAt(EffectUnknown, path, line, "callback")
	case root.name == "expect" && m == "poll":
		if inlineCallback(toks, i+1, 0) {
			return nav
		}
		return effectAt(EffectUnknown, path, line, "callback")
	case root.name == "expect" && effectCallbackMatchers[m]:
		if s := segs[len(segs)-2]; s.call && inlineCallback(toks, s.at+1, 0) {
			return nav
		}
		return effectAt(EffectUnknown, path, line, "callback")
	case root.name == "expect" && (m == "soft" || (len(m) > 2 && strings.HasPrefix(m, "to") && m[2] >= 'A' && m[2] <= 'Z')):
		return nav
	case !root.call && len(segs) == 2 && effectPureMembers[root.name][m]:
		return nav
	case effectStateCalls[m]:
		return effectAt(EffectState, path, line, m)
	}
	// Navigation, waits and reads count only on the Playwright page itself: `page` in a spec, or
	// `page` / `this.page` in a page object, in a file that never rebinds them.
	onPage := !root.call && (root.name == "page" || self != "" && root.name == "this" && len(segs) > 2 && segs[1].name == "page" && !segs[1].call)
	switch {
	case !onPage || !r.pageBound(path, toks):
	case effectClicks[m]:
		if linkTarget(toks, segs) {
			return nav
		}
	case effectNavigationCalls[m]:
		return nav
	}
	return effectAt(EffectUnknown, path, line, m)
}

// inlineCallback reports whether argument arg (from 0) of the call whose '(' is toks[open] is a
// function written in place: a `function` expression that is the whole argument, or an arrow with
// a parenthesized or single parameter, optionally async. An arrow's body runs to the argument's
// end, but a `function` expression can be an operand (`function() {} && reset`), so its extent
// must end the argument. A named callback's body lies outside the span, so it is never admitted.
func inlineCallback(toks []token, open, arg int) bool {
	k := open + 1
	for ; arg > 0; arg-- {
		k = skipValue(toks, k)
		if k >= len(toks) || !isPunct(toks[k], ",") {
			return false
		}
		k++
	}
	if k < len(toks) && toks[k].kind == tokIdent && toks[k].text == "async" {
		k++
	}
	switch {
	case k >= len(toks):
		return false
	case toks[k].kind == tokIdent && toks[k].text == "function":
		return functionExpressionArgument(toks, k)
	case isPunct(toks[k], "("):
		k = closeParen(toks, k) + 1
	case toks[k].kind == tokIdent:
		k++
	default:
		return false
	}
	return k+1 < len(toks) && isPunct(toks[k], "=") && toks[k+1].glued && isPunct(toks[k+1], ">")
}

// functionExpressionArgument reports whether the `function` expression at toks[k] (an optional
// name, a parameter list and a body) is the whole call argument: the next token ends it.
func functionExpressionArgument(toks []token, k int) bool {
	k++
	if k < len(toks) && toks[k].kind == tokIdent {
		k++
	}
	if k >= len(toks) || !isPunct(toks[k], "(") {
		return false
	}
	k = closeParen(toks, k) + 1
	if k >= len(toks) || !isPunct(toks[k], "{") {
		return false
	}
	k = closeParen(toks, k) + 1
	return k < len(toks) && (isPunct(toks[k], ",") || isPunct(toks[k], ")"))
}

// defaultParameter reports whether the one-line method header ending before toks[body] (on line
// start) gives a parameter a default or holds a template substitution: a default runs when the
// method is called, but the header is not part of the classified body.
func defaultParameter(toks []token, body, start int) bool {
	k := body
	for k > 0 && toks[k-1].line == start {
		k--
	}
	for ; k < body && !isPunct(toks[k], "("); k++ {
	}
	for ; k < body; k++ {
		if toks[k].subst || isPunct(toks[k], "=") && !(k+1 < len(toks) && toks[k+1].glued && isPunct(toks[k+1], ">")) {
			return true
		}
	}
	return false
}

// hasMember reports whether the test file at path declares a class member named name.
func (r *effectReader) hasMember(path, name string) bool {
	for _, m := range r.b.facts[path].methods {
		if m.name == name {
			return true
		}
	}
	return false
}

// construct classifies what constructing the page-object class in the test file at path runs:
// every token outside its recognized non-constructor methods (the constructor, field initializers
// and any module code), so an effect a construction may perform is never omitted.
func (r *effectReader) construct(path string, depth int, callPath string, callLine int) stepEffect {
	if depth > maxImportHops || r.b.files[path] == nil {
		return effectAt(EffectUnknown, callPath, callLine, "new")
	}
	if why := r.opaque(path); why != "" {
		return effectAt(EffectUnknown, callPath, callLine, why)
	}
	key := path + "#new"
	if e, ok := r.memo[key]; ok {
		if e == nil {
			return effectAt(EffectUnknown, callPath, callLine, "new")
		}
		return *e
	}
	r.memo[key] = nil
	methods := r.b.facts[path].methods
	inMethod := func(line int) bool {
		for _, m := range methods {
			if m.member && m.name != "constructor" && line >= m.start && line <= m.end {
				return true
			}
		}
		return false
	}
	toks := r.toks(path)
	e := stepEffect{kind: EffectNavigation, from: []Anchor{r.whole(path)}}
	for a := 0; a < len(toks); {
		if inMethod(toks[a].line) {
			a++
			continue
		}
		z := a
		for z < len(toks) && !inMethod(toks[z].line) {
			z++
		}
		e.join(r.span(path, toks, a-1, z, nil, nil, path, depth))
		a = z
	}
	if len(toks) > 0 && toks[0].unsure {
		e.join(effectAt(EffectUnknown, path, 1, "unbounded-source"))
	}
	r.memo[key] = &e
	return e
}

// method classifies the body of the one class member named name in the test file at path,
// following `this.m()` calls up to maxImportHops deep; a missing, duplicated or recursive member
// is unknown.
func (r *effectReader) method(path, name string, depth int, callPath string, callLine int) stepEffect {
	if depth > maxImportHops {
		return effectAt(EffectUnknown, callPath, callLine, name)
	}
	if why := r.opaque(path); why != "" {
		return effectAt(EffectUnknown, callPath, callLine, why)
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
	if defaultParameter(toks, from, found.start) {
		e := effectAt(EffectUnknown, path, found.start, "default-parameter")
		r.memo[key] = &e
		return e
	}
	e := r.span(path, toks, from, to, nil, nil, path, depth)
	e.join(stepEffect{kind: EffectNavigation, from: []Anchor{r.whole(path)}})
	r.memo[key] = &e
	return e
}
