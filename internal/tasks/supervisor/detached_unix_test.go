//go:build darwin || linux

package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// detachedServer is a fake OpenCode standalone server: it leaves the host's
// process group with setsid, records its PID, inherits the host standard
// error, and sleeps; IGN=1 makes it ignore SIGTERM. ORPHAN=1 first forks so
// that its parent exits before it leaves the group, the shape no
// observation through a live parent can find.
const detachedServer = `perl -e 'use POSIX; if ($ENV{ORPHAN}) { exit 0 if fork; select(undef,undef,undef,0.1) } POSIX::setsid(); open F, ">", $ENV{PIDF}; print F $$; close F; $SIG{TERM}="IGNORE" if $ENV{IGN}; sleep 300'`

func detachedCapsule(t *testing.T, script string, env ...string) (Capsule, string) {
	t.Helper()
	if _, e := os.Stat("/usr/bin/perl"); e != nil {
		t.Skip("perl unavailable for the fake detached server")
	}
	exe, e := filepath.EvalSymlinks("/bin/sh")
	if e != nil {
		t.Fatal(e)
	}
	binary, e := ReadBounded(exe, 256<<20)
	if e != nil {
		t.Fatal(e)
	}
	pidf := filepath.Join(t.TempDir(), "server.pid")
	c := Capsule{Profile: "taskman-codex-supervisor/0", Effect: Digest([]byte(t.Name())), Executable: exe, ExecutableSHA256: Digest(binary), Directory: t.TempDir(), Host: HostOpenCode,
		Env: append([]string{"PATH=/usr/bin:/bin", "OPENCODE_PRINT_LOGS=1", "PIDF=" + pidf}, env...), Argv: []string{"-c", script}}
	return c, pidf
}

// serverGone reads the fake server PID and reports whether it no longer
// runs; a survivor is killed so no test leaks it.
func serverGone(t *testing.T, pidf string) bool {
	t.Helper()
	raw, e := os.ReadFile(pidf)
	if e != nil {
		t.Fatalf("server never started: %v", e)
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(raw)))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 50; i++ {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return true
		}
		if id, _ := ProcessIdentity(pid); id == "" {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	return false
}

func shortenDetached(t *testing.T) {
	force, quiesce := escapeForceAfter, detachedQuiesce
	escapeForceAfter, detachedQuiesce = 300*time.Millisecond, time.Second
	t.Cleanup(func() { escapeForceAfter, detachedQuiesce = force, quiesce })
}

func TestCALV0077_DetachedHostEnvRequired(t *testing.T) {
	c, _ := detachedCapsule(t, "exit 0")
	if e := ValidateCapsule(c); e != nil {
		t.Fatalf("detached capsule refused: %v", e)
	}
	for name, env := range map[string][]string{
		"absent":   {"PATH=/usr/bin:/bin"},
		"disabled": {"PATH=/usr/bin:/bin", "OPENCODE_PRINT_LOGS=0"},
		"last off": {"OPENCODE_PRINT_LOGS=1", "OPENCODE_PRINT_LOGS=0"},
	} {
		c.Env = env
		if e := ValidateCapsule(c); e == nil {
			t.Errorf("%s: detached capsule without inherited server logs admitted", name)
		}
	}
}

// TestCALV0077_DetachedServerTimeout: the host hangs, the run deadline
// interrupts it, and the escaped server group is discovered, drained and
// proved gone by end of file on the host output.
func TestCALV0077_DetachedServerTimeout(t *testing.T) {
	shortenDetached(t)
	c, pidf := detachedCapsule(t, detachedServer+" & wait")
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	out, e := Run(ctx, os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
	if e == nil || out.Class != "INTERRUPTED" || !out.Clean {
		t.Fatalf("timeout outcome %+v %v", out, e)
	}
	if !serverGone(t, pidf) {
		t.Fatal("escaped server survived a clean stop")
	}
}

// TestCALV0077_DetachedServerForcedKill: a server that ignores SIGTERM is
// killed after escapeForceAfter.
func TestCALV0077_DetachedServerForcedKill(t *testing.T) {
	shortenDetached(t)
	c, pidf := detachedCapsule(t, detachedServer+" & wait", "IGN=1")
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	out, _ := Run(ctx, os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
	if !out.Clean {
		t.Fatalf("forced-kill outcome %+v", out)
	}
	if !serverGone(t, pidf) {
		t.Fatal("TERM-ignoring server survived a clean stop")
	}
}

// TestCALV0077_DetachedServerHostCrash: the host dies by SIGKILL while its
// server runs; the server, discovered while the host lived, is drained.
func TestCALV0077_DetachedServerHostCrash(t *testing.T) {
	shortenDetached(t)
	c, pidf := detachedCapsule(t, detachedServer+" & sleep 0.6; kill -9 $$")
	out, e := Run(context.Background(), os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
	if e == nil || out.Class != "EXIT_NONZERO" || !out.Clean {
		t.Fatalf("crash outcome %+v %v", out, e)
	}
	if !serverGone(t, pidf) {
		t.Fatal("crashed host's server survived a clean stop")
	}
}

// TestCALV0077_DetachedOrphanFailsClosed: a process that left the group after
// its parent exited is never discovered; it still holds the host standard
// error, so the stop is not clean.
func TestCALV0077_DetachedOrphanFailsClosed(t *testing.T) {
	shortenDetached(t)
	c, pidf := detachedCapsule(t, detachedServer+"; sleep 0.5", "ORPHAN=1")
	out, _ := Run(context.Background(), os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
	gone := serverGone(t, pidf)
	if out.Clean {
		t.Fatalf("undiscovered orphan reported clean (orphan gone=%v): %+v", gone, out)
	}
}

// fakeTable is a synthetic process table for escape observation tests.
type fakeTable struct {
	rows     []processRow
	identity map[int]string
	signaled map[int]bool
}

func (f *fakeTable) escapes(owned Boot) *escapes {
	x := newEscapes(owned)
	x.rows = func() ([]processRow, error) { return f.rows, nil }
	x.identity = func(pid int) (string, error) { return f.identity[pid], nil }
	x.groupOf = func(pid int) (int, error) {
		for _, r := range f.rows {
			if r.pid == pid {
				return r.pgid, nil
			}
		}
		return 0, syscall.ESRCH
	}
	x.exists = func(group int) (bool, error) {
		for _, r := range f.rows {
			if r.pgid == group && !r.zombie {
				return true, nil
			}
		}
		return false, nil
	}
	x.newGroup = func(b Boot) *ownedGroup {
		g := newOwnedGroup(b)
		g.identity, g.groupOf, g.exists = x.identity, x.groupOf, x.exists
		g.inventory = func(group int) (map[int]string, error) {
			m := map[int]string{}
			for _, r := range f.rows {
				if r.pgid == group {
					m[r.pid] = f.identity[r.pid]
				}
			}
			return m, nil
		}
		g.signal = func(targets map[int]string, _ syscall.Signal) error {
			for pid := range targets {
				f.signaled[pid] = true
			}
			return fmt.Errorf("synthetic table cannot retire processes")
		}
		return g
	}
	return x
}

func TestCALV0077_EscapeObservationUncertain(t *testing.T) {
	f := &fakeTable{rows: []processRow{{pid: 100, ppid: 1, pgid: 100}, {pid: 101, ppid: 100, pgid: 100}, {pid: 102, ppid: 101, pgid: 102}, {pid: 103, ppid: 102, pgid: 103}, {pid: 104, ppid: 1, pgid: 104}},
		identity: map[int]string{100: "id-100", 101: "id-101", 102: "id-102", 103: "id-103", 104: "id-104"}}
	x := f.escapes(Boot{PID: 100, Started: "id-100"})
	x.scan()
	if x.uncertain || len(x.groups) != 2 || x.groups[102] != "id-102" || x.groups[103] != "id-103" {
		t.Fatalf("escapes %+v uncertain %v", x.groups, x.uncertain)
	}
	f = &fakeTable{rows: []processRow{{pid: 100, ppid: 1, pgid: 100}, {pid: 102, ppid: 100, pgid: 7}}, identity: map[int]string{100: "id", 102: "id"}}
	x = f.escapes(Boot{PID: 100, Started: "id"})
	if x.scan(); !x.uncertain || x.drain() {
		t.Fatal("escape joined to a foreign group was not uncertain")
	}
}

// TestCALV0077_EscapeGroupReuse: an escape that exits leaves its PGID free;
// a process that reuses it, and its children, are never adopted or signalled,
// whether the reuse happened before the discovery snapshot or after it, and a
// candidate is recorded only under the identity it held across a snapshot.
func TestCALV0077_EscapeGroupReuse(t *testing.T) {
	owned := Boot{PID: 100, Started: "h"}
	live := []processRow{{pid: 100, ppid: 1, pgid: 100}, {pid: 101, ppid: 100, pgid: 100}}
	f := &fakeTable{rows: append(live[:2:2], processRow{pid: 500, ppid: 101, pgid: 500}), identity: map[int]string{100: "h", 101: "c", 500: "e1"}, signaled: map[int]bool{}}
	x := f.escapes(owned)
	if x.scan(); x.uncertain || x.groups[500] != "e1" {
		t.Fatalf("escape not recorded: %+v uncertain %v", x.groups, x.uncertain)
	}
	// The escape exits; an unrelated process reuses PID and group 500 and
	// starts a child that leads its own group.
	f.rows = append(live[:2:2], processRow{pid: 500, ppid: 1, pgid: 500}, processRow{pid: 600, ppid: 500, pgid: 600})
	f.identity[500], f.identity[600] = "u", "u-child"
	if x.scan(); x.uncertain || len(x.groups) != 0 {
		t.Fatalf("reused group adopted: %+v uncertain %v", x.groups, x.uncertain)
	}
	if !x.drain() || len(f.signaled) != 0 {
		t.Fatalf("drain after reuse signalled %v", f.signaled)
	}
	// Reuse that completes just as the next snapshot is taken: the identity
	// still reads "e1" until the process table is read. An anchor checked
	// before the snapshot would adopt the reused group's child.
	f = &fakeTable{rows: append(live[:2:2], processRow{pid: 500, ppid: 101, pgid: 500}), identity: map[int]string{100: "h", 101: "c", 500: "e1"}, signaled: map[int]bool{}}
	x = f.escapes(owned)
	x.scan()
	snapshot := x.rows
	x.rows = func() ([]processRow, error) {
		f.rows = append(live[:2:2], processRow{pid: 500, ppid: 1, pgid: 500}, processRow{pid: 600, ppid: 500, pgid: 600})
		f.identity[500], f.identity[600] = "u", "u-child"
		return snapshot()
	}
	if x.scan(); x.uncertain || len(x.groups) != 0 || !x.drain() || len(f.signaled) != 0 {
		t.Fatalf("group reused at the snapshot adopted: %+v uncertain %v signalled %v", x.groups, x.uncertain, f.signaled)
	}
	// A candidate whose PID changes owner between snapshots is recorded only
	// under the identity seen on both sides of one snapshot.
	f = &fakeTable{rows: append(live[:2:2], processRow{pid: 700, ppid: 101, pgid: 700}), identity: map[int]string{100: "h", 101: "c", 700: "first"}, signaled: map[int]bool{}}
	x = f.escapes(owned)
	reads := 0
	base := x.identity
	x.identity = func(pid int) (string, error) {
		if pid == 700 {
			if reads++; reads > 1 {
				return "second", nil
			}
		}
		return base(pid)
	}
	if x.scan(); x.groups[700] != "second" {
		t.Fatalf("candidate recorded as %q", x.groups[700])
	}
}

// TestCALV0077_RecoverDetachedHost: recovery of a detached host drains the
// escapes it can observe but never proves quiescence, with the host alive or
// gone, since no witness survives the supervisor crash.
func TestCALV0077_RecoverDetachedHost(t *testing.T) {
	shortenDetached(t)
	for _, tc := range []struct {
		name   string
		script string
		server bool
	}{
		{"host alive", "sh -c '" + strings.ReplaceAll(detachedServer, "'", `'\''`) + " & wait' & wait", true},
		{"host gone", "sleep 300", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, pidf, boot := startRecoveryHost(t, tc.script)
			if tc.server {
				waitFile(t, pidf)
			}
			if RecoverHost(dir, boot) {
				t.Fatal("detached recovery proved quiescence")
			}
			if tc.server && !serverGone(t, pidf) {
				t.Fatal("observed escape survived recovery")
			}
		})
	}
}

// TestCALV0077_RecoverDetachedLateEscape: the host starts its server after
// recovery's discovery snapshot and before the drain. The server escapes the
// drain, and recovery still does not prove quiescence.
func TestCALV0077_RecoverDetachedLateEscape(t *testing.T) {
	shortenDetached(t)
	trigger := filepath.Join(t.TempDir(), "go")
	script := "while [ ! -e " + trigger + " ]; do sleep 0.02; done; " + detachedServer + " & wait"
	dir, pidf, boot := startRecoveryHost(t, script)
	hook := afterRecoveryScan
	t.Cleanup(func() { afterRecoveryScan = hook })
	afterRecoveryScan = func() {
		if e := os.WriteFile(trigger, nil, 0o600); e != nil {
			t.Error(e)
		}
		waitFile(t, pidf)
	}
	if RecoverHost(dir, boot) {
		t.Fatal("recovery proved quiescence with a late escape")
	}
	if serverGone(t, pidf) {
		t.Fatal("late escape was drained; the window was not exercised")
	}
}

func startRecoveryHost(t *testing.T, script string) (string, string, Boot) {
	t.Helper()
	c, pidf := detachedCapsule(t, script)
	dir := t.TempDir()
	if e := Publish(dir, "capsule", c); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(c.Executable, c.Argv...)
	cmd.Env = c.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	go cmd.Wait()
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	started, e := ProcessIdentity(cmd.Process.Pid)
	if e != nil {
		t.Fatal(e)
	}
	return dir, pidf, Boot{PID: cmd.Process.Pid, Started: started}
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 250; i++ {
		if raw, e := os.ReadFile(path); e == nil && len(raw) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("%s never appeared", path)
}
