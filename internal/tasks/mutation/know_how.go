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

// knowHowStep appends one KNOWHOW_ADD or KNOWHOW_RETRACT entry (KHN-V0-003)
// to a live native home ticket. Actor, role, time and seq come from the
// trusted context; nothing acceptance-relevant changes, so finalize bumps
// revision alone. The secret screen and the provenance check (KHN-V0-008)
// run here, after request replay, so only a fresh write is checked.
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
	}
	work.KnowHow = append(append([]ticket.KnowHowEntry{}, work.KnowHow...), entry)
	return nil
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
	extra := append([]string{}, add.Routes...)
	for _, a := range add.Anchors {
		extra = append(extra, a.Path)
	}
	if add.EvidencePath != nil {
		extra = append(extra, *add.EvidencePath)
	}
	for _, s := range extra {
		if secretscreen.MatchString(s) {
			return refuse(OutcomeValidationFailed, wire.CodeSecretDetected, "%s: a route or path in /payload matches the secret screen", KnowHowSecretDetail)
		}
	}
	return nil
}
