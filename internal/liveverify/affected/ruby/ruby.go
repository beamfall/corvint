// Package ruby is the Ruby implementation of the affected-selection language
// seam.
//
// A unit is one .rb file. Test units carry a framework-qualified identity so a
// caller can choose the file-level command the detected runner actually
// supports. The plugin reads source text only; it never invokes Ruby, Bundler,
// Rails, or a test framework.
package ruby

import (
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	// FrontierUnreadableSource reports source that could not be read or decoded.
	FrontierUnreadableSource = "ruby:unreadable-source"
	// FrontierDynamicLoad reports a require/load target not fixed in source.
	FrontierDynamicLoad = "ruby:dynamic-load"
	// FrontierConditionalLoad reports a literal load nested in a block.
	FrontierConditionalLoad = "ruby:conditional-load"
	// FrontierAmbiguousLoad reports a literal matching several repository files.
	FrontierAmbiguousLoad = "ruby:ambiguous-load"
	// FrontierRelativeLoad reports a require_relative target with no observed file.
	FrontierRelativeLoad = "ruby:relative-load-unresolved"
	// FrontierDynamicTestDefinition reports generated or reflection-driven tests.
	FrontierDynamicTestDefinition = "ruby:dynamic-test-definition"
	// FrontierRailsAutoload reports Rails constant loading not expressed by requires.
	FrontierRailsAutoload = "ruby:rails-autoload"
	// FrontierCucumberOwnership reports neutral Gherkin scenarios this plugin cannot own.
	FrontierCucumberOwnership = "ruby:cucumber-feature-ownership"
	// FrontierImplicitTestDependency reports a test with no resolvable repository edge.
	FrontierImplicitTestDependency = "ruby:implicit-test-dependency"
	// FrontierAmbiguousFramework reports a test whose runner is not fixed by its own source.
	FrontierAmbiguousFramework = "ruby:ambiguous-test-framework"
)

type framework string

const (
	frameworkSource    framework = "source"
	frameworkRSpec     framework = "rspec"
	frameworkMinitest  framework = "minitest"
	frameworkRailsTest framework = "rails-test"
	markerRSpec                  = "\x00corvint-rspec-require\x00"
	markerMinitest               = "\x00corvint-minitest-require\x00"
	markerRails                  = "\x00corvint-rails-require\x00"
)

var (
	literalLoadPattern = regexp.MustCompile(`^(require_relative|require|load)\s*(?:\(\s*)?["']([^"']+)["']\s*\)?`)
	embeddedLoad       = regexp.MustCompile(`\b(require_relative|require|load)\s*(?:\(\s*)?["']([^"']+)["']\s*\)?`)
	anyLoadCall        = regexp.MustCompile(`\b(?:require_relative|require|load)\s*(?:\(|\s)`)
	anyAutoload        = regexp.MustCompile(`\bautoload\s*(?:\(|\s)`)
	loadStartPattern   = regexp.MustCompile(`^(?:Kernel\.)?(?:require_relative|require|load)\b`)
	rspecDescribe      = regexp.MustCompile(`(?m)^\s*(?:RSpec\.)?(?:describe|context)\b`)
	rspecQualified     = regexp.MustCompile(`(?m)^\s*RSpec\.(?:describe|context)\b`)
	rspecSignal        = regexp.MustCompile(`(?m)^\s*RSpec\.(?:describe|context|configure|shared_examples|shared_context)\b`)
	rspecExample       = regexp.MustCompile(`(?m)^\s*(?:it|specify|example)\b`)
	minitestClass      = regexp.MustCompile(`(?m)<\s*(?:Minitest::Test|MiniTest::Test|Test::Unit::TestCase|ActiveSupport::TestCase|ActionDispatch::IntegrationTest|ApplicationSystemTestCase)\b`)
	minitestMethod     = regexp.MustCompile(`(?m)^\s*def\s+test_[[:alnum:]_!?]+`)
	declarativeTest    = regexp.MustCompile(`(?m)^[\t ]*test(?:[\t ]*\(|[\t ]+[^\s=])`)
	railsTestClass     = regexp.MustCompile(`(?m)<\s*(?:ActiveSupport|ActionDispatch|ActiveJob|ActionMailer)::[A-Za-z0-9_:]*Test(?:Case)?\b|<\s*ApplicationSystemTestCase\b`)
	dynamicDefinition  = regexp.MustCompile(`\b(?:define_method|define_singleton_method|class_eval|module_eval|const_get|const_set)\b|\b(?:eval|Class\.new)\s*\(|\b(?:public_send|send|__send__)\s*\(\s*:(?:test|it|specify|example)\b`)
	dynamicLoop        = regexp.MustCompile(`\.(?:each(?:_[a-z_]+)?|times|map|upto|downto)\s*(?:do|\{)|(?m)^\s*for\b`)
	dynamicTestCall    = regexp.MustCompile(`\b(?:it|specify|example|test)\s*(?:\(|["'{])`)
	heredocStart       = regexp.MustCompile(`<<[-~]?["']?([A-Za-z_][A-Za-z0-9_]*)["']?`)
	rakeLibDeclaration = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\.libs\b`)
	rakeLiteralLib     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\.libs\s*<<\s*["']([^"'#{}]+)["']\s*(?:#.*)?$`)
)

// Language observes Ruby source and test files under one repository root.
type Language struct{}

// New returns the Ruby language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "ruby" }

// Owns reports whether a path is Ruby source text. Neutral Gherkin and YAML
// assets deliberately remain unclaimed.
func (Language) Owns(relative string) bool { return strings.HasSuffix(relative, ".rb") }

type observation struct {
	relative string
	kind     framework
	id       string
	loads    []loadRef
}

type loadRef struct {
	target   string
	relative bool
	owner    string
}

// Units observes every Ruby file in the repository rooted at root.
func (language Language) Units(root string) (affected.Result, error) {
	files, err := affected.SourceFiles(root, func(name string) bool {
		return strings.HasSuffix(name, ".rb")
	})
	if err != nil {
		return affected.Result{}, err
	}
	features, err := affected.SourceFiles(root, func(name string) bool {
		return strings.HasSuffix(name, ".feature")
	})
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	if len(features) != 0 {
		frontier[FrontierCucumberOwnership] = true
	}
	loadRoots, err := rakeLoadRoots(root, frontier)
	if err != nil {
		return affected.Result{}, err
	}
	rails := hasRailsLayout(files)
	railsObserved := rails
	fileSet := make(map[string]bool, len(files))
	for _, relative := range files {
		fileSet[relative] = true
	}
	observed := make([]observation, 0, len(files))
	for _, relative := range files {
		body, readErr := affected.ReadSource(root, relative)
		if readErr != nil || !utf8.Valid(body) {
			frontier[FrontierUnreadableSource] = true
			continue
		}
		text := string(body)
		code := codeForDetection(text)
		loads, flags := scanLoads(relative, text)
		for flag := range flags {
			frontier[flag] = true
		}
		kind := classify(relative, code, rails)
		hint, hintConflict, helperRails := helperFramework(root, relative, loads, fileSet, loadRoots)
		railsObserved = railsObserved || helperRails
		if hintConflict {
			frontier[FrontierAmbiguousFramework] = true
		}
		if hint != frameworkSource && !hasDirectFramework(code) && !(kind == frameworkRSpec && hint == frameworkRailsTest) {
			kind = hint
		}
		railsObserved = railsObserved || kind == frameworkRailsTest
		if frameworkAmbiguous(relative, code, kind, rails) && hint == frameworkSource {
			frontier[FrontierAmbiguousFramework] = true
		}
		if hasDynamicTestDefinition(relative, code, kind) {
			frontier[FrontierDynamicTestDefinition] = true
		}
		observed = append(observed, observation{
			relative: relative,
			kind:     kind,
			id:       unitID(kind, relative),
			loads:    loads,
		})
	}
	if railsObserved && hasTests(observed) {
		frontier[FrontierRailsAutoload] = true
	}
	aliases := indexAliases(observed, loadRoots)
	paths := indexPaths(observed)
	units := make([]affected.Unit, 0, len(observed))
	for _, item := range observed {
		imports := resolveLoads(item, aliases, frontier)
		mapped := addConventionalCounterpart(item, paths, imports, frontier)
		if item.kind != frameworkSource && !rails && len(imports) == 0 && !mapped {
			frontier[FrontierImplicitTestDependency] = true
		}
		unit := affected.Unit{ID: item.id, Imports: sortedKeys(imports)}
		if item.kind == frameworkSource {
			unit.Sources = []string{item.relative}
		} else {
			unit.Tests = []string{item.relative}
		}
		units = append(units, unit)
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

func classify(relative, body string, rails bool) framework {
	base := path.Base(relative)
	if strings.HasSuffix(base, "_helper.rb") && !hasRunnableDSL(body) || base == "application_system_test_case.rb" {
		return frameworkSource
	}
	if rspecQualified.MatchString(body) && !explicitMinitest(body) {
		return frameworkRSpec
	}
	if strings.HasSuffix(base, "_spec.rb") {
		if explicitMinitest(body) {
			return frameworkMinitest
		}
		return frameworkRSpec
	}
	if railsTestClass.MatchString(body) {
		return frameworkRailsTest
	}
	if strings.HasSuffix(base, "_test.rb") {
		return minitestForFile(relative, body, rails)
	}
	if insideTestTree(relative, "test") && (strings.HasPrefix(base, "test_") || strings.HasPrefix(base, "test-") || strings.HasPrefix(base, "spec_")) {
		return minitestForFile(relative, body, rails)
	}
	if insideTestTree(relative, "spec") && rspecDescribe.MatchString(body) && rspecExample.MatchString(body) {
		return frameworkRSpec
	}
	if minitestClass.MatchString(body) || insideTestTree(relative, "test") && minitestMethod.MatchString(body) {
		if strings.Contains(body, "ActiveSupport::TestCase") || strings.Contains(body, "ActionDispatch::IntegrationTest") || strings.Contains(body, "ApplicationSystemTestCase") {
			return frameworkRailsTest
		}
		return frameworkMinitest
	}
	if explicitMinitest(body) && rspecDescribe.MatchString(body) && rspecExample.MatchString(body) {
		return frameworkMinitest
	}
	return frameworkSource
}

func explicitMinitest(body string) bool {
	return strings.Contains(body, markerMinitest) || minitestClass.MatchString(body)
}

func hasRunnableDSL(body string) bool {
	if rspecQualified.MatchString(body) && rspecExample.MatchString(body) {
		return true
	}
	// A bare test base class (`class WidgetTestCase < Minitest::Test`) defines no test by itself.
	if minitestMethod.MatchString(body) {
		return true
	}
	if minitestClass.MatchString(body) && declarativeTest.MatchString(body) {
		return true
	}
	return explicitMinitest(body) && rspecDescribe.MatchString(body) && rspecExample.MatchString(body)
}

func hasDirectFramework(body string) bool {
	return explicitMinitest(body) || rspecQualified.MatchString(body) || railsTestClass.MatchString(body)
}

func helperFramework(root, owner string, loads []loadRef, files map[string]bool, loadRoots []string) (framework, bool, bool) {
	runnerHints := make(map[framework]bool, 2)
	rails := false
	visited := make(map[string]bool)
	var visit func(string, []loadRef)
	visit = func(loadOwner string, references []loadRef) {
		for _, ref := range references {
			for _, candidate := range helperCandidates(loadOwner, ref, loadRoots) {
				if !files[candidate] {
					continue
				}
				if visited[candidate] {
					continue
				}
				visited[candidate] = true
				body, err := affected.ReadSource(root, candidate)
				if err != nil || !utf8.Valid(body) {
					continue
				}
				code := codeForDetection(string(body))
				for hint := range frameworkHints(code) {
					if hint == frameworkRailsTest {
						rails = true
						continue
					}
					runnerHints[hint] = true
				}
				nested, _ := scanLoads(candidate, string(body))
				visit(candidate, nested)
			}
		}
	}
	for _, ref := range loads {
		if strings.Contains(path.Base(ref.target), "helper") {
			visit(owner, []loadRef{ref})
		}
	}
	if len(runnerHints) > 1 {
		return frameworkSource, true, rails
	}
	if runnerHints[frameworkRSpec] {
		return frameworkRSpec, false, rails
	}
	if runnerHints[frameworkMinitest] {
		if rails {
			return frameworkRailsTest, false, true
		}
		return frameworkMinitest, false, false
	}
	if rails {
		return frameworkRailsTest, false, true
	}
	return frameworkSource, false, false
}

func helperCandidates(owner string, ref loadRef, loadRoots []string) []string {
	target := strings.TrimSuffix(ref.target, ".rb") + ".rb"
	if ref.relative {
		return []string{path.Clean(path.Join(path.Dir(owner), target))}
	}
	candidates := map[string]bool{
		target:                             true,
		path.Join("lib", target):           true,
		path.Join(path.Dir(owner), target): true,
		path.Join("spec", target):          true,
		path.Join("test", target):          true,
	}
	if prefix, _, ok := testRelative(owner); ok && prefix != "" {
		candidates[path.Join(prefix, "lib", target)] = true
		candidates[path.Join(prefix, "spec", target)] = true
		candidates[path.Join(prefix, "test", target)] = true
	}
	for _, root := range loadRoots {
		candidates[path.Join(root, target)] = true
	}
	return sortedKeys(candidates)
}

func frameworkHints(body string) map[framework]bool {
	hints := make(map[framework]bool, 3)
	if strings.Contains(body, markerRSpec) || rspecSignal.MatchString(body) {
		hints[frameworkRSpec] = true
	}
	if explicitMinitest(body) {
		hints[frameworkMinitest] = true
	}
	if strings.Contains(body, markerRails) || strings.Contains(body, "Rails.application") {
		hints[frameworkRailsTest] = true
	}
	return hints
}

func minitestForFile(relative, body string, rails bool) framework {
	if rails && strings.HasPrefix(relative, "test/") && !explicitMinitest(body) {
		return frameworkRailsTest
	}
	return frameworkMinitest
}

func frameworkAmbiguous(relative, body string, kind framework, rails bool) bool {
	base := path.Base(relative)
	if strings.HasSuffix(base, "_spec.rb") && !explicitMinitest(body) && !rspecQualified.MatchString(body) {
		return true
	}
	if kind == frameworkSource && rspecDescribe.MatchString(body) && rspecExample.MatchString(body) {
		return true
	}
	return rails && kind == frameworkMinitest && insideTestTree(relative, "test") && !strings.HasPrefix(relative, "test/")
}

func unitID(kind framework, relative string) string {
	return "ruby:" + string(kind) + ":" + strings.TrimSuffix(relative, ".rb")
}

func scanLoads(owner, body string) ([]loadRef, map[string]bool) {
	refs := make([]loadRef, 0, 8)
	flags := make(map[string]bool, 2)
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "$LOAD_PATH") || strings.Contains(line, "$:") || strings.Contains(line, "autoload_paths") {
			flags[FrontierDynamicLoad] = true
		}
		statements := strings.Split(line, ";")
		first := strings.TrimSpace(statements[0])
		conditionalLine := strings.HasPrefix(first, "if ") || strings.HasPrefix(first, "unless ")
		for statementIndex, statement := range statements {
			trimmed := strings.TrimSpace(statement)
			embedded := embeddedLoad.FindAllStringSubmatch(trimmed, -1)
			if len(embedded) == 0 {
				if loadStartPattern.MatchString(trimmed) || anyLoadCall.MatchString(trimmed) || anyAutoload.MatchString(trimmed) || strings.Contains(trimmed, ".require(") || strings.Contains(trimmed, "send(:require") {
					flags[FrontierDynamicLoad] = true
				}
				continue
			}
			for _, match := range embedded {
				if strings.Contains(match[2], "#{") {
					flags[FrontierDynamicLoad] = true
					continue
				}
				refs = append(refs, loadRef{
					target:   strings.TrimSuffix(path.Clean(match[2]), ".rb"),
					relative: match[1] == "require_relative",
					owner:    owner,
				})
			}
			firstMatch := embeddedLoad.FindStringIndex(trimmed)
			literal := literalLoadPattern.FindStringIndex(trimmed)
			suffix := ""
			if firstMatch != nil {
				suffix = strings.TrimSpace(trimmed[firstMatch[1]:])
			}
			if len(embedded) > 1 || literal == nil || firstMatch[0] != 0 || suffix != "" && !strings.HasPrefix(suffix, "#") || statementIndex > 0 && conditionalLine || len(line) != len(strings.TrimLeft(line, " \t")) {
				flags[FrontierConditionalLoad] = true
			}
		}
	}
	return refs, flags
}

func resolveLoads(item observation, aliases map[string][]string, frontier map[string]bool) map[string]bool {
	imports := make(map[string]bool, len(item.loads)+1)
	for _, ref := range item.loads {
		target := ref.target
		if ref.relative {
			target = path.Clean(path.Join(path.Dir(ref.owner), target))
			if target == ".." || strings.HasPrefix(target, "../") {
				frontier[FrontierRelativeLoad] = true
				continue
			}
		}
		matches := aliases[target]
		if len(matches) == 0 {
			if ref.relative {
				frontier[FrontierRelativeLoad] = true
			}
			continue
		}
		if len(matches) > 1 {
			frontier[FrontierAmbiguousLoad] = true
			continue
		}
		if matches[0] != item.id {
			imports[matches[0]] = true
		}
	}
	return imports
}

func indexAliases(observed []observation, loadRoots []string) map[string][]string {
	aliases := make(map[string][]string, len(observed)*2)
	for _, item := range observed {
		for _, alias := range aliasesFor(item, loadRoots) {
			aliases[alias] = append(aliases[alias], item.id)
		}
	}
	for alias := range aliases {
		sort.Strings(aliases[alias])
	}
	return aliases
}

func aliasesFor(item observation, loadRoots []string) []string {
	base := strings.TrimSuffix(item.relative, ".rb")
	aliases := map[string]bool{base: true}
	if strings.HasSuffix(base, "/init") {
		aliases[strings.TrimSuffix(base, "/init")] = true
	}
	components := strings.Split(base, "/")
	for index, component := range components {
		if component == "lib" || component == "app" {
			if index+1 < len(components) {
				aliases[strings.Join(components[index+1:], "/")] = true
			}
		}
	}
	for _, root := range []string{"spec/", "test/"} {
		if strings.HasPrefix(base, root) {
			aliases[strings.TrimPrefix(base, root)] = true
		}
	}
	for _, root := range loadRoots {
		prefix := root + "/"
		if strings.HasPrefix(base, prefix) {
			aliases[strings.TrimPrefix(base, prefix)] = true
		}
	}
	return sortedKeys(aliases)
}

func rakeLoadRoots(root string, frontier map[string]bool) ([]string, error) {
	files, err := affected.SourceFiles(root, func(name string) bool { return name == "Rakefile" })
	if err != nil {
		return nil, err
	}
	roots := make(map[string]bool)
	for _, relative := range files {
		body, readErr := affected.ReadSource(root, relative)
		if readErr != nil || !utf8.Valid(body) {
			frontier[FrontierUnreadableSource] = true
			continue
		}
		text := string(body)
		if !strings.Contains(text, "Rake::TestTask") {
			continue
		}
		for _, line := range strings.Split(text, "\n") {
			declaration := strings.TrimSpace(line)
			if !rakeLibDeclaration.MatchString(declaration) {
				continue
			}
			match := rakeLiteralLib.FindStringSubmatch(declaration)
			if len(match) != 2 {
				frontier[FrontierDynamicLoad] = true
				continue
			}
			if strings.Contains(match[1], `\`) {
				frontier[FrontierDynamicLoad] = true
				continue
			}
			loadRoot := path.Clean(strings.TrimSpace(match[1]))
			if loadRoot == "." {
				continue
			}
			if path.IsAbs(loadRoot) || loadRoot == ".." || strings.HasPrefix(loadRoot, "../") {
				frontier[FrontierDynamicLoad] = true
				continue
			}
			roots[loadRoot] = true
		}
	}
	return sortedKeys(roots), nil
}

func indexPaths(observed []observation) map[string]string {
	indexed := make(map[string]string, len(observed))
	for _, item := range observed {
		indexed[item.relative] = item.id
	}
	return indexed
}

func addConventionalCounterpart(item observation, paths map[string]string, imports map[string]bool, frontier map[string]bool) bool {
	if item.kind == frameworkSource {
		return false
	}
	prefix, relative, ok := testRelative(item.relative)
	if !ok {
		return false
	}
	stem := strings.TrimSuffix(relative, ".rb")
	stem = strings.TrimSuffix(stem, "_spec")
	stem = strings.TrimSuffix(stem, "_test")
	stem = strings.TrimPrefix(stem, "test_")
	candidates := make([]string, 0, 2)
	for _, sourceRoot := range []string{"app/", "lib/"} {
		candidate := prefix + sourceRoot + stem + ".rb"
		if _, exists := paths[candidate]; exists {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) > 1 {
		frontier[FrontierAmbiguousLoad] = true
		return false
	}
	if len(candidates) == 0 {
		return false
	}
	imports[paths[candidates[0]]] = true
	return true
}

func testRelative(relative string) (string, string, bool) {
	components := strings.Split(relative, "/")
	for index, component := range components {
		if component != "spec" && component != "test" {
			continue
		}
		prefix := strings.Join(components[:index], "/")
		if prefix != "" {
			prefix += "/"
		}
		return prefix, strings.Join(components[index+1:], "/"), true
	}
	return "", "", false
}

func hasDynamicTestDefinition(relative, body string, kind framework) bool {
	if dynamicDefinition.MatchString(body) {
		return true
	}
	if strings.Contains(body, "shared_examples") || strings.Contains(body, "shared_context") {
		return true
	}
	testContext := kind != frameworkSource || insideTestTree(relative, "spec") || insideTestTree(relative, "test")
	testContext = testContext || strings.Contains(body, "Minitest::Test") || strings.Contains(body, "Test::Unit::TestCase") || rspecQualified.MatchString(body)
	testContext = testContext || dynamicLoop.MatchString(body) && dynamicTestCall.MatchString(body)
	if !testContext && !strings.Contains(body, "test_") {
		return false
	}
	return dynamicLoop.MatchString(body) && (dynamicTestCall.MatchString(body) || minitestMethod.MatchString(body) || strings.Contains(body, "define_method"))
}

// codeWithoutFullLineComments removes Ruby's full-line comment and heredoc
// forms before framework detection. Dependency scanning remains conservative,
// but documentation examples must not become runnable test units.
func codeWithoutFullLineComments(body string) string {
	lines := make([]string, 0, strings.Count(body, "\n")+1)
	blockComment := false
	heredocEnd := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if heredocEnd != "" {
			if trimmed == heredocEnd {
				heredocEnd = ""
			}
			continue
		}
		if trimmed == "__END__" {
			break
		}
		if trimmed == "=begin" {
			blockComment = true
			continue
		}
		if blockComment {
			if trimmed == "=end" {
				blockComment = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		lines = append(lines, line)
		if marker := heredocMarker(line); marker != "" {
			heredocEnd = marker
		}
	}
	return strings.Join(lines, "\n")
}

func codeForDetection(body string) string {
	withoutBlocks := codeWithoutFullLineComments(body)
	masked := maskQuotedLiteralsAndInlineComments(withoutBlocks)
	loads := literalFrameworkLoads(withoutBlocks, masked)
	markers := make([]string, 0, len(loads))
	for _, ref := range loads {
		switch {
		case strings.HasPrefix(ref.target, "rspec/"):
			markers = append(markers, markerRSpec)
		case strings.HasPrefix(ref.target, "minitest/") || ref.target == "test/unit":
			markers = append(markers, markerMinitest)
		case ref.target == "active_support/test_help" || ref.target == "config/environment":
			markers = append(markers, markerRails)
		}
	}
	return masked + "\n" + strings.Join(markers, "\n")
}

func literalFrameworkLoads(body, masked string) []loadRef {
	loads := make([]loadRef, 0, 2)
	bodyLines := strings.Split(body, "\n")
	maskedLines := strings.Split(masked, "\n")
	for lineIndex, maskedLine := range maskedLines {
		statementStart := 0
		for statementEnd := 0; statementEnd <= len(maskedLine); statementEnd++ {
			if statementEnd != len(maskedLine) && maskedLine[statementEnd] != ';' {
				continue
			}
			maskedStatement := strings.TrimSpace(maskedLine[statementStart:statementEnd])
			statement := strings.TrimSpace(bodyLines[lineIndex][statementStart:statementEnd])
			statementStart = statementEnd + 1
			if !loadStartPattern.MatchString(maskedStatement) {
				continue
			}
			match := literalLoadPattern.FindStringSubmatch(statement)
			if len(match) == 0 || strings.Contains(match[2], "#{") {
				continue
			}
			loads = append(loads, loadRef{target: strings.TrimSuffix(path.Clean(match[2]), ".rb")})
		}
	}
	return loads
}

func maskQuotedLiteralsAndInlineComments(body string) string {
	masked := []byte(body)
	quote := byte(0)
	percentOpen := byte(0)
	percentClose := byte(0)
	percentDepth := 0
	escaped := false
	comment := false
	for index := 0; index < len(masked); index++ {
		current := masked[index]
		if current == '\n' {
			escaped = false
			comment = false
			continue
		}
		if comment {
			masked[index] = ' '
			continue
		}
		if percentClose != 0 {
			masked[index] = ' '
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if percentOpen != percentClose && current == percentOpen {
				percentDepth++
				continue
			}
			if current == percentClose {
				percentDepth--
				if percentDepth == 0 {
					percentOpen = 0
					percentClose = 0
				}
			}
			continue
		}
		if quote != 0 {
			masked[index] = ' '
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == quote {
				quote = 0
			}
			continue
		}
		if current == '#' {
			comment = true
			masked[index] = ' '
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			masked[index] = ' '
			continue
		}
		if open, close, delimiterIndex, ok := rubyPercentLiteral(masked, index); ok {
			for maskIndex := index; maskIndex <= delimiterIndex; maskIndex++ {
				masked[maskIndex] = ' '
			}
			percentOpen = open
			percentClose = close
			percentDepth = 1
			index = delimiterIndex
		}
	}
	return string(masked)
}

func rubyPercentLiteral(body []byte, start int) (byte, byte, int, bool) {
	if body[start] != '%' || start+1 >= len(body) {
		return 0, 0, 0, false
	}
	delimiterIndex := start + 1
	if strings.ContainsRune("qQwWiIrxs", rune(body[delimiterIndex])) {
		delimiterIndex++
	}
	if delimiterIndex >= len(body) {
		return 0, 0, 0, false
	}
	open := body[delimiterIndex]
	if open == '_' || open >= '0' && open <= '9' || open >= 'A' && open <= 'Z' || open >= 'a' && open <= 'z' || open == ' ' || open == '\t' || open == '\n' {
		return 0, 0, 0, false
	}
	close := open
	switch open {
	case '(':
		close = ')'
	case '[':
		close = ']'
	case '{':
		close = '}'
	case '<':
		close = '>'
	}
	return open, close, delimiterIndex, true
}

func heredocMarker(line string) string {
	match := heredocStart.FindStringSubmatch(line)
	if len(match) == 0 {
		return ""
	}
	return match[1]
}

func hasTests(observed []observation) bool {
	for _, item := range observed {
		if item.kind != frameworkSource {
			return true
		}
	}
	return false
}

func hasRailsLayout(paths []string) bool {
	for _, relative := range paths {
		if relative == "config/application.rb" || strings.HasSuffix(relative, "/config/application.rb") {
			return true
		}
	}
	return false
}

func insideTestTree(relative, directory string) bool {
	components := strings.Split(path.Dir(relative), "/")
	for index, component := range components {
		if component == directory {
			return index == 0 || components[index-1] != "lib" && components[index-1] != "app"
		}
	}
	return false
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
