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

// multiRunnerExpected is the EXPECTED.json of the two-profile case shared with
// the portable reader.
type multiRunnerExpected struct {
	BaseRevision   string `json:"baseRevision"`
	TargetRevision string `json:"targetRevision"`
	Accept         struct {
		Map, Outcome, Stage string
	} `json:"accept"`
	Refusals []struct {
		Name, Map, Outcome, Stage, Code string
	} `json:"refusals"`
}

// TestCEMV1577NativeReaderChecksTwoProfileMap: the native stable verifier
// ACCEPTs the two-profile map (a go-test and a node-test receipt) and refuses
// each tampered variant at its expected stage and code; the portable reader
// asserts the same over the same files (CEM-V1-577).
func TestCEMV1577NativeReaderChecksTwoProfileMap(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	c := &neutralCase{root: filepath.Join(repositoryRoot(t), "interop/cem01-go/testdata/multi-runner")}
	var want multiRunnerExpected
	data, err := os.ReadFile(filepath.Join(c.root, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(want.Refusals) == 0 {
		t.Fatal("two-profile case lists no tampered variants")
	}
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	check := func(name, path, outcome, stage, code string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(c.root, path))
		if err != nil {
			t.Fatal(err)
		}
		r, exit := Stable(context.Background(), raw, StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root})
		got := ""
		if r.Code != nil {
			got = *r.Code
		}
		if r.Outcome != outcome || r.Stage != stage || (outcome == "ACCEPT") != (exit == 0) || code != "" && got != code && !slices.Contains(r.IssueCodes, code) {
			b, _ := json.Marshal(r)
			t.Fatalf("%s: exit=%d result=%s, want %s at %s with %q", name, exit, b, outcome, stage, code)
		}
	}
	check("accept", want.Accept.Map, want.Accept.Outcome, want.Accept.Stage, "")
	for _, refusal := range want.Refusals {
		check(refusal.Name, refusal.Map, refusal.Outcome, refusal.Stage, refusal.Code)
	}
}
