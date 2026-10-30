//go:build darwin && (arm64 || amd64)

package groupreap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRunContainedRetiringKeepsUnprovenOrphan is the batch F composition
// witness (TRE-V0-030 with TRE-V0-034): after a normal leader exit the Retirer
// kills the token-bearing worker and browser, but cannot prove the helper,
// which was launched without the token after the last run-time sample. The
// structural pass must still retire the helper that the Retirer orphans.
func TestRunContainedRetiringKeepsUnprovenOrphan(t *testing.T) {
	r, err := NewRetirer()
	if err != nil {
		t.Fatal(err)
	}
	// No run-time sample observes the tree, as when the helper starts just
	// before the leader exits.
	prev := sampleProcesses
	sampleProcesses = func() (map[int]Process, error) { return map[int]Process{}, nil }
	t.Cleanup(func() { sampleProcesses = prev })
	f := newEscapeFixture(t)
	cmd := f.command(context.Background(), "leader", "exit")
	cmd.Env = append(cmd.Env, OwnerEnvironmentKey+"="+r.Token(), "CORVINT_ESCAPE_STRIP_OWNER=1")
	done := make(chan struct {
		c   Containment
		err error
	}, 1)
	go func() {
		c, err := RunContainedRetiring(cmd, r)
		done <- struct {
			c   Containment
			err error
		}{c, err}
	}()
	browser, helper := f.identity("browser"), f.identity("helper")
	var result struct {
		c   Containment
		err error
	}
	select {
	case result = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("RunContainedRetiring did not return")
	}
	retired := map[int]bool{}
	for _, p := range r.Result().Retired {
		retired[p.PID] = true
	}
	if !retired[browser.PID] || retired[helper.PID] {
		t.Fatalf("token record %+v, want browser %d and not helper %d", r.Result(), browser.PID, helper.PID)
	}
	if alive(t, browser) || alive(t, helper) {
		t.Fatalf("survivors: browser=%v helper=%v containment=%+v", alive(t, browser), alive(t, helper), result.c)
	}
	if !result.c.Complete() {
		t.Fatalf("containment incomplete: %+v", result.c)
	}
}

// A pre-retirement freeze that fails before reaching the tokenless helper,
// followed by a structural retry that stops it and then fails too, must not
// leave the helper suspended once the Retirer kills its token-bearing parent
// (batch F re-review).
func TestRunContainedRetiringRetiresHelperStoppedByFailedFreezes(t *testing.T) {
	r, err := NewRetirer()
	if err != nil {
		t.Fatal(err)
	}
	prevSample, prevSnap := sampleProcesses, snapshotProcesses
	t.Cleanup(func() { sampleProcesses, snapshotProcesses = prevSample, prevSnap })
	sampleProcesses = func() (map[int]Process, error) { return map[int]Process{}, nil }
	// Post-exit table reads: 1 stops the browser, 2 fails the first freeze,
	// 3 stops the helper in the structural retry, 4 fails that retry.
	calls := 0
	snapshotProcesses = func() (map[int]Process, error) {
		if calls++; calls == 2 || calls == 4 {
			return nil, errors.New("injected")
		}
		return prevSnap()
	}
	f := newEscapeFixture(t)
	cmd := f.command(context.Background(), "leader", "exit-late")
	cmd.Env = append(cmd.Env, OwnerEnvironmentKey+"="+r.Token(), "CORVINT_ESCAPE_STRIP_OWNER=1")
	// Identities are read before the run so they do not consume table reads.
	ready := make(chan struct{})
	var browser, helper Process
	go func() {
		defer close(ready)
		for _, name := range []string{"browser", "helper"} {
			for {
				if b, err := os.ReadFile(filepath.Join(f.dir, name+".pid")); err == nil && len(b) > 0 {
					pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
					if p, ok := prevSnap(); ok == nil {
						if name == "browser" {
							browser = p[pid]
						} else {
							helper = p[pid]
						}
					}
					break
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()
	c, runErr := RunContainedRetiring(cmd, r)
	<-ready
	if browser.PID == 0 || helper.PID == 0 {
		t.Fatalf("fixture identities missing: browser %v helper %v (%v)", browser, helper, runErr)
	}
	f.cleanup(browser)
	f.cleanup(helper)
	if calls < 4 {
		t.Fatalf("injected failures not reached: %d table reads", calls)
	}
	if c.Complete() {
		t.Fatalf("failed freezes reported complete containment: %+v", c)
	}
	if alive(t, browser) || alive(t, helper) {
		t.Fatalf("survivors: browser=%v helper=%v containment=%+v", alive(t, browser), alive(t, helper), c)
	}
}
