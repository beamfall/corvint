package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestCEMV1936NativeReaderAgreesOnBinarySectionVector checks CEM-V1-936 on
// the shared vector: the native stable verifier ACCEPTs the map of a range
// whose canonical patch carries a binary create, modify and delete beside one
// textual hunk, maps only the textual path, pins the patch EXPECTED.json
// gives and refuses an altered patch digest. The portable reader asserts the
// same file.
func TestCEMV1936NativeReaderAgreesOnBinarySectionVector(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	dir := filepath.Join(repositoryRoot(t), "interop/cem01-go/testdata/binary-section")
	var want struct {
		BaseRevision   string   `json:"baseRevision"`
		TargetRevision string   `json:"targetRevision"`
		Map            string   `json:"map"`
		ArtifactRoot   string   `json:"artifactRoot"`
		PatchSha256    string   `json:"patchSha256"`
		MappedPaths    []string `json:"mappedPaths"`
		Outcome, Stage string
	}
	data, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	c := &neutralCase{root: dir}
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	raw, err := os.ReadFile(filepath.Join(dir, want.Map))
	if err != nil {
		t.Fatal(err)
	}
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: filepath.Join(dir, want.ArtifactRoot)}
	r, exit := Stable(context.Background(), raw, options)
	b, _ := json.Marshal(r)
	if r.Outcome != want.Outcome || r.Stage != want.Stage || exit != 0 || r.PatchSha256 == nil || *r.PatchSha256 != want.PatchSha256 {
		t.Fatalf("exit=%d result=%s, want %s at %s with patch %s", exit, b, want.Outcome, want.Stage, want.PatchSha256)
	}
	var mapped []string
	for _, hunk := range neutralResult(t, r)["hunks"].([]any) {
		if path, _ := hunk.(map[string]any)["path"].(string); !slices.Contains(mapped, path) {
			mapped = append(mapped, path)
		}
	}
	if !slices.Equal(mapped, want.MappedPaths) {
		t.Fatalf("mapped paths %v, want %v", mapped, want.MappedPaths)
	}
	altered := []byte(strings.Replace(string(raw), want.PatchSha256, strings.Repeat("0", 64), 1))
	if r, exit := Stable(context.Background(), altered, options); r.Outcome != "REJECT" || exit == 0 {
		b, _ := json.Marshal(r)
		t.Fatalf("altered patch digest: exit=%d result=%s", exit, b)
	}
}
