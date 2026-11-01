package main

import (
	"context"
	"maps"
	"path/filepath"
	"strings"
)

// runClaudePreToolRead is the `claude-code pre-tool` hook event for Read and Grep (AHI-805,
// V1-0402). Corvint holds no authority over a read, so every answer is permissionDecision
// `defer`. A Read file_path or Grep path inside the project is compiled as a one-path user-prompt
// anchor, adding that path's evidence as additionalContext or a named degradation; any other tool
// or target gets the bare defer. It records nothing the user-prompt path does not already record:
// unplanned-read rows stay gated on the operator marker (AGENTS.md invariant 4, SOL-V0-010).
func runClaudePreToolRead(ctx context.Context, payload map[string]any) map[string]any {
	output := map[string]any{}
	if target, ok := preToolReadTarget(ctx, payload); ok {
		prompt := maps.Clone(payload)
		prompt["prompt"] = "`" + target + "`"
		prompt["hook_event_name"] = "UserPromptSubmit"
		output = runClaudeAdapter(ctx, "user-prompt", prompt)
	}
	for _, member := range []string{"decision", "reason", "continue", "stopReason"} {
		delete(output, member)
	}
	hook, _ := output["hookSpecificOutput"].(map[string]any)
	hook = maps.Clone(hook)
	if hook == nil {
		hook = map[string]any{}
	}
	delete(hook, "permissionDecisionReason")
	hook["hookEventName"] = "PreToolUse"
	hook["permissionDecision"] = "defer"
	output["hookSpecificOutput"] = hook
	return output
}

// preToolReadTarget is the project-relative path a Read or Grep call names, or false for another
// tool, an absent path, the project root itself, or a path outside the project.
func preToolReadTarget(ctx context.Context, payload map[string]any) (string, bool) {
	input, _ := payload["tool_input"].(map[string]any)
	var target string
	switch payload["tool_name"] {
	case "Read":
		target, _ = input["file_path"].(string)
	case "Grep":
		target, _ = input["path"].(string)
	}
	if strings.TrimSpace(target) == "" {
		return "", false
	}
	root, err := claudeAdapterRoot(ctx)
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	resolved, ok := resolvePostToolTarget(target)
	if !ok {
		return "", false
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}
