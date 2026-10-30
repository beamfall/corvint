package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// TestCEMV1260DriftResourceLimitNative runs the shared drift-resource-limit
// case (CEM-V1-260) through the native verifier: the Git operation limit,
// lowered to the case's operations value through the test seam, stops the
// drift stage part way, and the UNSUPPORTED result keeps exactly the rows the
// pinned per-record drift charge completes. interop/cem01-go asserts the same
// files.
func TestCEMV1260DriftResourceLimitNative(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	top := repositoryRoot(t)
	checkStableCaseManifest(t, top, "drift-resource-limit/")
	root := filepath.Join(top, "protocol/cem-1.0/stable/drift-resource-limit")
	want := readDriftResourceLimitExpected(t, root)
	c := &neutralCase{root: root}
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	raw, err := os.ReadFile(filepath.Join(root, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, exit := Stable(context.Background(), raw, StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: root, seam: &stableSeam{operations: want.Operations}})
	if exit != want.Exit || result.Outcome != want.Outcome || result.Stage != want.Stage || result.Code == nil || *result.Code != want.Code || len(result.Drift) != want.DriftItems {
		t.Fatalf("Stable exit=%d drift=%d result=%s, want exit %d %s/%s %s with %d drift rows", exit, len(result.Drift), stableResultDebug(result), want.Exit, want.Outcome, want.Stage, want.Code, want.DriftItems)
	}
	checkResultBytes(t, filepath.Join(root, want.Result), result)
	// OCM-V0-026: the unencoded cause names the bound that fired and its
	// value, while the pinned result bytes above stay unchanged.
	if cause := fmt.Sprintf("git-budget-exceeded: Git operation budget exhausted at its %d-operation bound", want.Operations); result.LimitCause != cause {
		t.Fatalf("LimitCause=%q, want %q", result.LimitCause, cause)
	}
}

type driftResourceLimitExpected struct {
	Profile        string `json:"profile"`
	BaseRevision   string `json:"baseRevision"`
	TargetRevision string `json:"targetRevision"`
	Operations     int    `json:"operations"`
	Exit           int    `json:"exit"`
	Outcome        string `json:"outcome"`
	Stage          string `json:"stage"`
	Code           string `json:"code"`
	DriftItems     int    `json:"driftItems"`
	VerdictSha256  string `json:"verdictSha256"`
	Result         string `json:"result"`
}

// readDriftResourceLimitExpected reads EXPECTED.json and checks that its
// verdict digest and drift count are the ones result.json pins.
func readDriftResourceLimitExpected(t *testing.T, root string) driftResourceLimitExpected {
	t.Helper()
	var want driftResourceLimitExpected
	data, err := os.ReadFile(filepath.Join(root, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-drift-resource-limit/1" || want.Result != "result.json" || want.Operations <= 0 || want.Operations >= gitrun.DefaultOperations {
		t.Fatalf("EXPECTED.json profile %q result %q operations %d", want.Profile, want.Result, want.Operations)
	}
	var pinned struct {
		Drift         []json.RawMessage `json:"drift"`
		VerdictSha256 string            `json:"verdictSha256"`
	}
	data, err = os.ReadFile(filepath.Join(root, want.Result))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &pinned); err != nil {
		t.Fatal(err)
	}
	if len(pinned.Drift) != want.DriftItems || pinned.VerdictSha256 != want.VerdictSha256 {
		t.Fatalf("result.json drift %d verdict %s, EXPECTED.json %d %s", len(pinned.Drift), pinned.VerdictSha256, want.DriftItems, want.VerdictSha256)
	}
	return want
}
