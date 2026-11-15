//go:build linux

// SPDX-License-Identifier: Apache-2.0

package main

import (
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

// GLS-V0-058, GLS-V0-059: a mutant whose test allocates without bound (as a
// loop mutant that never advances a lexer does, V1-5336) is stopped by the
// suite run's RSS watchdog: failingTests returns errRSSLimit naming the
// limit, which the qualifier records as an invalid mutant with that reason,
// the test binary is gone, and the caller and a sibling process survive.
func TestQualifyRunOverRSSLimitIsInvalid(t *testing.T) {
	sibling := exec.Command("sleep", "60")
	if err := sibling.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sibling.Process.Kill(); _ = sibling.Wait() })

	pidFile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("LINECOV_RSS_FIXTURE_PID", pidFile)
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n\ngo 1.22\n")
	write(t, root, "p/p.go", "package p\n")
	// The 4 GiB cap keeps a broken watchdog from exhausting the host: the
	// package then passes and the test fails.
	write(t, root, "p/p_test.go", `package p

import (
	"os"
	"strconv"
	"testing"
)

func TestHog(t *testing.T) {
	_ = os.WriteFile(os.Getenv("LINECOV_RSS_FIXTURE_PID"), []byte(strconv.Itoa(os.Getpid())), 0o600)
	var keep [][]byte
	for i := 0; i < 4<<10; i++ {
		b := make([]byte, 1<<20)
		for j := 0; j < len(b); j += 4096 {
			b[j] = 1
		}
		keep = append(keep, b)
	}
	t.Log(len(keep))
}
`)
	// 512 MiB leaves room for the go command, compiler and linker the run's
	// group also holds while it builds the fixture.
	const limit = 512 << 20
	start := time.Now()
	_, _, err := failingTests(root, "example.com/m", []string{"p"}, time.Minute, limit)
	if !errors.Is(err, errRSSLimit) || errors.Is(err, errQualifyInterrupted) {
		t.Fatalf("an over-limit run must end in errRSSLimit, got %v", err)
	}
	if !strings.Contains(err.Error(), "rss-limit-exceeded") || !strings.Contains(err.Error(), "512 MiB limit") || !strings.Contains(err.Error(), "running p") {
		t.Fatalf("the recorded reason must name the limit and the package: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Second {
		t.Fatalf("the watchdog took %s", elapsed)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the fixture test never started: %v", err)
	}
	pid, _ := strconv.Atoi(string(data))
	for deadline := time.Now().Add(10 * time.Second); syscall.Kill(pid, 0) == nil; {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("test binary %d survived its run's kill", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := sibling.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("a sibling of the run was killed: %v", err)
	}
}
