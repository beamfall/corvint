package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
)

// TestCAXV0024PortableReaderAgreesOnNonRegularVector is the portable side of
// CAX-V0-024: verify-stable-canonical --non-regular unknown ACCEPTs the map of
// a range that adds a symlink and retypes a file into one, with the patch
// digest, mapped paths and per-path unknown rows EXPECTED.json gives; without
// the flag it REJECTs patch-digest-mismatch; any other value is refused
// invalid-arguments. The native reader asserts the same file.
func TestCAXV0024PortableReaderAgreesOnNonRegularVector(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "non-regular"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		BaseRevision   string            `json:"baseRevision"`
		TargetRevision string            `json:"targetRevision"`
		Map            string            `json:"map"`
		ArtifactRoot   string            `json:"artifactRoot"`
		MappedPaths    []string          `json:"mappedPaths"`
		NonRegular     []nonRegularEntry `json:"nonRegular"`
		Cases          []struct {
			Name, NonRegular, Outcome, Stage, IssueCode, PatchSha256 string
		} `json:"cases"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if len(want.Cases) != 2 || len(want.NonRegular) == 0 {
		t.Fatalf("non-regular case lists %d cases and %d rows", len(want.Cases), len(want.NonRegular))
	}
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	argv := func(extra ...string) []string {
		return append([]string{"--repository", repo, "--map", filepath.Join(dir, want.Map), "--expected-base", base,
			"--target", target, "--artifacts", filepath.Join(dir, want.ArtifactRoot)}, extra...)
	}
	for _, tc := range want.Cases {
		var extra []string
		if tc.NonRegular != "" {
			extra = []string{"--non-regular", tc.NonRegular}
		}
		r, exit, _, _, rows := executeStableCanonicalCLIProfile(context.Background(), argv(extra...))
		b, _ := json.Marshal(r)
		if r.Outcome != tc.Outcome || r.Stage != tc.Stage || (tc.Outcome == "ACCEPT") != (exit == 0) ||
			tc.IssueCode != "" && !slices.Contains(r.IssueCodes, tc.IssueCode) || r.PatchSHA256 == nil || *r.PatchSHA256 != tc.PatchSha256 {
			t.Fatalf("%s: exit=%d result=%s, want %s at %s with %q and patch %s", tc.Name, exit, b, tc.Outcome, tc.Stage, tc.IssueCode, tc.PatchSha256)
		}
		if tc.NonRegular == "" {
			if rows != nil {
				t.Fatalf("%s: rows %v reported without the profile", tc.Name, *rows)
			}
			continue
		}
		if rows == nil || !reflect.DeepEqual(*rows, want.NonRegular) {
			t.Fatalf("%s: rows %v, want %+v", tc.Name, rows, want.NonRegular)
		}
		var mapped []string
		for _, hunk := range neutralResultMap(t, r)["hunks"].([]any) {
			if path, _ := hunk.(map[string]any)["path"].(string); !slices.Contains(mapped, path) {
				mapped = append(mapped, path)
			}
		}
		if !slices.Equal(mapped, want.MappedPaths) {
			t.Fatalf("%s: mapped paths %v, want %v", tc.Name, mapped, want.MappedPaths)
		}
	}
	r, _, _, _, rows := executeStableCanonicalCLIProfile(context.Background(), argv("--non-regular", "follow"))
	if r.Code == nil || *r.Code != "invalid-arguments" || r.Stage != "arguments" || rows != nil {
		b, _ := json.Marshal(r)
		t.Fatalf("--non-regular follow: %s", b)
	}
}

// TestCAXV0024PortableProfileLeavesRegularRangesUnchanged checks CAX-V0-020
// on the portable side: on a range with no non-regular path the profile
// excludes nothing and the result equals the plain cem/1.0 result.
func TestCAXV0024PortableProfileLeavesRegularRangesUnchanged(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "multi-runner"))
	if err != nil {
		t.Fatal(err)
	}
	repo, base, target := neutralRepository(t, dir)
	argv := []string{"--repository", repo, "--map", filepath.Join(dir, "stable.json"), "--expected-base", base, "--target", target, "--artifacts", dir}
	plain, plainExit, _, _, _ := executeStableCanonicalCLIProfile(context.Background(), argv)
	profiled, profiledExit, _, _, rows := executeStableCanonicalCLIProfile(context.Background(), append(argv, "--non-regular", "unknown"))
	if plain.Outcome != "ACCEPT" || plainExit != profiledExit || rows == nil || len(*rows) != 0 ||
		!reflect.DeepEqual(neutralResultMap(t, plain), neutralResultMap(t, profiled)) {
		t.Fatalf("the profile changed a regular-only verdict: plain %+v (exit %d), profiled %+v (exit %d)", plain, plainExit, profiled, profiledExit)
	}
}
