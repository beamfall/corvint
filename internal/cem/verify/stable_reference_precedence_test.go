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

// TestCEMV1177ReferencePrecedenceNative runs the shared reference-precedence
// cases (CEM-V1-176, CEM-V1-177) through the native verifier against the
// neutral-criterion repository and compares each result line with its pinned
// bytes. interop/cem01-go asserts the same files.
func TestCEMV1177ReferencePrecedenceNative(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	top := repositoryRoot(t)
	checkStableCaseManifest(t, top, "reference-precedence/")
	root := filepath.Join(top, "protocol/cem-1.0/stable/reference-precedence")
	data, err := os.ReadFile(filepath.Join(root, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Profile        string `json:"profile"`
		Repository     string `json:"repository"`
		ArtifactRoot   string `json:"artifactRoot"`
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Cases          []struct {
			Name    string  `json:"name"`
			Map     string  `json:"map"`
			Result  string  `json:"result"`
			Exit    int     `json:"exit"`
			Outcome string  `json:"outcome"`
			Stage   string  `json:"stage"`
			Code    *string `json:"code"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-reference-precedence/1" || want.Repository != "../neutral-criterion/repository.recipe.json" || want.ArtifactRoot != "../neutral-criterion" || len(want.Cases) != 6 {
		t.Fatalf("EXPECTED.json profile %q repository %q artifactRoot %q cases %d", want.Profile, want.Repository, want.ArtifactRoot, len(want.Cases))
	}
	c := &neutralCase{root: filepath.Join(root, want.ArtifactRoot)}
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root}
	for _, tc := range want.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.Map)))
			if err != nil {
				t.Fatal(err)
			}
			result, exit := Stable(context.Background(), raw, options)
			if exit != tc.Exit || result.Outcome != tc.Outcome || result.Stage != tc.Stage || !reflect.DeepEqual(result.Code, tc.Code) {
				t.Fatalf("Stable exit=%d result=%s, want the pinned %s verdict", exit, stableResultDebug(result), tc.Name)
			}
			checkResultBytes(t, filepath.Join(root, filepath.FromSlash(tc.Result)), result)
		})
	}
}
