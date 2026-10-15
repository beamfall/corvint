package main

// CEM-V1-176/177 portable conformance: every case under
// protocol/cem-1.0/stable/reference-precedence returns its pinned stage and
// code and the pinned result line. internal/cem/verify asserts the same files.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestCEMV1177ReferencePrecedencePortable(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := stableCaseFiles(t, "reference-precedence")
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
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
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-reference-precedence/1" || want.Repository != "../neutral-criterion/repository.recipe.json" || want.ArtifactRoot != "../neutral-criterion" || len(want.Cases) != 6 {
		t.Fatalf("EXPECTED.json profile %q repository %q artifactRoot %q cases %d", want.Profile, want.Repository, want.ArtifactRoot, len(want.Cases))
	}
	neutral := neutralCaseFiles(t)
	repo, base, target := neutralRepository(t, neutral)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	for _, tc := range want.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			mapBytes, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tc.Map)))
			if err != nil {
				t.Fatal(err)
			}
			r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, neutral, nil)
			if exit != tc.Exit || r.Outcome != tc.Outcome || r.Stage != tc.Stage || !reflect.DeepEqual(r.Code, tc.Code) {
				b, _ := json.Marshal(r)
				t.Fatalf("runStableCanonical exit=%d result=%s, want the pinned %s verdict", exit, b, tc.Name)
			}
			checkResultBytes(t, filepath.Join(dir, filepath.FromSlash(tc.Result)), r)
		})
	}
}
