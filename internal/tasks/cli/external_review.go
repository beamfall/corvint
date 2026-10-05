package cli

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// externalReviewGateViews is the native workState observation of one
// ticket's external review gates (ERG-V0-009): every gate reference on the
// record, read through the pure adapter against its current binding. fold is
// the receipt history's binding audit (store.FoldExternalReviews); it names
// the submissions that superseded a subject (ERG-V0-006).
func externalReviewGateViews(repo *intent.Repository, rec *ticket.Record, policy *intent.Policy, attempts map[string]*snapshot.Attempt, fold *transaction.ExternalReviewReceiptAudit) (map[string]dispatch.GateView, error) {
	refs := map[string]snapshot.ExternalReviewRef{}
	for gate, ref := range rec.ExternalReviews {
		refs[gate] = snapshot.ExternalReviewRef{Generation: ref.Generation, Revision: ref.Revision, Head: ref.Head}
	}
	blob := store.ExternalReviewBlob(repo)
	views, err := transaction.ExternalReviewGates(rec.TicketID.Raw, refs, blob, transaction.ExternalReviewCurrentBindings(rec, policy, attempts, blob, fold.Superseded))
	if err != nil {
		return nil, err
	}
	out := make(map[string]dispatch.GateView, len(views))
	for gate, v := range views {
		g := dispatch.GateView{Status: v.Status, Generation: string(v.Generation), Revision: string(v.Revision), Resubmitted: v.Resubmitted, Head: string(refs[gate].Head)}
		if v.Verdict != nil {
			g.Verdict = *v.Verdict
		}
		out[gate] = g
	}
	return out, nil
}

// externalReviewReceiptBinding folds every retained receipt through the pure
// ERG-V0-009 binding audit, so `receipt audit` refuses a review event that
// does not record its receipt's transition.
func externalReviewReceiptBinding(repo *intent.Repository, last wire.Size) error {
	_, err := store.FoldExternalReviews(repo, last.Uint64(), nil)
	return err
}

type reviewFlags struct {
	mutateFlags
	gate, verdict, subject, reviewer, author, priorReturn string
	expectedGeneration, expectedRevision                  string
	reasons                                               []string
}

// gateReviewCommand runs `gate record|resubmit` (ERG-V0-009). It composes the
// closed review request from audited reads (acceptance revision, definition
// and policy digests, the subject's BUILT receipt and the named leases) and
// submits an ordinary REVIEW_* mutation; the locked writer re-derives and
// re-checks every fact, so nothing composed here is authority.
func gateReviewCommand(env Env, verb string, args []string) *wire.Result {
	cmd := []string{"gate", verb}
	operation := map[string]string{"record": mutation.OpReviewRecord, "resubmit": mutation.OpReviewResubmit}[verb]
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	f := reviewFlags{mutateFlags: mutateFlags{role: "OPERATOR", target: args[0]}}
	set := map[string]*string{
		"--gate": &f.gate, "--subject-receipt": &f.subject, "--expected-generation": &f.expectedGeneration,
		"--expected-revision": &f.expectedRevision, "--request-id": &f.requestID, "--issued-at": &f.issuedAt, "--role": &f.role,
	}
	if operation == mutation.OpReviewRecord {
		set["--verdict"], set["--reviewer-attempt"] = &f.verdict, &f.reviewer
	} else {
		set["--author-attempt"], set["--prior-return"] = &f.author, &f.priorReturn
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		if i+1 >= len(rest) {
			return usage(cmd, rest[i]+" needs a value")
		}
		if rest[i] == "--reason" {
			f.reasons = append(f.reasons, rest[i+1])
			i++
			continue
		}
		dest, ok := set[rest[i]]
		if !ok {
			return usage(cmd, "unknown flag "+rest[i])
		}
		i++
		*dest = rest[i]
	}
	if f.requestID == "" || f.gate == "" || f.subject == "" || f.expectedGeneration == "" || f.expectedRevision == "" {
		return usage(cmd, "--request-id, --gate, --subject-receipt, --expected-generation and --expected-revision are required")
	}
	if operation == mutation.OpReviewRecord && f.verdict == "" {
		return usage(cmd, "--verdict PASS|RETURN is required")
	}
	if operation == mutation.OpReviewResubmit && f.author == "" {
		return usage(cmd, "--author-attempt is required")
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	var request wire.Value
	rc, err := withStore(env, func(rc *readCtx) error {
		v, retained, err := retainedReviewRequest(rc, operation, actor, f)
		if err != nil || retained {
			request = v
			return err
		}
		v, err = composeReviewRequest(rc, operation, actor, f)
		request = v
		return err
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	payload := wire.NewObject().Set("request", request)
	return submitMutation(env, cmd, operation, actor, f.mutateFlags, wire.ObjectValue(payload))
}

// retainedReviewRequest makes a retry reproducible (ERG-V0-009): when the
// gate's retained chain already holds an event for --request-id whose
// request matches every caller input (action, counters, subject, verdict,
// reasons, leases, prior RETURN when given, actor), the retained request
// bytes are resubmitted instead of a fresh composition, so a later policy,
// lease or head change cannot alter a retry's envelope. A retained event for
// --request-id whose request differs in any caller input refuses at once as
// REQUEST_ID_CONFLICT, before any fresh composition or policy lookup.
func retainedReviewRequest(rc *readCtx, operation string, actor mutation.Binding, f reviewFlags) (wire.Value, bool, error) {
	id, err := resolveTicketArg(rc, f.target)
	if err != nil {
		return wire.Value{}, false, err
	}
	rec, ok := rc.store.Inventory.Get(id)
	if !ok {
		return wire.Value{}, false, nil
	}
	ref, ok := rec.ExternalReviews[f.gate]
	if !ok {
		return wire.Value{}, false, nil
	}
	blob := store.ExternalReviewBlob(rc.repo)
	at := &ref.Head
	for n := 0; at != nil && n < snapshot.MaxExternalReviewEvents; n++ {
		raw, ok := blob(*at)
		if !ok || wire.Sum(raw) != *at {
			return wire.Value{}, false, nil
		}
		e, err := snapshot.CanonicalExternalReviewEvent(raw)
		if err != nil || e.Request.TicketID != id || e.Request.GateID != f.gate {
			return wire.Value{}, false, nil
		}
		if e.Request.RequestID == f.requestID {
			if !retainedReviewMatches(e, operation, actor, f) {
				return wire.Value{}, false, wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request %s is retained on gate %s with different inputs", f.requestID, f.gate)
			}
			body, err := e.Request.Encode()
			if err != nil {
				return wire.Value{}, false, err
			}
			v, err := wire.Parse(body)
			return v, err == nil, err
		}
		at = e.Previous
	}
	return wire.Value{}, false, nil
}

func retainedReviewMatches(e *snapshot.ExternalReviewEvent, operation string, actor mutation.Binding, f reviewFlags) bool {
	q := e.Request
	if e.ActorID != actor.ID || e.ActorRole != actor.Role || string(q.ExpectedGeneration) != f.expectedGeneration || string(q.ExpectedRevision) != f.expectedRevision ||
		string(q.Subject.ReceiptSeq) != f.subject || len(q.Reasons) != len(f.reasons) {
		return false
	}
	for i, r := range q.Reasons {
		if r.Code+":"+r.Text != f.reasons[i] {
			return false
		}
	}
	if operation == mutation.OpReviewRecord {
		reviewer := ""
		if q.ReviewerLease != nil {
			reviewer = q.ReviewerLease.AttemptID
		}
		return q.Action == "RECORD" && q.Verdict != nil && *q.Verdict == f.verdict && reviewer == f.reviewer
	}
	return q.Action == "RESUBMIT" && q.AuthorLease != nil && q.AuthorLease.AttemptID == f.author &&
		(f.priorReturn == "" || (q.PriorReturn != nil && string(*q.PriorReturn) == f.priorReturn))
}

func composeReviewRequest(rc *readCtx, operation string, actor mutation.Binding, f reviewFlags) (wire.Value, error) {
	id, err := resolveTicketArg(rc, f.target)
	if err != nil {
		return wire.Value{}, err
	}
	rec, ok := rc.store.Inventory.Get(id)
	if !ok {
		return wire.Value{}, wire.Errorf(wire.CodeMalformed, "ticket", "ticket %s does not exist in this queue", id)
	}
	def := rc.store.Policy.ExternalReview(f.gate)
	if def == nil {
		return wire.Value{}, wire.Errorf(wire.CodeGateUnknown, "--gate", "policy declares no external review gate %s", f.gate)
	}
	in, _, err := planInput(rc)
	if err != nil {
		return wire.Value{}, err
	}
	q := snapshot.ExternalReviewRequest{RequestID: f.requestID, Action: "RECORD", TicketID: id, GateID: f.gate, ExpectedGeneration: wire.Count(f.expectedGeneration), ExpectedRevision: wire.Count(f.expectedRevision), AcceptanceRevision: rec.AcceptanceRevision, DefinitionSha256: def.Sha256, PolicySha256: wire.Sum(rc.store.Policy.Raw), Reasons: []snapshot.ExternalReviewReason{}, Evidence: []snapshot.GateEvidence{}}
	for _, r := range f.reasons {
		code, text, ok := strings.Cut(r, ":")
		if !ok {
			return wire.Value{}, wire.Errorf(wire.CodeMalformed, "--reason", "a reason is CODE:TEXT")
		}
		q.Reasons = append(q.Reasons, snapshot.ExternalReviewReason{Code: code, Text: text})
	}
	if q.Subject, q.Candidate, err = reviewSubject(rc.repo, f.subject); err != nil {
		return wire.Value{}, err
	}
	lease := func(attemptID string) (*snapshot.ExternalReviewLease, error) {
		a := in.Attempts[attemptID]
		if a == nil || a.Lease == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "attempt", "attempt %s holds no lease", attemptID)
		}
		return &snapshot.ExternalReviewLease{AttemptID: a.AttemptID, Holder: a.Lease.Holder, Generation: a.Generation}, nil
	}
	if operation == mutation.OpReviewRecord {
		verdict := f.verdict
		q.Verdict = &verdict
		if f.reviewer != "" {
			if q.ReviewerLease, err = lease(f.reviewer); err != nil {
				return wire.Value{}, err
			}
		}
	} else {
		q.Action = "RESUBMIT"
		if q.AuthorLease, err = lease(f.author); err != nil {
			return wire.Value{}, err
		}
		prior := wire.Digest(f.priorReturn)
		if f.priorReturn == "" {
			ref, ok := rec.ExternalReviews[f.gate]
			if !ok {
				return wire.Value{}, wire.Errorf(wire.CodeMalformed, "--prior-return", "gate %s has no RETURN to resubmit against", f.gate)
			}
			prior = ref.Head
		}
		q.PriorReturn = &prior
	}
	raw, err := q.Encode()
	if err != nil {
		return wire.Value{}, err
	}
	return wire.Parse(raw)
}

// reviewSubject reads the BUILT transition receipt the reviewer names and
// composes the subject and TREE candidate it posts.
func reviewSubject(repo *intent.Repository, seqArg string) (snapshot.ExternalReviewSubject, snapshot.ExternalReviewCandidate, error) {
	none := func(f string, a ...any) (snapshot.ExternalReviewSubject, snapshot.ExternalReviewCandidate, error) {
		return snapshot.ExternalReviewSubject{}, snapshot.ExternalReviewCandidate{}, wire.Errorf(wire.CodeMalformed, "--subject-receipt", f, a...)
	}
	seq, err := strconv.ParseUint(seqArg, 10, 64)
	if err != nil || seq == 0 {
		return none("subject receipt is a positive sequence")
	}
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		return none("%v", err)
	}
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "receipts", name), wire.MaxReceiptFileBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return none("receipt %d does not exist", seq)
	}
	if err != nil {
		return none("%v", err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil || rc.AttemptID == nil || rc.Generation == nil {
		return none("receipt %d is not an attempt transition", seq)
	}
	for _, p := range rc.Post {
		if p.Path != "attempts/"+*rc.AttemptID+".json" || p.Record == nil || p.Sha256 == nil {
			continue
		}
		a, err := snapshot.DecodeAttempt(wire.EncodeFile(*p.Record))
		if err != nil || a.CandidateTreeOid == nil {
			return none("receipt %d posts no candidate tree", seq)
		}
		return snapshot.ExternalReviewSubject{AttemptID: *rc.AttemptID, Generation: *rc.Generation, ReceiptSeq: rc.Seq, ReceiptSha256: wire.Sum(raw), AttemptSha256: *p.Sha256}, snapshot.ExternalReviewCandidate{Kind: "TREE", TreeOID: *a.CandidateTreeOid}, nil
	}
	return none("receipt %d posts no inline attempt record", seq)
}

// gateHistory runs `gate history` (ERG-V0-009): one gate's events newest
// first, anchored at the ticket's head reference; page at most 50 events
// (default 20) and 1 MiB. A truncated page names its next cursor.
func gateHistory(env Env, args []string) *wire.Result {
	cmd := []string{"gate", "history"}
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	var gate, cursor, limit string
	set := map[string]*string{"--gate": &gate, "--cursor": &cursor, "--limit": &limit}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		dest, ok := set[rest[i]]
		if !ok || i+1 >= len(rest) {
			return usage(cmd, "unknown flag or missing value "+rest[i])
		}
		i++
		*dest = rest[i]
	}
	if gate == "" {
		return usage(cmd, "--gate is required")
	}
	n := 0
	if limit != "" {
		v, err := strconv.Atoi(limit)
		if err != nil || v < 1 || v > transaction.ExternalReviewHistoryMax {
			return usage(cmd, "--limit is 1..50")
		}
		n = v
	}
	var at *wire.Digest
	if cursor != "" {
		d, err := wire.ParseDigest("--cursor", cursor)
		if err != nil {
			return usage(cmd, err.Error())
		}
		at = &d
	}
	var page transaction.ExternalReviewPage
	rc, err := withStore(env, func(rc *readCtx) error {
		id, err := resolveTicketArg(rc, args[0])
		if err != nil {
			return err
		}
		rec, ok := rc.store.Inventory.Get(id)
		if !ok {
			return wire.Errorf(wire.CodeMalformed, "ticket", "ticket %s does not exist in this queue", id)
		}
		ref, ok := rec.ExternalReviews[gate]
		if !ok {
			page = transaction.ExternalReviewPage{}
			return nil
		}
		page, err = transaction.ExternalReviewHistory(snapshot.ExternalReviewRef{Generation: ref.Generation, Revision: ref.Revision, Head: ref.Head}, store.ExternalReviewBlob(rc.repo), id, gate, at, n)
		if err != nil {
			return wire.Errorf(wire.CodeMissingEvidence, "gate history", "%v", err)
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{}
	for _, raw := range page.Events {
		v, err := wire.Parse(raw)
		if err != nil {
			return failure(cmd, rc, err)
		}
		o := wire.NewObject().Set("sha256", wire.String(string(wire.Sum(raw)))).Set("event", v)
		res.Items = append(res.Items, wire.ObjectValue(o))
	}
	if page.Next != nil {
		res.Warnings = append(res.Warnings, "truncated: next --cursor "+string(*page.Next))
	}
	res.Untrusted = true
	return res
}
