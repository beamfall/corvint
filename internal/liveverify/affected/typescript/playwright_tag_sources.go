package typescript

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// playwrightTagSources decides whether the tags a listing reports for a test file are exactly
// the tags of its grep titles, from the config, the test file and their repository-relative static
// import closure. Playwright reports a config or describe tag only as its `@\S+` runs, so the
// listing alone cannot show a tag that contains whitespace or is `@` alone; when every tag in the
// closure is a literal `@\S+` string the listing hides nothing. The literal tags are returned so the
// caller can also require each reported tag to be one of them (AFP-V0-409, AFP-V0-410). Every
// file of the proof must be in the source set the discovery receipt binds, so a change to any of
// them invalidates the receipt (AFP-V0-409).
type playwrightTagSources struct {
	root, configPath string
	bound            map[string]bool
	files            map[string]playwrightTagFile
	// names holds every bound file name (with no body) for alias resolution, and aliases the
	// parsed alias configs among them: each tsconfig.json, and each jsconfig.json with no
	// tsconfig.json beside it, which Playwright reads first (AFP-V0-418).
	names   map[string]string
	aliases []typeScriptAliases
}

// playwrightTagFile is one checked file. stop, when the file is not exact, says why: the first
// import it cannot follow, or that its text is not literal (AFP-V0-419).
type playwrightTagFile struct {
	exact   bool
	imports []string
	tags    []string
	stop    string
}

func newPlaywrightTagSources(root, configPath string) *playwrightTagSources {
	return &playwrightTagSources{root: root, configPath: configPath, files: map[string]playwrightTagFile{}}
}

// boundFiles returns the files ObservePlaywrightSources digests into the receipt's source binding,
// or nil when they cannot be listed.
func (sources *playwrightTagSources) boundFiles() map[string]bool {
	if sources.bound == nil {
		files, err := affected.SourceFiles(sources.root, playwrightObservedName)
		if err != nil {
			return nil
		}
		sources.bound = map[string]bool{sources.configPath: true}
		sources.names = map[string]string{sources.configPath: ""}
		for _, file := range files {
			sources.bound[file] = true
			sources.names[file] = ""
		}
		// Playwright reads the nearest folder holding tsconfig.json or jsconfig.json, and
		// tsconfig.json first within one folder (AFP-V0-418).
		bodies := map[string]string{}
		for _, file := range files {
			if base := path.Base(file); base != "tsconfig.json" && (base != "jsconfig.json" || sources.bound[path.Join(path.Dir(file), "tsconfig.json")]) {
				continue
			}
			if body, err := affected.ReadSource(sources.root, file); err == nil && utf8.Valid(body) {
				bodies[file] = string(body)
			} else {
				bodies[file] = "" // an unreadable config parses as invalid and blocks its aliases
			}
		}
		// A config naming `tsconfig` can point Playwright at another file than the nearest one, so
		// no alias is followed, as TJAA-V0-037 refuses for the dependency graph.
		config, err := affected.ReadSource(sources.root, sources.configPath)
		if err == nil && len(playwrightTokenOffsets(string(config), "tsconfig")) == 0 {
			for _, name := range sortedKeys(bodies) {
				sources.aliases = append(sources.aliases, parseTypeScriptAliases(name, bodies[name]))
			}
		}
	}
	return sources.bound
}

// exact returns the literal tags of every file reachable from entries when each is in the bound
// source set and passes playwrightTagFileExact, or nil when one does not.
func (sources *playwrightTagSources) exact(entries ...string) map[string]bool {
	tags, _ := sources.exactOrStop(entries...)
	return tags
}

// exactOrStop is exact that also returns, when the tags are nil, where the closure stopped
// (AFP-V0-419).
func (sources *playwrightTagSources) exactOrStop(entries ...string) (map[string]bool, string) {
	bound := sources.boundFiles()
	if bound == nil {
		return nil, "the bound source set cannot be listed"
	}
	tags := map[string]bool{}
	seen := map[string]bool{}
	queue := slices.Clone(entries)
	for len(queue) != 0 {
		file := queue[0]
		queue = queue[1:]
		if seen[file] {
			continue
		}
		seen[file] = true
		if !bound[file] {
			return nil, file + " is outside the bound sources"
		}
		checked, ok := sources.files[file]
		if !ok {
			checked = playwrightTagFileExact(sources.root, file, sources.resolveAlias)
			sources.files[file] = checked
		}
		if !checked.exact {
			return nil, checked.stop
		}
		for _, tag := range checked.tags {
			tags[tag] = true
		}
		queue = append(queue, checked.imports...)
	}
	return tags, ""
}

// resolveAlias resolves a bare specifier of owner through the nearest tsconfig.json or
// jsconfig.json paths or baseUrl to the one bound repository file it names, as the dependency
// graph does (TJAA-V0-037). A specifier the nearest config does not claim, a config that is not
// valid, an owner that is not .ts or .tsx under a config without allowJs (Playwright then maps
// nothing), a Playwright config that names `tsconfig`, and an ambiguous or missing target all
// leave it unresolved (AFP-V0-418).
func (sources *playwrightTagSources) resolveAlias(owner, ref string) (string, bool) {
	nearest := nearestTypeScriptAliases(owner, sources.aliases)
	typeScript := strings.HasSuffix(owner, ".ts") || strings.HasSuffix(owner, ".tsx")
	if nearest == nil || !nearest.valid || !typeScript && !nearest.allowJS || externalScheme(ref) {
		return "", false
	}
	target, _ := nearest.resolve(ref, sources.names)
	return target, target != "" && sources.bound[target]
}

// playwrightTagNeutralModules are bare modules whose import gives a case no tag.
var playwrightTagNeutralModules = map[string]bool{"@playwright/test": true, "dotenv": true, "dotenv/config": true, "path": true, "os": true, "url": true, "fs": true}

// playwrightTagIndirection are names through which a module can build, rename or enumerate a
// property at run time, so a tag could exist without the text `tag` in source: the reflective
// globals, the routes that reach them without naming them (`constructor`, `prototype`,
// `__proto__`, the global object aliases), and the enumerating and defining helpers. Any such name
// in code keeps the file inexact (AFP-V0-409). The in-place array mutators (`push`, `fill` and the
// like) are not among them: every literal tag array is the value of a literal `tag` key, so the
// only routes to it name `tag` outside a key, index it by a computed member, or pass one of these
// names, and each of those is already inexact (AFP-V0-425). The one exempt use is a plain
// `JSON.stringify` call (playwrightPlainStringify, TJAA-V0-090).
var playwrightTagIndirection = []string{
	"Object", "Reflect", "JSON", "Proxy", "eval", "Function", "globalThis", "global", "window", "self", "structuredClone",
	"constructor", "prototype", "__proto__", "__defineGetter__", "__defineSetter__",
	"values", "entries", "assign", "defineProperty", "defineProperties", "fromEntries", "getOwnPropertyNames", "getOwnPropertyDescriptor", "getOwnPropertyDescriptors", "setPrototypeOf",
}

// playwrightTagFileExact checks one file. Its static loads must all resolve: a relative import to
// one repository file, or a bare specifier that alias resolves to one through tsconfig paths or
// baseUrl (AFP-V0-418), is followed; any other bare module than @playwright/test, dotenv or a
// neutral Node built-in, an unresolved import, or a dynamic load is not exact, and stop names the
// first such specifier (AFP-V0-419). Every `tag` in its code must be an object-literal property key whose
// value is a literal string or an array of literal strings, each `@` followed by one or more
// non-whitespace characters without an escape. A string literal "tag", an escaped identifier or
// string property key, a computed object key, a computed member assignment or access or a name in
// playwrightTagIndirection is not exact (AFP-V0-409).
func playwrightTagFileExact(root, relative string, alias func(owner, ref string) (string, bool)) playwrightTagFile {
	body, err := affected.ReadSource(root, relative)
	if err != nil || !utf8.Valid(body) {
		return playwrightTagFile{stop: relative + " is not readable as bounded UTF-8 source"}
	}
	scan := scanImportSites(relative, string(body))
	if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || scan.syntax >= 0 {
		return playwrightTagFile{stop: relative + " has a module load that is not static"}
	}
	result := playwrightTagFile{exact: true}
	for _, ref := range scan.refs {
		if playwrightTagNeutralModules[ref] || strings.HasPrefix(ref, "node:") {
			continue
		}
		resolved, ok := playwrightResolveRelative(root, relative, ref)
		if !ok && !strings.HasPrefix(ref, ".") && alias != nil {
			resolved, ok = alias(relative, ref)
		}
		if !ok {
			return playwrightTagFile{stop: relative + " imports " + strconv.Quote(ref) + ", which resolves to no single repository file"}
		}
		result.imports = append(result.imports, resolved)
	}
	clean, err := stripComments(string(body), jsxSource(relative))
	if err != nil {
		return playwrightTagFile{stop: relative + " does not tokenize"}
	}
	tags, at, why := playwrightTagTextLiteral(clean)
	if why != "" {
		// The stop names the first construct and its line, so an owner can see which one keeps the
		// closure inexact (AFP-V0-425).
		return playwrightTagFile{stop: relative + " holds a tag or construct that is not one literal @\\S+ string (line " + strconv.Itoa(lineAt(clean, at)) + ": " + why + ")"}
	}
	result.tags = tags
	return result
}

// playwrightResolveRelative resolves a relative static import of owner to the one existing source
// file inside root among the candidates the import graph tries. When two candidates exist, such as
// tags.cjs and tags.js, the module Playwright loads depends on its resolver's order, so the import
// is not resolved (AFP-V0-409).
func playwrightResolveRelative(root, owner, ref string) (string, bool) {
	clean := strings.SplitN(strings.SplitN(ref, "?", 2)[0], "#", 2)[0]
	if !strings.HasPrefix(clean, ".") {
		return "", false
	}
	base := path.Clean(path.Join(path.Dir(owner), clean))
	if base == "." || base == ".." || strings.HasPrefix(base, "../") {
		return "", false
	}
	found := ""
	for _, candidate := range moduleCandidates(base) {
		if !affected.ValidRelativePath(candidate) || candidate == found {
			continue
		}
		_, err := affected.ReadSource(root, candidate)
		switch {
		case err == nil && found != "":
			return "", false
		case err == nil:
			found = candidate
		case !errors.Is(err, fs.ErrNotExist):
			return "", false
		}
	}
	return found, found != ""
}

// playwrightTagTextLiteral applies the per-file text rules of playwrightTagFileExact to a
// comment-stripped module and returns its literal tags. When the module is not literal, why names
// the first rule it breaks and at is that construct's offset (AFP-V0-425).
func playwrightTagTextLiteral(clean string) (tags []string, at int, why string) {
	lexed, ok := playwrightLex(clean)
	if !ok {
		return nil, 0, "the module does not lex"
	}
	first, name := -1, ""
	for _, candidate := range playwrightTagIndirection {
		for _, offset := range playwrightTokenOffsets(clean, candidate) {
			if lexed.code[offset] && (first < 0 || offset < first) && (candidate != "JSON" || !playwrightPlainStringify(clean, lexed, offset)) {
				first, name = offset, candidate
			}
		}
	}
	if first >= 0 {
		return nil, first, "`" + name + "` can build or enumerate a property at run time"
	}
	for _, quoted := range []string{"'tag'", `"tag"`, "`tag`"} {
		if offset := strings.Index(clean, quoted); offset >= 0 {
			return nil, offset, "the string " + quoted + " can name a tag key"
		}
	}
	for index := 0; index < len(clean); index++ {
		if !lexed.code[index] {
			if quote := clean[index]; (quote == '\'' || quote == '"') && (index == 0 || lexed.code[index-1]) {
				end, ok := playwrightQuotedEnd(clean, index)
				if !ok {
					return nil, index, "a string does not close"
				}
				if next := playwrightNextCode(clean, end); strings.ContainsRune(clean[index:end], '\\') && next < len(clean) && clean[next] == ':' {
					return nil, index, "an escaped string property key can spell tag"
				}
				index = end - 1
			}
			continue
		}
		switch clean[index] {
		case '\\':
			return nil, index, "an identifier escape can spell tag"
		case '[':
			previous := playwrightPreviousCode(clean, index)
			closeAt, closed := lexed.closes[index]
			if !closed {
				return nil, index, "a bracket does not close"
			}
			next := playwrightNextCode(clean, closeAt+1)
			if previous >= 0 && (clean[previous] == '{' || clean[previous] == ',') && next < len(clean) && (clean[next] == ':' || clean[next] == '(') {
				return nil, index, "a computed object key or method can name a tag key"
			}
			if playwrightMemberAccessAt(clean, previous) && !playwrightLiteralIndex(clean[index+1:closeAt]) {
				return nil, index, "a computed member access can reach a tag array without naming it"
			}
		case ']':
			// `T[] =` is a type annotation; any other `] =` writes a computed member.
			if previous := playwrightPreviousCode(clean, index); (previous < 0 || clean[previous] != '[') && playwrightAssignmentAt(clean, playwrightNextCode(clean, index+1)) {
				return nil, index, "a computed member write can set a tag"
			}
		}
	}
	tags = []string{}
	for _, offset := range playwrightTokenOffsets(clean, "tag") {
		if !lexed.code[offset] {
			continue
		}
		literal, ok := playwrightLiteralTagProperty(clean, lexed, offset)
		if !ok {
			return nil, offset, "`tag` is not an object-literal key whose value is literal @\\S+ strings"
		}
		tags = append(tags, literal...)
	}
	return tags, 0, ""
}

// playwrightPlainStringify reports whether the `JSON` token at offset begins a direct call
// `JSON.stringify(value)`, `JSON.stringify(value, null)` or `JSON.stringify(value, null, space)`.
// Its value is a string, and a string reaches a test only as a title, which the listing shows
// exactly, or as a tag value, which the tag rule already refuses; building a `tag` key from it
// needs JSON.parse, eval, a computed key or another name this rule keeps. With no replacer
// function the call runs only the toJSON methods and getters of its argument, user code under the
// same rules, which cannot reach a tag array without naming `tag`, a computed member or a
// reflective name. A replacer, a spread argument, `JSON?.`, a member `x.JSON`, `JSON.stringify`
// read without a direct call, and any other JSON member stay inexact (TJAA-V0-090).
func playwrightPlainStringify(clean string, lexed playwrightLexed, at int) bool {
	if previous := playwrightPreviousCode(clean, at); previous >= 0 && clean[previous] == '.' {
		return false
	}
	dot := playwrightNextCode(clean, at+len("JSON"))
	if dot >= len(clean) || clean[dot] != '.' {
		return false
	}
	member := playwrightNextCode(clean, dot+1)
	if !strings.HasPrefix(clean[member:], "stringify") || member+len("stringify") < len(clean) && playwrightIdentifierByte(clean[member+len("stringify")]) {
		return false
	}
	open := playwrightNextCode(clean, member+len("stringify"))
	if open >= len(clean) || clean[open] != '(' || !lexed.code[open] {
		return false
	}
	closeAt, closed := lexed.closes[open]
	if !closed {
		return false
	}
	arguments, start := []string{}, open+1
	for index := open + 1; index <= closeAt; index++ {
		if index == closeAt || lexed.code[index] && lexed.parent[index] == open && clean[index] == ',' {
			arguments = append(arguments, strings.TrimSpace(clean[start:index]))
			start = index + 1
		}
	}
	if len(arguments) == 0 || len(arguments) > 3 || len(arguments) >= 2 && arguments[1] != "null" {
		return false
	}
	for _, argument := range arguments {
		if argument == "" || strings.HasPrefix(argument, "...") {
			return false
		}
	}
	return true
}

// playwrightMemberAccessAt reports whether a `[` whose previous code byte is at previous opens a
// member access rather than an array literal: it follows an identifier other than a keyword that
// precedes an expression, a `)`, a `]` or the `.` of `?.`.
func playwrightMemberAccessAt(clean string, previous int) bool {
	if previous < 0 {
		return false
	}
	switch character := clean[previous]; {
	case character == ')' || character == ']' || character == '.':
		return true
	case !playwrightIdentifierByte(character):
		return false
	}
	start := previous
	for start > 0 && playwrightIdentifierByte(clean[start-1]) {
		start--
	}
	return !slices.Contains(playwrightExpressionKeywords, clean[start:previous+1])
}

// playwrightExpressionKeywords are the keywords after which `[` starts an array literal.
var playwrightExpressionKeywords = []string{"return", "of", "in", "typeof", "instanceof", "case", "yield", "await", "else", "do", "void", "delete", "throw", "new", "default", "extends"}

func playwrightIdentifierByte(character byte) bool {
	return character == '_' || character == '$' || character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= 0x80
}

// playwrightLiteralIndex reports whether a member access index is empty (a type such as
// `string[]`), a decimal integer or one escape-free string literal.
func playwrightLiteralIndex(raw string) bool {
	raw = strings.Trim(raw, " \t\r\n")
	if raw == "" || strings.Trim(raw, "0123456789") == "" {
		return true
	}
	_, literal := playwrightString(raw)
	return literal && !strings.ContainsRune(raw, '\\')
}

// playwrightNextCode returns the index of the first non-space byte at or after index.
func playwrightNextCode(raw string, index int) int {
	for ; index < len(raw) && strings.IndexByte(" \t\r\n", raw[index]) >= 0; index++ {
	}
	return index
}

// playwrightAssignmentAt reports whether an assignment operator (=, +=, ||=, ??=, &&= and the
// other compound forms) starts at index.
func playwrightAssignmentAt(raw string, index int) bool {
	rest := raw[min(index, len(raw)):]
	for _, operator := range []string{"**=", "<<=", ">>>=", ">>=", "||=", "&&=", "??=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^="} {
		if strings.HasPrefix(rest, operator) {
			return true
		}
	}
	return strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") && !strings.HasPrefix(rest, "=>")
}

// playwrightLiteralTagProperty reports whether the `tag` token at offset is a key of its enclosing
// object literal whose value is one or more literal `@\S+` strings, and returns them.
func playwrightLiteralTagProperty(clean string, lexed playwrightLexed, at int) ([]string, bool) {
	previous := playwrightPreviousCode(clean, at)
	next := playwrightNextCode(clean, at+len("tag"))
	if previous < 0 || clean[previous] != '{' && clean[previous] != ',' || next >= len(clean) || clean[next] != ':' {
		return nil, false
	}
	open := lexed.parent[at]
	if open < 0 || clean[open] != '{' {
		return nil, false
	}
	closeAt, closed := lexed.closes[open]
	if !closed {
		return nil, false
	}
	properties, ok := playwrightObjectProperties(clean[open : closeAt+1])
	value, found := properties["tag"]
	if !ok || !found {
		return nil, false
	}
	items, array := playwrightArrayItems(value)
	if !array {
		items = []string{value}
	}
	tags := make([]string, 0, len(items))
	for _, item := range items {
		tag, literal := playwrightString(item)
		if !literal || strings.ContainsRune(item, '\\') || len(tag) < 2 || tag[0] != '@' || strings.ContainsFunc(tag, playwrightJSSpace) {
			return nil, false
		}
		tags = append(tags, tag)
	}
	return tags, len(tags) != 0
}
