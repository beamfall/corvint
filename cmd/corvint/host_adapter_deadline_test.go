package main

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// deadlineAdapterContext is an adapter context whose dogfood event expires after limit while read
// runs, with the host load average replaced by load.
func deadlineAdapterContext(t *testing.T, limit time.Duration, load hostLoadFunc, read func(context.Context)) (context.Context, string, func()) {
	t.Helper()
	root := queryCLIRepository(t)
	release := make(chan struct{})
	ctx := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root})
	ctx = context.WithValue(ctx, dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return limit })
	ctx = context.WithValue(ctx, dogfoodHostLoadKey{}, load)
	ctx = context.WithValue(ctx, dogfoodEventReadKey{}, func(ctx context.Context, _ options, _ map[string]any) (map[string]any, error) {
		read(ctx)
		<-release
		return nil, errors.New("released")
	})
	return ctx, root, func() { close(release) }
}

func deadlineContextText(t *testing.T, output map[string]any) string {
	t.Helper()
	hook, _ := output["hookSpecificOutput"].(map[string]any)
	text, _ := hook["additionalContext"].(string)
	if adapterDegradationReason(output) != claudeDeadlineReason {
		t.Fatalf("deadline output lost its ledger reason: %+v", output)
	}
	return text
}

const deadlineTestRevision = "0123456789abcdef0123456789abcdef01234567"

// AHI-800 (V1-1496): a fast status on a loaded host names host load, the phase that ran out of
// time and the status measurement, and does not name a large worktree.
func TestAHI800DeadlineNamesLoadDominatedCause(t *testing.T) {
	t.Parallel()
	loaded := func() (float64, bool) { return float64(4 * runtime.NumCPU()), true }
	ctx, _, release := deadlineAdapterContext(t, 100*time.Millisecond, loaded, func(ctx context.Context) {
		probe := dogfoodEventProbeOf(ctx)
		probe.observed(12, deadlineTestRevision)
		probe.enter("context-packet")
	})
	defer release()
	text := deadlineContextText(t, runClaudeAdapter(ctx, "user-prompt", map[string]any{"session_id": "s", "prompt": "inspect requirement"}))
	for _, want := range []string{"Phase that ran out of time: context-packet, after ", " ms of the 100 ms limit.", "with 12 changed paths.", "Host load: 1-minute load average ", "\nDominant cause: host load.\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("load-dominated notice lacks %q: %q", want, text)
		}
	}
	if strings.Contains(text, "large worktree") {
		t.Fatalf("fast status named a large worktree: %q", text)
	}
}

// AHI-800 (V1-1496): a status read that used most of the limit on a quiet host names a large
// worktree with its path count, and not host load.
func TestAHI800DeadlineNamesDirtySetDominatedCause(t *testing.T) {
	t.Parallel()
	quiet := func() (float64, bool) { return 0.05, true }
	ctx, _, release := deadlineAdapterContext(t, 600*time.Millisecond, quiet, func(ctx context.Context) {
		time.Sleep(350 * time.Millisecond) // the stand-in for a slow git status over a large dirty set
		dogfoodEventProbeOf(ctx).observed(48200, deadlineTestRevision)
	})
	defer release()
	text := deadlineContextText(t, runClaudeAdapter(ctx, "user-prompt", map[string]any{"session_id": "s", "prompt": "inspect requirement"}))
	for _, want := range []string{"Phase that ran out of time: repository-observation, after ", "with 48200 changed paths.", "\nDominant cause: large worktree.\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("dirty-set-dominated notice lacks %q: %q", want, text)
		}
	}
	if strings.Contains(text, "host load.") {
		t.Fatalf("quiet host named host load: %q", text)
	}
}

// AHI-800: with no finished status and no load average, the notice claims no cause.
func TestAHI800DeadlineWithoutMeasurementsSaysNotMeasured(t *testing.T) {
	t.Parallel()
	unavailable := func() (float64, bool) { return 0, false }
	ctx, _, release := deadlineAdapterContext(t, 50*time.Millisecond, unavailable, func(context.Context) {})
	defer release()
	text := deadlineContextText(t, runClaudeAdapter(ctx, "user-prompt", map[string]any{"session_id": "s", "prompt": "inspect requirement"}))
	for _, want := range []string{"git status: did not finish before the limit, not measured.", "Host load: not measured on this OS.", "\nDominant cause: not measured.\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unmeasured notice lacks %q: %q", want, text)
		}
	}
}

// AHI-800: a load-average read that never returns costs the notice at most hostLoadMeasureBound,
// so the measurement cannot push the hook past its deadline.
func TestAHI800HostLoadMeasurementIsBounded(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	defer close(block)
	ctx := context.WithValue(context.Background(), dogfoodHostLoadKey{}, hostLoadFunc(func() (float64, bool) { <-block; return 99, true }))
	start := time.Now()
	text := deadlineMeasuredLines(ctx)
	if elapsed := time.Since(start); elapsed > hostLoadMeasureBound+250*time.Millisecond {
		t.Fatalf("blocked load read held the notice for %v", elapsed)
	}
	if !strings.Contains(text, "Host load: not measured") || !strings.Contains(text, "Dominant cause: not measured.") {
		t.Fatalf("blocked load read was reported as measured: %q", text)
	}
	if load, ok := hostLoadAverage(); runtime.GOOS == "linux" && (!ok || load < 0) {
		t.Fatalf("linux load average unreadable: %v %v", load, ok)
	}
}

func reducedPacketOf(t *testing.T, text string) map[string]any {
	t.Helper()
	_, after, found := strings.Cut(text, "Corvint reduced packet (degraded;")
	start := strings.Index(after, "{")
	end := strings.LastIndex(after, "}")
	if !found || start < 0 || end < start {
		t.Fatalf("no reduced packet: %q", text)
	}
	var packet map[string]any
	if err := json.Unmarshal([]byte(after[start:end+1]), &packet); err != nil {
		t.Fatalf("reduced packet is not JSON: %v: %q", err, after)
	}
	return packet
}

// AHI-801 (V1-3556): an expired SessionStart(compact) still emits a bounded rehydration packet,
// marked degraded, after the unchanged frame: the compaction disclosure and the typed degradation.
func TestAHI801CompactSessionStartDeadlineEmitsReducedPacket(t *testing.T) {
	t.Parallel()
	ctx, root, release := deadlineAdapterContext(t, 50*time.Millisecond, func() (float64, bool) { return 0, false }, func(context.Context) {})
	defer release()
	output := runClaudeAdapter(ctx, "session-start", map[string]any{"session_id": "s", "source": "compact"})
	text := deadlineContextText(t, output)
	if first, _, _ := strings.Cut(text, "\n"); first != "Corvint FALLBACK degraded: "+claudeDeadlineReason+"; coding continues" {
		t.Fatalf("frame line changed: %q", first)
	}
	if !strings.Contains(text, compactionDisclosure) || len(text) > adapterOutputLimit {
		t.Fatalf("compact rehydration lacks its disclosure or is unbounded: %q", text)
	}
	packet := reducedPacketOf(t, text)
	repository, _ := packet["repository"].(map[string]any)
	recovery, _ := json.Marshal(packet["recovery"])
	if packet["profile"] != reducedPacketProfile || packet["degraded"] != true || packet["degradation"] != "dogfood-event-deadline" ||
		packet["startSource"] != "compact" || packet["evidence"] != "NOT_PRODUCED" || repository["status"] != "NOT_OBSERVED" ||
		!strings.Contains(string(recovery), root) {
		t.Fatalf("compact reduced packet: %+v", packet)
	}
}

// AHI-801 (V1-3556): an expired UserPromptSubmit emits a reduced packet with its typed
// degradation reason and the revision the finished opening observation measured.
func TestAHI801UserPromptDeadlineEmitsReducedPacket(t *testing.T) {
	t.Parallel()
	ctx, _, release := deadlineAdapterContext(t, 100*time.Millisecond, func() (float64, bool) { return 0, false }, func(ctx context.Context) {
		dogfoodEventProbeOf(ctx).observed(7, deadlineTestRevision)
	})
	defer release()
	output := runClaudeAdapter(ctx, "user-prompt", map[string]any{"session_id": "s", "prompt": "inspect requirement"})
	packet := reducedPacketOf(t, deadlineContextText(t, output))
	repository, _ := packet["repository"].(map[string]any)
	if packet["event"] != "user-prompt" || packet["degradation"] != "dogfood-event-deadline" || packet["degraded"] != true ||
		repository["status"] != "OBSERVED" || repository["commitRevision"] != deadlineTestRevision || repository["changedPathCount"] != float64(7) {
		t.Fatalf("user-prompt reduced packet: %+v", packet)
	}
	if hook, _ := output["hookSpecificOutput"].(map[string]any); hook["hookEventName"] != "UserPromptSubmit" || output["systemMessage"] != nil {
		t.Fatalf("reduced packet changed channel: %+v", output)
	}
}
