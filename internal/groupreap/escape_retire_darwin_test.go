//go:build darwin && (arm64 || amd64)

package groupreap

import (
	"context"
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
