//go:build darwin || linux

package groupreap

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These real-process owner proofs run with the platform default primitives on
// Darwin and Linux (V1-0668). Darwin's quiet proof is signal 0 (EPERM while
// only the unreaped leader remains); Linux signal 0 succeeds for a zombie-only
// group, so its default quiet proof is the /proc observation (PGO-V0-006).
// owner_linux_test.go adds the Linux-only fixtures.

func bounded(d time.Duration) <-chan struct{} {
	limit := make(chan struct{})
	time.AfterFunc(d, func() { close(limit) })
	return limit
}

// startWithMember starts a leader that leaves one sleeping group member and
// prints that member's process ID.
func startWithMember(t *testing.T, tail string, p Primitives) (*Owner, int) {
	t.Helper()
	return startScriptWithMember(t, "sleep 300 >/dev/null 2>&1 & echo $!; "+tail, p)
}

// startScriptWithMember starts script as an owned group leader; its first
// output line is the process ID of the group member the test retires.
func startScriptWithMember(t *testing.T, script string, p Primitives) (*Owner, int) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", script)
	command.Stdout = write
	owner, err := StartWith(command, p)
	write.Close()
	if err != nil {
		t.Fatal(err)
	}
	// A test that fails before Finish still retires its group through the
	// owner's own pre-reap signal.
	t.Cleanup(func() {
		if s := owner.State(); s == OwnedRunning || s == OwnedExitObserved {
			owner.Stop()
			owner.Finish(bounded(10 * time.Second))
		}
	})
	line, err := bufio.NewReader(read).ReadString('\n')
	read.Close()
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	return owner, member
}

// requireReleasedOrQuietHold accepts RELEASED, and also the one outcome the
// OQ-12 rule makes possible for a group that held a killed descendant: the
// single post-reap observation saw EPERM because that descendant was still an
// unreaped zombie of its new parent. The owner then HOLDs without a retry.
// The test, not the owner, measures how long the group ID then lingers.
func requireReleasedOrQuietHold(t *testing.T, owner *Owner, result Result) bool {
	t.Helper()
	if result.State == Released {
		return true
	}
	if result.State != Hold || !result.PostReapObserved || result.PostReap != ProbeQuiet {
		t.Fatalf("result = %+v events = %v", result, owner.Events())
	}
	start := time.Now()
	for syscall.Kill(-owner.leader, 0) != syscall.ESRCH {
		if time.Since(start) > 5*time.Second {
			t.Fatalf("group %d still present 5s after a quiet post-reap observation", owner.leader)
		}
		time.Sleep(100 * time.Microsecond)
	}
	t.Logf("OQ-12 step two observed EPERM after the reap (HOLD, no retry); group absent %v later", time.Since(start))
	return false
}

// pinMember reads the member's process-table identity while it is a live
// member of the leader's group, so later test-owned signals go through
// signalPinned (a pidfd on Linux, a start-time recheck on Darwin) and cannot
// reach a process that reused its PID.
func pinMember(t *testing.T, member, leader int) Process {
	t.Helper()
	table, err := readProcessTable()
	if err != nil {
		t.Fatal(err)
	}
	identity, ok := table[member]
	if !ok || identity.PGID != leader || identity.State == StateZombie {
		t.Fatalf("member %d is not a live member of group %d: %+v (present %v)", member, leader, identity, ok)
	}
	return identity
}

// killPinned is test-owned cleanup: one SIGKILL to the pinned member only.
func killPinned(identity Process) {
	signalPinned(identity, false, syscall.SIGKILL)
}

func requireGone(t *testing.T, processID int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(processID, 0) != syscall.ESRCH {
		if time.Now().After(deadline) {
			t.Fatalf("process %d is still present", processID)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestOwnerRetiresLingeringMemberAfterNormalExit(t *testing.T) {
	owner, member := startWithMember(t, "exit 0", Primitives{})
	result := owner.Finish(bounded(10 * time.Second))
	requireReleasedOrQuietHold(t, owner, result)
	if result.WaitErr != nil {
		t.Fatalf("result = %+v", result)
	}
	requireGone(t, member)
}

func TestOwnerStopRetiresRunningGroup(t *testing.T) {
	owner, member := startWithMember(t, "wait", Primitives{})
	owner.Stop()
	result := owner.Finish(bounded(10 * time.Second))
	requireReleasedOrQuietHold(t, owner, result)
	if result.WaitErr == nil {
		t.Fatalf("result = %+v", result)
	}
	requireGone(t, member)
}

// OQ-12 E1b: with a live same-UID member left running and the leader reaped,
// signal 0 to the group must succeed. EPERM alone is therefore not an
// absence proof, and step two can tell a survivor from an absent group.
func TestOQ12LiveMemberAfterReapIsObservedLive(t *testing.T) {
	for trial := 0; trial < 12; trial++ {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command("/bin/sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!; exit 0")
		command.Stdout = write
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		write.Close()
		line, _ := bufio.NewReader(read).ReadString('\n')
		read.Close()
		member, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			t.Fatal(err)
		}
		leader := command.Process.Pid
		identity := pinMember(t, member, leader)
		t.Cleanup(func() { killPinned(identity) })
		if err := leaderUnreaped(leader); err != nil {
			t.Fatal(err)
		}
		before := syscall.Kill(-leader, 0)
		if err := command.Wait(); err != nil {
			t.Fatal(err)
		}
		after := syscall.Kill(-leader, 0)
		// The member is alive and was started by this test: its pinned
		// identity, not the group ID, is signalled for cleanup.
		killPinned(identity)
		t.Logf("E1b trial %d: unreaped=%v reaped=%v", trial, before, after)
		if before != nil || after != nil {
			t.Fatalf("trial %d: live member not observed: unreaped=%v reaped=%v", trial, before, after)
		}
		requireGone(t, member)
	}
}

// OQ-12 E1c: the full two-step observation on a retired group. Step one ends
// on a non-success observation with the leader unreaped; step two, the single
// observation after the reap, reports the group absent.
func TestOQ12TwoStepAbsenceOnRetiredGroup(t *testing.T) {
	real := defaultPrimitives()
	quietHolds := 0
	defer func() { t.Logf("E1c summary: %d trials, %d post-reap EPERM holds", 24, quietHolds) }()
	for trial := 0; trial < 24; trial++ {
		var probes []Probe
		p := Primitives{ProbeGroup: func(leader int) (Probe, error) {
			probe, err := real.ProbeGroup(leader)
			probes = append(probes, probe)
			return probe, err
		}}
		if runtime.GOOS == "linux" {
			// Linux signal 0 cannot prove a zombie-only group quiet, so the
			// pre-reap step keeps the /proc proof (PGO-V0-006) and the
			// recorder sees only the post-reap signal-0 observations.
			p.QuietProof = real.QuietProof
		}
		tail := "wait"
		if trial%2 == 1 {
			tail = "exit 0"
		}
		owner, member := startWithMember(t, tail, p)
		if tail == "wait" {
			owner.Stop()
		}
		result := owner.Finish(bounded(10 * time.Second))
		events := owner.Events()
		t.Logf("E1c trial %d (%s): probes=%v reap=%v events=%v", trial, tail, probes, result.WaitErr, events)
		if !requireReleasedOrQuietHold(t, owner, result) {
			quietHolds++
			requireGone(t, member)
			continue
		}
		if len(probes) < 1 || probes[len(probes)-1] != ProbeAbsent {
			t.Fatalf("trial %d: probes = %v", trial, probes)
		}
		// On Darwin the recorder also saw the pre-reap quiet observation.
		if runtime.GOOS == "darwin" && (len(probes) < 2 || probes[len(probes)-2] == ProbeLive) {
			t.Fatalf("trial %d: probes = %v", trial, probes)
		}
		reap := slices.Index(events, "reap")
		if reap < 0 || events[reap-1] != "probe-quiet" || events[reap+1] != "probe-final" || len(events) != reap+3 {
			t.Fatalf("trial %d: events = %v", trial, events)
		}
		if slices.Index(events, "kill-group") > reap || strings.Count(strings.Join(events, ","), "kill-group") != 1 {
			t.Fatalf("trial %d: events = %v", trial, events)
		}
		requireGone(t, member)
	}
}

func TestOwnerHoldsOnInjectedSignalFailureWithoutReaping(t *testing.T) {
	p := Primitives{KillGroup: func(int) error { return syscall.EINVAL }}
	owner, member := startWithMember(t, "exit 0", p)
	identity := pinMember(t, member, owner.leader)
	// Test-owned cleanup of the processes the HOLD deliberately retained.
	t.Cleanup(func() {
		killPinned(identity)
		owner.command.Wait()
	})
	result := owner.Finish(bounded(10 * time.Second))
	if result.State != Hold || slices.Contains(owner.Events(), "reap") {
		t.Fatalf("result = %+v events = %v", result, owner.Events())
	}
}

func TestOwnerPostReapPollReleasesAfterTransientQuietGroup(t *testing.T) {
	f := newFakeHost(ProbeQuiet, ProbeQuiet, ProbeAbsent)
	o := adopt(nil, 4242, f.primitives())
	f.exit <- nil
	<-o.Exited()
	result := o.Finish(open())
	if result.State != Released || result.PostReap != ProbeAbsent || !result.PostReapObserved {
		t.Fatalf("result = %+v events = %v", result, o.Events())
	}
	requireCalls(t, f, "kill,probe,reap,probe,probe")
	if strings.Count(strings.Join(o.Events(), ","), "kill-group") != 1 {
		t.Fatalf("events = %v", o.Events())
	}
}

// V1-0668 hostile shapes, retired with the platform default primitives: a
// leader and member that ignore SIGTERM, and a member orphaned inside the group
// by an exited intermediate parent (a forked grandchild of the leader). Each is
// retired by the single pre-reap group SIGKILL; nothing signals the group
// after the reap.
func TestOwnerRetiresHostileGroups(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		stop         bool
	}{
		{"sigterm-ignoring", "trap '' TERM HUP INT; sleep 300 >/dev/null 2>&1 & echo $!; wait", true},
		{"orphaned-grandchild-exited-leader", "(sleep 300 >/dev/null 2>&1 & echo $!); exit 0", false},
		{"orphaned-grandchild-running-leader", "(sleep 300 >/dev/null 2>&1 & echo $!); exec sleep 300", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, member := startScriptWithMember(t, tc.script, Primitives{})
			identity := pinMember(t, member, owner.leader)
			t.Cleanup(func() { killPinned(identity) })
			if tc.name == "sigterm-ignoring" {
				// Test-owned single-process signals, not group signals: both
				// processes must survive SIGTERM, so only SIGKILL retires them.
				// The leader is this test's unreaped child, so its PID is held.
				if err := signalPinned(identity, false, syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
				syscall.Kill(owner.leader, syscall.SIGTERM)
				time.Sleep(200 * time.Millisecond)
				// A zombie answers signal 0, so liveness is read from the
				// process table: same identity, not a zombie.
				if now := pinMember(t, member, owner.leader); !now.same(identity) {
					t.Fatalf("member identity changed: %v then %v", identity, now)
				}
				select {
				case <-owner.Exited():
					t.Fatal("SIGTERM-ignoring leader exited")
				default:
				}
			}
			if tc.stop {
				owner.Stop()
			}
			result := owner.Finish(bounded(10 * time.Second))
			requireReleasedOrQuietHold(t, owner, result)
			events := owner.Events()
			reap := slices.Index(events, "reap")
			if reap < 0 || strings.Count(strings.Join(events, ","), "kill-group") != 1 || slices.Index(events, "kill-group") > reap {
				t.Fatalf("events = %v", events)
			}
			requireGone(t, member)
		})
	}
}

// ATR-V0-153: a group stopped while its leader forks a member is retired.
// Darwin's group SIGKILL can miss a child forked alongside it; that child
// then lives on in the pinned group and a single signal HOLDs at the bound
// with a live survivor. With RepeatKillWhileLive no iteration may leave one.
func TestOwnerRepeatKillRetiresMemberForkedDuringStop(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 150; i++ {
		started := filepath.Join(dir, strconv.Itoa(i))
		owner, err := StartWith(exec.Command("/bin/sh", "-c", "touch "+started+"; sleep 60"), Primitives{RepeatKillWhileLive: true})
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for _, err := os.Stat(started); err != nil; _, err = os.Stat(started) {
			if time.Now().After(deadline) {
				owner.Stop()
				owner.Finish(bounded(10 * time.Second))
				t.Fatalf("iteration %d: %s never appeared", i, started)
			}
			time.Sleep(50 * time.Microsecond)
		}
		owner.Stop()
		result := owner.Finish(bounded(10 * time.Second))
		if result.State == Hold && !slices.Contains(owner.Events(), "reap") {
			// The reap never began, so the leader is still held unreaped
			// and the group ID is ours.
			_ = syscall.Kill(-owner.leader, syscall.SIGKILL)
			t.Fatalf("iteration %d: %+v, live member survived the stop", i, result)
		}
		requireReleasedOrQuietHold(t, owner, result)
	}
}
