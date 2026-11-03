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

// Dialect is one core reading a plugin's sources select: its kind, the
// dialect name the core reading answers, and the suffixes it reads.
type Dialect struct {
	Kind     DialectKind
	Name     string
	Suffixes []string
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
	{Name: "dotnet", Dialects: []Dialect{{ModuleDialect, "csharp", []string{".cs"}}}},
	// .kts is a build or script file nothing imports, so it carries no reading.
	{Name: "kotlin", Dialects: []Dialect{{ModuleDialect, "kotlin", []string{".kt"}}}},
	{
		Name:         "ruby",
		Dialects:     []Dialect{{ImpactDialect, "ruby", []string{".rb"}}, {RegionDialect, "ruby", []string{".rb", ".rake"}}},
		Dependencies: map[string]string{"rspec": "rspec", "rspec-core": "rspec", "rspec-rails": "rspec", "minitest": "minitest"},
	},
	{Name: "swift", Dialects: []Dialect{{ModuleDialect, "swift", []string{".swift"}}}},
	{
		Name: "typescript",
		Dialects: []Dialect{
			{ImpactDialect, "javascript", []string{".js", ".jsx", ".mjs", ".cjs"}},
			{ImpactDialect, "typescript", []string{".ts", ".tsx"}},
			{RegionDialect, "web", []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}},
		},
		Dependencies: map[string]string{"jest": "jest", "vitest": "vitest", "@playwright/test": "playwright", "mocha": "mocha",
			"cypress": "cypress", "jasmine": "jasmine", "ava": "ava", "testcafe": "testcafe",
			"@storybook/test-runner": "storybook-test-runner", "@wdio/cli": "webdriverio"},
	},
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
