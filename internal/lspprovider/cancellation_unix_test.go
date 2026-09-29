//go:build darwin || linux

package lspprovider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// EEP-V0-027: cancellation of the actual provider retires its spawned child,
// not only a mocked dialogue. The provider's temporary cache is removed too.
func TestContextLSPCancellationRetiresDescendants(t *testing.T) {
	t.Run("EEP-V0-027 cancellation cleanup", func(t *testing.T) {
		m := newModule(t)
		dir := t.TempDir()
		pidFile := filepath.Join(dir, "pid")
		cacheFile := filepath.Join(dir, "cache")
		executable := filepath.Join(dir, "gopls")
		body := fmt.Sprintf("#!/bin/sh\nsleep 60 &\nprintf '%%s' $! > '%s'\nprintf '%%s' \"$GOPLSCACHE\" > '%s'\nwait\n", pidFile, cacheFile)
		if err := os.WriteFile(executable, []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		request := m.request(t, executable, "a/a.go")
		done := make(chan Result, 1)
		go func() { done <- Expand(ctx, request) }()
		deadline := time.Now().Add(5 * time.Second)
		var pid int
		var cache string
		for time.Now().Before(deadline) {
			bytes, _ := os.ReadFile(pidFile)
			pid, _ = strconv.Atoi(string(bytes))
			b, _ := os.ReadFile(cacheFile)
			cache = string(b)
			if pid > 0 && cache != "" {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if pid == 0 || cache == "" {
			t.Fatal("provider child did not start")
		}
		cancel()
		select {
		case result := <-done:
			if result.Failure != "gopls cancelled" || result.Record != nil {
				t.Fatalf("%+v", result)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("provider did not cancel")
		}
		for time.Now().Before(deadline) && syscall.Kill(pid, 0) == nil {
			time.Sleep(10 * time.Millisecond)
		}
		if syscall.Kill(pid, 0) == nil {
			t.Fatal("provider descendant survived cancellation")
		}
		if _, err := os.Stat(cache); !os.IsNotExist(err) {
			t.Fatal("provider cache survived", err)
		}
	})
}

// EEP-V0-027: descendants cannot resolve executables relative to the repository.
func TestContextLSPEnvironmentDropsRelativePATH(t *testing.T) {
	t.Setenv("PATH", ".:relative:/usr/bin::/bin")
	env := strings.Join(environment("/tmp/private"), "\n")
	if !strings.Contains(env, "PATH=/usr/bin:/bin\n") || !strings.Contains(env, "GOTELEMETRY=off") || !strings.Contains(env, "GOPROXY=off") {
		t.Fatal(env)
	}
}
