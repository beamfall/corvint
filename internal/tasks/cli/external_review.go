package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
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
func externalReviewGateViews(repo *intent.Repository, rec *ticket.Record, policy *intent.Policy, fold *transaction.ExternalReviewReceiptAudit) (map[string]dispatch.GateView, error) {
	views, err := externalReviewViews(repo, rec, policy, fold)
	if err != nil {
		return nil, err
	}
	out := make(map[string]dispatch.GateView, len(views))
	for gate, v := range views {
		g := dispatch.GateView{Status: v.Status, Generation: string(v.Generation), Revision: string(v.Revision), Resubmitted: v.Resubmitted, Head: string(rec.ExternalReviews[gate].Head)}
		if v.Verdict != nil {
			g.Verdict = *v.Verdict
		}
		if c := v.Candidate; c != nil {
			g.Candidate = dispatch.GateCandidate{Kind: c.Kind, TreeOID: c.TreeOID, Sha256: string(c.Sha256), Bytes: string(c.Bytes)}
		}
		if q := v.Subject; q != nil {
			g.Subject = dispatch.GateSubject{AttemptID: q.AttemptID, Generation: string(q.Generation), ReceiptSeq: string(q.ReceiptSeq), ReceiptSha256: string(q.ReceiptSha256), AttemptSha256: string(q.AttemptSha256)}
		}
		g.Trust = dispatch.GateTrust{ActorAuthentication: v.ActorAuthentication, Independence: v.Independence, Source: v.TrustSource}
		if v.EvidenceSha256 != nil {
			g.EvidenceSha256 = string(*v.EvidenceSha256)
		}
		out[gate] = g
	}
	return out, nil
}

// externalReviewViews reads every gate reference on the record through the
// pure adapter against its current binding (ERG-V0-009).
func externalReviewViews(repo *intent.Repository, rec *ticket.Record, policy *intent.Policy, fold *transaction.ExternalReviewReceiptAudit) (map[string]transaction.ExternalReviewView, error) {
	refs := map[string]snapshot.ExternalReviewRef{}
	for gate, ref := range rec.ExternalReviews {
		refs[gate] = snapshot.ExternalReviewRef{Generation: ref.Generation, Revision: ref.Revision, Head: ref.Head}
	}
	blob := store.ExternalReviewBlob(repo)
	return transaction.ExternalReviewGates(rec.TicketID.Raw, refs, blob, transaction.ExternalReviewCurrentBindings(rec, policy, blob, fold.Superseded))
}

// completionOffers derives the ERG-V0-011 read-only completion offer for
// tickets of one read. It is pure: it reads the audited plan input, retained
// review events and receipts only, folds the receipts at most once and only
// for a ticket that carries a review reference under a policy declaring
// review gates, and gives no offer (nil) when the journal is absent, the
// planner reports any claim blocker or unknown for the ticket, the fold or a
// gate view refuses, or any required gate is not a CURRENT PASS.
type completionOffers struct {
	rc     *readCtx
	in     transaction.PlanInput
	fold   *transaction.ExternalReviewReceiptAudit
	folded bool
}

func (o *completionOffers) evidence(rec *ticket.Record) []wire.Digest {
	policy := o.rc.store.Policy
	if o.rc.journalAbsent || rec == nil || policy == nil || len(policy.ExternalReviews) == 0 || len(rec.ExternalReviews) == 0 {
		return nil
	}
	if !o.folded {
		o.folded = true
		o.fold, _ = store.FoldExternalReviews(o.rc.repo, o.rc.snap.Head.LastSeq.Uint64(), nil)
	}
	if o.fold == nil {
		return nil
	}
	views, err := externalReviewViews(o.rc.repo, rec, policy, o.fold)
	if err != nil {
		return nil
	}
	return transaction.ExternalReviewCompletionOffer(policy, views)
}

// offer applies the completion offer to one view. It is the one predicate
// `ticket show`, `ticket blockers` and `plan preview` share: the ticket view
// must be an unblocked admit, and the planner's claim-blocker derivation over
// the same plan input (queue pause, execution cutover, budget, pool
// eligibility, retry exhaustion, holds, dependencies and every unknown) must
// be empty, so an entry the plan would block never offers.
func (o *completionOffers) offer(v *ticket.View) {
	if o.rc.journalAbsent || v.Record == nil || v.NextAction != "admit" || len(v.Blockers) != 0 || len(v.Unknowns) != 0 {
		return
	}
	if len(transaction.ClaimBlockerObservations(o.in, v.Record)) != 0 {
		return
	}
	v.OfferCompletion(o.evidence(v.Record))
}

// receiptMaterialBindings folds every retained receipt through the pure
// ERG-V0-009 review and ESC-V0-010 escalation binding audits, so `receipt
// audit` refuses an event that does not record its receipt's transition.
// A complete audit that folded every receipt it validated supplies the
// outcome; otherwise every receipt is read and folded again.
func receiptMaterialBindings(repo *intent.Repository, audited *journal.Result) error {
	folded, err := audited.ReceiptFold()
	if err != nil {
		return err
	}
	if audited.Mode == journal.ModeFull && folded != 0 && folded == audited.LastSeq.Uint64() {
		return nil
	}
	return store.FoldReceiptBindings(repo, audited.LastSeq.Uint64(), nil)
}

type reviewFlags struct {
	mutateFlags
	gate, verdict, subject, reviewer, author, priorReturn string
	expectedGeneration, expectedRevision                  string
	reasons                                               []string
	// candidate is --candidate-evidence SHA256:BYTES (or derived from
	// --from-acceptance); evidence holds each --evidence LABEL=SHA256:BYTES.
	candidate, acceptance string
	evidence              []string
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
		"--candidate-evidence": &f.candidate,
	}
	if operation == mutation.OpReviewRecord {
		set["--verdict"], set["--reviewer-attempt"], set["--from-acceptance"] = &f.verdict, &f.reviewer, &f.acceptance
	} else {
		set["--author-attempt"], set["--prior-return"] = &f.author, &f.priorReturn
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		if i+1 >= len(rest) {
			return usage(cmd, rest[i]+" needs a value")
		}
		if rest[i] == "--reason" || rest[i] == "--evidence" {
			if rest[i] == "--reason" {
				f.reasons = append(f.reasons, rest[i+1])
			} else {
				f.evidence = append(f.evidence, rest[i+1])
			}
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
	if f.acceptance != "" {
		if err := acceptanceFlags(&f); err != nil {
			return errorResult(cmd, err)
		}
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

// acceptanceFlags reads an issue 394 acceptance report (`corvint tests
// accept`) and derives the verdict and EVIDENCE candidate it supports: the
// report's own digest and size, which the writer admits only when an
// executable gate run of the subject retained exactly those bytes, and
// accepted to PASS or rejected to RETURN (owner decision 2026-10-04). A
// blocked report supports no verdict and refuses as MISSING_EVIDENCE; a
// file that is not such a report, or a conflicting --verdict or
// --candidate-evidence, is malformed. The writer re-applies the mapping to
// the retained artifact, so this composition is not authority.
func acceptanceFlags(f *reviewFlags) error {
	raw, err := intent.ReadFile(f.acceptance, wire.MaxGateOutputBytes)
	if err != nil {
		return err
	}
	verdict, report, err := transaction.ExternalAcceptanceVerdict(raw)
	switch {
	case err != nil:
		return err
	case !report:
		return wire.Errorf(wire.CodeMalformed, "--from-acceptance", "the file is not a corvint-new-e2e-assessment report")
	case verdict == "":
		return wire.Errorf(wire.CodeMissingEvidence, "--from-acceptance", "acceptance verdict blocked supports no review verdict")
	}
	candidate := string(wire.Sum(raw)) + ":" + strconv.Itoa(len(raw))
	if (f.verdict != "" && f.verdict != verdict) || (f.candidate != "" && f.candidate != candidate) {
		return wire.Errorf(wire.CodeMalformed, "--from-acceptance", "the report supports verdict %s on candidate %s", verdict, candidate)
	}
	f.verdict, f.candidate = verdict, candidate
	return nil
}

// artifactArg parses SHA256:BYTES.
func artifactArg(flag, arg string) (wire.Digest, wire.Size, error) {
	sha, n, ok := strings.Cut(arg, ":")
	d, err := wire.ParseDigest(flag, sha)
	if err != nil || !ok {
		return "", "", wire.Errorf(wire.CodeMalformed, flag, "an artifact is SHA256:BYTES")
	}
	size, err := wire.ParseSize(flag, n)
	if err != nil {
		return "", "", err
	}
	return d, size, nil
}

// retainedReviewRequest makes a retry reproducible (ERG-V0-009): when the
// gate's retained chain already holds an event for --request-id whose
// request matches every caller input (action, counters, subject, verdict,
// reasons, leases, prior RETURN when given, actor), the retained request
// bytes are resubmitted instead of a fresh composition, so a later policy,
// lease or head change cannot alter a retry's envelope. A retained event for
// --request-id whose request differs in any caller input refuses at once as
// REQUEST_ID_CONFLICT, before any fresh composition or policy lookup; so does
// a request id the queue-wide request index retains for another ticket, gate
// or operation.
func retainedReviewRequest(rc *readCtx, operation string, actor mutation.Binding, f reviewFlags) (wire.Value, bool, error) {
	id, err := resolveTicketArg(rc, f.target)
	if err != nil {
		return wire.Value{}, false, err
	}
	v, found, err := retainedChainRequest(rc, id, operation, actor, f)
	if err != nil || found {
		return v, found, err
	}
	return wire.Value{}, false, retainedElsewhere(rc, id, f)
}

// retainedElsewhere refuses a request id the queue already retains outside
// the selected gate's chain. The chain walk covers every event a gate can
// hold, so a completed request that is not on it was another ticket, gate
// or operation; so was any retained request for another ticket. A request
// left uncompleted on the same ticket falls through to the writer, which
// replays or refuses it by its mutation digest. The afterimage path is
// probed first, so a fresh request id costs no extra journal audit; an
// absent afterimage the journal still binds is the writer's to refuse.
func retainedElsewhere(rc *readCtx, id string, f reviewFlags) error {
	path, err := snapshot.RequestPath(f.requestID)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(rc.repo.StateDir, path)); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}}
	entry, found, err := idx.Lookup(f.requestID)
	if err != nil {
		return err
	}
	if idx.Identity.HeadSha256 != rc.snap.HeadSha256 {
		return wire.Errorf(wire.CodeSnapshotMoved, "--request-id", "journal and outer snapshot differ")
	}
	if !found || (idx.TicketID == id && entry.Outcome.Outcome != mutation.OutcomeCompleted) {
		return nil
	}
	return wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request %s is retained for another ticket, gate or operation", f.requestID)
}

// retainedChainRequest walks the selected gate's event chain for
// --request-id: a matching retained request is returned for resubmission and
// a mismatching one is REQUEST_ID_CONFLICT.
func retainedChainRequest(rc *readCtx, id, operation string, actor mutation.Binding, f reviewFlags) (wire.Value, bool, error) {
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
		string(q.Subject.ReceiptSeq) != f.subject || len(q.Reasons) != len(f.reasons) || len(q.Evidence) != len(f.evidence) {
		return false
	}
	for i, r := range q.Reasons {
		if r.Code+":"+r.Text != f.reasons[i] {
			return false
		}
	}
	for i, e := range q.Evidence {
		if e.Label+"="+string(e.Sha256)+":"+string(e.Bytes) != f.evidence[i] {
			return false
		}
	}
	if candidate := ""; q.Candidate.Kind == "EVIDENCE" {
		if candidate = string(q.Candidate.Sha256) + ":" + string(q.Candidate.Bytes); candidate != f.candidate {
			return false
		}
	} else if f.candidate != "" {
		return false
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
	if f.candidate != "" {
		sha, n, err := artifactArg("--candidate-evidence", f.candidate)
		if err != nil {
			return wire.Value{}, err
		}
		q.Candidate = snapshot.ExternalReviewCandidate{Kind: "EVIDENCE", Sha256: sha, Bytes: n}
	}
	for _, e := range f.evidence {
		label, artifact, ok := strings.Cut(e, "=")
		sha, n, err := artifactArg("--evidence", artifact)
		if !ok || err != nil {
			return wire.Value{}, wire.Errorf(wire.CodeMalformed, "--evidence", "evidence is LABEL=SHA256:BYTES")
		}
		q.Evidence = append(q.Evidence, snapshot.GateEvidence{Label: label, Sha256: sha, Bytes: n})
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

// gateState runs `gate state <ticket>`: the native ERG-V0-009 workState of
// every external review gate the ticket references, one item per gate in
// byte order, each with the closed field set
// {gate,verdict,generation,revision,resubmitted,status,candidate,subject,
// trust,evidenceSha256}. It is read from typed events through the receipt
// binding fold and the same adapter the dispatcher uses, never from prose. A
// ticket without references has no items. A refused fold or an unreadable
// gate set fails the read rather than reporting a guess.
func gateState(env Env, args []string) *wire.Result {
	cmd := []string{"gate", "state"}
	if len(args) != 1 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the only argument is the ticket id or local token")
	}
	var views map[string]transaction.ExternalReviewView
	rc, err := withStore(env, func(rc *readCtx) error {
		id, err := resolveTicketArg(rc, args[0])
		if err != nil {
			return err
		}
		rec, ok := rc.store.Inventory.Get(id)
		if !ok {
			return wire.Errorf(wire.CodeMalformed, "ticket", "ticket %s does not exist in this queue", id)
		}
		if len(rec.ExternalReviews) == 0 {
			return nil
		}
		if rc.journalAbsent {
			return wire.Errorf(wire.CodeMissingEvidence, "gate state", "no journal: review references cannot be audited")
		}
		fold, err := store.FoldExternalReviews(rc.repo, rc.snap.Head.LastSeq.Uint64(), nil)
		if err != nil {
			return err
		}
		views, err = externalReviewViews(rc.repo, rec, rc.store.Policy, fold)
		return err
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{}
	gates := make([]string, 0, len(views))
	for g := range views {
		gates = append(gates, g)
	}
	sort.Strings(gates)
	for _, g := range gates {
		res.Items = append(res.Items, gateStateValue(g, views[g]))
	}
	return res
}

// gateStateValue is one gate's closed ERG-V0-009 workState object.
func gateStateValue(gate string, v transaction.ExternalReviewView) wire.Value {
	candidate, subject, trust, evidence := wire.Null(), wire.Null(), wire.Null(), wire.Null()
	if v.Candidate != nil {
		candidate = v.Candidate.Value()
	}
	if v.Subject != nil {
		subject = v.Subject.Value()
	}
	if v.TrustSource != "" {
		trust = wire.ObjectValue(wire.NewObject().Set("actorAuthentication", wire.String(v.ActorAuthentication)).
			Set("independence", wire.String(v.Independence)).Set("source", wire.String(v.TrustSource)))
	}
	if v.EvidenceSha256 != nil {
		evidence = wire.String(string(*v.EvidenceSha256))
	}
	o := wire.NewObject().Set("gate", wire.String(gate)).Set("verdict", wire.StringOrNull(v.Verdict)).
		Set("generation", wire.String(string(v.Generation))).Set("revision", wire.String(string(v.Revision))).
		Set("resubmitted", wire.Bool(v.Resubmitted)).Set("status", wire.String(v.Status)).
		Set("candidate", candidate).Set("subject", subject).Set("trust", trust).Set("evidenceSha256", evidence)
	return wire.ObjectValue(o)
}
