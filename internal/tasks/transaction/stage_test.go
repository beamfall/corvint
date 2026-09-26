package transaction

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func maximalDescriptor(op string) Descriptor {
	q := "queue:a:" + strings.Repeat("q", 120)
	if op == KeepJournal || op == AdoptFile {
		q = "queue:a:" + strings.Repeat("q", 54)
	}
	d := Descriptor{QueueID: q, Operation: op, RequestID: strings.Repeat(`"`, 64), RequestSha256: wire.Sum([]byte("request")), RecordedAt: timestamp}
	if op != Init {
		d.Base = &Base{LastSeq: "999999", LastReceiptSha256: wire.Sum([]byte("base"))}
	}
	seq := uint64(1000000)
	if op == Init {
		seq = 1
	}
	name, _ := snapshot.ReceiptName(seq)
	rp, _ := snapshot.RequestPath(d.RequestID)
	add := func(role, target string, n uint64, hash wire.Digest) {
		d.Artifacts = append(d.Artifacts, Description{Role: role, Target: target, Bytes: wire.SizeOf(n), Sha256: hash})
	}
	hash := wire.Sum([]byte("bytes"))
	receiptBytes := uint64(1048576)
	if op == Unpause {
		receiptBytes = 1683
	}
	add("HEAD", "head.json", 4096, hash)
	add("RECEIPT", "receipts/"+name, receiptBytes, hash)
	requestBytes := uint64(563)
	if op == Init {
		requestBytes = 551
	}
	if op == KeepJournal || op == AdoptFile {
		requestBytes = 579
	}
	add("POST", rp, requestBytes, hash)
	switch op {
	case Init:
		for j, p := range []struct {
			path string
			n    uint64
		}{{"VERSION", 16}, {"intent/queue.json", 1048576}, {"intent/policy.json", 262144}} {
			h := wire.Sum([]byte(fmt.Sprint(j)))
			add("POST", p.path, p.n, h)
			add("EVIDENCE", "evidence/"+string(h), p.n, h)
		}
		add("POST", "reservations.json", 194, hash)
		add("POST", "pinned/"+string(hash)+".json", 8465, hash)
	case Pause:
		add("POST", "barrier.json", 4096, hash)
	case KeepJournal, AdoptFile:
		add("POST", "intent/tickets/"+strings.Repeat("t", 64)+".json", 131072, hash)
		add("EVIDENCE", "evidence/"+string(hash), 131072, hash)
		if op == KeepJournal {
			h := wire.Sum([]byte("discard"))
			add("POST", "evidence/"+string(h), 131072, h)
		}
	}
	sort.Slice(d.Artifacts, func(i, j int) bool {
		a, b := d.Artifacts[i], d.Artifacts[j]
		if a.Role != b.Role {
			return a.Role < b.Role
		}
		return a.Target < b.Target
	})
	for i := range d.Artifacts {
		d.Artifacts[i].Slot = slotName(i)
	}
	return d
}
func TestTMV0002_AS10_UnpauseActualCodecParity(t *testing.T) {
	in, _ := initialized(t)
	p := Model(admin(Pause, "pause"), in)
	in = commitModel(t, in, p.Plan)
	// Seven-digit receipt sequence and twenty-digit generation are codec-domain
	// upper witnesses only. No million-entry inventory or capacity grant is used.
	h, _ := snapshot.DecodeHead(in.Head)
	h.LastSeq = "999999"
	h.Generation = "18446744073709551615"
	r := admin(Unpause, strings.Repeat(`"`, 64))
	r.Actor.ID = strings.Repeat(`"`, 64)
	d, e := Digest(r)
	if e != nil {
		t.Fatal(e)
	}
	plan, out, e := freeze(r, d, timestamp, in.Inventory, h, map[string][]byte{"barrier.json": nil}, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	rp, _ := snapshot.RequestPath(r.RequestID)
	raw, e := out.Encode()
	if e != nil {
		t.Fatal(e)
	}
	if len(plan.receipt) != 1683 || len(plan.posts[rp]) != 563 || len(raw) != 308 {
		t.Fatalf("codec parity: %d/%d/%d", len(plan.receipt), len(plan.posts[rp]), len(raw))
	}
	if UnpauseTemporaryBytes != 1683+563+4096+2*1098 {
		t.Fatal("Pb derivation")
	}
	// Actual head codec with a feasible PathText reaches its declared 4096 cap.
	h.PrimaryWorktree = "/"
	baseline := len(wire.EncodeFile(h.Value()))
	h.PrimaryWorktree = "/" + strings.Repeat("x", 4096-baseline)
	head := wire.EncodeFile(h.Value())
	if len(head) != 4096 {
		t.Fatal(len(head))
	}
	if _, e = snapshot.DecodeHead(head); e != nil {
		t.Fatal(e)
	}
	h.PrimaryWorktree += "x"
	if _, e = snapshot.DecodeHead(wire.EncodeFile(h.Value())); e == nil {
		t.Fatal("head cap+1")
	}
}
func stageObservation(p *Plan) StageObservation {
	o := StageObservation{Inventory: p.base, BaseState: "UNCHANGED", ReceiptInventory: "COMPLETE_NO_NEXT", Premise: FixtureNoRuntime, Files: []StageFile{{Name: "active.json", Type: "REGULAR", Data: p.Descriptor()}}}
	if p.baseHead != nil {
		o.CurrentHead = wire.EncodeFile(p.baseHead.Value())
	}
	return o
}
func TestTMV0009_AS11_StagePreparationCorruptionAndRedo(t *testing.T) {
	_, p := initialized(t)
	o := stageObservation(p)
	got, e := ClassifyStage(o, p)
	if e != nil || got.Kind != "PreparationPending" {
		t.Fatal(got, e)
	}
	for _, a := range p.Artifacts() {
		o.Files = append(o.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: a.Data})
	}
	got, e = ClassifyStage(o, p)
	if e != nil || got.Kind != "FullPlanPrepared" {
		t.Fatal(got, e)
	}
	for i := 1; i < len(o.Files); i++ {
		x := o
		x.Files = append([]StageFile(nil), o.Files...)
		x.Files[i].Data = bytes.Clone(o.Files[i].Data)
		x.Files[i].Data[0] ^= 1
		if _, e = ClassifyStage(x, p); e == nil {
			t.Fatal("complete wrong hash")
		}
		x.Files[i].Data = x.Files[i].Data[:len(x.Files[i].Data)-1]
		got, e = ClassifyStage(x, p)
		if e != nil || got.Kind != "PreparationPending" {
			t.Fatal("partial", got, e)
		}
	}
	o.Inventory = p.base.clone()
	for _, a := range p.artifacts {
		if a.Role == "RECEIPT" || strings.HasPrefix(a.Target, "evidence/") {
			if e = o.Inventory.put(entry(a), true); e != nil {
				t.Fatal(e)
			}
		}
	}
	o.ReceiptInventory = "MATCHING_NEXT"
	o.LinkedReceipt = p.Receipt()
	o.Projections = "PRE_OR_POST"
	got, e = ClassifyStage(o, p)
	if e != nil || got.Kind != "RedoPending" {
		t.Fatal(got, e)
	}
	// Missing slots do not mint another receipt on pending redo.
	o.Files = o.Files[:1]
	got, e = ClassifyStage(o, p)
	if e != nil || got.Kind != "RedoPending" {
		t.Fatal(got, e)
	}
	c, e := CheckCapacity(p)
	if e != nil {
		t.Fatal(e)
	}
	o.Inventory = c.Final
	o.ReceiptInventory = "MATCHING_COMPLETED"
	o.CurrentHead = p.Head()
	o.Projections = "POST"
	got, e = ClassifyStage(o, p)
	if e != nil || got.Kind != "CompletedCleanup" {
		t.Fatal(got, e)
	}
	o.LinkedReceipt = []byte("wrong")
	if _, e = ClassifyStage(o, p); e == nil {
		t.Fatal("receipt mismatch")
	}
	o = stageObservation(p)
	o.Files[0].Data = []byte("{malformed")
	if _, e = ClassifyStage(o, p); e == nil {
		t.Fatal("malformed active authority")
	}
}
func TestTMV0009_AS27_TempOnlyEveryLengthAndCleanupDurability(t *testing.T) {
	for n := 0; n <= 2422; n++ {
		o := StageObservation{Inventory: &Inventory{files: map[string]archive.FileEntry{}, dirs: map[string]bool{}}, BaseState: "UNCHANGED", ReceiptInventory: "COMPLETE_NO_NEXT", Premise: FixtureNoRuntime, Files: []StageFile{{Name: "active.json.tmp", Type: "REGULAR", Data: bytes.Repeat([]byte("!"), n)}}}
		got, e := ClassifyStage(o, nil)
		if e != nil || got.Kind != "TempPreparationAbortable" {
			t.Fatal(n, got, e)
		}
		before, e := Cleanup(o, CleanupState{Removed: []string{"active.json.tmp"}})
		if e != nil || before.TemporaryBytes != uint64(n) || before.Entries != 2 || before.FreshDescriptor != "HELD" {
			t.Fatal(n, before, e)
		}
		after, e := Cleanup(o, CleanupState{Removed: []string{"active.json.tmp"}, PayloadSynced: true})
		if e != nil || after.TemporaryBytes != 0 || after.Entries != 1 || after.FreshDescriptor != "MODEL_READY" {
			t.Fatal(n, after, e)
		}
	}
	for _, change := range []func(*StageObservation){func(o *StageObservation) { o.Files[0].Data = make([]byte, 2423) }, func(o *StageObservation) { o.Files[0].Type = "SYMLINK" }, func(o *StageObservation) { o.Files = append(o.Files, StageFile{Name: "a00", Type: "REGULAR"}) }, func(o *StageObservation) { o.Files = append(o.Files, StageFile{Name: "unknown", Type: "REGULAR"}) }, func(o *StageObservation) { o.BaseState = "CHANGED" }, func(o *StageObservation) { o.ReceiptInventory = "UNKNOWN" }, func(o *StageObservation) { o.ReceiptInventory = "MATCHING_NEXT" }, func(o *StageObservation) { o.Files[0].Name = "active.json" }} {
		o := StageObservation{Inventory: &Inventory{files: map[string]archive.FileEntry{}, dirs: map[string]bool{}}, BaseState: "UNCHANGED", ReceiptInventory: "COMPLETE_NO_NEXT", Premise: FixtureNoRuntime, Files: []StageFile{{Name: "active.json.tmp", Type: "REGULAR", Data: []byte("{partial")}}}
		change(&o)
		if _, e := ClassifyStage(o, nil); e == nil {
			t.Fatal("unsafe temp exception")
		}
	}
}
func TestTMV0009_AS27_ExhaustiveSlotSubsetsAndAbortCleanup(t *testing.T) {
	// 2^11 bounded structural slot subsets; no production-size bodies allocated.
	d := maximalDescriptor(Init)
	raw, e := d.Encode()
	if e != nil {
		t.Fatal(e)
	}
	for mask := 0; mask < 1<<len(d.Artifacts); mask++ {
		o := StageObservation{Inventory: &Inventory{files: map[string]archive.FileEntry{}, dirs: map[string]bool{}}, BaseState: "UNCHANGED", ReceiptInventory: "COMPLETE_NO_NEXT", Premise: FixtureNoRuntime, Files: []StageFile{{Name: "active.json", Type: "REGULAR", Data: raw}}}
		removed := []string{}
		for j, a := range d.Artifacts {
			if mask&(1<<j) != 0 {
				o.Files = append(o.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: []byte("x")})
				removed = append(removed, a.Slot)
			}
		}
		got, e := ClassifyStage(o, nil)
		if e != nil || got.Kind != "PreparationPending" {
			t.Fatal(mask, got, e)
		}
		// Every removal prefix retains its charge until payload fsync.
		for n := 0; n <= len(removed); n++ {
			c, e := Cleanup(o, CleanupState{Removed: removed[:n]})
			if e != nil || c.Entries != uint64(len(o.Files)+1) || c.FreshDescriptor != "HELD" {
				t.Fatal(mask, n, c, e)
			}
		}
		for _, synced := range []bool{false, true} {
			c, e := Cleanup(o, CleanupState{Removed: removed, PayloadSynced: true, DescriptorUnlinked: true, DescriptorSynced: synced})
			if e != nil {
				t.Fatal(mask, e)
			}
			if synced {
				if c.PromisedBytes != 0 || c.Entries != 1 || c.FreshDescriptor != "MODEL_READY" {
					t.Fatal(c)
				}
			} else if c.PromisedBytes == 0 || c.Entries != 2 || c.FreshDescriptor != "HELD" {
				t.Fatal(c)
			}
		}
		if _, e = Cleanup(o, CleanupState{DescriptorUnlinked: true}); e == nil {
			t.Fatal("early descriptor unlink")
		}
	}
}

func TestTMV0002_AS10_OwnedTempAndPartialArtifactBounds(t *testing.T) {
	in := paused(t)
	result := Model(admin(Unpause, "unpause"), in)
	p := result.Plan
	o := stageObservation(p)
	o.Files = append(o.Files, StageFile{Name: "active.json.tmp", Type: "REGULAR", Data: p.Descriptor()})
	for _, a := range p.Artifacts() {
		o.Files = append(o.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: a.Data})
	}
	got, e := ClassifyStage(o, p)
	if e != nil || got.TemporaryBytes > 8538 {
		t.Fatal(got, e)
	}
	o.Files[1].Data = bytes.Repeat([]byte("!"), len(p.Descriptor()))
	if _, e = ClassifyStage(o, p); e == nil {
		t.Fatal("complete wrong descriptor temp")
	}
	o.Files[1].Data = o.Files[1].Data[:len(o.Files[1].Data)-1]
	if _, e = ClassifyStage(o, p); e != nil {
		t.Fatal("partial owned descriptor temp", e)
	}
	o.Files[1].Data = bytes.Repeat([]byte("!"), 2422)
	if _, e = ClassifyStage(o, p); e == nil {
		t.Fatal("global temp cap incorrectly used under active UNPAUSE")
	}
	o = stageObservation(p)
	a := p.Artifacts()[0]
	o.Files = append(o.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: make([]byte, len(a.Data)+1)})
	if _, e = ClassifyStage(o, p); e == nil {
		t.Fatal("artifact cap+1")
	}
	o.Files[1].Type = "DIRECTORY"
	if _, e = ClassifyStage(o, p); e == nil {
		t.Fatal("directory slot")
	}
}
