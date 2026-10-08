package mutation

import (
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// KnowHowSecretDetail prefixes the refusal detail of a know-how write whose
// free text matches the secret screen (KHN-V0-004). The refusal is
// VALIDATION_FAILED with detail code MALFORMED; the detail names the field,
// never the matched text.
const KnowHowSecretDetail = "KNOWHOW_SECRET_DETECTED"

// ScreenKnowHowArgs refuses a know-how command whose raw arguments match the
// secret screen before any parse or pin error can echo them (KHN-V0-004).
// The error is MALFORMED with the KnowHowSecretDetail prefix and never
// repeats the argument; the mutation step screens the payload again.
func ScreenKnowHowArgs(args []string) error {
	for _, s := range args {
		if secretscreen.MatchString(s) {
			return wire.Errorf(wire.CodeMalformed, "/payload", "%s: an argument matches the secret screen; remove the secret and retry with a new request ID", KnowHowSecretDetail)
		}
	}
	return nil
}

// knowHowStep appends one KNOWHOW_ADD or KNOWHOW_RETRACT entry (KHN-V0-003)
// to a live native home ticket. Actor, role, time and seq come from the
// trusted context; nothing acceptance-relevant changes, so finalize bumps
// revision alone. The secret screen runs here, after request replay, so only
// a fresh write is screened.
func (ctx *Context) knowHowStep(work *ticket.Record, p Payload) *refusal {
	op := p.operation()
	if !isLive(work.Status) {
		return refuse(OutcomeBlocked, wire.CodeTicketState, "%s requires status DRAFT, OPEN or HELD (is %s)", op, work.Status)
	}
	if work.Source.Kind != "NATIVE" {
		return refuse(OutcomeBlocked, wire.CodeTicketState, "%s requires a native record (source.kind is %s)", op, work.Source.Kind)
	}
	if len(work.KnowHow) >= wire.KnowHowMaxEntries {
		return refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "more than %d know-how entries on one ticket", wire.KnowHowMaxEntries)
	}
	entry := ticket.KnowHowEntry{
		Seq:        wire.CountOf(int64(len(work.KnowHow) + 1)),
		ActorID:    ctx.Binding.ID,
		ActorRole:  ctx.Binding.Role,
		RecordedAt: ctx.Now,
	}
	active := ticket.KnowHowActiveSeqs(work.KnowHow)
	switch p := p.(type) {
	case *KnowHowAddPayload:
		if p.Supersedes != nil && !active[*p.Supersedes] {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "supersedes names note %s, which is not an active note on this ticket", *p.Supersedes)
		}
		fields := map[string]string{"text": p.Text}
		if p.Reason != nil {
			fields["reason"] = *p.Reason
		}
		if r := screenKnowHow(fields, p); r != nil {
			return r
		}
		entry.Operation = ticket.KnowHowAdd
		entry.Text = p.Text
		entry.Anchors = append([]ticket.KnowHowAnchor{}, p.Anchors...)
		entry.Routes = append([]string{}, p.Routes...)
		entry.Commit = p.Commit
		entry.Supersedes = p.Supersedes
		entry.Reason = p.Reason
		entry.Attempt = p.Attempt
		entry.Generation = p.Generation
		entry.EvidencePath = p.EvidencePath
	case *KnowHowRetractPayload:
		if !active[p.Note] {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "note %s is not an active note on this ticket", p.Note)
		}
		if r := screenKnowHow(map[string]string{"reason": p.Reason}, nil); r != nil {
			return r
		}
		reason := p.Reason
		entry.Operation = ticket.KnowHowRetract
		entry.Note = p.Note
		entry.Reason = &reason
	}
	work.KnowHow = append(append([]ticket.KnowHowEntry{}, work.KnowHow...), entry)
	return nil
}

// screenKnowHow refuses a write whose free text, route tokens or paths match
// the shared Core secret screen (decision 0397, V1-0955 addendum). The detail
// names only the field.
func screenKnowHow(fields map[string]string, add *KnowHowAddPayload) *refusal {
	for _, name := range []string{"text", "reason"} {
		if s, ok := fields[name]; ok && secretscreen.MatchString(s) {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s: /payload/%s matches the secret screen; remove the secret and retry with a new request ID", KnowHowSecretDetail, name)
		}
	}
	if add == nil {
		return nil
	}
	extra := append([]string{}, add.Routes...)
	for _, a := range add.Anchors {
		extra = append(extra, a.Path)
	}
	if add.EvidencePath != nil {
		extra = append(extra, *add.EvidencePath)
	}
	for _, s := range extra {
		if secretscreen.MatchString(s) {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s: a route or path in /payload matches the secret screen", KnowHowSecretDetail)
		}
	}
	return nil
}

// Stable detail prefixes of the WORKER KNOWHOW_ADD scope refusals
// (KHN-V0-017). Each refusal reuses a closed §11 code; the prefix names the
// reason so a client can tell the cases apart without a new code.
const (
	KnowHowWorkerSupersede       = "KNOWHOW_WORKER_SUPERSEDE"
	KnowHowWorkerAttemptRequired = "KNOWHOW_WORKER_ATTEMPT_REQUIRED"
	KnowHowWorkerAttemptStale    = "KNOWHOW_WORKER_ATTEMPT_STALE"
	KnowHowWorkerAttemptForeign  = "KNOWHOW_WORKER_ATTEMPT_FOREIGN"
	KnowHowWorkerOtherTicket     = "KNOWHOW_WORKER_OTHER_TICKET"
	KnowHowWorkerAnchorScope     = "KNOWHOW_WORKER_ANCHOR_OUT_OF_SCOPE"
)

// WorkerAttemptObservation is the transaction layer's read, under the store
// lock, of the attempt a WORKER KNOWHOW_ADD names (KHN-V0-017). Live is true
// only for a present attempt in a non-terminal phase whose lease is held and
// unexpired; every other state, including an absent attempt, is not live.
type WorkerAttemptObservation struct {
	Live       bool
	TicketID   string
	Generation wire.Size
	Holder     string
}

// workerKnowHowScope is the WORKER-only KNOWHOW_ADD scope (KHN-V0-017),
// reached only when policy knowHow.workerAdd granted the operation
// (KHN-V0-016). The note must name the live attempt the binding holds on
// this ticket at its current generation, and every anchor must lie inside
// the ticket's effects.touchPaths, which cannot change while that attempt is
// live. A worker cannot supersede. Details name payload fields, never their
// values, because the secret screen has not run yet. The ordinary knowHowStep checks (status,
// entry cap, secret screen) still run afterwards.
func (ctx *Context) workerKnowHowScope(work *ticket.Record, p *KnowHowAddPayload) *refusal {
	if p.Supersedes != nil {
		return refuse(OutcomeUnauthorized, "", "%s: WORKER may not supersede a know-how note", KnowHowWorkerSupersede)
	}
	if p.Attempt == nil || p.Generation == nil {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s: WORKER KNOWHOW_ADD must name its live attempt and generation", KnowHowWorkerAttemptRequired)
	}
	a := ctx.WorkerAttempt
	if a == nil || !a.Live || a.Generation != *p.Generation {
		return refuse(OutcomeRevisionConflict, wire.CodeFenced, "%s: /payload/attempt and /payload/generation do not name a live attempt generation", KnowHowWorkerAttemptStale)
	}
	if a.Holder != ctx.Binding.ID {
		return refuse(OutcomeUnauthorized, "", "%s: /payload/attempt is not held by the acting binding", KnowHowWorkerAttemptForeign)
	}
	if a.TicketID != work.TicketID.Raw {
		return refuse(OutcomeBlocked, wire.CodeOutOfScope, "%s: /payload/attempt belongs to another ticket", KnowHowWorkerOtherTicket)
	}
	for i, an := range p.Anchors {
		covered := false
		for _, tp := range work.Effects.TouchPaths {
			if ticket.PathCovers(tp, an.Path) {
				covered = true
				break
			}
		}
		if !covered {
			return refuse(OutcomeBlocked, wire.CodeOutOfScope, "%s: /payload/anchors/%d is outside the ticket's effects.touchPaths", KnowHowWorkerAnchorScope, i)
		}
	}
	return nil
}
