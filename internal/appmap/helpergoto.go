package appmap

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// maxInheritDepth bounds the `extends` hops a `this.m(...)` call walks up the test-root class
// chain (AMAP-V0-374).
const maxInheritDepth = 8

// maxHelperCalls bounds the plain calls one test file records for helper navigation (AMAP-V0-342).
const maxHelperCalls = 512

// rawCall is a plain call `name(...)`: its line and, per argument position, the exact literal
// passed there ("" when the argument is not one).
type rawCall struct {
	name string
	line int
	lits []string
}

// helperForm is how a navigation helper builds its URL from the parameter (AMAP-V0-367): a
// template literal's literal prefix (`/app#${p}`), or the hash of a URL object (`u.hash = p`).
type helperForm struct {
	prefix string
	hash   bool
}

// navHelpers returns the top-level functions of toks (`function F(...)`, `const F = (...) =>`)
// that pass one parameter unchanged into a `.goto(...)` or `.waitForURL(...)` URL, alone, after
// `X +` (a base prefix), as the one trailing substitution of a template literal, or as the hash
// of a `new URL(...)` object the call navigates to: the parameter's position by function name, -1
// for a name declared twice or navigating through two parameters (AMAP-V0-342, AMAP-V0-367). A
// parameter the body assigns is not passed unchanged. forms holds the template prefix or URL-hash
// form of a helper that uses one, and gaps, for a function that navigates only by forms the
// reader does not follow, `line:reason` of its first such call.
func navHelpers(toks []token) (out map[string]int, forms map[string]helperForm, gaps map[string]string) {
	out, forms, gaps = map[string]int{}, map[string]helperForm{}, map[string]string{}
	depth := 0
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == tokPunct {
			switch t.text {
			case "{", "(", "[":
				depth++
			case "}", ")", "]":
				depth--
			}
		}
		if depth != 0 || t.kind != tokIdent {
			continue
		}
		name, open := "", -1
		switch {
		case t.text == "function" && i+2 < len(toks) && toks[i+1].kind == tokIdent && isPunct(toks[i+2], "("):
			name, open = toks[i+1].text, i+2
		case (t.text == "const" || t.text == "let") && i+3 < len(toks) && toks[i+1].kind == tokIdent && isPunct(toks[i+2], "="):
			name, open = toks[i+1].text, i+3
			if word(toks, open, "async") {
				open++
			}
			if !next(toks, open, "(") {
				continue
			}
		default:
			continue
		}
		params, closing, why := helperParamList(toks, open)
		if why == "unread" {
			continue
		}
		k := functionBody(toks, closing+1)
		if k < 0 {
			continue
		}
		end := closeParen(toks, k)
		pos, form, gap := -1, helperForm{}, ""
		if why != "" {
			// A default the reader does not read may be any value: name it when the body
			// navigates (AMAP-V0-370).
			if navigates(toks, k, end) {
				gap = strconv.Itoa(t.line) + ":" + why
			}
		} else {
			pos, form, gap = helperParam(toks, k, end, params)
		}
		if _, seen := out[name]; seen {
			pos = -1 // declared twice
			delete(gaps, name)
		} else if pos < 0 && gap != "" {
			gaps[name] = gap
		}
		out[name] = pos
		if form != (helperForm{}) {
			forms[name] = form
		}
	}
	for name, pos := range out {
		if pos < 0 {
			delete(out, name)
			delete(forms, name)
		}
	}
	return out, forms, gaps
}

// helperParamList reads the parameter list opened at toks[open] as simpleParams does, also
// stepping over a default that is one literal (`timeout: number = 60_000`, `x = 'a'`,
// `flag = true`) (AMAP-V0-370). why is "default-parameter" for any other default, which the caller
// names as a gap, and "unread" for any other form.
func helperParamList(toks []token, open int) ([]param, int, string) {
	out := []param{}
	k := open + 1
	if next(toks, k, ")") {
		return out, k, ""
	}
	for k < len(toks) && toks[k].kind == tokIdent {
		out = append(out, param{toks[k].text, k})
		k++
		if next(toks, k, "?") {
			k++
		}
		if next(toks, k, ":") {
			if k = skipHelperType(toks, k+1); k < 0 {
				return nil, 0, "unread"
			}
		}
		if next(toks, k, "=") {
			v := k + 1
			if v >= len(toks) || !(literal(toks[v]) || toks[v].kind == tokNumber || literalWord[toks[v].text] && toks[v].kind == tokIdent) ||
				!next(toks, v+1, ",", ")") {
				if c := closeParen(toks, open); c < len(toks) && isPunct(toks[c], ")") {
					return nil, c, "default-parameter"
				}
				return nil, 0, "unread"
			}
			k = v + 1
		}
		if next(toks, k, ",") {
			k++
			if next(toks, k, ")") {
				return out, k, ""
			}
			continue
		}
		if next(toks, k, ")") {
			return out, k, ""
		}
		return nil, 0, "unread"
	}
	return nil, 0, "unread"
}

// literalWord are the keyword values a default parameter may be (see helperParamList).
var literalWord = map[string]bool{"true": true, "false": true, "null": true, "undefined": true}

// skipHelperType steps over a parameter's type as skipParamType does, also stopping at a `=`
// that starts a default.
func skipHelperType(toks []token, k int) int {
	if e := skipParamType(toks, k); e >= 0 {
		return e
	}
	depth, start := 0, k
	for ; k < len(toks); k++ {
		t := toks[k]
		switch {
		case t.kind == tokIdent || literal(t) || isPunct(t, ".") || isPunct(t, "|") || isPunct(t, "&"):
		case isPunct(t, "<") || isPunct(t, "["):
			depth++
		case isPunct(t, ">") || isPunct(t, "]"):
			if depth--; depth < 0 {
				return -1
			}
		case isPunct(t, ",") && depth > 0:
		case isPunct(t, "=") && depth == 0 && k > start && !next(toks, k+1, ">"):
			return k
		default:
			return -1
		}
	}
	return -1
}

// functionBody steps from toks[k], right after a parameter list, over a return type and an
// arrow to the body's `{`, or -1.
func functionBody(toks []token, k int) int {
	if next(toks, k, ":") { // a return type, `Promise<void>` included: step to the body
		for k++; k < len(toks) && !next(toks, k, "{") && !(next(toks, k, "=") && next(toks, k+1, ">")) && !next(toks, k, ";"); k++ {
		}
	}
	if next(toks, k, "=") && next(toks, k+1, ">") {
		k += 2
	}
	if !next(toks, k, "{") {
		return -1
	}
	return k
}

// navigates reports whether toks[from:to] holds a `.goto(` or `.waitForURL(` call.
func navigates(toks []token, from, to int) bool {
	for j := from; j < to && j+2 < len(toks); j++ {
		if isPunct(toks[j], ".") && (word(toks, j+1, "goto") || word(toks, j+1, "waitForURL")) && isPunct(toks[j+2], "(") {
			return true
		}
	}
	return false
}

// helperParam returns the position of the one parameter the body toks[from:to] passes unchanged
// into a navigation URL and the form it is passed in, or -1. When no call is followed, gap is
// `line:reason` for the first navigation call whose URL is not a literal: `template-form` (a
// template literal other than one trailing parameter substitution), `url-object` (a value other
// than a parameter whose hash the body sets from a parameter) or `expression` (AMAP-V0-367).
func helperParam(toks []token, from, to int, params []param) (int, helperForm, string) {
	index := map[string]int{}
	for k, p := range params {
		index[p.name] = k
	}
	pos, form, gap := -1, helperForm{}, ""
	for j := from; j < to && j+3 < len(toks); j++ {
		if !(isPunct(toks[j], ".") && toks[j+1].kind == tokIdent && (toks[j+1].text == "goto" || toks[j+1].text == "waitForURL") && isPunct(toks[j+2], "(")) {
			continue
		}
		a := j + 3
		k, f, why := navArg(toks, a, from, j, index)
		if k < 0 {
			if why != "" && gap == "" {
				gap = strconv.Itoa(toks[j+1].line) + ":" + why
			}
			continue
		}
		if pos >= 0 && (pos != k || form != f) {
			return -1, helperForm{}, ""
		}
		pos, form = k, f
	}
	if pos < 0 {
		return -1, helperForm{}, gap
	}
	for j := from; j < to; j++ { // `p = ...` or `p += ...` changes what the helper navigates to
		if toks[j].kind == tokIdent && toks[j].text == params[pos].name && !property(toks, j) && assigned(toks, j+1) {
			return -1, helperForm{}, ""
		}
	}
	return pos, form, ""
}

// navArg reads the URL argument at toks[a] of a navigation call at toks[call] in a body starting
// at toks[from]: the parameter it passes and its form, or -1 and why the form is not followed
// ("" for a literal URL, which names no parameter).
func navArg(toks []token, a, from, call int, index map[string]int) (int, helperForm, string) {
	whole := func(k int) bool { return next(toks, k, ")") || next(toks, k, ",") }
	if a >= len(toks) {
		return -1, helperForm{}, ""
	}
	if _, param := index[toks[a].text]; a+2 < len(toks) && !param && (toks[a].kind == tokIdent || literal(toks[a])) && isPunct(toks[a+1], "+") && !next(toks, a+2, "=") {
		a += 2 // a base prefix that is not itself a parameter
	}
	t := toks[a]
	if k, ok := index[t.text]; ok && t.kind == tokIdent && whole(a+1) {
		return k, helperForm{}, ""
	}
	switch {
	case literal(t) && whole(a+1):
		return -1, helperForm{}, ""
	case t.kind == tokTemplate && t.subst:
		prefix, rest, _ := strings.Cut(t.text, substMark)
		sub := strings.TrimSuffix(strings.TrimPrefix(t.code, "${"), "`")
		if k, ok := index[strings.TrimSuffix(sub, "}")]; ok && whole(a+1) && rest == "" && !t.inexact && !t.unsure && strings.HasSuffix(sub, "}") {
			return k, helperForm{prefix: prefix}, ""
		}
		return -1, helperForm{}, "template-form"
	case t.kind == tokIdent && !word(toks, a, "this"):
		e := a + 1
		if next(toks, e, ".") && (word(toks, e+1, "href") || word(toks, e+1, "toString") && next(toks, e+2, "(") && next(toks, e+3, ")")) {
			e += 2
			if word(toks, e-1, "toString") {
				e += 2
			}
		}
		if !whole(e) {
			return -1, helperForm{}, "expression"
		}
		if k := urlHash(toks, from, call, t.text, index); k >= 0 {
			return k, helperForm{hash: true}, ""
		}
		return -1, helperForm{}, "url-object"
	}
	return -1, helperForm{}, "expression"
}

// currentPage reports whether toks[open] opens the one argument `(X.url())`, `(location.href)` or
// `(window.location.href)`: the URL of the page the helper is on (AMAP-V0-367).
func currentPage(toks []token, open int) bool {
	if !next(toks, open, "(") {
		return false
	}
	var parts []string
	end := closeParen(toks, open)
	for k := open + 1; k < end && k < len(toks); k++ {
		parts = append(parts, toks[k].text)
	}
	got := strings.Join(parts, " ")
	return got == "location . href" || got == "window . location . href" ||
		len(parts) == 5 && toks[open+1].kind == tokIdent && strings.Join(parts[1:], " ") == ". url ( )"
}

// urlHash returns the position of the parameter p when toks[from:call] declares `const v = new
// URL(page.url())` (see currentPage), sets `v.hash = p` once and writes v nowhere else, so the call navigates to the
// current page with p as its hash (AMAP-V0-367); otherwise -1.
func urlHash(toks []token, from, call int, v string, index map[string]int) int {
	declared, pos := false, -1
	for j := from; j < call; j++ {
		if toks[j].kind != tokIdent || toks[j].text != v || property(toks, j) {
			continue
		}
		switch {
		case word(toks, j-1, "const") && next(toks, j+1, "=") && word(toks, j+2, "new") && word(toks, j+3, "URL") && currentPage(toks, j+4):
			if declared {
				return -1
			}
			declared = true
		case next(toks, j+1, ".") && word(toks, j+2, "hash") && assigned(toks, j+3):
			k, ok := index[toks[j+4].text]
			if !ok || toks[j+4].kind != tokIdent || !isPunct(toks[j+3], "=") || pos >= 0 ||
				!(next(toks, j+5, ";") || next(toks, j+5, "}") || j+5 < len(toks) && toks[j+5].line != toks[j+4].line) {
				return -1
			}
			pos = k
		default:
			return -1 // any other use may write the URL
		}
	}
	if !declared {
		return -1
	}
	return pos
}

// plainCalls records each plain call `name(...)` of toks with the exact literal at each argument
// position, up to maxHelperCalls.
func plainCalls(toks []token) []rawCall {
	out := []rawCall{}
	for i := 0; i+1 < len(toks) && len(out) < maxHelperCalls; i++ {
		t := toks[i]
		if t.kind != tokIdent || !isPunct(toks[i+1], "(") || (i > 0 && (isPunct(toks[i-1], ".") || word(toks, i-1, "function") || word(toks, i-1, "new"))) {
			continue
		}
		out = append(out, rawCall{name: t.text, line: t.line, lits: callLits(toks, i+1)})
	}
	return out
}

// thisCalls records each call `this.name(...)` of toks as plainCalls does (AMAP-V0-370).
func thisCalls(toks []token) []rawCall {
	out := []rawCall{}
	for i := 2; i+1 < len(toks) && len(out) < maxHelperCalls; i++ {
		if toks[i].kind == tokIdent && isPunct(toks[i+1], "(") && isPunct(toks[i-1], ".") && word(toks, i-2, "this") && !property(toks, i-2) {
			out = append(out, rawCall{name: toks[i].text, line: toks[i].line, lits: callLits(toks, i+1)})
		}
	}
	return out
}

// callLits returns, per argument of the call whose `(` is toks[open], the exact literal passed
// there: a string, or a template literal whose substitutions read as substMark (AMAP-V0-370); ""
// for any other argument.
func callLits(toks []token, open int) []string {
	var lits []string
	end := closeParen(toks, open)
	start, depth := open+1, 0
	for j := open + 1; j <= end && j < len(toks); j++ {
		tk := toks[j]
		switch {
		case tk.kind == tokPunct && (tk.text == "(" || tk.text == "[" || tk.text == "{"):
			depth++
		case tk.kind == tokPunct && (tk.text == ")" || tk.text == "]" || tk.text == "}") && j != end:
			depth--
		case depth == 0 && (j == end || isPunct(tk, ",")):
			lit := ""
			if a := toks[start]; j == start+1 && !a.inexact && (literal(a) || a.kind == tokTemplate && !a.unsure) {
				lit = a.text
			}
			if j > start {
				lits = append(lits, lit)
			}
			start = j + 1
		}
	}
	return lits
}

// navMethod is a method of a page object's class that navigates through one of its parameters
// (AMAP-V0-370): by itself (callee "", with form), or by passing it unchanged to the plain call
// callee at position at.
type navMethod struct {
	param  int
	callee string
	at     int
	form   helperForm
}

// classNav reads the first class of toks: the name its `extends` clause names ("" for none or a
// member chain), for each method how it may navigate through a parameter (see navMethod), the
// names of every method it declares, and its body's tokens. A method declared twice is not read.
func classNav(toks []token) (base string, methods map[string][]navMethod, declared map[string]bool, body []token) {
	methods, declared = map[string][]navMethod{}, map[string]bool{}
	open := -1
	for i := 0; i+1 < len(toks); i++ {
		if !word(toks, i, "class") || property(toks, i) {
			continue
		}
		for j := i + 1; j < len(toks); j++ {
			if next(toks, j, "{") {
				open = j
				break
			}
			if word(toks, j, "extends") && j+1 < len(toks) && toks[j+1].kind == tokIdent && !next(toks, j+2, ".") {
				base = toks[j+1].text
			}
		}
		break
	}
	if open < 0 {
		return "", methods, declared, nil
	}
	close := closeParen(toks, open)
	seen := map[string]bool{}
	for k := open + 1; k < close && k+1 < len(toks); k++ {
		if isPunct(toks[k], "{") || isPunct(toks[k], "(") || isPunct(toks[k], "[") {
			k = closeParen(toks, k)
			continue
		}
		if toks[k].kind != tokIdent || !isPunct(toks[k+1], "(") || word(toks, k, "constructor") {
			continue
		}
		name := toks[k].text
		declared[name] = true // an override hides the base method even when it is not read
		// A second declaration clears the first before anything of it is read, so a form the
		// reader stops at cannot leave the earlier reading standing.
		dup := seen[name]
		seen[name] = true
		if dup {
			delete(methods, name)
		}
		params, closing, why := helperParamList(toks, k+1)
		body := -1
		if why == "" {
			body = functionBody(toks, closing+1)
		}
		if body < 0 {
			if c := closeParen(toks, k+1); c > k {
				k = c
			}
			continue
		}
		end := closeParen(toks, body)
		if m := methodNav(toks, body, end, params); !dup && len(m) > 0 {
			methods[name] = m
		}
		k = end
	}
	return base, methods, declared, toks[open:min(close+1, len(toks))]
}

// methodNav reads a method body toks[from:to]: its own navigation (helperParam), or each plain
// call that receives a parameter, alone, as an argument; the caller keeps the one whose callee
// is a navigation helper. A body that assigns a parameter it passes on forwards nothing.
func methodNav(toks []token, from, to int, params []param) []navMethod {
	if pos, form, _ := helperParam(toks, from, to, params); pos >= 0 {
		return []navMethod{{param: pos, form: form}}
	}
	index := map[string]int{}
	for k, p := range params {
		index[p.name] = k
	}
	var out []navMethod
	for j := from + 1; j+1 < to; j++ {
		t := toks[j]
		if _, isParam := index[t.text]; isParam && t.kind == tokIdent && !property(toks, j) && assigned(toks, j+1) {
			return nil
		}
		if t.kind != tokIdent || !isPunct(toks[j+1], "(") || isPunct(toks[j-1], ".") || word(toks, j-1, "function") || word(toks, j-1, "new") {
			continue
		}
		end := closeParen(toks, j+1)
		at, start, depth := 0, j+2, 0
		for a := j + 2; a <= end && a < len(toks); a++ {
			tk := toks[a]
			switch {
			case tk.kind == tokPunct && (tk.text == "(" || tk.text == "[" || tk.text == "{"):
				depth++
			case tk.kind == tokPunct && (tk.text == ")" || tk.text == "]" || tk.text == "}") && a != end:
				depth--
			case depth == 0 && (a == end || isPunct(tk, ",")):
				if p, ok := index[toks[start].text]; ok && a == start+1 && toks[start].kind == tokIdent {
					out = append(out, navMethod{param: p, callee: t.text, at: at})
				}
				at, start = at+1, a+1
			}
		}
	}
	return out
}

// helperGotos returns, for a page object that navigates nowhere itself, one goto per plain call
// that passes a literal URL to a navigation helper of its own file or of a test file it imports by
// name: learned from one hop, offered as a remedy candidate and never a binding (AMAP-V0-342).
// A template helper's URL is its prefix and the literal, read from the hash on when the prefix
// holds one and the router is hash-based; a URL-hash helper's is the literal as the hash, placed
// only under a hash-based router (AMAP-V0-367), under the declared base path (AMAP-V0-370). A
// call `this.m(...)` of a method of the page object's class, or of the class its `extends` clause
// imports by name from a test file, follows one hop more when m navigates itself or passes its
// parameter on to one navigation helper of its file (AMAP-V0-370). A template literal argument
// places a screen when each substitution is a whole segment that a parameter segment matches.
// gaps lists, sorted, each call of a function whose navigation the reader does not follow as
// `helper-unfollowed:<file>:<line>:<name>:<reason>`.
func (b *builder) helperGotos(tf *TestFile) (out []Goto, gaps []string) {
	f := b.facts[tf.Path]
	helpers := b.navHelpersOf(tf.Path, f)
	out = []Goto{}
	seenGap := map[string]bool{}
	gap := func(h navHelper, name string) {
		line, why, _ := strings.Cut(h.gap, ":")
		if g := "helper-unfollowed:" + h.path + ":" + line + ":" + name + ":" + why; !seenGap[g] {
			seenGap[g] = true
			gaps = append(gaps, g)
		}
	}
	offer := func(c rawCall, h navHelper, pos int, name, at string) {
		if pos >= len(c.lits) || c.lits[pos] == "" {
			return
		}
		if g, ok := b.helperGoto(c.line, c.lits[pos], h.form, "helper-candidate:"+at+":"+name+":"+strconv.Itoa(pos)); ok {
			out = append(out, g)
		}
	}
	for _, c := range f.calls {
		h, ok := helpers[c.name]
		switch {
		case !ok:
		case h.gap != "":
			gap(h, c.name)
		default:
			offer(c, h, h.pos, c.name, h.path)
		}
	}
	for _, c := range f.thisCalls {
		path, ms := tf.Path, f.navMethods[c.name]
		if !f.navDeclared[c.name] {
			var lost bool
			if path, ms, lost = b.inheritedMethods(tf.Path, f, c.name); lost && slices.ContainsFunc(c.lits, func(l string) bool { return l != "" }) {
				// A call passing a literal into a chain the reader cannot finish is a named gap,
				// never `no-navigation` (AMAP-V0-374).
				if g := "helper-unfollowed:" + tf.Path + ":" + strconv.Itoa(c.line) + ":this." + c.name + ":inherited-depth"; !seenGap[g] {
					seenGap[g] = true
					gaps = append(gaps, g)
				}
				continue
			}
		}
		var hit []navHelper
		var via []navMethod
		var gapped []navHelper
		for _, m := range ms {
			if m.callee == "" {
				hit, via = append(hit, navHelper{path: path, pos: m.at, form: m.form}), append(via, m)
				continue
			}
			h, ok := b.navHelpersOf(path, b.facts[path])[m.callee]
			switch {
			case !ok:
			case h.gap != "":
				gapped = append(gapped, h)
			case h.pos == m.at:
				hit, via = append(hit, h), append(via, m)
			}
		}
		switch {
		case len(hit) == 1:
			offer(c, hit[0], via[0].param, "this."+c.name, hit[0].path)
		case len(hit) == 0 && len(gapped) == 1:
			gap(gapped[0], "this."+c.name)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	sort.Strings(gaps)
	return out, gaps
}

// navHelper is a navigation helper visible in one test file (AMAP-V0-342): the file declaring it,
// its URL parameter's position and form, or the gap naming why it is not followed.
type navHelper struct {
	path string
	pos  int
	form helperForm
	gap  string
}

// navHelpersOf returns the navigation helpers of the test file p: its own and those it imports by
// name from another test file.
func (b *builder) navHelpersOf(p string, f fileFacts) map[string]navHelper {
	helpers := map[string]navHelper{}
	add := func(local, path, name string, facts fileFacts) {
		if pos, ok := facts.helpers[name]; ok {
			helpers[local] = navHelper{path, pos, facts.helperForms[name], ""}
		} else if gap, ok := facts.helperGaps[name]; ok {
			helpers[local] = navHelper{path, -1, helperForm{}, gap}
		}
	}
	for name := range f.helpers {
		add(name, p, name, f)
	}
	for name := range f.helperGaps {
		add(name, p, name, f)
	}
	tf := b.files[p]
	if tf == nil {
		return helpers
	}
	for _, bd := range b.binds[p] {
		for _, imp := range tf.Imports {
			t := imp.testTarget()
			if imp.Specifier != bd.Module || t == "" || b.files[t] == nil {
				continue
			}
			add(bd.Local, t, bd.Exported, b.facts[t])
		}
	}
	return helpers
}

// baseClass returns the test file declaring the class that the class of the test file p (facts
// f) extends, when p imports it by name from a test file whose class is that name, or by its
// default clause from a test file whose class is its default export (AMAP-V0-394); "" with ok for
// a class without `extends`, and ok false for an `extends` the reader cannot resolve (AMAP-V0-370).
func (b *builder) baseClass(p string, f fileFacts) (string, bool) {
	tf := b.files[p]
	if f.navBase == "" {
		return "", true
	}
	if tf == nil {
		return "", false
	}
	for _, bd := range b.binds[p] {
		if bd.Local != f.navBase {
			continue
		}
		for _, imp := range tf.Imports {
			t := imp.testTarget()
			if imp.Specifier != bd.Module || t == "" || b.files[t] == nil {
				continue
			}
			if base := b.facts[t]; base.class == bd.Exported || bd.Exported == "default" && base.classDefault {
				return t, true
			}
		}
	}
	if module, ok := f.defaultImports[f.navBase]; ok {
		for _, imp := range tf.Imports {
			if t := imp.testTarget(); imp.Specifier == module && t != "" && b.files[t] != nil && b.facts[t].classDefault {
				return t, true
			}
		}
	}
	return "", false
}

// inheritedMethods walks the test-root `extends` chain above the class of the test file p, at
// most maxInheritDepth hops, to the nearest class that declares method name, and returns its
// file and readings (AMAP-V0-374). lost is true when the walk cannot finish: an `extends` the
// reader cannot resolve, a cycle, or more than maxInheritDepth hops. A chain that ends without
// declaring name returns nothing and is not lost.
func (b *builder) inheritedMethods(p string, f fileFacts, name string) (path string, ms []navMethod, lost bool) {
	seen := map[string]bool{p: true}
	for hop := 0; hop < maxInheritDepth; hop++ {
		base, ok := b.baseClass(p, f)
		switch {
		case !ok || seen[base]:
			return "", nil, true
		case base == "":
			return "", nil, false
		}
		seen[base] = true
		p, f = base, b.facts[base]
		if f.navDeclared[name] {
			return p, f.navMethods[name], false
		}
	}
	return "", nil, true
}

// helperGoto is the candidate goto of a helper call at line passing lit under form (see
// helperGotos); false when the URL is secret-shaped.
func (b *builder) helperGoto(line int, lit string, form helperForm, reason string) (Goto, bool) {
	g := Goto{Line: line, URL: form.prefix + lit, Status: StatusUnknown, Reason: reason}
	u, place := g.URL, true
	if hash := strings.Index(g.URL, "#"); form.hash || hash >= 0 && strings.Contains(form.prefix, "#") {
		if form.hash {
			g.URL, hash = "#"+strings.TrimPrefix(lit, "#"), 0
		}
		// The page before the hash serves the hash router's states, at the declared base path;
		// an explicit page outside that base path (AMAP-V0-191), or a hash the router's prefix
		// does not start, places no screen.
		_, outside := b.idx.basePathStripped(g.URL[:hash])
		u = b.idx.basePath + "/" + g.URL[hash:]
		place = !outside && b.idx.hashPrefix != "" && strings.HasPrefix(g.URL[hash:], b.idx.hashPrefix)
	}
	if secretscreen.MatchString(g.URL) {
		return Goto{}, false
	}
	if place {
		if id, _ := b.idx.byURL(u); id != "" {
			g.Screen = id
		}
	}
	g.URL = strings.ReplaceAll(g.URL, substMark, "${}")
	return g, true
}
