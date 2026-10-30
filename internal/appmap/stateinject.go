package appmap

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// A router function often receives its state-name table by AngularJS injection rather than an
// import: `const Routes = ($stateProvider, Names) => $stateProvider.state(Names.HOME, ...)`, with
// the table registered elsewhere as `.constant('Names', T)`. AMAP-V0-021..023 resolve such a read
// only when the router file proves `Names` is an injected parameter, the manifest's di_constants
// scope holds exactly one registration of 'Names', and T is a table the AMAP-V0-016 rules read
// whole. Anything else resolves nothing, so the caller keeps its UNKNOWN reason.

const (
	maxDIScopePaths = 16
	maxDIScopeFiles = 20000
	maxDIScopeBytes = 128 << 20
	// maxImportClosure bounds the files an injected table's examined closure may hold.
	maxImportClosure = 256
	// maxBarrelDepth bounds the `export ... from` hops a router's own table import follows
	// (AMAP-V0-208).
	maxBarrelDepth = 8
	// maxBarrelVisits bounds the barrels one re-export walk reads (AMAP-V0-208).
	maxBarrelVisits = 256
)

// diScope is the manifest's di_constants scope: its sources, read once on first use.
type diScope struct {
	sources []string // sorted tracked web sources under the scope paths
	poison  bool     // a registration the reader cannot attribute: every injected name is unprovable
	// poisonAt names the first source that poisoned the scope, `path` or `path:line` (AMAP-V0-202).
	poisonAt string
	scanned  bool
	regs     map[string][]diReg
	err      error // a bound breached while reading the scope
}

// diReg is one `.constant('NAME', T)` registration.
type diReg struct {
	file        *constFile
	ident       string     // T as an identifier
	at          int        // T's token index
	name        int        // the registered name's token index
	table       *constDecl // T as an object literal argument
	first, last int        // the call's lines
	bad         bool       // T is neither, or the name came from an object-map registration
}

// diBinding records how a router file binds one name: whether every binding is an injected
// parameter, and the bodies of the functions that receive it.
type diBinding struct {
	ok  bool
	fns []diFunc
	// shadows are the other functions with a plain parameter of the name (a nested controller's
	// own injection, a callback): a read inside one is not the router's injection (AMAP-V0-204).
	shadows []diFunc
	why     string // when not ok, the router-side refusal an injected read names (AMAP-V0-202)
}

type diFunc struct {
	from, to    int   // body tokens [from, to)
	first, last int   // the parameter list's lines
	annot       []int // the string tokens of its annotation
	// mismatch, on a shadow, names the annotation that injects the function but not this
	// parameter by name and position (AMAP-V0-207): a read inside it never guesses which value.
	mismatch string
}

// newDIScope collects the indexed web sources under the scope paths (AMAP-V0-021).
func newDIScope(ix *contextindex.Index, paths []string) (*diScope, error) {
	if ix == nil {
		return nil, invalidManifest("di_constants needs the revision index")
	}
	s := &diScope{regs: map[string][]diReg{}}
	inScope := func(p string) bool {
		for _, d := range paths {
			if under(p, d) {
				return true
			}
		}
		return false
	}
	for p := range ix.Sources {
		if webSuffix[strings.ToLower(path.Ext(p))] && inScope(p) {
			s.sources = append(s.sources, p)
		}
	}
	sort.Strings(s.sources)
	for _, d := range paths {
		found := false
		for _, p := range s.sources {
			if under(p, d) {
				found = true
				break
			}
		}
		if !found {
			return nil, invalidManifest("di_constants path %q holds no indexed JavaScript or TypeScript source", d)
		}
	}
	if len(s.sources) > maxDIScopeFiles {
		return nil, bound(fmt.Sprintf("more than %d di_constants sources", maxDIScopeFiles))
	}
	for _, ex := range ix.Exclusions {
		if webSuffix[strings.ToLower(path.Ext(ex.Path))] && inScope(ex.Path) {
			s.poisoned(ex.Path) // an unread source could register any name
		}
	}
	return s, nil
}

// poisoned marks the scope unprovable, keeping the first source named as the reason.
func (s *diScope) poisoned(at string) {
	if !s.poison {
		s.poison, s.poisonAt = true, at
	}
}

// registrations reads the scope's `.constant(...)` calls once and returns those of name.
func (t *constTable) registrations(name string) []diReg {
	s := t.di
	if !s.scanned {
		s.scanned = true
		total := 0
		for _, p := range s.sources {
			text, valid, loaded := t.ix.Sources[p].Text()
			if !valid || !loaded || len(text) > maxRouterBytes {
				s.poisoned(p)
				continue
			}
			if total += len(text); total > maxDIScopeBytes {
				s.err = bound(fmt.Sprintf("di_constants sources exceed %d bytes", maxDIScopeBytes))
				return nil
			}
			if !strings.Contains(text, "constant") {
				continue
			}
			if f := t.file(p); f != nil {
				s.collect(f)
			} else {
				s.poisoned(p)
			}
		}
	}
	return s.regs[name]
}

// collect records every `.constant(...)` call of one scope file.
func (s *diScope) collect(f *constFile) {
	toks := f.toks
	for j := 1; j+1 < len(toks); j++ {
		if toks[j].kind != tokIdent || toks[j].text != "constant" || !isPunct(toks[j-1], ".") || !isPunct(toks[j+1], "(") {
			continue
		}
		at := fmt.Sprintf("%s:%d", f.entry.path, toks[j].line)
		end := closeParen(toks, j+1)
		if !balanced(toks, j+1, end) {
			s.poisoned(at) // an unclosed call: its arguments are unknown
			continue
		}
		a := toks[j+2]
		switch {
		case j >= 2 && toks[j-2].kind == tokIdent && (toks[j-2].text == "_" || toks[j-2].text == "lodash") && (j < 3 || !isPunct(toks[j-3], ".")):
			// lodash's constant function, whatever its arguments: not a registration
		case !isPunct(toks[end], ")"):
			s.poisoned(at) // a mismatched closing bracket
		case isPunct(a, ")"):
			// no argument: registers nothing
		case literal(a) && next(toks, j+3, ","):
			r := diReg{file: f, name: j + 2, first: toks[j].line, last: toks[end].line}
			k := j + 4
			switch {
			case k < end && toks[k].kind == tokIdent && wholeCallArg(toks, k+1, end):
				r.ident, r.at = toks[k].text, k
			case k < end && isPunct(toks[k], "{"):
				if v, after := parseValue(toks, k); wholeCallArg(toks, after, end) {
					r.table = objectMembers(v)
				} else {
					r.bad = true
				}
			default:
				r.bad = true
			}
			s.regs[a.text] = append(s.regs[a.text], r)
		case literal(a) && wholeCallArg(toks, j+3, end):
			s.regs[a.text] = append(s.regs[a.text], diReg{file: f, first: toks[j].line, last: toks[end].line, bad: true}) // a value the reader cannot see
		case isPunct(a, "{"):
			// An object map registers each key; its values are not read.
			v, after := parseValue(toks, j+2)
			if v.kind != "object" || !wholeCallArg(toks, after, end) {
				s.poisoned(at)
				continue
			}
			for _, p := range v.obj {
				if p.key == "" {
					s.poisoned(at)
					continue
				}
				s.regs[p.key] = append(s.regs[p.key], diReg{file: f, first: toks[j].line, last: toks[end].line, bad: true})
			}
		default:
			s.poisoned(at) // a computed name could register anything
		}
	}
}

// diReach are the names through which code reaches an injected value by a name it may compute: the
// injector service and the decorator hooks.
var diReach = map[string]bool{"$injector": true, "$provide": true, "injector": true, "decorator": true}

// diQuiet reports whether f names the constant registered as name nowhere but where the reader
// checks it (AMAP-V0-025). f fails on any of:
//   - a string or template equal to name, a template whose literal text holds it, or a string
//     the lexer cannot decode (a regular expression is not a string), except the registration's
//     own name token reg (or -1), a lodash `_.constant(...)` name, and an annotation entry of a
//     function injection(name) accepts;
//   - a use of the identifier name other than as an object key, unless injection(name) accepts
//     every use or onlyRead(name, allow) proves it only read;
//   - a use of the identifier `_name_` other than as an object key: AngularJS strips matching
//     surrounding underscores from an implicit parameter name and injects name;
//   - an injector or decorator token under any name (diReach);
//   - an annotation the reader cannot read: a `$inject`, or an inline array whose names are not
//     exact strings;
//   - an injector or decorator name inside a template substitution, or any substitution that is
//     not one name, member chain or literal (see substOpaque), which alone may run code;
//   - an `angular.module` dependency list holding a name, unless exempt (see bindsAngular).
//
// `.run(['Name', function (s) { s.K = v }])`, `$injector.get('Name').K = v` or
// `function (Name) { Name.K = v }` would otherwise write the table the router reads. A file the
// reader does not examine is not seen (see examined).
//
// It returns "" when f is quiet, otherwise every distinct `line:construct`, separated by `;`
// (`unread` alone for an unread file). The closure prefixes each item with its path (AMAP-V0-422).
func (f *constFile) diQuiet(name string, reg, allow int, exempt bool, arrays map[int]bool, proof string) string {
	key := quietKey{name, reg, allow, exempt, proof}
	if v, ok := f.quiet[key]; ok {
		return v
	}
	if f.quiet == nil {
		f.quiet = map[quietKey]string{}
	}
	v := f.quietFor(name, reg, allow, exempt, arrays)
	f.quiet[key] = v
	return v
}

// quietFor is diQuiet without the memo. An opaque substitution (hiddenCode) is lexed and its tokens
// pass the same checks as the file's own (substQuiet), so one that names neither the injector nor
// the constant, by any string or computed annotation, leaves the file quiet (AMAP-V0-373); without
// one, a name inside a substitution is only read (AMAP-V0-204).
func (f *constFile) quietFor(name string, reg, allow int, exempt bool, arrays map[int]bool) string {
	var failures []string
	seen := map[string]bool{}
	at := func(line int, what string) {
		v := strconv.Itoa(line) + ":" + what
		if !seen[v] {
			seen[v] = true
			failures = append(failures, v)
		}
	}
	if f.unlisted {
		return "unread"
	}
	if !f.annotations() {
		for _, line := range f.annotBadLines {
			at(line, "$inject")
		}
	}
	for _, line := range computedAnnotations(f.toks, exempt, arrays) {
		at(line, "computed-annotation")
	}
	for _, h := range slices.Sorted(maps.Keys(diReach)) {
		if f.hidden[h] {
			for _, line := range f.hiddenAt[h] {
				at(line, h)
			}
		}
	}
	if f.hiddenCode {
		for _, t := range f.toks {
			if t.kind == tokTemplate && t.subst && !substQuiet(t.code, name, exempt, 0) {
				at(t.line, "template-substitution")
			}
		}
	}
	b := f.injection(name)
	accepted := map[int]bool{reg: true}
	for _, fn := range append(slices.Clone(b.fns), b.shadows...) {
		for _, k := range fn.annot {
			accepted[k] = true
		}
	}
	toks := f.toks
	var used []int
	otherGets := f.otherServiceGets(name)
	for i, t := range toks {
		switch {
		case t.kind == tokIdent && diReach[t.text]:
			if !otherGets[i] {
				at(t.line, t.text)
			}
		case t.kind == tokIdent && t.text == "_"+name+"_" && !property(toks, i) && !objectKey(toks, i):
			at(t.line, t.text) // AngularJS injects an implicit parameter `_name_` as name; its uses are not checked
		case t.kind == tokIdent:
			if t.text == name && !f.skip[i] && !property(toks, i) && !objectKey(toks, i) {
				used = append(used, t.line)
			}
		case (t.kind == tokString || t.kind == tokTemplate) && !accepted[i] && !lodashName(toks, i) &&
			(t.inexact && t.text != "" || diReach[t.text] || t.text == name || t.subst && strings.Contains(t.text, name)):
			at(t.line, "string")
		}
	}
	if len(used) > 0 && !b.ok && !f.onlyRead(name, allow) {
		for _, line := range used {
			at(line, name)
		}
	}
	return strings.Join(failures, ";")
}

// objectKey reports whether toks[i] is a key of an object literal, `{ K: v }`, which names no
// binding.
func objectKey(toks []token, i int) bool {
	if i == 0 || !isPunct(toks[i-1], "{") && !isPunct(toks[i-1], ",") || !next(toks, i+1, ":") || next(toks, i+2, ":") {
		return false
	}
	o := enclosingOpen(toks, i)
	return o >= 0 && isPunct(toks[o], "{")
}

// computedAnnotationIn returns the line of the first array literal of toks that may be an inline annotation
// whose names the reader cannot read: when any element but the last is not one exact string, every
// element must be a value that cannot be a function -- an exact string or number, `true`, `false`,
// `null`, an object or an array -- whatever its position. A function or arrow, a name or member
// that may hold one (`undefined`, `NaN` and `Infinity` are names that can be shadowed), a call that
// may return one, a spread or a hole fails. A spread fails in any position, alone included: it may
// expand to a whole annotation. One trailing comma ends the list without an element. The
// dependency list of `angular.module('name', [...])` when exempt (see bindsAngular) and a binding
// pattern (`const [a, b] = v`) are no annotation; neither is an array outside a call's arguments
// whose last element cannot be a function, which the injector refuses to invoke (AMAP-V0-384), nor
// one only a const binding holds that reaches no call (see constHeld, AMAP-V0-385). Any other
// array holding a name fails closed. A plain-ended array a binding or member is assigned (`= [...]`)
// is not exempt by its last element: a later `.pop()` and `.push(fn)` may complete it.
func computedAnnotationIn(toks []token, exempt bool) int {
	if lines := computedAnnotations(toks, exempt, nil); len(lines) > 0 {
		return lines[0]
	}
	return 0
}

// computedAnnotations retains every blocking array. Proven non-injector uses are supplied by
// the closure's arrayReach; without that evidence (including substitutions) the scan stays closed.
func computedAnnotations(toks []token, exempt bool, quiet map[int]bool) []int {
	var lines []int
	for i := range toks {
		if !isPunct(toks[i], "[") || quiet[i] || exempt && moduleDeps(toks, i) || i > 0 && (word(toks, i-1, "const") || word(toks, i-1, "let") || word(toks, i-1, "var")) ||
			decoratorMetadata(toks, i) || parameterPattern(toks, i) {
			continue
		}
		end := closeParen(toks, i)
		if next(toks, end+1, "=") && !next(toks, end+2, "=") && !next(toks, end+2, ">") {
			continue // an assignment pattern
		}
		var elems [][2]int
		for k := i + 1; k < end; {
			e := min(skipValue(toks, k), end)
			elems = append(elems, [2]int{k, e})
			k = e + 1
		}
		if n := len(elems); n > 0 && (plainElement(toks, elems[n-1][0], elems[n-1][1]) && !callArgument(toks, i) && !isPunct(toks[max(i-1, 0)], "=") ||
			!functionLiteral(toks, elems[n-1][0], elems[n-1][1]) && constHeld(toks, i, end)) {
			continue
		}
		computed, plain := false, true
		for n, el := range elems {
			if n < len(elems)-1 && (el[1] != el[0]+1 || !literal(toks[el[0]])) || isPunct(toks[el[0]], ".") {
				computed = true // a spread may supply every name and the function
			}
			plain = plain && plainElement(toks, el[0], el[1])
		}
		if computed && !plain {
			lines = append(lines, toks[i].line)
		}
	}
	return lines
}

// parameterPattern reports whether the `[` at toks[i] lies in an array binding pattern that is a
// whole parameter of an arrow or a `function` (AMAP-V0-425): the outermost `[` follows the
// parameter list's `(` or a `,` in it and its `]` is followed by `,` or `)`; the pattern holds only
// names, holes, rest dots and nested `[...]`; and the list's `)` is followed by a glued `=>`, or its
// `(` follows `function` or `function NAME`. Such a pattern binds elements and is no array value. A
// default (`=`), a type (`:`), an object pattern or a rest parameter `...[a]` is not read here.
func parameterPattern(toks []token, i int) bool {
	o := i
	for p := enclosingOpen(toks, o); p >= 0 && isPunct(toks[p], "["); p = enclosingOpen(toks, p) {
		o = p
	}
	po, end := enclosingOpen(toks, o), closeParen(toks, o)
	if po < 1 || !isPunct(toks[po], "(") || !next(toks, o-1, "(", ",") || !next(toks, end+1, ",", ")") {
		return false
	}
	for k := o + 1; k < end; k++ {
		if toks[k].kind != tokIdent && !next(toks, k, ",", ".", "[", "]") {
			return false
		}
	}
	c := closeParen(toks, po)
	return next(toks, c+1, "=") && next(toks, c+2, ">") && toks[c+2].glued ||
		word(toks, po-1, "function") && !property(toks, po-1) ||
		toks[po-1].kind == tokIdent && word(toks, po-2, "function") && !property(toks, po-2)
}

// callArgument reports whether the array at toks[i] lies directly in parentheses, as a call's
// argument may: the callee may complete it into an annotation before the injector reads it.
func callArgument(toks []token, i int) bool {
	o := enclosingOpen(toks, i)
	return o >= 0 && isPunct(toks[o], "(")
}

// functionLiteral reports whether toks[k:e] may be a function literal: it starts with `function`,
// `async` or `class`, or holds an arrow `=>` anywhere.
func functionLiteral(toks []token, k, e int) bool {
	if word(toks, k, "function") || word(toks, k, "async") || word(toks, k, "class") {
		return true
	}
	for j := k; j+1 < e; j++ {
		if isPunct(toks[j], "=") && isPunct(toks[j+1], ">") {
			return true
		}
	}
	return false
}

// constHeld reports whether the array toks[i:end+1] is the whole initializer of an unexported
// `const NAME = [...]`, ended by `;`, `}`, the end of the file or a name or decorator on a later
// line, whose every use is the whole value of a class decorator's metadata property (see
// decoratorMetadata) or a spread `...NAME` in an array literal, which is then judged by itself. No
// call, export, shorthand property, other binding or template names NAME, so the array reaches no
// injector (AMAP-V0-385).
func constHeld(toks []token, i, end int) bool {
	if i < 3 || !isPunct(toks[i-1], "=") || toks[i-2].kind != tokIdent || !word(toks, i-3, "const") || word(toks, i-4, "export") {
		return false
	}
	if a := end + 1; a < len(toks) && !next(toks, a, ";", "}") && (toks[a].kind != tokIdent && !isPunct(toks[a], "@") || toks[a].line == toks[end].line) {
		return false
	}
	name := toks[i-2].text
	for j, t := range toks {
		switch {
		case t.kind == tokTemplate && strings.Contains(t.code, name):
			return false
		case j == i-2 || t.kind != tokIdent || t.text != name || property(toks, j) || objectKey(toks, j):
		case j >= 3 && isPunct(toks[j-1], ".") && isPunct(toks[j-2], ".") && isPunct(toks[j-3], ".") && next(toks, j+1, ",", "]") &&
			enclosingOpen(toks, j) >= 0 && isPunct(toks[enclosingOpen(toks, j)], "["):
		case j > 0 && isPunct(toks[j-1], ":") && next(toks, j+1, ",", "}") && decoratorMetadata(toks, j):
		default:
			return false
		}
	}
	return true
}

// decoratorMetadata reports whether the array at toks[i] is the value of a property `K: [...]` of
// the object literal that is the whole first argument of a class decorator `@D({...})` or
// `@A.B({...})`: one followed, past any further decorators, by `export`, `default`, `abstract` or
// `class`. Decorator metadata is read by its framework, never by the AngularJS injector, so it is
// no inline annotation (AMAP-V0-373); an array nested in it is judged by itself.
func decoratorMetadata(toks []token, i int) bool {
	if i < 2 || !isPunct(toks[i-1], ":") || !objectKey(toks, i-2) {
		return false
	}
	o := enclosingOpen(toks, i)
	if o < 2 || !isPunct(toks[o], "{") || !isPunct(toks[o-1], "(") || !next(toks, closeParen(toks, o)+1, ")") {
		return false
	}
	k := o - 2 // the decorator's dotted name, back to its `@`
	for k >= 2 && toks[k].kind == tokIdent && isPunct(toks[k-1], ".") {
		k -= 2
	}
	if k < 1 || toks[k].kind != tokIdent || !isPunct(toks[k-1], "@") {
		return false
	}
	for j := closeParen(toks, o-1) + 1; j < len(toks); j++ {
		switch {
		case isPunct(toks[j], "@"): // another decorator: its name, then its arguments
			for j++; j+1 < len(toks) && toks[j].kind == tokIdent && isPunct(toks[j+1], "."); j += 2 {
			}
			if next(toks, j+1, "(") {
				j = closeParen(toks, j+1)
			}
		case word(toks, j, "export") || word(toks, j, "default") || word(toks, j, "abstract"):
		default:
			return word(toks, j, "class")
		}
	}
	return false
}

// maxSubstDepth bounds the template nesting substQuiet reads; a deeper one is not quiet.
const maxSubstDepth = 8

// substQuiet reports whether every substitution of a template's code (from its first `${` to its
// closing backtick) provably neither names name, its implicit `_name_` nor an injector or
// decorator (diReach), as an identifier or a string, nor holds an inline annotation the reader
// cannot read (computedAnnotationIn) or a nested template that is not quiet itself
// (AMAP-V0-373). A substitution whose end the scan cannot place, or that the lexer doubts, is not
// quiet.
func substQuiet(code, name string, exempt bool, depth int) bool {
	if depth > maxSubstDepth {
		return false
	}
	for i := 0; i < len(code); {
		switch {
		case code[i] == '\\':
			i += 2
			continue
		case code[i] == '`':
			return true
		case code[i] != '$' || i+1 >= len(code) || code[i+1] != '{':
			i++
			continue
		}
		end := substEnd(code, i+2)
		if end < 0 {
			return false
		}
		toks, _ := lexJS(code[i+2 : end])
		for _, t := range toks {
			switch {
			case t.unsure || t.inexact:
				return false
			case t.kind == tokIdent && (diReach[t.text] || t.text == name || t.text == "_"+name+"_"):
				return false
			case t.kind == tokString && (diReach[t.text] || strings.Contains(t.text, name)):
				return false
			case t.kind == tokTemplate && (strings.Contains(t.text, name) || t.subst && !substQuiet(t.code, name, exempt, depth+1)):
				return false
			}
		}
		if computedAnnotationIn(toks, exempt) > 0 {
			return false
		}
		i = end + 1
	}
	return false // no closing backtick
}

// substEnd returns the index of the `}` closing a substitution whose body starts at code[i],
// stepping over strings, nested templates and brackets, or -1.
func substEnd(code string, i int) int {
	depth := 0
	for i < len(code) {
		switch c := code[i]; c {
		case '\'', '"':
			_, _, closed, end := readQuoted(code, i)
			if !closed {
				return -1
			}
			i = end
			continue
		case '`':
			j := i + 1
			for j < len(code) && code[j] != '`' {
				switch {
				case code[j] == '\\':
					j += 2
				case code[j] == '$' && j+1 < len(code) && code[j+1] == '{':
					if j = substEnd(code, j+2); j < 0 {
						return -1
					}
					j++
				default:
					j++
				}
			}
			if j >= len(code) {
				return -1
			}
			i = j + 1
			continue
		case '/':
			return -1 // a comment or a regular expression may hold any brace
		case '{', '(', '[':
			depth++
		case ')', ']':
			depth--
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		}
		i++
	}
	return -1
}

// moduleDeps reports whether toks[i] opens the dependency list of `angular.module('name', [...])`,
// with `angular` itself an identifier, not a property. Any other method named module is an
// ordinary call whose array arguments may be annotations.
func moduleDeps(toks []token, i int) bool {
	return i >= 6 && isPunct(toks[i-1], ",") && literal(toks[i-2]) && isPunct(toks[i-3], "(") && word(toks, i-4, "module") &&
		isPunct(toks[i-5], ".") && word(toks, i-6, "angular") && !property(toks, i-6)
}

// plainElement reports whether toks[k:e] is one exact literal, `true`, `false`, `null`, an object
// or an array: a value that cannot be a function. An empty element (a hole) is not.
func plainElement(toks []token, k, e int) bool {
	switch {
	case e == k+1:
		t := toks[k]
		return literal(t) || t.kind == tokNumber || word(toks, k, "true") || word(toks, k, "false") || word(toks, k, "null")
	case e > k && (isPunct(toks[k], "{") || isPunct(toks[k], "[")):
		return closeParen(toks, k) == e-1
	}
	return false
}

// quiet is diQuiet for one examined file; the registering file may hold its own registration.
// exempt grants the `angular.module` dependency-list exemption (see bindsAngular).
func (t *constTable) quiet(f *constFile, name string, exempt bool, reach *arrayReach) string {
	reg, allow := -1, -1
	if r := t.di.regs[name]; len(r) == 1 && r[0].file.entry.path == f.entry.path {
		reg = r[0].name
		if r[0].ident != "" {
			allow = r[0].at
		}
	}
	arrays, proof := reach.quietArrays(f)
	return f.diQuiet(name, reg, allow, exempt, arrays, proof)
}

// bindsAngular reports whether f uses the name angular other than to call the global AngularJS
// object's module function: every identifier angular must be the head of `angular.module(` (the
// member module called at once) or the binding of the real AngularJS module, `import angular
// from 'angular'`, `import * as angular from 'angular'` or `const angular = require('angular');`,
// where 'angular' resolves to a package outside the repository (see external). Any other use
// counts: a parameter, a declaration, another import, a catch parameter, a destructuring target,
// a value passed on, a read or write of angular.module (`saved = angular.module`, `angular.module
// = f`), any other member (`angular.x`), a computed member (`angular['module']`) and an optional
// chain (`angular?.module`). A file may also bind or reach angular by a name no identifier shows,
// so these count too: a `with` statement (its object's keys shadow names), `eval` or `Function`
// anywhere (`new Function`, `(0, eval)`), and a string or template whose value is exactly angular
// (`{ 'angular': x }`, `window['angular']`) other than the specifier of an accepted package
// binding. A file may also reach angular through a global object or a computed name, so these
// count as well: a global object's name (globalObjects) other than a property or an object key,
// a reflective name (reflective) in any position, as an identifier or an exact string, and a
// computed member or object key whose name is not one number or string (computedName). Then
// `angular.module('x', [...])` in any examined file may call another function, so no dependency
// list is exempt.
func (t *constTable) bindsAngular(f *constFile) bool {
	toks := f.toks
	for i, tok := range toks {
		switch {
		case tok.kind == tokIdent && (tok.text == "with" && !property(toks, i) || tok.text == "eval" || tok.text == "Function"):
			return true
		case (tok.kind == tokString || tok.kind == tokTemplate) && tok.text == "angular":
			if !angularSpecifier(toks, i) {
				return true
			}
		case (tok.kind == tokIdent || literal(tok)) && reflective[tok.text],
			tok.kind == tokIdent && globalObjects[tok.text] && !property(toks, i) && !objectKey(toks, i),
			isPunct(tok, "[") && computedName(toks, i):
			return true
		case tok.kind != tokIdent || tok.text != "angular":
		case next(toks, i+1, ".") && word(toks, i+2, "module") && next(toks, i+3, "("):
		case !angularImport(toks, i) || !t.angularPackage(f):
			return true
		}
	}
	return false
}

var (
	// globalObjects are the names of the global object, through which any global, angular
	// included, can be read or replaced.
	globalObjects = map[string]bool{"window": true, "globalThis": true, "self": true, "global": true,
		"top": true, "parent": true, "frames": true}
	// reflective are the names that can define or replace a property, or a whole object's
	// behavior, without a visible member write (assign stands for Object.assign), or reach
	// Function or call a function with another this or arguments: constructor
	// (`({})['constructor']['constructor']` is Function), prototype, call, apply and bind; or reach
	// the global object or a property's value: this, Object, document, defaultView and
	// getOwnPropertyDescriptor(s). require and import need no entry: a call with a literal
	// specifier loads a closure file or names 'angular' (see angularSpecifier), and any other use
	// loads a module the reader cannot name (moduleArg, moduleLoader), which leaves the closure
	// unresolved.
	reflective = map[string]bool{"Reflect": true, "Proxy": true, "defineProperty": true,
		"defineProperties": true, "setPrototypeOf": true, "__proto__": true, "__defineGetter__": true,
		"__defineSetter__": true, "assign": true, "constructor": true, "prototype": true, "call": true,
		"apply": true, "bind": true, "this": true, "Object": true, "document": true, "defaultView": true,
		"getOwnPropertyDescriptor": true, "getOwnPropertyDescriptors": true}
	// arrayAfter is the punctuation after which a `[` can only open an array literal or pattern.
	// `{`, `,` and `;` are decided by what follows the group; after `)`, `]`, `}`, `.` (an optional
	// `?.[` or a spread) and any other punctuation a `[` counts as a computed member.
	arrayAfter = map[string]bool{"(": true, "=": true, ":": true, "[": true, "?": true, "~": true,
		"&": true, "|": true, "^": true, "*": true, "%": true, "/": true, "!": true, "+": true,
		"-": true, "<": true, ">": true}
	// arrayWords are the reserved words after which a `[` opens an array literal or pattern. The
	// contextual words of, yield and await may also be plain names, so a `[` after them counts as
	// a computed member.
	arrayWords = map[string]bool{"return": true, "typeof": true, "case": true, "in": true,
		"else": true, "do": true, "void": true, "delete": true, "throw": true, "new": true,
		"instanceof": true, "extends": true, "default": true, "const": true, "let": true, "var": true}
)

// computedName reports whether the `[` at toks[i] opens a computed member (`x[k]`, `x?.[k]`) or
// a computed object or class key (`{ [k]: v }`, `[k]() {}`, `[k] = v`) whose name is not exactly
// one number or string, or a group that does not close. After `{`, `,` or `;` the group is a key
// when a `:`, `(` or `=` follows it, so an array pattern assigned there counts too; after a name,
// a literal or a closing bracket it is a member. Empty brackets (a TypeScript `T[]` type) and
// array literals do not count.
func computedName(toks []token, i int) bool {
	end := closeParen(toks, i)
	if !isPunct(toks[end], "]") || !balanced(toks, i, end) {
		return true
	}
	if i == 0 {
		return false
	}
	switch p := toks[i-1]; {
	case next(toks, i-1, "{", ",", ";"):
		if !next(toks, end+1, ":", "(", "=") {
			return false
		}
	case p.kind == tokPunct && arrayAfter[p.text], p.kind == tokIdent && arrayWords[p.text]:
		return false
	}
	switch end - i {
	case 1:
		return false
	case 2:
		return toks[i+1].kind != tokNumber && !literal(toks[i+1])
	}
	return true
}

// angularImport reports whether toks[i] is the binding of `import angular from 'angular'`,
// `import * as angular from 'angular'` or `const angular = require('angular');`, where a `;` or
// the file's end closes the require statement.
func angularImport(toks []token, i int) bool {
	from := func(k int) bool {
		return word(toks, k, "from") && k+1 < len(toks) && literal(toks[k+1]) && toks[k+1].text == "angular"
	}
	switch {
	case word(toks, i-1, "import") && !property(toks, i-1):
		return from(i + 1)
	case word(toks, i-1, "as") && i >= 3 && isPunct(toks[i-2], "*") && word(toks, i-3, "import") && !property(toks, i-3):
		return from(i + 1)
	case word(toks, i-1, "const") && next(toks, i+1, "=") && word(toks, i+2, "require") && next(toks, i+3, "(") &&
		i+5 < len(toks) && literal(toks[i+4]) && toks[i+4].text == "angular" && isPunct(toks[i+5], ")"):
		return i+6 >= len(toks) || isPunct(toks[i+6], ";") // an operator or call after it may bind another value
	}
	return false
}

// angularSpecifier reports whether toks[k] is the module specifier of a binding angularImport
// accepts (bindsAngular checks the binding itself).
func angularSpecifier(toks []token, k int) bool {
	return k >= 2 && word(toks, k-1, "from") && word(toks, k-2, "angular") && angularImport(toks, k-2) ||
		k >= 4 && isPunct(toks[k-1], "(") && word(toks, k-2, "require") && word(toks, k-4, "angular") && angularImport(toks, k-4)
}

// angularPackage reports whether 'angular', loaded from f, resolves to a package outside the
// repository.
func (t *constTable) angularPackage(f *constFile) bool {
	if t.resolver == nil {
		return false
	}
	res := t.resolver.Resolve(f.entry.path, "angular")
	return res.State == contextindex.WebImportPackage && t.external("angular")
}

// substOpaque reports whether a template literal's raw source from its first `${` (token.code)
// holds a substitution that is not one name or member chain (`a`, `a.b`, `a?.b`) or one literal
// (a string, a number, or an untagged template whose substitutions pass the same test). A call,
// function, arrow, assignment, operator, spread, `new`, computed member or tagged template there
// may run code no token shows: an injection, an annotation or a binding.
func substOpaque(code string) bool {
	_, opaque := templateText(code, 0)
	return opaque
}

// templateText scans template text from code[i] to its closing backtick (or the end) and returns
// the index after it, and whether a substitution in it is opaque.
func templateText(code string, i int) (int, bool) {
	for i < len(code) {
		switch {
		case code[i] == '\\':
			i += 2
		case code[i] == '`':
			return i + 1, false
		case code[i] == '$' && i+1 < len(code) && code[i+1] == '{':
			var ok bool
			if i, ok = plainSubst(code, i+2); !ok {
				return i, true
			}
		default:
			i++
		}
	}
	return i, false
}

// plainSubst reads one substitution body from code[i] through its `}` and reports whether it is
// one name, member chain or literal.
func plainSubst(code string, i int) (int, bool) {
	space := func() {
		for i < len(code) && (code[i] == ' ' || code[i] == '\t' || code[i] == '\n' || code[i] == '\r') {
			i++
		}
	}
	ident := func() bool {
		if i >= len(code) || !isIdentStart(code[i]) {
			return false
		}
		for i < len(code) && isIdentPart(code[i]) {
			i++
		}
		return true
	}
	space()
	switch {
	case i >= len(code):
		return i, false
	case code[i] == '\'' || code[i] == '"':
		_, _, closed, end := readQuoted(code, i)
		if !closed {
			return end, false
		}
		i = end
	case code[i] == '`':
		end, opaque := templateText(code, i+1)
		if opaque {
			return end, false
		}
		i = end
	case code[i] >= '0' && code[i] <= '9':
		for i < len(code) && (isIdentPart(code[i]) || code[i] == '.') {
			i++
		}
	case ident():
		for {
			switch {
			case i+1 < len(code) && code[i] == '.' && isIdentStart(code[i+1]):
				i++
			case i+2 < len(code) && code[i] == '?' && code[i+1] == '.' && isIdentStart(code[i+2]):
				i += 2
			default:
				space()
				return i + 1, i < len(code) && code[i] == '}'
			}
			ident()
		}
	default:
		return i, false
	}
	space()
	return i + 1, i < len(code) && code[i] == '}'
}

// lodashName reports whether toks[i] is the first argument of lodash's `_.constant(...)`, which
// collect does not read as a registration.
func lodashName(toks []token, i int) bool {
	return i >= 4 && isPunct(toks[i-1], "(") && word(toks, i-2, "constant") && isPunct(toks[i-3], ".") &&
		(word(toks, i-4, "_") || word(toks, i-4, "lodash")) && !property(toks, i-4)
}

// examined reports whether every file of the closure of roots (see closure) is diQuiet for inj,
// and whether every one outside chain that may reach decl holds no binding of the table decl
// declares under names, or one of a module that may hold it, that is not provably only read
// (bindingsRead, as for the chain's own files): a module any examined file loads, by any import
// form and at any depth, runs before the router reads the table and may inject or write it. A
// closure the reader cannot follow fails. Without decl the closure may hold at most limit files;
// with decl it may hold maxReachFiles, and limit bounds only the files whose import graph may
// reach decl (reaching), since a module whose every import is followed and none reaches decl
// cannot bind decl's table at all (AMAP-V0-369). Each file stays examined for injection.
func (t *constTable) examined(roots, chain []*constFile, decl *constFile, inj string, names ...string) bool {
	bound := t.limit()
	if decl != nil {
		bound = maxReachFiles
	}
	files, why := t.closure(roots, bound)
	if why != "" {
		t.bind = why // the closure hop that failed (AMAP-V0-206)
		return false
	}
	var reach map[*constFile]bool
	if decl != nil {
		if reach = t.reaching(files, decl); len(reach) > t.limit() {
			t.bind = fmt.Sprintf("closure-limit:%d", t.limit())
			return false
		}
	}
	exempt := true
	for _, h := range files {
		exempt = exempt && !t.bindsAngular(h)
	}
	var failures []string
	if t.arrayUses == nil || !slices.Equal(t.arrayUses.files, files) {
		t.arrayUses = &arrayReach{table: t, files: files, proofs: map[*constFile]arrayProof{}, names: map[arrayName]bool{}}
	}
	for _, h := range files {
		if why := t.quiet(h, inj, exempt, t.arrayUses); why != "" {
			for _, item := range strings.Split(why, ";") {
				failures = append(failures, "closure-not-quiet:"+h.entry.path+":"+item)
			}
		}
		if decl != nil && reach[h] && !slices.Contains(chain, h) && !t.bindingsRead(h, decl, "", names...) {
			failures = append(failures, t.bind)
		}
	}
	if len(failures) > 0 {
		t.bind = strings.Join(failures, ";")
		return false
	}
	return true
}

// limit is the manifest's di_closure_limit, or maxImportClosure when it declares none
// (AMAP-V0-369).
func (t *constTable) limit() int {
	if t.closureLimit > 0 {
		return t.closureLimit
	}
	return maxImportClosure
}

// reaching returns the files of a followed closure (see closure) whose import graph may reach
// decl: decl, every file the reader does not read item by item (unlisted or opaque), and every
// file that loads one of them, transitively, through any module closure follows (AMAP-V0-369).
func (t *constTable) reaching(files []*constFile, decl *constFile) map[*constFile]bool {
	in := map[string]*constFile{}
	for _, h := range files {
		in[h.entry.path] = h
	}
	loaders := map[*constFile][]*constFile{}
	out := map[*constFile]bool{}
	var queue []*constFile
	for _, x := range files {
		if x == decl || x.unlisted || x.opaque {
			out[x] = true
			queue = append(queue, x)
		}
		if t.resolver == nil {
			continue
		}
		for _, m := range x.modules() {
			if res := t.resolver.Resolve(x.entry.path, m); res.State == contextindex.WebImportRepository {
				if y := in[res.Target]; y != nil {
					loaders[y] = append(loaders[y], x)
				}
			}
		}
	}
	for len(queue) > 0 {
		y := queue[0]
		queue = queue[1:]
		for _, x := range loaders[y] {
			if !out[x] {
				out[x] = true
				queue = append(queue, x)
			}
		}
	}
	return out
}

// codeSuffix are the extensions of the JavaScript and TypeScript modules the web resolver reaches,
// declaration files included; assetSuffix are modules that run no code (stylesheets, templates,
// JSON, images and fonts). The closure reads the first and skips the second; any other extension
// fails it.
var (
	codeSuffix  = map[string]bool{".ts": true, ".tsx": true, ".mts": true, ".cts": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true}
	assetSuffix = map[string]bool{".css": true, ".scss": true, ".sass": true, ".less": true, ".styl": true, ".html": true, ".htm": true,
		".json": true, ".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".txt": true, ".md": true}
)

// closure returns roots (nil skipped) and every repository file they load, transitively, through
// any import form: a static, side-effect (`import './x'`), type, namespace or dynamic import, a
// `require`, an unread import item, or a re-export. ok is false when one names a module the
// reader cannot follow: a name that is not an exact literal, an unresolved specifier, a package
// that may be repository code (see external), or a repository source the reader cannot read; or
// when the closure holds more than bound files. A module of a known asset extension
// runs no code and is skipped; one of any extension the reader does not know fails. A failure
// returns why, naming the file and module that stopped the closure (AMAP-V0-206).
func (t *constTable) closure(roots []*constFile, bound int) ([]*constFile, string) {
	var out []*constFile
	seen := map[string]bool{}
	for _, r := range roots {
		if r != nil && !seen[r.entry.path] {
			seen[r.entry.path] = true
			out = append(out, r)
		}
	}
	if t.resolver == nil {
		return out, ""
	}
	for n := 0; n < len(out); n++ {
		x := out[n]
		for i, m := range x.modules() {
			if m == "" { // spaces come first in modules
				return nil, "closure-module-not-literal:" + x.spaceAt(i)
			}
			res := t.resolver.Resolve(x.entry.path, m)
			switch {
			case res.State == contextindex.WebImportPackage && t.external(m):
				continue
			case res.State != contextindex.WebImportRepository:
				return nil, fmt.Sprintf("closure-%s:%s:%s", res.State, x.entry.path, m)
			case seen[res.Target]:
				continue
			}
			switch ext := strings.ToLower(path.Ext(res.Target)); {
			case assetSuffix[ext]:
				continue
			case !codeSuffix[ext]:
				return nil, "closure-unknown-extension:" + res.Target // a module the reader cannot tell runs no code
			}
			h := t.file(res.Target)
			switch {
			case h == nil:
				return nil, "closure-unreadable:" + res.Target
			case len(out) >= bound:
				return nil, fmt.Sprintf("closure-limit:%d", bound)
			}
			seen[res.Target] = true
			out = append(out, h)
		}
	}
	return out, ""
}

// modules lists every module name f loads: its import bindings', namespace, dynamic and `require`
// modules (spaces), and each exact literal after `from` or a statement-level `import`.
func (f *constFile) modules() []string {
	mods := append([]string(nil), f.spaces...)
	for _, local := range slices.Sorted(maps.Keys(f.imports)) { // a stable closure order (AMAP-V0-196)
		mods = append(mods, f.imports[local].module)
	}
	for _, r := range f.reexports {
		mods = append(mods, r.module)
	}
	for i, tok := range f.toks {
		if i > 0 && literal(tok) && (word(f.toks, i-1, "from") || word(f.toks, i-1, "import")) && !property(f.toks, i-1) {
			mods = append(mods, tok.text)
		}
	}
	return mods
}

// balanced reports whether toks[open..end] is a bracket group that closes at end, as
// closeParen's file-end fallback does not guarantee.
func balanced(toks []token, open, end int) bool {
	depth := 0
	for k := open; k <= end; k++ {
		if toks[k].kind != tokPunct {
			continue
		}
		switch toks[k].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
	}
	return end > open && depth == 0
}

// wholeCallArg reports whether the argument that ended at toks[i] is the call's last, so the
// registration holds exactly that value.
func wholeCallArg(toks []token, i, end int) bool {
	return i == end || (i+1 == end && isPunct(toks[i], ","))
}

// injected resolves local.member, read at token tok of the router file own, through the one
// `.constant('local', T)` registration in scope (AMAP-V0-022). When it cannot, detail names the
// refusal for the router's unknown (AMAP-V0-202).
func (t *constTable) injected(own *constFile, local, member string, tok int) (string, []Anchor, string, bool) {
	b := own.injection(local)
	if !b.ok {
		return "", nil, b.why, false
	}
	// The innermost function binding local around the read decides which value it is.
	var fn *diFunc
	for i := range b.fns {
		if tok >= b.fns[i].from && tok < b.fns[i].to && (fn == nil || b.fns[i].from > fn.from) {
			fn = &b.fns[i]
		}
	}
	for _, sh := range b.shadows {
		if tok >= sh.from && tok < sh.to && (fn == nil || sh.from > fn.from) {
			if sh.mismatch != "" {
				return "", nil, sh.mismatch, false
			}
			return "", nil, fmt.Sprintf("shadowed:%s:%d:%s", own.entry.path, sh.first, local), false
		}
	}
	if fn == nil {
		return "", nil, fmt.Sprintf("outside-injectable:%s:%d:%s", own.entry.path, own.toks[tok].line, local), false
	}
	regs := t.registrations(local)
	switch {
	case t.di.err != nil:
		return "", nil, "", false // the build refuses
	case t.di.poison:
		return "", nil, "scope-unreadable:" + t.di.poisonAt, false
	case len(regs) == 0:
		return "", nil, "no-registration:" + local, false
	case len(regs) > 1:
		at := make([]string, len(regs))
		for i, r := range regs {
			at[i] = fmt.Sprintf("%s:%d", r.file.entry.path, r.first)
		}
		return "", nil, "ambiguous-registration:" + local + ":" + strings.Join(at, ","), false
	}
	r := regs[0]
	var v, why string
	var at []Anchor
	t.hop, t.bind = "", "" // a hop belongs to this lookup only
	switch {
	case r.bad:
		why = "unreadable-registration"
	case r.table != nil:
		v, at, why = r.file.read(r.table, member, true)
		if why == "" && !t.examined([]*constFile{own, r.file}, nil, nil, local) {
			why = t.fail("registration-examined") // another injection or lookup of the name may write the table
		}
	default:
		v, at, why = t.lookup(r.file, r.ident, member, r.at, local, own)
	}
	if why != "" {
		// The one in-scope binding of local did not resolve: say why (AMAP-V0-026).
		// Every hop the chain names is carried (AMAP-V0-072, AMAP-V0-196, AMAP-V0-201).
		u := Unknown{Kind: "di-constant", Ref: local, Reason: why, Path: r.file.entry.path, Line: r.first, Detail: t.hop}
		t.diag = append(t.diag, u)
		detail := "di-constant:" + local + ":" + why
		if t.hop != "" {
			detail += ":" + t.hop
		}
		return "", nil, detail, false
	}
	if reg := spanOf(r.file.entry, r.file.data, r.first, r.last); len(at) == 0 || at[len(at)-1] != reg {
		at = append(at, reg) // an object literal argument on the call's line is carried once
	}
	return v, append(at, spanOf(own.entry, own.data, fn.first, fn.last)), "", true
}

// injection reads how the router file binds name: ok only when every use is a plain parameter of
// an injectable function or of another function (a shadow, AMAP-V0-204), or is provably only read
// (see readUse) and is a member read or a `typeof`. The write checks run before the `typeof` form narrows what counts as a read; a
// parameter is a binding position, not a use. Nothing in an unread file, and no name used inside a
// template substitution, is injected.
func (f *constFile) injection(name string) *diBinding {
	if f.inject == nil {
		f.inject = map[string]*diBinding{}
	}
	if b, seen := f.inject[name]; seen {
		return b
	}
	b := &diBinding{}
	f.inject[name] = b
	switch {
	case f.unlisted:
		b.why = "router-file-unread:" + f.entry.path
		return b
	case f.hidden[name] && f.hiddenCode:
		// A substitution that is one name, member chain or literal only reads (AMAP-V0-204); any
		// other one may write or rebind the name where no token shows it.
		b.why = "router-template-use:" + f.entry.path + ":" + name
		return b
	case !f.annotations():
		b.why = fmt.Sprintf("router-annotation:%s:%d", f.entry.path, f.annotBad)
		return b
	}
	toks := f.toks
	refuse := func(i int) *diBinding {
		b.why = fmt.Sprintf("router-binding:%s:%d:%s", f.entry.path, toks[i].line, name)
		return b
	}
	for i := range toks {
		if f.skip[i] || toks[i].kind != tokIdent || toks[i].text != name || property(toks, i) {
			continue
		}
		read, member := readUse(toks, i)
		switch {
		case member || word(toks, i-1, "typeof"):
			if !read {
				return refuse(i)
			}
		default:
			if fn, mismatch, ok := f.injectable(i, name); ok {
				b.fns = append(b.fns, fn)
			} else if pf, ok := f.paramFunc(i, name); ok {
				pf.fn.annot = inlineAnnotation(toks, pf, name)
				pf.fn.mismatch = mismatch
				b.shadows = append(b.shadows, pf.fn)
			} else {
				return refuse(i)
			}
		}
	}
	b.ok = len(b.fns) > 0
	if !b.ok {
		b.why = "not-injected:" + f.entry.path + ":" + name // no parameter binds it
		for _, sh := range b.shadows {
			if sh.mismatch != "" {
				b.why = sh.mismatch // an injectable whose annotation does not name it there (AMAP-V0-207)
				break
			}
		}
	}
	return b
}

// injectable reports whether toks[i] is a plain parameter of a function AngularJS injects: a
// top-level arrow or function, or the direct argument of `.config(...)`, alone or as the last
// element of an inline annotation (a trailing comma after it allowed). An annotation must name the
// parameter at its position: one of another length, or naming another injectable there, binds each
// string to its own position only, so the function is refused and mismatch names why
// (`annotation-arity:path:line:X` or `annotation-position:path:line:X`, AMAP-V0-207).
func (f *constFile) injectable(i int, name string) (diFunc, string, bool) {
	toks := f.toks
	pf, ok := f.paramFunc(i, name)
	if !ok {
		return diFunc{}, "", false
	}
	fn, head, fname, after, params, pos := pf.fn, pf.head, pf.fname, pf.after, pf.params, pf.pos
	var lists [][]string // every annotation that injects the function (AMAP-V0-341)
	switch {
	case f.depthAt(head) == 0:
		if fname == "" && head >= 3 && isPunct(toks[head-1], "=") && toks[head-2].kind == tokIdent && toks[head-3].kind == tokIdent && toks[head-3].text == "const" {
			fname = toks[head-2].text
		}
		switch {
		case fname != "":
			if !f.onlyInjected(fname) {
				return diFunc{}, "", false
			}
			if names, ok := f.annot[fname]; ok {
				lists = append(lists, names)
			}
			fn.annot = slices.Clone(f.annotAt[fname])
			for k, t := range toks {
				if t.kind == tokIdent && t.text == fname && !property(toks, k) {
					if inline, at, ok := configArrayNames(toks, k, k+1); ok {
						lists = append(lists, inline)
						fn.annot = append(fn.annot, at...)
					}
				}
			}
		case len(f.annot) > 0:
			return diFunc{}, "", false // an annotation this reader cannot tie to an unnamed function
		}
	case configArg(toks, head-1) && next(toks, after, ")"):
	case head >= 1 && isPunct(toks[head-1], ","):
		names, at, ok := configArrayNames(toks, head, after)
		if !ok {
			return diFunc{}, "", false
		}
		lists, fn.annot = append(lists, names), append(fn.annot, at...)
	default:
		return diFunc{}, "", false
	}
	for _, names := range lists {
		switch {
		case len(names) != len(params):
			return diFunc{}, fmt.Sprintf("annotation-arity:%s:%d:%s", f.entry.path, fn.first, name), false
		case names[pos] != name:
			return diFunc{}, fmt.Sprintf("annotation-position:%s:%d:%s", f.entry.path, fn.first, name), false
		}
	}
	return fn, "", true
}

// inConfigArray reports whether toks[i] is the last element of a `.config([...])` annotation.
func inConfigArray(toks []token, i int) bool {
	_, _, ok := configArrayNames(toks, i, i+1)
	return ok
}

// configArrayNames reads the inline annotation `.config(['a', ..., F])` whose last element spans
// toks[first:after], a function or an identifier naming one (AMAP-V0-341), with a trailing comma
// allowed: its names and their string tokens.
func configArrayNames(toks []token, first, after int) ([]string, []int, bool) {
	var names []string
	var at []int
	k := first - 1
	for k >= 1 && isPunct(toks[k], ",") && literal(toks[k-1]) {
		names = append([]string{toks[k-1].text}, names...)
		at = append(at, k-1)
		k -= 2
	}
	if e := arrayEnd(toks, after); k < 0 || !isPunct(toks[k], "[") || !configArg(toks, k-1) || e < 0 || !next(toks, e+1, ")") {
		return nil, nil, false
	}
	return names, at, true
}

// arrayEnd returns the index of the `]` that closes an array literal whose last element ends
// before toks[k], allowing one trailing comma, or -1.
func arrayEnd(toks []token, k int) int {
	if next(toks, k, ",") {
		k++
	}
	if next(toks, k, "]") {
		return k
	}
	return -1
}

// inlineAnnotation returns the string tokens of the inline annotation `['a', 'name', fn]` whose
// last element is pf's function, when it names every parameter and name at pf's position; nil
// otherwise, so that an annotation which injects name under another parameter stays unaccepted
// (AMAP-V0-204).
func inlineAnnotation(toks []token, pf paramFn, name string) []int {
	var at []int
	var names []string
	k := pf.head - 1
	for k >= 1 && isPunct(toks[k], ",") && literal(toks[k-1]) {
		names = append([]string{toks[k-1].text}, names...)
		at = append(at, k-1)
		k -= 2
	}
	if k < 0 || !isPunct(toks[k], "[") || arrayEnd(toks, pf.after) < 0 || len(names) != len(pf.params) || names[pf.pos] != name ||
		slices.Index(names[pf.pos+1:], name) >= 0 || slices.Index(names[:pf.pos], name) >= 0 { // a second copy may bind an unchecked parameter
		return nil
	}
	return at
}

// paramFn is the function a plain parameter belongs to (see paramFunc).
type paramFn struct {
	fn          diFunc
	head, after int // the function's first token, and the token after it
	fname       string
	params      []param
	pos         int // the parameter's position
}

// paramFunc reads the function whose plain parameter list (see simpleParams) binds name at
// toks[i] exactly once: an arrow, or a function with a block body, and its body's tokens.
func (f *constFile) paramFunc(i int, name string) (paramFn, bool) {
	toks := f.toks
	open := i - 1
	for open >= 0 && (toks[open].kind == tokIdent || literal(toks[open]) || isPunct(toks[open], ",") || isPunct(toks[open], ":") ||
		isPunct(toks[open], ".") || isPunct(toks[open], "?") || typePunct(toks[open])) {
		open--
	}
	if open < 0 || !isPunct(toks[open], "(") {
		return paramFn{}, false
	}
	params, closing, ok := simpleParams(toks, open)
	pos := -1
	for k, p := range params {
		if p.name == name {
			if pos >= 0 {
				return paramFn{}, false
			}
			pos = k
		}
	}
	if !ok || pos < 0 || params[pos].tok != i {
		return paramFn{}, false
	}
	k := closing + 1
	if next(toks, k, ":") { // a return type
		if k = skipTypeName(toks, k+1); k < 0 {
			return paramFn{}, false // a return type the reader cannot step over
		}
	}
	head, fname, bodyStart := open, "", -1
	switch {
	case open >= 1 && toks[open-1].kind == tokIdent && toks[open-1].text == "function":
		head = open - 1
	case open >= 2 && toks[open-1].kind == tokIdent && toks[open-2].kind == tokIdent && toks[open-2].text == "function":
		head, fname = open-2, toks[open-1].text
	case k+1 < len(toks) && isPunct(toks[k], "=") && isPunct(toks[k+1], ">"):
		bodyStart = k + 2
		if open >= 1 && toks[open-1].kind == tokIdent && toks[open-1].text == "async" {
			head = open - 1
		}
	default:
		return paramFn{}, false
	}
	if bodyStart < 0 { // a function: its body is a block
		if !next(toks, k, "{") {
			return paramFn{}, false
		}
		bodyStart = k
	}
	var after int // the token after the function
	fn := diFunc{from: bodyStart, first: toks[open].line, last: toks[closing].line}
	if next(toks, bodyStart, "{") {
		fn.to = closeParen(toks, bodyStart)
		after = fn.to + 1
	} else {
		fn.to = exprEnd(toks, bodyStart)
		after = fn.to
	}
	return paramFn{fn: fn, head: head, after: after, fname: fname, params: params, pos: pos}, true
}

type param struct {
	name string
	tok  int
}

// simpleParams reads a parameter list of plain identifiers, each optionally `?` or typed (see
// skipParamType); ok is false for a destructured, defaulted or rest parameter.
func simpleParams(toks []token, open int) ([]param, int, bool) {
	out := []param{}
	k := open + 1
	if next(toks, k, ")") {
		return out, k, true
	}
	for k < len(toks) && toks[k].kind == tokIdent {
		out = append(out, param{toks[k].text, k})
		k++
		if next(toks, k, "?") {
			k++
		}
		if next(toks, k, ":") {
			if k = skipParamType(toks, k+1); k < 0 {
				return nil, 0, false
			}
		}
		if next(toks, k, ",") {
			k++
			if next(toks, k, ")") {
				return out, k, true
			}
			continue
		}
		if next(toks, k, ")") {
			return out, k, true
		}
		return nil, 0, false
	}
	return nil, 0, false
}

// skipParamType steps over a parameter's type annotation at toks[k] and returns the index of the
// `,` or `)` that ends it, or -1. A type is names (`typeof` and `keyof` included), dotted names,
// literals, unions and intersections, arrays and type arguments (`ng.IStateProvider`, `typeof
// RoutesConstants`, `Array<X | 'a'>`, `X[]`) (AMAP-V0-204). Anything else -- a function or object
// type, a default value -- fails, so no expression a parameter list may run is stepped over.
func skipParamType(toks []token, k int) int {
	depth := 0
	start := k
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
		case (isPunct(t, ",") || isPunct(t, ")")) && depth == 0 && k > start:
			return k
		default:
			return -1
		}
	}
	return -1
}

// typePunct reports whether t is a punctuator skipParamType steps over.
func typePunct(t token) bool {
	return t.kind == tokPunct && (t.text == "<" || t.text == ">" || t.text == "[" || t.text == "]" || t.text == "|" || t.text == "&")
}

// skipTypeName steps over a dotted type name at toks[k] and returns the index after it, or -1.
func skipTypeName(toks []token, k int) int {
	if k >= len(toks) || toks[k].kind != tokIdent {
		return -1
	}
	k++
	for k+1 < len(toks) && isPunct(toks[k], ".") && toks[k+1].kind == tokIdent {
		k += 2
	}
	return k
}

// exprEnd returns the index after an arrow function's expression body starting at toks[i]: the
// first `;` or `,` at its own depth, a closing bracket below it, or a new line that does not
// continue a member chain.
func exprEnd(toks []token, i int) int {
	depth := 0
	for k := i; k < len(toks); k++ {
		t := toks[k]
		if depth == 0 && k > i && t.line > toks[k-1].line && !isPunct(t, ".") {
			return k
		}
		if t.kind != tokPunct {
			continue
		}
		switch t.text {
		case "{", "[", "(":
			depth++
		case "}", "]", ")":
			if depth == 0 {
				return k
			}
			depth--
		case ";", ",":
			if depth == 0 {
				return k
			}
		}
	}
	return len(toks)
}

// configArg reports whether toks[k] is the `(` of a `.config(` call.
func configArg(toks []token, k int) bool {
	return k >= 2 && isPunct(toks[k], "(") && toks[k-1].kind == tokIdent && toks[k-1].text == "config" && isPunct(toks[k-2], ".")
}

// depthAt returns how many brackets enclose toks[i].
func (f *constFile) depthAt(i int) int {
	if f.depth == nil {
		f.depth = make([]int, len(f.toks))
		d := 0
		for k, t := range f.toks {
			if t.kind == tokPunct && (t.text == "}" || t.text == "]" || t.text == ")") {
				d--
			}
			f.depth[k] = d
			if t.kind == tokPunct && (t.text == "{" || t.text == "[" || t.text == "(") {
				d++
			}
		}
	}
	return f.depth[i]
}

// annotations reads every `F.$inject = ['a', ...]` statement and every `static $inject = [...]`
// field of a top-level class declaration (AMAP-V0-423) of the file once; false when a `$inject` is
// anything else, or a function or class is annotated twice.
func (f *constFile) annotations() bool {
	if f.annot != nil {
		return f.annotOK
	}
	f.annot, f.annotAt, f.annotOK = map[string][]string{}, map[string][]int{}, true
	toks := f.toks
	for k, t := range toks {
		if t.kind != tokIdent || t.text != "$inject" {
			continue
		}
		fname, eq := "", -1
		if k >= 2 && isPunct(toks[k-1], ".") && toks[k-2].kind == tokIdent && f.depthAt(k-2) == 0 &&
			(k < 3 || isPunct(toks[k-3], ";") || isPunct(toks[k-3], "}") || toks[k-3].line < toks[k-2].line) {
			fname, eq = toks[k-2].text, k+1
		} else {
			fname, eq = f.staticInject(k)
		}
		ok := eq > 0 && next(toks, eq, "=") && !next(toks, eq+1, "=", ">")
		if ok {
			v, after := parseValue(toks, eq+1)
			names, list := v.stringList()
			_, dup := f.annot[fname]
			ok = list && !dup && statementEnd(toks, after)
			if ok {
				f.annot[fname] = names
				for j := eq + 1; j < after && j < len(toks); j++ {
					if toks[j].kind == tokString || toks[j].kind == tokTemplate {
						f.annotAt[fname] = append(f.annotAt[fname], j)
					}
				}
			}
		}
		if !ok {
			if f.annotOK {
				f.annotBad = t.line
			}
			f.annotOK = false
			f.annotBadLines = append(f.annotBadLines, t.line)
		}
	}
	return f.annotOK
}

// statementEnd reports whether an annotation value ending before toks[after] ends its statement or
// class member: the end of the file, `;`, `}`, or a later line starting with a token that cannot
// continue the expression (a name other than continuingWords, a string, a number or `@`), so
// `['a']\n.concat(x)` or `['a']\n(x)` stays one unread value (AMAP-V0-423).
func statementEnd(toks []token, after int) bool {
	if after >= len(toks) || next(toks, after, ";", "}") {
		return true
	}
	t := toks[after]
	return t.line > toks[after-1].line && (t.kind == tokIdent && !continuingWords[t.text] || t.kind == tokString ||
		t.kind == tokNumber || isPunct(t, "@"))
}

// staticInject reads a `static $inject` field at toks[k], optionally `readonly` and typed, as a
// member of the body of a top-level class declaration with a name: that name and the index of the
// `=` after the field (eq is -1 otherwise). An instance field, a class expression, a nested or
// anonymous class, or a computed key is no annotation the reader ties to a class (AMAP-V0-423).
func (f *constFile) staticInject(k int) (name string, eq int) {
	toks := f.toks
	s := k - 1
	if word(toks, s, "readonly") {
		s--
	}
	if !word(toks, s, "static") || property(toks, k) {
		return "", -1
	}
	open := enclosingOpen(toks, s)
	if open < 1 || !isPunct(toks[open], "{") || !memberStart(toks, s, open) {
		return "", -1
	}
	c := open - 1
	for c > 0 && !word(toks, c, "class") && (toks[c].kind == tokIdent || isPunct(toks[c], ".") || isPunct(toks[c], ",") ||
		isPunct(toks[c], "<") || isPunct(toks[c], ">")) {
		c--
	}
	if !word(toks, c, "class") || c+1 >= open || toks[c+1].kind != tokIdent || toks[c+1].text == "extends" ||
		toks[c+1].text == "implements" || classBodyOpen(toks, c+2) != open || f.depthAt(c) != 0 || !classDeclaration(toks, c) {
		return "", -1
	}
	eq = k + 1
	if next(toks, eq, ":") {
		if eq = typeEnd(toks, eq+1); eq < 0 {
			return "", -1
		}
	}
	return toks[c+1].text, eq
}

// classDeclaration reports whether the `class` keyword at toks[c] starts a declaration rather
// than a class expression: at the start of the file or a statement, after `export`, `export
// default` or `abstract`, or after a decorator's `)` or a name on an earlier line.
func classDeclaration(toks []token, c int) bool {
	p := c - 1
	if word(toks, p, "abstract") {
		p--
	}
	if word(toks, p, "default") && word(toks, p-1, "export") {
		return true
	}
	return p < 0 || next(toks, p, ";", "}") || word(toks, p, "export") ||
		toks[p].line < toks[c].line && (isPunct(toks[p], ")") || toks[p].kind == tokIdent && !continuingWords[toks[p].text])
}

// typeEnd steps over a type annotation starting at toks[k] -- names, dotted names, literals,
// unions, intersections, arrays and type arguments -- and returns the index of the first token
// after it at its own depth, or -1 when it holds anything else or does not close.
func typeEnd(toks []token, k int) int {
	depth := 0
	start := k
	for ; k < len(toks); k++ {
		t := toks[k]
		switch {
		case t.kind == tokIdent || literal(t) || t.kind == tokNumber || isPunct(t, ".") || isPunct(t, "|") || isPunct(t, "&"):
		case isPunct(t, "<") || isPunct(t, "["):
			depth++
		case isPunct(t, ">") || isPunct(t, "]"):
			if depth--; depth < 0 {
				return -1
			}
		case isPunct(t, ",") && depth > 0:
		default:
			if depth != 0 || k == start {
				return -1
			}
			return k
		}
	}
	return -1
}

// onlyInjected reports whether a top-level function name is used only where it is declared,
// annotated with `$inject`, passed to `.config(F)` or as the last element of a `.config([...])`
// annotation (AMAP-V0-341), or exported; any other use could hand the
// function to an annotation this reader does not see.
func (f *constFile) onlyInjected(fname string) bool {
	toks := f.toks
	declared := 0
	for i, t := range toks {
		if t.kind != tokIdent || t.text != fname || property(toks, i) {
			continue
		}
		prev := ""
		if i > 0 && toks[i-1].kind == tokIdent {
			prev = toks[i-1].text
		}
		switch {
		case prev == "const" || prev == "function":
			declared++
		case next(toks, i+1, ".") && i+2 < len(toks) && toks[i+2].text == "$inject":
		case i >= 1 && configArg(toks, i-1) && next(toks, i+1, ")"):
		case inConfigArray(toks, i):
		case prev == "default" && i > 1 && toks[i-2].text == "export":
		case inExportList(toks, i):
		default:
			return false
		}
	}
	return declared == 1
}
