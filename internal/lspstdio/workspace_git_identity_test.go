// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LQP-V0-004: symbolic HEAD bytes are not a resolved-ref currentness oracle.
func TestWorkspaceSameTreeRefAdvance(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(fmt.Sprint(linked), func(t *testing.T) {
			original, _ := filepath.EvalSymlinks(contextFixture(t))
			root := original
			if linked {
				dir, _ := filepath.EvalSymlinks(t.TempDir())
				root = filepath.Join(dir, "linked")
				fixtureGit(t, original, "worktree", "add", "-qb", "linked", root, "HEAD")
			}
			ctx := context.Background()
			baseline, e := observeWorkspace(ctx, root)
			if e != nil {
				t.Fatal(e)
			}
			branch := strings.TrimSpace(string(fixtureGit(t, root, "symbolic-ref", "HEAD")))
			tree := strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "HEAD^{tree}")))
			commit := strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "HEAD")))
			next := strings.TrimSpace(string(fixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-p", commit, "-m", "same tree distinct commit")))
			calls := 0
			_, e = observeWorkspaceUsing(ctx, root, func(c context.Context, r string) (workspaceGitIdentity, error) {
				v, err := observeWorkspaceGit(c, r)
				calls++
				if calls == 1 && err == nil {
					fixtureGit(t, root, "update-ref", branch, next, commit)
				}
				return v, err
			})
			if e == nil || calls != 2 {
				t.Fatalf("same-HEAD ref drift passed or not bracketed: %v calls%d", e, calls)
			}
			g := workspaceGuard{baseline: baseline}
			budget := workspaceObservationBudget
			if g.check(ctx, root, &budget) == nil || !g.stale {
				t.Fatal("persistent ref drift not sticky")
			}
		})
	}
}
func TestWorkspaceGitParserAndDetached(t *testing.T) {
	root, _ := filepath.EvalSymlinks(contextFixture(t))
	ctx := context.Background()
	mapping, e := workspaceMapping(root)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := boundedGit(ctx, root, "rev-parse", "--show-object-format", "--absolute-git-dir", "--path-format=absolute", "--git-common-dir", "HEAD^{commit}", "HEAD^{tree}")
	if e != nil {
		t.Fatal(e)
	}
	good, e := parseWorkspaceGit(raw, mapping)
	if e != nil {
		t.Fatal(e)
	}
	negatives := [][]byte{append(append([]byte(nil), raw...), []byte("extra\n")...), []byte(strings.Replace(string(raw), "sha1", "SHA1", 1)), []byte(strings.Replace(string(raw), "\n", "\r\n", 1)), []byte(strings.Replace(string(raw), good.commit, "1.0", 1)), []byte(strings.Replace(string(raw), good.adminPath, "relative", 1))}
	for _, bad := range negatives {
		if _, e := parseWorkspaceGit(bad, mapping); e == nil {
			t.Fatalf("malformed identity passed %q", bad)
		}
	}
	sha := mapping
	sha.headText = "ref: refs/heads/main\n"
	control := fmt.Sprintf("sha256\n%s\n%s\n%s\n%s\n", good.adminPath, good.commonPath, strings.Repeat("a", 64), strings.Repeat("b", 64))
	if _, e := parseWorkspaceGit([]byte(control), sha); e != nil {
		t.Fatal("SHA256 control", e)
	}
	fixtureGit(t, root, "checkout", "-q", "--detach", good.commit)
	if _, e := observeWorkspaceGit(ctx, root); e != nil {
		t.Fatal("detached commit", e)
	}
	tag := strings.TrimSpace(string(fixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "tag", "-a", "tag", "-m", "tag")))
	_ = tag
	tagOID := strings.TrimSpace(string(fixtureGit(t, root, "rev-parse", "refs/tags/tag")))
	for _, oid := range []string{tagOID, good.tree} {
		if e := os.WriteFile(filepath.Join(good.adminPath, "HEAD"), []byte(oid+"\n"), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := observeWorkspaceGit(ctx, root); e == nil {
			t.Fatal("detached noncommit accepted")
		}
	}
}
