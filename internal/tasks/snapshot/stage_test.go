package snapshot

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
	"strings"
	"testing"
)

func maximalDescriptor(op string) StageDescriptor {
	q := "queue:a:" + strings.Repeat("q", 120)
	if op == StageKeepJournal || op == StageAdoptFile || op == StageMutate {
		q = "queue:a:" + strings.Repeat("q", 54)
	}
	// attempts/<attemptId>.json is an Identifier, so a lease queue id is at
	// most 79 bytes.
	if op == StageLease {
		q = "queue:a:" + strings.Repeat("q", 71)
	}
	d := StageDescriptor{QueueID: q, Operation: op, RequestID: strings.Repeat(`"`, 64), RequestSha256: wire.Sum([]byte("request")), RecordedAt: wire.Timestamp("2026-09-06T00:00:00Z")}
	if op != StageInit {
		d.Base = &StageBase{LastSeq: "999999", LastReceiptSha256: wire.Sum([]byte("base"))}
	}
	seq := uint64(1000000)
	if op == StageInit {
		seq = 1
	}
	name, _ := ReceiptName(seq)
	rp, _ := RequestPath(d.RequestID)
	add := func(role, target string, n uint64, hash wire.Digest) {
		d.Artifacts = append(d.Artifacts, StageDescription{Role: role, Target: target, Bytes: wire.SizeOf(n), Sha256: hash})
	}
	hash := wire.Sum([]byte("bytes"))
	receiptBytes := uint64(1048576)
	if op == StageUnpause {
		receiptBytes = 1683
	}
	add("HEAD", "head.json", 4096, hash)
	add("RECEIPT", "receipts/"+name, receiptBytes, hash)
	requestBytes := uint64(563)
	if op == StageInit {
		requestBytes = 551
	}
	if op == StageKeepJournal || op == StageAdoptFile || op == StageMutate || op == StagePolicyUpdate || op == StageAuthoritySwitch || op == StageQualification || op == StageLease {
		requestBytes = 579
	}
	add("POST", rp, requestBytes, hash)
	switch op {
	case StageInit:
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
	case StageRelease:
		add("POST", "intent/releases/v1.json", 131072, hash)
	case StagePause:
		add("POST", "barrier.json", 4096, hash)
	case StageAuthoritySwitch:
		add("POST", "intent/queue.json", 1048576, hash)
		add("EVIDENCE", "evidence/"+string(hash), 1048576, hash)
	case StageQualification:
		add("POST", "intent/queue.json", 1048576, hash)
		run := wire.Sum([]byte("run"))
		add("POST", "evidence/"+string(run), 16777216, run)
		add("EVIDENCE", "evidence/"+string(hash), 1048576, hash)
	case StagePolicyUpdate:
		add("POST", "intent/policy.json", 262144, hash)
		add("EVIDENCE", "evidence/"+string(hash), 262144, hash)
	case StageMutate:
		add("POST", "intent/tickets/"+strings.Repeat("t", 64)+".json", 131072, hash)
		add("EVIDENCE", "evidence/"+string(hash), 131072, hash)
		add("POST", "intent/queue.json", 1048576, wire.Sum([]byte("queue")))
	case StageLease:
		poolHash := wire.Sum([]byte("pool"))
		add("POST", "pools.json", MaxPoolStateBytes, poolHash)
		add("EVIDENCE", "evidence/"+string(poolHash), wire.MaxReservationSetBytes, poolHash)
		add("POST", "attempts/attempt:a:"+strings.Repeat("q", 71)+":"+strings.Repeat("a", 32)+".json", wire.MaxAttemptRecordBytes, hash)
		add("POST", "reservations.json", wire.MaxReservationSetBytes, wire.Sum([]byte("reservations")))
		add("EVIDENCE", "evidence/"+string(hash), wire.MaxReservationSetBytes, hash)
		// A 79-byte queue id leaves a 47-byte ticket local token.
		ticketHash := wire.Sum([]byte("ticket"))
		add("POST", "intent/tickets/"+strings.Repeat("t", 47)+".json", 131072, ticketHash)
		add("EVIDENCE", "evidence/"+string(ticketHash), wire.MaxReservationSetBytes, ticketHash)
		manifest := wire.Sum([]byte("manifest"))
		add("POST", "evidence/"+string(manifest), wire.MaxGateOutputBytes, manifest)
	case StageKeepJournal, StageAdoptFile:
		add("POST", "intent/tickets/"+strings.Repeat("t", 64)+".json", 131072, hash)
		add("EVIDENCE", "evidence/"+string(hash), 131072, hash)
		if op == StageKeepJournal {
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
		d.Artifacts[i].Slot = stageSlot(i)
	}
	return d
}

// maximalRecoveryDescriptor preserves legacy fixture semantics for other
// tests while measuring the larger request-bearing MUTATE descriptor.
func maximalRecoveryDescriptor(op string) StageDescriptor {
	d := maximalDescriptor(op)
	if op != StageMutate {
		return d
	}
	for i, a := range d.Artifacts {
		if a.Role == "POST" && a.Target == "intent/queue.json" {
			d.Artifacts[i].Target = "evidence/" + string(d.RequestSha256)
			d.Artifacts[i].Sha256 = d.RequestSha256
			d.Artifacts[i].Bytes = wire.SizeOf(wire.MaxMutationEnvelopeBytes)
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
		d.Artifacts[i].Slot = stageSlot(i)
	}
	return d
}

func TestTMV0002_AS10_StageCodecActualMaxima(t *testing.T) {
	for _, op := range []string{StageInit, StagePause, StageUnpause, StageKeepJournal, StageAdoptFile, StageMutate, StagePolicyUpdate, StageAuthoritySwitch, StageLease, StageQualification} {
		d := maximalRecoveryDescriptor(op)
		raw, e := d.Encode()
		if e != nil {
			t.Fatalf("%stageString: %v (wire=%d)", op, e, len(wire.EncodeFile(d.Value())))
		}
		slots, cap := StageLimits(op)
		if len(raw) != cap || len(d.Artifacts) != slots {
			t.Fatalf("%stageString actual %d/%d want %d/%d", op, len(raw), len(d.Artifacts), cap, slots)
		}
		decoded, e := DecodeStageDescriptor(raw)
		if e != nil {
			t.Fatal(e)
		}
		again, e := decoded.Encode()
		if e != nil || !bytes.Equal(raw, again) {
			t.Fatal("roundtrip", e)
		}
		for i := range d.Artifacts {
			x := maximalRecoveryDescriptor(op)
			x.Artifacts[i].Bytes = wire.SizeOf(x.Artifacts[i].Bytes.Uint64() + 1)
			if _, e = x.Encode(); e == nil {
				t.Fatalf("%stageString artifact %d cap+1 accepted", op, i)
			}
		}
		x := maximalRecoveryDescriptor(op)
		x.Artifacts[0].Slot = "a01"
		if _, e = x.Encode(); e == nil {
			t.Fatal("slot order accepted")
		}
		x = maximalRecoveryDescriptor(op)
		x.Artifacts = append(x.Artifacts, x.Artifacts[0])
		if _, e = x.Encode(); e == nil {
			t.Fatal("extra slot")
		}
		v := d.Value()
		v.Obj.Set("authority", wire.Bool(true))
		if _, e = DecodeStageDescriptor(wire.EncodeFile(v)); e == nil {
			t.Fatal("unknown field")
		}
		if _, e = DecodeStageDescriptor(append([]byte(" "), raw...)); e == nil {
			t.Fatal("noncanonical")
		}
	}
}

func TestCALV0043_MutationRequestEvidenceBounds(t *testing.T) {
	t.Run("CAL-V0-043 retained request descriptor bounds", func(t *testing.T) {
		for _, name := range []string{"valid", "legacy", "digest", "path", "oversized", "extra", "queue", "operation"} {
			t.Run(name, func(t *testing.T) {
				d := maximalRecoveryDescriptor(StageMutate)
				idx := -1
				for i, a := range d.Artifacts {
					if a.Role == "POST" && strings.HasPrefix(a.Target, "evidence/") {
						idx = i
					}
				}
				if idx < 0 {
					t.Fatal("no request artifact")
				}
				switch name {
				case "legacy":
					d.Artifacts[idx].Target = "intent/queue.json"
					d.Artifacts[idx].Bytes = "1048576"
					d.Artifacts[idx].Sha256 = wire.Sum([]byte("queue"))
				case "digest":
					d.Artifacts[idx].Sha256 = wire.Sum([]byte("other"))
				case "path":
					d.Artifacts[idx].Target = "evidence/" + string(wire.Sum([]byte("other")))
				case "oversized":
					d.Artifacts[idx].Bytes = wire.SizeOf(wire.MaxMutationEnvelopeBytes + 1)
				case "extra":
					d.Artifacts = append(d.Artifacts, d.Artifacts[idx])
				case "queue":
					d.Artifacts = append(d.Artifacts, StageDescription{Role: "POST", Target: "intent/queue.json", Bytes: "1", Sha256: wire.Sum([]byte("q"))})
				case "operation":
					d.Operation = StageAdoptFile
				}
				sort.Slice(d.Artifacts, func(i, j int) bool {
					if d.Artifacts[i].Role != d.Artifacts[j].Role {
						return d.Artifacts[i].Role < d.Artifacts[j].Role
					}
					return d.Artifacts[i].Target < d.Artifacts[j].Target
				})
				for i := range d.Artifacts {
					d.Artifacts[i].Slot = fmt.Sprintf("a%02d", i)
				}
				_, err := d.Encode()
				if (err == nil) != (name == "valid" || name == "legacy") {
					t.Fatalf("%s: %v", name, err)
				}
			})
		}
	})
}

// maximalDerivedEventDescriptor is the worst-case NOTE_SET/NOTE_CLEAR stage:
// the maximal MUTATE ticket with its inline-overflow EVIDENCE blob, plus one
// maximal derived event in place of the CREATE queue post (ON-V0-006).
func maximalDerivedEventDescriptor() StageDescriptor {
	d := maximalDescriptor(StageMutate)
	for i, a := range d.Artifacts {
		if a.Role == "POST" && a.Target == "intent/queue.json" {
			h := wire.Sum([]byte("derived"))
			d.Artifacts[i] = StageDescription{Role: "POST", Target: "evidence/" + string(h), Sha256: h, Bytes: wire.SizeOf(MaxDerivedEventBytes)}
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
		d.Artifacts[i].Slot = stageSlot(i)
	}
	return d
}

// TestONV0006_DerivedEventSlotMeasuredAndNarrow measures the actual maximal
// MUTATE descriptor carrying the derived note event (6 artifacts, 1669 of
// 1670 bytes) and refuses every widening: a second event, an event beside the
// CREATE queue post or the retained request envelope, an oversized event, a
// non-content-addressed target, and the slot on another operation.
func TestONV0006_DerivedEventSlotMeasuredAndNarrow(t *testing.T) {
	d := maximalDerivedEventDescriptor()
	raw, err := d.Encode()
	if err != nil {
		t.Fatalf("maximal derived-event descriptor: %v", err)
	}
	slots, cap := StageLimits(StageMutate)
	if len(d.Artifacts) != slots || len(raw) != 1669 || len(raw) > cap {
		t.Fatalf("measured %d bytes / %d artifacts; want 1669 within %d / %d", len(raw), len(d.Artifacts), cap, slots)
	}
	derived := func(d *StageDescriptor) int {
		for i, a := range d.Artifacts {
			if a.Role == "POST" && strings.HasPrefix(a.Target, "evidence/") {
				return i
			}
		}
		t.Fatal("no derived event")
		return -1
	}
	want := map[string]string{
		"second":    "MALFORMED: stage: derived event count",
		"queue":     "MALFORMED: stage: derived event count",
		"request":   "MALFORMED: stage: derived event count",
		"oversized": "LIMIT_EXCEEDED: stage: artifact bytes",
		"address":   "MALFORMED: stage: mutation request evidence identity",
		"operation": "MALFORMED: stage: post outside operation",
	}
	for _, name := range []string{"second", "queue", "request", "oversized", "address", "operation"} {
		t.Run(name, func(t *testing.T) {
			x := maximalDerivedEventDescriptor()
			i := derived(&x)
			switch name {
			case "second":
				h := wire.Sum([]byte("second"))
				x.Artifacts[0] = StageDescription{Role: "POST", Target: "evidence/" + string(h), Sha256: h, Bytes: "1"}
			case "queue":
				x.Artifacts[0] = StageDescription{Role: "POST", Target: "intent/queue.json", Sha256: wire.Sum([]byte("q")), Bytes: "1"}
			case "request":
				x.Artifacts[0] = StageDescription{Role: "POST", Target: "evidence/" + string(x.RequestSha256), Sha256: x.RequestSha256, Bytes: "1"}
			case "oversized":
				x.Artifacts[i].Bytes = wire.SizeOf(MaxDerivedEventBytes + 1)
			case "address":
				x.Artifacts[i].Sha256 = wire.Sum([]byte("other"))
			case "operation":
				x.Operation = StageAdoptFile
				x.Artifacts = append(x.Artifacts[:0], x.Artifacts[1:]...)
			}
			sort.Slice(x.Artifacts, func(a, b int) bool {
				if x.Artifacts[a].Role != x.Artifacts[b].Role {
					return x.Artifacts[a].Role < x.Artifacts[b].Role
				}
				return x.Artifacts[a].Target < x.Artifacts[b].Target
			})
			for j := range x.Artifacts {
				x.Artifacts[j].Slot = stageSlot(j)
			}
			_, err := x.Encode()
			if err == nil {
				t.Fatalf("%s accepted", name)
			}
			if err.Error() != want[name] {
				t.Fatalf("%s refused for the wrong reason: %v, want %s", name, err, want[name])
			}
		})
	}
}
