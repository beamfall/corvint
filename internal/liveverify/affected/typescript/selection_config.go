// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// keyShapedConfigMarker matches the five markers R42 (panel pick) A6 adds to
// dynamicConfigMarkers (SLF-V1-005) only where they are an object key, quoted
// or not, so `rootDir:` or `__dir` is no marker. Group 1 is the key.
var keyShapedConfigMarker = regexp.MustCompile(`(?:^|[^A-Za-z0-9_$.])["']?(roots|testPathIgnorePatterns|moduleNameMapper|dir|root)["']?[\t ]*:`)

// keyShapedConfigMarkerAt returns the offset of the first key-shaped marker
// key in body, or -1.
func keyShapedConfigMarkerAt(body string) int {
	match := keyShapedConfigMarker.FindStringSubmatchIndex(body)
	if match == nil {
		return -1
	}
	return match[2]
}

// literalEvaluatedKeys are the marker keys SLF-V1-005 evaluates. A marker on
// any other key keeps the frontier. setupFilesAfterEnv is the Jest key the
// `setupFiles` marker also finds.
var literalEvaluatedKeys = map[string]bool{
	"include": true, "exclude": true, "testMatch": true, "testRegex": true, "testDir": true,
	"setupFiles": true, "setupFilesAfterEnv": true, "globalSetup": true, "roots": true,
	"testPathIgnorePatterns": true, "moduleNameMapper": true, "dir": true, "root": true,
	"projects": true,
}

// vitestIgnoredParents are the Vite and Vitest option objects whose include
// and exclude name no test file: a literal value there is read and ignored.
var vitestIgnoredParents = map[string]bool{
	"coverage": true, "benchmark": true, "optimizeDeps": true, "esbuild": true, "deps": true,
	"server": true, "build": true, "ssr": true, "css": true, "resolve": true, "worker": true,
}

// literalEvaluation is what a fully literal Vitest or Jest config tells the
// plugin (SLF-V1-005): the test files it adds below the config, and the
// owned modules every test of the config loads. Exclusions are read but
// remove nothing, so evaluation only widens what the plugin already models.
type literalEvaluation struct {
	directory string
	bases     []string
	globs     []*regexp.Regexp
	regexes   []*regexp.Regexp
	loads     []string
}

// member reports a file below the config that a literal membership pattern
// matches, relative to the config directory or to a literal root.
func (evaluation *literalEvaluation) member(relative string) bool {
	if evaluation == nil || !inside(relative, evaluation.directory) {
		return false
	}
	for _, expression := range evaluation.regexes {
		if expression.MatchString("/"+relative) || expression.MatchString("/"+strings.TrimPrefix(relative, evaluation.directory+"/")) {
			return true
		}
	}
	for _, base := range evaluation.bases {
		if !inside(relative, base) {
			continue
		}
		rest := relative
		if base != "." {
			rest = strings.TrimPrefix(relative, base+"/")
		}
		for _, expression := range evaluation.globs {
			if expression.MatchString(rest) {
				return true
			}
		}
	}
	return false
}

// configKey is one object key of a config's comment-stripped text: its name,
// the key of the object holding it ("" for an object no key holds), the
// offset of its name and its value's text.
type configKey struct {
	name   string
	parent string
	offset int
	value  string
}

// evaluateLiteralConfig evaluates a Vitest or Jest config (SLF-V1-005). It
// reports false, keeping the frontier, when the text does not tokenize, holds
// a spread or a relative import or require anywhere, or when any recognised
// marker is not an evaluated key with a literal value in a place the planner
// reads. files is every observed path and owns the plugin's ownership test.
func evaluateLiteralConfig(relative, runner, body string, configPaths []string, files map[string]bool, owns func(string) bool) (*literalEvaluation, bool) {
	if runner != runnerVitest && runner != runnerJest {
		return nil, false
	}
	clean, err := stripComments(body, false)
	if err != nil || strings.Contains(clean, "...") {
		return nil, false
	}
	scan := scanImportSites(relative, body)
	if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || len(scan.mocks) != 0 {
		return nil, false
	}
	for _, ref := range scan.refs {
		if strings.HasPrefix(ref, ".") || strings.HasPrefix(ref, "/") {
			return nil, false
		}
	}
	keys, parsed := configKeys(clean)
	if !parsed {
		return nil, false
	}
	byOffset := make(map[int]configKey, len(keys))
	for _, key := range keys {
		byOffset[key.offset] = key
	}
	directory := path.Dir(relative)
	evaluation := &literalEvaluation{directory: directory, bases: []string{directory}}
	for _, offset := range configMarkerOffsets(clean) {
		key, found := byOffset[offset]
		if !found || !literalEvaluatedKeys[key.name] {
			return nil, false
		}
		value, literal := parseLiteral(key.value)
		if !literal || !evaluation.read(key, value, runner, configPaths, files, owns) {
			return nil, false
		}
	}
	slices.Sort(evaluation.bases)
	evaluation.bases = slices.Compact(evaluation.bases)
	slices.Sort(evaluation.loads)
	evaluation.loads = slices.Compact(evaluation.loads)
	return evaluation, true
}

// configMarkerOffsets lists, sorted and unique, the offset of every recognised
// marker in clean: each dynamicConfigMarkers occurrence and each key-shaped
// marker key.
func configMarkerOffsets(clean string) []int {
	var offsets []int
	for _, marker := range dynamicConfigMarkers {
		for start := 0; ; {
			found := strings.Index(clean[start:], marker)
			if found < 0 {
				break
			}
			offsets = append(offsets, start+found)
			start += found + len(marker)
		}
	}
	for _, match := range keyShapedConfigMarker.FindAllStringSubmatchIndex(clean, -1) {
		offsets = append(offsets, match[2])
	}
	slices.Sort(offsets)
	return slices.Compact(offsets)
}

// read applies one literal marker value; false keeps the frontier.
func (evaluation *literalEvaluation) read(key configKey, value literalValue, runner string, configPaths []string, files map[string]bool, owns func(string) bool) bool {
	switch key.name {
	case "include", "exclude":
		switch {
		case key.parent == "test" || key.parent == "typecheck":
			if key.name == "include" {
				return evaluation.addGlobs(value)
			}
			return value.strings() != nil || value.kind == literalArray
		case vitestIgnoredParents[key.parent]:
			return true
		}
		return false
	case "testMatch":
		return evaluation.addGlobs(value)
	case "testRegex":
		return evaluation.addRegexes(value)
	case "testDir", "roots", "dir", "root":
		if key.name == "root" && key.parent != "" && key.parent != "test" || key.name == "dir" && key.parent != "test" {
			return false
		}
		entries := value.strings()
		if entries == nil {
			return false
		}
		for _, entry := range entries {
			base, inside := evaluation.path(entry)
			if !inside {
				return false
			}
			evaluation.bases = append(evaluation.bases, base)
		}
		return true
	case "setupFiles", "setupFilesAfterEnv", "globalSetup":
		entries := value.strings()
		if entries == nil {
			return false
		}
		for _, entry := range entries {
			if !evaluation.load(entry, files, owns) {
				return false
			}
		}
		return true
	case "moduleNameMapper":
		if value.kind != literalObject {
			return false
		}
		for _, entry := range value.entries {
			replacements := entry.value.strings()
			if replacements == nil {
				return false
			}
			for _, replacement := range replacements {
				if strings.Contains(replacement, "$") {
					if !aliasOnlyPattern(entry.key) {
						return false
					}
					continue
				}
				if !evaluation.load(replacement, files, owns) {
					return false
				}
			}
		}
		return true
	case "testPathIgnorePatterns":
		return value.strings() != nil
	case "projects":
		entries := value.strings()
		if entries == nil {
			return false
		}
		for _, entry := range entries {
			project, inside := evaluation.path(entry)
			if !inside {
				return false
			}
			if path.Ext(project) != "" && !strings.ContainsAny(project, "*?[{") && !slices.Contains(configPaths, project) {
				return false
			}
		}
		return true
	}
	return false
}

// path resolves a configured path against the config directory, `<rootDir>`
// included, and reports whether it stays inside the repository.
func (evaluation *literalEvaluation) path(entry string) (string, bool) {
	entry = strings.TrimPrefix(strings.ReplaceAll(entry, "<rootDir>", "."), "/")
	if entry == "" || path.IsAbs(entry) {
		return "", false
	}
	return repositoryJoin(evaluation.directory, entry)
}

// load records the owned module a setup or mapper path names. A path that
// names no owned file is a package when it is bare and refused otherwise.
func (evaluation *literalEvaluation) load(entry string, files map[string]bool, owns func(string) bool) bool {
	relativePath := strings.HasPrefix(entry, ".") || strings.HasPrefix(entry, "<rootDir>") || strings.HasPrefix(entry, "/")
	resolved, inside := evaluation.path(entry)
	if inside {
		for _, candidate := range moduleCandidates(resolved) {
			if files[candidate] {
				if owns(candidate) {
					evaluation.loads = append(evaluation.loads, candidate)
				}
				return true
			}
		}
	}
	return !relativePath
}

// addGlobs records membership globs, `<rootDir>/` read as the config directory.
func (evaluation *literalEvaluation) addGlobs(value literalValue) bool {
	entries := value.strings()
	if entries == nil {
		return false
	}
	for _, entry := range entries {
		entry = strings.TrimPrefix(strings.TrimPrefix(entry, "<rootDir>/"), "./")
		if strings.HasPrefix(entry, "!") {
			continue
		}
		expression, ok := globRegexp(entry)
		if !ok {
			return false
		}
		evaluation.globs = append(evaluation.globs, expression)
	}
	return true
}

// addRegexes records testRegex expressions, strings or regular expression
// literals, matched against a slash-prefixed path.
func (evaluation *literalEvaluation) addRegexes(value literalValue) bool {
	items := []literalValue{value}
	if value.kind == literalArray {
		items = value.items
	}
	for _, item := range items {
		switch item.kind {
		case literalString:
			expression, err := regexp.Compile(item.text)
			if err != nil {
				return false
			}
			evaluation.regexes = append(evaluation.regexes, expression)
		case literalRegex:
			evaluation.regexes = append(evaluation.regexes, item.regex)
		default:
			return false
		}
	}
	return true
}

// aliasOnlyPattern reports a moduleNameMapper key that no npm package name can
// match: after an optional `^` it starts with `@/`, `~`, `#` or `.`. Such a
// mapping is a path alias, whose imports the plugin already resolves through
// tsconfig paths or raises as an alias frontier, so its capturing replacement
// loads nothing the plan does not already model.
func aliasOnlyPattern(key string) bool {
	key = strings.TrimPrefix(key, "^")
	return strings.HasPrefix(key, "@/") || strings.HasPrefix(key, "~") || strings.HasPrefix(key, "#") || strings.HasPrefix(key, ".") || strings.HasPrefix(key, `\.`)
}

// The literal value kinds parseLiteral reads.
const (
	literalString = iota + 1
	literalRegex
	literalArray
	literalObject
	literalBool
)

// literalValue is one parsed literal: a string, a regular expression, a
// boolean, an array or an object of literals.
type literalValue struct {
	kind    int
	text    string
	regex   *regexp.Regexp
	items   []literalValue
	entries []literalEntry
}

type literalEntry struct {
	key   string
	value literalValue
}

// strings returns a string value or an array of strings, or nil.
func (value literalValue) strings() []string {
	switch value.kind {
	case literalString:
		return []string{value.text}
	case literalArray:
		values := make([]string, 0, len(value.items))
		for _, item := range value.items {
			if item.kind != literalString {
				return nil
			}
			values = append(values, item.text)
		}
		return values
	}
	return nil
}

// parseLiteral reads a value text that is a string, a regular expression, a
// boolean, or an array or object of those; anything else (an identifier, a
// call, a member read, a computed key, a template substitution) is not
// literal.
func parseLiteral(text string) (literalValue, bool) {
	text = strings.TrimSpace(text)
	if text == "true" || text == "false" {
		return literalValue{kind: literalBool, text: text}, true
	}
	if value, ok := stringLiteral(text); ok {
		return literalValue{kind: literalString, text: value}, true
	}
	if expression, ok := regexLiteral(text); ok {
		return literalValue{kind: literalRegex, regex: expression}, true
	}
	if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
		items, ok := callArguments("("+text[1:len(text)-1]+")", 0)
		if !ok {
			return literalValue{}, false
		}
		value := literalValue{kind: literalArray}
		for _, item := range items {
			if item == "" {
				continue
			}
			parsed, literal := parseLiteral(item)
			if !literal {
				return literalValue{}, false
			}
			value.items = append(value.items, parsed)
		}
		return value, true
	}
	if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
		items, ok := callArguments("("+text[1:len(text)-1]+")", 0)
		if !ok {
			return literalValue{}, false
		}
		value := literalValue{kind: literalObject}
		for _, item := range items {
			if item == "" {
				continue
			}
			key, rest, found := cutObjectKey(item)
			if !found {
				return literalValue{}, false
			}
			parsed, literal := parseLiteral(rest)
			if !literal {
				return literalValue{}, false
			}
			value.entries = append(value.entries, literalEntry{key: key, value: parsed})
		}
		return value, true
	}
	return literalValue{}, false
}

// cutObjectKey splits one object member into its literal key (an identifier or
// a quoted string) and its value text.
func cutObjectKey(member string) (string, string, bool) {
	if member == "" {
		return "", "", false
	}
	if member[0] == '\'' || member[0] == '"' {
		end := quotedEnd(member, 0)
		if end > len(member) {
			return "", "", false
		}
		key, ok := stringLiteral(member[:end])
		rest := strings.TrimSpace(member[end:])
		if !ok || !strings.HasPrefix(rest, ":") {
			return "", "", false
		}
		return key, rest[1:], true
	}
	end := 0
	for end < len(member) && isIdentifier(member[end]) {
		end++
	}
	rest := strings.TrimSpace(member[end:])
	if end == 0 || !strings.HasPrefix(rest, ":") {
		return "", "", false
	}
	return member[:end], rest[1:], true
}

// configFrame is one open bracket of configKeys' walk and the key whose value
// it opens, or "".
type configFrame struct {
	open byte
	key  string
}

// configKeys walks a comment-stripped config and lists every object key with
// the key of the object that holds it and its value text. Strings, template
// literals and regular expressions are skipped whole. It reports false when a
// bracket, string or template does not close.
func configKeys(clean string) ([]configKey, bool) {
	var keys []configKey
	var stack []configFrame
	pending := ""
	previous := byte('{')
	parent := func() string {
		for index := len(stack) - 1; index >= 0; index-- {
			if stack[index].open == '{' {
				return stack[index].key
			}
		}
		return ""
	}
	inObject := func() bool { return len(stack) != 0 && stack[len(stack)-1].open == '{' }
	keyAt := func(start, nameEnd, after int, name string) int {
		colon := skipSpace(clean, after)
		if !inObject() || (previous != '{' && previous != ',') || colon == len(clean) || clean[colon] != ':' {
			return -1
		}
		valueStart := colon + 1
		valueEnd, ok := configValueEnd(clean, valueStart)
		if !ok {
			return -2
		}
		keys = append(keys, configKey{name: name, parent: parent(), offset: start, value: clean[valueStart:valueEnd]})
		next := skipSpace(clean, valueStart)
		if next < len(clean) && (clean[next] == '{' || clean[next] == '[') {
			pending = name
		}
		previous = ':'
		return next
	}
	for index := 0; index < len(clean); {
		character := clean[index]
		switch {
		case character == ' ' || character == '\t' || character == '\n' || character == '\r':
			index++
			continue
		case character == '\'' || character == '"':
			end := quotedEnd(clean, index)
			if end > len(clean) {
				return nil, false
			}
			if value, ok := stringLiteral(clean[index:end]); ok {
				if next := keyAt(index+1, end, end, value); next == -2 {
					return nil, false
				} else if next >= 0 {
					index = next
					continue
				}
			}
			previous, index = '"', end
			continue
		case character == '`':
			end := templateEnd(clean, index)
			if end > len(clean) {
				return nil, false
			}
			previous, index = '`', end
			continue
		case character == '/':
			if end, regex := regexEnd(clean, index); regex {
				previous, index = '/', end
				continue
			}
		case isIdentifier(character) && (index == 0 || !isIdentifier(clean[index-1])):
			end := index
			for end < len(clean) && isIdentifier(clean[end]) {
				end++
			}
			if next := keyAt(index, end, end, clean[index:end]); next == -2 {
				return nil, false
			} else if next >= 0 {
				index = next
				continue
			}
			previous, index = 'a', end
			continue
		case character == '{' || character == '[' || character == '(':
			stack = append(stack, configFrame{open: character, key: pending})
			pending = ""
		case character == '}' || character == ']' || character == ')':
			if len(stack) == 0 {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		}
		previous = character
		index++
	}
	return keys, len(stack) == 0
}

// configValueEnd returns the offset where the value starting at start ends:
// the first `,` or closing bracket outside nested brackets, strings, templates
// and regular expressions.
func configValueEnd(clean string, start int) (int, bool) {
	depth := 0
	for index := start; index < len(clean); {
		switch character := clean[index]; {
		case character == '\'' || character == '"':
			index = quotedEnd(clean, index)
			if index > len(clean) {
				return 0, false
			}
			continue
		case character == '`':
			index = templateEnd(clean, index)
			if index > len(clean) {
				return 0, false
			}
			continue
		case character == '/':
			if end, regex := regexEnd(clean, index); regex {
				index = end
				continue
			}
		case character == '{' || character == '[' || character == '(':
			depth++
		case character == '}' || character == ']' || character == ')':
			if depth == 0 {
				return index, true
			}
			depth--
		case character == ',' && depth == 0:
			return index, true
		}
		index++
	}
	return len(clean), true
}

// literalMember reports a file an evaluated config's literal membership
// pattern makes a test (SLF-V1-005).
func literalMember(relative string, configs []config) bool {
	for _, config := range configs {
		if config.literal.member(relative) {
			return true
		}
	}
	return false
}

// literalLoads lists the modules an evaluated config at configPath loads for
// every one of its tests (SLF-V1-005).
func literalLoads(configPath string, configs []config) []string {
	for _, config := range configs {
		if config.path == configPath && config.literal != nil {
			return config.literal.loads
		}
	}
	return nil
}
