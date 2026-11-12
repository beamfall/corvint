package appmap

import (
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// The shared resolver for values a reader can prove from one file's tokens: a same-file array
// whose elements never change (AMAP-V0-970/971), and the values a `for (const X of ITER)` loop
// binds to X (AMAP-V0-972). `flows appmap build` (readFacts, the React router) and
// `flows appmap infer` (readTests) both read through it, so the two verbs place the same screens
// (AMAP-V0-974). Every function refuses, rather than guesses, when the proof is incomplete.

// arrayUses are the only uses of a proven array besides its declaration: none of them can change
// its elements or let other code reach it.
var arrayUses = map[string]bool{"map": true, "forEach": true, "length": true}

// constArray returns the element spans [a, b) of the array literal name binds, when the file
// binds name exactly once, by a non-exported `const` at any depth (an optional type annotation and
// `as const` allowed), to an array literal or to `useMemo(() => [...], deps)` (also
// `React.useMemo` and `=> ([...])`); every other use of name is `name.map(`, `name.forEach(`,
// `name.length` or the iterable of a `for (... of name)` header; no template substitution mentions
// name; no element is empty or a spread; and the lexer met nothing it doubts (AMAP-V0-971).
func constArray(toks []token, name string) ([][2]int, bool) {
	if len(toks) == 0 || toks[0].unsure {
		return nil, false
	}
	decl := -1
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
		case next(toks, k+1, ".") && k+2 < len(toks) && toks[k+2].kind == tokIdent && arrayUses[toks[k+2].text] &&
			(toks[k+2].text == "length" || next(toks, k+3, "(")):
		case word(toks, k-1, "of") && next(toks, k+1, ")"):
		default:
			return nil, false
		}
	}
	if decl < 0 {
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
	return arrayElements(toks, open)
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
			if spans, ok := constArray(toks, toks[iter[0]].text); ok {
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

// rebinds reports whether toks(open, close) may bind name again: a declaration (`const`, `let`,
// `var`, `function`, `class`), an assignment, a single arrow parameter, a function, arrow or catch
// parameter list or destructuring pattern holding name, or a token the reader doubts.
func rebinds(toks []token, open, close int, name string) bool {
	for k := open + 1; k < close; k++ {
		t := toks[k]
		if t.escaped || t.unsure || isPunct(t, "\\") {
			return true
		}
		if t.kind != tokIdent || t.text != name || property(toks, k) {
			continue
		}
		if k > 0 && toks[k-1].kind == tokIdent {
			switch toks[k-1].text {
			case "const", "let", "var", "function", "class":
				return true
			}
		}
		if next(toks, k+1, "=") && !next(toks, k+2, "=") {
			return true // an assignment or a single arrow parameter `name =>`
		}
		// Walk the enclosing brackets inside the loop body: a parameter list or a pattern binds.
		for c := k; ; {
			o := enclosingOpen(toks, c)
			if o <= open {
				break
			}
			end := closeParen(toks, o)
			switch {
			case isPunct(toks[o], "("):
				if next(toks, end+1, "=") && next(toks, end+2, ">") || word(toks, o-1, "catch") {
					return true
				}
				if next(toks, end+1, "{", ":") && !(o > 0 && toks[o-1].kind == tokIdent && controlWords[toks[o-1].text]) {
					return true // a function or method parameter list
				}
			case next(toks, end+1, "=") && !next(toks, end+2, "=", ">"):
				return true // a destructuring pattern
			}
			c = o
		}
	}
	return false
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
	t := toks[a]
	name := ""
	switch {
	case t.kind == tokIdent && !t.escaped:
		name = t.text
	case t.kind == tokTemplate:
		name = soleSubst(t)
	}
	if name != "" {
		if values, ok := loopValues(toks, a, name); ok {
			out := make([]rawGoto, 0, len(values))
			for _, v := range values {
				url := v
				if t.kind == tokTemplate {
					url, _ = fillTemplate(t, func(string) (string, bool) { return v, true })
				}
				out = append(out, one(url))
			}
			return out
		}
	}
	if (t.kind == tokString || t.kind == tokTemplate) && !t.inexact {
		return []rawGoto{one(t.text)}
	}
	return []rawGoto{{line: line, reason: "non-literal-url"}}
}
