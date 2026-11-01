package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// AHI-804 (V1-0401): a SubagentStart payload gets the user-prompt packet for its agent_prompt under
// hookEventName SubagentStart, or a named degradation, and never a member that could block.
func TestAHI804SubagentStartReturnsBoundedPacketWithoutBlocking(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	run := func(fields map[string]any) (map[string]any, string) {
		t.Helper()
		payload := map[string]any{
			"session_id": "native-session", "transcript_path": "/Users/dev/.claude/projects/p/native-session.jsonl",
			"cwd": root, "hook_event_name": "SubagentStart", "agent_id": "agent-7", "agent_type": "Explore",
		}
		for key, value := range fields {
			payload[key] = value
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var stdout bytes.Buffer
		ctx := adapterEnvContext(lifecycleDeadlineContext(), map[string]string{"CLAUDE_PROJECT_DIR": root})
		if status := runHostAdapter(ctx, []string{"claude-code", "subagent-start"}, bytes.NewReader(raw), &stdout); status != 0 {
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
				t.Fatalf("subagent-start output carries %q: %s", member, &stdout)
			}
		}
		return output, stdout.String()
	}
	output, text := run(map[string]any{"agent_prompt": "Explain how internal/parser/token.go handles ParseToken delimiters."})
	if output["systemMessage"] != nil {
		t.Fatalf("anchored subagent prompt degraded: %s", text)
	}
	hook := claudeHookOutput(t, output)
	contextText, _ := hook["additionalContext"].(string)
	if hook["hookEventName"] != "SubagentStart" || !strings.Contains(contextText, untrustedDataPrefix) || !strings.Contains(contextText, "internal/parser/token.go") {
		t.Fatalf("subagent packet: %s", text)
	}
	if strings.Contains(contextText, "Corvint local workflow argv") {
		t.Fatalf("subagent packet carries the session workflow argv: %s", text)
	}
	if _, text := run(nil); !strings.Contains(text, "missing-prompt") {
		t.Fatalf("absent agent_prompt is not a named degradation: %s", text)
	}
}
