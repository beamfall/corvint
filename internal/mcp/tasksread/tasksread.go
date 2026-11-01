// Package tasksread is the read-only Corvint Tasks MCP profile (TMR-V0): five
// typed, paged tools that each run one corvint-tasks read verb through the
// CLI, so the CLI stays the one authority for every answer.
package tasksread

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	// Name is the read-only Corvint Tasks MCP server (TMR-V0-950).
	Name = "corvint-tasks-mcp"
	// tasksMCPToolError is its closed tool-error profile (TMR-V0-952).
	tasksMCPToolError = "corvint-tasks-mcp-tool-error/0"
)

// tasksMCPTool is one read-only tool: the CLI read verb it runs, its closed
// arguments, and how each argument becomes that verb's argv (TMR-V0-951).
type tasksMCPTool struct {
	name, description string
	verb              []string
	properties        map[string]any
	required          []string
	// positional names the argument passed as the verb's one positional word.
	positional string
}

// tasksMCPDescriptor is the tools/list entry, the MCP Tool shape.
type tasksMCPDescriptor struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  map[string]any  `json:"inputSchema"`
	OutputSchema map[string]any  `json:"outputSchema"`
	Annotations  map[string]bool `json:"annotations"`
}

var ticketIDPattern = `^[A-Za-z0-9][A-Za-z0-9:._-]{0,199}$`

func pageProperties() map[string]any {
	return map[string]any{
		"offset": map[string]any{"type": "integer", "minimum": 0, "default": 0},
		"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": wire.PageMax, "default": wire.PageDefault},
	}
}

func enumOf(values []string) []any {
	enum := make([]any, len(values))
	for index, value := range values {
		enum[index] = value
	}
	return enum
}

// tasksMCPTools are the five read-only tools, in name order.
func tasksMCPTools() []tasksMCPTool {
	label := map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "pattern": `^[^\x00-\x1f\x7f]+$`}
	search := pageProperties()
	search["status"] = map[string]any{"type": "string", "enum": enumOf(ticket.Statuses)}
	search["kind"] = map[string]any{"type": "string", "enum": enumOf(ticket.Kinds)}
	search["priority"] = map[string]any{"type": "string", "enum": []any{"P0", "P1", "P2", "P3", "P4"}}
	search["owner"], search["milestone"], search["label"] = label, label, label
	search["text"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 512, "pattern": `^[^\x00-\x1f\x7f]+$`}
	return []tasksMCPTool{
		{name: "corvint.tasks.queue_status", verb: []string{"queue", "status"}, properties: map[string]any{},
			description: "Report the Corvint Tasks queue: counts by status, blocked and eligible tickets, the last completion and journal facts, exactly as `corvint-tasks queue status` prints it (taskman-command-result/0)."},
		{name: "corvint.tasks.receipt_audit", verb: []string{"receipt", "audit"}, properties: map[string]any{},
			description: "Audit the Corvint Tasks journal receipts without writing, exactly as `corvint-tasks receipt audit` prints it (taskman-command-result/0)."},
		{name: "corvint.tasks.roadmap", verb: []string{"roadmap"}, properties: pageProperties(),
			description: "Page through the Corvint Tasks roadmap in queue order, exactly as `corvint-tasks roadmap --offset N --limit N` prints it; page.total and page.truncated say whether to fetch the next offset."},
		{name: "corvint.tasks.ticket_search", verb: []string{"ticket", "search"}, properties: search,
			description: "Search Corvint Tasks tickets by status, kind, priority, owner, milestone, label or text and page through the matches, exactly as `corvint-tasks ticket search` prints it; page.total and page.truncated say whether to fetch the next offset."},
		{name: "corvint.tasks.ticket_show", verb: []string{"ticket", "show"}, positional: "ticket", required: []string{"ticket"},
			properties:  map[string]any{"ticket": map[string]any{"type": "string", "minLength": 1, "maxLength": 200, "pattern": ticketIDPattern}},
			description: "Show one Corvint Tasks ticket (a full ticketId or a local ID such as V1-0001) with its record, blockers and unknowns, exactly as `corvint-tasks ticket show ID` prints it."},
	}
}

func (tool tasksMCPTool) descriptor() tasksMCPDescriptor {
	return tasksMCPDescriptor{
		Name: tool.name, Description: tool.description,
		InputSchema: map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
			"properties": tool.properties, "required": enumOf(tool.required), "additionalProperties": false,
		},
		// TMR-V0-951: every result's structuredContent is the CLI envelope or the tool-error object.
		OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"profile": map[string]any{"type": "string"}}, "required": []any{"profile"}},
		Annotations:  map[string]bool{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
	}
}

// argv turns validated arguments into the verb's argv, or names the first
// argument that breaks the advertised schema.
func (tool tasksMCPTool) argv(arguments map[string]any) ([]string, *protocol.RPCError) {
	invalid := func(reason, field, rule string, allowed []any) *protocol.RPCError {
		failure := protocol.InvalidParams("Invalid params")
		failure.Data = protocol.InvalidParamsData{Reason: reason, Field: "arguments." + field, Rule: rule, Allowed: allowed}
		return failure
	}
	for _, key := range slices.Sorted(mapKeysOf(arguments)) {
		if _, declared := tool.properties[key]; !declared {
			return nil, invalid("unknown-argument", key, "", enumOf(slices.Sorted(mapKeysOf(tool.properties))))
		}
	}
	for _, key := range tool.required {
		if _, present := arguments[key]; !present {
			return nil, invalid("missing-argument", key, "", nil)
		}
	}
	argv := slices.Clone(tool.verb)
	for _, key := range slices.Sorted(mapKeysOf(arguments)) {
		schema := tool.properties[key].(map[string]any)
		var word string
		switch schema["type"] {
		case "integer":
			number, ok := arguments[key].(json.Number)
			if !ok {
				return nil, invalid("invalid-type", key, "integer", nil)
			}
			value, err := number.Int64()
			minimum, _ := schema["minimum"].(int)
			if err != nil {
				return nil, invalid("invalid-type", key, "integer", nil)
			}
			if value < int64(minimum) || (schema["maximum"] != nil && value > int64(schema["maximum"].(int))) {
				return nil, invalid("out-of-range", key, fmt.Sprintf("minimum %d", minimum), nil)
			}
			word = strconv.FormatInt(value, 10)
		case "string":
			value, ok := arguments[key].(string)
			if !ok {
				return nil, invalid("invalid-type", key, "string", nil)
			}
			if enum, enumerated := schema["enum"].([]any); enumerated && !slices.Contains(enum, any(value)) {
				return nil, invalid("invalid-enum", key, "", enum)
			}
			if pattern, patterned := schema["pattern"].(string); patterned && !regexp.MustCompile(pattern).MatchString(value) {
				return nil, invalid("pattern-mismatch", key, pattern, nil)
			}
			if maximum, _ := schema["maxLength"].(int); maximum != 0 && len([]rune(value)) > maximum {
				return nil, invalid("out-of-range", key, fmt.Sprintf("maxLength %d", maximum), nil)
			}
			word = value
		}
		if key == tool.positional {
			argv = append(argv, word)
		} else {
			argv = append(argv, "--"+key, word)
		}
	}
	return argv, nil
}

func mapKeysOf[V any](values map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for key := range values {
			if !yield(key) {
				return
			}
		}
	}
}

// Run runs one corvint-tasks invocation, writing its envelope to stdout, and
// returns its exit code.
type Run func(args []string, stdout io.Writer) int

// Handler answers tools/list and tools/call. Each call runs the read verb
// through run in the server's own directory, so the server holds no state
// between calls.
type Handler struct {
	run   Run
	tools map[string]tasksMCPTool
}

// New binds the five tools to run.
func New(run Run) *Handler {
	handler := &Handler{run: run, tools: map[string]tasksMCPTool{}}
	for _, tool := range tasksMCPTools() {
		handler.tools[tool.name] = tool
	}
	return handler
}

func (handler *Handler) Handle(_ context.Context, request protocol.Request, _ server.Notifier) (map[string]any, *protocol.RPCError) {
	switch request.Method {
	case "tools/list":
		for key := range request.Params {
			if key != "_meta" && key != "cursor" {
				return nil, protocol.InvalidParamsAt("Invalid params", "unknown-param", key)
			}
		}
		if cursor, present := request.Params["cursor"]; present && cursor != "" {
			return nil, protocol.InvalidParamsAt("Invalid params", "invalid-cursor", "cursor")
		}
		tools := make([]tasksMCPDescriptor, 0, len(handler.tools))
		for _, tool := range handler.tools {
			tools = append(tools, tool.descriptor())
		}
		sort.Slice(tools, func(left, right int) bool { return tools[left].Name < tools[right].Name })
		return map[string]any{"cacheScope": "private", "tools": tools, "ttlMs": 300_000}, nil
	case "tools/call":
		return handler.call(request.Params)
	default:
		return nil, protocol.MethodNotFound()
	}
}

func (handler *Handler) call(params map[string]any) (map[string]any, *protocol.RPCError) {
	for key := range params {
		if key != "_meta" && key != "name" && key != "arguments" {
			return nil, protocol.InvalidParamsAt("Invalid params", "unknown-param", key)
		}
	}
	name, _ := params["name"].(string)
	tool, known := handler.tools[name]
	if !known {
		failure := protocol.InvalidParams("Invalid params")
		failure.Data = protocol.InvalidParamsData{Reason: "unknown-tool", Field: "name", Allowed: enumOf(slices.Sorted(mapKeysOf(handler.tools)))}
		return nil, failure
	}
	arguments := map[string]any{}
	if raw, present := params["arguments"]; present {
		object, isObject := raw.(map[string]any)
		if !isObject {
			return nil, protocol.InvalidParamsAt("Invalid params", "invalid-type", "arguments")
		}
		arguments = object
	}
	argv, refusal := tool.argv(arguments)
	if refusal != nil {
		return nil, refusal
	}
	var stdout bytes.Buffer
	exit := handler.run(argv, &stdout)
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return toolFailureResult(name, "CLI_OUTPUT_UNREADABLE", nil)
	}
	if exit != 0 || envelope["outcome"] != wire.OutcomeOK {
		codes, _ := envelope["codes"].([]any)
		return toolFailureResult(name, "CLI_REFUSED", map[string]any{"codes": codes, "envelope": envelope})
	}
	// Ticket titles and bodies are repository-authored text, so the text
	// block is framed by the shared untrusted-data envelope (AHI-004).
	framed, err := repoenvelope.Frame(stdout.String())
	if err != nil {
		return toolFailureResult(name, repoenvelope.CollisionCode, nil)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": envelope,
	}, nil
}

// toolFailureResult is the closed tool-error object; extra carries the CLI's
// own refusal codes and envelope when the verb refused.
func toolFailureResult(name, code string, extra map[string]any) (map[string]any, *protocol.RPCError) {
	value := map[string]any{"code": code, "mutates": false, "profile": tasksMCPToolError, "tool": name}
	for key, member := range extra {
		value[key] = member
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	// A refusal echoes the CLI envelope, which can carry ticket text, so only
	// it is framed; a bare code is not repository text.
	text := string(raw)
	if extra != nil {
		framed, frameErr := repoenvelope.Frame(text)
		if frameErr != nil {
			return toolFailureResult(name, repoenvelope.CollisionCode, nil)
		}
		text = framed
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": text}},
		"isError":           true,
		"structuredContent": value,
	}, nil
}
