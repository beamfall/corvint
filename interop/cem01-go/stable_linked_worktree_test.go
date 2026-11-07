package main

import (
	"context"
	"encoding/json"
	"os"
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

	// The canonical route refuses a repository path through a symlinked ancestor (CEM-V1-017,
	// CEM-V1-960) and t.TempDir can sit under one (macOS /var is /private/var), so the checkouts
	// live under resolved directories; the alias case below pins that refusal on every host (V1-4551).
	linked := filepath.Join(s0eTemp(t), "linked")
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
	alias := filepath.Join(s0eTemp(t), "alias")
	if err := os.Symlink(filepath.Dir(linked), alias); err != nil {
		t.Fatal(err)
	}
	if r, n := runStableCanonical(ctx, filepath.Join(alias, "linked"), raw, f.base, f.target, t.TempDir(), nil); n != 2 || r.Stage != "repository" || r.Code == nil || *r.Code != "repository-object-unavailable" {
		b, _ := json.Marshal(r)
		t.Fatalf("linked worktree through a symlinked ancestor on verify-stable-canonical: exit=%d result=%s, want UNSUPPORTED repository repository-object-unavailable", n, b)
	}

	shared := filepath.Join(s0eTemp(t), "shared")
	citationGit(t, filepath.Dir(shared), "clone", "-q", "--shared", "--no-checkout", f.repo, shared)
	citationGit(t, shared, "checkout", "-q", "--detach", f.target)
	r, n = runStable(ctx, shared, raw, f.base, f.target, t.TempDir())
	if n != 2 || r.Code == nil || *r.Code != "unsupported-object-alternates" {
		b, _ := json.Marshal(r)
		t.Fatalf("alternates clone on verify-stable: exit=%d result=%s, want unsupported-object-alternates", n, b)
	}
}
