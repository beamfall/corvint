package store_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
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
	for i, op := range ops {
		if report.EscalationEvents[i].Operation != op {
			t.Fatalf("event %d: %s, want %s", i, report.EscalationEvents[i].Operation, op)
		}
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
// two events in the one lease stage, inside its existing artifact bound.
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
	next := escalate(t, s, holder, openRequest(t, "q-2", src, "q-1", string(refs["q-1"].Revision)), 2)
	restore()
	if next.Outcome.Outcome != mutation.OutcomeCompleted || len(next.EscalationEvents) != 2 {
		t.Fatalf("supersede %+v", next)
	}
	// The maximal escalation stage (request, ticket, two events, receipt and
	// head) re-encoded from the published descriptions with a same-length
	// request digest stays inside the StageLease bound (ESC-V0-010).
	sort.Slice(published, func(i, j int) bool { return published[i].Slot < published[j].Slot })
	desc := snapshot.StageDescriptor{QueueID: fixture.QueueID, Operation: transaction.Lease, RequestID: "q-2", RequestSha256: wire.Sum(nil), RecordedAt: s.at(t, 2),
		Base: &snapshot.StageBase{LastSeq: *first.Outcome.ReceiptSeq, LastReceiptSha256: wire.Sum(nil)}, Artifacts: published}
	raw, err := desc.Encode()
	if err != nil {
		t.Fatalf("descriptor: %v", err)
	}
	t.Logf("supersession stage: %d artifacts, %d descriptor bytes", len(published), len(raw))
	if limit, bytes := snapshot.StageLimits(transaction.Lease); len(published) > limit || len(raw) > bytes {
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
// expired lease is fenced, and a reclaimed generation is stale; none writes.
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
