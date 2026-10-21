package main

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedWalkCancelContext cancels once the affected source walker has
// visited ten entries, and counts the walker's later context checks, so a
// test proves the walk stops promptly instead of running to completion
// (AFP-V0-151).
type affectedWalkCancelContext struct {
	context.Context
	cancel   context.CancelFunc
	deadline bool
	visits   atomic.Int32
	after    atomic.Int32
}

func (c *affectedWalkCancelContext) Err() error {
	pcs := make([]uintptr, 128)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "liveverify/affected.walkSourceFilesContext.func") {
			if c.Context.Err() != nil {
				c.after.Add(1)
			} else if c.visits.Add(1) == 10 {
				c.cancel()
			}
			break
		}
		if !more {
			break
		}
	}
	return c.Context.Err()
}

// cancelAffectedGraphWalk wraps the context the plan path hands
// affected.Build, recording whether the caller's context carried a deadline.
func cancelAffectedGraphWalk(t *testing.T) *atomic.Pointer[affectedWalkCancelContext] {
	t.Helper()
	var injected atomic.Pointer[affectedWalkCancelContext]
	t.Cleanup(func() { affectedGraphContext = func(ctx context.Context) context.Context { return ctx } })
	affectedGraphContext = func(parent context.Context) context.Context {
		_, deadline := parent.Deadline()
		ctx, cancel := context.WithCancel(parent)
		t.Cleanup(cancel)
		wrapped := &affectedWalkCancelContext{Context: ctx, cancel: cancel, deadline: deadline}
		injected.Store(wrapped)
		return wrapped
	}
	return &injected
}

func assertAffectedWalkStopped(t *testing.T, ctx *affectedWalkCancelContext) {
	t.Helper()
	if ctx == nil || ctx.visits.Load() < 10 {
		t.Fatal("cancellation never reached the graph walk")
	}
	// The overcap fixture holds more than 4100 files; a walk that ignored the
	// context would keep checking it thousands of times after cancellation.
	if after := ctx.after.Load(); after > 2 {
		t.Fatalf("walk kept running after cancellation: %d further context checks", after)
	}
}

// AFP-V0-151 (V1-0875): a cancelled standalone `corvint affected` over a large
// worktree stops its walk and refuses with a typed code and no plan document.
func TestAFPV0151StandaloneAffectedCancellationRefuses(t *testing.T) {
	root, _ := guidanceOverCapFixture(t)
	injected := cancelAffectedGraphWalk(t)
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "affected"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported-affected-graph") || !strings.Contains(stderr.String(), context.Canceled.Error()) {
		t.Fatalf("cancelled affected did not refuse: exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertAffectedWalkStopped(t, injected.Load())
}

// AFP-V0-151 (V1-0875): `review --base` over a capped guidance inventory plans
// through the standalone affected path (RGV-V0-013) under the review's own
// deadline; cancelling that fallback refuses with the same typed code and
// yields no review receipt.
func TestAFPV0151ReviewFallbackCancellationRefuses(t *testing.T) {
	root, base := guidanceOverCapFixture(t)
	injected := cancelAffectedGraphWalk(t)
	raw, err := compileGuidance(context.Background(), guidanceInvocation{Root: root, Command: "review", Base: base, MaxRefs: 32}, nil)
	var kernelError *gokernel.Error
	if err == nil || raw != nil || !errors.As(err, &kernelError) || kernelError.Code != "unsupported-affected-graph" || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("cancelled review fallback did not refuse: err=%v receipt=%s", err, raw)
	}
	assertAffectedWalkStopped(t, injected.Load())
	if !injected.Load().deadline {
		t.Fatal("the review fallback did not hand its deadline to the graph build")
	}
}

// AFP-V0-150 (V1-1087): HEAD is read again after the plan path's last source
// observation, so a same-tree commit that lands after the graph, advice and
// selection reads is refused as drift rather than bound to the old revision.
func TestAFPV0150PlanHeadDriftAfterSourcesRefuses(t *testing.T) {
	root := affectedFixtureRepository(t)
	t.Cleanup(func() { affectedPlanSourcesObserved = func() {} })
	affectedPlanSourcesObserved = func() { affectedGit(t, root, "commit", "-q", "--allow-empty", "-m", "same tree, new HEAD") }
	receipt, stdout, stderr, code := runAffectedCLI(t, root)
	if code != 2 || len(stdout) != 0 || receipt != nil || !strings.Contains(stderr, "unsupported-affected-drift") || !strings.Contains(stderr, "HEAD changed") {
		t.Fatalf("HEAD drift after the last source observation was accepted: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	affectedPlanSourcesObserved = func() {}
	if _, _, stderr, code := runAffectedCLI(t, root); code != 0 {
		t.Fatalf("control plan refused: exit=%d stderr=%s", code, stderr)
	}
}
