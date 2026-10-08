//go:build darwin || linux

package groupreap

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestEscapeHelper is re-executed as the process tree of a detached-browser
// runner: leader -> worker (leader group) -> browser (own session) -> helper.
func TestEscapeHelper(t *testing.T) {
	role := os.Getenv("CORVINT_ESCAPE_ROLE")
	if role == "" {
		return
	}
	dir := os.Getenv("CORVINT_ESCAPE_DIR")
	spawn := func(role string, setsid bool) *exec.Cmd {
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestEscapeHelper$")
		child.Env = append(os.Environ(), "CORVINT_ESCAPE_ROLE="+role)
		// Inherit the leader's output, as a native browser does.
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if setsid {
			child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}
		if child.Start() != nil {
			os.Exit(9)
		}
		return child
	}
	ready := func() {
		for {
			if _, err := os.Stat(filepath.Join(dir, "helper.pid")); err == nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	switch role {
	case "leader":
		spawn("worker", false)
		ready()
		if os.Getenv("CORVINT_ESCAPE_LEADER") == "exit" {
			os.Exit(0)
		}
	case "orphan-leader":
		spawn("browser", true)
		ready()
		time.Sleep(3 * sampleInterval)
		os.Exit(0)
	case "worker":
		spawn("browser", true)
	case "browser":
		spawn("helper", false)
		_ = os.WriteFile(filepath.Join(dir, "browser.pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
	case "helper", "bystander":
		_ = os.WriteFile(filepath.Join(dir, role+".pid.tmp"), []byte(strconv.Itoa(os.Getpid())), 0600)
		_ = os.Rename(filepath.Join(dir, role+".pid.tmp"), filepath.Join(dir, role+".pid"))
	}
	for {
		time.Sleep(time.Second)
	}
}

type escapeFixture struct {
	t   *testing.T
	dir string
}

func newEscapeFixture(t *testing.T) escapeFixture {
	return escapeFixture{t: t, dir: t.TempDir()}
}

func (f escapeFixture) command(ctx context.Context, role, leaderMode string) *exec.Cmd {
	exe, err := os.Executable()
	if err != nil {
		f.t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestEscapeHelper$")
	cmd.Env = append(os.Environ(), "CORVINT_ESCAPE_ROLE="+role, "CORVINT_ESCAPE_DIR="+f.dir, "CORVINT_ESCAPE_LEADER="+leaderMode)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return cmd.Process.Kill() }
	return cmd
}

// identity waits for name.pid and returns that process's table identity.
func (f escapeFixture) identity(name string) Process {
	f.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(f.dir, name+".pid"))
		if err == nil && len(data) > 0 {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				f.t.Fatal(err)
			}
			table, err := snapshotProcesses()
			if err != nil {
				f.t.Fatal(err)
			}
			if p, ok := table[pid]; ok {
				f.cleanup(p)
				return p
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatalf("%s never became ready", name)
	return Process{}
}

// cleanup retires a process this test started, after re-checking identity.
func (f escapeFixture) cleanup(p Process) {
	f.t.Cleanup(func() {
		if alive(f.t, p) {
			_ = syscall.Kill(p.PID, syscall.SIGKILL)
		}
	})
}

func alive(t *testing.T, p Process) bool {
	t.Helper()
	table, err := snapshotProcesses()
	if err != nil {
		t.Fatal(err)
	}
	q, ok := table[p.PID]
	return ok && q.same(p) && q.State != StateZombie
}

func TestProcessTableIdentity(t *testing.T) {
	table, err := snapshotProcesses()
	if err != nil {
		t.Fatal(err)
	}
	self, ok := table[os.Getpid()]
	if !ok || self.PPID != os.Getppid() || self.PGID != syscall.Getpgrp() || self.Start <= 0 || self.State != StateRunning {
		t.Fatalf("own identity %+v ppid=%d pgid=%d", self, os.Getppid(), syscall.Getpgrp())
	}
	child := exec.Command("/bin/sleep", "30")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	if err := child.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		table, err = snapshotProcesses()
		if err != nil {
			t.Fatal(err)
		}
		p := table[child.Process.Pid]
		if p.PPID != os.Getpid() || p.PGID != p.PID || p.Start < self.Start {
			t.Fatalf("child identity %+v", p)
		}
		if p.State == StateStopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stopped child observed as %v", p.State)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if sid, err := sessionOf(os.Getpid()); err != nil || sid <= 0 {
		t.Fatalf("session %d %v", sid, err)
	}
}

func runContainedAsync(cmd *exec.Cmd) <-chan struct {
	c   Containment
	err error
} {
	done := make(chan struct {
		c   Containment
		err error
	}, 1)
	go func() {
		c, err := RunContained(cmd)
		done <- struct {
			c   Containment
			err error
		}{c, err}
	}()
	return done
}

func TestRunContainedRetiresEscapedSessionWithoutCollateral(t *testing.T) {
	for _, mode := range []string{"cancel", "normal-exit"} {
		t.Run(mode, func(t *testing.T) {
			f := newEscapeFixture(t)
			// A bystander in its own session, outside the owned tree, stands
			// in for the host's own browser and must never be signalled.
			bystander := f.command(context.Background(), "bystander", "")
			bystander.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := bystander.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = bystander.Process.Kill(); _ = bystander.Wait() }()
			other := f.identity("bystander")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			leaderMode := "wait"
			if mode == "normal-exit" {
				leaderMode = "exit"
			}
			cmd := f.command(ctx, "leader", leaderMode)
			done := runContainedAsync(cmd)
			browser, helper := f.identity("browser"), f.identity("helper")
			if browser.PGID != browser.PID || helper.PGID != browser.PID || browser.PGID == cmd.Process.Pid {
				t.Fatalf("fixture did not escape the leader group: browser %v helper %v leader %d", browser, helper, cmd.Process.Pid)
			}
			if mode == "cancel" {
				cancel()
			}
			var result struct {
				c   Containment
				err error
			}
			select {
			case result = <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("RunContained did not return")
			}
			if !result.c.Complete() {
				t.Fatalf("containment incomplete: %+v", result.c)
			}
			if alive(t, browser) || alive(t, helper) {
				t.Fatalf("escaped descendants survived: browser=%v helper=%v", alive(t, browser), alive(t, helper))
			}
			retired := map[int]bool{}
			for _, p := range result.c.Retired {
				retired[p.PID] = true
			}
			if !retired[browser.PID] || !retired[helper.PID] {
				t.Fatalf("retired %v, want browser %d and helper %d", result.c.Retired, browser.PID, helper.PID)
			}
			if !alive(t, other) {
				t.Fatal("bystander outside the owned tree was signalled")
			}
		})
	}
}

// An escapee orphaned before the leader exits has no structural owner at the
// sweep: it is retired only by the identity a run-time sample proved owned.
func TestRunContainedRetiresSampledOrphanByIdentity(t *testing.T) {
	f := newEscapeFixture(t)
	bystander := f.command(context.Background(), "bystander", "")
	bystander.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := bystander.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bystander.Process.Kill(); _ = bystander.Wait() }()
	other := f.identity("bystander")
	cmd := f.command(context.Background(), "orphan-leader", "")
	// A pipe makes Wait depend on every holder of the inherited output.
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	done := runContainedAsync(cmd)
	browser, helper := f.identity("browser"), f.identity("helper")
	var result struct {
		c   Containment
		err error
	}
	select {
	case result = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("RunContained waited on an orphan holding the output pipe")
	}
	if !result.c.Complete() {
		t.Fatalf("sampled orphan not contained: %+v", result.c)
	}
	retired := map[int]bool{}
	for _, p := range result.c.Retired {
		retired[p.PID] = true
	}
	if !retired[browser.PID] || !retired[helper.PID] || alive(t, browser) || alive(t, helper) {
		t.Fatalf("retired %v, want browser %d and helper %d gone", result.c.Retired, browser.PID, helper.PID)
	}
	if !alive(t, other) {
		t.Fatal("bystander outside the owned tree was signalled")
	}
}

// Sampled orphans are signalled only under an unchanged PID and start time,
// deepest first, and never twice.
func TestRetireSampledOrphansIdentityBoundary(t *testing.T) {
	seen := map[int]sampled{
		2000: {Process{PID: 2000, PGID: 2000, Start: 12}, 1},
		2001: {Process{PID: 2001, PGID: 2000, Start: 13}, 2},
		2002: {Process{PID: 2002, PGID: 2000, Start: 14}, 2}, // PID reused below
		2003: {Process{PID: 2003, PGID: 2000, Start: 15}, 2}, // zombie below
		2004: {Process{PID: 2004, PGID: 2000, Start: 16}, 2}, // already retired
		2005: {Process{PID: 2005, PGID: 2000, Start: 17}, 2}, // gone
	}
	var s fakeSignals
	s.install(t, table(
		Process{PID: 2000, PPID: 1, PGID: 2000, Start: 12, State: StateRunning},
		Process{PID: 2001, PPID: 2000, PGID: 2000, Start: 13, State: StateRunning},
		Process{PID: 2002, PPID: 1, PGID: 2002, Start: 99, State: StateRunning},
		Process{PID: 2003, PPID: 2000, PGID: 2000, Start: 15, State: StateZombie},
		Process{PID: 2004, PPID: 2000, PGID: 2000, Start: 16, State: StateRunning},
		Process{PID: 3000, PPID: 1, PGID: 3000, Start: 1, State: StateRunning}, // host browser
	))
	got, err := retireSampledOrphans(seen, []Process{{PID: 2004, Start: 16}})
	if err != nil {
		t.Fatal(err)
	}
	if sent := strings.Join(s.sent, " "); sent != "2001:KILL 2000:KILL" {
		t.Fatalf("signals %q", sent)
	}
	if len(got) != 2 || got[0].PID != 2001 || got[1].PID != 2000 {
		t.Fatalf("retired %v", got)
	}
}

type fakeSignals struct {
	sent []string
}

func signalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGSTOP:
		return "STOP"
	case syscall.SIGKILL:
		return "KILL"
	}
	return sig.String()
}

func (s *fakeSignals) install(t *testing.T, tables ...map[int]Process) {
	prevSnap, prevSig, prevGroup, prevSession := snapshotProcesses, signalProcess, signalGroup, sessionOf
	t.Cleanup(func() {
		snapshotProcesses, signalProcess, signalGroup, sessionOf = prevSnap, prevSig, prevGroup, prevSession
	})
	i := 0
	snapshotProcesses = func() (map[int]Process, error) {
		table := tables[min(i, len(tables)-1)]
		i++
		return table, nil
	}
	signalProcess = func(p Process, group bool, sig syscall.Signal) error {
		target := p.PID
		if group {
			target = -p.PID
		}
		s.sent = append(s.sent, strconv.Itoa(target)+":"+signalName(sig))
		return nil
	}
	signalGroup = func(pid int, sig syscall.Signal) error {
		s.sent = append(s.sent, "group"+strconv.Itoa(pid)+":"+signalName(sig))
		return nil
	}
	sessionOf = func(pid int) (int, error) { return pid, nil }
}

func table(ps ...Process) map[int]Process {
	out := map[int]Process{}
	for _, p := range ps {
		out[p.PID] = p
	}
	return out
}

// Identity, reuse and collateral negatives: only processes that descend from
// the stopped owned group through stopped owned parents are signalled.
func TestRetireEscapedIdentityBoundary(t *testing.T) {
	const leader = 1000
	if self := os.Getpid(); self >= leader && self <= 3002 {
		t.Skipf("test process pid %d collides with the synthetic table, which ownedTree excludes it from", self)
	}
	stoppedTree := table(
		Process{PID: leader, PPID: os.Getpid(), PGID: leader, Start: 10, State: StateZombie},
		Process{PID: 1001, PPID: 1, PGID: leader, Start: 11, State: StateStopped},  // worker
		Process{PID: 2000, PPID: 1001, PGID: 2000, Start: 12, State: StateStopped}, // browser
		Process{PID: 2001, PPID: 2000, PGID: 2000, Start: 13, State: StateStopped}, // helper
		// Host browser: own session, unrelated parent.
		Process{PID: 3000, PPID: 1, PGID: 3000, Start: 1, State: StateRunning},
		// Reused PID: parent link names the worker but predates it.
		Process{PID: 3001, PPID: 1001, PGID: 3001, Start: 5, State: StateRunning},
		// Same group as the browser but reparented away: not individually owned.
		Process{PID: 3002, PPID: 1, PGID: 2000, Start: 14, State: StateRunning},
	)
	var s fakeSignals
	s.install(t, stoppedTree)
	retired, err := retireEscaped(leader)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(s.sent, " ")
	want := "group-1000:STOP 2001:KILL -2000:KILL"
	if got != want {
		t.Fatalf("signals %q, want %q", got, want)
	}
	if len(retired) != 2 || retired[0].PID != 2001 || retired[1].PID != 2000 {
		t.Fatalf("retired %v", retired)
	}

	// A descendant whose parent never stops is never signalled to die.
	running := table(
		Process{PID: leader, PPID: os.Getpid(), PGID: leader, Start: 10, State: StateZombie},
		Process{PID: 1001, PPID: 1, PGID: leader, Start: 11, State: StateRunning},
		Process{PID: 2000, PPID: 1001, PGID: 2000, Start: 12, State: StateRunning},
	)
	s = fakeSignals{}
	s.install(t, running)
	if _, err := retireEscaped(leader); err == nil {
		t.Fatal("unstopped owned tree reported retired")
	}
	for _, sent := range s.sent {
		if !strings.HasPrefix(sent, "group-1000:") {
			t.Fatalf("signal outside the owned group without a stopped parent: %s", sent)
		}
	}

	// Session check: a non-session-leading escapee is killed individually.
	s = fakeSignals{}
	s.install(t, stoppedTree)
	sessionOf = func(int) (int, error) { return 1, nil }
	if _, err := retireEscaped(leader); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.sent, " "); got != "group-1000:STOP 2001:KILL 2000:KILL" {
		t.Fatalf("signals %q", got)
	}
}

func TestObserveSurvivorsUsesIdentity(t *testing.T) {
	retired := []Process{{PID: 2000, Start: 12}, {PID: 2001, Start: 13}}
	var s fakeSignals
	s.install(t, table(
		Process{PID: 2000, Start: 99, State: StateRunning}, // PID reused: not a survivor
		Process{PID: 2001, Start: 13, State: StateZombie},  // dead
	))
	survivors, err := observeSurvivors(retired, nil)
	if err != nil || len(survivors) != 0 || len(s.sent) != 0 {
		t.Fatalf("survivors %v err %v signals %v", survivors, err, s.sent)
	}
	s.install(t, table(Process{PID: 2001, Start: 13, State: StateStopped}))
	survivors, err = observeSurvivors(retired, nil)
	if err != nil || len(survivors) != 1 || survivors[0].PID != 2001 || len(s.sent) != 0 {
		t.Fatalf("survivors %v err %v signals %v", survivors, err, s.sent)
	}
}

func TestRunContainedPropagatesTableFailure(t *testing.T) {
	prev := snapshotProcesses
	t.Cleanup(func() { snapshotProcesses = prev })
	snapshotProcesses = func() (map[int]Process, error) { return nil, errors.New("injected") }
	c, err := RunContained(exec.Command("/bin/sh", "-c", "exit 0"))
	if err != nil || c.Complete() || len(c.Retired) != 0 {
		t.Fatalf("containment %+v err %v", c, err)
	}
}

// /proc/<pid>/stat reports only the leader thread, so a stopped or zombie
// leader is classified from every task: stopped only when every live task is
// group-stopped, a zombie only when no task is live. A tracing stop is not a
// stop.
func TestReadProcTableThreadAwareState(t *testing.T) {
	f := newFakeProc(t)
	line := func(id int, state string) string {
		return strconv.Itoa(id) + " (x) " + state + " 1 9 9" + strings.Repeat(" 0", 15) + " 777 0\n"
	}
	proc := func(pid int, leader string, tasks map[int]string) {
		f.write(filepath.Join(strconv.Itoa(pid), "stat"), line(pid, leader))
		for tid, state := range tasks {
			f.write(filepath.Join(strconv.Itoa(pid), "task", strconv.Itoa(tid), "stat"), line(tid, state))
		}
	}
	proc(200, "Z", map[int]string{200: "Z", 210: "S"}) // live thread: running
	proc(201, "Z", map[int]string{201: "Z"})           // dead
	proc(202, "T", map[int]string{202: "T", 212: "T"}) // whole group stopped
	proc(203, "T", map[int]string{203: "T", 213: "R"}) // partial stop: running
	proc(204, "Z", map[int]string{204: "Z", 214: "T"}) // exited leader, stopped thread
	proc(205, "t", map[int]string{205: "t"})           // tracing stop: running
	proc(206, "S", nil)                                // running leader needs no task read
	got, err := readProcTable(f.root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]ProcState{200: StateRunning, 201: StateZombie, 202: StateStopped, 203: StateRunning, 204: StateStopped, 205: StateRunning, 206: StateRunning}
	for pid, state := range want {
		if got[pid].State != state || got[pid].Start != 777 || got[pid].PGID != 9 {
			t.Errorf("pid %d: %+v, want state %d", pid, got[pid], state)
		}
	}
}

// Identity-pinned signalling: a PID whose start time no longer matches (a
// reused PID, as after an auto-reaping parent released it) is never
// signalled; the matching identity is.
func TestSignalPinnedIdentity(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})
	table, err := readProcessTable()
	if err != nil {
		t.Fatal(err)
	}
	p, ok := table[cmd.Process.Pid]
	if !ok {
		t.Fatal("started process missing from the table")
	}
	reused := p
	reused.Start--
	if err := signalPinned(reused, false, syscall.SIGKILL); !errors.Is(err, errIdentityChanged) {
		t.Fatalf("reused identity: %v", err)
	}
	select {
	case <-done:
		t.Fatal("a changed identity was signalled")
	case <-time.After(200 * time.Millisecond):
	}
	if err := signalPinned(p, false, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("pinned identity was not signalled")
	}
	if err := signalPinned(p, false, syscall.SIGKILL); !notSignalled(err) {
		t.Fatalf("reaped identity: %v", err)
	}
}

// Identities beyond the bound make sampling incomplete; a known identity
// still updates.
func TestRecordEscapedBound(t *testing.T) {
	// Synthetic PIDs above Linux's pid_max never collide with this process,
	// which ownedTree excludes.
	const leader = 1 << 23
	tbl := table(Process{PID: leader, PPID: os.Getpid(), PGID: leader, Start: 1, State: StateRunning})
	for i := 0; i <= maxEscaped; i++ {
		pid := leader + 1 + i
		tbl[pid] = Process{PID: pid, PPID: leader, PGID: pid, Start: 2, State: StateRunning}
	}
	seen := map[int]sampled{}
	if recordEscaped(seen, tbl, leader) || len(seen) != maxEscaped {
		t.Fatalf("overflow not reported: %d tracked", len(seen))
	}
	for pid := range seen {
		if !recordEscaped(seen, table(tbl[leader], tbl[pid]), leader) {
			t.Fatal("a tracked identity counted as overflow")
		}
		break
	}
}
