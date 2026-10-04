// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// memStore is an exact in-memory artifact reader for synthetic proofs.
type memStore struct{ data map[string][]byte }

func (s *memStore) ReadArtifact(id, _ string, _ int64) ([]byte, error) {
	data, ok := s.data[id]
	if !ok {
		return nil, errors.New("missing artifact " + id)
	}
	return bytes.Clone(data), nil
}

func (s *memStore) put(id string, data []byte) ArtifactRefV2 {
	s.data[id] = bytes.Clone(data)
	return ArtifactRefV2{ID: id, SHA256: sha256Hex(data), Bytes: int64(len(data))}
}

func testContext() context.Context { return context.Background() }

func mustJSON(t testing.TB, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

var (
	testBootID    = []byte("4f9c5a52-7d1e-4c55-9a43-2b0f1e6d8c11\n")
	testNSLink    = []byte("pid:[4026531836]")
	testNSDevice  = uint64(4)
	testNSInode   = uint64(4026531836)
	testLstart    = []byte("Sun Oct  4 12:00:00 2026\n")
	testTrimStart = "Sun Oct  4 12:00:00 2026"
	testJoinStart = "Sun Oct 4 12:00:00 2026"
	noArtifact    = ArtifactRefV2{SHA256: sha256Hex(nil)}
)

func statBytes(pid int, comm string, state byte, ppid int, start uint64) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "%d (%s) %c %d", pid, comm, state, ppid)
	for range 17 {
		b.WriteString(" 0")
	}
	fmt.Fprintf(&b, " %d", start)
	for range 30 {
		b.WriteString(" 0")
	}
	b.WriteString("\n")
	return []byte(b.String())
}

// procWorld is one admitted policy, implementation and host with its
// synthetic executables and, optionally, a passing qualification report.
type procWorld struct {
	store              *memStore
	exe                map[string]ArtifactRefV2
	impl               ImplementationV2
	host               HostTupleV2
	policy             ProcessPolicyV2
	policyBytes        []byte
	policyRef          ArtifactRefV2
	admission          ProcessAdmissionV2
	qualificationBytes []byte
}

func newProcWorld(t testing.TB) *procWorld {
	w := &procWorld{store: &memStore{data: map[string][]byte{}}, exe: map[string]ArtifactRefV2{}}
	for _, name := range []string{"supervisor", "observer", "sh", "runner", "server", "chrome", "ps"} {
		w.exe[name] = w.store.put("exe/"+name, []byte(name+"-binary"))
	}
	w.impl = ImplementationV2{SourceCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
		VerifierSHA256: sha256Hex([]byte("verifier")), ObserverSHA256: w.exe["observer"].SHA256, SupervisorSHA256: w.exe["supervisor"].SHA256}
	w.host = HostTupleV2{OS: "linux", Architecture: "amd64", KernelRelease: "6.8.0-test",
		ImageManifestSHA256: sha256Hex([]byte("image")), LauncherPolicySHA256: sha256Hex([]byte("launcher"))}
	owned := func(role, exe, parent string) RoleRuleV2 {
		return RoleRuleV2{Role: role, Derivation: "owned-launch/0", Executable: w.exe[exe], LaunchPurpose: role, ParentRole: parent,
			Slot: "default", RequiredSwitches: []string{}, ForbiddenSwitches: []string{}}
	}
	browser := func(role, parent, slot string, required, forbidden []string) RoleRuleV2 {
		return RoleRuleV2{Role: role, Derivation: "chromium-switch-role/0", Executable: w.exe["chrome"], LaunchPurpose: role,
			ParentRole: parent, Slot: slot, RequiredSwitches: required, ForbiddenSwitches: forbidden}
	}
	w.policy = ProcessPolicyV2{Profile: "postmerge-process-policy/2", Implementation: w.impl, Host: w.host,
		Rules: []RoleRuleV2{
			owned("host-supervisor", "supervisor", "outside"), owned("workflow-root", "sh", "host-supervisor"),
			owned("runner", "runner", "workflow-root"), owned("server", "server", "runner"),
			owned("observer", "observer", "host-supervisor"),
			browser("browser-main", "runner", "main", []string{"--headless"}, []string{"--type"}),
			browser("browser-renderer", "browser-main", "sandboxed", []string{"--type=renderer"}, []string{}),
		},
		Observation: ObservationPolicyV2{Profile: "linux-procfs-birth-command/0", Scope: "observed-owned-process-tree",
			TargetIntervalMS: 20, CaptureLimit: 64, ProcessLimit: 64, RequirePairedStat: true, RequireFinalIndependentSweep: true,
			Limitations: []string{}}}
	w.sealPolicy(t)
	return w
}

func (w *procWorld) sealPolicy(t testing.TB) {
	w.policyBytes = mustJSON(t, w.policy)
	w.policyRef = w.store.put("policy", w.policyBytes)
	w.admission = ProcessAdmissionV2{Policy: w.policyRef, HostTuple: w.host, Implementation: w.impl}
}

type procSpec struct {
	pid, ppid int
	start     uint64
	comm      string
	argv      []string
	exe       string
	native    bool
}

func (w *procWorld) live(index int, phase string, p procSpec) ProcCaptureV2 {
	c := ProcCaptureV2{Index: index, Phase: phase, BootIDBytes: testBootID, NamespaceLinkBytes: testNSLink,
		NamespaceDevice: testNSDevice, NamespaceInode: testNSInode,
		StatBefore: statBytes(p.pid, p.comm, 'S', p.ppid, p.start), StatAfter: statBytes(p.pid, p.comm, 'S', p.ppid, p.start),
		Cmdline: argvBytes(p.argv), CmdlineAfter: argvBytes(p.argv),
		ExecutableLinkBytes: []byte("/usr/bin/" + p.exe), ExecutableLinkAfterBytes: []byte("/usr/bin/" + p.exe),
		Executable: w.exe[p.exe], ExecutableAfter: w.exe[p.exe], NativeStartOutput: []byte{}, NativeStartTool: noArtifact}
	if p.native {
		c.NativeStartOutput, c.NativeStartTool = testLstart, w.exe["ps"]
	}
	return c
}

func (w *procWorld) zombie(index int, p procSpec, exit int) ProcCaptureV2 {
	return ProcCaptureV2{Index: index, Phase: "retirement", BootIDBytes: testBootID, NamespaceLinkBytes: testNSLink,
		NamespaceDevice: testNSDevice, NamespaceInode: testNSInode,
		StatBefore: statBytes(p.pid, p.comm, 'Z', p.ppid, p.start), StatAfter: statBytes(p.pid, p.comm, 'Z', p.ppid, p.start),
		Cmdline: []byte{}, CmdlineAfter: []byte{}, ExecutableLinkBytes: []byte{}, ExecutableLinkAfterBytes: []byte{},
		Executable: noArtifact, ExecutableAfter: noArtifact, NativeStartOutput: []byte{}, NativeStartTool: noArtifact, ExitCode: &exit}
}

type fixtureSpec struct {
	name         string
	pidBase      int
	startBase    uint64
	ordinal      int
	cancelRunner bool
}

// procFixture is one synthetic owned execution: supervisor, trusted-start
// observer, workflow root, runner, server, an observed chromium main and
// renderer, the root's reaped retirement and an independent final sweep.
type procFixture struct {
	t        testing.TB
	w        *procWorld
	spec     fixtureSpec
	graph    GraphBindingV2
	proof    ProcessProofV2
	request  ArtifactRefV2
	provider map[string]any
	report   map[string]any
	procs    map[string]procSpec
}

func intPtr(v int) *int { return &v }

func (w *procWorld) fixture(t testing.TB, spec fixtureSpec) *procFixture {
	b, s := spec.pidBase, spec.startBase+10
	f := &procFixture{t: t, w: w, spec: spec}
	f.procs = map[string]procSpec{
		"supervisor": {pid: 100, ppid: 1, start: 5, comm: "supervisor", argv: []string{"supervisor"}, exe: "supervisor"},
		"trusted":    {pid: 150 + b, ppid: 100, start: s + 1, comm: "observer", argv: []string{"observer", "start"}, exe: "observer"},
		"root":       {pid: 200 + b, ppid: 100, start: s + 2, comm: "sh", argv: []string{"sh", "-c", "run"}, exe: "sh"},
		"runner":     {pid: 300 + b, ppid: 200 + b, start: s + 3, comm: "runner", argv: []string{"runner", "t1"}, exe: "runner"},
		"server":     {pid: 310 + b, ppid: 300 + b, start: s + 4, comm: "server", argv: []string{"server"}, exe: "server", native: true},
		"main":       {pid: 320 + b, ppid: 300 + b, start: s + 5, comm: "chrome", argv: []string{"chrome", "--headless"}, exe: "chrome", native: true},
		"renderer":   {pid: 330 + b, ppid: 320 + b, start: s + 6, comm: "chrome", argv: []string{"chrome", "--type=renderer"}, exe: "chrome"},
		"sweeper":    {pid: 900 + b, ppid: 100, start: s + 9, comm: "observer", argv: []string{"observer", "sweep"}, exe: "observer"},
	}
	p := f.procs
	f.request = w.store.put(spec.name+"/request", []byte("request "+spec.name))
	f.provider = map[string]any{"freshness": map[string]any{"leader": map[string]any{"pid": p["server"].pid, "start": testTrimStart}}}
	f.report = map[string]any{"controls": []any{}, "runs": []any{map[string]any{
		"kind": "repeat", "ordinal": spec.ordinal,
		"cleanup": map[string]any{"owned_group": true, "status": "owned-process-group", "qualification": "process-group",
			"cancelled": false, "timed_out": false,
			"descendants": map[string]any{"scope": "session", "interval_ms": 20, "absent": true, "failures": []any{}, "limitations": []any{},
				"processes": []any{map[string]any{"pid": p["main"].pid, "parent_pid": p["main"].ppid, "start": testJoinStart, "state": "S"}}}},
	}}}
	f.graph = GraphBindingV2{Profile: "postmerge-native-graph-binding/2", ExecutionID: spec.name, Request: f.request,
		ProductCommit: strings.Repeat("1", 40), TestCommit: strings.Repeat("2", 40), DraftCommit: strings.Repeat("3", 40),
		Contexts: []RunContextV2{
			{RunKind: "workflow", TestIDs: []string{}},
			{RunIndex: intPtr(0), RunKind: "repeat", RunOrdinal: spec.ordinal, TestIDs: []string{"t1"}},
		}}
	f.proof = ProcessProofV2{Profile: "postmerge-process-proof/2", ExecutionID: spec.name, PolicySHA256: w.policyRef.SHA256,
		Captures: []ProcCaptureV2{
			w.live(0, "start-barrier", p["supervisor"]), w.live(1, "start-barrier", p["trusted"]), w.live(2, "start-barrier", p["root"]),
			w.live(3, "during-run", p["runner"]), w.live(4, "during-run", p["server"]), w.live(5, "during-run", p["main"]),
			w.live(6, "during-run", p["renderer"]), w.zombie(7, p["root"], 0),
			w.live(8, "final-sweep", p["sweeper"]), w.zombie(9, p["sweeper"], 0),
		},
		Launches: []OwnedLaunchV2{
			{Index: 0, ContextIndex: 0, Purpose: "host-supervisor", StartCaptureIndex: 0, ExecutedCaptureIndex: 0},
			{Index: 1, ContextIndex: 0, Purpose: "observer", ParentCaptureIndex: intPtr(0), StartCaptureIndex: 1, ExecutedCaptureIndex: 1},
			{Index: 2, ContextIndex: 0, Purpose: "workflow-root", ParentCaptureIndex: intPtr(0), StartCaptureIndex: 2, ExecutedCaptureIndex: 2,
				RetirementCaptureIndex: intPtr(7), Completed: true},
			{Index: 3, ContextIndex: 1, Purpose: "runner", ParentCaptureIndex: intPtr(2), StartCaptureIndex: 3, ExecutedCaptureIndex: 3,
				Cancelled: spec.cancelRunner},
			{Index: 4, ContextIndex: 1, Purpose: "server", ParentCaptureIndex: intPtr(3), StartCaptureIndex: 4, ExecutedCaptureIndex: 4},
			{Index: 5, ContextIndex: 0, Purpose: "observer", ParentCaptureIndex: intPtr(0), StartCaptureIndex: 8, ExecutedCaptureIndex: 8,
				RetirementCaptureIndex: intPtr(9), Completed: true},
		},
		Cleanup: []CleanupJoinV2{{Pointer: "/runs/0/cleanup", ContextIndex: 1, Facts: CleanupFactsV2{OwnedGroup: true,
			Status: "owned-process-group", DerivedState: "observed-absent", DescendantsPresent: true, Absent: true,
			Qualification: "process-group", Scope: "session", IntervalMS: 20, Failures: []string{}, Limitations: []string{}}}},
	}
	launchProc := []string{"supervisor", "trusted", "root", "runner", "server", "sweeper"}
	for i := range f.proof.Launches {
		f.reinvoke(i, launchProc[i], nil)
	}
	f.storeNative()
	f.proof.NativeReferences = []NativeProcessReferenceV2{
		{ArtifactID: f.providerID(), Pointer: "/freshness/leader", Kind: "fresh-process", ContextIndex: 1, CaptureIndex: 4},
		{ArtifactID: f.graph.Report.ID, Pointer: "/runs/0/cleanup/descendants/processes/0", Kind: "descendant", ContextIndex: 1, CaptureIndex: 5},
	}
	f.storeTrustedStart(nil)
	f.setSweep([]SweepProcessV2{
		{PID: 1, StatBytes: statBytes(1, "init", 'S', 0, 1)},
		{PID: 100, StatBytes: f.proof.Captures[0].StatBefore},
		{PID: p["sweeper"].pid, StatBytes: f.proof.Captures[8].StatBefore},
	}, []string{})
	return f
}

func (f *procFixture) providerID() string { return f.spec.name + "/provider" }

// reinvoke stores launch i's invocation preimage for proc, applying mutate.
func (f *procFixture) reinvoke(i int, proc string, mutate func(*InvocationV2)) {
	p, l := f.procs[proc], &f.proof.Launches[i]
	inv := InvocationV2{Profile: "postmerge-owned-invocation/2", ExecutionID: f.spec.name, Purpose: l.Purpose, Executable: f.w.exe[p.exe],
		Argv: p.argv, Environment: []EnvironmentV2{{Key: "LC_ALL", Value: "C"}, {Key: "PATH", Value: "/usr/bin:/bin"}},
		StdinSHA256: noArtifact.SHA256, ContextIndex: l.ContextIndex, LaunchIndex: i}
	if mutate != nil {
		mutate(&inv)
	}
	l.Invocation, l.Stdin = f.w.store.put(f.spec.name+"/invocation/"+strconv.Itoa(i), mustJSON(f.t, inv)), noArtifact
	// Launch 1 is the trusted-start observer and launch 5 the final sweep observer.
	if i == 1 {
		f.proof.TrustedStartInvocation = l.Invocation
	}
	if i == 5 {
		f.proof.FinalSweep.ObserverInvocation = l.Invocation
	}
}

// storeNative stores the provider and report, rebuilds the sorted sources and
// rebinds the proof to the new graph identity.
func (f *procFixture) storeNative() {
	provider := f.w.store.put(f.providerID(), mustJSON(f.t, f.provider))
	run := f.report["runs"].([]any)[0].(map[string]any)
	run["receipt_sha256"] = "sha256:" + provider.SHA256
	f.graph.Report = f.w.store.put(f.spec.name+"/report", mustJSON(f.t, f.report))
	f.graph.Sources = []SourceBindingV2{{Kind: "provider", Artifact: provider}, {Kind: "report", Artifact: f.graph.Report},
		{Kind: "request", Artifact: f.request}}
	for i := range f.proof.Cleanup {
		f.proof.Cleanup[i].ArtifactID = f.graph.Report.ID
	}
	for i := range f.proof.NativeReferences {
		if f.proof.NativeReferences[i].Kind == "descendant" {
			f.proof.NativeReferences[i].ArtifactID = f.graph.Report.ID
		}
	}
	f.rebind()
}

func (f *procFixture) rebind() {
	sha, err := GraphBindingSHA256V2(f.graph)
	if err != nil {
		f.t.Fatal(err)
	}
	f.proof.GraphBindingSHA256 = sha
}

func (f *procFixture) storeTrustedStart(mutate func(*TrustedStartV2)) {
	hostSHA, err := HostTupleSHA256V2(f.w.host)
	if err != nil {
		f.t.Fatal(err)
	}
	start := TrustedStartV2{Profile: "postmerge-trusted-start/2", ExecutionID: f.spec.name, PolicySHA256: f.w.policyRef.SHA256,
		RequestSHA256: f.request.SHA256, SupervisorSHA256: f.w.impl.SupervisorSHA256, ObserverSHA256: f.w.impl.ObserverSHA256,
		HostTupleSHA256: hostSHA, RootCaptureIndex: 2}
	if mutate != nil {
		mutate(&start)
	}
	f.proof.TrustedStartStdout = f.w.store.put(f.spec.name+"/trusted-start", mustJSON(f.t, start))
}

// setSweep sets the final sweep rows in both the proof and the observer's
// retained output document.
func (f *procFixture) setSweep(rows []SweepProcessV2, failures []string) {
	var directory []byte
	for _, row := range rows {
		directory = append(directory, strconv.Itoa(row.PID)+"\n"...)
	}
	sweep := &f.proof.FinalSweep
	sweep.BootIDBytes, sweep.NamespaceLinkBytes, sweep.PIDDirectoryBytes = testBootID, testNSLink, append([]byte{}, directory...)
	sweep.Processes, sweep.ReadFailures = rows, failures
	f.storeSweepDocument(nil)
}

func (f *procFixture) storeSweepDocument(mutate func(*AbsenceSweepDocumentV2)) {
	sweep := &f.proof.FinalSweep
	document := AbsenceSweepDocumentV2{Profile: "postmerge-absence-sweep/2", ExecutionID: f.spec.name, BootIDBytes: sweep.BootIDBytes,
		NamespaceLinkBytes: sweep.NamespaceLinkBytes, PIDDirectoryBytes: sweep.PIDDirectoryBytes, Processes: sweep.Processes,
		ReadFailures: sweep.ReadFailures}
	if mutate != nil {
		mutate(&document)
	}
	sweep.ObserverStdout = f.w.store.put(f.spec.name+"/sweep", mustJSON(f.t, document))
}

// swapWitness reorders the two observed chromium witnesses without changing
// any physical fact.
func (f *procFixture) swapWitness() {
	c := f.proof.Captures
	c[5], c[6] = c[6], c[5]
	c[5].Index, c[6].Index = 5, 6
	for i := range f.proof.NativeReferences {
		if f.proof.NativeReferences[i].CaptureIndex == 5 {
			f.proof.NativeReferences[i].CaptureIndex = 6
		}
	}
}

func (f *procFixture) proofBytes() []byte { return mustJSON(f.t, f.proof) }

func (f *procFixture) verifyRaw() (rawProcessResultV2, error) {
	return verifyRawProcessV2(testContext(), f.w.policy, f.w.policyRef.SHA256, f.graph, f.proofBytes(), f.w.store)
}

// qualify builds and admits a passing qualification report with all ten
// fixed cases from fresh synthetic fixtures.
func (w *procWorld) qualify(t testing.TB) {
	type run = qualRunV2
	fresh := func(name string, spec fixtureSpec) *procFixture {
		spec.name = name
		return w.fixture(t, spec)
	}
	single := func(f *procFixture) []run { return []run{{f.graph, f.proofBytes()}} }
	ambiguous := fresh("q-ambiguous", fixtureSpec{})
	ambiguous.addDuplicateRunner()
	reuse := fresh("q-reuse", fixtureSpec{})
	runner := reuse.procs["runner"]
	reuse.proof.Captures[3].StatAfter = statBytes(runner.pid, runner.comm, 'S', runner.ppid, runner.start+1)
	substA, substB := fresh("q-subst-a", fixtureSpec{}), fresh("q-subst-b", fixtureSpec{})
	freshA, freshB := fresh("q-fresh-a", fixtureSpec{}), fresh("q-fresh-b", fixtureSpec{pidBase: 1000, startBase: 1000})
	orderA, orderB := fresh("q-order-a", fixtureSpec{ordinal: 1}), fresh("q-order-b", fixtureSpec{ordinal: 2})
	witness := fresh("q-witness", fixtureSpec{})
	plain := witness.proofBytes()
	witness.swapWitness()
	legacy := fresh("q-legacy", fixtureSpec{})
	cases := map[string][]run{
		"owned-launch-joins":              single(fresh("q-owned", fixtureSpec{})),
		"two-fresh-births":                {{freshA.graph, freshA.proofBytes()}, {freshB.graph, freshB.proofBytes()}},
		"cleanup-grandchild-interruption": single(fresh("q-cancel", fixtureSpec{cancelRunner: true})),
		"independent-absence":             single(fresh("q-absence", fixtureSpec{})),
		"legacy-samples-refused":          {{legacy.graph, []byte(`{"processes":[{"pid":300,"start":"Sun Oct 4 12:00:00 2026"}]}`)}},
		"roles-ambiguous-refused":         single(ambiguous),
		"graph-substitution-refused":      {{substB.graph, substA.proofBytes()}},
		"pid-reuse-refused":               single(reuse),
		"schedule-order-preserved":        {{orderA.graph, orderA.proofBytes()}, {orderB.graph, orderB.proofBytes()}},
		"role-witness-order-invariant":    {{witness.graph, plain}, {witness.graph, witness.proofBytes()}},
	}
	report := ProcessQualificationV2{Profile: "postmerge-process-qualification/2", PolicySHA256: w.policyRef.SHA256,
		Implementation: w.impl, Host: w.host, Limitations: []string{"synthetic unit fixture"}}
	for _, id := range qualificationCaseIDs {
		report.Cases = append(report.Cases, w.qualificationCase(t, id, cases[id]))
	}
	w.qualificationBytes = mustJSON(t, report)
	w.admission.Qualification = w.store.put("qualification", w.qualificationBytes)
}

type qualRunV2 struct {
	graph GraphBindingV2
	proof []byte
}

func (w *procWorld) qualificationCase(t testing.TB, id string, runs []qualRunV2) QualificationCaseV2 {
	c := QualificationCaseV2{ID: id, Policy: w.policyRef, GraphBindings: []ArtifactRefV2{}, Proofs: []ArtifactRefV2{}}
	for i, r := range runs {
		c.GraphBindings = append(c.GraphBindings, w.store.put("q/"+id+"/graph/"+strconv.Itoa(i), mustJSON(t, r.graph)))
		c.Proofs = append(c.Proofs, w.store.put("q/"+id+"/proof/"+strconv.Itoa(i), r.proof))
	}
	harness := InvocationV2{Profile: "postmerge-owned-invocation/2", ExecutionID: runs[0].graph.ExecutionID, Purpose: "host-supervisor",
		Executable: w.exe["supervisor"], Argv: []string{"supervisor", "qualify", id}, Environment: []EnvironmentV2{},
		StdinSHA256: noArtifact.SHA256}
	c.HarnessInvocation = w.store.put("q/"+id+"/harness", mustJSON(t, harness))
	c.HarnessStdout = w.store.put("q/"+id+"/stdout", []byte("PASS "+id+"\n"))
	return c
}

// addDuplicateRunner appends a second owned runner birth in context 1, so two
// births derive the same logical node.
func (f *procFixture) addDuplicateRunner() {
	extra := f.procs["runner"]
	extra.pid, extra.start, extra.argv = extra.pid+40, extra.start+40, []string{"runner", "t2"}
	f.procs["runner-2"] = extra
	index := len(f.proof.Captures)
	f.proof.Captures = append(f.proof.Captures, f.w.live(index, "during-run", extra))
	f.proof.Launches = append(f.proof.Launches, OwnedLaunchV2{Index: len(f.proof.Launches), ContextIndex: 1, Purpose: "runner",
		ParentCaptureIndex: intPtr(2), StartCaptureIndex: index, ExecutedCaptureIndex: index})
	f.reinvoke(len(f.proof.Launches)-1, "runner-2", nil)
}
