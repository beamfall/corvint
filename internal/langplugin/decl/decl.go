// SPDX-License-Identifier: Apache-2.0

// Package decl is the provider-free part of the language plugin manifests
// (docs/specs/language-plugins-v0.md, LGP-V0-015 to LGP-V0-017): each
// language's dialects and runner dependencies. It imports no affected-selection
// provider, so a core package outside the planner's link closure, such as
// internal/contextindex in corvint-tasks (NGI-V0-988), reads the declarations
// without linking the providers. internal/langplugin's manifests take their
// Dialects and Dependencies from here.
package decl

import (
	"path"
	"sort"
)

// DialectKind names a core reading a language's sources select by suffix
// (LGP-V0-015).
type DialectKind string

const (
	// ImpactDialect is the bounded syntax profile of non-Go impact.
	ImpactDialect DialectKind = "impact"
	// ModuleDialect is the module-membership reading of impact.
	ModuleDialect DialectKind = "module"
	// RegionDialect is the symbol-region normalizer family.
	RegionDialect DialectKind = "region"
)

// DialectKinds is every dialect kind, in declaration order.
var DialectKinds = []DialectKind{ImpactDialect, ModuleDialect, RegionDialect}

// Rule names the reverse-reference algorithm core runs for a dialect
// (LGP-V0-024). Core dispatches on the rule a plugin declares, never on the
// plugin's or dialect's name, so a second language sharing a rule needs no
// core edit.
type Rule string

const (
	// ConstantReferences resolves a changed path to the constants its
	// autoload map names and reads their references (an impact rule).
	ConstantReferences Rule = "constant-references"
	// ModuleImports reads ECMAScript import and require specifiers (an
	// impact rule).
	ModuleImports Rule = "module-imports"
	// NamespaceUsing reads namespace declarations and using directives (a
	// module rule).
	NamespaceUsing Rule = "namespace-using"
	// PackageImports reads package declarations and imports (a module rule).
	PackageImports Rule = "package-imports"
	// TargetModules reads build-target membership and module imports (a
	// module rule).
	TargetModules Rule = "target-modules"
)

// RuleKind returns the dialect kind rule belongs to, or "" for a rule core
// does not implement (LGP-V0-024).
func RuleKind(rule Rule) DialectKind {
	switch rule {
	case ConstantReferences, ModuleImports:
		return ImpactDialect
	case NamespaceUsing, PackageImports, TargetModules:
		return ModuleDialect
	}
	return ""
}

// Dialect is one core reading a plugin's sources select: its kind, the
// dialect name the core reading answers, the suffixes it reads and, for an
// impact or module dialect, the reverse-reference rule core runs for it.
type Dialect struct {
	Kind     DialectKind
	Name     string
	Suffixes []string
	Rule     Rule
}

// Language is the provider-free declaration of one language plugin.
type Language struct {
	// Name is the plugin namespace, equal to its langplugin manifest's Name.
	Name string
	// Dialects are the core readings this language's sources select.
	Dialects []Dialect
	// Dependencies maps an ecosystem package (a gem, an npm package) to the
	// plugin runner it brings (LGP-V0-017).
	Dependencies map[string]string
}

var languages = []Language{
	{Name: "dotnet", Dialects: []Dialect{{ModuleDialect, "csharp", []string{".cs"}, NamespaceUsing}}},
	// .kts is a build or script file nothing imports, so it carries no reading.
	{Name: "kotlin", Dialects: []Dialect{{ModuleDialect, "kotlin", []string{".kt"}, PackageImports}}},
	{
		Name:         "ruby",
		Dialects:     []Dialect{{ImpactDialect, "ruby", []string{".rb"}, ConstantReferences}, {RegionDialect, "ruby", []string{".rb", ".rake"}, ""}},
		Dependencies: map[string]string{"rspec": "rspec", "rspec-core": "rspec", "rspec-rails": "rspec", "minitest": "minitest"},
	},
	{Name: "swift", Dialects: []Dialect{{ModuleDialect, "swift", []string{".swift"}, TargetModules}}},
	{
		Name: "typescript",
		Dialects: []Dialect{
			{ImpactDialect, "javascript", []string{".js", ".jsx", ".mjs", ".cjs"}, ModuleImports},
			{ImpactDialect, "typescript", []string{".ts", ".tsx"}, ModuleImports},
			{RegionDialect, "web", []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}, ""},
		},
		Dependencies: map[string]string{"jest": "jest", "vitest": "vitest", "@playwright/test": "playwright", "mocha": "mocha",
			"cypress": "cypress", "jasmine": "jasmine", "ava": "ava", "testcafe": "testcafe",
			"@storybook/test-runner": "storybook-test-runner", "@wdio/cli": "webdriverio"},
	},
}

// pluginDirs lists every registered plugin, in registration order, with its
// directories: its owned paths and every sub-plugin path, sorted. It repeats
// langplugin.Dirs so the per-plugin test entry point reads it without
// compiling any provider (LGP-V0-021); TestDeclDirsMatchManifests_LGPV0021
// keeps the two equal.
var pluginDirs = []struct {
	name string
	dirs []string
}{
	{"cfamily", []string{"cmd/corvint-analyzer-c-jni", "cmd/corvint-analyzer-objective-c", "internal/analyzernativebridge", "internal/liveverify/affected/cfamily"}},
	{"dotnet", []string{"cmd/corvint-analyzer-dotnet", "internal/analyzerdotnet", "internal/liveverify/affected/dotnet"}},
	{"go", []string{"cmd/corvint-analyzer-go", "cmd/corvint-go-test-provider", "internal/analyzergo", "internal/gotestrun", "internal/linecov", "internal/liveverify/affected/golang", "internal/liveverify/godiscovery", "internal/liveverify/gorunner", "internal/liveverify/gotest", "internal/liveverify/mutate"}},
	{"java", []string{"cmd/corvint-analyzer-java", "internal/analyzernativebridge", "internal/liveverify/affected/java"}},
	{"kotlin", []string{"cmd/corvint-analyzer-kotlin-android", "internal/analyzerkotlinandroid", "internal/liveverify/affected/kotlin"}},
	{"python", []string{"cmd/corvint-analyzer-python", "internal/analyzerpython", "internal/liveverify/affected/python", "internal/liveverify/pymutate", "internal/liveverify/pyresolve", "internal/pythongrammar", "internal/pythonsyntax"}},
	{"ruby", []string{"cmd/corvint-analyzer-ruby", "cmd/corvint-ruby-test-provider", "internal/analyzerruby", "internal/extmutate", "internal/liveverify/affected/ruby", "internal/liveverify/rbmutate", "internal/rubyautoload", "internal/rubydialect", "internal/rubylinecov", "internal/rubyprofile", "internal/rubytestprovider"}},
	{"rust", []string{"cmd/corvint-analyzer-rust", "internal/analyzerrust", "internal/liveverify/affected/rust"}},
	{"shell", []string{"cmd/corvint-analyzer-shell", "internal/analyzershell", "internal/liveverify/affected/shell"}},
	{"swift", []string{"cmd/corvint-analyzer-swift", "internal/analyzerswift", "internal/liveverify/affected/swift", "internal/swiftimpact"}},
	{"typescript", []string{"cmd/corvint-analyzer-js", "cmd/corvint-js-test-provider", "internal/analyzerjs", "internal/browsercov", "internal/extmutate", "internal/jslinecov", "internal/jstestprovider", "internal/linecov", "internal/linecov/v8", "internal/liveverify/affected/typescript", "internal/liveverify/jsresolve"}},
	{"sql", []string{"cmd/corvint-analyzer-sql-native"}},
	{"shader", []string{"cmd/corvint-analyzer-shader", "internal/analyzershader"}},
	{"html-css", []string{"cmd/corvint-analyzer-html-css", "internal/analyzerhtmlcss"}},
	{"structured-data", []string{"cmd/corvint-analyzer-structured-data", "internal/analyzerstructured"}},
}

// Names returns every registered plugin name, in registration order.
func Names() []string {
	out := make([]string, 0, len(pluginDirs))
	for _, p := range pluginDirs {
		out = append(out, p.name)
	}
	return out
}

// Dirs returns the directories of the plugin named name, and false for an
// unknown name (LGP-V0-020).
func Dirs(name string) ([]string, bool) {
	for _, p := range pluginDirs {
		if p.name == name {
			return append([]string(nil), p.dirs...), true
		}
	}
	return nil, false
}

// Languages returns every declaration, in registration order.
func Languages() []Language { return append([]Language(nil), languages...) }

// Of returns the declaration of the plugin named name; a plugin with no
// dialect or dependency has the zero declaration.
func Of(name string) Language {
	for _, l := range languages {
		if l.Name == name {
			return l
		}
	}
	return Language{}
}

// DialectFor returns the dialect of kind whose suffixes include path's
// extension, or "" when no plugin declares one (LGP-V0-015).
func DialectFor(kind DialectKind, name string) string {
	ext := path.Ext(name)
	for _, l := range languages {
		for _, d := range l.Dialects {
			if d.Kind == kind && contains(d.Suffixes, ext) {
				return d.Name
			}
		}
	}
	return ""
}

// RuleFor returns the rule of the dialect of kind whose suffixes include
// name's extension, or "" when no plugin declares one (LGP-V0-024).
func RuleFor(kind DialectKind, name string) Rule {
	ext := path.Ext(name)
	for _, l := range languages {
		for _, d := range l.Dialects {
			if d.Kind == kind && contains(d.Suffixes, ext) {
				return d.Rule
			}
		}
	}
	return ""
}

// RuleOf returns the rule of the dialect of kind named dialect, or "".
func RuleOf(kind DialectKind, dialect string) Rule {
	for _, l := range languages {
		for _, d := range l.Dialects {
			if d.Kind == kind && d.Name == dialect {
				return d.Rule
			}
		}
	}
	return ""
}

// Dialects returns every declared dialect name of kind, sorted.
func Dialects(kind DialectKind) []string {
	var out []string
	for _, l := range languages {
		for _, d := range l.Dialects {
			if d.Kind == kind && !contains(out, d.Name) {
				out = append(out, d.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// DependencyRunners returns a copy of language's package-to-runner map
// (LGP-V0-017).
func DependencyRunners(language string) map[string]string {
	deps := Of(language).Dependencies
	out := make(map[string]string, len(deps))
	for pkg, runner := range deps {
		out[pkg] = runner
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
