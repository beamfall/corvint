package contextindex

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"sort"
	"strings"
)

// SetupSelectSchema names the `corvint tests setup-select` result (TSA-V0-001).
const SetupSelectSchema = "corvint-tests-setup-select/0"

// maxSetupReasons caps the reasons an ALL result lists; it shortens a list and claims nothing.
const maxSetupReasons = 100

// SetupSelectReason is one typed cause of an ALL result (TSA-V0-003).
type SetupSelectReason struct {
	Code   string `json:"code"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// SetupLink is one step of an inclusion chain: the file reached and the edge that reached it from
// the previous step ("spec" for the chain's start) (TSA-V0-647).
type SetupLink struct {
	Path   string `json:"path"`
	Edge   string `json:"edge"`
	Detail string `json:"detail,omitempty"`
}

// SetupInclusion is why one setup file was proved reachable: the chain from a named spec to it
// (TSA-V0-647). A mode `all` result lists only the files it proved; the rest are its fail-closed
// widening.
type SetupInclusion struct {
	Path  string      `json:"path"`
	Chain []SetupLink `json:"chain"`
}

// SetupNotFollowed is a reached file whose imports were deliberately not followed (TSA-V0-004).
type SetupNotFollowed struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// SetupSelection is the setup files a spec list depends on at one revision (TSA-V0-001..006).
// Mode is "selected" when every dependency closed, or "all" with at least one reason, in which
// case SetupFiles is the whole setup universe.
type SetupSelection struct {
	Schema      string              `json:"schema"`
	Revision    string              `json:"revision"`
	Mode        string              `json:"mode"`
	Specs       []string            `json:"specs"`
	SetupFiles  []string            `json:"setupFiles"`
	Universe    int                 `json:"universe"`
	Reasons     []SetupSelectReason `json:"reasons"`
	NotFollowed []SetupNotFollowed  `json:"notFollowed"`
	Inclusions  []SetupInclusion    `json:"inclusions"`
	InputDigest string              `json:"inputDigest"`
}

// IsSetupFile reports a JavaScript or TypeScript setup or seed file: a base name whose stem ends
// in `.setup` or `.seed` (TSA-V0-002).
func IsSetupFile(name string) bool {
	extension := path.Ext(name)
	if language := NonGoImpactLanguage(name); language != "javascript" && language != "typescript" {
		return false
	}
	stem := strings.TrimSuffix(path.Base(name), extension)
	return strings.HasSuffix(stem, ".setup") || strings.HasSuffix(stem, ".seed")
}

type setupSelector struct {
	index       *Index
	resolver    *WebImportResolver
	dialect     *registryDialect
	universe    map[string]bool
	registries  map[string]bool
	read        map[string]string
	reasons     []SetupSelectReason
	notFollowed map[string]string
	visited     map[string]bool
	queue       []string
	// keyTargets are the modules registry keys resolved to: the data contexts the specs read.
	keyTargets map[string]bool
	seeds      map[string]bool
	// shared is the shared-constant key bookkeeping (TSA-V0-006, TSA-V0-007).
	shared *sharedState
	// parent is the first edge that reached each non-spec file (TSA-V0-647).
	parent map[string]setupEdge
}

// setupEdge is how a file was first reached: from a closure file, by an edge kind.
type setupEdge struct {
	from, edge, detail string
}

// SelectSetup returns the setup files specs transitively depend on in index: through resolved
// value imports, literal require() and import() calls, and literal keys of declared
// string-key-registry/0 accessors (TSA-V0-001..006). It never invents a narrower answer: any
// dependency it cannot close returns the whole setup universe with typed reasons.
func SelectSetup(index *Index, specs []string) SetupSelection {
	s := &setupSelector{index: index, resolver: NewWebImportResolver(index), dialect: loadRegistryDialect(index),
		universe: map[string]bool{}, registries: map[string]bool{}, read: map[string]string{},
		notFollowed: map[string]string{}, visited: map[string]bool{}, keyTargets: map[string]bool{}, seeds: map[string]bool{},
		shared: newSharedState(), parent: map[string]setupEdge{}}
	// The universe is every tracked setup path, including generated or oversized files the index
	// does not read, so ALL is always the whole safe superset (TSA-V0-003).
	for name := range index.Tracked {
		if _, skipped := index.Skipped[name]; !skipped && IsSetupFile(name) {
			s.universe[name] = true
		}
	}
	for _, registry := range s.dialect.registries {
		s.registries[registry.declaration.Path] = true
		if registry.blob != "" {
			s.read[registry.declaration.Path] = registry.blob
		}
	}
	// Whether a registry is written after its declaration decides whether
	// it closes (NGI-V0-066), so every source the write scan read is an input.
	for name, blob := range s.dialect.writeInputs {
		s.read[name] = blob
	}
	for name := range index.Sources {
		if path.Ext(name) == ".json" {
			if text, valid, loaded := index.Sources[name].Text(); valid && loaded && strings.Contains(text, "application-map-manifest") {
				s.read[name] = index.Sources[name].BlobHash
			}
		}
	}
	s.readSetupShared()
	if s.dialect.invalid != 0 {
		s.reason("registry-unresolved", "", 0, "an application-map manifest declares invalid registries")
	}
	cleaned := []string{}
	for _, spec := range specs {
		if spec = strings.TrimSpace(spec); spec != "" {
			cleaned = append(cleaned, spec)
		}
	}
	sort.Strings(cleaned)
	if len(cleaned) == 0 {
		s.reason("empty-spec-list", "", 0, "no spec paths were given")
	}
	for _, spec := range cleaned {
		name := path.Clean(spec)
		_, indexed := index.Sources[name]
		if language := NonGoImpactLanguage(name); !indexed || name != spec || (language != "javascript" && language != "typescript") {
			s.reason("missing-path", spec, 0, "not a tracked JavaScript or TypeScript source at the revision")
			continue
		}
		s.enqueue(name, true, setupEdge{})
	}
	// A seeder a registry key or a shared constant key selects is itself traversed, and the seeders
	// of any key it reaches are selected in turn, until the closure stops growing (TSA-V0-001,
	// TSA-V0-006). Non-literal shared reads are judged only against the final closure (TSA-V0-007).
	for s.drain() {
		registry := s.seedersOfKeyTargets()
		if shared := s.seedersOfSharedKeys(); !registry && !shared {
			s.sharedReasons()
			break
		}
	}
	return s.result(index, cleaned)
}

// drain visits the queued closure, reporting false when the bound stopped it.
func (s *setupSelector) drain() bool {
	for len(s.queue) > 0 {
		name := s.queue[0]
		s.queue = s.queue[1:]
		if !setupClosureBound.admits(len(s.visited)) {
			s.reason(setupClosureBound.outcome, "", 0, setupClosureBound.exhausted("the dependency closure"))
			return false
		}
		s.visit(name)
	}
	return true
}

func (s *setupSelector) reason(code, name string, line int, detail string) {
	s.reasons = append(s.reasons, SetupSelectReason{Code: code, Path: name, Line: line, Detail: detail})
}

// enqueue follows a reached file unless it is a declared registry or a hub (TSA-V0-004), and
// records the first edge that reached it (TSA-V0-647).
func (s *setupSelector) enqueue(name string, spec bool, edge setupEdge) {
	if s.visited[name] || s.notFollowed[name] != "" {
		return
	}
	if _, seen := s.parent[name]; !spec && !seen {
		s.parent[name] = edge
	}
	if !spec && s.registries[name] {
		s.notFollowed[name] = "registry"
		return
	}
	if !spec && s.hub(name) {
		s.notFollowed[name] = "hub"
		return
	}
	s.visited[name] = true
	s.queue = append(s.queue, name)
}

// hub reports a file whose resolved imports name every setup file of a universe of two or more:
// following it would select everything, which is the over-selection the command exists to avoid.
func (s *setupSelector) hub(name string) bool {
	if len(s.universe) < 2 || s.universe[name] {
		return false
	}
	text, ok := s.text(name)
	if !ok {
		return false
	}
	kinds, _ := WebImportKinds(text)
	named := map[string]bool{}
	for specifier := range kinds {
		if resolution := s.resolver.Resolve(name, specifier); resolution.State == WebImportRepository && s.universe[resolution.Target] {
			named[resolution.Target] = true
		}
	}
	return len(named) == len(s.universe)
}

func (s *setupSelector) text(name string) (string, bool) {
	source, indexed := s.index.Sources[name]
	if !indexed {
		return "", false
	}
	text, valid, loaded := source.Text()
	if !valid || !loaded {
		return "", false
	}
	s.read[name] = source.BlobHash
	return text, true
}

func (s *setupSelector) follow(from, specifier string, line int, via string, edge setupEdge) {
	resolution := s.resolver.Resolve(from, specifier)
	switch resolution.State {
	case WebImportRepository:
		if via != "" {
			s.keyTargets[resolution.Target] = true
		}
		s.enqueue(resolution.Target, false, edge)
	case WebImportUnresolved:
		code := "import-unresolved"
		if via != "" {
			code = "registry-unresolved"
		}
		s.reason(code, from, line, strings.TrimSpace(via+" "+specifier))
	}
}

// visit follows one closure file's imports, require() and import() calls and registry keys.
func (s *setupSelector) visit(name string) {
	text, ok := s.text(name)
	if !ok {
		s.reason("source-unreadable", name, 0, "the source cannot be read")
		return
	}
	kinds, unterminated := WebImportKinds(text)
	tokens, complete := registryTokens(text)
	if unterminated != "" || !complete {
		s.reason("source-unreadable", name, 0, "the source ends inside an unterminated construct")
	}
	specifiers := make([]string, 0, len(kinds))
	for specifier, typeOnly := range kinds {
		if !typeOnly && !s.importsOnlyTypes(name, specifier, tokens) {
			specifiers = append(specifiers, specifier)
		}
	}
	sort.Strings(specifiers)
	for _, specifier := range specifiers {
		s.follow(name, specifier, 0, "", setupEdge{from: name, edge: "import", detail: specifier})
	}
	for at, token := range tokens {
		if token.kind != webWord || (token.text != "require" && token.text != "import") || registryMember(tokens, at) ||
			at+1 >= len(tokens) || !isRegistryPunct(tokens[at+1], "(") {
			continue
		}
		close := registryClose(tokens, at+1)
		literal := at+3 == close && tokens[at+2].kind == webString && tokens[at+2].quoted
		switch {
		case !literal:
			s.reason("computed-lookup", name, token.line, "a "+token.text+"() whose module is not a string literal")
		case token.text == "require":
			s.follow(name, tokens[at+2].text, token.line, "", setupEdge{from: name, edge: "require", detail: tokens[at+2].text})
		}
	}
	for _, site := range s.dialect.sites {
		if site.path == name {
			s.lookup(site)
		}
	}
	s.collectShared(name)
}

// lookup follows a registry accessor call by its literal key (TSA-V0-003).
func (s *setupSelector) lookup(site registrySite) {
	if !site.literal {
		s.reason("computed-lookup", site.path, site.line, "a "+site.accessor+" call whose key is not a literal")
		return
	}
	found, unclosed := false, false
	for _, registry := range s.dialect.registries {
		declares := false
		for _, accessor := range registry.declaration.Accessors {
			declares = declares || accessor == site.accessor
		}
		if !declares {
			continue
		}
		// A registry written after its declaration, or an entry whose
		// constructor arguments may reach another module, does not close
		// either (NGI-V0-066, NGI-V0-067).
		if registry.missing || registry.unresolved != 0 || registry.written != 0 {
			unclosed = true
		}
		for _, entry := range registry.entries {
			if entry.key == site.key {
				found = true
				unclosed = unclosed || entry.arguments
				s.follow(registry.declaration.Path, entry.binding.specifier, entry.line, "key "+site.key+":",
					setupEdge{from: site.path, edge: "registry-key", detail: site.accessor + "(" + site.key + ")"})
			}
		}
	}
	switch {
	case unclosed:
		s.reason("registry-unresolved", site.path, site.line, "a registry "+site.accessor+" reads has entries that do not close")
	case !found:
		s.reason("unknown-key", site.path, site.line, site.accessor+"("+site.key+") names no registry key")
	}
}

// seedersOfKeyTargets selects every setup file that directly imports a module a registry key
// resolved to: the setup that seeds a data context the specs read (TSA-V0-001). Each new seeder
// is enqueued so its own dependencies are traversed; it reports whether any was.
func (s *setupSelector) seedersOfKeyTargets() bool {
	if len(s.keyTargets) == 0 {
		return false
	}
	enqueued := false
	names := make([]string, 0, len(s.universe))
	for name := range s.universe {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if s.visited[name] || s.seeds[name] {
			continue
		}
		text, ok := s.text(name)
		if !ok {
			s.reason("source-unreadable", name, 0, "the source cannot be read")
			continue
		}
		kinds, _ := WebImportKinds(text)
		specifiers := make([]string, 0, len(kinds))
		for specifier := range kinds {
			specifiers = append(specifiers, specifier)
		}
		sort.Strings(specifiers)
		edge := setupEdge{}
		for _, specifier := range specifiers {
			if resolution := s.resolver.Resolve(name, specifier); !kinds[specifier] && resolution.State == WebImportRepository && s.keyTargets[resolution.Target] {
				if !s.seeds[name] {
					edge = setupEdge{from: resolution.Target, edge: "registry-seeder", detail: specifier}
				}
				s.seeds[name] = true
			}
		}
		if s.seeds[name] {
			s.enqueue(name, false, edge)
			enqueued = true
		}
	}
	return enqueued
}

func (s *setupSelector) result(index *Index, specs []string) SetupSelection {
	out := SetupSelection{Schema: SetupSelectSchema, Revision: index.CommitRevision, Mode: "selected", Specs: specs,
		SetupFiles: []string{}, Universe: len(s.universe), Reasons: []SetupSelectReason{}, NotFollowed: []SetupNotFollowed{}}
	if out.Specs == nil {
		out.Specs = []string{}
	}
	for name := range s.universe {
		if s.visited[name] || s.seeds[name] || len(s.reasons) != 0 {
			out.SetupFiles = append(out.SetupFiles, name)
		}
	}
	sort.Strings(out.SetupFiles)
	if len(s.reasons) != 0 {
		out.Mode = "all"
		sort.Slice(s.reasons, func(a, b int) bool {
			x, y := s.reasons[a], s.reasons[b]
			if x.Code != y.Code {
				return x.Code < y.Code
			}
			if x.Path != y.Path {
				return x.Path < y.Path
			}
			if x.Line != y.Line {
				return x.Line < y.Line
			}
			return x.Detail < y.Detail
		})
		for i, reason := range s.reasons {
			if i > 0 && reason == s.reasons[i-1] {
				continue
			}
			if len(out.Reasons) == maxSetupReasons {
				break
			}
			out.Reasons = append(out.Reasons, reason)
		}
	}
	for name, why := range s.notFollowed {
		out.NotFollowed = append(out.NotFollowed, SetupNotFollowed{Path: name, Reason: why})
	}
	sort.Slice(out.NotFollowed, func(a, b int) bool { return out.NotFollowed[a].Path < out.NotFollowed[b].Path })
	out.Inclusions = s.inclusions()
	names := make([]string, 0, len(s.read))
	for name := range s.read {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		digest.Write([]byte(name + "\t" + s.read[name] + "\n"))
	}
	out.InputDigest = "sha256:" + hex.EncodeToString(digest.Sum(nil))
	return out
}

// inclusions returns, for every setup file the closure proved (visited or seeded), the chain of
// first edges from a named spec to it (TSA-V0-647). Each recorded edge starts at a file reached
// before it, so every chain ends at a spec; the walk is still bounded by the reached-file count.
func (s *setupSelector) inclusions() []SetupInclusion {
	out := []SetupInclusion{}
	names := make([]string, 0, len(s.universe))
	for name := range s.universe {
		if s.visited[name] || s.seeds[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		chain, at := []SetupLink{}, name
		for steps := 0; steps <= len(s.parent); steps++ {
			edge, reached := s.parent[at]
			if !reached {
				chain = append(chain, SetupLink{Path: at, Edge: "spec"})
				break
			}
			chain = append(chain, SetupLink{Path: at, Edge: edge.edge, Detail: edge.detail})
			at = edge.from
		}
		for a, b := 0, len(chain)-1; a < b; a, b = a+1, b-1 {
			chain[a], chain[b] = chain[b], chain[a]
		}
		out = append(out, SetupInclusion{Path: name, Chain: chain})
	}
	return out
}

// importsOnlyTypes reports whether every name a closure file imports from specifier is declared
// there as an exported type or interface, and the file uses the module in no other way (a
// side-effect import, import(), require() or `export ... from`), so that TypeScript erases the
// import and it is no runtime dependency (TSA-V0-839, TSA-V0-845).
func (s *setupSelector) importsOnlyTypes(name, specifier string, tokens []registryToken) bool {
	table, ok := s.exportsOf(name)
	if !ok {
		return false
	}
	// Any mention of the module other than an import clause's `from` is a runtime use: a
	// side-effect import, import() or require(), or an `export ... from` re-export.
	for at, token := range tokens {
		if token.kind == webString && token.quoted && token.text == specifier && (at == 0 || !isRegistryWord(tokens[at-1], "from")) {
			return false
		}
	}
	for _, binding := range table.from {
		if binding.specifier == specifier {
			return false
		}
	}
	for _, star := range table.stars {
		if star == specifier {
			return false
		}
	}
	resolution := s.resolver.Resolve(name, specifier)
	if resolution.State != WebImportRepository {
		return false
	}
	target, ok := s.exportsOf(resolution.Target)
	if !ok {
		return false
	}
	names := 0
	for _, binding := range table.bindings {
		if binding.specifier != specifier {
			continue
		}
		names++
		if !exportsOnlyType(target, binding.imported) {
			return false
		}
	}
	return names != 0
}

// exportsOnlyType reports whether table exports name only as a type, so that an import of it is
// erased (TSA-V0-839, TSA-V0-848): `export [declare] type|interface NAME`, a local type or
// interface exported through `export { NAME }`, or NAME exported by an `export type { ... }` list
// or an inline `export { type X as NAME }`, with or without `from`. A value declaration of NAME
// (`const|let|var|function|class|enum|namespace|module NAME`, a later or destructured declarator,
// or `import NAME =`, TSA-V0-851), a plain `export { ... } from`
// forward of it, or an export of an alias or of an imported binding under that name is not.
func exportsOnlyType(table *moduleExports, name string) bool {
	if _, forwarded := table.from[name]; forwarded {
		return false
	}
	local, listed := table.local[name]
	if listed && local != name {
		return false
	}
	_, imported := table.bindings[name]
	tokens := table.tokens
	declared, exported := false, false
	for at := 0; at+1 < len(tokens); at++ {
		if registryMember(tokens, at) {
			continue
		}
		word, next := tokens[at], tokens[at+1]
		if (isRegistryWord(word, "const") || isRegistryWord(word, "let") || isRegistryWord(word, "var")) && bindsValue(tokens, at, name) {
			return false
		}
		if next.kind == webWord && next.text == name {
			if isRegistryWord(word, "import") && at+2 < len(tokens) && isRegistryPunct(tokens[at+2], "=") {
				return false
			}
			for _, keyword := range []string{"const", "let", "var", "function", "class", "enum", "namespace", "module"} {
				if isRegistryWord(word, keyword) {
					return false
				}
			}
			if typeDeclaration(tokens, at) {
				declared = true
				exported = exported || (at > 0 && isRegistryWord(tokens[at-1], "export")) ||
					(at > 1 && isRegistryWord(tokens[at-1], "declare") && isRegistryWord(tokens[at-2], "export"))
			}
		}
		if !isRegistryWord(word, "export") {
			continue
		}
		open, wholeList := at+1, false
		if isRegistryWord(next, "type") && at+2 < len(tokens) && isRegistryPunct(tokens[at+2], "{") {
			open, wholeList = at+2, true
		}
		if !isRegistryPunct(tokens[open], "{") {
			continue
		}
		for _, part := range sharedListParts(tokens[open+1 : registryClose(tokens, open)]) {
			typed := wholeList
			if len(part) > 1 && isRegistryWord(part[0], "type") && !isRegistryWord(part[1], "as") {
				typed, part = true, part[1:]
			}
			if typed && len(part) > 0 && (part[len(part)-1].text == name) && (len(part) == 1 || len(part) == 3 && isRegistryWord(part[1], "as")) {
				exported = true
			}
		}
	}
	return exported || (listed && declared && !imported)
}

// bindsValue reports whether the `const|let|var` statement whose keyword is tokens[at] binds
// name in any declarator (TSA-V0-851): a later declarator (`const a = 0, NAME = 1`) or a
// destructuring pattern (`const { a: NAME } = o`). A mention in a declarator's type annotation or
// initializer is not a binding; a mention the scan cannot place reads as one, which only keeps
// the import followed.
func bindsValue(tokens []registryToken, at int, name string) bool {
	const binding, annotation, initializer = 0, 1, 2
	state, depth := binding, 0
	for scan := at + 1; scan < len(tokens); scan++ {
		token := tokens[scan]
		if depth == 0 && scan > at+1 && token.line > tokens[scan-1].line && !sharedContinues(tokens, scan) {
			return false
		}
		if token.kind == webWord && token.text == name && state == binding && !registryMember(tokens, scan) {
			return true
		}
		if token.kind != webPunct {
			continue
		}
		switch text := token.text; {
		case strings.Contains("([{", text) || state == annotation && text == "<":
			depth++
		case strings.Contains(")]}", text) || state == annotation && text == ">" && !isRegistryPunct(tokens[scan-1], "="):
			if depth--; depth < 0 {
				return false
			}
		case depth != 0:
		case text == ";":
			return false
		case text == ",":
			state = binding
		case text == ":" && state == binding:
			state = annotation
		case text == "=" && state != initializer && !(scan+1 < len(tokens) && isRegistryPunct(tokens[scan+1], ">")):
			state = initializer
		}
	}
	return false
}

// typeDeclaration reports a `type NAME =|<` or `interface NAME {|<|extends` declaration whose
// keyword is tokens[at].
func typeDeclaration(tokens []registryToken, at int) bool {
	if at+2 >= len(tokens) {
		return false
	}
	after := tokens[at+2]
	switch {
	case isRegistryWord(tokens[at], "type"):
		return isRegistryPunct(after, "=") || isRegistryPunct(after, "<")
	case isRegistryWord(tokens[at], "interface"):
		return isRegistryPunct(after, "{") || isRegistryPunct(after, "<") || isRegistryWord(after, "extends")
	}
	return false
}
