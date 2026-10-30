package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0060_WritersRetainACheckpointReadsResumeFromIt drives real writers
// and the real CLI: writers leave the derived checkpoint (CAL-V0-060), reads
// resume from it and report so, never write it, and give the same answer as
// the complete audit they fall back to (CAL-V0-061).
func TestCALV0060_WritersRetainACheckpointReadsResumeFromIt(t *testing.T) {
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
	headRaw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		t.Fatal(err)
	}
	if cp.QueueID != head.QueueID || cp.Seq.Uint64() < 1 || cp.Seq.Uint64() > head.LastSeq.Uint64() {
		t.Fatalf("checkpoint seq %s for head %s", cp.Seq, head.LastSeq)
	}
	if entries, err := filepath.Glob(path + ".tmp*"); err != nil || len(entries) != 0 {
		t.Fatalf("checkpoint temporaries left behind: %v %v", entries, err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	reads := [][]string{{"queue", "status"}, {"plan", "preview"}, {"receipt", "audit"}, {"pending"}}
	mode := func(x run) string {
		if len(x.res.Items) == 0 || x.res.Items[0].Obj == nil {
			return ""
		}
		return field(x.res.Items[0], "journalAudit").Str
	}
	// normalized drops the member that names how the audit ran and the
	// wall-clock holder observation, which may tick between two reads.
	var scrub func(v wire.Value)
	scrub = func(v wire.Value) {
		switch v.Kind {
		case wire.KindObject:
			for _, k := range []string{"journalAudit", "observedAt", "holderStatus"} {
				if _, ok := v.Obj.Get(k); ok {
					v.Obj.Set(k, wire.String(""))
				}
			}
			for _, k := range v.Obj.SortedKeys() {
				child, _ := v.Obj.Get(k)
				scrub(child)
			}
		case wire.KindArray:
			for _, child := range v.Arr {
				scrub(child)
			}
		}
	}
	normalized := func(x run) []byte {
		v, err := wire.Parse(x.stdout)
		if err != nil {
			t.Fatal(err)
		}
		scrub(v)
		return wire.Encode(v)
	}
	fast := make([][]byte, len(reads))
	for i, args := range reads {
		x := atm(t, root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s %v", args, x.res.Outcome, x.res.Codes)
		}
		fast[i] = normalized(x)
		if args[0] == "queue" && mode(x) != journal.ModeCheckpoint {
			t.Fatalf("queue status audit mode %q", mode(x))
		}
	}
	after, err := os.Lstat(path)
	if err != nil || !after.ModTime().Equal(info.ModTime()) || !os.SameFile(info, after) {
		t.Fatalf("a read replaced the checkpoint: %v", err)
	}
	if now, err := os.ReadFile(path); err != nil || !bytes.Equal(now, raw) {
		t.Fatalf("a read changed the checkpoint: %v", err)
	}

	// Unusable checkpoints cost one complete audit and change no answer.
	for _, unusable := range []struct {
		name    string
		replace func()
	}{
		{"corrupt", func() { os.WriteFile(path, []byte("{"), 0o644) }},
		{"foreign", func() {
			other := *cp
			other.ReceiptSha256 = wire.Sum([]byte("other"))
			os.WriteFile(path, other.Encode(), 0o644)
		}},
		{"absent", func() { os.Remove(path) }},
	} {
		name := unusable.name
		unusable.replace()
		for i, args := range reads {
			x := atm(t, root, nil, args...)
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%s %v: %s %v", name, args, x.res.Outcome, x.res.Codes)
			}
			if args[0] == "queue" && mode(x) != journal.ModeFull {
				t.Fatalf("%s: queue status audit mode %q", name, mode(x))
			}
			if !bytes.Equal(normalized(x), fast[i]) {
				t.Fatalf("%s %v: answer differs from the checkpoint read", name, args)
			}
		}
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("a read recreated the checkpoint: %v", err)
	}
}
