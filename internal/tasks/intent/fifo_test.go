//go:build darwin || linux

package intent

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const fifoChildLifetime = 15 * time.Second

// A blocking os.Open cannot be cancelled in a goroutine if opening the FIFO's
// release end fails. Confine that old-boundary witness to a killable child.
// The parent polls/reaps directly; signal cancellation is stopped after cleanup.
func TestTMV0008_AS07_FIFOChild(t *testing.T) {
	args := os.Args
	if len(args) < 6 || args[len(args)-5] != "atm-fifo-child" {
		return
	}
	// Backup for an abruptly lost parent; ordinary cancellation still requires
	// the parent to kill and reap us. The timer is stopped on every normal exit.
	lifetime := time.AfterFunc(fifoChildLifetime, func() { os.Exit(124) })
	defer lifetime.Stop()
	mode, path, ready, fault := args[len(args)-4], args[len(args)-3], args[len(args)-2], args[len(args)-1]
	old := openReadFile
	defer func() { openReadFile = old }()
	replace := func() error {
		if fault == "remove" {
			return syscall.EACCES
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		if fault == "mkfifo" {
			return syscall.ENOSPC
		}
		if err := syscall.Mkfifo(path, 0600); err != nil {
			return err
		}
		return os.WriteFile(ready, []byte("ready"), 0600)
	}
	var err error
	if mode == "root" {
		root, e := safeopen.Root(filepath.Dir(path))
		if e != nil {
			t.Fatal(e)
		}
		defer root.Close()
		if _, e = statRegular(root, path, "record", 32); e != nil {
			t.Fatal(e)
		}
		if e = replace(); e != nil {
			t.Fatal(e)
		}
		_, err = readInRoot(root, path, "record", 32)
		if wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("FIFO: %v", err)
		}
		return
	}
	openReadFile = func(p string) (*os.File, error) {
		if err := replace(); err != nil {
			return nil, err
		}
		if mode == "legacy" {
			return os.Open(p)
		}
		return safeopen.File(p)
	}
	_, err = readBounded(path, 32)
	if fault != "" {
		t.Fatalf("injected setup failure: %v", err)
	}
	if wire.CodeOf(err) != wire.CodeSnapshotMoved {
		t.Fatalf("replacement not refused: %v", err)
	}
}

type fifoObservation struct {
	ready, blocked bool
	status         syscall.WaitStatus
	output         string
}

// Cleanup is registered before launch. All readiness, completion and reaping
// waits are bounded, including setup failure and failure to open the release end.
type fifoHooks struct {
	base     string
	launched func(int)
	blocked  func(int)
}

func fifoWitness(t *testing.T, mode, fault string, interrupt syscall.Signal, hooks ...fifoHooks) (observed fifoObservation) {
	t.Helper()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // registered first: subscription ends only after child cleanup
	cleaning := false
	checkCancellation := func() {
		if !cleaning && ctx.Err() != nil {
			t.Fatal("FIFO parent interrupted")
		}
	}
	baseDir := ""
	if len(hooks) != 0 {
		baseDir = hooks[0].base
	}
	if baseDir == "" {
		baseDir = t.TempDir()
	}
	base, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	path, ready := filepath.Join(base, "record"), filepath.Join(base, "ready")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(base, "child.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	var child *os.Process
	var childPID int
	var release *os.File
	joined := false
	poll := func() bool {
		if joined {
			return true
		}
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(child.Pid, &status, syscall.WNOHANG, nil)
		if err == syscall.EINTR {
			return false
		}
		if err != nil {
			t.Fatalf("wait child: %v", err)
		}
		if pid == 0 {
			return false
		}
		joined = true
		observed.status = status
		if err := child.Release(); err != nil {
			t.Errorf("release process handle: %v", err)
		}
		return true
	}
	wait := func(duration time.Duration) bool {
		deadline := time.Now().Add(duration)
		for time.Now().Before(deadline) {
			checkCancellation()
			if poll() {
				return true
			}
			time.Sleep(time.Millisecond)
		}
		return poll()
	}
	cleanup := func() {
		cleaning = true
		if child != nil && !joined {
			if err := child.Kill(); err != nil && err != os.ErrProcessDone {
				t.Errorf("kill child: %v", err)
			}
			if !wait(5 * time.Second) {
				t.Errorf("killed FIFO child was not reaped within 5s")
			}
		}
		if release != nil {
			if err := release.Close(); err != nil {
				t.Errorf("close FIFO release: %v", err)
			}
			release = nil
		}
	}
	defer cleanup()
	child, err = os.StartProcess(os.Args[0], []string{os.Args[0], "-test.run=^TestTMV0008_AS07_FIFOChild$", "--", "atm-fifo-child", mode, path, ready, fault}, &os.ProcAttr{Env: os.Environ(), Files: []*os.File{nil, log, log}})
	if err != nil {
		t.Fatal(err)
	}
	childPID = child.Pid
	if len(hooks) != 0 && hooks[0].launched != nil {
		hooks[0].launched(childPID)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		checkCancellation()
		_, err := os.Stat(ready)
		observed.ready = err == nil
		if observed.ready || poll() {
			break
		}
		if !os.IsNotExist(err) {
			t.Fatalf("readiness: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("FIFO readiness timed out")
		}
		time.Sleep(time.Millisecond)
	}
	if observed.ready {
		observed.blocked = !wait(2 * time.Second)
		if observed.blocked {
			if len(hooks) != 0 && hooks[0].blocked != nil {
				hooks[0].blocked(childPID)
			}
			if fault == "parent" {
				if !wait(5 * time.Second) {
					t.Fatal("FIFO parent completion timed out")
				}
			} else if fault == "deadline" {
				cleanup() // the readiness-to-completion deadline already elapsed
			} else if interrupt != 0 {
				if err := child.Signal(interrupt); err != nil {
					t.Fatal(err)
				}
			} else if fault == "release" {
				// Inject the release-open failure and exercise the exact same kill/reap
				// path used for an actual failure, rather than abandoning blocked work.
				err = syscall.EIO
			} else {
				release, err = os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
			}
			if err != nil {
				cleanup()
			}
			if !wait(5 * time.Second) {
				t.Fatal("FIFO completion timed out")
			}
		}
	}
	cleanup()
	if !joined {
		t.Fatal("FIFO child not joined")
	}
	if err := syscall.Kill(childPID, 0); err != syscall.ESRCH {
		t.Fatalf("reaped child remains: %v", err)
	}
	raw, err := os.ReadFile(log.Name())
	if err != nil {
		t.Fatal(err)
	}
	observed.output = string(raw)
	return observed
}

func TestTMV0008_AS07_ReadBoundedFIFORegression(t *testing.T) {
	for _, mode := range []string{"legacy", "safe"} {
		t.Run(mode, func(t *testing.T) {
			got := fifoWitness(t, mode, "", 0)
			t.Logf("boundary=%s ready=%v blocked=%v exit=%v", mode, got.ready, got.blocked, got.status)
			if !got.ready || got.blocked != (mode == "legacy") || got.status != 0 {
				t.Fatalf("FIFO: %+v", got)
			}
		})
	}
}
func TestTMV0008_AS07_ReadInRootFIFORegression(t *testing.T) {
	got := fifoWitness(t, "root", "", 0)
	if !got.ready || got.blocked || got.status != 0 {
		t.Fatalf("FIFO: %+v", got)
	}
}
func TestTMV0008_AS07_FIFOSetupFailure(t *testing.T) {
	for _, fault := range []string{"mkfifo", "remove"} {
		t.Run(fault, func(t *testing.T) {
			got := fifoWitness(t, "legacy", fault, 0)
			want := syscall.ENOSPC
			if fault == "remove" {
				want = syscall.EACCES
			}
			if got.ready || got.status.ExitStatus() != 1 || !strings.Contains(got.output, want.Error()) {
				t.Fatalf("setup failure: %+v", got)
			}
		})
	}
}
func TestTMV0008_AS07_FIFOReleaseFailureAndInterruption(t *testing.T) {
	for _, sig := range []syscall.Signal{0, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL} {
		t.Run(fmt.Sprint(sig), func(t *testing.T) {
			fault := ""
			if sig == 0 {
				fault = "release"
			}
			got := fifoWitness(t, "legacy", fault, sig)
			want := sig
			if sig == 0 {
				want = syscall.SIGKILL
			}
			if !got.ready || !got.blocked || !got.status.Signaled() || got.status.Signal() != want {
				t.Fatalf("interruption: %+v", got)
			}
		})
	}
}

func TestTMV0008_AS07_FIFODeadline(t *testing.T) {
	got := fifoWitness(t, "legacy", "deadline", 0)
	if !got.ready || !got.blocked || !got.status.Signaled() || got.status.Signal() != syscall.SIGKILL {
		t.Fatalf("deadline: %+v", got)
	}
}

// This helper is the actual parent of the FIFO child. Only the outer fixture
// signals its PID; no process group or signal sent to the child stands in for it.
func TestTMV0008_AS07_FIFOParentHelper(t *testing.T) {
	args := os.Args
	if len(args) < 4 || args[len(args)-3] != "atm-fifo-parent" {
		return
	}
	base, action := args[len(args)-2], args[len(args)-1]
	writePID := func(name string, pid int) {
		path := filepath.Join(base, name)
		if err := os.WriteFile(path+".tmp", []byte(strconv.Itoa(pid)), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			t.Fatal(err)
		}
	}
	fifoWitness(t, "legacy", "parent", 0, fifoHooks{
		base:     base,
		launched: func(pid int) { writePID("child.pid", pid) },
		blocked: func(pid int) {
			writePID("blocked.pid", pid)
			if action == "assertion" {
				t.Fatal("controlled parent assertion")
			}
		},
	})
	t.Fatal("parent wait unexpectedly returned")
}

func TestTMV0008_AS07_FIFOParentCancellation(t *testing.T) {
	for _, action := range []string{"INT", "TERM", "assertion", "deadline", "expiry", "owner-return", "owner-timeout", "owner-cancel", "owner-INT", "owner-TERM"} {
		t.Run(action, func(t *testing.T) { fifoParentWitness(t, action) })
	}
}

func awaitPublishedChildExpiry(pid int, duration time.Duration, wait func(time.Duration, func() bool) bool, probe func(int, syscall.Signal) error) bool {
	return wait(duration, func() bool { return pid > 1 && probe(pid, 0) == syscall.ESRCH })
}

func TestTMV0008_AS07_FIFOStaleChildPIDIsDiagnosticOnly(t *testing.T) {
	const unrelatedPID = 4242
	var probes []syscall.Signal
	probe := func(pid int, sig syscall.Signal) error {
		if pid != unrelatedPID {
			t.Fatalf("probed PID %d, want controlled unrelated PID %d", pid, unrelatedPID)
		}
		probes = append(probes, sig)
		return nil // the unrelated identity remains live through the deadline
	}
	wait := func(duration time.Duration, done func() bool) bool {
		if duration != fifoChildLifetime+5*time.Second {
			t.Fatalf("fallback wait %v does not accommodate child lifetime %v", duration, fifoChildLifetime)
		}
		return done()
	}
	if awaitPublishedChildExpiry(unrelatedPID, fifoChildLifetime+5*time.Second, wait, probe) {
		t.Fatal("stale PID reported gone")
	}
	if len(probes) == 0 {
		t.Fatal("stale PID was not inspected")
	}
	for _, sig := range probes {
		if sig != 0 {
			t.Fatalf("signal %v sent to unrelated live identity", sig)
		}
	}
}

// The outer fixture owns the helper and tracks its child's published PID only
// for bounded diagnostics. Destructive child cleanup remains in the helper while
// it can reap; a stale numeric PID is never signalled by the outer fixture. All
// paths retain the subscription until cleanup.
func fifoParentWitness(t *testing.T, action string) {
	t.Helper()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(base, "parent.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	var parent *os.Process
	var parentPID int
	var status syscall.WaitStatus
	joined := false
	childPID := 0
	readPID := func(name string) (int, bool) {
		raw, err := os.ReadFile(filepath.Join(base, name))
		if os.IsNotExist(err) {
			return 0, false
		}
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			return 0, false
		}
		pid, err := strconv.Atoi(string(raw))
		if err != nil || pid <= 1 {
			t.Errorf("invalid %s: %q", name, raw)
			return 0, false
		}
		return pid, true
	}
	poll := func() bool {
		if joined {
			return true
		}
		pid, err := syscall.Wait4(parent.Pid, &status, syscall.WNOHANG, nil)
		if err == syscall.EINTR {
			return false
		}
		if err != nil {
			t.Errorf("wait helper parent: %v", err)
			return false
		}
		if pid == 0 {
			return false
		}
		joined = true
		if err := parent.Release(); err != nil {
			t.Errorf("release helper parent: %v", err)
		}
		return true
	}
	wait := func(duration time.Duration, done func() bool) bool {
		deadline := time.Now().Add(duration)
		for time.Now().Before(deadline) {
			if done() {
				return true
			}
			time.Sleep(time.Millisecond)
		}
		return done()
	}
	childGone := func() bool { return childPID > 1 && syscall.Kill(childPID, 0) == syscall.ESRCH }
	cleanup := func() {
		if parent == nil {
			return
		}
		if !joined {
			if err := parent.Signal(syscall.SIGTERM); err != nil && err != os.ErrProcessDone {
				t.Errorf("cancel helper: %v", err)
			}
			wait(6*time.Second, poll)
		}
		if childPID == 0 {
			childPID, _ = readPID("child.pid")
		}
		if !joined {
			if err := parent.Kill(); err != nil && err != os.ErrProcessDone {
				t.Errorf("kill helper parent: %v", err)
			}
			if !wait(5*time.Second, poll) {
				t.Error("helper parent was not reaped within 5s")
			}
		}
		if childPID > 1 && !awaitPublishedChildExpiry(childPID, fifoChildLifetime+5*time.Second, wait, syscall.Kill) {
			t.Error("published FIFO child PID still live after independent deadline; ownership unknown, no signal sent")
		}
	}
	var blocked bool
	reason := func() string {
		defer cleanup() // includes assertion failures and every early return below
		parent, err = os.StartProcess(os.Args[0], []string{os.Args[0], "-test.run=^TestTMV0008_AS07_FIFOParentHelper$", "--", "atm-fifo-parent", base, action}, &os.ProcAttr{Env: os.Environ(), Files: []*os.File{nil, log, log}})
		if err != nil {
			t.Fatal(err)
		}
		parentPID = parent.Pid
		ready := wait(5*time.Second, func() bool {
			if ctx.Err() != nil {
				return true
			}
			childPID, blocked = readPID("blocked.pid")
			return blocked || poll()
		})
		if ctx.Err() != nil {
			return "interrupted"
		}
		if !ready || !blocked {
			t.Fatal("helper never observed its FIFO child blocked")
		}
		if childPID <= 1 {
			t.Fatal("missing owned child PID")
		}
		switch action {
		case "owner-return":
			return "early return"
		case "owner-timeout":
			wait(20*time.Millisecond, func() bool { return ctx.Err() != nil })
			return "owner timeout"
		case "owner-cancel":
			cancel()
		case "owner-INT", "owner-TERM":
			sig := syscall.SIGINT
			if action == "owner-TERM" {
				sig = syscall.SIGTERM
			}
			if err := syscall.Kill(os.Getpid(), sig); err != nil {
				t.Fatal(err)
			}
		case "INT", "TERM":
			sig := syscall.SIGINT
			if action == "TERM" {
				sig = syscall.SIGTERM
			}
			if err := parent.Signal(sig); err != nil {
				t.Fatal(err)
			}
		case "expiry":
			if err := parent.Kill(); err != nil && err != os.ErrProcessDone {
				t.Fatal(err)
			}
			if !wait(5*time.Second, poll) {
				t.Fatal("orphaning helper was not reaped within 5s")
			}
			if !awaitPublishedChildExpiry(childPID, fifoChildLifetime+5*time.Second, wait, syscall.Kill) {
				t.Fatal("orphaned FIFO child exceeded its independent lifetime")
			}
			return "child lifetime expired"
		}
		if !wait(7*time.Second, func() bool { return ctx.Err() != nil || poll() }) {
			t.Fatal("helper completion timed out")
		}
		if ctx.Err() != nil {
			return "interrupted"
		}
		// Check BEFORE outer cleanup: the helper itself must have reaped its child.
		if !childGone() {
			t.Error("helper exited without reaping its blocked FIFO child")
		}
		return "helper exited"
	}()
	if !joined || syscall.Kill(parentPID, 0) != syscall.ESRCH || !childGone() {
		t.Fatal("owned process survived cleanup")
	}
	if reason == "interrupted" && action != "owner-cancel" && action != "owner-INT" && action != "owner-TERM" {
		t.Fatal("outer FIFO fixture interrupted after cleanup")
	}
	raw, err := os.ReadFile(log.Name())
	if err != nil {
		t.Fatal(err)
	}
	want := "FIFO parent interrupted"
	if action == "assertion" {
		want = "controlled parent assertion"
	}
	if action == "deadline" {
		want = "FIFO parent completion timed out"
	}
	statusOK := status.ExitStatus() == 1
	if action == "expiry" {
		statusOK = status.Signaled() && status.Signal() == syscall.SIGKILL
		want = ""
	}
	if !statusOK || !strings.Contains(string(raw), want) {
		t.Fatalf("parent cancellation path: status=%v output=%s", status, raw)
	}
	t.Logf("%s: parent PID=%d reaped, blocked child PID=%d gone, reason=%s", action, parentPID, childPID, reason)
}
