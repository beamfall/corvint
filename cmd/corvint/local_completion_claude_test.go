package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/localcompletion"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

func TestClaudeNativeDogfoodLifecycle(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	base := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	keyBytes := sha256.Sum256([]byte("corvint-local-completion-session/claude-code/0\x00native-session-é"))
	key := hex.EncodeToString(keyBytes[:])
	plan, err := json.Marshal(localcompletion.Plan{Base: base, Intents: []string{"AGENTS.md"}, Checks: []localcompletion.Check{{ID: "unrun", Argv: []string{"false"}, TimeoutSeconds: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localcompletion.Begin(context.Background(), root, key, plan); err != nil {
		t.Fatal(err)
	}
	before := repositoryBytesDigest(t, root)
	t.Run("LCP-V0-008 first and recursive Stop preserve unresolved enrollment", func(t *testing.T) {
		for _, active := range []bool{false, true, true} {
			output := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "stop", map[string]any{"session_id": "native-session-é", "stop_hook_active": active})
			if !active && output["decision"] != "block" {
				t.Fatalf("first Stop did not block: %v", output)
			}
			if active && (output["decision"] != nil || !strings.Contains(fmt.Sprint(output["systemMessage"]), "unresolved")) {
				t.Fatalf("recursive Stop did not release unresolved: %v", output)
			}
		}
	})
	t.Run("LCP-V0-002 native identity is isolated from explicit handoff", func(t *testing.T) {
		otherKey := localcompletion.HashSession("native-session-é")
		if key == otherKey {
			t.Fatal("host session namespaces collided")
		}
		other, err := localcompletion.Evaluate(context.Background(), root, otherKey)
		if err != nil || other.Satisfied || other.Lifecycle != "inactive" {
			t.Fatalf("other native key consumed enrollment: %+v %v", other, err)
		}
		if _, err := localcompletion.Begin(context.Background(), root, otherKey, plan); err == nil {
			t.Fatal("other native key replaced active owner")
		}
		otherRoot := queryCLIRepository(t)
		wrongRoot, err := localcompletion.Evaluate(context.Background(), otherRoot, key)
		if err != nil || wrongRoot.Satisfied || wrongRoot.Lifecycle != "inactive" {
			t.Fatalf("wrong root adopted enrollment: %+v %v", wrongRoot, err)
		}
		for _, host := range []string{"codex", "claude-code"} {
			args := dogfoodEventArguments("stop")
			args[1], args[3] = host, dogfoodHostVersions[host]
			var stdout, stderr bytes.Buffer
			status := runLocalCompletionEvent(lifecycleDeadlineContext(), root, args, strings.NewReader(`{"sessionIdSha256":"`+key+`"}`), &stdout, &stderr)
			if status != 0 || !bytes.Contains(stdout.Bytes(), []byte(`"decision":"block"`)) {
				t.Fatalf("explicit key/root not resumable from %s: %d %s %s", host, status, &stdout, &stderr)
			}
		}
	})
	t.Run("LCP-V0-010 LCP-V0-011 governed prompt retains declared scope and uncertainty", func(t *testing.T) {
		output := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "user-prompt", map[string]any{"session_id": "native-session-é", "prompt": "what about it"})
		contextText, _ := claudeHookOutput(t, output)["additionalContext"].(string)
		if strings.Contains(contextText, "what about it") || !strings.Contains(contextText, `"explicit-task-anchor-required"`) || !strings.Contains(contextText, `"declared_scope"`) || !strings.Contains(contextText, `"coverage"`) {
			t.Fatalf("native context lost privacy/scope/uncertainty: %s", contextText)
		}
		start := strings.Index(contextText, "{\"adapter\":")
		end := strings.LastIndex(contextText, "\nEND CORVINT REPOSITORY DATA")
		if start < 0 || end <= start {
			t.Fatalf("native receipt not preserved: %s", contextText)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(contextText[start:end]), &result); err != nil {
			t.Fatal(err)
		}
		encoded, err := dogfoodEventBytes(result, 8000)
		if err != nil || !bytes.Equal(bytes.TrimSuffix(encoded, []byte{'\n'}), []byte(contextText[start:end])) {
			t.Fatalf("adapter changed the sealed native receipt: %v", err)
		}
	})
	t.Run("AHI-003 AHI-012 IDX-SNAP-V0-012 native startup resume clear compact stay read only", func(t *testing.T) {
		for _, source := range []string{"startup", "resume", "clear", "compact"} {
			output := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "session-start", map[string]any{"session_id": "native-session-é", "source": source})
			if claudeHookOutput(t, output)["hookEventName"] != "SessionStart" {
				t.Fatalf("wrong native startup envelope: %v", output)
			}
		}
	})
	t.Run("AHI-014 minimum identity and boolean Stop reject before core", func(t *testing.T) {
		for _, payload := range []map[string]any{{}, {"session_id": 5}, {"session_id": "x", "stop_hook_active": 1}} {
			output := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "stop", payload)
			if output["decision"] != nil || !strings.Contains(fmt.Sprint(output["systemMessage"]), "degraded") {
				t.Fatalf("invalid identity indicated completion: %v", output)
			}
		}
	})
	t.Run("LCP-V0-009 AHI-009 host admission remains closed", func(t *testing.T) {
		for _, field := range []int{1, 3, 5, 7} {
			args := dogfoodEventArguments("stop")
			args[1] = "claude-code"
			args[field] = "unsupported"
			var stdout, stderr bytes.Buffer
			status := runLocalCompletionEvent(lifecycleDeadlineContext(), root, args, strings.NewReader(`{}`), &stdout, &stderr)
			if status != 2 || stdout.Len() != 0 || (!strings.Contains(stderr.String(), "unsupported-dogfood-event-host") && !strings.Contains(stderr.String(), "invalid-dogfood-event-arguments")) {
				t.Fatalf("unqualified tuple admitted: %v %d %s %s", args, status, &stdout, &stderr)
			}
		}
	})
	if after := repositoryBytesDigest(t, root); after != before {
		t.Fatal("native hook events changed repository/Git entries, bytes or modes")
	}
	state, err := localcompletion.Evaluate(context.Background(), root, key)
	if err != nil || state.Satisfied || state.Lifecycle != "active" {
		t.Fatalf("read-only hooks completed or lost enrollment: %+v %v", state, err)
	}
}

// lifecycleDeadlineContext lifts the event deadline for tests that verify
// lifecycle semantics, not host latency; the production deadline stays pinned by
// TestDogfoodEventDeadlineBelowDeclaredHostKill.
func lifecycleDeadlineContext() context.Context {
	return context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return 10 * time.Minute })
}

// adapterEnvContext supplies the adapter's process-boundary variables from env alone, so a
// test neither calls t.Setenv nor sees the process environment.
func adapterEnvContext(parent context.Context, env map[string]string) context.Context {
	return context.WithValue(parent, adapterEnvKey{}, runtimeenv.Lookup(func(name string) (string, bool) { value, present := env[name]; return value, present }))
}

func runClaudeAdapterTest(ctx context.Context, t *testing.T, root, event string, payload map[string]any) map[string]any {
	t.Helper()
	output := runClaudeAdapter(adapterEnvContext(ctx, map[string]string{"CLAUDE_PROJECT_DIR": root}), event, payload)
	raw, err := json.Marshal(output)
	if err != nil || len(raw)+1 > adapterOutputLimit {
		t.Fatalf("native adapter output: %v %d", err, len(raw)+1)
	}
	return output
}

func claudeHookOutput(t *testing.T, output map[string]any) map[string]any {
	t.Helper()
	hook, ok := output["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("native adapter output has no hookSpecificOutput: %v", output)
	}
	return hook
}
