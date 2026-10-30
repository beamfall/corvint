//go:build darwin || linux

package jstestprovider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// serverFixture is the pidfile shell-child pattern from
// internal/console/lifecycle_test.go's ownedFixture, adapted to stand in for
// the app server RunE2E owns: it backgrounds one long-lived "descendant"
// (standing in for a node http server or a browser process), records that
// descendant's pid, and answers SIGINT/SIGTERM the way a well-behaved
// server would - by cleaning up its own child on the way out.
func serverFixture(t *testing.T) (script, pidfile string) {
	t.Helper()
	dir := t.TempDir()
	pidfile = filepath.Join(dir, "descendant.pid")
	script = filepath.Join(dir, "server")
	body := fmt.Sprintf("#!/bin/sh\n/bin/sleep 60 &\nchild=$!\ntrap 'kill \"$child\" 2>/dev/null; wait \"$child\" 2>/dev/null; exit 0' INT TERM\necho $child > '%s'\nwait\n", pidfile)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidfile); err == nil {
			if pid, _ := strconv.Atoi(strings.TrimSpace(string(raw))); pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	return script, pidfile
}

func waitForPidfile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(path); err == nil {
			if pid, _ := strconv.Atoi(strings.TrimSpace(string(raw))); pid > 0 {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("descendant never started")
	return 0
}

func assertDescendantGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		out, err := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
		cancel()
		if err != nil && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		if strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned server descendant %d survived interruption", pid)
}

// TestServerDescendantCleanupOnCancel proves the same shape of guarantee
// RunE2E depends on: starting a long-lived "server" under procgroup, then
// cancelling its context (the way RunE2E does once the test command
// completes, or an operator interrupts the run), leaves no descendant
// process behind. This is the "descendant cleanup on cancel" unit test the
// slice calls for, using a synthetic fixture rather than a real npm/node
// server so it runs with no network dependency.
func TestServerDescendantCleanupOnCancel(t *testing.T) {
	script, pidfile := serverFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan procgroup.Observation, 1)
	go func() {
		done <- procgroup.Run(ctx, procgroup.Spec{Argv: []string{script}, Dir: filepath.Dir(script), Env: os.Environ(), Timeout: 5 * time.Second})
	}()

	pid := waitForPidfile(t, pidfile)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatal("descendant was not live before cancellation")
	}

	cancel()

	select {
	case obs := <-done:
		if !obs.Cancelled && !obs.OwnedProcessGroupCleanup {
			t.Fatalf("want cancellation or owned-process-group cleanup recorded, got %+v", obs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server leader did not join after cancellation")
	}

	assertDescendantGone(t, pid)
}

func TestIsStaleAppBuild(t *testing.T) {
	unknown := AppBuildIdentity{Unknown: true}
	a := AppBuildIdentity{Digest: "aaa"}
	b := AppBuildIdentity{Digest: "bbb"}
	if isStaleAppBuild(unknown, b) {
		t.Fatal("want no staleness verdict when the start identity is unknown")
	}
	if isStaleAppBuild(a, unknown) {
		t.Fatal("want no staleness verdict when the publish identity is unknown")
	}
	if isStaleAppBuild(a, a) {
		t.Fatal("want not stale when both digests match")
	}
	if !isStaleAppBuild(a, b) {
		t.Fatal("want stale when digests differ and both are known")
	}
}

// TestRunE2E_ServerDescendantsGoneAfterRun drives the production RunE2E path
// end to end rather than procgroup directly, which is what
// TestServerDescendantCleanupOnCancel above exercises. It is the checked-in
// proof for the slice's "interruption cleanup must prove no owned server or
// browser descendants survive" line on a run that completed normally: the
// receipt's serverDescendantsGone field (runner.go, from
// procgroup.Observation.OwnedProcessGroupCleanup) was previously asserted by
// no test in this repository, so a regression that left it false - or left a
// real descendant alive - would have surfaced only in the uncommitted live
// evidence run. The stand-in test command blocks until the owned server has
// actually recorded a live descendant, so the server is never torn down
// before it forked and the assertion cannot pass vacuously.
func TestRunE2E_ServerDescendantsGoneAfterRun(t *testing.T) {
	script, pidfile := serverFixture(t)
	report, err := filepath.Abs(filepath.Join("testdata", "playwright-mixed.json"))
	if err != nil {
		t.Fatal(err)
	}
	waitThenReport := fmt.Sprintf("while [ ! -s '%s' ]; do sleep 0.01; done; cat '%s'", pidfile, report)

	receipt, err := RunE2E(context.Background(), E2EConfig{
		Config:     Config{Dir: filepath.Dir(script), RunnerName: "playwright", Timeout: 20 * time.Second},
		ServerArgv: []string{script},
		TestArgv:   []string{"/bin/sh", "-c", waitThenReport},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receipt.Infrastructure != nil {
		t.Fatalf("want a real observation, got infrastructure failure %+v", receipt.Infrastructure)
	}
	if receipt.Cancelled {
		t.Fatalf("want Cancelled=false on a run that completed normally, got %+v", receipt)
	}
	if len(receipt.Tests) == 0 {
		t.Fatal("want the parsed per-test outcomes, got none")
	}
	if receipt.ServerDescendantsGone == nil {
		t.Fatal("want serverDescendantsGone recorded after an e2e run, got nil")
	}
	if !*receipt.ServerDescendantsGone {
		t.Fatal("want serverDescendantsGone=true after RunE2E tore the owned server down, got false")
	}

	assertDescendantGone(t, waitForPidfile(t, pidfile))
}

// TestRunE2E_ServerNotReadyRecordsTeardown drives a server that never answers
// its readiness URL. The receipt must still record the torn-down server's
// descendant cleanup and must not publish a known-looking empty app-build
// identity for a run that never reached publish.
//
// RunE2E fixes the readiness deadline when the wait starts and bounds each
// probe at one second (runner.go:waitReady), so no readiness endpoint can hold
// teardown until the fixture has forked. Under host load the limit can lapse
// first; the descendant assertion then has no pid to check. A pidfile present
// after RunE2E returns was written before teardown, because the returned
// receipt already proved the owned process group quiescent, so an attempt
// without one is retried with a longer limit instead of asserting vacuously.
func TestRunE2E_ServerNotReadyRecordsTeardown(t *testing.T) {
	for _, limit := range []time.Duration{500 * time.Millisecond, 2 * time.Second, 8 * time.Second} {
		script, pidfile := serverFixture(t)
		receipt, err := RunE2E(context.Background(), E2EConfig{
			Config:           Config{Dir: filepath.Dir(script), RunnerName: "playwright", Timeout: 20 * time.Second},
			ServerArgv:       []string{script},
			ServerReadyURL:   "http://127.0.0.1:1/",
			ServerReadyLimit: limit,
			TestArgv:         []string{"/bin/sh", "-c", "exit 0"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "server-not-ready" || receipt.Cancelled {
			t.Fatalf("want server-not-ready without cancellation, got %+v", receipt)
		}
		if receipt.ServerDescendantsGone == nil || !*receipt.ServerDescendantsGone {
			t.Fatalf("want serverDescendantsGone=true, got %v", receipt.ServerDescendantsGone)
		}
		if !receipt.AppBuildAtPublish.Unknown || receipt.AppBuildAtPublish.Reason == "" {
			t.Fatalf("want an explicit unknown appBuildAtPublish, got %+v", receipt.AppBuildAtPublish)
		}
		raw, _ := os.ReadFile(pidfile)
		if pid, _ := strconv.Atoi(strings.TrimSpace(string(raw))); pid > 0 {
			assertDescendantGone(t, pid)
			return
		}
		t.Logf("server fixture had not forked its descendant within the %v readiness limit; retrying", limit)
	}
	t.Fatal("server fixture never forked its descendant before the readiness limit lapsed")
}

// TestRunE2E_CancelledDuringReadinessWait cancels the run while RunE2E is
// still waiting for the server: an operator interruption must read as
// cancelled, not as a server that failed to become ready.
func TestRunE2E_CancelledDuringReadinessWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(200*time.Millisecond, cancel)
	receipt, err := RunE2E(ctx, E2EConfig{
		Config:           Config{Dir: t.TempDir(), RunnerName: "playwright", Timeout: 20 * time.Second},
		ServerArgv:       []string{"/bin/sleep", "60"},
		ServerReadyURL:   "http://127.0.0.1:1/",
		ServerReadyLimit: 10 * time.Second,
		TestArgv:         []string{"/bin/sh", "-c", "exit 0"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !receipt.Cancelled || receipt.Infrastructure == nil || receipt.Infrastructure.Reason != "cancelled" {
		t.Fatalf("want a cancelled receipt, got cancelled=%v infrastructure=%+v", receipt.Cancelled, receipt.Infrastructure)
	}
}
