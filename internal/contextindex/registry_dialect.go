package contextindex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The string-keyed registry dialect (NGI-V0-007 to NGI-V0-011) carries the
// non-Go syntax impact of a module that tests reach only through a literal key:
// an exported object literal maps each key to an imported binding, and an
// adopter-declared accessor takes that key at its call sites. It is a declared,
// syntax-only dialect in the style of the application map's router dialects. It
// evaluates nothing, infers no accessor, and folds no constant beyond a literal;
// every construct it cannot read widens selection instead of dropping a key.

// RegistryDialect is the one admitted registry dialect name.
const RegistryDialect = "string-key-registry/0"

const (
	registryManifestSchema   = "application-map-manifest/0"
	maxRegistryManifestBytes = 256 << 10
	maxRegistryDeclarations  = 64
	maxRegistryAccessors     = 16
	maxRegistryKeyBytes      = 256
)

var registryIdentifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`)

// RegistryDeclaration names one exported object-literal registry and the
// accessor methods whose first argument is one of its keys. It is the
// `registries` member of a committed application-map-manifest/0 document.
type RegistryDeclaration struct {
	Path      string   `json:"path"`
	Export    string   `json:"export"`
	Dialect   string   `json:"dialect"`
	Accessors []string `json:"accessors"`
}

// ValidateRegistryDeclarations is the closed check the application map's
// manifest decoder and impact's discovery share (NGI-V0-007).
func ValidateRegistryDeclarations(declarations []RegistryDeclaration) error {
	if len(declarations) > maxRegistryDeclarations {
		return fmt.Errorf("registries must list at most %d entries", maxRegistryDeclarations)
	}
	seen := map[string]bool{}
	for _, declaration := range declarations {
		if declaration.Dialect != RegistryDialect {
			return fmt.Errorf("registry %s: unsupported dialect %s", declaration.Path, declaration.Dialect)
		}
		language := NonGoImpactLanguage(declaration.Path)
		first, _, _ := strings.Cut(declaration.Path, "/")
		if path.Clean(declaration.Path) != declaration.Path || path.IsAbs(declaration.Path) || strings.HasPrefix(declaration.Path, "../") ||
			strings.Contains(declaration.Path, "\\") || strings.EqualFold(first, ".git") || (language != "javascript" && language != "typescript") {
			return fmt.Errorf("registry path %q is not a repository-relative JavaScript or TypeScript path", declaration.Path)
		}
		if !registryIdentifier.MatchString(declaration.Export) {
			return fmt.Errorf("registry %s: export must match %s", declaration.Path, registryIdentifier)
		}
		if seen[declaration.Path+"\x00"+declaration.Export] {
			return fmt.Errorf("registry %s export %s is repeated", declaration.Path, declaration.Export)
		}
		seen[declaration.Path+"\x00"+declaration.Export] = true
		if len(declaration.Accessors) == 0 || len(declaration.Accessors) > maxRegistryAccessors {
			return fmt.Errorf("registry %s: accessors must list 1 to %d names", declaration.Path, maxRegistryAccessors)
		}
		names := map[string]bool{}
		for _, accessor := range declaration.Accessors {
			if !registryIdentifier.MatchString(accessor) || names[accessor] {
				return fmt.Errorf("registry %s: accessor %q is not a unique identifier", declaration.Path, accessor)
			}
			names[accessor] = true
		}
	}
	return nil
}

// declaredRegistries reads the `registries` member of every committed
// application-map-manifest/0 document at the evaluated revision. A manifest
// whose member does not decode or validate is counted, never skipped silently;
// so is a manifest-naming document that mentions `registries` but is too large
// or not strict JSON (a duplicate member included), as the map refuses it too.
// The schema name is matched before decoding, so a manifest that spells it
// with `\u` escapes of its letters is not discovered (a retained limit).
func declaredRegistries(index *Index) ([]RegistryDeclaration, int) {
	names := make([]string, 0)
	for name := range index.Sources {
		if path.Ext(name) == ".json" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	declarations, invalid := []RegistryDeclaration{}, 0
	for _, name := range names {
		text, valid, loaded := index.Sources[name].Text()
		if !valid || !loaded || !strings.Contains(text, "application-map-manifest") {
			continue
		}
		if len(text) > maxRegistryManifestBytes || !strictJSON(text) {
			if strings.Contains(text, "registries") {
				invalid++
			}
			continue
		}
		var head struct {
			Schema     string          `json:"schema"`
			Registries json.RawMessage `json:"registries"`
		}
		if json.Unmarshal([]byte(text), &head) != nil || head.Schema != registryManifestSchema || head.Registries == nil {
			continue
		}
		var members []RegistryDeclaration
		decoder := json.NewDecoder(bytes.NewReader(head.Registries))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&members) != nil || decoder.Decode(new(any)) != io.EOF ||
			ValidateRegistryDeclarations(members) != nil {
			invalid++
			continue
		}
		declarations = append(declarations, members...)
	}
	return declarations, invalid
}

// strictJSON reports whether text is one JSON document without duplicate
// members, the reading the application map's manifest decoder enforces.
func strictJSON(text string) bool {
	_, err := wire.Parse([]byte(text))
	return err == nil
}

// registryToken is a web lexer token with the 1-based line it ends on.
type registryToken struct {
	webToken
	line int
}

func registryTokens(text string) ([]registryToken, bool) {
	lexer := &webLexer{text: text}
	tokens := make([]registryToken, 0)
	line, counted := 1, 0
	for {
		token, ok := lexer.next()
		if !ok {
			break
		}
		if end := lexer.position - 1; end > counted {
			line += strings.Count(text[counted:end], "\n")
			counted = end
		}
		tokens = append(tokens, registryToken{token, line})
	}
	return tokens, lexer.unterminated == ""
}

func isRegistryPunct(token registryToken, text string) bool {
	return token.kind == webPunct && token.text == text
}

func isRegistryWord(token registryToken, text string) bool {
	return token.kind == webWord && token.text == text
}

// registryLiteral admits a closed quoted string or a substitution-free
// template with a printable body and no escape: the only key it compares.
func registryLiteral(token registryToken) (string, bool) {
	if token.kind != webString || !(token.quoted || token.template) || token.text == "" || len(token.text) > maxRegistryKeyBytes {
		return "", false
	}
	for _, character := range token.text {
		if character < 0x20 || character == 0x7f || character == '\\' {
			return "", false
		}
	}
	return token.text, true
}

// registryClose returns the index of the bracket closing the one at open, or
// len(tokens) when it is never closed.
func registryClose(tokens []registryToken, open int) int {
	depth := 0
	for at := open; at < len(tokens); at++ {
		if tokens[at].kind != webPunct {
			continue
		}
		switch tokens[at].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return at
			}
		}
	}
	return len(tokens)
}

// registryBinding is the module specifier an import statement binds a local
// name from, and the name it imports ("default", "*" or the exported name).
type registryBinding struct{ specifier, imported string }

// registryImports reads the value bindings of every static import statement.
// Type-only statements bind nothing a registry value can construct.
func registryImports(tokens []registryToken) map[string]registryBinding {
	bindings := map[string]registryBinding{}
	for at := 0; at < len(tokens); at++ {
		if !isRegistryWord(tokens[at], "import") || (at > 0 && isRegistryPunct(tokens[at-1], ".")) {
			continue
		}
		end := at + 1
		for end < len(tokens) && !(isRegistryWord(tokens[end], "from") && end+1 < len(tokens) && tokens[end+1].quoted) && webClauseToken(tokens[end].webToken) {
			end++
		}
		if end+1 >= len(tokens) || !isRegistryWord(tokens[end], "from") || !tokens[end+1].quoted {
			continue
		}
		specifier, clause := tokens[end+1].text, tokens[at+1:end]
		if len(clause) > 1 && isRegistryWord(clause[0], "type") && !isRegistryPunct(clause[1], ",") {
			continue
		}
		for position := 0; position < len(clause); position++ {
			switch {
			case isRegistryPunct(clause[position], "*") && position+2 < len(clause) && isRegistryWord(clause[position+1], "as"):
				bindings[clause[position+2].text] = registryBinding{specifier, "*"}
				position += 2
			case isRegistryPunct(clause[position], "{"):
				position++
				for position < len(clause) && !isRegistryPunct(clause[position], "}") {
					start := position
					for position < len(clause) && !isRegistryPunct(clause[position], ",") && !isRegistryPunct(clause[position], "}") {
						position++
					}
					parts := clause[start:position]
					if position < len(clause) && isRegistryPunct(clause[position], ",") {
						position++
					}
					if len(parts) == 0 || (len(parts) > 1 && isRegistryWord(parts[0], "type") && !isRegistryWord(parts[1], "as")) {
						continue
					}
					local := parts[0].text
					if len(parts) == 3 && isRegistryWord(parts[1], "as") {
						local = parts[2].text
					}
					bindings[local] = registryBinding{specifier, parts[0].text}
				}
			case clause[position].kind == webWord:
				bindings[clause[position].text] = registryBinding{specifier, "default"}
			}
		}
	}
	return bindings
}

// registryObject finds `export const|let|var NAME [: Type] = {` and returns
// the object literal's opening and closing token indexes.
func registryObject(tokens []registryToken, export string) (int, int, bool) {
	for at := 0; at+3 < len(tokens); at++ {
		if !isRegistryWord(tokens[at], "export") || (at > 0 && isRegistryPunct(tokens[at-1], ".")) {
			continue
		}
		keyword := tokens[at+1]
		if !(isRegistryWord(keyword, "const") || isRegistryWord(keyword, "let") || isRegistryWord(keyword, "var")) || !isRegistryWord(tokens[at+2], export) {
			continue
		}
		next := at + 3
		if isRegistryPunct(tokens[next], ":") {
			depth := 0
			for next++; next < len(tokens); next++ {
				token := tokens[next]
				if isRegistryPunct(token, "=") && next+1 < len(tokens) && isRegistryPunct(tokens[next+1], ">") {
					next++
					continue
				}
				if depth == 0 && isRegistryPunct(token, "=") {
					break
				}
				switch {
				case token.kind != webPunct:
				case strings.Contains("([{<", token.text):
					depth++
				case strings.Contains(")]}>", token.text) && depth > 0:
					depth--
				}
			}
		}
		if next+1 < len(tokens) && isRegistryPunct(tokens[next], "=") && isRegistryPunct(tokens[next+1], "{") {
			return next + 1, registryClose(tokens, next+1), true
		}
		return 0, 0, false
	}
	return 0, 0, false
}

// registryEntry is one resolved `key -> imported binding` edge of a registry.
type registryEntry struct {
	key     string
	line    int
	binding registryBinding
}

// registryFacts is one declared registry as read at the evaluated revision.
// unresolved counts the entries whose key or module syntax cannot close: a
// computed key, a spread, a method, or a value that is not an imported
// binding. missing marks a declared export that is not an object literal in a
// readable declared file, such as a re-export; none of its keys are known.
type registryFacts struct {
	declaration RegistryDeclaration
	blob        string
	entries     []registryEntry
	unresolved  int
	missing     bool
}

func parseRegistry(index *Index, declaration RegistryDeclaration) registryFacts {
	facts := registryFacts{declaration: declaration}
	source, indexed := index.Sources[declaration.Path]
	text, valid, loaded := source.Text()
	if !indexed || !valid || !loaded {
		facts.missing = true
		return facts
	}
	facts.blob = source.BlobHash
	tokens, complete := registryTokens(text)
	open, close, found := registryObject(tokens, declaration.Export)
	if !complete || !found || close >= len(tokens) {
		facts.missing = true
		return facts
	}
	bindings := registryImports(tokens)
	for at := open + 1; at < close; {
		end := at
		for end < close && !isRegistryPunct(tokens[end], ",") {
			if token := tokens[end]; token.kind == webPunct && strings.Contains("([{", token.text) {
				end = registryClose(tokens, end)
			}
			end++
		}
		if end > close {
			facts.unresolved++
			break
		}
		if end > at {
			if entry, ok := registryEntryOf(tokens[at:end], bindings); ok {
				facts.entries = append(facts.entries, entry)
			} else {
				facts.unresolved++
			}
		}
		at = end + 1
	}
	return facts
}

// registryEntryOf reads one property: a literal, identifier or single-literal
// computed key, then `new X(...)`, `X` or a shorthand `X`, where X (or the
// namespace before its first dot) is an imported binding.
func registryEntryOf(property []registryToken, bindings map[string]registryBinding) (registryEntry, bool) {
	first := property[0]
	key, ok, value := "", false, []registryToken(nil)
	switch {
	case first.kind == webString:
		key, ok = registryLiteral(first)
		value = property[1:]
	case first.kind == webWord:
		key, ok = first.text, len(first.text) <= maxRegistryKeyBytes
		value = property[1:]
		if len(value) == 0 {
			value = property[:1]
		} else if !isRegistryPunct(value[0], ":") {
			ok = false
		}
	case isRegistryPunct(first, "[") && len(property) > 2 && isRegistryPunct(property[2], "]"):
		key, ok = registryLiteral(property[1])
		value = property[3:]
	}
	if !ok || len(value) == 0 {
		return registryEntry{}, false
	}
	if isRegistryPunct(value[0], ":") {
		value = value[1:]
	}
	binding, ok := registryValueBinding(value, bindings)
	return registryEntry{key: key, line: first.line, binding: binding}, ok
}

func registryValueBinding(value []registryToken, bindings map[string]registryBinding) (registryBinding, bool) {
	at := 0
	constructed := len(value) > 0 && isRegistryWord(value[0], "new")
	if constructed {
		at++
	}
	if at >= len(value) || value[at].kind != webWord {
		return registryBinding{}, false
	}
	head := value[at].text
	for at++; at+1 < len(value) && isRegistryPunct(value[at], ".") && value[at+1].kind == webWord; at += 2 {
	}
	if constructed && at < len(value) && isRegistryPunct(value[at], "(") {
		at = registryClose(value, at) + 1
	}
	if at < len(value) && (isRegistryWord(value[at], "as") || isRegistryWord(value[at], "satisfies")) {
		at = len(value)
	}
	binding, imported := bindings[head]
	return binding, imported && at == len(value)
}

// registrySite is one call of a declared accessor: literal when its first
// argument is a registry literal, otherwise a non-literal key.
type registrySite struct {
	path, blob, accessor, key string
	line                      int
	literal                   bool
}

// registryDefinitionPrefix is what may precede a method or function name
// being declared rather than called.
var registryDefinitionPrefix = map[string]bool{
	"{": true, "}": true, ";": true, ",": true, "*": true, "function": true, "async": true, "static": true, "public": true,
	"private": true, "protected": true, "override": true, "abstract": true, "declare": true, "get": true, "set": true, "readonly": true,
}

// registryCallSites scans every indexed JavaScript/TypeScript source for calls
// of the declared accessors, including an accessor imported under an alias.
// An unreadable or unterminated source that names an accessor is counted.
func registryCallSites(index *Index, accessors map[string]bool) ([]registrySite, int) {
	names := make([]string, 0)
	for name := range index.Sources {
		if language := NonGoImpactLanguage(name); language == "javascript" || language == "typescript" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	sites, unavailable := []registrySite{}, 0
	for _, name := range names {
		source := index.Sources[name]
		text, valid, loaded := source.Text()
		if !valid || !loaded {
			unavailable++
			continue
		}
		mentioned := false
		for accessor := range accessors {
			mentioned = mentioned || strings.Contains(text, accessor)
		}
		if !mentioned {
			continue
		}
		tokens, complete := registryTokens(text)
		if !complete {
			unavailable++
		}
		local := map[string]string{}
		for accessor := range accessors {
			local[accessor] = accessor
		}
		for alias, binding := range registryImports(tokens) {
			if accessors[binding.imported] {
				local[alias] = binding.imported
			}
		}
		for at, token := range tokens {
			accessor, named := local[token.text]
			if token.kind != webWord || !named {
				continue
			}
			member := at > 0 && isRegistryPunct(tokens[at-1], ".")
			open := at + 1
			if open < len(tokens) && isRegistryPunct(tokens[open], "<") {
				for depth := 0; open < len(tokens); open++ {
					if isRegistryPunct(tokens[open], "<") {
						depth++
					} else if isRegistryPunct(tokens[open], ">") {
						if depth--; depth == 0 {
							open++
							break
						}
					}
				}
			}
			site := registrySite{path: name, blob: source.BlobHash, accessor: accessor, line: token.line}
			if open >= len(tokens) || !isRegistryPunct(tokens[open], "(") {
				// An uncalled member reference (`dp.createTestData`) passes the
				// accessor on; its keys are not visible here.
				if member {
					sites = append(sites, site)
				}
				continue
			}
			close := registryClose(tokens, open)
			key, literal := "", false
			if open+1 < close {
				key, literal = registryLiteral(tokens[open+1])
				literal = literal && (open+2 == close || isRegistryPunct(tokens[open+2], ","))
			}
			after := close + 1
			declared := !member && !literal && after < len(tokens) && (isRegistryPunct(tokens[after], "{") || isRegistryPunct(tokens[after], ":")) &&
				(at == 0 || registryDefinitionPrefix[tokens[at-1].text])
			if declared {
				continue
			}
			site.key, site.literal = key, literal
			sites = append(sites, site)
		}
	}
	return sites, unavailable
}

// registryDialect is the declared registries and accessor call sites of one
// impact call, read once and shared by every changed path.
type registryDialect struct {
	invalid     int
	registries  []registryFacts
	sites       []registrySite
	unavailable int
}

func loadRegistryDialect(index *Index) *registryDialect {
	declarations, invalid := declaredRegistries(index)
	dialect := &registryDialect{invalid: invalid}
	accessors := map[string]bool{}
	for _, declaration := range declarations {
		dialect.registries = append(dialect.registries, parseRegistry(index, declaration))
		for _, accessor := range declaration.Accessors {
			accessors[accessor] = true
		}
	}
	sort.SliceStable(dialect.registries, func(left, right int) bool {
		a, b := dialect.registries[left].declaration, dialect.registries[right].declaration
		return a.Path < b.Path || a.Path == b.Path && a.Export < b.Export
	})
	if len(accessors) != 0 {
		dialect.sites, dialect.unavailable = registryCallSites(index, accessors)
	}
	return dialect
}

func (dialect *registryDialect) declared() bool {
	return dialect.invalid != 0 || len(dialect.registries) != 0
}

// registryCaller accumulates one calling file's row for a changed path.
type registryCaller struct {
	precise  bool
	evidence []any
	seen     map[string]bool
}

func (caller *registryCaller) add(item map[string]any) {
	identity := fmt.Sprintf("%s:%v:%s", item["path"], item["line"], item["reason"])
	if caller.seen[identity] || len(caller.evidence) == maxEvidence {
		return
	}
	caller.seen[identity] = true
	caller.evidence = append(caller.evidence, item)
}

// impact returns the caller rows and structured unknowns the dialect adds for
// one changed path. importers are the changed path's resolved reverse-import
// edges; a registry applies when it is the changed path or imports it.
func (dialect *registryDialect) impact(index *Index, changedPath string, importers []importer) ([]rankedResult, []any) {
	language := NonGoImpactLanguage(changedPath)
	unknowns := []any{}
	unknown := func(code string, count int, fields ...string) {
		row := map[string]any{"path": changedPath, "language": language, "code": code, "line": 0, "count": count}
		for at := 0; at+1 < len(fields); at += 2 {
			row[fields[at]] = fields[at+1]
		}
		unknowns = append(unknowns, row)
	}
	if dialect.invalid != 0 {
		unknown("registry-declaration-invalid", dialect.invalid)
	}
	callers := map[string]*registryCaller{}
	caller := func(site registrySite) *registryCaller {
		if callers[site.path] == nil {
			callers[site.path] = &registryCaller{seen: map[string]bool{}}
		}
		return callers[site.path]
	}
	for _, registry := range dialect.registries {
		declaration := registry.declaration
		specifiers := map[string]bool{}
		for _, edge := range importers {
			if edge.path == declaration.Path {
				specifiers[edge.imported] = true
			}
		}
		changedRegistry := changedPath == declaration.Path
		// A registry whose export cannot be read may map any key to any
		// path, so it applies to every changed path, widened.
		if !changedRegistry && len(specifiers) == 0 && !registry.missing {
			continue
		}
		accessors := map[string]bool{}
		for _, accessor := range declaration.Accessors {
			accessors[accessor] = true
		}
		widen := changedRegistry || registry.missing || registry.unresolved != 0
		if registry.missing {
			unknown("registry-export-unresolved", 1, "registry", declaration.Path)
		}
		if registry.unresolved != 0 {
			unknown("registry-key-unresolved", registry.unresolved, "registry", declaration.Path)
		}
		nonLiteral := map[string]int{}
		for _, site := range dialect.sites {
			if accessors[site.accessor] && !site.literal {
				nonLiteral[site.accessor]++
				widen = true
			}
		}
		for _, accessor := range declaration.Accessors {
			if count := nonLiteral[accessor]; count != 0 {
				unknown("accessor-key-nonliteral", count, "registry", declaration.Path, "accessor", accessor)
			}
		}
		if dialect.unavailable != 0 {
			unknown("registry-caller-unavailable", dialect.unavailable, "registry", declaration.Path)
		}
		entries := map[string][]registryEntry{}
		for _, entry := range registry.entries {
			if specifiers[entry.binding.specifier] {
				entries[entry.key] = append(entries[entry.key], entry)
			}
		}
		for _, site := range dialect.sites {
			if !accessors[site.accessor] || site.path == changedPath {
				continue
			}
			if matched := entries[site.key]; site.literal && len(matched) != 0 {
				row := caller(site)
				row.precise = true
				for _, entry := range matched {
					row.add(evidence(declaration.Path, entry.line, registry.blob,
						"registry "+declaration.Export+" maps key "+site.key+" to "+changedPath, "high", "syntax"))
					row.add(evidence(site.path, site.line, site.blob,
						"passes key "+site.key+" to accessor "+site.accessor, "high", "syntax"))
				}
				continue
			}
			if !widen {
				continue
			}
			reason := "calls accessor " + site.accessor + " of changed registry " + declaration.Path
			switch {
			case changedRegistry:
			case registry.missing:
				reason = "calls accessor " + site.accessor + " of registry " + declaration.Path + " whose export is unresolved"
			case !site.literal:
				reason = "calls accessor " + site.accessor + " with a non-literal key"
			case registry.unresolved != 0:
				reason = "calls accessor " + site.accessor + " while registry " + declaration.Path + " keys are not closed"
			default:
				reason = "calls accessor " + site.accessor + " while another call of a " + declaration.Path + " accessor passes a non-literal key"
			}
			caller(site).add(evidence(site.path, site.line, site.blob, reason, "medium", "syntax"))
		}
	}
	paths := make([]string, 0, len(callers))
	for name := range callers {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	ranked := make([]rankedResult, 0, len(paths))
	for _, name := range paths {
		row := callers[name]
		kind := "reference"
		if isTestPath(name) {
			kind = "test"
		}
		// The precise registry and call-site pair leads a mixed row's evidence.
		sort.SliceStable(row.evidence, func(left, right int) bool {
			return row.evidence[left].(map[string]any)["confidence"] == "high" && row.evidence[right].(map[string]any)["confidence"] != "high"
		})
		score, summary := 600, "calls a registry accessor whose keys do not close over "+changedPath
		if row.precise {
			score, summary = 640, "passes a registry key bound to "+changedPath
		}
		ranked = append(ranked, rankedResult{score: score, order: 1, key: name, result: map[string]any{
			"kind": kind, "id": name, "score": score, "summary": summary, "evidence": row.evidence,
		}})
	}
	return ranked, unknowns
}
