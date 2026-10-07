package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// AHI-045, AHI-046: the projection keeps actionable rows only, and a non-SessionStart event with
// nothing actionable projects nothing.
func TestAHI046HookContextProjectionSilenceRule(t *testing.T) {
	t.Parallel()
	baseline := []any{"frontier-authority-unavailable", "host-version-unknown"}
	current := []any{map[string]any{"path": "AGENTS.md", "state": "current"}}
	governance := []any{map[string]any{"path": "AGENTS.md", "authority": "project-instructions"}}
	unresolved := map[string]any{"anchors": float64(1), "state": "unresolved", "reason": "anchor-not-found"}
	quiet := map[string]any{
		"adapter": map[string]any{"host": "codex"}, "repository": map[string]any{"treeRevision": "abc"},
		"requestSha256": "r", "resultDigest": "d", "degradations": baseline,
		"policy":  map[string]any{"lifecycle": "inactive"},
		"context": map[string]any{"governance": governance, "declared_scope": current, "resolution": unresolved, "freshness": "clean", "coverage": map[string]any{"included_results": float64(3), "critical_missing": []any{}}},
	}
	for _, event := range []string{"user-prompt", "post-tool", "session-start"} {
		if projection := hookContextProjection(event, quiet, false); projection != nil {
			t.Fatalf("%s: governance, current scope, baseline degradations and an unresolved anchor triggered %v", event, projection)
		}
	}
	full := hookContextProjection("session-start", quiet, true)
	want := map[string]any{"profile": hookContextProfile, "event": "session-start", "governance": governance, "declared_scope": current, "degradations": baseline, "resolution": unresolved}
	if !reflect.DeepEqual(full, want) {
		t.Fatalf("main-thread SessionStart projection\n got %v\nwant %v", full, want)
	}

	triggers := map[string]func(map[string]any, map[string]any){
		"task_evidence": func(_, packet map[string]any) {
			packet["task_evidence"] = []any{map[string]any{"path": "a.go", "blob_hash": "b"}}
		},
		"declared_scope": func(_, packet map[string]any) {
			packet["declared_scope"] = []any{map[string]any{"path": "x", "state": "stale"}}
		},
		"omitted": func(_, packet map[string]any) {
			packet["coverage"] = map[string]any{"critical_missing": []any{"path:x"}}
		},
		"unavailable": func(_, packet map[string]any) {
			packet["coverage"] = map[string]any{"unavailable_selectors": []any{"path:x"}}
		},
		"degradations": func(result, _ map[string]any) {
			result["degradations"] = append(append([]any{}, baseline...), "prompt-over-query-bound")
		},
		"compaction": func(_, packet map[string]any) { packet["compaction"] = map[string]any{"revision": "abc"} },
	}
	for key, apply := range triggers {
		result := map[string]any{"degradations": baseline, "policy": map[string]any{"lifecycle": "active", "satisfied": false, "unmet": []any{"verify"}}}
		packet := map[string]any{"governance": governance, "resolution": unresolved, "freshness": "mixed-worktree"}
		result["context"] = packet
		apply(result, packet)
		projection := hookContextProjection("user-prompt", result, false)
		if projection == nil || projection[key] == nil {
			t.Fatalf("%s did not trigger the projection: %v", key, projection)
		}
		// Once emitted, the packet carries governance, the unresolved anchor, freshness and policy.
		if projection["governance"] == nil || projection["resolution"] == nil || projection["freshness"] != "mixed-worktree" || projection["policy"] == nil {
			t.Fatalf("%s projection lost a qualifying field: %v", key, projection)
		}
		if key != "degradations" && projection["degradations"] != nil {
			t.Fatalf("%s projection repeated baseline degradations: %v", key, projection)
		}
		raw, _ := json.Marshal(projection)
		for _, dropped := range []string{"adapter", "repository", "requestSha256", "resultDigest", "included_results"} {
			if strings.Contains(string(raw), dropped) {
				t.Fatalf("%s projection carries %s: %s", key, dropped, raw)
			}
		}
	}
}

// AHI-046: silence reaches the host as empty output for both adapters, and an operator's kernel
// opt-in still gets its own context.
func TestAHI046SilentProjectionRendersNothing(t *testing.T) {
	t.Parallel()
	result := map[string]any{"degradations": []any{"frontier-authority-unavailable"}, "context": map[string]any{}}
	for _, host := range []string{"claude-code", "codex"} {
		if output := renderAdapterResult(host, "UserPromptSubmit", "user-prompt", "/repo", map[string]any{"sessionIdSha256": strings.Repeat("a", 64)}, result, false); len(output) != 0 {
			t.Fatalf("%s rendered a silent projection: %v", host, output)
		}
	}
	if output := withHookContextSuffix(map[string]any{}, "UserPromptSubmit", "\nkernel block"); !reflect.DeepEqual(output, claudeContextOutput("UserPromptSubmit", "kernel block")) {
		t.Fatalf("kernel opt-in lost on a silent prompt: %v", output)
	}
	if output := withHookContextSuffix(map[string]any{}, "UserPromptSubmit", ""); len(output) != 0 {
		t.Fatalf("silent prompt without a kernel emitted %v", output)
	}
}

// AHI-047, V1-0939: the workflow argv belongs to a main-thread SessionStart only.
func TestAHI047GuidanceIsMainThreadSessionStartOnly(t *testing.T) {
	t.Parallel()
	if !claudeSubagent(map[string]any{"agent_id": "a1"}) || claudeSubagent(map[string]any{"agent_id": "  "}) || claudeSubagent(map[string]any{"agent_type": "general-purpose"}) {
		t.Fatal("subagent detection must key on a non-empty agent_id only")
	}
	if claudeSessionGuidance("/repo", "k", "session-start", false) != claudeGuidance("/repo", "k") {
		t.Fatal("main-thread SessionStart lost the workflow argv")
	}
	for _, event := range []string{"user-prompt", "post-tool", "stop"} {
		if guidance := claudeSessionGuidance("/repo", "k", event, false); guidance != "" {
			t.Fatalf("%s carries guidance: %q", event, guidance)
		}
	}
	if guidance := claudeSessionGuidance("/repo", "k", "session-start", true); guidance != "" {
		t.Fatalf("subagent SessionStart carries guidance: %q", guidance)
	}
}

// AHI-045, AHI-046 end to end through the Codex adapter: an anchorless prompt is silent, an
// anchored one injects the framed projection, and SessionStart always projects.
func TestAHI046CodexPromptSilenceAndProjection(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	ctx := lifecycleDeadlineContext()
	if output := runCodexAdapter(ctx, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "cwd": root, "prompt": "what about it"}); len(output) != 0 {
		t.Fatalf("anchorless codex prompt injected %v", output)
	}
	for _, payload := range []map[string]any{
		{"hook_event_name": "UserPromptSubmit", "session_id": "s", "cwd": root, "prompt": "Explain `AGENTS.md`"},
		{"hook_event_name": "SessionStart", "session_id": "s", "cwd": root, "source": "startup"},
	} {
		output := runCodexAdapter(ctx, payload)
		hook, _ := output["hookSpecificOutput"].(map[string]any)
		text, _ := hook["additionalContext"].(string)
		var projection map[string]any
		if err := json.Unmarshal(envelopedProjection(t, text), &projection); err != nil || projection["profile"] != hookContextProfile || projection["repository"] != nil || projection["adapter"] != nil {
			t.Fatalf("%s projection=%v err=%v", payload["hook_event_name"], projection, err)
		}
	}
}
