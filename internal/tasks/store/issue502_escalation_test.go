package store_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var holder = mutation.Binding{ID: "agent-1", Role: "OWNER"}

// escalationClaim is one ticket with a live claim by agent-1 and the audited
// source that claim's ADMIT receipt names.
func escalationClaim(t *testing.T) (*leaseStore, string, ticket.EscalationSource) {
	t.Helper()
	s := newLeaseStore(t)
	id := s.ticket(t, "escalate")
	claim := s.claim(t, "claim-1", id, 0)
	receipt, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", claim.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	post, err := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", claim.AttemptID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	a := s.attempt(t, claim.AttemptID)
	return s, id, ticket.EscalationSource{
		QueueID: fixture.QueueID, TicketID: id, AttemptID: claim.AttemptID, Generation: claim.Generation, Holder: "agent-1",
		AcceptanceRevision: a.TicketRevision, ReceiptSequence: *claim.Outcome.ReceiptSeq,
		ReceiptSha256: wire.Sum(receipt), PostAttemptSha256: wire.Sum(post), TicketRecordSha256: a.TicketRecordSha256,
	}
}

func openRequest(t *testing.T, requestID string, src ticket.EscalationSource, supersedes, expected string) []byte {
	t.Helper()
	raw, err := ticket.EncodeEscalationRequest(ticket.EscalationRequest{
		Profile: ticket.EscalationRequestProfile, QueueID: src.QueueID, TicketID: src.TicketID, RequestID: requestID,
		Actor: src.Holder, ActorRole: "OWNER", Operation: "OPEN",
		Open: &ticket.EscalationOpen{Source: src, Kind: "decision", Question: "which way?", Options: []string{"a", "b"}, Supersedes: supersedes, ExpectedRevision: wire.Count(expected)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func answerRequest(t *testing.T, requestID, ticketID string, actor mutation.Binding, target, expected string) []byte {
	t.Helper()
	raw, err := ticket.EncodeEscalationRequest(ticket.EscalationRequest{
		Profile: ticket.EscalationRequestProfile, QueueID: fixture.QueueID, TicketID: ticketID, RequestID: requestID,
		Actor: actor.ID, ActorRole: actor.Role, Operation: "ANSWER",
		Answer: &ticket.EscalationAnswer{Text: "take a", RequestID: target, ExpectedRevision: wire.Count(expected)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func escalate(t *testing.T, s *leaseStore, actor mutation.Binding, raw []byte, minutes int) *store.Report {
	t.Helper()
	report, err := store.Escalate(context.Background(), s.repo, actor, fixture.QueueID, raw, s.at(t, minutes))
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	return report
}

func answer(t *testing.T, s *leaseStore, actor mutation.Binding, raw []byte, minutes int) *store.Report {
	t.Helper()
	report, err := store.AnswerEscalation(context.Background(), s.repo, actor, fixture.QueueID, raw, s.at(t, minutes))
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	return report
}

func committed(t *testing.T, report *store.Report, ops ...string) {
	t.Helper()
	if report.Outcome.Outcome != mutation.OutcomeCompleted || len(report.EscalationEvents) != len(ops) {
		t.Fatalf("want committed %v, got %+v", ops, report)
	}
	// Events are reported in receipt post order, which is digest order.
	got := []string{}
	for _, e := range report.EscalationEvents {
		got = append(got, e.Operation)
	}
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(ops))) {
		t.Fatalf("events %v, want %v", got, ops)
	}
}

func escalationRefs(t *testing.T, s *leaseStore, id string) (*ticket.Record, map[string]ticket.EscalationRef) {
	t.Helper()
	parsed, err := wire.ParseTicketID("id", id)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, ".taskman", "tickets", parsed.Local+".json"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ticket.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]ticket.EscalationRef{}
	if rec.Escalations != nil {
		for _, e := range rec.Escalations.Entries {
			out[e.RequestID] = e
		}
	}
	return rec, out
}

func refusedEscalation(t *testing.T, report *store.Report, outcome, code, reducer string) {
	t.Helper()
	if report.Outcome.Outcome != outcome || (code != "" && !has(report.Outcome.Codes, code)) {
		t.Fatalf("want %s %s, got %+v", outcome, code, report)
	}
	if report.Escalation == nil || report.Escalation.Code != reducer || report.Outcome.ReceiptSeq != nil {
		t.Fatalf("want reducer %s and no receipt, got %+v %+v", reducer, report.Escalation, report.Outcome)
	}
}

// TestIssue502_OpenAnswerCommitsTicketAndEvents: a holder's OPEN audits its
// claim receipt and posts the ticket and the OPEN event in one LEASE
// transaction without moving the acceptance revision or fencing the attempt
// (ESC-V0-001, ESC-V0-003, ESC-V0-010); a retry replays the same event; the
// sole-open ANSWER shorthand resolves and reports its target (ESC-V0-004).
func TestIssue502_OpenAnswerCommitsTicketAndEvents(t *testing.T) {
	s, id, src := escalationClaim(t)
	before, _ := escalationRefs(t, s, id)
	open := openRequest(t, "q-1", src, "", "")
	first := escalate(t, s, holder, open, 1)
	committed(t, first, "OPEN")
	if first.Ticket != id || first.Kind != "Transaction" {
		t.Fatalf("report %+v", first)
	}
	rec, refs := escalationRefs(t, s, id)
	if refs["q-1"].State != "OPEN" || rec.AcceptanceRevision != before.AcceptanceRevision {
		t.Fatalf("refs %+v acceptance %s -> %s", refs, before.AcceptanceRevision, rec.AcceptanceRevision)
	}
	replay := escalate(t, s, holder, open, 2)
	committed(t, replay, "OPEN")
	if replay.Kind != "Replay" || *replay.Outcome.ReceiptSeq != *first.Outcome.ReceiptSeq || replay.EscalationEvents[0].RequestSha256 != first.EscalationEvents[0].RequestSha256 {
		t.Fatalf("replay %+v", replay)
	}
	renewed := s.lease(t, "renew-1", transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: src.AttemptID, Generation: src.Generation, LeaseMinutes: "60"}, 3, nil)
	if renewed.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("the escalation fenced the attempt: %+v", renewed)
	}

	answered := answer(t, s, operator(), answerRequest(t, "a-1", id, operator(), "", ""), 4)
	committed(t, answered, "ANSWER")
	if answered.EscalationEvents[0].EscalationID != "q-1" {
		t.Fatalf("answer resolved %q", answered.EscalationEvents[0].EscalationID)
	}
	if _, refs := escalationRefs(t, s, id); refs["q-1"].State != "ANSWERED" {
		t.Fatalf("refs after answer %+v", refs)
	}
}

// TestIssue502_SupersedePostsBothEvents: a supersession posts the ticket and
// two events in one stage frozen under the closed ESCALATION operation, inside
// its measured bound (ESC-V0-010).
func TestIssue502_SupersedePostsBothEvents(t *testing.T) {
	s, id, src := escalationClaim(t)
	first := escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1)
	committed(t, first, "OPEN")
	_, refs := escalationRefs(t, s, id)
	var published []transaction.Description
	restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
		published = append(published, a.Description)
		return nil
	})
	defer restore()
	next := escalate(t, s, holder, openRequest(t, "q-2", src, "q-1", string(refs["q-1"].Revision)), 2)
	restore()
	if next.Outcome.Outcome != mutation.OutcomeCompleted || len(next.EscalationEvents) != 2 {
		t.Fatalf("supersede %+v", next)
	}
	// The maximal escalation stage (request, ticket, two events, receipt and
	// head), re-encoded from the published descriptions with a same-length
	// request digest, is a valid closed ESCALATION descriptor inside that
	// operation's measured bound (ESC-V0-010).
	sort.Slice(published, func(i, j int) bool { return published[i].Slot < published[j].Slot })
	desc := snapshot.StageDescriptor{QueueID: fixture.QueueID, Operation: snapshot.StageEscalation, RequestID: "q-2", RequestSha256: wire.Sum(nil), RecordedAt: s.at(t, 2),
		Base: &snapshot.StageBase{LastSeq: *first.Outcome.ReceiptSeq, LastReceiptSha256: wire.Sum(nil)}, Artifacts: published}
	raw, err := desc.Encode()
	if err != nil {
		t.Fatalf("descriptor: %v", err)
	}
	if _, err := snapshot.DecodeStageDescriptor(raw); err != nil {
		t.Fatalf("descriptor round trip: %v", err)
	}
	t.Logf("supersession stage: %d artifacts, %d descriptor bytes", len(published), len(raw))
	if limit, bytes := snapshot.StageLimits(snapshot.StageEscalation); len(published) > limit || len(raw) > bytes {
		t.Fatalf("stage %d artifacts, %d descriptor bytes", len(published), len(raw))
	}
	if _, refs := escalationRefs(t, s, id); refs["q-1"].State != "SUPERSEDED" || refs["q-2"].State != "OPEN" {
		t.Fatalf("refs %+v", refs)
	}
	// The first request still replays its own committed OPEN after a later
	// question changed the ticket.
	replay := escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 3)
	committed(t, replay, "OPEN")
	if replay.Kind != "Replay" || replay.EscalationEvents[0].EscalationID != "q-1" {
		t.Fatalf("replay %+v", replay)
	}
}

// TestIssue502_RedoRepublishesTheTicket: a crash after the receipt was
// linked in but before its ticket projection and head were published is
// finished by the retry, which then replays (§5.2 crash point C2). The
// events are retained evidence published before the commit point, so the
// retry reads them back rather than rewriting them.
func TestIssue502_RedoRepublishesTheTicket(t *testing.T) {
	s, id, src := escalationClaim(t)
	head := filepath.Join(s.repo.StateDir, "head.json")
	before, err := os.ReadFile(head)
	if err != nil {
		t.Fatal(err)
	}
	beforeTicket, _ := escalationRefs(t, s, id)
	open := openRequest(t, "q-1", src, "", "")
	first := escalate(t, s, holder, open, 1)
	committed(t, first, "OPEN")
	parsed, _ := wire.ParseTicketID("id", id)
	projection := filepath.Join(s.repo.PrimaryWorktree, ".taskman", "tickets", parsed.Local+".json")
	if err := os.WriteFile(head, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projection, beforeTicket.Encode(), 0o600); err != nil {
		t.Fatal(err)
	}
	again := escalate(t, s, holder, open, 2)
	if !again.Redone {
		t.Error("the pending receipt was not redone")
	}
	committed(t, again, "OPEN")
	if _, refs := escalationRefs(t, s, id); refs["q-1"].State != "OPEN" {
		t.Fatalf("refs after redo %+v", refs)
	}
}

// TestIssue502_ActorBindingBeforeReplay: another actor presenting the
// holder's request is refused ACTOR_BINDING before and after it commits,
// never REQUEST_ID_CONFLICT, and an OPERATOR without an explicit grant may
// not answer (ESC-V0-001, ESC-V0-004).
func TestIssue502_ActorBindingBeforeReplay(t *testing.T) {
	s, id, src := escalationClaim(t)
	open := openRequest(t, "q-1", src, "", "")
	refusedEscalation(t, escalate(t, s, operator(), open, 1), mutation.OutcomeUnauthorized, "", "ACTOR_BINDING")
	committed(t, escalate(t, s, holder, open, 2), "OPEN")
	refusedEscalation(t, escalate(t, s, operator(), open, 3), mutation.OutcomeUnauthorized, "", "ACTOR_BINDING")

	op := mutation.Binding{ID: "op-1", Role: "OPERATOR"}
	refusedEscalation(t, answer(t, s, op, answerRequest(t, "a-1", id, op, "", ""), 4), mutation.OutcomeUnauthorized, "", "POLICY_NOT_ALLOWED")
}

// TestIssue502_OpenAuditsTheClaim: a forged receipt digest abstains, an
// expired lease is fenced; neither writes.
func TestIssue502_OpenAuditsTheClaim(t *testing.T) {
	s, _, src := escalationClaim(t)
	forged := src
	forged.ReceiptSha256 = wire.Sum([]byte("forged"))
	refusedEscalation(t, escalate(t, s, holder, openRequest(t, "q-1", forged, "", ""), 1), mutation.OutcomeBlocked, wire.CodeMissingEvidence, "MISSING_ADMISSION_CONTEXT")
	refusedEscalation(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 61), mutation.OutcomeRevisionConflict, wire.CodeFenced, "EXPIRED_ADMISSION")
}

// TestIssue502_AnswerRaceHasOneWinner: two answers under the same question
// CAS race through the store lock; exactly one commits.
func TestIssue502_AnswerRaceHasOneWinner(t *testing.T) {
	s, id, src := escalationClaim(t)
	committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
	_, refs := escalationRefs(t, s, id)
	rev := string(refs["q-1"].Revision)
	reports := make([]*store.Report, 2)
	var wg sync.WaitGroup
	for i, rid := range []string{"a-1", "a-2"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := store.AnswerEscalation(context.Background(), s.repo, operator(), fixture.QueueID, answerRequest(t, rid, id, operator(), "q-1", rev), s.at(t, 2))
			if err != nil {
				t.Error(err)
			}
			reports[i] = r
		}()
	}
	wg.Wait()
	won := 0
	for _, r := range reports {
		switch {
		case r == nil:
		case r.Outcome.Outcome == mutation.OutcomeCompleted:
			won++
		default:
			refusedEscalation(t, r, mutation.OutcomeRevisionConflict, wire.CodeStaleTicket, "STALE_QUESTION_CAS")
		}
	}
	if won != 1 {
		t.Fatalf("winners %d: %+v %+v", won, reports[0], reports[1])
	}
}

// TestIssue502_SupervisedAttemptIsUnsupported: an attempt a program attached
// to after its claim cannot escalate through the external-agent path until
// the supervised grant adapter is qualified (ESC-V0-001).
func TestIssue502_SupervisedAttemptIsUnsupported(t *testing.T) {
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(digest), "fileSha256", str(digest), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
	if r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("supervised-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", r, e)
	}
	id := s.ticket(t, "supervised")
	c := claimOf(id, "path")
	c.Stage, c.Pool = "implement", "db"
	claim := s.lease(t, "supervised-claim", c, 0, nil)
	p := snapshot.Program{CurrentAttempt: claim.AttemptID, CurrentGeneration: string(claim.Generation), Assignment: 1, ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 99, OwnerStarted: "observed-test-identity", Epoch: 1, ConfigSHA256: digest, Phase: "ADMITTED", Base: "0123456789012345678901234567890123456789", Worktree: "/fixture"}
	if r, e := store.ProgramTransition(context.Background(), s.repo, operator(), fixture.QueueID, "program-admit", p); e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("program %+v %v", r, e)
	}
	if r, e := store.SupervisorTransition(context.Background(), s.repo, operator(), fixture.QueueID, "step-attach", claim.AttemptID, claim.Generation, transaction.SupervisorChange{Action: "ATTACH", Pool: "db", ProgramID: p.ID, OwnerPID: p.OwnerPID, OwnerStarted: p.OwnerStarted}); e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("attach %+v %v", r, e)
	}
	receipt, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", claim.Receipt))
	post, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", claim.AttemptID+".json"))
	a := s.attempt(t, claim.AttemptID)
	if a.Supervision == nil {
		t.Fatal("ATTACH did not supervise the attempt")
	}
	// The claim's own POST attempt is unsupervised; read its digest from the
	// receipt rather than from the attached projection.
	rc, err := snapshot.DecodeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rc.Post {
		if p.Path == "attempts/"+claim.AttemptID+".json" && p.Sha256 != nil {
			post = nil
			src := ticket.EscalationSource{QueueID: fixture.QueueID, TicketID: id, AttemptID: claim.AttemptID, Generation: claim.Generation, Holder: "agent-1",
				AcceptanceRevision: a.TicketRevision, ReceiptSequence: *claim.Outcome.ReceiptSeq, ReceiptSha256: wire.Sum(receipt), PostAttemptSha256: *p.Sha256, TicketRecordSha256: a.TicketRecordSha256}
			refusedWith(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), mutation.OutcomeUnsupported, wire.CodeUnsupported)
		}
	}
	if post != nil {
		t.Fatal("the claim receipt posts no attempt")
	}
}

// TestIssue502_DeletedEventIsJournalDamage: an event is retained evidence
// published before the commit point, so a redo or replay that finds it
// missing refuses JOURNAL_FORKED rather than recreating it.
func TestIssue502_DeletedEventIsJournalDamage(t *testing.T) {
	s, _, src := escalationClaim(t)
	open := openRequest(t, "q-1", src, "", "")
	committed(t, escalate(t, s, holder, open, 1), "OPEN")
	dir := filepath.Join(s.repo.StateDir, "evidence")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err == nil && bytes.Contains(raw, []byte(ticket.EscalationEventProfile)) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				t.Fatal(err)
			}
			removed++
		}
	}
	if removed != 1 {
		t.Fatalf("removed %d event files", removed)
	}
	report, err := store.Escalate(context.Background(), s.repo, holder, fixture.QueueID, open, s.at(t, 2))
	if wire.CodeOf(err) != wire.CodeJournalForked {
		t.Fatalf("replay without its event: %v %+v", err, report)
	}
}

// TestIssue502_ShorthandAnswerReplaysAfterLaterOpen: a shorthand answer
// replays the question it resolved, even after a later OPEN would make the
// same shorthand resolve another question (ESC-V0-004, ESC-V0-010).
func TestIssue502_ShorthandAnswerReplaysAfterLaterOpen(t *testing.T) {
	s, id, src := escalationClaim(t)
	committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
	reply := answerRequest(t, "a-1", id, operator(), "", "")
	first := answer(t, s, operator(), reply, 2)
	committed(t, first, "ANSWER")
	committed(t, escalate(t, s, holder, openRequest(t, "q-2", src, "", ""), 3), "OPEN")
	replay := answer(t, s, operator(), reply, 4)
	committed(t, replay, "ANSWER")
	if replay.Kind != "Replay" || replay.EscalationEvents[0].EscalationID != "q-1" || *replay.Outcome.ReceiptSeq != *first.Outcome.ReceiptSeq {
		t.Fatalf("replay %+v", replay)
	}
	if _, refs := escalationRefs(t, s, id); refs["q-1"].State != "ANSWERED" || refs["q-2"].State != "OPEN" {
		t.Fatalf("refs %+v", refs)
	}
}

// TestIssue502_OperatorAnswersThroughExplicitGrant: OPERATOR may answer only
// through a policy.roles.OPERATOR row that names ANSWER (ESC-V0-004), and a
// row for any other non-owner role may not name it at all.
func TestIssue502_OperatorAnswersThroughExplicitGrant(t *testing.T) {
	s, id, src := escalationClaim(t)
	committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
	op := mutation.Binding{ID: "op-1", Role: "OPERATOR"}
	refusedEscalation(t, answer(t, s, op, answerRequest(t, "a-1", id, op, "", ""), 2), mutation.OutcomeUnauthorized, "", "POLICY_NOT_ALLOWED")

	policy := func(version, role string, ops []string) []byte {
		v := fixture.PolicyValue()
		v.Obj.Set("policyVersion", str(version))
		v.Obj.Set("capacity", obj("maxActiveAttempts", str("4"), "maxWorkersTotal", str("4"), "classes", wire.Array()))
		budgets, _ := v.Obj.Get("budgets")
		budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
		v.Obj.Set("roles", obj(role, wire.Strings(ops)))
		return wire.EncodeFile(v)
	}
	r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("worker-answer", "2", policy("3", "WORKER", []string{"ANSWER", "REFINE"})), s.at(t, 2))
	if detail := fmt.Sprint(e); e == nil {
		detail = r.Detail
		if r.Outcome.Outcome == mutation.OutcomeCompleted || !strings.Contains(detail, "operation for WORKER") {
			t.Fatalf("a WORKER row naming ANSWER: %+v", r)
		}
	} else if !strings.Contains(detail, "operation for WORKER") {
		t.Fatalf("a WORKER row naming ANSWER: %v", e)
	}
	if r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("operator-answer", "2", policy("3", "OPERATOR", []string{"ANSWER"})), s.at(t, 2)); e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", r, e)
	}
	committed(t, answer(t, s, op, answerRequest(t, "a-2", id, op, "", ""), 4), "ANSWER")
	if _, refs := escalationRefs(t, s, id); refs["q-1"].State != "ANSWERED" {
		t.Fatalf("q-1 %+v", refs["q-1"])
	}

	// Full audit decodes every historical policy, so dropping the grant from
	// policy does not make a revert safe: a binary without the grant still
	// refuses the journal's earlier OPERATOR row.
	if r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("operator-revoke", "3", policy("4", "OWNER", slices.Sorted(slices.Values(intent.Operations)))), s.at(t, 5)); e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("revoke %+v %v", r, e)
	}
	auditOK(t, s.repo)
	granted := intent.ExplicitGrantOperations
	defer func() { intent.ExplicitGrantOperations = granted }()
	intent.ExplicitGrantOperations = map[string][]string{"OPERATOR": {"NOTE_SET", "NOTE_CLEAR"}}
	q, _ := wire.ParseQueueID("queueId", fixture.QueueID)
	if _, err := (journal.Reader{Source: journal.Native{StateDir: s.repo.StateDir, PrimaryWorktree: s.repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: s.repo.PrimaryWorktree}).Audit(); err == nil || !strings.Contains(err.Error(), "operation for OPERATOR") {
		t.Fatalf("audit without the grant: %v", err)
	}
}

// TestIssue502_InterruptedSupersessionRedoes: a supersession interrupted
// before each of its published artifacts is finished by the retry: before
// the receipt it commits afresh, after it the pending receipt is redone, and
// either way both events, the references and the audit agree (ESC-V0-010).
func TestIssue502_InterruptedSupersessionRedoes(t *testing.T) {
	probe, probeID, probeSrc := escalationClaim(t)
	committed(t, escalate(t, probe, holder, openRequest(t, "q-1", probeSrc, "", ""), 1), "OPEN")
	_, probeRefs := escalationRefs(t, probe, probeID)
	var all []transaction.Description
	restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error { all = append(all, a.Description); return nil })
	defer restore()
	committed(t, escalate(t, probe, holder, openRequest(t, "q-2", probeSrc, "q-1", string(probeRefs["q-1"].Revision)), 2), "OPEN", "SUPERSEDE")
	restore()
	if len(all) == 0 {
		t.Fatal("no artifact was published")
	}
	injected := errors.New("publication interrupted")
	for k, at := range all {
		t.Run(fmt.Sprintf("%02d-%s-%s", k, at.Role, strings.SplitN(at.Target, "/", 2)[0]), func(t *testing.T) {
			s, id, src := escalationClaim(t)
			committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
			_, refs := escalationRefs(t, s, id)
			req := openRequest(t, "q-2", src, "q-1", string(refs["q-1"].Revision))
			n := 0
			restore := store.SetPublishFaultForTest(func(transaction.Artifact) error {
				n++
				if n == k+1 {
					return injected
				}
				return nil
			})
			defer restore()
			_, err := store.Escalate(context.Background(), s.repo, holder, fixture.QueueID, req, s.at(t, 2))
			restore()
			if err == nil {
				t.Fatalf("fault at artifact %d not injected", k)
			}
			again := escalate(t, s, holder, req, 2)
			committed(t, again, "OPEN", "SUPERSEDE")
			afterReceipt := false
			for _, d := range all[:k] {
				afterReceipt = afterReceipt || d.Role == "RECEIPT"
			}
			if again.Redone != afterReceipt {
				t.Fatalf("redone %v, want %v: %+v", again.Redone, afterReceipt, again)
			}
			if _, refs := escalationRefs(t, s, id); refs["q-1"].State != "SUPERSEDED" || refs["q-2"].State != "OPEN" {
				t.Fatalf("refs %+v", refs)
			}
			auditOK(t, s.repo)
			if err := store.FoldReceiptBindings(s.repo, again.Outcome.ReceiptSeq.Uint64(), nil); err != nil {
				t.Fatalf("binding fold: %v", err)
			}
		})
	}
}

// escReceiptForge rewrites receipt seq so that it posts forged instead of
// its escalation event: the forged event is published under its own digest,
// and the receipt's evidence entries and ticket post (question references)
// are rehashed to name it. Every byte stays self-consistent; only the
// escalation transition is untrue. It returns the forged receipt bytes and
// the forged ticket record bytes.
func escReceiptForge(t *testing.T, repo *intent.Repository, seq uint64, forge func(*ticket.EscalationEvent)) (receipt, record []byte) {
	t.Helper()
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.StateDir, "receipts", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rv, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	posts, _ := rv.Obj.Get("post")
	var old, forged wire.Digest
	for _, p := range posts.Arr {
		at, _ := p.Obj.Get("path")
		if !strings.HasPrefix(at.Str, "evidence/") {
			continue
		}
		old = wire.Digest(strings.TrimPrefix(at.Str, "evidence/"))
		event, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(old)))
		if err != nil {
			t.Fatal(err)
		}
		e, err := ticket.DecodeEscalationEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		forge(&e)
		if e.RequestSha256, err = requestSum(e.OriginalRequest); err != nil {
			t.Fatal(err)
		}
		if event, err = ticket.EncodeEscalationEvent(e); err != nil {
			t.Fatal(err)
		}
		forged = wire.Sum(event)
		fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(forged)), event)
		p.Obj.Set("path", wire.String("evidence/"+string(forged))).Set("sha256", wire.String(string(forged))).Set("blobSha256", wire.String(string(forged)))
	}
	if old == "" {
		t.Fatalf("receipt %d posts no escalation event", seq)
	}
	pres, _ := rv.Obj.Get("pre")
	for _, p := range pres.Arr {
		if at, _ := p.Obj.Get("path"); at.Str == "evidence/"+string(old) {
			p.Obj.Set("path", wire.String("evidence/"+string(forged)))
		}
	}
	for _, p := range posts.Arr {
		at, _ := p.Obj.Get("path")
		if !strings.HasPrefix(at.Str, "intent/tickets/") {
			continue
		}
		rec, _ := p.Obj.Get("record")
		escalations, _ := rec.Obj.Get("escalations")
		entries, _ := escalations.Obj.Get("entries")
		for _, ref := range entries.Arr {
			for _, key := range []string{"originSha256", "headSha256"} {
				if v, _ := ref.Obj.Get(key); v.Str == string(old) {
					ref.Obj.Set(key, wire.String(string(forged)))
				}
			}
		}
		record = wire.EncodeFile(rec)
		p.Obj.Set("sha256", wire.String(string(wire.Sum(record))))
	}
	receipt = wire.EncodeFile(rv)
	fixture.Write(t, path, receipt)
	return receipt, record
}

func requestSum(r ticket.EscalationRequest) (wire.Digest, error) {
	raw, err := ticket.EncodeEscalationRequest(r)
	return wire.Sum(raw), err
}

// TestIssue502_ForgedEventRefusesAsJournalDamage: a rehashed OPEN event
// whose time, question or actor does not reproduce its receipt's
// transition passes the generic journal audit but refuses redo as
// JOURNAL_FORKED with the projection unchanged, and once settled refuses the
// receipt binding fold that receipt audit runs (ESC-V0-010).
func TestIssue502_ForgedEventRefusesAsJournalDamage(t *testing.T) {
	cases := []struct {
		name, detail string
		forge        func(*ticket.EscalationEvent)
	}{
		{"control", "", nil},
		{"recorded-at", "the posted events are not the replayed events", func(e *ticket.EscalationEvent) { e.RecordedAt = "2026-01-01T00:00:00Z" }},
		{"question", "the request entry does not bind the typed request", func(e *ticket.EscalationEvent) { e.OriginalRequest.Open.Question = "another question?" }},
		{"actor", "does not name this receipt's request, actor or ticket", func(e *ticket.EscalationEvent) {
			e.Actor, e.OriginalRequest.Actor = "someone-else", "someone-else"
			e.Source.Holder, e.OriginalRequest.Open.Source.Holder = "someone-else", "someone-else"
		}},
	}
	for _, tc := range cases {
		for _, settled := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/pending", true: "/settled"}[settled], func(t *testing.T) {
				s, id, src := escalationClaim(t)
				parsed, _ := wire.ParseTicketID("id", id)
				headPath := filepath.Join(s.repo.StateDir, "head.json")
				ticketPath := filepath.Join(s.repo.PrimaryWorktree, ".taskman", "tickets", parsed.Local+".json")
				preHead, _ := os.ReadFile(headPath)
				preTicket, _ := os.ReadFile(ticketPath)
				open := openRequest(t, "q-1", src, "", "")
				first := escalate(t, s, holder, open, 1)
				committed(t, first, "OPEN")
				seq := first.Outcome.ReceiptSeq.Uint64()
				var receipt, record []byte
				if tc.forge != nil {
					receipt, record = escReceiptForge(t, s.repo, seq, tc.forge)
				}
				if settled {
					if tc.forge != nil {
						editJSON(t, headPath, func(v wire.Value) { v.Obj.Set("lastReceiptSha256", wire.String(string(wire.Sum(receipt)))) })
						fixture.Write(t, ticketPath, record)
					}
					auditOK(t, s.repo)
					err := store.FoldReceiptBindings(s.repo, seq, nil)
					forkedWith(t, err, tc.detail)
					return
				}
				// Crash after the receipt: rewind head and the projection.
				fixture.Write(t, headPath, preHead)
				fixture.Write(t, ticketPath, preTicket)
				report, err := store.Escalate(context.Background(), s.repo, holder, fixture.QueueID, open, s.at(t, 2))
				if err == nil && tc.forge != nil {
					t.Fatalf("a forged pending event was redone: %+v", report)
				}
				if tc.forge == nil {
					if err != nil || !report.Redone {
						t.Fatalf("control redo: %+v %v", report, err)
					}
					return
				}
				forkedWith(t, err, tc.detail)
				for path, want := range map[string][]byte{headPath: preHead, ticketPath: preTicket} {
					if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
						t.Fatalf("%s changed on a refused redo", path)
					}
				}
			})
		}
	}
}

func forkedWith(t *testing.T, err error, detail string) {
	t.Helper()
	if detail == "" {
		if err != nil {
			t.Fatalf("control: %v", err)
		}
		return
	}
	if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "escalation binding: ") || !strings.Contains(err.Error(), detail) {
		t.Fatalf("want JOURNAL_FORKED %q, got %v", detail, err)
	}
}

// TestIssue502_EscalationBindingFoldRefusals drives the receipt binding fold
// over a real history with one receipt altered in memory: question
// references may change only in one completed escalation transition that
// posts its request entry, under a recoverable grant, from a retained claim
// admission, and are never dropped (ESC-V0-010).
func TestIssue502_EscalationBindingFoldRefusals(t *testing.T) {
	s, _, src := escalationClaim(t)
	first := escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1)
	committed(t, first, "OPEN")
	open := first.Outcome.ReceiptSeq.Uint64()
	history, sums := receiptHistory(t, s, open)
	ticketPost := func(rc *snapshot.Receipt) snapshot.PostEntry {
		for _, p := range rc.Post {
			if strings.HasPrefix(p.Path, "intent/tickets/") {
				return p
			}
		}
		t.Fatal("no ticket post")
		return snapshot.PostEntry{}
	}
	cases := []struct {
		name, detail string
		alter        func(rcs []*snapshot.Receipt) []*snapshot.Receipt
	}{
		{"control", "", func(rcs []*snapshot.Receipt) []*snapshot.Receipt { return rcs }},
		{"not-a-transition", "changed question references outside one escalation transition", func(rcs []*snapshot.Receipt) []*snapshot.Receipt {
			rcs[open-1].Kind = "MUTATION"
			return rcs
		}},
		{"no-request-entry", "the transition posts no request entry", func(rcs []*snapshot.Receipt) []*snapshot.Receipt {
			rcs[open-1].Post = slices.DeleteFunc(rcs[open-1].Post, func(p snapshot.PostEntry) bool { return strings.HasPrefix(p.Path, "requests/") })
			return rcs
		}},
		{"no-policy", "the grant cannot be recovered", func(rcs []*snapshot.Receipt) []*snapshot.Receipt {
			for _, rc := range rcs[:open-1] {
				rc.Post = slices.DeleteFunc(rc.Post, func(p snapshot.PostEntry) bool { return p.Path == "intent/policy.json" })
			}
			return rcs
		}},
		{"unretained-admission", "the question's source is not a retained claim admission", func(rcs []*snapshot.Receipt) []*snapshot.Receipt {
			rcs[src.ReceiptSequence.Uint64()-1].Kind = "TRANSITION"
			return rcs
		}},
		{"dropped", "dropped its question references", func(rcs []*snapshot.Receipt) []*snapshot.Receipt {
			drop := *rcs[open-1]
			drop.Seq = wire.SizeOf(open + 1)
			drop.Post = []snapshot.PostEntry{{Path: ticketPost(rcs[open-1]).Path}}
			return append(rcs, &drop)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rcs := make([]*snapshot.Receipt, len(history))
			for i, rc := range history {
				c := *rc
				c.Post = slices.Clone(rc.Post)
				rcs[i] = &c
			}
			rcs = tc.alter(rcs)
			fold := &transaction.EscalationReceiptAudit{}
			var err error
			for i, rc := range rcs {
				sum := wire.Sum([]byte("appended"))
				if i < len(sums) {
					sum = sums[i]
				}
				if err = fold.Step(rc, sum, store.ExternalReviewBlob(s.repo)); err != nil {
					break
				}
			}
			forkedWith(t, err, tc.detail)
		})
	}
}

// receiptHistory decodes receipts 1..last with their digests.
func receiptHistory(t *testing.T, s *leaseStore, last uint64) ([]*snapshot.Receipt, []wire.Digest) {
	t.Helper()
	var history []*snapshot.Receipt
	var sums []wire.Digest
	for seq := uint64(1); seq <= last; seq++ {
		name, _ := snapshot.ReceiptName(seq)
		raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", name))
		if err != nil {
			t.Fatal(err)
		}
		rc, err := snapshot.DecodeReceipt(raw)
		if err != nil {
			t.Fatal(err)
		}
		history, sums = append(history, rc), append(sums, wire.Sum(raw))
	}
	return history, sums
}

// TestIssue502_SupervisedSinceClaimFoldRefuses: an OPEN whose source attempt
// some receipt between the claim and the OPEN posted with supervision is
// refused by the binding fold, though the claim's own POST is unsupervised.
// The history is the real one with the ATTACH receipt's attempt post moved
// ahead of the OPEN (ESC-V0-010).
func TestIssue502_SupervisedSinceClaimFoldRefuses(t *testing.T) {
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a"}), "memberConfig", obj("a", obj("health", health)))))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(digest), "fileSha256", str(digest), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
	if r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("supervised-policy", "2", wire.EncodeFile(v)), now(t)); e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", r, e)
	}
	id := s.ticket(t, "supervised")
	c := claimOf(id, "path")
	c.Stage, c.Pool = "implement", "db"
	claim := s.lease(t, "supervised-claim", c, 0, nil)
	receipt, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", claim.Receipt))
	post, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", claim.AttemptID+".json"))
	a := s.attempt(t, claim.AttemptID)
	src := ticket.EscalationSource{QueueID: fixture.QueueID, TicketID: id, AttemptID: claim.AttemptID, Generation: claim.Generation, Holder: "agent-1",
		AcceptanceRevision: a.TicketRevision, ReceiptSequence: *claim.Outcome.ReceiptSeq, ReceiptSha256: wire.Sum(receipt), PostAttemptSha256: wire.Sum(post), TicketRecordSha256: a.TicketRecordSha256}
	// The program and supervisor writers stamp wall time, so the OPEN does too.
	first, err := store.Escalate(context.Background(), s.repo, holder, fixture.QueueID, openRequest(t, "q-1", src, "", ""), now(t))
	if err != nil {
		t.Fatal(err)
	}
	committed(t, first, "OPEN")
	open := first.Outcome.ReceiptSeq.Uint64()
	p := snapshot.Program{CurrentAttempt: claim.AttemptID, CurrentGeneration: string(claim.Generation), Assignment: 1, ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 99, OwnerStarted: "observed-test-identity", Epoch: 1, ConfigSHA256: digest, Phase: "ADMITTED", Base: "0123456789012345678901234567890123456789", Worktree: "/fixture"}
	if r, e := store.ProgramTransition(context.Background(), s.repo, operator(), fixture.QueueID, "program-admit", p); e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("program %+v %v", r, e)
	}
	attach, e := store.SupervisorTransition(context.Background(), s.repo, operator(), fixture.QueueID, "step-attach", claim.AttemptID, claim.Generation, transaction.SupervisorChange{Action: "ATTACH", Pool: "db", ProgramID: p.ID, OwnerPID: p.OwnerPID, OwnerStarted: p.OwnerStarted})
	if e != nil || attach.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("attach %+v %v", attach, e)
	}
	history, sums := receiptHistory(t, s, attach.Outcome.ReceiptSeq.Uint64())
	moved := *history[len(history)-1]
	moved.Post = slices.DeleteFunc(slices.Clone(moved.Post), func(p snapshot.PostEntry) bool { return !strings.HasPrefix(p.Path, "attempts/") })
	for _, order := range []struct {
		name   string
		before bool
	}{{"after", false}, {"before", true}} {
		t.Run(order.name, func(t *testing.T) {
			fold := &transaction.EscalationReceiptAudit{}
			var err error
			for i, rc := range history[:open] {
				if order.before && uint64(i+1) == open {
					if err = fold.Step(&moved, wire.Sum([]byte("moved")), store.ExternalReviewBlob(s.repo)); err != nil {
						break
					}
				}
				if err = fold.Step(rc, sums[i], store.ExternalReviewBlob(s.repo)); err != nil {
					break
				}
			}
			if order.before {
				forkedWith(t, err, "the question's source attempt was supervised")
			} else {
				forkedWith(t, err, "")
			}
		})
	}
}
