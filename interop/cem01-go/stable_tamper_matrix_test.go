package main

// CEM-V1-226..229 portable conformance: every case of the shared stable
// tamper matrix under protocol/cem-1.0/stable/tamper-matrix returns its pinned
// verdict. internal/cem/verify asserts the same EXPECTED.json natively.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// tamperMatrixClasses are the tamper classes the shared matrix must cover;
// the native verifier checks the same list.
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

// TestCEMV1226TamperMatrixPortable runs every case of the shared stable tamper
// matrix through the portable canonical verifier and asserts its pinned
// verdict: exit, outcome, stage, closed code and issue codes, and for an
// ACCEPT the whole citation block.
func TestCEMV1226TamperMatrixPortable(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := stableCaseFiles(t, "tamper-matrix")
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
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
	if err := json.Unmarshal(raw, &want); err != nil {
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
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	artifacts := filepath.Join(dir, want.ArtifactRoot)
	for _, tc := range want.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			mapBytes, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tc.Map)))
			if err != nil {
				t.Fatal(err)
			}
			r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, artifacts, nil)
			got := neutralResultMap(t, r)
			var gotCitation map[string]any
			if v, ok := got["citation"].(map[string]any); ok {
				gotCitation = v
			}
			if exit != tc.Exit || r.Outcome != tc.Outcome || r.Stage != tc.Stage || !reflect.DeepEqual(r.Code, tc.Code) ||
				!reflect.DeepEqual(append([]string{}, r.IssueCodes...), append([]string{}, tc.IssueCodes...)) || !reflect.DeepEqual(gotCitation, tc.Citation) {
				b, _ := json.Marshal(r)
				t.Fatalf("runStableCanonical exit=%d result=%s, want the pinned %s verdict", exit, b, tc.Name)
			}
		})
	}
}
