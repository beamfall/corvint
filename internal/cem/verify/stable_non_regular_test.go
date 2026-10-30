package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
)

// nonRegularExpected is the EXPECTED.json of the cem-non-regular/1 case shared
// with the portable reader (CAX-V0-024).
type nonRegularExpected struct {
	BaseRevision   string                    `json:"baseRevision"`
	TargetRevision string                    `json:"targetRevision"`
	Map            string                    `json:"map"`
	ArtifactRoot   string                    `json:"artifactRoot"`
	MappedPaths    []string                  `json:"mappedPaths"`
	NonRegular     []gitauth.NonRegularEntry `json:"nonRegular"`
	Cases          []struct {
		Name, NonRegular, Outcome, Stage, IssueCode, PatchSha256 string
	} `json:"cases"`
}

// TestCAXV0024NativeReaderAgreesOnNonRegularVector checks CAX-V0-020 and
// CAX-V0-024 on the shared vector: under cem-non-regular/1 the native stable
// verifier ACCEPTs the map of a range that adds a symlink and retypes a file
// into one, pins the regular-only patch, maps exactly the regular paths and
// reports each non-regular path once; without the profile the same map is
// refused patch-digest-mismatch, as a plain cem/1.0 reader must. The portable
// reader asserts the same file.
func TestCAXV0024NativeReaderAgreesOnNonRegularVector(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	dir := filepath.Join(repositoryRoot(t), "interop/cem01-go/testdata/non-regular")
	var want nonRegularExpected
	data, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(want.Cases) != 2 || len(want.NonRegular) == 0 {
		t.Fatalf("non-regular case lists %d cases and %d rows", len(want.Cases), len(want.NonRegular))
	}
	c := &neutralCase{root: dir}
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	raw, err := os.ReadFile(filepath.Join(dir, want.Map))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range want.Cases {
		opted := tc.NonRegular == "unknown"
		r, exit := Stable(context.Background(), raw, StableOptions{Repository: repo, ExpectedBase: base, Target: target,
			ArtifactRoot: filepath.Join(dir, want.ArtifactRoot), NonRegular: opted})
		b, _ := json.Marshal(r)
		if r.Outcome != tc.Outcome || r.Stage != tc.Stage || (tc.Outcome == "ACCEPT") != (exit == 0) ||
			tc.IssueCode != "" && !slices.Contains(r.IssueCodes, tc.IssueCode) || r.PatchSha256 == nil || *r.PatchSha256 != tc.PatchSha256 {
			t.Fatalf("%s: exit=%d result=%s, want %s at %s with %q and patch %s", tc.Name, exit, b, tc.Outcome, tc.Stage, tc.IssueCode, tc.PatchSha256)
		}
		if !opted {
			if r.NonRegular != nil {
				t.Fatalf("%s: rows %v reported without the profile", tc.Name, r.NonRegular)
			}
			continue
		}
		if !reflect.DeepEqual(r.NonRegular, want.NonRegular) {
			t.Fatalf("%s: rows %+v, want %+v", tc.Name, r.NonRegular, want.NonRegular)
		}
		var mapped []string
		for _, hunk := range neutralResult(t, r)["hunks"].([]any) {
			if path, _ := hunk.(map[string]any)["path"].(string); !slices.Contains(mapped, path) {
				mapped = append(mapped, path)
			}
		}
		if !slices.Equal(mapped, want.MappedPaths) {
			t.Fatalf("%s: mapped paths %v, want %v", tc.Name, mapped, want.MappedPaths)
		}
	}
}

// TestCAXV0024ProfileLeavesRegularRangesUnchanged checks CAX-V0-020: on a
// range with no non-regular path the profile excludes nothing, and the result
// equals the plain cem/1.0 result field for field, so cem/1.0 artifacts verify
// unchanged.
func TestCAXV0024ProfileLeavesRegularRangesUnchanged(t *testing.T) {
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
	c.expect.BaseRevision, c.expect.TargetRevision = want.BaseRevision, want.TargetRevision
	repo, base, target := c.materialize(t)
	raw, err := os.ReadFile(filepath.Join(c.root, want.Accept.Map))
	if err != nil {
		t.Fatal(err)
	}
	options := StableOptions{Repository: repo, ExpectedBase: base, Target: target, ArtifactRoot: c.root}
	plain, plainExit := Stable(context.Background(), raw, options)
	options.NonRegular = true
	profiled, profiledExit := Stable(context.Background(), raw, options)
	if plain.Outcome != "ACCEPT" || plainExit != profiledExit || len(profiled.NonRegular) != 0 ||
		!reflect.DeepEqual(neutralResult(t, plain), neutralResult(t, profiled)) {
		t.Fatalf("the profile changed a regular-only verdict: plain %+v (exit %d), profiled %+v (exit %d)", plain, plainExit, profiled, profiledExit)
	}
}
