package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestCEMV1577PortableReaderChecksTwoProfileMap is the portable side of
// CEM-V1-577: the two-profile map (a go-test and a node-test receipt) ACCEPTs
// and each tampered variant is refused at the stage and code EXPECTED.json
// gives, which the native reader asserts over the same files.
func TestCEMV1577PortableReaderChecksTwoProfileMap(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "multi-runner"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		BaseRevision   string `json:"baseRevision"`
		TargetRevision string `json:"targetRevision"`
		Accept         struct {
			Map, Outcome, Stage string
		} `json:"accept"`
		Refusals []struct {
			Name, Map, Outcome, Stage, Code string
		} `json:"refusals"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if len(want.Refusals) == 0 {
		t.Fatal("two-profile case lists no tampered variants")
	}
	repo, base, target := neutralRepository(t, dir)
	if base != want.BaseRevision || target != want.TargetRevision {
		t.Fatalf("recipe commits %s..%s, want %s..%s", base, target, want.BaseRevision, want.TargetRevision)
	}
	check := func(name, path, outcome, stage, code string) {
		t.Helper()
		mapBytes, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, dir, nil)
		got := ""
		if r.Code != nil {
			got = *r.Code
		}
		if r.Outcome != outcome || r.Stage != stage || (outcome == "ACCEPT") != (exit == 0) || code != "" && got != code && !slices.Contains(r.IssueCodes, code) {
			b, _ := json.Marshal(r)
			t.Fatalf("%s: exit=%d result=%s, want %s at %s with %q", name, exit, b, outcome, stage, code)
		}
	}
	check("accept", want.Accept.Map, want.Accept.Outcome, want.Accept.Stage, "")
	for _, refusal := range want.Refusals {
		check(refusal.Name, refusal.Map, refusal.Outcome, refusal.Stage, refusal.Code)
	}
}
