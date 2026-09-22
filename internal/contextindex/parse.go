package contextindex

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	recordStart       = regexp.MustCompile(`^  -[\t-\r \x1c-\x1f\x{0085}\p{Z}]+([A-Za-z][A-Za-z0-9]*):[\t-\r \x1c-\x1f\x{0085}\p{Z}]*(.*)$`)
	recordField       = regexp.MustCompile(`^    ([A-Za-z][A-Za-z0-9]*):[\t-\r \x1c-\x1f\x{0085}\p{Z}]*(.*)$`)
	goFuncPrefix      = regexp.MustCompile(`^[\t-\r \x1c-\x1f\x{0085}\p{Z}]*func[\t-\r \x1c-\x1f\x{0085}\p{Z}]+(?:\([^)]*\)[\t-\r \x1c-\x1f\x{0085}\p{Z}]*)?`)
	goTypePrefix      = regexp.MustCompile(`^[\t-\r \x1c-\x1f\x{0085}\p{Z}]*type[\t-\r \x1c-\x1f\x{0085}\p{Z}]+`)
	goValuePrefix     = regexp.MustCompile(`^[\t-\r \x1c-\x1f\x{0085}\p{Z}]*(?:var|const)[\t-\r \x1c-\x1f\x{0085}\p{Z}]+`)
	documentReference = regexp.MustCompile(`(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+`)
	goImportQuoted    = regexp.MustCompile(`"([^"\n]+)"`)
	genericImport     = regexp.MustCompile(`(?:from[\t-\r \x1c-\x1f\x{0085}\p{Z}]+|import[\t-\r \x1c-\x1f\x{0085}\p{Z}]*(?:\([\t-\r \x1c-\x1f\x{0085}\p{Z}]*)?)[\x22\x27]([^\x22\x27]+)[\x22\x27]`)
	// A document's status is a labelled field, and this repository writes that field
	// four ways: bare (`Status: accepted`), as a list item (`- Status: accepted`),
	// qualified by one word (`Intent status: accepted`), and packed among other
	// fields on one line (`Date: 2026-09-05. Status: accepted. Authority: ...`). A
	// preceding field segment must itself look like `Key: value.` so that ordinary
	// prose ending in a full stop cannot introduce a status, and the field still
	// starts at column zero, so an indented key inside a nested block is not one.
	// The captured value is
	// the status token alone, not the rest of the sentence, because callers compare
	// it against a fixed vocabulary and `accepted (owner instruction 2026-09-07)`
	// is the same status as `accepted`. Keep this identical to the oracle's pattern
	// in `src/context_corvint_index.py` (`GPK-V0-002`).
	documentStatus  = regexp.MustCompile(`(?i)^(?:[-*+][\t ]+)?(?:[^\n:]*:[^\n.]*\.[\t ]+)*(?:\*\*)?(?:[a-z]+[\t ]+)?status(?:\*\*)?:[\t-\r \x1c-\x1f\x{0085}\p{Z}]*[\x22\x27]?([a-z][a-z0-9]*(?:-[a-z0-9]+)*(?::[\t ]*[a-z0-9.-]+)?)`)
	documentTitle   = regexp.MustCompile(`^#[\t-\r \x1c-\x1f\x{0085}\p{Z}]+(.+?)[\t-\r \x1c-\x1f\x{0085}\p{Z}]*$`)
	documentHeading = regexp.MustCompile(`^#{1,6}[\t-\r \x1c-\x1f\x{0085}\p{Z}]+(.+?)[\t-\r \x1c-\x1f\x{0085}\p{Z}]*$`)
)

type pythonInteger string

func parseRecords(source Source, section, kind string) map[string]Record {
	result := make(map[string]Record)
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		return result
	}
	inSection := false
	var current map[string]any
	currentLine := 0
	commit := func() {
		if current == nil {
			return
		}
		identifier, ok := current["id"].(string)
		if ok {
			result[identifier] = Record{kind, identifier, source.Path, source.BlobHash, currentLine, current}
		}
	}
	for index, rawLine := range strings.Split(text, "\n") {
		lineNumber := index + 1
		if !inSection {
			if strings.TrimSpace(rawLine) == section+":" {
				inSection = true
			}
			continue
		}
		if rawLine != "" && rawLine[0] != ' ' && rawLine[0] != '#' {
			break
		}
		if match := recordStart.FindStringSubmatch(rawLine); match != nil {
			commit()
			current = map[string]any{match[1]: scalar(match[2])}
			currentLine = lineNumber
			continue
		}
		if current != nil {
			if match := recordField.FindStringSubmatch(rawLine); match != nil {
				current[match[1]] = scalar(match[2])
			}
		}
	}
	commit()
	return result
}

func scalar(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return []any{}
		}
		parts := splitScalarList(inner)
		literal := make([]any, 0, len(parts))
		validLiteral := true
		for _, part := range parts {
			item, ok := parsePythonLiteral(strings.TrimSpace(part))
			if !ok {
				validLiteral = false
				break
			}
			literal = append(literal, item)
		}
		if validLiteral {
			return literal
		}
		fallback := make([]any, 0, len(parts))
		for _, part := range parts {
			fallback = append(fallback, strings.Trim(strings.TrimSpace(part), `'"`))
		}
		return fallback
	}
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		if decoded, ok := decodePythonString(value); ok {
			return decoded
		}
		return value[1 : len(value)-1]
	}
	switch value {
	case "true":
		return true
	case "false":
		return false
	case "null", "~":
		return nil
	}
	if integer, ok := parsePythonInteger(value); ok {
		return integer
	}
	return value
}

func decodePythonString(value string) (string, bool) {
	if value[0] == '"' {
		decoded, err := strconv.Unquote(value)
		return decoded, err == nil
	}
	var converted strings.Builder
	converted.WriteByte('"')
	inner := value[1 : len(value)-1]
	for index := 0; index < len(inner); index++ {
		character := inner[index]
		if character == '\\' && index+1 < len(inner) {
			if inner[index+1] == '\'' {
				converted.WriteByte('\'')
				index++
				continue
			}
			converted.WriteByte(character)
			index++
			converted.WriteByte(inner[index])
			continue
		}
		if character == '"' {
			converted.WriteByte('\\')
		}
		converted.WriteByte(character)
	}
	converted.WriteByte('"')
	decoded, err := strconv.Unquote(converted.String())
	return decoded, err == nil
}

func parsePythonLiteral(value string) (any, bool) {
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return scalar(value), true
	}
	if integer, ok := parsePythonInteger(value); ok {
		return integer, true
	}
	switch value {
	case "True":
		return true, true
	case "False":
		return false, true
	case "None":
		return nil, true
	default:
		return nil, false
	}
}

func documentKind(value string) string {
	if strings.ToLower(path.Ext(value)) != ".md" {
		return ""
	}
	parts := strings.Split(strings.ToLower(path.Dir(value)), "/")
	stem := strings.TrimSuffix(strings.ToLower(path.Base(value)), ".md")
	name := strings.ToLower(path.Base(value))
	if name == "agents.md" || name == "claude.md" || name == "gemini.md" || name == "copilot-instructions.md" ||
		(len(parts) >= 2 && parts[0] == ".github" && parts[1] == "instructions" && strings.HasSuffix(name, ".instructions.md")) {
		return "instructions"
	}
	for _, part := range parts {
		if part == "adr" || part == "adrs" || part == "decision" || part == "decisions" {
			return "decision"
		}
	}
	if decisionStem.MatchString(stem) {
		return "decision"
	}
	for _, part := range parts {
		if part == "spec" || part == "specs" || part == "requirement" || part == "requirements" {
			return "spec"
		}
	}
	if strings.Contains(stem, "spec") {
		return "spec"
	}
	return ""
}

// documentRecord takes the source's text rather than reading it, because its
// only whole-repository caller has already materialised it for the same source
// -- and Source.Text copies the body on every call, so reading it twice copied
// the whole document corpus twice per build.
func documentRecord(source Source, text string, sources map[string]Source) (Record, bool) {
	kind := documentKind(source.Path)
	if kind == "" {
		return Record{}, false
	}
	lines := strings.Split(text, "\n")
	title := strings.TrimSuffix(path.Base(source.Path), path.Ext(source.Path))
	titleLine := 1
	titleFound := false
	status := ""
	headings := make([]string, 0)
	summary := ""
	inFrontmatter := false
	for index, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if match := documentTitle.FindStringSubmatch(raw); match != nil && !titleFound {
			title = strings.TrimSpace(match[1])
			titleLine = index + 1
			titleFound = true
		}
		if match := documentHeading.FindStringSubmatch(raw); match != nil {
			headings = append(headings, match[1])
		}
		if status == "" {
			if match := documentStatus.FindStringSubmatch(raw); match != nil {
				status = strings.ToLower(strings.TrimSpace(match[1]))
			}
		}
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if summary == "" && !inFrontmatter && trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "```") && !strings.HasPrefix(trimmed, ">") {
			summary = truncateRunes(trimmed, 500)
		}
	}
	referencesSet := make(map[string]struct{})
	for _, candidate := range documentReference.FindAllString(text, -1) {
		if normalized := normalizedReference(source.Path, candidate, sources); normalized != "" && normalized != source.Path {
			referencesSet[normalized] = struct{}{}
		}
	}
	references := keys(referencesSet)
	if len(references) > 100 {
		references = references[:100]
	}
	headingText := strings.Join(headings, " ")
	headingText = truncateRunes(headingText, 4000)
	return Record{kind, source.Path, source.Path, source.BlobHash, titleLine, map[string]any{
		"title": title, "summary": summary, "status": status,
		"headings": headingText, "references": references,
	}}, true
}

// goSymbols extracts a Go source's top-level declaration symbols, and reports
// the reason the grammar refused the source when it did.
//
// The extractor is the go/parser AST because a line scanner structurally cannot
// do this job: it carries no lexical state, so it cannot separate a top-level
// `var` from a function-local one, cannot reach the members of a parenthesized
// `const (...)` or `type (...)` group, and cannot tell source from the interior
// of a raw string literal -- a `func` written between backticks reads to it
// exactly like a declaration. Measured over the 3,477 first-party Go files of
// the Beamfall workspace, the scanner emitted 53,859 symbols against 48,012
// real ones: 10,180 false positives, 18.9% of its own output, and 4,333 false
// negatives, 9.0% of the real set. Nothing could observe either number.
//
// A refusal is never a silent zero. The second return value carries it to the
// caller, which records it in Index.Unparsed, and goSymbolsScan runs as the
// lossy fallback so a source that does not compile still contributes the
// symbols a scanner can see rather than none at all.
func goSymbols(source Source, text string) ([]Symbol, string) {
	fileSet := token.NewFileSet()
	// go/parser reports malformed input as an error rather than a panic, so no
	// recovery wrapper is needed for totality. Object resolution is skipped: no
	// symbol below reads a resolved identifier, only its own name and position.
	file, err := parser.ParseFile(fileSet, "source.go", text, parser.SkipObjectResolution)
	if err != nil {
		return goSymbolsScan(source, text), truncateRunes(err.Error(), 200)
	}
	result := make([]Symbol, 0, len(file.Decls))
	for _, declaration := range file.Decls {
		for _, declared := range goDeclaredNames(declaration) {
			line := fileSet.Position(declared.position).Line
			result = append(result, Symbol{Kind: declared.kind, Name: declared.name, Path: source.Path, BlobHash: source.BlobHash, Line: line})
		}
	}
	return result, ""
}

// goDeclaredName is one name a top-level declaration binds, with the position
// the index records for it.
type goDeclaredName struct {
	kind, name string
	position   token.Pos
}

// goDeclaredNames names what one top-level declaration declares.
//
// `const` is reported under kind "var". The pre-parser scanner matched `var`
// and `const` with a single pattern and labelled both "var", and the ranking
// tables key off that label, so the kind vocabulary stays exactly the three
// values it has always had. An ImportSpec declares no symbol: import paths are
// their own table.
func goDeclaredNames(declaration ast.Decl) []goDeclaredName {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		// Pos() is the `func` keyword, not the doc comment above it, which is
		// the line the scanner reported for the same declaration.
		return []goDeclaredName{{"func", typed.Name.Name, typed.Pos()}}
	case *ast.GenDecl:
		return goGroupNames(typed)
	}
	return nil
}

// goGroupNames flattens a declaration group. A parenthesized group and a
// single-spec declaration are the same node in the AST, so both reach here and
// every member of a `const (...)`, `var (...)` or `type (...)` block is named.
func goGroupNames(declaration *ast.GenDecl) []goDeclaredName {
	result := make([]goDeclaredName, 0, len(declaration.Specs))
	for _, spec := range declaration.Specs {
		result = append(result, goSpecNames(spec)...)
	}
	return result
}

func goSpecNames(spec ast.Spec) []goDeclaredName {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return []goDeclaredName{{"type", typed.Name.Name, typed.Pos()}}
	case *ast.ValueSpec:
		return goValueNames(typed)
	}
	return nil
}

// goValueNames names every identifier a value spec binds, so `var a, b = 1, 2`
// contributes both. The scanner stopped at the first name.
func goValueNames(spec *ast.ValueSpec) []goDeclaredName {
	result := make([]goDeclaredName, 0, len(spec.Names))
	for _, name := range spec.Names {
		result = append(result, goDeclaredName{"var", name.Name, name.Pos()})
	}
	return result
}

// goSymbolsScan is the pre-parser line scanner, retained only as the
// parse-failure fallback above and as the differential-test baseline. Its
// output is an approximation in both directions -- see goSymbols for the
// measured error rates -- and it is never reached for a source the grammar
// accepts.
func goSymbolsScan(source Source, text string) []Symbol {
	result := make([]Symbol, 0)
	for index, line := range strings.Split(text, "\n") {
		if kind, name, ok := goSymbol(line); ok {
			result = append(result, Symbol{Kind: kind, Name: name, Path: source.Path, BlobHash: source.BlobHash, Line: index + 1})
		}
	}
	return result
}

func goSymbol(line string) (string, string, bool) {
	for _, candidate := range []struct {
		kind   string
		prefix *regexp.Regexp
		suffix func(string) bool
	}{
		{"func", goFuncPrefix, func(value string) bool { return strings.HasPrefix(strings.TrimLeftFunc(value, unicode.IsSpace), "(") }},
		{"type", goTypePrefix, func(value string) bool { first, _ := utf8.DecodeRuneInString(value); return unicode.IsSpace(first) }},
		{"var", goValuePrefix, func(string) bool { return true }},
	} {
		location := candidate.prefix.FindStringIndex(line)
		if location == nil {
			continue
		}
		name, consumed := pythonWord(line[location[1]:])
		if name != "" && candidate.suffix(line[location[1]+consumed:]) {
			return candidate.kind, name, true
		}
	}
	return "", "", false
}

func pythonWord(value string) (string, int) {
	if value == "" || !((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z') || value[0] == '_') {
		return "", 0
	}
	consumed := 0
	for _, character := range value {
		if !isPythonWordRune(character) {
			break
		}
		consumed += utf8.RuneLen(character)
	}
	return value[:consumed], consumed
}

func isPythonWordRune(character rune) bool {
	return character == '_' || unicode.IsLetter(character) || unicode.IsNumber(character)
}

// decisionStem is compiled once. It was previously built by regexp.MustCompile
// on every documentKind call, once per source, which cost 10.5% of user-prompt
// allocated bytes and 12.7% of allocated objects.
var decisionStem = regexp.MustCompile(`^(?:adr-)?\d{3,5}-`)

type markerMatch struct {
	kind, id string
	column   int
}

func markerMatches(line string) []markerMatch {
	result := make([]markerMatch, 0)
	for _, match := range markerPattern.FindAllStringSubmatchIndex(line, -1) {
		if match[0] != 0 {
			previous, _ := utf8.DecodeLastRuneInString(line[:match[0]])
			if isPythonWordRune(previous) {
				continue
			}
		}
		if match[1] != len(line) {
			next, _ := utf8.DecodeRuneInString(line[match[1]:])
			if isPythonWordRune(next) {
				continue
			}
		}
		result = append(result, markerMatch{line[match[2]:match[3]], line[match[4]:match[5]], match[0]})
	}
	return result
}

func splitScalarList(value string) []string {
	result := make([]string, 0)
	start := 0
	var quote byte
	escaped := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 && character == '\\' {
			escaped = true
			continue
		}
		if character == '\'' || character == '"' {
			if quote == 0 {
				quote = character
			} else if quote == character {
				quote = 0
			}
			continue
		}
		if character == ',' && quote == 0 {
			result = append(result, value[start:index])
			start = index + 1
		}
	}
	return append(result, value[start:])
}

func parsePythonInteger(value string) (any, bool) {
	negative := false
	if strings.HasPrefix(value, "-") {
		negative = true
		value = value[1:]
	}
	if value == "" {
		return nil, false
	}
	var normalized strings.Builder
	if negative {
		normalized.WriteByte('-')
	}
	for _, character := range value {
		digit, ok := unicodeDecimalDigit(character)
		if !ok {
			return nil, false
		}
		normalized.WriteByte(byte('0' + digit))
	}
	integer, ok := new(big.Int).SetString(normalized.String(), 10)
	if !ok {
		return nil, false
	}
	if integer.IsInt64() {
		return integer.Int64(), true
	}
	return pythonInteger(integer.String()), true
}

func unicodeDecimalDigit(character rune) (int, bool) {
	if !unicode.IsDigit(character) {
		return 0, false
	}
	for value := 0; value <= 9; value++ {
		zero := character - rune(value)
		if unicode.IsDigit(zero) && (value == 9 || !unicode.IsDigit(zero-1)) {
			return value, true
		}
	}
	return 0, false
}

// goImports reports the import paths declared by a Go source. It is total: it
// is called on every .go blob during an index build, including blobs that do
// not compile, and must never panic.
//
// The edge set feeds internal/workqueue's direct-neighbour collision closure.
// A missed import can omit a real collision group, while a spurious import only
// over-constrains a wave proposal.
//
// So the parse-failure path UNIONS whatever the parser recovered with the
// legacy line scanner (goImportsScan) rather than returning the parser's
// partial result alone or an empty set. On a file the parser rejects, an empty
// map is indistinguishable from "this file imports nothing", which is exactly
// the false-negative collision the closure cannot survive; the union is >= both
// implementations and can only over-approximate. Signalling unresolvability
// INSTEAD of the union was the rejected alternative: every consumer would have
// to opt in to the refusal for it to be sound, and over-approximation is sound
// by default for consumers that never hear about it.
//
// So the union stays, and the refusal rides beside it rather than replacing it:
// the second return value carries the reason to the caller, which records it in
// Index.Unparsed. A consumer that never looks still gets the sound
// over-approximation; one that does can see which sources it rests on.
func goImports(text string) (map[string]struct{}, string) {
	// Exact gate, not a heuristic: the Go grammar admits no import declaration
	// without the keyword, so a source whose bytes lack the substring declares
	// no imports and never needs the parser.
	if !strings.Contains(text, "import") {
		return nil, ""
	}
	// ImportsOnly stops after the import declarations, and handles the forms the
	// line scanner cannot: block comments spanning an import block, `import(` with
	// no space, single-line grouped imports, blank and dot imports, and quoted
	// paths carrying escapes.
	// go/parser accepts arbitrary bytes and reports malformed input as an error
	// rather than a panic, so no recovery wrapper is needed for totality.
	file, err := parser.ParseFile(token.NewFileSet(), "source.go", text, parser.ImportsOnly)
	refusal := ""
	if err != nil {
		refusal = truncateRunes(err.Error(), 200)
	}
	degraded := err != nil
	result := make(map[string]struct{})
	if file != nil {
		for _, spec := range file.Imports {
			value, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				degraded = true
				if refusal == "" {
					refusal = truncateRunes("unquotable import path "+spec.Path.Value, 200)
				}
				continue
			}
			result[value] = struct{}{}
		}
	}
	if degraded {
		for value := range goImportsScan(text) {
			result[value] = struct{}{}
		}
	}
	return result, refusal
}

// goImportsScan is the pre-parser line scanner, retained only as the
// parse-failure fallback above and as the differential-test baseline. It
// over-approximates on quoted strings inside import blocks and under-reports
// whenever a line that is exactly ")" appears inside a block comment within an
// import block, or when the block opens as `import(`.
func goImportsScan(text string) map[string]struct{} {
	result := make(map[string]struct{})
	inBlock := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import (") {
			inBlock = true
			continue
		}
		if inBlock && trimmed == ")" {
			inBlock = false
			continue
		}
		if inBlock || strings.HasPrefix(trimmed, "import ") {
			if match := goImportQuoted.FindStringSubmatch(trimmed); match != nil {
				result[match[1]] = struct{}{}
			}
		}
	}
	return result
}

// importExtraction is one source's import edges together with how they were
// derived, so a consumer can tell an edge set the grammar produced from one a
// regular expression guessed at.
type importExtraction struct {
	imports map[string]struct{}
	// refusal is the extractor's rejection reason, empty when nothing was
	// refused. A non-empty refusal means the edges below are incomplete.
	refusal string
	// reason names the Unparsed class the refusal belongs to, so a caller
	// files a web lexer's unterminated construct and a Go grammar rejection
	// under different headings instead of one borrowed label.
	reason string
	// approximate marks edges no grammar produced. Two extractors set it, for
	// different residual risks:
	//
	//   - genericImport, the unstructured scanner still used for every
	//     language with neither a grammar nor a lexer. It has no comment,
	//     string or literal awareness at all, so it records import-shaped text
	//     found inside `//` comments and inside multi-line string literals as
	//     though each were a module name.
	//   - webImports, which does carry that lexical state and so invents none
	//     of those, but is still a lexer rather than a parser: it recognises
	//     no `require()` edge and resolves no specifier.
	//
	// Both remain approximate because neither is a grammar; the field does not
	// claim they are wrong in the same way.
	approximate bool
}

func sourceImports(sourcePath, text string) importExtraction {
	extension := strings.ToLower(path.Ext(sourcePath))
	switch extension {
	case ".go":
		imports, refusal := goImports(text)
		return importExtraction{imports: imports, refusal: refusal, reason: UnparsedGoGrammar}
	case ".py":
		// Unreachable from index building: compileSources routes a .py source
		// to pythonSourceFacts, which parses it through the closed grammar and
		// files its dotted modules, and compileMarkerTestImports admits only
		// .go marker paths. This arm exists so the scanner below never sees a
		// Python source, whose `from x import y` the unstructured regex would
		// mis-read; the grammar owns that language.
		return importExtraction{imports: map[string]struct{}{}}
	default:
		// The web languages get a lexer instead of the scanner, so a
		// commented-out or quoted specifier cannot become an edge. Every other
		// extension keeps genericImport, which mirrors the frozen oracle.
		if webSuffixes[extension] {
			imports, refusal := webImports(text)
			return importExtraction{imports: imports, refusal: refusal, reason: UnparsedWebLexical, approximate: len(imports) != 0}
		}
		result := make(map[string]struct{})
		// genericImport can only match where the body holds both a quote and
		// one of its two keywords -- every alternative ends in a quoted
		// string opened right after "from" or "import". Bodies that fail
		// this necessary condition (a 25 MB JSON blob, say) cannot match, so
		// skipping the NFA for them loses no match the regex would have
		// found.
		if strings.ContainsAny(text, `"'`) && (strings.Contains(text, "from") || strings.Contains(text, "import")) {
			for _, match := range genericImport.FindAllStringSubmatchIndex(text, -1) {
				if match[0] != 0 {
					previous, _ := utf8.DecodeLastRuneInString(text[:match[0]])
					if isPythonWordRune(previous) {
						continue
					}
				}
				result[text[match[2]:match[3]]] = struct{}{}
			}
		}
		return importExtraction{imports: result, approximate: len(result) != 0}
	}
}

func stringsField(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

// truncateRunes caps value at maximum runes. The rune count is taken without
// materialising the slice -- RuneCountInString counts an invalid byte as the one
// replacement rune the conversion would have produced, so the two agree -- and
// the slice is built only on the truncating branch, which is the branch that
// actually needs it. The query path calls this once per ranked symbol over a
// 2000-rune window that is usually already shorter than the cap.
func truncateRunes(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func sortedRecords(values map[string]Record) []Record {
	result := make([]Record, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}
