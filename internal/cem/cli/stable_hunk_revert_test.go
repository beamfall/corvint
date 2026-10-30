package cli

import (
	"reflect"
	"testing"
)

// CEM-V1-088: `cem verify --hunk-revert` is opt-in and reports beside the
// result. The stable result stays exactly what verify-stable reports, the
// falsifier block appears only with the flag, and status does not accept it.
func TestCEMV1088VerifyHunkRevertIsOptIn(t *testing.T) {
	f := materializeStableRead(t)
	wantExit, want := runVerifyStable(t, f, f.artifacts)
	for _, revert := range []bool{false, true} {
		arguments := coreStableArguments("verify", f, f.artifacts)
		if revert {
			arguments = append(arguments, "--hunk-revert")
		}
		exit, envelope, refusal := runCoreRead(t, f.repo, arguments...)
		if refusal != nil {
			t.Fatalf("verify --hunk-revert=%v refused: %v", revert, refusal)
		}
		if got, _ := envelope["stableVerification"].(map[string]any); exit != wantExit || !reflect.DeepEqual(got, want) {
			t.Errorf("verify --hunk-revert=%v differs from verify-stable: exit %d/%d", revert, exit, wantExit)
		}
		if _, present := envelope["hunkRevert"]; present != revert {
			t.Errorf("verify --hunk-revert=%v: hunkRevert present = %v", revert, present)
		}
	}
	if _, _, refusal := runCoreRead(t, f.repo, append(coreStableArguments("status", f, f.artifacts), "--hunk-revert")...); refusal == nil {
		t.Error("status accepted --hunk-revert")
	}
}
