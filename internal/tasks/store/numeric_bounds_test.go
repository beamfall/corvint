//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-035/037: a persisted Size must not alias a native PID. The store
// and canonical receipt chain are admitted before the identity spy is used.
func TestV13179_SupervisorHeartbeatLeaderPIDRange(t *testing.T) {
	f := newResumeFixture(t)
	f.now = WallClock()
	released := stagedWrite(f.lease("numeric-release-seed", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation}), nil)
	if !released.completed() {
		t.Fatalf("release seed: %+v %v", released.rep, released.err)
	}
	ctx := context.Background()
	digest := string(wire.Sum(nil))
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("3"))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Array())
	lane, _ := budgets.Obj.Get("lane")
	lane.Obj.Set("inputTokens", wire.String("0"))
	lane.Obj.Set("outputTokens", wire.String("0"))
	v.Obj.Set("runtimes", wire.Array(historyObject("runtimeId", wire.String(snapshot.SupervisedProfile), "executable", historyObject("pathSha256", wire.String(digest), "fileSha256", wire.String(digest), "mode", wire.String("0755")), "argvPrefix", wire.Array(), "capabilityProfileSha256", wire.String(digest), "observedBudgetFields", wire.Array(), "roles", wire.Strings([]string{"BUILDER"}), "maxWorkers", wire.String("1"), "enabled", wire.Bool(true))))
	report, err := PolicyUpdate(ctx, f.repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "numeric-policy", ExpectedPolicyVersion: "2", Policy: wire.EncodeFile(v)}, WallClock())
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
	created := stagedWrite(writerCreate(f.repo, "numeric-ticket", WallClock()), nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	attemptEntropies.Store(f.repo.StateDir, fixedEntropy(12))
	f.now = WallClock()
	claimed := stagedWrite(f.lease("numeric-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: "agent-1", LeaseMinutes: "60", Scope: []string{"numeric/"}, Stage: "implement"}), nil)
	if !claimed.completed() {
		t.Fatalf("claim: %+v %v", claimed.rep, claimed.err)
	}
	f.claim = claimed.rep
	p := snapshot.Program{CurrentAttempt: f.claim.AttemptID, CurrentGeneration: string(f.claim.Generation), Assignment: 1, ID: "numeric-program", Profile: snapshot.SupervisedProfile, OwnerPID: 99, OwnerStarted: "owner-identity", Epoch: 1, ConfigSHA256: digest, Phase: "ADMITTED", Base: "0123456789012345678901234567890123456789", Worktree: f.root}
	report, err = ProgramTransition(ctx, f.repo, historyActor, fixture.QueueID, "numeric-program", p)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("program: %+v %v", report, err)
	}
	step := func(id, action string, change transaction.SupervisorChange, generation wire.Size) (*Report, error) {
		change.Action = action
		change.ProgramID = p.ID
		change.OwnerPID = p.OwnerPID
		change.OwnerStarted = p.OwnerStarted
		return SupervisorTransition(ctx, f.repo, historyActor, fixture.QueueID, id, f.claim.AttemptID, generation, change)
	}
	for _, c := range []struct {
		action string
		change transaction.SupervisorChange
	}{
		{"ATTACH", transaction.SupervisorChange{}},
		{"DISPATCH", transaction.SupervisorChange{Stage: "implement", Holder: "agent-1", Worktree: f.root}},
		{"BOOT", transaction.SupervisorChange{LeaderPID: 100, LeaderStarted: "leader-identity"}},
	} {
		report, err = step("numeric-"+c.action, c.action, c.change, f.claim.Generation)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %+v %v", c.action, report, err)
		}
	}
	proof, err := readLeaseProof(ctx, f.repo)
	if err != nil {
		t.Fatalf("initialized fixture audit: %v", err)
	}
	good, ok := lockedAttempt(proof, f.claim.AttemptID)
	if !ok || good.Lane == nil || good.Supervision == nil {
		t.Fatal("initialized fixture missing lane")
	}
	previous := recordedProcessIdentity
	t.Cleanup(func() { recordedProcessIdentity = previous })
	var calls []int
	recordedProcessIdentity = func(pid int) (string, error) {
		calls = append(calls, pid)
		if pid == 99 {
			return "owner-identity", nil
		}
		if pid == 100 {
			return "leader-identity", nil
		}
		return "unexpected-identity", nil
	}
	report, err = step("numeric-valid-heartbeat", "HEARTBEAT", transaction.SupervisorChange{}, f.claim.Generation)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || !reflect.DeepEqual(calls, []int{99, 100}) {
		t.Fatalf("valid heartbeat: %+v %v calls=%v", report, err, calls)
	}
	appendAttempt := func(raw []byte, id string) {
		t.Helper()
		requestPath, err := snapshot.RequestPath(id)
		if err != nil {
			t.Fatal(err)
		}
		historyAppend(t, f.repo, map[string][]byte{
			"attempts/" + good.AttemptID + ".json": raw,
			requestPath:                            historyRequest(id, headSeqOf(t, f.repo)+1),
		}, id)
	}
	widePID := uint64(4294967297)
	widePIDCalls := []int{99}
	if widePID <= uint64(^uint(0)>>1) {
		widePIDCalls = append(widePIDCalls, int(widePID))
	}
	// Synthesize canonical persisted afterimages, preserving a valid receipt
	// chain. Each case audits before testing lookup and write side effects.
	for i, c := range []struct {
		name             string
		pid              uint64
		absent, mismatch bool
		wantCalls        []int
		code             string
	}{
		{"zero", 0, false, false, []int{99}, wire.CodeSupervisorLost},
		{"native overflow", uint64(^uint(0)>>1) + 1, false, false, []int{99}, wire.CodeSupervisorLost},
		{"uint64 maximum", ^uint64(0), false, false, []int{99}, wire.CodeSupervisorLost},
		{"wide positive alias", widePID, false, false, widePIDCalls, wire.CodeSupervisorLost},
		{"absent lane", 100, true, false, nil, wire.CodeFenced},
		{"identity mismatch", 100, false, true, []int{99, 100}, wire.CodeSupervisorLost},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := *good
			lane := *good.Lane
			supervision := *good.Supervision
			a.Lane = &lane
			a.Supervision = &supervision
			a.Lane.LeaderPid = wire.SizeOf(c.pid)
			if c.absent {
				a.Lane = nil
			}
			if c.mismatch {
				a.Supervision.LeaderStarted = "other-identity"
			}
			raw, err := a.Encode()
			if err != nil {
				t.Fatalf("fixture encode: %v", err)
			}
			appendAttempt(raw, fmt.Sprintf("numeric-inject-%d", i))
			admitted, err := readLeaseProof(ctx, f.repo)
			if err != nil {
				t.Fatalf("injected fixture audit: %v", err)
			}
			decoded, ok := lockedAttempt(admitted, a.AttemptID)
			if !ok || (!c.absent && decoded.Lane.LeaderPid != a.Lane.LeaderPid) {
				t.Fatal("injection was not admitted")
			}
			before := fixture.TreeSnapshot(t, f.repo.StateDir)
			calls = nil
			report, err = step(fmt.Sprintf("numeric-refuse-%d", i), "HEARTBEAT", transaction.SupervisorChange{}, f.claim.Generation)
			if wire.CodeOf(err) != c.code || report != nil || !reflect.DeepEqual(calls, c.wantCalls) {
				t.Fatalf("refusal: %+v %v calls=%v want=%v", report, err, calls, c.wantCalls)
			}
			if !fixture.SameTree(before, fixture.TreeSnapshot(t, f.repo.StateDir)) {
				t.Fatal("refused heartbeat mutated state")
			}
		})
	}
	// Restore a valid canonical record before checking generation fencing.
	raw, err := good.Encode()
	if err != nil {
		t.Fatal(err)
	}
	appendAttempt(raw, "numeric-restore")
	report, err = step("numeric-stale-generation", "HEARTBEAT", transaction.SupervisorChange{}, wire.SizeOf(f.claim.Generation.Uint64()+1))
	if err != nil || report == nil || !report.Outcome.HasCode(wire.CodeFenced) {
		t.Fatalf("generation: %+v %v", report, err)
	}
}
