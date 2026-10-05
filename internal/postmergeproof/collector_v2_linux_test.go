//go:build linux && (amd64 || arm64)

// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/postmergehost"
	"github.com/Beamfall/corvint/internal/postmergeproof"
)

// collectorStore is in-memory parent-owned retention for the collector and
// the artifact reader for the verifier.
type collectorStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (s *collectorStore) RetainProcessArtifactV2(id string, data []byte) (postmergeproof.ArtifactRefV2, error) {
	return s.put(id, data), nil
}

func (s *collectorStore) put(id string, data []byte) postmergeproof.ArtifactRefV2 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = bytes.Clone(data)
	sum := sha256.Sum256(data)
	return postmergeproof.ArtifactRefV2{ID: id, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}
}

func (s *collectorStore) ReadArtifact(id, _ string, _ int64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.data[id]
	if !ok {
		return nil, errors.New("missing artifact " + id)
	}
	return bytes.Clone(data), nil
}

func marshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fileArtifact(t *testing.T, store *collectorStore, path string) postmergeproof.ArtifactRefV2 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("NOT_OBSERVED: " + path + ": " + err.Error())
	}
	sum := sha256.Sum256(data)
	return store.put("exe/"+hex.EncodeToString(sum[:]), data)
}

// collectorRun is one owned execution collected by the host collector with
// this test binary as both supervisor and pinned observer. The native report,
// request and graph are synthetic: this is collector evidence on this host,
// never a host tuple qualification.
type collectorRun struct {
	store     *collectorStore
	host      postmergeproof.HostTupleV2
	impl      postmergeproof.ImplementationV2
	policy    postmergeproof.ProcessPolicyV2
	policyRef postmergeproof.ArtifactRefV2
	graph     postmergeproof.GraphBindingV2
	proof     postmergeproof.ProcessProofV2
	bytes     []byte
}

func collect(t *testing.T, name string, retireRunner bool) *collectorRun {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	sh, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Skip("NOT_OBSERVED: no /bin/sh: " + err.Error())
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("NOT_OBSERVED: no sleep: " + err.Error())
	}
	r := &collectorRun{store: &collectorStore{data: map[string][]byte{}}}
	exe := map[string]postmergeproof.ArtifactRefV2{"self": fileArtifact(t, r.store, "/proc/self/exe"),
		"sh": fileArtifact(t, r.store, sh), "sleep": fileArtifact(t, r.store, sleep)}
	release, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		t.Fatal(err)
	}
	digest := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	r.host = postmergeproof.HostTupleV2{OS: "linux", Architecture: runtime.GOARCH, KernelRelease: strings.TrimSpace(string(release)),
		ImageManifestSHA256: digest("image"), LauncherPolicySHA256: digest("launcher")}
	r.impl = postmergeproof.ImplementationV2{SourceCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
		VerifierSHA256: digest("verifier"), ObserverSHA256: exe["self"].SHA256, SupervisorSHA256: exe["self"].SHA256}
	owned := func(role, executable, parent string) postmergeproof.RoleRuleV2 {
		return postmergeproof.RoleRuleV2{Role: role, Derivation: "owned-launch/0", Executable: exe[executable], LaunchPurpose: role,
			ParentRole: parent, Slot: "default", RequiredSwitches: []string{}, ForbiddenSwitches: []string{}}
	}
	r.policy = postmergeproof.ProcessPolicyV2{Profile: "postmerge-process-policy/2", Implementation: r.impl, Host: r.host,
		Rules: []postmergeproof.RoleRuleV2{owned("host-supervisor", "self", "outside"), owned("workflow-root", "sh", "host-supervisor"),
			owned("runner", "sleep", "workflow-root"), owned("observer", "self", "host-supervisor")},
		Observation: postmergeproof.ObservationPolicyV2{Profile: "linux-procfs-birth-command/0", Scope: "observed-owned-process-tree",
			TargetIntervalMS: 20, CaptureLimit: 64, ProcessLimit: 64, RequirePairedStat: true, RequireFinalIndependentSweep: true,
			Limitations: []string{}}}
	r.policyRef = r.store.put("policy", marshal(t, r.policy))
	request := r.store.put(name+"/request", []byte("request "+name))
	report := r.store.put(name+"/report", marshal(t, map[string]any{"controls": []any{}, "runs": []any{map[string]any{
		"kind": "repeat", "ordinal": 0,
		"cleanup": map[string]any{"owned_group": true, "status": "owned-process-group", "qualification": "process-group",
			"cancelled": false, "timed_out": false,
			"descendants": map[string]any{"scope": "session", "interval_ms": 20, "absent": true, "failures": []any{},
				"limitations": []any{}, "processes": []any{}}}}}}))
	zero := 0
	r.graph = postmergeproof.GraphBindingV2{Profile: "postmerge-native-graph-binding/2", ExecutionID: name, Request: request, Report: report,
		ProductCommit: strings.Repeat("1", 40), TestCommit: strings.Repeat("2", 40), DraftCommit: strings.Repeat("3", 40),
		Sources: []postmergeproof.SourceBindingV2{{Kind: "report", Artifact: report}, {Kind: "request", Artifact: request}},
		Contexts: []postmergeproof.RunContextV2{{RunKind: "workflow", TestIDs: []string{}},
			{RunIndex: &zero, RunKind: "repeat", RunOrdinal: 0, TestIDs: []string{"t1"}}}}
	graphSHA, err := postmergeproof.GraphBindingSHA256V2(r.graph)
	if err != nil {
		t.Fatal(err)
	}
	hostSHA, err := postmergeproof.HostTupleSHA256V2(r.host)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	c, err := postmergehost.NewProcessCollectorV2(ctx, name, graphSHA, r.policyRef.SHA256,
		postmergehost.ObserverCommandV2{Path: self, Args: []string{"--internal-process-observer"}}, r.store)
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := c.StartTrustedObserver(ctx)
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + filepath.Dir(sleep) + ":/usr/bin:/bin"}
	rootCmd := exec.Command(sh, "-c", "sleep 30 & wait")
	rootCmd.Env = env
	root, err := c.Start(ctx, rootCmd, postmergehost.OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if rootCmd.ProcessState == nil {
			_ = killChild(rootCmd.Process.Pid)
			_ = rootCmd.Process.Kill()
			_ = rootCmd.Wait()
		}
	})
	runner, err := c.AwaitChild(ctx, sleep, []string{"sleep", "30"}, env, nil,
		postmergehost.OwnedLaunchSpecV2{Purpose: "runner", ContextIndex: 1, Parent: root.Capture, Phase: "during-run"})
	if err != nil {
		t.Fatal(err)
	}
	if retireRunner {
		c.SetOutcome(runner, true, false)
		if err := killChild(rootCmd.Process.Pid); err != nil {
			t.Fatal(err)
		}
		if err := c.Retire(ctx, root); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.CompleteTrustedStart(ctx, trusted, postmergeproof.TrustedStartV2{Profile: "postmerge-trusted-start/2",
		ExecutionID: name, PolicySHA256: r.policyRef.SHA256, RequestSHA256: request.SHA256, SupervisorSHA256: r.impl.SupervisorSHA256,
		ObserverSHA256: r.impl.ObserverSHA256, HostTupleSHA256: hostSHA, RootCaptureIndex: root.Capture}); err != nil {
		t.Fatal(err)
	}
	c.AddCleanupJoin(postmergeproof.CleanupJoinV2{ArtifactID: report.ID, Pointer: "/runs/0/cleanup", ContextIndex: 1,
		Facts: postmergeproof.CleanupFactsV2{OwnedGroup: true, Status: "owned-process-group", DerivedState: "observed-absent",
			DescendantsPresent: true, Absent: true, Qualification: "process-group", Scope: "session", IntervalMS: 20,
			Failures: []string{}, Limitations: []string{}}})
	if err := c.FinalSweep(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(ctx, exec.Command(sleep, "1"), postmergehost.OwnedLaunchSpecV2{Purpose: "runner", Parent: root.Capture, Phase: "during-run"}, nil); err == nil {
		t.Fatal("a launch after the final sweep was accepted")
	}
	if r.bytes, err = c.Proof(); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(r.bytes, &r.proof); err != nil {
		t.Fatal(err)
	}
	return r
}

// killChild terminates the workflow root's sleep child so the root's wait
// returns: the runner is cancelled, the root completes.
func killChild(parent int) error {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		stat, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if err != nil {
			continue
		}
		end := bytes.LastIndexByte(stat, ')')
		fields := strings.Fields(string(stat[end+1:]))
		if pid, err := strconv.Atoi(entry.Name()); err == nil && len(fields) > 1 && fields[1] == strconv.Itoa(parent) {
			return syscall.Kill(pid, syscall.SIGTERM)
		}
	}
	return errors.New("workflow root has no child")
}

func TestHostCollectorV2OwnedExecution(t *testing.T) {
	r := collect(t, "collector-owned", true)
	logical, err := postmergeproof.VerifyRawLogicalForTestV2(context.Background(), r.policy, r.policyRef.SHA256, r.graph, r.bytes, r.store)
	if err != nil {
		t.Fatal(err)
	}
	rootRetirement := r.proof.Launches[2].RetirementCaptureIndex
	if r.proof.Launches[2].Purpose != "workflow-root" || rootRetirement == nil {
		t.Fatalf("launch order: %+v", r.proof.Launches)
	}
	facts, err := postmergeproof.ExitFactsForTestV2(r.proof.Captures[*rootRetirement])
	if err != nil {
		t.Fatal(err)
	}
	supervisor, root := "proc/0/host-supervisor/default", "proc/0/workflow-root/default"
	want := []postmergeproof.LogicalProcessV2{
		{Node: supervisor, Role: "host-supervisor", RunKind: "workflow", State: "outside-workload/exit:unknown", BirthDistinctFrom: []string{}},
		{Node: root, Role: "workflow-root", RunKind: "workflow", Parent: &supervisor, State: "completed/" + facts, BirthDistinctFrom: []string{}},
		{Node: "proc/1/runner/default", Role: "runner", RunKind: "repeat", Parent: &root, State: "cancelled/unreaped", BirthDistinctFrom: []string{}},
	}
	if !reflect.DeepEqual(logical, want) {
		t.Fatalf("logical graph:\n got %+v\nwant %+v", logical, want)
	}
	t.Logf("%d captures, %d launches, %d swept rows, root %s", len(r.proof.Captures), len(r.proof.Launches),
		len(r.proof.FinalSweep.Processes), want[1].State)
	// Collected bytes still mint no token without an admitted qualification.
	admission := postmergeproof.ProcessAdmissionV2{Policy: r.policyRef, HostTuple: r.host, Implementation: r.impl}
	_, err = postmergeproof.VerifyProcessV2(context.Background(), admission, r.graph, marshal(t, r.policy), nil, r.bytes, r.store)
	expectProcessRefusal(t, err, "BLOCKED", "process-qualification-unavailable")
}

func TestHostCollectorV2SweepFindsSurvivor(t *testing.T) {
	r := collect(t, "collector-survivor", false)
	_, err := postmergeproof.VerifyRawLogicalForTestV2(context.Background(), r.policy, r.policyRef.SHA256, r.graph, r.bytes, r.store)
	expectProcessRefusal(t, err, "REJECTED", "process-sweep-survivor")
}

func expectProcessRefusal(t *testing.T, err error, outcome, code string) {
	t.Helper()
	var refusal *postmergeproof.ProcessErrorV2
	if !errors.As(err, &refusal) || refusal.Outcome != outcome || refusal.Code != code {
		t.Fatalf("got %v, want %s %s", err, outcome, code)
	}
}
