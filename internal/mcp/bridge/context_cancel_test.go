package bridge

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const cancelContextArguments = `{"task":"change widget Value","subject":"internal/widget/widget.go","limit":5}`

// TestContextCancellationNeverReturnsReady covers MCPV0-034 (V1-0485): a
// corvint.context request cancelled before Registry.Call returns yields the
// `cancelled` failure and never a READY receipt, whether the cancellation
// lands before the call (early), before the production loader on a cold
// repository (cold), or after the production compile has already produced its
// packet (late). A healthy request afterwards returns the same bound receipt
// as before, and nothing under the root is written.
func TestContextCancellationNeverReturnsReady(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native context is qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	registry, err := NewTaskReview(root)
	if err != nil {
		t.Fatal(err)
	}
	before := rootDigest(t, root)
	baseline := healthyContextCall(t, registry)
	production := registry.operations.context
	cases := map[string]func(context.Context, context.CancelFunc, string, contextInput) (*contextindex.Index, map[string]any, error){
		"early": nil,
		"cold": func(ctx context.Context, cancel context.CancelFunc, root string, input contextInput) (*contextindex.Index, map[string]any, error) {
			cancel()
			return production(ctx, root, input)
		},
		"late": func(ctx context.Context, cancel context.CancelFunc, root string, input contextInput) (*contextindex.Index, map[string]any, error) {
			index, packet, err := production(ctx, root, input)
			if err != nil || packet["state"] != "READY" {
				t.Errorf("late case: production compile did not finish: state=%v err=%v", packet["state"], err)
			}
			cancel()
			return index, packet, err
		},
	}
	for _, name := range []string{"early", "cold", "late"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			registry.operations.context = production
			if operation := cases[name]; operation != nil {
				registry.operations.context = func(ctx context.Context, root string, input contextInput) (*contextindex.Index, map[string]any, error) {
					return operation(ctx, cancel, root, input)
				}
			} else {
				cancel()
			}
			result, callErr := registry.Call(ctx, ToolContext, []byte(cancelContextArguments))
			if callErr == nil || callErr.Code != "cancelled" || result.State != "" || result.Receipt != nil {
				t.Fatalf("%s cancellation: state=%q err=%#v, want cancelled and no receipt", name, result.State, callErr)
			}
			assertContextWorkRetired(t)
		})
	}
	registry.operations.context = production
	if again := healthyContextCall(t, registry); again != baseline {
		t.Fatalf("healthy receipt changed after cancellations:\n%s\n%s", baseline, again)
	}
	if rootDigest(t, root) != before {
		t.Fatal("cancelled corvint.context requests wrote under the repository root")
	}
}

// TestContextTimedCancellationRetiresWork cancels real corvint.context calls
// at fractions of a measured uncancelled call, so cancellation lands in the
// cold build, the compile, or after it. Every call returns either a READY
// receipt (the call finished first) or `cancelled`; no started work survives
// the return; and a healthy call afterwards succeeds. The cancel-to-return
// latency is logged as measured evidence, not asserted beyond a hang bound.
func TestContextTimedCancellationRetiresWork(t *testing.T) {
	if runtimeUnsupported() {
		t.Skip("native context is qualified only on Darwin and Linux")
	}
	root := makeRepository(t)
	registry, err := NewTaskReview(root)
	if err != nil {
		t.Fatal(err)
	}
	before := rootDigest(t, root)
	started := time.Now()
	baseline := healthyContextCall(t, registry)
	uncancelled := time.Since(started)
	var slowest time.Duration
	for _, eighths := range []int{0, 1, 2, 3, 4, 6, 7} {
		delay := uncancelled * time.Duration(eighths) / 8
		ctx, cancel := context.WithCancel(context.Background())
		fired := make(chan time.Time, 1)
		timer := time.AfterFunc(delay, func() {
			cancel()
			fired <- time.Now()
		})
		result, callErr := registry.Call(ctx, ToolContext, []byte(cancelContextArguments))
		returned := time.Now()
		timer.Stop()
		cancel()
		switch {
		case callErr != nil && callErr.Code == "cancelled" && result.Receipt == nil:
			latency := returned.Sub(<-fired)
			slowest = max(slowest, latency)
			t.Logf("cancel at %d/8 of %s: cancelled, returned %s after cancellation", eighths, uncancelled, latency)
			if latency > 10*time.Second {
				t.Fatalf("cancellation took %s to retire", latency)
			}
		case callErr == nil && result.State == "READY":
			t.Logf("cancel at %d/8 of %s: completed first", eighths, uncancelled)
		default:
			t.Fatalf("cancel at %d/8: state=%q err=%#v", eighths, result.State, callErr)
		}
		assertContextWorkRetired(t)
	}
	t.Logf("slowest measured cancel-to-return: %s (uncancelled call %s)", slowest, uncancelled)
	if again := healthyContextCall(t, registry); again != baseline {
		t.Fatalf("healthy receipt changed after timed cancellations:\n%s\n%s", baseline, again)
	}
	if rootDigest(t, root) != before {
		t.Fatal("timed cancellations wrote under the repository root")
	}
}

func healthyContextCall(t *testing.T, registry *Registry) string {
	t.Helper()
	result, callErr := registry.Call(context.Background(), ToolContext, []byte(cancelContextArguments))
	if callErr != nil {
		t.Fatal(callErr)
	}
	assertObservedBinding(t, result)
	raw, err := result.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// assertContextWorkRetired fails if a goroutine running context-index code is
// still alive shortly after Registry.Call returned. A reader observed in its
// deferred exit gets a bounded grace period; a joined reader never needs it.
func assertContextWorkRetired(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		buffer := make([]byte, 1<<20)
		stacks := string(buffer[:runtime.Stack(buffer, true)])
		live := ""
		for _, stack := range strings.Split(stacks, "\n\n") {
			if strings.Contains(stack, "internal/contextindex.") && !strings.Contains(stack, "assertContextWorkRetired") {
				live = stack
				break
			}
		}
		if live == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("context-index work outlived the call:\n%s", live)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
