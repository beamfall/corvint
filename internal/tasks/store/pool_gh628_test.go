//go:build unix

package store_test

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gh628Health installs one member "a" whose health command is health.
func gh628Health(t *testing.T, health wire.Value) *leaseStore {
	t.Helper()
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
	if rep, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("gh628-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", rep, e)
	}
	return s
}

// gh628External is an external tool checkout at its real path with two
// commits; it returns the path and both revisions (old, head).
func gh628External(t *testing.T) (string, string, string) {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	gitRun(t, dir, "init", "-q", "-b", "main")
	rev := func() string {
		c := exec.Command("git", "rev-parse", "HEAD")
		c.Dir = dir
		raw, e := c.Output()
		if e != nil {
			t.Fatal(e)
		}
		return strings.TrimSpace(string(raw))
	}
	commit := func(body string) string {
		if e := os.WriteFile(filepath.Join(dir, "reset.sh"), []byte(body), 0644); e != nil {
			t.Fatal(e)
		}
		gitRun(t, dir, "add", "reset.sh")
		gitRun(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "tool")
		return rev()
	}
	old := commit("echo old\n")
	return dir, old, commit("pwd\n")
}
func gh628Pin(path, revision string) wire.Value {
	return obj("kind", str("PINNED_REPOSITORY"), "path", str(path), "revision", str(revision))
}

// PSR-V0-011: a health bound above the former 300 s maximum is accepted and
// still enforced: with the injected short second, timeoutSeconds 3600 kills a
// long command, records TIMEOUT and keeps the member quarantined.
func TestPSRV0011ExtendedBoundEnforced(t *testing.T) {
	s := gh628Health(t, obj("argv", wire.Strings([]string{"/bin/sleep", "30"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3600")))
	restore := store.SetPoolCommandSecondForTest(100 * time.Microsecond)
	defer restore()
	started := time.Now()
	rep, e := store.PoolCommand(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "long-health", Root: s.root, Lease: transaction.LeaseRequest{Member: "a"}}, "health")
	if e != nil || rep == nil || !rep.Unretryable {
		t.Fatalf("health %+v %v", rep, e)
	}
	if elapsed := time.Since(started); elapsed > 20*time.Second {
		t.Fatalf("bound not enforced: %v", elapsed)
	}
	entries := psrPools(t, s).Entries
	if len(entries) != 1 || entries[0].State != "QUARANTINED" || !strings.HasPrefix(entries[0].Reason, "TIMEOUT;") {
		t.Fatalf("timeout not retained: %+v", entries)
	}
}

// PSR-V0-012: health runs in a pinned external worktree only while its HEAD is
// the pinned revision and its inputs are clean; every mismatch refuses before
// preparation with a named code and runs nothing.
func TestPSRV0012PinnedExternalCwd(t *testing.T) {
	for _, scenario := range []string{"pinned", "mismatched-revision", "dirty-tracked", "dirty-untracked", "alias", "subdirectory", "not-a-worktree"} {
		t.Run(scenario, func(t *testing.T) {
			ext, old, head := gh628External(t)
			marker := filepath.Join(t.TempDir(), "ran")
			path, revision, want := ext, head, ""
			switch scenario {
			case "mismatched-revision":
				revision, want = old, wire.CodeStaleTree
			case "dirty-tracked":
				if e := os.WriteFile(filepath.Join(ext, "reset.sh"), []byte("changed\n"), 0644); e != nil {
					t.Fatal(e)
				}
				want = wire.CodeDirtyWorktree
			case "dirty-untracked":
				if e := os.WriteFile(filepath.Join(ext, "extra.sh"), []byte("x\n"), 0644); e != nil {
					t.Fatal(e)
				}
				want = wire.CodeDirtyWorktree
			case "alias":
				path = filepath.Join(t.TempDir(), "link")
				if e := os.Symlink(ext, path); e != nil {
					t.Fatal(e)
				}
				want = wire.CodeMissingEvidence
			case "subdirectory":
				path = filepath.Join(ext, "sub")
				if e := os.Mkdir(path, 0755); e != nil {
					t.Fatal(e)
				}
				want = wire.CodeMissingEvidence
			case "not-a-worktree":
				path, _ = filepath.EvalSymlinks(t.TempDir())
				want = wire.CodeMissingEvidence
			}
			s := gh628Health(t, obj("argv", wire.Strings([]string{"/bin/sh", "-c", "/bin/sh reset.sh > " + marker}), "cwd", gh628Pin(path, revision), "env", wire.Array(), "timeoutSeconds", str("3600")))
			rep, e := store.PoolCommand(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "pinned-health", Root: s.root, Lease: transaction.LeaseRequest{Member: "a"}}, "health")
			if want == "" {
				if e != nil || rep == nil || !rep.Unretryable {
					t.Fatalf("pinned health %+v %v", rep, e)
				}
				raw, e := os.ReadFile(marker)
				if e != nil || strings.TrimSpace(string(raw)) != ext {
					t.Fatalf("not run in the pinned worktree: %q %v", raw, e)
				}
				if entries := psrPools(t, s).Entries; len(entries) != 1 || !strings.HasPrefix(entries[0].Reason, "EXIT_ZERO;") {
					t.Fatalf("observation %+v", entries)
				}
				return
			}
			code := wire.CodeOf(e)
			if code == "" && rep != nil && rep.Outcome.HasCode(want) {
				code = want
			}
			if code != want || (rep != nil && rep.Unretryable) {
				t.Fatalf("want %s, got %+v %v", want, rep, e)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("refused pin ran the command: %v", err)
			}
			if _, err := os.Stat(filepath.Join(s.repo.StateDir, "pools.json")); !os.IsNotExist(err) {
				if entries := psrPools(t, s).Entries; len(entries) != 0 {
					t.Fatalf("refused pin recorded ownership: %+v", entries)
				}
			}
		})
	}
}

// PSR-V0-012: pool sweep runs pinned cleanup, reset and verify in the external
// worktree and frees the member; a mismatched pin refuses admission and runs
// no phase.
func TestPSRV0012PinnedSweep(t *testing.T) {
	for _, scenario := range []string{"pinned", "mismatched-revision"} {
		t.Run(scenario, func(t *testing.T) {
			ext, old, head := gh628External(t)
			marker := filepath.Join(t.TempDir(), "phases")
			revision := head
			if scenario != "pinned" {
				revision = old
			}
			s, _ := psrFixtureConfigured(t, "pwd >> "+marker, "pwd", ext, "1", false, func(v wire.Value) {
				pools, _ := v.Obj.Get("pools")
				config, _ := pools.Arr[0].Obj.Get("memberConfig")
				member, _ := config.Obj.Get("a")
				reuse, _ := member.Obj.Get("safeReuse")
				reuse.Obj.Set("cwd", gh628Pin(ext, revision))
				member.Obj.Set("cleanup", obj("argv", wire.Strings([]string{"/bin/sh", "-c", "pwd >> " + marker}), "env", wire.Strings(nil), "cwd", gh628Pin(ext, revision), "timeoutSeconds", str("3600")))
			})
			out, e := psrRun(s, "pinned-sweep")
			if scenario == "pinned" {
				if e != nil || out.Report == nil || out.Report.Outcome.Outcome != mutation.OutcomeCompleted || len(psrPools(t, s).Entries) != 0 {
					t.Fatalf("pinned sweep %+v %v %+v", out, e, psrPools(t, s).Entries)
				}
				raw, e := os.ReadFile(marker)
				if e != nil || string(raw) != ext+"\n"+ext+"\n" {
					t.Fatalf("cleanup and reset not run in the pinned worktree: %q %v", raw, e)
				}
				return
			}
			if wire.CodeOf(e) != wire.CodeStaleTree && (out == nil || out.Report == nil || !out.Report.Outcome.HasCode(wire.CodeStaleTree)) {
				t.Fatalf("mismatched pin admitted %+v %v", out, e)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("refused sweep ran a phase: %v", err)
			}
			if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || p.Entries[0].Sweep != nil {
				t.Fatalf("refused sweep changed the member: %+v", p.Entries)
			}
		})
	}
}
