//go:build unix

package store

import (
	"context"
	"testing"
	"time"
)

// PSR-V0-023: a sweep's fixed observation bounds scale with its
// --timeout-seconds budget between the fixed floor and a 5x cap.
func TestPSRV0023SweepBoundsScaleWithBudget(t *testing.T) {
	for _, tc := range []struct {
		budget time.Duration
		fixed  time.Duration
		want   time.Duration
	}{
		{0, 2 * time.Second, 2 * time.Second},
		{time.Second, 2 * time.Second, 2 * time.Second},
		{10 * time.Second, 2 * time.Second, 2 * time.Second},
		{15 * time.Second, 2 * time.Second, 3 * time.Second},
		{30 * time.Second, 2 * time.Second, 6 * time.Second},
		{30 * time.Second, time.Second, 3 * time.Second},
		{30 * time.Second, 250 * time.Millisecond, 750 * time.Millisecond},
		{1800 * time.Second, 2 * time.Second, 10 * time.Second},
		{1800 * time.Second, time.Second, 5 * time.Second},
	} {
		if got := sweepScaledBound(withSweepBudget(context.Background(), tc.budget), tc.fixed); got != tc.want {
			t.Errorf("budget %v fixed %v: got %v want %v", tc.budget, tc.fixed, got, tc.want)
		}
	}
	// The Git probe carries whole seconds and rounds a fractional bound up.
	for budget, want := range map[time.Duration]int64{0: 2, 14 * time.Second: 3, 15 * time.Second: 3, 30 * time.Second: 6, 1800 * time.Second: 10} {
		if got := sweepGitBoundSeconds(withSweepBudget(context.Background(), budget)).Int(); got != want {
			t.Errorf("git bound at budget %v: got %ds want %ds", budget, got, want)
		}
	}
	// Outside a sweep there is no budget: the fixed bound applies.
	if got := sweepScaledBound(context.Background(), time.Second); got != time.Second {
		t.Fatalf("no budget: %v", got)
	}
	// A detached cleanup proof keeps the budget but never a test lift.
	lifted := PSRTestLiftObservationBounds(withSweepBudget(context.Background(), 30*time.Second), time.Minute)
	if got := sweepObservationBound(lifted, time.Second); got != time.Minute {
		t.Fatalf("lift: %v", got)
	}
	if got := sweepObservationBound(sweepBudgetOnly(lifted), time.Second); got != 3*time.Second {
		t.Fatalf("cleanup proof: %v", got)
	}
}

// PSR-V0-023: the scaled bound is used by the real probe, and the cap still
// stops a probe that outlives it.
func TestPSRV0023ScaledProbeBoundEnforced(t *testing.T) {
	// 1.5s exceeds the fixed 1s probe bound but not its 5s scaled bound.
	if _, e := poolProbe(withSweepBudget(context.Background(), 50*time.Second), "/bin/sleep", "1.5"); e != nil {
		t.Fatalf("scaled bound not applied: %v", e)
	}
	if _, e := poolProbe(context.Background(), "/bin/sleep", "1.5"); e == nil {
		t.Fatal("fixed bound not applied outside a sweep")
	}
	// The largest budget still caps the probe at 5s.
	start := time.Now()
	if _, e := poolProbe(withSweepBudget(context.Background(), 1800*time.Second), "/bin/sleep", "30"); e == nil {
		t.Fatal("capped bound not enforced")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("capped bound overran: %v", elapsed)
	}
}
