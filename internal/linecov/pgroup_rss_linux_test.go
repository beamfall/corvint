//go:build linux

// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// rssHogEnv makes this test binary allocate and touch memory without bound
// (up to a 4 GiB safety cap, so a broken watchdog fails the test rather than
// the host), writing its pid to the file the variable names first.
const rssHogEnv = "LINECOV_RSS_HOG_PIDFILE"

func init() {
	pidFile := os.Getenv(rssHogEnv)
	if pidFile == "" {
		return
	}
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600)
	var keep [][]byte
	for i := 0; i < 4<<10; i++ {
		b := make([]byte, 1<<20)
		for j := 0; j < len(b); j += 4096 {
			b[j] = 1
		}
		keep = append(keep, b)
	}
	time.Sleep(time.Minute)
	runtime.KeepAlive(keep)
	os.Exit(0)
}

// GLS-V0-058: a descendant of the run that allocates without bound pushes
// the group's resident memory past the limit; the watchdog kills the whole
// group within seconds, the run returns *RSSLimitError, and a sibling of the
// run in the caller's group, like the caller, survives.
func TestRunIsolatedRSSKillsGroupOverLimit(t *testing.T) {
	if !RSSWatchdogEnforced() {
		t.Fatal("the RSS watchdog must be enforced on Linux")
	}
	sibling := exec.Command("sleep", "60")
	if err := sibling.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sibling.Process.Kill(); _ = sibling.Wait() })

	pidFile := filepath.Join(t.TempDir(), "pid")
	// The hog is a grandchild: sh is the group leader, so only a group-wide
	// sample sees the hog's memory.
	cmd := exec.CommandContext(context.Background(), "sh", "-c", `"$0" -test.run='^$'; echo unreachable`, os.Args[0])
	cmd.Env = append(os.Environ(), rssHogEnv+"="+pidFile)
	const limit = 256 << 20
	start := time.Now()
	err := RunIsolatedRSS(cmd, limit)
	elapsed := time.Since(start)
	var over *RSSLimitError
	if !errors.As(err, &over) {
		t.Fatalf("an over-limit run must return *RSSLimitError, got %v", err)
	}
	if over.Limit != limit || over.Observed <= limit {
		t.Fatalf("limit error does not name the bound and an over-limit size: %+v", over)
	}
	if elapsed > 20*time.Second {
		t.Fatalf("the watchdog took %s to stop the run", elapsed)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the hog never started: %v", err)
	}
	hog, _ := strconv.Atoi(string(data))
	for deadline := time.Now().Add(10 * time.Second); syscall.Kill(hog, 0) == nil; {
		if time.Now().After(deadline) {
			_ = syscall.Kill(hog, syscall.SIGKILL)
			t.Fatalf("the hog %d survived its group's kill", hog)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := sibling.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("a sibling outside the run's group was killed: %v", err)
	}
}

// GLS-V0-058: a run under the limit is untouched and returns its own result.
func TestRunIsolatedRSSUnderLimitKeepsResult(t *testing.T) {
	if err := RunIsolatedRSS(exec.CommandContext(context.Background(), "sh", "-c", "exit 0"), 256<<20); err != nil {
		t.Fatalf("an under-limit run failed: %v", err)
	}
	err := RunIsolatedRSS(exec.CommandContext(context.Background(), "sh", "-c", "exit 3"), 256<<20)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("an under-limit run must keep its exit status, got %v", err)
	}
	if rss := groupRSS(os.Getpid()); rss != 0 && syscall.Getpgrp() != os.Getpid() {
		t.Fatalf("a pid that leads no group sampled %d bytes", rss)
	}
}
