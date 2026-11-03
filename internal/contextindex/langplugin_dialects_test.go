package contextindex

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin"
)

// TestPluginDialectsImplemented_LGPV0016 binds the language plugins' declared
// dialects to the readings this package implements: a plugin may not declare a
// dialect core lacks, and core may not implement one no plugin declares.
func TestPluginDialectsImplemented_LGPV0016(t *testing.T) {
	for _, kind := range langplugin.DialectKinds {
		declared := strings.Join(langplugin.Dialects(kind), ",")
		implemented := strings.Join(ImplementedDialects(kind), ",")
		if declared != implemented {
			t.Errorf("%s dialects: plugins declare [%s], contextindex implements [%s]", kind, declared, implemented)
		}
	}
}

// TestPluginDialectSuffixesUnchanged_LGPV0015 pins the suffix answers the
// hand-written switches gave before the plugins declared them.
func TestPluginDialectSuffixesUnchanged_LGPV0015(t *testing.T) {
	for name, want := range map[string][3]string{
		"a.rb":     {"ruby", "", "ruby"},
		"a.rake":   {"", "", "ruby"},
		"a.js":     {"javascript", "", "web"},
		"a.jsx":    {"javascript", "", "web"},
		"a.mjs":    {"javascript", "", "web"},
		"a.cjs":    {"javascript", "", "web"},
		"a.ts":     {"typescript", "", "web"},
		"a.tsx":    {"typescript", "", "web"},
		"a.swift":  {"", "swift", ""},
		"a.kt":     {"", "kotlin", ""},
		"a.cs":     {"", "csharp", ""},
		"a.go":     {"", "", ""},
		"a.py":     {"", "", ""},
		"Makefile": {"", "", ""},
	} {
		got := [3]string{NonGoImpactLanguage(name), ModuleRuleLanguage(name), SymbolRegionLanguage(name)}
		if got != want {
			t.Errorf("%s: impact/module/region = %q, want %q", name, got, want)
		}
	}
}
