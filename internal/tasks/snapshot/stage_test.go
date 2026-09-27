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
	if op == StageKeepJournal || op == StageAdoptFile || op == StageMutate || op == StagePolicyUpdate || op == StageAuthoritySwitch || op == StageLease {
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
	case StagePause:
		add("POST", "barrier.json", 4096, hash)
	case StageAuthoritySwitch:
		add("POST", "intent/queue.json", 1048576, hash)
		add("EVIDENCE", "evidence/"+string(hash), 1048576, hash)
	case StagePolicyUpdate:
		add("POST", "intent/policy.json", 262144, hash)
		add("EVIDENCE", "evidence/"+string(hash), 262144, hash)
	case StageMutate:
		add("POST", "intent/tickets/"+strings.Repeat("t", 64)+".json", 131072, hash)
		add("EVIDENCE", "evidence/"+string(hash), 131072, hash)
		add("POST", "intent/queue.json", 1048576, wire.Sum([]byte("queue")))
	case StageLease:
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
func TestTMV0002_AS10_StageCodecActualMaxima(t *testing.T) {
	for _, op := range []string{StageInit, StagePause, StageUnpause, StageKeepJournal, StageAdoptFile, StageMutate, StagePolicyUpdate, StageAuthoritySwitch, StageLease} {
		d := maximalDescriptor(op)
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
			x := maximalDescriptor(op)
			x.Artifacts[i].Bytes = wire.SizeOf(x.Artifacts[i].Bytes.Uint64() + 1)
			if _, e = x.Encode(); e == nil {
				t.Fatalf("%stageString artifact %d cap+1 accepted", op, i)
			}
		}
		x := maximalDescriptor(op)
		x.Artifacts[0].Slot = "a01"
		if _, e = x.Encode(); e == nil {
			t.Fatal("slot order accepted")
		}
		x = maximalDescriptor(op)
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
