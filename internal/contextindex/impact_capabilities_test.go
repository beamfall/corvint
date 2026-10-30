package contextindex

import (
	"sort"
	"strings"
	"testing"
)

// TestGPKV0085ImpactCapabilitiesAgreeWithAdmission proves the listing and the
// admission/rule predicates read the same tables: every admitted suffix is
// listed exactly once, every listed suffix is admitted, and a listed rule is
// one ImpactRuleNamed applies.
func TestGPKV0085ImpactCapabilitiesAgreeWithAdmission(t *testing.T) {
	listing := ImpactCapabilities(10)
	suffixes := listing["suffixes"].(map[string]any)
	ruled := suffixes["withReverseImportRule"].([]string)
	unruled := suffixes["admittedWithoutRule"].([]string)
	seen := map[string]int{}
	for _, suffix := range ruled {
		seen[suffix]++
		if !ImpactPathAdmitted("a"+suffix) || !ImpactRuleNamed("a"+suffix) {
			t.Fatalf("%s listed with a rule but admission=%v rule=%v", suffix, ImpactPathAdmitted("a"+suffix), ImpactRuleNamed("a"+suffix))
		}
	}
	for _, suffix := range unruled {
		seen[suffix]++
		if !ImpactPathAdmitted("a"+suffix) || ImpactRuleNamed("a"+suffix) {
			t.Fatalf("%s listed without a rule but admission=%v rule=%v", suffix, ImpactPathAdmitted("a"+suffix), ImpactRuleNamed("a"+suffix))
		}
	}
	for suffix := range textSuffixes {
		if seen[suffix] != 1 {
			t.Fatalf("admitted suffix %s listed %d times", suffix, seen[suffix])
		}
	}
	if len(seen) != len(textSuffixes) {
		t.Fatalf("listing names %d suffixes, admission table has %d", len(seen), len(textSuffixes))
	}
	for _, suffix := range []string{".exe", ".groovy", ".csv", ".png"} {
		if ImpactPathAdmitted("a"+suffix) || seen[suffix] != 0 {
			t.Fatalf("unadmitted suffix %s is admitted or listed", suffix)
		}
	}
	if !sort.StringsAreSorted(ruled) || !sort.StringsAreSorted(unruled) {
		t.Fatal("listing order is not sorted")
	}
	for _, suffix := range suffixes["ruleCaseSensitive"].([]string) {
		if ImpactRuleNamed("a" + strings.ToUpper(suffix)) {
			t.Fatalf("%s listed case-sensitive but the upper-case rule applies", suffix)
		}
	}
	for _, suffix := range ruled {
		if !ImpactPathAdmitted("a" + strings.ToUpper(suffix)) {
			t.Fatalf("admission of %s is not case-insensitive", suffix)
		}
	}
	limits := listing["limits"].(map[string]any)
	if limits["pathsMax"] != maxImpactPaths || limits["pathCharsMax"] != maxPathChars || limits["limitMax"] != maxLimit || limits["limitDefault"] != 10 || limits["limitMin"] != 1 || limits["pathsMin"] != 1 {
		t.Fatalf("limits disagree with the admission bounds: %v", limits)
	}
}
