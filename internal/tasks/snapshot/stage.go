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
	MaxStageDescriptorBytes        = 2658
	MaxStageReceiptSeq      uint64 = 1000000
	UnpauseReceiptBytes            = 1683
	UnpauseIndexBytes              = 563
	UnpauseOutcomeBytes            = 308
	UnpauseTemporaryBytes          = 8538
)

// StageImportApply posts a batch of CTS-V0-003 shadow-imported ticket records
// under one IMPORT_APPLY receipt; the operation name is the receipt kind.
const StageImportApply = "IMPORT_APPLY"

// StageAuthoritySwitch posts intent/queue.json with canonicalWriter NATIVE
// under one AUTHORITY_SWITCH receipt (TCP-00 §5.4 A5, CAL-V0-004); the
// operation name is the receipt kind.
const StageAuthoritySwitch = "AUTHORITY_SWITCH"

// StageLease carries one CAL-V0 lease transaction: its receipt kind is ADMIT,
// TRANSITION, GATE_RESULT or MANIFEST. A recorded FENCED refusal is a TRANSITION
// with REVISION_CONFLICT outcome and posts only its request.
const StageLease = "LEASE"

// StageQualification records the execution cutover (CAL-V0-020): one
// QUALIFICATION receipt posts the CAL-V0-019 run as evidence and
// intent/queue.json with executionCutover set; the operation name is the
// receipt kind.
const StageQualification = "QUALIFICATION"

// StageEscalation carries one committed typed escalation (ESC-V0-010): an
// ESCALATE or ANSWER lease transaction whose TRANSITION receipt posts its
// request, the ticket and one or two escalation events (two only for a
// supersession), with at most the ticket's own blob as evidence. Nothing
// else is admitted: no attempt, reservation, pool, program or gate output.
// Its maximum stays inside the StageLease bounds (decision 0428 item 7).
const StageEscalation = "ESCALATION"

// MaxEscalationEventBytes bounds one escalation event POST; it equals
// ticket.EscalationMaxEventBytes, which snapshot does not import.
const MaxEscalationEventBytes = 65536

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
		return 6, 1670
	case StageRelease:
		return 6, 2600
	case StagePolicyUpdate:
		return 5, 1470
	case StageImportApply:
		return 11, 2422
	case StageAuthoritySwitch:
		return 5, 1474
	case StageLease:
		return 11, 2658
	case StageQualification:
		return 6, 1680
	case StageEscalation:
		return 7, 1879
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
	if err := wire.ProfileVersion("stage/profile", v, "taskman-stage/0"); err != nil {
		return nil, err
	}
	r.Closed("profile", "queueId", "operation", "requestId", "requestSha256", "recordedAt", "base", "artifacts")
	if e := r.Err(); e != nil {
		return nil, e
	}
	if e = wire.CheckProfile("stage/profile", r.Field("profile").String(), "taskman-stage/0"); e != nil {
		return nil, e
	}
	d := &StageDescriptor{QueueID: r.Field("queueId").QueueID().Raw, Operation: r.Field("operation").Enum(StageInit, StagePause, StageUnpause, StageKeepJournal, StageAdoptFile, StageMutate, StageRelease, StagePolicyUpdate, StageImportApply, StageAuthoritySwitch, StageLease, StageQualification, StageEscalation), RequestID: r.Field("requestId").String(), RequestSha256: r.Field("requestSha256").Digest(), RecordedAt: r.Field("recordedAt").Timestamp()}
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
			if d.Operation == StageAuthoritySwitch || d.Operation == StageQualification {
				cap = wire.MaxQueueFileBytes
			}
			if d.Operation == StageEscalation {
				cap = wire.MaxTicketFileBytes
			}
			if d.Operation == StageLease {
				cap = wire.MaxReservationSetBytes
				if cap < MaxPoolStateBytes {
					cap = MaxPoolStateBytes
				}
			}
		case "POST":
			switch {
			case a.Target == requestPath:
				key = "request"
				cap = 563
				if d.Operation == StageInit {
					cap = 551
				}
				// A LEASE request is at most REVISION_CONFLICT with FENCED.
				if d.Operation == StageKeepJournal || d.Operation == StageAdoptFile || d.Operation == StageMutate || d.Operation == StageRelease || d.Operation == StagePolicyUpdate || d.Operation == StageImportApply || d.Operation == StageAuthoritySwitch || d.Operation == StageLease || d.Operation == StageQualification || d.Operation == StageEscalation {
					cap = 579
				}
			case strings.HasPrefix(a.Target, "attempts/") && d.Operation == StageLease:
				q, e := AttemptQueue(strings.TrimSuffix(strings.TrimPrefix(a.Target, "attempts/"), ".json"))
				if e != nil || q.Raw != d.QueueID || !strings.HasSuffix(a.Target, ".json") {
					return stageMalformed("attempt target scope")
				}
				key = "attempt"
				cap = wire.MaxAttemptRecordBytes
			case a.Target == "programs.json" && d.Operation == StageLease:
				key = "programs"
				cap = MaxProgramsBytes
			case a.Target == "pools.json" && d.Operation == StageLease:
				key = "pools"
				cap = MaxPoolStateBytes
			case a.Target == "reservations.json" && d.Operation == StageLease:
				key = "reservations"
				cap = wire.MaxReservationSetBytes
			case a.Target == "barrier.json" && d.Operation == StagePause:
				key = "barrier"
				cap = 4096
			case strings.HasPrefix(a.Target, "intent/tickets/") && (d.Operation == StageKeepJournal || d.Operation == StageAdoptFile || d.Operation == StageMutate || d.Operation == StageImportApply || d.Operation == StageLease || d.Operation == StageEscalation):
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
			case strings.HasPrefix(a.Target, "evidence/") && d.Operation == StageMutate:
				key, cap = mutateEvidenceSlot(d, a)
				if key == "" {
					return stageMalformed("mutation request evidence identity")
				}
			case strings.HasPrefix(a.Target, "evidence/") && d.Operation == StageLease:
				// A gate run posts its captured output and its gate result; a
				// completion posts its manifest (CAL-V0-016, CAL-V0-017).
				if a.Target != "evidence/"+string(a.Sha256) {
					return stageMalformed("gate evidence identity")
				}
				key = "gate"
				cap = wire.MaxGateOutputBytes
			case strings.HasPrefix(a.Target, "evidence/") && d.Operation == StageEscalation:
				if a.Target != "evidence/"+string(a.Sha256) {
					return stageMalformed("escalation event identity")
				}
				key = "escalation-event"
				cap = MaxEscalationEventBytes
			case strings.HasPrefix(a.Target, "evidence/") && d.Operation == StageQualification:
				if a.Target != "evidence/"+string(a.Sha256) {
					return stageMalformed("qualification run identity")
				}
				key = "run"
				cap = wire.MaxGateOutputBytes
			case a.Target == "intent/queue.json" && (d.Operation == StageMutate || d.Operation == StageAuthoritySwitch || d.Operation == StageQualification):
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
	case StageAuthoritySwitch:
		required["queue"] = 1
	case StageQualification:
		required["queue"] = 1
		required["run"] = 1
	case StageEscalation:
		required["ticket"] = 1
	case StageRelease:
		required["release"] = 1
		if counts["discard"] != 0 {
			required["discard"] = 1
		}
	}
	// A MUTATE queue post is present only when CREATE allocated a serial, so it
	// is optional rather than required.
	if d.Operation == StageMutate {
		// CREATE's queue allocation and OPEN recovery's retained envelope
		// are distinct operations; accepting both would widen this contract.
		if counts["mutation-request"] > 1 || (counts["mutation-request"] != 0 && counts["queue"] != 0) {
			return stageMalformed("mutation request evidence count")
		}
		if counts["derived-event"] > 1 || (counts["derived-event"] != 0 && counts["queue"]+counts["mutation-request"] != 0) {
			return stageMalformed("derived event count")
		}
		delete(counts, "mutation-request")
		delete(counts, "queue")
		delete(counts, "derived-event")
	}
	// A lease transaction posts at most one attempt and one reservation set;
	// a gate run adds its output and result, and a completion its ticket and
	// manifest, so the three optional kinds together stay within three. A
	// recorded FENCED refusal posts none of them.
	if d.Operation == StageLease && counts["programs"] == 1 && counts["pools"] == 0 && counts["attempt"] == 0 && counts["reservations"] == 0 && counts["ticket"] == 0 && counts["gate"] <= 2 {
		delete(counts, "programs")
	}
	if d.Operation == StageLease && counts["pools"] <= 1 && counts["attempt"] <= 1 && counts["reservations"] <= 1 && counts["ticket"] <= 1 && counts["gate"] <= 2 && counts["reservations"]+counts["ticket"]+counts["gate"] <= 3 {
		delete(counts, "pools")
		delete(counts, "attempt")
		delete(counts, "reservations")
		delete(counts, "ticket")
		delete(counts, "gate")
	}
	// OPEN and ANSWER post one event; only a supersession posts two.
	if d.Operation == StageEscalation {
		if counts["escalation-event"] < 1 || counts["escalation-event"] > 2 {
			return stageMalformed("escalation event count")
		}
		delete(counts, "escalation-event")
	}
	// An import batch posts one or more ticket records; the slot cap bounds it.
	if d.Operation == StageImportApply && counts["ticket"] >= 1 {
		delete(counts, "ticket")
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
	case StageKeepJournal, StageAdoptFile, StageMutate, StageRelease, StagePolicyUpdate, StageAuthoritySwitch, StageQualification, StageEscalation:
		maxEvidence = 1
	case StageLease:
		// Reservation, completed ticket and pool projection may all be blobs.
		maxEvidence = 3
	case StageImportApply:
		// A ticket record over the inline post bound is carried as a blob.
		maxEvidence = 8
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
	// The one escalation blob can only be the ticket record over the inline
	// post bound: events are already posted at their evidence paths.
	if d.Operation == StageEscalation {
		for _, a := range d.Artifacts {
			if a.Role != "EVIDENCE" {
				continue
			}
			matched := false
			for _, p := range d.Artifacts {
				matched = matched || (p.Role == "POST" && strings.HasPrefix(p.Target, "intent/tickets/") && p.Sha256 == a.Sha256 && p.Bytes == a.Bytes)
			}
			if !matched {
				return stageMalformed("escalation blob is not the ticket record")
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

// MaxDerivedEventBytes bounds the one content-addressed event a MUTATE may
// derive and post beside its ticket (ON-V0-004: the operator note event). It
// equals the note event bound and is the shared slot for later derived ticket
// events.
const MaxDerivedEventBytes = 65536

// mutateEvidenceSlot classifies a MUTATE evidence POST: the retained request
// envelope (OPEN recovery) when its digest is the request's, otherwise the
// single derived event. An empty key is a malformed identity.
func mutateEvidenceSlot(d StageDescriptor, a StageDescription) (string, uint64) {
	if a.Target != "evidence/"+string(a.Sha256) {
		return "", 0
	}
	if a.Sha256 == d.RequestSha256 {
		return "mutation-request", wire.MaxMutationEnvelopeBytes
	}
	return "derived-event", MaxDerivedEventBytes
}
