//go:build darwin || linux

package verify

import (
	"path/filepath"
	"testing"
)

// TestCEMV1844TasksBoundLiveResultValidates runs the sealed Tasks-bound
// ledger-336-evidence case through the native verifier and validates its
// complete live ACCEPT result against both committed RESULT.schema.json copies
// as they are, so the Tasks binding stays admitted beside the CEM-V1-881 plain
// form on a result the verifier wrote, not only on the sealed expectation
// (CEM-V1-844, V1-2520).
func TestCEMV1844TasksBoundLiveResultValidates(t *testing.T) {
	t.Parallel()
	packet := loadS0EPacket(t)
	requireStableLifecyclePlatform(t)
	c := s0eBindRuntime(t, packet, packet.find(t, "ledger-336-evidence"))
	result, exit, run := runS0E(t, packet, c, s0eRecipes[c.ID])
	if !requireS0EResult(t, c, result, exit) {
		if run.diagnose != nil {
			run.diagnose()
		}
		t.Fatal("ledger-336-evidence did not reproduce its sealed result")
	}
	live := stableResultObject(t, result)
	refs, _ := live.(map[string]any)["references"].(map[string]any)
	bindings, _ := refs["criterionBindings"].([]any)
	if exit != 0 || result.Outcome != "ACCEPT" || len(bindings) == 0 {
		t.Fatalf("ledger-336-evidence exit=%d result=%s, want a Tasks-bound ACCEPT", exit, stableResultDebug(result))
	}
	top := repositoryRoot(t)
	for _, rel := range stableSchemaCopies {
		schema := loadStableSchema(t, filepath.Join(top, rel))
		if err := schemaValidate(schema, schema, live, "$"); err != nil {
			t.Fatalf("%s rejects the live Tasks-bound result: %v", rel, err)
		}
	}
}
