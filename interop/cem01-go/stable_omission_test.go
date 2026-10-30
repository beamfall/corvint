package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestCEMV1633PortableReaderVerdictIgnoresReceiptOutcomeAndSelection is the
// portable side of CEM-V1-633: a map citing a receipt that records a FAILED
// test, a map whose plan selects fewer tests than the target defines, and a
// map from which a failed run was left out whole each ACCEPT with the receipt
// semantics, inventory and adequacy axes NOT_OBSERVED and reference integrity
// only, and each partial omission is refused at the stage and code
// EXPECTED.json gives, which the native reader asserts over the same files.
func TestCEMV1633PortableReaderVerdictIgnoresReceiptOutcomeAndSelection(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "omission"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Repository         string   `json:"repository"`
		BaseRevision       string   `json:"baseRevision"`
		TargetRevision     string   `json:"targetRevision"`
		NotObservedAxes    []string `json:"notObservedAxes"`
		ReferenceIntegrity string   `json:"referenceIntegrity"`
		Cases              []struct {
			Name, Case, Map, ArtifactRoot, Outcome, Stage, Code string
		} `json:"cases"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range want.Cases {
		seen[c.Case] = true
	}
	if !seen["a"] || !seen["b"] || !seen["c"] || len(want.NotObservedAxes) == 0 {
		t.Fatalf("omission case lists cases %v and axes %v, want a, b and c", seen, want.NotObservedAxes)
	}
	repo, base, target := neutralRepository(t, filepath.Join(dir, want.Repository))
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	for _, tc := range want.Cases {
		mapBytes, err := os.ReadFile(filepath.Join(dir, tc.Map))
		if err != nil {
			t.Fatal(err)
		}
		r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, filepath.Join(dir, tc.ArtifactRoot), nil)
		got := ""
		if r.Code != nil {
			got = *r.Code
		}
		b, _ := json.Marshal(r)
		if r.Outcome != tc.Outcome || r.Stage != tc.Stage || (tc.Outcome == "ACCEPT") != (exit == 0) || tc.Code != "" && got != tc.Code && !slices.Contains(r.IssueCodes, tc.Code) {
			t.Fatalf("%s: exit=%d result=%s, want %s at %s with %q", tc.Name, exit, b, tc.Outcome, tc.Stage, tc.Code)
		}
		if tc.Outcome != "ACCEPT" {
			continue
		}
		if r.VerificationMode != "canonical-and-reference-integrity" || r.Axes["referenceIntegrity"] != want.ReferenceIntegrity {
			t.Fatalf("%s: an ACCEPT must be scoped to reference integrity: %s", tc.Name, b)
		}
		for _, axis := range want.NotObservedAxes {
			if r.Axes[axis] != "NOT_OBSERVED" {
				t.Fatalf("%s: axis %s = %q, want NOT_OBSERVED: %s", tc.Name, axis, r.Axes[axis], b)
			}
		}
	}
}
