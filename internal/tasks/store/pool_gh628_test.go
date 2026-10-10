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
// enforced at the declared value. With the injected 1 ms second the old 300 s
// limit is 0.3 s and the new 3600 s limit is 3.6 s: a 1.5 s command must pass
// (a runner clamping to 300 would kill it) and a 30 s command must be killed
// with TIMEOUT quarantine.
func TestPSRV0011ExtendedBoundEnforced(t *testing.T) {
	restore := store.SetPoolCommandSecondForTest(time.Millisecond)
	defer restore()
	for _, tc := range []struct{ sleep, reason string }{{"1.5", "EXIT_ZERO;"}, {"30", "TIMEOUT;"}} {
		t.Run(tc.sleep, func(t *testing.T) {
			s := gh628Health(t, obj("argv", wire.Strings([]string{"/bin/sleep", tc.sleep}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3600")))
			started := time.Now()
			rep, e := store.PoolCommand(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "long-health", Root: s.root, Lease: transaction.LeaseRequest{Member: "a"}}, "health")
			if e != nil || rep == nil || !rep.Unretryable {
				t.Fatalf("health %+v %v", rep, e)
			}
			if elapsed := time.Since(started); elapsed > 20*time.Second {
				t.Fatalf("bound not enforced: %v", elapsed)
			}
			entries := psrPools(t, s).Entries
			if len(entries) != 1 || entries[0].State != "QUARANTINED" || !strings.HasPrefix(entries[0].Reason, tc.reason) {
				t.Fatalf("want %s, got %+v", tc.reason, entries)
			}
		})
	}
}

// gh628PinCalls installs a pin step hook that calls act(call, step), where call
// counts proofs: 1 preparation observer, 2 pre-launch, 3 post-exit.
func gh628PinCalls(t *testing.T, act func(call int, step string) error) {
	t.Helper()
	call := 0
	t.Cleanup(store.SetPoolPinStepForTest(func(step string) error {
		if step == "identity" {
			call++
		}
		return act(call, step)
	}))
}
func gh628PinnedHealth(t *testing.T, script string) (*leaseStore, string, string, string) {
	t.Helper()
	ext, old, head := gh628External(t)
	marker := filepath.Join(t.TempDir(), "ran")
	s := gh628Health(t, obj("argv", wire.Strings([]string{"/bin/sh", "-c", strings.ReplaceAll(script, "MARKER", marker)}), "cwd", gh628Pin(ext, head), "env", wire.Array(), "timeoutSeconds", str("3600")))
	return s, ext, old, marker
}
func gh628RunHealth(t *testing.T, s *leaseStore) (*store.Report, error) {
	t.Helper()
	return store.PoolCommand(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "pinned-health", Root: s.root, Lease: transaction.LeaseRequest{Member: "a"}}, "health")
}

// PSR-V0-012: the clean status is bound to the pinned revision by a second
// HEAD read, and the proved directory identity is re-checked immediately
// before process start. A revision swapped in between HEAD and status, at
// preparation or pre-launch, and a directory swapped for an alias after the
// proof, all run nothing.
func TestPSRV0012PinBoundToLaunch(t *testing.T) {
	for _, tc := range []struct {
		name, step string
		call       int
		refused    bool
	}{
		{"revision-swap-preparation", "head", 1, true},
		{"revision-swap-prelaunch", "head", 2, false},
		{"directory-swap-before-start", "validated", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, ext, old, marker := gh628PinnedHealth(t, "echo ran > MARKER")
			decoy := t.TempDir()
			gh628PinCalls(t, func(call int, step string) error {
				if call != tc.call || step != tc.step {
					return nil
				}
				if step == "head" {
					gitRun(t, ext, "checkout", "-q", "--detach", old)
					return nil
				}
				if e := os.Rename(ext, ext+".moved"); e != nil {
					t.Fatal(e)
				}
				return os.Symlink(decoy, ext)
			})
			rep, e := gh628RunHealth(t, s)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("swapped pin ran the command: %v", err)
			}
			if tc.refused {
				if wire.CodeOf(e) != wire.CodeStaleTree && (rep == nil || !rep.Outcome.HasCode(wire.CodeStaleTree)) {
					t.Fatalf("want STALE_TREE, got %+v %v", rep, e)
				}
				return
			}
			if e != nil || rep == nil || !rep.Unretryable {
				t.Fatalf("health %+v %v", rep, e)
			}
			if entries := psrPools(t, s).Entries; len(entries) != 1 || entries[0].State != "QUARANTINED" || !strings.HasPrefix(entries[0].Reason, "SOURCE_CHANGED;") {
				t.Fatalf("swap not recorded: %+v", entries)
			}
		})
	}
}

// PSR-V0-012/PSR-V0-004: the post-exit pin proof is detection only. A bounded
// probe TIMEOUT/INTERRUPTED keeps the command's own terminal failure and only
// replaces a success it cannot confirm; SOURCE_CHANGED requires shown drift.
func TestPSRV0012PostExitClassification(t *testing.T) {
	restore := store.SetPoolCommandSecondForTest(time.Millisecond)
	defer restore()
	for _, tc := range []struct{ name, script, inject, reason string }{
		{"timeout-kept", "sleep 30", "INTERRUPTED", "TIMEOUT;"},
		{"exit-nonzero-kept", "exit 3", "TIMEOUT", "EXIT_NONZERO;"},
		{"success-unconfirmed", "true", "TIMEOUT", "TIMEOUT;"},
		{"shown-drift", "git -c user.name=t -c user.email=t@example.com commit -q --allow-empty -m drift", "", "SOURCE_CHANGED;"},
		{"no-drift", "true", "", "EXIT_ZERO;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, _ := gh628PinnedHealth(t, tc.script)
			gh628PinCalls(t, func(call int, step string) error {
				if tc.inject != "" && call == 3 && step == "identity" {
					return store.PoolBoundedProbeErrorForTest(tc.inject)
				}
				return nil
			})
			rep, e := gh628RunHealth(t, s)
			if e != nil || rep == nil || !rep.Unretryable {
				t.Fatalf("health %+v %v", rep, e)
			}
			if entries := psrPools(t, s).Entries; len(entries) != 1 || !strings.HasPrefix(entries[0].Reason, tc.reason) {
				t.Fatalf("want %s, got %+v", tc.reason, entries)
			}
		})
	}
}

// PSR-V0-012: health runs in a pinned external worktree only while its HEAD is
// the pinned revision and its inputs are clean; every mismatch refuses before
// preparation with a named code and runs nothing.
func TestPSRV0012PinnedExternalCwd(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
				// Each phase adds bounded pin probes before and after it, all
				// inside the shared member deadline.
				reuse.Obj.Set("timeoutSeconds", str("60"))
				member.Obj.Set("cleanup", obj("argv", wire.Strings([]string{"/bin/sh", "-c", "pwd >> " + marker}), "env", wire.Strings(nil), "cwd", gh628Pin(ext, revision), "timeoutSeconds", str("3600")))
			})
			out, e := store.PoolSweep(context.Background(), s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "pinned-sweep", Root: s.root, TimeoutSeconds: "120"})
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

// PSR-V0-012: verify may expect a nonzero exit, so a bounded post-exit proof
// failure must still fail the phase. With expectExit 7 and matching stdout, a
// sweep whose verify post-exit probe times out (member deadline still live)
// keeps the member quarantined; the control without the fault frees it.
func TestPSRV0012ExpectedNonzeroNeedsProof(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fault bool
		exit  string
	}{{"control", false, "7"}, {"bounded-post-probe", true, "7"}, {"failed-exit-kept", true, "3"}} {
		fault := tc.fault
		t.Run(tc.name, func(t *testing.T) {
			ext, _, head := gh628External(t)
			ran := filepath.Join(t.TempDir(), "verify-ran")
			s, _ := psrFixtureConfigured(t, "true", "touch "+ran+"; printf ok; exit "+tc.exit, "ok", "1", false, func(v wire.Value) {
				pools, _ := v.Obj.Get("pools")
				config, _ := pools.Arr[0].Obj.Get("memberConfig")
				member, _ := config.Obj.Get("a")
				reuse, _ := member.Obj.Get("safeReuse")
				reuse.Obj.Set("cwd", gh628Pin(ext, head))
				reuse.Obj.Set("timeoutSeconds", str("60"))
				verify, _ := reuse.Obj.Get("verify")
				verify.Obj.Set("expectExit", str("7"))
			})
			injected := false
			t.Cleanup(store.SetPoolPinStepForTest(func(step string) error {
				if _, e := os.Stat(ran); fault && !injected && step == "identity" && e == nil {
					injected = true
					return store.PoolBoundedProbeErrorForTest("TIMEOUT")
				}
				return nil
			}))
			out, e := store.PoolSweep(context.Background(), s.repo, operator(), store.PoolSweepChoice{QueueID: fixture.QueueID, RequestID: "nonzero-sweep", Root: s.root, TimeoutSeconds: "120"})
			if e != nil || out == nil || out.Report == nil {
				t.Fatalf("sweep %+v %v", out, e)
			}
			if _, err := os.Stat(ran); err != nil {
				t.Fatalf("verify did not run: %v", err)
			}
			entries := psrPools(t, s).Entries
			if !fault {
				if len(entries) != 0 {
					t.Fatalf("control not freed: %+v", entries)
				}
				return
			}
			if !injected || len(entries) != 1 || entries[0].State != "QUARANTINED" {
				t.Fatalf("unproved verify freed the member (injected %v): %+v", injected, entries)
			}
			// A phase that fails its own predicate keeps its exit class: a
			// bounded post-exit probe replaces only a result that could pass.
			want := map[string]string{"7": "TIMEOUT", "3": "EXIT_NONZERO"}[tc.exit]
			if !strings.Contains(entries[0].Reason, want) {
				t.Fatalf("want %s, got %+v", want, entries[0])
			}
		})
	}
}
