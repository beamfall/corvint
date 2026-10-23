//go:build linux

package procgroup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// PGO-V0-022: on Linux an exited leader held unreaped keeps signal 0 to its
// group succeeding, so before V1-2399 a group that obeyed SIGTERM waited out
// the whole TerminateGrace. The /proc held-group proof now ends the grace as
// soon as the group is zombie-only, while a member that ignores SIGTERM keeps
// the group live until the grace ends and is still SIGKILLed.
func TestPGOV0022LinuxTermObeyingGroupEndsGraceEarly(t *testing.T) {
	t.Run("obeys TERM", func(t *testing.T) {
		fixture := t.TempDir()
		ready := filepath.Join(fixture, "ready")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			waitForProcessFile(t, ready)
			cancel()
		}()
		var stopped time.Time
		observation := Run(ctx, Spec{
			Argv: []string{"/bin/sh", "-c", `sleep 30 & echo >"$1/ready"; wait`, "sh", fixture},
			Dir:  fixture, Env: []string{"PATH=/usr/bin:/bin"}, Timeout: time.Minute, ShutdownTimeout: 20 * time.Second, TerminateGrace: 5 * time.Second,
			BeforeStop: func(context.Context, int) error { stopped = time.Now(); return nil },
		})
		elapsed := time.Since(stopped)
		if !observation.Cancelled || observation.StopSignal != "SIGTERM" || !observation.OwnedProcessGroupCleanup {
			t.Fatalf("TERM-obeying group stop = %+v", observation)
		}
		if elapsed >= time.Second {
			t.Fatalf("TERM-obeying group with a 5 s grace stopped after %s, want under 1 s", elapsed)
		}
	})
	t.Run("ignores TERM", func(t *testing.T) {
		fixture := t.TempDir()
		ready := filepath.Join(fixture, "ready")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			waitForProcessFile(t, ready)
			cancel()
		}()
		const grace = 500 * time.Millisecond
		var stopped time.Time
		observation := Run(ctx, Spec{
			Argv: []string{"/bin/sh", "-c", `/bin/sh -c 'trap "" TERM; echo $$ >"$1/child"; echo >"$1/ready"; while :; do sleep 0.05; done' sh "$1" & wait`, "sh", fixture},
			Dir:  fixture, Env: []string{"PATH=/usr/bin:/bin"}, Timeout: time.Minute, ShutdownTimeout: 20 * time.Second, TerminateGrace: grace,
			BeforeStop: func(context.Context, int) error { stopped = time.Now(); return nil },
		})
		elapsed := time.Since(stopped)
		if !observation.Cancelled || !observation.OwnedProcessGroupCleanup {
			t.Fatalf("TERM-ignoring group stop = %+v", observation)
		}
		if elapsed < grace {
			t.Fatalf("TERM-ignoring member let the grace end after %s, before %s", elapsed, grace)
		}
		raw, err := os.ReadFile(filepath.Join(fixture, "child"))
		if err != nil {
			t.Fatal(err)
		}
		child, err := strconv.Atoi(string(bytes.TrimSpace(raw)))
		if err != nil {
			t.Fatal(err)
		}
		// A reaping init removes the killed member; a non-reaping one leaves
		// it a zombie. Either way it no longer runs.
		if stat, err := os.ReadFile("/proc/" + strconv.Itoa(child) + "/stat"); err == nil {
			if end := bytes.LastIndexByte(stat, ')'); end < 0 || end+2 >= len(stat) || stat[end+2] != 'Z' {
				t.Fatalf("TERM-ignoring member %d still runs after the grace: %s", child, stat)
			}
		}
	})
}

// PGO-V0-022: the grace probe proves quiet from the held-group proof while
// signal 0 still reaches the group; graceHeldQuiet backs its proofs off only
// once it runs them; without a proof or with a failing one the grace waits
// until its deadline.
func TestPGOV0022GraceProbeUsesHeldGroupProof(t *testing.T) {
	reachable := func(int, syscall.Signal) error { return nil }
	var proofs atomic.Int64
	quietOnThird := func(int) bool { return proofs.Add(1) >= 3 }
	started := time.Now()
	if !processGroupQuiescentBy(1, started.Add(5*time.Second), reachable, quietOnThird) {
		t.Fatal("held-group proof did not end the grace")
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("grace ended after %s, want under 1 s", elapsed)
	}
	var leaderHeld atomic.Bool
	proofs.Store(0)
	never := graceHeldQuiet(&leaderHeld, func(int) (bool, error) { proofs.Add(1); return false, nil })
	// Polls while the leader still runs neither prove nor widen the backoff.
	if processGroupQuiescentBy(1, time.Now().Add(100*time.Millisecond), reachable, never) || proofs.Load() != 0 {
		t.Fatalf("proof ran before the leader's exit: %d", proofs.Load())
	}
	leaderHeld.Store(true)
	if never(1) || proofs.Load() != 1 {
		t.Fatalf("first proof after the exit did not run at once: %d", proofs.Load())
	}
	if processGroupQuiescentBy(1, time.Now().Add(300*time.Millisecond), reachable, never) {
		t.Fatal("an unproved group read quiet")
	}
	// 300 ms of 5 ms polls is about 60 probes; backed-off proofs are far fewer.
	if n := proofs.Load(); n < 3 || n > 20 {
		t.Fatalf("%d held-group proofs in 300 ms, want a backed-off few", n)
	}
	if processGroupQuiescentBy(1, time.Now().Add(50*time.Millisecond), reachable, nil) {
		t.Fatal("a reachable group read quiet without a proof")
	}
}

// PGO-V0-022: the held-group proof is never consulted before run has
// observed the leader's exit with the leader still unreaped, and a proof
// failure is not quiescence.
func TestPGOV0022HeldProofWaitsForObservedExit(t *testing.T) {
	var leaderHeld atomic.Bool
	var calls atomic.Int64
	proof := func(int) (bool, error) { calls.Add(1); return true, nil }
	heldQuiet := graceHeldQuiet(&leaderHeld, proof)
	if heldQuiet == nil {
		t.Fatal("Linux grace has no held-group proof")
	}
	if heldQuiet(1) || calls.Load() != 0 {
		t.Fatalf("proof consulted before the leader's exit was observed: %d calls", calls.Load())
	}
	leaderHeld.Store(true)
	if !heldQuiet(1) || calls.Load() != 1 {
		t.Fatal("held leader's quiet proof ignored")
	}
	failing := graceHeldQuiet(&leaderHeld, func(int) (bool, error) { return true, errors.New("proof unavailable") })
	if failing(1) {
		t.Fatal("a failed proof read quiet")
	}
}
