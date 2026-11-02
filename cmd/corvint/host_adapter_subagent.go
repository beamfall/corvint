package main

import (
	"context"
	"maps"
	"strings"
)

// runClaudeSubagentStart is the `claude-code subagent-start` hook event (AHI-804, V1-0401). The
// documented SubagentStart payload (session_id, transcript_path, cwd, hook_event_name, agent_id,
// agent_type) carries no task text, so that payload abstains silently with {} and a missing-prompt
// ledger row; no task is ever derived from agent_type or the transcript. When a host supplies
// `agent_prompt`, it is compiled through the user-prompt path, with its budget, anchors and
// degradation codes, and rendered for SubagentStart. The host cannot block a spawn, and the
// adapter never asks to: any decision or continuation member is dropped.
func runClaudeSubagentStart(ctx context.Context, payload map[string]any) map[string]any {
	text, _ := payload["agent_prompt"].(string)
	if strings.TrimSpace(text) == "" {
		if root, err := claudeAdapterRoot(ctx); err == nil {
			recordAdapterReason(ctx, root, "claude-code", "subagent-start", "missing-prompt")
		}
		return map[string]any{}
	}
	prompt := maps.Clone(payload)
	prompt["prompt"] = text
	prompt["hook_event_name"] = "UserPromptSubmit"
	output := runClaudeAdapter(ctx, "user-prompt", prompt)
	for _, member := range []string{"decision", "reason", "continue", "stopReason"} {
		delete(output, member)
	}
	if hook, ok := output["hookSpecificOutput"].(map[string]any); ok {
		hook = maps.Clone(hook)
		hook["hookEventName"] = "SubagentStart"
		output["hookSpecificOutput"] = hook
	}
	return output
}
