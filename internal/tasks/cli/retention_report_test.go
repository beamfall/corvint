package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LGL-V0-803 (V1-1058): a read names why its journal audit was complete,
// from the read checkpoint as it found it, and writes nothing.
func TestLGLV0803_ReadsNameWhyTheAuditWasComplete(t *testing.T) {
	root, _ := leaseCLIStore(t, 2, time.Now().UTC().Truncate(time.Second))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	path := journal.CheckpointPath(repo.StateDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("writers retained no checkpoint: %v", err)
	}
	cp, err := journal.DecodeCheckpoint(raw)
	if err != nil {
		t.Fatal(err)
	}
	read := func(want, wantMode string) {
		t.Helper()
		stateDir := filepath.Join(root, ".git", "taskman")
		before := fixture.TreeSnapshot(t, stateDir)
		cpBefore, _ := os.ReadFile(path)
		for _, args := range [][]string{{"queue", "status"}, {"doctor"}} {
			x := atm(t, root, nil, args...)
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %s %v", args, x.res.Outcome, x.res.Codes)
			}
			item := x.res.Items[0]
			if args[0] == "doctor" {
				item = field(item, "scan")
			}
			if got := field(item, "journalAuditCheckpoint").Str; got != want {
				t.Fatalf("%v: journalAuditCheckpoint %q, want %q", args, got, want)
			}
			if got := field(item, "journalAudit").Str; got != wantMode {
				t.Fatalf("%v: journalAudit %q, want %q", args, got, wantMode)
			}
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, stateDir)) {
			t.Fatal("a read changed the state tree")
		}
		if cpAfter, _ := os.ReadFile(path); !bytes.Equal(cpBefore, cpAfter) {
			t.Fatal("a read changed the checkpoint")
		}
	}
	read("USED", journal.ModeCheckpoint)
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	read("MALFORMED", journal.ModeFull)
	other := *cp
	other.ReceiptSha256 = wire.Sum([]byte("other"))
	if err := os.WriteFile(path, other.Encode(), 0o600); err != nil {
		t.Fatal(err)
	}
	read("NOT_APPLIED", journal.ModeFull)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	read("ABSENT", journal.ModeFull)
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("a read recreated the checkpoint: %v", err)
	}
}

// LGL-V0-803 (V1-1058): a writer whose checkpoint retention fails commits
// exactly as it would have, and its result says the retention failed; the
// next write that retains it says nothing.
func TestLGLV0803_WriterReportsAFailedRetention(t *testing.T) {
	root, _ := ergStore(t)
	ticket, _ := tcdTicket(t, root, "kept", "")
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	path := journal.CheckpointPath(repo.StateDir)
	headSeq := func() uint64 {
		raw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
		if err != nil {
			t.Fatal(err)
		}
		h, err := snapshot.DecodeHead(raw)
		if err != nil {
			t.Fatal(err)
		}
		return h.LastSeq.Uint64()
	}
	// A non-empty directory where the checkpoint goes: no rename can
	// replace it.
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path, "blocker"), 0o700); err != nil {
		t.Fatal(err)
	}
	before := headSeq()
	x := atm(t, root, nil, "ticket", "note", "set", ticket, "--request-id", "note-blocked", "--text", "first note")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("note set with retention blocked: %s", x.stdout)
	}
	if headSeq() != before+1 {
		t.Fatalf("head %d after one write from %d", headSeq(), before)
	}
	warned := false
	for _, w := range x.res.Warnings {
		if strings.HasPrefix(w, "CHECKPOINT_RETENTION_FAILED: derived file "+filepath.Base(path)+" ") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("no retention warning for %s: %q", filepath.Base(path), x.res.Warnings)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	x = atm(t, root, nil, "ticket", "note", "set", ticket, "--request-id", "note-clear", "--text", "second note")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("note set: %s", x.stdout)
	}
	for _, w := range x.res.Warnings {
		if strings.HasPrefix(w, "CHECKPOINT_RETENTION_FAILED") {
			t.Fatalf("retention warning after a retained write: %q", w)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the clear write retained no checkpoint: %v", err)
	}
}
