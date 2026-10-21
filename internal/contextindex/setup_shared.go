package contextindex

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Shared constant keys (TSA-V0-006, TSA-V0-007): a setup file and a reached data context are often
// linked by nothing but a literal property of one exported constant object, for example a setup that
// seeds SHARED.scenario_a and a context that reads SHARED.scenario_a. This file reads those property
// accesses so setup-select can select the seeder, or return ALL when a read cannot be closed.

// Its bounds, sharedReexportBound, sharedLookupBound and sharedTemplateBound, are declared with
// the other analyzer bounds (analyzer_bounds.go, TSA-V0-460).

const (
	// sharedDefaultObject is the local name recorded for `export default { ... }`.
	sharedDefaultObject = "\x00default-object"
)

// sharedConst identifies one exported constant object by the module that declares it and its
// local name there, so every import or re-export path to the same object compares equal. A
// module namespace is {module, "*"}.
type sharedConst struct{ module, local string }

// sharedUse is what one file reads of one shared constant: its literal property keys, and the
// line of the first read whose key is not a literal (zero when every read is literal). The
// closure's aggregate keeps only keys; its non-literal reads are kept as sharedSite rows.
type sharedUse struct {
	keys map[string]bool
	open int
}

// sharedSite is one read of a shared constant the selector cannot reduce to literal keys.
type sharedSite struct {
	path  string
	line  int
	name  string
	value sharedConst
	// any marks a read the selector cannot attribute to one constant: a namespace or module used
	// as a value, a resolution a bound stopped, or a template nested past the bound. It is matched
	// against every setup file that reads any shared constant (TSA-V0-007).
	any    bool
	detail string
}

// moduleExports is the export table of one module.
type moduleExports struct {
	tokens   []registryToken
	bindings map[string]registryBinding
	// local maps an exported name to the module-local name it exports.
	local map[string]string
	// from maps an exported name to the module and name an `export ... from` forwards.
	from  map[string]registryBinding
	stars []string
	// truncated is the line of the first template substitution nested past sharedTemplateBound.
	truncated int
}

type sharedKind int

const (
	// sharedNone: the name is not an exported object constant.
	sharedNone sharedKind = iota
	// sharedFound: value names the constant object.
	sharedFound
	// sharedNamespace: value.module is a module namespace (`export * as NS from`).
	sharedNamespace
	// sharedUnknown: a bound, an unreadable module or an unresolved specifier stopped the resolution.
	sharedUnknown
)

type sharedResolution struct {
	value sharedConst
	kind  sharedKind
	why   string
}

// sharedState is the selector's shared-constant bookkeeping.
type sharedState struct {
	exports map[string]*moduleExports
	// resolved caches only path-independent resolutions: never one a cycle marker or the hop or
	// lookup bound shaped (TSA-V0-462).
	resolved  map[[2]string]sharedResolution
	active    map[[2]string]bool
	steps     int
	setupUses map[string]map[sharedConst]*sharedUse
	// setupOpen keeps, per setup file, the reads it cannot attribute to one constant.
	setupOpen  map[string][]sharedSite
	unreadable []string
	reached    map[sharedConst]*sharedUse
	sites      []sharedSite
}

func newSharedState() *sharedState {
	return &sharedState{exports: map[string]*moduleExports{}, resolved: map[[2]string]sharedResolution{},
		active: map[[2]string]bool{}, setupUses: map[string]map[sharedConst]*sharedUse{},
		setupOpen: map[string][]sharedSite{}, reached: map[sharedConst]*sharedUse{}}
}

// sharedTokens lexes text like registryTokens and also lexes the `${...}` substitutions of every
// template literal, which the web lexer folds into one opaque string token, so a property read
// inside a template is still seen. Each substitution is wrapped in parentheses. truncated is the
// line of the first substitution nested past sharedTemplateBound, which is not lexed, and zero
// when there is none.
func sharedTokens(text string, depth int) (tokens []registryToken, complete bool, truncated int) {
	lexer := &webLexer{text: text}
	tokens = make([]registryToken, 0)
	line, counted := 1, 0
	for {
		before := lexer.position
		token, ok := lexer.next()
		if !ok {
			break
		}
		end := lexer.position
		if end-1 > counted {
			line += strings.Count(text[counted:end-1], "\n")
			counted = end - 1
		}
		tokens = append(tokens, registryToken{token, line})
		if token.kind != webString || token.quoted || token.template || end == 0 || text[end-1] != '`' {
			continue
		}
		start := sharedTokenStart(text, before)
		first := line - strings.Count(text[start:end], "\n")
		bodies := templateSubstitutions(text[start:end])
		if !sharedTemplateBound.admits(depth + 1) {
			if len(bodies) != 0 && truncated == 0 {
				truncated = first
			}
			continue
		}
		for _, body := range bodies {
			inner, _, innerTruncated := sharedTokens(body, depth+1)
			if innerTruncated != 0 && truncated == 0 {
				truncated = innerTruncated + first - 1
			}
			tokens = append(tokens, registryToken{webToken{kind: webPunct, text: "("}, first})
			for _, token := range inner {
				token.line += first - 1
				tokens = append(tokens, token)
			}
			tokens = append(tokens, registryToken{webToken{kind: webPunct, text: ")"}, first})
		}
	}
	return tokens, lexer.unterminated == "", truncated
}

// sharedTokenStart skips the white space and comments the lexer skipped before a token.
func sharedTokenStart(text string, at int) int {
	for at < len(text) {
		character, width := utf8.DecodeRuneInString(text[at:])
		switch {
		case unicode.IsSpace(character):
			at += width
		case strings.HasPrefix(text[at:], "//"):
			end := strings.IndexAny(text[at:], lineCommentTerminators)
			if end < 0 {
				return len(text)
			}
			at += end
		case strings.HasPrefix(text[at:], "/*"):
			end := strings.Index(text[at+2:], "*/")
			if end < 0 {
				return len(text)
			}
			at += 2 + end + 2
		default:
			return at
		}
	}
	return at
}

// templateSubstitutions returns the source of each top-level `${...}` in a template literal.
func templateSubstitutions(raw string) []string {
	bodies := []string{}
	for at := 1; at < len(raw); {
		switch {
		case raw[at] == '\\':
			at += 2
		case strings.HasPrefix(raw[at:], "${"):
			start := at + 2
			lexer := &webLexer{text: raw[start:], previous: webToken{kind: webPunct, text: "{"}}
			depth, end := 0, len(raw)
			for {
				token, ok := lexer.next()
				if !ok {
					break
				}
				if token.kind == webPunct && token.text == "{" {
					depth++
				} else if token.kind == webPunct && token.text == "}" {
					if depth == 0 {
						end = start + lexer.position - 1
						break
					}
					depth--
				}
			}
			bodies = append(bodies, raw[start:end])
			at = end + 1
		default:
			at++
		}
	}
	return bodies
}

// exportsOf parses the export table of a repository module, or reports false when it is unreadable.
func (s *setupSelector) exportsOf(module string) (*moduleExports, bool) {
	if table, ok := s.shared.exports[module]; ok {
		return table, table != nil
	}
	text, ok := s.text(module)
	if !ok {
		s.shared.exports[module] = nil
		return nil, false
	}
	tokens, _, truncated := sharedTokens(text, 0)
	table := &moduleExports{tokens: tokens, bindings: registryImports(tokens), local: map[string]string{},
		from: map[string]registryBinding{}, truncated: truncated}
	for at := 0; at+1 < len(tokens); at++ {
		if !isRegistryWord(tokens[at], "export") || registryMember(tokens, at) {
			continue
		}
		next := tokens[at+1]
		switch {
		case isRegistryWord(next, "const") || isRegistryWord(next, "let") || isRegistryWord(next, "var"):
			if at+2 < len(tokens) && tokens[at+2].kind == webWord {
				table.local[tokens[at+2].text] = tokens[at+2].text
			}
		case isRegistryWord(next, "default"):
			if at+2 < len(tokens) && isRegistryPunct(tokens[at+2], "{") {
				table.local["default"] = sharedDefaultObject
			} else if at+2 < len(tokens) && tokens[at+2].kind == webWord {
				table.local["default"] = tokens[at+2].text
			}
		case isRegistryPunct(next, "*"):
			end := at + 2
			alias := ""
			if end+1 < len(tokens) && isRegistryWord(tokens[end], "as") {
				alias, end = tokens[end+1].text, end+2
			}
			if end+1 < len(tokens) && isRegistryWord(tokens[end], "from") && tokens[end+1].quoted {
				if alias == "" {
					table.stars = append(table.stars, tokens[end+1].text)
				} else {
					table.from[alias] = registryBinding{tokens[end+1].text, "*"}
				}
			}
		case isRegistryPunct(next, "{"):
			close := registryClose(tokens, at+1)
			specifier := ""
			if close+2 < len(tokens) && isRegistryWord(tokens[close+1], "from") && tokens[close+2].quoted {
				specifier = tokens[close+2].text
			}
			for _, part := range sharedListParts(tokens[at+2 : close]) {
				if len(part) == 0 || (len(part) > 1 && isRegistryWord(part[0], "type") && !isRegistryWord(part[1], "as")) {
					continue
				}
				name, exported := part[0].text, part[0].text
				if len(part) == 3 && isRegistryWord(part[1], "as") {
					exported = part[2].text
				}
				if specifier != "" {
					table.from[exported] = registryBinding{specifier, name}
				} else {
					table.local[exported] = name
				}
			}
		}
	}
	s.shared.exports[module] = table
	return table, true
}

// sharedListParts splits the tokens of an `{ a, b as c }` list at its top-level commas.
func sharedListParts(tokens []registryToken) [][]registryToken {
	parts, start, depth := [][]registryToken{}, 0, 0
	for at, token := range tokens {
		switch {
		case token.kind != webPunct:
		case strings.Contains("([{", token.text):
			depth++
		case strings.Contains(")]}", token.text):
			depth--
		case token.text == "," && depth == 0:
			parts = append(parts, tokens[start:at])
			start = at + 1
		}
	}
	return append(parts, tokens[start:])
}

// constObject reports a `const|let|var NAME [: Type] = {` declaration of local in tokens.
func constObject(tokens []registryToken, local string) bool {
	for at := 0; at+1 < len(tokens); at++ {
		if word := tokens[at]; registryMember(tokens, at) || !(isRegistryWord(word, "const") || isRegistryWord(word, "let") || isRegistryWord(word, "var")) ||
			!isRegistryWord(tokens[at+1], local) {
			continue
		}
		next := registryAnnotationEnd(tokens, at+2)
		return next+1 < len(tokens) && isRegistryPunct(tokens[next], "=") && isRegistryPunct(tokens[next+1], "{")
	}
	return false
}

// resolveConst follows name exported by module, through `export ... from`, `export *`,
// `export * as` and local re-exports of imports, to the constant object that declares it or to a
// module namespace. A chain beyond sharedReexportBound, more than sharedLookupBound lookups, an
// unreadable module or an unresolved specifier is sharedUnknown, never sharedNone (TSA-V0-007).
func (s *setupSelector) resolveConst(module, name string) sharedResolution {
	s.shared.steps = 0
	result, _ := s.resolveExport(module, name, 0)
	return result
}

// resolveExport is one step of resolveConst. exact is false when the result depends on the path
// that reached it (a module already being resolved, or the hop or step bound), so it is not cached.
func (s *setupSelector) resolveExport(module, name string, depth int) (sharedResolution, bool) {
	key := [2]string{module, name}
	if result, ok := s.shared.resolved[key]; ok {
		return result, true
	}
	if s.shared.active[key] {
		// An `export *` cycle: no binding along this path, which says nothing about other paths.
		return sharedResolution{}, false
	}
	if !sharedReexportBound.admits(depth) {
		return sharedResolution{kind: sharedUnknown, why: sharedReexportBound.exhausted("a re-export chain")}, false
	}
	if s.shared.steps++; !sharedLookupBound.admits(s.shared.steps) {
		return sharedResolution{kind: sharedUnknown, why: sharedLookupBound.exhausted("a re-export graph")}, false
	}
	table, ok := s.exportsOf(module)
	if !ok {
		result := sharedResolution{kind: sharedUnknown, why: "a re-export through " + module + ", which cannot be read"}
		s.shared.resolved[key] = result
		return result, true
	}
	s.shared.active[key] = true
	defer delete(s.shared.active, key)
	// exhausted is the first branch a bound stopped. It outranks any later branch's answer: the
	// stopped branch may bind name too, which a found result elsewhere cannot rule out
	// (TSA-V0-461).
	result, exact, exhausted := sharedResolution{}, true, sharedResolution{}
	forward := func(specifier, imported string) {
		if result.kind == sharedFound || result.kind == sharedNamespace || exhausted.kind == sharedUnknown {
			return
		}
		resolution := s.resolver.Resolve(module, specifier)
		var next sharedResolution
		switch {
		case resolution.State == WebImportUnresolved:
			next = sharedResolution{kind: sharedUnknown, why: "a re-export of " + specifier + ", which does not resolve"}
		case resolution.State != WebImportRepository:
			return
		case imported == "*":
			next = sharedResolution{value: sharedConst{resolution.Target, "*"}, kind: sharedNamespace}
		default:
			var nextExact bool
			next, nextExact = s.resolveExport(resolution.Target, imported, depth+1)
			exact = exact && nextExact
			if next.kind == sharedUnknown && !nextExact {
				exhausted = next
			}
		}
		if next.kind != sharedNone {
			result = next
		}
	}
	if local, exported := table.local[name]; exported {
		switch binding, imported := table.bindings[local]; {
		case local == sharedDefaultObject:
			result = sharedResolution{value: sharedConst{module, local}, kind: sharedFound}
		case imported:
			forward(binding.specifier, binding.imported)
		case constObject(table.tokens, local):
			result = sharedResolution{value: sharedConst{module, local}, kind: sharedFound}
		}
	} else if binding, ok := table.from[name]; ok {
		forward(binding.specifier, binding.imported)
	} else if name != "default" {
		for _, specifier := range table.stars {
			forward(specifier, name)
		}
	}
	if exhausted.kind == sharedUnknown {
		result = exhausted
	}
	if exact {
		s.shared.resolved[key] = result
	}
	return result, exact
}

// sharedUses reads every property access of a shared constant in one file: of an imported
// constant, of a namespace member, of `require("m").NAME` or `import("m").NAME`, and of an object
// constant the file itself exports (TSA-V0-006). Reads it cannot reduce to literal keys are
// returned as sites; a site marked any cannot even be attributed to one constant.
func (s *setupSelector) sharedUses(name string, table *moduleExports) (map[sharedConst]*sharedUse, []sharedSite) {
	tokens := table.tokens
	uses, sites := map[sharedConst]*sharedUse{}, []sharedSite{}
	locals := map[string]sharedResolution{}
	for _, local := range table.local {
		if _, imported := table.bindings[local]; !imported && local != sharedDefaultObject && constObject(tokens, local) {
			locals[local] = sharedResolution{value: sharedConst{name, local}, kind: sharedFound}
		}
	}
	// binding resolves one imported local name once per file.
	binding := func(local string) (sharedResolution, bool) {
		if result, ok := locals[local]; ok {
			return result, true
		}
		imported, ok := table.bindings[local]
		if !ok {
			return sharedResolution{}, false
		}
		result := sharedResolution{}
		switch resolution := s.resolver.Resolve(name, imported.specifier); {
		case resolution.State == WebImportUnresolved:
			result = sharedResolution{kind: sharedUnknown, why: "an import of " + imported.specifier + ", which does not resolve"}
		case resolution.State != WebImportRepository:
		case imported.imported == "*":
			result = sharedResolution{value: sharedConst{resolution.Target, "*"}, kind: sharedNamespace}
		default:
			result = s.resolveConst(resolution.Target, imported.imported)
		}
		locals[local] = result
		return result, true
	}
	record := func(value sharedConst, at int, label string) {
		use := uses[value]
		if use == nil {
			use = &sharedUse{keys: map[string]bool{}}
			uses[value] = use
		}
		keys, closed := sharedAccessKeys(tokens, at)
		for _, key := range keys {
			use.keys[key] = true
		}
		if !closed {
			if use.open == 0 {
				use.open = tokens[at].line
			}
			sites = append(sites, sharedSite{path: name, line: tokens[at].line, name: label, value: value,
				detail: "a non-literal read of " + label})
		}
	}
	// reference follows `.NAME` members while the value is a namespace, then records the read.
	reference := func(result sharedResolution, at, end int, label string) {
		for result.kind == sharedNamespace && end+2 < len(tokens) && isRegistryPunct(tokens[end+1], ".") && tokens[end+2].kind == webWord {
			end += 2
			label += "." + tokens[end].text
			result = s.resolveConst(result.value.module, tokens[end].text)
		}
		switch result.kind {
		case sharedFound:
			record(result.value, end, label)
		case sharedNamespace:
			sites = append(sites, sharedSite{path: name, line: tokens[at].line, name: label, any: true,
				detail: "namespace " + label + " used other than as NAMESPACE.EXPORT"})
		case sharedUnknown:
			sites = append(sites, sharedSite{path: name, line: tokens[at].line, name: label, any: true,
				detail: label + " is " + result.why})
		}
	}
	if table.truncated != 0 {
		sites = append(sites, sharedSite{path: name, line: table.truncated, any: true,
			detail: sharedTemplateBound.exhausted("template substitutions nested") + ", which are not read"})
	}
	for at, token := range tokens {
		if token.kind != webWord || registryMember(tokens, at) {
			continue
		}
		if (token.text == "require" || token.text == "import") && at+3 < len(tokens) && isRegistryPunct(tokens[at+1], "(") &&
			tokens[at+2].kind == webString && tokens[at+2].quoted && isRegistryPunct(tokens[at+3], ")") {
			if at > 0 && isRegistryWord(tokens[at-1], "typeof") {
				continue
			}
			label := token.text + "(" + tokens[at+2].text + ")"
			switch resolution := s.resolver.Resolve(name, tokens[at+2].text); resolution.State {
			case WebImportRepository:
				reference(sharedResolution{value: sharedConst{resolution.Target, "*"}, kind: sharedNamespace}, at, at+3, label)
			case WebImportUnresolved:
				reference(sharedResolution{kind: sharedUnknown, why: "a module that does not resolve"}, at, at+3, label)
			}
			continue
		}
		if sharedNonRead(tokens, at) || sharedDeclaration(tokens, at) {
			continue
		}
		if result, bound := binding(token.text); bound {
			reference(result, at, at, token.text)
		}
	}
	return uses, sites
}

// sharedDeclaration reports a mention that declares a new binding rather than reading one:
// `const|let|var|function|class NAME`, an arrow parameter `NAME =>`, or a simple parameter of a
// `function`, arrow or `catch` parameter list. A body that reads the shadowing binding is still
// attributed to the imported constant, which can only over-select; a shadowing form not listed
// here (a destructured or method parameter) reads as a whole-object use and fails closed.
func sharedDeclaration(tokens []registryToken, at int) bool {
	if at > 0 {
		for _, keyword := range []string{"const", "let", "var", "function", "class"} {
			if isRegistryWord(tokens[at-1], keyword) {
				return true
			}
		}
	}
	if at+2 < len(tokens) && isRegistryPunct(tokens[at+1], "=") && isRegistryPunct(tokens[at+2], ">") {
		return true
	}
	if at == 0 || at+1 >= len(tokens) || !(isRegistryPunct(tokens[at-1], "(") || isRegistryPunct(tokens[at-1], ",")) {
		return false
	}
	if after := tokens[at+1]; after.kind != webPunct || !(after.text == "," || after.text == ")" || after.text == ":" || after.text == "=" || after.text == "?") {
		return false
	}
	open, depth := -1, 0
	for before := at - 1; before >= 0 && open < 0; before-- {
		switch token := tokens[before]; {
		case token.kind != webPunct:
		case strings.Contains(")]}", token.text):
			depth++
		case strings.Contains("([{", token.text):
			if depth == 0 {
				if token.text != "(" {
					return false
				}
				open = before
			}
			depth--
		}
	}
	if open < 0 {
		return false
	}
	if open > 0 && (isRegistryWord(tokens[open-1], "function") || isRegistryWord(tokens[open-1], "catch") ||
		(open > 1 && tokens[open-1].kind == webWord && isRegistryWord(tokens[open-2], "function"))) {
		return true
	}
	close := registryClose(tokens, open)
	return close+2 < len(tokens) && isRegistryPunct(tokens[close+1], "=") && isRegistryPunct(tokens[close+2], ">")
}

// sharedNonRead reports a mention that reads no property: inside an import or `export { ... }`
// clause, or the operand of `typeof`.
func sharedNonRead(tokens []registryToken, at int) bool {
	if at > 0 && isRegistryWord(tokens[at-1], "typeof") {
		return true
	}
	for before := at - 1; before >= 0 && webClauseToken(tokens[before].webToken); before-- {
		if registryMember(tokens, before) {
			return false
		}
		if isRegistryWord(tokens[before], "import") || (isRegistryWord(tokens[before], "export") && before+1 < len(tokens) && isRegistryPunct(tokens[before+1], "{")) {
			return true
		}
	}
	return false
}

// sharedAccessKeys returns the literal property keys read by the reference ending at tokens[at]:
// `.key`, `?.key`, `!.key`, `["key"]`, or a destructuring `{ key, other: x } = REF`. closed is
// false for any other use (a computed key, passing or aliasing the whole object, a rest element).
func sharedAccessKeys(tokens []registryToken, at int) ([]string, bool) {
	next := at + 1
	if next+1 < len(tokens) && (isRegistryPunct(tokens[next], "?") || isRegistryPunct(tokens[next], "!")) &&
		(isRegistryPunct(tokens[next+1], ".") || isRegistryPunct(tokens[next+1], "[")) {
		next++
	}
	if next+1 < len(tokens) && isRegistryPunct(tokens[next], ".") && isRegistryPunct(tokens[next+1], "[") {
		next++ // optional chaining `?.[`
	}
	switch {
	case next+1 < len(tokens) && isRegistryPunct(tokens[next], ".") && tokens[next+1].kind == webWord:
		return []string{tokens[next+1].text}, true
	case next+2 < len(tokens) && isRegistryPunct(tokens[next], "[") && isRegistryPunct(tokens[next+2], "]"):
		key, literal := registryLiteral(tokens[next+1])
		return []string{key}, literal
	case next < len(tokens) && (isRegistryPunct(tokens[next], ".") || isRegistryPunct(tokens[next], "[")):
		return nil, false
	}
	if at < 2 || !isRegistryPunct(tokens[at-1], "=") || !isRegistryPunct(tokens[at-2], "}") {
		return nil, false
	}
	open, depth := -1, 0
	for before := at - 2; before >= 0; before-- {
		if tokens[before].kind != webPunct {
			continue
		}
		if strings.Contains(")]}", tokens[before].text) {
			depth++
		} else if strings.Contains("([{", tokens[before].text) {
			if depth--; depth == 0 {
				open = before
				break
			}
		}
	}
	if open < 0 {
		return nil, false
	}
	keys := []string{}
	for _, part := range sharedListParts(tokens[open+1 : at-2]) {
		switch {
		case len(part) == 0:
		case part[0].kind == webWord:
			keys = append(keys, part[0].text)
		case part[0].kind == webString:
			key, literal := registryLiteral(part[0])
			if !literal {
				return keys, false
			}
			keys = append(keys, key)
		default:
			return keys, false
		}
	}
	return keys, true
}

// readSetupShared records what every setup file in the universe reads of shared constants, once,
// before the closure is traversed.
func (s *setupSelector) readSetupShared() {
	names := make([]string, 0, len(s.universe))
	for name := range s.universe {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		table, ok := s.exportsOf(name)
		if !ok {
			s.shared.unreadable = append(s.shared.unreadable, name)
			continue
		}
		uses, sites := s.sharedUses(name, table)
		if len(uses) != 0 {
			s.shared.setupUses[name] = uses
		}
		for _, site := range sites {
			if site.any {
				s.shared.setupOpen[name] = append(s.shared.setupOpen[name], site)
			}
		}
	}
}

// collectShared adds a reached file's shared-constant reads to the closure's.
func (s *setupSelector) collectShared(name string) {
	table, ok := s.exportsOf(name)
	if !ok {
		return
	}
	uses, sites := s.sharedUses(name, table)
	for value, use := range uses {
		reached := s.shared.reached[value]
		if reached == nil {
			reached = &sharedUse{keys: map[string]bool{}}
			s.shared.reached[value] = reached
		}
		for key := range use.keys {
			reached.keys[key] = true
		}
	}
	s.shared.sites = append(s.shared.sites, sites...)
}

// seedersOfSharedKeys selects every setup file that reads a literal key the closure reads of the
// same exported constant, or reads that constant through a non-literal key (TSA-V0-006). Each new
// seeder is enqueued so its own dependencies are traversed; it reports whether any was.
func (s *setupSelector) seedersOfSharedKeys() bool {
	names := make([]string, 0, len(s.shared.setupUses))
	for name := range s.shared.setupUses {
		names = append(names, name)
	}
	sort.Strings(names)
	enqueued := false
	for _, name := range names {
		if s.visited[name] || s.seeds[name] {
			continue
		}
		for value, use := range s.shared.setupUses[name] {
			reached := s.shared.reached[value]
			if reached == nil || s.seeds[name] {
				continue
			}
			for key := range use.keys {
				s.seeds[name] = s.seeds[name] || reached.keys[key]
			}
			s.seeds[name] = s.seeds[name] || (use.open != 0 && len(reached.keys) != 0)
		}
		if s.seeds[name] {
			s.enqueue(name, false)
			enqueued = true
		}
	}
	return enqueued
}

// sharedReasons fails closed once the closure is complete (TSA-V0-007). While any shared-constant
// read is reached, ALL is returned for: a reached non-literal read of a constant that an
// unselected setup file also reads; a reached read the selector cannot attribute to one constant
// while an unselected setup file reads any; an unselected setup file holding such a read itself;
// and an unreadable setup file.
func (s *setupSelector) sharedReasons() {
	if len(s.shared.reached) == 0 && len(s.shared.sites) == 0 {
		return
	}
	candidates := map[string]bool{}
	for name := range s.shared.setupUses {
		candidates[name] = true
	}
	for name := range s.shared.setupOpen {
		candidates[name] = true
	}
	setups := make([]string, 0, len(candidates))
	for name := range candidates {
		if !s.visited[name] && !s.seeds[name] {
			setups = append(setups, name)
		}
	}
	sort.Strings(setups)
	for _, site := range s.shared.sites {
		for _, setup := range setups {
			_, reads := s.shared.setupUses[setup][site.value]
			if site.any {
				reads = len(s.shared.setupUses[setup]) != 0 || len(s.shared.setupOpen[setup]) != 0
			}
			if setup != site.path && reads {
				detail := site.detail + " that " + setup + " also reads"
				if site.any {
					detail = site.detail + " while " + setup + " reads a shared constant"
				}
				s.reason("shared-constant-key-unresolved", site.path, site.line, detail)
				break
			}
		}
	}
	for _, setup := range setups {
		if open := s.shared.setupOpen[setup]; len(open) != 0 {
			s.reason("shared-constant-key-unresolved", setup, open[0].line, "a setup file that is not selected reads a shared constant the selector cannot attribute: "+open[0].detail)
		}
	}
	for _, name := range s.shared.unreadable {
		s.reason("source-unreadable", name, 0, "a setup file that may read a shared constant cannot be read")
	}
}
