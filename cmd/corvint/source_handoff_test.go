package main

import (
	"context"
	"testing"
	"time"
)

var sourceViewFixture = []byte("# Example\r\n\r\n- `EX-V0-001`: Preserve café.\r\n  Including failures.\r\n\r\n- `EX-V0-002`: Next.\r\n")

// ESV-V0-003: the source view and each of its Git commands are bounded by the frozen 10 seconds
// unless a test lifts the bound (V1-1302). It runs on every
// platform, Windows included, so the production value is pinned there too (V1-1414).
func TestSourceDeadlineReplacesFrozenBound(t *testing.T) {
	t.Parallel()
	if got := sourceDeadline(context.Background()); got != 10*time.Second {
		t.Fatalf("frozen source bound = %s, want 10s", got)
	}
	if got := sourceDeadline(liftedSourceDeadline(context.Background())); got != 10*time.Minute {
		t.Fatalf("lifted source bound = %s, want 10m", got)
	}
}

// liftedSourceDeadline lifts the source view's frozen 10-second bound for tests that verify
// refusal semantics, not latency: on a loaded host the bound expired mid-view, through the
// scripted Git stand-in's shell wrapper or real Git alike, and turned the asserted outcome into an
// earlier command's refusal (blob-budget, stale-commit, stale-tree, missing-path; V1-1302,
// decision 0082). The ten minutes are a hang detector; the frozen bound stays pinned by
// TestSourceDeadlineReplacesFrozenBound.
func liftedSourceDeadline(ctx context.Context) context.Context {
	return context.WithValue(ctx, sourceDeadlineKey{}, 10*time.Minute)
}
