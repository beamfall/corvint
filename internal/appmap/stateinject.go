package appmap

import (
	"fmt"
	"path"
	"sort"
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
)

// diScope is the manifest's di_constants scope: its sources, read once on first use.
type diScope struct {
	sources []string // sorted tracked web sources under the scope paths
	poison  bool     // a registration the reader cannot attribute: every injected name is unprovable
	scanned bool
	regs    map[string][]diReg
	err     error // a bound breached while reading the scope
}

// diReg is one `.constant('NAME', T)` registration.
type diReg struct {
	file        *constFile
	ident       string     // T as an identifier
	at          int        // T's token index
	table       *constDecl // T as an object literal argument
	first, last int        // the call's lines
	bad         bool       // T is neither, or the name came from an object-map registration
}

// diBinding records how a router file binds one name: whether every binding is an injected
// parameter, and the bodies of the functions that receive it.
type diBinding struct {
	ok  bool
	fns []diFunc
}

type diFunc struct {
	from, to    int // body tokens [from, to)
	first, last int // the parameter list's lines
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
			s.poison = true // an unread source could register any name
		}
	}
	return s, nil
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
				s.poison = true
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
				s.poison = true
			}
		}
	}
	return s.regs[name]
}

// collect records every `.constant(...)` call of one scope file.
func (s *diScope) collect(f *constFile) {
	toks := f.toks
	for j := 1; j+2 < len(toks); j++ {
		if toks[j].kind != tokIdent || toks[j].text != "constant" || !isPunct(toks[j-1], ".") || !isPunct(toks[j+1], "(") {
			continue
		}
		end := closeParen(toks, j+1)
		a := toks[j+2]
		switch {
		case isPunct(a, ")"):
			// no argument: registers nothing
		case literal(a) && next(toks, j+3, ","):
			r := diReg{file: f, first: toks[j].line, last: toks[end].line}
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
		case literal(a):
			s.regs[a.text] = append(s.regs[a.text], diReg{file: f, bad: true}) // a value the reader cannot see
		case isPunct(a, "{"):
			// An object map registers each key; its values are not read.
			v, _ := parseValue(toks, j+2)
			if v.kind != "object" {
				s.poison = true
				continue
			}
			for _, p := range v.obj {
				if p.key == "" {
					s.poison = true
					continue
				}
				s.regs[p.key] = append(s.regs[p.key], diReg{file: f, bad: true})
			}
		case j >= 2 && toks[j-2].kind == tokIdent && (toks[j-2].text == "_" || toks[j-2].text == "lodash") && (j < 3 || !isPunct(toks[j-3], ".")):
			// lodash's constant function, not a registration
		default:
			s.poison = true // a computed name could register anything
		}
	}
}

// wholeCallArg reports whether the argument that ended at toks[i] is the call's last, so the
// registration holds exactly that value.
func wholeCallArg(toks []token, i, end int) bool {
	return i == end || (i+1 == end && isPunct(toks[i], ","))
}

// injected resolves local.member, read at token tok of the router file own, through the one
// `.constant('local', T)` registration in scope (AMAP-V0-022).
func (t *constTable) injected(own *constFile, local, member string, tok int) (string, []Anchor, bool) {
	b := own.injection(local)
	if !b.ok {
		return "", nil, false
	}
	var fn *diFunc
	for i := range b.fns {
		if tok >= b.fns[i].from && tok < b.fns[i].to {
			fn = &b.fns[i]
			break
		}
	}
	if fn == nil {
		return "", nil, false
	}
	regs := t.registrations(local)
	if t.di.err != nil || t.di.poison || len(regs) != 1 || regs[0].bad {
		return "", nil, false
	}
	r := regs[0]
	var v string
	var at []Anchor
	var ok bool
	if r.table != nil {
		v, at, ok = r.file.member(r.table, member)
	} else {
		v, at, ok = t.resolve(r.file, r.ident, member, r.at)
	}
	if !ok {
		return "", nil, false
	}
	if reg := spanOf(r.file.entry, r.file.data, r.first, r.last); len(at) == 0 || at[len(at)-1] != reg {
		at = append(at, reg) // an object literal argument on the call's line is carried once
	}
	return v, append(at, spanOf(own.entry, own.data, fn.first, fn.last)), true
}

// injection reads how the router file binds name: ok only when every use is an unwritten member
// read, a `typeof`, or a plain parameter of an injectable function.
func (f *constFile) injection(name string) *diBinding {
	if f.inject == nil {
		f.inject = map[string]*diBinding{}
	}
	if b, seen := f.inject[name]; seen {
		return b
	}
	b := &diBinding{}
	f.inject[name] = b
	if !f.annotations() {
		return b
	}
	toks := f.toks
	for i := range toks {
		if f.skip[i] || toks[i].kind != tokIdent || toks[i].text != name || (i > 0 && isPunct(toks[i-1], ".")) {
			continue
		}
		switch {
		case i > 0 && toks[i-1].kind == tokIdent && toks[i-1].text == "typeof":
		case i+2 < len(toks) && isPunct(toks[i+1], ".") && toks[i+2].kind == tokIdent:
			if written(toks, i) {
				return b
			}
		default:
			fn, ok := f.injectable(i, name)
			if !ok {
				return b
			}
			b.fns = append(b.fns, fn)
		}
	}
	b.ok = len(b.fns) > 0
	return b
}

// injectable reports whether toks[i] is a plain parameter of a function AngularJS injects: a
// top-level arrow or function, or the direct argument of `.config(...)`, alone or as the last
// element of an inline annotation. An annotation must name the parameter at its position.
func (f *constFile) injectable(i int, name string) (diFunc, bool) {
	toks := f.toks
	open := i - 1
	for open >= 0 && (toks[open].kind == tokIdent || isPunct(toks[open], ",") || isPunct(toks[open], ":") || isPunct(toks[open], ".") || isPunct(toks[open], "?")) {
		open--
	}
	if open < 0 || !isPunct(toks[open], "(") {
		return diFunc{}, false
	}
	params, closing, ok := simpleParams(toks, open)
	pos := -1
	for k, p := range params {
		if p.name == name {
			if pos >= 0 {
				return diFunc{}, false
			}
			pos = k
		}
	}
	if !ok || pos < 0 || params[pos].tok != i {
		return diFunc{}, false
	}
	k := closing + 1
	if next(toks, k, ":") { // a return type
		k = skipTypeName(toks, k+1)
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
		return diFunc{}, false
	}
	if bodyStart < 0 { // a function: its body is a block
		if !next(toks, k, "{") {
			return diFunc{}, false
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
	var names []string
	annotated := false
	switch {
	case f.depthAt(head) == 0:
		if fname == "" && head >= 3 && isPunct(toks[head-1], "=") && toks[head-2].kind == tokIdent && toks[head-3].kind == tokIdent && toks[head-3].text == "const" {
			fname = toks[head-2].text
		}
		switch {
		case fname != "":
			if !f.onlyInjected(fname) {
				return diFunc{}, false
			}
			names, annotated = f.annot[fname]
		case len(f.annot) > 0:
			return diFunc{}, false // an annotation this reader cannot tie to an unnamed function
		}
	case configArg(toks, head-1) && next(toks, after, ")"):
	case head >= 1 && isPunct(toks[head-1], ","):
		k := head - 1
		for k >= 1 && isPunct(toks[k], ",") && literal(toks[k-1]) {
			names = append([]string{toks[k-1].text}, names...)
			k -= 2
		}
		if k < 0 || !isPunct(toks[k], "[") || !configArg(toks, k-1) || !next(toks, after, "]") || !next(toks, after+1, ")") {
			return diFunc{}, false
		}
		annotated = true
	default:
		return diFunc{}, false
	}
	if annotated && (len(names) != len(params) || names[pos] != name) {
		return diFunc{}, false
	}
	return fn, true
}

type param struct {
	name string
	tok  int
}

// simpleParams reads a parameter list of plain identifiers, each optionally `?` or typed with a
// dotted type name; ok is false for a destructured, defaulted or rest parameter.
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
			if k = skipTypeName(toks, k+1); k < 0 {
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

// annotations reads every `F.$inject = ['a', ...]` statement of the file once; false when a
// `$inject` is anything else, or a function is annotated twice.
func (f *constFile) annotations() bool {
	if f.annot != nil {
		return f.annotOK
	}
	f.annot, f.annotOK = map[string][]string{}, true
	toks := f.toks
	for k, t := range toks {
		if t.kind != tokIdent || t.text != "$inject" {
			continue
		}
		ok := k >= 2 && isPunct(toks[k-1], ".") && toks[k-2].kind == tokIdent && f.depthAt(k-2) == 0 &&
			(k < 3 || isPunct(toks[k-3], ";") || isPunct(toks[k-3], "}") || toks[k-3].line < toks[k-2].line) &&
			next(toks, k+1, "=") && !next(toks, k+2, "=")
		if ok {
			v, after := parseValue(toks, k+2)
			names, list := v.stringList()
			_, dup := f.annot[toks[k-2].text]
			ok = list && !dup && (after >= len(toks) || isPunct(toks[after], ";") || toks[after].line > toks[after-1].line)
			if ok {
				f.annot[toks[k-2].text] = names
			}
		}
		if !ok {
			f.annotOK = false
			return false
		}
	}
	return true
}

// onlyInjected reports whether a top-level function name is used only where it is declared,
// annotated with `$inject`, passed to `.config(F)` or exported; any other use could hand the
// function to an annotation this reader does not see.
func (f *constFile) onlyInjected(fname string) bool {
	toks := f.toks
	declared := 0
	for i, t := range toks {
		if t.kind != tokIdent || t.text != fname || (i > 0 && isPunct(toks[i-1], ".")) {
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
		case prev == "default" && i > 1 && toks[i-2].text == "export":
		case inExportList(toks, i):
		default:
			return false
		}
	}
	return declared == 1
}
