package dispatch

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// HostEvent is one standard-output line's session-outcome meaning under a
// declared usage format (TQD-V0-030). HostNone is every line the format's
// vocabulary does not recognise.
type HostEvent int

const (
	HostNone HostEvent = iota
	// HostStart opens a turn or step; HostFinish closes one successfully.
	HostStart
	HostFinish
	// HostError is a provider or host failure event.
	HostError
	// HostToolOK and HostToolFailed are one completed tool call.
	HostToolOK
	HostToolFailed
)

// codexToolItems are the Codex item types that reach outside the prompt.
var codexToolItems = []string{"command_execution", "file_change", "mcp_tool_call", "web_search"}

// ClassifyHostLine reads one line of a worker's stdout.log under format,
// one of UsageFormats, with the member rules the usage reader applies
// (CAL-V0-157): a line that is not a qualified event of the format, longer
// than the usage line bound, or of an unknown format is HostNone.
func ClassifyHostLine(format string, b []byte) HostEvent {
	if len(b) > maxUsageLine || len(bytes.TrimSpace(b)) == 0 || !slices.Contains(UsageFormats, format) {
		return HostNone
	}
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(b, &head) != nil {
		return HostNone
	}
	switch format {
	case "codex":
		return codexHostEvent(head.Type, b)
	case "opencode":
		kind, _, _, _, _, ok := supervisor.OpenCodeLineUsage(b)
		switch {
		case !ok:
			return HostNone
		case kind == "error":
			return HostError
		case kind == "step_start":
			return HostStart
		case kind == "step_finish":
			return HostFinish
		}
	case "claude-code":
		return claudeHostEvent(head.Type, b)
	}
	return HostNone
}

func codexHostEvent(kind string, b []byte) HostEvent {
	switch kind {
	case "turn.started", "turn.completed", "turn.failed", "error":
		if supervisor.ExactMembers(b, []string{"type", "usage"}) != nil {
			return HostNone
		}
		switch kind {
		case "turn.started":
			return HostStart
		case "turn.completed":
			return HostFinish
		}
		return HostError
	case "item.completed":
		if supervisor.ExactMembers(b, []string{"type", "item"}) != nil {
			return HostNone
		}
		var ev struct {
			Item struct {
				Type     string          `json:"type"`
				Status   string          `json:"status"`
				ExitCode json.RawMessage `json:"exit_code"`
			} `json:"item"`
		}
		if json.Unmarshal(b, &ev) != nil || !slices.Contains(codexToolItems, ev.Item.Type) {
			return HostNone
		}
		if ev.Item.Status == "failed" || ev.Item.Status == "declined" {
			return HostToolFailed
		}
		if code := bytes.TrimSpace(ev.Item.ExitCode); len(code) > 0 && !bytes.Equal(code, []byte("null")) && !bytes.Equal(code, []byte("0")) {
			return HostToolFailed
		}
		return HostToolOK
	}
	return HostNone
}

// claudeHostEvent reads Claude Code stream-json: a result object is a
// finish or, under ClaudeResultFailed, an error; a top-level user message's
// tool_result blocks are tool calls (a subagent's, with a parent tool use,
// are not the session's own).
func claudeHostEvent(kind string, b []byte) HostEvent {
	switch kind {
	case "result":
		if supervisor.ClaudeResultFailed(b) {
			return HostError
		}
		return HostFinish
	case "user":
		if supervisor.ExactMembers(b, []string{"type", "message"}) != nil {
			return HostNone
		}
		var ev struct {
			Parent  json.RawMessage `json:"parent_tool_use_id"`
			Message struct {
				Content []struct {
					Type    string `json:"type"`
					IsError *bool  `json:"is_error"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(b, &ev) != nil {
			return HostNone
		}
		if p := bytes.TrimSpace(ev.Parent); len(p) > 0 && !bytes.Equal(p, []byte("null")) {
			return HostNone
		}
		out := HostNone
		for _, c := range ev.Message.Content {
			if c.Type != "tool_result" {
				continue
			}
			if c.IsError != nil && *c.IsError {
				return HostToolFailed
			}
			out = HostToolOK
		}
		return out
	}
	return HostNone
}
