package ticket

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// AttachedEvidence is one TEA-V0-001 entry: digests and a short reason
// recorded against an OPEN ticket without changing its acceptance. Actor and
// RecordedAt come from the writer's trusted binding and clock; the entry
// names the acceptance revision it was attached at.
type AttachedEvidence struct {
	AcceptanceRevision wire.Count
	Actor              string
	Evidence           []wire.Digest
	Reason             string
	RecordedAt         wire.Timestamp
}

// ReadAttachedEvidence decodes the optional attachedEvidence member: a
// non-empty array in append order (absence is the empty list) of closed
// entries, each with 1..AttachedEvidenceMaxDigests sorted unique digests.
func ReadAttachedEvidence(r *wire.Reader) []AttachedEvidence {
	entries := r.Array(wire.AttachedEvidenceMaxEntries, true)
	if r.Err() == nil && len(entries) == 0 {
		r.Fail(wire.CodeMalformed, "attachedEvidence is omitted when empty")
		return nil
	}
	out := make([]AttachedEvidence, 0, len(entries))
	for _, e := range entries {
		e.Closed("acceptanceRevision", "actor", "evidence", "reason", "recordedAt")
		a := AttachedEvidence{
			AcceptanceRevision: e.Field("acceptanceRevision").Count(),
			Actor:              e.Field("actor").Label(),
			Evidence:           ReadEvidenceDigests(e.Field("evidence")),
			Reason:             ReadEvidenceReason(e.Field("reason")),
			RecordedAt:         e.Field("recordedAt").Timestamp(),
		}
		out = append(out, a)
	}
	return out
}

// ReadEvidenceDigests reads 1..AttachedEvidenceMaxDigests sorted unique
// digests (a set; the CLI boundary sorts, the wire refuses unsorted).
func ReadEvidenceDigests(r *wire.Reader) []wire.Digest {
	items := r.Array(wire.AttachedEvidenceMaxDigests, false)
	if r.Err() == nil && len(items) == 0 {
		r.Fail(wire.CodeMalformed, "evidence names at least one digest")
		return nil
	}
	out := make([]wire.Digest, 0, len(items))
	for _, d := range items {
		out = append(out, d.Digest())
	}
	return out
}

// ReadEvidenceReason reads a nonblank prose reason of at most
// AttachedEvidenceMaxReasonBytes.
func ReadEvidenceReason(r *wire.Reader) string {
	s := r.Prose(1, wire.AttachedEvidenceMaxReasonBytes)
	if r.Err() == nil && strings.TrimSpace(s) == "" {
		r.Fail(wire.CodeMalformed, "reason must contain non-whitespace prose")
		return ""
	}
	return s
}

// AttachedEvidenceValue encodes the entries in append order.
func AttachedEvidenceValue(entries []AttachedEvidence) wire.Value {
	vs := make([]wire.Value, 0, len(entries))
	for _, a := range entries {
		o := wire.NewObject()
		o.Set("acceptanceRevision", wire.String(string(a.AcceptanceRevision)))
		o.Set("actor", wire.String(a.Actor))
		o.Set("evidence", DigestsValue(a.Evidence))
		o.Set("reason", wire.String(a.Reason))
		o.Set("recordedAt", wire.String(string(a.RecordedAt)))
		vs = append(vs, wire.ObjectValue(o))
	}
	return wire.Array(vs...)
}

// AttachedAt reports whether digest d is already attached at acceptance
// revision acc.
func AttachedAt(entries []AttachedEvidence, acc wire.Count, d wire.Digest) bool {
	for _, a := range entries {
		if a.AcceptanceRevision != acc {
			continue
		}
		for _, x := range a.Evidence {
			if x == d {
				return true
			}
		}
	}
	return false
}

// validateAttachedEvidence enforces the TEA-V0-001 relationships a closed
// type check cannot: each entry names an acceptance revision in
// 1..acceptanceRevision, entries never go back in acceptance revision, and no
// digest repeats within one acceptance revision.
func (rec *Record) validateAttachedEvidence() error {
	prev := int64(0)
	for i, a := range rec.AttachedEvidence {
		where := "/attachedEvidence/" + idx(i)
		n := a.AcceptanceRevision.Int()
		if n < 1 || n > rec.AcceptanceRevision.Int() {
			return wire.Errorf(wire.CodeMalformed, where+"/acceptanceRevision", "entry acceptanceRevision %s is outside 1..%s", a.AcceptanceRevision, rec.AcceptanceRevision)
		}
		if n < prev {
			return wire.Errorf(wire.CodeMalformed, where+"/acceptanceRevision", "entries are in append order; acceptanceRevision %s follows a later one", a.AcceptanceRevision)
		}
		prev = n
		for _, d := range a.Evidence {
			if AttachedAt(rec.AttachedEvidence[:i], a.AcceptanceRevision, d) {
				return wire.Errorf(wire.CodeDuplicateID, where+"/evidence", "digest %s is already attached at acceptanceRevision %s", d, a.AcceptanceRevision)
			}
		}
	}
	return nil
}
