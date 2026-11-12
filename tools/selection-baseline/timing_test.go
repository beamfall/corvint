// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
	"time"
)

// ASC-V0-782: a replayed change's TIMING line names its steps in replay order,
// and the summary names the replayed count, workers, wall, each step's summed
// seconds with its share, and the slowest changes first.
func TestTimingLinesAttributeReplaySteps_ASCV0782(t *testing.T) {
	slow := &stepTimes{steps: map[string]time.Duration{"checkout": time.Second, "affected": 2 * time.Second, "selector": 3 * time.Second, "test": 4 * time.Second, "guard": 0}, total: 10 * time.Second}
	fast := &stepTimes{steps: map[string]time.Duration{"checkout": time.Second, "test": time.Second}, total: 2 * time.Second}
	if got, want := slow.String(), "total=10.0s checkout=1.0s affected=2.0s selector=3.0s test=4.0s guard=0.0s"; got != want {
		t.Fatalf("change line %q, want %q", got, want)
	}
	changes := []change{{ID: "reused"}, {ID: "fast"}, {ID: "slow"}}
	got := timingSummary(changes, []*stepTimes{nil, fast, slow}, 2, 7*time.Second)
	want := "selection-baseline: TIMING replayed 2 of 3 changes on 2 workers in 7.0s wall; step seconds summed over changes: checkout=2.0s (17%) affected=2.0s (17%) selector=3.0s (25%) test=5.0s (42%) guard=0.0s (0%); slowest: slow=10.0s fast=2.0s"
	if got != want {
		t.Fatalf("summary\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, "reused") {
		t.Fatal("a reused change has no timing")
	}
}

// ASC-V0-782: mark charges the elapsed time since start to its step and
// returns the next step's start, so consecutive steps never overlap.
func TestStepTimesMarkChainsSteps_ASCV0782(t *testing.T) {
	s := &stepTimes{}
	start := time.Now().Add(-50 * time.Millisecond)
	next := s.mark("checkout", start)
	if !next.After(start) || s.steps["checkout"] < 50*time.Millisecond || s.total != s.steps["checkout"] {
		t.Fatalf("mark: next=%v steps=%v total=%v", next, s.steps, s.total)
	}
	s.mark("affected", next)
	if s.total != s.steps["checkout"]+s.steps["affected"] {
		t.Fatalf("total %v is not the sum of %v", s.total, s.steps)
	}
}
