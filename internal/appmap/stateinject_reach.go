package appmap

import (
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// arrayReach proves a few non-injector uses inside the already followed closure. Unknown calls,
// namespace consumers, re-exports and opaque code are escapes, not evidence of absence. In
// particular, merely lacking an AngularJS token in the array's own file proves nothing.
type arrayReach struct {
	table  *constTable
	files  []*constFile
	proofs map[*constFile]arrayProof
	names  map[arrayName]bool
}

type arrayProof struct {
	quiet map[int]bool
	key   string
}

type arrayName struct {
	file        *constFile
	name        string
	declaration int
	dormant     bool
}

// quietArrays returns literal token positions with a closed non-injector use proof (AMAP-V0-420).
// Keep the proof in the quietness memo key: another router may have a different consumer closure.
func (r *arrayReach) quietArrays(f *constFile) (map[int]bool, string) {
	if proof, ok := r.proofs[f]; ok {
		return proof.quiet, proof.key
	}
	quiet := map[int]bool{}
	var key strings.Builder
	for i, tok := range f.toks {
		if !isPunct(tok, "[") {
			continue
		}
		end := closeParen(f.toks, i)
		if !balanced(f.toks, i, end) || !next(f.toks, end, "]") {
			continue
		}
		// An actual array argument remains injector-facing even in a dormant declaration. An
		// arrow's returned array is not itself an argument (its preceding token is `>`).
		directArgument := callArgument(f.toks, i) && i > 0 && next(f.toks, i-1, "(", ",")
		if elementUse(f.toks, i, end+1) || r.projectedArray(f, i, end) ||
			!directArgument && r.dormantFunction(f, i) || r.heldArray(f, i, end) || localArrayCopy(f, i, end) ||
			pushedLocal(f, i, end) || r.methodArgument(f, i, end) {
			quiet[i] = true
			key.WriteString(strconv.Itoa(i))
			key.WriteByte(',')
		}
	}
	proof := arrayProof{quiet, key.String()}
	r.proofs[f] = proof
	return proof.quiet, proof.key
}

// projectedArray recognizes the issue's RxJS mergeMap projection. Its synchronous array result
// is flattened into elements, so this array identity never reaches an injector. This is a known
// package operation, not a name heuristic: local modules, rebinding, async callbacks (which emit
// the resolved array), other operators and unknown callbacks retain uncertainty.
func (r arrayReach) projectedArray(f *constFile, i, end int) bool {
	toks := f.toks
	open := enclosingOpen(toks, i)
	if open < 1 || !next(toks, open, "(") || toks[open-1].kind != tokIdent || property(toks, open-1) ||
		!next(toks, i-2, "=") || !next(toks, i-1, ">") || !next(toks, end+1, ")") || closeParen(toks, open) != end+1 {
		return false
	}
	if i != open+4 || toks[open+1].kind != tokIdent {
		_, last, ok := simpleParams(toks, open+1)
		if !next(toks, open+1, "(") || !ok || last != i-3 {
			return false
		}
	}
	name := toks[open-1].text
	imp, ok := f.imports[name]
	if !ok || imp.bad || imp.exported != "mergeMap" || imp.module != "rxjs" && imp.module != "rxjs/operators" ||
		r.table.resolver == nil || !r.table.external(imp.module) ||
		r.table.resolver.Resolve(f.entry.path, imp.module).State != contextindex.WebImportPackage || f.hidden[name] {
		return false
	}
	for j, tok := range toks {
		if tok.kind != tokIdent || tok.text != name || property(toks, j) {
			continue
		}
		if f.skip[j] && tok.line >= imp.first && tok.line <= imp.last {
			continue
		}
		if !next(toks, j+1, "(") || word(toks, j-1, "function") || word(toks, j-1, "new") ||
			next(toks, closeParen(toks, j+1)+1, "{") {
			return false
		}
	}
	return true
}

// localArrayCopy proves a parameter copied by `xs = [...xs]` is thereafter used only element-wise
// (copyUse, AMAP-V0-424). Returning or passing xs, indexing it, or another write is an escape. The helper itself
// may execute: the copied array cannot leave it, unlike a helper returning an annotation.
func localArrayCopy(f *constFile, i, end int) bool {
	toks := f.toks
	if i < 2 || toks[i-2].kind != tokIdent || property(toks, i-2) || !next(toks, i-1, "=") ||
		end != i+5 || !next(toks, i+1, ".") || !next(toks, i+2, ".") || !next(toks, i+3, ".") ||
		!word(toks, i+4, toks[i-2].text) || !arrayInitializerEnd(toks, end) {
		return false
	}
	name := toks[i-2].text
	if f.hidden[name] {
		return false
	}
	for p := 0; p < i; p++ {
		if !word(toks, p, name) {
			continue
		}
		pf, ok := f.paramFunc(p, name)
		if !ok || i <= pf.fn.from || end >= pf.fn.to {
			continue
		}
		for j := pf.fn.from; j < pf.fn.to; j++ {
			if !word(toks, j, name) || property(toks, j) || objectKey(toks, j) || j == i-2 || j == i+4 {
				continue
			}
			if !copyUse(toks, j) {
				return false
			}
		}
		return true
	}
	return false
}

// scalarArrayUse admits only a read of length or join with an exact separator: neither exposes
// this array or a callable result. Other member calls can return or mutate the array and refuse.
func scalarArrayUse(toks []token, start, after int) bool {
	if !next(toks, after, ".") {
		return false
	}
	if word(toks, after+1, "length") {
		return pureRead(toks, start, after+2)
	}
	if !word(toks, after+1, "join") || !next(toks, after+2, "(") {
		return false
	}
	end := closeParen(toks, after+2)
	return next(toks, end, ")") && (end == after+3 || end == after+4 && literal(toks[after+3]))
}

func (r arrayReach) heldArray(f *constFile, i, end int) bool {
	toks := f.toks
	at := constName(toks, i)
	if at < 0 || !arrayInitializerEnd(toks, end) {
		return false
	}
	// Preserve the existing refusal for an explicit function element; this proof deliberately
	// admits data lists only, not unconsumed hand-written annotations. An arrow nested in an
	// element's call (`...list.map(x => x.n)`) is not itself an element (AMAP-V0-424).
	for k := i + 1; k < end; {
		e := min(skipValue(toks, k), end)
		if elementFunction(toks, k, e) {
			return false
		}
		k = e + 1
	}
	return r.dataName(f, toks[at].text, at, false)
}

// constName returns the index of NAME when the array at toks[i] is the initializer of `const NAME =`
// or `const NAME: T =` with a plain type annotation (see typeEnd, AMAP-V0-424), or -1.
func constName(toks []token, i int) int {
	if i < 3 || !next(toks, i-1, "=") {
		return -1
	}
	if word(toks, i-3, "const") && toks[i-2].kind == tokIdent {
		return i - 2
	}
	for c := i - 2; c >= 2 && c > i-40; c-- {
		if next(toks, c, ":") {
			if toks[c-1].kind == tokIdent && word(toks, c-2, "const") && typeEnd(toks, c+1) == i-1 {
				return c - 1
			}
			return -1
		}
	}
	return -1
}

// elementFunction reports whether the element toks[k:e] may itself be a function literal: it
// starts with `function`, `async` or `class`, or holds an arrow `=>` outside any bracket group.
func elementFunction(toks []token, k, e int) bool {
	if word(toks, k, "function") || word(toks, k, "async") || word(toks, k, "class") {
		return true
	}
	depth := 0
	for j := k; j+1 < e; j++ {
		switch {
		case next(toks, j, "(", "[", "{"):
			depth++
		case next(toks, j, ")", "]", "}"):
			depth--
		case depth == 0 && isPunct(toks[j], "=") && isPunct(toks[j+1], ">"):
			return true
		}
	}
	return false
}

func arrayInitializerEnd(toks []token, end int) bool {
	a := end + 1
	return a == len(toks) || next(toks, a, ";", "}") ||
		a < len(toks) && (toks[a].kind == tokIdent || next(toks, a, "@")) && toks[a].line > toks[end].line
}

// dormantFunction proves a named, top-level function declaration has no executable use in the closure.
// Arrays in its body (including nested callback bodies) cannot reach an injector. A call, passing
// the function, a second binding, or any unresolved export consumer keeps its arrays blocking.
func (r arrayReach) dormantFunction(f *constFile, at int) bool {
	toks := f.toks
	for i := 0; i+3 < at; i++ {
		if !word(toks, i, "function") || f.depthAt(i) != 0 || toks[i+1].kind != tokIdent || !next(toks, i+2, "(") ||
			!statementStart(toks, i) {
			continue
		}
		_, end, ok := simpleParams(toks, i+2)
		if !ok || !next(toks, end+1, "{") || at <= end+1 || at >= closeParen(toks, end+1) {
			continue
		}
		return r.dataName(f, toks[i+1].text, i+1, true)
	}
	return false
}

// statementStart reports whether toks[i] begins a declaration statement. A named function
// expression (after `=`, `(`, `return`, `default` or an operator) can still run, so it is not one.
func statementStart(toks []token, i int) bool {
	return i == 0 || next(toks, i-1, ";", "}") || word(toks, i-1, "export")
}

// dataName accepts only nonescaping local uses and named imports with the same proof. It does
// not chase re-exports or aliases: those need a broader graph than this bounded data-list proof.
func (r arrayReach) dataName(f *constFile, name string, declaration int, dormant bool) (quiet bool) {
	key := arrayName{f, name, declaration, dormant}
	if result, ok := r.names[key]; ok {
		return result
	}
	defer func() { r.names[key] = quiet }()
	if !r.dataNameUses(f, name, declaration, dormant) {
		return false
	}
	if !f.exports[name] {
		if !dormant {
			return true
		}
		// A script-level function may be a global binding in another examined script. Only a
		// module-local declaration can be proved dormant from its local references alone.
		for j := range f.toks {
			if f.depthAt(j) == 0 && (word(f.toks, j, "import") || word(f.toks, j, "export")) {
				return true
			}
		}
		return false
	}
	if r.table.resolver == nil || f.listed[name] || f.dflt == name {
		return false
	}
	for _, consumer := range r.files {
		if consumer.unlisted || consumer.opaque {
			return false
		}
		loads := func(module string) bool {
			res := r.table.resolver.Resolve(consumer.entry.path, module)
			return res.State == contextindex.WebImportRepository && res.Target == f.entry.path
		}
		for _, module := range consumer.spaces {
			if module == "" || loads(module) {
				return false
			}
		}
		for _, exp := range consumer.reexports {
			if loads(exp.module) {
				return false
			}
		}
		for local, imp := range consumer.imports {
			if loads(imp.module) && imp.exported == name &&
				(imp.bad || consumer.exports[local] || !r.dataNameUses(consumer, local, -1, dormant)) {
				return false
			}
		}
	}
	return true
}

// dataNameUses also admits a reference that is a whole argument of a proved `this.m(...)` call
// (methodArgument, AMAP-V0-426): the method's parameter is then used only element-wise.
func (r arrayReach) dataNameUses(f *constFile, name string, declaration int, dormant bool) bool {
	if f.unlisted || f.opaque || f.hidden[name] {
		return false
	}
	for j, tok := range f.toks {
		if word(f.toks, j, "eval") || word(f.toks, j, "Function") ||
			(tok.kind == tokString || tok.kind == tokTemplate) && strings.Contains(tok.text, name) {
			return false
		}
		if j == declaration || tok.kind != tokIdent || tok.text != name || property(f.toks, j) || objectKey(f.toks, j) {
			continue
		}
		// Only import headers may be skipped here, not another const declaration of this name.
		if imp, ok := f.imports[name]; ok && tok.line >= imp.first && tok.line <= imp.last && f.skip[j] {
			continue
		}
		if !dormant && (copyUse(f.toks, j) ||
			j > 0 && next(f.toks, j-1, ":") && next(f.toks, j+1, ",", "}") && decoratorMetadata(f.toks, j) ||
			r.methodArgument(f, j, j)) {
			continue
		}
		return false
	}
	return true
}

// copyUse reports whether the reference toks[j] to a binding holding the array uses it only
// element-wise (AMAP-V0-424): one elementUse, the iterable of a `for (const|let V of NAME)` head
// whose V is used only as elementBinding admits, or a whole spread element `...NAME` of an array
// literal that is not an assignment pattern, which is then judged by itself. Neither the array
// identity nor any element reaches a call, a container or another binding.
func copyUse(toks []token, j int) bool {
	if elementUse(toks, j, j+1) {
		return true
	}
	if o := enclosingOpen(toks, j); word(toks, j-1, "of") && next(toks, j+1, ")") && o > 0 && isPunct(toks[o], "(") &&
		(word(toks, o-1, "for") || word(toks, o-1, "await") && word(toks, o-2, "for")) {
		return o == j-4 && (word(toks, j-3, "const") || word(toks, j-3, "let")) && toks[j-2].kind == tokIdent &&
			elementBinding(toks, toks[j-2].text, j+2, scopeEnd(toks, o))
	}
	o := enclosingOpen(toks, j)
	if j < 3 || !next(toks, j-1, ".") || !next(toks, j-2, ".") || !next(toks, j-3, ".") || !next(toks, j+1, ",", "]") ||
		o < 0 || !isPunct(toks[o], "[") {
		return false
	}
	c := closeParen(toks, o)
	return !next(toks, c+1, "=") || next(toks, c+2, "=")
}

// elementMethods are array methods whose only argument is a callback receiving an element and
// its index, and whose result is a boolean, a number, undefined or an element: never the array.
var elementMethods = map[string]bool{"forEach": true, "some": true, "every": true, "find": true, "findIndex": true,
	"findLast": true, "findLastIndex": true}

// elementUse reports whether the receiver toks[start:after] is used by one member operation that
// cannot expose the array or a copy of its elements (AMAP-V0-424): scalarArrayUse; `includes`,
// `indexOf` or `lastIndexOf` with arguments; `shift()` or `pop()` whose element result is an
// elementSink or boundElement; or an elementMethods call whose only argument is an arrow with at
// most two plain parameters (the third would receive the array) whose element parameter is used
// only as elementBinding admits, the element `find` or `findLast` returns being an elementSink or
// boundElement too. An element stored, passed or returned could rebuild the list, so a named or
// `function` callback, `map`, `filter`, `slice`, `concat` and other derived arrays refuse.
func elementUse(toks []token, start, after int) bool {
	if scalarArrayUse(toks, start, after) {
		return true
	}
	if !next(toks, after, ".") || after+2 >= len(toks) || toks[after+1].kind != tokIdent || !next(toks, after+2, "(") {
		return false
	}
	c := closeParen(toks, after+2)
	switch m := toks[after+1].text; {
	case m == "includes" || m == "indexOf" || m == "lastIndexOf":
		return c > after+3
	case m == "shift" || m == "pop":
		return c == after+3 && (elementSink(toks, start, c+1) || boundElement(toks, start, c+1))
	case elementMethods[m]:
		v, body, ok := arrowArgument(toks, after+3, c)
		if !ok || !elementBinding(toks, v, body, c) {
			return false
		}
		return m != "find" && m != "findLast" || elementSink(toks, start, c+1) || boundElement(toks, start, c+1)
	}
	return false
}

// boundElement reports whether the element value toks[s:e] is the whole initializer of
// `const|let NAME = ...` ended by statementEnd, NAME being used only as elementBinding admits up to
// the end of the enclosing block (AMAP-V0-424).
func boundElement(toks []token, s, e int) bool {
	return s >= 3 && next(toks, s-1, "=") && !(toks[s-1].glued && next(toks, s-2, "=", "!", "<", ">")) &&
		toks[s-2].kind == tokIdent && (word(toks, s-3, "const") || word(toks, s-3, "let")) && statementEnd(toks, e) &&
		elementBinding(toks, toks[s-2].text, e, scopeEnd(toks, s))
}

// elementBinding reports whether every reference to name in toks[from:to] is an elementSink, no
// template there names it and no `eval` there could read it (AMAP-V0-424). An empty name, an
// unnamed element, has no references.
func elementBinding(toks []token, name string, from, to int) bool {
	if name == "" {
		return true
	}
	for j := from; j < to && j < len(toks); j++ {
		t := toks[j]
		if t.kind == tokTemplate && strings.Contains(t.code, name) || t.kind == tokIdent && t.text == "eval" {
			return false
		}
		if t.kind == tokIdent && t.text == name && !property(toks, j) && !objectKey(toks, j) && !elementSink(toks, j, j+1) {
			return false
		}
	}
	return true
}

// scopeEnd returns the index of the `}` closing the innermost block that encloses toks[k], or
// the end of the file: an over-approximation of the scope of a block-scoped binding declared there,
// so a `for (...)` head's binding covers the loop body.
func scopeEnd(toks []token, k int) int {
	o := enclosingOpen(toks, k)
	for o >= 0 && !isPunct(toks[o], "{") {
		o = enclosingOpen(toks, o)
	}
	if o < 0 {
		return len(toks)
	}
	return closeParen(toks, o)
}

// elementSink reports whether the element value toks[s:e] ends in a use that can neither store,
// pass, call nor return it (AMAP-V0-424): an operand of `==`, `!=`, `===` or `!==`; the operand of
// `typeof`; a discarded expression statement; or the whole argument of `includes`, `indexOf` or
// `lastIndexOf` on an array literal. Any other use, as an array element or a spread included,
// could rebuild the list.
func elementSink(toks []token, s, e int) bool {
	terminal := e >= len(toks) || !next(toks, e, ".", "(", "[") && toks[e].kind != tokTemplate &&
		!(next(toks, e, "?") && next(toks, e+1, ".")) && !(next(toks, e, "=") && !(next(toks, e+1, "=") && toks[e+1].glued))
	switch {
	case next(toks, e, "=", "!") && next(toks, e+1, "=") && toks[e+1].glued:
		return true
	case s >= 2 && next(toks, s-1, "=") && toks[s-1].glued && next(toks, s-2, "=", "!"):
		return terminal
	case word(toks, s-1, "typeof"):
		return terminal
	case (s == 0 || next(toks, s-1, ";", "{", "}")) && next(toks, e, ";"):
		return true
	case s >= 4 && next(toks, s-1, "(") && (word(toks, s-2, "includes") || word(toks, s-2, "indexOf") || word(toks, s-2, "lastIndexOf")) &&
		next(toks, s-3, ".") && next(toks, s-4, "]") && next(toks, e, ")"):
		q := enclosingOpen(toks, s-4)
		return q >= 0 && isPunct(toks[q], "[") && (q == 0 || word(toks, q-1, "return") ||
			toks[q-1].kind == tokPunct && !next(toks, q-1, ")", "]", "}"))
	}
	return false
}

// arrowArgument reads a call's whole argument list toks[from:close] as one arrow `v => body` or
// `(a, b) => body` with at most two plain parameters, returning the first parameter's name (empty
// when there is none) and the body's first token.
func arrowArgument(toks []token, from, close int) (string, int, bool) {
	if from >= close || skipValue(toks, from) != close {
		return "", 0, false
	}
	k, v := from+1, ""
	switch {
	case toks[from].kind == tokIdent && next(toks, k, "=") && next(toks, k+1, ">"):
		v = toks[from].text
	case next(toks, from, "("):
		params, end, ok := simpleParams(toks, from)
		if !ok || len(params) > 2 {
			return "", 0, false
		}
		if len(params) > 0 {
			v = params[0].name
		}
		k = end + 1
		if !next(toks, k, "=") || !next(toks, k+1, ">") {
			return "", 0, false
		}
	default:
		return "", 0, false
	}
	return v, k + 2, k+2 < close
}

// pushedLocal proves an array that is the whole argument of `NAME.push(...)` never leaves NAME
// (AMAP-V0-424): NAME is declared once in the file, as `const` or `let` initialized to exactly `[]`
// (statementEnd follows), and
// every other reference is another `NAME.push(...)` or a read of `length`, `join`, `includes`,
// `indexOf` or `lastIndexOf`. Returning, passing, iterating, indexing or spreading NAME would hand
// out this array as an element, so each refuses.
func pushedLocal(f *constFile, i, end int) bool {
	toks := f.toks
	if i < 4 || !next(toks, i-1, "(") || !word(toks, i-2, "push") || !next(toks, i-3, ".") || toks[i-4].kind != tokIdent ||
		property(toks, i-4) || !next(toks, end+1, ")") {
		return false
	}
	name := toks[i-4].text
	if f.hidden[name] || f.exports[name] {
		return false
	}
	declared := 0
	for j, tok := range toks {
		if tok.kind == tokTemplate && strings.Contains(tok.code, name) {
			return false
		}
		if tok.kind != tokIdent || tok.text != name || property(toks, j) || objectKey(toks, j) {
			continue
		}
		switch {
		case (word(toks, j-1, "const") || word(toks, j-1, "let")) && next(toks, j+1, "=") && next(toks, j+2, "[") && next(toks, j+3, "]") &&
			statementEnd(toks, j+4):
			declared++
		case next(toks, j+1, ".") && word(toks, j+2, "push") && next(toks, j+3, "("):
		case next(toks, j+1, ".") && (word(toks, j+2, "includes") || word(toks, j+2, "indexOf") || word(toks, j+2, "lastIndexOf")):
			if !elementUse(toks, j, j+1) {
				return false
			}
		default:
			if !scalarArrayUse(toks, j, j+1) {
				return false
			}
		}
	}
	return declared == 1
}

// injectorMethods are the AngularJS module, provider and injector members that read an
// annotation; a `this.m(...)` call by one of these names is never proved (AMAP-V0-424).
var injectorMethods = map[string]bool{"run": true, "config": true, "controller": true, "service": true, "factory": true,
	"provider": true, "directive": true, "component": true, "filter": true, "constant": true, "value": true,
	"decorator": true, "animation": true, "invoke": true, "instantiate": true, "annotate": true, "get": true,
	"register": true, "has": true}

// methodArgument proves an array passed whole to `this.m(...)` reaches only an element-wise
// parameter (AMAP-V0-424). The call's `this` is the instance of the class whose method body
// encloses it (only arrows, blocks and object literals lie between; a `function` or object
// method rebinds it), no argument before the array is a spread, every `this` in the class body
// is a plain `this.name` member access (an alias or `this[...]` refuses), m is not an
// injectorMethods name, no examined closure file has dynamicMembers, and m is defined exactly
// once in the whole examined closure: by that class's own plain method, whose every other
// occurrence anywhere is a property call `.m(...)`. No string or template there names m. The
// method's parameter at the array's position is then used only by copyUse, never as a spread
// element, and its body never reads `arguments`. An override, an object or field holding m, an
// unread or opaque closure file, or any other use keeps the array blocking.
func (r arrayReach) methodArgument(f *constFile, i, end int) bool {
	toks := f.toks
	o := enclosingOpen(toks, i)
	if o < 3 || !isPunct(toks[o], "(") || toks[o-1].kind != tokIdent || !next(toks, o-2, ".") || !word(toks, o-3, "this") ||
		property(toks, o-3) || !next(toks, i-1, "(", ",") || !next(toks, end+1, ")", ",") {
		return false
	}
	m := toks[o-1].text
	if injectorMethods[m] {
		return false
	}
	pos := 0 // the array's argument position; a spread before it could deliver it to any parameter
	for k := o + 1; skipValue(toks, k) < i; k = skipValue(toks, k) + 1 {
		if next(toks, k, ".") {
			return false
		}
		pos++
	}
	body := thisMethodBody(toks, o)
	if body < 0 {
		return false
	}
	class := enclosingOpen(toks, body)
	classEnd := closeParen(toks, class)
	decl := -1
	for k := class + 1; k < classEnd; k++ {
		if word(toks, k, "this") && !(next(toks, k+1, ".") && k+2 < len(toks) && toks[k+2].kind == tokIdent) {
			return false
		}
		if word(toks, k, m) && f.depthAt(k) == f.depthAt(class)+1 && next(toks, k+1, "(") && !property(toks, k) {
			if decl >= 0 {
				return false
			}
			decl = k
		}
	}
	if decl < 0 || !methodHead(toks, decl, class) {
		return false
	}
	for _, g := range append([]*constFile{f}, r.files...) {
		if g.unlisted || g.opaque || g.hidden[m] || dynamicMembers(g.toks) {
			return false
		}
		for j, tok := range g.toks {
			switch {
			case (tok.kind == tokString || tok.kind == tokTemplate) && (strings.Contains(tok.text, m) || strings.Contains(tok.code, m)):
				return false
			case tok.kind != tokIdent || tok.text != m || g == f && j == decl:
			case !property(g.toks, j) || !next(g.toks, j+1, "("):
				return false
			}
		}
	}
	params, closing, ok := simpleParams(toks, decl+1)
	if !ok || pos >= len(params) {
		return false
	}
	open := closing + 1
	if next(toks, open, ":") {
		open = typeEnd(toks, open+1)
	}
	if !next(toks, open, "{") {
		return false
	}
	p := params[pos].name
	if f.hidden[p] {
		return false
	}
	for j, last := open+1, closeParen(toks, open); j < last; j++ {
		if word(toks, j, "arguments") {
			return false
		}
		// A spread `...p` copies the elements into a literal whose own exemption (AMAP-V0-384) need not
		// cover the copy's reach, so the parameter must be used without one (AMAP-V0-426).
		if word(toks, j, p) && !property(toks, j) && !objectKey(toks, j) && (!copyUse(toks, j) || next(toks, j-1, ".")) {
			return false
		}
	}
	return true
}

// reflectiveNames are names through which code can define or replace a member without spelling
// it: property descriptors, prototypes, proxies, merges, entries and code evaluation.
var reflectiveNames = map[string]bool{"defineProperty": true, "defineProperties": true, "setPrototypeOf": true,
	"__proto__": true, "__defineGetter__": true, "Reflect": true, "Proxy": true, "eval": true, "assign": true,
	"fromEntries": true, "with": true}

// dynamicMembers reports whether a file may define or replace a member under a name its tokens do
// not spell (AMAP-V0-424): a computed class member or object key, an assignment (also `||=`,
// `&&=`, `??=`) to a computed member `E[k]` or through an assignment pattern, a reflectiveNames
// name, or a `Function(...)` call. An empty `[]` or `{}` before `=` is no such write. methodArgument
// then cannot prove that the method it read is the one a call reaches.
func dynamicMembers(toks []token) bool {
	for k, t := range toks {
		switch {
		case t.kind == tokIdent && (reflectiveNames[t.text] || t.text == "Function" && next(toks, k+1, "(")):
			return true
		case isPunct(t, "["):
			o := enclosingOpen(toks, k)
			if o >= 0 && isPunct(toks[o], "{") && (next(toks, k-1, "{", ",") ||
				classBody(toks, o) && !next(toks, k-1, "=") && !next(toks, k+1, "]")) {
				return true
			}
		case (isPunct(t, "]") || isPunct(t, "}")) && !next(toks, k-1, "[", "{"):
			// An empty `[]` or `{}` names no member: it is a type (`const ns: Kind[] = ...`,
			// AMAP-V0-426) or an empty pattern that assigns nothing.
			a := k + 1
			for next(toks, a, "|", "&", "?") {
				a++
			}
			if !next(toks, a, "=") || next(toks, a+1, "=", ">") {
				continue
			}
			o := enclosingOpen(toks, k)
			if o >= 0 && !(word(toks, o-1, "const") || word(toks, o-1, "let") || word(toks, o-1, "var")) {
				return true
			}
		}
	}
	return false
}

// methodHead reports whether the name toks[k], directly in the class body opened at toks[class],
// begins a plain method member: at a member start, or after only access modifiers or `async`.
// `static`, `get`, `set`, a generator and a field are not plain methods of the instance.
func methodHead(toks []token, k, class int) bool {
	s := k
	for word(toks, s-1, "public") || word(toks, s-1, "private") || word(toks, s-1, "protected") ||
		word(toks, s-1, "override") || word(toks, s-1, "async") {
		s--
	}
	return memberStart(toks, s, class)
}

// thisMethodBody returns the `{` of the class method body in which `this` at the call opened by
// toks[o] is the instance, or -1. It walks out through arrows, control blocks, plain blocks,
// parentheses, arrays and object literals; any other function body (a `function`, an object
// method, a getter of an object) rebinds `this` and fails.
func thisMethodBody(toks []token, o int) int {
	for k := enclosingOpen(toks, o); k >= 0; k = enclosingOpen(toks, k) {
		if !isPunct(toks[k], "{") || k == 0 {
			continue
		}
		p := k - 1
		q := p // step back over a method's return type `): T {`
		for q > 0 && (toks[q].kind == tokIdent || typePunct(toks[q]) || isPunct(toks[q], ".")) {
			q--
		}
		if q < p && next(toks, q, ":") && next(toks, q-1, ")") {
			p = q - 1
		}
		if !next(toks, p, ")") {
			continue // an arrow body, block, class body or object literal
		}
		po := enclosingOpen(toks, p)
		if po > 0 && (word(toks, po-1, "if") || word(toks, po-1, "for") || word(toks, po-1, "while") ||
			word(toks, po-1, "switch") || word(toks, po-1, "catch") || word(toks, po-1, "with") || word(toks, po-1, "await")) {
			continue
		}
		if po < 1 || toks[po-1].kind != tokIdent || word(toks, po-1, "function") || word(toks, po-2, "function") {
			return -1
		}
		c := enclosingOpen(toks, k)
		if c < 1 || !isPunct(toks[c], "{") || !classBody(toks, c) {
			return -1
		}
		return k
	}
	return -1
}

// classBody reports whether the `{` at toks[open] opens a class body: `class Name ... {`, after
// heritage clauses read by classBodyOpen.
func classBody(toks []token, open int) bool {
	for c := open - 1; c >= 0 && (toks[c].kind == tokIdent || isPunct(toks[c], ".") || isPunct(toks[c], ",") ||
		isPunct(toks[c], "<") || isPunct(toks[c], ">")); c-- {
		if word(toks, c, "class") {
			return classBodyOpen(toks, c+1) == open
		}
	}
	return false
}

// otherServiceGets exempts only the exact get form and its parameter declaration when every
// reference in that function is such a get. A dynamic/same-name get, mutation, alias or other
// injector operation remains blocking (AMAP-V0-421).
func (f *constFile) otherServiceGets(name string) map[int]bool {
	toks := f.toks
	allowed := map[int]bool{}
	for i, tok := range toks {
		if tok.kind != tokIdent || tok.text != "$injector" && tok.text != "injector" {
			continue
		}
		if f.exactOtherGet(i, name) {
			allowed[i] = true
		}
	}
	for i, tok := range toks {
		if tok.kind != tokIdent || tok.text != "$injector" && tok.text != "injector" {
			continue
		}
		pf, ok := f.paramFunc(i, tok.text)
		if !ok {
			continue
		}
		all := true
		for j := pf.fn.from; j < pf.fn.to; j++ {
			if word(toks, j, tok.text) && !allowed[j] {
				all = false
			}
		}
		if all && !f.hidden[tok.text] {
			allowed[i] = true
		}
	}
	return allowed
}

func (f *constFile) exactOtherGet(i int, name string) bool {
	toks := f.toks
	if property(toks, i) || !next(toks, i+1, ".") || !word(toks, i+2, "get") || !next(toks, i+3, "(") ||
		i+5 >= len(toks) || !literal(toks[i+4]) || toks[i+4].text == name || !next(toks, i+5, ")") {
		return false
	}
	// A whole return, discarded expression, or const whose uses are all proven reads. Passing
	// the result, chaining and computed/member writes are intentionally not exempt.
	end := i + 6
	if end != len(toks) && !next(toks, end, ";", "}") {
		return false
	}
	if i >= 3 && word(toks, i-3, "const") && toks[i-2].kind == tokIdent && next(toks, i-1, "=") {
		binding := toks[i-2].text
		if f.exports[binding] || f.hidden[binding] {
			return false
		}
		for j, tok := range toks {
			if j == i-2 || tok.kind != tokIdent || tok.text != binding || property(toks, j) || objectKey(toks, j) {
				continue
			}
			if read, member := readUse(toks, j); !read || !member {
				return false
			}
		}
		return true
	}
	return i == 0 || next(toks, i-1, ";", "{") || word(toks, i-1, "return")
}
