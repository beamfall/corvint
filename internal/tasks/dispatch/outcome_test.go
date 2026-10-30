package dispatch

import (
	"slices"
	"strings"
	"testing"
)

// TQD-V0-030: each host line is classified only from its format's
// recognised vocabulary; every other line, and every line of an unknown
// format, is HostNone.
func TestTQDV0030_ClassifyHostLine(t *testing.T) {
	finish := `{"type":"step_finish","timestamp":2,"sessionID":"ses_1","part":{"type":"step-finish","reason":"stop","tokens":{"input":5,"output":2,"reasoning":0,"cache":{"read":1,"write":0}}}}`
	for _, c := range []struct {
		format, line string
		want         HostEvent
	}{
		{"codex", `{"type":"turn.started"}`, HostStart},
		{"codex", `{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`, HostFinish},
		{"codex", `{"type":"turn.failed","error":{"message":"overloaded"}}`, HostError},
		{"codex", `{"type":"error","message":"stream disconnected"}`, HostError},
		{"codex", `{"type":"item.completed","item":{"id":"i","type":"command_execution","exit_code":0,"status":"completed"}}`, HostToolOK},
		{"codex", `{"type":"item.completed","item":{"id":"i","type":"command_execution","exit_code":1,"status":"failed"}}`, HostToolFailed},
		{"codex", `{"type":"item.completed","item":{"id":"i","type":"file_change","status":"declined"}}`, HostToolFailed},
		{"codex", `{"type":"item.completed","item":{"id":"i","type":"agent_message","text":"hi"}}`, HostNone},
		{"codex", `{"type":"turn.started","Type":"turn.started"}`, HostNone},
		{"codex", `{"type":"turn.started","type":"turn.completed"}`, HostNone},
		{"codex", `not json`, HostNone},
		{"codex", `{"type":"thread.started"}`, HostNone},
		{"opencode", `{"type":"step_start","timestamp":1,"sessionID":"ses_1","part":{"type":"step-start"}}`, HostStart},
		{"opencode", finish, HostFinish},
		{"opencode", `{"type":"error","timestamp":"t","sessionID":"ses_1"}`, HostError},
		{"opencode", `{"type":"text","part":{"text":"x"}}`, HostNone},
		{"claude-code", `{"type":"result","subtype":"success","is_error":false,"session_id":"s","result":"x","usage":{"input_tokens":1,"output_tokens":1}}`, HostFinish},
		{"claude-code", `{"type":"result","subtype":"error_during_execution","is_error":true,"session_id":"s"}`, HostError},
		{"claude-code", `{"type":"user","message":{"content":[{"type":"tool_result","is_error":true,"content":"denied"}]}}`, HostToolFailed},
		{"claude-code", `{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}`, HostToolOK},
		{"claude-code", `{"type":"user","parent_tool_use_id":"toolu_1","message":{"content":[{"type":"tool_result","is_error":true}]}}`, HostNone},
		{"claude-code", `{"type":"assistant","message":{"content":[]}}`, HostNone},
		{"other", `{"type":"turn.started"}`, HostNone},
		{"codex", `{"type":"turn.started","pad":"` + strings.Repeat("x", maxUsageLine) + `"}`, HostNone},
	} {
		var want []HostEvent
		if c.want != HostNone {
			want = []HostEvent{c.want}
		}
		if got := ClassifyHostLine(c.format, []byte(c.line)); !slices.Equal(got, want) {
			n := min(len(c.line), 120)
			t.Errorf("%s %s: got %v, want %v", c.format, c.line[:n], got, want)
		}
	}
	// A Claude Code result the CAL-V0-075 member rules reject is no event,
	// never a host error; each tool_result block is one call, in order.
	failed := `{"type":"tool_result","is_error":true,"content":"denied"}`
	for _, c := range []struct {
		line string
		want []HostEvent
	}{
		{`{"type":"result","type":"result","subtype":"error_during_execution","is_error":true,"session_id":"s"}`, nil},
		{`{"type":"result","Type":"result","subtype":"error_during_execution","is_error":true,"session_id":"s"}`, nil},
		{`{"type":"user","message":{"content":[` + failed + `,` + failed + `,` + failed + `]}}`, []HostEvent{HostToolFailed, HostToolFailed, HostToolFailed}},
		{`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"},` + failed + `]}}`, []HostEvent{HostToolOK, HostToolFailed}},
	} {
		if got := ClassifyHostLine("claude-code", []byte(c.line)); !slices.Equal(got, c.want) {
			t.Errorf("claude-code %s: got %v, want %v", c.line, got, c.want)
		}
	}
}
