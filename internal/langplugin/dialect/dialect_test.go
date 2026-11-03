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
			impact := complete()
			mutate(&impact)
			defer func() {
				got, _ := recover().(string)
				if !strings.Contains(got, want) {
					t.Fatalf("panic = %q, want it to name %q", got, want)
				}
				if _, ok := LookupImpact("incomplete-test"); ok {
					t.Fatal("an incomplete profile was registered")
				}
			}()
			RegisterImpact("incomplete-test", impact)
		})
	}
	defer func() {
		if got, _ := recover().(string); !strings.Contains(got, "lacks Normalize") {
			t.Fatalf("panic = %q, want lacks Normalize", got)
		}
	}()
	RegisterRegion("incomplete-test", Region{})
}
