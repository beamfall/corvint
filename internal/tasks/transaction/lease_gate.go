package transaction

import (
	"bytes"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// S5 lease verbs: record a candidate tree, run one command gate at it, and
// complete the ticket from a commit of it (CAL-V0-015..017, CAL-V0-024).
const (
	LeaseSubmit   = "SUBMIT"
	LeaseGateRun  = "GATE_RUN"
	LeaseComplete = "COMPLETE"
)

var gateVerbs = map[string]bool{LeaseSubmit: true, LeaseGateRun: true, LeaseComplete: true}

// gateFacts are the caller's observations for the S5 verbs. ChangedPaths is
// the rename-free diff from the attempt's base tree to the submitted tree.
// GateRecord and GateOutput are one gate run, made before the store lock
// was taken. CommitTree and CommitReachable describe the completing commit;
// CommitUpstream names the remote-tracking upstream of the intent branch that
// was also checked ("" when none), and UnintegratedRepository the extra
// repository whose candidate is not integrated when the intent side is.
// GateResults holds the bytes of every gate result the attempt names,
// keyed by digest; each must match its evidence/ file. ReuseSource is the
// earlier PASSED result a reused GateRecord names, which must match its
// evidence/ file; a reuse carries no GateOutput (CAL-V0-240).
// ReuseDescends is the store's verdict on a reuse whose source is at another
// tree: true only when a commit carrying the attempt's candidate tree is an
// ancestor of a commit carrying the source's tree on the protected intent
// branch, else ReuseDescent is the refusal detail. DescendantResults names
// the attempt's reused results at another tree that complete may count,
// rechecked the same way (TCD-V0-151, TCD-V0-152). A ReuseSource that is a
// retained CI run record carries CIHeadTree, the tree the store resolved for
// the run's head commit, and ReuseDescends is then the commit-level verdict
// (TCD-V0-155).
type gateFacts struct {
	ChangedPaths           []string
	GateRecord, GateOutput []byte
	ReuseSource            []byte
	ReuseDescends          bool
	ReuseDescent           string
	CIHeadTree             string
	DescendantResults      map[wire.Digest]bool
	CommitTree             string
	CommitReachable        bool
	CommitUpstream         string
	UnintegratedRepository string
	GateResults            map[wire.Digest][]byte
}

// GateFacts builds the S5 facts; the field set is private so only the store
// observers construct it.
func GateFacts(changed []string, record, output []byte, commitTree string, reachable bool, results map[wire.Digest][]byte) LeaseFacts {
	return LeaseFacts{gateFacts: gateFacts{ChangedPaths: changed, GateRecord: record, GateOutput: output, CommitTree: commitTree, CommitReachable: reachable, GateResults: results}}
}

// GateReuseFacts builds the facts of a gate run satisfied by reuse: the
// rebound record and the source result it names, with no output
// (CAL-V0-240).
func GateReuseFacts(record, source []byte, results map[wire.Digest][]byte) LeaseFacts {
	f := GateFacts(nil, record, nil, "", false, results)
	f.ReuseSource = source
	return f
}

// GateDescendantReuseFacts is GateReuseFacts for a source at another tree,
// with the store's ancestry verdict on it: descent is "" when it descends,
// else the refusal detail (TCD-V0-151, TCD-V0-152).
func GateDescendantReuseFacts(record, source []byte, descent string, results map[wire.Digest][]byte) LeaseFacts {
	f := GateReuseFacts(record, source, results)
	f.ReuseDescends, f.ReuseDescent = descent == "", descent
	return f
}

// WithDescendantResults adds the attempt's reused results at another tree
// whose ancestry the store verified to complete's facts (TCD-V0-151).
func (f LeaseFacts) WithDescendantResults(ok map[wire.Digest]bool) LeaseFacts {
	f.DescendantResults = ok
	return f
}

// ReusedGateResult is the record a reuse of src (whose digest is from)
// writes for attempt a: src rebound to a's attempt, generation, ticket
// revision, policy and config, naming from as its source; every observed
// field, the definition, inputs and environment digests and the evidence
// are src's own (CAL-V0-240). The store builds it, and the model and the
// receipt audit require the posted record to be exactly these bytes.
func ReusedGateResult(src *snapshot.GateResult, from wire.Digest, a *snapshot.Attempt) snapshot.GateResult {
	g := *src
	g.AttemptID, g.Generation, g.TicketRevision, g.PolicySha256, g.ConfigSha256 = a.AttemptID, a.Generation, a.TicketRevision, a.PolicySha256, a.ConfigSha256
	g.Evidence = append([]snapshot.GateEvidence{}, src.Evidence...)
	g.ReusedFrom = &from
	return g
}

// CompleteFacts builds the complete verb's facts: the commit's (composite)
// tree, whether it is integrated, the upstream ref also checked and the
// extra repository that is not integrated (CAL-V0-017, CAL-V0-087).
func CompleteFacts(commitTree string, reachable bool, upstream, unintegrated string, results map[wire.Digest][]byte) LeaseFacts {
	f := GateFacts(nil, nil, nil, commitTree, reachable, results)
	f.CommitUpstream, f.UnintegratedRepository = upstream, unintegrated
	return f
}

// commitNotIntegrated reports that neither the intent branch nor its
// remote-tracking upstream contains the commit (A27). completeFacts checks
// extra repositories only once the intent side is reachable, so an empty
// UnintegratedRepository on an unreachable commit means the intent side.
func (f gateFacts) commitNotIntegrated() bool {
	return !f.CommitReachable && f.UnintegratedRepository == ""
}

func checkGateFields(l *LeaseRequest) error {
	for where, v := range map[string]string{"tree": l.Tree, "commit": l.Commit} {
		if v == "" {
			continue
		}
		if _, e := wire.ParseOID(where, v); e != nil {
			return e
		}
	}
	return checkLabels(map[string]string{"gate": l.Gate})
}

// checkable refuses a verb outside BUILT or CHECKING; SUBMIT also takes
// RUNNING.
func (c leaseContext) checkable(a *snapshot.Attempt, running bool) *leaseOutcome {
	if running && a.Phase == "RUNNING" {
		return nil
	}
	if a.Phase != "BUILT" && a.Phase != "CHECKING" && !(a.Supervision != nil && a.Phase == "READY_FOR_INTEGRATION") {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeTicketState, "the attempt is "+a.Phase)
		return &out
	}
	if a.CandidateTreeOid == nil {
		out := c.fail(malformed("a " + a.Phase + " attempt has no candidate tree"))
		return &out
	}
	return nil
}

// outOfScope lists every changed path the attempt's scope does not cover.
func outOfScope(a *snapshot.Attempt, changed []string) []string {
	out := []string{}
	for _, p := range changed {
		if !covered(a.Scope.Resources, p) {
			out = append(out, p)
		}
	}
	return out
}

func covered(resources []ticket.Resource, path string) bool {
	for _, r := range resources {
		if r.Class == "WHOLE_REPOSITORY" || (r.Class == "PATH" && ticket.PathCovers(r.Key, path)) {
			return true
		}
	}
	return false
}

// exactKeyCause names each PATH key without a trailing "/" that one of the
// offending paths lies beneath: such a key names one path, and only a
// directory key ending in "/" covers the paths under it (CAL-V0-021,
// CAL-V0-024, V1-1090).
func exactKeyCause(resources []ticket.Resource, bad []string) string {
	keys := []string{}
	for _, r := range resources {
		if r.Class != "PATH" || strings.HasSuffix(r.Key, "/") {
			continue
		}
		for _, p := range bad {
			if strings.HasPrefix(p, r.Key+"/") {
				keys = append(keys, r.Key)
				break
			}
		}
	}
	if len(keys) == 0 {
		return ""
	}
	return "; a PATH key without a trailing \"/\" names one path, not the paths beneath it: " + strings.Join(keys, " ") + " (a directory key ends in \"/\")"
}

// planSubmit records the candidate tree when every changed path is within
// scope (CAL-V0-015, CAL-V0-024). A resubmitted tree leaves the earlier gate
// results in place: they stay bound to their own tree, so COMPLETE reads
// each as STALE.
func planSubmit(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	if out := c.checkable(a, true); out != nil {
		return *out
	}
	if bad := outOfScope(a, c.in.LeaseFacts.ChangedPaths); len(bad) > 0 {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeOutOfScope, "outside the attempt's scope: "+strings.Join(bad, " ")+exactKeyCause(a.Scope.Resources, bad))
	}
	if a.CandidateTreeOid != nil && *a.CandidateTreeOid == c.l.Tree {
		return c.unchanged()
	}
	next := *a
	tree := c.l.Tree
	next.CandidateTreeOid, next.Phase, next.PhaseSinceSeq, next.ScopeCheck = &tree, "BUILT", c.seq, "WITHIN"
	return c.write(&next, nil, "TRANSITION", false)
}

// gateResults decodes every gate result the attempt names, each checked
// against its evidence/ file.
func (c leaseContext) gateResults(a *snapshot.Attempt) (map[string]*snapshot.GateResult, error) {
	out := map[string]*snapshot.GateResult{}
	for _, d := range a.GateResults {
		raw, ok := c.in.LeaseFacts.GateResults[wire.Digest(d)]
		if !ok || !c.in.Inventory.matches("evidence/"+d, raw) {
			return nil, malformed("gate result " + d + " differs from its evidence file")
		}
		g, e := snapshot.DecodeGateResult(raw)
		if e != nil {
			return nil, e
		}
		out[d] = g
	}
	return out, nil
}

// bound says the gate result was produced for this attempt, generation,
// ticket revision and policy.
func bound(g *snapshot.GateResult, a *snapshot.Attempt) bool {
	return g.AttemptID == a.AttemptID && g.Generation == a.Generation && g.TicketRevision == a.TicketRevision && g.PolicySha256 == a.PolicySha256 && g.ConfigSha256 == a.ConfigSha256
}

// planGateRun records one gate run the caller made at the candidate tree
// (CAL-V0-016): its output and result go to evidence/, and the result
// replaces any earlier one for the same gate in the attempt's set.
func planGateRun(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	if out := c.checkable(a, false); out != nil {
		return *out
	}
	f := c.in.LeaseFacts
	if f.GateRecord == nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeTicketState, "the attempt changed while the gate was not run; run it again")
	}
	g, e := snapshot.DecodeGateResult(f.GateRecord)
	if e != nil {
		return c.fail(e)
	}
	ran := g.ReusedFrom == nil && len(g.Evidence) == 1 && g.Evidence[0].Sha256 == wire.Sum(f.GateOutput) && g.Evidence[0].Bytes.Uint64() == uint64(len(f.GateOutput))
	if g.GateID != c.l.Gate || !bound(g, a) || (g.ReusedFrom == nil && !ran) {
		return c.fail(malformed("gate record does not describe this run"))
	}
	var ci *snapshot.CIRun
	if g.ReusedFrom != nil {
		if ci, _ = snapshot.DecodeCIRun(f.ReuseSource); ci != nil && f.GateOutput == nil && wire.Sum(f.ReuseSource) == *g.ReusedFrom {
			if out := c.ciRefusal(a, g, ci); out != nil {
				return *out
			}
		} else if out := c.reuseRefusal(a, g); out != nil {
			return *out
		}
	}
	if g.CandidateTreeOid != *a.CandidateTreeOid {
		switch {
		case g.ReusedFrom == nil:
			return c.refuse(mutation.OutcomeBlocked, wire.CodeStaleTree, "the gate ran at "+g.CandidateTreeOid+", not the candidate "+*a.CandidateTreeOid)
		case !f.ReuseDescends:
			// TCD-V0-152: a source at another tree covers the candidate only
			// on the protected branch and only as its descendant.
			detail := f.ReuseDescent
			if detail == "" {
				detail = "the reused result ran at " + g.CandidateTreeOid + ", not the candidate " + *a.CandidateTreeOid + ", and its ancestry was not observed"
			}
			return c.refuse(mutation.OutcomeBlocked, wire.CodeStaleTree, detail)
		}
	}
	if g.DefinitionSha256 != c.st.policy.GateDefinitionSha256(g.GateID) {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeGateStale, "gate "+g.GateID+" is not the current policy definition")
	}
	prior, e := c.gateResults(a)
	if e != nil {
		return c.fail(e)
	}
	digest := string(wire.Sum(f.GateRecord))
	set := []string{digest}
	for _, d := range a.GateResults {
		if prior[d].GateID != g.GateID {
			set = append(set, d)
		}
	}
	sort.Strings(set)
	next := *a
	next.Phase, next.PhaseSinceSeq, next.GateResults = "CHECKING", c.seq, set
	if a.RetryAccounting != nil && g.State != "PASSED" {
		accounting := *a.RetryAccounting
		accounting.FailedOrUnknown = true
		next.RetryAccounting = &accounting
	}
	out := c.write(&next, nil, "GATE_RESULT", false)
	if out.result != nil {
		return out
	}
	if g.ReusedFrom == nil {
		c.postEvidence(out.posts, f.GateOutput)
	}
	if ci != nil {
		c.postEvidence(out.posts, f.ReuseSource)
	}
	c.postEvidence(out.posts, f.GateRecord)
	return out
}

// reuseRefusal checks a reused gate result under the lock (CAL-V0-240..242):
// the gate is declared reusable, and the source is an unreused PASSED
// result of the same gate, still named by its own attempt, which the record
// rebinds byte for byte. A reuse carries no output of its own. The record
// so carries the source's tree and definition digest, which planGateRun
// then checks as it does an executed result's (STALE_TREE, GATE_STALE).
func (c leaseContext) reuseRefusal(a *snapshot.Attempt, g *snapshot.GateResult) *leaseOutcome {
	f := c.in.LeaseFacts
	from := string(*g.ReusedFrom)
	if f.GateOutput != nil || f.ReuseSource == nil || wire.Sum(f.ReuseSource) != *g.ReusedFrom || !c.in.Inventory.matches("evidence/"+from, f.ReuseSource) {
		out := c.fail(malformed("reused gate result source " + from + " differs from its evidence file"))
		return &out
	}
	src, e := snapshot.DecodeGateResult(f.ReuseSource)
	if e != nil {
		out := c.fail(e)
		return &out
	}
	refuse := func(code, detail string) *leaseOutcome {
		out := c.refuse(mutation.OutcomeBlocked, code, detail)
		return &out
	}
	def := c.st.policy.Gate(g.GateID)
	switch {
	case def == nil || !def.Reusable:
		return refuse(wire.CodeUnsupported, "gate "+g.GateID+" is not declared reusable; run it")
	case src.GateID != g.GateID || src.ReusedFrom != nil:
		out := c.fail(malformed("reused gate result " + from + " is not an executed result of gate " + g.GateID))
		return &out
	case src.State != "PASSED":
		return refuse(wire.CodeGateFailed, "the reused result "+from+" is "+src.State)
	}
	owner := c.st.attempts[src.AttemptID]
	if owner == nil || !slices.Contains(owner.GateResults, from) {
		return refuse(wire.CodeGateStale, "the reused result "+from+" is no longer named by attempt "+src.AttemptID+"; run the gate again")
	}
	want := ReusedGateResult(src, *g.ReusedFrom, a)
	raw, e := want.Encode()
	if e != nil || !bytes.Equal(raw, f.GateRecord) {
		out := c.fail(malformed("reused gate result is not its source rebound to this attempt"))
		return &out
	}
	return nil
}

// postEvidence posts raw under its digest unless the store already holds it.
func (c leaseContext) postEvidence(posts map[string][]byte, raw []byte) {
	path := "evidence/" + string(wire.Sum(raw))
	if _, ok := c.in.Inventory.files[path]; !ok {
		posts[path] = append([]byte{}, raw...) // never nil, which would post a deletion
	}
}

// requiredGates is the policy's required gates and the ticket's own, sorted
// and duplicate-free.
func (c leaseContext) requiredGates(rec *ticket.Record) []string {
	return RequiredGates(c.st.policy, rec)
}

// RequiredGates is the effective gate set complete enforces for rec: the
// policy gates marked required and the ticket's own requiredGates, sorted and
// duplicate-free. Reads report it so a closeout sees every gate complete will
// demand (V1-2038).
func RequiredGates(p *intent.Policy, rec *ticket.Record) []string {
	set := map[string]bool{}
	for _, g := range p.Gates {
		if g.Required {
			set[g.GateID] = true
		}
	}
	for _, g := range rec.RequiredGates {
		set[g] = true
	}
	out := make([]string, 0, len(set))
	for g := range set {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// gateVerdict names the required gate's current result digest, or refuses
// MISSING_GATE, GATE_STALE or GATE_FAILED.
func (c leaseContext) gateVerdict(a *snapshot.Attempt, results map[string]*snapshot.GateResult, gateID string) (string, string, string) {
	for d, g := range results {
		if g.GateID != gateID {
			continue
		}
		elsewhere := g.CandidateTreeOid != *a.CandidateTreeOid && (g.ReusedFrom == nil || !c.in.LeaseFacts.DescendantResults[wire.Digest(d)])
		switch {
		case elsewhere || g.DefinitionSha256 != c.st.policy.GateDefinitionSha256(gateID) || !bound(g, a):
			return "", wire.CodeGateStale, "gate " + gateID + " last ran at another tree, definition or binding"
		case g.State != "PASSED":
			return "", wire.CodeGateFailed, "gate " + gateID + " is " + g.State
		}
		return d, "", ""
	}
	return "", wire.CodeMissingGate, "gate " + gateID + " has no result"
}

// completionBlocker checks the ticket and attempt against the §7.3 reducer
// rows the external-agent path applies (amendment A13).
func (c leaseContext) completionBlocker(a *snapshot.Attempt, rec *ticket.Record) (string, string) {
	f := c.in.LeaseFacts
	switch {
	case !f.CommitReachable:
		return wire.CodeStaleTree, c.notIntegrated()
	case f.CommitTree != *a.CandidateTreeOid:
		return wire.CodeStaleTree, "commit tree " + f.CommitTree + " is not the candidate " + *a.CandidateTreeOid + "; " + postMergeSequence
	case rec.Status == ticket.StatusHeld:
		return wire.CodeTicketHeld, "the ticket is HELD"
	case rec.Status != ticket.StatusOpen:
		return wire.CodeTicketState, "the ticket is " + rec.Status
	case a.TicketRevision != rec.AcceptanceRevision:
		return wire.CodeStaleTicket, "the attempt read acceptanceRevision " + string(a.TicketRevision) + ", the ticket is at " + string(rec.AcceptanceRevision)
	case a.PolicySha256 != wire.Sum(c.st.policy.Raw):
		return wire.CodeStalePolicy, "the policy changed since the claim"
	case a.ScopeCheck != "WITHIN":
		return wire.CodeOutOfScope, "the scope check is " + a.ScopeCheck
	}
	return approvalBlocker(rec)
}

// notIntegrated names every ref the unreachable commit was checked against
// and the recovery that moves no checked-out branch (CAL-V0-017, V1-1081),
// and, for one repository, the post-merge sequence (TCB-V0-047).
func (c leaseContext) notIntegrated() string {
	f := c.in.LeaseFacts
	if f.UnintegratedRepository != "" {
		return "repository " + f.UnintegratedRepository + " candidate is not integrated in its designated integration branch"
	}
	local := "refs/heads/" + c.st.queue.IntentBranch
	if f.CommitUpstream == "" {
		return "commit " + c.l.Commit + " is not reachable from " + local + ", which has no remote-tracking upstream configured; integrate it into " + local + ", or set that branch's upstream (git branch --set-upstream-to) and fetch it so the remote-tracking ref contains the commit; " + postMergeSequence
	}
	return "commit " + c.l.Commit + " is not reachable from " + local + " or its upstream " + f.CommitUpstream + "; fetch the upstream (git fetch) so " + f.CommitUpstream + " contains the commit, then complete again; " + postMergeSequence
}

// postMergeSequence is the one route both STALE_TREE refusals name for a
// change integrated by a new merge commit, as a merge queue makes: complete
// keeps tree equality (CAL-V0-017), so the merge commit's own tree is
// submitted and gated before it completes (TCB-V0-047, V1-1895). submit and
// complete take full OIDs, so the route resolves each with git rev-parse.
const postMergeSequence = "if the change landed as a new merge commit (a merge queue), submit that commit's full tree OID (submit --tree $(git rev-parse <merge>^{tree})), run each required gate on it (gate run), then complete --commit $(git rev-parse <merge>)"

// approvalBlocker requires an unrevoked COMPLETE grant at the current
// acceptanceRevision for an APPROVAL_REQUIRED ticket (§7.3 row 12).
func approvalBlocker(rec *ticket.Record) (string, string) {
	if rec.ExecutionClass != "APPROVAL_REQUIRED" {
		return "", ""
	}
	revoked := false
	for _, g := range rec.Approvals {
		if g.Operation == "COMPLETE" && g.TargetRevision == rec.AcceptanceRevision {
			if !g.Revoked {
				return "", ""
			}
			revoked = true
		}
	}
	if revoked {
		return wire.CodeApprovalRevoked, "the COMPLETE grant at acceptanceRevision " + string(rec.AcceptanceRevision) + " is revoked"
	}
	return wire.CodeApprovalMissing, "no COMPLETE grant at acceptanceRevision " + string(rec.AcceptanceRevision)
}

// planComplete completes the ticket VERIFIED from a commit of the candidate
// tree whose required gates all passed there, ends the attempt COMPLETED and
// FENCED, and frees its reservation, in one MANIFEST transaction
// (CAL-V0-017).
func planComplete(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	if out := c.checkable(a, false); out != nil {
		return *out
	}
	if a.Supervision != nil {
		s := a.Supervision
		if a.Phase != "READY_FOR_INTEGRATION" || a.Stage != "integrate" || s.Worker || a.Quiescence != "PROVED" || len(a.PendingEffects) != 0 || a.CandidateTreeOid == nil || s.ReviewTree != *a.CandidateTreeOid || s.ReviewDigest == "" || s.ReviewerHolder == s.AuthorHolder || s.ReviewerSession == s.AuthorSession || s.IntegrationGrant == "" || s.IntegrationCommit != c.l.Commit {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeMissingEvidence, "supervised completion obligations missing")
		}
	}
	rec, ok := c.st.tickets.Get(a.TicketID.Raw)
	if !ok {
		return c.fail(malformed("attempt names an absent ticket"))
	}
	if code, detail := c.completionBlocker(a, rec); code != "" {
		out := c.refuse(mutation.OutcomeBlocked, code, detail)
		if c.in.LeaseFacts.commitNotIntegrated() {
			// V1-1081: the second code separates an intent commit that is
			// not integrated from a tree mismatch or an extra repository.
			out.result.Outcome.Codes = append(out.result.Outcome.Codes, wire.CodeCommitNotIntegrated)
		}
		return out
	}
	results, e := c.gateResults(a)
	if e != nil {
		return c.fail(e)
	}
	passed := []string{}
	for _, gateID := range c.requiredGates(rec) {
		d, code, detail := c.gateVerdict(a, results, gateID)
		if code != "" {
			return c.refuse(mutation.OutcomeBlocked, code, detail)
		}
		passed = append(passed, d)
	}
	sort.Strings(passed)
	return c.completed(a, rec, passed)
}

func (c leaseContext) completed(a *snapshot.Attempt, rec *ticket.Record, passed []string) leaseOutcome {
	m := snapshot.Manifest{Supervision: snapshot.CloneSupervision(a.Supervision), AttemptID: a.AttemptID, Generation: a.Generation, TicketID: a.TicketID, TicketRevision: a.TicketRevision, TicketRecordSha256: rec.FileDigest(), PolicySha256: a.PolicySha256, ConfigSha256: a.ConfigSha256, BaseCommit: a.BaseCommit, CandidateTreeOid: *a.CandidateTreeOid, GateResults: passed, Budget: a.Budget, ScopeCheck: a.ScopeCheck, Mode: a.Mode}
	manifest, e := m.Encode()
	if e != nil {
		return c.fail(e)
	}
	digest := wire.Sum(manifest)
	evidence := make([]wire.Digest, 0, len(passed))
	for _, d := range passed {
		evidence = append(evidence, wire.Digest(d))
	}
	ctx := mutation.Context{Binding: c.r.Actor, Queue: c.st.queue, Policy: c.st.policy, Inventory: c.st.tickets, Attempts: c.oracle(), Requests: absentIndex{}, Now: c.in.RecordedAt}
	post, e := mutation.CompleteVerified(ctx, rec, evidence, digest)
	if e != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeOf(e), e.Error())
	}
	path := "intent/tickets/" + rec.TicketID.Local + ".json"
	if !c.in.Inventory.matches(path, wire.EncodeFile(rec.Value())) {
		return c.fail(malformed("physical projection differs from canonical record"))
	}
	next := *a
	next.Phase, next.PhaseSinceSeq, next.Quiescence, next.ManifestSha256 = "COMPLETED", c.seq, "FENCED", &digest
	if a.Supervision != nil {
		next.Quiescence = "PROVED"
	}
	out := c.write(&next, c.without(a.AttemptID), "MANIFEST", false)
	if out.result != nil {
		return out
	}
	out.posts[path] = wire.EncodeFile(post.Value())
	c.postEvidence(out.posts, manifest)
	out.ticket = &ticketEffect{pre: rec, post: post, kind: "MANIFEST"}
	return out
}
