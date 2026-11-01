// Package tasksbridge is the corvint-mcp tasks tool group (TAW-V0-015): the
// role-scoped corvint-tasks work verbs as MCP tools. Each call runs the same
// CLI verb in process against the declared root, with the session class
// set, and returns its command envelope. It opens no listener and starts no
// process: the server is stdio and runs on the host that holds the store.
package tasksbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/tasks/cli"
)

// The tasks group tool names (TAW-V0-015).
const (
	ToolStart        = "tasks_work_start"
	ToolDeliver      = "tasks_work_deliver"
	ToolBlock        = "tasks_work_block"
	ToolFinding      = "tasks_work_finding"
	ToolCloseMerged  = "tasks_work_close_merged"
	ToolReleaseSync  = "tasks_work_release_sync"
	ErrorProfile     = "corvint-tasks-mcp-tool-error/0"
	maxTickets       = 32
	maxTextBytes     = 512
	maxFindings      = 64
	maxRefBytes      = 256
	maxRequestBytes  = 128
	maxPayloadsBytes = 1 << 20
)

// Classes are the --tasks-role values; AGENT is the default.
var Classes = []string{cli.ClassAgent, cli.ClassOperator, cli.ClassOwner}

// Group answers the tasks tools for one store root under one class.
type Group struct {
	Root  string
	Class string
}

func stringSchema(max int) map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
}

func arraySchema(items map[string]any, max int) map[string]any {
	return map[string]any{"type": "array", "items": items, "minItems": 1, "maxItems": max}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	req := make([]any, 0, len(required))
	for _, r := range required {
		req = append(req, r)
	}
	return map[string]any{"type": "object", "properties": properties, "required": req, "additionalProperties": false}
}

func laneSchema(reasonRequired bool) map[string]any {
	props := map[string]any{
		"tickets":      arraySchema(stringSchema(64), maxTickets),
		"reason":       stringSchema(maxTextBytes),
		"lane":         stringSchema(64),
		"allowUnknown": map[string]any{"type": "boolean"},
	}
	if reasonRequired {
		return objectSchema(props, "tickets", "reason")
	}
	return objectSchema(props, "tickets")
}

// Tools lists the tools of the class: the lane verbs and finding for every
// class, the store-host verbs only for OPERATOR and OWNER.
func (g Group) Tools() []bridge.ToolDescriptor {
	lane := bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	write := bridge.ToolAnnotations{IdempotentHint: true}
	tools := []bridge.ToolDescriptor{
		{Name: ToolStart, Description: "Validate a lane's start of work on tickets against the local store copy and return the Corvint-Ticket commit trailers to add; writes nothing (docs/specs/corvint-tasks-agent-workflow-v0.md TAW-V0-003).", InputSchema: laneSchema(false), Annotations: lane},
		{Name: ToolDeliver, Description: "Validate a lane's delivery of tickets and return the Corvint-Ticket-State: delivered commit trailers; the store host completes the ticket once that commit is on the integration ref. Writes nothing (TAW-V0-003).", InputSchema: laneSchema(false), Annotations: lane},
		{Name: ToolBlock, Description: "Validate a lane's block on tickets with a one-line reason and return the Corvint-Ticket-State: blocked commit trailers; writes nothing (TAW-V0-003).", InputSchema: laneSchema(true), Annotations: lane},
		{Name: ToolFinding, Description: "File a finding after the similar-ticket check: a duplicate attaches to the existing ticket as a recurrence, newDespite overrides named matches, findings dedupes a batch, and supersede/by archives a duplicate with supersededBy (TAW-V0-010 to TAW-V0-012).", Annotations: write,
			InputSchema: objectSchema(map[string]any{
				"payload":    map[string]any{"type": "object"},
				"findings":   map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "minItems": 1, "maxItems": maxFindings},
				"supersede":  stringSchema(64),
				"by":         stringSchema(64),
				"newDespite": arraySchema(stringSchema(64), maxTickets),
				"requestId":  stringSchema(maxRequestBytes),
				"issuedAt":   stringSchema(32),
			}, "requestId")},
	}
	if g.Class == cli.ClassOperator || g.Class == cli.ClassOwner {
		tools = append(tools,
			bridge.ToolDescriptor{Name: ToolCloseMerged, Description: "On the store host, ingest the Corvint-Ticket trailers reachable from the integration ref: complete delivered tickets with an evidence document, record blocks, and report lane deliveries not yet integrated; idempotent (TAW-V0-004 to TAW-V0-008).", Annotations: write,
				InputSchema: objectSchema(map[string]any{
					"integration": stringSchema(maxRefBytes),
					"since":       stringSchema(64),
					"laneRefs":    arraySchema(stringSchema(maxRefBytes), maxTickets),
					"dryRun":      map[string]any{"type": "boolean"},
				}, "integration")},
			bridge.ToolDescriptor{Name: ToolReleaseSync, Description: "On the store host, add the OPEN tickets of a milestone missing from a release in one membership write and report members outside the milestone; idempotent (TAW-V0-014).", Annotations: write,
				InputSchema: objectSchema(map[string]any{
					"release":   stringSchema(64),
					"milestone": stringSchema(64),
					"requestId": stringSchema(maxRequestBytes),
					"issuedAt":  stringSchema(32),
					"dryRun":    map[string]any{"type": "boolean"},
				}, "release", "milestone", "requestId")},
		)
	}
	return tools
}

// args reads the closed arguments object: every key must be one the schema
// names, with the schema's type. Errors are invalid params.
type args map[string]any

func (a args) str(key string, max int) (string, bool) {
	v, present := a[key]
	if !present {
		return "", true
	}
	s, ok := v.(string)
	return s, ok && s != "" && len(s) <= max
}

func (a args) strs(key string, max, maxLen int) ([]string, bool) {
	v, present := a[key]
	if !present {
		return nil, true
	}
	list, ok := v.([]any)
	if !ok || len(list) == 0 || len(list) > max {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok || s == "" || len(s) > maxLen {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func (a args) flag(key string) (bool, bool) {
	v, present := a[key]
	if !present {
		return false, true
	}
	b, ok := v.(bool)
	return b, ok
}

func (a args) closed(keys ...string) bool {
	for k := range a {
		found := false
		for _, want := range keys {
			found = found || k == want
		}
		if !found {
			return false
		}
	}
	return true
}

var errInvalid = protocol.InvalidParams("Invalid params")

// argv maps one call to the corvint-tasks argument vector and its stdin.
func (g Group) argv(name string, a args) ([]string, []byte, bool) {
	listed := false
	for _, tool := range g.Tools() {
		listed = listed || tool.Name == name
	}
	if !listed {
		return nil, nil, false
	}
	switch name {
	case ToolStart, ToolDeliver, ToolBlock:
		tickets, ok1 := a.strs("tickets", maxTickets, 64)
		reason, ok2 := a.str("reason", maxTextBytes)
		lane, ok3 := a.str("lane", 64)
		unknown, ok4 := a.flag("allowUnknown")
		if !ok1 || !ok2 || !ok3 || !ok4 || len(tickets) == 0 || !a.closed("tickets", "reason", "lane", "allowUnknown") {
			return nil, nil, false
		}
		verb := map[string]string{ToolStart: "start", ToolDeliver: "deliver", ToolBlock: "block"}[name]
		out := append([]string{"work", verb}, tickets...)
		if reason != "" {
			out = append(out, "--reason", reason)
		}
		if lane != "" {
			out = append(out, "--lane", lane)
		}
		if unknown {
			out = append(out, "--allow-unknown")
		}
		return out, nil, true
	case ToolFinding:
		request, ok1 := a.str("requestId", maxRequestBytes)
		issued, ok2 := a.str("issuedAt", 32)
		sup, ok3 := a.str("supersede", 64)
		by, ok4 := a.str("by", 64)
		despite, ok5 := a.strs("newDespite", maxTickets, 64)
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || request == "" || !a.closed("payload", "findings", "supersede", "by", "newDespite", "requestId", "issuedAt") {
			return nil, nil, false
		}
		out := []string{"work", "finding"}
		var stdin []byte
		payload, hasPayload := a["payload"].(map[string]any)
		findings, hasFindings := a["findings"].([]any)
		if _, present := a["payload"]; present && !hasPayload {
			return nil, nil, false
		}
		if _, present := a["findings"]; present && (!hasFindings || len(findings) == 0 || len(findings) > maxFindings) {
			return nil, nil, false
		}
		switch {
		case hasPayload:
			stdin = encodeStdin(payload)
			out = append(out, "--payload", "-")
		case hasFindings:
			stdin = encodeStdin(findings)
			out = append(out, "--ingest", "-")
		}
		if len(stdin) > maxPayloadsBytes {
			return nil, nil, false
		}
		if sup != "" {
			out = append(out, "--supersede", sup)
		}
		if by != "" {
			out = append(out, "--by", by)
		}
		for _, d := range despite {
			out = append(out, "--new-despite", d)
		}
		out = append(out, "--request-id", request)
		if issued != "" {
			out = append(out, "--issued-at", issued)
		}
		return out, stdin, true
	case ToolCloseMerged:
		ref, ok1 := a.str("integration", maxRefBytes)
		since, ok2 := a.str("since", 64)
		lanes, ok3 := a.strs("laneRefs", maxTickets, maxRefBytes)
		dry, ok4 := a.flag("dryRun")
		if !ok1 || !ok2 || !ok3 || !ok4 || ref == "" || !a.closed("integration", "since", "laneRefs", "dryRun") {
			return nil, nil, false
		}
		out := []string{"work", "close-merged", "--integration", ref}
		if since != "" {
			out = append(out, "--since", since)
		}
		for _, l := range lanes {
			out = append(out, "--lane-ref", l)
		}
		if dry {
			out = append(out, "--dry-run")
		}
		return out, nil, true
	case ToolReleaseSync:
		rel, ok1 := a.str("release", 64)
		ms, ok2 := a.str("milestone", 64)
		request, ok3 := a.str("requestId", maxRequestBytes)
		dry, ok4 := a.flag("dryRun")
		issued, ok5 := a.str("issuedAt", 32)
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || rel == "" || ms == "" || request == "" || !a.closed("release", "milestone", "requestId", "issuedAt", "dryRun") {
			return nil, nil, false
		}
		out := []string{"work", "release-sync", "--release", rel, "--milestone", ms, "--request-id", request}
		if issued != "" {
			out = append(out, "--issued-at", issued)
		}
		if dry {
			out = append(out, "--dry-run")
		}
		return out, nil, true
	}
	return nil, nil, false
}

// Call runs one listed tool. A tool the class does not list, or arguments
// outside its schema, are invalid params; a refused verb is a tool error
// that carries the command envelope.
func (g Group) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	argv, stdin, ok := g.argv(name, args(arguments))
	if !ok {
		return nil, errInvalid
	}
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run(cli.Env{Cwd: g.Root, Args: argv, Stdin: bytes.NewReader(stdin), Stdout: &stdout, Stderr: &stderr, Role: g.Class})
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		return failure(name, "TASKS_ENVELOPE_UNREADABLE", fmt.Sprintf("corvint-tasks exited %d without an envelope", code))
	}
	// Ticket titles and reasons are repository-authored text, so the text
	// block is framed (AHI-004); structuredContent stays unwrapped.
	framed, err := repoenvelope.Frame(stdout.String())
	if err != nil {
		return failure(name, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           code != 0,
		"structuredContent": envelope,
	}, nil
}

// encodeStdin writes a decoded JSON argument back as one canonical-key JSON
// document with the trailing newline the corvint-tasks reader requires; the
// verb itself canonicalizes and validates it.
func encodeStdin(value any) []byte {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(value) != nil {
		return nil
	}
	return buf.Bytes()
}

func failure(name, code, message string) (map[string]any, *protocol.RPCError) {
	value := map[string]any{"code": code, "error": message, "profile": ErrorProfile, "tool": name}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(raw)}},
		"isError":           true,
		"structuredContent": value,
	}, nil
}
