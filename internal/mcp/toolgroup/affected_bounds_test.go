// SPDX-License-Identifier: Apache-2.0
package toolgroup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
)

// boundRoot is a bound repository-shaped root for the group.
func boundRoot(t *testing.T) (string, *rootbind.Binding) {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding, ok := rootbind.Bind(root)
	if !ok {
		t.Fatal("bind")
	}
	return root, binding
}

// MMR-V0-1118: at most one planner runs at a time per root; a second root is
// not held behind the first; a call waiting behind a run that is cancelled
// answers -32603 without ever starting the planner.
func TestMMRV01118AffectedSerializesPlannerPerRoot(t *testing.T) {
	root, binding := boundRoot(t)
	other, otherBinding := boundRoot(t)
	var running, peak, runs atomic.Int32
	entered := make(chan string, 8)
	release := make(chan struct{})
	runner := func(ctx context.Context, got, _ string, stdout, _ io.Writer) int {
		runs.Add(1)
		if got == root {
			now := running.Add(1)
			for {
				seen := peak.Load()
				if now <= seen || peak.CompareAndSwap(seen, now) {
					break
				}
			}
			defer running.Add(-1)
		}
		entered <- got
		select {
		case <-release:
		case <-ctx.Done():
			return 2
		}
		_, _ = io.WriteString(stdout, `{"profile":"affected-summary/1"}`)
		return 0
	}
	group := NewAffected(root, binding, runner)
	otherGroup := NewAffected(other, otherBinding, runner)
	ctx := context.Background()
	type outcome struct {
		result  map[string]any
		failure *protocol.RPCError
	}
	call := func(group Affected, ctx context.Context) <-chan outcome {
		done := make(chan outcome, 1)
		go func() {
			result, failure := group.Call(ctx, AffectedTool, map[string]any{})
			done <- outcome{result, failure}
		}()
		return done
	}
	awaitEntry := func(want string) {
		t.Helper()
		select {
		case got := <-entered:
			if got != want {
				t.Fatalf("planner entered for %q, want %q", got, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("no planner run for %q", want)
		}
	}
	first := call(group, ctx)
	awaitEntry(root)
	second := call(group, ctx)
	select {
	case got := <-entered:
		t.Fatalf("a second planner entered for %q while the first held the root", got)
	case <-time.After(200 * time.Millisecond):
	}
	otherDone := call(otherGroup, ctx)
	awaitEntry(other)
	waiting, cancel := context.WithCancel(ctx)
	third := call(group, waiting)
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case got := <-third:
		// DMF-V1-010: a cancelled call is the cancelled tool error, not -32603.
		if got.failure != nil || got.result["isError"] != true || got.result["structuredContent"].(map[string]any)["code"] != CancelledCode {
			t.Fatalf("cancelled waiter: %#v %#v", got.result, got.failure)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the cancelled waiter never answered")
	}
	if runs.Load() != 2 {
		t.Fatalf("%d planner runs before release, want 2 (one per root)", runs.Load())
	}
	close(release)
	for name, done := range map[string]<-chan outcome{"first": first, "other": otherDone, "second": second} {
		select {
		case got := <-done:
			if got.failure != nil || got.result["isError"] != false {
				t.Fatalf("%s: %#v %#v", name, got.result, got.failure)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s never answered", name)
		}
	}
	if name := <-entered; name != root {
		t.Fatalf("the released second run entered for %q", name)
	}
	if peak.Load() != 1 || runs.Load() != 3 {
		t.Fatalf("peak concurrent runs for the root %d (want 1), runs %d (want 3)", peak.Load(), runs.Load())
	}
}

// MMR-V0-1118: the planner's code and error are read from the last non-empty
// stderr line, so a DEGRADED notice before a refusal keeps the refusal's code,
// and every message, parsed or raw, is cut to affectedFailureLimit bytes.
func TestMMRV01118AffectedFailureReadsLastLineBounded(t *testing.T) {
	root, binding := boundRoot(t)
	long := strings.Repeat("x", affectedFailureLimit+500)
	degraded := "corvint: DEGRADED self-binary-stale: this binary was built from commit abc1234; rebuild it\n"
	cases := map[string]struct {
		stderr        string
		code, message string
	}{
		"degraded then refusal":  {degraded + `{"code":"invalid-arguments","error":"argument --base: unknown","ok":false}` + "\n", "invalid-arguments", "argument --base: unknown"},
		"degraded then raw":      {degraded + "panic: planner\n", "affected-failed", "panic: planner"},
		"trailing blank lines":   {`{"code":"unsupported-affected-drift","error":"refused","ok":false}` + "\n\n\n", "unsupported-affected-drift", "refused"},
		"codeless object":        {`{"error":"no code"}`, "affected-failed", "no code"},
		"long parsed message":    {`{"code":"invalid-arguments","error":"` + long + `"}` + "\n", "invalid-arguments", long[:affectedFailureLimit]},
		"long unlisted code":     {`{"code":"long","error":"` + long + `"}` + "\n", "affected-failed", ("long: " + long)[:affectedFailureLimit]},
		"long raw line":          {degraded + long + "\n", "affected-failed", long[:affectedFailureLimit]},
		"only a degraded line":   {degraded, "affected-failed", strings.TrimSpace(degraded)},
		"nothing":                {"", "affected-failed", "corvint affected --summary failed"},
		"whitespace only":        {" \n\t\n", "affected-failed", "corvint affected --summary failed"},
		"multibyte at the bound": {strings.Repeat("é", affectedFailureLimit/2+1), "affected-failed", strings.Repeat("é", affectedFailureLimit/2)},
	}
	for name, test := range cases {
		stderrText := test.stderr
		group := NewAffected(root, binding, func(_ context.Context, _, _ string, _, stderr io.Writer) int {
			_, _ = io.WriteString(stderr, stderrText)
			return 2
		})
		result, failure := group.Call(context.Background(), AffectedTool, map[string]any{})
		structured, _ := result["structuredContent"].(map[string]any)
		if failure != nil || result["isError"] != true || structured["code"] != test.code || structured["error"] != test.message {
			got, _ := structured["error"].(string)
			t.Fatalf("%s: code %v message %d bytes %q (want %q, %d bytes); failure %#v", name, structured["code"], len(got), got[:min(len(got), 80)], test.message[:min(len(test.message), 80)], len(test.message), failure)
		}
		if message, _ := structured["error"].(string); len(message) > affectedFailureLimit {
			t.Fatalf("%s: message %d bytes over the %d bound", name, len(message), affectedFailureLimit)
		}
	}
}

// MMR-V0-1118: a verdict whose result cannot fit one MCP message is the
// group's own tool error verdict-oversized naming the CLI, never the server's
// output-bound-exceeded; a verdict under the bound is answered whole.
func TestMMRV01118AffectedOversizedVerdictIsNamed(t *testing.T) {
	root, binding := boundRoot(t)
	verdictOf := func(unitBytes int) string {
		units := make([]string, 0, unitBytes/64)
		for len(units)*64 < unitBytes {
			units = append(units, fmt.Sprintf("example.com/fixture/pkg%06d.TestUnit%020d", len(units), len(units)))
		}
		raw, err := json.Marshal(map[string]any{"profile": "affected-summary/1", "tier0": units})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw) + "\n"
	}
	var verdict string
	var mu sync.Mutex
	group := NewAffected(root, binding, func(_ context.Context, _, _ string, stdout, _ io.Writer) int {
		mu.Lock()
		defer mu.Unlock()
		_, _ = io.WriteString(stdout, verdict)
		return 0
	})
	ctx := context.Background()
	mu.Lock()
	verdict = verdictOf(600 * 1024)
	mu.Unlock()
	result, failure := group.Call(ctx, AffectedTool, map[string]any{})
	structured, _ := result["structuredContent"].(map[string]any)
	message, _ := structured["error"].(string)
	if failure != nil || result["isError"] != true || structured["code"] != AffectedOversizedCode || structured["profile"] != AffectedErrorProfile ||
		!strings.Contains(message, "corvint affected --summary") || !strings.Contains(message, "MCP message bound") {
		t.Fatalf("oversized verdict: %#v %#v", result, failure)
	}
	if size, over := affectedOversized(result); over || size > 8*1024 {
		t.Fatalf("the refusal itself encodes to %d bytes", size)
	}
	mu.Lock()
	verdict = verdictOf(400 * 1024)
	mu.Unlock()
	result, failure = group.Call(ctx, AffectedTool, map[string]any{})
	structured, _ = result["structuredContent"].(map[string]any)
	if failure != nil || result["isError"] != false || structured["profile"] != "affected-summary/1" {
		t.Fatalf("verdict under the bound: isError %v failure %#v", result["isError"], failure)
	}
	if size, over := affectedOversized(result); over || size <= 700*1024 {
		t.Fatalf("the answered result encodes to %d bytes, over=%v", size, over)
	}
}
