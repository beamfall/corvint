package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// AHI-805 (V1-0402), AHI-806 (V1-0878): Read, Grep and ExitPlanMode PreToolUse calls are answered with permissionDecision defer and
// bounded evidence or a named degradation; no case ever emits deny, ask or a blocking member.
func TestAHI805PreToolReadAlwaysDefers(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	run := func(tool string, input map[string]any) (map[string]any, string) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{
			"session_id": "native-session", "cwd": root, "hook_event_name": "PreToolUse", "tool_name": tool, "tool_input": input,
		})
		if err != nil {
			t.Fatal(err)
		}
		var stdout bytes.Buffer
		ctx := adapterEnvContext(lifecycleDeadlineContext(), map[string]string{"CLAUDE_PROJECT_DIR": root})
		if status := runHostAdapter(ctx, []string{"claude-code", "pre-tool"}, bytes.NewReader(raw), &stdout); status != 0 {
			t.Fatalf("status=%d output=%s", status, &stdout)
		}
		if stdout.Len() > adapterOutputLimit+1 {
			t.Fatalf("output exceeds the adapter bound: %d bytes", stdout.Len())
		}
		var output map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			t.Fatalf("output=%s err=%v", &stdout, err)
		}
		for _, member := range []string{"decision", "reason", "continue", "stopReason"} {
			if _, ok := output[member]; ok {
				t.Fatalf("%s: pre-tool output carries %q: %s", tool, member, &stdout)
			}
		}
		hook := claudeHookOutput(t, output)
		if hook["hookEventName"] != "PreToolUse" || hook["permissionDecision"] != "defer" || strings.Contains(stdout.String(), `"deny"`) || strings.Contains(stdout.String(), `"ask"`) {
			t.Fatalf("%s: pre-tool answer is not a bare defer: %s", tool, &stdout)
		}
		return hook, stdout.String()
	}
	for _, spelled := range []string{"internal/parser/token.go", filepath.Join(root, "internal", "parser", "token.go")} {
		hook, text := run("Read", map[string]any{"file_path": spelled})
		contextText, _ := hook["additionalContext"].(string)
		if !strings.Contains(contextText, untrustedDataPrefix) || !strings.Contains(contextText, "internal/parser/token.go") {
			t.Fatalf("%s: Read target has no evidence: %s", spelled, text)
		}
	}
	if hook, text := run("Grep", map[string]any{"pattern": "ParseToken", "path": "internal/parser/token.go"}); hook["additionalContext"] == nil {
		t.Fatalf("Grep target has no evidence: %s", text)
	}
	// AHI-806 (V1-0878): a submitted plan gets its anchors' evidence and governance, never approval.
	plan := "1. Change internal/parser/token.go so ParseToken accepts tabs.\n2. Add a test."
	if hook, text := run("ExitPlanMode", map[string]any{"plan": plan}); !strings.Contains(text, "internal/parser/token.go") || !strings.Contains(hook["additionalContext"].(string), untrustedDataPrefix) {
		t.Fatalf("plan has no evidence: %s", text)
	}
	if hook, text := run("ExitPlanMode", map[string]any{"plan": "  "}); len(hook) != 2 {
		t.Fatalf("an empty plan must get the bare defer: %s", text)
	}
	// A missing file is a named path degradation, still deferred.
	if _, text := run("Read", map[string]any{"file_path": "internal/parser/absent.go"}); !strings.Contains(text, "defer") {
		t.Fatalf("missing target: %s", text)
	}
	// Another tool, an absent path and an outside path get the bare defer.
	for _, call := range []struct {
		tool  string
		input map[string]any
	}{{"Bash", map[string]any{"command": "ls"}}, {"Grep", map[string]any{"pattern": "x"}}, {"Read", map[string]any{"file_path": filepath.Join(t.TempDir(), "x.go")}}} {
		if hook, text := run(call.tool, call.input); len(hook) != 2 {
			t.Fatalf("%s: expected a bare defer: %s", call.tool, text)
		}
	}
}
