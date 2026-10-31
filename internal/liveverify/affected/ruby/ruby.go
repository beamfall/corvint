// Package ruby is the Ruby implementation of the affected-selection language
// seam.
//
// A unit is one .rb file. Test units carry a framework-qualified identity so a
// caller can choose the file-level command the detected runner actually
// supports. The plugin reads source text only; it never invokes Ruby, Bundler,
// Rails, or a test framework.
package ruby

import (
	"context"
	"maps"
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
	// stockBundlerRequire is the generated Rails gem-group load, which loads
	// Gemfile gems, never application code (RUBY-AFFECTED-V0-013).
	stockBundlerRequire = regexp.MustCompile(`^Bundler\.require\(\*Rails\.groups\)[\t ]*(?:#.*)?$`)
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
	text     string
	// frontier holds the located sites that bear only on this file's unit
	// (RUBY-AFFECTED-V0-047).
	frontier map[string]bool
}

type loadRef struct {
	target   string
	relative bool
	owner    string
	// line is the 1-based line of the load; conditional reports a load the
	// scan cannot prove runs unconditionally (RUBY-AFFECTED-V0-046).
	line        int
	conditional bool
}

// Units observes every Ruby file in the repository rooted at root.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	files, err := root.Files(func(name string) bool {
		return strings.HasSuffix(name, ".rb")
	})
	if err != nil {
		return affected.Result{}, err
	}
	features, err := root.Files(func(name string) bool {
		return strings.HasSuffix(name, ".feature")
	})
	if err != nil {
		return affected.Result{}, err
	}
	if goModuleRoot(root) {
		files, features = withoutTestdata(files), withoutTestdata(features)
	}
	// Every frontier entry is located (RUBY-AFFECTED-V0-046). A cause that
	// can hide an edge to any change stays plugin-wide; a site that bears on
	// its own file's unit alone is that unit's frontier (RUBY-AFFECTED-V0-047).
	frontier := map[string]bool{}
	for _, feature := range features {
		frontier[located(FrontierCucumberOwnership, feature, 0)] = true
	}
	loadRoots, err := rakeLoadRoots(root, frontier)
	if err != nil {
		return affected.Result{}, err
	}
	declared, declarationValid := readFrontierDeclaration(root)
	if !declarationValid {
		frontier[located(FrontierDeclarationInvalid, FrontierDeclarationPath, 0)] = true
	}
	rails := hasRailsLayout(files)
	railsObserved := rails
	fileSet := make(map[string]bool, len(files))
	for _, relative := range files {
		fileSet[relative] = true
	}
	observed := make([]observation, 0, len(files))
	autoloadSites := make(map[string][]int)
	var railsTests []string
	scans := helperScans{}
	for _, relative := range files {
		scanned := scans.scan(root, relative)
		if !scanned.readable {
			frontier[located(FrontierUnreadableSource, relative, 0)] = true
			continue
		}
		text := scanned.text
		code := codeForDetection(text)
		local := map[string]bool{}
		loads, sites := scanned.loads, scanned.sites
		for _, site := range sites {
			if site.scope == siteAutoload {
				autoloadSites[relative] = append(autoloadSites[relative], site.line)
				continue
			}
			frontier[located(site.reason, relative, site.line)] = true
		}
		kind := classify(relative, code, rails)
		hint, hintConflict, helperRails := helperFramework(root, scans, relative, loads, fileSet, loadRoots)
		railsObserved = railsObserved || helperRails
		if hintConflict {
			local[located(FrontierAmbiguousFramework, relative, 0)] = true
		}
		// A helper that defines no test of its own stays source whatever it
		// loads: rails_helper requiring spec_helper is not a spec
		// (RUBY-AFFECTED-V0-012).
		helper := kind == frameworkSource && strings.HasSuffix(path.Base(relative), "_helper.rb")
		if hint != frameworkSource && !helper && !hasDirectFramework(code) && !(kind == frameworkRSpec && hint == frameworkRailsTest) {
			kind = hint
		}
		if kind == frameworkRailsTest {
			railsObserved = true
			railsTests = append(railsTests, relative)
		}
		if frameworkAmbiguous(relative, code, kind, rails) && hint == frameworkSource {
			local[located(FrontierAmbiguousFramework, relative, 0)] = true
		}
		if hasDynamicTestDefinition(relative, code, kind) && !sharedSupportDefinition(relative, code, kind) {
			local[located(FrontierDynamicTestDefinition, relative, dynamicDefinitionLine(text))] = true
		}
		observed = append(observed, observation{
			relative: relative,
			kind:     kind,
			id:       unitID(kind, relative),
			loads:    loads,
			text:     text,
			frontier: local,
		})
	}
	// A Rails application loads constants by name, not by require. Its
	// constant references become advisory edges; the coarse frontier remains
	// only where the mapping cannot bound them (RUBY-AFFECTED-V0-009,
	// RUBY-AFFECTED-V0-010).
	// A repository with no Rails application but autoload-convention
	// sources gets the same constant edges impact reports, without the Rails
	// boot and route edges (RUBY-AFFECTED-V0-045).
	var autoload *railsAutoload
	if rails {
		autoload = newRailsAutoload(root, observed, files)
	} else {
		autoload = newConventionAutoload(root, observed, files)
	}
	// Autoload-root configuration in a source file is what the autoload
	// mapper reads: it is resolved by a valid autoload declaration and
	// otherwise located as a ruby:rails-autoload cause. Anywhere the mapper
	// does not read, it stays a located dynamic load.
	for _, item := range observed {
		for _, line := range autoloadSites[item.relative] {
			if autoload == nil || item.kind != frameworkSource || insideTestTree(item.relative, "spec") || insideTestTree(item.relative, "test") {
				frontier[located(FrontierDynamicLoad, item.relative, line)] = true
			}
		}
	}
	tests := hasTests(observed)
	if tests && railsObserved && !rails {
		for _, relative := range railsTests {
			frontier[located(FrontierRailsAutoload, relative, 0)] = true
		}
		if len(railsTests) == 0 {
			frontier[FrontierRailsAutoload] = true
		}
	}
	if tests && autoload != nil {
		for _, site := range autoload.unresolved {
			frontier[site] = true
		}
	}
	aliases := indexAliases(observed, loadRoots)
	paths := indexPaths(observed)
	advisory := make(map[string]map[string]bool, len(observed))
	routes := make(map[string]map[string]bool, len(observed))
	constants := make(map[string]int, len(observed))
	if autoload != nil {
		for _, item := range observed {
			edges, unbounded := autoload.edges(item)
			// A dynamic lookup or an unmapped association can name any
			// constant, so its site bears on every change.
			if tests {
				for _, line := range unbounded {
					frontier[located(FrontierRailsAutoload, item.relative, line)] = true
				}
			}
			constants[item.id] = len(edges)
			if rails && item.kind != frameworkSource {
				boot, routed := autoload.bootEdges(item)
				for id := range boot {
					edges[id] = true
				}
				routes[item.id] = routed
			}
			advisory[item.id] = edges
		}
	}
	units := make([]affected.Unit, 0, len(observed))
	siteSets := make([]map[string]bool, 0, len(observed)+1)
	support := supportIndex{}
	for _, item := range observed {
		imports := resolveLoads(item, aliases, item.frontier)
		mapped := addConventionalCounterpart(item, paths, imports)
		reached := len(imports) != 0 || mapped || constants[item.id] != 0
		// A test with no visible edge can depend on any change.
		if item.kind != frameworkSource && !reached {
			frontier[located(FrontierImplicitTestDependency, item.relative, 0)] = true
		}
		for id := range support.supportEdges(item, paths) {
			imports[id] = true
		}
		unit := affected.Unit{ID: item.id, Imports: sortedKeys(imports), AdvisoryImports: sortedEdges(advisory[item.id], imports)}
		// A controller the test also names by constant keeps that edge.
		named := maps.Clone(imports)
		maps.Copy(named, advisory[item.id])
		unit.RouteDispatchImports = sortedEdges(routes[item.id], named)
		if len(unit.Imports) > affected.MaxPathsPerUnit || len(unit.AdvisoryImports) > affected.MaxPathsPerUnit ||
			len(unit.RouteDispatchImports) > affected.MaxPathsPerUnit {
			// Dropped edges can no longer reach the unit, so its frontier
			// stays plugin-wide.
			frontier[located(FrontierImplicitTestDependency, item.relative, 0)] = true
			unit.Imports, unit.AdvisoryImports, unit.RouteDispatchImports = nil, nil, nil
		}
		if item.kind == frameworkSource {
			unit.Sources = []string{item.relative}
		} else {
			unit.Tests = []string{item.relative}
		}
		unit.Frontier = undeclared(item.frontier, declared)
		siteSets = append(siteSets, item.frontier)
		units = append(units, unit)
	}
	// A declared entry that resolves no observed site is stale and stays
	// visible as a plugin-wide site (RUBY-AFFECTED-V0-960).
	for _, stale := range staleDeclarations(declared, append(siteSets, frontier)) {
		frontier[stale] = true
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: undeclared(frontier, declared)}, nil
}

// goModuleRoot reports a repository whose root declares a Go module. There a
// testdata directory is fixture data by the go tool's convention, never a
// test or source tree of the host repository (RUBY-AFFECTED-V0-041).
func goModuleRoot(root *affected.Source) bool {
	_, err := root.Read("go.mod")
	return err == nil
}

// withoutTestdata drops paths with a testdata component (RUBY-AFFECTED-V0-041).
func withoutTestdata(files []string) []string {
	kept := files[:0:0]
	for _, relative := range files {
		if !hasTestdataComponent(relative) {
			kept = append(kept, relative)
		}
	}
	return kept
}

func hasTestdataComponent(relative string) bool {
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "testdata" {
			return true
		}
	}
	return false
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

// helperScan is one observation's single read of a Ruby file, shared by the
// source loop and helperFramework: every spec of a suite loads the same
// helpers, and a helper is also a source file, so one Build reads and scans
// each file once instead of once as a source and once per loading spec
// (AFP-V0-394). readable is false for a file that could not be read or
// decoded, which contributes nothing either way. hints is derived on first
// use as a helper.
type helperScan struct {
	readable bool
	text     string
	loads    []loadRef
	sites    []loadSite
	hinted   bool
	hints    map[framework]bool
}

// helperScans memoizes helperScan by repository path within one observation.
type helperScans map[string]*helperScan

func (scans helperScans) scan(root *affected.Source, candidate string) *helperScan {
	if scanned, ok := scans[candidate]; ok {
		return scanned
	}
	scanned := &helperScan{}
	if body, err := root.Read(candidate); err == nil && utf8.Valid(body) {
		scanned.readable, scanned.text = true, string(body)
		scanned.loads, scanned.sites = scanLoads(candidate, scanned.text)
	}
	scans[candidate] = scanned
	return scanned
}

func (scanned *helperScan) frameworkHints() map[framework]bool {
	if !scanned.hinted {
		scanned.hints, scanned.hinted = frameworkHints(codeForDetection(scanned.text)), true
	}
	return scanned.hints
}

func helperFramework(root *affected.Source, scans helperScans, owner string, loads []loadRef, files map[string]bool, loadRoots []string) (framework, bool, bool) {
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
				scanned := scans.scan(root, candidate)
				if !scanned.readable {
					continue
				}
				for hint := range scanned.frameworkHints() {
					if hint == frameworkRailsTest {
						rails = true
						continue
					}
					runnerHints[hint] = true
				}
				visit(candidate, scanned.loads)
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

// siteScope says how far a located load site bears (RUBY-AFFECTED-V0-046).
type siteScope int

const (
	// siteGlobal bears on every change: it changes how loads resolve, or
	// hides what its own load reaches, so no reverse closure bounds it.
	siteGlobal siteScope = iota
	// siteAutoload is autoload-root configuration, which a valid autoload
	// declaration stands in for.
	siteAutoload
)

// loadSite is one located construct the load scan cannot bound.
type loadSite struct {
	reason string
	line   int
	scope  siteScope
}

func scanLoads(owner, body string) ([]loadRef, []loadSite) {
	refs := make([]loadRef, 0, 8)
	sites := make([]loadSite, 0, 2)
	for index, line := range strings.Split(body, "\n") {
		number := index + 1
		if coveredSupportGlob(owner, line) || stockGemGroupLoad(owner, line) {
			continue
		}
		switch {
		case strings.Contains(line, "$LOAD_PATH") || strings.Contains(line, "$:"):
			sites = append(sites, loadSite{FrontierDynamicLoad, number, siteGlobal})
		case strings.Contains(line, "autoload_paths"):
			sites = append(sites, loadSite{FrontierDynamicLoad, number, siteAutoload})
		}
		statements := strings.Split(line, ";")
		first := strings.TrimSpace(statements[0])
		conditionalLine := strings.HasPrefix(first, "if ") || strings.HasPrefix(first, "unless ")
		for statementIndex, statement := range statements {
			trimmed := strings.TrimSpace(statement)
			embedded := embeddedLoad.FindAllStringSubmatch(trimmed, -1)
			if len(embedded) == 0 {
				if loadStartPattern.MatchString(trimmed) || anyLoadCall.MatchString(trimmed) || anyAutoload.MatchString(trimmed) || strings.Contains(trimmed, ".require(") || strings.Contains(trimmed, "send(:require") {
					sites = append(sites, loadSite{FrontierDynamicLoad, number, siteGlobal})
				}
				continue
			}
			firstMatch := embeddedLoad.FindStringIndex(trimmed)
			literal := literalLoadPattern.FindStringIndex(trimmed)
			suffix := ""
			if firstMatch != nil {
				suffix = strings.TrimSpace(trimmed[firstMatch[1]:])
			}
			conditional := len(embedded) > 1 || literal == nil || firstMatch[0] != 0 || suffix != "" && !strings.HasPrefix(suffix, "#") || statementIndex > 0 && conditionalLine || len(line) != len(strings.TrimLeft(line, " \t"))
			for _, match := range embedded {
				if strings.Contains(match[2], "#{") {
					sites = append(sites, loadSite{FrontierDynamicLoad, number, siteGlobal})
					continue
				}
				refs = append(refs, loadRef{
					target:      strings.TrimSuffix(path.Clean(match[2]), ".rb"),
					relative:    match[1] == "require_relative",
					owner:       owner,
					line:        number,
					conditional: conditional,
				})
			}
		}
	}
	return refs, sites
}

// located names one frontier site: the reason, then the repository path and
// the 1-based line where they are known (RUBY-AFFECTED-V0-046).
func located(reason, relative string, line int) string {
	switch {
	case relative == "":
		return reason
	case line > 0:
		return reason + ":" + relative + ":" + strconv.Itoa(line)
	default:
		return reason + ":" + relative
	}
}

// stockGemGroupLoad reports the unindented `Bundler.require(*Rails.groups)`
// line of a Rails application's config/application.rb. Any other spelling,
// owner, or nesting keeps the ruby:dynamic-load frontier (RUBY-AFFECTED-V0-013).
func stockGemGroupLoad(owner, line string) bool {
	return (owner == "config/application.rb" || strings.HasSuffix(owner, "/config/application.rb")) &&
		stockBundlerRequire.MatchString(strings.TrimRight(line, "\r"))
}

// resolveLoads turns item's literal loads into import edges. A load several
// units answer to gets an edge to each, a sound superset; a relative load no
// unit answers, and a conditional load of a repository unit, are located on
// the unit (RUBY-AFFECTED-V0-046, RUBY-AFFECTED-V0-047).
func resolveLoads(item observation, aliases map[string][]string, local map[string]bool) map[string]bool {
	imports := make(map[string]bool, len(item.loads)+1)
	for _, ref := range item.loads {
		target := ref.target
		if ref.relative {
			target = path.Clean(path.Join(path.Dir(ref.owner), target))
			if target == ".." || strings.HasPrefix(target, "../") {
				local[located(FrontierRelativeLoad, item.relative, ref.line)] = true
				continue
			}
		}
		matches := aliases[target]
		if len(matches) == 0 {
			if ref.relative {
				local[located(FrontierRelativeLoad, item.relative, ref.line)] = true
			}
			continue
		}
		if ref.conditional {
			local[located(FrontierConditionalLoad, item.relative, ref.line)] = true
		}
		for _, match := range matches {
			if match != item.id {
				imports[match] = true
			}
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

func rakeLoadRoots(root *affected.Source, frontier map[string]bool) ([]string, error) {
	files, err := root.Files(func(name string) bool { return name == "Rakefile" })
	if err != nil {
		return nil, err
	}
	roots := make(map[string]bool)
	for _, relative := range files {
		body, readErr := root.Read(relative)
		if readErr != nil || !utf8.Valid(body) {
			frontier[located(FrontierUnreadableSource, relative, 0)] = true
			continue
		}
		text := string(body)
		if !strings.Contains(text, "Rake::TestTask") {
			continue
		}
		for index, line := range strings.Split(text, "\n") {
			declaration := strings.TrimSpace(line)
			if !rakeLibDeclaration.MatchString(declaration) {
				continue
			}
			match := rakeLiteralLib.FindStringSubmatch(declaration)
			if len(match) != 2 {
				frontier[located(FrontierDynamicLoad, relative, index+1)] = true
				continue
			}
			if strings.Contains(match[1], `\`) {
				frontier[located(FrontierDynamicLoad, relative, index+1)] = true
				continue
			}
			loadRoot := path.Clean(strings.TrimSpace(match[1]))
			if loadRoot == "." {
				continue
			}
			if path.IsAbs(loadRoot) || loadRoot == ".." || strings.HasPrefix(loadRoot, "../") {
				frontier[located(FrontierDynamicLoad, relative, index+1)] = true
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

// addConventionalCounterpart adds the edge from a test to the source its
// path names; when both app/ and lib/ hold one, it adds both
// (RUBY-AFFECTED-V0-046).
func addConventionalCounterpart(item observation, paths map[string]string, imports map[string]bool) bool {
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
	mapped := false
	for _, sourceRoot := range []string{"app/", "lib/"} {
		if id, exists := paths[prefix+sourceRoot+stem+".rb"]; exists {
			imports[id] = true
			mapped = true
		}
	}
	return mapped
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

// sharedSupportDefinition reports a source file below a suite's support
// directory whose only dynamic definition is shared examples or a shared
// context: every test of the suite imports it (RUBY-AFFECTED-V0-012), and the
// examples run under the including test file's identity.
func sharedSupportDefinition(relative, body string, kind framework) bool {
	if kind != frameworkSource || !supportDirectory(relative) || dynamicDefinition.MatchString(body) {
		return false
	}
	without := strings.NewReplacer("shared_examples", "", "shared_context", "").Replace(body)
	return !hasDynamicTestDefinition(relative, without, kind)
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
		if end, ok := rubySlashRegex(body, index); ok {
			for maskIndex := index; maskIndex <= end; maskIndex++ {
				masked[maskIndex] = ' '
			}
			index = end
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

// regexOperandKeywords are the keywords after which a slash opens a regex
// literal rather than dividing.
var regexOperandKeywords = map[string]bool{
	"and": true, "elsif": true, "if": true, "in": true, "not": true, "or": true, "return": true,
	"unless": true, "until": true, "when": true, "while": true, "yield": true,
}

// rubySlashRegex reports the index of the closing slash of a /.../ regex
// literal opening at start (RUBY-AFFECTED-V0-044). A slash opens one only in an
// operand position: at the start of its line, after an operator or opening
// bracket, or after a keyword such as when or if. Division (`a / b`, `a/b`,
// `f(x) / 2`) is never an operand position. A literal that does not close on
// its own line is not masked, so an unusual division only keeps its text.
func rubySlashRegex(body string, start int) (int, bool) {
	if body[start] != '/' {
		return 0, false
	}
	before := start - 1
	for before >= 0 && (body[before] == ' ' || body[before] == '\t') {
		before--
	}
	if before >= 0 && body[before] != '\n' {
		previous := body[before]
		switch {
		case strings.IndexByte("(,=~!|&{[;<>+-*%^:", previous) >= 0:
		case previous == '?' && before > 0 && (body[before-1] == ' ' || body[before-1] == '\t'):
		case isWordByte(previous):
			wordStart := before
			for wordStart > 0 && isWordByte(body[wordStart-1]) {
				wordStart--
			}
			if wordStart > 0 && (body[wordStart-1] == '.' || body[wordStart-1] == ':') {
				return 0, false
			}
			if !regexOperandKeywords[body[wordStart:before+1]] {
				return 0, false
			}
		default:
			return 0, false
		}
	}
	class := false
	for index := start + 1; index < len(body); index++ {
		switch body[index] {
		case '\n':
			return 0, false
		case '\\':
			if index+1 < len(body) && body[index+1] == '\n' {
				return 0, false
			}
			index++
		case '[':
			class = true
		case ']':
			class = false
		case '/':
			if !class {
				return index, true
			}
		}
	}
	return 0, false
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
