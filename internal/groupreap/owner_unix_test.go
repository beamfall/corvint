//go:build darwin || linux

package groupreap

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These real-process owner proofs observe Darwin signal-0 semantics (EPERM
// while only the unreaped leader remains). Linux signal 0 succeeds for a
// zombie-only group, so its real-process lifecycle, including the /proc quiet
// proof (PGO-V0-006), is observed by owner_linux_test.go instead.
func requireObservedOwnerPlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("NOT_RUN: owned-group lifecycle is observed on Darwin only")
	}
}

func bounded(d time.Duration) <-chan struct{} {
	limit := make(chan struct{})
	time.AfterFunc(d, func() { close(limit) })
	return limit
}

// startWithMember starts a leader that leaves one sleeping group member and
// prints that member's process ID.
func startWithMember(t *testing.T, tail string, p Primitives) (*Owner, int) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!; "+tail)
	command.Stdout = write
	owner, err := StartWith(command, p)
	write.Close()
	if err != nil {
		t.Fatal(err)
	}
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
	requireObservedOwnerPlatform(t)
	owner, member := startWithMember(t, "exit 0", Primitives{})
	result := owner.Finish(bounded(10 * time.Second))
	requireReleasedOrQuietHold(t, owner, result)
	if result.WaitErr != nil {
		t.Fatalf("result = %+v", result)
	}
	requireGone(t, member)
}

func TestOwnerStopRetiresRunningGroup(t *testing.T) {
	requireObservedOwnerPlatform(t)
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
	requireObservedOwnerPlatform(t)
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
		if err := leaderUnreaped(leader); err != nil {
			t.Fatal(err)
		}
		before := syscall.Kill(-leader, 0)
		if err := command.Wait(); err != nil {
			t.Fatal(err)
		}
		after := syscall.Kill(-leader, 0)
		// The member is alive and was started by this test: its own process
		// ID, not the group ID, is signalled for cleanup.
		syscall.Kill(member, syscall.SIGKILL)
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
	requireObservedOwnerPlatform(t)
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
		if len(probes) < 2 || probes[len(probes)-1] != ProbeAbsent || probes[len(probes)-2] == ProbeLive {
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
	requireObservedOwnerPlatform(t)
	p := Primitives{KillGroup: func(int) error { return syscall.EINVAL }}
	owner, member := startWithMember(t, "exit 0", p)
	result := owner.Finish(bounded(10 * time.Second))
	if result.State != Hold || slices.Contains(owner.Events(), "reap") {
		t.Fatalf("result = %+v events = %v", result, owner.Events())
	}
	// Test-owned cleanup of the processes the HOLD deliberately retained.
	syscall.Kill(member, syscall.SIGKILL)
	owner.command.Wait()
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
