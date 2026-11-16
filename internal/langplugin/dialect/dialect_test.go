// SPDX-License-Identifier: Apache-2.0

package dialect

import (
	"regexp"
	"strings"
	"testing"
)

// TestRegisterRefusesIncompleteHooks_LGPV0023 falsifies registration: an
// impact profile missing a required function, or carrying a probe without a
// code or pattern, panics instead of failing during analysis.
func TestRegisterRefusesIncompleteHooks_LGPV0023(t *testing.T) {
	complete := func() Impact {
		return Impact{
			CodeLines:  func(string) []string { return nil },
			Loads:      regexp.MustCompile(`load`),
			StaticLoad: func(string) bool { return false },
		}
	}
	for want, mutate := range map[string]func(*Impact){
		"lacks CodeLines":              func(i *Impact) { i.CodeLines = nil },
		"probe without a code":         func(i *Impact) { i.Probes = []Probe{{Pattern: Reflection}} },
		"probe without a code or patt": func(i *Impact) { i.Probes = []Probe{{Code: "c"}} },
	} {
		t.Run(want, func(t *testing.T) {
			registry := NewRegistry()
			impact := complete()
			mutate(&impact)
			defer func() {
				got, _ := recover().(string)
				if !strings.Contains(got, want) {
					t.Fatalf("panic = %q, want it to name %q", got, want)
				}
				if _, ok := registry.LookupImpact("incomplete-test"); ok {
					t.Fatal("an incomplete profile was registered")
				}
			}()
			registry.registerImpact("incomplete-test", impact)
		})
	}
	defer func() {
		if got, _ := recover().(string); !strings.Contains(got, "lacks Normalize") {
			t.Fatalf("panic = %q, want lacks Normalize", got)
		}
	}()
	NewRegistry(Hooks{Regions: map[string]Region{"incomplete-test": {}}})
}

// LGP-V0-8600: construction order and callers cannot mutate another registry.
func TestExplicitRegistryOrderAndIsolation(t *testing.T) {
	a := Hooks{Regions: map[string]Region{"a": {Normalize: func(string) ([]Line, bool) { return nil, true }}}}
	b := Hooks{Impacts: map[string]Impact{"b": {CodeLines: func(s string) []string { return []string{s} }, Loads: Reflection, StaticLoad: func(string) bool { return true }, Probes: []Probe{{Code: "original", Pattern: Reflection}}}}}
	first, second := NewRegistry(a, b), NewRegistry(b, a)
	delete(a.Regions, "a")
	value := b.Impacts["b"]
	value.Probes[0].Code = "changed"
	for _, registry := range []Registry{first, second} {
		if _, ok := registry.LookupRegion("a"); !ok {
			t.Fatal("registry changed with source map")
		}
		profile, ok := registry.LookupImpact("b")
		if !ok || profile.Probes[0].Code != "original" {
			t.Fatal("registry changed with source probes")
		}
		profile.Probes[0].Code = "returned-value-change"
		again, _ := registry.LookupImpact("b")
		if again.Probes[0].Code != "original" {
			t.Fatal("lookup exposed mutable probes")
		}
	}
	if _, ok := NewRegistry().LookupImpact("b"); ok {
		t.Fatal("construction leaked into an empty registry")
	}
	t.Run("duplicate", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("duplicate admitted")
			}
		}()
		NewRegistry(b, b)
	})
	t.Run("empty", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("empty name admitted")
			}
		}()
		NewRegistry(Hooks{Regions: map[string]Region{"": {Normalize: func(string) ([]Line, bool) { return nil, true }}}})
	})
}
