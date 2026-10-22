package behaviorfalsify

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// infraReceiptFixture drives one control through the real ExecuteReceipt producer. A one-second
// hook budget makes the "timeout" scenario a genuine hook timeout; stagingUnavailable removes the
// staging directory after planning so the hook never starts.
func infraReceiptFixture(t *testing.T, scenario string, timeoutSeconds int, stagingUnavailable bool) (Plan, EvidenceReceipt) {
	t.Helper()
	request := liveRequest(t, []ControlSpec{liveControl(t, "control", scenario)})
	request.TimeoutSeconds = timeoutSeconds
	request.WallClockSeconds = 20
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	if stagingUnavailable {
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	}
	r, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
	if err != nil {
		t.Fatal(err)
	}
	return plan, r
}

func resealReceipt(r *EvidenceReceipt, plan Plan) {
	for i := range r.Report.Results {
		result := &r.Report.Results[i]
		result.Status, result.Reasons = aggregateAttempts(result.Attempts, plan.Request.Attempts)
	}
	recountReport(&r.Report, plan)
	r.Report.Digest = digestJSON(r.Report)
	r.Digest = evidenceDigest(*r)
}

func TestBBFV0018InfrastructureFailedAttemptVerifies(t *testing.T) {
	tests := []struct {
		name, scenario     string
		timeout            int
		stagingUnavailable bool
		status             Status
		stdout             []byte
	}{
		{"hook timeout", "timeout", 1, false, StatusInfrastructureFailed, nil},
		{"hook crash after partial stdout", "partial-crash", 5, false, StatusInfrastructureFailed, []byte("partial hook output\n")},
		{"undecodable stdout", "timeout", 5, false, StatusInvalidControl, nil},
		{"command staging unavailable", "crash", 5, true, StatusInfrastructureFailed, nil},
	}
	for _, tt := range tests {
		t.Run("BBF-V0-018 "+tt.name, func(t *testing.T) {
			plan, r := infraReceiptFixture(t, tt.scenario, tt.timeout, tt.stagingUnavailable)
			a := r.Report.Results[0].Attempts[0]
			if a.Receipt != nil || a.Status != tt.status || r.Report.Results[0].Status != tt.status || r.Report.Counts[StatusKilled] != 0 || r.Report.Counts[StatusSurvived] != 0 {
				t.Fatalf("unexpected producer outcome: %+v", r.Report.Results[0])
			}
			if tt.stagingUnavailable {
				if a.HookProcess.StdoutSHA256 != "" || len(r.RawAttempts) != 0 {
					t.Fatalf("unobserved stdout retained: %+v", r.RawAttempts)
				}
			} else {
				if len(r.RawAttempts) != 1 {
					t.Fatalf("raw attempts = %d", len(r.RawAttempts))
				}
				raw := r.RawAttempts[0]
				if a.HookProcess.StdoutSHA256 == "" || raw.HookSHA256 != a.HookProcess.StdoutSHA256 || digestBytes(raw.HookBytes) != raw.HookSHA256 ||
					!bytes.Equal(raw.HookBytes, tt.stdout) || raw.NativeBytes != nil || raw.NativeSHA256 != "" || len(raw.Omissions) != 0 {
					t.Fatalf("raw stdout not retained and bound: %+v", raw)
				}
			}
			if err := os.RemoveAll(plan.Request.DisposableRoot); err != nil {
				t.Fatal(err)
			}
			if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
				t.Fatalf("well-formed infrastructure-failed receipt refused: %v", err)
			}
		})
	}
}

func TestBBFV0018ForgedNilReceiptAttempts(t *testing.T) {
	t.Parallel()
	plan, original := infraReceiptFixture(t, "partial-crash", 5, false)
	_, killed := receiptFixture(t, "killed")
	encoded, err := EncodeEvidence(original)
	if err != nil {
		t.Fatal(err)
	}
	succeeded := killed.Report.Results[0].Attempts[0].HookProcess
	admitted := killed.RawAttempts[0].HookBytes
	tests := []struct {
		name   string
		mutate func(*EvidenceReceipt)
	}{
		{"forged kill", func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			a.Status, a.Reasons = StatusKilled, []string{"expected-assertion-failed"}
		}},
		{"forged survivor", func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			a.Status, a.Reasons = StatusSurvived, []string{"target-criterion-survived"}
		}},
		{"forged kill with successful process", func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			process := succeeded
			process.StdoutSHA256 = a.HookProcess.StdoutSHA256
			a.HookProcess = process
			a.Status, a.Reasons = StatusKilled, []string{"expected-assertion-failed"}
		}},
		{"tampered stdout byte", func(r *EvidenceReceipt) { r.RawAttempts[0].HookBytes[0] ^= 1 }},
		{"tampered stdout byte with raw digest", func(r *EvidenceReceipt) {
			r.RawAttempts[0].HookBytes[0] ^= 1
			r.RawAttempts[0].HookSHA256 = digestBytes(r.RawAttempts[0].HookBytes)
		}},
		{"hidden admitted receipt", func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			r.RawAttempts[0].HookBytes = bytes.Clone(admitted)
			r.RawAttempts[0].HookSHA256 = digestBytes(admitted)
			a.HookProcess = succeeded
			a.Status, a.Reasons = StatusInvalidControl, []string{"hook-output-invalid"}
		}},
		{"native report retained", func(r *EvidenceReceipt) {
			r.RawAttempts[0].NativeBytes = []byte(`{"trace":"synthetic"}`)
			r.RawAttempts[0].NativeSHA256 = digestBytes(r.RawAttempts[0].NativeBytes)
		}},
		{"retained artifacts", func(r *EvidenceReceipt) {
			r.Report.Results[0].Attempts[0].RetainedArtifacts = []Artifact{{Path: "artifact.json", SHA256: digestN("artifact")}}
		}},
		{"injected reason", func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			a.Reasons = append(a.Reasons, "expected-assertion-failed")
		}},
		{"workspace baseline", func(r *EvidenceReceipt) { r.Report.Results[0].Attempts[0].WorkspaceBefore = digestN("other") }},
		{"omission", func(r *EvidenceReceipt) { r.RawAttempts[0].Omissions = []string{"retained-byte-bound"} }},
		{"application", func(r *EvidenceReceipt) { r.Report.Results[0].Attempts[0].Application = ApplicationAppliedFailed }},
	}
	t.Run("BBF-V0-018 resealed original verifies", func(t *testing.T) {
		var r EvidenceReceipt
		if err := Decode(encoded, &r); err != nil {
			t.Fatal(err)
		}
		resealReceipt(&r, plan)
		if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
			t.Fatal(err)
		}
	})
	for _, tt := range tests {
		t.Run("BBF-V0-018 rejects "+tt.name, func(t *testing.T) {
			var r EvidenceReceipt
			if err := Decode(encoded, &r); err != nil {
				t.Fatal(err)
			}
			tt.mutate(&r)
			// Self-consistent aggregates and digests cannot repair a contradictory attempt.
			resealReceipt(&r, plan)
			if VerifyReceipt(r, plan.Digest, receiptTool().Executable) == nil {
				t.Fatal("forged nil-receipt attempt accepted")
			}
		})
	}
}

func TestBBFV0018ForgedUnobservedAndHiddenReceipts(t *testing.T) {
	killedPlan, killed := receiptFixture(t, "killed")
	if killed.Report.Results[0].Status != StatusKilled {
		t.Fatalf("killed fixture unavailable: %+v", killed.Report.Results[0])
	}
	// The staging fixture's TMPDIR override lasts for the rest of this test, so it runs last.
	plan, staged := infraReceiptFixture(t, "crash", 5, true)
	tests := []struct {
		name    string
		plan    Plan
		receipt EvidenceReceipt
		mutate  func(*EvidenceReceipt)
	}{
		{"unobserved attempt as not_supported", plan, staged, func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			a.Status, a.Reasons = StatusNotSupported, []string{"caller-hook-not-supported"}
		}},
		{"unobserved attempt with unknown status", plan, staged, func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			a.Status = Status("pending")
		}},
		{"unobserved attempt as kill", plan, staged, func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			a.Status, a.Reasons = StatusKilled, []string{"expected-assertion-failed"}
		}},
		{"unobserved attempt with started hook", plan, staged, func(r *EvidenceReceipt) {
			r.Report.Results[0].Attempts[0].HookProcess.Started = true
		}},
		{"unobserved attempt with application", plan, staged, func(r *EvidenceReceipt) {
			r.Report.Results[0].Attempts[0].Application = ApplicationAppliedFailed
		}},
		{"admitted receipt hidden behind forged timeout", killedPlan, killed, func(r *EvidenceReceipt) {
			a := &r.Report.Results[0].Attempts[0]
			a.Receipt, a.RetainedArtifacts = nil, []Artifact{}
			a.HookProcess.TimedOut = true
			a.Status, a.Reasons = StatusInfrastructureFailed, []string{"hook-process-failed", "execution-boundary-failed"}
			r.RawAttempts[0].NativeBytes, r.RawAttempts[0].NativeSHA256 = nil, ""
		}},
	}
	for _, tt := range tests {
		t.Run("BBF-V0-018 rejects "+tt.name, func(t *testing.T) {
			encoded, err := EncodeEvidence(tt.receipt)
			if err != nil {
				t.Fatal(err)
			}
			var r EvidenceReceipt
			if err := Decode(encoded, &r); err != nil {
				t.Fatal(err)
			}
			resealReceipt(&r, tt.plan)
			if err := VerifyReceipt(r, tt.plan.Digest, receiptTool().Executable); err != nil {
				t.Fatalf("resealed original refused: %v", err)
			}
			tt.mutate(&r)
			resealReceipt(&r, tt.plan)
			if VerifyReceipt(r, tt.plan.Digest, receiptTool().Executable) == nil {
				t.Fatal("forged attempt accepted")
			}
		})
	}
}

// The executor's two pre-hook attempt shapes (executeAttempt's baseline-drift and wall-clock
// returns) observe no stdout and still verify; the staged receipt supplies the surrounding plan.
func TestBBFV0018PreHookAttemptShapesVerify(t *testing.T) {
	plan, staged := infraReceiptFixture(t, "crash", 5, true)
	shapes := map[string]AttemptResult{
		"workspace baseline drift": {Attempt: 1, Status: StatusInvalidControl, Reasons: []string{"workspace-baseline-drift"}, RetainedArtifacts: []Artifact{}},
		"wall clock exhausted":     {Attempt: 1, Status: StatusNotRun, Reasons: []string{"wall-clock-budget-exhausted"}, WorkspaceBefore: plan.WorkspaceSHA256, RetainedArtifacts: []Artifact{}},
	}
	for name, shape := range shapes {
		t.Run("BBF-V0-018 "+name, func(t *testing.T) {
			r := staged
			r.Report.Results = slices.Clone(staged.Report.Results)
			r.Report.Results[0].Attempts = []AttemptResult{shape}
			r.RawAttempts = nil
			resealReceipt(&r, plan)
			if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
				t.Fatalf("pre-hook attempt refused: %v", err)
			}
		})
	}
}
