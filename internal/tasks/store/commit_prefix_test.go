// SPDX-License-Identifier: Apache-2.0
package store

import (
	"crypto/sha1"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// TCL-V0-7206: a blob sharing a prefix does not make one commit ambiguous.
func TestTCLV07206_CommitPrefixCountsOnlyCommits(t *testing.T) {
	root := t.TempDir()
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	commit := holdGitOut(t, root, "rev-parse", "HEAD")
	prefix := commit[:4]
	found := false
	for i := 0; i < 2000000; i++ {
		raw := fmt.Sprintf("blob collision %d", i)
		oid := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(raw), raw))))
		if !strings.HasPrefix(oid, prefix) {
			continue
		}
		cmd := exec.Command("git", "hash-object", "-w", "--stdin")
		cmd.Dir, cmd.Stdin = root, strings.NewReader(raw)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("write blob: %s %v", out, err)
		}
		found = true
		break
	}
	if !found {
		t.Fatal("no blob collision found")
	}
	objects := strings.Fields(holdGitOut(t, root, "rev-parse", "--disambiguate="+prefix))
	if len(objects) != 2 {
		t.Fatalf("expected commit + blob, got %v", objects)
	}
	if ambiguous, err := CommitPrefixAmbiguous(root, prefix); err != nil || ambiguous {
		t.Fatalf("commit + blob: %v %v", ambiguous, err)
	}
	if resolved, err := ResolveCommit(root, prefix); err != nil || resolved != commit {
		t.Fatalf("resolve commit + blob: %s %v", resolved, err)
	}
}
