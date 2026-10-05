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
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/postmergeproof"
	"github.com/Beamfall/corvint/internal/postmergeproof/procfs"
)

// ProcessRetainerV2 is the parent-owned protected retention a process
// collector writes exact bytes to. It returns the reference the offline
// verifier later reads back through postmergeproof.ArtifactReaderV2.
type ProcessRetainerV2 interface {
	RetainProcessArtifactV2(id string, data []byte) (postmergeproof.ArtifactRefV2, error)
}

// ObserverCommandV2 is the pinned observer executable and the argv prefix that
// selects its internal process-observer role, for example the host launcher
// with `--internal-process-observer`.
type ObserverCommandV2 struct {
	Path string
	Args []string
}

// OwnedLaunchSpecV2 names one owned launch: its purpose, native context, the
// capture index of its already captured parent and its start phase.
type OwnedLaunchSpecV2 struct {
	Purpose      string
	ContextIndex int
	Parent       int
	Phase        string
}

// OwnedProcessV2 is one collected owned launch. Cmd is nil for an awaited
// descendant, which its own parent reaps.
type OwnedProcessV2 struct {
	Cmd     *exec.Cmd
	Launch  int
	Capture int
	pid     int
	stdin   io.WriteCloser
	stdout  *bytes.Buffer
}

// ProcessCollectorV2 assembles exact postmerge-process-proof/2 bytes from real
// procfs captures of owned launches (PMR-V2-006). It records observations
// only: it derives no role, absence or acceptance fact and cannot construct
// postmergeproof.VerifiedProcessesV2. Its methods are not safe for concurrent use.
type ProcessCollectorV2 struct {
	executionID string
	retain      ProcessRetainerV2
	observer    ObserverCommandV2
	proof       postmergeproof.ProcessProofV2
	pids        []int
	pending     []*pendingInvocationV2
	refs        map[string]postmergeproof.ArtifactRefV2
	supervisor  int
	trusted     *OwnedProcessV2
	sweeper     *OwnedProcessV2
	sealed      bool
	swept       bool
	// reaped holds the births the collector itself reaped; a later launch
	// may not name one as its parent.
	reaped map[procBirthV2]bool
	// failed is the first failure after a process started or the proof
	// changed; a failed collection emits no proof.
	failed error
}

// procBirthV2 is a PID with its starttime, which a reused PID cannot repeat
// while the original process exists.
type procBirthV2 struct {
	pid   int
	start uint64
}

type pendingInvocationV2 struct {
	invocation postmergeproof.InvocationV2
	retained   bool
}

const (
	processSettleV2      = 10 * time.Second
	processPollV2        = 5 * time.Millisecond
	processStdinLimitV2  = 1 << 20
	processStdoutLimitV2 = 32 << 20
	// processExecutableLimitV2 matches the verifier's executable byte bound.
	processExecutableLimitV2 = 1 << 30
	// processSweepLimitV2 is the fixed observer recipe's PID inventory bound.
	processSweepLimitV2 = 1 << 15
	// processWaitDelayV2 bounds how long Wait keeps copying output that an
	// escaped process still holds open after the leader exited.
	processWaitDelayV2 = 3 * time.Second
)

func processRefusedV2(detail string) error {
	return &postmergeproof.ProcessErrorV2{Outcome: postmergeproof.OutcomeBlockedV2, Code: "process-collection-refused", Detail: detail}
}

func processFailedV2(detail string, err error) error {
	if err != nil {
		detail += ": " + err.Error()
	}
	return &postmergeproof.ProcessErrorV2{Outcome: postmergeproof.OutcomeBlockedV2, Code: "process-collection-failed", Detail: detail}
}

// NewProcessCollectorV2 starts a collection for one native execution. It
// captures this process as the outside-workload host supervisor (capture 0,
// context 0, no parent). On a host without the Linux procfs profile it returns
// procfs.UnsupportedError: collection there is NOT_OBSERVED.
func NewProcessCollectorV2(ctx context.Context, executionID, graphSHA256, policySHA256 string,
	observer ObserverCommandV2, retain ProcessRetainerV2) (*ProcessCollectorV2, error) {
	if !procfs.Supported {
		return nil, procfs.UnsupportedError{}
	}
	if executionID == "" || strings.IndexByte(executionID, 0) >= 0 || retain == nil || observer.Path == "" {
		return nil, processRefusedV2("execution, retention and observer are required")
	}
	c := &ProcessCollectorV2{executionID: executionID, retain: retain, refs: map[string]postmergeproof.ArtifactRefV2{},
		reaped:   map[procBirthV2]bool{},
		observer: ObserverCommandV2{Path: observer.Path, Args: slices.Clone(observer.Args)},
		proof: postmergeproof.ProcessProofV2{Profile: "postmerge-process-proof/2", ExecutionID: executionID,
			GraphBindingSHA256: graphSHA256, PolicySHA256: policySHA256, Captures: []postmergeproof.ProcCaptureV2{},
			Launches: []postmergeproof.OwnedLaunchV2{}, NativeReferences: []postmergeproof.NativeProcessReferenceV2{},
			Cleanup: []postmergeproof.CleanupJoinV2{}}}
	index, err := c.capture(ctx, "start-barrier", os.Getpid())
	if err != nil {
		return nil, err
	}
	captured := c.proof.Captures[index]
	argv := strings.Split(strings.TrimSuffix(string(captured.Cmdline), "\x00"), "\x00")
	env := []string{}
	for _, key := range postmergeproof.InvocationEnvironmentKeysV2() {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	if _, err := c.addLaunch("host-supervisor", 0, nil, index, captured.Executable, argv, env); err != nil {
		return nil, err
	}
	c.supervisor = index
	return c, c.finishInvocation(0, nil)
}

// SupervisorCapture is the host supervisor's start capture index.
func (c *ProcessCollectorV2) SupervisorCapture() int { return c.supervisor }

// Start launches cmd as an owned process, waits for its exec to settle,
// captures its birth and then delivers stdin and closes it. cmd must be
// unstarted, have no preset Stdin and an explicit Env inside the verifier's
// invocation allowlist; a nil Env would inherit unrecorded variables. A
// zero WaitDelay is set to a bound, so an escaped process holding its output
// cannot stall Wait.
func (c *ProcessCollectorV2) Start(ctx context.Context, cmd *exec.Cmd, spec OwnedLaunchSpecV2, stdin []byte) (*OwnedProcessV2, error) {
	if err := c.checkSpec(spec); err != nil {
		return nil, err
	}
	p, err := c.start(ctx, cmd, spec, nil)
	if err != nil {
		return nil, err
	}
	if err := c.deliver(ctx, p, stdin); err != nil {
		return nil, c.abandonFailed(p, err)
	}
	return p, nil
}

// AwaitChild waits for a child of the parent capture whose argv is exactly
// argv and records it as an owned launch of an approved invocation: path is
// its approved executable and env and stdin its approved preimages. The child
// is reaped by its own parent, so it never has a collector retirement. The
// parent must still be its captured birth before discovery and after the
// child's capture, and the child's actual environment must equal env.
func (c *ProcessCollectorV2) AwaitChild(ctx context.Context, path string, argv, env []string, stdin []byte, spec OwnedLaunchSpecV2) (*OwnedProcessV2, error) {
	if err := c.checkSpec(spec); err != nil {
		return nil, err
	}
	sorted, err := invocationEnvironmentV2(env)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, processRefusedV2("awaited child has no argv")
	}
	parent, err := c.liveParentV2(spec.Parent)
	if err != nil {
		return nil, err
	}
	executable, err := c.retainExecutable(path)
	if err != nil {
		return nil, err
	}
	want := argvBytesV2(argv)
	var pid int
	if err := waitProcessV2(ctx, 0, "child of "+strconv.Itoa(parent.pid), func() bool {
		pid = childProcessV2(parent.pid, want)
		return pid != 0
	}); err != nil {
		return nil, c.fail(err)
	}
	index, err := c.capture(ctx, spec.Phase, pid)
	if err != nil {
		return nil, c.fail(err)
	}
	// The parent is still its captured birth after the child's capture, so
	// its PID named that one process throughout discovery: a PID is never
	// reused while its holder exists. Both child stats name that parent.
	if err := sameBirthV2(parent); err != nil {
		return nil, c.fail(err)
	}
	child := c.proof.Captures[index]
	for _, stat := range [][]byte{child.StatBefore, child.StatAfter} {
		if _, ppid, _, ok := statFieldsV2(stat); !ok || ppid != parent.pid {
			return nil, c.fail(processFailedV2("awaited child "+strconv.Itoa(pid)+" is not a child of its captured parent", nil))
		}
	}
	if err := environmentMatchesV2(pid, sorted); err != nil {
		return nil, c.fail(err)
	}
	launch, err := c.addLaunch(spec.Purpose, spec.ContextIndex, &spec.Parent, index, executable, argv, sorted)
	if err != nil {
		return nil, c.fail(err)
	}
	if err := c.finishInvocation(launch, stdin); err != nil {
		return nil, c.fail(err)
	}
	return &OwnedProcessV2{Launch: launch, Capture: index, pid: pid}, nil
}

// liveParentV2 returns the birth of a parent capture that is neither a
// retirement nor a birth the collector reaped and that its PID still names.
func (c *ProcessCollectorV2) liveParentV2(index int) (procBirthV2, error) {
	_, _, start, ok := statFieldsV2(c.proof.Captures[index].StatBefore)
	birth := procBirthV2{pid: c.pids[index], start: start}
	if !ok || c.proof.Captures[index].Phase == "retirement" || c.reaped[birth] {
		return procBirthV2{}, processRefusedV2("parent capture " + strconv.Itoa(index) + " is not a live owned process")
	}
	return birth, sameBirthV2(birth)
}

// sameBirthV2 refuses a PID that no longer names its captured live birth.
func sameBirthV2(birth procBirthV2) error {
	if state, _, start, ok := processStatV2(birth.pid); !ok || start != birth.start || state == 'Z' || state == 'X' {
		return processFailedV2("process "+strconv.Itoa(birth.pid)+" is no longer its captured birth", nil)
	}
	return nil
}

// Capture records one more birth capture of an owned process, for example a
// during-run witness, and returns its index.
func (c *ProcessCollectorV2) Capture(ctx context.Context, p *OwnedProcessV2, phase string) (int, error) {
	if phase != "during-run" {
		return 0, processRefusedV2("an extra capture must be during-run")
	}
	if c.sealed {
		return 0, processRefusedV2("no workload capture follows the final sweep")
	}
	index, err := c.capture(ctx, phase, p.pid)
	if err != nil {
		return 0, c.fail(err)
	}
	return index, nil
}

// SetOutcome records native cancellation or timeout of an owned launch.
func (c *ProcessCollectorV2) SetOutcome(p *OwnedProcessV2, cancelled, timedOut bool) {
	c.proof.Launches[p.Launch].Cancelled, c.proof.Launches[p.Launch].TimedOut = cancelled, timedOut
}

// Retire waits until the owned process is an unreaped zombie, captures that
// retirement, reaps it and records its exact wait status. The launch is then
// completed with its reaped retirement. A Wait error other than the exit
// status, such as output still held open after the Wait delay, fails the
// collection.
func (c *ProcessCollectorV2) Retire(ctx context.Context, p *OwnedProcessV2) error {
	if p.Cmd == nil || p.Cmd.Process == nil || p.Cmd.ProcessState != nil {
		return processRefusedV2("only an unreaped collector-started process can be retired")
	}
	if c.sealed && p != c.sweeper {
		return processRefusedV2("no workload retirement follows the final sweep")
	}
	if p.stdin != nil {
		return processRefusedV2("stdin has not been delivered")
	}
	if err := waitProcessV2(ctx, -1, "zombie "+strconv.Itoa(p.pid), func() bool { return processZombieV2(p.pid) }); err != nil {
		return c.abandonFailed(p, err)
	}
	index, err := c.capture(ctx, "retirement", p.pid)
	if err != nil {
		return c.abandonFailed(p, err)
	}
	if _, _, start, ok := statFieldsV2(c.proof.Captures[p.Capture].StatBefore); ok {
		c.reaped[procBirthV2{pid: p.pid, start: start}] = true
	}
	waitErr := p.Cmd.Wait()
	capture := &c.proof.Captures[index]
	if status, ok := p.Cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		signal := int(status.Signal())
		capture.ExitSignal = &signal
	} else {
		code := p.Cmd.ProcessState.ExitCode()
		capture.ExitCode = &code
	}
	launch := &c.proof.Launches[p.Launch]
	launch.RetirementCaptureIndex, launch.Completed = &index, true
	var exit *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exit) {
		return c.fail(processFailedV2("wait for "+strconv.Itoa(p.pid), waitErr))
	}
	return nil
}

// AddCleanupJoin passes one native cleanup join through unchanged; the
// verifier compares it with the native report.
func (c *ProcessCollectorV2) AddCleanupJoin(join postmergeproof.CleanupJoinV2) {
	c.proof.Cleanup = append(c.proof.Cleanup, join)
}

// StartTrustedObserver launches the trusted-start observer at the launch
// barrier, before the workflow root. Its stdin stays open until
// CompleteTrustedStart delivers the TrustedStartV2 naming the root capture.
func (c *ProcessCollectorV2) StartTrustedObserver(ctx context.Context) (*OwnedProcessV2, error) {
	if c.trusted != nil || c.sealed {
		return nil, processRefusedV2("trusted start observer already launched")
	}
	cmd := exec.Command(c.observer.Path, append(slices.Clone(c.observer.Args), "trusted-start")...)
	cmd.Env = []string{}
	p, err := c.start(ctx, cmd, OwnedLaunchSpecV2{Purpose: "observer", Parent: c.supervisor, Phase: "start-barrier"}, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}
	c.trusted = p
	return p, nil
}

// CompleteTrustedStart delivers start to the trusted-start observer, retires
// it and retains its exact stdout as the proof's trusted start. A non-zero
// observer exit refuses the start.
func (c *ProcessCollectorV2) CompleteTrustedStart(ctx context.Context, p *OwnedProcessV2, start postmergeproof.TrustedStartV2) error {
	if p == nil || p != c.trusted || c.proof.TrustedStartInvocation != (postmergeproof.ArtifactRefV2{}) {
		return processRefusedV2("not the pending trusted start observer")
	}
	data, err := json.Marshal(start)
	if err != nil {
		return processFailedV2("trusted start encoding", err)
	}
	if err := c.deliver(ctx, p, data); err != nil {
		return c.abandonFailed(p, err)
	}
	stdout, err := c.retireObserver(ctx, p)
	if err != nil {
		return c.fail(err)
	}
	if c.proof.TrustedStartStdout, err = c.retainDocument("trusted-start", stdout); err != nil {
		return c.fail(err)
	}
	c.proof.TrustedStartInvocation = c.proof.Launches[p.Launch].Invocation
	return nil
}

// FinalSweep launches the independent absence-sweep observer after workload
// retirement, captures its birth, releases it, reaps it and copies its exact
// AbsenceSweepDocumentV2 into the proof. No capture follows the sweep.
func (c *ProcessCollectorV2) FinalSweep(ctx context.Context) error {
	if c.sealed || c.proof.TrustedStartInvocation == (postmergeproof.ArtifactRefV2{}) {
		return processRefusedV2("final sweep needs a completed trusted start and runs once")
	}
	cmd := exec.Command(c.observer.Path, append(slices.Clone(c.observer.Args), "absence-sweep", c.executionID)...)
	cmd.Env = []string{}
	p, err := c.start(ctx, cmd, OwnedLaunchSpecV2{Purpose: "observer", Parent: c.supervisor, Phase: "final-sweep"}, new(bytes.Buffer))
	if err != nil {
		return err
	}
	c.sealed, c.sweeper = true, p
	if err := c.deliver(ctx, p, nil); err != nil {
		return c.abandonFailed(p, err)
	}
	stdout, err := c.retireObserver(ctx, p)
	if err != nil {
		return c.fail(err)
	}
	var document postmergeproof.AbsenceSweepDocumentV2
	if err := json.Unmarshal(stdout, &document); err != nil {
		return c.fail(processFailedV2("absence sweep output", err))
	}
	ref, err := c.retainDocument("absence-sweep", stdout)
	if err != nil {
		return c.fail(err)
	}
	c.proof.FinalSweep = postmergeproof.AbsenceSweepV2{ObserverInvocation: c.proof.Launches[p.Launch].Invocation, ObserverStdout: ref,
		BootIDBytes: document.BootIDBytes, NamespaceLinkBytes: document.NamespaceLinkBytes, PIDDirectoryBytes: document.PIDDirectoryBytes,
		Processes: document.Processes, ReadFailures: document.ReadFailures}
	c.swept = true
	return nil
}

// Proof returns the exact postmerge-process-proof/2 bytes. It refuses after
// any failed collection step, before the trusted start and a successful final
// sweep exist and while any invocation is pending.
func (c *ProcessCollectorV2) Proof() ([]byte, error) {
	if c.failed != nil {
		return nil, processFailedV2("an earlier collection step failed", c.failed)
	}
	if !c.swept || c.proof.TrustedStartInvocation == (postmergeproof.ArtifactRefV2{}) {
		return nil, processRefusedV2("proof needs the trusted start and final sweep")
	}
	for i, pending := range c.pending {
		if !pending.retained {
			return nil, processRefusedV2("launch " + strconv.Itoa(i) + " invocation is pending")
		}
	}
	return json.Marshal(c.proof)
}

func (c *ProcessCollectorV2) checkSpec(spec OwnedLaunchSpecV2) error {
	if c.sealed {
		return processRefusedV2("no launch follows the final sweep")
	}
	if spec.Purpose == "" || spec.ContextIndex < 0 || spec.Parent < 0 || spec.Parent >= len(c.pids) ||
		(spec.Phase != "start-barrier" && spec.Phase != "during-run") {
		return processRefusedV2("launch purpose, context, parent capture or phase is invalid")
	}
	return nil
}

// start launches cmd, waits for its exec to settle and captures it. With a
// non-nil stdout buffer the process's stdout is retained there.
func (c *ProcessCollectorV2) start(ctx context.Context, cmd *exec.Cmd, spec OwnedLaunchSpecV2, stdout *bytes.Buffer) (*OwnedProcessV2, error) {
	if cmd == nil || cmd.Process != nil || cmd.Err != nil || cmd.Stdin != nil || cmd.Env == nil || len(cmd.Args) == 0 ||
		(stdout != nil && cmd.Stdout != nil) {
		return nil, processRefusedV2("command must be unstarted with explicit Env and no preset Stdin")
	}
	if c.sealed {
		return nil, processRefusedV2("no launch follows the final sweep")
	}
	env, err := invocationEnvironmentV2(cmd.Env)
	if err != nil {
		return nil, err
	}
	executable, err := c.retainExecutable(cmd.Path)
	if err != nil {
		return nil, err
	}
	if stdout != nil {
		cmd.Stdout = &limitedBufferV2{buffer: stdout, limit: processStdoutLimitV2}
	}
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = processWaitDelayV2
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, processFailedV2("stdin pipe", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, processFailedV2("start "+cmd.Path, err)
	}
	p := &OwnedProcessV2{Cmd: cmd, pid: cmd.Process.Pid, stdin: stdin, stdout: stdout}
	// Start returns once exec closed its CLOEXEC pipe, before the kernel
	// publishes the new argv; a capture in that window would read the old
	// image. Wait until the exact argv is visible.
	want := argvBytesV2(cmd.Args)
	if err := waitProcessV2(ctx, processSettleV2, "exec of "+cmd.Path, func() bool {
		cmdline, _ := os.ReadFile("/proc/" + strconv.Itoa(p.pid) + "/cmdline")
		return bytes.Equal(cmdline, want)
	}); err != nil {
		return nil, c.abandonFailed(p, err)
	}
	if err := environmentMatchesV2(p.pid, env); err != nil {
		return nil, c.abandonFailed(p, err)
	}
	index, err := c.capture(ctx, spec.Phase, p.pid)
	if err != nil {
		return nil, c.abandonFailed(p, err)
	}
	p.Capture = index
	if p.Launch, err = c.addLaunch(spec.Purpose, spec.ContextIndex, &spec.Parent, index, executable, slices.Clone(cmd.Args), env); err != nil {
		return nil, c.abandonFailed(p, err)
	}
	return p, nil
}

// fail records the first failure after a process started or the proof
// changed and returns err.
func (c *ProcessCollectorV2) fail(err error) error {
	if c.failed == nil {
		c.failed = err
	}
	return err
}

// abandonFailed retires the owned tree of a collector-started process after
// a failed step, reaps its leader within the Wait delay and records the
// failure. An unresolved cleanup is part of the returned failure.
func (c *ProcessCollectorV2) abandonFailed(p *OwnedProcessV2, err error) error {
	if p.stdin != nil {
		_ = p.stdin.Close()
		p.stdin = nil
	}
	cleanup := retireTreeV2(p.pid)
	if p.Cmd.ProcessState == nil {
		_ = p.Cmd.Process.Kill()
		_ = p.Cmd.Wait()
	}
	if cleanup != nil {
		err = processFailedV2(err.Error()+"; owned cleanup", cleanup)
	}
	return c.fail(err)
}

// retireTreeListedV2 lets a test act on a frozen member's listed children
// before the walk freezes them; it is nil outside tests.
var retireTreeListedV2 func(member int, children []procBirthV2) []procBirthV2

// retireTreeV2 is the owned cleanup boundary of a failed launch. It freezes
// the leader and then every descendant reachable through PPID links, and
// signals each one only through a pidfd whose process was confirmed to have
// the listed birth, so a recycled PID is never signalled or walked. A member
// counts as frozen only when every one of its threads is stopped: a frozen
// member cannot fork, and its exited children stay unreaped, so its listed
// children are stable. Only frozen members are walked, and the walk repeats
// until a pass finds every member frozen and no new child. It then kills
// every confirmed handle, frozen or not, and waits, within its own bound,
// until none is still running. A member that exited or changed before its
// freeze was confirmed may have had children that were reparented away; that,
// a member never confirmed frozen, a recycled PID or a member still running
// is reported as unresolved. A descendant already reparented away before the
// walk, such as a double fork, is outside the boundary, as it is for the
// final sweep.
func retireTreeV2(leader int) error {
	if !procfs.Supported {
		return nil
	}
	deadline := time.Now().Add(processSettleV2)
	var unresolved []string
	handles := map[procBirthV2]int{}
	members := map[procBirthV2]bool{}
	defer func() {
		for _, fd := range handles {
			_ = syscall.Close(fd)
		}
	}()
	_, _, start, ok := processStatV2(leader)
	if !ok {
		return errors.New("unresolved: leader " + strconv.Itoa(leader) + " is not observable")
	}
	seen := map[procBirthV2]bool{{pid: leader, start: start}: true}
	queue := []procBirthV2{{pid: leader, start: start}}
	for settled := false; !settled; {
		if time.Now().After(deadline) {
			unresolved = append(unresolved, "the walk did not settle")
			break
		}
		for _, birth := range queue {
			fd, err := freezeMemberV2(birth, -1, deadline)
			if fd >= 0 {
				handles[birth] = fd
			}
			if err != nil {
				unresolved = append(unresolved, err.Error())
				continue
			}
			members[birth] = true
		}
		queue, settled = nil, true
		for birth := range members {
			fd := handles[birth]
			if !frozenV2(fd, birth) {
				settled = false
				if _, err := freezeMemberV2(birth, fd, deadline); err != nil {
					unresolved = append(unresolved, err.Error())
					delete(members, birth)
					continue
				}
			}
			children := childBirthsV2(birth.pid, true)
			if !frozenV2(fd, birth) {
				settled = false
				continue
			}
			if retireTreeListedV2 != nil {
				children = retireTreeListedV2(birth.pid, children)
			}
			for _, child := range children {
				if !seen[child] {
					seen[child], settled = true, false
					queue = append(queue, child)
				}
			}
		}
	}
	for _, fd := range handles {
		_ = signalProcessV2(fd, syscall.SIGKILL)
	}
	running := func() []string {
		var out []string
		for birth, fd := range handles {
			if state, ok := observeMemberV2(fd, birth); ok && state != 'Z' && state != 'X' {
				out = append(out, strconv.Itoa(birth.pid))
			}
		}
		slices.Sort(out)
		return out
	}
	if err := waitProcessV2(context.Background(), processSettleV2, "member exit", func() bool {
		return len(running()) == 0
	}); err != nil {
		unresolved = append(unresolved, "still running: "+strings.Join(running(), ","))
	}
	if len(unresolved) != 0 {
		slices.Sort(unresolved)
		return errors.New("unresolved: " + strings.Join(unresolved, "; "))
	}
	return nil
}

// freezeMemberV2 opens a pidfd for birth when fd is negative, confirms the
// birth through it, stops the process and waits until all its threads are
// stopped. It returns an error naming why the member could not be confirmed
// frozen. It returns the pidfd whenever the birth was confirmed through it,
// even with an error, so the caller can still kill that process; a pidfd it
// opened for a birth it could not confirm is closed.
func freezeMemberV2(birth procBirthV2, fd int, deadline time.Time) (int, error) {
	name := "pid " + strconv.Itoa(birth.pid)
	if fd < 0 {
		var err error
		if fd, err = openProcessV2(birth.pid); errors.Is(err, syscall.ESRCH) {
			return -1, errors.New(name + " exited before its freeze was confirmed")
		} else if err != nil {
			return -1, errors.New(name + " could not be opened: " + err.Error())
		}
		if _, ok := observeMemberV2(fd, birth); !ok {
			_ = syscall.Close(fd)
			return -1, errors.New(name + " is no longer its listed birth")
		}
	}
	fail := func(detail string) (int, error) {
		return fd, errors.New(name + " " + detail)
	}
	state, ok := observeMemberV2(fd, birth)
	if !ok || state == 'Z' || state == 'X' {
		return fail("exited before its freeze was confirmed")
	}
	if err := signalProcessV2(fd, syscall.SIGSTOP); err != nil {
		return fail("could not be stopped: " + err.Error())
	}
	var exited bool
	if err := waitProcessV2(context.Background(), max(time.Until(deadline), processPollV2), "freeze of "+name, func() bool {
		if state, ok := observeMemberV2(fd, birth); !ok || state == 'Z' || state == 'X' {
			exited = true
		}
		return exited || frozenV2(fd, birth)
	}); err != nil {
		return fail("was not confirmed frozen")
	}
	if exited {
		return fail("exited before its freeze was confirmed")
	}
	return fd, nil
}

// observeMemberV2 reads the state of birth and confirms through its pidfd
// that the PID still named that process during the read; ok is false once
// the process was reaped or the PID names another birth.
func observeMemberV2(fd int, birth procBirthV2) (byte, bool) {
	state, _, start, ok := processStatV2(birth.pid)
	if !ok || start != birth.start || signalProcessV2(fd, 0) != nil {
		return 0, false
	}
	return state, true
}

// frozenV2 reports whether every thread of birth is stopped; the leader's
// state alone would let another thread fork before the group stop ends.
func frozenV2(fd int, birth procBirthV2) bool {
	tasks, err := os.ReadDir("/proc/" + strconv.Itoa(birth.pid) + "/task")
	if err != nil || len(tasks) == 0 {
		return false
	}
	for _, task := range tasks {
		data, err := os.ReadFile("/proc/" + strconv.Itoa(birth.pid) + "/task/" + task.Name() + "/stat")
		if state, _, _, ok := statFieldsV2(data); err != nil || !ok || state != 'T' {
			return false
		}
	}
	state, ok := observeMemberV2(fd, birth)
	return ok && state == 'T'
}

// deliver retains and writes stdin, closes it and finishes the invocation.
// A cancelled ctx closes the pipe, so a child that never reads stdin cannot
// stall delivery; the caller then retires the child.
func (c *ProcessCollectorV2) deliver(ctx context.Context, p *OwnedProcessV2, stdin []byte) error {
	if p.stdin == nil {
		return processRefusedV2("stdin already delivered")
	}
	if err := c.finishInvocation(p.Launch, stdin); err != nil {
		return err
	}
	pipe := p.stdin
	p.stdin = nil
	written := make(chan error, 1)
	go func() {
		_, err := pipe.Write(stdin)
		written <- err
	}()
	var err error
	select {
	case err = <-written:
	case <-ctx.Done():
		_ = pipe.Close()
		return processFailedV2("deliver stdin", ctx.Err())
	}
	if closeErr := pipe.Close(); err == nil {
		err = closeErr
	}
	if err != nil && !errors.Is(err, syscall.EPIPE) {
		return processFailedV2("deliver stdin", err)
	}
	return nil
}

func (c *ProcessCollectorV2) retireObserver(ctx context.Context, p *OwnedProcessV2) ([]byte, error) {
	if err := c.Retire(ctx, p); err != nil {
		return nil, err
	}
	writer := p.Cmd.Stdout.(*limitedBufferV2)
	if writer.exceeded {
		return nil, processFailedV2("observer stdout exceeds its bound", nil)
	}
	retired := c.proof.Captures[*c.proof.Launches[p.Launch].RetirementCaptureIndex]
	if retired.ExitCode == nil || *retired.ExitCode != 0 {
		return nil, processFailedV2("observer did not exit successfully", nil)
	}
	return writer.buffer.Bytes(), nil
}

func (c *ProcessCollectorV2) addLaunch(purpose string, contextIndex int, parent *int, captureIndex int,
	executable postmergeproof.ArtifactRefV2, argv, env []string) (int, error) {
	environment := make([]postmergeproof.EnvironmentV2, 0, len(env))
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		environment = append(environment, postmergeproof.EnvironmentV2{Key: key, Value: value})
	}
	for _, arg := range argv {
		if strings.IndexByte(arg, 0) >= 0 {
			return 0, processRefusedV2("argv contains NUL")
		}
	}
	index := len(c.proof.Launches)
	c.pending = append(c.pending, &pendingInvocationV2{invocation: postmergeproof.InvocationV2{Profile: "postmerge-owned-invocation/2",
		ExecutionID: c.executionID, Purpose: purpose, Executable: executable, Argv: argv, Environment: environment,
		ContextIndex: contextIndex, LaunchIndex: index}})
	c.proof.Launches = append(c.proof.Launches, postmergeproof.OwnedLaunchV2{Index: index, ContextIndex: contextIndex, Purpose: purpose,
		ParentCaptureIndex: parent, StartCaptureIndex: captureIndex, ExecutedCaptureIndex: captureIndex})
	return index, nil
}

// finishInvocation binds the delivered stdin and retains the invocation.
func (c *ProcessCollectorV2) finishInvocation(launch int, stdin []byte) error {
	pending := c.pending[launch]
	if pending.retained {
		return processRefusedV2("invocation already retained")
	}
	if len(stdin) > processStdinLimitV2 {
		return processRefusedV2("stdin exceeds its bound")
	}
	ref := emptyArtifactV2()
	if len(stdin) != 0 {
		var err error
		if ref, err = c.retainArtifact(c.executionID+"/launch/"+strconv.Itoa(launch)+"/stdin", stdin); err != nil {
			return err
		}
	}
	pending.invocation.StdinSHA256 = ref.SHA256
	data, err := json.Marshal(pending.invocation)
	if err != nil {
		return processFailedV2("invocation encoding", err)
	}
	invocation, err := c.retainArtifact(c.executionID+"/launch/"+strconv.Itoa(launch)+"/invocation", data)
	if err != nil {
		return err
	}
	c.proof.Launches[launch].Stdin, c.proof.Launches[launch].Invocation, pending.retained = ref, invocation, true
	return nil
}

// capture records one bracketed procfs birth capture and returns its index.
func (c *ProcessCollectorV2) capture(ctx context.Context, phase string, pid int) (int, error) {
	if c.sealed && phase != "final-sweep" && phase != "retirement" {
		return 0, processRefusedV2("no workload capture follows the final sweep")
	}
	raw, err := procfs.CaptureBirth(ctx, pid, processExecutableLimitV2)
	if err != nil {
		var unsupported procfs.UnsupportedError
		if errors.As(err, &unsupported) {
			return 0, err
		}
		return 0, processFailedV2("capture "+strconv.Itoa(pid), err)
	}
	executable, after := emptyArtifactV2(), emptyArtifactV2()
	if len(raw.Executable) != 0 {
		if executable, err = c.retainBytes(raw.Executable); err != nil {
			return 0, err
		}
	}
	if raw.ExecutableAfterSHA256 != "" {
		after = postmergeproof.ArtifactRefV2{ID: "exe/" + raw.ExecutableAfterSHA256, SHA256: raw.ExecutableAfterSHA256, Bytes: raw.ExecutableAfterBytes}
	}
	tool := emptyArtifactV2()
	if len(raw.NativeStartTool) != 0 {
		if tool, err = c.retainBytes(raw.NativeStartTool); err != nil {
			return 0, err
		}
	}
	index := len(c.proof.Captures)
	c.proof.Captures = append(c.proof.Captures, postmergeproof.ProcCaptureV2{Index: index, Phase: phase, BootIDBytes: raw.BootID,
		NamespaceLinkBytes: raw.NamespaceLink, NamespaceDevice: raw.NamespaceDevice, NamespaceInode: raw.NamespaceInode,
		StatBefore: raw.StatBefore, Cmdline: nonNilV2(raw.Cmdline), CmdlineAfter: nonNilV2(raw.CmdlineAfter),
		ExecutableLinkBytes: nonNilV2(raw.ExecutableLink), ExecutableLinkAfterBytes: nonNilV2(raw.ExecutableLinkAfter),
		Executable: executable, ExecutableAfter: after, StatAfter: raw.StatAfter,
		NativeStartOutput: nonNilV2(raw.NativeStartOutput), NativeStartTool: tool})
	c.pids = append(c.pids, pid)
	return index, nil
}

// retainExecutable retains the full bytes of an approved executable path.
func (c *ProcessCollectorV2) retainExecutable(path string) (postmergeproof.ArtifactRefV2, error) {
	file, err := os.Open(path)
	if err != nil {
		return postmergeproof.ArtifactRefV2{}, processFailedV2("approved executable", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, processExecutableLimitV2+1))
	if err != nil || len(data) == 0 || len(data) > processExecutableLimitV2 {
		return postmergeproof.ArtifactRefV2{}, processFailedV2("approved executable bytes", err)
	}
	return c.retainBytes(data)
}

// retainBytes retains executable content under its content-addressed ID.
func (c *ProcessCollectorV2) retainBytes(data []byte) (postmergeproof.ArtifactRefV2, error) {
	sum := sha256.Sum256(data)
	id := "exe/" + hex.EncodeToString(sum[:])
	if ref, ok := c.refs[id]; ok {
		return ref, nil
	}
	ref, err := c.retainArtifact(id, data)
	if err == nil {
		c.refs[id] = ref
	}
	return ref, err
}

func (c *ProcessCollectorV2) retainDocument(name string, data []byte) (postmergeproof.ArtifactRefV2, error) {
	return c.retainArtifact(c.executionID+"/"+name, data)
}

// retainArtifact retains bytes and refuses a reference that is not theirs.
func (c *ProcessCollectorV2) retainArtifact(id string, data []byte) (postmergeproof.ArtifactRefV2, error) {
	ref, err := c.retain.RetainProcessArtifactV2(id, data)
	if err != nil {
		return postmergeproof.ArtifactRefV2{}, processFailedV2("retain "+id, err)
	}
	sum := sha256.Sum256(data)
	if ref.ID != id || ref.SHA256 != hex.EncodeToString(sum[:]) || ref.Bytes != int64(len(data)) {
		return postmergeproof.ArtifactRefV2{}, processFailedV2("retention returned another reference for "+id, nil)
	}
	return ref, nil
}

func emptyArtifactV2() postmergeproof.ArtifactRefV2 {
	sum := sha256.Sum256(nil)
	return postmergeproof.ArtifactRefV2{SHA256: hex.EncodeToString(sum[:])}
}

func nonNilV2(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

func argvBytesV2(argv []string) []byte {
	var out []byte
	for _, arg := range argv {
		out = append(append(out, arg...), 0)
	}
	return out
}

// invocationEnvironmentV2 refuses an environment outside the fixed allowlist
// and returns it sorted by key, as the invocation preimage records it.
func invocationEnvironmentV2(env []string) ([]string, error) {
	allowed := postmergeproof.InvocationEnvironmentKeysV2()
	sorted := slices.Clone(env)
	slices.Sort(sorted)
	for i, entry := range sorted {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !slices.Contains(allowed, key) || strings.IndexByte(value, 0) >= 0 ||
			(i > 0 && strings.SplitN(sorted[i-1], "=", 2)[0] == key) {
			return nil, processRefusedV2("environment is outside the fixed invocation allowlist")
		}
	}
	return sorted, nil
}

// waitProcessV2 polls done until it holds, ctx ends or the bound passes; a
// negative bound waits on ctx alone.
func waitProcessV2(ctx context.Context, bound time.Duration, what string, done func() bool) error {
	if bound == 0 {
		bound = processSettleV2
	}
	deadline := time.Now().Add(bound)
	for !done() {
		if err := ctx.Err(); err != nil {
			return processFailedV2("waiting for "+what, err)
		}
		if bound > 0 && time.Now().After(deadline) {
			return processFailedV2("timed out waiting for "+what, nil)
		}
		time.Sleep(processPollV2)
	}
	return nil
}

// statFieldsV2 parses state (field 3), PPID (4) and starttime (22) from raw
// /proc stat bytes.
func statFieldsV2(data []byte) (state byte, ppid int, start uint64, ok bool) {
	end := bytes.LastIndexByte(data, ')')
	if end < 0 {
		return 0, 0, 0, false
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 || len(fields[0]) != 1 {
		return 0, 0, 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, 0, false
	}
	if start, err = strconv.ParseUint(fields[19], 10, 64); err != nil {
		return 0, 0, 0, false
	}
	return fields[0][0], ppid, start, true
}

// processStatV2 returns the state, parent and starttime of a /proc entry.
func processStatV2(pid int) (state byte, ppid int, start uint64, ok bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, 0, 0, false
	}
	return statFieldsV2(data)
}

func processZombieV2(pid int) bool {
	state, _, _, ok := processStatV2(pid)
	return ok && state == 'Z'
}

// childBirthsV2 lists the children of parent, without zombies unless
// zombies is set.
func childBirthsV2(parent int, zombies bool) []procBirthV2 {
	var out []procBirthV2
	entries, _ := os.ReadDir("/proc")
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if state, ppid, start, ok := processStatV2(pid); ok && ppid == parent && (zombies || (state != 'Z' && state != 'X')) {
			out = append(out, procBirthV2{pid: pid, start: start})
		}
	}
	return out
}

// childProcessV2 finds the live child of parent whose argv is exactly argv.
func childProcessV2(parent int, argv []byte) int {
	for _, child := range childBirthsV2(parent, false) {
		if cmdline, _ := os.ReadFile("/proc/" + strconv.Itoa(child.pid) + "/cmdline"); bytes.Equal(cmdline, argv) {
			return child.pid
		}
	}
	return 0
}

// environmentMatchesV2 refuses a process whose actual initial environment is
// not exactly the sorted invocation environment.
func environmentMatchesV2(pid int, sorted []string) error {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return processFailedV2("environment of "+strconv.Itoa(pid), err)
	}
	actual := []string{}
	for _, entry := range strings.Split(string(data), "\x00") {
		if entry != "" {
			actual = append(actual, entry)
		}
	}
	slices.Sort(actual)
	if !slices.Equal(actual, sorted) {
		return processFailedV2("environment of "+strconv.Itoa(pid)+" differs from its invocation", nil)
	}
	return nil
}

// limitedBufferV2 keeps at most limit bytes of observer stdout.
type limitedBufferV2 struct {
	buffer   *bytes.Buffer
	limit    int
	exceeded bool
}

func (w *limitedBufferV2) Write(p []byte) (int, error) {
	if w.buffer.Len()+len(p) > w.limit {
		w.exceeded = true
		return 0, errors.New("observer stdout exceeds its bound")
	}
	return w.buffer.Write(p)
}
