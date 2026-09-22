package main

import "testing"

func TestMissedEvidenceVerdictIntervalCompatibility(t *testing.T) {
	tests := []struct {
		name                       string
		delta, relative, low, high float64
		controlMiss                float64
		want                       string
	}{
		{"met", 0.1, 0.2, 0.01, 0.15, 0.5, "met: at least 20% fewer missed-evidence findings"},
		{"treatment missed more", -0.05, -0.1, -0.08, -0.01, 0.5, "not met: treatment missed more evidence than control"},
		{"below target excludes zero", 0.04, 0.05, 0.03, 0.06, 0.8, "below target but interval excludes 0"},
		{"covers zero and absolute target", 0.05, 0.1, -0.01, 0.12, 0.5, "consistent with 20% and with 0"},
		{"covers zero excludes target", 0.018, 0.02, 0, 0.06, 0.9, "consistent with 0 but not with 20%"},
		{"includes both endpoints", 0.05, 0.1, 0, 0.1, 0.5, "consistent with 20% and with 0"},
		{"zero upper bound", -0.01, -0.02, -0.03, 0, 0.5, "consistent with 0 but not with 20%"},
	}
	for _, tt := range tests {
		t.Run("CRT-V0-008/"+tt.name, func(t *testing.T) {
			summary := map[string]any{"citable": map[string]any{}, "verifier": map[string]any{}}
			got := verdicts(tt.delta, tt.relative, tt.low, tt.high, tt.controlMiss, summary)["missed_evidence"]
			if got != tt.want {
				t.Fatalf("missed_evidence = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCitableAndVerifierSkipNonZeroExitLanes(t *testing.T) {
	t.Run("CRT-V0-007/non-zero exit without error text", func(t *testing.T) {
		counts := map[string]any{"total": 4.0, "mechanical": 0.0, "supported": 4.0}
		document := &report{Lanes: []*lane{
			{Arm: "treatment", ExitCode: 0, Citations: []citation{{}}, CEM: map[string]any{"counts": counts, "verify_ok": true}},
			{Arm: "treatment", ExitCode: 1, Citations: []citation{{}}, CEM: map[string]any{"counts": map[string]any{"total": 4.0, "mechanical": 0.0, "supported": 0.0}, "verify_ok": false, "replay_ok": true}},
		}}
		if got := citable(document); got["lanes"] != 1 || got["fraction"] != 1.0 {
			t.Fatalf("citable = %v, want the exit-1 lane dropped", got)
		}
		if got := verifierRates(document); got["citing_lanes"] != 1 || got["incorrect_hard_failures"] != 0 {
			t.Fatalf("verifier = %v, want the exit-1 lane dropped", got)
		}
	})
}

func TestSinglePairLeavesTheIntervalUnestimated(t *testing.T) {
	t.Run("CRT-V0-008/one scored pair", func(t *testing.T) {
		document := syntheticReport()
		document.Lanes = document.Lanes[:2]
		document.Changes = document.Changes[:1]
		finalize(document)
		if got := document.Summary["delta_ci95"]; got != notObserved {
			t.Fatalf("delta_ci95 = %v, want %s", got, notObserved)
		}
		if got := document.Summary["verdict"].(map[string]any)["missed_evidence"]; got != notEstimable {
			t.Fatalf("missed_evidence = %q, want %q", got, notEstimable)
		}
	})
}
