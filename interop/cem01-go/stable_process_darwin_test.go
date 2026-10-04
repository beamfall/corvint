//go:build darwin

package main

// Lifecycle tests for the Darwin process-group owner and the OQ-12 two-step
// retirement rule. They need no packet; each uses a fresh minimal repository.

import (
	"bytes"
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

func TestStableLifecycleKeeperProtocolError(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
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
