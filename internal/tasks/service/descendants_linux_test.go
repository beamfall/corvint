//go:build linux

package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestLinuxHelperTreeFixture is the re-executed helper tree, not a test:
// leader starts a same-group child and an escaper; the escaper starts a
// new-session sleeper and exits, orphaning it.
func TestLinuxHelperTreeFixture(t *testing.T) {
	mode, out := os.Getenv("CORVINT_HELPER_TREE"), os.Getenv("CORVINT_HELPER_TREE_OUT")
	if mode == "" {
		t.Skip("fixture process only")
	}
	self := func(next string, setsid bool) int {
		c := exec.Command(os.Args[0], "-test.run=^TestLinuxHelperTreeFixture$")
		c.Env = append(os.Environ(), "CORVINT_HELPER_TREE="+next)
		if setsid {
			c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}
		if err := c.Start(); err != nil {
			os.Exit(3)
		}
		return c.Process.Pid
	}
	switch mode {
	case "leader", "leader-exit":
		group := self("sleep", false)
		self("escape", false)
		_ = os.WriteFile(out+".group", []byte(strconv.Itoa(group)), 0o600)
		if mode == "leader-exit" {
			waitFile(out + ".escaped")
			os.Exit(0)
		}
		time.Sleep(time.Hour)
	case "escape":
		pid := self("sleep", true)
		_ = os.WriteFile(out+".escaped", []byte(strconv.Itoa(pid)), 0o600)
		os.Exit(0)
	case "sleep":
		time.Sleep(time.Hour)
	}
	os.Exit(0)
}

func waitFile(path string) int {
	for i := 0; i < 2000; i++ {
		if raw, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return 0
}

// alive reports whether pid exists, zombie or not: Retire must have
// reaped every descendant.
func alive(pid int) bool {
	_, err := os.Stat("/proc/" + strconv.Itoa(pid))
	return err == nil
}

func helperTree(t *testing.T, mode string) (helperSpawner, helperProc, string) {
	t.Helper()
	sp, err := helperRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if err := sp.Quiet(); err != nil {
		t.Fatalf("test process already has children: %v", err)
	}
	out := filepath.Join(t.TempDir(), "tree")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLinuxHelperTreeFixture$")
	cmd.Env = append(os.Environ(), "CORVINT_HELPER_TREE="+mode, "CORVINT_HELPER_TREE_OUT="+out)
	p, err := sp.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	return sp, p, out
}

func TestSERVICE500_LinuxHelperRetiresEscapedDescendants(t *testing.T) {
	sp, p, out := helperTree(t, "leader")
	ident, err := p.Verify()
	if err != nil || ident == "" {
		t.Fatalf("verify: %q %v", ident, err)
	}
	group, escaped := waitFile(out+".group"), waitFile(out+".escaped")
	if group == 0 || escaped == 0 {
		t.Fatal("fixture tree did not start")
	}
	// The new-session sleeper left the helper's group and session and was
	// reparented to the wrapper, the child subreaper.
	deadline := time.Now().Add(10 * time.Second)
	for {
		ppid, pg, err := procParent(escaped)
		if err == nil && ppid == os.Getpid() && pg != p.PID() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("escaped descendant not reparented: ppid=%d pg=%d err=%v", ppid, pg, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := p.Retire(10 * time.Second); err != nil {
		t.Fatalf("retire: %v", err)
	}
	for _, pid := range []int{p.PID(), group, escaped} {
		if alive(pid) {
			t.Fatalf("descendant %d survived retirement", pid)
		}
	}
	if err := sp.Quiet(); err != nil {
		t.Fatalf("wrapper still has a child: %v", err)
	}
}

func TestSERVICE500_LinuxHelperExitLeavesNoOrphan(t *testing.T) {
	sp, p, out := helperTree(t, "leader-exit")
	if _, err := p.Verify(); err != nil {
		// The leader may already have exited; ownership is still proved by
		// retirement below.
		t.Logf("verify after early exit: %v", err)
	}
	escaped := waitFile(out + ".escaped")
	select {
	case <-p.Exited():
	case <-time.After(10 * time.Second):
		t.Fatal("leader did not exit")
	}
	if !alive(escaped) {
		t.Fatal("fixture orphan already gone")
	}
	exit, err := p.Retire(10 * time.Second)
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	if exit != "exited" {
		t.Logf("leader exit %s", exit)
	}
	if alive(escaped) {
		t.Fatal("an orphaned helper descendant survived")
	}
	if err := sp.Quiet(); err != nil {
		t.Fatalf("wrapper still has a child: %v", err)
	}
	if _, err := p.Verify(); err == nil {
		t.Fatal("a retired leader verified")
	}
}
