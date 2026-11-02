package appflows

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// AFU-V1-810 (V1-2907), served by the ACX-V0-856 frontierExclusions: the e2e-safe selection names each plan frontier exclusion it relied on,
// narrowed to the reasons that would have widened it, and omits the member otherwise.
func TestAFUV1810FrontierExclusionNamed(t *testing.T) {
	units := []string{"typescript:web/src"}
	plan := affected.Plan{Dirty: []string{"web/src/a.ts"}, FrontierExcluded: []affected.FrontierExclusion{
		{Language: "typescript", Reason: affected.FrontierExclusionReason, Units: units, Frontier: []string{
			typescript.FrontierDynamicImport,
			typescript.FrontierConfig,
			typescript.FrontierOverBoundSource + ":web/src/other.ts",
			typescript.FrontierOverBoundSource + ":web/src/a.ts",
		}},
		{Language: "golang", Reason: affected.FrontierExclusionReason, Units: []string{"go:x"}, Frontier: []string{typescript.FrontierConfig}},
	}}
	s := &e2eSelector{in: E2EInput{Plan: plan}, changed: plan.Dirty}
	want := []E2EFrontierExclusion{{Language: "typescript", Units: units,
		Frontier: []string{typescript.FrontierDynamicImport, typescript.FrontierOverBoundSource + ":web/src/a.ts"}}}
	got := s.frontierExclusions()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("relied exclusions = %+v, want %+v", got, want)
	}
	if plan.FrontierExcluded[0].Frontier[1] != typescript.FrontierConfig {
		t.Fatal("narrowing must not mutate the plan's exclusion")
	}
	data, err := json.Marshal(E2ESelection{FrontierExcluded: got})
	if err != nil || !strings.Contains(string(data), `"frontier_excluded":[{"language":"typescript"`) {
		t.Fatalf("selection JSON = %s, %v", data, err)
	}
	none := &e2eSelector{in: E2EInput{Plan: affected.Plan{Dirty: plan.Dirty}}, changed: plan.Dirty}
	if got := none.frontierExclusions(); got != nil {
		t.Fatalf("plan without exclusions: %+v", got)
	}
	data, _ = json.Marshal(E2ESelection{})
	if strings.Contains(string(data), "frontier_excluded") {
		t.Fatalf("empty member not omitted: %s", data)
	}
}
