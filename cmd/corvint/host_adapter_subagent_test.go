package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
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
}

// AHI-804 (V1-0401): the payload shaped exactly as the Claude Code hooks reference documents
// SubagentStart (no prompt field) abstains silently with {}, never a degradation banner or a
// packet derived from agent_type, and a blank agent_prompt does the same.
func TestAHI804DocumentedSubagentStartPayloadAbstainsSilently(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	documented := map[string]any{
		"session_id": "abc123", "transcript_path": "/Users/dev/.claude/projects/p/abc123.jsonl",
		"cwd": root, "hook_event_name": "SubagentStart", "agent_id": "agent-abc123", "agent_type": "Explore",
	}
	blank := map[string]any{"agent_prompt": "   "}
	for key, value := range documented {
		blank[key] = value
	}
	for _, payload := range []map[string]any{documented, blank} {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var stdout bytes.Buffer
		ctx := adapterEnvContext(lifecycleDeadlineContext(), map[string]string{"CLAUDE_PROJECT_DIR": root})
		if status := runHostAdapter(ctx, []string{"claude-code", "subagent-start"}, bytes.NewReader(raw), &stdout); status != 0 {
			t.Fatalf("status=%d output=%s", status, &stdout)
		}
		if strings.TrimSpace(stdout.String()) != "{}" {
			t.Fatalf("documented SubagentStart payload must abstain silently: %s", &stdout)
		}
	}
}

// AHI-804, AHI-805: the opt-in events run under the 2 s host kill like a registered event, so a
// blocked stdin read degrades to adapter-host-kill-deadline before the kill, with no permission
// decision, instead of running unbounded.
func TestAHI804OptInEventsRunUnderTheHostKill(t *testing.T) {
	t.Parallel()
	for _, event := range []string{"subagent-start", "pre-tool"} {
		t.Run(event, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			arguments := []string{"adapter", "claude-code", event}
			env := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": t.TempDir()})
			ctx, cancel := adapterHostKillContext(env, arguments, start)
			defer cancel()
			deadline, ok := ctx.Deadline()
			if !ok || !deadline.Equal(start.Add(2*time.Second-adapterProcessReserve)) {
				t.Fatalf("%s: deadline %v (bounded=%v), want the 2 s kill less reserve", event, deadline, ok)
			}
			stdin, writer := io.Pipe()
			t.Cleanup(func() { _ = writer.Close() })
			done := make(chan string, 1)
			go func() {
				var stdout bytes.Buffer
				runHostAdapter(ctx, arguments[1:], stdin, &stdout)
				done <- stdout.String()
			}()
			select {
			case output := <-done:
				if !strings.Contains(output, "adapter-host-kill-deadline") || strings.Contains(output, "permissionDecision") {
					t.Fatalf("%s: watchdog output=%q", event, output)
				}
			case <-time.After(time.Minute): // hang detector, not a budget
				t.Fatalf("%s: adapter did not self-terminate while its stdin read was blocked", event)
			}
		})
	}
}
