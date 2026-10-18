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

const (
	// maxSharedReexport bounds the `export ... from` chain followed to an exported constant.
	maxSharedReexport = 8
	// maxSharedTemplateDepth bounds template substitutions nested inside substitutions.
	maxSharedTemplateDepth = 8
	// sharedDefaultObject is the local name recorded for `export default { ... }`.
	sharedDefaultObject = "\x00default-object"
)

// sharedConst identifies one exported constant object by the module that declares it and its
// local name there, so every import or re-export path to the same object compares equal.
type sharedConst struct{ module, local string }

// sharedUse is what one file reads of one shared constant: its literal property keys, and the
// line of the first read whose key is not a literal (zero when every read is literal). The
// closure's aggregate keeps only keys; its non-literal reads are kept as sharedSite rows.
type sharedUse struct {
	keys map[string]bool
	open int
}

// sharedSite is one non-literal read of a shared constant in a reached file.
type sharedSite struct {
	path  string
	line  int
	name  string
	value sharedConst
	// namespace marks a namespace import used other than as `ns.NAME`: any constant it exports may
	// be read, so it is matched against every constant a setup file reads.
	namespace bool
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
}

type sharedResolution struct {
	value sharedConst
	ok    bool
}

// sharedState is the selector's shared-constant bookkeeping.
type sharedState struct {
	exports    map[string]*moduleExports
	resolved   map[[2]string]sharedResolution
	setupUses  map[string]map[sharedConst]*sharedUse
	unreadable []string
	reached    map[sharedConst]*sharedUse
	sites      []sharedSite
}

func newSharedState() *sharedState {
	return &sharedState{exports: map[string]*moduleExports{}, resolved: map[[2]string]sharedResolution{},
		setupUses: map[string]map[sharedConst]*sharedUse{}, reached: map[sharedConst]*sharedUse{}}
}

// sharedTokens lexes text like registryTokens and also lexes the `${...}` substitutions of every
// template literal, which the web lexer folds into one opaque string token, so a property read
// inside a template is still seen. Each substitution is wrapped in parentheses.
func sharedTokens(text string, depth int) ([]registryToken, bool) {
	lexer := &webLexer{text: text}
	tokens := make([]registryToken, 0)
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
		if token.kind != webString || token.quoted || token.template || end == 0 || text[end-1] != '`' || depth >= maxSharedTemplateDepth {
			continue
		}
		start := sharedTokenStart(text, before)
		first := line - strings.Count(text[start:end], "\n")
		for _, body := range templateSubstitutions(text[start:end]) {
			inner, _ := sharedTokens(body, depth+1)
			tokens = append(tokens, registryToken{webToken{kind: webPunct, text: "("}, first})
			for _, token := range inner {
				token.line += first - 1
				tokens = append(tokens, token)
			}
			tokens = append(tokens, registryToken{webToken{kind: webPunct, text: ")"}, first})
		}
	}
	return tokens, lexer.unterminated == ""
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
	tokens, _ := sharedTokens(text, 0)
	table := &moduleExports{tokens: tokens, bindings: registryImports(tokens), local: map[string]string{}, from: map[string]registryBinding{}}
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

// resolveConst follows name exported by module, through `export ... from`, `export *` and local
// re-exports of imports, to the constant object that declares it. It reports false for anything
// that is not an exported object literal, and for a chain beyond maxSharedReexport.
func (s *setupSelector) resolveConst(module, name string, depth int) (sharedConst, bool) {
	key := [2]string{module, name}
	if result, ok := s.shared.resolved[key]; ok {
		return result.value, result.ok
	}
	s.shared.resolved[key] = sharedResolution{}
	result := sharedResolution{}
	if table, ok := s.exportsOf(module); ok && depth <= maxSharedReexport {
		forward := func(specifier, imported string) {
			if imported == "*" || result.ok {
				return
			}
			if resolution := s.resolver.Resolve(module, specifier); resolution.State == WebImportRepository {
				result.value, result.ok = s.resolveConst(resolution.Target, imported, depth+1)
			}
		}
		if local, exported := table.local[name]; exported {
			switch binding, imported := table.bindings[local]; {
			case local == sharedDefaultObject:
				result = sharedResolution{sharedConst{module, local}, true}
			case imported:
				forward(binding.specifier, binding.imported)
			case constObject(table.tokens, local):
				result = sharedResolution{sharedConst{module, local}, true}
			}
		} else if binding, ok := table.from[name]; ok {
			forward(binding.specifier, binding.imported)
		} else if name != "default" {
			for _, specifier := range table.stars {
				forward(specifier, name)
			}
		}
	}
	s.shared.resolved[key] = result
	return result.value, result.ok
}

// sharedUses reads every property access of an imported shared constant in one file.
func (s *setupSelector) sharedUses(name string, tokens []registryToken) (map[sharedConst]*sharedUse, []sharedSite) {
	uses, sites := map[sharedConst]*sharedUse{}, []sharedSite{}
	type target struct {
		module, imported string
	}
	locals := map[string]target{}
	for local, binding := range registryImports(tokens) {
		if resolution := s.resolver.Resolve(name, binding.specifier); resolution.State == WebImportRepository {
			locals[local] = target{resolution.Target, binding.imported}
		}
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
			sites = append(sites, sharedSite{path: name, line: tokens[at].line, name: label, value: value})
		}
	}
	for at, token := range tokens {
		bound, imported := locals[token.text]
		if token.kind != webWord || !imported || registryMember(tokens, at) || sharedNonRead(tokens, at) {
			continue
		}
		if bound.imported != "*" {
			if value, ok := s.resolveConst(bound.module, bound.imported, 0); ok {
				record(value, at, token.text)
			}
			continue
		}
		// A namespace import: `ns.NAME` names one export; any other use may read every export.
		if at+2 < len(tokens) && isRegistryPunct(tokens[at+1], ".") && tokens[at+2].kind == webWord {
			if value, ok := s.resolveConst(bound.module, tokens[at+2].text, 0); ok {
				record(value, at+2, token.text+"."+tokens[at+2].text)
			}
			continue
		}
		sites = append(sites, sharedSite{path: name, line: token.line, name: token.text, namespace: true})
	}
	return uses, sites
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
		text, ok := s.text(name)
		if !ok {
			s.shared.unreadable = append(s.shared.unreadable, name)
			continue
		}
		tokens, _ := sharedTokens(text, 0)
		uses, _ := s.sharedUses(name, tokens)
		if len(uses) != 0 {
			s.shared.setupUses[name] = uses
		}
	}
}

// collectShared adds a reached file's shared-constant reads to the closure's.
func (s *setupSelector) collectShared(name, text string) {
	tokens, _ := sharedTokens(text, 0)
	uses, sites := s.sharedUses(name, tokens)
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

// sharedReasons fails closed once the closure is complete (TSA-V0-007): a reached non-literal read
// of a constant that an unselected setup file also reads, or any shared-constant read while a setup
// file cannot be read, returns ALL.
func (s *setupSelector) sharedReasons() {
	setups := make([]string, 0, len(s.shared.setupUses))
	for name := range s.shared.setupUses {
		if !s.visited[name] && !s.seeds[name] {
			setups = append(setups, name)
		}
	}
	sort.Strings(setups)
	for _, site := range s.shared.sites {
		for _, setup := range setups {
			_, reads := s.shared.setupUses[setup][site.value]
			if setup != site.path && (reads || site.namespace) {
				detail := "a non-literal read of " + site.name + " that " + setup + " also reads"
				if site.namespace {
					detail = "namespace " + site.name + " used other than as NAMESPACE.EXPORT while " + setup + " reads a shared constant"
				}
				s.reason("shared-constant-key-unresolved", site.path, site.line, detail)
				break
			}
		}
	}
	if len(s.shared.reached) != 0 || len(s.shared.sites) != 0 {
		for _, name := range s.shared.unreadable {
			s.reason("source-unreadable", name, 0, "a setup file that may read a shared constant cannot be read")
		}
	}
}
