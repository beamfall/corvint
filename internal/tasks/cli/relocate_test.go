package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// moveRepo renames a fixture repository's primary worktree to a sibling and
// returns the new path, as an operator moving the checkout would.
func moveRepo(t *testing.T, root, name string) string {
	t.Helper()
	moved := filepath.Join(filepath.Dir(root), name)
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	return moved
}

func headPrimary(t *testing.T, stateDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(stateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := snapshot.DecodeHead(raw)
	if err != nil {
		t.Fatal(err)
	}
	return h.PrimaryWorktree
}

// relocateDescriptor returns the one taskman-relocate/0 descriptor the store
// pins, failing unless exactly one exists.
func relocateDescriptor(t *testing.T, stateDir string) *snapshot.Relocate {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(stateDir, "pinned", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var found *snapshot.Relocate
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), snapshot.ProfileRelocate) {
			continue
		}
		d, err := snapshot.DecodeRelocate(raw)
		if err != nil || found != nil {
			t.Fatalf("relocate descriptor %s: %v (duplicate=%t)", p, err, found != nil)
		}
		found = d
	}
	if found == nil {
		t.Fatal("no relocate descriptor")
	}
	return found
}

// TRL-V0-001, TRL-V0-002: a moved store refuses ordinary commands and names
// the relocate step; an explicit relocate from the new primary worktree
// commits one RELOCATE receipt naming both paths, re-pins the head, and
// leaves an audit-consistent store that reads and writes at the new path.
func TestTRLV0001_RelocateMovedRepository(t *testing.T) {
	r := receiptFixture(t)
	old := r.Root
	moved := moveRepo(t, old, "moved")
	state := filepath.Join(moved, ".git", "taskman")
	x := atm(t, moved, nil, "queue", "status")
	if !hasCode(x.res, wire.CodeUnsupportedFilesystem) || !strings.Contains(strings.Join(x.res.Warnings, " ")+string(x.stdout), "corvint-tasks relocate --from") {
		t.Fatalf("moved read: %s", x.stdout)
	}
	if x := atm(t, moved, nil, "pause", "--request-id", "p"); !hasCode(x.res, wire.CodeUnsupported) {
		t.Fatalf("moved write: %s", x.stdout)
	}
	before := fixture.TreeSnapshot(t, state)
	for _, args := range [][]string{
		{"relocate", "--request-id", "r"},
		{"relocate", "--from", old},
		{"relocate", "--from", old, "--request-id", "r", "--role", "WORKER"},
		{"relocate", "--from", old, "--from", old, "--request-id", "r"},
		{"relocate", "--from", filepath.Join(filepath.Dir(old), "elsewhere"), "--request-id", "r"},
		{"relocate", "--from", moved, "--request-id", "r"},
	} {
		if x := atm(t, moved, nil, args...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%v accepted: %s", args, x.stdout)
		}
	}
	if after := fixture.TreeSnapshot(t, state); len(after) != len(before) {
		t.Fatalf("refused relocations changed the store")
	}
	x = atm(t, moved, nil, "relocate", "--from", old, "--request-id", "relocate-1")
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "receipt").Str == "" {
		t.Fatalf("relocate: %s", x.stdout)
	}
	if !strings.Contains(strings.Join(x.res.Warnings, " "), "NOT_OBSERVED") {
		t.Fatalf("dispatcher check not reported: %v", x.res.Warnings)
	}
	if got := headPrimary(t, state); got != moved {
		t.Fatalf("head primary %q", got)
	}
	d := relocateDescriptor(t, state)
	if d.FromPrimaryWorktree != old || d.ToPrimaryWorktree != moved || d.RequestID != "relocate-1" {
		t.Fatalf("descriptor %+v", d)
	}
	raw, err := os.ReadFile(filepath.Join(state, "receipts", "000000000003.json"))
	if err != nil {
		t.Fatal(err)
	}
	if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Kind != "RELOCATE" {
		t.Fatalf("receipt %v %+v", err, rc)
	}
	audit := atm(t, moved, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("audit: %s", audit.stdout)
	}
	if x := atm(t, moved, nil, "pause", "--request-id", "after"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("write after relocate: %s", x.stdout)
	}
	if x := atm(t, moved, nil, "queue", "status"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("read after relocate: %s", x.stdout)
	}
	// TRL-V0-004: the same request replays; a fresh one refuses, since the
	// head no longer records --from.
	if x := atm(t, moved, nil, "relocate", "--from", old, "--request-id", "relocate-1"); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("replay: %s", x.stdout)
	}
	if x := atm(t, moved, nil, "relocate", "--from", old, "--request-id", "relocate-2"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("second relocation accepted: %s", x.stdout)
	}
}

// TRL-V0-002: relocation is never inferred. A head re-pinned without a
// RELOCATE receipt, or one pinned back to the old path after a RELOCATE,
// differs from the primary the receipt chain records and refuses
// JOURNAL_FORKED.
func TestTRLV0002_HeadRepinWithoutReceiptForks(t *testing.T) {
	rewrite := func(t *testing.T, stateDir, primary string) {
		t.Helper()
		p := filepath.Join(stateDir, "head.json")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		h, err := snapshot.DecodeHead(raw)
		if err != nil {
			t.Fatal(err)
		}
		h.PrimaryWorktree = primary
		fixture.Write(t, p, wire.EncodeFile(h.Value()))
	}
	t.Run("forged", func(t *testing.T) {
		r := receiptFixture(t)
		moved := moveRepo(t, r.Root, "moved")
		rewrite(t, filepath.Join(moved, ".git", "taskman"), moved)
		if x := atm(t, moved, nil, "receipt", "audit"); !hasCode(x.res, wire.CodeJournalForked) {
			t.Fatalf("forged head: %s", x.stdout)
		}
	})
	t.Run("reverted", func(t *testing.T) {
		r := receiptFixture(t)
		old := r.Root
		moved := moveRepo(t, old, "moved")
		if x := atm(t, moved, nil, "relocate", "--from", old, "--request-id", "relocate-1"); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("relocate: %s", x.stdout)
		}
		back := moveRepo(t, moved, filepath.Base(old))
		rewrite(t, filepath.Join(back, ".git", "taskman"), back)
		if x := atm(t, back, nil, "receipt", "audit"); !hasCode(x.res, wire.CodeJournalForked) {
			t.Fatalf("reverted head: %s", x.stdout)
		}
	})
}

// TRL-V0-003: a live attempt's reservation refuses relocation.
func TestTRLV0003_RelocateRefusesLiveAttempt(t *testing.T) {
	root, _ := leaseCLIStore(t, 1, time.Now().UTC().Add(-20*time.Minute))
	moved := moveRepo(t, root, "moved")
	state := filepath.Join(moved, ".git", "taskman")
	before := fixture.TreeSnapshot(t, state)
	x := atm(t, moved, nil, "relocate", "--from", root, "--request-id", "relocate-live")
	if !hasCode(x.res, wire.CodeQuiescenceUnproved) {
		t.Fatalf("live attempt: %s", x.stdout)
	}
	if after := fixture.TreeSnapshot(t, state); len(after) != len(before) || headPrimary(t, state) != root {
		t.Fatal("refused relocation changed the store")
	}
}

// TRL-V0-004: a RELOCATE whose head publication was lost (a crash after the
// receipt and descriptor) is settled by the retry, which re-pins the head
// from the committed descriptor and then replays.
func TestTRLV0004_RelocateRedoRepinsHead(t *testing.T) {
	r := receiptFixture(t)
	old := r.Root
	moved := moveRepo(t, old, "moved")
	state := filepath.Join(moved, ".git", "taskman")
	prior, err := os.ReadFile(filepath.Join(state, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	if x := atm(t, moved, nil, "relocate", "--from", old, "--request-id", "relocate-1"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("relocate: %s", x.stdout)
	}
	fixture.Write(t, filepath.Join(state, "head.json"), prior)
	x := atm(t, moved, nil, "relocate", "--from", old, "--request-id", "relocate-1")
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool || !strings.Contains(strings.Join(x.res.Warnings, " "), "redo") {
		t.Fatalf("redo: %s", x.stdout)
	}
	if got := headPrimary(t, state); got != moved {
		t.Fatalf("head primary after redo %q", got)
	}
	if audit := atm(t, moved, nil, "receipt", "audit"); audit.res.Outcome != wire.OutcomeOK {
		t.Fatalf("audit: %s", audit.stdout)
	}
}

// TRL-V0-006: a copy taken while the store was quiescent binds to its new
// primary worktree by an explicit relocate while the original keeps working;
// a torn copy refuses and names the quiescence requirement.
func TestTRLV0006_RelocateCopiedStore(t *testing.T) {
	r := receiptFixture(t)
	copyTo := func(t *testing.T, name string) string {
		t.Helper()
		dst := filepath.Join(filepath.Dir(r.Root), name)
		if err := os.CopyFS(dst, os.DirFS(r.Root)); err != nil {
			t.Fatal(err)
		}
		return dst
	}
	t.Run("consistent", func(t *testing.T) {
		dst := copyTo(t, "copy")
		if x := atm(t, dst, nil, "relocate", "--from", r.Root, "--request-id", "bind-copy"); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("bind copy: %s", x.stdout)
		}
		if headPrimary(t, filepath.Join(dst, ".git", "taskman")) != dst || headPrimary(t, r.StateDir) != r.Root {
			t.Fatal("copy binding did not re-pin only the copy")
		}
		for _, root := range []string{r.Root, dst} {
			if x := atm(t, root, nil, "queue", "status"); x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%s status: %s", root, x.stdout)
			}
		}
	})
	t.Run("torn", func(t *testing.T) {
		dst := copyTo(t, "torn")
		// The head names a receipt the copy never captured.
		if err := os.Remove(filepath.Join(dst, ".git", "taskman", "receipts", "000000000002.json")); err != nil {
			t.Fatal(err)
		}
		x := atm(t, dst, nil, "relocate", "--from", r.Root, "--request-id", "bind-torn")
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "consistent snapshot") {
			t.Fatalf("torn copy: %s", x.stdout)
		}
	})
}
