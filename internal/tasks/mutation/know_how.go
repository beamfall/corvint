package mutation

import (
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// KnowHowSecretDetail prefixes the refusal detail of a know-how write whose
// free text matches the secret screen. Since KHN-V0-010 the refusal carries
// the owned detail code SECRET_DETECTED instead of MALFORMED; the prefix is
// kept as a deprecated compatibility alias for scripts that matched the
// MALFORMED detail (KHN-V0-011). The detail names the field, never the text.
const KnowHowSecretDetail = "KNOWHOW_SECRET_DETECTED"

// matchesSecretScreen is the one call into the shared secret screen, kept in
// this file so the decision 0397 V1-0955 import edge stays single-file; the
// TOL-V0-009 obligation screen reuses it.
func matchesSecretScreen(s string) bool { return secretscreen.MatchString(s) }

// ScreenKnowHowArgs refuses a know-how command whose raw arguments match the
// secret screen before any parse or pin error can echo them (KHN-V0-004).
// The error is SECRET_DETECTED with the KnowHowSecretDetail prefix and never
// repeats the argument; the mutation step screens the payload again.
func ScreenKnowHowArgs(args []string) error {
	for _, s := range args {
		if secretscreen.MatchString(s) {
			return wire.Errorf(wire.CodeSecretDetected, "/payload", "%s: an argument matches the secret screen; remove the secret and retry with a new request ID", KnowHowSecretDetail)
		}
	}
	return nil
}

// knowHowStep appends one KNOWHOW_ADD, KNOWHOW_RETRACT or KNOWHOW_RECONFIRM
// entry (KHN-V0-003, KHN-V0-018) to a live native home ticket. Actor, role,
// time and seq come from the trusted context; nothing acceptance-relevant
// changes, so finalize bumps revision alone. The secret screen and the
// provenance check (KHN-V0-008) run here, after request replay, so only a
// fresh write is checked.
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
		if err := CheckKnowHowProvenance(ctx.KnowHowAttempts, work.TicketID, p.Attempt, p.Generation); err != nil {
			return refuseErr(err)
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
	case *KnowHowReconfirmPayload:
		if !active[p.Note] {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "note %s is not an active note on this ticket", p.Note)
		}
		if why := ticket.KnowHowReconfirmRefusal(knowHowEffectivePins(work.KnowHow, p.Note), p.Anchors); why != "" {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s", why)
		}
		if r := screenKnowHowPaths(p.Anchors, nil, nil); r != nil {
			return r
		}
		if err := CheckKnowHowProvenance(ctx.KnowHowAttempts, work.TicketID, p.Attempt, p.Generation); err != nil {
			return refuseErr(err)
		}
		entry.Operation = ticket.KnowHowReconfirm
		entry.Note = p.Note
		entry.Anchors = append([]ticket.KnowHowAnchor{}, p.Anchors...)
		entry.Commit = p.Commit
		entry.Attempt = p.Attempt
		entry.Generation = p.Generation
	}
	work.KnowHow = append(append([]ticket.KnowHowEntry{}, work.KnowHow...), entry)
	return nil
}

// knowHowEffectivePins is note's anchors as last pinned: by its latest
// RECONFIRM, else by its ADD.
func knowHowEffectivePins(entries []ticket.KnowHowEntry, note wire.Count) []ticket.KnowHowAnchor {
	var pins []ticket.KnowHowAnchor
	for _, k := range entries {
		if (k.Operation == ticket.KnowHowAdd && k.Seq == note) || (k.Operation == ticket.KnowHowReconfirm && k.Note == note) {
			pins = k.Anchors
		}
	}
	return pins
}

// screenKnowHow refuses a write whose free text, route tokens or paths match
// the shared Core secret screen (decision 0397, V1-0955 addendum). The detail
// names only the field; the code is SECRET_DETECTED (KHN-V0-010).
func screenKnowHow(fields map[string]string, add *KnowHowAddPayload) *refusal {
	for _, name := range []string{"text", "reason"} {
		if s, ok := fields[name]; ok && secretscreen.MatchString(s) {
			return refuse(OutcomeValidationFailed, wire.CodeSecretDetected, "%s: /payload/%s matches the secret screen; remove the secret and retry with a new request ID", KnowHowSecretDetail, name)
		}
	}
	if add == nil {
		return nil
	}
	return screenKnowHowPaths(add.Anchors, add.Routes, add.EvidencePath)
}

// screenKnowHowPaths refuses route tokens, anchor paths, symbol names or an
// evidence path that match the shared secret screen.
func screenKnowHowPaths(anchors []ticket.KnowHowAnchor, routes []string, evidencePath *string) *refusal {
	extra := append([]string{}, routes...)
	for _, a := range anchors {
		extra = append(extra, a.Path)
		if a.Symbol != "" {
			extra = append(extra, a.Symbol)
		}
	}
	if evidencePath != nil {
		extra = append(extra, *evidencePath)
	}
	for _, s := range extra {
		if secretscreen.MatchString(s) {
			return refuse(OutcomeValidationFailed, wire.CodeSecretDetected, "%s: a route or path in /payload matches the secret screen", KnowHowSecretDetail)
		}
	}
	return nil
}

// Stable detail prefixes of the WORKER KNOWHOW_ADD scope refusals
// (KHN-V0-022). Each refusal reuses a closed §11 code; the prefix names the
// reason so a client can tell the cases apart without a new code.
const (
	KnowHowWorkerSupersede       = "KNOWHOW_WORKER_SUPERSEDE"
	KnowHowWorkerAttemptRequired = "KNOWHOW_WORKER_ATTEMPT_REQUIRED"
	KnowHowWorkerAttemptStale    = "KNOWHOW_WORKER_ATTEMPT_STALE"
	KnowHowWorkerAttemptForeign  = "KNOWHOW_WORKER_ATTEMPT_FOREIGN"
	KnowHowWorkerAnchorScope     = "KNOWHOW_WORKER_ANCHOR_OUT_OF_SCOPE"
)

// workerKnowHowScope is the WORKER-only KNOWHOW_ADD scope (KHN-V0-022),
// reached only when policy knowHow.workerAdd granted the operation
// (KHN-V0-021). A worker cannot supersede and must name its attempt and
// generation. The shared provenance check (KHN-V0-008) then verifies the
// attempt belongs to this ticket at a recorded generation; this scope adds
// only what is WORKER-specific: the generation is current, the attempt is
// live under an unexpired lease the binding holds, and every anchor lies
// inside the ticket's effects.touchPaths, which cannot change while that
// attempt is live. Details name payload fields, never their values, because
// the secret screen has not run yet. The ordinary knowHowStep checks
// (status, entry cap, secret screen) still run afterwards.
func (ctx *Context) workerKnowHowScope(work *ticket.Record, p *KnowHowAddPayload) *refusal {
	if p.Supersedes != nil {
		return refuse(OutcomeUnauthorized, "", "%s: WORKER may not supersede a know-how note", KnowHowWorkerSupersede)
	}
	if p.Attempt == nil || p.Generation == nil {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "%s: WORKER KNOWHOW_ADD must name its live attempt and generation", KnowHowWorkerAttemptRequired)
	}
	if err := CheckKnowHowProvenance(ctx.KnowHowAttempts, work.TicketID, p.Attempt, p.Generation); err != nil {
		return refuseErr(err)
	}
	a := ctx.KnowHowAttempts[*p.Attempt]
	if !a.Live || a.Holder == "" || a.Generation != *p.Generation {
		return refuse(OutcomeRevisionConflict, wire.CodeFenced, "%s: /payload/attempt and /payload/generation do not name a live attempt generation", KnowHowWorkerAttemptStale)
	}
	if a.Holder != ctx.Binding.ID {
		return refuse(OutcomeUnauthorized, "", "%s: /payload/attempt is not held by the acting binding", KnowHowWorkerAttemptForeign)
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
