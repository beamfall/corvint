package appflows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// ACX-V0-856 (V1-2907): a plan whose TypeScript frontier was moved to plan.frontierExcluded
// yields an e2e-safe result naming the excluded entries that would have unbounded the closure,
// leaving out the ones the profile answers itself; a plan without exclusions omits the member.
func TestE2ESelectionNamesFrontierExclusions_ACXV0856(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "e2e-provider.json"), inventoryProvider(1, ""), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := affected.Plan{Dirty: []string{"src/a.go"}, FrontierExcluded: []affected.FrontierExclusion{{
		Language: "typescript", Reason: affected.FrontierExclusionReason,
		Frontier: []string{"typescript:dynamic-import", "typescript:e2e-runtime-dependency"},
		Units:    []string{"typescript:web/app.ts"},
	}, {
		Language: "other", Reason: affected.FrontierExclusionReason,
		Frontier: []string{"typescript:executable-config-unresolved"},
		Units:    []string{"other:x"},
	}}}
	in := E2EInput{Provider: "e2e-provider.json", Revision: "HEAD", Graph: &affected.Graph{}, Plan: plan}
	out := SelectE2E(t.Context(), root, in)
	want := []E2EFrontierExclusion{{Language: "typescript", Frontier: []string{"typescript:dynamic-import"}, Units: []string{"typescript:web/app.ts"}}}
	if !reflect.DeepEqual(out.FrontierExcluded, want) {
		t.Fatalf("frontier_excluded = %+v (state %s/%s), want %+v", out.FrontierExcluded, out.State, out.StateReason, want)
	}
	encoded, err := json.Marshal(out)
	if err != nil || !strings.Contains(string(encoded), `"frontier_excluded":[{"language":"typescript","frontier":["typescript:dynamic-import"]`) {
		t.Fatalf("encoded result does not name the exclusion: %s %v", encoded, err)
	}
	in.Plan.FrontierExcluded = nil
	if out := SelectE2E(t.Context(), root, in); out.FrontierExcluded != nil {
		t.Fatalf("plan without exclusions carried %+v", out.FrontierExcluded)
	}
	if encoded, _ := json.Marshal(SelectE2E(t.Context(), root, in)); strings.Contains(string(encoded), "frontier_excluded") {
		t.Fatalf("member not omitted: %s", encoded)
	}
}
