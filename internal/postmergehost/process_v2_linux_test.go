//go:build linux && (amd64 || arm64)

// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/postmergeproof"
)

type memRetainV2 map[string][]byte

func (m memRetainV2) RetainProcessArtifactV2(id string, data []byte) (postmergeproof.ArtifactRefV2, error) {
	m[id] = bytes.Clone(data)
	sum := sha256.Sum256(data)
	return postmergeproof.ArtifactRefV2{ID: id, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}, nil
}

func TestProcessCollectorV2RefusesMisuse(t *testing.T) {
	ctx := context.Background()
	c, err := NewProcessCollectorV2(ctx, "misuse", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.proof.Captures) != 1 || len(c.proof.Launches) != 1 || c.proof.Launches[0].Purpose != "host-supervisor" ||
		c.proof.Launches[0].ParentCaptureIndex != nil || c.SupervisorCapture() != 0 {
		t.Fatalf("supervisor boundary: %+v", c.proof.Launches)
	}
	spec := OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"}
	inherit := exec.Command("/bin/sh", "-c", "exit 0")
	preset := exec.Command("/bin/sh", "-c", "exit 0")
	preset.Env, preset.Stdin = []string{}, strings.NewReader("x")
	secret := exec.Command("/bin/sh", "-c", "exit 0")
	secret.Env = []string{"GITHUB_TOKEN=x"}
	parentless := exec.Command("/bin/sh", "-c", "exit 0")
	parentless.Env = []string{}
	for name, cmd := range map[string]*exec.Cmd{"inherited env": inherit, "preset stdin": preset, "secret env": secret} {
		if _, err := c.Start(ctx, cmd, spec, nil); err == nil || cmd.Process != nil {
			t.Fatalf("%s: started", name)
		}
	}
	for _, bad := range []OwnedLaunchSpecV2{{Purpose: "runner", Parent: 7, Phase: "during-run"}, {Purpose: "runner", Phase: "final-sweep"},
		{Purpose: "runner", Phase: "retirement"}, {Phase: "during-run"}} {
		_, err := c.Start(ctx, parentless, bad, nil)
		expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
	}
	if parentless.Process != nil {
		t.Fatal("refused launch started")
	}
	_, err = c.Proof()
	expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
	err = c.FinalSweep(ctx)
	expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
}

func TestProcessObserverV2Modes(t *testing.T) {
	ctx := context.Background()
	for _, args := range [][]string{nil, {"replay"}, {"absence-sweep"}, {"absence-sweep", ""}, {"trusted-start", "x"}} {
		var out bytes.Buffer
		err := RunProcessObserverV2(ctx, args, strings.NewReader(""), &out)
		expectProcessCode(t, err, "BLOCKED", "process-observer-refused")
		if out.Len() != 0 {
			t.Fatalf("%v wrote output", args)
		}
	}
	var out bytes.Buffer
	err := RunProcessObserverV2(ctx, []string{"absence-sweep", "x"}, strings.NewReader("early"), &out)
	expectProcessCode(t, err, "BLOCKED", "process-observer-refused")

	self, err := selfExecutableSHA256V2()
	if err != nil {
		t.Fatal(err)
	}
	start := postmergeproof.TrustedStartV2{Profile: "postmerge-trusted-start/2", ExecutionID: "x", PolicySHA256: strings.Repeat("1", 64),
		RequestSHA256: strings.Repeat("2", 64), SupervisorSHA256: self, ObserverSHA256: strings.Repeat("3", 64),
		HostTupleSHA256: strings.Repeat("4", 64), RootCaptureIndex: 2}
	data, _ := json.Marshal(start)
	err = RunProcessObserverV2(ctx, []string{"trusted-start"}, bytes.NewReader(data), &out)
	expectProcessCode(t, err, "BLOCKED", "process-observer-refused")
	err = RunProcessObserverV2(ctx, []string{"trusted-start"}, strings.NewReader(`{"profile":"postmerge-trusted-start/2","extra":1}`), &out)
	expectProcessCode(t, err, "REJECTED", "process-wire-invalid")
	start.ObserverSHA256 = self
	data, _ = json.Marshal(start)
	if err := RunProcessObserverV2(ctx, []string{"trusted-start"}, bytes.NewReader(data), &out); err != nil || !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("trusted start: %v %s", err, out.Bytes())
	}
	out.Reset()
	if err := RunProcessObserverV2(ctx, []string{"absence-sweep", "x"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	var sweep postmergeproof.AbsenceSweepDocumentV2
	if err := json.Unmarshal(out.Bytes(), &sweep); err != nil || sweep.ExecutionID != "x" || len(sweep.Processes) == 0 {
		t.Fatalf("absence sweep: %v %+v", err, sweep.ExecutionID)
	}
}

// hookRetainV2 runs before on one artifact ID and then fails or retains it.
type hookRetainV2 struct {
	memRetainV2
	id     string
	before func()
	fail   bool
}

func (h hookRetainV2) RetainProcessArtifactV2(id string, data []byte) (postmergeproof.ArtifactRefV2, error) {
	if id == h.id {
		h.before()
		if h.fail {
			return postmergeproof.ArtifactRefV2{}, errors.New("retention unavailable for " + id)
		}
	}
	return h.memRetainV2.RetainProcessArtifactV2(id, data)
}

func shellV2(t *testing.T, script string) *exec.Cmd {
	t.Helper()
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("NOT_OBSERVED: no sleep: " + err.Error())
	}
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Env = []string{"PATH=" + sleep[:strings.LastIndexByte(sleep, '/')] + ":/usr/bin:/bin"}
	return cmd
}

// findProcessV2 waits for the process whose argv is exactly argv.
func findProcessV2(argv ...string) (procBirthV2, bool) {
	want := argvBytesV2(argv)
	for deadline := time.Now().Add(processSettleV2); time.Now().Before(deadline); time.Sleep(processPollV2) {
		entries, _ := os.ReadDir("/proc")
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}
			if cmdline, _ := os.ReadFile("/proc/" + entry.Name() + "/cmdline"); bytes.Equal(cmdline, want) {
				if state, _, start, ok := processStatV2(pid); ok && state != 'Z' {
					return procBirthV2{pid: pid, start: start}, true
				}
			}
		}
	}
	return procBirthV2{}, false
}

func runningV2(b procBirthV2) bool {
	state, _, start, ok := processStatV2(b.pid)
	return ok && start == b.start && state != 'Z' && state != 'X'
}

// A parent the collector reaped, a retirement capture and a PID that names
// another birth (the controlled PID-reuse case: the parent capture's
// starttime no longer matches the live PID) are never awaited parents.
func TestProcessCollectorV2AwaitChildRefusesReusedParent(t *testing.T) {
	ctx := context.Background()
	c, err := NewProcessCollectorV2(ctx, "reuse", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	sleep, _ := exec.LookPath("sleep")
	exited := shellV2(t, "read x; exit 0")
	p, err := c.Start(ctx, exited, OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Retire(ctx, p); err != nil {
		t.Fatal(err)
	}
	retirement := *c.proof.Launches[p.Launch].RetirementCaptureIndex
	for _, parent := range []int{p.Capture, retirement} {
		_, err := c.AwaitChild(ctx, sleep, []string{"sleep", "30"}, exited.Env, nil, OwnedLaunchSpecV2{Purpose: "runner", ContextIndex: 1, Parent: parent, Phase: "during-run"})
		expectProcessCode(t, err, "BLOCKED", "process-collection-refused")
	}

	live := shellV2(t, "sleep 32.5 & wait")
	q, err := c.Start(ctx, live, OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	child, ok := findProcessV2("sleep", "32.5")
	if !ok {
		t.Fatal("no sleep child")
	}
	stat := c.proof.Captures[q.Capture].StatBefore
	end := bytes.LastIndexByte(stat, ')')
	fields := strings.Fields(string(stat[end+1:]))
	start, _ := strconv.ParseUint(fields[19], 10, 64)
	fields[19] = strconv.FormatUint(start+1, 10)
	c.proof.Captures[q.Capture].StatBefore = []byte(string(stat[:end+1]) + " " + strings.Join(fields, " ") + "\n")
	_, err = c.AwaitChild(ctx, sleep, []string{"sleep", "32.5"}, live.Env, nil, OwnedLaunchSpecV2{Purpose: "runner", ContextIndex: 1, Parent: q.Capture, Phase: "during-run"})
	expectProcessCode(t, err, "BLOCKED", "process-collection-failed")
	if len(c.proof.Launches) != 3 {
		t.Fatalf("a refused child was recorded: %+v", c.proof.Launches)
	}

	// The owned cleanup boundary retires the live tree, descendants first.
	_ = c.abandonFailed(q, errors.New("test cleanup"))
	if live.ProcessState == nil || runningV2(child) {
		t.Fatal("owned tree survived its cleanup")
	}
	_, err = c.Proof()
	expectProcessCode(t, err, "BLOCKED", "process-collection-failed")
}

// A retention failure after the launched shell forked retires the forked
// descendant as well as the leader, and Start returns no handle.
func TestProcessCollectorV2FailedLaunchRetiresDescendants(t *testing.T) {
	ctx := context.Background()
	var child procBirthV2
	var found bool
	retain := hookRetainV2{memRetainV2: memRetainV2{}, id: "fork/launch/1/invocation", fail: true,
		before: func() { child, found = findProcessV2("sleep", "31.5") }}
	c, err := NewProcessCollectorV2(ctx, "fork", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, retain)
	if err != nil {
		t.Fatal(err)
	}
	cmd := shellV2(t, "sleep 31.5 & wait")
	p, err := c.Start(ctx, cmd, OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"}, nil)
	expectProcessCode(t, err, "BLOCKED", "process-collection-failed")
	if p != nil || !found {
		t.Fatalf("handle %v, forked child found %v", p, found)
	}
	if cmd.ProcessState == nil || runningV2(child) {
		t.Fatal("failed launch left its leader or forked descendant running")
	}
	_, err = c.Proof()
	expectProcessCode(t, err, "BLOCKED", "process-collection-failed")
}

// Cancellation ends a stdin delivery that a child never reads and retires
// the child within the cleanup bound.
func TestProcessCollectorV2CancelledDeliveryRetires(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	retain := hookRetainV2{memRetainV2: memRetainV2{}, id: "blocked/launch/1/invocation",
		before: func() { time.AfterFunc(200*time.Millisecond, cancel) }}
	c, err := NewProcessCollectorV2(context.Background(), "blocked", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, retain)
	if err != nil {
		t.Fatal(err)
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("NOT_OBSERVED: no sleep: " + err.Error())
	}
	cmd := exec.Command(sleep, "30")
	cmd.Env = []string{}
	began := time.Now()
	p, err := c.Start(ctx, cmd, OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"},
		bytes.Repeat([]byte("x"), processStdinLimitV2))
	expectProcessCode(t, err, "BLOCKED", "process-collection-failed")
	if p != nil || cmd.ProcessState == nil || time.Since(began) > processSettleV2 {
		t.Fatalf("blocked delivery: handle %v, reaped %v, after %s", p, cmd.ProcessState != nil, time.Since(began))
	}
}

// A cancelled context, which the launcher derives from SIGTERM, ends an
// observer blocked on its stdin.
func TestProcessObserverV2CancelledInput(t *testing.T) {
	for _, args := range [][]string{{"trusted-start"}, {"absence-sweep", "x"}} {
		reader, writer := io.Pipe()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		var out bytes.Buffer
		err := RunProcessObserverV2(ctx, args, reader, &out)
		cancel()
		_ = writer.Close()
		expectProcessCode(t, err, "BLOCKED", "process-observer-refused")
		if out.Len() != 0 {
			t.Fatalf("%v wrote output", args)
		}
	}
}

// The observer command is parent-owned: the collector keeps its own copy.
func TestProcessCollectorV2OwnsObserverCommand(t *testing.T) {
	args := []string{"--internal-process-observer"}
	c, err := NewProcessCollectorV2(context.Background(), "owned-observer", "g", "p", ObserverCommandV2{Path: "/proc/self/exe", Args: args}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	args[0] = "--internal-envelope"
	if c.observer.Args[0] != "--internal-process-observer" {
		t.Fatal("observer argv follows caller mutation")
	}
}
