package main

// CEM-V1-260 portable conformance: the shared drift-resource-limit case, with
// the operation limit lowered to its operations value through the test hook,
// stops the drift stage at that limit and returns the pinned
// UNSUPPORTED result with exactly the pinned drift rows. internal/cem/verify
// asserts the same files.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCEMV1260DriftResourceLimitPortable(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := stableCaseFiles(t, "drift-resource-limit")
	var want struct {
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
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if want.Profile != "cem-stable-drift-resource-limit/1" || want.Result != "result.json" || want.Operations <= 0 || want.Operations >= stableCallLimit {
		t.Fatalf("EXPECTED.json profile %q result %q operations %d", want.Profile, want.Result, want.Operations)
	}
	var pinned struct {
		Drift         []json.RawMessage `json:"drift"`
		VerdictSha256 string            `json:"verdictSha256"`
	}
	raw, err = os.ReadFile(filepath.Join(dir, want.Result))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &pinned); err != nil {
		t.Fatal(err)
	}
	if len(pinned.Drift) != want.DriftItems || pinned.VerdictSha256 != want.VerdictSha256 {
		t.Fatalf("result.json drift %d verdict %s, EXPECTED.json %d %s", len(pinned.Drift), pinned.VerdictSha256, want.DriftItems, want.VerdictSha256)
	}
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	mapBytes, err := os.ReadFile(filepath.Join(dir, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, dir, &stableHooks{callLimit: want.Operations})
	if exit != want.Exit || r.Outcome != want.Outcome || r.Stage != want.Stage || r.Code == nil || *r.Code != want.Code || len(r.Drift) != want.DriftItems {
		b, _ := json.Marshal(r)
		t.Fatalf("runStableCanonical exit=%d drift=%d result=%s, want exit %d %s/%s %s with %d drift rows", exit, len(r.Drift), b, want.Exit, want.Outcome, want.Stage, want.Code, want.DriftItems)
	}
	checkResultBytes(t, filepath.Join(dir, want.Result), r)
}
