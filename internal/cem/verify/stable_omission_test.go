package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// omissionExpected is the EXPECTED.json of the V1-2599 omission case shared
// with the portable reader.
type omissionExpected struct {
	Repository         string   `json:"repository"`
	BaseRevision       string   `json:"baseRevision"`
	TargetRevision     string   `json:"targetRevision"`
	NotObservedAxes    []string `json:"notObservedAxes"`
	ReferenceIntegrity string   `json:"referenceIntegrity"`
	Cases              []struct {
		Name, Case, Map, ArtifactRoot, Outcome, Stage, Code string
	} `json:"cases"`
}

// TestCEMV1633NativeReaderVerdictIgnoresReceiptOutcomeAndSelection: the
// native stable verifier ACCEPTs a map citing a receipt that records a FAILED
// test, a map whose plan selects fewer tests than the target defines, and a
// map from which a failed run was left out whole, each with the receipt
// semantics, inventory and adequacy axes NOT_OBSERVED and reference integrity
// only; a partial omission is refused at its reference stage and code. The
// portable reader asserts the same over the same files (CEM-V1-633).
func TestCEMV1633NativeReaderVerdictIgnoresReceiptOutcomeAndSelection(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	dir := filepath.Join(repositoryRoot(t), "interop/cem01-go/testdata/omission")
	var want omissionExpected
	data, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range want.Cases {
		seen[c.Case] = true
	}
	if !seen["a"] || !seen["b"] || !seen["c"] || len(want.NotObservedAxes) == 0 {
		t.Fatalf("omission case lists cases %v and axes %v, want a, b and c", seen, want.NotObservedAxes)
	}
	c := &neutralCase{root: filepath.Join(dir, want.Repository)}
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	for _, tc := range want.Cases {
		raw, err := os.ReadFile(filepath.Join(dir, tc.Map))
		if err != nil {
			t.Fatal(err)
		}
		r, exit := Stable(context.Background(), raw, StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: filepath.Join(dir, tc.ArtifactRoot)})
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
