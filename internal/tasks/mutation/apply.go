package mutation

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Binding is the trusted invocation actor: the role and id of the principal
// that invoked the tool, as established by the caller's authority layer. It
// is deliberately a separate type from Actor so that it can never be filled
// from an envelope by accident. An empty Binding is not "anonymous" and not
// "OWNER"; it fails closed with UNAUTHORIZED.
//
// Limit: this package validates the binding it is handed (well-formed label,
// known role, equal to the envelope claim). It cannot authenticate the process
// that handed it over, and it cannot tell two processes under the same UID
// apart. Deciding who may furnish which binding is the runtime enforcement
// profile's job (qualification work), not this library's.
type Binding struct {
	ID   string
	Role string
}

func (b Binding) validate() *refusal {
	if b.ID == "" && b.Role == "" {
		return refuse(OutcomeUnauthorized, "", "no trusted actor binding: the envelope actor is a claim, not authentication")
	}
	if _, err := wire.ParseLabel("/binding/id", b.ID); err != nil {
		return refuse(OutcomeUnauthorized, "", "trusted binding id is not a label: %v", err)
	}
	for _, r := range intent.Roles {
		if r == b.Role {
			return nil
		}
	}
	return refuse(OutcomeUnauthorized, "", "trusted binding role %q is not a §3.3 role", b.Role)
}

// Context carries every fact a mutation depends on. All of it is explicit
// input; nothing is read from disk, a clock or the environment.
type Context struct {
	// Binding is the trusted actor (see Binding). Required.
	Binding Binding
	// Queue is the current queue manifest (canonical record). Required.
	Queue *intent.Queue
	// Policy is the current policy (roles row removal, gate ids). Required.
	Policy *intent.Policy
	// Inventory holds every current ticket record of the queue, including
	// tombstones, so that ID membership, dependency existence and cycles
	// are checked against the full set (TM-V0-005). Required.
	Inventory *ticket.Inventory
	// Attempts answers attempt liveness (§3.2). A nil oracle or a
	// NOT_OBSERVED answer is treated as "possibly live": an acceptance-
	// relevant mutation is then refused BLOCKED/ATTEMPT_LIVE (fail closed).
	Attempts ticket.AttemptOracle
	// Requests is the request-ID index consulted for TM-V0-006 replay.
	// Required: a nil index is refused VALIDATION_FAILED/MALFORMED before any
	// computation, never treated as "no prior request" (a Context built
	// without its index would otherwise commit every retry twice).
	Requests RequestIndex
	// Now is the logical timestamp recorded as updatedAt, createdAt,
	// placedAt, grantedAt and recordedAt. Advisory only (§2).
	Now wire.Timestamp
}

// Plan is the pure result of validating and computing one mutation. It is
// explicitly uncommitted: Outcome.ReceiptSeq is nil, no file changed, no
// receipt exists, and the request index was only read. A Plan with outcome
// COMPLETED says what the post record would be; the transaction writer of
// TCP-02b decides whether it becomes real.
type Plan struct {
	Outcome        Outcome
	MutationSha256 wire.Digest // SHA-256 of the canonical mutation bytes (TM-V0-006)
	Detail         string      // human explanation of a refusal; never queue prose
	Pre            *ticket.Record
	Post           *ticket.Record // non-nil iff Outcome is a fresh COMPLETED
	QueuePost      *intent.Queue  // non-nil iff CREATE allocated a serial
	Composed       []string       // ADOPT_FILE: the composed operations in order
}

// Planned reports whether the plan is a fresh (not replayed) COMPLETED
// result carrying a post record.
func (p *Plan) Planned() bool {
	return p.Outcome.Outcome == OutcomeCompleted && !p.Outcome.Replayed && p.Post != nil
}

type refusal struct {
	outcome string
	code    string
	detail  string
}

func refuse(outcome, code, format string, args ...interface{}) *refusal {
	return &refusal{outcome: outcome, code: code, detail: fmt.Sprintf(format, args...)}
}

// refuseErr maps a wire validation error to an outcome: an unsupported
// profile version is UNSUPPORTED, everything else VALIDATION_FAILED with the
// error's §11 code.
func refuseErr(err error) *refusal {
	code := wire.CodeOf(err)
	if code == wire.CodeUnsupportedVersion {
		return &refusal{outcome: OutcomeUnsupported, code: code, detail: err.Error()}
	}
	return &refusal{outcome: OutcomeValidationFailed, code: code, detail: err.Error()}
}

func (p *Plan) refused(r *refusal) *Plan {
	p.Outcome.Outcome = r.outcome
	p.Outcome.Replayed = false
	p.Outcome.ResultingRevision = nil
	p.Outcome.ResultingAcceptanceRevision = nil
	p.Outcome.ReceiptSeq = nil
	p.Outcome.Codes = []string{}
	if r.code != "" {
		p.Outcome.Codes = []string{r.code}
	}
	p.Detail = r.detail
	p.Post = nil
	p.QueuePost = nil
	return p
}

func (p *Plan) completed(post *ticket.Record) *Plan {
	rev := post.Revision
	acc := post.AcceptanceRevision
	p.Outcome.Outcome = OutcomeCompleted
	p.Outcome.Replayed = false
	p.Outcome.ResultingRevision = &rev
	p.Outcome.ResultingAcceptanceRevision = &acc
	p.Outcome.ReceiptSeq = nil
	p.Outcome.Codes = []string{}
	p.Post = post
	return p
}

// Apply validates one decoded envelope against the context and computes the
// post record. It never mutates its inputs: the canonical record, the
// inventory, the queue and the envelope are read only, and the result is
// built from fresh copies.
func Apply(ctx Context, env *Envelope) *Plan {
	plan := &Plan{}
	if env == nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "no envelope"))
	}
	plan.Outcome.RequestID = env.RequestID
	plan.MutationSha256 = env.Sha256()
	// Trust boundary first: the envelope actor is a claim; the binding decides.
	if r := ctx.Binding.validate(); r != nil {
		return plan.refused(r)
	}
	if env.Actor.ID != ctx.Binding.ID || env.Actor.Role != ctx.Binding.Role {
		return plan.refused(refuse(OutcomeUnauthorized, "", "envelope actor %s/%s does not match the trusted binding %s/%s",
			env.Actor.Role, env.Actor.ID, ctx.Binding.Role, ctx.Binding.ID))
	}
	if r := ctx.checkInputs(); r != nil {
		return plan.refused(r)
	}
	if env.QueueID.Raw != ctx.Queue.QueueID.Raw {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "envelope queue %s is not %s", env.QueueID.Raw, ctx.Queue.QueueID.Raw))
	}
	// TM-V0-006: identical retry replays, different bytes conflict.
	if out := replay(ctx.Requests, env.RequestID, plan.MutationSha256); out != nil {
		plan.Outcome = *out
		return plan
	}
	if r := ctx.checkRole(env.Operation); r != nil {
		return plan.refused(r)
	}
	if env.Operation == OpCreate {
		p, ok := env.Payload.(*CreatePayload)
		if !ok {
			return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "CREATE payload has the wrong type"))
		}
		return ctx.create(plan, p)
	}
	if env.Payload == nil || env.Payload.operation() != env.Operation {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "payload does not belong to operation %s", env.Operation))
	}
	pre, ok := ctx.Inventory.Get(env.TargetID.Raw)
	if !ok {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "target %s does not exist in the queue", env.TargetID.Raw))
	}
	plan.Pre = pre
	// TM-V0-005: expectedRevision must equal the current revision.
	if *env.ExpectedRevision != pre.Revision {
		return plan.refused(refuse(OutcomeRevisionConflict, "", "expectedRevision %s but the canonical record is at revision %s", *env.ExpectedRevision, pre.Revision))
	}
	work, err := clone(pre)
	if err != nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeOf(err), "canonical record is not valid: %v", err))
	}
	if r := ctx.step(work, env.Payload); r != nil {
		return plan.refused(r)
	}
	if r := ctx.finalize(pre, work, forcesAcceptanceBump(env.Operation)); r != nil {
		return plan.refused(r)
	}
	return plan.completed(work)
}

func (ctx *Context) checkInputs() *refusal {
	if ctx.Queue == nil || ctx.Policy == nil || ctx.Inventory == nil {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "context requires the queue manifest, the policy and the ticket inventory")
	}
	if missingIndex(ctx.Requests) {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "context requires the request index (TM-V0-006); an absent index is not an empty one")
	}
	if ctx.Inventory.QueueID.Raw != ctx.Queue.QueueID.Raw {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "inventory queue %s is not the manifest queue %s", ctx.Inventory.QueueID.Raw, ctx.Queue.QueueID.Raw)
	}
	if _, err := wire.ParseTimestamp("/context/now", string(ctx.Now)); err != nil {
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "context timestamp: %v", err)
	}
	return nil
}

// permittedOps is the §3.2 role matrix after policy row removal: a role
// named in policy.roles is limited to that list; an unnamed role keeps its
// default row. Policy can never add an operation (enforced at policy load).
func (ctx *Context) permittedOps(role string) []string {
	if ops, ok := ctx.Policy.Roles[role]; ok {
		return ops
	}
	return intent.DefaultRoleMatrix[role]
}

func (ctx *Context) checkRole(op string) *refusal {
	for _, o := range ctx.permittedOps(ctx.Binding.Role) {
		if o == op {
			return nil
		}
	}
	return refuse(OutcomeUnauthorized, "", "role %s may not issue %s", ctx.Binding.Role, op)
}

// forcesAcceptanceBump lists the operations that bump acceptanceRevision
// regardless of field changes (§3.1).
func forcesAcceptanceBump(op string) bool {
	return op == OpReopen || op == OpRestore || op == OpCompleteManual
}

// clone deep-copies a record by canonical round trip, which also re-proves
// the record valid. The copy shares nothing with the original.
func clone(rec *ticket.Record) (*ticket.Record, error) {
	return ticket.FromValue(rec.Value())
}

// opens reports whether a record may leave DRAFT (§3.2): non-empty
// acceptanceCriteria for autonomous kinds; MANUAL and EXTERNAL kinds have no
// autonomous acceptance and open on creation.
func opens(rec *ticket.Record) bool {
	return len(rec.AcceptanceCriteria) > 0 || rec.Kind == "MANUAL" || rec.Kind == "EXTERNAL"
}

func isLive(status string) bool {
	return status == ticket.StatusDraft || status == ticket.StatusOpen || status == ticket.StatusHeld
}

// step applies one operation's role restrictions, status rule and field
// computation to the working record in place. It never touches revision,
// acceptanceRevision, previousRecordSha256, updatedAt or updatedBy; finalize
// does, exactly once per plan, so that ADOPT_FILE can chain several steps
// into one revision.
func (ctx *Context) step(work *ticket.Record, p Payload) *refusal {
	op := p.operation()
	if r := ctx.checkRole(op); r != nil {
		return r
	}
	// §3.2 restrictions inside a permitted row.
	switch ctx.Binding.Role {
	case "WORKER":
		rp, ok := p.(*RefinePayload)
		if !ok || len(rp.Present) != 1 || !rp.Has("body") {
			return refuse(OutcomeUnauthorized, "", "WORKER may only REFINE body")
		}
	case "IMPORTER":
		if work.Source.Kind != "IMPORT" {
			return refuse(OutcomeUnauthorized, "", "IMPORTER may only touch records with source.kind IMPORT")
		}
	case "SYSTEM":
		hp, ok := p.(*HoldPayload)
		if !ok || hp.HoldID != "ESCALATED" {
			return refuse(OutcomeUnauthorized, "", "SYSTEM may only HOLD with holdId ESCALATED")
		}
	case "OPERATOR":
		if gp, ok := p.(*GrantApprovalPayload); ok && gp.Operation == "ADJUDICATE" {
			return refuse(OutcomeUnauthorized, "", "OPERATOR may not GRANT_APPROVAL for ADJUDICATE")
		}
	case "REVIEWER":
		return refuse(OutcomeUnauthorized, "", "REVIEWER issues no ticket mutation")
	}
	// §3.2: a tombstone accepts only RESTORE.
	if work.Status == ticket.StatusArchived && op != OpRestore {
		return refuse(OutcomeBlocked, wire.CodeTicketState, "ticket is ARCHIVED; only RESTORE is permitted")
	}
	switch p := p.(type) {
	case *RefinePayload:
		if p.Has("title") {
			work.Title = p.Title
		}
		if p.Has("body") {
			work.Body = copyString(p.Body)
		}
		if p.Has("kind") {
			work.Kind = p.Kind
		}
		if p.Has("owner") {
			work.Owner = copyString(p.Owner)
		}
		if p.Has("milestone") {
			work.Milestone = copyString(p.Milestone)
		}
		if p.Has("labels") {
			work.Labels = copyStrings(p.Labels)
		}
		if p.Has("acceptanceCriteria") {
			work.AcceptanceCriteria = copyStrings(p.AcceptanceCriteria)
		}
		if p.Has("requirementRefs") {
			work.RequirementRefs = copyStrings(p.RequirementRefs)
		}
		if p.Has("dueDate") {
			work.DueDate = copyString(p.DueDate)
		}
		if p.Has("estimateMinutes") {
			work.EstimateMinutes = copyCount(p.EstimateMinutes)
		}
		if p.Has("supersedes") {
			work.Supersedes = copyTicket(p.Supersedes)
		}
		// §3.2 DRAFT→OPEN by refine.
		if work.Status == ticket.StatusDraft && opens(work) {
			work.Status = ticket.StatusOpen
		}
	case *PrioritizePayload:
		work.Priority = p.Priority
		work.Order = p.Order
	case *SetDependenciesPayload:
		if !isLive(work.Status) {
			return refuse(OutcomeBlocked, wire.CodeTicketState, "SET_DEPENDENCIES requires status DRAFT, OPEN or HELD (is %s)", work.Status)
		}
		work.Dependencies = copyDependencies(p.Dependencies)
	case *SetGatesPayload:
		if !isLive(work.Status) {
			return refuse(OutcomeBlocked, wire.CodeTicketState, "SET_GATES requires status DRAFT, OPEN or HELD (is %s)", work.Status)
		}
		work.RequiredGates = copyStrings(p.RequiredGates)
	case *SetEffectsPayload:
		if !isLive(work.Status) {
			return refuse(OutcomeBlocked, wire.CodeTicketState, "SET_EFFECTS requires status DRAFT, OPEN or HELD (is %s)", work.Status)
		}
		work.Effects = copyEffects(p.Effects)
		work.Capabilities = copyStrings(p.Capabilities)
		work.ExecutionClass = p.ExecutionClass
	case *HoldPayload:
		if work.Status != ticket.StatusOpen && work.Status != ticket.StatusHeld {
			return refuse(OutcomeBlocked, wire.CodeTicketState, "HOLD requires status OPEN or HELD (is %s)", work.Status)
		}
		for _, h := range work.Holds {
			if h.HoldID == p.HoldID {
				return refuse(OutcomeValidationFailed, wire.CodeDuplicateID, "hold %q is already placed", p.HoldID)
			}
		}
		if len(work.Holds) >= wire.MaxHolds {
			return refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "more than %d holds", wire.MaxHolds)
		}
		// Actor and time come from the trusted context, never from the payload.
		work.Holds = append(copyHolds(work.Holds), ticket.Hold{HoldID: p.HoldID, Actor: ctx.Binding.ID, Reason: p.Reason, PlacedAt: ctx.Now})
		work.Status = ticket.StatusHeld
	case *ReleaseHoldPayload:
		if work.Status != ticket.StatusHeld {
			return refuse(OutcomeBlocked, wire.CodeTicketState, "RELEASE_HOLD requires status HELD (is %s)", work.Status)
		}
		idx := -1
		for i, h := range work.Holds {
			if h.HoldID == p.HoldID {
				idx = i
			}
		}
		if idx < 0 {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "no hold %q on the ticket", p.HoldID)
		}
		holds := copyHolds(work.Holds)
		work.Holds = append(holds[:idx], holds[idx+1:]...)
		if len(work.Holds) == 0 {
			work.Status = ticket.StatusOpen
		}
	case *ReasonPayload:
		switch p.Op {
		case OpReopen:
			if work.Status != ticket.StatusCompleted {
				return refuse(OutcomeBlocked, wire.CodeTicketState, "REOPEN requires status COMPLETED (is %s)", work.Status)
			}
			// §3.1/§3.2: completion is present iff the effective status is
			// COMPLETED. The prior completion stays in the chained prior
			// record and its receipts (AS-05 history); the new OPEN record
			// carries none, so no reader keyed on completion can report a
			// reopened ticket as completed or verified (AS-06).
			work.Completion = nil
			work.Status = ticket.StatusOpen
		case OpArchive:
			from := work.Status
			work.ArchivedFrom = &from
			work.Status = ticket.StatusArchived
		case OpRestore:
			if work.Status != ticket.StatusArchived || work.ArchivedFrom == nil {
				return refuse(OutcomeBlocked, wire.CodeTicketState, "RESTORE requires status ARCHIVED (is %s)", work.Status)
			}
			work.Status = *work.ArchivedFrom
			work.ArchivedFrom = nil
		default:
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "unknown reason operation %q", p.Op)
		}
	case *CompleteManualPayload:
		if work.Status != ticket.StatusOpen {
			return refuse(OutcomeBlocked, wire.CodeTicketState, "COMPLETE_MANUAL requires status OPEN (is %s)", work.Status)
		}
		reason := p.Reason
		work.Completion = &ticket.Completion{
			Kind:       "MANUAL",
			Actor:      ctx.Binding.ID,
			Reason:     &reason,
			Evidence:   append([]wire.Digest{}, p.Evidence...),
			RecordedAt: ctx.Now,
		}
		work.Status = ticket.StatusCompleted
	case *GrantApprovalPayload:
		if p.Actor != ctx.Binding.ID {
			return refuse(OutcomeUnauthorized, "", "grant actor %q is not the invoking actor %q", p.Actor, ctx.Binding.ID)
		}
		if p.TargetRevision != work.AcceptanceRevision {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "grant targetRevision %s is not the current acceptanceRevision %s", p.TargetRevision, work.AcceptanceRevision)
		}
		for _, a := range work.Approvals {
			if a.GrantID == p.GrantID {
				return refuse(OutcomeValidationFailed, wire.CodeDuplicateID, "grant %q already exists", p.GrantID)
			}
		}
		if len(work.Approvals) >= wire.MaxApprovals {
			return refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "more than %d approvals", wire.MaxApprovals)
		}
		work.Approvals = append(copyApprovals(work.Approvals), ticket.Approval{
			GrantID:        p.GrantID,
			Actor:          p.Actor,
			Operation:      p.Operation,
			TargetRevision: p.TargetRevision,
			Scope:          copyStrings(p.Scope),
			Revoked:        false,
			GrantedAt:      ctx.Now,
		})
	case *RevokeApprovalPayload:
		idx := -1
		for i, a := range work.Approvals {
			if a.GrantID == p.GrantID {
				idx = i
			}
		}
		if idx < 0 {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "no grant %q on the ticket", p.GrantID)
		}
		if work.Approvals[idx].Revoked {
			return refuse(OutcomeValidationFailed, wire.CodeMalformed, "grant %q is already revoked", p.GrantID)
		}
		work.Approvals = copyApprovals(work.Approvals)
		work.Approvals[idx].Revoked = true
	default:
		return refuse(OutcomeValidationFailed, wire.CodeMalformed, "unsupported payload for %s", op)
	}
	return nil
}

// acceptanceChanged reports whether any §3.1 acceptance-relevant field
// differs between two records, by canonical bytes.
func acceptanceChanged(pre, post *ticket.Record) bool {
	a := pre.Value().Obj
	b := post.Value().Obj
	for _, f := range ticket.AcceptanceRelevantFields {
		av, _ := a.Get(f)
		bv, _ := b.Get(f)
		if !wire.Equal(av, bv) {
			return true
		}
	}
	return false
}

func fieldChanged(pre, post *ticket.Record, field string) bool {
	av, _ := pre.Value().Obj.Get(field)
	bv, _ := post.Value().Obj.Get(field)
	return !wire.Equal(av, bv)
}

// checkRecord runs the inventory-level checks of TM-V0-005 on a proposed
// record: dependency existence and cycles (when dependencies changed or the
// record is new), gate existence in policy, and supersession references.
func (ctx *Context) checkRecord(pre, work *ticket.Record) *refusal {
	depsChanged := pre == nil || fieldChanged(pre, work, "dependencies")
	gatesChanged := pre == nil || fieldChanged(pre, work, "requiredGates")
	if depsChanged {
		if err := ctx.Inventory.CheckDependencies(work); err != nil {
			return refuseErr(err)
		}
	}
	if depsChanged || gatesChanged {
		known := ctx.Policy.GateIDs()
		for _, g := range work.RequiredGates {
			if !known[g] {
				return refuse(OutcomeValidationFailed, wire.CodeGateUnknown, "requiredGates names unknown gate %q", g)
			}
		}
		for i, d := range work.Dependencies {
			if d.GateID != nil && !known[*d.GateID] {
				return refuse(OutcomeValidationFailed, wire.CodeGateUnknown, "dependency %d names unknown gate %q", i, *d.GateID)
			}
		}
	}
	if pre == nil || fieldChanged(pre, work, "supersedes") {
		if work.Supersedes != nil {
			if _, ok := ctx.Inventory.Get(work.Supersedes.Raw); !ok {
				return refuse(OutcomeValidationFailed, wire.CodeDependencyMissing, "supersedes names %s, which does not exist in the queue", work.Supersedes.Raw)
			}
		}
	}
	if pre == nil && work.SupersededBy != nil {
		if _, ok := ctx.Inventory.Get(work.SupersededBy.Raw); !ok {
			return refuse(OutcomeValidationFailed, wire.CodeDependencyMissing, "supersededBy names %s, which does not exist in the queue", work.SupersededBy.Raw)
		}
	}
	return nil
}

// finalize performs the checks that depend on the whole change and then
// sets the chain fields exactly once: revision +1, acceptanceRevision +1 iff
// an acceptance-relevant field changed or the operation forces it,
// previousRecordSha256 from the canonical record, updatedAt/updatedBy from
// the context.
func (ctx *Context) finalize(pre, work *ticket.Record, forced bool) *refusal {
	changed := acceptanceChanged(pre, work)
	// A COMPLETED ticket's acceptance is bound to its completion: change it
	// only after REOPEN.
	if pre.Status == ticket.StatusCompleted && changed {
		return refuse(OutcomeBlocked, wire.CodeTicketState, "ticket is COMPLETED; REOPEN before changing acceptance-relevant fields")
	}
	if r := ctx.checkRecord(pre, work); r != nil {
		return r
	}
	bump := forced || changed
	// §3.2: an acceptance-relevant change while an attempt is live is refused;
	// unknown liveness is treated as live.
	if bump {
		obs := ticket.NotObserved
		if ctx.Attempts != nil {
			obs = ctx.Attempts.LiveAttempt(work.TicketID.Raw)
		}
		switch obs {
		case ticket.Unsatisfied:
		case ticket.Satisfied:
			return refuse(OutcomeBlocked, wire.CodeAttemptLive, "an attempt is live; this mutation would change acceptanceRevision")
		default:
			return refuse(OutcomeBlocked, wire.CodeAttemptLive, "attempt liveness NOT_OBSERVED; an acceptance-relevant mutation is refused until it is")
		}
	}
	rev := pre.Revision.Int() + 1
	if rev > wire.MaxCountValue {
		return refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "revision would exceed the Count maximum")
	}
	work.Revision = wire.CountOf(rev)
	work.AcceptanceRevision = pre.AcceptanceRevision
	if bump {
		acc := pre.AcceptanceRevision.Int() + 1
		if acc > wire.MaxCountValue {
			return refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "acceptanceRevision would exceed the Count maximum")
		}
		work.AcceptanceRevision = wire.CountOf(acc)
	}
	prev := pre.FileDigest()
	work.PreviousRecordSha256 = &prev
	work.UpdatedAt = ctx.Now
	work.UpdatedBy = ctx.Binding.ID
	return ctx.revalidate(work)
}

// revalidate re-proves the post record through the closed §3.1 validator
// and the §1 file bound before it is offered as a plan.
func (ctx *Context) revalidate(work *ticket.Record) *refusal {
	if _, err := ticket.FromValue(work.Value()); err != nil {
		return refuseErr(err)
	}
	if n := len(work.Encode()); n > wire.MaxTicketFileBytes {
		return refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "post record is %d bytes, over the %d byte ticket file bound", n, wire.MaxTicketFileBytes)
	}
	return nil
}

// create computes a CREATE. Serial allocation is explicit: the queue's
// nextSerial is read from the context and the advanced manifest is returned
// as QueuePost, never written.
func (ctx *Context) create(plan *Plan, p *CreatePayload) *Plan {
	switch ctx.Binding.Role {
	case "IMPORTER":
		if p.Source.Kind != "IMPORT" {
			return plan.refused(refuse(OutcomeUnauthorized, "", "IMPORTER may only CREATE with source.kind IMPORT"))
		}
	case "WORKER", "SYSTEM", "REVIEWER":
		return plan.refused(refuse(OutcomeUnauthorized, "", "role %s may not CREATE", ctx.Binding.Role))
	}
	if ctx.Inventory.Len() >= wire.MaxTicketsPerQueue {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "queue already holds %d tickets, the §1 bound", wire.MaxTicketsPerQueue))
	}
	q := *ctx.Queue
	local := ""
	if p.LocalToken != nil {
		local = *p.LocalToken
		if err := ctx.Inventory.CheckLocalToken(local); err != nil {
			return plan.refused(refuseErr(err))
		}
	} else {
		var r *refusal
		local, q.NextSerial, r = ctx.allocateSerial(q)
		if r != nil {
			return plan.refused(r)
		}
	}
	id, err := wire.ParseTicketID("/payload/localToken", "ticket:"+q.QueueID.Authority+":"+q.QueueID.Queue+":"+local)
	if err != nil {
		return plan.refused(refuseErr(err))
	}
	rec := &ticket.Record{
		TicketID:           id,
		Revision:           "1",
		AcceptanceRevision: "1",
		Status:             ticket.StatusDraft,
		Title:              p.Title,
		Body:               copyString(p.Body),
		Kind:               p.Kind,
		Owner:              copyString(p.Owner),
		Milestone:          copyString(p.Milestone),
		Priority:           p.Priority,
		Order:              p.Order,
		Labels:             copyStrings(p.Labels),
		Dependencies:       copyDependencies(p.Dependencies),
		AcceptanceCriteria: copyStrings(p.AcceptanceCriteria),
		RequirementRefs:    copyStrings(p.RequirementRefs),
		Source:             copySource(p.Source),
		Effects:            copyEffects(p.Effects),
		Capabilities:       copyStrings(p.Capabilities),
		RequiredGates:      copyStrings(p.RequiredGates),
		Holds:              []ticket.Hold{},
		ExecutionClass:     p.ExecutionClass,
		Approvals:          []ticket.Approval{},
		DueDate:            copyString(p.DueDate),
		EstimateMinutes:    copyCount(p.EstimateMinutes),
		Supersedes:         copyTicket(p.Supersedes),
		SupersededBy:       copyTicket(p.SupersededBy),
		ShadowOverlay:      false,
		CreatedAt:          ctx.Now,
		UpdatedAt:          ctx.Now,
		UpdatedBy:          ctx.Binding.ID,
	}
	if opens(rec) {
		rec.Status = ticket.StatusOpen
	}
	work, err := clone(rec)
	if err != nil {
		return plan.refused(refuseErr(err))
	}
	if r := ctx.checkRecord(nil, work); r != nil {
		return plan.refused(r)
	}
	if r := ctx.revalidate(work); r != nil {
		return plan.refused(r)
	}
	if p.LocalToken == nil {
		plan.QueuePost = &q
	}
	return plan.completed(work)
}

// allocateSerial picks the first serial at or after nextSerial whose token
// `<prefix>-<serial>` is not occupied, exactly or under ASCII case folding,
// by any live or tombstoned ticket, and returns the token with the advanced
// nextSerial (one past the allocated serial). An explicit localToken that
// happened to take the next serial therefore never wedges later automatic
// CREATEs. The scan is bounded: at most Inventory.Len() tokens can be
// occupied, so at most Len()+1 candidates are tried, and a serial whose
// successor would leave the Count range is refused LIMIT_EXCEEDED before any
// overflow, leaving the manifest unchanged.
func (ctx *Context) allocateSerial(q intent.Queue) (string, wire.Count, *refusal) {
	occupied := make(map[string]bool, ctx.Inventory.Len())
	for _, id := range ctx.Inventory.IDs() {
		if rec, ok := ctx.Inventory.Get(id); ok {
			occupied[wire.FoldToken(rec.TicketID.Local)] = true
		}
	}
	n := q.NextSerial.Int()
	for tries := 0; tries <= ctx.Inventory.Len(); tries++ {
		if n < 0 || n+1 > wire.MaxCountValue {
			return "", "", refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "nextSerial %d would exceed the Count maximum; no serial allocated", n)
		}
		cand := fmt.Sprintf("%s-%04d", q.Prefix, n)
		if !occupied[wire.FoldToken(cand)] {
			if _, err := wire.ParseToken("/payload/localToken", cand, wire.MaxLocalTokenBytes); err != nil {
				return "", "", refuseErr(err)
			}
			return cand, wire.CountOf(n + 1), nil
		}
		n++
	}
	return "", "", refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "every serial from %s is occupied within the inventory bound", q.NextSerial)
}

func copyString(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

func copyCount(c *wire.Count) *wire.Count {
	if c == nil {
		return nil
	}
	v := *c
	return &v
}

func copyTicket(t *wire.TicketID) *wire.TicketID {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

func copyStrings(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	return out
}

func copyDependencies(ds []ticket.Dependency) []ticket.Dependency {
	out := make([]ticket.Dependency, len(ds))
	for i, d := range ds {
		out[i] = ticket.Dependency{TicketID: d.TicketID, Obligation: d.Obligation, GateID: copyString(d.GateID)}
	}
	return out
}

func copySource(s ticket.Source) ticket.Source {
	out := ticket.Source{Kind: s.Kind, SourceQueueID: s.SourceQueueID, SourceItemID: copyString(s.SourceItemID)}
	if s.SourceRevisionSha256 != nil {
		d := *s.SourceRevisionSha256
		out.SourceRevisionSha256 = &d
	}
	return out
}

func copyEffects(e ticket.Effects) ticket.Effects {
	out := ticket.Effects{Coverage: e.Coverage, TouchPaths: copyStrings(e.TouchPaths), ExternalUnbounded: e.ExternalUnbounded}
	out.Resources = make([]ticket.Resource, len(e.Resources))
	copy(out.Resources, e.Resources)
	return out
}

func copyHolds(hs []ticket.Hold) []ticket.Hold {
	out := make([]ticket.Hold, len(hs))
	copy(out, hs)
	return out
}

func copyApprovals(as []ticket.Approval) []ticket.Approval {
	out := make([]ticket.Approval, len(as))
	for i, a := range as {
		out[i] = a
		out[i].Scope = copyStrings(a.Scope)
	}
	return out
}
