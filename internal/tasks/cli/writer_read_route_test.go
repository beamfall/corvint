package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LGL-V0-807 (V1-4203): with no read checkpoint, `ticket show` and `queue
// status` resume from the writer checkpoint a real write retained, answer as
// the complete audit answers, write nothing, and still refuse a tampered
// receipt.
func TestLGLV0807_TicketShowResumesFromTheWriterCheckpoint(t *testing.T) {
	root, _ := paddedWalkStore(t, 2, 300)
	status := atm(t, root, nil, "queue", "status")
	live := field(status.res.Items[0], "liveAttempts").Arr
	if len(live) == 0 {
		t.Fatalf("no live attempt: %s", status.stdout)
	}
	id := field(live[0], "ticketId").Str
	// The padded receipts are timed up to a few seconds ahead of now, and a
	// write may not be recorded before the head (friction in the tkp-u33
	// build log): wait until the head's second has passed. No CLI clock
	// seam exists for this package.
	if h, err := intent.Resolve(root); err == nil {
		if raw, err := os.ReadFile(filepath.Join(h.StateDir, "head.json")); err == nil {
			if hd, err := snapshot.DecodeHead(raw); err == nil {
				name, _ := snapshot.ReceiptName(hd.LastSeq.Uint64())
				if rb, err := os.ReadFile(filepath.Join(h.StateDir, "receipts", name)); err == nil {
					if rc, _, _, err := snapshot.DecodeRetainedReceipt(rb); err == nil {
						if at, err := time.Parse(time.RFC3339, string(rc.RecordedAt)); err == nil {
							if wait := time.Until(at.Add(time.Second)); wait > 0 && wait < 10*time.Second {
								time.Sleep(wait)
							}
						}
					}
				}
			}
		}
	}
	if x := atm(t, root, nil, "ticket", "note", "set", id, "--request-id", "wr-note", "--text", "writer route"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("note set: %s", x.stdout)
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	readCp := journal.CheckpointPath(repo.StateDir)
	writerFiles := []string{journal.WriterCheckpointPath(repo.StateDir), journal.WriterBasePath(repo.StateDir)}
	for _, p := range writerFiles {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("the write retained no writer checkpoint: %v", err)
		}
	}
	if err := os.Remove(readCp); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(root, ".git", "taskman")
	scrubbed := func(x run) []byte {
		v, err := wire.Parse(x.stdout)
		if err != nil {
			t.Fatal(err)
		}
		var scrub func(v wire.Value)
		scrub = func(v wire.Value) {
			switch v.Kind {
			case wire.KindObject:
				for _, k := range []string{"journalAudit", "journalAuditCheckpoint", "observedAt", "holderStatus"} {
					if _, ok := v.Obj.Get(k); ok {
						v.Obj.Set(k, wire.String(""))
					}
				}
				for _, k := range v.Obj.SortedKeys() {
					c, _ := v.Obj.Get(k)
					scrub(c)
				}
			case wire.KindArray:
				for _, c := range v.Arr {
					scrub(c)
				}
			}
		}
		scrub(v)
		return wire.Encode(v)
	}
	reads := func(wantCheckpoint, wantMode string) [][]byte {
		t.Helper()
		before := fixture.TreeSnapshot(t, stateDir)
		var out [][]byte
		for _, args := range [][]string{{"queue", "status"}, {"ticket", "show", id}} {
			x := atm(t, root, nil, args...)
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %s", args, x.stdout)
			}
			if args[0] == "queue" {
				item := x.res.Items[0]
				if got := field(item, "journalAuditCheckpoint").Str; got != wantCheckpoint {
					t.Fatalf("journalAuditCheckpoint %q, want %q", got, wantCheckpoint)
				}
				if got := field(item, "journalAudit").Str; got != wantMode {
					t.Fatalf("journalAudit %q, want %q", got, wantMode)
				}
			}
			out = append(out, scrubbed(x))
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, stateDir)) {
			t.Fatal("a read changed the state tree")
		}
		if _, err := os.Lstat(readCp); !os.IsNotExist(err) {
			t.Fatalf("a read retained a read checkpoint: %v", err)
		}
		return out
	}
	resumed := reads("WRITER_CHECKPOINT", journal.ModeWriter)

	// The same reads with the writer checkpoint moved aside pay the complete
	// audit and must say the same thing.
	aside := t.TempDir()
	move := func(from, to string) {
		t.Helper()
		for _, p := range writerFiles {
			if err := os.Rename(filepath.Join(from, filepath.Base(p)), filepath.Join(to, filepath.Base(p))); err != nil {
				t.Fatal(err)
			}
		}
	}
	move(filepath.Dir(writerFiles[0]), aside)
	complete := reads("ABSENT", journal.ModeFull)
	for i := range resumed {
		if !bytes.Equal(resumed[i], complete[i]) {
			t.Fatalf("read %d differs between the writer-checkpoint route and the complete audit:\n%s\n%s", i, resumed[i], complete[i])
		}
	}
	move(aside, filepath.Dir(writerFiles[0]))

	// Tamper with the head receipt, the one the checkpoint names: the read
	// refuses as the complete audit does.
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil {
		t.Fatal(err)
	}
	name, _ := snapshot.ReceiptName(head.LastSeq.Uint64())
	rp := filepath.Join(repo.StateDir, "receipts", name)
	rb, err := os.ReadFile(rp)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(rb, []byte(`"wr-note"`), []byte(`"wr-nota"`), 1)
	if bytes.Equal(rb, tampered) {
		t.Fatal("request id not found in the head receipt")
	}
	if err := os.WriteFile(rp, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	withWriter := atm(t, root, nil, "ticket", "show", id)
	move(filepath.Dir(writerFiles[0]), aside)
	without := atm(t, root, nil, "ticket", "show", id)
	if withWriter.res.Outcome == wire.OutcomeOK || without.res.Outcome == wire.OutcomeOK {
		t.Fatalf("a tampered receipt was accepted: with %s, without %s", withWriter.stdout, without.stdout)
	}
	if !equalCodes(withWriter.res.Codes, without.res.Codes) {
		t.Fatalf("refusal codes differ: with %v, without %v", withWriter.res.Codes, without.res.Codes)
	}
}

func equalCodes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
