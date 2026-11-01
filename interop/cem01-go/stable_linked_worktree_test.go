package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCEMV1753LegacyEnvelopeNamesLinkedWorktree is CEM-V1-753 (V1-3800). The legacy
// verify-stable route keeps its primary-clean-config-bounded/1 envelope, which
// refuses linked worktrees (ALGORITHMS.md). A linked worktree with no
// alternates file is refused as outside that envelope, not as an alternates
// store, and verify-stable-canonical accepts it. A real alternates-backed
// clone keeps unsupported-object-alternates.
func TestCEMV1753LegacyEnvelopeNamesLinkedWorktree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("NOT_RUN: verify-stable refuses process containment on Windows before repository admission")
	}
	f := newCitationFixture(t, patch24Tree())
	raw, err := json.Marshal(f.stableMap(t, patch24Careful()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	linked := filepath.Join(t.TempDir(), "linked")
	citationGit(t, f.repo, "worktree", "add", "-q", "--detach", linked, f.target)
	r, n := runStable(ctx, linked, raw, f.base, f.target, t.TempDir())
	if n != 2 || r.Stage != "repository" || r.Code == nil || *r.Code != "unsupported-repository-envelope" {
		b, _ := json.Marshal(r)
		t.Fatalf("linked worktree on verify-stable: exit=%d result=%s, want UNSUPPORTED repository unsupported-repository-envelope", n, b)
	}
	if r, n := runStableCanonical(ctx, linked, raw, f.base, f.target, t.TempDir(), nil); n != 0 || r.Outcome != "ACCEPT" {
		b, _ := json.Marshal(r)
		t.Fatalf("linked worktree on verify-stable-canonical: exit=%d result=%s, want ACCEPT", n, b)
	}

	shared := filepath.Join(t.TempDir(), "shared")
	citationGit(t, filepath.Dir(shared), "clone", "-q", "--shared", "--no-checkout", f.repo, shared)
	citationGit(t, shared, "checkout", "-q", "--detach", f.target)
	r, n = runStable(ctx, shared, raw, f.base, f.target, t.TempDir())
	if n != 2 || r.Code == nil || *r.Code != "unsupported-object-alternates" {
		b, _ := json.Marshal(r)
		t.Fatalf("alternates clone on verify-stable: exit=%d result=%s, want unsupported-object-alternates", n, b)
	}
}
