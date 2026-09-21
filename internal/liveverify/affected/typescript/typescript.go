// Package typescript is the JavaScript and TypeScript implementation of the
// affected-selection language seam.
//
// A unit is one source or test file because that is the smallest common unit
// Vitest, Jest, AVA, node:test, Playwright, Bun, Deno, Cypress, WebdriverIO,
// TestCafe, Nightwatch, and Detox can address. The legacy Storybook test-runner
// is the exception: it can address only one configured story set, so its story
// files are one aggregate unit. Imports are observed from source text; no
// runtime, package manager, or executable project configuration is started.
package typescript

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	FrontierUnreadableSource       = "typescript:unreadable-source"
	FrontierUnparsedSource         = "typescript:unparsed-source"
	FrontierDynamicImport          = "typescript:dynamic-import"
	FrontierUnresolvedImport       = "typescript:relative-import-unresolved"
	FrontierPathAlias              = "typescript:path-alias-unresolved"
	FrontierAmbiguousRunner        = "typescript:test-runner-ambiguous"
	FrontierUnknownRunner          = "typescript:test-runner-unknown"
	FrontierConfig                 = "typescript:executable-config-unresolved"
	FrontierRuntimeFlags           = "typescript:runtime-flags-unresolved"
	FrontierE2ERuntimeDependency   = "typescript:e2e-runtime-dependency"
	FrontierPuppeteerHost          = "typescript:puppeteer-host-runner-unresolved"
	FrontierDetoxConfiguration     = "typescript:detox-configuration-unresolved"
	FrontierStorybookAddressing    = "typescript:storybook-story-set-addressing"
	FrontierCrossLanguageTestAsset = "typescript:cross-language-test-asset-unowned"
)

const (
	runnerVitest          = "vitest"
	runnerJest            = "jest"
	runnerAVA             = "ava"
	runnerNode            = "node-test"
	runnerPlaywright      = "playwright"
	runnerBun             = "bun-test"
	runnerDeno            = "deno-test"
	runnerCypress         = "cypress"
	runnerWebdriverIO     = "webdriverio"
	runnerTestCafe        = "testcafe"
	runnerNightwatch      = "nightwatch"
	runnerDetox           = "detox"
	runnerStorybook       = "storybook-test-runner"
	runnerStorybookVitest = "storybook-vitest"
	runnerUnknown         = "unknown"
)

var sourceExtensions = []string{".cjs", ".js", ".jsx", ".mjs", ".ts", ".tsx"}

// Language observes JavaScript and TypeScript modules under one repository
// root.
type Language struct{}

// New returns the JavaScript and TypeScript language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "typescript" }

// Owns reports whether a path is JavaScript or TypeScript source text. Data,
// Gherkin, YAML, JSON, MDX, and host-independent E2E assets deliberately remain
// unowned because the seam has no shared-input ownership model.
func (Language) Owns(relative string) bool { return hasSourceExtension(relative) }

type observation struct {
	path      string
	body      string
	refs      []string
	runner    string
	configs   []string
	puppeteer bool
	test      bool
}

type config struct {
	path      string
	directory string
	runner    string
}

type packageScope struct {
	directory  string
	name       string
	runners    map[string]bool
	configured map[string]bool
	packages   map[string]bool
}

// Units observes every owned module and every supported test runner that can
// be identified without executing repository code.
func (language Language) Units(root string) (affected.Result, error) {
	return language.units(root, false)
}

func (language Language) units(root string, resolveAliases bool) (affected.Result, error) {
	files, err := affected.SourceFiles(root, observedName)
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	bodies := make(map[string]string, len(files))
	configs := make([]config, 0, 16)
	scopes := make([]packageScope, 0, 16)
	owned := make([]string, 0, len(files))
	for _, relative := range files {
		if unreadModuleSource(relative) {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		body, readErr := affected.ReadSource(root, relative)
		if readErr != nil || !utf8.Valid(body) {
			frontier[FrontierUnreadableSource] = true
			continue
		}
		text := string(body)
		bodies[relative] = text
		if ambiguousTestAsset(relative) {
			frontier[FrontierCrossLanguageTestAsset] = true
			continue
		}
		if path.Base(relative) == "package.json" {
			scope, parseErr := readPackageScope(relative, body)
			if parseErr != nil {
				frontier[FrontierConfig] = true
				continue
			}
			if scope.name != "" || len(scope.runners) != 0 || len(scope.configured) != 0 || len(scope.packages) != 0 {
				scopes = append(scopes, scope)
			}
			continue
		}
		if runner, known := configRunner(relative, text); known {
			configs = append(configs, config{path: relative, directory: path.Dir(relative), runner: runner})
			if executableConfigIsDynamic(text) {
				frontier[FrontierConfig] = true
			}
		}
		if manifestRunner := runtimeManifestRunner(relative); manifestRunner != "" {
			scopes = append(scopes, packageScope{directory: path.Dir(relative), runners: map[string]bool{manifestRunner: true}, configured: map[string]bool{manifestRunner: true}})
		}
		if hasPathAliases(relative, text) && !resolveAliases {
			frontier[FrontierPathAlias] = true
		}
		if language.Owns(relative) {
			owned = append(owned, relative)
		}
	}
	sort.Slice(configs, func(left, right int) bool { return configs[left].path < configs[right].path })
	sort.Slice(scopes, func(left, right int) bool { return scopes[left].directory < scopes[right].directory })
	var aliases []typeScriptAliases
	if resolveAliases {
		aliases = readTypeScriptAliases(bodies, frontier)
	}

	observations := make([]observation, 0, len(owned))
	for _, relative := range owned {
		body, readable := bodies[relative]
		if !readable {
			continue
		}
		refs, dynamic, parseErr := scanImports(relative, body)
		if parseErr != nil {
			frontier[FrontierUnparsedSource] = true
		}
		if dynamic {
			frontier[FrontierDynamicImport] = true
		}
		resolved := refs
		if resolveAliases {
			resolved = resolveTypeScriptAliases(relative, refs, aliases, bodies, frontier)
		}
		if hasUnresolvedBareImport(relative, resolved, scopes) {
			frontier[FrontierPathAlias] = true
		}
		observation := classify(relative, body, refs, configs, scopes, frontier)
		observation.refs = resolved
		observations = append(observations, observation)
	}
	return buildResult(observations, configs, frontier), nil
}

func buildResult(observations []observation, configs []config, frontier map[string]bool) affected.Result {
	units := make([]affected.Unit, 0, len(observations))
	pathToID := make(map[string]string, len(observations))
	refs := make(map[string]map[string]bool, len(observations))
	direct := make(map[string]map[string]bool, len(observations))
	storybookGroups := make(map[string]*affected.Unit)
	storybookRefs := make(map[string]map[string]bool)

	for _, observation := range observations {
		if observation.runner == runnerStorybook {
			groupID := storybookUnitID(observation.path)
			unit := storybookGroups[groupID]
			if unit == nil {
				unit = &affected.Unit{ID: groupID}
				storybookGroups[groupID] = unit
				storybookRefs[groupID] = make(map[string]bool)
			}
			unit.Tests = append(unit.Tests, observation.path)
			pathToID[observation.path] = groupID
			for _, ref := range observation.refs {
				storybookRefs[groupID][ref] = true
			}
			continue
		}
		id := unitID(observation.runner, observation.path)
		unit := affected.Unit{ID: id}
		if observation.test {
			unit.Tests = []string{observation.path}
		} else {
			unit.Sources = []string{observation.path}
		}
		units = append(units, unit)
		pathToID[observation.path] = id
		refs[id] = stringSet(observation.refs)
		direct[id] = make(map[string]bool)
		for _, configPath := range observation.configs {
			direct[id][configPath] = true
		}
	}
	for id, unit := range storybookGroups {
		sort.Strings(unit.Tests)
		units = append(units, *unit)
		refs[id] = storybookRefs[id]
		direct[id] = make(map[string]bool)
		for _, config := range configs {
			if config.runner == runnerStorybook && inside(unit.Tests[0], config.directory) {
				direct[id][config.path] = true
			}
		}
	}

	for index := range units {
		unit := &units[index]
		edges := make(map[string]bool)
		owner := firstPath(*unit)
		for ref := range refs[unit.ID] {
			target, local, unresolved := resolveImport(owner, ref, pathToID)
			if target != "" && target != unit.ID {
				edges[target] = true
			}
			if local && unresolved {
				frontier[FrontierUnresolvedImport] = true
			}
			if !local && unresolved && looksLikeAlias(ref) {
				frontier[FrontierPathAlias] = true
			}
		}
		for configPath := range direct[unit.ID] {
			if target := pathToID[configPath]; target != "" && target != unit.ID {
				edges[target] = true
			}
		}
		unit.Imports = sortedKeys(edges)
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}
}

func classify(relative, body string, refs []string, configs []config, scopes []packageScope, frontier map[string]bool) observation {
	observation := observation{path: relative}
	explicit := explicitRunners(body, refs)
	possible := possibleRunners(relative, configs, scopes)
	observation.puppeteer = importsPackage(refs, "puppeteer") || importsPackage(refs, "puppeteer-core")
	candidate := isTestCandidate(relative) || len(explicit) != 0 || observation.puppeteer || conventionCandidate(relative, conventionRunners(relative, possible, scopes))
	if !candidate || isConfigPath(relative, body) {
		return observation
	}
	observation.test = true
	if len(explicit) == 1 {
		observation.runner = sortedKeys(explicit)[0]
	} else if len(explicit) > 1 {
		observation.runner = runnerUnknown
		frontier[FrontierAmbiguousRunner] = true
	} else {
		if len(possible) == 1 {
			observation.runner = sortedKeys(possible)[0]
		} else if len(possible) > 1 {
			observation.runner = runnerUnknown
			frontier[FrontierAmbiguousRunner] = true
		} else {
			observation.runner = runnerUnknown
			frontier[FrontierUnknownRunner] = true
		}
	}
	observation.configs = nearestConfigs(relative, observation.runner, configs)
	if len(observation.configs) > 1 {
		frontier[FrontierAmbiguousRunner] = true
	}
	if observation.puppeteer && observation.runner == runnerUnknown {
		frontier[FrontierPuppeteerHost] = true
	}
	if requiresRuntimeFlags(relative, observation.runner) {
		frontier[FrontierRuntimeFlags] = true
	}
	if isE2ERunner(observation.runner) || observation.puppeteer {
		frontier[FrontierE2ERuntimeDependency] = true
	}
	if observation.runner == runnerDetox {
		frontier[FrontierDetoxConfiguration] = true
	}
	if observation.runner == runnerStorybook {
		frontier[FrontierStorybookAddressing] = true
	}
	return observation
}

func explicitRunners(body string, refs []string) map[string]bool {
	runners := make(map[string]bool)
	for _, ref := range refs {
		switch {
		case importsPackage([]string{ref}, "vitest"):
			runners[runnerVitest] = true
		case ref == "@jest/globals":
			runners[runnerJest] = true
		case importsPackage([]string{ref}, "ava"):
			runners[runnerAVA] = true
		case ref == "node:test" || strings.HasPrefix(ref, "node:test/"):
			runners[runnerNode] = true
		case ref == "@playwright/test":
			runners[runnerPlaywright] = true
		case ref == "bun:test":
			runners[runnerBun] = true
		case importsPackage([]string{ref}, "cypress"):
			runners[runnerCypress] = true
		case ref == "@wdio/globals":
			runners[runnerWebdriverIO] = true
		case importsPackage([]string{ref}, "testcafe"):
			runners[runnerTestCafe] = true
		case importsPackage([]string{ref}, "nightwatch"):
			runners[runnerNightwatch] = true
		case importsPackage([]string{ref}, "detox"):
			runners[runnerDetox] = true
		case ref == "@storybook/addon-vitest/vitest-plugin":
			runners[runnerStorybookVitest] = true
		}
	}
	if strings.Contains(body, "Deno.test") {
		runners[runnerDeno] = true
	}
	return runners
}

func possibleRunners(relative string, configs []config, scopes []packageScope) map[string]bool {
	runners := make(map[string]bool)
	for _, config := range configs {
		if inside(relative, config.directory) {
			runners[config.runner] = true
		}
	}
	if scope, known := nearestScope(relative, scopes); known {
		for runner := range scope.configured {
			runners[runner] = true
		}
		for _, runner := range []string{runnerStorybook, runnerStorybookVitest} {
			if scope.runners[runner] {
				runners[runner] = true
			}
		}
	}
	base := path.Base(relative)
	slashed := "/" + relative
	if strings.Contains(base, ".cy.") || strings.Contains(slashed, "/cypress/") {
		return keepRunner(runners, runnerCypress)
	}
	if strings.Contains(base, ".stories.") {
		if runners[runnerStorybookVitest] {
			return map[string]bool{runnerStorybookVitest: true}
		}
		return keepRunner(runners, runnerStorybook)
	}
	if strings.Contains(slashed, "/playwright/") {
		return keepRunner(runners, runnerPlaywright)
	}
	return runners
}

func nearestConfigs(relative, runner string, configs []config) []string {
	best := -1
	paths := make([]string, 0, 2)
	for _, config := range configs {
		if config.runner != runner || !inside(relative, config.directory) {
			continue
		}
		depth := len(strings.Split(config.directory, "/"))
		if depth < best {
			continue
		}
		if depth > best {
			best = depth
			paths = paths[:0]
		}
		paths = append(paths, config.path)
	}
	sort.Strings(paths)
	return paths
}

func scanImports(relative, body string) ([]string, bool, error) {
	withoutComments, parseErr := stripComments(body, jsxSource(relative))
	refs := make(map[string]bool)
	for _, pattern := range staticImportPatterns {
		for _, match := range pattern.FindAllStringSubmatch(withoutComments, -1) {
			refs[match[1]] = true
		}
	}
	requires, computedRequire, requiresParsed := scanRequires(withoutComments)
	if !requiresParsed {
		parseErr = strconv.ErrSyntax
	}
	for _, ref := range requires {
		refs[ref] = true
	}
	for _, match := range referencePathPattern.FindAllStringSubmatch(body, -1) {
		refs[relativeReference(match[1])] = true
	}
	dynamic := dynamicImportPattern.MatchString(withoutComments) || computedRequire
	return sortedKeys(refs), dynamic, parseErr
}

// dynamicImportPattern allows any whitespace before the call parenthesis, including the spaces a
// stripped block comment leaves behind (`import /* chunk */ ("./lazy")`).
var dynamicImportPattern = regexp.MustCompile(`import\s*\(`)

// referencePathPattern reads `/// <reference path="...">` from the unstripped body, because the
// directive is a line comment. Its path is always file-relative, even without a leading `./`.
var referencePathPattern = regexp.MustCompile(`(?m)^[\t ]*///[\t ]*<reference[\t ]+path[\t ]*=[\t ]*["']([^"'\r\n]+)["']`)

func relativeReference(reference string) string {
	if strings.HasPrefix(reference, ".") || strings.HasPrefix(reference, "/") {
		return reference
	}
	return "./" + reference
}

var staticImportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)(?:^|;)[\t ]*import(?:[\t ]+type)?[\t ]*["']([^"'\r\n]+)["']`),
	regexp.MustCompile(`(?ms)(?:^|;)[\t ]*(?:import|export)\b[^;]*?\bfrom[\t \r\n]*["']([^"'\r\n]+)["']`),
}

func scanRequires(body string) ([]string, bool, bool) {
	refs := make([]string, 0, 4)
	dynamic := false
	_, parsed := scanRequireCode(body, 0, false, &refs, &dynamic)
	return refs, dynamic, parsed
}

func scanRequireCode(body string, index int, stopAtBrace bool, refs *[]string, dynamic *bool) (int, bool) {
	depth := 0
	for index < len(body) {
		if body[index] == '\'' || body[index] == '"' {
			index = quotedEnd(body, index)
			continue
		}
		if body[index] == '`' {
			var closed bool
			index, closed = scanRequireTemplate(body, index+1, refs, dynamic)
			if !closed {
				return len(body), false
			}
			continue
		}
		if index+1 < len(body) && body[index] == '/' && (body[index+1] == '/' || body[index+1] == '*') {
			var closed bool
			index, closed = jsCommentEnd(body, index)
			if !closed {
				return len(body), false
			}
			continue
		}
		if end, regex := regexEnd(body, index); regex {
			index = end
			continue
		}
		if body[index] == '{' {
			depth++
			index++
			continue
		}
		if body[index] == '}' {
			if stopAtBrace && depth == 0 {
				return index + 1, true
			}
			if depth > 0 {
				depth--
			}
			index++
			continue
		}
		if !strings.HasPrefix(body[index:], "require") || index > 0 && (isIdentifier(body[index-1]) || body[index-1] == '.') {
			index++
			continue
		}
		after := index + len("require")
		if after < len(body) && isIdentifier(body[after]) {
			index = after
			continue
		}
		after = skipSpace(body, after)
		if after == len(body) || body[after] != '(' {
			index = after
			continue
		}
		after = skipSpace(body, after+1)
		if after == len(body) || body[after] != '\'' && body[after] != '"' {
			*dynamic = true
			index = after
			continue
		}
		end := quotedEnd(body, after)
		if end > len(body) {
			return len(body), false
		}
		*refs = append(*refs, body[after+1:end-1])
		index = end
	}
	return len(body), !stopAtBrace
}

func scanRequireTemplate(body string, index int, refs *[]string, dynamic *bool) (int, bool) {
	for index < len(body) {
		switch {
		case body[index] == '\\':
			index += 2
		case strings.HasPrefix(body[index:], "${"):
			var closed bool
			index, closed = scanRequireCode(body, index+2, true, refs, dynamic)
			if !closed {
				return len(body), false
			}
		case body[index] == '`':
			return index + 1, true
		default:
			index++
		}
	}
	return len(body), false
}

func jsCommentEnd(body string, index int) (int, bool) {
	line := body[index+1] == '/'
	for index += 2; index < len(body); index++ {
		if line && (body[index] == '\n' || body[index] == '\r') {
			return index, true
		}
		if !line && index+1 < len(body) && body[index] == '*' && body[index+1] == '/' {
			return index + 2, true
		}
	}
	return len(body), line
}

func stripComments(body string, rejectAmbiguousJSXQuotes bool) (string, error) {
	clean := []byte(body)
	for index := 0; index < len(clean); {
		if clean[index] == '\'' || clean[index] == '"' || clean[index] == '`' {
			end := quotedEnd(string(clean), index)
			if end > len(clean) {
				return string(clean), strconv.ErrSyntax
			}
			ambiguousJSX := rejectAmbiguousJSXQuotes && clean[index] != '`'
			if ambiguousJSX && (!jsxQuoteStartsLiteral(clean, index) || jsxQuotedTokenCouldHideRequire(clean[index:end]) || strings.ContainsAny(string(clean[index:end]), "\r\n")) {
				return string(clean), strconv.ErrSyntax
			}
			index = end
			continue
		}
		if index+1 == len(clean) || clean[index] != '/' || clean[index+1] != '/' && clean[index+1] != '*' {
			if end, regex := regexEnd(string(clean), index); regex {
				index = end
				continue
			}
			index++
			continue
		}
		line := clean[index+1] == '/'
		clean[index], clean[index+1] = ' ', ' '
		index += 2
		for index < len(clean) {
			if line && (clean[index] == '\n' || clean[index] == '\r') {
				break
			}
			if !line && index+1 < len(clean) && clean[index] == '*' && clean[index+1] == '/' {
				clean[index], clean[index+1] = ' ', ' '
				index += 2
				break
			}
			if clean[index] != '\n' && clean[index] != '\r' {
				clean[index] = ' '
			}
			index++
		}
		if !line && index == len(clean) {
			return string(clean), strconv.ErrSyntax
		}
	}
	return string(clean), nil
}

func jsxSource(relative string) bool {
	extension := strings.ToLower(path.Ext(relative))
	return extension == ".jsx" || extension == ".tsx"
}

func jsxQuotedTokenCouldHideRequire(token []byte) bool {
	text := string(token)
	for offset := 0; offset < len(text); {
		found := strings.Index(text[offset:], "require")
		if found < 0 {
			return false
		}
		found += offset
		beforeIdentifier := found > 0 && isIdentifier(text[found-1])
		after := found + len("require")
		afterIdentifier := after < len(text) && isIdentifier(text[after])
		if !beforeIdentifier && !afterIdentifier {
			return true
		}
		offset = after
	}
	return false
}

func jsxQuoteStartsLiteral(body []byte, index int) bool {
	prefix := strings.TrimRight(string(body[:index]), " \t")
	if prefix == "" || strings.HasSuffix(prefix, "\n") || strings.HasSuffix(prefix, "\r") {
		return true
	}
	if strings.HasSuffix(prefix, "=>") || strings.ContainsRune("([={,:;!?&|+-*%^~", rune(prefix[len(prefix)-1])) {
		return true
	}
	wordStart := len(prefix)
	for wordStart > 0 && isIdentifier(prefix[wordStart-1]) {
		wordStart--
	}
	switch prefix[wordStart:] {
	case "as", "await", "case", "default", "delete", "do", "else", "export", "from", "import", "in", "instanceof", "new", "of", "return", "throw", "typeof", "void", "yield":
		return true
	default:
		return false
	}
}

// regexOpeners are the bytes after which an expression must begin, so a `/`
// there opens a regular expression and cannot be division; the start of text
// counts as one. It mirrors the documented jsresolve rule (FPK-V0-018). Every
// other predecessor keeps `/` as division: a word, a literal, `)`, `]`, and `}`
// need grammar state, `+` and `-` may end `++` or `--`, `!` may be a non-null
// assertion, and `<` and `>` may be JSX or type arguments.
const regexOpeners = "(,=:[&|?{;~^%*"

// regexEnd returns the offset past a regular-expression literal and its flags
// when the `/` at index follows a regexOpeners byte (ignoring whitespace) and
// closes before any line terminator. Otherwise it reports false and the `/`
// stays division. Callers check for a comment start first.
func regexEnd(body string, index int) (int, bool) {
	if body[index] != '/' {
		return 0, false
	}
	previous := strings.TrimRight(body[:index], " \t\r\n")
	if previous != "" && !strings.Contains(regexOpeners, previous[len(previous)-1:]) {
		return 0, false
	}
	escaped, inClass := false, false
	for position := index + 1; position < len(body); {
		character, width := utf8.DecodeRuneInString(body[position:])
		position += width
		switch {
		case character == '\n' || character == '\r' || character == '\u2028' || character == '\u2029':
			return 0, false
		case escaped:
			escaped = false
		case character == '\\':
			escaped = true
		case inClass:
			inClass = character != ']'
		case character == '[':
			inClass = true
		case character == '/':
			for position < len(body) && isIdentifier(body[position]) {
				position++
			}
			return position, true
		}
	}
	return 0, false
}

func quotedEnd(body string, start int) int {
	delimiter := body[start]
	for index := start + 1; index < len(body); index++ {
		if body[index] == '\\' {
			index++
			continue
		}
		if body[index] == delimiter {
			return index + 1
		}
	}
	return len(body) + 1
}

func skipSpace(body string, index int) int {
	for index < len(body) && (body[index] == ' ' || body[index] == '\t' || body[index] == '\r' || body[index] == '\n') {
		index++
	}
	return index
}

func isIdentifier(value byte) bool {
	return value == '_' || value == '$' || value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func resolveImport(owner, reference string, paths map[string]string) (string, bool, bool) {
	clean := strings.SplitN(strings.SplitN(reference, "?", 2)[0], "#", 2)[0]
	if !strings.HasPrefix(clean, ".") {
		return "", false, looksLikeAlias(clean)
	}
	base := path.Clean(path.Join(path.Dir(owner), clean))
	if base == "." || base == ".." || strings.HasPrefix(base, "../") {
		return "", true, true
	}
	for _, candidate := range moduleCandidates(base) {
		if id := paths[candidate]; id != "" {
			return id, true, false
		}
	}
	return "", true, true
}

func moduleCandidates(base string) []string {
	candidates := []string{base}
	extension := path.Ext(base)
	if hasSourceExtension(base) {
		stem := strings.TrimSuffix(base, extension)
		for _, alternate := range sourceExtensions {
			candidates = append(candidates, stem+alternate)
		}
	} else {
		for _, extension := range sourceExtensions {
			candidates = append(candidates, base+extension)
		}
		for _, extension := range sourceExtensions {
			candidates = append(candidates, path.Join(base, "index"+extension))
		}
	}
	return candidates
}

func unitID(runner, relative string) string {
	if runner == "" {
		return "typescript:" + relative
	}
	return "typescript:" + runner + ":" + relative
}

func storybookUnitID(relative string) string {
	directory := "."
	parts := strings.Split(relative, "/")
	for index, part := range parts {
		if part == "src" || part == "stories" {
			if index > 0 {
				directory = strings.Join(parts[:index], "/")
			}
			break
		}
	}
	return "typescript:" + runnerStorybook + ":" + directory
}

func observedName(name string) bool {
	_, frameworkConfig := configRunner(name, "")
	return hasSourceExtension(name) || unreadModuleSource(name) || frameworkConfig || prefixedConfig(name, "vite.config") ||
		name == "package.json" || name == "deno.json" || name == "deno.jsonc" ||
		name == "bunfig.toml" || name == "tsconfig.json" || name == "nightwatch.json" ||
		name == ".detoxrc" || name == ".detoxrc.json" || name == ".testcaferc.json" ||
		strings.HasSuffix(name, ".feature") || strings.HasSuffix(name, ".yaml") ||
		strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".mdx")
}

func configRunner(relative, body string) (string, bool) {
	base := path.Base(relative)
	switch {
	case prefixedConfig(base, "vitest.config"):
		return runnerVitest, true
	case prefixedConfig(base, "vite.config") && (strings.Contains(body, "vitest/config") || strings.Contains(body, "test:")):
		return runnerVitest, true
	case prefixedConfig(base, "jest.config") || base == "jest.config.json":
		if strings.Contains(body, "detox/runners/jest") {
			return runnerDetox, true
		}
		return runnerJest, true
	case prefixedConfig(base, "playwright.config") || strings.HasPrefix(base, "playwright.") && strings.Contains(base, ".config."):
		return runnerPlaywright, true
	case prefixedConfig(base, "cypress.config"):
		return runnerCypress, true
	case strings.HasPrefix(base, "wdio") && strings.Contains(base, ".conf."):
		return runnerWebdriverIO, true
	case base == ".testcaferc.js" || base == ".testcaferc.cjs" || base == ".testcaferc.json":
		return runnerTestCafe, true
	case prefixedConfig(base, "nightwatch.conf") || prefixedConfig(base, "nightwatch.config") || base == "nightwatch.json":
		return runnerNightwatch, true
	case base == ".detoxrc" || strings.HasPrefix(base, ".detoxrc.") || prefixedConfig(base, "detox.config"):
		return runnerDetox, true
	case strings.HasPrefix(base, "test-runner.") && path.Base(path.Dir(relative)) == ".storybook":
		return runnerStorybook, true
	}
	return "", false
}

func runtimeManifestRunner(relative string) string {
	switch path.Base(relative) {
	case "deno.json", "deno.jsonc":
		return runnerDeno
	case "bunfig.toml":
		return runnerBun
	}
	return ""
}

func readPackageScope(relative string, body []byte) (packageScope, error) {
	var manifest struct {
		Name                 string                     `json:"name"`
		Dependencies         map[string]json.RawMessage `json:"dependencies"`
		DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
		PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
		OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
		Jest                 json.RawMessage            `json:"jest"`
		Detox                json.RawMessage            `json:"detox"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return packageScope{}, err
	}
	dependencies := make(map[string]bool)
	for _, group := range []map[string]json.RawMessage{manifest.Dependencies, manifest.DevDependencies, manifest.PeerDependencies, manifest.OptionalDependencies} {
		for dependency := range group {
			dependencies[dependency] = true
		}
	}
	runners := make(map[string]bool)
	configured := make(map[string]bool)
	dependencyRunners := map[string]string{
		"vitest": runnerVitest, "jest": runnerJest, "@jest/core": runnerJest, "ava": runnerAVA,
		"@playwright/test": runnerPlaywright, "cypress": runnerCypress,
		"@wdio/cli": runnerWebdriverIO, "webdriverio": runnerWebdriverIO,
		"testcafe": runnerTestCafe, "nightwatch": runnerNightwatch, "detox": runnerDetox,
		"@storybook/test-runner": runnerStorybook, "@storybook/addon-vitest": runnerStorybookVitest,
	}
	for dependency, runner := range dependencyRunners {
		if dependencies[dependency] {
			runners[runner] = true
		}
	}
	if len(manifest.Jest) != 0 && string(manifest.Jest) != "null" {
		configured[runnerJest] = true
	}
	if len(manifest.Detox) != 0 && string(manifest.Detox) != "null" {
		configured[runnerDetox] = true
	}
	return packageScope{
		directory:  path.Dir(relative),
		name:       manifest.Name,
		runners:    runners,
		configured: configured,
		packages:   dependencies,
	}, nil
}

func executableConfigIsDynamic(body string) bool {
	for _, marker := range []string{"testMatch", "testRegex", "testDir", "specPattern", "specs:", "src:", "projects:", "include:", "exclude:", "setupFiles", "globalSetup", "capabilities:", "browsers:"} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func hasPathAliases(relative, body string) bool {
	return path.Base(relative) == "tsconfig.json" && (strings.Contains(body, `"paths"`) || strings.Contains(body, `"baseUrl"`))
}

func ambiguousTestAsset(relative string) bool {
	base := path.Base(relative)
	if strings.HasSuffix(base, ".feature") || strings.Contains(base, ".stories.mdx") {
		return true
	}
	if !strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".yml") {
		return false
	}
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "maestro" || component == "e2e" || component == "features" {
			return true
		}
	}
	return false
}

func isTestCandidate(relative string) bool {
	components := strings.Split(relative, "/")
	if len(components) > 1 && (components[0] == "test" || components[0] == "tests") {
		return true
	}
	base := path.Base(relative)
	for _, marker := range []string{".test.", ".spec.", "_test.", "_spec.", ".cy.", ".stories."} {
		if strings.Contains(base, marker) {
			return true
		}
	}
	return false
}

// conventionRunners widens convention candidacy with the nearest manifest's dependency runners. A
// dependency alone still assigns no runner (TJAA-V0-003); it only keeps a conventional test
// directory from being silently classified as source.
func conventionRunners(relative string, possible map[string]bool, scopes []packageScope) map[string]bool {
	scope, known := nearestScope(relative, scopes)
	if !known {
		return possible
	}
	runners := make(map[string]bool, len(possible)+len(scope.runners))
	for runner := range possible {
		runners[runner] = true
	}
	for runner := range scope.runners {
		runners[runner] = true
	}
	return runners
}

func conventionCandidate(relative string, runners map[string]bool) bool {
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "__tests__" && runners[runnerJest] {
			return true
		}
		if component == "cypress" && runners[runnerCypress] || component == "playwright" && runners[runnerPlaywright] {
			return true
		}
		if component != "e2e" && component != "spec" && component != "specs" {
			continue
		}
		for _, runner := range []string{runnerPlaywright, runnerCypress, runnerWebdriverIO, runnerTestCafe, runnerNightwatch, runnerDetox} {
			if runners[runner] {
				return true
			}
		}
	}
	return false
}

func isConfigPath(relative, body string) bool {
	_, known := configRunner(relative, body)
	return known
}

func requiresRuntimeFlags(relative, runner string) bool {
	return runner == runnerNode && (strings.HasSuffix(relative, ".ts") || strings.HasSuffix(relative, ".tsx"))
}

func isE2ERunner(runner string) bool {
	switch runner {
	case runnerPlaywright, runnerCypress, runnerWebdriverIO, runnerTestCafe, runnerNightwatch, runnerDetox:
		return true
	}
	return false
}

// unreadModuleSource reports a `.mts` or `.cts` module. TJAA-V0-001 leaves it
// unowned, so its imports are never read; it may be a test of an owned module,
// and a missing edge must widen rather than exclude.
func unreadModuleSource(relative string) bool {
	extension := strings.ToLower(path.Ext(relative))
	return extension == ".mts" || extension == ".cts"
}

func hasSourceExtension(relative string) bool {
	extension := strings.ToLower(path.Ext(relative))
	for _, candidate := range sourceExtensions {
		if extension == candidate {
			return true
		}
	}
	return false
}

func importsPackage(refs []string, packageName string) bool {
	for _, ref := range refs {
		if ref == packageName || strings.HasPrefix(ref, packageName+"/") {
			return true
		}
	}
	return false
}

func looksLikeAlias(reference string) bool {
	return strings.HasPrefix(reference, "@/") || strings.HasPrefix(reference, "~/") || strings.HasPrefix(reference, "#")
}

func hasUnresolvedBareImport(relative string, refs []string, scopes []packageScope) bool {
	for _, ref := range refs {
		if strings.HasPrefix(ref, ".") || externalScheme(ref) {
			continue
		}
		packageName := importedPackage(ref)
		if packageName == "" {
			continue
		}
		if localPackage(packageName, scopes) {
			return true
		}
		if declaredPackage(relative, packageName, scopes) {
			continue
		}
		return true
	}
	return false
}

func importedPackage(reference string) string {
	parts := strings.Split(reference, "/")
	if len(parts) == 0 {
		return ""
	}
	if strings.HasPrefix(reference, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func declaredPackage(relative, packageName string, scopes []packageScope) bool {
	for _, scope := range scopes {
		if inside(relative, scope.directory) && scope.packages[packageName] {
			return true
		}
	}
	return false
}

func localPackage(packageName string, scopes []packageScope) bool {
	for _, scope := range scopes {
		if scope.name == packageName {
			return true
		}
	}
	return false
}

func externalScheme(reference string) bool {
	for _, prefix := range []string{"node:", "bun:", "deno:", "jsr:", "npm:", "http:", "https:", "data:"} {
		if strings.HasPrefix(reference, prefix) {
			return true
		}
	}
	return false
}

func prefixedConfig(base, prefix string) bool {
	return strings.HasPrefix(base, prefix+".")
}

func nearestScope(relative string, scopes []packageScope) (packageScope, bool) {
	best := -1
	var selected packageScope
	for _, scope := range scopes {
		if !inside(relative, scope.directory) {
			continue
		}
		depth := len(strings.Split(scope.directory, "/"))
		if depth > best {
			best = depth
			selected = scope
		}
	}
	return selected, best >= 0
}

func inside(relative, directory string) bool {
	return directory == "." || relative != directory && strings.HasPrefix(relative, directory+"/")
}

func keepRunner(runners map[string]bool, runner string) map[string]bool {
	if runners[runner] {
		return map[string]bool{runner: true}
	}
	return map[string]bool{}
}

func firstPath(unit affected.Unit) string {
	if len(unit.Sources) != 0 {
		return unit.Sources[0]
	}
	return unit.Tests[0]
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
