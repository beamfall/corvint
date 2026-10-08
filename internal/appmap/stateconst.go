package appmap

import (
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// A `.state()` name or `parent` written as a member expression `X.Y` resolves only when `X` is a
// constant table the map can read whole (AMAP-V0-016): a top-level `const X = {Y: '...'}` (with an
// optional `as const`, or wrapped in `Object.freeze(...)`) or a string `enum X {Y = '...'}`,
// declared in the router file itself or in the tracked file an ES `import` of `X` resolves to.
// Anything else -- a spread or computed key, a duplicate declaration, a non-string member, a use
// of `X` other than a member read, an unresolvable or untracked import -- resolves nothing, so the
// caller keeps its UNKNOWN reason. Each declaring file is lexed at most once per build.

// constTable memoizes declaring files for one build.
type constTable struct {
	ix       *contextindex.Index
	resolver *contextindex.WebImportResolver
	files    map[string]*constFile // tracked path -> parsed file; nil when unreadable
	di       *diScope              // the manifest's di_constants scope (AMAP-V0-021); nil without one
}

// newConstTable prepares one build's constant resolution; a di_constants scope that names no
// indexed source, or too many, refuses (AMAP-V0-021).
func newConstTable(ix *contextindex.Index, scope []string) (*constTable, error) {
	t := &constTable{ix: ix, files: map[string]*constFile{}}
	if ix != nil {
		t.resolver = contextindex.NewWebImportResolver(ix)
	}
	if len(scope) > 0 {
		di, err := newDIScope(ix, scope)
		if err != nil {
			return nil, err
		}
		t.di = di
	}
	return t, nil
}

// err reports a bound the lazily read di_constants scope exceeded (AMAP-V0-021).
func (t *constTable) err() error {
	if t.di == nil {
		return nil
	}
	return t.di.err
}

// constFile is one lexed source: its top-level constant tables, the names a static import binds,
// and which names are only ever read as `name.member`.
type constFile struct {
	entry   blobEntry
	data    []byte
	toks    []token
	decls   map[string]*constDecl // local name -> declaration; "default" for `export default {...}`
	dflt    string                // the local name `export default NAME` exports, or "default"
	imports map[string]constImport
	skip    []bool // tokens of import statements and constant declaration headers
	reads   map[readKey]bool
	// inject, depth and annot are the router-side injection reads (AMAP-V0-022), built on demand.
	inject  map[string]*diBinding
	depth   []int
	annot   map[string][]string
	annotOK bool
}

// readKey memoizes onlyRead for one name and the one token (or -1) allowed to pass it along.
type readKey struct {
	name  string
	allow int
}

type constDecl struct {
	members  map[string]constMember
	exported bool
	bad      bool // spread or computed member, unreadable initializer, or declared more than once
}

type constMember struct {
	value string
	line  int
}

type constImport struct {
	module, exported string
	first, last      int  // the import statement's lines
	bad              bool // the local name is bound more than once
}

// forRouter returns the resolver a router file's `X.Y` references go through.
func (t *constTable) forRouter(e blobEntry, data []byte) constLookup {
	var own *constFile
	return func(ref string, tok int) (string, []Anchor, bool) {
		local, member, ok := strings.Cut(ref, ".")
		if !ok {
			return "", nil, false
		}
		if own == nil {
			own = parseConstFile(e, data)
		}
		_, declared := own.decls[local]
		_, imported := own.imports[local]
		if !declared && !imported && t.di != nil {
			return t.injected(own, local, member, tok)
		}
		return t.resolve(own, local, member, -1)
	}
}

// resolve reads local.member through a table f declares or imports; allow is the one token (or -1)
// that may pass the table along, the `.constant(...)` argument that registers it (AMAP-V0-022).
func (t *constTable) resolve(f *constFile, local, member string, allow int) (string, []Anchor, bool) {
	if !f.onlyRead(local, allow) {
		return "", nil, false
	}
	decl, declared := f.decls[local]
	imp, imported := f.imports[local]
	switch {
	case declared && imported:
		return "", nil, false // two bindings for one name: ambiguous
	case declared:
		return f.member(decl, member)
	case imported && !imp.bad && t.resolver != nil:
		res := t.resolver.Resolve(f.entry.path, imp.module)
		if res.State != contextindex.WebImportRepository {
			return "", nil, false
		}
		g := t.file(res.Target)
		if g == nil {
			return "", nil, false
		}
		name := imp.exported
		if name == "default" {
			name = g.dflt // the default export: an object literal, or a table declared under a name
		}
		d := g.decls[name]
		switch {
		case d == nil, imp.exported != "default" && !d.exported:
			return "", nil, false
		case name != "default" && !g.onlyRead(name, -1):
			return "", nil, false
		}
		v, at, ok := g.member(d, member)
		if !ok {
			return "", nil, false
		}
		// The binding is evidence too: re-pointing the import changes what the name reads.
		return v, append(at, spanOf(f.entry, f.data, imp.first, imp.last)), true
	}
	return "", nil, false
}

// file reads one tracked source from the revision's index, once.
func (t *constTable) file(p string) *constFile {
	if f, seen := t.files[p]; seen {
		return f
	}
	t.files[p] = nil
	src, ok := t.ix.Sources[p]
	if !ok {
		return nil
	}
	text, valid, loaded := src.Text()
	if !valid || !loaded || len(text) > maxRouterBytes {
		return nil
	}
	f := parseConstFile(blobEntry{path: p, oid: src.BlobHash, size: len(text)}, []byte(text))
	t.files[p] = f
	return f
}

func (f *constFile) member(d *constDecl, name string) (string, []Anchor, bool) {
	if d.bad {
		return "", nil, false
	}
	m, ok := d.members[name]
	if !ok {
		return "", nil, false
	}
	return m.value, []Anchor{spanOf(f.entry, f.data, m.line, m.line)}, true
}

// onlyRead reports whether every use of name outside import statements and constant declarations
// is a member read `name.member` that is not assigned, deleted or incremented, a `typeof name`, or
// an `export { name }` / `export default name`. Any other use could mutate or rebind the table.
// The token at allow (or none, -1) is exempt: it registers the table for injection.
func (f *constFile) onlyRead(name string, allow int) bool {
	key := readKey{name, allow}
	if v, ok := f.reads[key]; ok {
		return v
	}
	ok := true
	toks := f.toks
	for i := 0; i < len(toks) && ok; i++ {
		if i == allow || f.skip[i] || toks[i].kind != tokIdent || toks[i].text != name || (i > 0 && isPunct(toks[i-1], ".")) {
			continue
		}
		prev := ""
		if i > 0 && toks[i-1].kind == tokIdent {
			prev = toks[i-1].text
		}
		switch {
		case prev == "typeof":
		case prev == "default" && i > 1 && toks[i-2].text == "export":
		case inExportList(toks, i):
		case i+2 < len(toks) && isPunct(toks[i+1], ".") && toks[i+2].kind == tokIdent:
			ok = !written(toks, i)
		default:
			ok = false
		}
	}
	f.reads[key] = ok
	return ok
}

// written reports whether the member expression toks[i..i+2] is deleted, incremented or assigned,
// looking through parentheses (`(X.Y) = ...`, `++X.Y`) and closing brackets of a destructuring
// pattern (`[X.Y] = ...`, `({a: X.Y} = ...)`). A computed key `o[X.Y] = ...` reads as written too.
func written(toks []token, i int) bool {
	b := i - 1
	for b >= 0 && isPunct(toks[b], "(") {
		b--
	}
	if b >= 0 && toks[b].kind == tokIdent && toks[b].text == "delete" {
		return true
	}
	if b >= 1 && (isPunct(toks[b], "+") && isPunct(toks[b-1], "+") || isPunct(toks[b], "-") && isPunct(toks[b-1], "-")) {
		return true
	}
	a := i + 3
	for a < len(toks) && (isPunct(toks[a], ")") || isPunct(toks[a], "]") || isPunct(toks[a], "}")) {
		a++
	}
	return assigned(toks, a)
}

// assigned reports whether the operator at toks[i] assigns to, or increments, what precedes it.
func assigned(toks []token, i int) bool {
	run := ""
	for ; i < len(toks) && toks[i].kind == tokPunct && strings.Contains("+-*/%&|^?<>", toks[i].text); i++ {
		run += toks[i].text
	}
	if run == "++" || run == "--" {
		return true
	}
	if i >= len(toks) || !isPunct(toks[i], "=") || next(toks, i+1, "=") {
		return false
	}
	switch run {
	case "", "+", "-", "*", "/", "%", "**", "&", "|", "^", "<<", ">>", ">>>", "&&", "||", "??":
		return true
	}
	return false // `<=`, `>=`
}

// inExportList reports whether toks[i] sits inside `export { ... }` without a `from` clause.
func inExportList(toks []token, i int) bool {
	j := i - 1
	for ; j >= 0 && !isPunct(toks[j], "{"); j-- {
		if toks[j].kind != tokIdent && !isPunct(toks[j], ",") {
			return false
		}
	}
	if j < 1 || toks[j-1].text != "export" {
		return false
	}
	k := i + 1
	for ; k < len(toks) && !isPunct(toks[k], "}"); k++ {
	}
	return !(k+1 < len(toks) && toks[k+1].kind == tokIdent && toks[k+1].text == "from")
}

// parseConstFile lexes one file and reads its top-level imports and constant tables.
func parseConstFile(e blobEntry, data []byte) *constFile {
	toks, _ := lexJS(string(data))
	f := &constFile{entry: e, data: data, toks: toks, decls: map[string]*constDecl{}, imports: map[string]constImport{},
		skip: make([]bool, len(toks)), reads: map[readKey]bool{}}
	declare := func(name string, d *constDecl) {
		if prior, dup := f.decls[name]; dup {
			prior.bad = true
			return
		}
		f.decls[name] = d
	}
	depth, exportAt := 0, -1
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == tokPunct {
			switch t.text {
			case "{", "[", "(":
				depth++
			case "}", "]", ")":
				depth--
			}
			exportAt = -1
			continue
		}
		if depth != 0 || t.kind != tokIdent || (i > 0 && isPunct(toks[i-1], ".")) {
			exportAt = -1
			continue
		}
		if t.text == "export" {
			exportAt = i
			continue
		}
		start, exported := i, exportAt >= 0
		if exported {
			start = exportAt
		}
		exportAt = -1
		switch {
		case t.text == "default" && exported && i+1 < len(toks):
			if isPunct(toks[i+1], "{") {
				v, after := parseValue(toks, i+1)
				d := objectDecl(v, toks, after)
				d.exported = true
				declare("default", d)
				f.dflt = "default"
				f.mark(start, i+1)
				i = after - 1
			} else if toks[i+1].kind == tokIdent && (i+2 >= len(toks) || isPunct(toks[i+2], ";") || toks[i+2].line != toks[i+1].line) {
				f.dflt = toks[i+1].text // only a bare name: `export default X.Y` exports a value, not X
			}
		case t.text == "import" && !exported:
			end := f.readImport(i)
			f.mark(i, end)
			i = end - 1
		case t.text == "const" && i+2 < len(toks) && toks[i+1].kind == tokIdent && toks[i+1].text != "enum":
			name, k := toks[i+1].text, i+2
			if !isPunct(toks[k], "=") || k+1 >= len(toks) {
				declare(name, &constDecl{bad: true}) // a type annotation: not read
				continue
			}
			k++
			frozen := k+4 < len(toks) && toks[k].text == "Object" && isPunct(toks[k+1], ".") && toks[k+2].text == "freeze" && isPunct(toks[k+3], "(")
			if frozen {
				k += 4
			}
			if !isPunct(toks[k], "{") {
				declare(name, &constDecl{bad: true})
				continue
			}
			v, after := parseValue(toks, k)
			if frozen {
				if !next(toks, after, ")") {
					v = jsValue{kind: "other"}
				}
				after++
			}
			d := objectDecl(v, toks, after)
			d.exported = exported
			declare(name, d)
			f.mark(start, i+2) // the header only: the initializer may alias another table
			i = after - 1
		case t.text == "enum" || (t.text == "const" && i+1 < len(toks) && toks[i+1].text == "enum"):
			if t.text == "const" {
				i++
			}
			if i+2 >= len(toks) || toks[i+1].kind != tokIdent || !isPunct(toks[i+2], "{") {
				continue
			}
			d, after := enumDecl(toks, i+2)
			d.exported = exported
			declare(toks[i+1].text, d)
			f.mark(start, i+2)
			i = after - 1
		case t.text == "let" || t.text == "var" || t.text == "function" || t.text == "class":
			if i+1 < len(toks) && toks[i+1].kind == tokIdent {
				declare(toks[i+1].text, &constDecl{bad: true})
			}
		}
	}
	return f
}

func (f *constFile) mark(from, to int) {
	for k := from; k < to && k < len(f.skip); k++ {
		f.skip[k] = true
	}
}

// objectDecl reads a constant table from an object literal; the declaration must end at after
// (`;`, `as const`, a new line or the end of the file), or the table is not what the name holds.
func objectDecl(v jsValue, toks []token, after int) *constDecl {
	d := objectMembers(v)
	if after+1 < len(toks) && toks[after].text == "as" && toks[after+1].text == "const" {
		after += 2
	}
	if after < len(toks) && !isPunct(toks[after], ";") && (toks[after].kind == tokPunct || toks[after].line == toks[after-1].line) {
		d.bad = true
	}
	return d
}

// objectMembers reads the string members of an object literal; anything that may set a member the
// reader cannot see makes the table unreadable.
func objectMembers(v jsValue) *constDecl {
	d := &constDecl{members: map[string]constMember{}}
	if v.kind != "object" {
		d.bad = true
		return d
	}
	for _, p := range v.obj {
		if p.key == "" {
			d.bad = true // a spread, computed key or repeated key may set any member
		}
		if p.value.kind == "string" {
			d.members[p.key] = constMember{value: p.value.str, line: p.value.line}
		}
	}
	return d
}

// enumDecl reads `{ A = 'a', B = 'b' }` at toks[i]; a member without a string initializer is
// absent, and anything the reader cannot follow makes the whole enum unreadable.
func enumDecl(toks []token, i int) (*constDecl, int) {
	d := &constDecl{members: map[string]constMember{}}
	end := closeParen(toks, i)
	for k := i + 1; k < end; {
		key := toks[k]
		if key.kind != tokIdent && (key.kind != tokString || key.inexact) {
			d.bad = true
			break
		}
		if _, dup := d.members[key.text]; dup {
			d.bad = true
		}
		k++
		if next(toks, k, "=") {
			if k+1 < end && literal(toks[k+1]) && next(toks, k+2, ",", "}") {
				d.members[key.text] = constMember{value: toks[k+1].text, line: toks[k+1].line}
			}
			k = skipValue(toks, k+1)
		}
		if next(toks, k, ",") {
			k++
		} else if k != end {
			d.bad = true
			break
		}
	}
	return d, end + 1
}

// readImport reads one static `import` statement at toks[i] and returns the index after it. Only
// default and named value bindings are kept; a namespace or type-only binding holds no table.
func (f *constFile) readImport(i int) int {
	toks := f.toks
	j := i + 1
	if j >= len(toks) || toks[j].kind == tokPunct && (toks[j].text == "(" || toks[j].text == ".") {
		return j // a dynamic import or import.meta
	}
	if toks[j].kind != tokIdent && toks[j].kind != tokPunct {
		return j + 1 // a side-effect import
	}
	typeOnly := toks[j].text == "type" && j+1 < len(toks) && (isPunct(toks[j+1], "{") || (toks[j+1].kind == tokIdent && toks[j+1].text != "from"))
	if typeOnly {
		j++
	}
	type item struct{ local, exported string }
	items := []item{}
	for j < len(toks) && !(toks[j].kind == tokIdent && toks[j].text == "from") {
		switch t := toks[j]; {
		case isPunct(t, "{"):
			end := closeParen(toks, j)
			for k := j + 1; k < end; k = skipValue(toks, k) + 1 {
				switch {
				case toks[k].text == "type" && k+1 < end && toks[k+1].kind == tokIdent && toks[k+1].text != "as":
					// a type-only item binds no value
				case toks[k].kind == tokIdent && next(toks, k+1, ",", "}"):
					items = append(items, item{toks[k].text, toks[k].text})
				case toks[k].kind == tokIdent && k+3 <= end && toks[k+1].text == "as" && toks[k+2].kind == tokIdent && next(toks, k+3, ",", "}"):
					items = append(items, item{toks[k+2].text, toks[k].text})
				}
			}
			j = end + 1
		case isPunct(t, "*"):
			j += 3 // `* as NS`
		case t.kind == tokIdent:
			items = append(items, item{t.text, "default"})
			j++
		default:
			return j // not an import statement this reader follows
		}
		if next(toks, j, ",") {
			j++
		}
	}
	if j+1 >= len(toks) || !literal(toks[j+1]) {
		return j
	}
	for _, it := range items {
		if typeOnly {
			break
		}
		if prior, dup := f.imports[it.local]; dup {
			prior.bad = true
			f.imports[it.local] = prior
			continue
		}
		f.imports[it.local] = constImport{module: toks[j+1].text, exported: it.exported, first: toks[i].line, last: toks[j+1].line}
	}
	return j + 2
}
