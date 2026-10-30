package appmap

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
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
	// local holds the package names repository code may stand behind (see external); anyLocal
	// marks a manifest the reader cannot read, after which any package may.
	local    map[string]bool
	anyLocal bool
	// hop names the check of the last di lookup that failed (AMAP-V0-072); empty when none did.
	hop string
	// bind is the binding a bindingsRead refusal names (AMAP-V0-196); fail appends it to hop.
	bind string
	// reach memoizes reaches by (module, declaring file) path (AMAP-V0-203).
	reach map[[2]string]string
	// closureLimit is the manifest's di_closure_limit (AMAP-V0-369); 0 keeps maxImportClosure.
	closureLimit int
	// arrayUses caches proofs for the last examined closure, not for unseen consumers.
	arrayUses *arrayReach
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
	// hiddenAt holds the lines of every hidden name's substitutions (AMAP-V0-422).
	hiddenAt map[string][]int
	// hiddenCode marks a template substitution that is not one name, member chain or literal (see
	// substOpaque): it may inject or write a table where no token shows it.
	hiddenCode bool
	// spaces are the modules of the file's namespace imports, dynamic imports, `require` calls and
	// import items the reader cannot read, each of which may reach a table under another name; ""
	// is a module name that is not an exact literal.
	spaces []string
	// spaceLines holds each space's line (0 when no token shows it), for the binding a
	// not-read-whole refusal names (AMAP-V0-196).
	spaceLines []int
	// spaceWhat names the construct of each space whose module is not an exact literal (a loader
	// name, `import(`, `require(`, `import=`), "" when the reader has no name for it (AMAP-V0-365).
	spaceWhat []string
	// inject, depth and annot are the router-side injection reads (AMAP-V0-022), built on demand.
	inject        map[string]*diBinding
	depth         []int
	annot         map[string][]string
	annotAt       map[string][]int // the string tokens of each `$inject` list
	annotOK       bool
	annotBad      int                 // the line of the `$inject` annotations refused (AMAP-V0-202)
	annotBadLines []int               // every refused annotation, in token order (AMAP-V0-422)
	quiet         map[quietKey]string // memoized diQuiet
}

// quietKey memoizes diQuiet for one name, registration token, allowed token and module exemption.
type quietKey struct {
	name       string
	reg, allow int
	exempt     bool
	arrays     string // closure-specific non-injector array proofs
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
	return func(ref string, tok int) (string, []Anchor, string, bool) {
		local, member, ok := strings.Cut(ref, ".")
		if !ok {
			return "", nil, "", false
		}
		if own == nil {
			own = parseConstFile(e, data)
		}
		_, declared := own.decls[local]
		_, imported := own.imports[local]
		if !declared && !imported && t.di != nil {
			return t.injected(own, local, member, tok)
		}
		// A router's own table names why it did not read (AMAP-V0-205).
		v, at, why := t.lookup(own, local, member, -1, "")
		if why == "" {
			return v, at, "", true
		}
		if t.hop != "" {
			why += ":" + t.hop
		}
		return "", nil, "const:" + local + ":" + why, false
	}
}

// resolve reads local.member through a table f declares or imports; allow is the one token (or -1)
// that may pass the table along, the `.constant(...)` argument that registers it (AMAP-V0-022).
func (t *constTable) resolve(f *constFile, local, member string, allow int) (string, []Anchor, bool) {
	v, at, why := t.lookup(f, local, member, allow, "")
	return v, at, why == ""
}

// lookup is resolve with the reason it fails (AMAP-V0-026). A registration injected as inj (di)
// also reads an enum only when every member is a literal string (AMAP-V0-024), follows one level
// of re-export in the imported file (AMAP-V0-025), and requires every file the registering file f,
// the chain and the examined roots (the router) load to pass examined; a router's own tables keep
// the AMAP-V0-016 rules.
func (t *constTable) lookup(f *constFile, local, member string, allow int, inj string, roots ...*constFile) (string, []Anchor, string) {
	di := inj != ""
	t.hop, t.bind = "", ""
	if !f.onlyRead(local, allow) {
		return "", nil, t.fail("registering-file-use")
	}
	decl, declared := f.decls[local]
	imp, imported := f.imports[local]
	switch {
	case declared && imported, imported && imp.bad:
		t.hop = fmt.Sprintf("binding:%s:%d:%s", f.entry.path, imp.first, local)
		return "", nil, "ambiguous-binding" // two bindings for one name
	case declared && di && !t.bindingsRead(f, f, "", tableNames(f, local)...):
		return "", nil, t.fail("registering-file-bindings")
	case declared:
		v, at, why := f.read(decl, member, di)
		if why == "" && di && !t.examined(append(roots, f), []*constFile{f}, f, inj, tableNames(f, local)...) {
			return "", nil, t.fail("registering-file-examined")
		}
		return v, at, why
	case !imported:
		t.hop = "unbound:" + f.entry.path + ":" + local
		return "", nil, "identifier-not-found"
	case t.resolver == nil:
		return "", nil, "out-of-scope"
	}
	g, why := t.target(f.entry.path, imp.first, imp.module)
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
		if g, name, via, why = t.reexported(g, imp.exported, 1); why != "" {
			return "", nil, why
		}
		d = g.decls[name]
	case d == nil && len(g.reexports) > 0:
		// A router's own table follows `export ... from` chains (AMAP-V0-208); a barrel that
		// provably does not export the name keeps the not-exported detail below.
		r := t.starFollow(g, imp.exported, maxBarrelDepth)
		if r.why != "" && !r.absent {
			return "", nil, r.why
		}
		if !r.absent {
			g, name, via, d = r.hit, r.local, r.via, r.hit.decls[r.local]
			break
		}
		fallthrough
	case d == nil, imp.exported != "default" && !d.exported:
		t.hop = "not-exported:" + g.entry.path + ":" + imp.exported
		if d != nil && g.exports[imp.exported] {
			t.hop = "not-a-table:" + g.entry.path + ":" + imp.exported // exported, but not as a literal table
		}
		return "", nil, "identifier-not-found"
	}
	// An unread file never yields a declaration, an `export default {...}` object included.
	if g.unlisted || name != "default" && !g.onlyRead(name, -1) {
		return "", nil, t.fail("declaring-file-unread")
	}
	// Every file on the chain may hold the table under another binding (AMAP-V0-025).
	if di && !(t.bindingsRead(f, g, local, imp.exported) && t.bindingsRead(g, g, "", tableNames(g, name)...) &&
		(barrel == nil || t.bindingsRead(barrel, g, "", append(tableNames(g, name), imp.exported)...))) {
		return "", nil, t.fail("import-chain-bindings")
	}
	v, at, why := g.read(d, member, di)
	if why != "" {
		return "", nil, why
	}
	// No file on the chain, nor one any examined file loads, may inject or look up the name another
	// way, nor hold the table under a binding it may write.
	if di && !t.examined(append(roots, f, g, barrel), []*constFile{f, g, barrel}, g, inj, append(tableNames(g, name), imp.exported)...) {
		return "", nil, t.fail("import-chain-examined")
	}
	// The bindings are evidence too: re-pointing the import or the re-export changes what the name reads.
	return v, append(append(at, via...), spanOf(f.entry, f.data, imp.first, imp.last)), ""
}

// fail records the hop of a not-read-whole refusal and returns its reason. When a binding check
// refused, the hop also names that binding as `path:line:name` (AMAP-V0-196).
func (t *constTable) fail(hop string) string {
	t.hop = hop
	if t.bind != "" {
		t.hop += ":" + t.bind
	}
	return "not-read-whole"
}

// space records a module reference of f the reader does not read item by item (see spaces).
func (f *constFile) space(m string, line int) { f.spaceOf(m, line, "") }

// spaceOf records a module reference as space does, naming its construct (see spaceWhat).
func (f *constFile) spaceOf(m string, line int, what string) {
	f.spaces = append(f.spaces, m)
	f.spaceLines = append(f.spaceLines, line)
	f.spaceWhat = append(f.spaceWhat, what)
}

// spaceAt is the `path:line` of space i, followed by `:construct` when the reader names it
// (AMAP-V0-206, AMAP-V0-365).
func (f *constFile) spaceAt(i int) string {
	at := fmt.Sprintf("%s:%d", f.entry.path, f.spaceLines[i])
	if i < len(f.spaceWhat) && f.spaceWhat[i] != "" {
		at += ":" + f.spaceWhat[i]
	}
	return at
}

// bindingsRead reports whether every other binding f holds of a table that decl declares is only
// read (AMAP-V0-025). An import under a local other than skip must pass onlyRead when it imports
// one of names (an alias `import { T as U }`, from any module, so another table of the same name
// counts too) or whatever name it imports from a module that may hold the table (mayHold): the
// declaring file's default, a re-export alias along the chain, or any other export of it is then
// not provably a different value. No namespace import, dynamic import, `require` or unread import
// item may reach such a module at all.
//
// A refusal records the binding that failed in t.bind (`path:line:local` for an import,
// `path:line:module` for a module reference, "*" for one that is not an exact literal), reading
// the imports in name order so the first one named is the same on every run (AMAP-V0-196), and
// appends why that module may hold the table (`:why`, see mayHold; `table-name` for an import of
// one of names) (AMAP-V0-203).
func (t *constTable) bindingsRead(f, decl *constFile, skip string, names ...string) bool {
	for _, local := range slices.Sorted(maps.Keys(f.imports)) {
		imp := f.imports[local]
		if local == skip || f.onlyRead(local, -1) {
			continue
		}
		why := "table-name"
		if !slices.Contains(names, imp.exported) {
			why = t.mayHold(f, imp.module, decl)
		}
		if why != "" {
			t.bind = fmt.Sprintf("%s:%d:%s:%s", f.entry.path, imp.first, local, why)
			return false
		}
	}
	for i, m := range f.spaces {
		if why := t.mayHold(f, m, decl); why != "" {
			if m == "" {
				m = "*"
			}
			t.bind = fmt.Sprintf("%s:%d:%s:%s", f.entry.path, f.spaceLines[i], m, why)
			return false
		}
	}
	return true
}

// mayHold reports whether module m, imported by f, may export the table decl declares under some
// name, and why: decl itself, a file of the repository that re-exports or may pass on a binding it
// imports (relays: a barrel on the chain or any other) and whose module graph may reach decl (see
// reaches), or one the reader cannot read. A module name that is not an exact literal, a relative
// one the reader cannot resolve without an index, and an unresolved specifier could be any of
// them; a package cannot hold the table. The reason ("" when m cannot hold it) is the suffix a
// binding refusal names (AMAP-V0-203).
func (t *constTable) mayHold(f *constFile, m string, decl *constFile) string {
	if m == "" {
		return "module-not-literal"
	}
	if t.resolver == nil {
		if strings.HasPrefix(m, ".") || strings.HasPrefix(m, "/") {
			return "module-unresolved"
		}
		return ""
	}
	switch res := t.resolver.Resolve(f.entry.path, m); res.State {
	case contextindex.WebImportPackage:
		if t.external(m) {
			return ""
		}
		return "local-package"
	case contextindex.WebImportRepository:
		if assetSuffix[strings.ToLower(path.Ext(res.Target))] {
			return "" // an asset module's value is opaque data that holds no table (AMAP-V0-340)
		}
		h := t.file(res.Target)
		switch {
		case h == nil:
			return "module-unreadable:" + res.Target
		case h == decl:
			return "declaring-module"
		case h.unlisted || h.opaque:
			return "module-unread:" + res.Target
		case len(h.reexports) > 0 || t.relays(h):
			return t.reaches(h, decl)
		}
		return ""
	}
	return "module-unresolved"
}

// maxReachFiles bounds the module walk of one reaches question; a larger graph fails closed.
const maxReachFiles = 4096

// reaches reports why the module graph of h -- every import, re-export, namespace or dynamic
// import and `require` of h and, transitively, of each repository file they resolve to -- may
// reach decl's file, or "" when it provably cannot. A value moves between modules only through
// those references, so a relaying file whose graph never loads decl cannot export decl's table.
// Every module the walk cannot follow -- a name that is not an exact literal, an unresolved
// specifier, a package repository code may stand behind (see external), an unreadable or unread
// file, or a graph over maxReachFiles -- may reach it, and its reason names the hop
// (AMAP-V0-203). An external package cannot hold a repository table (see mayHold).
func (t *constTable) reaches(h, decl *constFile) string {
	key := [2]string{h.entry.path, decl.entry.path}
	if why, ok := t.reach[key]; ok {
		return why
	}
	why := t.walk(h, decl)
	if t.reach == nil {
		t.reach = map[[2]string]string{}
	}
	t.reach[key] = why
	return why
}

func (t *constTable) walk(h, decl *constFile) string {
	seen := map[string]bool{h.entry.path: true}
	queue := []*constFile{h}
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		for i, m := range x.modules() { // side-effect imports load a module too
			if m == "" { // spaces come first in modules: name the reference's line (AMAP-V0-206)
				return "reach-module-not-literal:" + x.spaceAt(i)
			}
			if t.resolver == nil {
				return "reach-unresolved:" + x.entry.path + ":" + m
			}
			res := t.resolver.Resolve(x.entry.path, m)
			switch res.State {
			case contextindex.WebImportPackage:
				if t.external(m) {
					continue
				}
				return "reach-local-package:" + x.entry.path + ":" + m
			case contextindex.WebImportRepository:
			default:
				return "reach-unresolved:" + x.entry.path + ":" + m
			}
			if seen[res.Target] || assetSuffix[strings.ToLower(path.Ext(res.Target))] {
				continue // a template, stylesheet or data module runs no code (AMAP-V0-340)
			}
			seen[res.Target] = true
			y := t.file(res.Target)
			switch {
			case y == nil:
				return "reach-unreadable:" + res.Target
			case y == decl:
				return "reaches-table:" + x.entry.path + ":" + m
			case y.unlisted || y.opaque:
				return "reach-unread:" + res.Target
			case len(seen) > maxReachFiles:
				return "reach-limit:" + h.entry.path
			}
			queue = append(queue, y)
		}
	}
	return ""
}

// relays reports whether f may pass on a value it imports, so that one of its exports may be
// another module's table. A file holds another module's value only through an import binding or a
// module reference, and a value reaches an export through any expression, local or member chain
// (`export default (NS.T)`, `const S = W.T; export { S }`, a function returning it), so this asks
// no question of the export forms or of how the binding is used: f relays when it exports or
// mentions a binding imported from a module that is not a package's anywhere outside its import
// statements (a member read of a namespace or of an object that
// holds the table is the table), holds a namespace import, dynamic import, `require` or unread
// import item that is not a package's, or re-exports anything.
func (t *constTable) relays(f *constFile) bool {
	for local, imp := range f.imports {
		if !t.pkg(f, imp.module) && (f.exports[local] || f.listed[local] || f.dflt == local || f.hidden[local] || f.mentions(local)) {
			return true
		}
	}
	for _, m := range f.spaces {
		if !t.pkg(f, m) {
			return true
		}
	}
	return len(f.reexports) > 0
}

// pkg reports whether module m, imported by f, is an external package's, which cannot hold a
// repository table (see external); a module name that is not an exact literal, or a relative
// one, could be any module.
func (t *constTable) pkg(f *constFile, m string) bool {
	if m == "" {
		return false
	}
	if t.resolver == nil {
		return !strings.HasPrefix(m, ".") && !strings.HasPrefix(m, "/")
	}
	return t.resolver.Resolve(f.entry.path, m).State == contextindex.WebImportPackage && t.external(m)
}

// external reports whether package specifier m names code outside the repository. The resolver's
// package state also covers workspace and nested packages and dependencies installed from the
// repository itself, so a package is external only when no tracked package.json names it (its
// own "name", which a workspace or a self-reference resolves to) and none installs or overrides
// anything from a local protocol or path (`file:`, `link:`, `workspace:`, `portal:`, `.`, `/`,
// or a value that is not a string), and every tracked package.json is readable. A scheme other
// than `node:` (a bundler's virtual module) may be generated from repository code.
func (t *constTable) external(m string) bool {
	if t.local == nil {
		t.readManifests()
	}
	if t.anyLocal {
		return false
	}
	if scheme, _, ok := strings.Cut(m, ":"); ok && !strings.Contains(scheme, "/") {
		return scheme == "node"
	}
	parts := strings.SplitN(m, "/", 3)
	name := parts[0]
	if strings.HasPrefix(m, "@") && len(parts) >= 2 {
		name += "/" + parts[1]
	}
	return !t.local[name]
}

// readManifests fills local from every tracked package.json (see external).
func (t *constTable) readManifests() {
	t.local = map[string]bool{}
	if t.ix == nil {
		return
	}
	for p, src := range t.ix.Sources {
		if path.Base(p) != "package.json" {
			continue
		}
		text, valid, loaded := src.Text()
		var fields map[string]json.RawMessage
		if !valid || !loaded || json.Unmarshal([]byte(text), &fields) != nil {
			t.anyLocal = true
			return
		}
		var name string
		if raw, ok := fields["name"]; ok && json.Unmarshal(raw, &name) == nil && name != "" {
			t.local[name] = true
		}
		for _, group := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
			var deps map[string]json.RawMessage
			if raw, ok := fields[group]; ok && json.Unmarshal(raw, &deps) != nil {
				t.anyLocal = true
				return
			}
			for dep, spec := range deps {
				if localSpec(spec) {
					t.local[dep] = true
				}
			}
		}
		// An override or resolution may point any package, at any depth, at repository code.
		for _, field := range []string{"overrides", "resolutions", "pnpm"} {
			if raw, ok := fields[field]; ok && localLeaf(raw) {
				t.anyLocal = true
				return
			}
		}
	}
}

// localSpec reports whether a dependency's version spec may install repository code: a local
// protocol or path, or a value that is not a string.
func localSpec(raw json.RawMessage) bool {
	var spec string
	return json.Unmarshal(raw, &spec) != nil || localPath(spec)
}

// localPath reports whether a version spec names a local protocol or path.
func localPath(spec string) bool {
	spec = strings.TrimSpace(spec)
	for _, prefix := range []string{"file:", "link:", "workspace:", "portal:", ".", "/"} {
		if strings.HasPrefix(spec, prefix) {
			return true
		}
	}
	return false
}

// localLeaf reports whether any string in raw, at any depth, is a local spec (see localSpec).
func localLeaf(raw json.RawMessage) bool {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return true
	}
	var walk func(any) bool
	walk = func(v any) bool {
		switch v := v.(type) {
		case string:
			return localPath(v)
		case map[string]any:
			for _, e := range v {
				if walk(e) {
					return true
				}
			}
		case []any:
			for _, e := range v {
				if walk(e) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}

// mentions reports whether name appears as a binding (not a property name) outside f's import
// statements and constant declaration headers.
func (f *constFile) mentions(name string) bool {
	for i, tok := range f.toks {
		if !f.skip[i] && tok.kind == tokIdent && tok.text == name && !property(f.toks, i) {
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

// target reads the tracked file an import of module, at line of path from, resolves to. A
// refusal names the hop as `import-<state>:from:line:module` (AMAP-V0-201).
func (t *constTable) target(from string, line int, module string) (*constFile, string) {
	res := t.resolver.Resolve(from, module)
	if res.State != contextindex.WebImportRepository {
		t.hop = fmt.Sprintf("import-%s:%s:%d:%s", res.State, from, line, module)
		return nil, "out-of-scope"
	}
	g := t.file(res.Target)
	if g == nil {
		t.hop = fmt.Sprintf("import-unreadable:%s:%d:%s", from, line, module)
		return nil, "out-of-scope"
	}
	return g, ""
}

// reexported finds the file and local name of the table g exports as name through exactly one of
// its `export ... from` statements, whose module must declare it (AMAP-V0-025), and the anchor of
// that statement. A named re-export of name shadows every `export *`, as in ECMAScript; anything
// the reader cannot prove unique fails with its reason, and names the barrel hop that refused in
// t.hop (AMAP-V0-201). An injected table follows one level; a router's own table follows up to
// maxBarrelDepth (AMAP-V0-208).
func (t *constTable) reexported(g *constFile, name string, depth int) (*constFile, string, []Anchor, string) {
	r := t.starFollow(g, name, depth)
	return r.hit, r.local, r.via, r.why
}

// barrelResult is one barrel's answer for a name: the declaring file, its local name and the
// re-export anchors, or why it refused and its hop. absent means the barrel provably does not
// export the name, so a star re-export reaching it contributes no binding; cut means a cycle on
// the current path was cut below it, so the answer holds only for that path.
type barrelResult struct {
	hit         *constFile
	local       string
	via         []Anchor
	why, hop    string
	absent, cut bool
}

// barrelWalk is one re-export walk (AMAP-V0-208): the (barrel, name) pairs on the current path,
// the settled answer of each (barrel, name, depth) whose walk cut no cycle, and the barrels read
// so far, bounded by maxBarrelVisits.
type barrelWalk struct {
	root   string
	stack  map[string]bool
	memo   map[string]barrelResult
	visits int
}

// starFollow is reexported that also reports absent, walking from g with a fresh barrelWalk.
func (t *constTable) starFollow(g *constFile, name string, depth int) barrelResult {
	w := &barrelWalk{root: g.entry.path, stack: map[string]bool{}, memo: map[string]barrelResult{}}
	return t.follow(w, g, name, depth)
}

// follow answers (g, name, depth) once per walk; an answer that cut a cycle depends on the path
// and is recomputed.
func (t *constTable) follow(w *barrelWalk, g *constFile, name string, depth int) barrelResult {
	key := fmt.Sprintf("%s\x00%s\x00%d", g.entry.path, name, depth)
	if r, ok := w.memo[key]; ok {
		t.hop = r.hop
		return r
	}
	r := t.followBarrel(w, g, name, depth)
	r.hop = t.hop
	if !r.cut {
		w.memo[key] = r
	}
	return r
}

// followBarrel reads one barrel. A (barrel, name) pair already on the path is a cycle and, as in
// ECMAScript, exports nothing through it; the same barrel reached under another name is not.
// Duplicate edges to one (module, name) are followed once. Two star sources that each export
// name, other than one declaration reached twice, are ambiguous.
func (t *constTable) followBarrel(w *barrelWalk, g *constFile, name string, depth int) barrelResult {
	at := g.entry.path
	if w.visits++; w.visits > maxBarrelVisits {
		t.hop = fmt.Sprintf("barrel-visit-limit:%s:%d", w.root, maxBarrelVisits)
		return barrelResult{why: "barrel-depth-exceeded"}
	}
	if g.opaque || g.unlisted {
		t.hop = "barrel-unlisted:" + at
		return barrelResult{why: "ambiguous-barrel"}
	}
	if g.exports[name] {
		t.hop = "barrel-local-export:" + at + ":" + name
		return barrelResult{why: "identifier-not-found"} // a local export the reader does not follow shadows every re-export
	}
	self := at + "\x00" + name
	w.stack[self] = true
	defer delete(w.stack, self)
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
		t.hop = fmt.Sprintf("barrel-named-reexports:%s:%d,%d", at, named[0].first, named[1].first)
		return barrelResult{why: "ambiguous-barrel"}
	}
	if len(named) == 0 && (name == "default" || len(stars) == 0) {
		t.hop = "barrel-no-reexport:" + at + ":" + name
		return barrelResult{why: "identifier-not-found", absent: true} // `export *` never re-exports a default
	}
	cands := named
	if len(named) == 0 {
		cands = stars
	}
	var hit *constFile
	var local string
	var via []Anchor
	found, deeper, unread, cut := 0, "", "", false
	holders := map[string]bool{} // distinct declarations, path:local, that export name
	edges := map[string]bool{}   // (module, name) pairs already followed from g
	for _, r := range cands {
		h, why := t.target(g.entry.path, r.first, r.module)
		if why != "" {
			return barrelResult{why: why, cut: cut} // a module the reader cannot see could export name
		}
		src := r.source
		if src == "" {
			src = name
		}
		if src == "*" {
			t.hop = fmt.Sprintf("barrel-namespace-reexport:%s:%d", at, r.first)
			return barrelResult{why: "identifier-not-found", cut: cut} // a namespace object, not a table
		}
		edge := h.entry.path + "\x00" + src
		if edges[edge] {
			continue
		}
		edges[edge] = true
		l := src
		if src == "default" {
			l = h.dflt
		}
		stmt := spanOf(g.entry, g.data, r.first, r.last)
		switch d := h.decls[l]; {
		case h.unlisted:
			found++ // an unread candidate may export or write name: never accept its declaration
			unread = cmp.Or(unread, h.entry.path)
		case d != nil && (src == "default" || d.exported):
			if key := h.entry.path + ":" + l; !holders[key] {
				holders[key] = true
				found, hit, local, via = found+1, h, l, []Anchor{stmt}
			}
		case src != "default" && (h.exports[src] || h.unlisted):
			found++ // exported in a form the reader does not read as a table
			unread = cmp.Or(unread, h.entry.path)
		case !h.mayReexport(src):
		case w.stack[edge]:
			cut = true // a cycle back to this (barrel, name) on the path exports nothing through it
		case depth <= 1:
			// h may re-export it from a further module
			deeper = cmp.Or(deeper, fmt.Sprintf("%s:%d:%s", at, r.first, h.entry.path))
		default:
			k := t.follow(w, h, src, depth-1)
			cut = cut || k.cut
			if k.absent {
				break
			}
			if k.why != "" {
				return barrelResult{why: k.why, cut: cut} // a deeper barrel may export name: its refusal stands
			}
			if key := k.hit.entry.path + ":" + k.local; !holders[key] {
				holders[key] = true
				found, hit, local, via = found+1, k.hit, k.local, append(slices.Clip(k.via), stmt)
			}
		}
	}
	switch {
	case found > 1:
		paths := make([]string, 0, len(holders))
		for k := range holders {
			paths = append(paths, k[:strings.LastIndexByte(k, ':')])
		}
		t.hop = "ambiguous-reexport:" + strings.Join(uniqueSorted(paths), ",")
		return barrelResult{why: "ambiguous-barrel", cut: cut}
	case deeper != "":
		t.hop = "barrel-depth:" + deeper
		return barrelResult{why: "barrel-depth-exceeded", cut: cut}
	case unread != "":
		t.hop = "barrel-candidate-unread:" + unread + ":" + name
		return barrelResult{why: "identifier-not-found", cut: cut}
	case found == 0:
		t.hop = "barrel-no-candidate:" + at + ":" + name
		return barrelResult{why: "identifier-not-found", absent: true, cut: cut}
	}
	t.hop = "" // an absent deeper barrel's hop is not this result's
	return barrelResult{hit: hit, local: local, via: via, cut: cut}
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
// only read, nor a name used inside a template substitution when any substitution of the file is
// opaque (see substOpaque): an escaped identifier, a body the reader skipped or such a
// substitution could write it. A substitution that is one name, member chain or literal only
// reads (AMAP-V0-204).
func (f *constFile) onlyRead(name string, allow int) bool {
	key := readKey{name, allow}
	if v, ok := f.reads[key]; ok {
		return v
	}
	ok := !f.unlisted && !(f.hidden[name] && f.hiddenCode) // a plain substitution only reads (AMAP-V0-204)
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

// skipAssertedType steps over the type of an `as` or `satisfies` assertion at toks[k] -- names,
// dotted names, literals, unions, intersections, arrays and type arguments (`string`, `const`,
// `keyof typeof X`, `Foo<'a' | B>[]`) -- and returns the index after it, or -1 for a type it does
// not read (AMAP-V0-204).
func skipAssertedType(toks []token, k int) int {
	depth, start := 0, k
	for ; k < len(toks); k++ {
		t := toks[k]
		switch {
		case t.kind == tokIdent || literal(t) || isPunct(t, ".") || isPunct(t, "|") || isPunct(t, "&"):
			if depth == 0 && k > start && t.kind == tokIdent && !isPunct(toks[k-1], ".") && !isPunct(toks[k-1], "|") &&
				!isPunct(toks[k-1], "&") && !word(toks, k-1, "typeof") && !word(toks, k-1, "keyof") {
				return k // a new statement after automatic semicolon insertion, or `instanceof`
			}
		case isPunct(t, "<") || isPunct(t, "["):
			depth++
		case isPunct(t, ">") || isPunct(t, "]"):
			if depth--; depth < 0 {
				return -1
			}
		case isPunct(t, ",") && depth > 0:
		case depth == 0 && k > start:
			return k
		default:
			return -1
		}
	}
	if depth != 0 || k == start {
		return -1
	}
	return k
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
			case "as", "satisfies":
				// A type assertion is the same value: step over its type and check what follows,
				// so `(X.Y as T) = v` still writes (AMAP-V0-204).
				if end = skipAssertedType(toks, end+1); end < 0 {
					return false
				}
				continue
			case "in", "of":
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
			if !next(toks, end+1, "=") {
				end++ // a non-null assertion `X.Y!`: the same value (AMAP-V0-204)
				continue
			}
			return next(toks, end+2, "=") // `!==`; `X.Y! = v` lexes as `X.Y != v`
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
		hidden: map[string]bool{}, hiddenAt: map[string][]int{}}
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
	loaderLine, loaderWhat := 0, "" // the first template substitution that names a module loader
	typeOnly := classFieldTypes(toks)
	for ti, t := range toks {
		if t.unsure {
			f.unread()
		}
		if (word(toks, ti, "import") || word(toks, ti, "require")) && !property(toks, ti) && next(toks, ti+1, "(") {
			what := ""
			m := moduleArg(toks, ti+2)
			if m == "" {
				what = t.text + "("
			}
			f.spaceOf(m, t.line, what) // a dynamic import or CommonJS require
		} else if !typeOnly[ti] && moduleLoader(toks, ti) {
			f.spaceOf("", t.line, t.text) // a module the reader cannot name
		}
		if t.kind != tokTemplate {
			continue
		}
		f.hiddenCode = f.hiddenCode || substOpaque(t.code) || taggedTemplate(toks, ti) && strings.Contains(t.code, "${")
		for k := 0; k < len(t.code); k++ {
			if !isIdentStart(t.code[k]) {
				continue
			}
			e := k + 1
			for e < len(t.code) && isIdentPart(t.code[e]) {
				e++
			}
			f.hidden[t.code[k:e]] = true
			if lines := f.hiddenAt[t.code[k:e]]; len(lines) == 0 || lines[len(lines)-1] != t.line {
				f.hiddenAt[t.code[k:e]] = append(lines, t.line)
			}
			if name := t.code[k:e]; loaderLine == 0 && (loaderName(name) || hiddenLoaders[name]) {
				loaderLine, loaderWhat = t.line, name
			}
			k = e - 1
		}
	}
	for name := range f.hidden {
		if loaderName(name) || hiddenLoaders[name] {
			f.spaceOf("", loaderLine, loaderWhat) // a module loader inside a template substitution
			break
		}
	}
	return f
}

// fieldModifiers are the TypeScript modifiers that may precede a class field's name.
var fieldModifiers = map[string]bool{"public": true, "private": true, "protected": true, "readonly": true,
	"static": true, "declare": true, "override": true, "abstract": true, "accessor": true}

// classFieldTypes marks the tokens of every type annotation of a class field (`name: Type`,
// `name?: Type`, `name!: Type`, after any fieldModifiers) directly in a class body, which the
// compiler erases and which therefore load no module (AMAP-V0-365): `onSuccess: Function` names
// the type, not the Function constructor. A field starts right after the body's `{`, a `;`, a
// member body's `}`, or on a new line after an identifier, literal, `)` or `]`. Its type runs to
// the first `;`, `=`, line break or closing bracket at its own bracket depth; an initializer after
// `=`, a method's parameters and any class whose heading is not plain names, `.`, `,` and `<...>`
// stay code, so the scan fails closed on what it does not recognize.
func classFieldTypes(toks []token) map[int]bool {
	var out map[int]bool
	for i := range toks {
		if !word(toks, i, "class") || property(toks, i) {
			continue
		}
		open := classBodyOpen(toks, i+1)
		if open < 0 {
			continue
		}
		depth := 1
		for k := open + 1; k < len(toks) && depth > 0; k++ {
			switch {
			case isPunct(toks[k], "(") || isPunct(toks[k], "[") || isPunct(toks[k], "{"):
				depth++
				continue
			case isPunct(toks[k], ")") || isPunct(toks[k], "]") || isPunct(toks[k], "}"):
				depth--
				continue
			}
			if depth != 1 || !memberStart(toks, k, open) {
				continue
			}
			n := k
			for toks[n].kind == tokIdent && fieldModifiers[toks[n].text] && n+1 < len(toks) && toks[n+1].kind == tokIdent {
				n++
			}
			if toks[n].kind != tokIdent {
				continue
			}
			c := n + 1
			if next(toks, c, "?") || next(toks, c, "!") {
				c++
			}
			if !next(toks, c, ":") {
				continue
			}
			for j, d := c+1, 0; j < len(toks); j++ {
				t := toks[j]
				if d == 0 && (isPunct(t, ";") || isPunct(t, "=") || t.line != toks[j-1].line) {
					break
				}
				if isPunct(t, "(") || isPunct(t, "[") || isPunct(t, "{") {
					d++
				} else if isPunct(t, ")") || isPunct(t, "]") || isPunct(t, "}") {
					if d == 0 {
						break
					}
					d--
				}
				if out == nil {
					out = map[int]bool{}
				}
				out[j] = true
			}
			k = c
		}
	}
	return out
}

// classBodyOpen returns the `{` opening the body of the class whose heading starts at toks[k], or
// -1 when the heading holds anything but names, `.`, `,` and `<...>` (see classFieldTypes).
func classBodyOpen(toks []token, k int) int {
	angle := 0
	for ; k < len(toks); k++ {
		t := toks[k]
		switch {
		case isPunct(t, "{") && angle == 0:
			return k
		case isPunct(t, "<"):
			angle++
		case isPunct(t, ">") && angle > 0:
			angle--
		case t.kind == tokIdent || isPunct(t, ".") || isPunct(t, ","):
		default:
			return -1
		}
	}
	return -1
}

// memberStart reports whether toks[k], directly in the class body opened at toks[open], starts a
// member (see classFieldTypes).
func memberStart(toks []token, k, open int) bool {
	p := toks[k-1]
	switch {
	case k-1 == open || isPunct(p, ";") || isPunct(p, "}"):
		return true
	case p.line == toks[k].line:
		return false
	}
	return p.kind == tokIdent && !continuingWords[p.text] || p.kind == tokString || p.kind == tokNumber || isPunct(p, ")") || isPunct(p, "]")
}

// continuingWords are the keywords after which an expression continues on the next line, so a
// line break after one starts no member (`f = c ? typeof\n a : Function`).
var continuingWords = map[string]bool{"typeof": true, "new": true, "void": true, "delete": true, "in": true,
	"instanceof": true, "await": true, "yield": true, "return": true, "case": true, "of": true, "keyof": true,
	"extends": true, "as": true, "satisfies": true, "is": true, "throw": true, "else": true, "do": true, "infer": true,
	"unique": true, "asserts": true}

// loaderNames are the names through which code may load, or reach the value of, another module
// without an import statement or a literal `require('m')` / `import('m')` call: CommonJS and
// bundler loaders, the main module, code evaluated from a string (which may itself import), and
// worker script loading. Any occurrence, as a binding, a property name (`module.require`,
// `process.mainModule.require`) or an exact string key (`globalThis['eval']`), makes the file
// load a module the reader cannot name (AMAP-V0-025 fails closed on it).
var loaderNames = map[string]bool{
	"require": true, "createRequire": true, "requirejs": true, "parcelRequire": true,
	"getBuiltinModule": true, "mainModule": true, "importScripts": true, "eval": true,
	"execScript": true, "Function": true, "setTimeout": true, "setInterval": true,
}

// hiddenLoaders are the context-dependent loaders of moduleLoader, counted inside a template
// substitution whatever their context, which the reader does not see.
var hiddenLoaders = map[string]bool{"import": true, "constructor": true, "module": true, "define": true, "System": true}

// loaderName reports whether name is a loader by itself (see loaderNames), including the
// generated module registries of webpack (`__webpack_require__`, `__webpack_modules__`,
// `webpackChunk...`, `webpackJsonp...`) and Vite's SSR transform (`__vite_ssr_import__`).
func loaderName(name string) bool {
	return loaderNames[name] || strings.HasPrefix(name, "__webpack_") || strings.HasPrefix(name, "__non_webpack_") ||
		strings.HasPrefix(name, "webpackChunk") || strings.HasPrefix(name, "webpackJsonp") || strings.HasPrefix(name, "__vite_ssr_")
}

// moduleLoader reports whether toks[i] may load or reach another module's value other than by an
// import statement, a re-export or a literal `require('m')` / `import('m')` call, which name
// their module: a loader name (see loaderName) anywhere, or an exact string key equal to one;
// `import.meta` and `x.import(...)` (`System.import`); `System.register`; a `.constructor`
// property (`({}).constructor.constructor` is Function); CommonJS `module` other than
// `module.exports` (`module.children`, `module.parent`); and an AMD `define(...)`.
func moduleLoader(toks []token, i int) bool {
	t := toks[i]
	if t.kind == tokString || t.kind == tokTemplate && !t.subst {
		return loaderName(t.text)
	}
	if t.kind != tokIdent {
		return false
	}
	switch t.text {
	case "setTimeout", "setInterval":
		return !timerCallback(toks, i) // a function callback is code the reader sees, not a string
	case "require":
		return property(toks, i) || !next(toks, i+1, "(") || moduleArg(toks, i+2) == ""
	case "import":
		return property(toks, i) || next(toks, i+1, ".")
	case "register":
		return property(toks, i) && word(toks, i-2, "System")
	case "constructor":
		return property(toks, i) && !staticMember(toks, i) && !constructorInert(toks, i)
	case "module":
		return !property(toks, i) && !(next(toks, i+1, ".") && word(toks, i+2, "exports"))
	case "define":
		return !property(toks, i) && next(toks, i+1, "(")
	}
	return loaderName(t.text)
}

// constructorReach are the members of a `.constructor` value that reach or call the constructor
// itself, so `x.constructor.constructor` (Function) or `x.constructor.call(...)` stays a loader.
var constructorReach = map[string]bool{"constructor": true, "call": true, "apply": true, "bind": true}

// staticMember reports whether the `.constructor` property at toks[i] is followed by a plain
// dotted member `.Y` other than constructorReach, as in `x.constructor.$delete(...)`: a static
// member of the object's class, which names no module and evaluates no string (AMAP-V0-209).
// The constructor value used bare, called, read through `?.` or a computed `[k]` stays a loader.
func staticMember(toks []token, i int) bool {
	return next(toks, i+1, ".") && i+2 < len(toks) && toks[i+2].kind == tokIdent && !constructorReach[toks[i+2].text]
}

// constructorInert reports whether the `.constructor` property at toks[i], ending a plain dotted
// chain of names (`x.constructor`, `this.constructor`), is only tested or constructed with no
// arguments (AMAP-V0-369): the left operand of `&&` (`x.constructor && x.constructor.ns`), which
// yields the value only when it is falsy; negated (`!x.constructor`) with nothing after it but
// `)`, `;`, `,`, `}`, `&` or `|`; or constructed with no arguments, `new x.constructor()` or `new
// (x.constructor)()` with an optional `as T` cast inside the parentheses, since `new Function()`
// evaluates no string. Any other use, a computed or optional chain included, stays a loader.
func constructorInert(toks []token, i int) bool {
	s := i - 1 // the chain's first name
	for s >= 1 && isPunct(toks[s], ".") && toks[s-1].kind == tokIdent {
		if s -= 1; s >= 1 && isPunct(toks[s-1], ".") {
			s--
			continue
		}
		break
	}
	if s < 0 || toks[s].kind != tokIdent || s == i {
		return false
	}
	glued := func(k int, text string) bool { return k < len(toks) && toks[k].glued && isPunct(toks[k], text) }
	switch {
	case next(toks, i+1, "&") && glued(i+2, "&") && !glued(i+3, "="):
		return true
	case s >= 1 && isPunct(toks[s-1], "!"):
		return next(toks, i+1, ")", ";", ",", "}", "&", "|") && !glued(i+2, "=")
	case s >= 1 && word(toks, s-1, "new") && !property(toks, s-1):
		return next(toks, i+1, "(") && next(toks, i+2, ")")
	case s >= 2 && isPunct(toks[s-1], "(") && word(toks, s-2, "new") && !property(toks, s-2):
		k := i + 1
		if word(toks, k, "as") && k+1 < len(toks) && toks[k+1].kind == tokIdent {
			k += 2
		}
		return next(toks, k, ")") && next(toks, k+1, "(") && next(toks, k+2, ")")
	}
	return false
}

// timerCallback reports whether toks[i] (setTimeout or setInterval) is called with a function
// literal first argument -- `function`, `async function`, `x =>` or `(...) =>`, without a return
// type -- which the timer calls instead of evaluating a string, so it loads no module
// (AMAP-V0-206). A `function` expression counts only when its body is followed by `,` or `)`:
// one called in place hands the timer its result. Any other form, the bare name included, stays
// a loader.
func timerCallback(toks []token, i int) bool {
	k := i + 2
	if !next(toks, i+1, "(") || k >= len(toks) {
		return false
	}
	if word(toks, k, "async") && !next(toks, k+1, ",", ")") {
		k++
	}
	arrow := func(j int) bool {
		return j+1 < len(toks) && isPunct(toks[j], "=") && isPunct(toks[j+1], ">") && toks[j+1].glued
	}
	switch {
	case word(toks, k, "function"):
		k++
		if next(toks, k, "*") {
			k++
		}
		if k < len(toks) && toks[k].kind == tokIdent {
			k++ // a named function expression
		}
		if !next(toks, k, "(") {
			return false
		}
		body := closeParen(toks, k) + 1
		if !next(toks, body, "{") {
			return false // a return type, or not a function body
		}
		return next(toks, closeParen(toks, body)+1, ",", ")")
	case next(toks, k, "("):
		return arrow(closeParen(toks, k) + 1)
	case k < len(toks) && toks[k].kind == tokIdent:
		return arrow(k + 1)
	}
	return false
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
				f.spaceOf("", toks[i].line, "import=") // `import T = NS.X`, or a form the reader does not follow
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
		f.space(m, toks[i].line)
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

// taggedTemplate reports whether the template at toks[i] follows a tag expression -- a name that
// is not an operator keyword, a member, a call or index result, or another template -- so that its
// substitution values are passed to a function the reader does not follow (AMAP-V0-204).
func taggedTemplate(toks []token, i int) bool {
	if i == 0 {
		return false
	}
	p := toks[i-1]
	switch {
	case p.kind == tokIdent:
		switch p.text {
		case "return", "typeof", "case", "in", "of", "yield", "await", "void", "delete", "throw", "instanceof", "else", "do":
			return property(toks, i-1)
		}
		return true
	case p.kind == tokTemplate:
		return true
	}
	return isPunct(p, ")") || isPunct(p, "]")
}
