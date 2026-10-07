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
	t.Run("LCP-V0-010 LCP-V0-011 AHI-046 anchorless prompt keeps scope and uncertainty in the receipt and injects nothing", func(t *testing.T) {
		output := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "user-prompt", map[string]any{"session_id": "native-session-é", "prompt": "what about it"})
		if len(output) != 0 {
			t.Fatalf("anchorless prompt injected context: %v", output)
		}
		result, reason := invokeDogfoodEvent(lifecycleDeadlineContext(), root, "claude-code", "user-prompt", map[string]any{"sessionIdSha256": key, "task": "what about it"}, adapterOutputLimit)
		if reason != "" {
			t.Fatal(reason)
		}
		raw, _ := json.Marshal(result)
		if bytes.Contains(raw, []byte("what about it")) || !bytes.Contains(raw, []byte(`"explicit-task-anchor-required"`)) || !bytes.Contains(raw, []byte(`"declared_scope":[{`)) || !bytes.Contains(raw, []byte(`"coverage"`)) {
			t.Fatalf("engine receipt lost privacy/scope/uncertainty: %s", raw)
		}
		if projection := hookContextProjection("user-prompt", result, false); projection != nil {
			t.Fatalf("anchorless prompt projected %v", projection)
		}
	})
	t.Run("AHI-045 AHI-047 anchored prompt injects the projection without guidance", func(t *testing.T) {
		output := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "user-prompt", map[string]any{"session_id": "native-session-é", "prompt": "Explain `AGENTS.md`"})
		contextText, _ := claudeHookOutput(t, output)["additionalContext"].(string)
		if strings.Contains(contextText, "workflow argv") || !strings.HasPrefix(contextText, untrustedDataPrefix) {
			t.Fatalf("prompt context carries guidance or lacks the envelope: %s", contextText)
		}
		var projection map[string]any
		if err := json.Unmarshal(envelopedProjection(t, contextText), &projection); err != nil {
			t.Fatal(err)
		}
		evidence, _ := projection["task_evidence"].([]any)
		policy, _ := projection["policy"].(map[string]any)
		if projection["profile"] != hookContextProfile || len(evidence) == 0 || policy["lifecycle"] != "active" {
			t.Fatalf("anchored prompt projection: %v", projection)
		}
		for _, dropped := range []string{"adapter", "repository", "requestSha256", "resultDigest", "frontier", "completion", "context", "coverage", "degradations"} {
			if _, ok := projection[dropped]; ok {
				t.Fatalf("projection carries %s: %v", dropped, projection)
			}
		}
	})
	t.Run("AHI-003 AHI-012 AHI-047 IDX-SNAP-V0-012 native startup resume clear compact stay read only", func(t *testing.T) {
		for _, source := range []string{"startup", "resume", "clear", "compact"} {
			output := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "session-start", map[string]any{"session_id": "native-session-é", "source": source})
			hook := claudeHookOutput(t, output)
			contextText, _ := hook["additionalContext"].(string)
			if hook["hookEventName"] != "SessionStart" || !strings.Contains(contextText, `"--session-key","`+key+`"`) {
				t.Fatalf("main-thread %s start lacks the workflow argv: %v", source, output)
			}
			// A subagent SessionStart (agent_id present) takes the prompt rule: a clean tree with
			// nothing actionable injects nothing, guidance included.
			subagent := runClaudeAdapterTest(lifecycleDeadlineContext(), t, root, "session-start", map[string]any{"session_id": "native-session-é", "source": source, "agent_id": "a1b2c3", "agent_type": "general-purpose"})
			if len(subagent) != 0 {
				t.Fatalf("subagent %s start injected: %v", source, subagent)
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
