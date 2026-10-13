package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// tamperMatrixClasses are the tamper classes the shared matrix must cover
// (CEM-V1-226); the portable consumer checks the same list.
var tamperMatrixClasses = []string{"control", "PatchDigestMismatch", "FabricatedEvidenceID", "UncitedHunk", "InvalidSpan",
	"SpanOutOfRange", "EvidenceIDSwap", "EvidenceDrop", "HunkReorder", "SpanRangeEdit"}

type tamperMatrixCase struct {
	Name       string         `json:"name"`
	Tamper     string         `json:"tamper"`
	Map        string         `json:"map"`
	Exit       int            `json:"exit"`
	Outcome    string         `json:"outcome"`
	Stage      string         `json:"stage"`
	Code       *string        `json:"code"`
	IssueCodes []string       `json:"issueCodes"`
	Citation   map[string]any `json:"citation"`
}

// TestCEMV1226TamperMatrixNative runs every case of the shared stable tamper
// matrix through the native verifier and asserts its pinned verdict: exit,
// outcome, stage, closed code and issue codes, and for an ACCEPT the whole
// citation block (CEM-V1-226..229). interop/cem01-go asserts the same file.
func TestCEMV1226TamperMatrixNative(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	top := repositoryRoot(t)
	checkStableCaseManifest(t, top, "tamper-matrix/")
	c := &neutralCase{root: filepath.Join(top, "protocol/cem-1.0/stable/tamper-matrix")}
	data, err := os.ReadFile(filepath.Join(c.root, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Profile        string             `json:"profile"`
		ArtifactRoot   string             `json:"artifactRoot"`
		BaseRevision   string             `json:"baseRevision"`
		TargetRevision string             `json:"targetRevision"`
		Cases          []tamperMatrixCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-tamper-matrix/1" || want.ArtifactRoot != "." {
		t.Fatalf("EXPECTED.json profile %q artifactRoot %q", want.Profile, want.ArtifactRoot)
	}
	covered := map[string]bool{}
	for _, tc := range want.Cases {
		covered[tc.Tamper] = true
	}
	for _, class := range tamperMatrixClasses {
		if !covered[class] {
			t.Errorf("tamper matrix has no %s case", class)
		}
	}
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: filepath.Join(c.root, want.ArtifactRoot)}
	for _, tc := range want.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(tc.Map)))
			if err != nil {
				t.Fatal(err)
			}
			result, exit := Stable(context.Background(), raw, options)
			got := neutralResult(t, result)
			var gotCitation map[string]any
			if v, ok := got["citation"].(map[string]any); ok {
				gotCitation = v
			}
			if exit != tc.Exit || result.Outcome != tc.Outcome || result.Stage != tc.Stage ||
				!reflect.DeepEqual(result.Code, tc.Code) || !reflect.DeepEqual(append([]string{}, result.IssueCodes...), append([]string{}, tc.IssueCodes...)) ||
				!reflect.DeepEqual(gotCitation, tc.Citation) {
				t.Fatalf("Stable exit=%d result=%s, want the pinned %s verdict", exit, stableResultDebug(result), tc.Name)
			}
		})
	}
}
