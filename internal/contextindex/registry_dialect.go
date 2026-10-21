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

	"github.com/Beamfall/corvint/internal/projectprofile"
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

// registryObject finds `export const|let|var NAME [: Type] = {`, or a
// top-level `const NAME [: Type] = {` the file does not export (NGI-V0-014),
// and returns the object literal's opening and closing token indexes.
func registryObject(tokens []registryToken, export string) (int, int, bool) {
	nesting := 0
	for at := 0; at+3 < len(tokens); at++ {
		if token := tokens[at]; token.kind == webPunct && strings.Contains("([{", token.text) {
			nesting++
		} else if token.kind == webPunct && strings.Contains(")]}", token.text) && nesting > 0 {
			nesting--
		}
		if at > 0 && isRegistryPunct(tokens[at-1], ".") {
			continue
		}
		// keyword is the index of const, let or var.
		keyword := at + 1
		switch {
		case isRegistryWord(tokens[at], "export"):
			if word := tokens[keyword]; !isRegistryWord(word, "const") && !isRegistryWord(word, "let") && !isRegistryWord(word, "var") {
				continue
			}
		case nesting == 0 && isRegistryWord(tokens[at], "const") && (at == 0 || !isRegistryWord(tokens[at-1], "export")) &&
			!registryExportedOtherwise(tokens, export):
			keyword = at
		default:
			continue
		}
		if !isRegistryWord(tokens[keyword+1], export) {
			continue
		}
		next := registryAnnotationEnd(tokens, keyword+2)
		if next+1 < len(tokens) && isRegistryPunct(tokens[next], "=") && isRegistryPunct(tokens[next+1], "{") {
			return next + 1, registryClose(tokens, next+1), true
		}
		return 0, 0, false
	}
	return 0, 0, false
}

// registryAnnotationEnd skips an optional `: Type` annotation starting at next
// and returns the index of the top-level `=` that ends it, or next unchanged
// when no annotation starts there.
func registryAnnotationEnd(tokens []registryToken, next int) int {
	if next >= len(tokens) || !isRegistryPunct(tokens[next], ":") {
		return next
	}
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
	return next
}

// registryExportedOtherwise reports whether the file may export some other
// binding under name, so that a non-exported `const name` is not what the
// declaration names: an `export { ... }` clause mentioning name, an
// `export *`, an `export =`, or a CommonJS `exports` reference.
func registryExportedOtherwise(tokens []registryToken, name string) bool {
	for at, token := range tokens {
		if isRegistryWord(token, "exports") {
			return true
		}
		if !isRegistryWord(token, "export") || registryMember(tokens, at) || at+1 >= len(tokens) {
			continue
		}
		if next := tokens[at+1]; isRegistryPunct(next, "*") || isRegistryPunct(next, "=") {
			return true
		} else if !isRegistryPunct(next, "{") {
			continue
		}
		for inner := at + 2; inner < len(tokens) && !isRegistryPunct(tokens[inner], "}"); inner++ {
			if isRegistryWord(tokens[inner], name) {
				return true
			}
		}
	}
	return false
}

// registryEntry is one resolved `key -> imported binding` edge of a registry.
// A lazy entry (NGI-V0-013) names its module only by the specifier of a
// require() or import() inside an arrow; target is that specifier resolved
// by the reverse-import rule, without its suffix.
type registryEntry struct {
	key     string
	line    int
	binding registryBinding
	lazy    bool
	target  string
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
	loader := registryRequireFree(tokens)
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
			entry, ok := registryEntryOf(tokens[at:end], bindings, loader)
			if ok && entry.lazy {
				entry.target, ok = registryLazyTarget(index, declaration.Path, entry.binding.specifier)
			}
			if ok {
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
func registryEntryOf(property []registryToken, bindings map[string]registryBinding, loader bool) (registryEntry, bool) {
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
	if binding, ok := registryLazyBinding(value, loader); ok {
		return registryEntry{key: key, line: first.line, binding: binding, lazy: true}, true
	}
	binding, ok := registryValueBinding(value, bindings)
	return registryEntry{key: key, line: first.line, binding: binding}, ok
}

// registryLazyBinding reads a lazy loader value (NGI-V0-013): an arrow with
// no parameters whose body is `new (require(S).X)()`, `require(S).X` or
// `import(S).then(P => P.X)` (or `P => new P.X()`), optionally followed by
// `as` or `satisfies` assertions, where S is a closed string literal. The
// require forms count only when loader reports that `require` in this file
// can only be the module loader. Constructor arguments and any other callback
// may reach another module, so any other arrow stays unresolved.
func registryLazyBinding(value []registryToken, loader bool) (registryBinding, bool) {
	if len(value) < 4 || !isRegistryPunct(value[0], "(") || !isRegistryPunct(value[1], ")") ||
		!isRegistryPunct(value[2], "=") || !isRegistryPunct(value[3], ">") {
		return registryBinding{}, false
	}
	body := value[4:]
	load := func(at int, keyword string) (string, int, bool) {
		if at+3 >= len(body) || !isRegistryWord(body[at], keyword) || !isRegistryPunct(body[at+1], "(") || !isRegistryPunct(body[at+3], ")") {
			return "", 0, false
		}
		specifier, ok := registryLiteral(body[at+2])
		return specifier, at + 4, ok
	}
	member := func(at int) (string, bool) {
		if at+1 >= len(body) || !isRegistryPunct(body[at], ".") || body[at+1].kind != webWord {
			return "", false
		}
		return body[at+1].text, true
	}
	binding, at, ok := registryBinding{}, 0, false
	if specifier, next, loaded := load(2, "require"); loader && loaded && isRegistryWord(body[0], "new") && isRegistryPunct(body[1], "(") {
		// new (require(S).X)()
		name, named := member(next)
		if ok = named && next+4 < len(body) && isRegistryPunct(body[next+2], ")") && isRegistryPunct(body[next+3], "(") &&
			isRegistryPunct(body[next+4], ")"); ok {
			binding, at = registryBinding{specifier, name}, next+5
		}
	} else if specifier, next, loaded := load(0, "require"); loader && loaded {
		// require(S).X
		name, named := member(next)
		binding, at, ok = registryBinding{specifier, name}, next+2, named
	} else if specifier, next, loaded := load(0, "import"); loaded {
		// import(S).then(P => P.X)
		name, named := member(next)
		if ok = named && name == "then" && next+2 < len(body) && isRegistryPunct(body[next+2], "("); ok {
			close := registryClose(body, next+2)
			ok = close < len(body) && registryThenCallback(body[next+3:close])
			binding, at = registryBinding{specifier, "*"}, close+1
		}
	}
	return binding, ok && registryAssertionTail(body, at)
}

// registryThenCallback admits only `P => P.X` and `P => new P.X()`, P
// optionally parenthesised: a callback that reads nothing but the loaded
// module.
func registryThenCallback(callback []registryToken) bool {
	at := 0
	parenthesised := len(callback) > 0 && isRegistryPunct(callback[0], "(")
	if parenthesised {
		at++
	}
	if at >= len(callback) || callback[at].kind != webWord {
		return false
	}
	parameter := callback[at].text
	at++
	if parenthesised {
		if at >= len(callback) || !isRegistryPunct(callback[at], ")") {
			return false
		}
		at++
	}
	if at+1 >= len(callback) || !isRegistryPunct(callback[at], "=") || !isRegistryPunct(callback[at+1], ">") {
		return false
	}
	at += 2
	constructed := at < len(callback) && isRegistryWord(callback[at], "new")
	if constructed {
		at++
	}
	if at+2 >= len(callback) || !isRegistryWord(callback[at], parameter) || !isRegistryPunct(callback[at+1], ".") || callback[at+2].kind != webWord {
		return false
	}
	at += 3
	if constructed {
		if at+1 >= len(callback) || !isRegistryPunct(callback[at], "(") || !isRegistryPunct(callback[at+1], ")") {
			return false
		}
		at += 2
	}
	return at == len(callback)
}

// registryAssertionTail reports whether value[at:] is empty or only `as T` and
// `satisfies T` assertions, T a dotted type name. Anything else (a generic, an
// operator, a call) may continue the runtime expression past the binding.
func registryAssertionTail(value []registryToken, at int) bool {
	for at < len(value) {
		if !(isRegistryWord(value[at], "as") || isRegistryWord(value[at], "satisfies")) || at+1 >= len(value) || value[at+1].kind != webWord {
			return false
		}
		for at += 2; at+1 < len(value) && isRegistryPunct(value[at], ".") && value[at+1].kind == webWord; at += 2 {
		}
	}
	return true
}

// registryRequireFree reports whether `require` in this file can only be the
// module loader: the file has no identifier escape, and every occurrence
// outside a member access is a call that does not declare a function. Any
// other occurrence (a declaration, parameter, import or assignment) may bind
// the name to something else. A TypeScript ambient `declare const|let|var
// require: TYPE` only types the loader and binds no value (V1-1479).
func registryRequireFree(tokens []registryToken) bool {
	for at, token := range tokens {
		if isRegistryPunct(token, "\\") {
			return false
		}
		if !isRegistryWord(token, "require") || registryMember(tokens, at) || registryAmbientDeclaration(tokens, at) {
			continue
		}
		if at+1 >= len(tokens) || !isRegistryPunct(tokens[at+1], "(") ||
			at > 0 && (isRegistryWord(tokens[at-1], "function") || isRegistryPunct(tokens[at-1], "*")) {
			return false
		}
	}
	return true
}

// registryAmbientDeclaration reports whether the word at at is the name in a
// TypeScript ambient `declare const|let|var NAME:` declaration on one line,
// which emits no runtime binding.
func registryAmbientDeclaration(tokens []registryToken, at int) bool {
	if at < 2 || at+1 >= len(tokens) || !isRegistryPunct(tokens[at+1], ":") {
		return false
	}
	keyword, declare := tokens[at-1], tokens[at-2]
	return isRegistryWord(declare, "declare") && declare.line == keyword.line && !registryMember(tokens, at-2) &&
		(isRegistryWord(keyword, "const") || isRegistryWord(keyword, "let") || isRegistryWord(keyword, "var"))
}

// registryMember reports whether the token at at follows a member-access dot.
// The lexer splits a spread or rest `...` into three dots, which is not one.
func registryMember(tokens []registryToken, at int) bool {
	return at > 0 && isRegistryPunct(tokens[at-1], ".") && !(at > 1 && isRegistryPunct(tokens[at-2], "."))
}

// registryLazyTarget resolves a lazy entry's specifier from the registry file
// by the reverse-import rule (a relative or profile-alias specifier) and
// reports false unless it names an indexed JavaScript or TypeScript source; a
// bare or unmatched specifier leaves the entry unresolved.
func registryLazyTarget(index *Index, from, specifier string) (string, bool) {
	profile := projectprofile.ByID(index.ProfileID)
	if !webSpecifierAdmitted(specifier, profile) {
		return "", false
	}
	target := webImportTarget(from, specifier, profile)
	for suffix := range webSuffixes {
		for _, candidate := range []string{target + suffix, strings.TrimRight(target, "/") + "/index" + suffix} {
			if _, indexed := index.Sources[candidate]; indexed {
				return target, true
			}
		}
	}
	return "", false
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
	binding, imported := bindings[head]
	return binding, imported && registryAssertionTail(value, at)
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
		// arguments maps a call site of this file whose first argument is a
		// lone identifier to that argument's token index (NGI-V0-015).
		arguments := map[int]int{}
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
			key, literal, lone := "", false, false
			if open+1 < close {
				key, literal = registryLiteral(tokens[open+1])
				lone = open+2 == close || isRegistryPunct(tokens[open+2], ",")
				literal = literal && lone
			}
			after := close + 1
			declared := !member && !literal && after < len(tokens) && (isRegistryPunct(tokens[after], "{") || isRegistryPunct(tokens[after], ":")) &&
				(at == 0 || registryDefinitionPrefix[tokens[at-1].text])
			if declared {
				continue
			}
			site.key, site.literal = key, literal
			if lone && tokens[open+1].kind == webWord {
				arguments[len(sites)] = open + 1
			}
			sites = append(sites, site)
		}
		constants := registryConstants(tokens, arguments)
		for at, argument := range arguments {
			if key, bound := constants[tokens[argument].text]; bound {
				sites[at].key, sites[at].literal = key, true
			}
		}
	}
	return sites, unavailable
}

// registryConstants reads the file's top-level `const NAME = LITERAL`
// bindings (NGI-V0-015), LITERAL a closed registry literal optionally
// followed by `as const` and ending the statement. A name is kept only when
// every other occurrence of it outside a member access is a lone accessor
// argument in arguments: any other occurrence may rebind or shadow it (a
// parameter, a nested or repeated declaration, an import, an
// assignment), so the name stays a non-literal key.
func registryConstants(tokens []registryToken, arguments map[int]int) map[string]string {
	if len(arguments) == 0 {
		return nil
	}
	allowed := map[int]bool{}
	for _, argument := range arguments {
		allowed[argument] = true
	}
	constants, declared, nesting := map[string]string{}, map[string]int{}, 0
	for at, token := range tokens {
		// A sloppy-mode `with (object)` statement may resolve any name to a
		// property of object, and an identifier escape (`\u004bEY`) may spell
		// a name the lexer does not see, so no name of such a file is closed.
		if isRegistryPunct(token, "\\") ||
			isRegistryWord(token, "with") && at+1 < len(tokens) && isRegistryPunct(tokens[at+1], "(") && !registryMember(tokens, at) {
			return nil
		}
		if token.kind == webPunct && strings.Contains("([{", token.text) {
			nesting++
		} else if token.kind == webPunct && strings.Contains(")]}", token.text) && nesting > 0 {
			nesting--
		}
		if nesting != 0 || !isRegistryWord(token, "const") || registryMember(tokens, at) ||
			at+3 >= len(tokens) || tokens[at+1].kind != webWord || !isRegistryPunct(tokens[at+2], "=") {
			continue
		}
		key, ok := registryLiteral(tokens[at+3])
		end := at + 4
		if end+1 < len(tokens) && isRegistryWord(tokens[end], "as") && isRegistryWord(tokens[end+1], "const") {
			end += 2
		}
		if end < len(tokens) {
			next := tokens[end]
			continued := next.kind == webWord && (next.text == "in" || next.text == "instanceof" || next.text == "as" || next.text == "satisfies")
			ok = ok && (isRegistryPunct(next, ";") || isRegistryPunct(next, ",") ||
				next.kind == webWord && next.line > tokens[end-1].line && !continued)
		}
		name := tokens[at+1].text
		if _, repeated := declared[name]; ok && !repeated {
			constants[name], declared[name] = key, at+1
		} else {
			delete(constants, name)
			declared[name] = -1
		}
	}
	for at, token := range tokens {
		position, named := declared[token.text]
		if token.kind != webWord || !named || at == position || allowed[at] || registryMember(tokens, at) {
			continue
		}
		delete(constants, token.text)
	}
	return constants
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
	// A lazy entry binds a JS/TS changed path the way rule (c) binds an
	// importer to it: by the specifier's suffix-free target.
	web := webSuffixes[strings.ToLower(pythonPathSuffix(changedPath))]
	stem := pythonWithoutSuffix(changedPath)
	bound := func(entry registryEntry) bool {
		return entry.lazy && web && (entry.target == stem || strings.TrimRight(entry.target, "/")+"/index" == stem)
	}
	for _, registry := range dialect.registries {
		declaration := registry.declaration
		specifiers := map[string]bool{}
		for _, edge := range importers {
			if edge.path == declaration.Path {
				specifiers[edge.imported] = true
			}
		}
		lazy := false
		for _, entry := range registry.entries {
			lazy = lazy || bound(entry)
		}
		changedRegistry := changedPath == declaration.Path
		// A registry whose export cannot be read, or that holds an entry
		// whose module is not closed, may map a key to any JS/TS path, so it
		// applies to every JS/TS changed path, widened (NGI-V0-010,
		// NGI-V0-012), and to no other language's path (V1-1479).
		webPath := language == "javascript" || language == "typescript"
		openEntry := (registry.unresolved != 0 || registry.missing) && webPath
		if !changedRegistry && len(specifiers) == 0 && !lazy && !openEntry {
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
			if !entry.lazy && specifiers[entry.binding.specifier] || bound(entry) {
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
