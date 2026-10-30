package transaction

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

// PSR-V0-003/008: at the durable before-confirm barrier only the exact owner and predecessor can free it.
func TestPSRBeforeConfirmOwnerPredecessor(t *testing.T) {
	for _, mode := range []string{"exact", "manual-owner", "wrong-predecessor", "after-reset"} {
		t.Run(mode, func(t *testing.T) {
			q, e := intent.DecodeQueue(fixture.QueueBytes())
			if e != nil {
				t.Fatal(e)
			}
			pv := fixture.PolicyValue()
			pv.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a"})))))
			p, e := intent.DecodePolicy(wire.EncodeFile(pv))
			if e != nil {
				t.Fatal(e)
			}
			p.Pools = []intent.Pool{{ID: "db", Members: []string{"a"}, MemberConfig: map[string]intent.MemberConfig{"a": {SafeReuse: &intent.SafeReuse{}}}}}
			d := wire.Sum(nil)
			revision := strings.Repeat("1", 40)
			tree := strings.Repeat("2", 40)
			x := wire.Count("0")
			o := snapshot.PoolSweepObservation{AllocationID: d, DefinitionSha256: p.MemberDefinition("db", "a"), Owner: d, Log: d, Environment: d, EnvFile: d, Phase: "verify", Attempt: "1", Revision: revision, Tree: tree, Class: "EXIT_ZERO", Exit: &x, Passed: true, GroupClean: true, Stdout: d, Stderr: d, Timing: snapshot.PoolSweepTiming{StartedAt: "1", Deadline: "2", WaitReturnedAt: "2", CleanupEndedAt: "3", ExecutionMillis: "0", CleanupMillis: "0", CleanupAllowanceMillis: "5000"}}
			raw := o.Encode()
			if _, e = snapshot.DecodePoolSweepObservation(raw); e != nil {
				t.Fatal(e)
			}
			last := wire.Sum(raw)
			en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: d, DefinitionSha256: o.DefinitionSha256, AllocatedSeq: "1"}, State: "CLEANING", Holder: "runner", Stage: "implement", Generation: "0", ChangedSeq: "1", PolicySha256: d, RequestSha256: d, ObservationSha256: &last, RunnerPID: "1", RunnerStarted: "identity", CommandKind: "sweep", CommandRevision: revision, Sweep: &snapshot.PoolSweepOwner{RequestSha256: d, Previous: &last, Phase: "confirm", Attempt: "1", Tree: tree}}
			inv, e := NewInventory([]archive.FileEntry{{Path: "evidence/" + string(last), Sha256: last, Bytes: wire.SizeOf(uint64(len(raw)))}}, []string{"evidence"})
			if e != nil {
				t.Fatal(e)
			}
			c := leaseContext{l: &LeaseRequest{Evidence: string(d)}, in: Input{Inventory: inv, LeaseFacts: LeaseFacts{Pool: PoolFacts{Observation: raw}}}, st: inputState{policy: p, pools: &snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{en}}}, seq: "2"}
			switch mode {
			case "manual-owner":
				c.l.Evidence = "manual"
			case "wrong-predecessor":
				other := wire.Sum([]byte("other"))
				en.Sweep.Previous = &other
			case "after-reset":
				en.Sweep.Phase = "verify"
			}
			result := planPoolSweepSafe(c, &en)
			if (result.effect != nil && result.effect.outcome == mutation.OutcomeCompleted) != (mode == "exact") {
				t.Fatalf("%s %+v", mode, result)
			}
			if mode == "exact" {
				state, e := snapshot.DecodePools(result.posts["pools.json"])
				if e != nil || len(state.Entries) != 0 {
					t.Fatal(state, e)
				}
			} else if result.result == nil || len(result.posts) != 0 {
				t.Fatal("refusal mutated pools")
			}
		})
	}
}

func TestPSRAggregateCapacityBeforeOwnership(t *testing.T) {
	d := wire.Sum(nil)
	for _, tc := range []struct {
		name          string
		n             int
		escaped, want bool
	}{{"ascii-256", 256, false, true}, {"escaped-256", 256, true, false}, {"escaped-200", 200, true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			selections := []PoolSweepSelection{}
			rows := []wire.Value{}
			for i := 0; i < tc.n; i++ {
				suffix := strings.Repeat("x", 61)
				if tc.escaped {
					suffix = strings.Repeat("\"", 61)
				}
				member := fmt.Sprintf("%03d", i) + suffix
				if _, e := wire.ParseLabel("member", member); e != nil {
					t.Fatal("valid member fixture", e)
				}
				selections = append(selections, PoolSweepSelection{Pool: "db", Member: member, Allocation: d, Definition: d})
				rows = append(rows, wire.ObjectValue(wire.NewObject().Set("member", wire.String(member)).Set("allocation", wire.String(string(d))).Set("free", wire.Bool(false)).Set("observation", wire.String(string(d)))))
			}
			raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-pool-sweep-result/0")).Set("owner", wire.String(string(d))).Set("members", wire.Array(rows...))))
			if _, e := wire.Parse(raw); e != nil {
				t.Fatal("canonical aggregate fixture", e)
			}
			e := CheckPoolSweepResultCapacity(d, selections)
			if (e == nil) != tc.want {
				t.Fatalf("capacity bytes%d err%v", len(raw), e)
			}
			if !tc.want {
				l := &LeaseRequest{Verb: LeasePoolSweep, SweepSeconds: "30"}
				c := leaseContext{r: Request{Operation: Lease, QueueID: fixture.QueueID, RequestID: "capacity", Actor: mutation.Binding{ID: "owner", Role: "OWNER"}, Lease: l}, l: l, in: Input{LeaseFacts: LeaseFacts{Pool: PoolFacts{RunnerPID: "1", RunnerStarted: "fixture", Revision: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40), SweepSelections: selections}}}, st: inputState{pools: &snapshot.PoolState{}}}
				result := planPoolSweep(c)
				if len(result.posts) != 0 || result.effect != nil || result.result == nil {
					t.Fatal("oversized aggregate acquired ownership")
				}
			} else {
				inv, e := NewInventory([]archive.FileEntry{{Path: "evidence/" + string(d), Sha256: d, Bytes: "1"}}, []string{"evidence"})
				if e != nil {
					t.Fatal(e)
				}
				c := leaseContext{l: &LeaseRequest{Evidence: string(d)}, in: Input{Inventory: inv, LeaseFacts: LeaseFacts{Pool: PoolFacts{SweepResult: raw}}}, st: inputState{pools: &snapshot.PoolState{}}}
				result := planPoolSweepFinish(c)
				if result.effect == nil || result.effect.outcome != mutation.OutcomeCompleted {
					t.Fatal("supported aggregate refused", result)
				}
				if !bytes.Equal(result.posts["evidence/"+string(wire.Sum(raw))], raw) {
					t.Fatal("aggregate changed")
				}
			}
			t.Logf("canonical aggregate %d members %d bytes admitted=%v", tc.n, len(raw), tc.want)
		})
	}
}

// Exercise the actual observation reducer and frozen writer material, not a
// hand-written descriptor. A large projection adds exactly one evidence blob.
func TestPSRMaxStageMaterial(t *testing.T) {
	for _, large := range []bool{false, true} {
		t.Run(fmt.Sprint("large=", large), func(t *testing.T) {
			q, e := intent.DecodeQueue(fixture.QueueBytes())
			if e != nil {
				t.Fatal(e)
			}
			pv := fixture.PolicyValue()
			pv.Obj.Set("pools", wire.Array(object("id", s("db"), "members", wire.Strings([]string{"a"}))))
			policy, e := intent.DecodePolicy(wire.EncodeFile(pv))
			if e != nil {
				t.Fatal(e)
			}
			policy.Pools[0].MemberConfig["a"] = intent.MemberConfig{SafeReuse: &intent.SafeReuse{MaxAttempts: "2"}}
			d := wire.Sum(nil)
			rev := strings.Repeat("a", 40)
			tree := strings.Repeat("b", 40)
			zero := wire.Count("0")
			log := snapshot.EncodePoolSweepLog(bytes.Repeat([]byte("x"), 65536), nil)
			if len(log) != 65540 {
				t.Fatal("maximum log fixture", len(log))
			}
			observation := snapshot.PoolSweepObservation{AllocationID: d, DefinitionSha256: policy.MemberDefinition("db", "a"), Owner: d, Log: wire.Sum(log), Environment: d, EnvFile: d, Phase: "reset", Attempt: "1", Revision: rev, Tree: tree, Class: "EXIT_ZERO", Exit: &zero, Passed: true, GroupClean: true, Stdout: wire.Sum(bytes.Repeat([]byte("x"), 65536)), Stderr: wire.Sum(nil), Timing: snapshot.PoolSweepTiming{StartedAt: "1", Deadline: "2", WaitReturnedAt: "2", CleanupEndedAt: "3", ExecutionMillis: "0", CleanupMillis: "0", CleanupAllowanceMillis: "5000"}}
			raw := observation.Encode()
			if _, e = snapshot.DecodePoolSweepObservation(raw); e != nil {
				t.Fatal(e)
			}
			en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: d, DefinitionSha256: observation.DefinitionSha256, AllocatedSeq: "1"}, State: "CLEANING", Holder: "runner", Stage: "implement", Generation: "0", ChangedSeq: "1", PolicySha256: d, RequestSha256: d, RunnerPID: "1", RunnerStarted: "identity", CommandKind: "sweep", CommandRevision: rev, Sweep: &snapshot.PoolSweepOwner{RequestSha256: d, Phase: "reset", Attempt: "1", Tree: tree}}
			state := &snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{en}}
			if large {
				for i := 1; i < 200; i++ {
					other := en
					other.MemberID = fmt.Sprintf("%03d", i) + strings.Repeat("z", 61)
					other.Reason = strings.Repeat("r", 4096)
					state.Entries = append(state.Entries, other)
				}
			}
			prior, e := state.Encode()
			if e != nil {
				t.Fatal("canonical fixture", e)
			}
			initial, _ := initialized(t)
			inv := initial.Inventory.clone()
			inv.files["pools.json"] = archive.FileEntry{Path: "pools.json", Sha256: wire.Sum(prior), Bytes: wire.SizeOf(uint64(len(prior)))}

			request := Request{Operation: Lease, QueueID: q.QueueID.Raw, RequestID: strings.Repeat(`"`, 64), Actor: mutation.Binding{ID: strings.Repeat(`"`, 64), Role: "OWNER"}, Lease: &LeaseRequest{Verb: LeasePoolObserve, Member: "a", Allocation: string(d), Evidence: string(d)}}
			context := leaseContext{r: request, l: request.Lease, in: Input{Inventory: inv, LeaseFacts: LeaseFacts{Pool: PoolFacts{Observation: raw, SweepLog: log}}}, st: inputState{policy: policy, pools: state}, seq: "2"}
			result := planPoolSweepObserve(context, &en)
			if result.effect == nil || result.result != nil {
				t.Fatalf("observe %+v", result)
			}
			base, e := snapshot.DecodeHead(initial.Head)
			if e != nil {
				t.Fatal(e)
			}
			digest, e := Digest(request)
			if e != nil {
				t.Fatal(e)
			}
			plan, _, e := freeze(request, digest, timestamp, inv, base, result.posts, nil, nil, result.effect)
			if e != nil {
				t.Fatal(e)
			}
			want := 6
			if large {
				want = 7
			}
			if len(plan.Artifacts()) != want || len(plan.Descriptor()) > 2658 {
				t.Fatalf("material %d artifacts %d bytes", len(plan.Artifacts()), len(plan.Descriptor()))
			}
			if _, e = snapshot.DecodeStageDescriptor(plan.Descriptor()); e != nil {
				t.Fatal(e)
			}
			if _, e = CheckCapacity(plan); e != nil {
				t.Fatal(e)
			}
			stage := stageObservation(plan)
			for _, a := range plan.Artifacts() {
				stage.Files = append(stage.Files, StageFile{Name: a.Slot, Type: "REGULAR", Data: a.Data})
			}
			if got, e := ClassifyStage(stage, plan); e != nil || got.Kind != "FullPlanPrepared" {
				t.Fatal(got, e)
			}
			t.Logf("actual StageLease artifacts=%d descriptor=%d pool=%d log=%d", len(plan.Artifacts()), len(plan.Descriptor()), len(result.posts["pools.json"]), len(log))
			// Identical content-addressed log already present consumes no new path.
			inv.files["evidence/"+string(wire.Sum(log))] = archive.FileEntry{Path: "evidence/" + string(wire.Sum(log)), Sha256: wire.Sum(log), Bytes: wire.SizeOf(uint64(len(log)))}
			same, _, e := freeze(request, digest, timestamp, inv, base, result.posts, nil, nil, result.effect)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = CheckCapacity(same); e != nil {
				t.Fatal(e)
			}
			// Overflow is rejected by the actual observation reducer before posts exist.
			context.in.LeaseFacts.Pool.SweepLog = append(bytes.Clone(log), 0)
			over := observation
			over.Log = wire.Sum(context.in.LeaseFacts.Pool.SweepLog)
			context.in.LeaseFacts.Pool.Observation = over.Encode()
			en.Sweep = &snapshot.PoolSweepOwner{RequestSha256: d, Phase: "reset", Attempt: "1", Tree: tree}
			if bad := planPoolSweepObserve(context, &en); bad.effect != nil || len(bad.posts) != 0 {
				t.Fatal("overflow manufactured posts")
			}
		})
	}
}

// PSR-V0-008: under an ALL barrier no further phase command is authorized; only an
// owned successful verify may still advance to its exact delegated confirmation.
func TestPSRAllBarrierObserveNoNextPhase(t *testing.T) {
	for _, tc := range []struct {
		name, scope, phase, class, wantState, wantPhase string
	}{
		{"reset-pass-open", "", "reset", "EXIT_ZERO", "CLEANING", "verify"},
		{"reset-pass-admission", "ADMISSION", "reset", "EXIT_ZERO", "CLEANING", "verify"},
		{"reset-pass-all", "ALL", "reset", "EXIT_ZERO", "QUARANTINED", ""},
		{"reset-retry-open", "", "reset", "EXIT_NONZERO", "CLEANING", "reset"},
		{"reset-retry-all", "ALL", "reset", "EXIT_NONZERO", "QUARANTINED", ""},
		{"verify-pass-all", "ALL", "verify", "EXIT_ZERO", "CLEANING", "confirm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, e := intent.DecodeQueue(fixture.QueueBytes())
			if e != nil {
				t.Fatal(e)
			}
			pv := fixture.PolicyValue()
			pv.Obj.Set("pools", wire.Array(object("id", s("db"), "members", wire.Strings([]string{"a"}))))
			policy, e := intent.DecodePolicy(wire.EncodeFile(pv))
			if e != nil {
				t.Fatal(e)
			}
			policy.Pools[0].MemberConfig["a"] = intent.MemberConfig{SafeReuse: &intent.SafeReuse{MaxAttempts: "2", ExpectExit: "0"}}
			d := wire.Sum(nil)
			rev, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
			exit := wire.Count("0")
			if tc.class == "EXIT_NONZERO" {
				exit = "1"
			}
			log := snapshot.EncodePoolSweepLog(nil, nil)
			o := snapshot.PoolSweepObservation{AllocationID: d, DefinitionSha256: policy.MemberDefinition("db", "a"), Owner: d, Log: wire.Sum(log), Environment: d, EnvFile: d, Phase: tc.phase, Attempt: "1", Revision: rev, Tree: tree, Class: tc.class, Exit: &exit, Passed: tc.class == "EXIT_ZERO", GroupClean: true, Stdout: d, Stderr: d, Timing: snapshot.PoolSweepTiming{StartedAt: "1", Deadline: "2", WaitReturnedAt: "2", CleanupEndedAt: "3", ExecutionMillis: "0", CleanupMillis: "0", CleanupAllowanceMillis: "5000"}}
			raw := o.Encode()
			if _, e = snapshot.DecodePoolSweepObservation(raw); e != nil {
				t.Fatal(e)
			}
			en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: d, DefinitionSha256: o.DefinitionSha256, AllocatedSeq: "1"}, State: "CLEANING", Holder: "runner", Stage: "implement", Generation: "0", ChangedSeq: "1", PolicySha256: d, RequestSha256: d, RunnerPID: "1", RunnerStarted: "identity", CommandKind: "sweep", CommandRevision: rev, Sweep: &snapshot.PoolSweepOwner{RequestSha256: d, Phase: tc.phase, Attempt: "1", Tree: tree}}
			var barrier *snapshot.Barrier
			if tc.scope != "" {
				barrier = &snapshot.Barrier{QueueID: q.QueueID, Scope: tc.scope, Reason: "OPERATOR"}
			}
			c := leaseContext{l: &LeaseRequest{Verb: LeasePoolObserve, Member: "a", Allocation: string(d), Evidence: string(d)}, in: Input{LeaseFacts: LeaseFacts{Pool: PoolFacts{Observation: raw, SweepLog: log}}}, st: inputState{policy: policy, barrier: barrier, pools: &snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{en}}}, seq: "2"}
			result := planPoolSweepObserve(c, &en)
			if result.effect == nil || result.effect.outcome != mutation.OutcomeCompleted {
				t.Fatalf("observe %+v", result)
			}
			state, e := snapshot.DecodePools(result.posts["pools.json"])
			if e != nil || len(state.Entries) != 1 {
				t.Fatal(state, e)
			}
			got := state.Entries[0]
			if got.State != tc.wantState {
				t.Fatalf("state %s want %s", got.State, tc.wantState)
			}
			if tc.wantPhase == "" {
				if got.Sweep != nil || !strings.Contains(got.Reason, "ALL barrier") {
					t.Fatalf("ALL left owner or unexplained reason %+v", got)
				}
			} else if got.Sweep == nil || got.Sweep.Phase != tc.wantPhase {
				t.Fatalf("phase %+v want %s", got.Sweep, tc.wantPhase)
			}
		})
	}
}

// PSR-V0-003: an original request whose phase never committed may end with the
// owner digest as a non-witness placeholder only after its owner was released.
func TestPSRFinishReleasedOwnerPlaceholder(t *testing.T) {
	owner := wire.Sum([]byte("owner"))
	d := wire.Sum(nil)
	for _, mode := range []string{"released", "held", "free-claim", "foreign-digest"} {
		t.Run(mode, func(t *testing.T) {
			free, observation := false, owner
			en := snapshot.PoolEntry{PoolAllocation: snapshot.PoolAllocation{PoolID: "db", MemberID: "a", AllocationID: d, DefinitionSha256: d, AllocatedSeq: "1"}, State: "QUARANTINED", Holder: "runner", Stage: "implement", Generation: "0", ChangedSeq: "1", PolicySha256: d, RequestSha256: d, RunnerPID: "0", CommandKind: "sweep"}
			switch mode {
			case "held":
				en.State = "CLEANING"
				en.Sweep = &snapshot.PoolSweepOwner{RequestSha256: owner, Phase: "reset", Attempt: "1", Tree: strings.Repeat("b", 40)}
			case "free-claim":
				free = true
			case "foreign-digest":
				observation = wire.Sum([]byte("unwitnessed"))
			}
			row := wire.ObjectValue(wire.NewObject().Set("member", wire.String("a")).Set("allocation", wire.String(string(d))).Set("free", wire.Bool(free)).Set("observation", wire.String(string(observation))))
			raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-pool-sweep-result/0")).Set("owner", wire.String(string(owner))).Set("members", wire.Array(row))))
			inv, e := NewInventory(nil, []string{"evidence"})
			if e != nil {
				t.Fatal(e)
			}
			c := leaseContext{r: Request{RequestID: "finish"}, l: &LeaseRequest{Verb: LeasePoolSweepFinish, Evidence: string(owner)}, in: Input{Inventory: inv, LeaseFacts: LeaseFacts{Pool: PoolFacts{SweepResult: raw}}}, st: inputState{pools: &snapshot.PoolState{Entries: []snapshot.PoolEntry{en}}}}
			result := planPoolSweepFinish(c)
			completed := result.effect != nil && result.effect.outcome == mutation.OutcomeCompleted
			if completed != (mode == "released") {
				t.Fatalf("%s completed=%v %+v", mode, completed, result.result)
			}
		})
	}
}
