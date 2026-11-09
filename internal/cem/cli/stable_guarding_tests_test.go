package cli

import (
	"reflect"
	"testing"
)

// TCQ-V0-062: `cem verify --guarding-tests` is opt-in and reports beside the
// result. The stable result stays exactly what verify-stable reports, the
// search block appears only with the flag, and status refuses it.
func TestTCQV0062VerifyGuardingTestsIsOptIn(t *testing.T) {
	f := materializeStableRead(t)
	wantExit, want := runVerifyStable(t, f, f.artifacts)
	for _, guards := range []bool{false, true} {
		arguments := coreStableArguments("verify", f, f.artifacts)
		if guards {
			arguments = append(arguments, "--guarding-tests")
		}
		exit, envelope, refusal := runCoreRead(t, f.repo, arguments...)
		if refusal != nil {
			t.Fatalf("verify --guarding-tests=%v refused: %v", guards, refusal)
		}
		if got, _ := envelope["stableVerification"].(map[string]any); exit != wantExit || !reflect.DeepEqual(got, want) {
			t.Errorf("verify --guarding-tests=%v differs from verify-stable: exit %d/%d", guards, exit, wantExit)
		}
		if _, present := envelope["guardingTests"]; present != guards {
			t.Errorf("verify --guarding-tests=%v: guardingTests present = %v", guards, present)
		}
	}
	if _, _, refusal := runCoreRead(t, f.repo, append(coreStableArguments("status", f, f.artifacts), "--guarding-tests")...); refusal == nil {
		t.Error("status accepted --guarding-tests")
	}
}
