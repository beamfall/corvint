package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// maxHookPayload bounds the hook payload read from stdin (ATR-V0-101).
const maxHookPayload = 64 << 10

// dispatchContextHook is `dispatch context-hook`, the Claude Code hook that
// delivers a worker's undelivered WARN and HANDOFF notices into its session
// (ATR-V0-101). It writes the hook's own JSON, not an envelope, and always
// exits 0 so it never blocks the session; failures go to stderr.
func dispatchContextHook(env Env, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(env.Stderr, "corvint-tasks dispatch context-hook: takes no arguments")
		return 0
	}
	event := "PostToolUse"
	raw, _ := io.ReadAll(io.LimitReader(env.Stdin, maxHookPayload))
	var payload struct {
		HookEventName string `json:"hook_event_name"`
	}
	if json.Unmarshal(raw, &payload) == nil && payload.HookEventName != "" && len(payload.HookEventName) <= 64 {
		event = payload.HookEventName
	}
	notice, err := dispatch.ContextHookNotice(os.Getenv("CORVINT_DISPATCH_CONTEXT_FILE"))
	if err != nil {
		fmt.Fprintln(env.Stderr, "corvint-tasks dispatch context-hook: "+err.Error())
	}
	if notice == "" {
		return 0
	}
	out, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": event, "additionalContext": notice}})
	fmt.Fprintln(env.Stdout, string(out))
	return 0
}
