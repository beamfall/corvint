// SPDX-License-Identifier: Apache-2.0

// Package langplugin is the in-process language plugin manifest
// (docs/specs/language-plugins-v0.md, LGP-V0-001..022). Each language declares
// the paths it owns, its affected-selection provider, its analyzercap
// analyzers and its sub-plugins: test runners, test providers, coverage
// collectors and mutation tools. A piece a language lacks is declared absent
// with a reason, so a missing piece is never silent.
//
// Core packages read languages through this manifest instead of naming them.
// Registration is in-process; the analyzercap subprocess protocol stays the
// external form of an analyzer.
package langplugin

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/cfamily"
	"github.com/Beamfall/corvint/internal/liveverify/affected/dotnet"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/java"
	"github.com/Beamfall/corvint/internal/liveverify/affected/kotlin"
	"github.com/Beamfall/corvint/internal/liveverify/affected/python"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
	"github.com/Beamfall/corvint/internal/liveverify/affected/rust"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
	"github.com/Beamfall/corvint/internal/liveverify/affected/swift"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// Piece names one declarable part of a language plugin (LGP-V0-002).
type Piece string

const (
	Provider     Piece = "provider"
	Analyzer     Piece = "analyzer"
	Runners      Piece = "runners"
	TestProvider Piece = "test-provider"
	Coverage     Piece = "coverage"
	Mutation     Piece = "mutation"
)

func dialectKindNames() []string {
	out := make([]string, len(DialectKinds))
	for i, k := range DialectKinds {
		out[i] = string(k)
	}
	return out
}

// Pieces is every piece, in declaration order.
var Pieces = []Piece{Provider, Analyzer, Runners, TestProvider, Coverage, Mutation}

// SubPlugin is one tool of a language: an analyzer binary, a test provider,
// a coverage collector or a mutation tool, with the repository-relative
// directories that implement it.
type SubPlugin struct {
	Name  string
	Paths []string
	// Suffixes are the source suffixes an in-process mutation engine
	// mutates (LGP-V0-018); other sub-plugins leave it empty.
	Suffixes []string
}

// DialectKind and Dialect are declared in the provider-free decl package
// (LGP-V0-015), so a core package can read them without linking a provider.
type (
	DialectKind = decl.DialectKind
	Dialect     = decl.Dialect
)

const (
	ImpactDialect = decl.ImpactDialect
	ModuleDialect = decl.ModuleDialect
	RegionDialect = decl.RegionDialect
)

// DialectKinds is every dialect kind, in declaration order.
var DialectKinds = decl.DialectKinds

// Manifest is one language plugin (LGP-V0-001).
type Manifest struct {
	// Name is the plugin namespace; with a provider it equals Provider().Name().
	Name string
	// Paths are the repository-relative directories this language owns
	// exclusively. Shared directories appear only in sub-plugin Paths.
	Paths []string
	// Provider builds the affected-selection provider (affected.Language).
	Provider func() affected.Language
	// Analyzers are the analyzercap binaries, by cmd/ directory name.
	Analyzers []SubPlugin
	// Runners are testrunner registry names; their build, parse and
	// normalize live in the registry family that registers them.
	Runners       []string
	TestProviders []SubPlugin
	Coverage      []SubPlugin
	Mutation      []SubPlugin
	// Absent declares each piece the language lacks, with its reason.
	Absent map[Piece]string
	// Dialects are the core readings this language's sources select
	// (LGP-V0-015), taken from decl.Of(Name).
	Dialects []Dialect
	// Dependencies maps an ecosystem package (a gem, an npm package) to the
	// plugin runner it brings (LGP-V0-017), taken from decl.Of(Name).
	Dependencies map[string]string
}

func sub(name string, paths ...string) SubPlugin { return SubPlugin{Name: name, Paths: paths} }

const noCollector = "no line-coverage collector yet (LPCV-V0-955, MCJ-V0)"
const noMutation = "no mutation tool yet"
const noTestProvider = "no Corvint test provider binary"
const noProvider = "no affected-selection provider; changes widen scope"

var manifests = []Manifest{
	{
		Name:      "cfamily",
		Paths:     []string{"cmd/corvint-analyzer-c-jni", "cmd/corvint-analyzer-objective-c", "internal/liveverify/affected/cfamily"},
		Provider:  func() affected.Language { return cfamily.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-c-jni", "internal/analyzernativebridge"), sub("corvint-analyzer-objective-c", "internal/analyzernativebridge")},
		Runners:   []string{"boost-test-junit", "catch2", "cmocka-xml", "ctest", "googletest"},
		Absent:    map[Piece]string{TestProvider: noTestProvider, Mutation: noMutation},
		Coverage:  []SubPlugin{sub("linecov-native", "internal/nativelinecov")},
	},
	{
		Name:      "dotnet",
		Paths:     []string{"cmd/corvint-analyzer-dotnet", "internal/analyzerdotnet", "internal/liveverify/affected/dotnet"},
		Provider:  func() affected.Language { return dotnet.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-dotnet", "internal/analyzerdotnet")},
		Runners:   []string{"dotnet-mtp-mstest", "dotnet-mtp-nunit", "dotnet-mtp-xunit", "dotnet-vstest-mstest", "dotnet-vstest-nunit", "dotnet-vstest-xunit"},
		Absent:    map[Piece]string{TestProvider: noTestProvider, Mutation: noMutation},
		Coverage:  []SubPlugin{sub("linecov-native", "internal/nativelinecov")},
	},
	{
		Name: "go",
		Paths: []string{"cmd/corvint-analyzer-go", "cmd/corvint-go-test-provider", "internal/analyzergo", "internal/gotestrun",
			"internal/liveverify/affected/golang", "internal/liveverify/godiscovery", "internal/liveverify/gorunner",
			"internal/liveverify/gotest", "internal/liveverify/mutate"},
		Provider:      func() affected.Language { return golang.New() },
		Analyzers:     []SubPlugin{sub("corvint-analyzer-go", "internal/analyzergo")},
		Runners:       []string{"ginkgo-v2", "go-test"},
		TestProviders: []SubPlugin{sub("corvint-go-test-provider", "cmd/corvint-go-test-provider", "internal/liveverify/gotest")},
		Coverage:      []SubPlugin{sub("linecov-go", "internal/linecov", "internal/liveverify/gorunner")},
		Mutation:      []SubPlugin{{Name: "mutate-go", Paths: []string{"internal/liveverify/mutate"}, Suffixes: []string{".go"}}},
	},
	{
		Name:      "java",
		Paths:     []string{"cmd/corvint-analyzer-java", "internal/liveverify/affected/java"},
		Provider:  func() affected.Language { return java.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-java", "internal/analyzernativebridge")},
		Runners:   []string{"gradle-junit", "junit-platform", "junit4", "maven-surefire", "testng"},
		Absent:    map[Piece]string{TestProvider: noTestProvider, Mutation: noMutation},
		Coverage:  []SubPlugin{sub("linecov-native", "internal/nativelinecov")},
	},
	{
		Name:      "kotlin",
		Paths:     []string{"cmd/corvint-analyzer-kotlin-android", "internal/analyzerkotlinandroid", "internal/liveverify/affected/kotlin"},
		Provider:  func() affected.Language { return kotlin.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-kotlin-android", "internal/analyzerkotlinandroid")},
		// The Android UI runners, Appium included, test Android applications.
		Runners:  []string{"android-junit", "appium", "appium-uiautomator2-wdio", "espresso", "kotest", "kotlin-test", "robolectric", "uiautomator"},
		Absent:   map[Piece]string{TestProvider: noTestProvider, Mutation: noMutation},
		Coverage: []SubPlugin{sub("linecov-native", "internal/nativelinecov")},
	},
	{
		Name: "python",
		Paths: []string{"cmd/corvint-analyzer-python", "internal/analyzerpython", "internal/liveverify/affected/python",
			"internal/liveverify/pymutate", "internal/liveverify/pyresolve", "internal/pythongrammar", "internal/pythonsyntax"},
		Provider:  func() affected.Language { return python.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-python", "internal/analyzerpython")},
		Runners:   []string{"pytest", "unittest"},
		Mutation:  []SubPlugin{{Name: "pymutate", Paths: []string{"internal/liveverify/pymutate"}, Suffixes: []string{".py"}}},
		Absent:    map[Piece]string{TestProvider: noTestProvider, Coverage: noCollector},
	},
	{
		Name: "ruby",
		Paths: []string{"cmd/corvint-analyzer-ruby", "cmd/corvint-ruby-test-provider", "internal/analyzerruby",
			"internal/liveverify/affected/ruby", "internal/liveverify/rbmutate", "internal/rubyautoload", "internal/rubydialect", "internal/rubylinecov",
			"internal/rubyprofile", "internal/rubytestprovider"},
		Provider:      func() affected.Language { return ruby.New() },
		Analyzers:     []SubPlugin{sub("corvint-analyzer-ruby", "internal/analyzerruby")},
		Runners:       []string{"minitest", "rails-test", "rspec", "test-unit"},
		TestProviders: []SubPlugin{sub("corvint-ruby-test-provider", "cmd/corvint-ruby-test-provider", "internal/rubytestprovider")},
		Mutation: []SubPlugin{{Name: "rbmutate", Paths: []string{"internal/liveverify/rbmutate"}, Suffixes: []string{".rb"}},
			sub("extmutate", "internal/extmutate")},
		Coverage: []SubPlugin{sub("linecov-ruby", "internal/rubylinecov")},
	},
	{
		Name:      "rust",
		Paths:     []string{"cmd/corvint-analyzer-rust", "internal/analyzerrust", "internal/liveverify/affected/rust"},
		Provider:  func() affected.Language { return rust.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-rust", "internal/analyzerrust")},
		Runners:   []string{"cargo-bin", "cargo-doctest", "cargo-integration", "cargo-test", "nextest"},
		Absent:    map[Piece]string{TestProvider: noTestProvider, Mutation: noMutation},
		Coverage:  []SubPlugin{sub("linecov-native", "internal/nativelinecov")},
	},
	{
		Name:      "shell",
		Paths:     []string{"cmd/corvint-analyzer-shell", "internal/analyzershell", "internal/liveverify/affected/shell"},
		Provider:  func() affected.Language { return shell.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-shell", "internal/analyzershell")},
		Runners:   []string{"bats", "shellspec"},
		Absent:    map[Piece]string{TestProvider: noTestProvider, Coverage: noCollector, Mutation: noMutation},
	},
	{
		Name:      "swift",
		Paths:     []string{"cmd/corvint-analyzer-swift", "internal/analyzerswift", "internal/liveverify/affected/swift", "internal/swiftimpact"},
		Provider:  func() affected.Language { return swift.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-swift", "internal/analyzerswift")},
		Runners:   []string{"swift-testing", "swift-xctest", "xcode-xctest"},
		Absent:    map[Piece]string{TestProvider: noTestProvider, Mutation: noMutation},
		Coverage:  []SubPlugin{sub("linecov-native", "internal/nativelinecov")},
	},
	{
		Name: "typescript",
		Paths: []string{"cmd/corvint-analyzer-js", "cmd/corvint-js-test-provider", "internal/analyzerjs", "internal/browsercov",
			"internal/jslinecov", "internal/jstestprovider", "internal/liveverify/affected/typescript", "internal/liveverify/jsresolve", "internal/webdialect"},
		Provider:  func() affected.Language { return typescript.New() },
		Analyzers: []SubPlugin{sub("corvint-analyzer-js", "internal/analyzerjs")},
		Runners: []string{"ava", "bun-test", "cypress", "deno-test", "detox", "jasmine", "jest", "mocha", "nightwatch", "node-test",
			"playwright", "storybook-test-runner", "storybook-vitest", "testcafe", "vitest", "webdriverio"},
		TestProviders: []SubPlugin{sub("corvint-js-test-provider", "cmd/corvint-js-test-provider", "internal/jstestprovider")},
		Coverage: []SubPlugin{sub("linecov-js", "internal/jslinecov", "internal/linecov", "internal/linecov/v8"),
			sub("linecov-browser", "internal/browsercov")},
		Mutation: []SubPlugin{sub("extmutate", "internal/extmutate")},
	},
	{
		Name:      "sql",
		Paths:     []string{"cmd/corvint-analyzer-sql-native"},
		Analyzers: []SubPlugin{sub("corvint-analyzer-sql-native", "cmd/corvint-analyzer-sql-native")},
		Runners:   []string{"pgtap", "sql-pgtap", "sql-sqllogictest-sqlite", "sqllogictest"},
		Absent:    map[Piece]string{Provider: noProvider, TestProvider: noTestProvider, Coverage: noCollector, Mutation: noMutation},
	},
	{
		Name:      "shader",
		Paths:     []string{"cmd/corvint-analyzer-shader", "internal/analyzershader"},
		Analyzers: []SubPlugin{sub("corvint-analyzer-shader", "internal/analyzershader")},
		Runners:   []string{"shader-behavior"},
		Absent:    map[Piece]string{Provider: noProvider, TestProvider: noTestProvider, Coverage: noCollector, Mutation: noMutation},
	},
	{
		Name:      "html-css",
		Paths:     []string{"cmd/corvint-analyzer-html-css", "internal/analyzerhtmlcss"},
		Analyzers: []SubPlugin{sub("corvint-analyzer-html-css", "internal/analyzerhtmlcss")},
		Runners:   []string{"html-behavior"},
		Absent:    map[Piece]string{Provider: noProvider, TestProvider: noTestProvider, Coverage: noCollector, Mutation: noMutation},
	},
	{
		Name:      "structured-data",
		Paths:     []string{"cmd/corvint-analyzer-structured-data", "internal/analyzerstructured"},
		Analyzers: []SubPlugin{sub("corvint-analyzer-structured-data", "internal/analyzerstructured")},
		Runners:   []string{"structured-data-behavior"},
		Absent:    map[Piece]string{Provider: noProvider, TestProvider: noTestProvider, Coverage: noCollector, Mutation: noMutation},
	},
}

func init() {
	for i, m := range manifests {
		d := decl.Of(m.Name)
		manifests[i].Dialects, manifests[i].Dependencies = d.Dialects, d.Dependencies
	}
}

// All returns every registered language plugin, in registration order.
func All() []Manifest { return append([]Manifest(nil), manifests...) }

// Lookup returns the plugin named name.
func Lookup(name string) (Manifest, bool) {
	for _, m := range manifests {
		if m.Name == name {
			return m, true
		}
	}
	return Manifest{}, false
}

// Providers returns a fresh affected-selection provider for every plugin that
// declares one, in registration order (LGP-V0-005).
func Providers() []affected.Language {
	var out []affected.Language
	for _, m := range manifests {
		if m.Provider != nil {
			out = append(out, m.Provider())
		}
	}
	return out
}

// RunnerLanguage returns the plugin that declares runner (LGP-V0-006).
func RunnerLanguage(runner string) (string, bool) {
	for _, m := range manifests {
		for _, r := range m.Runners {
			if r == runner {
				return m.Name, true
			}
		}
	}
	return "", false
}

// DialectFor returns the dialect of kind whose suffixes include path's
// extension, or "" when no plugin declares one (LGP-V0-015).
func DialectFor(kind DialectKind, name string) string { return decl.DialectFor(kind, name) }

// Dialects returns every declared dialect name of kind, sorted.
func Dialects(kind DialectKind) []string { return decl.Dialects(kind) }

// DependencyRunners returns a copy of language's package-to-runner map
// (LGP-V0-017).
func DependencyRunners(language string) map[string]string { return decl.DependencyRunners(language) }

// MutationLanguage returns the plugin whose in-process mutation engine
// mutates path's suffix, or "" (LGP-V0-018).
func MutationLanguage(name string) string {
	ext := path.Ext(name)
	for _, m := range manifests {
		for _, s := range m.Mutation {
			if contains(s.Suffixes, ext) {
				return m.Name
			}
		}
	}
	return ""
}

// Dirs lists the plugin's directories: its owned paths and every
// sub-plugin path, sorted and without duplicates (LGP-V0-020).
func Dirs(name string) ([]string, bool) {
	m, ok := Lookup(name)
	if !ok {
		return nil, false
	}
	out := append([]string{}, m.Paths...)
	for _, list := range [][]SubPlugin{m.Analyzers, m.TestProviders, m.Coverage, m.Mutation} {
		for _, s := range list {
			for _, p := range s.Paths {
				if !contains(out, p) {
					out = append(out, p)
				}
			}
		}
	}
	sort.Strings(out)
	return out, true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Names returns every plugin name and every declared runner name, sorted.
func Names() []string {
	var out []string
	for _, m := range manifests {
		out = append(out, m.Name)
		out = append(out, m.Runners...)
	}
	sort.Strings(out)
	return out
}

// present reports whether m declares piece.
func (m Manifest) present(piece Piece) bool {
	switch piece {
	case Provider:
		return m.Provider != nil
	case Analyzer:
		return len(m.Analyzers) > 0
	case Runners:
		return len(m.Runners) > 0
	case TestProvider:
		return len(m.TestProviders) > 0
	case Coverage:
		return len(m.Coverage) > 0
	case Mutation:
		return len(m.Mutation) > 0
	}
	return false
}

// Validate checks one manifest's own declaration (LGP-V0-007): a name, every
// piece either present or absent with a reason and never both, a provider
// whose namespace is the plugin name, and no empty sub-plugin or path: a
// blank or padded path would resolve to the repository root.
func Validate(m Manifest) error {
	var errs []error
	if m.Name == "" || strings.TrimSpace(m.Name) != m.Name {
		errs = append(errs, fmt.Errorf("plugin name %q is empty or padded", m.Name))
	}
	if len(m.Paths) == 0 {
		errs = append(errs, fmt.Errorf("%s: owns no path", m.Name))
	}
	for _, p := range m.Paths {
		if blankPath(p) {
			errs = append(errs, fmt.Errorf("%s: owned path %q is empty or padded", m.Name, p))
		}
	}
	known := map[Piece]bool{}
	for _, piece := range Pieces {
		known[piece] = true
		reason, absent := m.Absent[piece]
		switch {
		case m.present(piece) && absent:
			errs = append(errs, fmt.Errorf("%s: %s is declared and also absent", m.Name, piece))
		case !m.present(piece) && !absent:
			errs = append(errs, fmt.Errorf("%s: %s is missing and not declared absent", m.Name, piece))
		case absent && strings.TrimSpace(reason) == "":
			errs = append(errs, fmt.Errorf("%s: %s is absent without a reason", m.Name, piece))
		}
	}
	for piece := range m.Absent {
		if !known[piece] {
			errs = append(errs, fmt.Errorf("%s: unknown piece %q", m.Name, piece))
		}
	}
	if m.Provider != nil {
		if p := m.Provider(); p == nil || p.Name() != m.Name {
			errs = append(errs, fmt.Errorf("%s: provider namespace differs from the plugin name", m.Name))
		}
	}
	for _, list := range [][]SubPlugin{m.Analyzers, m.TestProviders, m.Coverage, m.Mutation} {
		for _, s := range list {
			if s.Name == "" || len(s.Paths) == 0 {
				errs = append(errs, fmt.Errorf("%s: sub-plugin %q names no path", m.Name, s.Name))
			}
			for _, p := range s.Paths {
				if blankPath(p) {
					errs = append(errs, fmt.Errorf("%s: sub-plugin %q path %q is empty or padded", m.Name, s.Name, p))
				}
			}
		}
	}
	for _, r := range m.Runners {
		if r == "" {
			errs = append(errs, fmt.Errorf("%s: empty runner name", m.Name))
		}
	}
	for _, d := range m.Dialects {
		if !contains(dialectKindNames(), string(d.Kind)) {
			errs = append(errs, fmt.Errorf("%s: unknown dialect kind %q", m.Name, d.Kind))
		}
		if d.Name == "" || len(d.Suffixes) == 0 {
			errs = append(errs, fmt.Errorf("%s: %s dialect %q is unnamed or reads no suffix", m.Name, d.Kind, d.Name))
		}
		if wantsRule := d.Kind == ImpactDialect || d.Kind == ModuleDialect; wantsRule != (d.Rule != "") {
			errs = append(errs, fmt.Errorf("%s: %s dialect %q declares rule %q; impact and module dialects name one, region dialects none (LGP-V0-024)", m.Name, d.Kind, d.Name, d.Rule))
		}
		if d.Rule != "" && decl.RuleKind(d.Rule) != d.Kind {
			errs = append(errs, fmt.Errorf("%s: %s dialect %q declares rule %q, which core does not implement for that kind (LGP-V0-024)", m.Name, d.Kind, d.Name, d.Rule))
		}
		for _, suffix := range d.Suffixes {
			if len(suffix) < 2 || suffix[0] != '.' || strings.TrimSpace(suffix) != suffix {
				errs = append(errs, fmt.Errorf("%s: %s dialect %s suffix %q is empty or padded", m.Name, d.Kind, d.Name, suffix))
			}
		}
	}
	for pkg, runner := range m.Dependencies {
		if pkg == "" {
			errs = append(errs, fmt.Errorf("%s: empty dependency name", m.Name))
		}
		if !contains(m.Runners, runner) {
			errs = append(errs, fmt.Errorf("%s: dependency %s names runner %q the plugin does not declare", m.Name, pkg, runner))
		}
	}
	return errors.Join(errs...)
}

// blankPath reports a declared path that is empty, all space or padded.
func blankPath(p string) bool { return strings.TrimSpace(p) == "" || strings.TrimSpace(p) != p }
