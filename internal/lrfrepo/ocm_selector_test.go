package lrfrepo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func TestOCMClaimSelectorMiss(t *testing.T) {
	t.Run("OCM-V0-007 normalized hint preserves exact selection", func(t *testing.T) {
		claims := []ocmClaim{{id: "claim-one", selector: "test:TestWork/case:wqo-v0"}}
		_, err := resolveClaimSelectors(claims, []string{"test:TestWork/case:WQO-V0-032"})
		if CodeOf(err) != "claim-selector-out-of-range" || !strings.Contains(err.Error(), "normalized case fragment: wqo-v0") {
			t.Fatalf("missing normalized hint: %v", err)
		}
		got, err := resolveClaimSelectors(claims, []string{claims[0].selector})
		if err != nil || !reflect.DeepEqual(got, claims) {
			t.Fatalf("exact selection: %v %v", got, err)
		}
		claims = append(claims, ocmClaim{id: "claim-two", selector: claims[0].selector})
		if _, err := resolveClaimSelectors(claims, []string{claims[0].selector}); CodeOf(err) != "ambiguous-claim-selector" {
			t.Fatalf("normalized collision accepted: %v", err)
		}
	})
	t.Run("OCM-V0-007 hints do not promise a match", func(t *testing.T) {
		for _, suffix := range []string{"WQO-V0-032", "123", strings.Repeat("ABC-", 100)} {
			_, err := resolveClaimSelectors(nil, []string{"test:TestWork/case:" + suffix})
			want := "claim-selector-out-of-range: claim selector is outside the extracted worklist; normalized case fragment: " + selectorFragment(suffix) + goCaseAnchorShapes
			if err == nil || err.Error() != want {
				t.Fatalf("got %v want %s", err, want)
			}
			if len(err.Error()) > 320 {
				t.Fatalf("unbounded diagnostic: %d", len(err.Error()))
			}
		}
	})
	t.Run("OCM-V0-007 unrelated and normalized misses retain message", func(t *testing.T) {
		for _, selector := range []string{"1", "claim:sha256:missing", "test:TestWork"} {
			_, err := resolveClaimSelectors(nil, []string{selector})
			if err == nil || err.Error() != "claim-selector-out-of-range: claim selector is outside the extracted worklist" {
				t.Fatalf("selector %q: %v", selector, err)
			}
		}
		_, err := resolveClaimSelectors(nil, []string{"test:TestWork/case:wqo-v0"})
		if err == nil || err.Error() != "claim-selector-out-of-range: claim selector is outside the extracted worklist"+goCaseAnchorShapes {
			t.Fatalf("normalized case miss: %v", err)
		}
	})
	t.Run("OCM-V0-007 a map-keyed table case is no claim and the miss names the supported shapes", func(t *testing.T) {
		source := "package p\nimport \"testing\"\nfunc TestTable(t *testing.T) {\n\tcases := map[string]struct{ want int }{\n\t\t\"PTR-V0-003 two bound checks\": {want: 1},\n\t}\n\tfor name, tc := range cases {\n\t\tt.Run(name, func(t *testing.T) { _ = tc })\n\t}\n}\n"
		claims, err := enumerateClaims("table_test.go", strings.Repeat("a", 40), []byte(source))
		if err != nil || len(claims) != 1 || claims[0].selector != "test:TestTable" {
			t.Fatalf("map key extracted as a claim: %v %v", claims, err)
		}
		_, err = resolveClaimSelectors(claims, []string{"test:TestTable/case:ptr-v0-two-bound-check"})
		if CodeOf(err) != "claim-selector-out-of-range" || !strings.Contains(err.Error(), "name/testName/test_name field or .Run first-argument literal, not a map key") {
			t.Fatalf("miss does not name the supported shapes: %v", err)
		}
	})
}

func TestOCMFullHunkSelectors(t *testing.T) {
	t.Run("OCM-V0-006 full IDs require supported current hunks", func(t *testing.T) {
		id := "hunk:sha256:" + strings.Repeat("a", 64)
		cem := &wire.Map{Hunks: []wire.Hunk{{ID: id, Disposition: "supported"}}}
		for _, selector := range []string{"1", id} {
			got, err := resolveHunkSelectors(cem, []string{selector})
			if err != nil || !reflect.DeepEqual(got, []string{id}) {
				t.Fatalf("%q: %v %v", selector, got, err)
			}
		}
		for _, disposition := range []string{"unknown", "mechanical"} {
			cem.Hunks[0].Disposition = disposition
			if _, err := resolveHunkSelectors(cem, []string{id}); CodeOf(err) != "unsupported-hunk-selector" {
				t.Fatalf("%s: %v", disposition, err)
			}
		}
		cem.Hunks[0].Disposition = "supported"
		if _, err := resolveHunkSelectors(cem, []string{"hunk:sha256:" + strings.Repeat("b", 64)}); CodeOf(err) != "unsupported-hunk-selector" {
			t.Fatalf("absent/stale ID: %v", err)
		}
	})
}

// Regression for the 2026-09-07 "claims: []" report: extraction reads both
// selector forms; only `prepare` leaves claims empty, by design.
func TestOCMRequirementIDSelectorsProduceClaims(t *testing.T) {
	t.Run("OCM-V0-005 t.Run and test() anchors naming a requirement id extract", func(t *testing.T) {
		blobs := map[string]string{
			"framework_test.go": "package example\nimport \"testing\"\nfunc TestFramework(t *testing.T) {\n\tt.Run(\"TFC-V0-001 exact-runtime-members\", func(t *testing.T) {})\n}\n",
			"reporter.test.mjs": "import test from 'node:test';\ntest('TFC-V0-001 synthetic metadata does not change closed reporter bytes', async () => {});\n",
		}
		want := map[string]string{
			"framework_test.go": "test:TestFramework/case:tfc-v0-exact-runtime-member",
			"reporter.test.mjs": "test:tfc-v0-synthetic-metadata-does-not-change-closed-reporter-byte",
		}
		for path, source := range blobs {
			claims, err := enumerateClaims(path, strings.Repeat("a", 40), []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			selected, err := resolveClaimSelectors(claims, []string{want[path]})
			if err != nil || len(selected) != 1 {
				t.Fatalf("%s: claims=%v selected=%v err=%v", path, claims, selected, err)
			}
			anchor := source[selected[0].span.start:selected[0].span.end]
			if !containsExactRequirement([]byte(anchor), "TFC-V0-001") {
				t.Fatalf("%s: anchor %q lacks the exact obligation id", path, anchor)
			}
		}
	})
}
