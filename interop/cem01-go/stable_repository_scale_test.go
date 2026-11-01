package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// CEM-V1-961/CEM-V1-962 (V1-3756): the 4096-entry tree bound and the
// [core]/[extensions]-only config profile belong to the legacy portable
// `verify-stable` route alone (CEM-V1-352). The repository-scale route,
// `verify-stable-canonical`, verifies a map whose snapshots hold more than 4096
// entries in an ordinary checkout whose config carries remote and branch
// sections, and the legacy snapshot of the same commit still refuses at its
// declared bound rather than reading part of the tree.
func TestCEMV1961CanonicalRouteHasNoLegacyTreeOrConfigBound(t *testing.T) {
	if !stableOwnerSupported() {
		t.Skip("NOT_RUN: process containment is not qualified on " + runtime.GOOS)
	}
	dir := neutralCaseFiles(t)
	repo, base, target := neutralRepositoryWith(t, dir, 5000)
	config := filepath.Join(repo, ".git", "config")
	ordinary, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	ordinary = append(ordinary, []byte("[remote \"origin\"]\n\turl = https://example.invalid/repo.git\n"+
		"\tfetch = +refs/heads/*:refs/remotes/origin/*\n[branch \"main\"]\n\tremote = origin\n\tmerge = refs/heads/main\n")...)
	if err := os.WriteFile(config, ordinary, 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "stable.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var recipeBase string
	if err := json.Unmarshal(m["baseRevision"], &recipeBase); err != nil || recipeBase == "" {
		t.Fatalf("stable.json baseRevision %s: %v", m["baseRevision"], err)
	}
	// The bulk entries change the commit IDs only; the patch and every
	// evidence span are the recipe's own, so only the base binding moves.
	mapBytes := bytes.Replace(raw, []byte(`"`+recipeBase+`"`), []byte(`"`+base+`"`), 1)

	r, exit := runStableCanonical(context.Background(), repo, mapBytes, base, target, dir, nil)
	if exit != 0 || r.Outcome != "ACCEPT" {
		b, _ := json.Marshal(r)
		t.Fatalf("canonical route over a 5000-entry ordinary checkout: exit=%d result=%s, want ACCEPT", exit, b)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	v := &verifier{ctx: ctx, repo: repo, blobs: map[string][]byte{}, trees: map[string]*treeEntry{}}
	if entries, e := v.snapshot03(target); e == nil || e.code != "unsupported-resource-limit" || entries != nil {
		t.Fatalf("legacy snapshot of the same target: %d entries, err %#v, want unsupported-resource-limit", len(entries), e)
	}
	if e := repository03(repo); e == nil || e.code != "unsupported-repository-envelope" || !e.operational {
		t.Fatalf("legacy repository envelope over an ordinary config: %#v, want unsupported-repository-envelope", e)
	}
}
