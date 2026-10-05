package cli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Escalation read bounds (ESC-V0-009): a page is 1..50 entries, default 20,
// and its items encode to at most 1 MiB.
const (
	escalationPageDefault = 20
	escalationPageMax     = 50
	escalationPageBytes   = 1 << 20
)

var escalationKinds = map[string]bool{"decision": true, "infrastructure": true, "scope": true, "blocked": true}
var escalationStates = map[string]bool{"OPEN": true, "ANSWERED": true, "SUPERSEDED": true}

// escalateCommand runs `ticket escalate` (ESC-V0-001). The origin comes from
// the worker's invocation context, the claim receipt and attempt the claim
// returned; ticket, generation, holder and acceptance revision are read from
// that receipt, so the caller supplies none of them.
func escalateCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "escalate"}
	fl, pos, err := flags(args, "attempt", "claim-receipt", "kind", "question", "options", "supersedes", "expected-request-revision", "blocked-by", "gate", "expected-revision", "request-id", "role")
	if err != nil {
		return errorResult(cmd, err)
	}
	if len(pos) > 0 {
		return usage(cmd, "ticket escalate takes no positional argument: the ticket comes from the claim receipt")
	}
	for _, need := range []string{"attempt", "claim-receipt", "kind", "question", "request-id"} {
		if fl[need] == "" {
			return usage(cmd, "--"+need+" is required")
		}
	}
	if (fl["supersedes"] == "") != (fl["expected-request-revision"] == "") {
		return usage(cmd, "--supersedes and --expected-request-revision go together")
	}
	if fl["gate"] != "" && fl["blocked-by"] == "" {
		return usage(cmd, "--gate needs --blocked-by")
	}
	seq, err := parseClaimReceipt(fl["claim-receipt"])
	if err != nil {
		return errorResult(cmd, err)
	}
	actor, repo, queueID, err := escalationWriter(env, fl)
	if err != nil {
		return errorResult(cmd, err)
	}
	src, ok, err := store.ClaimSource(repo, queueID, fl["attempt"], seq.Uint64())
	if err != nil {
		return errorResult(cmd, err)
	}
	if !ok {
		o := wire.NewObject().Set("outcome", wire.String(mutation.OutcomeBlocked)).Set("escalationCode", wire.String("MISSING_ADMISSION_CONTEXT")).Set("requestIds", wire.Strings([]string{}))
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeRefused, Codes: []string{wire.CodeMissingEvidence}, Items: []wire.Value{wire.ObjectValue(o)},
			Warnings: []string{"claim receipt " + string(seq) + " is not a completed admission of attempt " + prose(fl["attempt"]) + " with a readable POST attempt; nothing was written"}}
	}
	options := []string{}
	if fl["options"] != "" {
		options = strings.Split(fl["options"], ",")
	}
	open := &ticket.EscalationOpen{Source: src, Kind: fl["kind"], Question: fl["question"], Options: options, Supersedes: fl["supersedes"], ExpectedRevision: wire.Count(fl["expected-request-revision"])}
	if fl["blocked-by"] != "" {
		open.BlockedBy = &ticket.EscalationBlockedBy{TicketID: qualifyTicket(queueID, fl["blocked-by"]), Gate: fl["gate"]}
	}
	req := ticket.EscalationRequest{Profile: ticket.EscalationRequestProfile, QueueID: queueID, TicketID: src.TicketID, RequestID: fl["request-id"], Actor: actor.ID, ActorRole: actor.Role, Operation: "OPEN", Open: open}
	return escalationSubmit(cmd, fl, req, repo, actor, store.Escalate)
}

// answerCommand runs `ticket answer` (ESC-V0-004): the sole same-acceptance
// OPEN question at commit, or the exact --request/--expected-request-revision
// compare-and-set. Shorthand never answers all or the newest.
func answerCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "answer"}
	fl, pos, err := flags(args, "target", "text", "request", "expected-request-revision", "expected-revision", "request-id", "role")
	if err != nil {
		return errorResult(cmd, err)
	}
	if len(pos) > 0 {
		return usage(cmd, "ticket answer takes no positional argument: name the ticket with --target")
	}
	for _, need := range []string{"target", "text", "request-id"} {
		if fl[need] == "" {
			return usage(cmd, "--"+need+" is required")
		}
	}
	if (fl["request"] == "") != (fl["expected-request-revision"] == "") {
		return usage(cmd, "--request and --expected-request-revision go together")
	}
	actor, repo, queueID, err := escalationWriter(env, fl)
	if err != nil {
		return errorResult(cmd, err)
	}
	target := qualifyTicket(queueID, fl["target"])
	answer := &ticket.EscalationAnswer{Text: fl["text"], RequestID: fl["request"], ExpectedRevision: wire.Count(fl["expected-request-revision"])}
	req := ticket.EscalationRequest{Profile: ticket.EscalationRequestProfile, QueueID: queueID, TicketID: target, RequestID: fl["request-id"], Actor: actor.ID, ActorRole: actor.Role, Operation: "ANSWER", Answer: answer}
	return escalationSubmit(cmd, fl, req, repo, actor, store.AnswerEscalation)
}

// parseClaimReceipt accepts the receipt sequence or its receipt file name, as
// the claim result prints it.
func parseClaimReceipt(s string) (wire.Size, error) {
	digits := strings.TrimSuffix(s, ".json")
	if trimmed := strings.TrimLeft(digits, "0"); trimmed != "" && len(digits) == 12 {
		digits = trimmed
	}
	seq, err := wire.ParseSize("--claim-receipt", digits)
	if err == nil && seq == "0" {
		err = wire.Errorf(wire.CodeMalformed, "--claim-receipt", "receipt sequence must be positive")
	}
	return seq, err
}

func escalationWriter(env Env, fl map[string]string) (mutation.Binding, *intent.Repository, string, error) {
	role := fl["role"]
	if role == "" {
		role = "OWNER"
	}
	if _, err := mutation.ParseRequestID("requestId", fl["request-id"]); err != nil {
		return mutation.Binding{}, nil, "", err
	}
	actor, err := initActor(role)
	if err != nil {
		return actor, nil, "", err
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return actor, nil, "", err
	}
	observed, err := snapshot.Probe(repo.StateDir)
	if err != nil {
		return actor, nil, "", err
	}
	if observed.Head == nil {
		return actor, nil, "", wire.Errorf(wire.CodeMissingEvidence, "journal", "the native store has no journal head")
	}
	return actor, repo, observed.Head.QueueID.Raw, nil
}

type escalationWrite func(context.Context, *intent.Repository, mutation.Binding, string, []byte, wire.Timestamp) (*store.Report, error)

func escalationSubmit(cmd []string, fl map[string]string, req ticket.EscalationRequest, repo *intent.Repository, actor mutation.Binding, write escalationWrite) *wire.Result {
	if fl["expected-revision"] != "" {
		c, err := wire.ParseCount("--expected-revision", fl["expected-revision"])
		if err != nil {
			return errorResult(cmd, err)
		}
		req.ExpectedTicketRevision = &c
	}
	raw, err := ticket.EncodeEscalationRequest(req)
	if err != nil {
		// Keep a typed bound refusal such as LIMIT_EXCEEDED; only untyped
		// codec errors become MALFORMED.
		var typed *wire.Error
		if !errors.As(err, &typed) {
			err = wire.Errorf(wire.CodeMalformed, "request", "%v", err)
		}
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, err := write(ctx, repo, actor, req.QueueID, raw, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	return escalationResult(cmd, report)
}

// escalationResult adds the typed refusal code, the request IDs an
// AMBIGUOUS_OPEN_QUESTIONS refusal names, and the committed events, so a
// shorthand answer reports the question it resolved.
func escalationResult(cmd []string, report *store.Report) *wire.Result {
	res := mutateResult(cmd, report)
	o := res.Items[0].Obj
	code, ids := wire.Null(), []string{}
	if report.Escalation != nil {
		code = wire.String(report.Escalation.Code)
		ids = append(ids, report.Escalation.RequestIDs...)
	}
	o.Set("escalationCode", code)
	o.Set("requestIds", wire.Strings(ids))
	events := make([]wire.Value, 0, len(report.EscalationEvents))
	for _, ev := range report.EscalationEvents {
		e := wire.NewObject().Set("operation", wire.String(ev.Operation)).Set("escalationId", wire.String(ev.EscalationID)).Set("revision", wire.String(string(ev.Revision)))
		e.Set("resolvedRequestId", wire.String(ev.ResolvedRequestID)).Set("resolvedPreviousRevision", wire.String(string(ev.ResolvedPreviousRevision)))
		e.Set("replacementId", stringOrNull(ev.ReplacementID)).Set("recordedAt", wire.String(string(ev.RecordedAt)))
		events = append(events, wire.ObjectValue(e))
	}
	o.Set("events", wire.Array(events...))
	return res
}

// escalationReadCommand runs `ticket escalation list|show|history`
// (ESC-V0-009). Each is a pure read: it writes no ledger, hydrates no
// evidence and renders question and answer text as untrusted queue data.
func escalationReadCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"ticket", "escalation"}, "ticket escalation needs a verb: list, show or history")
	}
	cmd := []string{"ticket", "escalation", args[0]}
	switch args[0] {
	case "list":
		return escalationList(env, cmd, args[1:])
	case "show":
		return escalationShow(env, cmd, args[1:])
	case "history":
		return escalationHistory(env, cmd, args[1:])
	}
	return usage([]string{"ticket", "escalation"}, "unknown ticket escalation verb "+args[0])
}

func escalationLimit(fl map[string]string) (wire.Count, error) {
	v, ok := fl["limit"]
	if !ok {
		return wire.CountOf(escalationPageDefault), nil
	}
	c, err := wire.ParseCount("--limit", v)
	if err != nil {
		return c, err
	}
	if c.Int() < 1 || c.Int() > escalationPageMax {
		return c, wire.Errorf(wire.CodeLimitExceeded, "--limit", "limit must be 1..%d", escalationPageMax)
	}
	return c, nil
}

type escalationRow struct {
	rec *ticket.Record
	ref ticket.EscalationRef
}

func (r escalationRow) after(ticketID, requestID string) bool {
	return r.rec.TicketID.Raw > ticketID || (r.rec.TicketID.Raw == ticketID && r.ref.RequestID > requestID)
}

// escalationList pages the entries of every ticket in (ticketId, requestId)
// order. The cursor is the immutable origin digest of the last entry of the
// previous page, so a page resumes after that key even when the entry's state
// has since changed.
func escalationList(env Env, cmd []string, args []string) *wire.Result {
	fl, pos, err := flags(args, "kind", "state", "target", "program", "limit", "cursor")
	if err == nil && len(pos) > 0 {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket escalation list takes no positional argument")
	}
	if err == nil && fl["kind"] != "" && !escalationKinds[fl["kind"]] {
		err = wire.Errorf(wire.CodeMalformed, "--kind", "kind must be decision, infrastructure, scope or blocked")
	}
	if err == nil && fl["state"] != "" && !escalationStates[fl["state"]] {
		err = wire.Errorf(wire.CodeMalformed, "--state", "state must be OPEN, ANSWERED or SUPERSEDED")
	}
	var cursor wire.Digest
	if err == nil && fl["cursor"] != "" {
		cursor, err = wire.ParseDigest("--cursor", fl["cursor"])
	}
	var limit wire.Count
	if err == nil {
		limit, err = escalationLimit(fl)
	}
	if err != nil {
		return failure(cmd, nil, err)
	}
	var items []wire.Value
	var pg *wire.Page
	var warnings []string
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		items, pg, warnings = nil, nil, nil
		ids := rc.store.Inventory.Sorted()
		if fl["target"] != "" {
			id, err := resolveTicketArg(rc, fl["target"])
			if err != nil {
				return err
			}
			ids = []string{id}
		}
		sort.Strings(ids)
		var all []escalationRow
		for _, id := range ids {
			rec, ok := rc.store.Inventory.Get(id)
			if !ok || rec.Escalations == nil {
				continue
			}
			for _, ref := range rec.Escalations.Entries {
				all = append(all, escalationRow{rec, ref})
			}
		}
		start := 0
		if cursor != "" {
			found := false
			for i, r := range all {
				if r.ref.OriginSha256 == cursor {
					start, found = i+1, true
					break
				}
			}
			if !found {
				return wire.Errorf(wire.CodeMalformed, "--cursor", "cursor %s names no escalation origin in this listing", cursor)
			}
		}
		var rows []escalationRow
		offset := 0
		for i, r := range all {
			if (fl["kind"] != "" && r.ref.Kind != fl["kind"]) || (fl["state"] != "" && r.ref.State != fl["state"]) || (fl["program"] != "" && fl["program"] != "UNKNOWN") {
				continue
			}
			if i < start {
				offset++
				continue
			}
			rows = append(rows, r)
		}
		if fl["program"] != "" && fl["program"] != "UNKNOWN" {
			warnings = append(warnings, "no admitted producer mapping names a program, so every entry's program is UNKNOWN and --program "+prose(fl["program"])+" matches none")
		}
		now := time.Now().UTC()
		loaded := map[string]escalationMaterial{}
		size := 0
		for _, r := range rows {
			if len(items) == int(limit.Int()) {
				break
			}
			m, ok := loaded[r.rec.TicketID.Raw]
			if !ok {
				m = loadEscalations(rc.repo, rc.store.Queue.QueueID.Raw, r.rec)
				loaded[r.rec.TicketID.Raw] = m
				if m.err != nil {
					warnings = append(warnings, "ticket "+r.rec.TicketID.Raw+" escalation material is unavailable: "+prose(m.err.Error()))
				}
			}
			v := escalationEntryValue(r.rec, r.ref, m, now, false)
			n := len(wire.EncodeFile(v))
			if len(items) > 0 && size+n > escalationPageBytes {
				break
			}
			size += n
			items = append(items, v)
		}
		total := wire.CountOf(int64(offset + len(rows)))
		pg = &wire.Page{Offset: wire.CountOf(int64(offset)), Limit: limit, Total: &total, Truncated: len(items) < len(rows)}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items, res.Page, res.Untrusted = items, pg, true
	if res.Items == nil {
		res.Items = []wire.Value{}
	}
	res.Warnings = append(res.Warnings, warnings...)
	res.Warnings = append(res.Warnings, "retryState is NOT_OBSERVED: the native list reads no dispatcher ledger")
	return res
}

// escalationMaterial is one ticket's escalation blobs after validation, or
// the typed reason they cannot be served.
type escalationMaterial struct {
	blobs map[wire.Digest][]byte
	err   error
}

// loadEscalations reads every origin and head blob of one ticket by digest,
// each within the event bound, and runs the reducer's validator over them:
// digest, identity, terminal chain and supersession pairs. A failure is
// reported, never repaired; a missing blob is MISSING_EVIDENCE.
func loadEscalations(repo *intent.Repository, queueID string, rec *ticket.Record) escalationMaterial {
	blobs := map[wire.Digest][]byte{}
	for _, ref := range rec.Escalations.Entries {
		for _, d := range []wire.Digest{ref.OriginSha256, ref.HeadSha256} {
			if _, ok := blobs[d]; ok {
				continue
			}
			raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)), ticket.EscalationMaxEventBytes)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return escalationMaterial{err: err}
			}
			blobs[d] = raw
		}
	}
	snap := transaction.EscalationSnapshot{QueueID: queueID, TicketID: rec.TicketID.Raw, TicketRevision: rec.Revision, AcceptanceRevision: rec.AcceptanceRevision, Refs: rec.Escalations, Blobs: blobs}
	if _, _, err := transaction.EscalationHolds(snap); err != nil {
		return escalationMaterial{err: escalationReadError(err)}
	}
	return escalationMaterial{blobs: blobs}
}

func escalationReadError(err error) error {
	var r *transaction.EscalationRefusal
	if !errors.As(err, &r) {
		return err
	}
	// The record already decoded, so every other refusal is stored material
	// that disagrees with it: journal damage, never malformed caller input.
	code := wire.CodeJournalForked
	if r.Code == "MISSING_EVIDENCE" {
		code = wire.CodeMissingEvidence
	}
	return wire.Errorf(code, "/escalations", "escalation material refused %s", r.Code)
}

// escalationEntryValue renders one entry with honest age and provenance: the
// original OPEN time, a nonnegative age that is clockUncertain when the local
// clock is behind it, applicability from acceptance revisions, and the
// source generation and holder. Program is UNKNOWN: no admitted producer
// mapping exists, and caller text never supplies one.
func escalationEntryValue(rec *ticket.Record, ref ticket.EscalationRef, m escalationMaterial, now time.Time, detail bool) wire.Value {
	o := wire.NewObject().Set("ticketId", wire.String(rec.TicketID.Raw)).Set("requestId", wire.String(ref.RequestID))
	o.Set("kind", wire.String(ref.Kind)).Set("state", wire.String(ref.State))
	applicability := "CURRENT"
	if ref.AcceptanceRevision != rec.AcceptanceRevision {
		applicability = "STALE"
	}
	o.Set("applicability", wire.String(applicability))
	o.Set("eventRevision", wire.String(string(ref.Revision))).Set("acceptanceRevision", wire.String(string(ref.AcceptanceRevision)))
	o.Set("originSha256", wire.String(string(ref.OriginSha256))).Set("headSha256", wire.String(string(ref.HeadSha256)))
	o.Set("program", wire.String("UNKNOWN")).Set("retryState", wire.String("NOT_OBSERVED"))
	if m.err != nil {
		o.Set("material", wire.String("UNAVAILABLE")).Set("code", wire.String(wire.CodeOf(m.err)))
		return wire.ObjectValue(o)
	}
	origin, err := ticket.DecodeEscalationEvent(m.blobs[ref.OriginSha256])
	if err != nil {
		o.Set("material", wire.String("UNAVAILABLE")).Set("code", wire.String(wire.CodeMalformed))
		return wire.ObjectValue(o)
	}
	o.Set("material", wire.String("AUDITED"))
	open := origin.OriginalRequest.Open
	o.Set("question", wire.String(open.Question)).Set("options", wire.Strings(append([]string{}, open.Options...)))
	o.Set("recordedAt", wire.String(string(origin.RecordedAt)))
	age, uncertain := int64(0), true
	if at, err := time.Parse("2006-01-02T15:04:05Z", string(origin.RecordedAt)); err == nil {
		if d := now.Sub(at); d >= 0 {
			age, uncertain = int64(d/time.Second), false
		}
	}
	o.Set("ageSeconds", wire.String(string(wire.CountOf(age)))).Set("clockUncertain", wire.Bool(uncertain))
	s := origin.Source
	src := wire.NewObject().Set("attemptId", wire.String(s.AttemptID)).Set("generation", wire.String(string(s.Generation))).Set("holder", wire.String(s.Holder))
	src.Set("receiptSequence", wire.String(string(s.ReceiptSequence))).Set("acceptanceRevision", wire.String(string(s.AcceptanceRevision)))
	o.Set("source", wire.ObjectValue(src))
	if !detail {
		return wire.ObjectValue(o)
	}
	o.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(origin.Actor)).Set("role", wire.String(origin.ActorRole))))
	o.Set("supersedes", stringOrNull(open.Supersedes))
	blocked := wire.Null()
	if open.BlockedBy != nil {
		blocked = wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(open.BlockedBy.TicketID)).Set("gate", stringOrNull(open.BlockedBy.Gate)))
	}
	o.Set("blockedBy", blocked)
	answer, replacement := wire.Null(), wire.Null()
	if head, err := ticket.DecodeEscalationEvent(m.blobs[ref.HeadSha256]); err == nil {
		switch ref.State {
		case "ANSWERED":
			a := wire.NewObject().Set("requestId", wire.String(head.OriginalRequest.RequestID)).Set("text", wire.String(head.OriginalRequest.Answer.Text))
			a.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(head.Actor)).Set("role", wire.String(head.ActorRole))))
			a.Set("recordedAt", wire.String(string(head.RecordedAt)))
			answer = wire.ObjectValue(a)
		case "SUPERSEDED":
			replacement = wire.String(head.ReplacementID)
		}
	}
	o.Set("answer", answer).Set("replacementId", replacement)
	return wire.ObjectValue(o)
}

// escalationTarget resolves `<ticket> <requestId>` to one validated entry.
// notFound names what is absent; a material failure refuses outright.
func escalationTarget(rc *readCtx, args []string) (*ticket.Record, ticket.EscalationRef, escalationMaterial, string, error) {
	id, err := resolveTicketArg(rc, args[0])
	if err != nil {
		return nil, ticket.EscalationRef{}, escalationMaterial{}, "", err
	}
	rec, ok := rc.store.Inventory.Get(id)
	if !ok {
		return nil, ticket.EscalationRef{}, escalationMaterial{}, "ticket " + id + " does not exist in this queue", nil
	}
	if rec.Escalations != nil {
		for _, ref := range rec.Escalations.Entries {
			if ref.RequestID == args[1] {
				m := loadEscalations(rc.repo, rc.store.Queue.QueueID.Raw, rec)
				return rec, ref, m, "", m.err
			}
		}
	}
	return nil, ticket.EscalationRef{}, escalationMaterial{}, "ticket " + id + " has no escalation " + prose(args[1]), nil
}

func escalationShow(env Env, cmd []string, args []string) *wire.Result {
	if len(args) != 2 || strings.HasPrefix(args[0], "--") || strings.HasPrefix(args[1], "--") {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "ticket escalation show takes a ticket id or local token and a request id"))
	}
	var item *wire.Value
	notFound := ""
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		item, notFound = nil, ""
		rec, ref, m, missing, err := escalationTarget(rc, args)
		if err != nil || missing != "" {
			notFound = missing
			return err
		}
		v := escalationEntryValue(rec, ref, m, time.Now().UTC(), true)
		item = &v
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	if notFound != "" {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, notFound)
		return res
	}
	res.Items, res.Untrusted = []wire.Value{*item}, true
	return res
}

// escalationHistory pages one request's events newest first, walking from
// the current head to the immutable origin. Each step checks the digest, the
// queue, ticket and escalation identity and origin binding, a revision
// decrement of exactly one and that no digest repeats. The cursor is the
// digest of the next event to return and must lie on that chain.
func escalationHistory(env Env, cmd []string, args []string) *wire.Result {
	fl, pos, err := flags(args, "limit", "cursor")
	if err == nil && len(pos) != 2 {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket escalation history takes a ticket id or local token and a request id")
	}
	var cursor wire.Digest
	if err == nil && fl["cursor"] != "" {
		cursor, err = wire.ParseDigest("--cursor", fl["cursor"])
	}
	var limit wire.Count
	if err == nil {
		limit, err = escalationLimit(fl)
	}
	if err != nil {
		return failure(cmd, nil, err)
	}
	var items []wire.Value
	var pg *wire.Page
	notFound := ""
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		items, pg, notFound = nil, nil, ""
		rec, ref, m, missing, err := escalationTarget(rc, pos)
		if err != nil || missing != "" {
			notFound = missing
			return err
		}
		chain, err := escalationChain(rc.repo, rc.store.Queue.QueueID.Raw, rec, ref, m.blobs)
		// Each event carries the question's original time, age, applicability
		// and source, so a page that starts at an answer keeps its provenance.
		summary := escalationEntryValue(rec, ref, m, time.Now().UTC(), false)
		if err != nil {
			return err
		}
		start := 0
		if cursor != "" {
			start = -1
			for i, ev := range chain {
				if ev.sha == cursor {
					start = i
				}
			}
			if start < 0 {
				return wire.Errorf(wire.CodeMalformed, "--cursor", "cursor %s is not an event of this escalation", cursor)
			}
		}
		end := min(start+int(limit.Int()), len(chain))
		for _, ev := range chain[start:end] {
			v := escalationEventValue(ev)
			v.Obj.Set("escalation", summary)
			items = append(items, v)
		}
		total := wire.CountOf(int64(len(chain)))
		pg = &wire.Page{Offset: wire.CountOf(int64(start)), Limit: limit, Total: &total, Truncated: end < len(chain)}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	if notFound != "" {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, notFound)
		return res
	}
	res.Items, res.Page, res.Untrusted = items, pg, true
	return res
}

type escalationLink struct {
	sha wire.Digest
	ev  ticket.EscalationEvent
}

func escalationChain(repo *intent.Repository, queueID string, rec *ticket.Record, ref ticket.EscalationRef, blobs map[wire.Digest][]byte) ([]escalationLink, error) {
	forked := func(msg string) error {
		return wire.Errorf(wire.CodeJournalForked, "/escalations", "escalation %s history %s", ref.RequestID, msg)
	}
	var chain []escalationLink
	seen := map[wire.Digest]bool{}
	for d := ref.HeadSha256; ; {
		if seen[d] || len(chain) >= int(ref.Revision.Int()) {
			return nil, forked("repeats an event or exceeds its revision")
		}
		seen[d] = true
		raw, ok := blobs[d]
		if !ok {
			var err error
			raw, err = intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)), ticket.EscalationMaxEventBytes)
			if errors.Is(err, fs.ErrNotExist) {
				return nil, wire.Errorf(wire.CodeMissingEvidence, "/escalations", "escalation event %s is not in the evidence store", d)
			}
			if err != nil {
				return nil, err
			}
		}
		if wire.Sum(raw) != d {
			return nil, forked("event digest differs")
		}
		ev, err := ticket.DecodeEscalationEvent(raw)
		if err != nil {
			return nil, err
		}
		want := ref.Revision
		if len(chain) > 0 {
			want = wire.CountOf(chain[len(chain)-1].ev.Revision.Int() - 1)
		}
		if ev.QueueID != queueID || ev.TicketID != rec.TicketID.Raw || ev.EscalationID != ref.RequestID || ev.Revision != want {
			return nil, forked("breaks identity or revision order")
		}
		chain = append(chain, escalationLink{d, ev})
		if ev.Operation == "OPEN" {
			if d != ref.OriginSha256 || ev.Revision != "1" || ev.PreviousSha256 != nil {
				return nil, forked("does not end at its origin")
			}
			return chain, nil
		}
		if ev.QuestionOriginSha256 == nil || *ev.QuestionOriginSha256 != ref.OriginSha256 || ev.PreviousSha256 == nil {
			return nil, forked("does not bind its origin")
		}
		d = *ev.PreviousSha256
	}
}

func escalationEventValue(l escalationLink) wire.Value {
	ev := l.ev
	o := wire.NewObject().Set("sha256", wire.String(string(l.sha))).Set("operation", wire.String(ev.Operation)).Set("revision", wire.String(string(ev.Revision)))
	o.Set("requestId", wire.String(ev.OriginalRequest.RequestID)).Set("requestSha256", wire.String(string(ev.RequestSha256)))
	o.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(ev.Actor)).Set("role", wire.String(ev.ActorRole))))
	o.Set("recordedAt", wire.String(string(ev.RecordedAt)))
	prev := wire.Null()
	if ev.PreviousSha256 != nil {
		prev = wire.String(string(*ev.PreviousSha256))
	}
	o.Set("previousSha256", prev).Set("replacementId", stringOrNull(ev.ReplacementID))
	text := wire.Null()
	switch {
	case ev.Operation == "OPEN" && ev.OriginalRequest.Open != nil:
		text = wire.String(ev.OriginalRequest.Open.Question)
	case ev.Operation == "ANSWER" && ev.OriginalRequest.Answer != nil:
		text = wire.String(ev.OriginalRequest.Answer.Text)
	}
	o.Set("text", text)
	return wire.ObjectValue(o)
}
