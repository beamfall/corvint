package main

import (
	"context"
	"maps"
)

// runClaudeSubagentStart is the `claude-code subagent-start` hook event (AHI-804, V1-0401). It
// compiles the spawned subagent's `agent_prompt` through the user-prompt path, with its budget,
// anchors and degradation codes, and renders the result for SubagentStart. The host cannot block
// a spawn, and the adapter never asks to: any decision or continuation member is dropped.
func runClaudeSubagentStart(ctx context.Context, payload map[string]any) map[string]any {
	prompt := maps.Clone(payload)
	prompt["prompt"], _ = payload["agent_prompt"].(string)
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
