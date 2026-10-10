package appmap

import (
	"path"
	"slices"
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
	// marks a file whose exports the reader cannot list (see unread), so it may export any name;
	// claimed holds the `export` tokens a reader consumed, for that check.
	exports  map[string]bool
	listed   map[string]bool // the local names of `export { local as name }` lists
	unlisted bool
	claimed  map[int]bool
	skip     []bool // tokens of import and re-export statements and constant declaration headers
	reads    map[readKey]bool
	// hidden holds every identifier inside a template substitution, which no token shows: a name
	// there has a use the reader cannot check.
	hidden map[string]bool
	// spaces are the modules of the file's namespace imports, dynamic imports, `require` calls and
	// import items the reader cannot read, each of which may reach a table under another name; ""
	// is a module name that is not an exact literal.
	spaces []string
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
	case declared && di && !t.bindingsRead(f, f, "", tableNames(f, local)...):
		return "", nil, "not-read-whole"
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
	var barrel *constFile
	switch {
	case d == nil && di:
		barrel = g
		if g, name, via, why = t.reexported(g, imp.exported); why != "" {
			return "", nil, why
		}
		d = g.decls[name]
	case d == nil, imp.exported != "default" && !d.exported:
		return "", nil, "identifier-not-found"
	}
	// An unread file never yields a declaration, an `export default {...}` object included.
	if g.unlisted || name != "default" && !g.onlyRead(name, -1) {
		return "", nil, "not-read-whole"
	}
	// Every file on the chain may hold the table under another binding (AMAP-V0-025).
	if di && !(t.bindingsRead(f, g, local, imp.exported) && t.bindingsRead(g, g, "", tableNames(g, name)...) &&
		(barrel == nil || t.bindingsRead(barrel, g, "", append(tableNames(g, name), imp.exported)...))) {
		return "", nil, "not-read-whole"
	}
	v, at, why := g.read(d, member, di)
	if why != "" {
		return "", nil, why
	}
	// The bindings are evidence too: re-pointing the import or the re-export changes what the name reads.
	return v, append(append(at, via...), spanOf(f.entry, f.data, imp.first, imp.last)), ""
}

// bindingsRead reports whether every other binding f holds of a table that decl declares is only
// read (AMAP-V0-025). An import under a local other than skip must pass onlyRead when it imports
// one of names (an alias `import { T as U }`, from any module, so another table of the same name
// counts too) or whatever name it imports from a module that may hold the table (mayHold): the
// declaring file's default, a re-export alias along the chain, or any other export of it is then
// not provably a different value. No namespace import, dynamic import, `require` or unread import
// item may reach such a module at all.
func (t *constTable) bindingsRead(f, decl *constFile, skip string, names ...string) bool {
	for local, imp := range f.imports {
		if local != skip && (slices.Contains(names, imp.exported) || t.mayHold(f, imp.module, decl)) &&
			!f.onlyRead(local, -1) {
			return false
		}
	}
	for _, m := range f.spaces {
		if t.mayHold(f, m, decl) {
			return false
		}
	}
	return true
}

// mayHold reports whether module m, imported by f, may export the table decl declares under some
// name: decl itself, a file of the repository that re-exports or may pass on a binding it imports
// (relays: a barrel on the chain or any other), or one the reader cannot read. A module name that is not an
// exact literal, a relative one the reader cannot resolve without an index, and an unresolved
// specifier could be any of them; a package cannot hold the table.
func (t *constTable) mayHold(f *constFile, m string, decl *constFile) bool {
	if m == "" {
		return true
	}
	if t.resolver == nil {
		return strings.HasPrefix(m, ".") || strings.HasPrefix(m, "/")
	}
	switch res := t.resolver.Resolve(f.entry.path, m); res.State {
	case contextindex.WebImportPackage:
		return false
	case contextindex.WebImportRepository:
		h := t.file(res.Target)
		return h == nil || h == decl || h.unlisted || h.opaque || len(h.reexports) > 0 || t.relays(h)
	}
	return true
}

// relays reports whether f may pass on a value it imports, so that one of its exports may be
// another module's table. A file holds another module's value only through an import binding or a
// module reference, so this asks no question of the export forms: any import binding f exports by
// name (`export { T }`, `export default T`) or uses other than as a provable member read (an
// alias, an argument, `export default (T)`, `T as X`, any default-export or other expression the
// reader does not prove a read) relays, and so does an `export default T ...` expression that
// starts with it (`export default T || {}`), and any namespace import, dynamic import, `require`
// or unread import item that is not a package's.
func (t *constTable) relays(f *constFile) bool {
	for local := range f.imports {
		if f.exports[local] || f.listed[local] || f.dflt == local || !f.onlyRead(local, -1) {
			return true
		}
	}
	for i, tok := range f.toks {
		if _, imported := f.imports[tok.text]; imported && tok.kind == tokIdent && !f.skip[i] &&
			word(f.toks, i-1, "default") && word(f.toks, i-2, "export") {
			return true
		}
	}
	for _, m := range f.spaces {
		if m == "" || t.resolver == nil || t.resolver.Resolve(f.entry.path, m).State != contextindex.WebImportPackage {
			return true
		}
	}
	return false
}

// tableNames are the names an import of the table g declares as name binds: name, and "default"
// when g exports it as its default.
func tableNames(g *constFile, name string) []string {
	if name == "default" || g.dflt == name {
		return []string{name, "default"}
	}
	return []string{name}
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
		case h.unlisted:
			found, unread = found+1, true // an unread candidate may export or write name: never accept its declaration
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
// is provably only read (see readUse) and is a member read `name.member`, a `typeof name`, an
// `export { name }` / `export default name`, or the token at allow (or none, -1), which registers
// the table for injection. The write checks run on every use first; these forms only narrow what
// counts as a read. Any other use could mutate or rebind the table. Nothing in an unread file is
// only read, nor a name used inside a template substitution: an escaped identifier, a body the
// reader skipped or a substitution could write it.
func (f *constFile) onlyRead(name string, allow int) bool {
	key := readKey{name, allow}
	if v, ok := f.reads[key]; ok {
		return v
	}
	ok := !f.unlisted && !f.hidden[name]
	toks := f.toks
	for i := 0; i < len(toks) && ok; i++ {
		if f.skip[i] || toks[i].kind != tokIdent || toks[i].text != name || property(toks, i) {
			continue
		}
		read, member := readUse(toks, i)
		ok = read && (member || i == allow || word(toks, i-1, "typeof") ||
			word(toks, i-1, "default") && word(toks, i-2, "export") || inExportList(toks, i))
	}
	f.reads[key] = ok
	return ok
}

// word reports whether toks[k] is the identifier or keyword w.
func word(toks []token, k int, w string) bool {
	return k >= 0 && k < len(toks) && toks[k].kind == tokIdent && toks[k].text == w
}

// property reports whether toks[i] is a property name after `.` (`a.X`, `a?.X`), not a spread
// `...X`, which uses the binding.
func property(toks []token, i int) bool {
	return i > 0 && isPunct(toks[i-1], ".") && !(i > 1 && isPunct(toks[i-2], "."))
}

// readUse reports whether the use of a binding at toks[i] is provably only read, and whether it is
// a member read `X.Y`. A computed member `X[k]` is checked as one expression through its `]`; the
// caller decides whether a bare or computed use may count as a read at all.
func readUse(toks []token, i int) (read, member bool) {
	switch {
	case i+2 < len(toks) && isPunct(toks[i+1], ".") && toks[i+2].kind == tokIdent:
		return pureRead(toks, i, i+3), true
	case next(toks, i+1, "["):
		c := enclosingClose(toks, i+1)
		return c > 0 && isPunct(toks[c], "]") && pureRead(toks, i, c+1), false
	case next(toks, i+1, "."):
		return false, false // `X.#p`, `X..`: a member the rule does not place
	}
	return pureRead(toks, i, i+1), false
}

// pureRead reports whether the expression toks[start:end] (`X.Y`, `X[k]` or `X`) is provably only read. The
// rule is structural and fails closed instead of naming write forms: the expression, and then each
// bracket group it is an element of (climbing through `,` and closing brackets, which is how
// parenthesized targets and destructuring patterns enclose it), must be followed by a token of a
// closed set that ends or continues an expression without assigning to it -- `;` or the end of the
// file, a member access `.`/`[`, `:`, `{`, an identifier that starts a new statement or is
// `instanceof`, `==`/`===`/`!==`, or a binary operator other than `?` that assigned does not read
// as an assignment or increment -- and no `delete`, `++` or `--` may appear anywhere between the
// start of the statement and the expression outside a complete bracket group (see prefixWrite),
// so a prefix type assertion (`delete <any>X.Y`, `delete <{}>X.Y`) cannot hide one.
// Everything else counts as a possible write: `=` and compound assignments, `++`/`--`, a
// TypeScript assertion (`!`, `as`, `satisfies`, so `X.Y! = v` and `(X.Y as T) = v`), a for-in/of
// head (`in`, `of`), a call, optional call (`?`, `?.`) or tagged template through the table
// (`this` is X), a `<` that may open type arguments, and any token not listed. A `,` or `;`
// reached at statement level (no enclosing bracket) ends the climb as a read.
func pureRead(toks []token, start, end int) bool {
	for {
		if prefixWrite(toks, start) {
			return false
		}
		if end >= len(toks) {
			return true
		}
		t := toks[end]
		if t.kind == tokIdent {
			switch t.text {
			case "as", "satisfies", "in", "of":
				return false
			}
			return true // `instanceof`, or a new statement after automatic semicolon insertion
		}
		if t.kind != tokPunct {
			return false // a tagged template, or a token the rule does not place
		}
		switch t.text {
		case ";", ".", "[", "{", ":":
			return true
		case "=":
			return next(toks, end+1, "=") // `==`, `===`
		case "!":
			return next(toks, end+1, "=") && next(toks, end+2, "=") // `!==`; `X.Y! = v` lexes as `X.Y != v`
		case ",", ")", "]", "}":
			c := enclosingClose(toks, end)
			if c < 0 {
				return true // a `,` or `;` at statement level: no pattern or parenthesized target encloses it
			}
			o := enclosingOpen(toks, start)
			if o < 0 {
				return false // brackets the rule cannot match
			}
			start, end = o, c+1
			continue
		}
		if strings.Contains("+-*/%&|^>", t.text) {
			return !assigned(toks, end)
		}
		return false // `(`, `<`, `?`, and any other punctuator
	}
}

// prefixWrite reports whether `delete`, `++` or `--` appears before toks[start] in its statement
// outside every complete bracket group that precedes it there. The backward scan skips each
// balanced `(...)`, `[...]` and `{...}` group (an operand that is already whole: the `{}` of
// `delete <{}>X.Y`, or a block) and stops only at a `;` or an unmatched `{` outside every skipped
// group, or at the start of the file; it climbs out through an unmatched `(` or `[`, which encloses
// the expression. A `<...>` needs no matching of its own: any bracket inside type arguments nests,
// and a `;` there sits inside `{...}`. Brackets the scan cannot match fail closed.
func prefixWrite(toks []token, start int) bool {
	var want []string // the opener each skipped closer needs, innermost last
	for p := start - 1; p >= 0; p-- {
		t := toks[p]
		if t.kind == tokPunct {
			switch t.text {
			case ")", "]", "}":
				want = append(want, map[string]string{")": "(", "]": "[", "}": "{"}[t.text])
				continue
			case "(", "[", "{":
				if n := len(want); n > 0 {
					if want[n-1] != t.text {
						return true // brackets that do not match
					}
					want = want[:n-1]
				} else if t.text == "{" {
					return false // the block or object literal that holds the statement
				}
				continue
			case ";":
				if len(want) == 0 {
					return false
				}
				continue
			}
		}
		if len(want) == 0 && (t.kind == tokIdent && t.text == "delete" ||
			p >= 1 && (isPunct(t, "+") && isPunct(toks[p-1], "+") || isPunct(t, "-") && isPunct(toks[p-1], "-"))) {
			return true
		}
	}
	return len(want) > 0 // a closer with no opener
}

// enclosingClose returns the closing bracket of the innermost group enclosing toks[k], which is
// that bracket or a `,` inside the group, or -1 when a `;` at the same level or the end of the
// file comes first.
func enclosingClose(toks []token, k int) int {
	if isPunct(toks[k], ")") || isPunct(toks[k], "]") || isPunct(toks[k], "}") {
		return k
	}
	depth := 0
	for k++; k < len(toks); k++ {
		switch {
		case isPunct(toks[k], "(") || isPunct(toks[k], "[") || isPunct(toks[k], "{"):
			depth++
		case isPunct(toks[k], ")") || isPunct(toks[k], "]") || isPunct(toks[k], "}"):
			if depth == 0 {
				return k
			}
			depth--
		case isPunct(toks[k], ";") && depth == 0:
			return -1
		}
	}
	return -1
}

// enclosingOpen returns the opening bracket of the innermost group enclosing toks[k], or -1.
func enclosingOpen(toks []token, k int) int {
	depth := 0
	for k--; k >= 0; k-- {
		switch {
		case isPunct(toks[k], ")") || isPunct(toks[k], "]") || isPunct(toks[k], "}"):
			depth++
		case isPunct(toks[k], "(") || isPunct(toks[k], "[") || isPunct(toks[k], "{"):
			if depth == 0 {
				return k
			}
			depth--
		}
	}
	return -1
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
		exports: map[string]bool{}, listed: map[string]bool{}, claimed: map[int]bool{}, skip: make([]bool, len(toks)), reads: map[readKey]bool{},
		hidden: map[string]bool{}}
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
			switch {
			case !exactExport(toks, i):
				f.looseExport(i)
			case i+2 < len(toks) && (toks[i+1].text == "type" || toks[i+1].text == "interface") && toks[i+2].kind == tokIdent:
				f.claimed[i] = true // a type exports no value
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
			f.claimed[start] = true // `export *` never re-exports a default
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
			f.claimed[start] = exported
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
			f.claimed[start] = exported
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
				f.claimed[start] = exported
			}
			if exported && (t.text == "let" || t.text == "var" || t.text == "const") {
				f.looseExport(start) // further declarators or a destructuring bind more names
			}
		}
	}
	f.auditExports()
	// The lexer does not read JSX: text between tags (`<p>don't</p>`) would open a string that
	// hides code, so a JSX-capable file with any `<` is unread.
	if ext := strings.ToLower(path.Ext(e.path)); ext == ".tsx" || ext == ".jsx" {
		for _, t := range toks {
			if isPunct(t, "<") {
				f.unread()
				break
			}
		}
	}
	for ti, t := range toks {
		if t.unsure {
			f.unread()
		}
		if (word(toks, ti, "import") || word(toks, ti, "require")) && !property(toks, ti) && next(toks, ti+1, "(") {
			f.spaces = append(f.spaces, moduleArg(toks, ti+2)) // a dynamic import or CommonJS require
		}
		if t.kind != tokTemplate {
			continue
		}
		for k := 0; k < len(t.code); k++ {
			if !isIdentStart(t.code[k]) {
				continue
			}
			e := k + 1
			for e < len(t.code) && isIdentPart(t.code[e]) {
				e++
			}
			f.hidden[t.code[k:e]] = true
			k = e - 1
		}
	}
	return f
}

// unread marks f as a file whose exports the reader cannot list, so it may export any name
// (AMAP-V0-025): every reader path that rejects, skips or cannot decode part of an export
// statement ends here, and an injected table never resolves through such a file by elimination.
func (f *constFile) unread() {
	f.unlisted = true
}

// auditExports checks f by one whole-file token pass, independent of the declaration readers
// (whose skipped bodies could hide what follows): brackets that do not nest and match, a backslash
// outside a string (an escaped identifier the lexer splits), or a top-level `export` that no
// reader consumed makes the file unread.
func (f *constFile) auditExports() {
	var open []string // the closer each open bracket expects
	for k, t := range f.toks {
		switch {
		case t.kind == tokPunct && (t.text == "{" || t.text == "[" || t.text == "("):
			open = append(open, map[string]string{"{": "}", "[": "]", "(": ")"}[t.text])
		case t.kind == tokPunct && (t.text == "}" || t.text == "]" || t.text == ")"):
			if len(open) == 0 || open[len(open)-1] != t.text {
				f.unread() // a stray or mismatched closer
				return
			}
			open = open[:len(open)-1]
		case t.kind == tokPunct && t.text == "\\":
			f.unread()
		case t.kind == tokIdent && t.text == "export" && len(open) == 0 && (k == 0 || !isPunct(f.toks[k-1], ".")) && !f.claimed[k]:
			f.unread()
		}
	}
	if len(open) != 0 {
		f.unread()
	}
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
	f.claimed[i] = true
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
	items, opaque, list, quoted := []reexport{}, false, next(toks, j, "{"), false
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
				quoted = quoted || toks[k].kind != tokIdent
			case name(toks[k]) && k+3 <= end && toks[k+1].text == "as" && name(toks[k+2]) && next(toks, k+3, ",", "}"):
				items = append(items, reexport{exported: toks[k+2].text, source: toks[k].text})
				quoted = quoted || toks[k+2].kind != tokIdent
			default:
				opaque = true
			}
		}
		j = end + 1
	default:
		return 0, false
	}
	from := j < len(toks) && toks[j].kind == tokIdent && toks[j].text == "from"
	if !from && list {
		// a local export list: its names shadow every `export *` though the reader does not
		// follow them (AMAP-V0-025)
		f.claimed[i] = true
		if !typeOnly {
			for _, r := range items {
				f.exports[r.exported], f.listed[r.source] = true, true
			}
			f.opaque = f.opaque || opaque
			if quoted {
				f.unread() // a string export name
			}
		}
		return 0, false
	}
	if !from || j+1 >= len(toks) || !literal(toks[j+1]) {
		f.unread() // `export * as "N"`, or a module name the lexer cannot decode exactly
		return 0, false
	}
	f.claimed[i] = true
	if typeOnly {
		return j + 2, true
	}
	if quoted {
		f.unread() // a string export name
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
	items, space := []item{}, false
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
				default:
					space = true // a string import name, or an item the reader cannot read
				}
			}
			j = end + 1
		case isPunct(t, "*"):
			space = true
			j += 3 // `* as NS`
		case t.kind == tokIdent:
			items = append(items, item{t.text, "default"})
			j++
		default:
			if !typeOnly && !(isPunct(t, "=") && word(toks, j+1, "require")) { // the require scan reads that one
				f.spaces = append(f.spaces, "") // `import T = NS.X`, or a form the reader does not follow
			}
			return j
		}
		if next(toks, j, ",") {
			j++
		}
	}
	if space && !typeOnly {
		m := ""
		if j+1 < len(toks) && literal(toks[j+1]) {
			m = toks[j+1].text
		}
		f.spaces = append(f.spaces, m)
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

// moduleArg returns the module a call `import(...)` or `require(...)` whose argument starts at
// toks[k] names, or "" when the argument is not one exact literal.
func moduleArg(toks []token, k int) string {
	if k < len(toks) && literal(toks[k]) && next(toks, k+1, ")") {
		return toks[k].text
	}
	return ""
}
