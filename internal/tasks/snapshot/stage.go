// The proposed taskman-stage/0 codec is shared read-only structure, not
// archive-layout acceptance, a physical staging observer or writer permission.
package snapshot

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
)

const (
	StageInit        = "INIT"
	StagePause       = "PAUSE"
	StageUnpause     = "UNPAUSE"
	StageKeepJournal = "KEEP_JOURNAL"
	StageAdoptFile   = "ADOPT_FILE"
	// StageMutate carries one §3.3 ticket mutation envelope. Its receipt kind
	// is MUTATION, ARCHIVE or RESTORE (§3.1); the stage operation name is the
	// same for all of them because the staged artifact shape is identical.
	StageMutate                    = "MUTATE"
	StageRelease                   = "RELEASE"
	StagePolicyUpdate              = "POLICY_UPDATE"
	MaxStageDescriptorBytes        = 2422
	MaxStageReceiptSeq      uint64 = 1000000
	UnpauseReceiptBytes            = 1683
	UnpauseIndexBytes              = 563
	UnpauseOutcomeBytes            = 308
	UnpauseTemporaryBytes          = 8538
)

type StageBase struct {
	LastSeq           wire.Size
	LastReceiptSha256 wire.Digest
}
type StageDescription struct {
	Slot, Role, Target string
	Sha256             wire.Digest
	Bytes              wire.Size
}
type StageDescriptor struct {
	QueueID, Operation, RequestID string
	RequestSha256                 wire.Digest
	RecordedAt                    wire.Timestamp
	Base                          *StageBase
	Artifacts                     []StageDescription
}

func StageLimits(op string) (int, int) {
	switch op {
	case StageInit:
		return 11, 2422
	case StagePause:
		return 4, 1243
	case StageUnpause:
		return 3, 1098
	case StageKeepJournal:
		return 6, 1676
	case StageAdoptFile:
		return 5, 1467
	case StageMutate:
		return 6, 1615
	case StageRelease:
		return 6, 2600
	case StagePolicyUpdate:
		return 5, 1470
	}
	return 0, 0
}
func (d StageDescriptor) Value() wire.Value {
	base := wire.Null()
	if d.Base != nil {
		base = stageObject("lastSeq", stageString(string(d.Base.LastSeq)), "lastReceiptSha256", stageString(string(d.Base.LastReceiptSha256)))
	}
	a := make([]wire.Value, 0, len(d.Artifacts))
	for _, v := range d.Artifacts {
		a = append(a, stageObject("slot", stageString(v.Slot), "role", stageString(v.Role), "target", stageString(v.Target), "sha256", stageString(string(v.Sha256)), "bytes", stageString(string(v.Bytes))))
	}
	return stageObject("profile", stageString("taskman-stage/0"), "queueId", stageString(d.QueueID), "operation", stageString(d.Operation), "requestId", stageString(d.RequestID), "requestSha256", stageString(string(d.RequestSha256)), "recordedAt", stageString(string(d.RecordedAt)), "base", base, "artifacts", wire.Array(a...))
}
func (d StageDescriptor) Encode() ([]byte, error) {
	raw := wire.EncodeFile(d.Value())
	if _, e := DecodeStageDescriptor(raw); e != nil {
		return nil, e
	}
	return raw, nil
}
func DecodeStageDescriptor(raw []byte) (*StageDescriptor, error) {
	if len(raw) > MaxStageDescriptorBytes {
		return nil, stageLimit("stage descriptor")
	}
	v, e := wire.Parse(raw)
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(wire.EncodeFile(v), raw) {
		return nil, stageMalformed("noncanonical descriptor")
	}
	r := wire.NewReader(v, "stage")
	r.Closed("profile", "queueId", "operation", "requestId", "requestSha256", "recordedAt", "base", "artifacts")
	if e = wire.CheckProfile("stage/profile", r.Field("profile").String(), "taskman-stage/0"); e != nil {
		return nil, e
	}
	d := &StageDescriptor{QueueID: r.Field("queueId").QueueID().Raw, Operation: r.Field("operation").Enum(StageInit, StagePause, StageUnpause, StageKeepJournal, StageAdoptFile, StageMutate, StageRelease, StagePolicyUpdate), RequestID: r.Field("requestId").String(), RequestSha256: r.Field("requestSha256").Digest(), RecordedAt: r.Field("recordedAt").Timestamp()}
	b := r.Field("base")
	if !b.IsNull() {
		b.Closed("lastSeq", "lastReceiptSha256")
		d.Base = &StageBase{b.Field("lastSeq").Size(), b.Field("lastReceiptSha256").Digest()}
	}
	for _, a := range r.Field("artifacts").Array(11, true) {
		a.Closed("slot", "role", "target", "sha256", "bytes")
		d.Artifacts = append(d.Artifacts, StageDescription{a.Field("slot").String(), a.Field("role").Enum("EVIDENCE", "HEAD", "POST", "RECEIPT"), a.Field("target").Identifier(), a.Field("sha256").Digest(), a.Field("bytes").Size()})
	}
	if e = r.Err(); e != nil {
		return nil, e
	}
	if _, e = RequestPath(d.RequestID); e != nil {
		return nil, e
	}
	if e = d.shape(); e != nil {
		return nil, e
	}
	_, cap := StageLimits(d.Operation)
	if len(raw) > cap {
		return nil, stageLimit("operation descriptor cap")
	}
	return d, nil
}
func (d StageDescriptor) shape() error {
	max, _ := StageLimits(d.Operation)
	if len(d.Artifacts) > max {
		return stageLimit("operation slots")
	}
	if (d.Operation == StageInit) != (d.Base == nil) {
		return stageMalformed("descriptor base")
	}
	seq := uint64(1)
	if d.Base != nil {
		if d.Base.LastSeq.Uint64() < 1 || d.Base.LastSeq.Uint64() >= MaxStageReceiptSeq {
			return stageLimit("descriptor base sequence")
		}
		seq = d.Base.LastSeq.Uint64() + 1
	}
	name, _ := ReceiptName(seq)
	receiptPath := "receipts/" + name
	requestPath, _ := RequestPath(d.RequestID)
	seen := map[string]bool{}
	counts := map[string]int{}
	for i, a := range d.Artifacts {
		if a.Slot != stageSlot(i) {
			return stageMalformed("semantic slot order")
		}
		if i > 0 {
			p := d.Artifacts[i-1]
			if p.Role > a.Role || (p.Role == a.Role && p.Target >= a.Target) {
				return stageMalformed("semantic role/target order")
			}
		}
		if seen[a.Target] {
			return stageMalformed("duplicate target")
		}
		seen[a.Target] = true
		cap := uint64(0)
		key := a.Role
		switch a.Role {
		case "HEAD":
			if a.Target != "head.json" {
				return stageMalformed("head target")
			}
			cap = 4096
		case "RECEIPT":
			if a.Target != receiptPath {
				return stageMalformed("receipt target")
			}
			cap = wire.MaxReceiptFileBytes
			if d.Operation == StageUnpause {
				cap = UnpauseReceiptBytes
			}
		case "EVIDENCE":
			if !strings.HasPrefix(a.Target, "evidence/") || a.Target != "evidence/"+string(a.Sha256) {
				return stageMalformed("evidence target")
			}
			cap = wire.MaxTicketFileBytes
			if d.Operation == StageInit {
				cap = wire.MaxQueueFileBytes
			}
			if d.Operation == StageRelease {
				cap = wire.MaxReleaseFileBytes
			}
			if d.Operation == StagePolicyUpdate {
				cap = wire.MaxPolicyFileBytes
			}
		case "POST":
			switch {
			case a.Target == requestPath:
				key = "request"
				cap = 563
				if d.Operation == StageInit {
					cap = 551
				}
				if d.Operation == StageKeepJournal || d.Operation == StageAdoptFile || d.Operation == StageMutate || d.Operation == StageRelease || d.Operation == StagePolicyUpdate {
					cap = 579
				}
			case a.Target == "barrier.json" && d.Operation == StagePause:
				key = "barrier"
				cap = 4096
			case strings.HasPrefix(a.Target, "intent/tickets/") && (d.Operation == StageKeepJournal || d.Operation == StageAdoptFile || d.Operation == StageMutate):
				local := strings.TrimSuffix(strings.TrimPrefix(a.Target, "intent/tickets/"), ".json")
				q := strings.TrimPrefix(d.QueueID, "queue:")
				id, e := wire.ParseTicketID("target", "ticket:"+q+":"+local)
				if e != nil || a.Target != "intent/tickets/"+id.Local+".json" {
					return stageMalformed("ticket target scope")
				}
				key = "ticket"
				cap = 131072
			case strings.HasPrefix(a.Target, "intent/releases/") && d.Operation == StageRelease:
				id := strings.TrimSuffix(strings.TrimPrefix(a.Target, "intent/releases/"), ".json")
				if _, e := wire.ParseLabel("target", id); e != nil || a.Target != "intent/releases/"+id+".json" {
					return stageMalformed("release target")
				}
				key = "release"
				cap = wire.MaxReleaseFileBytes
			case strings.HasPrefix(a.Target, "evidence/") && (d.Operation == StageKeepJournal || d.Operation == StageRelease):
				if a.Target != "evidence/"+string(a.Sha256) {
					return stageMalformed("discarded evidence identity")
				}
				key = "discard"
				cap = 131072
			case a.Target == "intent/queue.json" && d.Operation == StageMutate:
				// CREATE advances nextSerial; no other mutation posts the manifest.
				key = "queue"
				cap = 1048576
			case a.Target == "intent/policy.json" && d.Operation == StagePolicyUpdate:
				key = "policy"
				cap = wire.MaxPolicyFileBytes
			case d.Operation == StageInit:
				key = a.Target
				switch {
				case a.Target == "VERSION":
					cap = 16
				case a.Target == "intent/queue.json":
					cap = 1048576
				case a.Target == "intent/policy.json":
					cap = 262144
				case a.Target == "reservations.json":
					cap = 194
				case a.Target == "pinned/"+string(a.Sha256)+".json":
					key = "pinned"
					cap = 8465
				default:
					return stageMalformed("INIT post target")
				}
			default:
				return stageMalformed("post outside operation")
			}
		default:
			return stageMalformed("role")
		}
		if a.Bytes.Uint64() > cap {
			return stageLimit("artifact bytes")
		}
		counts[key]++
	}
	required := map[string]int{"HEAD": 1, "RECEIPT": 1, "request": 1}
	switch d.Operation {
	case StageInit:
		required["VERSION"] = 1
		required["intent/queue.json"] = 1
		required["intent/policy.json"] = 1
		required["reservations.json"] = 1
		required["pinned"] = 1
	case StagePause:
		required["barrier"] = 1
	case StageKeepJournal:
		required["ticket"] = 1
		required["discard"] = 1
	case StageAdoptFile:
		required["ticket"] = 1
	case StageMutate:
		required["ticket"] = 1
	case StagePolicyUpdate:
		required["policy"] = 1
	case StageRelease:
		required["release"] = 1
		if counts["discard"] != 0 {
			required["discard"] = 1
		}
	}
	// A MUTATE queue post is present only when CREATE allocated a serial, so it
	// is optional rather than required.
	if d.Operation == StageMutate {
		delete(counts, "queue")
	}
	for k, n := range required {
		if counts[k] != n {
			return stageMalformed("missing/duplicate required artifact")
		}
		delete(counts, k)
	}
	ecount := counts["EVIDENCE"]
	delete(counts, "EVIDENCE")
	if len(counts) != 0 {
		return stageMalformed("unexpected artifact")
	}
	maxEvidence := 0
	switch d.Operation {
	case StageInit:
		maxEvidence = 3
	case StageKeepJournal, StageAdoptFile, StageMutate, StageRelease, StagePolicyUpdate:
		maxEvidence = 1
	}
	if ecount > maxEvidence {
		return stageLimit("evidence slots")
	}
	// INIT evidence can only serve VERSION, queue and policy; matching sizes and
	// hashes here prevents three independent queue maxima inflating the cap.
	if d.Operation == StageInit {
		used := map[string]bool{}
		for _, a := range d.Artifacts {
			if a.Role != "EVIDENCE" {
				continue
			}
			matched := false
			for _, p := range d.Artifacts {
				if p.Role == "POST" && (p.Target == "VERSION" || p.Target == "intent/queue.json" || p.Target == "intent/policy.json") && p.Sha256 == a.Sha256 && p.Bytes == a.Bytes && !used[p.Target] {
					used[p.Target] = true
					matched = true
					break
				}
			}
			if !matched {
				return stageMalformed("INIT blob has no matching post")
			}
		}
	}
	return nil
}

func stageObject(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
func stageString(s string) wire.Value { return wire.String(s) }
func stageMalformed(detail string) error {
	return wire.Errorf(wire.CodeMalformed, "stage", "%s", detail)
}
func stageLimit(detail string) error {
	return wire.Errorf(wire.CodeLimitExceeded, "stage", "%s", detail)
}
func stageSlot(i int) string { return fmt.Sprintf("a%02d", i) }
