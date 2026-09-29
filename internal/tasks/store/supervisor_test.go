package store_test

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSupervisorWaitRetainsScopeReleasesWorker(t *testing.T) {
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	v.Obj.Set("gates", wire.Array(commandGate("verify", "exit 3", "10", true)))
	health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
	v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a", "b", "review"}), "reservedFor", obj("review", str("review")), "memberConfig", obj("a", obj("health", health), "b", obj("health", health), "review", obj("health", health)))))
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	lane, _ := b.Obj.Get("lane")
	lane.Obj.Set("inputTokens", str("0"))
	lane.Obj.Set("outputTokens", str("0"))
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(digest), "fileSha256", str(digest), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER", "REVIEWER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
	report, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("supervised-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", report, e)
	}
	id := s.ticket(t, "supervised")
	c := claimOf(id, "path")
	c.Stage = "implement"
	c.Pool = "db"
	claim := s.lease(t, "supervised-claim", c, 0, nil)
	p := snapshot.Program{CurrentAttempt: claim.AttemptID, CurrentGeneration: string(claim.Generation), Assignment: 1, ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 99, OwnerStarted: "observed-test-identity", Epoch: 1, ConfigSHA256: digest, Phase: "ADMITTED", Base: "0123456789012345678901234567890123456789", Worktree: "/fixture"}
	report, e = store.ProgramTransition(context.Background(), s.repo, operator(), "queue:acme:main", "program-admit", p)
	if e != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("program %+v %v", report, e)
	}
	stepNumber := 0
	step := func(action string, f transaction.SupervisorChange) {
		t.Helper()
		stepNumber++
		f.Action = action
		f.Pool = "db"
		f.ProgramID = p.ID
		f.OwnerPID = p.OwnerPID
		f.OwnerStarted = p.OwnerStarted
		r, e := store.SupervisorTransition(context.Background(), s.repo, operator(), "queue:acme:main", fmt.Sprintf("step-%s-%d", action, stepNumber), claim.AttemptID, claim.Generation, f)
		if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s %+v %v", action, r, e)
		}
	}
	step("ATTACH", transaction.SupervisorChange{})
	step("DISPATCH", transaction.SupervisorChange{Stage: "implement", Holder: c.Holder, Worktree: "/fixture/implement"})
	step("BOOT", transaction.SupervisorChange{LeaderPID: 100, LeaderStarted: "observed-leader"})
	step("STOPPING", transaction.SupervisorChange{})
	step("STOPPED", transaction.SupervisorChange{Clean: true, Session: "session-1", Question: "question-1"})
	raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", claim.AttemptID+".json"))
	if e != nil {
		t.Fatal(e)
	}
	a, e := snapshot.DecodeAttempt(raw)
	if e != nil || a.Phase != "WAITING" || a.Supervision.Worker || a.Quiescence != "PROVED" {
		t.Fatalf("wait %+v %v", a, e)
	}
	raw, e = os.ReadFile(filepath.Join(s.repo.StateDir, "reservations.json"))
	if e != nil {
		t.Fatal(e)
	}
	rs, e := snapshot.DecodeReservations(raw)
	if e != nil || len(rs.Entries) != 1 || rs.Entries[0].Workers != "0" {
		t.Fatalf("reservation %+v %v", rs, e)
	}
	release, err := store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: "queue:acme:main", RequestID: "legacy-release", Root: s.repo.PrimaryWorktree, Lease: transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claim.AttemptID, Generation: claim.Generation}}, wire.Timestamp(time.Now().UTC().Format(time.RFC3339)))
	if err != nil {
		t.Fatal(err)
	}
	if !release.Outcome.HasCode(wire.CodeQuiescenceUnproved) {
		t.Fatalf("legacy release %+v", release)
	}
	step("ANSWER", transaction.SupervisorChange{Question: a.Supervision.QuestionID, Answer: "operator answer", AnswerRevision: a.TicketRevision})
	_, tree := s.commit(t, "path")
	step("DISPATCH", transaction.SupervisorChange{Stage: "implement", Holder: c.Holder, Worktree: s.root})
	step("BOOT", transaction.SupervisorChange{LeaderPID: 101, LeaderStarted: "resumed-leader"})
	step("STOPPING", transaction.SupervisorChange{})
	step("STOPPED", transaction.SupervisorChange{Clean: true, Session: "session-1", Tree: tree, ChangedPaths: []string{"path"}})
	step("DISPATCH", transaction.SupervisorChange{Stage: "review", Holder: "independent", Worktree: s.root})
	rr, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", claim.AttemptID+".json"))
	reviewAttempt, e := snapshot.DecodeAttempt(rr)
	if e != nil || reviewAttempt.PoolAllocation == nil || reviewAttempt.PoolAllocation.MemberID != "review" {
		t.Fatalf("review health allocation %+v %v", reviewAttempt, e)
	}
	step("BOOT", transaction.SupervisorChange{LeaderPID: 102, LeaderStarted: "review-leader"})
	step("STOPPING", transaction.SupervisorChange{})
	step("STOPPED", transaction.SupervisorChange{Clean: true, Session: "session-2", Holder: "independent", Tree: tree, Accepted: true, Claims: []string{string(wire.Sum([]byte("it exists")))}})
	target := gitOut(t, s.root, "rev-parse", "HEAD")
	index := gitOut(t, s.root, "write-tree")
	refusal := func(request, action, code string) {
		t.Helper()
		r, e := store.SupervisorTransition(context.Background(), s.repo, operator(), "queue:acme:main", request, claim.AttemptID, claim.Generation, transaction.SupervisorChange{Action: action, ProgramID: p.ID, OwnerPID: p.OwnerPID, OwnerStarted: p.OwnerStarted})
		if e != nil || !r.Outcome.HasCode(code) {
			t.Fatalf("%s %+v %v", action, r, e)
		}
		if gitOut(t, s.root, "rev-parse", "HEAD") != target || gitOut(t, s.root, "write-tree") != index {
			t.Fatal("refusal mutated integration checkout")
		}
	}
	refusal("missing-ready", "READY", wire.CodeMissingGate)
	gate, e := store.GateRun(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: "queue:acme:main", RequestID: "failed-gate", Root: s.root, Lease: gateOf(claim, "verify")}, s.root, time.Now)
	if e != nil || gate.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("gate %+v %v", gate, e)
	}
	refusal("failed-ready", "READY", wire.CodeGateFailed)
	refusal("failed-integrate", "INTEGRATE_INTENT", wire.CodeGateFailed)

}

func TestSupervisorSharedProgramThreeLaneCap(t *testing.T) {
	s := newLeaseStore(t)
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str("3"))
	digest := string(wire.Sum(nil))
	v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(digest), "fileSha256", str(digest), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER"}), "maxWorkers", str("3"), "enabled", wire.Bool(true))))
	v.Obj.Set("supervision", obj("profile", str(snapshot.SupervisedProfile), "contextRequired", wire.Bool(true), "maxRepairCycles", str("2"), "program", obj("turns", str("2"), "wallClockMinutes", str("10"), "inputTokens", str("100000"), "outputTokens", str("100000"))))
	r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("shared-policy", "2", wire.EncodeFile(v)), now(t))
	if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy %+v %v", r, e)
	}
	ps := []snapshot.Program{}
	for _, id := range []string{"lane-a", "lane-b", "lane-c"} {
		p := snapshot.Program{ID: id, Group: "shared", StartedAt: time.Now().UTC().Format(time.RFC3339), UsageKnown: true, Profile: snapshot.SupervisedProfile, OwnerPID: 99, OwnerStarted: "test-owner", Epoch: 1, ConfigSHA256: digest, Phase: "ADMITTED", Base: "0123456789012345678901234567890123456789", Quiescence: "PROVED"}
		for _, phase := range []string{"ADMITTED", "WORKTREE_ADD", "READY"} {
			p.Phase = phase
			r, e = store.ProgramTransition(context.Background(), s.repo, operator(), "queue:acme:main", id+phase, p)
			if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("%s %+v %v", phase, r, e)
			}
		}
		ps = append(ps, p)
	}
	results := make(chan *store.Report, 3)
	errs := make(chan error, 3)
	for _, p := range ps {
		go func(p snapshot.Program) {
			p.Phase = "SPAWNING"
			p.Turns = 1
			r, e := store.ProgramTransition(context.Background(), s.repo, operator(), "queue:acme:main", p.ID+"spawn", p)
			results <- r
			errs <- e
		}(p)
	}
	passed, limited := 0, 0
	for range ps {
		r := <-results
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
		if r.Outcome.Outcome == mutation.OutcomeCompleted {
			passed++
		} else if r.Outcome.HasCode(wire.CodeLimitExceeded) {
			limited++
		} else {
			t.Fatalf("unexpected %+v", r)
		}
	}
	if passed != 2 || limited != 1 {
		t.Fatalf("shared cap admitted=%d refused=%d", passed, limited)
	}
}

func TestSupervisorStaleIntentFencesContinuation(t *testing.T) {
	for _, changed := range []string{"usage", "policy"} {
		t.Run(changed, func(t *testing.T) {
			s := newLeaseStore(t)
			v := fixture.PolicyValue()
			v.Obj.Set("policyVersion", str("3"))
			v.Obj.Set("gates", wire.Array(commandGate("verify", "exit 3", "10", true)))
			health := obj("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3"))
			v.Obj.Set("pools", wire.Array(obj("id", str("db"), "members", wire.Strings([]string{"a", "b", "review"}), "reservedFor", obj("review", str("review")), "memberConfig", obj("a", obj("health", health), "b", obj("health", health), "review", obj("health", health)))))
			b, _ := v.Obj.Get("budgets")
			b.Obj.Set("requireEnforcedFields", wire.Array())
			lane, _ := b.Obj.Get("lane")
			lane.Obj.Set("inputTokens", str("0"))
			if changed == "usage" {
				lane.Obj.Set("inputTokens", str("100"))
			}
			lane.Obj.Set("outputTokens", str("0"))
			digest := string(wire.Sum(nil))
			v.Obj.Set("runtimes", wire.Array(obj("runtimeId", str(snapshot.SupervisedProfile), "executable", obj("pathSha256", str(digest), "fileSha256", str(digest), "mode", str("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", str(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER", "REVIEWER"}), "maxWorkers", str("1"), "enabled", wire.Bool(true))))
			report, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("supervised-policy", "2", wire.EncodeFile(v)), now(t))
			if e != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("policy %+v %v", report, e)
			}
			id := s.ticket(t, "supervised")
			c := claimOf(id, "path")
			c.Stage = "implement"
			claim := s.lease(t, "supervised-claim", c, 0, nil)
			p := snapshot.Program{CurrentAttempt: claim.AttemptID, CurrentGeneration: string(claim.Generation), Assignment: 1, ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 99, OwnerStarted: "observed-test-identity", Epoch: 1, ConfigSHA256: digest, Phase: "ADMITTED", Base: "0123456789012345678901234567890123456789", Worktree: "/fixture"}
			report, e = store.ProgramTransition(context.Background(), s.repo, operator(), "queue:acme:main", "program-admit", p)
			if e != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("program %+v %v", report, e)
			}
			stepNumber := 0
			step := func(action string, f transaction.SupervisorChange) {
				t.Helper()
				stepNumber++
				f.Action = action
				f.ProgramID = p.ID
				f.OwnerPID = p.OwnerPID
				f.OwnerStarted = p.OwnerStarted
				r, e := store.SupervisorTransition(context.Background(), s.repo, operator(), "queue:acme:main", fmt.Sprintf("step-%s-%d", action, stepNumber), claim.AttemptID, claim.Generation, f)
				if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("%s %+v %v", action, r, e)
				}
			}
			step("ATTACH", transaction.SupervisorChange{})

			step("DISPATCH", transaction.SupervisorChange{Stage: "implement", Holder: c.Holder, Worktree: s.root})
			step("BOOT", transaction.SupervisorChange{LeaderPID: 100, LeaderStarted: "leader"})
			if changed == "policy" {
				v.Obj.Set("policyVersion", str("4"))
				r, e := store.PolicyUpdate(context.Background(), s.repo, operator(), policyRequest("change-policy", "3", wire.EncodeFile(v)), now(t))
				if e != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("policy %+v %v", r, e)
				}
			}

			// Safety completion stays available after governing intent changes.
			step("STOPPING", transaction.SupervisorChange{})
			step("STOPPED", transaction.SupervisorChange{Clean: true, Session: "s", Question: "old question"})
			raw, _ := os.ReadFile(filepath.Join(s.repo.StateDir, "attempts", claim.AttemptID+".json"))
			a, e := snapshot.DecodeAttempt(raw)
			if e != nil {
				t.Fatal(e)
			}
			code := wire.CodeStalePolicy
			actions := []string{"ANSWER", "DISPATCH", "GRANT"}
			if changed == "usage" {
				step("ANSWER", transaction.SupervisorChange{Question: a.Supervision.QuestionID, Answer: "continue", AnswerRevision: a.TicketRevision})
				code = wire.CodeMissingEvidence
				actions = []string{"DISPATCH"}
			}

			for _, action := range actions {
				r, e := store.SupervisorTransition(context.Background(), s.repo, operator(), "queue:acme:main", "stale-"+action, claim.AttemptID, claim.Generation, transaction.SupervisorChange{Action: action, ProgramID: p.ID, OwnerPID: p.OwnerPID, OwnerStarted: p.OwnerStarted, Question: a.Supervision.QuestionID, Answer: "old answer", AnswerRevision: a.TicketRevision, Stage: "implement", Holder: c.Holder, Worktree: s.root})
				if e != nil || !r.Outcome.HasCode(code) {
					t.Fatalf("%s %+v %v", action, r, e)
				}
			}
		})
	}
}
