//go:build linux && (amd64 || arm64)

// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/postmergeproof/procfs"
)

// linuxRun drives real owned processes on this host and records their births
// through package procfs. The native report, request and graph are synthetic;
// this is unit and procfs evidence, never a host qualification.
type linuxRun struct {
	t         *testing.T
	name      string
	store     *memStore
	exe       map[string]ArtifactRefV2
	paths     map[string]string
	host      HostTupleV2
	impl      ImplementationV2
	policy    ProcessPolicyV2
	policyRef ArtifactRefV2
	graph     GraphBindingV2
	proof     ProcessProofV2
	cleanup   []func()
}

func newLinuxRun(t *testing.T, name string) *linuxRun {
	r := &linuxRun{t: t, name: name, store: &memStore{data: map[string][]byte{}}, exe: map[string]ArtifactRefV2{}, paths: map[string]string{}}
	t.Cleanup(func() {
		for i := len(r.cleanup) - 1; i >= 0; i-- {
			r.cleanup[i]()
		}
	})
	sh, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Skip("NOT_OBSERVED: no /bin/sh: " + err.Error())
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("NOT_OBSERVED: no sleep: " + err.Error())
	}
	r.paths = map[string]string{"supervisor": "/proc/self/exe", "observer": "/bin/cat", "sh": sh, "runner": sleep}
	for role, path := range r.paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skip("NOT_OBSERVED: " + role + " executable: " + err.Error())
		}
		r.exe[role] = r.artifact(data)
	}
	release, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		t.Fatal(err)
	}
	r.host = HostTupleV2{OS: "linux", Architecture: runtime.GOARCH, KernelRelease: strings.TrimSpace(string(release)),
		ImageManifestSHA256: sha256Hex([]byte("image")), LauncherPolicySHA256: sha256Hex([]byte("launcher"))}
	r.impl = ImplementationV2{SourceCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
		VerifierSHA256: sha256Hex([]byte("verifier")), ObserverSHA256: r.exe["observer"].SHA256, SupervisorSHA256: r.exe["supervisor"].SHA256}
	owned := func(role, exe, parent string) RoleRuleV2 {
		return RoleRuleV2{Role: role, Derivation: "owned-launch/0", Executable: r.exe[exe], LaunchPurpose: role, ParentRole: parent,
			Slot: "default", RequiredSwitches: []string{}, ForbiddenSwitches: []string{}}
	}
	r.policy = ProcessPolicyV2{Profile: "postmerge-process-policy/2", Implementation: r.impl, Host: r.host,
		Rules: []RoleRuleV2{owned("host-supervisor", "supervisor", "outside"), owned("workflow-root", "sh", "host-supervisor"),
			owned("runner", "runner", "workflow-root"), owned("observer", "observer", "host-supervisor")},
		Observation: ObservationPolicyV2{Profile: "linux-procfs-birth-command/0", Scope: "observed-owned-process-tree",
			TargetIntervalMS: 20, CaptureLimit: 64, ProcessLimit: 64, RequirePairedStat: true, RequireFinalIndependentSweep: true,
			Limitations: []string{}}}
	r.policyRef = r.store.put("policy", mustJSON(t, r.policy))
	request := r.store.put(name+"/request", []byte("request "+name))
	report := r.store.put(name+"/report", mustJSON(t, map[string]any{"controls": []any{}, "runs": []any{map[string]any{
		"kind": "repeat", "ordinal": 0,
		"cleanup": map[string]any{"owned_group": true, "status": "owned-process-group", "qualification": "process-group",
			"cancelled": false, "timed_out": false,
			"descendants": map[string]any{"scope": "session", "interval_ms": 20, "absent": true, "failures": []any{},
				"limitations": []any{}, "processes": []any{}}}}}}))
	r.graph = GraphBindingV2{Profile: "postmerge-native-graph-binding/2", ExecutionID: name, Request: request, Report: report,
		ProductCommit: strings.Repeat("1", 40), TestCommit: strings.Repeat("2", 40), DraftCommit: strings.Repeat("3", 40),
		Sources: []SourceBindingV2{{Kind: "report", Artifact: report}, {Kind: "request", Artifact: request}},
		Contexts: []RunContextV2{
			{RunKind: "workflow", TestIDs: []string{}},
			{RunIndex: intPtr(0), RunKind: "repeat", RunOrdinal: 0, TestIDs: []string{"t1"}},
		}}
	graphSHA, err := GraphBindingSHA256V2(r.graph)
	if err != nil {
		t.Fatal(err)
	}
	r.proof = ProcessProofV2{Profile: "postmerge-process-proof/2", ExecutionID: name, GraphBindingSHA256: graphSHA,
		PolicySHA256: r.policyRef.SHA256, Captures: []ProcCaptureV2{}, Launches: []OwnedLaunchV2{},
		NativeReferences: []NativeProcessReferenceV2{},
		Cleanup: []CleanupJoinV2{{ArtifactID: report.ID, Pointer: "/runs/0/cleanup", ContextIndex: 1, Facts: CleanupFactsV2{OwnedGroup: true,
			Status: "owned-process-group", DerivedState: "observed-absent", DescendantsPresent: true, Absent: true,
			Qualification: "process-group", Scope: "session", IntervalMS: 20, Failures: []string{}, Limitations: []string{}}}}}
	return r
}

func (r *linuxRun) artifact(data []byte) ArtifactRefV2 {
	if len(data) == 0 {
		return noArtifact
	}
	return r.store.put("exe/"+sha256Hex(data), data)
}

func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

// capture records one real procfs birth capture and returns its index.
func (r *linuxRun) capture(phase string, pid int) int {
	c, err := procfs.CaptureBirth(context.Background(), pid, 1<<30)
	if err != nil {
		r.t.Fatalf("capture %d: %v", pid, err)
	}
	executable, after := r.artifact(c.Executable), noArtifact
	if c.ExecutableAfterSHA256 != "" {
		after = ArtifactRefV2{ID: "exe/" + c.ExecutableAfterSHA256, SHA256: c.ExecutableAfterSHA256, Bytes: c.ExecutableAfterBytes}
	}
	index := len(r.proof.Captures)
	r.proof.Captures = append(r.proof.Captures, ProcCaptureV2{Index: index, Phase: phase, BootIDBytes: c.BootID,
		NamespaceLinkBytes: c.NamespaceLink, NamespaceDevice: c.NamespaceDevice, NamespaceInode: c.NamespaceInode,
		StatBefore: c.StatBefore, Cmdline: nonNil(c.Cmdline), CmdlineAfter: nonNil(c.CmdlineAfter),
		ExecutableLinkBytes: nonNil(c.ExecutableLink), ExecutableLinkAfterBytes: nonNil(c.ExecutableLinkAfter),
		Executable: executable, ExecutableAfter: after, StatAfter: c.StatAfter,
		NativeStartOutput: nonNil(c.NativeStartOutput), NativeStartTool: r.artifact(c.NativeStartTool)})
	return index
}

// launch records owned launch metadata and its invocation preimage.
func (r *linuxRun) launch(purpose string, contextIndex int, parent *int, captureIndex int, exe string, argv []string) *OwnedLaunchV2 {
	i := len(r.proof.Launches)
	inv := InvocationV2{Profile: "postmerge-owned-invocation/2", ExecutionID: r.name, Purpose: purpose, Executable: r.exe[exe],
		Argv: argv, Environment: []EnvironmentV2{}, StdinSHA256: noArtifact.SHA256, ContextIndex: contextIndex, LaunchIndex: i}
	r.proof.Launches = append(r.proof.Launches, OwnedLaunchV2{Index: i, ContextIndex: contextIndex, Purpose: purpose,
		Invocation: r.store.put(r.name+"/invocation/"+strconv.Itoa(i), mustJSON(r.t, inv)), Stdin: noArtifact,
		ParentCaptureIndex: parent, StartCaptureIndex: captureIndex, ExecutedCaptureIndex: captureIndex})
	return &r.proof.Launches[i]
}

func (r *linuxRun) start(path string, args ...string) (*exec.Cmd, io.WriteCloser) {
	cmd := exec.Command(path, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		r.t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		r.t.Fatal(err)
	}
	r.cleanup = append(r.cleanup, func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	// Start returns once exec has closed the CLOEXEC pipe, before the kernel
	// publishes the new argv: a capture in that window reads an empty
	// cmdline and is refused as process-bracket-changed. A collector waits
	// for the exec to settle, as this harness does.
	argv := argvBytes(cmd.Args)
	waitForV2(r.t, "exec of "+path, func() bool {
		cmdline, _ := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/cmdline")
		return bytes.Equal(cmdline, argv)
	})
	return cmd, stdin
}

func waitForV2(t *testing.T, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !done(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for " + what)
		}
	}
}

func statOf(pid int) (procStatV2, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return procStatV2{}, false
	}
	s, err := parseProcStatV2(data)
	return s, err == nil
}

// childRunning finds the child of parent whose argv is exactly argv.
func childRunning(parent int, argv []string) int {
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		cmdline, _ := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if s, ok := statOf(pid); ok && s.ppid == parent && bytes.Equal(cmdline, argvBytes(argv)) {
			return pid
		}
	}
	return 0
}

// retire waits until pid is an unreaped zombie, captures it, reaps it and
// records the exact wait status on the retirement capture.
func (r *linuxRun) retire(cmd *exec.Cmd) int {
	pid := cmd.Process.Pid
	waitForV2(r.t, "zombie "+strconv.Itoa(pid), func() bool { s, ok := statOf(pid); return ok && s.zombie() })
	index := r.capture("retirement", pid)
	_ = cmd.Wait()
	status := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if status.Signaled() {
		r.proof.Captures[index].ExitSignal = intPtr(int(status.Signal()))
	} else {
		r.proof.Captures[index].ExitCode = intPtr(status.ExitStatus())
	}
	return index
}

func selfArgv(t *testing.T) []string {
	data, err := os.ReadFile("/proc/self/cmdline")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

// run performs one owned execution. With retireRunner false the runner and
// workflow root are still alive at the independent final sweep.
func (r *linuxRun) run(retireRunner bool) []byte {
	supervisor := r.capture("start-barrier", os.Getpid())
	r.launch("host-supervisor", 0, nil, supervisor, "supervisor", selfArgv(r.t))

	trusted, trustedStdin := r.start(r.paths["observer"])
	r.launch("observer", 0, intPtr(supervisor), r.capture("start-barrier", trusted.Process.Pid), "observer", trusted.Args)
	root, _ := r.start(r.paths["sh"], "-c", "sleep 30 & wait")
	rootCapture := r.capture("start-barrier", root.Process.Pid)
	rootLaunch := len(r.proof.Launches)
	r.launch("workflow-root", 0, intPtr(supervisor), rootCapture, "sh", root.Args)

	runnerArgv := []string{"sleep", "30"}
	var runner int
	waitForV2(r.t, "runner exec", func() bool { runner = childRunning(root.Process.Pid, runnerArgv); return runner != 0 })
	r.cleanup = append(r.cleanup, func() { _ = syscall.Kill(runner, syscall.SIGKILL) })
	runnerLaunch := r.launch("runner", 1, intPtr(rootCapture), r.capture("during-run", runner), "runner", runnerArgv)
	if retireRunner {
		runnerLaunch.Cancelled = true
		if err := syscall.Kill(runner, syscall.SIGTERM); err != nil {
			r.t.Fatal(err)
		}
		retirement := r.retire(root)
		r.proof.Launches[rootLaunch].RetirementCaptureIndex, r.proof.Launches[rootLaunch].Completed = intPtr(retirement), true
	}
	trustedStdin.Close()
	_ = trusted.Wait()

	hostSHA, err := HostTupleSHA256V2(r.host)
	if err != nil {
		r.t.Fatal(err)
	}
	r.proof.TrustedStartInvocation = r.proof.Launches[1].Invocation
	r.proof.TrustedStartStdout = r.store.put(r.name+"/trusted-start", mustJSON(r.t, TrustedStartV2{Profile: "postmerge-trusted-start/2",
		ExecutionID: r.name, PolicySHA256: r.policyRef.SHA256, RequestSHA256: r.graph.Request.SHA256,
		SupervisorSHA256: r.impl.SupervisorSHA256, ObserverSHA256: r.impl.ObserverSHA256, HostTupleSHA256: hostSHA,
		RootCaptureIndex: rootCapture}))

	sweeper, sweeperStdin := r.start(r.paths["observer"])
	sweepLaunch := r.launch("observer", 0, intPtr(supervisor), r.capture("final-sweep", sweeper.Process.Pid), "observer", sweeper.Args)
	sweep, err := procfs.SweepProcesses(context.Background(), 4096)
	if err != nil {
		r.t.Fatal(err)
	}
	sweeperStdin.Close()
	sweepLaunch.RetirementCaptureIndex, sweepLaunch.Completed = intPtr(r.retire(sweeper)), true

	rows := make([]SweepProcessV2, 0, len(sweep.Processes))
	for _, row := range sweep.Processes {
		rows = append(rows, SweepProcessV2{PID: row.PID, StatBytes: row.Stat})
	}
	r.proof.FinalSweep = AbsenceSweepV2{ObserverInvocation: sweepLaunch.Invocation, BootIDBytes: sweep.BootID,
		NamespaceLinkBytes: sweep.NamespaceLink, PIDDirectoryBytes: sweep.PIDDirectory, Processes: rows, ReadFailures: sweep.ReadFailures}
	r.proof.FinalSweep.ObserverStdout = r.store.put(r.name+"/sweep", mustJSON(r.t, AbsenceSweepDocumentV2{Profile: "postmerge-absence-sweep/2",
		ExecutionID: r.name, BootIDBytes: sweep.BootID, NamespaceLinkBytes: sweep.NamespaceLink, PIDDirectoryBytes: sweep.PIDDirectory,
		Processes: rows, ReadFailures: sweep.ReadFailures}))
	return mustJSON(r.t, r.proof)
}

func TestLinuxProcfsOwnedExecution(t *testing.T) {
	r := newLinuxRun(t, "linux-owned")
	proof := r.run(true)
	result, err := verifyRawProcessV2(context.Background(), r.policy, r.policyRef.SHA256, r.graph, proof, r.store)
	if err != nil {
		t.Fatal(err)
	}
	want := []LogicalProcessV2{
		{Node: "proc/0/host-supervisor/default", Role: "host-supervisor", RunKind: "workflow", State: "outside-workload/live", BirthDistinctFrom: []string{}},
		{Node: "proc/0/workflow-root/default", Role: "workflow-root", RunKind: "workflow", Parent: strPtr("proc/0/host-supervisor/default"),
			State: "completed/" + mustExitFacts(t, r.proof.Captures[4]), BirthDistinctFrom: []string{}},
		{Node: "proc/1/runner/default", Role: "runner", RunKind: "repeat", Parent: strPtr("proc/0/workflow-root/default"),
			State: "cancelled/unreaped", BirthDistinctFrom: []string{}},
	}
	if !reflect.DeepEqual(result.logical, want) {
		t.Fatalf("logical graph:\n got %+v\nwant %+v", result.logical, want)
	}
	t.Logf("%d captures, %d swept rows, root %s, native start %q", len(r.proof.Captures), len(r.proof.FinalSweep.Processes),
		want[1].State, r.proof.Captures[3].NativeStartOutput)
	// Without a qualification report the verifier mints no token.
	admission := ProcessAdmissionV2{Policy: r.policyRef, HostTuple: r.host, Implementation: r.impl}
	_, err = VerifyProcessV2(context.Background(), admission, r.graph, mustJSON(t, r.policy), nil, proof, r.store)
	expectRefusal(t, err, "BLOCKED", "process-qualification-unavailable")
}

func TestLinuxProcfsSweepFindsSurvivor(t *testing.T) {
	r := newLinuxRun(t, "linux-survivor")
	proof := r.run(false)
	_, err := verifyRawProcessV2(context.Background(), r.policy, r.policyRef.SHA256, r.graph, proof, r.store)
	expectRefusal(t, err, "REJECTED", "process-sweep-survivor")
	t.Log(err)
}

func mustExitFacts(t *testing.T, c ProcCaptureV2) string {
	facts, err := exitFactsV2(&c)
	if err != nil {
		t.Fatal(err)
	}
	return facts
}
