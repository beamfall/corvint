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
// (CAL-V0-157), returning its events in line order: one for most events,
// one per tool_result block of a Claude Code user message. A line that is
// not a qualified event of the format, longer than the usage line bound, or
// of an unknown format has none.
func ClassifyHostLine(format string, b []byte) []HostEvent {
	if len(b) > maxUsageLine || len(bytes.TrimSpace(b)) == 0 || !slices.Contains(UsageFormats, format) {
		return nil
	}
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(b, &head) != nil {
		return nil
	}
	one := func(e HostEvent) []HostEvent {
		if e == HostNone {
			return nil
		}
		return []HostEvent{e}
	}
	switch format {
	case "codex":
		return one(codexHostEvent(head.Type, b))
	case "opencode":
		kind, _, _, _, _, ok := supervisor.OpenCodeLineUsage(b)
		switch {
		case !ok:
			return nil
		case kind == "error":
			return one(HostError)
		case kind == "step_start":
			return one(HostStart)
		case kind == "step_finish":
			return one(HostFinish)
		}
	case "claude-code":
		return claudeHostEvents(head.Type, b)
	}
	return nil
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

// claudeHostEvents reads Claude Code stream-json: a result object the
// CAL-V0-075 member rules admit is a finish or, under ClaudeResultFailed, an
// error, and a rejected one is nothing; each tool_result block of a
// top-level user message is one tool call, in block order (a subagent's,
// with a parent tool use, are not the session's own).
func claudeHostEvents(kind string, b []byte) []HostEvent {
	switch kind {
	case "result":
		switch {
		case !supervisor.ClaudeResultAdmitted(b):
			return nil
		case supervisor.ClaudeResultFailed(b):
			return []HostEvent{HostError}
		}
		return []HostEvent{HostFinish}
	case "user":
		if supervisor.ExactMembers(b, []string{"type", "message"}) != nil {
			return nil
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
			return nil
		}
		if p := bytes.TrimSpace(ev.Parent); len(p) > 0 && !bytes.Equal(p, []byte("null")) {
			return nil
		}
		var out []HostEvent
		for _, c := range ev.Message.Content {
			if c.Type != "tool_result" {
				continue
			}
			if c.IsError != nil && *c.IsError {
				out = append(out, HostToolFailed)
			} else {
				out = append(out, HostToolOK)
			}
		}
		return out
	}
	return nil
}
