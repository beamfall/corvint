//go:build darwin || linux

package supervisor

import (
	"context"
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

func TestCALV0077_EscapeObservationUncertain(t *testing.T) {
	x := newEscapes(100)
	x.identity = func(pid int) (string, error) { return "id-" + strconv.Itoa(pid), nil }
	x.exists = func(int) (bool, error) { return false, nil }
	x.rows = func() ([]processRow, error) {
		return []processRow{{pid: 100, ppid: 1, pgid: 100}, {pid: 101, ppid: 100, pgid: 100}, {pid: 102, ppid: 101, pgid: 102}, {pid: 103, ppid: 102, pgid: 103}, {pid: 104, ppid: 1, pgid: 104}}, nil
	}
	if !x.scan() || x.uncertain || len(x.groups) != 2 || x.groups[102] != "id-102" || x.groups[103] != "id-103" {
		t.Fatalf("escapes %+v uncertain %v", x.groups, x.uncertain)
	}
	x = newEscapes(100)
	x.identity = func(pid int) (string, error) { return "id", nil }
	x.rows = func() ([]processRow, error) {
		return []processRow{{pid: 100, ppid: 1, pgid: 100}, {pid: 102, ppid: 100, pgid: 7}}, nil
	}
	if x.scan() || !x.uncertain || x.drain() {
		t.Fatal("escape joined to a foreign group was not uncertain")
	}
}

// TestCALV0077_RecoverDetachedHost: recovery of a detached host drains its
// escapes only when the host itself is still alive to observe them through;
// with the host gone, quiescence stays uncertain.
func TestCALV0077_RecoverDetachedHost(t *testing.T) {
	shortenDetached(t)
	for _, tc := range []struct {
		name   string
		script string
		want   bool
	}{
		{"host alive", "sh -c '" + strings.ReplaceAll(detachedServer, "'", `'\''`) + " & wait' & wait", true},
		{"host gone", "sleep 300", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, pidf := detachedCapsule(t, tc.script)
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
			if tc.want {
				for i := 0; i < 100; i++ {
					if _, e := os.Stat(pidf); e == nil {
						break
					}
					time.Sleep(20 * time.Millisecond)
				}
			}
			started, e := ProcessIdentity(cmd.Process.Pid)
			if e != nil {
				t.Fatal(e)
			}
			if got := RecoverHost(dir, Boot{PID: cmd.Process.Pid, Started: started}); got != tc.want {
				t.Fatalf("recover %v, want %v", got, tc.want)
			}
			if tc.want && !serverGone(t, pidf) {
				t.Fatal("recovered host's server survived")
			}
		})
	}
}
