package transaction

import (
	"bytes"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
)

type SupervisorChange struct {
	Pool                                                                                                                  string
	AnswerRevision                                                                                                        wire.Count  `json:"answerRevision"`
	Expected                                                                                                              wire.Digest `json:"expected"`
	Action, ProgramID, OwnerStarted, Stage, Holder, Worktree, Session, Tree, Commit, Grant, Question, Answer, ResumePhase string
	OwnerPID                                                                                                              int `json:"ownerPid,string"`
	LeaderPID                                                                                                             int `json:"leaderPid,string"`
	LeaderStarted                                                                                                         string
	Clean                                                                                                                 bool
	Accepted                                                                                                              bool
	Claims                                                                                                                []string
	Handoff                                                                                                               []byte
	ChangedPaths                                                                                                          []string
}

func planSupervisor(c leaseContext) leaseOutcome {
	raw := c.in.LeaseFacts.Program
	if len(raw) > 65536 || string(wire.Sum(raw)) != c.l.Evidence {
		return c.fail(malformed("supervisor request binding"))
	}
	var f SupervisorChange
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&f); e != nil {
		return c.fail(e)
	}
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if a.Generation != c.l.Generation {
		return c.recordFenced(a, "supervised generation differs")
	}
	oldRaw, e := a.Encode()
	if e != nil {
		return c.fail(e)
	}
	if wire.Sum(oldRaw) != f.Expected {
		return c.recordFenced(a, "supervised preimage moved")
	}
	if !a.Live() {
		return c.recordFenced(a, "attempt terminal")
	}
	programs, e := snapshot.DecodePrograms(c.in.Programs)
	if e != nil {
		return c.fail(e)
	}
	owner := false
	recovered := false
	var repositories []snapshot.RepositoryRecord
	for _, p := range programs.Entries {
		if p.ID == f.ProgramID && p.OwnerPID == f.OwnerPID && p.OwnerStarted == f.OwnerStarted && p.CurrentAttempt == a.AttemptID && p.CurrentGeneration == string(a.Generation) {
			owner = true
			recovered = p.Phase == "FINISHED" && p.Quiescence == "PROVED"
			repositories = p.Repositories
		}
	}
	if !owner {
		return c.recordFenced(a, "supervisor owner identity differs")
	}
	next := *a
	next.Supervision = snapshot.CloneSupervision(a.Supervision)
	next.PhaseSinceSeq = c.seq
	entries := c.entries()
	worker := "0"
	if a.Supervision != nil && a.Supervision.Worker {
		worker = "1"
	}
	if f.Action == "ATTACH" {
		// PSR-V0-019: a supervised stage stop quarantines the whole allocation.
		if c.sharesAllocation(a) {
			return c.fail(wire.Errorf(wire.CodeUnsupported, "pool", "an attempt on a shared allocation cannot attach to a supervisor"))
		}
		if a.Supervision != nil || a.RuntimeID != snapshot.RuntimeExternalAgent || a.Phase != "RUNNING" {
			return c.fail(malformed("supervised attach phase"))
		}
		next.RuntimeID = snapshot.SupervisedProfile
		next.Phase = "ADMITTED"
		next.Quiescence = "PROVED"
		next.Supervision = &snapshot.Supervision{ProgramID: f.ProgramID, OwnerStarted: f.OwnerStarted, AuthorHolder: a.Lease.Holder, Turns: "0", QuestionRevision: "0"}
		next.Supervisor = &snapshot.Supervisor{Pid: wire.SizeOf(uint64(f.OwnerPID)), StartTime: "0"}
	} else {
		if a.Supervision == nil || a.Supervision.ProgramID != f.ProgramID || (a.Supervision.OwnerStarted != f.OwnerStarted && f.Action != "OWNERSHIP") {
			return c.recordFenced(a, "attempt supervisor differs")
		}
		if f.Action == "DISPATCH" || f.Action == "ANSWER" || f.Action == "GRANT" {
			rec, _ := c.st.tickets.Get(a.TicketID.Raw)
			if code := supervisedIntentCode(a, rec, c.st.policy.Raw); code != "" {
				return c.refuse(mutation.OutcomeBlocked, code, "governing ticket or policy changed")
			}
		}
		switch f.Action {
		case "OWNERSHIP":
			if a.Supervision.Worker || a.Quiescence != "PROVED" {
				if !recovered {
					return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "old worker not stopped")
				}
				next.Supervision.Worker = false
				worker = "0"
				next.Quiescence = "PROVED"
				next.Lane = nil
				next.PendingEffects = []string{}
				next.Phase = "WAITING"
				next.Supervision.Question = "recovered owner interruption; inspect preserved worktree"
				next.Supervision.Answer = ""
			}
			next.Supervision.OwnerStarted = f.OwnerStarted
			next.Supervisor = &snapshot.Supervisor{Pid: wire.SizeOf(uint64(f.OwnerPID)), StartTime: "0"}
		case "HEARTBEAT":
			if !a.Supervision.Worker || a.Lane == nil {
				return c.fail(malformed("heartbeat without live lane"))
			}
			lease := *a.Lease
			lease.ExpiresAt = addMinutes(c.in.RecordedAt, "60")
			next.Lease = &lease
		case "DISPATCH":
			for name, cap := range map[string]uint64{"inputTokens": c.st.policy.Lane.InputTokens.Uint64(), "outputTokens": c.st.policy.Lane.OutputTokens.Uint64()} {
				if cap == 0 {
					continue
				}
				usage := a.Budget[name]
				if a.Supervision.Turns.Int() > 0 && usage.State != "OBSERVED" {
					return c.refuse(mutation.OutcomeBlocked, wire.CodeMissingEvidence, "lane token usage unknown")
				}
				if usage.Value != nil && usage.Value.Uint64() >= cap {
					return c.refuse(mutation.OutcomeBlocked, wire.CodeLimitExceeded, "observed lane token cutoff")
				}
			}
			required := []string{"BUILDER"}
			if f.Stage == "review" {
				required = []string{"REVIEWER"}
			}
			if f.Stage == "integrate" {
				required = []string{"VERIFIER"}
			}
			if a.Phase == "RETURNED" {
				required = []string{"REPAIR"}
			}
			rec, _ := c.st.tickets.Get(a.TicketID.Raw)
			if roles := rec.RequiredRoles[f.Stage]; len(roles) > 0 {
				required = roles
			}
			runtimeOK := false
			limit := int64(0)
			for _, runtime := range c.st.policy.Runtimes {
				if runtime.RuntimeID == snapshot.SupervisedProfile && runtime.Enabled {
					runtimeOK = true
					limit = runtime.MaxWorkers.Int()
					for _, want := range required {
						has := false
						for _, role := range runtime.Roles {
							has = has || role == want
						}
						runtimeOK = runtimeOK && has
					}
				}
			}
			if !runtimeOK {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeUnsupported, "required stage role unavailable")
			}
			active := int64(0)
			for _, attempt := range c.st.attempts {
				if attempt.RuntimeID == snapshot.SupervisedProfile && attempt.Supervision != nil && attempt.Supervision.Worker {
					active++
				}
			}
			if active >= limit {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "runtime worker cap")
			}
			if a.Supervision.Worker || a.Quiescence != "PROVED" {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "prior worker not quiescent")
			}
			phase := ""
			switch f.Stage {
			case "implement":
				if a.Phase == "ADMITTED" {
					phase = "RUNNING"
				} else if a.Phase == "RETURNED" {
					phase = "REPAIRING"
				}
			case "review":
				if a.Phase == "BUILT" {
					phase = "REVIEWING"
				}
			case "integrate":
				if a.Phase == "READY_FOR_INTEGRATION" {
					phase = "RUNNING"
				}
			}
			if a.Phase == "WAITING" && f.Stage == a.Stage && a.Supervision.Answer != "" {
				phase = a.Supervision.ResumePhase
			}
			if phase == "" {
				return c.fail(malformed("stage dispatch phase"))
			}
			workers := int64(0)
			for _, en := range entries {
				workers += en.Workers.Int()
			}
			if workers >= c.st.policy.MaxWorkersTotal.Int() {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "worker capacity full")
			}
			if next.Supervision.Turns.Int() >= c.st.policy.Lane.Turns.Int() {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeLimitExceeded, "turn cap reached")
			}
			if f.Stage == "review" && f.Holder == a.Supervision.AuthorHolder {
				return c.fail(malformed("review holder is author"))
			}
			if f.Worktree == "" {
				return c.fail(malformed("stage worktree missing"))
			}
			if _, e := wire.ParsePathText("/worktree", f.Worktree); e != nil {
				return c.fail(e)
			}
			if a.PoolAllocation == nil {
				rec, _ := c.st.tickets.Get(a.TicketID.Raw)
				pool := rec.RequiresPool
				if pool == "" {
					pool = f.Pool
				}
				if pool != "" {
					l := *c.l
					l.Pool = pool
					l.Stage = f.Stage
					l.Holder = a.Lease.Holder
					cc := c
					cc.l = &l
					next.PoolAllocation, e = cc.allocate(&next)
					if e != nil {
						return c.fail(e)
					}
				}
			}
			next.Supervision.WorkerHolder = f.Holder
			next.Stage = f.Stage
			next.Phase = "ADMITTED"
			next.Supervision.ResumePhase = phase
			next.PendingEffects = []string{string(wire.Sum(raw))}
			next.WorktreePath = &f.Worktree
			next.Quiescence = "UNPROVED"
			next.Supervision.Worker = true
			next.Supervision.Turns = wire.CountOf(next.Supervision.Turns.Int() + 1)
			worker = "1"
		case "BOOT":
			if !a.Supervision.Worker || f.LeaderPID <= 0 || f.LeaderStarted == "" {
				return c.fail(malformed("supervised boot binding"))
			}
			if a.Phase != "ADMITTED" || len(a.PendingEffects) != 1 {
				return c.fail(malformed("boot without admitted effect"))
			}
			next.Phase = a.Supervision.ResumePhase
			next.PendingEffects = []string{}
			next.Supervision.LeaderStarted = f.LeaderStarted
			next.Lane = &snapshot.Lane{Pgid: wire.SizeOf(uint64(f.LeaderPID)), LeaderPid: wire.SizeOf(uint64(f.LeaderPID)), LeaderStartTime: "0", SpawnEffectKey: wire.Sum(raw)}
		case "STOPPING":
			if !a.Supervision.Worker {
				return c.fail(malformed("no running worker"))
			}
			if a.Phase != "ADMITTED" {
				next.Supervision.ResumePhase = a.Phase
			}
			next.Phase = "STOPPING"
		case "STOPPED":
			if a.Phase != "STOPPING" || !a.Supervision.Worker {
				return c.fail(malformed("no stopping worker"))
			}
			if !f.Clean {
				next.Phase = "BLOCKED_RECOVERY"
				next.Quiescence = "SURVIVORS"
				break
			}
			next.Quiescence = "PROVED"
			next.Lane = nil
			next.Supervision.Worker = false
			worker = "0"
			next.Supervision.SessionID = f.Session
			next.Budget = map[string]snapshot.BudgetField{}
			for k, v := range a.Budget {
				next.Budget[k] = v
			}
			turns := wire.SizeOf(uint64(next.Supervision.Turns.Int()))
			next.Budget["turns"] = snapshot.BudgetField{State: "OBSERVED", Value: &turns}
			input, output, known := policyHostUsage(c.st.policy, f.Handoff)
			for name, value := range map[string]uint64{"inputTokens": input, "outputTokens": output} {
				next.Budget[name] = cumulativeTokens(a.Budget[name], value, known, a.Supervision.Turns.Int() <= 1)
			}
			if len(f.Handoff) > 0 {
				next.Supervision.HandoffDigest = string(wire.Sum(f.Handoff))
			}
			if a.Stage == "implement" && len(outOfScope(a, f.ChangedPaths)) > 0 {
				next.ScopeCheck = "OUT_OF_SCOPE"
				next.Phase = "BLOCKED_RECOVERY"
				break
			}
			if f.Question != "" {
				next.Phase = "WAITING"
				next.Supervision.Question = f.Question
				next.Supervision.Answer = ""
				break
			}
			switch a.Stage {
			case "implement":
				if f.Session == "" || f.Tree == "" {
					next.Phase = "WAITING"
					next.Supervision.Question = "host result unavailable"
					break
				}
				if _, e = wire.ParseOID("candidate", f.Tree); e != nil {
					return c.fail(e)
				}
				if len(outOfScope(a, f.ChangedPaths)) != 0 {
					return c.refuse(mutation.OutcomeBlocked, wire.CodeOutOfScope, "candidate outside scope")
				}
				next.CandidateTreeOid = &f.Tree
				next.ScopeCheck = "WITHIN"
				next.Phase = "BUILT"
				next.Supervision.AuthorSession = f.Session
				next.Supervision.ReviewDigest = ""
				next.Supervision.ReviewTree = ""
				next.Supervision.IntegrationGrant = ""
				next.GateResults = []string{}
			case "review":
				if f.Holder != a.Supervision.WorkerHolder || f.Session == "" || f.Session == a.Supervision.AuthorSession || f.Holder == a.Supervision.AuthorHolder || a.CandidateTreeOid == nil || f.Tree != *a.CandidateTreeOid {
					return c.fail(malformed("review independence/tree binding"))
				}
				rec, _ := c.st.tickets.Get(a.TicketID.Raw)
				expected := []string{}
				for _, claim := range rec.AcceptanceCriteria {
					expected = append(expected, string(wire.Sum([]byte(claim))))
				}
				sort.Strings(expected)
				sort.Strings(f.Claims)
				if !f.Accepted || !equalStrings(expected, f.Claims) {
					next.Phase = "RETURNED"
					next.RepairRound = wire.CountOf(next.RepairRound.Int() + 1)
					if next.RepairRound.Int() > 2 || next.RepairRound.Int() > c.st.policy.RepairRounds.Int() || (c.st.policy.Supervision != nil && next.RepairRound.Int() > c.st.policy.Supervision.MaxRepairCycles.Int()) {
						next.Phase = "WAITING"
						next.Supervision.Question = "repair bound reached"
					}
					break
				}
				next.Phase = "CHECKING"
				next.Supervision.ReviewerHolder = f.Holder
				next.Supervision.ReviewerSession = f.Session
				next.Supervision.ReviewTree = f.Tree
				next.Supervision.ReviewDigest = string(wire.Sum(raw))
			case "integrate":
				next.Phase = "READY_FOR_INTEGRATION"
				next.PendingEffects = []string{}
				next.Supervision.IntegrationCommit = f.Commit
			}
		case "INTEGRATE_INTENT":
			if out := c.supervisedReady(a, true, repositories); out != nil {
				return *out
			}
			if a.Phase != "READY_FOR_INTEGRATION" || a.Supervision.IntegrationGrant == "" || a.Supervision.Worker || a.Quiescence != "PROVED" {
				return c.fail(malformed("integration intent phase"))
			}
			if _, e = wire.ParseOID("expected integration", f.Commit); e != nil {
				return c.fail(e)
			}
			next.Supervision.IntegrationExpected = f.Commit
			next.PendingEffects = []string{string(wire.Sum(raw))}
		case "INTEGRATED":
			if f.Commit != a.Supervision.IntegrationExpected || len(a.PendingEffects) != 1 {
				return c.fail(malformed("integration effect outcome differs"))
			}
			if a.Phase != "READY_FOR_INTEGRATION" || a.Stage != "integrate" || a.Supervision.IntegrationGrant == "" || a.Supervision.Worker || a.Quiescence != "PROVED" {
				return c.fail(malformed("integration outcome phase"))
			}
			if _, e = wire.ParseOID("integration commit", f.Commit); e != nil {
				return c.fail(e)
			}
			next.PendingEffects = []string{}
			next.Supervision.IntegrationCommit = f.Commit
		case "ANSWER":
			if a.Phase != "WAITING" || a.Supervision.QuestionID != f.Question || f.Answer == "" || f.AnswerRevision != a.TicketRevision || a.Supervision.QuestionRevision != a.TicketRevision {
				return c.fail(malformed("question answer binding"))
			}
			next.Supervision.Answer = f.Answer
		case "READY":
			if out := c.supervisedReady(a, false, nil); out != nil {
				return *out
			}
			if a.Phase != "CHECKING" || a.Supervision.ReviewDigest == "" {
				return c.fail(malformed("review not accepted"))
			}
			next.Phase = "READY_FOR_INTEGRATION"
		case "GRANT":
			if a.Phase != "READY_FOR_INTEGRATION" || f.Grant == "" {
				return c.fail(malformed("integration grant phase"))
			}
			rec, _ := c.st.tickets.Get(a.TicketID.Raw)
			found := false
			for _, grant := range rec.Approvals {
				if grant.GrantID == f.Grant && grant.Operation == "INTEGRATE" && !grant.Revoked && grant.TargetRevision == a.TicketRevision && len(grant.Scope) == 1 && a.CandidateTreeOid != nil && grant.Scope[0] == ProgramIntegrationScope(a.BaseCommit, *a.CandidateTreeOid, c.st.queue.IntentBranch, repositories) {
					found = true
				}
			}
			if !found {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeApprovalMissing, "exact candidate/base integration grant missing")
			}
			next.Supervision.IntegrationGrant = f.Grant
		case "DRAIN":
			if a.Supervision.Worker || a.Quiescence != "PROVED" {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "drain awaits owned worker")
			}
			next.Phase = "WAITING"
			next.Supervision.Question = "operator drain"
			next.Supervision.Answer = ""
		case "CANCEL":
			if a.Supervision.Worker || a.Quiescence != "PROVED" {
				return c.refuse(mutation.OutcomeBlocked, wire.CodeQuiescenceUnproved, "cancel requires stopped worker")
			}
			next.Phase = "CANCELLED"
			entries = c.without(a.AttemptID)
		default:
			return c.fail(malformed("unknown supervisor action"))
		}
	}
	if next.Phase == "WAITING" && (a.Supervision == nil || a.Phase != "WAITING" || next.Supervision.Question != a.Supervision.Question) {
		next.Supervision.QuestionRevision = a.TicketRevision
		next.Supervision.QuestionID = string(wire.Sum([]byte(a.AttemptID + ":" + string(a.Generation) + ":" + string(a.TicketRevision) + ":" + string(c.seq) + ":" + next.Supervision.Question)))
	}
	for i := range entries {
		if entries[i].AttemptID == a.AttemptID {
			entries[i].Workers = wire.Count(worker)
		}
	}
	var quarantined *snapshot.PoolState
	if a.PoolAllocation != nil && ((f.Action == "STOPPED" && f.Clean) || (f.Action == "OWNERSHIP" && recovered && a.Supervision.Worker)) {
		copy := *c.st.pools
		copy.Entries = append([]snapshot.PoolEntry{}, copy.Entries...)
		for i := range copy.Entries {
			if sameAllocation(&copy.Entries[i].PoolAllocation, a.PoolAllocation) {
				copy.Entries[i].State = "QUARANTINED"
				copy.Entries[i].ChangedSeq = c.seq
				copy.Entries[i].Reason = "stage stopped; exact operator safe reuse confirmation required"
			}
		}
		quarantined = &copy
		next.PoolAllocation = nil
	}
	out := c.write(&next, entries, "TRANSITION", false)
	if out.result != nil {
		return out
	}
	out.posts["evidence/"+string(wire.Sum(raw))] = raw
	if len(f.Handoff) > 0 {
		c.postEvidence(out.posts, f.Handoff)
	}
	if quarantined != nil {
		out.posts["pools.json"], e = quarantined.Encode()
		if e != nil {
			return c.fail(e)
		}
	}
	return out
}
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func IntegrationScope(base, tree, branch string) string {
	return "taskman-integration:" + string(wire.Sum([]byte(base+"\x00"+tree+"\x00"+branch)))
}

// ProgramIntegrationScope is the exact integration grant scope of a
// supervised program. A multi-repository program also binds, in name order,
// each extra repository's name and designated integration branch (empty when
// undesignated), so a grant names every target the supervisor may move; the
// composite tree already binds each repository's candidate (CAL-V0-087).
// Without extra repositories it is IntegrationScope.
func ProgramIntegrationScope(base, tree, branch string, repositories []snapshot.RepositoryRecord) string {
	if len(repositories) == 0 {
		return IntegrationScope(base, tree, branch)
	}
	bound := base + "\x00" + tree + "\x00" + branch
	for _, r := range repositories {
		bound += "\x00" + r.Name + "\x00" + r.IntegrationBranch
	}
	return "taskman-integration:" + string(wire.Sum([]byte(bound)))
}

func (c leaseContext) supervisedReady(a *snapshot.Attempt, grant bool, repositories []snapshot.RepositoryRecord) *leaseOutcome {
	refuse := func(code, detail string) *leaseOutcome {
		o := c.refuse(mutation.OutcomeBlocked, code, detail)
		return &o
	}
	rec, ok := c.st.tickets.Get(a.TicketID.Raw)
	if !ok {
		return refuse(wire.CodeMissingEvidence, "ticket missing")
	}
	if a.CandidateTreeOid == nil || a.Supervision == nil || a.Supervision.ReviewTree != *a.CandidateTreeOid || a.Supervision.ReviewDigest == "" {
		return refuse(wire.CodeMissingEvidence, "accepted exact review missing")
	}
	if rec.Status != "OPEN" || rec.AcceptanceRevision != a.TicketRevision {
		return refuse(wire.CodeStaleTicket, "ticket status/revision changed")
	}
	if a.PolicySha256 != wire.Sum(c.st.policy.Raw) {
		return refuse(wire.CodeStalePolicy, "policy changed")
	}
	if a.ScopeCheck != "WITHIN" {
		return refuse(wire.CodeOutOfScope, "scope not proved")
	}
	if code, detail := approvalBlocker(rec); code != "" {
		return refuse(code, detail)
	}
	results, e := c.gateResults(a)
	if e != nil {
		o := c.fail(e)
		return &o
	}
	for _, id := range c.requiredGates(rec) {
		if _, code, detail := c.gateVerdict(a, results, id); code != "" {
			return refuse(code, detail)
		}
	}
	if grant {
		found := false
		for _, g := range rec.Approvals {
			if g.GrantID == a.Supervision.IntegrationGrant && g.Operation == "INTEGRATE" && !g.Revoked && g.TargetRevision == a.TicketRevision && len(g.Scope) == 1 && g.Scope[0] == ProgramIntegrationScope(a.BaseCommit, *a.CandidateTreeOid, c.st.queue.IntentBranch, repositories) {
				found = true
			}
		}
		if !found {
			return refuse(wire.CodeApprovalMissing, "exact integration grant missing or revoked")
		}
	}
	return nil
}

// A later observation cannot fill an earlier missing token interval.
func cumulativeTokens(old snapshot.BudgetField, value uint64, known, first bool) snapshot.BudgetField {
	if !known || !first && (old.State != "OBSERVED" || old.Value == nil) {
		return snapshot.BudgetField{State: "NOT_OBSERVED"}
	}
	if !first {
		prior := old.Value.Uint64()
		if value > ^uint64(0)-prior {
			return snapshot.BudgetField{State: "NOT_OBSERVED"}
		}
		value += prior
	}
	n := wire.SizeOf(value)
	return snapshot.BudgetField{State: "OBSERVED", Value: &n}
}

func supervisedIntentCode(a *snapshot.Attempt, rec *ticket.Record, policy []byte) string {
	if rec == nil || rec.Status != "OPEN" || rec.AcceptanceRevision != a.TicketRevision {
		return wire.CodeStaleTicket
	}
	if a.PolicySha256 != wire.Sum(policy) {
		return wire.CodeStalePolicy
	}
	return ""
}

// policyHostUsage re-derives one supervised turn's usage from retained host
// output in the vocabulary of the policy's supervised host (CAL-V0-075); an
// absent policy or host is Codex.
func policyHostUsage(policy *intent.Policy, raw []byte) (uint64, uint64, bool) {
	var supervision *intent.SupervisionPolicy
	if policy != nil {
		supervision = policy.Supervision
	}
	vocabulary, ok := supervisor.HostVocabulary(supervision.SupervisedHost())
	if !ok {
		return 0, 0, false
	}
	return vocabulary.Usage(raw)
}
