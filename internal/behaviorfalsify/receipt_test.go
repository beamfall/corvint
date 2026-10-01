package behaviorfalsify

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func receiptTool() ToolIdentity {
	return ToolIdentity{Name: "corvint-behavior-falsify", Version: "test", Revision: strings.Repeat("a", 40), Executable: digestN("fixture-tool")}
}

func receiptFixture(t *testing.T, scenario string) (Plan, EvidenceReceipt) {
	t.Helper()
	t.Setenv(helperEnvironment, "1")
	plan, err := BuildPlan(liveRequest(t, []ControlSpec{liveControl(t, "control", scenario)}))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
	if err != nil {
		t.Fatal(err)
	}
	return plan, r
}

func TestBBFV0013OfflineReceipt(t *testing.T) {
	t.Run("BBF-V0-013 retained bytes verify after workspace removal", func(t *testing.T) {
		plan, r := receiptFixture(t, "killed")
		if r.Report.Counts[StatusKilled] != 1 || len(r.RawAttempts) != 1 {
			t.Fatalf("unexpected result: %+v", r.Report)
		}
		if err := os.RemoveAll(plan.Request.DisposableRoot); err != nil {
			t.Fatal(err)
		}
		if err := VerifyReceipt(r, plan.Digest, receiptTool().Executable); err != nil {
			t.Fatal(err)
		}
		if digestBytes(r.PlanPreimage) != plan.Digest || digestBytes(r.RawAttempts[0].HookBytes) != r.Report.Results[0].Attempts[0].HookProcess.StdoutSHA256 || digestBytes(r.RawAttempts[0].NativeBytes) != r.Report.Results[0].Attempts[0].Receipt.NativeReceiptSHA256 {
			t.Fatal("retained bytes do not reproduce independent digests")
		}
		one, err := EncodeEvidence(r)
		if err != nil {
			t.Fatal(err)
		}
		two, err := EncodeEvidence(r)
		if err != nil || !bytes.Equal(one, two) {
			t.Fatal("receipt encoding is not deterministic")
		}
	})
}

func TestBBFV0013ReceiptTampering(t *testing.T) {
	plan, original := receiptFixture(t, "killed")
	raw, err := EncodeEvidence(original)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*EvidenceReceipt)
	}{
		{"plan", func(r *EvidenceReceipt) { r.PlanPreimage[0] = '[' }},
		{"approval", func(r *EvidenceReceipt) {
			r.Approval.PlanDigest = digestN("other")
			r.Approval.Digest = approvalDigest(r.Approval)
		}},
		{"missing raw", func(r *EvidenceReceipt) { r.RawAttempts = nil }},
		{"hook bytes", func(r *EvidenceReceipt) { r.RawAttempts[0].HookBytes[0] = '[' }},
		{"native bytes", func(r *EvidenceReceipt) { r.RawAttempts[0].NativeBytes[0] = '[' }},
		{"native omission", func(r *EvidenceReceipt) { r.RawAttempts[0].NativeBytes = nil }},
		{"outcomes", func(r *EvidenceReceipt) {
			r.Report.Results[0].Attempts[0].Receipt.TargetObservation.AssertionID = "other"
		}},
		{"missing outcome", func(r *EvidenceReceipt) { r.Report.Results[0].Attempts[0].Receipt = nil }},
		{"duplicate attempt", func(r *EvidenceReceipt) { r.RawAttempts = append(r.RawAttempts, r.RawAttempts[0]) }},
		{"attempt ordinal", func(r *EvidenceReceipt) { r.Report.Results[0].Attempts[0].Attempt++ }},
		{"control ordinal", func(r *EvidenceReceipt) { r.Report.Results[0].Ordinal++ }},
		{"stale tool", func(r *EvidenceReceipt) { r.Tool.Executable = digestN("other") }},
		{"timeout", func(r *EvidenceReceipt) { r.Report.Results[0].Attempts[0].HookProcess.TimedOut = true }},
		{"cleanup", func(r *EvidenceReceipt) { r.Report.Results[0].Attempts[0].CleanupProcess.Exit = 9 }},
		{"workspace", func(r *EvidenceReceipt) { r.Report.Results[0].Attempts[0].WorkspaceAfter = digestN("other") }},
		{"count", func(r *EvidenceReceipt) { r.Report.Counts[StatusKilled]++ }},
		{"missing result", func(r *EvidenceReceipt) { r.Report.Results = nil }},
	}
	for _, tt := range tests {
		t.Run("BBF-V0-013 rejects "+tt.name, func(t *testing.T) {
			var r EvidenceReceipt
			if err := Decode(raw, &r); err != nil {
				t.Fatal(err)
			}
			tt.mutate(&r)
			// A self-consistent outer digest cannot repair contradictory bound observations.
			r.Report.Digest = digestJSON(r.Report)
			r.Digest = evidenceDigest(r)
			if VerifyReceipt(r, plan.Digest, receiptTool().Executable) == nil {
				t.Fatal("tampering accepted")
			}
		})
	}
}

func TestBBFV0013ReceiptFailuresNeverKill(t *testing.T) {
	for _, scenario := range []string{"unrelated-failure", "setup-failure", "wrong-assertion", "retry", "stale", "stale-artifact", "cleanup-fail", "native-missing", "crash", "timeout"} {
		t.Run("BBF-V0-013 "+scenario, func(t *testing.T) {
			_, r := receiptFixture(t, scenario)
			if r.Report.Counts[StatusKilled] != 0 || r.Report.Results[0].Status == StatusKilled {
				t.Fatal("failure became a kill")
			}
		})
	}
}

func TestBBFV0013SecretOutputRefused(t *testing.T) {
	for _, scenario := range []string{"secret-output", "malformed-secret-output", "failed-secret-output"} {
		t.Run("BBF-V0-013 "+scenario, func(t *testing.T) {
			t.Setenv(helperEnvironment, "1")
			plan, err := BuildPlan(liveRequest(t, []ControlSpec{liveControl(t, "control", scenario)}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = ExecuteReceipt(context.Background(), plan, plan.Digest, receiptTool())
			if err == nil || strings.Contains(err.Error(), "synthetic-credential") {
				t.Fatal("secret output not refused with value-free diagnostic")
			}
			if digest, err := workspaceDigest(plan.Request.DisposableRoot); err != nil || digest != plan.WorkspaceSHA256 {
				t.Fatal("secret-output refusal skipped cleanup")
			}
		})
	}
	t.Run("BBF-V0-013 escaped secret keys are screened before base64", func(t *testing.T) {
		if safeReceiptBytes([]byte(`{"pass\u0077ord":"synthetic-credential"}`)) {
			t.Fatal("escaped credential accepted")
		}
	})
}

func TestBBFV0013ReceiptApprovalAndBounds(t *testing.T) {
	plan, receipt := receiptFixture(t, "killed")
	t.Run("BBF-V0-013 approval cannot authorize another plan", func(t *testing.T) {
		if _, err := ExecuteReceipt(context.Background(), plan, digestN("other"), receiptTool()); err == nil {
			t.Fatal("wrong approval accepted")
		}
		if VerifyReceipt(receipt, digestN("other"), receiptTool().Executable) == nil {
			t.Fatal("stale plan accepted")
		}
	})
	t.Run("BBF-V0-013 encoded evidence respects document bound", func(t *testing.T) {
		receipt.RawAttempts[0].NativeBytes = bytes.Repeat([]byte("x"), maxDocumentBytes)
		if _, err := EncodeEvidence(receipt); err == nil {
			t.Fatal("oversized base64 envelope accepted")
		}
	})
}

func TestBBFV0013OversizePrecedesSecretScreen(t *testing.T) {
	for _, field := range []string{"native", "hook", "plan", "aggregate", "metadata"} {
		t.Run(field, func(t *testing.T) {
			r := EvidenceReceipt{RawAttempts: []RawAttempt{{}}}
			data := bytes.Repeat([]byte("x"), maxDocumentBytes)
			copy(data, []byte("password=hunter2 "))
			switch field {
			case "native":
				r.RawAttempts[0].NativeBytes = data
			case "hook":
				r.RawAttempts[0].HookBytes = data
			case "plan":
				r.PlanPreimage = data
			case "aggregate":
				r.RawAttempts[0].HookBytes = data[:maxDocumentBytes/2]
				r.RawAttempts[0].NativeBytes = data[:maxDocumentBytes/2]
			case "metadata":
				r.Tool.Version = string(data)
			}
			raw, err := EncodeEvidence(r)
			if len(raw) != 0 || err == nil || err.Error() != "document exceeds bound" {
				t.Fatalf("oversized secret-bearing evidence escaped size boundary: %d %v", len(raw), err)
			}
		})
	}
	small := EvidenceReceipt{RawAttempts: []RawAttempt{{NativeBytes: []byte("password=hunter2")}}}
	if raw, err := EncodeEvidence(small); len(raw) != 0 || err == nil || err.Error() != "receipt-secret-shaped-data" {
		t.Fatal("admitted-envelope secret screening weakened")
	}
}

func TestBBFV0013EncodedBoundaryAdjacent(t *testing.T) {
	t.Run("BBF-V0-013 final envelope and base64 adjacency", func(t *testing.T) {
		// A small secret refuses admitted envelopes before scanning a 32 MiB filler.
		// Size must still take precedence at the first byte beyond the actual JSON bound.
		r := EvidenceReceipt{PlanPreimage: []byte("password=hunter2")}
		empty, err := Encode(r)
		if err != nil {
			t.Fatal(err)
		}
		for _, delta := range []int{-1, 0, 1} {
			r.Tool.Version = strings.Repeat("x", maxDocumentBytes-len(empty)+delta)
			raw, err := EncodeEvidence(r)
			want := "receipt-secret-shaped-data"
			if delta > 0 {
				want = "document exceeds bound"
			}
			if len(raw) != 0 || err == nil || err.Error() != want {
				t.Fatalf("encoded boundary delta=%d: %d %v", delta, len(raw), err)
			}
		}
		r.Tool.Version = ""
		for _, n := range []int{maxDocumentBytes/4*3 - 1, maxDocumentBytes / 4 * 3, maxDocumentBytes/4*3 + 1} {
			r.RawAttempts = []RawAttempt{{NativeBytes: bytes.Repeat([]byte("x"), n)}}
			// Base64 plus the non-empty JSON envelope exceeds the bound in all three cases.
			if base64.StdEncoding.EncodedLen(n) < maxDocumentBytes-4 {
				t.Fatal("fixture is not boundary adjacent")
			}
			raw, err := EncodeEvidence(r)
			if len(raw) != 0 || err == nil || err.Error() != "document exceeds bound" {
				t.Fatalf("base64 boundary n=%d: %d %v", n, len(raw), err)
			}
		}
	})
}
