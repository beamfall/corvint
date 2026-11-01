package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// V1-3543 (AHI-808): adapter help lists the accepted events per host, and unsupported-hook-event
// names them while its frame, and so its ledger reason, stays unchanged.
func TestAHI808AdapterEventsAreDiscoverable(t *testing.T) {
	t.Parallel()
	events := func(prefix string) string {
		start := strings.Index(adapterHelp, prefix)
		if start < 0 {
			t.Fatalf("adapter help has no %q row:\n%s", prefix, adapterHelp)
		}
		return adapterHelp[start:]
	}
	for _, event := range claudeCodeAdapterEvents {
		if row := events("claude-code EVENT  "); !strings.Contains(row[:strings.Index(row, "codex  ")], event) {
			t.Fatalf("help omits claude-code event %s", event)
		}
	}
	for name := range codexHookEvents {
		if !strings.Contains(events("codex              "), name) {
			t.Fatalf("help omits codex hook_event_name %s", name)
		}
	}
	for _, event := range gokernel.SupportedEvents() {
		if !strings.Contains(events("pi EVENT           "), event) {
			t.Fatalf("help omits pi event %s", event)
		}
	}
	for _, test := range []struct {
		arguments []string
		names     []string
	}{
		{[]string{"claude-code", "nope"}, claudeCodeAdapterEvents},
		{[]string{"claude-code"}, claudeCodeAdapterEvents},
		{[]string{"codex", "user-prompt"}, []string{"takes no EVENT", "UserPromptSubmit", "SessionStart"}},
		{[]string{"nohost"}, []string{"codex", "claude-code EVENT", "pi EVENT"}},
	} {
		var stdout bytes.Buffer
		ctx := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": t.TempDir()})
		runHostAdapter(ctx, test.arguments, strings.NewReader(`{"session_id":"s"}`), &stdout)
		var output map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			t.Fatalf("%v: %s", test.arguments, &stdout)
		}
		message, _ := output["systemMessage"].(string)
		if adapterDegradationReason(output) != "unsupported-hook-event" {
			t.Fatalf("%v: frame changed: %q", test.arguments, message)
		}
		for _, name := range test.names {
			if !strings.Contains(message, name) {
				t.Fatalf("%v: message does not name %q: %q", test.arguments, name, message)
			}
		}
	}
}
