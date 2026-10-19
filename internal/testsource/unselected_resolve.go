package testsource

import (
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// Framework resolution through local aliases, re-exports and `extend`-derived test bindings
// (TSA-V0-147): a test root bound as `const test = scenarioTest`, `base.extend(...)` or an
// imported fixture is followed, across committed relative modules, to the framework module it
// comes from.

const (
	// frameworkUnresolved attributes a file whose test or describe root is declared in the file
	// from something the reading cannot follow (TSA-V0-147).
	frameworkUnresolved = "unresolved"
	// ReasonFrameworkUnresolved is the uncertain reason of such a file.
	ReasonFrameworkUnresolved = "framework-unresolved"
	// maxResolveDepth bounds the modules one binding is followed through; maxResolveReads bounds
	// the committed modules one report reads for resolution.
	maxResolveDepth = 8
	maxResolveReads = 256
	maxAliasHops    = 16
)

// importedName is a binding taken from another module: its specifier and the name it exports
// there ("default" for a default import, "" for a namespace or a whole required module).
type importedName struct{ spec, name string }

// moduleTable is what one module binds and exports, read lexically.
type moduleTable struct {
	imports       map[string]importedName // local name → the import that binds it
	reexports     map[string]importedName // exported name → `export { … } from` source
	exportAliases map[string]string       // exported name → local name (`export { local as exported }`)
	aliases       map[string]string       // local name → the binding it is derived from
	declared      map[string]bool         // top-level names declared in the module
	stars         []string                // `export * from` specifiers
	defaultLocal  string                  // the binding `export default` names
}

// moduleResolver reads committed modules for framework resolution, within maxResolveReads.
type moduleResolver struct {
	read    func(name string) (string, bool)
	sources map[string]*source
	tables  map[string]moduleTable
	reads   int
}

func newRepoResolver(r Repo, commit string) *moduleResolver {
	return &moduleResolver{sources: map[string]*source{}, tables: map[string]moduleTable{}, read: func(name string) (string, bool) {
		data, found, err := r.readOne(commit, name, MaxFileBytes)
		return string(data), found && err == nil
	}}
}

// module returns the parsed committed module, or nil when it is absent, unreadable or the read
// budget is spent.
func (m *moduleResolver) module(name string) *source {
	if s, ok := m.sources[name]; ok {
		return s
	}
	if m.reads >= maxResolveReads {
		return nil
	}
	m.reads++
	var s *source
	if text, ok := m.read(name); ok {
		s = parseSource(name, text)
	}
	m.sources[name] = s
	if s != nil {
		m.tables[name] = s.moduleTable()
	}
	return s
}

// follow resolves an imported binding to the framework it comes from, or "" when it cannot.
func (m *moduleResolver) follow(from string, b importedName, depth int) string {
	if framework := frameworkModules[b.spec]; framework != "" {
		return framework
	}
	if otherFrameworkModules[b.spec] {
		return frameworkOther
	}
	if m == nil || depth >= maxResolveDepth || b.name == "" || !relativeSpecifier(b.spec) {
		return ""
	}
	for _, candidate := range importCandidates(from, b.spec) {
		if s := m.module(candidate); s != nil {
			return m.export(s, b.name, depth+1)
		}
	}
	return ""
}

// export resolves the binding a module exports under name.
func (m *moduleResolver) export(s *source, name string, depth int) string {
	t := m.tables[s.path]
	if b, ok := t.reexports[name]; ok {
		return m.follow(s.path, b, depth)
	}
	local := name
	if alias, ok := t.exportAliases[name]; ok {
		local = alias
	} else if name == "default" {
		local = t.defaultLocal
	}
	if local != "" {
		if b, ok := t.origin(local); ok {
			return m.follow(s.path, b, depth)
		}
	}
	if name == "default" {
		return ""
	}
	for _, spec := range t.stars {
		if framework := m.follow(s.path, importedName{spec, name}, depth); framework != "" {
			return framework
		}
	}
	return ""
}

// origin follows a local name through its aliases to the import that binds it.
func (t moduleTable) origin(name string) (importedName, bool) {
	for range maxAliasHops {
		if b, ok := t.imports[name]; ok {
			return b, true
		}
		next, ok := t.aliases[name]
		if !ok {
			return importedName{}, false
		}
		name = next
	}
	return importedName{}, false
}

// rooted follows a local name through its aliases and reports the name it ends on.
func (t moduleTable) rooted(name string) string {
	for range maxAliasHops {
		next, ok := t.aliases[name]
		if !ok {
			return name
		}
		name = next
	}
	return name
}

func relativeSpecifier(spec string) bool {
	return spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// moduleTable reads the module's imports, requires, exports and top-level aliases.
func (s *source) moduleTable() moduleTable {
	t := moduleTable{imports: map[string]importedName{}, reexports: map[string]importedName{},
		exportAliases: map[string]string{}, aliases: map[string]string{}, declared: map[string]bool{}}
	depth := s.bracketDepths()
	for i, tok := range s.tokens {
		if tok.Kind == appmap.JSString && tok.Literal {
			if r, ok := s.moduleBinding(i); ok && r[1] > r[0] {
				s.readBinding(&t, tok.Text, r)
			}
			continue
		}
		if depth[i] != 0 || s.memberAccess(i) {
			continue
		}
		switch {
		case s.ident(i, "const") || s.ident(i, "let") || s.ident(i, "var"):
			s.readDeclaration(&t, i)
		case s.ident(i, "function") || s.ident(i, "class"):
			if s.ident(i+1, "") {
				t.declared[s.tokens[i+1].Text] = true
			}
		case s.ident(i, "export") && s.punct(i+1, "{") && s.match[i+1] > i+1 && !s.ident(s.match[i+1]+1, "from"):
			for _, e := range s.namedEntries(i+1, "as") {
				t.exportAliases[e[1]] = e[0]
			}
		case s.ident(i, "export") && s.ident(i+1, "default"):
			if target, ok := s.aliasTarget(i + 2); ok {
				t.defaultLocal = target
			}
		}
	}
	return t
}

// readBinding records the names one import, re-export or require of spec binds; r is the token
// range moduleBinding returns.
func (s *source) readBinding(t *moduleTable, spec string, r [2]int) {
	keyword := r[0] - 1
	switch {
	case s.ident(keyword, "import"):
		k := r[0]
		if s.ident(k, "type") && !s.punct(k+1, ",") && !s.ident(k+1, "from") {
			return
		}
		if s.ident(k, "") {
			t.imports[s.tokens[k].Text] = importedName{spec, "default"}
			k++
			if s.punct(k, ",") {
				k++
			}
		}
		switch {
		case s.punct(k, "*") && s.ident(k+1, "as") && s.ident(k+2, ""):
			t.imports[s.tokens[k+2].Text] = importedName{spec, ""}
		case s.punct(k, "{"):
			for _, e := range s.namedEntries(k, "as") {
				t.imports[e[1]] = importedName{spec, e[0]}
			}
		}
	case s.ident(keyword, "export"):
		k := r[0]
		switch {
		case s.punct(k, "*") && s.ident(k+1, "as") && s.ident(k+2, ""):
			t.reexports[s.tokens[k+2].Text] = importedName{spec, ""}
		case s.punct(k, "*"):
			t.stars = append(t.stars, spec)
		case s.punct(k, "{"):
			for _, e := range s.namedEntries(k, "as") {
				t.reexports[e[1]] = importedName{spec, e[0]}
			}
		}
	default: // const|let|var … = require("m")
		k := r[0]
		switch {
		case s.ident(k, "") && s.punct(k+1, "="):
			t.imports[s.tokens[k].Text] = importedName{spec, ""}
		case s.punct(k, "{"):
			for _, e := range s.namedEntries(k, ":") {
				t.imports[e[1]] = importedName{spec, e[0]}
			}
		}
	}
}

// readDeclaration records the names a top-level `const|let|var` at i declares and, for
// `name = target` or `name = target.extend(…)`, the alias.
func (s *source) readDeclaration(t *moduleTable, i int) {
	switch k := i + 1; {
	case s.ident(k, ""):
		t.declared[s.tokens[k].Text] = true
		if s.punct(k+1, "=") && !s.punct(k+2, "=") && !s.punct(k+2, ">") {
			if target, ok := s.aliasTarget(k + 2); ok {
				t.aliases[s.tokens[k].Text] = target
			}
		}
	case (s.punct(k, "{") || s.punct(k, "[")) && s.match[k] > k:
		for j := k + 1; j < s.match[k]; j++ {
			if s.ident(j, "") && !s.punct(j+1, ":") && !s.memberAccess(j) {
				t.declared[s.tokens[j].Text] = true
			}
		}
	}
}

// aliasTarget reads an alias value at k: an identifier ending the statement, or one whose
// `.extend(` or `.extend<` call derives the value.
func (s *source) aliasTarget(k int) (string, bool) {
	if !s.ident(k, "") || jsNotAliasWords[s.tokens[k].Text] {
		return "", false
	}
	next := k + 1
	switch {
	case next >= len(s.tokens) || s.punct(next, ";") || s.punct(next, ","):
	case s.punct(next, ".") && s.ident(next+1, "extend") && (s.punct(next+2, "(") || s.punct(next+2, "<")):
	case s.tokens[next].Line != s.tokens[k].Line && s.ident(next, "") && !jsContinuationWords[s.tokens[next].Text]:
	default:
		return "", false
	}
	return s.tokens[k].Text, true
}

// jsNotAliasWords are identifiers that start an expression rather than name a binding.
var jsNotAliasWords = map[string]bool{"new": true, "await": true, "function": true, "class": true, "async": true,
	"require": true, "import": true, "typeof": true, "void": true, "this": true, "null": true, "undefined": true,
	"true": true, "false": true, "yield": true}

// namedEntries reads a `{ a, b as c }` (sep "as") or `{ a, b: c }` (sep ":") list at open into
// (source name, local name) pairs; type-only and nested entries are skipped.
func (s *source) namedEntries(open int, sep string) [][2]string {
	out := [][2]string{}
	close := s.match[open]
	if close <= open {
		return out
	}
	name := func(k int) (string, bool) {
		if s.ident(k, "") {
			return s.tokens[k].Text, true
		}
		if k < len(s.tokens) && s.tokens[k].Kind == appmap.JSString && s.tokens[k].Literal {
			return s.tokens[k].Text, true
		}
		return "", false
	}
	for _, entry := range s.splitArgs(open, close) {
		a, b := entry[0], entry[1]
		if b-a > 1 && s.ident(a, "type") {
			a++
		}
		first, ok := name(a)
		switch {
		case !ok:
		case b-a == 1:
			out = append(out, [2]string{first, first})
		case b-a == 3 && (sep == "as" && s.ident(a+1, "as") || sep == ":" && s.punct(a+1, ":")) && s.ident(a+2, ""):
			out = append(out, [2]string{first, s.tokens[a+2].Text})
		}
	}
	return out
}

// bracketDepths returns, for every token, how many brackets enclose it.
func (s *source) bracketDepths() []int {
	depths := make([]int, len(s.tokens))
	depth := 0
	for i := range s.tokens {
		if s.punct(i, ")") || s.punct(i, "]") || s.punct(i, "}") {
			depth = max(depth-1, 0)
		}
		depths[i] = depth
		if s.punct(i, "(") || s.punct(i, "[") || s.punct(i, "{") {
			depth++
		}
	}
	return depths
}
