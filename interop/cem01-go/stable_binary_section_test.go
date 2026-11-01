package main

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

// binarySectionExpected is the EXPECTED.json of the canonical binary section
// case shared with the native reader (CEM-V1-936).
type binarySectionExpected struct {
	BaseRevision   string   `json:"baseRevision"`
	TargetRevision string   `json:"targetRevision"`
	Map            string   `json:"map"`
	ArtifactRoot   string   `json:"artifactRoot"`
	PatchSha256    string   `json:"patchSha256"`
	MappedPaths    []string `json:"mappedPaths"`
	BinaryPaths    []string `json:"binaryPaths"`
	Outcome, Stage string
}

// TestCEMV1936PortableReaderAgreesOnBinarySectionVector is the portable side
// of CEM-V1-936: both stable routes ACCEPT the native map of a range whose
// canonical patch carries a binary create, modify and delete beside one
// textual hunk, map only the textual path and pin the patch EXPECTED.json
// gives; an altered patch digest is refused. The native reader asserts the
// same file.
func TestCEMV1936PortableReaderAgreesOnBinarySectionVector(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "binary-section"))
	if err != nil {
		t.Fatal(err)
	}
	var want binarySectionExpected
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	mapPath := filepath.Join(dir, want.Map)
	m, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	canonical, exit, _, _, _ := executeStableCanonicalCLIProfile(context.Background(), []string{"--repository", repo, "--map", mapPath,
		"--expected-base", base, "--target", target, "--artifacts", artifacts})
	external, externalExit := runStable(context.Background(), repo, m, base, target, artifacts)
	for name, c := range map[string]struct {
		r    stableResult
		exit int
	}{"verify-stable-canonical": {canonical, exit}, "verify-stable": {external, externalExit}} {
		b, _ := json.Marshal(c.r)
		if c.r.Outcome != want.Outcome || c.r.Stage != want.Stage || c.exit != 0 || c.r.PatchSHA256 == nil || *c.r.PatchSHA256 != want.PatchSha256 {
			t.Fatalf("%s: exit=%d result=%s, want %s at %s with patch %s", name, c.exit, b, want.Outcome, want.Stage, want.PatchSha256)
		}
		var mapped []string
		for _, hunk := range neutralResultMap(t, c.r)["hunks"].([]any) {
			if path, _ := hunk.(map[string]any)["path"].(string); !slices.Contains(mapped, path) {
				mapped = append(mapped, path)
			}
		}
		if !slices.Equal(mapped, want.MappedPaths) {
			t.Fatalf("%s: mapped paths %v, want %v", name, mapped, want.MappedPaths)
		}
	}
	altered := []byte(strings.Replace(string(m), want.PatchSha256, strings.Repeat("0", 64), 1))
	if r, exit := runStable(context.Background(), repo, altered, base, target, artifacts); r.Outcome != "REJECT" || exit == 0 {
		b, _ := json.Marshal(r)
		t.Fatalf("altered patch digest: exit=%d result=%s", exit, b)
	}
}
