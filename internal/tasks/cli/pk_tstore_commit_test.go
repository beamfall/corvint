// SPDX-License-Identifier: Apache-2.0
package cli_test

import (
	"crypto/sha1"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// collisionCommits creates two valid commit objects sharing a four-hex prefix.
func collisionCommits(t *testing.T, root, tree string) string {
	t.Helper()
	seen := map[string]string{}
	for i := 0; i < 10000; i++ {
		raw := fmt.Sprintf("tree %s\nauthor t <t@example.test> 1 +0000\ncommitter t <t@example.test> 1 +0000\n\ncollision %d\n", tree, i)
		oid := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("commit %d\x00%s", len(raw), raw))))
		if prev, ok := seen[oid[:4]]; ok {
			for _, content := range []string{prev, raw} {
				cmd := exec.Command("git", "hash-object", "-w", "-t", "commit", "--stdin")
				cmd.Dir = root
				cmd.Stdin = strings.NewReader(content)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("write commit: %s %v", out, err)
				}
			}
			return oid[:4]
		}
		seen[oid[:4]] = raw
	}
	t.Fatal("no four-hex collision")
	return ""
}

// TCL-V0-7206: real complete accepts HEAD and unique short commits, rejects
// ambiguous/missing revisions, and retains the full OID in receipt-bound evidence.
func TestTCLV07206_CompleteResolvesCommitRevisions(t *testing.T) {
	for _, mode := range []string{"HEAD", "short"} {
		t.Run(mode, func(t *testing.T) {
			root, base, head, tree := diffScopeRepo(t)
			id, _ := effectsTicket(t, root, "commit-revs", "P1", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
			runOK := func(args ...string) run {
				t.Helper()
				x := atm(t, root, nil, args...)
				if x.res.Outcome != wire.OutcomeOK {
					t.Fatalf("%v: %s", args, x.stdout)
				}
				return x
			}
			c := runOK("claim", id, "--holder", "tester", "--request-id", "claim", "--base", base, "--scope-from-diff", head)
			a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
			runOK("submit", "--attempt", a, "--generation", g, "--request-id", "submit", "--tree", tree)
			clean := filepath.Join(t.TempDir(), "clean")
			git(t, root, "worktree", "add", "--detach", clean, head)
			runOK("gate", "run", "--attempt", a, "--generation", g, "--gate", "verify", "--request-id", "gate", "--worktree", clean)
			prefix := collisionCommits(t, root, tree)
			for _, bad := range []struct{ rev, detail, code string }{{prefix, "COMMIT_AMBIGUOUS", wire.CodeMalformed}, {"missing-revision", "COMMIT_NOT_FOUND", wire.CodeMissingEvidence}} {
				x := atm(t, root, nil, "complete", "--attempt", a, "--generation", g, "--request-id", "bad-"+bad.code, "--commit", bad.rev)
				if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), bad.detail) || len(x.res.Codes) != 1 || x.res.Codes[0] != bad.code {
					t.Fatalf("bad rev %s: %s", bad.rev, x.stdout)
				}
			}
			rev := head[:12]
			if mode == "HEAD" {
				rev = "HEAD"
			}
			runOK("complete", "--attempt", a, "--generation", g, "--request-id", "complete", "--commit", rev)
			show := runOK("ticket", "show", id)
			completion := field(field(show.res.Items[0], "record"), "completion")
			digest := field(completion, "manifestSha256").Str
			repo, err := intent.Resolve(root)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", digest))
			if err != nil {
				t.Fatal(err)
			}
			val, err := wire.Parse(raw)
			if err != nil || field(val, "completionCommit").Str != head {
				t.Fatalf("manifest commit: %s %v", raw, err)
			}
		})
	}
}
