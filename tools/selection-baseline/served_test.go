// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSelectionBaselineRefusesCommitsOriginDoesNotServe_TSPV0908 covers
// TSP-V0-908 (V1-3538): a change whose base or head neither origin/main nor
// another origin ref contains is refused with its id, before any replay, while
// commits on origin/main or only on an origin branch pass.
func TestSelectionBaselineRefusesCommitsOriginDoesNotServe_TSPV0908(t *testing.T) {
	dir := t.TempDir()
	up, down := filepath.Join(dir, "up"), filepath.Join(dir, "down")
	git := func(d string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = d
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(dir, "init", "-q", "-b", "main", up)
	git(up, "commit", "-q", "--allow-empty", "-m", "a")
	base := git(up, "rev-parse", "HEAD")
	git(up, "commit", "-q", "--allow-empty", "-m", "b")
	merged := git(up, "rev-parse", "HEAD")
	git(up, "checkout", "-q", "-b", "lane", base)
	git(up, "commit", "-q", "--allow-empty", "-m", "lane")
	lane := git(up, "rev-parse", "HEAD")
	git(up, "checkout", "-q", "main")
	git(dir, "clone", "-q", up, down)
	git(down, "commit", "-q", "--allow-empty", "-m", "rewritten before merge")
	local := git(down, "rev-parse", "HEAD")

	ctx := context.Background()
	ok := []change{{ID: "merged", Base: base, Head: merged}, {ID: "branch-only", Base: base, Head: lane}}
	if err := servedByOrigin(ctx, down, ok); err != nil {
		t.Fatalf("served commits refused: %v", err)
	}
	err := servedByOrigin(ctx, down, append(ok, change{ID: "ci-gone", Base: base, Head: local}))
	if err == nil || !strings.Contains(err.Error(), "change ci-gone: head "+local+" is not reachable") || strings.Contains(err.Error(), "merged") {
		t.Fatalf("unserved head: err = %v, want only ci-gone named", err)
	}
}
