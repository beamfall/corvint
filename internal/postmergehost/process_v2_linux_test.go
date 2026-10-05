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
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

func startOwnedV2(t *testing.T, c *ProcessCollectorV2, cmd *exec.Cmd) *OwnedProcessV2 {
	t.Helper()
	p, err := c.Start(context.Background(), cmd, OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return p
}

func killBirthV2(b procBirthV2) {
	if runningV2(b) {
		_ = syscall.Kill(b.pid, syscall.SIGKILL)
	}
}

// listedHookV2 installs the walk's test hook for one test.
func listedHookV2(t *testing.T, hook func(member int, children []procBirthV2) []procBirthV2) {
	retireTreeListedV2 = hook
	t.Cleanup(func() { retireTreeListedV2 = nil })
}

func expectUnresolvedV2(t *testing.T, err error, detail string) {
	t.Helper()
	expectProcessCode(t, err, "BLOCKED", "process-collection-failed")
	if !strings.Contains(err.Error(), "owned cleanup: unresolved:") || !strings.Contains(err.Error(), detail) {
		t.Fatalf("cleanup not reported unresolved with %q: %v", detail, err)
	}
}

// The walk never signals or walks a PID whose process is not the listed
// birth. The hook gives the listed child another start time, the controlled
// stand-in for a PID recycled between listing and signalling: the real
// process is neither stopped nor killed and the cleanup is unresolved.
func TestProcessCollectorV2CleanupSkipsRecycledPID(t *testing.T) {
	c, err := NewProcessCollectorV2(context.Background(), "recycled", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	cmd := shellV2(t, "sleep 31.1 & wait")
	p := startOwnedV2(t, c, cmd)
	child, ok := findProcessV2("sleep", "31.1")
	if !ok {
		t.Fatal("no sleep child")
	}
	t.Cleanup(func() { killBirthV2(child) })
	listedHookV2(t, func(_ int, children []procBirthV2) []procBirthV2 {
		for i := range children {
			if children[i] == child {
				children[i].start++
			}
		}
		return children
	})
	expectUnresolvedV2(t, c.abandonFailed(p, errors.New("test cleanup")), "is no longer its listed birth")
	if state, _, start, ok := processStatV2(child.pid); !ok || start != child.start || state == 'T' || state == 'Z' {
		t.Fatalf("the stand-in process was signalled: state %c", state)
	}
	if cmd.ProcessState == nil {
		t.Fatal("leader not reaped")
	}
}

// A child of a parent that ignores SIGCHLD is reaped as soon as it exits,
// freeing its PID. One that exits between listing and freezing is reported
// unresolved, never treated as retired.
func TestProcessCollectorV2CleanupReportsAutoReapedChild(t *testing.T) {
	perl, err := exec.LookPath("perl")
	if err != nil {
		t.Skip("NOT_OBSERVED: no perl: " + err.Error())
	}
	c, err := NewProcessCollectorV2(context.Background(), "auto-reaped", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(perl, "-e", `$SIG{CHLD} = "IGNORE"; exec "sleep", "31.2" unless fork; sleep 30`)
	cmd.Env = shellV2(t, "").Env
	p := startOwnedV2(t, c, cmd)
	child, ok := findProcessV2("sleep", "31.2")
	if !ok {
		t.Fatal("no sleep child")
	}
	t.Cleanup(func() { killBirthV2(child) })
	var reaped bool
	listedHookV2(t, func(member int, children []procBirthV2) []procBirthV2 {
		if member == p.pid && !reaped {
			_ = syscall.Kill(child.pid, syscall.SIGKILL)
			for deadline := time.Now().Add(processSettleV2); time.Now().Before(deadline) && !reaped; time.Sleep(processPollV2) {
				_, _, start, ok := processStatV2(child.pid)
				reaped = !ok || start != child.start
			}
		}
		return children
	})
	err = c.abandonFailed(p, errors.New("test cleanup"))
	if !reaped {
		t.Fatal("the child was not reaped automatically")
	}
	expectUnresolvedV2(t, err, "exited before its freeze was confirmed")
}

// A child that forks a grandchild and exits after it was listed but before
// it was frozen leaves the grandchild reparented outside the tree. Cleanup
// reports that unresolved instead of success.
func TestProcessCollectorV2CleanupReportsForkAndExit(t *testing.T) {
	c, err := NewProcessCollectorV2(context.Background(), "fork-exit", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(t.TempDir(), "release")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := shellV2(t, `sh -c 'read x <"$0"; sleep 31.3 & exit 0' `+fifo+` & sleep 30.5 & wait`)
	p := startOwnedV2(t, c, cmd)
	var grandchild procBirthV2
	var released bool
	t.Cleanup(func() { killBirthV2(grandchild) })
	listedHookV2(t, func(member int, children []procBirthV2) []procBirthV2 {
		if member != p.pid || released {
			return children
		}
		released = true
		if err := os.WriteFile(fifo, []byte("go\n"), 0o600); err != nil {
			t.Error(err)
			return children
		}
		grandchild, _ = findProcessV2("sleep", "31.3")
		for deadline := time.Now().Add(processSettleV2); time.Now().Before(deadline); time.Sleep(processPollV2) {
			for _, child := range children {
				if state, _, _, _ := processStatV2(child.pid); state == 'Z' {
					return children
				}
			}
		}
		t.Error("the released child did not exit")
		return children
	})
	expectUnresolvedV2(t, c.abandonFailed(p, errors.New("test cleanup")), "exited before its freeze was confirmed")
	if !runningV2(grandchild) {
		t.Fatal("the fixture's grandchild did not escape, so the case was not exercised")
	}
}

// TestProcessHelperV2 is a multithreaded Go process for the cleanup tests;
// it runs only when re-executed with the helper argument.
func TestProcessHelperV2(t *testing.T) {
	if flag.Arg(0) != "process-helper-sleep" {
		t.Skip("helper process only")
	}
	time.Sleep(30 * time.Second)
}

// A multithreaded member counts as frozen only once every thread is
// stopped, and such a member is still retired cleanly.
func TestProcessCollectorV2CleanupFreezesAllThreads(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewProcessCollectorV2(context.Background(), "threads", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, memRetainV2{})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^TestProcessHelperV2$", "--", "process-helper-sleep")
	cmd.Env = []string{}
	p := startOwnedV2(t, c, cmd)
	threads := 0
	for deadline := time.Now().Add(processSettleV2); time.Now().Before(deadline) && threads < 2; time.Sleep(processPollV2) {
		tasks, _ := os.ReadDir("/proc/" + strconv.Itoa(p.pid) + "/task")
		threads = len(tasks)
	}
	if threads < 2 {
		t.Fatal("helper is not multithreaded")
	}
	if err := c.abandonFailed(p, errors.New("test cleanup")); err == nil || err.Error() != "test cleanup" {
		t.Fatalf("multithreaded cleanup: %v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("leader not reaped")
	}
}

// An escaped process that holds the leader's output open cannot stall the
// leader's reaping beyond the Wait delay.
func TestProcessCollectorV2EscapedOutputDoesNotStallWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var orphan procBirthV2
	t.Cleanup(func() { killBirthV2(orphan) })
	retain := hookRetainV2{memRetainV2: memRetainV2{}, id: "held/launch/1/invocation", before: func() {
		orphan, _ = findProcessV2("sleep", "31.7")
		time.AfterFunc(200*time.Millisecond, cancel)
	}}
	c, err := NewProcessCollectorV2(context.Background(), "held", "g", "p", ObserverCommandV2{Path: "/proc/self/exe"}, retain)
	if err != nil {
		t.Fatal(err)
	}
	cmd := shellV2(t, "(sleep 31.7 &); sleep 30")
	cmd.Stdout = new(bytes.Buffer)
	began := time.Now()
	p, err := c.Start(ctx, cmd, OwnedLaunchSpecV2{Purpose: "workflow-root", Parent: c.SupervisorCapture(), Phase: "start-barrier"},
		bytes.Repeat([]byte("x"), processStdinLimitV2))
	expectProcessCode(t, err, "BLOCKED", "process-collection-failed")
	if p != nil || cmd.ProcessState == nil || time.Since(began) > processSettleV2+2*processWaitDelayV2 {
		t.Fatalf("held output: handle %v, reaped %v, after %s", p, cmd.ProcessState != nil, time.Since(began))
	}
	if !runningV2(orphan) {
		t.Fatal("no escaped process held the output, so the case was not exercised")
	}
}
