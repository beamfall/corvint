// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// churnLoop reports only successful creates as creates; refusals are counted
// apart, so a pool that cannot write reports zero throughput.
func TestChurnLoopCountsOnlySuccessfulCreates(t *testing.T) {
	for _, tc := range []struct {
		name            string
		fail            func(n int) bool
		creates, failed int
	}{
		{"all refused", func(int) bool { return true }, 0, 6},
		{"odd refused", func(n int) bool { return n%2 == 1 }, 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			res := churnLoop(ctx, func(n int) error {
				if calls++; calls == 6 {
					cancel()
				}
				if tc.fail(n) {
					return errors.New("refused")
				}
				return nil
			})
			if res.Creates != tc.creates || res.Failed != tc.failed {
				t.Fatalf("got %+v, want creates %d failed %d", res, tc.creates, tc.failed)
			}
		})
	}
}

// A writer start that fails after others started stops and reaps those
// already running instead of abandoning them.
func TestStartWritersReapsStartedWritersWhenALaterStartFails(t *testing.T) {
	var started []*exec.Cmd
	t0 := time.Now()
	pool, err := startWriters(context.Background(), [][]string{{"/bin/sleep", "60"}, {"/nonexistent/tasks-scale-writer"}}, func(_ int, c *exec.Cmd) { started = append(started, c) })
	if err == nil || pool != nil {
		t.Fatalf("startWriters: pool %v, err %v; want a start error", pool, err)
	}
	if len(started) != 2 || started[0].ProcessState == nil {
		t.Fatalf("the started writer was not reaped")
	}
	if d := time.Since(t0); d > 20*time.Second {
		t.Fatalf("reaping took %s; the writer ran to completion", d)
	}
}

// stop cancels running writers and waits for them.
func TestWriterPoolStopCancelsRunningWriters(t *testing.T) {
	pool, err := startWriters(context.Background(), [][]string{{"/bin/sleep", "60"}, {"/bin/sleep", "60"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	pool.stop()
	if d := time.Since(t0); d > 20*time.Second {
		t.Fatalf("stop took %s", d)
	}
	for i, c := range pool.cmds {
		if c.ProcessState == nil || c.ProcessState.Success() {
			t.Fatalf("writer %d: state %v, want stopped", i, c.ProcessState)
		}
	}
}
