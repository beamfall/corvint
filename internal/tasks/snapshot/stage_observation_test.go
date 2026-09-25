package snapshot

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"testing"
)

func TestTMV0002_AS10_StageObservationFiniteShapes(t *testing.T) {
	for _, op := range []string{StageInit, StagePause, StageUnpause, StageKeepJournal, StageAdoptFile} {
		t.Run(op, func(t *testing.T) {
			d := maximalDescriptor(op)
			bodies := map[string][]byte{}
			hashes := map[wire.Digest][]byte{}
			for i := range d.Artifacts {
				a := &d.Artifacts[i]
				b := bytes.Repeat([]byte{byte(i + 1)}, int(a.Bytes.Uint64()))
				if old, ok := hashes[a.Sha256]; ok && len(old) == len(b) {
					b = old
				} else {
					hashes[a.Sha256] = b
				}
				bodies[a.Slot] = b
			}
			// Rebind content-addressed paths after constructing matching INIT pairs.
			for i := range d.Artifacts {
				a := &d.Artifacts[i]
				a.Sha256 = wire.Sum(bodies[a.Slot])
				if a.Role == "EVIDENCE" || bytes.HasPrefix([]byte(a.Target), []byte("evidence/")) {
					a.Target = "evidence/" + string(a.Sha256)
				}
				if bytes.HasPrefix([]byte(a.Target), []byte("pinned/")) {
					a.Target = "pinned/" + string(a.Sha256) + ".json"
				}
			}
			// Re-sort after content-addressed target changes and preserve body pairing.
			byTarget := map[string][]byte{}
			for _, a := range d.Artifacts {
				byTarget[a.Role+"/"+a.Target] = bodies[a.Slot]
			}
			sortStageDescriptions(d.Artifacts)
			bodies = map[string][]byte{}
			for i := range d.Artifacts {
				a := &d.Artifacts[i]
				a.Slot = stageSlot(i)
				bodies[a.Slot] = byTarget[a.Role+"/"+a.Target]
			}
			active, e := d.Encode()
			if e != nil {
				t.Fatal(e)
			}
			// All absent/complete subsets, plus every individual partial and corrupt slot.
			for mask := 0; mask < (1 << len(d.Artifacts)); mask++ {
				fs := []StageFile{{"active.json", active}}
				for i, a := range d.Artifacts {
					if mask&(1<<i) != 0 {
						fs = append(fs, StageFile{a.Slot, bodies[a.Slot]})
					}
				}
				if _, e := ObserveStage(fs); e != nil {
					t.Fatalf("subset %d: %v", mask, e)
				}
			}
			for _, a := range d.Artifacts {
				b := bodies[a.Slot]
				for _, n := range []int{0, len(b) - 1, len(b)} {
					if _, e := ObserveStage([]StageFile{{"active.json", active}, {a.Slot, b[:n]}}); e != nil {
						t.Fatal(e)
					}
				}
				over := append(bytes.Clone(b), 0)
				_, e := ObserveStage([]StageFile{{"active.json", active}, {a.Slot, over}})
				if wire.CodeOf(e) != wire.CodeLimitExceeded {
					t.Fatal(e)
				}
				bad := bytes.Clone(b)
				bad[0] ^= 1
				_, e = ObserveStage([]StageFile{{"active.json", active}, {a.Slot, bad}})
				if wire.CodeOf(e) != wire.CodeJournalForked {
					t.Fatal(e)
				}
			}
		})
	}
}

func sortStageDescriptions(a []StageDescription) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && (a[j].Role < a[j-1].Role || a[j].Role == a[j-1].Role && a[j].Target < a[j-1].Target); j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func TestTMV0008_AS07_StageObservationTempAndClosedLayout(t *testing.T) {
	d := maximalDescriptor(StageUnpause)
	active, e := d.Encode()
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name  string
		files []StageFile
		code  string
	}{
		{"absent", nil, ""}, {"empty", []StageFile{}, ""},
		{"temp empty", []StageFile{{"active.json.tmp", nil}}, ""},
		{"temp malformed", []StageFile{{"active.json.tmp", bytes.Repeat([]byte("x"), 2422)}}, ""},
		{"temp cap", []StageFile{{"active.json.tmp", make([]byte, 2423)}}, wire.CodeLimitExceeded},
		{"N-1", []StageFile{{"active.json", active}, {"active.json.tmp", active[:len(active)-1]}}, ""},
		{"N same", []StageFile{{"active.json", active}, {"active.json.tmp", active}}, ""},
		{"N different", []StageFile{{"active.json", active}, {"active.json.tmp", make([]byte, len(active))}}, wire.CodeJournalForked},
		{"N+1", []StageFile{{"active.json", active}, {"active.json.tmp", make([]byte, len(active)+1)}}, wire.CodeLimitExceeded},
		{"bad active", []StageFile{{"active.json", []byte("{")}}, wire.CodeMalformed},
		{"undeclared", []StageFile{{"a00", nil}}, wire.CodeMalformed},
		{"alias", []StageFile{{"./active.json", active}}, wire.CodeMalformed},
		{"nested", []StageFile{{"a00/x", nil}}, wire.CodeMalformed},
		{"duplicate", []StageFile{{"active.json.tmp", nil}, {"active.json.tmp", nil}}, wire.CodeMalformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, e := ObserveStage(tc.files)
			if wire.CodeOf(e) != tc.code {
				t.Fatalf("%v want %s", e, tc.code)
			}
		})
	}
	fs := make([]StageFile, 14)
	for i := range fs {
		fs[i].Name = fmt.Sprint(i)
	}
	_, e = ObserveStage(fs)
	if wire.CodeOf(e) != wire.CodeLimitExceeded {
		t.Fatal(e)
	}
}

func TestTMV0002_AS10_StageObservationEmptyRaw(t *testing.T) {
	d := maximalDescriptor(StageKeepJournal)
	var slot string
	for i := range d.Artifacts {
		a := &d.Artifacts[i]
		if a.Role == "POST" && bytes.HasPrefix([]byte(a.Target), []byte("evidence/")) {
			a.Bytes = "0"
			a.Sha256 = wire.Sum(nil)
			a.Target = "evidence/" + string(a.Sha256)
			slot = a.Slot
		}
	}
	active, e := d.Encode()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ObserveStage([]StageFile{{"active.json", active}, {slot, nil}}); e != nil {
		t.Fatal(e)
	}
}

func TestTMV0002_AS10_StageObservationDelegatesStrictCodec(t *testing.T) {
	for _, mode := range []string{"version", "closed", "cap"} {
		t.Run(mode, func(t *testing.T) {
			d := maximalDescriptor(StageUnpause)
			v := d.Value()
			want := wire.CodeMalformed
			switch mode {
			case "version":
				v.Obj.Set("profile", wire.String("taskman-stage/1"))
				want = wire.CodeUnsupportedVersion
			case "closed":
				v.Obj.Set("unexpected", wire.String("x"))
			case "cap":
				d.Artifacts[0].Bytes = "4097"
				v = d.Value()
				want = wire.CodeLimitExceeded
			}
			_, e := ObserveStage([]StageFile{{"active.json", wire.EncodeFile(v)}})
			if wire.CodeOf(e) != want {
				t.Fatalf("%v want %s", e, want)
			}
		})
	}
}

// This fixture supplies only the bounded bytes consumed by Bind. It is not a
// full-plan or genesis-history acceptance witness; absent other slots stay absent.
func completedStageBinding(t *testing.T, operation, kind string) (*StageObservation, StageBinding) {
	t.Helper()
	d := maximalDescriptor(operation)
	q, _ := wire.ParseQueueID("", d.QueueID)
	queue := intent.Queue{QueueID: q, RepositoryAuthorityID: "repo:a", Prefix: "A", NextSerial: "1", SchemaVersion: "0", CanonicalWriter: "NATIVE", IntentBranch: "main", Fixture: true, WriteBarrier: intent.WriteBarrier{Reason: "NONE"}}
	seq := wire.Size("1000000")
	prev := wire.Null()
	if d.Base != nil {
		prev = stageString(string(d.Base.LastReceiptSha256))
	}
	if d.Base == nil {
		seq = "1"
		prev = wire.Null()
	}
	outcome := mutation.Outcome{RequestID: d.RequestID, Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq}
	request := stageObject("requestId", stageString(d.RequestID), "seq", stageString(string(seq)), "mutationSha256", stageString(string(d.RequestSha256)), "outcome", outcome.Value())
	requestRaw := wire.EncodeFile(request)
	rp, _ := RequestPath(d.RequestID)
	receipt := stageObject("profile", stageString(ProfileReceipt), "seq", stageString(string(seq)), "prev", prev, "kind", stageString(kind), "requestId", stageString(d.RequestID), "actor", stageObject("id", stageString("owner"), "role", stageString("OWNER")), "ticketId", wire.Null(), "attemptId", wire.Null(), "generation", wire.Null(), "expectedRevision", wire.Null(), "headGeneration", stageString("0"), "pre", wire.Array(stageObject("path", stageString(rp), "sha256", wire.Null())), "post", wire.Array(stageObject("path", stageString(rp), "sha256", stageString(string(wire.Sum(requestRaw))), "record", request, "blobSha256", wire.Null())), "outcome", stageString("COMPLETED"), "codes", wire.Array(), "recordedAt", stageString(string(d.RecordedAt)))
	receiptRaw := wire.EncodeFile(receipt)
	receiptHash := wire.Sum(receiptRaw)
	head := Head{QueueID: q, LastSeq: seq, LastReceiptSha256: &receiptHash, Generation: "0", InitSha256: receiptHash, PrimaryWorktree: "/fixture", VersionSha256: wire.Sum([]byte(VersionBytes))}
	headRaw := wire.EncodeFile(head.Value())
	for i := range d.Artifacts {
		a := &d.Artifacts[i]
		var raw []byte
		switch {
		case a.Role == "HEAD":
			raw = headRaw
		case a.Role == "RECEIPT":
			raw = receiptRaw
		case a.Target == rp:
			raw = requestRaw
		default:
			continue
		}
		a.Bytes = wire.SizeOf(uint64(len(raw)))
		a.Sha256 = wire.Sum(raw)
	}
	raw, e := d.Encode()
	if e != nil {
		t.Fatal(e)
	}
	o, e := ObserveStage([]StageFile{{"active.json", raw}})
	if e != nil {
		t.Fatal(e)
	}
	return o, StageBinding{QueueID: d.QueueID, QueueRaw: wire.EncodeFile(queue.Value()), HeadRaw: headRaw, ReceiptRaw: receiptRaw, RequestRaw: requestRaw}
}

func TestTMV0002_AS10_CompletedStageOperationAndTimestamp(t *testing.T) {
	mapping := map[string]string{StageInit: "INIT", StagePause: "PAUSE", StageUnpause: "UNPAUSE", StageKeepJournal: "RECONCILE", StageAdoptFile: "RECONCILE"}
	for op, kind := range mapping {
		t.Run(op, func(t *testing.T) {
			o, b := completedStageBinding(t, op, kind)
			if e := o.Bind(b); e != nil {
				t.Fatal(e)
			}
		})
	}
	for _, mode := range []string{"UNPAUSE-labelled-PAUSE", "PAUSE-labelled-UNPAUSE", "KEEP-labelled-PAUSE", "timestamp"} {
		t.Run(mode, func(t *testing.T) {
			op, kind := StageUnpause, "PAUSE"
			if mode == "PAUSE-labelled-UNPAUSE" {
				op, kind = StagePause, "UNPAUSE"
			}
			if mode == "KEEP-labelled-PAUSE" {
				op, kind = StageKeepJournal, "PAUSE"
			}
			if mode == "timestamp" {
				kind = "UNPAUSE"
			}
			o, b := completedStageBinding(t, op, kind)
			if mode == "timestamp" {
				o.Descriptor.RecordedAt = "2026-09-07T00:00:00Z"
				raw, e := o.Descriptor.Encode()
				if e != nil {
					t.Fatal(e)
				}
				o, e = ObserveStage([]StageFile{{"active.json", raw}})
				if e != nil {
					t.Fatal(e)
				}
			}
			if e := o.Bind(b); wire.CodeOf(e) != wire.CodeJournalForked {
				t.Fatalf("inconsistent current-byte label accepted: %v", e)
			}
		})
	}
}
