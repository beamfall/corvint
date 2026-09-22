// Package rust is the Rust implementation of the affected-selection language
// seam.
//
// A unit is one Cargo package, including one workspace member. Cargo can
// address that unit through its member Cargo.toml without relying on a package
// name being unique elsewhere in the repository. Package dependency edges are
// read from manifests and test anchors from Rust source text; Cargo, rustc, and
// test binaries are never executed.
package rust

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	FrontierConditionalCompilation = "rust:conditional-compilation"
	FrontierCucumberOwnership      = "rust:cucumber-feature-ownership-unresolved"
	FrontierCustomHarness          = "rust:custom-test-harness"
	FrontierGeneratedSource        = "rust:generated-source"
	FrontierGeneratedTests         = "rust:generated-test-discovery"
	FrontierManifest               = "rust:manifest-unresolved"
	FrontierNextestDoctest         = "rust:nextest-doctest-unsupported"
	FrontierSourceOutsidePackage   = "rust:source-outside-cargo-package"
	FrontierTargetConfiguration    = "rust:test-target-configuration"
	FrontierUnreadableSource       = "rust:unreadable-source"
	FrontierWebDriverRuntime       = "rust:webdriver-runtime"
)

var (
	testAttribute     = regexp.MustCompile(`(?m)#\s*\[\s*(?:[A-Za-z_][A-Za-z0-9_]*\s*::\s*)*test(?:\s*\([^]]*\))?\s*\]`)
	testLikeAttribute = regexp.MustCompile(`(?m)#\s*\[\s*(?:[A-Za-z_][A-Za-z0-9_]*\s*::\s*)*[A-Za-z_][A-Za-z0-9_]*test[A-Za-z0-9_]*(?:\s*\([^]]*\))?\s*\]`)
	generatedTest     = regexp.MustCompile(`(?m)(?:#\s*\[\s*(?:rstest|test_case|quickcheck|test_matrix|test_strategy(?:::\w+)?)(?:\s*\([^]]*\))?\s*\]|\b(?:[A-Za-z_][A-Za-z0-9_]*test[A-Za-z0-9_]*|quickcheck|parameterized)!\s*[\(\{\[])`)
	macroInvocation   = regexp.MustCompile(`\b((?:[A-Za-z_][A-Za-z0-9_]*::)*)([A-Za-z_][A-Za-z0-9_]*)!\s*[\(\{\[]`)
	macroDefinition   = regexp.MustCompile(`\bmacro_rules!\s*([A-Za-z_][A-Za-z0-9_]*)`)
	attributeName     = regexp.MustCompile(`#\s*\[\s*([A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)*)`)
	testAliasImport   = regexp.MustCompile(`(?m)\buse\s+(?:[A-Za-z_][A-Za-z0-9_]*::)+test\s+as\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	useStatement      = regexp.MustCompile(`(?s)\b(?:pub\s+)?use\s+([^;]+);`)
	// packageField matches only an inline-table package key, never the word
	// inside a path, version, or other string value.
	packageField = regexp.MustCompile(`(?:^|[{,])\s*(?:package|"package"|'package')\s*=\s*`)
	// workspaceField matches only an inline-table `workspace = true` key, never
	// the word inside a path or other string value.
	workspaceField = regexp.MustCompile(`(?:^|[{,])\s*(?:workspace|"workspace"|'workspace')\s*=\s*true\b`)
)

var nonGeneratingMacros = map[string]bool{
	"assert": true, "assert_eq": true, "assert_ne": true,
	"cfg": true, "column": true, "compile_error": true, "concat": true,
	"dbg": true, "debug_assert": true, "debug_assert_eq": true, "debug_assert_ne": true,
	"env": true, "eprint": true, "eprintln": true, "file": true,
	"format": true, "format_args": true, "line": true, "matches": true,
	"module_path": true, "option_env": true, "panic": true, "print": true,
	"println": true, "stringify": true, "thread_local": true, "todo": true,
	"try": true, "unimplemented": true, "unreachable": true, "vec": true,
	"write": true, "writeln": true,
}

var staticAttributes = map[string]bool{
	"allow": true, "cfg": true, "cfg_attr": true, "cold": true,
	"crate_name": true, "crate_type": true,
	"deprecated": true, "deny": true, "doc": true, "export_name": true,
	"feature": true, "forbid": true, "global_allocator": true, "ignore": true, "inline": true,
	"link": true, "link_name": true, "macro_export": true, "must_use": true,
	"naked": true, "no_implicit_prelude": true, "no_main": true, "no_mangle": true, "no_std": true,
	"non_exhaustive": true, "panic_handler": true,
	"path": true, "repr": true, "should_panic": true, "test": true,
	"recursion_limit": true, "type_length_limit": true, "used": true, "warn": true,
	"windows_subsystem": true,
}

var standardDerives = map[string]bool{
	"Clone": true, "Copy": true, "Debug": true, "Default": true,
	"Eq": true, "Hash": true, "Ord": true, "PartialEq": true, "PartialOrd": true,
}

// Language observes Cargo packages under one repository root.
type Language struct{}

// New returns the Rust language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "rust" }

// Owns reports whether a path is Rust source text. Cargo and Gherkin inputs are
// intentionally not claimed: both can be shared with other language plugins.
func (Language) Owns(relative string) bool { return strings.HasSuffix(relative, ".rs") }

// Units observes every Cargo package in the repository rooted at root.
func (Language) Units(root string) (affected.Result, error) {
	manifests, err := readManifests(root)
	if err != nil {
		return affected.Result{}, err
	}
	files, err := affected.SourceFiles(root, func(name string) bool {
		return strings.HasSuffix(name, ".rs")
	})
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	packages := packageManifests(manifests, frontier)
	assignments := assignSources(files, packages, frontier)
	units := make([]affected.Unit, 0, len(packages))
	dependencies := make(map[string][]dependency, len(packages))
	workspaceDependencies := collectWorkspaceDependencies(manifests, frontier)
	hasDoctests := false
	for index := range packages {
		manifest := &packages[index]
		unit := affected.Unit{ID: unitID(manifest.relative)}
		for _, relative := range assignments[manifest.relative] {
			body, readErr := affected.ReadSource(root, relative)
			if readErr != nil {
				frontier[FrontierUnreadableSource] = true
				unit.Tests = append(unit.Tests, relative)
				continue
			}
			isTest, isDoctest := classifySource(relative, manifest.directory, string(body), frontier)
			if isDoctest && !manifest.doctestDisabled {
				hasDoctests = true
			}
			if isTest || isDoctest && !manifest.doctestDisabled || manifest.testTargets[relative] {
				unit.Tests = append(unit.Tests, relative)
				continue
			}
			unit.Sources = append(unit.Sources, relative)
		}
		sort.Strings(unit.Sources)
		sort.Strings(unit.Tests)
		units = append(units, unit)
		resolvedDependencies := resolveWorkspaceDependencies(manifest.dependencies, workspaceDependencies)
		for _, dependency := range resolvedDependencies {
			manifest.noteDependency(dependency.name)
		}
		dependencies[unit.ID] = resolvedDependencies
		if manifest.customHarness {
			frontier[FrontierCustomHarness] = true
		}
		if manifest.targetConfig {
			frontier[FrontierTargetConfiguration] = true
		}
		if manifest.webdriver {
			frontier[FrontierWebDriverRuntime] = true
		}
		if manifest.cucumber {
			frontier[FrontierCucumberOwnership] = true
		}
	}
	features, err := affected.SourceFiles(root, func(name string) bool {
		return strings.HasSuffix(name, ".feature")
	})
	if err != nil {
		return affected.Result{}, err
	}
	if len(features) != 0 {
		frontier[FrontierCucumberOwnership] = true
	}
	if hasDoctests && nextestConfigured(root, manifests) {
		frontier[FrontierNextestDoctest] = true
	}
	resolve(units, packages, dependencies, frontier)
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

type dependency struct {
	name      string
	workspace bool
}

type manifest struct {
	relative          string
	directory         string
	packageName       string
	dependencies      []dependency
	workspaceDeps     map[string]string
	testTargets       map[string]bool
	doctestDisabled   bool
	customHarness     bool
	webdriver         bool
	cucumber          bool
	hasPackageSection bool
	unresolved        bool
	conditional       bool
	targetConfig      bool
}

func readManifests(root string) ([]manifest, error) {
	paths, err := affected.SourceFiles(root, func(name string) bool { return name == "Cargo.toml" })
	if err != nil {
		return nil, err
	}
	manifests := make([]manifest, 0, len(paths))
	for _, relative := range paths {
		body, readErr := affected.ReadSource(root, relative)
		if readErr != nil {
			manifests = append(manifests, manifest{relative: relative, directory: path.Dir(relative), unresolved: true})
			continue
		}
		parsed := parseManifest(relative, string(body))
		manifests = append(manifests, parsed)
	}
	return manifests, nil
}

func parseManifest(relative, body string) manifest {
	parsed := manifest{
		relative:      relative,
		directory:     path.Dir(relative),
		workspaceDeps: map[string]string{},
		testTargets:   map[string]bool{},
	}
	lines, complete := logicalLines(body)
	parsed.unresolved = !complete
	section := ""
	testPath := ""
	seen := map[string]bool{}
	seenHeaders := map[string]bool{}
	instances := map[string]int{}
	dottedDependencies := map[string]dependency{}
	sectionInstance := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(strings.TrimSpace(line), "[] ")
			// Cargo still reads the deprecated [project] table as [package].
			if section == "project" {
				section = "package"
			}
			sectionInstance = 0
			if strings.HasPrefix(line, "[[") {
				instances[section]++
				sectionInstance = instances[section]
			} else if seenHeaders[section] {
				parsed.unresolved = true
			} else {
				seenHeaders[section] = true
			}
			if section == "package" {
				parsed.hasPackageSection = true
			}
			if strings.HasPrefix(section, "target.") {
				parsed.conditional = true
			}
			if strings.Contains(section, "dependencies.") {
				parsed.unresolved = true
			}
			continue
		}
		key, value, ok := assignment(line)
		if !ok {
			parsed.unresolved = true
			continue
		}
		identity := section + "\x00" + strconv.Itoa(sectionInstance) + "\x00" + key
		if seen[identity] {
			parsed.unresolved = true
		}
		seen[identity] = true
		switch {
		case section == "package" && key == "name":
			parsed.packageName = quoted(value)
		case section == "package" && key == "autotests" && value == "false":
			parsed.targetConfig = true
		case section == "lib" && key == "doctest" && value == "false":
			parsed.doctestDisabled = true
		case isTargetSection(section) && key == "harness" && value == "false":
			parsed.customHarness = true
		case isTargetSection(section) && key == "test":
			parsed.targetConfig = true
		case isTargetSection(section) && key == "path":
			testPath = targetPath(parsed.directory, quoted(value))
			if testPath == "" {
				parsed.unresolved = true
			}
			if strings.HasPrefix(section, "test") && testPath != "" {
				parsed.testTargets[testPath] = true
			}
		case isTargetSection(section) && key == "required-features":
			parsed.conditional = true
		case dependencySection(section) && strings.Contains(key, "."):
			parsed.noteDottedDependency(dottedDependencies, section, key, value)
		case dependencySection(section):
			name := dependencyPackage(key, value)
			parsed.dependencies = append(parsed.dependencies, dependency{name: name, workspace: workspaceField.MatchString(value)})
			if strings.Contains(value, "optional") || strings.Contains(value, "features") {
				parsed.conditional = true
			}
			parsed.noteDependency(name)
		case section == "workspace.dependencies" && strings.Contains(key, "."):
			parsed.noteDottedWorkspaceDependency(key, value)
		case section == "workspace.dependencies":
			name := dependencyPackage(key, value)
			parsed.workspaceDeps[unquoteKey(key)] = name
			if strings.Contains(value, "optional") || strings.Contains(value, "features") {
				parsed.conditional = true
			}
			parsed.noteDependency(name)
		case section == "workspace" && strings.HasPrefix(key, "dependencies."):
			parsed.noteDottedWorkspaceDependency(strings.TrimPrefix(key, "dependencies."), value)
		case section == "features":
			parsed.conditional = true
		default:
			if section == "" && strings.Contains(key, ".") {
				parsed.unresolved = true
			}
		}
	}
	for _, key := range sortedKeys(dottedDependencies) {
		parsed.dependencies = append(parsed.dependencies, dottedDependencies[key])
	}
	return parsed
}

func (parsed *manifest) noteDottedDependency(dependencies map[string]dependency, section, key, value string) {
	alias, field, found := strings.Cut(key, ".")
	if !found {
		parsed.unresolved = true
		return
	}
	identity := section + "\x00" + alias
	entry, exists := dependencies[identity]
	if !exists {
		entry.name = unquoteKey(alias)
	}
	switch field {
	case "package":
		entry.name = quoted(value)
		if entry.name == "" {
			parsed.unresolved = true
		}
	case "workspace":
		entry.workspace = value == "true"
	case "features", "optional", "default-features":
		parsed.conditional = true
	case "path", "version", "registry", "git", "branch", "tag", "rev":
		// These fields do not change the first-party package name.
	default:
		parsed.unresolved = true
	}
	dependencies[identity] = entry
}

func (parsed *manifest) noteDottedWorkspaceDependency(key, value string) {
	alias, field, found := strings.Cut(key, ".")
	if !found {
		name := dependencyPackage(alias, value)
		parsed.workspaceDeps[unquoteKey(alias)] = name
		parsed.noteDependency(name)
		return
	}
	alias = unquoteKey(alias)
	if parsed.workspaceDeps[alias] == "" {
		parsed.workspaceDeps[alias] = alias
	}
	switch field {
	case "package":
		name := quoted(value)
		if name == "" {
			parsed.unresolved = true
			return
		}
		parsed.workspaceDeps[alias] = name
		parsed.noteDependency(name)
	case "features", "optional", "default-features":
		parsed.conditional = true
	case "path", "version", "registry", "git", "branch", "tag", "rev":
		// These fields do not change the first-party package name.
	default:
		parsed.unresolved = true
	}
}

func (parsed *manifest) noteDependency(name string) {
	switch name {
	case "thirtyfour", "fantoccini":
		parsed.webdriver = true
	case "cucumber", "cucumber-rs":
		parsed.cucumber = true
	}
}

func packageManifests(manifests []manifest, frontier map[string]bool) []manifest {
	packages := make([]manifest, 0, len(manifests))
	for _, parsed := range manifests {
		if parsed.unresolved {
			frontier[FrontierManifest] = true
		}
		if parsed.conditional {
			frontier[FrontierConditionalCompilation] = true
		}
		if parsed.packageName != "" {
			packages = append(packages, parsed)
			continue
		}
		if parsed.hasPackageSection {
			frontier[FrontierManifest] = true
		}
	}
	return packages
}

func assignSources(files []string, packages []manifest, frontier map[string]bool) map[string][]string {
	assigned := make(map[string][]string, len(packages))
	for _, relative := range files {
		owner := ""
		ownerLength := -1
		for _, parsed := range packages {
			if !inside(relative, parsed.directory) {
				continue
			}
			if len(parsed.directory) > ownerLength {
				owner = parsed.relative
				ownerLength = len(parsed.directory)
			}
		}
		if owner == "" {
			frontier[FrontierSourceOutsidePackage] = true
			continue
		}
		assigned[owner] = append(assigned[owner], relative)
	}
	return assigned
}

func classifySource(relative, packageDirectory, body string, frontier map[string]bool) (bool, bool) {
	code := withoutRustStringsAndComments(body)
	if strings.Contains(code, "include!(") || strings.Contains(code, "include_str!(") || strings.Contains(code, "include_bytes!(") || strings.Contains(code, "OUT_DIR") || path.Base(relative) == "build.rs" {
		frontier[FrontierGeneratedSource] = true
	}
	if strings.Contains(code, "#[path") || strings.Contains(code, "cfg_attr") || hasConditionalCompilation(code) {
		frontier[FrontierConditionalCompilation] = true
	}
	if strings.Contains(code, "#[doc") {
		frontier[FrontierGeneratedSource] = true
	}
	if strings.Contains(code, "```ignore-") {
		frontier[FrontierConditionalCompilation] = true
	}
	aliasedTest := hasAliasedTestAttribute(code)
	if generatedTest.MatchString(code) || testLikeAttribute.MatchString(code) || aliasedTest || strings.Contains(code, "macro_rules!") && testAttribute.MatchString(code) || hasUnknownMacro(code) || hasUnknownAttribute(code) {
		frontier[FrontierGeneratedTests] = true
	}
	isTest := integrationTest(relative, packageDirectory) || testAttribute.MatchString(code) || testLikeAttribute.MatchString(code) || generatedTest.MatchString(code) || aliasedTest
	if underCargoDirectory(relative, packageDirectory, "examples") || underCargoDirectory(relative, packageDirectory, "benches") {
		if isTest {
			frontier[FrontierTargetConfiguration] = true
		}
		isTest = false
	}
	return isTest, hasRunnableDoctest(body)
}

func hasUnknownMacro(code string) bool {
	for _, match := range macroInvocation.FindAllStringSubmatch(code, -1) {
		prefix := match[1]
		name := match[2]
		if prefix == "" && name == "macro_rules" {
			continue
		}
		if prefix == "" && nonGeneratingMacros[name] && !identifierImported(code, name) && !macroDefined(code, name) {
			continue
		}
		return true
	}
	return false
}

func hasUnknownAttribute(code string) bool {
	for _, match := range attributeName.FindAllStringSubmatchIndex(code, -1) {
		name := code[match[2]:match[3]]
		if name == "derive" && hasOnlyStandardDerives(code[match[0]:], code) {
			continue
		}
		if strings.HasSuffix(name, "::test") || staticAttributes[name] {
			continue
		}
		return true
	}
	return false
}

func hasOnlyStandardDerives(attribute, code string) bool {
	end := strings.Index(attribute, "]")
	if end < 0 {
		return false
	}
	attribute = attribute[:end]
	start := strings.Index(attribute, "(")
	close := strings.LastIndex(attribute, ")")
	if start < 0 || close <= start {
		return false
	}
	for _, name := range strings.Split(attribute[start+1:close], ",") {
		name = strings.TrimSpace(name)
		if !standardDerives[name] || identifierImported(code, name) {
			return false
		}
	}
	return true
}

func identifierImported(code, name string) bool {
	for _, match := range useStatement.FindAllStringSubmatch(code, -1) {
		if strings.Contains(match[1], "*") || containsIdentifier(match[1], name) {
			return true
		}
	}
	return false
}

func macroDefined(code, name string) bool {
	for _, match := range macroDefinition.FindAllStringSubmatch(code, -1) {
		if match[1] == name {
			return true
		}
	}
	return false
}

func containsIdentifier(text, name string) bool {
	for _, field := range strings.FieldsFunc(text, func(value rune) bool {
		return !(value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9')
	}) {
		if field == name {
			return true
		}
	}
	return false
}

func hasAliasedTestAttribute(code string) bool {
	attributes := map[string]bool{}
	for _, match := range attributeName.FindAllStringSubmatch(code, -1) {
		attributes[match[1]] = true
	}
	for _, match := range testAliasImport.FindAllStringSubmatch(code, -1) {
		if attributes[match[1]] {
			return true
		}
	}
	return false
}

func hasConditionalCompilation(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#[cfg(") {
			continue
		}
		if strings.ReplaceAll(trimmed, " ", "") == "#[cfg(test)]" {
			continue
		}
		return true
	}
	return false
}

func hasRunnableDoctest(body string) bool {
	for _, comment := range docComments(withoutRustStrings(body)) {
		if runnableDocComment(comment) {
			return true
		}
	}
	return false
}

// docComments returns each doc comment as its content lines with the doc
// marker removed and the remaining indentation kept. A run of consecutive
// `///` or `//!` lines is one comment.
func docComments(code string) [][]string {
	var comments [][]string
	var run []string
	runMarker := ""
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		marker := ""
		for _, candidate := range []string{"///", "//!"} {
			if strings.HasPrefix(trimmed, candidate) {
				marker = candidate
			}
		}
		if marker != runMarker && len(run) > 0 {
			comments = append(comments, run)
			run = nil
		}
		runMarker = marker
		if marker != "" {
			run = append(run, strings.TrimPrefix(trimmed, marker))
		}
	}
	if len(run) > 0 {
		comments = append(comments, run)
	}
	for _, marker := range []string{"/**", "/*!"} {
		rest := code
		for {
			start := strings.Index(rest, marker)
			if start < 0 {
				break
			}
			rest = rest[start+len(marker):]
			end := strings.Index(rest, "*/")
			if end < 0 {
				break
			}
			var lines []string
			for _, line := range strings.Split(rest[:end], "\n") {
				if starred, found := strings.CutPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "*"); found {
					line = starred
				}
				lines = append(lines, line)
			}
			comments = append(comments, lines)
			rest = rest[end+2:]
		}
	}
	return comments
}

// runnableDocComment reports a runnable fence or an indented code block.
// rustdoc tests an indented block like an unannotated fence. Markdown opens
// one only where no paragraph continues (at the start of the comment, after a
// blank line, heading, or closed fence) and never inside a fence. Indentation
// is measured before rustdoc removes the common indent, so the check can
// over-anchor but cannot miss a block.
func runnableDocComment(lines []string) bool {
	canOpenBlock := true
	fence := ""
	for _, line := range lines {
		comment := strings.TrimSpace(line)
		if fence != "" {
			marker, attributes, found := docFence(comment)
			if !found {
				continue
			}
			if attributes != "" {
				continue
			}
			if marker[0] != fence[0] {
				continue
			}
			if len(marker) < len(fence) {
				continue
			}
			fence = ""
			canOpenBlock = true
			continue
		}
		if comment == "" {
			canOpenBlock = true
			continue
		}
		if canOpenBlock && indentWidth(line) >= 4 {
			return true
		}
		if marker, _, found := docFence(comment); found {
			if runnableDocFence(comment) {
				return true
			}
			fence = marker
			canOpenBlock = false
			continue
		}
		canOpenBlock = strings.HasPrefix(comment, "#")
	}
	return false
}

// indentWidth counts leading whitespace columns with four-column tab stops.
func indentWidth(line string) int {
	width := 0
	for _, character := range line {
		switch character {
		case ' ':
			width++
		case '\t':
			width += 4 - width%4
		default:
			return width
		}
	}
	return width
}

// runnableDocFence accepts both Markdown fence forms rustdoc runs: backtick
// and tilde.
func runnableDocFence(comment string) bool {
	_, attributes, found := docFence(comment)
	if !found {
		return false
	}
	return attributes == "" || rustDocAttributes(attributes)
}

func docFence(comment string) (string, string, bool) {
	if len(comment) < 3 {
		return "", "", false
	}
	switch comment[0] {
	case '`', '~':
	default:
		return "", "", false
	}
	end := 1
	for end < len(comment) && comment[end] == comment[0] {
		end++
	}
	if end < 3 {
		return "", "", false
	}
	return comment[:end], strings.TrimSpace(comment[end:]), true
}

// withoutRustStrings preserves line structure while blanking quoted content.
// In particular, a raw string used by a code generator may contain text that
// looks exactly like a doc comment and must not become a phantom doctest.
func withoutRustStrings(body string) string {
	clean := []byte(body)
	for index := 0; index < len(clean); {
		if index+1 < len(clean) && clean[index] == '/' && clean[index+1] == '/' {
			index = lineCommentEnd(clean, index+2)
			continue
		}
		if index+1 < len(clean) && clean[index] == '/' && clean[index+1] == '*' {
			index = blockCommentEnd(clean, index+2)
			continue
		}
		if clean[index] == '\'' {
			if end := charLiteralEnd(clean, index); end != 0 {
				blankRange(clean, index, end)
				index = end
				continue
			}
		}
		hashes, rawEnd := rawStringStart(clean, index)
		if rawEnd != 0 {
			index = blankRawString(clean, index, rawEnd, hashes)
			continue
		}
		if clean[index] != '"' {
			index++
			continue
		}
		index = blankQuotedString(clean, index)
	}
	return string(clean)
}

func withoutRustStringsAndComments(body string) string {
	clean := []byte(withoutRustStrings(body))
	for index := 0; index < len(clean); {
		if index+1 < len(clean) && clean[index] == '/' && clean[index+1] == '/' {
			end := lineCommentEnd(clean, index+2)
			blankRange(clean, index, end)
			index = end
			continue
		}
		if index+1 < len(clean) && clean[index] == '/' && clean[index+1] == '*' {
			end := blockCommentEnd(clean, index+2)
			blankRange(clean, index, end)
			index = end
			continue
		}
		index++
	}
	return string(clean)
}

func lineCommentEnd(body []byte, start int) int {
	for index := start; index < len(body); index++ {
		if body[index] == '\n' {
			return index
		}
	}
	return len(body)
}

func blockCommentEnd(body []byte, start int) int {
	depth := 1
	for index := start; index+1 < len(body); index++ {
		if body[index] == '/' && body[index+1] == '*' {
			depth++
			index++
			continue
		}
		if body[index] == '*' && body[index+1] == '/' {
			depth--
			index++
			if depth == 0 {
				return index + 1
			}
		}
	}
	return len(body)
}

// charLiteralEnd returns the end of a char or byte literal opening at start, or
// 0 when the quote opens something else. A literal holds exactly one rune or
// one escape sequence, so a lifetime such as 'static is never a literal even
// when a later quote appears on the same line.
func charLiteralEnd(body []byte, start int) int {
	if start+1 < len(body) && body[start+1] != '\\' {
		value, size := utf8.DecodeRune(body[start+1:])
		closing := start + 1 + size
		if value == '\'' || value == '\n' || closing >= len(body) || body[closing] != '\'' {
			return 0
		}
		return closing + 1
	}
	escaped := false
	for index := start + 1; index < len(body); index++ {
		value := body[index]
		if value == '\n' {
			return 0
		}
		if value == '\'' && !escaped {
			return index + 1
		}
		if value == '\\' {
			escaped = !escaped
			continue
		}
		escaped = false
	}
	return 0
}

func rawStringStart(body []byte, index int) (int, int) {
	start := index
	if body[index] == 'b' {
		start++
		if start >= len(body) {
			return 0, 0
		}
	}
	if body[start] != 'r' {
		return 0, 0
	}
	start++
	hashes := 0
	for start < len(body) && body[start] == '#' {
		hashes++
		start++
	}
	if start >= len(body) || body[start] != '"' {
		return 0, 0
	}
	return hashes, start + 1
}

func blankRawString(body []byte, start, contentStart, hashes int) int {
	index := contentStart
	for index < len(body) {
		if body[index] == '"' && rawStringEnd(body, index+1, hashes) {
			end := index + hashes + 1
			blankRange(body, start, end)
			return end
		}
		index++
	}
	blankRange(body, start, len(body))
	return len(body)
}

func rawStringEnd(body []byte, start, hashes int) bool {
	if start+hashes > len(body) {
		return false
	}
	for offset := 0; offset < hashes; offset++ {
		if body[start+offset] != '#' {
			return false
		}
	}
	return true
}

func blankQuotedString(body []byte, start int) int {
	escaped := false
	for index := start + 1; index < len(body); index++ {
		value := body[index]
		if value == '"' && !escaped {
			blankRange(body, start, index+1)
			return index + 1
		}
		if value == '\\' {
			escaped = !escaped
			continue
		}
		escaped = false
	}
	blankRange(body, start, len(body))
	return len(body)
}

func blankRange(body []byte, start, end int) {
	for index := start; index < end; index++ {
		if body[index] != '\n' {
			body[index] = ' '
		}
	}
}

func rustDocAttributes(attributes string) bool {
	for _, attribute := range strings.FieldsFunc(attributes, func(value rune) bool { return value == ',' || value == ' ' }) {
		if attribute == "rust" || attribute == "ignore" || attribute == "should_panic" || attribute == "no_run" || attribute == "compile_fail" {
			continue
		}
		if len(attribute) == len("edition20xx") && strings.HasPrefix(attribute, "edition20") && attribute[9] >= '0' && attribute[9] <= '9' && attribute[10] >= '0' && attribute[10] <= '9' {
			continue
		}
		return false
	}
	return true
}

func collectWorkspaceDependencies(manifests []manifest, frontier map[string]bool) map[string]string {
	dependencies := map[string]string{}
	for _, parsed := range manifests {
		for alias, name := range parsed.workspaceDeps {
			if previous, exists := dependencies[alias]; exists && previous != name {
				frontier[FrontierManifest] = true
				continue
			}
			dependencies[alias] = name
		}
	}
	return dependencies
}

func resolveWorkspaceDependencies(values []dependency, workspace map[string]string) []dependency {
	resolved := make([]dependency, 0, len(values))
	for _, value := range values {
		if value.workspace {
			if name := workspace[value.name]; name != "" {
				value.name = name
			}
		}
		resolved = append(resolved, value)
	}
	return resolved
}

func resolve(units []affected.Unit, packages []manifest, dependencies map[string][]dependency, frontier map[string]bool) {
	byName := make(map[string][]string, len(packages))
	for _, parsed := range packages {
		byName[parsed.packageName] = append(byName[parsed.packageName], unitID(parsed.relative))
	}
	for index := range units {
		unit := &units[index]
		edges := map[string]bool{}
		for _, dependency := range dependencies[unit.ID] {
			targets := byName[dependency.name]
			if len(targets) > 1 {
				frontier[FrontierManifest] = true
			}
			for _, target := range targets {
				if target != unit.ID {
					edges[target] = true
				}
			}
		}
		unit.Imports = sortedKeys(edges)
	}
}

func nextestConfigured(root string, manifests []manifest) bool {
	candidates := map[string]bool{".config/nextest.toml": true, "nextest.toml": true}
	for _, parsed := range manifests {
		for _, suffix := range []string{".config/nextest.toml", "nextest.toml"} {
			candidates[path.Join(parsed.directory, suffix)] = true
		}
	}
	for relative := range candidates {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if _, err := os.Lstat(full); err == nil {
			return true
		}
	}
	return false
}

func dependencySection(section string) bool {
	// Cargo reads dev_dependencies and build_dependencies as aliases of the
	// hyphenated tables, including under target.
	if prefix, found := strings.CutSuffix(section, "_dependencies"); found {
		section = prefix + "-dependencies"
	}
	if section == "dependencies" || section == "dev-dependencies" || section == "build-dependencies" {
		return true
	}
	if !strings.HasPrefix(section, "target.") {
		return false
	}
	return strings.HasSuffix(section, ".dependencies") || strings.HasSuffix(section, ".dev-dependencies") || strings.HasSuffix(section, ".build-dependencies")
}

func isTargetSection(section string) bool {
	for _, target := range []string{"lib", "bin", "test", "example", "bench"} {
		if section == target || strings.HasPrefix(section, target+".") {
			return true
		}
	}
	return false
}

func assignment(line string) (string, string, bool) {
	cut := strings.Index(line, "=")
	if cut < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:cut])
	value := strings.TrimSpace(line[cut+1:])
	return key, value, key != ""
}

func dependencyPackage(key, value string) string {
	alias := unquoteKey(key)
	match := packageField.FindStringIndex(value)
	if match == nil {
		return alias
	}
	if name := quoted(value[match[1]:]); name != "" {
		return name
	}
	return alias
}

func quoted(value string) string {
	value = strings.TrimSpace(value)
	if cut := strings.IndexAny(value, ",}"); cut >= 0 {
		value = strings.TrimSpace(value[:cut])
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	parsed, err := strconv.Unquote(value)
	if err != nil {
		return ""
	}
	return parsed
}

func unquoteKey(key string) string {
	if value := quoted(key); value != "" {
		return value
	}
	return strings.TrimSpace(key)
}

func stripComment(line string) string {
	quoted := false
	escaped := false
	for index, value := range line {
		if value == '\\' && quoted {
			escaped = !escaped
			continue
		}
		if value == '"' && !escaped {
			quoted = !quoted
		}
		escaped = false
		if value == '#' && !quoted {
			return line[:index]
		}
	}
	return line
}

func logicalLines(body string) ([]string, bool) {
	lines := make([]string, 0, strings.Count(body, "\n")+1)
	current := ""
	depth := 0
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if current != "" {
			current += " "
		}
		current += line
		depth += delimiterBalance(line)
		if depth > 0 {
			continue
		}
		lines = append(lines, current)
		current = ""
		depth = 0
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines, depth == 0
}

func delimiterBalance(line string) int {
	depth := 0
	quoted := false
	escaped := false
	for _, value := range line {
		if value == '\\' && quoted {
			escaped = !escaped
			continue
		}
		if value == '"' && !escaped {
			quoted = !quoted
			continue
		}
		escaped = false
		if quoted {
			continue
		}
		switch value {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}
	return depth
}

func targetPath(directory, relative string) string {
	if relative == "" || strings.HasPrefix(relative, "/") {
		return ""
	}
	joined := path.Clean(path.Join(directory, relative))
	if joined == "." || joined == ".." || strings.HasPrefix(joined, "../") {
		return ""
	}
	return joined
}

func integrationTest(relative, directory string) bool {
	prefix := "tests/"
	if directory != "." {
		prefix = directory + "/tests/"
	}
	return strings.HasPrefix(relative, prefix)
}

func underCargoDirectory(relative, directory, name string) bool {
	prefix := name + "/"
	if directory != "." {
		prefix = directory + "/" + prefix
	}
	return strings.HasPrefix(relative, prefix)
}

func inside(relative, directory string) bool {
	return directory == "." || strings.HasPrefix(relative, directory+"/")
}

func unitID(manifestPath string) string { return "rust:" + manifestPath }

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
