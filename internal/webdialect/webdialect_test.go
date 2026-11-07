// SPDX-License-Identifier: Apache-2.0

package webdialect

import (
	"slices"
	"strings"
	"testing"
)

// AFP-V0-2247 (LGP-V0-027 follow-up): the TypeScript plugin's impact profile keeps the probes,
// the load pattern and the static-load rule core registered before the move, and masks with the
// code-lines function core passes in.
func TestImpactProfileIsUnchanged_AFPV02247(t *testing.T) {
	masked := func(text string) []string { return strings.Split(strings.ToUpper(text), "\n") }
	profile := Impact(masked)
	if got := profile.CodeLines("a\nb"); !slices.Equal(got, []string{"A", "B"}) {
		t.Fatalf("CodeLines = %v, want the core mask", got)
	}
	var codes []string
	for _, probe := range profile.Probes {
		codes = append(codes, probe.Code)
	}
	if !slices.Equal(codes, []string{"observed-reflective-dispatch", "observed-computed-dispatch"}) {
		t.Fatalf("probe codes = %v", codes)
	}
	if !profile.Probes[1].Pattern.MatchString("handlers[name] ?.(x)") || profile.Probes[1].Pattern.MatchString("handlers.name(x)") {
		t.Fatal("the computed-dispatch probe changed")
	}
	if !profile.Loads.MatchString("await import (x)") || !profile.Loads.MatchString("require(x)") || profile.Loads.MatchString("imports(x)") {
		t.Fatal("the load pattern changed")
	}
	for tail, want := range map[string]bool{`'./a.js')`: true, ` "./b" ) ;`: true, `'./a' + name)`: false, `name)`: false, `'unterminated`: false, `'a\'b')`: true} {
		if got := StaticLoad(tail); got != want {
			t.Errorf("StaticLoad(%q) = %v, want %v", tail, got, want)
		}
	}
}
