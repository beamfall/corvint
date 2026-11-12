package appmap

import (
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// The shared resolver for values a reader can prove from one file's tokens: a same-file array
// whose elements never change (AMAP-V0-970/971), and the values a `for (const X of ITER)` loop
// binds to X (AMAP-V0-972). `flows appmap build` (readFacts, the React router) and
// `flows appmap infer` (readTests) both read through it, so the two verbs place the same screens
// (AMAP-V0-974). Every function refuses, rather than guesses, when the proof is incomplete.

// arrayUse reports whether toks[k], an occurrence of a proven array's name, is one of the only
// uses allowed besides its declaration: the iterable of a `for (const|let x of name)` header, or
// `name.map(` with an inline arrow callback of at most two parameters (the element and its index; a
// third or rest parameter would receive the array itself). In both the body that sees the element
// must be clean (cleanBody). Every other use, `.length` and `.forEach(` included, is refused rather
// than checked for writes (AMAP-V0-971). passes reports that the body hands the element to a call,
// which constArray accepts only for elements that are exact strings.
func arrayUse(toks []token, k int) (ok, passes bool) {
	switch {
	case word(toks, k-1, "of") && next(toks, k+1, ")") && k >= 5 && toks[k-2].kind == tokIdent &&
		(word(toks, k-3, "const") || word(toks, k-3, "let")) && next(toks, k-4, "(") && word(toks, k-5, "for"):
		lo := k + 1 // the body is a braced block or one statement
		hi := bodyEnd(toks, lo+1)
		if hi < 0 {
			return false, false
		}
		return cleanBody(toks, lo, hi, []string{toks[k-2].text})
	case next(toks, k+1, ".") && word(toks, k+2, "map") && !property(toks, k) && next(toks, k+3, "("):
		params, body, ok := arrowCallback(toks, k+4)
		if !ok {
			return false, false
		}
		return cleanBody(toks, body, closeParen(toks, k+3), params)
	}
	return false, false
}

// bodyEnd returns the index just past the statement starting at toks[at]: a braced block's
// closing brace, or the first `;` outside brackets; -1 when neither bounds it.
func bodyEnd(toks []token, at int) int {
	if next(toks, at, "{") {
		if c := closeParen(toks, at); c > at {
			return c
		}
		return -1
	}
	depth := 0
	for j := at; j < len(toks); j++ {
		switch {
		case next(toks, j, "(", "[", "{"):
			depth++
		case next(toks, j, ")", "]", "}"):
			if depth--; depth < 0 {
				return -1
			}
		case depth == 0 && next(toks, j, ";"):
			return j
		}
	}
	return -1
}

// arrowCallback reads the arrow function starting at toks[at] whose parameter list holds at most two
// parameters and no rest parameter (`x =>`, `() =>`, `(x) =>`, `(x, i) =>` or a destructuring
// pattern in their place): every identifier of the list, and the index of the token just before
// its body (the `>` of `=>`).
func arrowCallback(toks []token, at int) (params []string, body int, ok bool) {
	if at < len(toks) && toks[at].kind == tokIdent && next(toks, at+1, "=") && next(toks, at+2, ">") {
		return []string{toks[at].text}, at + 2, true
	}
	if !next(toks, at, "(") {
		return nil, 0, false
	}
	end := closeParen(toks, at)
	if end <= at || !next(toks, end+1, "=") || !next(toks, end+2, ">") {
		return nil, 0, false
	}
	commas, depth := 0, 0
	for j := at + 1; j < end; j++ {
		switch {
		case next(toks, j, "(", "[", "{"):
			depth++
		case next(toks, j, ")", "]", "}"):
			depth--
		case depth == 0 && next(toks, j, ","):
			commas++
		case next(toks, j, "."):
			return nil, 0, false // a rest parameter
		case toks[j].kind == tokIdent:
			params = append(params, toks[j].text)
		}
	}
	if commas > 1 && !(commas == 2 && next(toks, end-1, ",")) { // a trailing comma after two
		return nil, 0, false
	}
	return params, end + 2, true
}

// cleanBody reports whether toks(lo, hi), the body that sees an array element through names, is
// provably unable to change it: no assignment operator other than a JSX attribute's `=`, no `++`,
// `--` or `delete`, no doubted token, no template substitution other than a plain member path, and
// no call, computed member or member call on a name. passes reports a name handed to a call as an
// argument, which only an immutable element survives.
func cleanBody(toks []token, lo, hi int, names []string) (ok, passes bool) {
	if hi <= lo || hi > len(toks) {
		return false, false
	}
	for j := lo + 1; j < hi; j++ {
		t := toks[j]
		switch {
		case t.escaped || t.unsure || isPunct(t, "\\") || word(toks, j, "delete"):
			return false, false
		case t.kind == tokTemplate:
			subs, ok := templateSubs(t.code)
			if !ok {
				return false, false
			}
			for _, sub := range subs {
				if !memberPath(sub) {
					return false, false
				}
			}
		case next(toks, j, "+") && next(toks, j+1, "+"), next(toks, j, "-") && next(toks, j+1, "-"):
			return false, false
		case next(toks, j, "=") && !comparisonOrArrow(toks, j) && !jsxAttribute(toks, j, lo):
			return false, false
		case t.kind == tokIdent && slices.Contains(names, t.text) && !property(toks, j):
			c := j + 1
			for next(toks, c, "?") && next(toks, c+1, ".") || next(toks, c, ".") {
				if next(toks, c, "?") {
					c++
				}
				if c+1 >= len(toks) || toks[c+1].kind != tokIdent {
					return false, false
				}
				c += 2
			}
			if next(toks, c, "(", "[") || c < len(toks) && toks[c].kind == tokTemplate {
				return false, false // a call or computed member through the element
			}
			if o := enclosingOpen(toks, j); o > lo && isPunct(toks[o], "(") && callParen(toks, o) {
				passes = true
			}
		}
	}
	return true, passes
}

// comparisonOrArrow reports whether the `=` at toks[j] belongs to `==`, `===`, `!=`, `!==`, `<=`,
// `>=` or `=>` rather than to an assignment.
func comparisonOrArrow(toks []token, j int) bool {
	switch {
	case next(toks, j+1, "=", ">"), next(toks, j-1, "=", "!"):
		return true
	case next(toks, j-1, "<"):
		return !next(toks, j-2, "<")
	case next(toks, j-1, ">"):
		return !next(toks, j-2, ">")
	}
	return false
}

// jsxAttribute reports whether the `=` at toks[j] joins a JSX attribute name to its `{...}` or
// string value inside an opening tag that starts after toks[lo].
func jsxAttribute(toks []token, j, lo int) bool {
	if j < 2 || toks[j-1].kind != tokIdent || next(toks, j-2, ".") || j+1 >= len(toks) ||
		!(next(toks, j+1, "{") || toks[j+1].kind == tokString) {
		return false
	}
	depth := 0
	for k := j - 2; k > lo; k-- {
		switch {
		case next(toks, k, ")", "]", "}"):
			depth++
		case next(toks, k, "(", "[", "{"):
			if depth == 0 {
				return false
			}
			depth--
		case depth > 0:
		case next(toks, k, "<"):
			return k+1 < len(toks) && toks[k+1].kind == tokIdent
		case next(toks, k, ">", ";"):
			return false
		}
	}
	return false
}

// callParen reports whether the `(` at toks[o] opens a call's argument list.
func callParen(toks []token, o int) bool {
	if o == 0 {
		return false
	}
	p := toks[o-1]
	if p.kind == tokIdent {
		return !controlWords[p.text] && !keywordBeforeParen[p.text]
	}
	return isPunct(p, ")") || isPunct(p, "]")
}

// keywordBeforeParen are words a `(` may follow without being a call.
var keywordBeforeParen = map[string]bool{"return": true, "typeof": true, "await": true, "yield": true,
	"case": true, "in": true, "of": true, "void": true, "catch": true, "function": true}

// memberPath reports whether s is a plain name or member path such as `s.path`.
func memberPath(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if part == "" || !identChars(part) || strings.ContainsAny(part, " \t\n()[]{}=+-*/%&|^!?:;,<>`'\"") {
			return false
		}
	}
	return true
}

// constArray returns the element spans [a, b) of the array literal name binds at toks[use], when
// the file binds name exactly once, by a non-exported `const` at any depth whose enclosing block
// contains use (an optional type annotation and `as const` allowed), to an array literal or to
// `useMemo(() => [...], deps)` (also `React.useMemo` and `=> ([...])`); every other use of name is
// an arrayUse or the iterable of a `for (... of name)` header; no template substitution mentions
// name; no element is empty or a spread; and the lexer met nothing it doubts (AMAP-V0-971).
func constArray(toks []token, name string, use int) ([][2]int, bool) {
	if len(toks) == 0 || toks[0].unsure {
		return nil, false
	}
	decl, passes := -1, false
	for k, t := range toks {
		switch {
		case t.escaped || isPunct(t, "\\") || t.unsure:
			return nil, false
		case t.kind == tokTemplate && strings.Contains(t.code, name):
			return nil, false
		case t.kind != tokIdent || t.text != name || property(toks, k):
		case word(toks, k-1, "const") && !word(toks, k-2, "export") && !next(toks, k-2, "."):
			if decl >= 0 {
				return nil, false
			}
			decl = k
		default:
			ok, p := arrayUse(toks, k)
			if !ok {
				return nil, false
			}
			passes = passes || p
		}
	}
	if decl < 0 {
		return nil, false
	}
	// The declaration's block must contain the use: a const of another function is another binding.
	if o := enclosingOpen(toks, decl); o >= 0 && !(o < use && use < closeParen(toks, o)) {
		return nil, false
	}
	init := decl + 1
	if next(toks, init, ":") { // a type annotation runs to the first `=` that is not `=>`
		depth := 0
		for init++; init < len(toks) && !(depth == 0 && next(toks, init, "=") && !next(toks, init+1, ">")); init++ {
			switch {
			case next(toks, init, "(", "[", "{"):
				depth++
			case next(toks, init, ")", "]", "}"):
				depth--
			}
			if depth < 0 {
				return nil, false
			}
		}
	}
	if !next(toks, init, "=") || next(toks, init+1, "=", ">") {
		return nil, false
	}
	open, end := init+1, -1
	switch {
	case next(toks, open, "["):
		end = closeParen(toks, open) + 1
	case word(toks, open, "useMemo") || word(toks, open, "React") && next(toks, open+1, ".") && word(toks, open+2, "useMemo"):
		if !word(toks, open, "useMemo") {
			open += 2
		}
		call := open + 1
		// useMemo ( ( ) => [ ... ] , deps )   or   useMemo ( ( ) => ( [ ... ] ) , deps )
		if !next(toks, call, "(") || !next(toks, call+1, "(") || !next(toks, call+2, ")") || !next(toks, call+3, "=") || !next(toks, call+4, ">") {
			return nil, false
		}
		body, wrapped := call+5, false
		if next(toks, body, "(") && next(toks, body+1, "[") {
			wrapped, body = true, body+1
		}
		if !next(toks, body, "[") {
			return nil, false
		}
		after := closeParen(toks, body) + 1
		if wrapped {
			if !next(toks, after, ")") {
				return nil, false
			}
			after++
		}
		if !next(toks, after, ",") || closeParen(toks, call) <= after {
			return nil, false
		}
		open, end = body, closeParen(toks, call)+1
	default:
		return nil, false
	}
	if end <= open || end > len(toks) {
		return nil, false
	}
	if word(toks, end, "as") && word(toks, end+1, "const") {
		end += 2
	}
	// The declaration must end here: `.filter(...)` or any other operator would compute a new value.
	if end < len(toks) && !next(toks, end, ";", "}") && !(toks[end].kind == tokIdent && toks[end].line > toks[end-1].line) {
		return nil, false
	}
	spans, ok := arrayElements(toks, open)
	if ok && passes { // an element handed to a call survives only when it is immutable
		if _, strs := exactStrings(toks, spans); !strs {
			return nil, false
		}
	}
	return spans, ok
}

// arrayElements splits the array literal whose `[` is toks[open] into its element spans; an empty
// element or a spread refuses. A trailing comma is allowed.
func arrayElements(toks []token, open int) ([][2]int, bool) {
	close := closeParen(toks, open)
	if close <= open || !next(toks, close, "]") {
		return nil, false
	}
	var out [][2]int
	start, depth := open+1, 0
	for k := open + 1; k <= close; k++ {
		switch {
		case next(toks, k, "(", "[", "{"):
			depth++
		case k < close && next(toks, k, ")", "]", "}"):
			depth--
		case depth == 0 && (k == close || next(toks, k, ",")):
			if k == start {
				if k == close && len(out) > 0 && next(toks, k-1, ",") {
					return out, true // trailing comma
				}
				if k == close && len(out) == 0 {
					return out, true // empty array
				}
				return nil, false
			}
			if next(toks, start, ".") {
				return nil, false // a spread
			}
			out = append(out, [2]int{start, k})
			start = k + 1
		}
	}
	return out, true
}

// exactStrings returns the values of spans that are each one exact string literal.
func exactStrings(toks []token, spans [][2]int) ([]string, bool) {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		if s[1]-s[0] != 1 || !literal(toks[s[0]]) {
			return nil, false
		}
		out = append(out, toks[s[0]].text)
	}
	return out, len(out) > 0
}

// bracketOpen returns the index of the bracket that the closing bracket toks[close] matches, or -1.
func bracketOpen(toks []token, close int) int {
	depth := 0
	for j := close; j >= 0; j-- {
		switch {
		case next(toks, j, ")", "]", "}"):
			depth++
		case next(toks, j, "(", "[", "{"):
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// forOfLoop is an enclosing braced `for (const name of ITER) { ... }`: its body's braces and the
// exact strings ITER holds (nil when ITER is not proven).
type forOfLoop struct {
	name        string
	open, close int
	values      []string
}

// enclosingLoops returns the braced `for (const X of ITER)` loops around toks[at], innermost first.
func enclosingLoops(toks []token, at int) []forOfLoop {
	var out []forOfLoop
	depth := 0
	for k := at - 1; k >= 0; k-- {
		switch {
		case next(toks, k, ")", "]", "}"):
			depth++
			continue
		case next(toks, k, "(", "["):
			if depth > 0 {
				depth--
			}
			continue
		case !next(toks, k, "{"):
			continue
		}
		if depth > 0 {
			depth--
			continue
		}
		// toks[k] is an enclosing `{`; it is a loop body when `for ( const X of ITER )` precedes it.
		if !next(toks, k-1, ")") {
			continue
		}
		op := bracketOpen(toks, k-1)
		if op < 1 || !word(toks, op-1, "for") || !word(toks, op+1, "const") || op+3 >= k-1 ||
			toks[op+2].kind != tokIdent || !word(toks, op+3, "of") {
			continue
		}
		l := forOfLoop{name: toks[op+2].text, open: k, close: closeParen(toks, k)}
		iter := [2]int{op + 4, k - 1}
		switch {
		case iter[1]-iter[0] == 1 && toks[iter[0]].kind == tokIdent && !toks[iter[0]].escaped:
			if spans, ok := constArray(toks, toks[iter[0]].text, iter[0]); ok {
				l.values, _ = exactStrings(toks, spans)
			}
		case next(toks, iter[0], "[") && closeParen(toks, iter[0]) == iter[1]-1:
			if spans, ok := arrayElements(toks, iter[0]); ok {
				l.values, _ = exactStrings(toks, spans)
			}
		}
		out = append(out, l)
	}
	return out
}

// loopValues returns the exact strings the innermost enclosing braced `for (const name of ITER)`
// binds to name at toks[at], when ITER is an inline array of exact strings or a constArray of them
// and nothing inside the loop body binds name again (AMAP-V0-972). An enclosing loop over another
// variable is passed through.
func loopValues(toks []token, at int, name string) ([]string, bool) {
	for _, l := range enclosingLoops(toks, at) {
		if l.name != name {
			continue
		}
		if l.values == nil || l.close < 0 || rebinds(toks, l.open, l.close, name) {
			return nil, false
		}
		return l.values, true
	}
	return nil, false
}

// rebinds reports whether toks(open, close) may bind name again. Rather than detect each binding
// form, it accepts only a body it can prove: no nested `for` (braced or not, `for await`
// included), no declaration or binding keyword (`const`, `let`, `var`, `function`, `class`,
// `catch`), no doubted token, name only as a whole call argument (`f(name)`, `f(a, name)`), and
// every template substitution that mentions name exactly name (AMAP-V0-972).
func rebinds(toks []token, open, close int, name string) bool {
	for k := open + 1; k < close; k++ {
		t := toks[k]
		switch {
		case t.escaped || t.unsure || isPunct(t, "\\"):
			return true
		case t.kind == tokTemplate:
			subs, ok := templateSubs(t.code)
			if !ok {
				return true
			}
			for _, sub := range subs {
				if sub != name && mentions(sub, name) {
					return true
				}
			}
		case t.kind != tokIdent || property(toks, k):
		case bindingWords[t.text]:
			return true
		case t.text != name:
		case !next(toks, k-1, "(", ",") || !next(toks, k+1, ")", ","):
			return true
		default:
			if o := enclosingOpen(toks, k); o <= open || !isPunct(toks[o], "(") || !callParen(toks, o) {
				return true
			}
		}
	}
	return false
}

// bindingWords start a loop header or a declaration that may bind a name inside a loop body.
var bindingWords = map[string]bool{"for": true, "const": true, "let": true, "var": true,
	"function": true, "class": true, "catch": true}

// mentions reports whether the source s names the identifier name other than as a member.
func mentions(s, name string) bool {
	identByte := func(c byte) bool {
		return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
	}
	for i := 0; ; {
		j := strings.Index(s[i:], name)
		if j < 0 {
			return false
		}
		j += i
		end := j + len(name)
		if (j == 0 || !identByte(s[j-1]) && s[j-1] != '.') && (end == len(s) || !identByte(s[end])) {
			return true
		}
		i = j + 1
	}
}

// controlWords open a parenthesised condition, never a parameter list.
var controlWords = map[string]bool{"if": true, "while": true, "for": true, "switch": true, "with": true}

// templateSubs returns the trimmed source of each `${...}` substitution in a template token's code,
// or false when a substitution cannot be bounded.
func templateSubs(code string) ([]string, bool) {
	var subs []string
	for rest := code; ; {
		i := strings.Index(rest, "${")
		if i < 0 {
			return subs, true
		}
		j := strings.IndexByte(rest[i:], '}')
		if j < 0 {
			return nil, false
		}
		subs = append(subs, strings.TrimSpace(rest[i+2:i+j]))
		rest = rest[i+j+1:]
	}
}

// fillTemplate returns a template literal's value with each substitution replaced by value(source),
// or false when the template is inexact, doubted, or any substitution has no value.
func fillTemplate(t token, value func(string) (string, bool)) (string, bool) {
	if t.kind != tokTemplate || !t.subst || t.inexact || t.unsure {
		return "", false
	}
	subs, ok := templateSubs(t.code)
	parts := strings.Split(t.text, substMark)
	if !ok || len(parts) != len(subs)+1 {
		return "", false
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for k, s := range subs {
		v, ok := value(s)
		if !ok {
			return "", false
		}
		b.WriteString(v)
		b.WriteString(parts[k+1])
	}
	return b.String(), true
}

// soleSubst returns the one identifier every substitution of a template token names, or "".
func soleSubst(t token) string {
	if t.kind != tokTemplate || !t.subst || t.inexact || t.unsure {
		return ""
	}
	subs, ok := templateSubs(t.code)
	if !ok || len(subs) == 0 {
		return ""
	}
	for _, s := range subs {
		if s != subs[0] || !identChars(s) {
			return ""
		}
	}
	return subs[0]
}

// gotoTargets resolves the argument of a goto, waitForURL or toHaveURL call at toks[a] (whole
// when it is the complete argument) into one raw goto per target URL: an exact string or template
// as before, or, for the variable of an enclosing literal for-of loop or a template whose every
// substitution is that variable, one goto per loop value (AMAP-V0-972). Anything else keeps
// `non-literal-url`; a secret-shaped URL is withheld (AMAP-V0-974).
func gotoTargets(toks []token, a int, whole bool, line int) []rawGoto {
	one := func(url string) rawGoto {
		if secretscreen.MatchString(url) {
			return rawGoto{line: line, reason: "secret-shaped"}
		}
		return rawGoto{line: line, url: url}
	}
	if a >= len(toks) || !whole {
		return []rawGoto{{line: line, reason: "non-literal-url"}}
	}
	if urls, ok := loopURLs(toks, a); ok {
		out := make([]rawGoto, 0, len(urls))
		for _, url := range urls {
			out = append(out, one(url))
		}
		return out
	}
	t := toks[a]
	if (t.kind == tokString || t.kind == tokTemplate) && !t.inexact {
		return []rawGoto{one(t.text)}
	}
	return []rawGoto{{line: line, reason: "non-literal-url"}}
}

// loopURLs returns one URL per loop value when the whole goto argument toks[a] is the variable of
// an enclosing literal for-of loop, or a template whose every substitution is that variable
// (AMAP-V0-972). gotoTargets and the reference check (scanRefs) both read loop gotos through it.
func loopURLs(toks []token, a int) ([]string, bool) {
	t := toks[a]
	name := ""
	switch {
	case t.kind == tokIdent && !t.escaped:
		name = t.text
	case t.kind == tokTemplate:
		name = soleSubst(t)
	}
	if name == "" {
		return nil, false
	}
	values, ok := loopValues(toks, a, name)
	if !ok {
		return nil, false
	}
	urls := make([]string, 0, len(values))
	for _, v := range values {
		url := v
		if t.kind == tokTemplate {
			url, _ = fillTemplate(t, func(string) (string, bool) { return v, true })
		}
		urls = append(urls, url)
	}
	return urls, true
}
