// Package criterionbinding validates optional criterion capture artifacts using
// the native record decoders. It has no execution or mutation capability.
package criterionbinding

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func native(raw string, command []string) (*wire.Result, error) {
	if len(raw) > wire.MaxCriterionCaptureBytes {
		return nil, fmt.Errorf("native capture exceeds bound")
	}
	r, e := wire.DecodeResult([]byte(raw))
	if e != nil {
		return nil, e
	}
	if r.Outcome != wire.OutcomeOK || r.Mutation != nil || r.Page != nil || len(r.Items) != 1 || len(r.Codes) != 0 || len(r.Command) != len(command) {
		return nil, fmt.Errorf("native read shape/outcome")
	}
	for i, s := range command {
		if r.Command[i] != s {
			return nil, fmt.Errorf("embedded native command mismatch")
		}
	}
	s := r.Snapshot
	if s == nil || s.HeadSeq == nil || s.HeadReceiptSha256 == nil || s.IntentTreeSha256 == nil || s.PrimaryWorktreeSha256 == nil || s.PendingRedo || s.Barrier != nil {
		return nil, fmt.Errorf("native snapshot incomplete or blocked")
	}
	return r, nil
}
func historical(s *wire.Snapshot) wire.CriterionSnapshot {
	return wire.CriterionSnapshot{HeadSeq: *s.HeadSeq, HeadReceiptSHA256: *s.HeadReceiptSha256, IntentTreeSHA256: *s.IntentTreeSha256, PrimaryWorktreeSHA256: *s.PrimaryWorktreeSha256}
}

// Verify is repository-independent. Historical lease expiry is retained, not
// compared with the current clock; live consumers perform that applicability check.
func Verify(raw []byte, verifier wire.CriterionIdentity) (wire.CriterionVerification, error) {
	var out wire.CriterionVerification
	c, e := wire.DecodeCriterionCapture(raw)
	if e != nil {
		return out, e
	}
	t, e := native(c.Ticket, []string{"ticket", "show"})
	if e != nil {
		return out, e
	}
	q, e := native(c.Queue, []string{"queue", "status"})
	if e != nil {
		return out, e
	}
	a, e := native(c.Attempt, []string{"attempt", "show"})
	if e != nil {
		return out, e
	}
	if historical(t.Snapshot) != historical(q.Snapshot) || historical(t.Snapshot) != historical(a.Snapshot) {
		return out, fmt.Errorf("native snapshot moved")
	}
	if t.Items[0].Obj == nil {
		return out, fmt.Errorf("ticket item is not an object")
	}
	record, exists := t.Items[0].Obj.Get("record")
	if !exists {
		return out, fmt.Errorf("ticket record missing")
	}
	rec, e := ticket.Decode(wire.EncodeFile(record))
	if e != nil {
		return out, e
	}
	attempt, e := snapshot.DecodeAttempt(wire.EncodeFile(a.Items[0]))
	if e != nil {
		return out, e
	}
	claimed, e := ticket.Decode([]byte(c.ClaimedTicket))
	if e != nil {
		return out, fmt.Errorf("mandatory canonical claimedTicket unavailable: %w", e)
	}
	if claimed.FileDigest() != attempt.TicketRecordSha256 || claimed.TicketID.Raw != rec.TicketID.Raw || claimed.AcceptanceRevision != attempt.TicketRevision || claimed.AcceptanceRevision != rec.AcceptanceRevision || !slices.Equal(claimed.AcceptanceCriteria, rec.AcceptanceCriteria) {
		return out, fmt.Errorf("claimed/current acceptance binding mismatch")
	}
	head := t.Snapshot.HeadSeq.Uint64()
	if attempt.Lease == nil || attempt.PhaseSinceSeq.Uint64() > head || attempt.Lease.GrantedSeq.Uint64() > head {
		return out, fmt.Errorf("attempt phase or lease is beyond historical head")
	}
	policy, e := intent.DecodePolicy([]byte(c.Policy))
	if e != nil {
		return out, e
	}
	qr := wire.NewReader(q.Items[0], "/queue")
	queueID := qr.Field("queueId").QueueID()
	policyID := qr.Field("policySha256").Digest()
	barrier := qr.Field("barrier")
	writeBarrier := qr.Field("writeBarrier").Exact("NONE")
	cutover := qr.Field("executionCutover").Bool()
	if e = qr.Err(); e != nil {
		return out, e
	}
	if !barrier.IsNull() || writeBarrier != "NONE" || !cutover || policyID != policy.PolicySha256() || attempt.PolicySha256 != wire.Sum(policy.Raw) {
		return out, fmt.Errorf("queue policy blocked or changed")
	}
	if queueID.Raw != rec.TicketID.QueueID() || attempt.TicketID.Raw != rec.TicketID.Raw || attempt.TicketRevision != rec.AcceptanceRevision || attempt.RuntimeID != snapshot.RuntimeExternalAgent || attempt.ConfigSha256 != attempt.PolicySha256 || attempt.Lease == nil {
		return out, fmt.Errorf("ticket/attempt/queue acceptance runtime binding mismatch")
	}
	// Encoding all inputs as exact native bytes must remain stable; the native
	// parser already enforces framing, key uniqueness and canonical spelling.
	if !bytes.Equal(raw, wire.EncodeFile(c.Value())) {
		return out, fmt.Errorf("capture is not canonical")
	}
	out = wire.CriterionVerification{Producer: c.Producer, Verifier: verifier, CaptureSHA256: wire.Sum(raw), Binding: wire.CriterionBinding{Snapshot: historical(t.Snapshot), TicketID: rec.TicketID, AcceptanceRevision: rec.AcceptanceRevision, AcceptanceCriteria: rec.AcceptanceCriteria, AttemptID: attempt.AttemptID, Generation: attempt.Generation, Phase: attempt.Phase, BaseCommit: attempt.BaseCommit, CandidateTreeOID: attempt.CandidateTreeOid, PolicySHA256: attempt.PolicySha256, PolicyContentIDSHA256: policy.PolicySha256(), ConfigSHA256: attempt.ConfigSha256, RuntimeID: attempt.RuntimeID, LeaseExpiresAt: attempt.Lease.ExpiresAt}}
	if _, e = wire.ReadCriterionVerification(out.Value()); e != nil {
		return wire.CriterionVerification{}, e
	}
	return out, nil
}
