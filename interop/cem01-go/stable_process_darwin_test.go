//go:build darwin

package main

// Lifecycle tests for the Darwin process-group owner and the OQ-12 two-step
// retirement rule. They need no packet; each uses a fresh minimal repository.

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func lifecycleSession(t *testing.T, ctx context.Context, h *stableHooks) (*stableSession, string) {
	t.Helper()
	B := s0eTemp(t)
	s0eGit(t, "/", "-c", "init.defaultBranch=main", "init", "-q", filepath.Join(B, "r"))
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	return &stableSession{ctx: ctx, hooks: h, exe: exe, git: git, admin: filepath.Join(B, "r", ".git"), env: stableGitEnv(), oidLen: 40, charged: map[string]bool{}}, B
}

func lifecycleShim(t *testing.T, B, name, body string) string {
	t.Helper()
	p := filepath.Join(B, name)
	s0eWrite(t, p, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	return p
}

func formatTx() *stableTx {
	return &stableTx{Operation: "format-query", Phase: "canonical", input: map[string]any{}, op: stableOpFormat, limit: stableFormatLimit}
}

func pidGone(t *testing.T, pidFile string) {
	t.Helper()
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("descendant pid not recorded: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("descendant %d still present after transaction (kill 0: %v)", pid, err)
	}
}

func TestStableLifecycleFormatQuery(t *testing.T) {
	s, _ := lifecycleSession(t, context.Background(), nil)
	tx := formatTx()
	if _, e := s.transact(tx); e != nil {
		t.Fatalf("format query: %s", e.code)
	}
	if string(tx.out) != "sha1\n" || s.hold || len(s.held) != 0 {
		t.Fatalf("format query output %q hold %v", tx.out, s.hold)
	}
}

func TestStableLifecycleLingeringDescendantRetired(t *testing.T) {
	s, B := lifecycleSession(t, context.Background(), nil)
	pid := filepath.Join(B, "linger.pid")
	shim := lifecycleShim(t, B, "linger", "/bin/sleep 300 &\necho $! > "+pid+"\nexec "+s.git+` "$@"`)
	s.hooks = &stableHooks{gitPath: func(*stableTx) string { return shim }}
	tx := formatTx()
	if _, e := s.transact(tx); e != nil {
		t.Fatalf("format query: %s", e.code)
	}
	pidGone(t, pid)
	if s.hold {
		t.Fatal("unexpected hold")
	}
}

func TestStableLifecycleCancelRetiresDescendants(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, B := lifecycleSession(t, ctx, nil)
	pid := filepath.Join(B, "cancel.pid")
	shim := lifecycleShim(t, B, "block", "/bin/sleep 300 &\necho $! > "+pid+"\nexec /bin/sleep 300")
	s.hooks = &stableHooks{
		gitPath: func(*stableTx) string { return shim },
		tx: func(_ *stableTx, p string) {
			if p != "spawned" {
				return
			}
			for i := 0; i < 400; i++ {
				if b, err := os.ReadFile(pid); err == nil && len(bytes.TrimSpace(b)) > 0 {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			cancel()
		},
	}
	start := time.Now()
	_, e := s.transact(formatTx())
	if e == nil || e.code != "verification-timeout" {
		t.Fatalf("want verification-timeout, got %v", e)
	}
	if d := time.Since(start); d > stableRetireBound {
		t.Fatalf("cancellation took %v", d)
	}
	pidGone(t, pid)
	if s.hold {
		t.Fatal("unexpected hold")
	}
}

func TestStableLifecycleOpTimeout(t *testing.T) {
	s, B := lifecycleSession(t, context.Background(), nil)
	shim := lifecycleShim(t, B, "slow", "exec /bin/sleep 30")
	s.hooks = &stableHooks{gitPath: func(*stableTx) string { return shim }, opTimeout: func(*stableTx) time.Duration { return 100 * time.Millisecond }}
	if _, e := s.transact(formatTx()); e == nil || e.code != "git-timeout" {
		t.Fatalf("want git-timeout, got %v", e)
	}
}

func TestStableLifecycleOverflow(t *testing.T) {
	s, B := lifecycleSession(t, context.Background(), nil)
	shim := lifecycleShim(t, B, "big", "/usr/bin/head -c 100 /dev/zero")
	s.hooks = &stableHooks{gitPath: func(*stableTx) string { return shim }}
	if _, e := s.transact(formatTx()); e == nil || e.code != "unsupported-resource-limit" {
		t.Fatalf("want unsupported-resource-limit, got %v", e)
	}
}

func TestStableLifecycleLaunchFailure(t *testing.T) {
	s, B := lifecycleSession(t, context.Background(), nil)
	missing := filepath.Join(B, "missing", "git")
	s.hooks = &stableHooks{gitPath: func(*stableTx) string { return missing }}
	_, _, e := s.resolve("target", strings.Repeat("a", 40))
	if e == nil || e.code != "git-read-failed" {
		t.Fatalf("want git-read-failed, got %v", e)
	}
}

func stableFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Skipf("cannot inspect /dev/fd: %v", err)
	}
	return len(entries)
}

func TestStableLifecycleSetupFailureClosesOpenedEndpoints(t *testing.T) {
	s := &stableSession{
		ctx: context.Background(),
		hooks: &stableHooks{setupFail: func(step string) bool {
			return step == "socketpair:after"
		}},
		charged: map[string]bool{},
	}
	before := stableFDCount(t)
	for i := 0; i < 20; i++ {
		e := stableRun(s, formatTx())
		if e == nil || e.code != "unsupported-process-containment" {
			t.Fatalf("want unsupported-process-containment, got %v", e)
		}
	}
	after := stableFDCount(t)
	if after > before+2 {
		t.Fatalf("setup failure leaked descriptors: before=%d after=%d", before, after)
	}
}

func TestStableLifecycleHoldKeeperClosed(t *testing.T) {
	s, _ := lifecycleSession(t, context.Background(), &stableHooks{holdKeeper: true})
	if _, e := s.transact(formatTx()); e != nil {
		t.Fatalf("format query: %s", e.code)
	}
	if len(s.held) == 0 {
		t.Fatal("hold seam kept no keeper")
	}
	stableClose(s)
	if len(s.held) != 0 || s.hold {
		t.Fatalf("final close left %d held, hold %v", len(s.held), s.hold)
	}
}

func TestStableLifecycleCloseFailHolds(t *testing.T) {
	s, _ := lifecycleSession(t, context.Background(), &stableHooks{holdKeeper: true, closeFail: func() bool { return true }})
	if _, e := s.transact(formatTx()); e != nil {
		t.Fatalf("format query: %s", e.code)
	}
	stableClose(s)
	if !s.hold {
		t.Fatal("close failure did not hold")
	}
	for _, v := range s.held {
		if k, ok := v.(stableHeld); ok {
			_ = syscall.Kill(-k.pgid, syscall.SIGKILL)
			_, _ = k.proc.Wait()
			for _, f := range k.files {
				f.Close()
			}
		}
	}
}

func startGroup(t *testing.T, script string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestStableLifecycleRetireBoundHold(t *testing.T) {
	cmd := startGroup(t, "exec /bin/sleep 300")
	pgid := cmd.Process.Pid
	defer func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		_ = cmd.Wait()
	}()
	if stableRetire(cmd.Process, pgid, time.Now().Add(50*time.Millisecond)) {
		t.Fatal("a live group must HOLD at the bound")
	}
}

func TestStableLifecycleRetireTwoStep(t *testing.T) {
	cmd := startGroup(t, "/bin/sleep 300 & exec /bin/sleep 300")
	pgid := cmd.Process.Pid
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if !stableRetire(cmd.Process, pgid, time.Now().Add(stableRetireBound)) {
		t.Fatal("killed group did not retire")
	}
	if err := syscall.Kill(-pgid, 0); err != syscall.ESRCH {
		t.Fatalf("post-reap probe: %v", err)
	}
}

func TestStableLifecycleRetirePostReapBoundIsInsideOuterBound(t *testing.T) {
	cmd := startGroup(t, "exec /bin/sleep 300")
	pgid := cmd.Process.Pid
	defer func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		_ = cmd.Wait()
	}()
	start := time.Now()
	if stableRetire(cmd.Process, pgid, start.Add(75*time.Millisecond)) {
		t.Fatal("live group must not retire")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("retire ignored the caller bound: %v", d)
	}
}

// lifecycleSocketpair mirrors the owner: both ends close on exec, created
// under ForkLock so no concurrently started child inherits either.
func lifecycleSocketpair() ([2]int, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err == nil {
		syscall.CloseOnExec(fds[0])
		syscall.CloseOnExec(fds[1])
	}
	return fds, err
}

func TestStableLifecycleKeeperProtocolError(t *testing.T) {
	fds, err := lifecycleSocketpair()
	if err != nil {
		t.Fatal(err)
	}
	owner := os.NewFile(uintptr(fds[0]), "owner")
	keeper := os.NewFile(uintptr(fds[1]), "keeper")
	defer owner.Close()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, stableKeeperProtocol)
	cmd.ExtraFiles = []*os.File{keeper}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	keeper.Close()
	if _, err := owner.Write([]byte("GARBAGE!!!")); err != nil {
		t.Fatal(err)
	}
	_ = owner.SetReadDeadline(time.Now().Add(10 * time.Second))
	st := make([]byte, 12)
	n, _ := owner.Read(st)
	owner.Close()
	_ = cmd.Wait()
	if n != 12 || string(st[:4]) != "CEMS" || st[5] != stableStatusProtocolError {
		t.Fatalf("keeper status %q", st[:n])
	}
}

const lifecycleOrphanEnv = "CEM01_TEST_ORPHAN_OWNER"

func init() {
	if git := os.Getenv(lifecycleOrphanEnv); git != "" && len(os.Args) == 1 {
		lifecycleOrphanOwner(git)
	}
}

// lifecycleOrphanOwner is a re-executed owner that starts a keeper for git,
// leaks its own control end into a long-lived holder, prints the keeper,
// holder and child pids, and exits without closing anything, as an owner
// killed mid-transaction would. The holder keeps the keeper's fd 3 open, so
// only the owner's exit can retire the keeper.
func lifecycleOrphanOwner(git string) {
	fds, err := lifecycleSocketpair()
	if err != nil {
		os.Exit(3)
	}
	owner, keeper := os.NewFile(uintptr(fds[0]), "owner"), os.NewFile(uintptr(fds[1]), "keeper")
	exe, err := os.Executable()
	if err != nil {
		os.Exit(3)
	}
	k := exec.Command(exe, stableKeeperProtocol)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, lifecycleOrphanEnv+"=") {
			k.Env = append(k.Env, e)
		}
	}
	k.ExtraFiles = []*os.File{keeper}
	k.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	h := exec.Command("/bin/sleep", "300")
	h.ExtraFiles = []*os.File{owner}
	h.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if k.Start() != nil || h.Start() != nil {
		os.Exit(3)
	}
	if _, err := owner.Write(stableEncodeFrame(stableOpFormat, 40, []string{git, filepath.Dir(git)})); err != nil {
		os.Exit(3)
	}
	_ = owner.SetReadDeadline(time.Now().Add(10 * time.Second))
	st := make([]byte, 12)
	if n, _ := io.ReadFull(owner, st); n != 12 || string(st[:4]) != "CEMS" {
		os.Exit(3)
	}
	fmt.Printf("%d %d %d %d\n", k.Process.Pid, h.Process.Pid, st[5], binary.BigEndian.Uint32(st[8:]))
	os.Exit(0)
}

// lifecycleOrphan runs the orphan owner and returns the keeper pid, the
// reported status and its value; the holder is killed at cleanup.
func lifecycleOrphan(t *testing.T, git string) (int, byte, int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), lifecycleOrphanEnv+"="+git)
	out, err := cmd.Output()
	var keeper, holder, kind, value int
	if _, serr := fmt.Sscan(string(out), &keeper, &holder, &kind, &value); err != nil || serr != nil {
		t.Fatalf("orphan owner: %v %q", err, out)
	}
	t.Cleanup(func() { _ = syscall.Kill(holder, syscall.SIGKILL) })
	if syscall.Kill(holder, 0) != nil {
		t.Fatal("holder of the leaked control end is not running")
	}
	return keeper, byte(kind), value
}

// lifecycleGoneWithin waits for each pid to be absent, test-side only.
func lifecycleGoneWithin(t *testing.T, d time.Duration, pids ...int) {
	t.Helper()
	deadline := time.Now().Add(d)
	for _, pid := range pids {
		for syscall.Kill(pid, 0) != syscall.ESRCH {
			if time.Now().After(deadline) {
				for _, p := range pids {
					_ = syscall.Kill(p, syscall.SIGKILL)
				}
				t.Fatalf("pid %d outlived its owner", pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// A keeper parked after a failed launch retires when its owner exits even
// though a leaked copy of the owner's control end keeps fd 3 open.
func TestStableLifecycleKeeperOwnerExitWhileHolding(t *testing.T) {
	keeper, kind, _ := lifecycleOrphan(t, filepath.Join(s0eTemp(t), "missing-git"))
	if kind != stableStatusLaunchFailed {
		t.Fatalf("status %d, want launch failure", kind)
	}
	lifecycleGoneWithin(t, 10*time.Second, keeper)
}

// A keeper waiting on a running Git child retires its whole group, child
// included, when its owner exits.
func TestStableLifecycleKeeperOwnerExitWhileRunning(t *testing.T) {
	git := filepath.Join(s0eTemp(t), "git")
	if err := os.WriteFile(git, []byte("#!/bin/sh\nexec /bin/sleep 300\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	keeper, kind, child := lifecycleOrphan(t, git)
	if kind != stableStatusStarted || child <= 1 {
		t.Fatalf("status %d child %d, want started", kind, child)
	}
	lifecycleGoneWithin(t, 10*time.Second, keeper, child)
}
