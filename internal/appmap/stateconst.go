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
	diag     []Unknown             // why an in-scope registration did not resolve (AMAP-V0-026)
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
	// reexports are the file's `export ... from` statements, read only through an injected
	// registration's import (AMAP-V0-025); opaque marks one the reader could not read item by item.
	reexports []reexport
	opaque    bool
	// exports are the names the file may export itself, by an exported declaration of any kind or a
	// local `export { ... }` list, whether or not the reader can read the value (AMAP-V0-025): every
	// identifier of an export statement the reader does not read name by name counts. unlisted
	// marks a file whose brackets do not balance, so an export statement may have gone unseen.
	exports  map[string]bool
	unlisted bool
	skip     []bool // tokens of import and re-export statements and constant declaration headers
	reads    map[readKey]bool
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
	partial  bool // an enum member without a literal string initializer (AMAP-V0-024)
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

// reexport is one name an `export ... from 'module'` statement exports: `export { source as
// exported }`, `export * as exported` (source "*"), or every name, `export *` (exported "").
type reexport struct {
	exported, source, module string
	first, last              int
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
	v, at, why := t.lookup(f, local, member, allow, false)
	return v, at, why == ""
}

// lookup is resolve with the reason it fails (AMAP-V0-026). An injected registration (di) also
// reads an enum only when every member is a literal string (AMAP-V0-024) and follows one level of
// re-export in the imported file (AMAP-V0-025); a router's own tables keep the AMAP-V0-016 rules.
func (t *constTable) lookup(f *constFile, local, member string, allow int, di bool) (string, []Anchor, string) {
	if !f.onlyRead(local, allow) {
		return "", nil, "not-read-whole"
	}
	decl, declared := f.decls[local]
	imp, imported := f.imports[local]
	switch {
	case declared && imported, imported && imp.bad:
		return "", nil, "ambiguous-binding" // two bindings for one name
	case declared:
		return f.read(decl, member, di)
	case !imported:
		return "", nil, "identifier-not-found"
	case t.resolver == nil:
		return "", nil, "out-of-scope"
	}
	g, why := t.target(f.entry.path, imp.module)
	if why != "" {
		return "", nil, why
	}
	name := imp.exported
	if name == "default" {
		name = g.dflt // the default export: an object literal, or a table declared under a name
	}
	d := g.decls[name]
	var via []Anchor // the re-export statement, when the table is one level behind the import
	switch {
	case d == nil && di:
		if g, name, via, why = t.reexported(g, imp.exported); why != "" {
			return "", nil, why
		}
		d = g.decls[name]
	case d == nil, imp.exported != "default" && !d.exported:
		return "", nil, "identifier-not-found"
	}
	if name != "default" && !g.onlyRead(name, -1) {
		return "", nil, "not-read-whole"
	}
	v, at, why := g.read(d, member, di)
	if why != "" {
		return "", nil, why
	}
	// The bindings are evidence too: re-pointing the import or the re-export changes what the name reads.
	return v, append(append(at, via...), spanOf(f.entry, f.data, imp.first, imp.last)), ""
}

// target reads the tracked file an import of module from path resolves to.
func (t *constTable) target(from, module string) (*constFile, string) {
	res := t.resolver.Resolve(from, module)
	if res.State != contextindex.WebImportRepository {
		return nil, "out-of-scope"
	}
	g := t.file(res.Target)
	if g == nil {
		return nil, "out-of-scope"
	}
	return g, ""
}

// reexported finds the file and local name of the table g exports as name through exactly one of
// its `export ... from` statements, whose module must declare it (AMAP-V0-025), and the anchor of
// that statement. A named re-export of name shadows every `export *`, as in ECMAScript; anything
// the reader cannot prove unique fails with its reason.
func (t *constTable) reexported(g *constFile, name string) (*constFile, string, []Anchor, string) {
	if g.opaque || g.unlisted {
		return nil, "", nil, "ambiguous-barrel"
	}
	if g.exports[name] {
		return nil, "", nil, "identifier-not-found" // a local export the reader does not follow shadows every re-export
	}
	var named, stars []reexport
	for _, r := range g.reexports {
		switch r.exported {
		case name:
			named = append(named, r)
		case "":
			stars = append(stars, r)
		}
	}
	if len(named) > 1 {
		return nil, "", nil, "ambiguous-barrel"
	}
	if len(named) == 0 && (name == "default" || len(stars) == 0) {
		return nil, "", nil, "identifier-not-found" // `export *` never re-exports a default
	}
	cands := named
	if len(named) == 0 {
		cands = stars
	}
	var hit *constFile
	var local string
	var via reexport
	found, deeper, unread := 0, false, false
	for _, r := range cands {
		h, why := t.target(g.entry.path, r.module)
		if why != "" {
			return nil, "", nil, why // a module the reader cannot see could export name
		}
		src := r.source
		if src == "" {
			src = name
		}
		if src == "*" {
			return nil, "", nil, "identifier-not-found" // a namespace object, not a table
		}
		l := src
		if src == "default" {
			l = h.dflt
		}
		switch d := h.decls[l]; {
		case d != nil && (src == "default" || d.exported):
			found, hit, local, via = found+1, h, l, r
		case src != "default" && (h.exports[src] || h.unlisted):
			found, unread = found+1, true // exported in a form the reader does not read as a table
		case h.mayReexport(src):
			deeper = true // h may re-export it from a further module
		}
	}
	switch {
	case found > 1:
		return nil, "", nil, "ambiguous-barrel"
	case deeper:
		return nil, "", nil, "barrel-depth-exceeded"
	case found == 0, unread:
		return nil, "", nil, "identifier-not-found"
	}
	return hit, local, []Anchor{spanOf(g.entry, g.data, via.first, via.last)}, ""
}

// mayReexport reports whether f could export name through one of its own re-exports.
func (f *constFile) mayReexport(name string) bool {
	for _, r := range f.reexports {
		if r.exported == name || r.exported == "" && name != "default" {
			return true
		}
	}
	return f.opaque
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
	v, at, why := f.read(d, name, false)
	return v, at, why == ""
}

// read is member with the reason it fails; an injected table (di) must be read whole, so an enum
// with any member that is not a literal string is no table (AMAP-V0-024).
func (f *constFile) read(d *constDecl, name string, di bool) (string, []Anchor, string) {
	if d.bad || di && d.partial {
		return "", nil, "non-literal-member"
	}
	m, ok := d.members[name]
	if !ok {
		return "", nil, "member-not-found"
	}
	return m.value, []Anchor{spanOf(f.entry, f.data, m.line, m.line)}, ""
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
		exports: map[string]bool{}, skip: make([]bool, len(toks)), reads: map[readKey]bool{}}
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
			if end, ok := f.readReexport(i); ok {
				f.mark(i, end)
				i = end - 1
				continue
			}
			if !exactExport(toks, i) {
				f.looseExport(i)
			}
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
			f.exports[name] = f.exports[name] || exported
			if !isPunct(toks[k], "=") || k+1 >= len(toks) {
				declare(name, &constDecl{bad: true}) // a type annotation: not read
				if exported {
					f.looseExport(start)
				}
				continue
			}
			k++
			frozen := k+4 < len(toks) && toks[k].text == "Object" && isPunct(toks[k+1], ".") && toks[k+2].text == "freeze" && isPunct(toks[k+3], "(")
			if frozen {
				k += 4
			}
			if !isPunct(toks[k], "{") {
				declare(name, &constDecl{bad: true})
				if exported {
					f.looseExport(start)
				}
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
			if exported && d.bad {
				f.looseExport(start) // a second declarator, or a table the reader does not read
			}
			declare(name, d)
			f.mark(start, i+2) // the header only: the initializer may alias another table
			i = after - 1
		case t.text == "enum" || (t.text == "const" && i+1 < len(toks) && toks[i+1].text == "enum"):
			if t.text == "const" {
				i++
			}
			if i+2 >= len(toks) || toks[i+1].kind != tokIdent || !isPunct(toks[i+2], "{") {
				if exported {
					f.looseExport(start)
				}
				continue
			}
			d, after := enumDecl(toks, i+2)
			d.exported = exported
			f.exports[toks[i+1].text] = f.exports[toks[i+1].text] || exported
			declare(toks[i+1].text, d)
			f.mark(start, i+2)
			i = after - 1
		case t.text == "let" || t.text == "var" || t.text == "const" || t.text == "function" || t.text == "class" || t.text == "async":
			k := i + 1
			if t.text == "async" && k < len(toks) && toks[k].kind == tokIdent && toks[k].text == "function" {
				k++
			}
			if k < len(toks) && (t.text == "function" || t.text == "async") && isPunct(toks[k], "*") {
				k++ // a generator
			}
			if k < len(toks) && toks[k].kind == tokIdent && (t.text != "async" || k > i+1) {
				declare(toks[k].text, &constDecl{bad: true})
				f.exports[toks[k].text] = f.exports[toks[k].text] || exported
			}
			if exported && (t.text == "let" || t.text == "var" || t.text == "const") {
				f.looseExport(start) // further declarators or a destructuring bind more names
			}
		}
	}
	f.unlisted = depth != 0
	return f
}

// exactExport reports whether the export statement at toks[i] is one whose exported names the
// parser reads exactly: a re-export or local list, a default export, an enum, a function or class,
// a type, or a declaration the declaration cases read (and widen through looseExport when they
// cannot). Any other form (`declare`, `abstract`, `namespace`, `export =`, ...) is read loosely.
func exactExport(toks []token, i int) bool {
	if i+1 >= len(toks) {
		return true
	}
	n := toks[i+1]
	if n.kind == tokPunct {
		return n.text == "{" || n.text == "*"
	}
	switch n.text {
	case "default", "enum", "function", "class", "interface", "type", "let", "var", "const":
		return n.kind == tokIdent
	case "async":
		return i+2 < len(toks) && toks[i+2].kind == tokIdent && toks[i+2].text == "function"
	}
	return false
}

// looseExport counts every identifier of the export statement at toks[i], up to its top-level `;`
// or the next top-level `export`, as a name the file may export (AMAP-V0-025): a form the reader
// does not read name by name must keep its uncertainty, never let a star source resolve.
func (f *constFile) looseExport(i int) {
	toks, depth := f.toks, 0
	for k := i + 1; k < len(toks); k++ {
		t := toks[k]
		if t.kind == tokPunct {
			switch t.text {
			case "{", "[", "(":
				depth++
			case "}", "]", ")":
				depth--
			case ";":
				if depth <= 0 {
					return
				}
			}
			continue
		}
		if t.kind != tokIdent {
			continue
		}
		if depth <= 0 && t.text == "export" && !isPunct(toks[k-1], ".") {
			return
		}
		f.exports[t.text] = true
	}
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
// absent and marks the enum partial, and anything the reader cannot follow makes the whole enum
// unreadable.
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
		literalInit := false
		if next(toks, k, "=") {
			if k+1 < end && literal(toks[k+1]) && next(toks, k+2, ",", "}") {
				d.members[key.text] = constMember{value: toks[k+1].text, line: toks[k+1].line}
				literalInit = true
			}
			k = skipValue(toks, k+1)
		}
		if !literalInit {
			d.partial = true // a numeric, computed or auto-numbered member (AMAP-V0-024)
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

// readReexport reads one `export * [as N] from 'm'` or `export { a [as b], ... } from 'm'` statement
// at toks[i] and returns the index after it; ok is false for any other export. A type-only
// statement or item exports no value; an item the reader cannot read marks the file opaque.
func (f *constFile) readReexport(i int) (int, bool) {
	toks := f.toks
	j := i + 1
	typeOnly := j+1 < len(toks) && toks[j].kind == tokIdent && toks[j].text == "type" && (isPunct(toks[j+1], "{") || isPunct(toks[j+1], "*"))
	if typeOnly {
		j++
	}
	items, opaque, list := []reexport{}, false, next(toks, j, "{")
	switch {
	case next(toks, j, "*"):
		j++
		if j+1 < len(toks) && toks[j].kind == tokIdent && toks[j].text == "as" && toks[j+1].kind == tokIdent {
			items = append(items, reexport{exported: toks[j+1].text, source: "*"})
			j += 2
		} else {
			items = append(items, reexport{})
		}
	case next(toks, j, "{"):
		end := closeParen(toks, j)
		for k := j + 1; k < end; k = skipValue(toks, k) + 1 {
			name := func(t token) bool { return t.kind == tokIdent || literal(t) }
			switch {
			case toks[k].text == "type" && k+1 < end && toks[k+1].kind == tokIdent && toks[k+1].text != "as":
				// a type-only item exports no value
			case name(toks[k]) && next(toks, k+1, ",", "}"):
				items = append(items, reexport{exported: toks[k].text, source: toks[k].text})
			case name(toks[k]) && k+3 <= end && toks[k+1].text == "as" && name(toks[k+2]) && next(toks, k+3, ",", "}"):
				items = append(items, reexport{exported: toks[k+2].text, source: toks[k].text})
			default:
				opaque = true
			}
		}
		j = end + 1
	default:
		return 0, false
	}
	if j+1 >= len(toks) || toks[j].kind != tokIdent || toks[j].text != "from" || !literal(toks[j+1]) {
		if list && !typeOnly {
			// a local export list: its names shadow every `export *` though the reader does not
			// follow them (AMAP-V0-025)
			for _, r := range items {
				f.exports[r.exported] = true
			}
			f.opaque = f.opaque || opaque
		}
		return 0, false // a local export list, or a statement this reader does not follow
	}
	if typeOnly {
		return j + 2, true
	}
	f.opaque = f.opaque || opaque
	for _, r := range items {
		r.module, r.first, r.last = toks[j+1].text, toks[i].line, toks[j+1].line
		f.reexports = append(f.reexports, r)
	}
	return j + 2, true
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
