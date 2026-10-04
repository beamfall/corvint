// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func refusalOf(t *testing.T, err error) (string, string) {
	t.Helper()
	var refusal *ProcessErrorV2
	if !errors.As(err, &refusal) {
		t.Fatalf("expected a process refusal, got %v", err)
	}
	return refusal.Outcome, refusal.Code
}

func expectRefusal(t *testing.T, err error, outcome, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s %s, got success", outcome, code)
	}
	if gotOutcome, gotCode := refusalOf(t, err); gotOutcome != outcome || gotCode != code {
		t.Fatalf("expected %s %s, got %v", outcome, code, err)
	}
}

func strPtr(s string) *string { return &s }

func expectedLogical(kind string, ordinal int, runnerState string) []LogicalProcessV2 {
	return []LogicalProcessV2{
		{Node: "proc/0/host-supervisor/default", Role: "host-supervisor", RunKind: "workflow", State: "outside-workload/live", BirthDistinctFrom: []string{}},
		{Node: "proc/0/workflow-root/default", Role: "workflow-root", RunKind: "workflow", Parent: strPtr("proc/0/host-supervisor/default"),
			State: "completed/exit:0", BirthDistinctFrom: []string{}},
		{Node: "proc/1/browser-main/main", Role: "browser-main", RunKind: kind, RunOrdinal: ordinal, Parent: strPtr("proc/1/runner/default"),
			State: "observed/exit:unknown", BirthDistinctFrom: []string{}},
		{Node: "proc/1/browser-renderer/sandboxed", Role: "browser-renderer", RunKind: kind, RunOrdinal: ordinal,
			Parent: strPtr("proc/1/browser-main/main"), State: "observed/exit:unknown", BirthDistinctFrom: []string{}},
		{Node: "proc/1/runner/default", Role: "runner", RunKind: kind, RunOrdinal: ordinal, Parent: strPtr("proc/0/workflow-root/default"),
			State: runnerState, BirthDistinctFrom: []string{}},
		{Node: "proc/1/server/default", Role: "server", RunKind: kind, RunOrdinal: ordinal, Parent: strPtr("proc/1/runner/default"),
			State: "incomplete/unreaped", BirthDistinctFrom: []string{}},
	}
}

func TestRawProcessProofDerivesLogicalGraph(t *testing.T) {
	f := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main"})
	result, err := f.verifyRaw()
	if err != nil {
		t.Fatal(err)
	}
	if want := expectedLogical("repeat", 0, "incomplete/unreaped"); !reflect.DeepEqual(result.logical, want) {
		t.Fatalf("logical graph:\n got %+v\nwant %+v", result.logical, want)
	}
	if result.references != 2 || result.cleanups != 1 || len(result.workload) != 5 {
		t.Fatalf("result counts: %+v", result)
	}
}

func TestVerifyProcessMintsBoundToken(t *testing.T) {
	w := newProcWorld(t)
	w.qualify(t)
	f := w.fixture(t, fixtureSpec{name: "exec-main"})
	proof := f.proofBytes()
	token, err := VerifyProcessV2(testContext(), w.admission, f.graph, w.policyBytes, w.qualificationBytes, proof, w.store)
	if err != nil {
		t.Fatal(err)
	}
	graphSHA, _ := GraphBindingSHA256V2(f.graph)
	ids := []string{"exec-main", graphSHA, w.policyRef.SHA256, w.admission.Qualification.SHA256, sha256Hex(proof)}
	if !token.ValidFor(ids[0], ids[1], ids[2], ids[3], ids[4]) {
		t.Fatal("token does not bind its own identities")
	}
	for i := range ids {
		wrong := append([]string{}, ids...)
		wrong[i] = sha256Hex([]byte("other"))
		if token.ValidFor(wrong[0], wrong[1], wrong[2], wrong[3], wrong[4]) {
			t.Fatalf("token validates with a wrong binding at %d", i)
		}
	}
	logical, err := token.LogicalProcesses()
	if err != nil {
		t.Fatal(err)
	}
	if want := expectedLogical("repeat", 0, "incomplete/unreaped"); !reflect.DeepEqual(logical, want) {
		t.Fatalf("logical graph:\n got %+v\nwant %+v", logical, want)
	}
	*logical[1].Parent = "tampered"
	logical[0].BirthDistinctFrom = append(logical[0].BirthDistinctFrom, "tampered")
	logical[2].State = "tampered"
	again, _ := token.LogicalProcesses()
	if want := expectedLogical("repeat", 0, "incomplete/unreaped"); !reflect.DeepEqual(again, want) {
		t.Fatal("LogicalProcesses does not return a deep copy")
	}
}

func TestZeroProcessTokenIsInvalid(t *testing.T) {
	var token VerifiedProcessesV2
	if token.ValidFor("", "", "", "", "") {
		t.Fatal("zero token validates")
	}
	_, err := token.LogicalProcesses()
	expectRefusal(t, err, "BLOCKED", "process-token-invalid")
}

func TestVerifyProcessAdmissionRefusals(t *testing.T) {
	w := newProcWorld(t)
	w.qualify(t)
	f := w.fixture(t, fixtureSpec{name: "exec-main"})
	proof := f.proofBytes()
	run := func(admission ProcessAdmissionV2, policy, qualification []byte, ctx context.Context) error {
		_, err := VerifyProcessV2(ctx, admission, f.graph, policy, qualification, proof, w.store)
		return err
	}
	cancelled, cancel := context.WithCancel(testContext())
	cancel()
	darwin := w.admission
	darwin.HostTuple.OS = "darwin"
	unqualified := w.admission
	unqualified.Qualification = ArtifactRefV2{}
	otherKernel := w.admission
	otherKernel.HostTuple.KernelRelease = "6.9.0-other"
	tests := []struct {
		name, outcome, code string
		err                 error
	}{
		{"cancelled", "BLOCKED", "process-verification-cancelled", run(w.admission, w.policyBytes, w.qualificationBytes, cancelled)},
		{"no reader", "BLOCKED", "process-artifact-unavailable", func() error {
			_, err := VerifyProcessV2(testContext(), w.admission, f.graph, w.policyBytes, w.qualificationBytes, proof, nil)
			return err
		}()},
		{"unsupported host", "BLOCKED", "process-host-unsupported", run(darwin, w.policyBytes, w.qualificationBytes, testContext())},
		{"policy bytes", "REJECTED", "process-policy-digest-mismatch", run(w.admission, append([]byte(" "), w.policyBytes...), w.qualificationBytes, testContext())},
		{"no qualification", "BLOCKED", "process-qualification-unavailable", run(unqualified, w.policyBytes, nil, testContext())},
		{"empty qualification bytes", "BLOCKED", "process-qualification-unavailable", run(w.admission, w.policyBytes, nil, testContext())},
		{"qualification bytes", "REJECTED", "process-qualification-invalid", run(w.admission, w.policyBytes, append([]byte(" "), w.qualificationBytes...), testContext())},
		{"admission mismatch", "REJECTED", "process-admission-mismatch", run(otherKernel, w.policyBytes, w.qualificationBytes, testContext())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectRefusal(t, tt.err, tt.outcome, tt.code) })
	}
}

// requalify rewrites the admitted qualification report through mutate.
func (w *procWorld) requalify(t *testing.T, mutate func(*ProcessQualificationV2)) {
	var report ProcessQualificationV2
	if err := json.Unmarshal(w.qualificationBytes, &report); err != nil {
		t.Fatal(err)
	}
	mutate(&report)
	w.qualificationBytes = mustJSON(t, report)
	w.admission.Qualification = w.store.put("qualification", w.qualificationBytes)
}

// adoptRuns gives c another case's runs and harness, keeping its own ID.
func adoptRuns(c *QualificationCaseV2, from QualificationCaseV2) {
	c.GraphBindings, c.Proofs, c.HarnessInvocation, c.HarnessStdout = from.GraphBindings, from.Proofs, from.HarnessInvocation, from.HarnessStdout
}

func caseIndex(id string) int {
	for i, c := range qualificationCaseIDs {
		if c == id {
			return i
		}
	}
	return -1
}

func TestQualificationRefusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *procWorld, *ProcessQualificationV2)
	}{
		{"missing case", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) { q.Cases = q.Cases[1:] }},
		{"duplicate case", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) { q.Cases[1] = q.Cases[0] }},
		{"other host", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) { q.Host.KernelRelease = "other" }},
		{"other policy", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			q.PolicySHA256 = sha256Hex([]byte("other"))
		}},
		{"case policy", func(_ *testing.T, w *procWorld, q *ProcessQualificationV2) {
			q.Cases[0].Policy = w.store.put("other-policy", append([]byte(" "), w.policyBytes...))
		}},
		{"pair count", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			c := &q.Cases[caseIndex("two-fresh-births")]
			c.GraphBindings, c.Proofs = c.GraphBindings[:1], c.Proofs[:1]
		}},
		{"negative control accepted", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			adoptRuns(&q.Cases[caseIndex("pid-reuse-refused")], q.Cases[caseIndex("owned-launch-joins")])
		}},
		{"negative control wrong code", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			adoptRuns(&q.Cases[caseIndex("pid-reuse-refused")], q.Cases[caseIndex("roles-ambiguous-refused")])
		}},
		{"positive control refused", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			adoptRuns(&q.Cases[caseIndex("owned-launch-joins")], q.Cases[caseIndex("pid-reuse-refused")])
		}},
		{"same births are not fresh", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			adoptRuns(&q.Cases[caseIndex("two-fresh-births")], q.Cases[caseIndex("schedule-order-preserved")])
		}},
		{"uninterrupted grandchild", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			adoptRuns(&q.Cases[caseIndex("cleanup-grandchild-interruption")], q.Cases[caseIndex("owned-launch-joins")])
		}},
		{"witness proofs identical", func(_ *testing.T, _ *procWorld, q *ProcessQualificationV2) {
			c := &q.Cases[caseIndex("role-witness-order-invariant")]
			c.Proofs[1] = c.Proofs[0]
		}},
		{"harness not supervisor", func(t *testing.T, w *procWorld, q *ProcessQualificationV2) {
			c := &q.Cases[0]
			harness := InvocationV2{Profile: "postmerge-owned-invocation/2", ExecutionID: "q-owned", Purpose: "observer",
				Executable: w.exe["observer"], Argv: []string{"observer"}, Environment: []EnvironmentV2{}, StdinSHA256: noArtifact.SHA256}
			c.HarnessInvocation = w.store.put("q/other-harness", mustJSON(t, harness))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newProcWorld(t)
			w.qualify(t)
			w.requalify(t, func(q *ProcessQualificationV2) { tt.mutate(t, w, q) })
			err := verifyQualificationV2(testContext(), w.admission, w.policy, w.policyRef.SHA256, w.qualificationBytes, w.store)
			expectRefusal(t, err, "REJECTED", "process-qualification-invalid")
		})
	}
	t.Run("case artifact unavailable", func(t *testing.T) {
		w := newProcWorld(t)
		w.qualify(t)
		delete(w.store.data, "q/owned-launch-joins/stdout")
		err := verifyQualificationV2(testContext(), w.admission, w.policy, w.policyRef.SHA256, w.qualificationBytes, w.store)
		expectRefusal(t, err, "BLOCKED", "process-artifact-unavailable")
	})
}

func (f *procFixture) restat(index int, proc string, state byte, ppid int, start uint64) {
	p := f.procs[proc]
	f.proof.Captures[index].StatBefore = statBytes(p.pid, p.comm, state, ppid, start)
	f.proof.Captures[index].StatAfter = statBytes(p.pid, p.comm, state, ppid, start)
}

func (f *procFixture) cleanupRun() map[string]any {
	return f.report["runs"].([]any)[0].(map[string]any)["cleanup"].(map[string]any)
}

func (f *procFixture) sweepRows(extra ...SweepProcessV2) []SweepProcessV2 {
	return append(append([]SweepProcessV2{}, f.proof.FinalSweep.Processes...), extra...)
}

func TestRawProcessRefusals(t *testing.T) {
	type mutation func(*testing.T, *procFixture) []byte
	proof := func(change func(*procFixture)) mutation {
		return func(_ *testing.T, f *procFixture) []byte { change(f); return nil }
	}
	tests := []struct {
		name, outcome, code string
		mutate              mutation
	}{
		{"graph substituted", "REJECTED", "process-graph-substituted", proof(func(f *procFixture) { f.graph.DraftCommit = "other" })},
		{"execution substituted", "REJECTED", "process-graph-substituted", proof(func(f *procFixture) { f.proof.ExecutionID = "other" })},
		{"proof policy", "REJECTED", "process-policy-digest-mismatch", proof(func(f *procFixture) { f.proof.PolicySHA256 = sha256Hex([]byte("other")) })},
		{"legacy rows", "BLOCKED", "process-legacy-sample-unsupported", func(*testing.T, *procFixture) []byte {
			return []byte(`[{"pid":300,"start":"Sun Oct 4 12:00:00 2026","state":"S"}]`)
		}},
		{"legacy observation", "BLOCKED", "process-legacy-sample-unsupported", func(*testing.T, *procFixture) []byte {
			return []byte(`{"scope":"session","interval_ms":20,"processes":[],"absent":true,"failures":[],"limitations":[]}`)
		}},
		{"unknown member", "REJECTED", "process-wire-invalid", func(t *testing.T, f *procFixture) []byte {
			return append([]byte(`{"extra":1,`), f.proofBytes()[1:]...)
		}},
		{"duplicate member", "REJECTED", "process-wire-invalid", func(t *testing.T, f *procFixture) []byte {
			return append([]byte(`{"execution_id":"exec-main",`), f.proofBytes()[1:]...)
		}},
		{"null array", "REJECTED", "process-wire-invalid", proof(func(f *procFixture) { f.proof.Cleanup = nil })},
		{"unsorted sources", "BLOCKED", "process-graph-invalid", proof(func(f *procFixture) {
			f.graph.Sources[0], f.graph.Sources[1] = f.graph.Sources[1], f.graph.Sources[0]
			f.rebind()
		})},
		{"duplicate source", "BLOCKED", "process-graph-invalid", proof(func(f *procFixture) {
			f.graph.Sources[2].Artifact.ID = f.graph.Sources[1].Artifact.ID
			f.rebind()
		})},
		{"context order", "BLOCKED", "process-graph-invalid", proof(func(f *procFixture) { f.graph.Contexts[1].RunOrdinal = 7; f.rebind() })},
		{"unsupported source", "BLOCKED", "process-native-source-unsupported", proof(func(f *procFixture) {
			hook := f.w.store.put("exec-main/hook", []byte(`{}`))
			f.graph.Sources = append([]SourceBindingV2{{Kind: "hook", Artifact: hook}}, f.graph.Sources...)
			f.rebind()
		})},
		{"report unavailable", "BLOCKED", "process-artifact-unavailable", proof(func(f *procFixture) { delete(f.w.store.data, f.graph.Report.ID) })},
		{"executable digest", "REJECTED", "process-artifact-digest-mismatch", proof(func(f *procFixture) { f.w.store.data["exe/runner"] = []byte("runner-binarY") })},
		{"capture bound", "BLOCKED", "process-bound-exceeded", proof(func(f *procFixture) {
			for range 60 {
				f.proof.Captures = append(f.proof.Captures, f.proof.Captures[3])
			}
		})},
		{"capture index", "REJECTED", "process-capture-malformed", proof(func(f *procFixture) { f.proof.Captures[3].Index = 4 })},
		{"stat without LF", "REJECTED", "process-stat-malformed", proof(func(f *procFixture) {
			c := &f.proof.Captures[3]
			c.StatBefore = c.StatBefore[:len(c.StatBefore)-1]
		})},
		{"pid reuse in bracket", "REJECTED", "process-birth-changed", proof(func(f *procFixture) {
			p := f.procs["runner"]
			f.proof.Captures[3].StatAfter = statBytes(p.pid, p.comm, 'S', p.ppid, p.start+1)
		})},
		{"argv changed in bracket", "REJECTED", "process-bracket-changed", proof(func(f *procFixture) {
			f.proof.Captures[3].CmdlineAfter = argvBytes([]string{"runner", "t2"})
		})},
		{"executable changed in bracket", "REJECTED", "process-bracket-changed", proof(func(f *procFixture) {
			f.proof.Captures[3].ExecutableAfter = f.w.exe["sh"]
		})},
		{"namespace inode", "REJECTED", "process-capture-malformed", proof(func(f *procFixture) { f.proof.Captures[3].NamespaceInode = 1 })},
		{"other namespace", "REJECTED", "process-namespace-mismatch", proof(func(f *procFixture) {
			f.proof.Captures[3].NamespaceLinkBytes, f.proof.Captures[3].NamespaceInode = []byte("pid:[4026531837]"), 4026531837
		})},
		{"other boot", "REJECTED", "process-namespace-mismatch", proof(func(f *procFixture) { f.proof.Captures[3].BootIDBytes = []byte("other\n") })},
		{"zombie claims argv", "REJECTED", "process-capture-malformed", proof(func(f *procFixture) {
			f.proof.Captures[7].Cmdline, f.proof.Captures[7].CmdlineAfter = argvBytes([]string{"sh"}), argvBytes([]string{"sh"})
		})},
		{"native start malformed", "REJECTED", "process-capture-malformed", proof(func(f *procFixture) {
			f.proof.Captures[4].NativeStartOutput = []byte("Sun Oct  4 12:00:00\n")
		})},
		{"exit on live capture", "REJECTED", "process-retirement-invalid", proof(func(f *procFixture) { f.proof.Captures[3].ExitCode = intPtr(0) })},
		{"exit and signal", "REJECTED", "process-retirement-invalid", proof(func(f *procFixture) { f.proof.Captures[7].ExitSignal = intPtr(9) })},
		{"capture after retirement", "REJECTED", "process-retirement-invalid", proof(func(f *procFixture) {
			f.proof.Captures[8] = f.w.live(8, "final-sweep", f.procs["root"])
			f.proof.Captures[7].Phase = "retirement"
		})},
		{"completed without exit", "REJECTED", "process-retirement-invalid", proof(func(f *procFixture) { f.proof.Captures[7].ExitCode = nil })},
		{"reparented birth", "BLOCKED", "process-parent-unverified", proof(func(f *procFixture) {
			p := f.procs["root"]
			f.restat(7, "root", 'Z', 1, p.start)
		})},
		{"wrong launch parent", "BLOCKED", "process-parent-unverified", proof(func(f *procFixture) { f.proof.Launches[4].ParentCaptureIndex = intPtr(2) })},
		{"no launch parent", "BLOCKED", "process-parent-unverified", proof(func(f *procFixture) { f.proof.Launches[4].ParentCaptureIndex = nil })},
		{"orphan observed birth", "BLOCKED", "process-parent-unverified", proof(func(f *procFixture) {
			p := f.procs["main"]
			f.restat(5, "main", 'S', 4242, p.start)
		})},
		{"invocation argv", "REJECTED", "process-executable-mismatch", proof(func(f *procFixture) {
			f.reinvoke(3, "runner", func(inv *InvocationV2) { inv.Argv = []string{"runner", "t9"} })
		})},
		{"invocation executable", "REJECTED", "process-executable-mismatch", proof(func(f *procFixture) {
			f.reinvoke(3, "runner", func(inv *InvocationV2) { inv.Executable = f.w.exe["server"] })
		})},
		{"unadmitted observer", "REJECTED", "process-executable-mismatch", func(t *testing.T, f *procFixture) []byte {
			f.w.impl.ObserverSHA256 = sha256Hex([]byte("other"))
			f.w.policy.Implementation = f.w.impl
			return nil
		}},
		{"invocation environment", "REJECTED", "process-invocation-mismatch", proof(func(f *procFixture) {
			f.reinvoke(3, "runner", func(inv *InvocationV2) { inv.Environment = []EnvironmentV2{{Key: "HOME", Value: "/root"}} })
		})},
		{"invocation environment order", "REJECTED", "process-invocation-mismatch", proof(func(f *procFixture) {
			f.reinvoke(3, "runner", func(inv *InvocationV2) {
				inv.Environment[0], inv.Environment[1] = inv.Environment[1], inv.Environment[0]
			})
		})},
		{"invocation execution", "REJECTED", "process-invocation-mismatch", proof(func(f *procFixture) {
			f.reinvoke(3, "runner", func(inv *InvocationV2) { inv.ExecutionID = "other" })
		})},
		{"invocation stdin", "REJECTED", "process-invocation-mismatch", proof(func(f *procFixture) {
			f.reinvoke(3, "runner", func(inv *InvocationV2) { inv.StdinSHA256 = sha256Hex([]byte("stdin")) })
		})},
		{"start and exec births", "REJECTED", "process-birth-changed", proof(func(f *procFixture) { f.proof.Launches[3].StartCaptureIndex = 2 })},
		{"two launches one birth", "BLOCKED", "process-role-ambiguous", proof(func(f *procFixture) {
			f.proof.Launches = append(f.proof.Launches, OwnedLaunchV2{Index: 6, ContextIndex: 1, Purpose: "runner",
				ParentCaptureIndex: intPtr(2), StartCaptureIndex: 3, ExecutedCaptureIndex: 3})
			f.reinvoke(6, "runner", nil)
		})},
		{"two births one node", "BLOCKED", "process-role-ambiguous", proof(func(f *procFixture) { f.addDuplicateRunner() })},
		{"ambiguous rules", "BLOCKED", "process-role-ambiguous", func(t *testing.T, f *procFixture) []byte {
			extra := f.w.policy.Rules[2]
			extra.Slot = "network"
			f.w.policy.Rules = append(f.w.policy.Rules, extra)
			return nil
		}},
		{"switches derive no role", "BLOCKED", "process-role-unsupported", proof(func(f *procFixture) {
			argv := argvBytes([]string{"chrome", "--type=gpu-process"})
			f.proof.Captures[6].Cmdline, f.proof.Captures[6].CmdlineAfter = argv, argv
		})},
		{"argv zero is not a switch", "BLOCKED", "process-role-unsupported", proof(func(f *procFixture) {
			argv := argvBytes([]string{"--headless"})
			f.proof.Captures[5].Cmdline, f.proof.Captures[5].CmdlineAfter = argv, argv
		})},
		{"observed image not admitted", "BLOCKED", "process-role-unsupported", proof(func(f *procFixture) {
			f.proof.Captures[6].Executable, f.proof.Captures[6].ExecutableAfter = f.w.exe["server"], f.w.exe["server"]
		})},
		{"browser under observer", "BLOCKED", "process-role-unsupported", proof(func(f *procFixture) {
			p := f.procs["main"]
			f.restat(5, "main", 'S', f.procs["trusted"].pid, p.start)
		})},
		{"trusted start root", "REJECTED", "process-trusted-start-mismatch", proof(func(f *procFixture) {
			f.storeTrustedStart(func(s *TrustedStartV2) { s.RootCaptureIndex = 3 })
		})},
		{"trusted start host", "REJECTED", "process-trusted-start-mismatch", proof(func(f *procFixture) {
			f.storeTrustedStart(func(s *TrustedStartV2) { s.HostTupleSHA256 = sha256Hex([]byte("other")) })
		})},
		{"trusted start request", "REJECTED", "process-trusted-start-mismatch", proof(func(f *procFixture) {
			f.storeTrustedStart(func(s *TrustedStartV2) { s.RequestSHA256 = sha256Hex([]byte("other")) })
		})},
		{"trusted start not observer", "REJECTED", "process-trusted-start-mismatch", proof(func(f *procFixture) {
			f.proof.TrustedStartInvocation = f.proof.Launches[2].Invocation
		})},
		{"launch purpose is not its role", "BLOCKED", "process-role-unsupported", proof(func(f *procFixture) {
			f.proof.Launches[3].Purpose, f.proof.Launches[3].ContextIndex = "workflow-root", 0
			f.reinvoke(3, "runner", nil)
		})},
		{"sweep is trusted observer", "REJECTED", "process-sweep-invalid", proof(func(f *procFixture) {
			f.proof.FinalSweep.ObserverInvocation = f.proof.Launches[1].Invocation
		})},
		{"unknown native locator", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) {
			f.proof.NativeReferences[0].Pointer = "/freshness//leader"
		})},
		{"escaped native locator alias", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) {
			f.proof.NativeReferences[0].Pointer = "/fresh~1ness/leader"
		})},
		{"duplicate native reference", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) {
			f.proof.NativeReferences = append(f.proof.NativeReferences, f.proof.NativeReferences[0])
		})},
		{"native reference kind", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) { f.proof.NativeReferences[0].Kind = "app-instance" })},
		{"native reference context", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) { f.proof.NativeReferences[0].ContextIndex = 0 })},
		{"native reference not server", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) { f.proof.NativeReferences[0].CaptureIndex = 5 })},
		{"native reference infra", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) {
			f.proof.NativeReferences[1].CaptureIndex, f.proof.NativeReferences[1].ContextIndex = 1, 1
		})},
		{"descendant state differs", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) {
			p := f.procs["main"]
			f.restat(5, "main", 'R', p.ppid, p.start)
		})},
		{"fresh start not trimmed", "REJECTED", "process-native-reference-invalid", proof(func(f *procFixture) {
			f.provider["freshness"].(map[string]any)["leader"].(map[string]any)["start"] = testJoinStart
			f.storeNative()
		})},
		{"native reference missing", "BLOCKED", "process-native-reference-missing", proof(func(f *procFixture) {
			f.proof.NativeReferences = f.proof.NativeReferences[:1]
		})},
		{"native start unavailable", "BLOCKED", "process-native-start-unavailable", proof(func(f *procFixture) {
			f.proof.Captures[4].NativeStartOutput, f.proof.Captures[4].NativeStartTool = []byte{}, noArtifact
		})},
		{"cleanup join missing", "BLOCKED", "process-cleanup-unknown", proof(func(f *procFixture) { f.proof.Cleanup = []CleanupJoinV2{} })},
		{"cleanup join facts", "REJECTED", "process-cleanup-join-invalid", proof(func(f *procFixture) { f.proof.Cleanup[0].Facts.Cancelled = true })},
		{"cleanup join context", "REJECTED", "process-cleanup-join-invalid", proof(func(f *procFixture) { f.proof.Cleanup[0].ContextIndex = 0 })},
		{"cleanup join pointer", "REJECTED", "process-cleanup-join-invalid", proof(func(f *procFixture) { f.proof.Cleanup[0].Pointer = "/runs/0/cleanup/" })},
		{"cleanup failures unknown", "BLOCKED", "process-cleanup-unknown", proof(func(f *procFixture) {
			f.cleanupRun()["descendants"].(map[string]any)["failures"] = []any{"read failed"}
			f.storeNative()
			f.proof.Cleanup[0].Facts.Failures, f.proof.Cleanup[0].Facts.DerivedState = []string{"read failed"}, "unknown"
		})},
		{"cleanup without descendants unknown", "BLOCKED", "process-cleanup-unknown", proof(func(f *procFixture) {
			delete(f.cleanupRun(), "descendants")
			f.storeNative()
			f.proof.NativeReferences = f.proof.NativeReferences[:1]
			f.proof.Cleanup[0].Facts = CleanupFactsV2{OwnedGroup: true, Status: "owned-process-group", DerivedState: "unknown",
				Qualification: "process-group", Failures: []string{}, Limitations: []string{}}
		})},
		{"cleanup survivors", "REJECTED", "process-cleanup-survivors", proof(func(f *procFixture) {
			f.cleanupRun()["descendants"].(map[string]any)["absent"] = false
			f.storeNative()
			f.proof.Cleanup[0].Facts.Absent, f.proof.Cleanup[0].Facts.DerivedState = false, "survivors"
		})},
		{"provider descendants survive", "REJECTED", "process-cleanup-survivors", proof(func(f *procFixture) {
			f.provider["descendantObservation"] = map[string]any{"scope": "session", "interval_ms": 20, "processes": []any{},
				"absent": false, "failures": []any{}, "limitations": []any{}}
			f.storeNative()
		})},
		{"sweep survivor", "REJECTED", "process-sweep-survivor", proof(func(f *procFixture) {
			f.setSweep(f.sweepRows(SweepProcessV2{PID: f.procs["runner"].pid, StatBytes: f.proof.Captures[3].StatBefore}), []string{})
		})},
		{"sweep zombie survivor", "REJECTED", "process-sweep-survivor", proof(func(f *procFixture) {
			f.setSweep(f.sweepRows(SweepProcessV2{PID: f.procs["root"].pid, StatBytes: f.proof.Captures[7].StatBefore}), []string{})
		})},
		{"sweep read failure", "BLOCKED", "process-sweep-incomplete", proof(func(f *procFixture) {
			f.setSweep(f.sweepRows(), []string{"300: stat unreadable"})
		})},
		{"sweep observer not waited", "BLOCKED", "process-sweep-incomplete", proof(func(f *procFixture) { f.proof.Launches[5].Completed = false })},
		{"sweep observer failed", "BLOCKED", "process-sweep-incomplete", proof(func(f *procFixture) { f.proof.Captures[9].ExitCode = intPtr(1) })},
		{"sweep untracked child", "BLOCKED", "process-sweep-scope-ambiguous", proof(func(f *procFixture) {
			f.setSweep(f.sweepRows(SweepProcessV2{PID: 777, StatBytes: statBytes(777, "x", 'S', f.procs["runner"].pid, 900)}), []string{})
		})},
		{"sweep document differs", "REJECTED", "process-sweep-invalid", proof(func(f *procFixture) {
			f.storeSweepDocument(func(d *AbsenceSweepDocumentV2) { d.Processes = d.Processes[:2] })
		})},
		{"sweep directory differs", "REJECTED", "process-sweep-invalid", proof(func(f *procFixture) {
			f.proof.FinalSweep.PIDDirectoryBytes = []byte("1\n100\n")
			f.storeSweepDocument(nil)
		})},
		{"sweep row pid differs", "REJECTED", "process-sweep-invalid", proof(func(f *procFixture) {
			rows := f.sweepRows()
			rows[0].PID = 2
			f.proof.FinalSweep.Processes = rows
			f.storeSweepDocument(nil)
		})},
		{"sweep lacks observer", "REJECTED", "process-sweep-invalid", proof(func(f *procFixture) { f.setSweep(f.sweepRows()[:2], []string{}) })},
		{"sweep other boot", "REJECTED", "process-namespace-mismatch", proof(func(f *procFixture) {
			f.proof.FinalSweep.BootIDBytes = []byte("other\n")
			f.storeSweepDocument(nil)
		})},
		{"workload after sweep start", "REJECTED", "process-sweep-invalid", proof(func(f *procFixture) {
			f.proof.Captures[8], f.proof.Captures[7] = f.proof.Captures[7], f.proof.Captures[8]
			f.proof.Captures[7].Index, f.proof.Captures[8].Index = 7, 8
			f.proof.Launches[2].RetirementCaptureIndex = intPtr(8)
			f.proof.Launches[5].StartCaptureIndex, f.proof.Launches[5].ExecutedCaptureIndex = 7, 7
		})},
		{"other phase in sweep", "REJECTED", "process-sweep-invalid", proof(func(f *procFixture) { f.proof.Captures[6].Phase = "final-sweep" })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main"})
			data := tt.mutate(t, f)
			if data == nil {
				data = f.proofBytes()
			}
			_, err := verifyRawProcessV2(testContext(), f.w.policy, f.w.policyRef.SHA256, f.graph, data, f.w.store)
			expectRefusal(t, err, tt.outcome, tt.code)
		})
	}
	t.Run("cancelled", func(t *testing.T) {
		f := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main"})
		ctx, cancel := context.WithCancel(testContext())
		cancel()
		_, err := verifyRawProcessV2(ctx, f.w.policy, f.w.policyRef.SHA256, f.graph, f.proofBytes(), f.w.store)
		expectRefusal(t, err, "BLOCKED", "process-verification-cancelled")
	})
}

func TestDistinctProcessStates(t *testing.T) {
	t.Run("signal", func(t *testing.T) {
		f := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main"})
		f.proof.Captures[7].ExitCode, f.proof.Captures[7].ExitSignal = nil, intPtr(9)
		result, err := f.verifyRaw()
		if err != nil {
			t.Fatal(err)
		}
		if got := result.logical[1].State; got != "completed/signal:9" {
			t.Fatalf("workflow root state %q", got)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		f := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main", cancelRunner: true})
		result, err := f.verifyRaw()
		if err != nil {
			t.Fatal(err)
		}
		if want := expectedLogical("repeat", 0, "cancelled/unreaped"); !reflect.DeepEqual(result.logical, want) {
			t.Fatalf("logical graph:\n got %+v\nwant %+v", result.logical, want)
		}
	})
	t.Run("zero descendants observed absent", func(t *testing.T) {
		// Zero rows with absent=true is observed absence, distinct from an
		// unknown cleanup (failures) and from survivors (absent=false).
		f := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main"})
		f.cleanupRun()["descendants"].(map[string]any)["processes"] = []any{}
		f.storeNative()
		f.proof.NativeReferences = f.proof.NativeReferences[:1]
		result, err := f.verifyRaw()
		if err != nil {
			t.Fatal(err)
		}
		if result.references != 1 || result.cleanups != 1 {
			t.Fatalf("result counts: %+v", result)
		}
	})
}

func TestRoleWitnessOrderInvariant(t *testing.T) {
	plain, err := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main"}).verifyRaw()
	if err != nil {
		t.Fatal(err)
	}
	f := newProcWorld(t).fixture(t, fixtureSpec{name: "exec-main"})
	f.swapWitness()
	swapped, err := f.verifyRaw()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain.logical, swapped.logical) {
		t.Fatalf("witness order changed the logical graph:\n%+v\n%+v", plain.logical, swapped.logical)
	}
}

func TestProcStatParser(t *testing.T) {
	valid := statBytes(300, "a) b", 'S', 1, 42)
	s, err := parseProcStatV2(valid)
	if err != nil || s.pid != 300 || s.state != 'S' || s.ppid != 1 || s.startTicks != 42 {
		t.Fatalf("comm containing \") \": %+v %v", s, err)
	}
	for name, raw := range map[string][]byte{
		"leading zero PID":    append([]byte("0"), valid...),
		"two LFs":             append(append([]byte{}, valid...), '\n'),
		"no LF":               valid[:len(valid)-1],
		"too few fields":      []byte("300 (x) S 1 0 0\n"),
		"empty field":         bytes.Replace(valid, []byte(" 0 "), []byte("  "), 1),
		"unsupported state":   statBytes(300, "x", 'Q', 1, 42),
		"starttime with zero": bytes.Replace(statBytes(300, "x", 'S', 1, 42), []byte(" 42 "), []byte(" 042 "), 1),
		"no comm":             []byte("300 x S 1\n"),
	} {
		if _, err := parseProcStatV2(raw); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

func TestPolicyRefusals(t *testing.T) {
	tests := []struct {
		name, outcome, code string
		mutate              func(*ProcessPolicyV2)
	}{
		{"duplicate role slot", "BLOCKED", "process-role-ambiguous", func(p *ProcessPolicyV2) { p.Rules = append(p.Rules, p.Rules[2]) }},
		{"switch required and forbidden", "BLOCKED", "process-role-ambiguous", func(p *ProcessPolicyV2) {
			p.Rules[5].ForbiddenSwitches = []string{"--headless"}
		}},
		{"playwright worker", "BLOCKED", "process-role-unsupported", func(p *ProcessPolicyV2) { p.Rules[2].Derivation = "playwright-node-worker/0" }},
		{"purpose differs from role", "BLOCKED", "process-role-unsupported", func(p *ProcessPolicyV2) { p.Rules[2].LaunchPurpose = "server" }},
		{"owned launch with switches", "BLOCKED", "process-role-unsupported", func(p *ProcessPolicyV2) {
			p.Rules[2].RequiredSwitches = []string{"--x"}
		}},
		{"malformed switch", "BLOCKED", "process-role-unsupported", func(p *ProcessPolicyV2) { p.Rules[5].RequiredSwitches = []string{"headless"} }},
		{"other host", "REJECTED", "process-admission-mismatch", func(p *ProcessPolicyV2) { p.Host.KernelRelease = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newProcWorld(t)
			policy := w.policy
			policy.Rules = append([]RoleRuleV2{}, w.policy.Rules...)
			tt.mutate(&policy)
			_, err := decodePolicyV2(mustJSON(t, policy), w.admission)
			expectRefusal(t, err, tt.outcome, tt.code)
		})
	}
}
