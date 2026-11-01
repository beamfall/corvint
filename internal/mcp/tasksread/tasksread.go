// Package tasksread is the read-only Corvint Tasks tool group of corvint-mcp
// (TMR-V0): five typed, paged tools that each run one installed corvint-tasks
// read verb in the repository root with CORVINT_TASKS_READ_ONLY=1 and answer
// with its own envelope, so the CLI stays the one authority. It links no Tasks
// code beyond the wire package (decision 0397).
package tasksread

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	// ToolErrorProfile is the group's closed tool-error profile (TMR-V0-952).
	ToolErrorProfile = "corvint-tasks-mcp-tool-error/0"
	// Executable is the binary each call runs, resolved on PATH per call.
	Executable = "corvint-tasks"
	// callTimeout bounds one read verb.
	callTimeout = 60 * time.Second
	// maxOutputBytes bounds the envelope one call may return.
	maxOutputBytes = 512 * 1024
)

// tool is one read-only tool: the CLI read verb it runs, its closed
// arguments, and the argument passed as the verb's positional word.
type tool struct {
	name, description string
	verb              []string
	properties        map[string]any
	required          []string
	positional        string
}

func pageProperties() map[string]any {
	return map[string]any{
		"offset": map[string]any{"type": "integer", "minimum": 0, "default": 0},
		"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": wire.PageMax, "default": wire.PageDefault},
	}
}

func tools() []tool {
	text := func(maximum int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": maximum, "pattern": `^[^\x00-\x1f\x7f]+$`}
	}
	word := map[string]any{"type": "string", "minLength": 1, "maxLength": 16, "pattern": `^[A-Z][A-Z_]*$`}
	search := pageProperties()
	search["status"], search["kind"] = word, word
	search["priority"] = map[string]any{"type": "string", "pattern": `^P[0-9]$`}
	search["owner"], search["milestone"], search["label"] = text(128), text(128), text(128)
	search["text"] = text(512)
	return []tool{
		{name: "corvint.tasks.queue_status", verb: []string{"queue", "status"}, properties: map[string]any{},
			description: "Report the Corvint Tasks queue of this repository (counts by status, blocked and eligible tickets, the last completion), exactly as `corvint-tasks queue status` prints it (taskman-command-result/0)."},
		{name: "corvint.tasks.receipt_audit", verb: []string{"receipt", "audit"}, properties: map[string]any{},
			description: "Audit the Corvint Tasks journal receipts without writing, exactly as `corvint-tasks receipt audit` prints it (taskman-command-result/0)."},
		{name: "corvint.tasks.roadmap", verb: []string{"roadmap"}, properties: pageProperties(),
			description: "Page through the Corvint Tasks roadmap in queue order, exactly as `corvint-tasks roadmap --offset N --limit N` prints it; page.total and page.truncated say whether to fetch the next offset."},
		{name: "corvint.tasks.ticket_search", verb: []string{"ticket", "search"}, properties: search,
			description: "Search Corvint Tasks tickets by status, kind, priority, owner, milestone, label or text and page through the matches, exactly as `corvint-tasks ticket search` prints it; page.total and page.truncated say whether to fetch the next offset."},
		{name: "corvint.tasks.ticket_show", verb: []string{"ticket", "show"}, positional: "ticket", required: []string{"ticket"},
			properties:  map[string]any{"ticket": map[string]any{"type": "string", "minLength": 1, "maxLength": 200, "pattern": `^[A-Za-z0-9][A-Za-z0-9:._-]*$`}},
			description: "Show one Corvint Tasks ticket (a full ticketId or a local ID such as V1-0001) with its record, blockers and unknowns, exactly as `corvint-tasks ticket show ID` prints it."},
	}
}

func (tool tool) descriptor() bridge.ToolDescriptor {
	required := make([]any, len(tool.required))
	for index, name := range tool.required {
		required[index] = name
	}
	return bridge.ToolDescriptor{
		Name: tool.name, Description: tool.description,
		InputSchema: map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
			"properties": tool.properties, "required": required, "additionalProperties": false,
		},
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false},
	}
}

// argv turns arguments into the verb's argv, or the -32602 naming the first
// argument that breaks the advertised schema (MCPV0-952 data).
func (tool tool) argv(arguments map[string]any) ([]string, *protocol.RPCError) {
	invalid := func(reason, field, rule string, allowed []any) *protocol.RPCError {
		failure := protocol.InvalidParams("Invalid params")
		failure.Data = protocol.InvalidParamsData{Reason: reason, Field: "arguments." + field, Rule: rule, Allowed: allowed}
		return failure
	}
	declared := slices.Sorted(maps.Keys(tool.properties))
	for _, key := range slices.Sorted(maps.Keys(arguments)) {
		if !slices.Contains(declared, key) {
			allowed := make([]any, len(declared))
			for index, name := range declared {
				allowed[index] = name
			}
			return nil, invalid("unknown-argument", key, "", allowed)
		}
	}
	for _, key := range tool.required {
		if _, present := arguments[key]; !present {
			return nil, invalid("missing-argument", key, "", nil)
		}
	}
	argv := slices.Clone(tool.verb)
	for _, key := range slices.Sorted(maps.Keys(arguments)) {
		schema := tool.properties[key].(map[string]any)
		var value string
		switch schema["type"] {
		case "integer":
			number, err := integerOf(arguments[key])
			if err != nil {
				return nil, invalid("invalid-type", key, "integer", nil)
			}
			minimum := schema["minimum"].(int)
			if number < int64(minimum) {
				return nil, invalid("out-of-range", key, fmt.Sprintf("minimum %d", minimum), nil)
			}
			if maximum, bounded := schema["maximum"].(int); bounded && number > int64(maximum) {
				return nil, invalid("out-of-range", key, fmt.Sprintf("maximum %d", maximum), nil)
			}
			value = strconv.FormatInt(number, 10)
		case "string":
			text, isString := arguments[key].(string)
			if !isString {
				return nil, invalid("invalid-type", key, "string", nil)
			}
			if minimum, _ := schema["minLength"].(int); len([]rune(text)) < minimum {
				return nil, invalid("out-of-range", key, fmt.Sprintf("minLength %d", minimum), nil)
			}
			if maximum, _ := schema["maxLength"].(int); maximum != 0 && len([]rune(text)) > maximum {
				return nil, invalid("out-of-range", key, fmt.Sprintf("maxLength %d", maximum), nil)
			}
			if pattern := schema["pattern"].(string); !regexp.MustCompile(pattern).MatchString(text) {
				return nil, invalid("pattern-mismatch", key, pattern, nil)
			}
			value = text
		}
		if key == tool.positional { // the pattern admits no leading dash
			argv = append(argv, value)
		} else {
			argv = append(argv, "--"+key, value)
		}
	}
	return argv, nil
}

func integerOf(value any) (int64, error) {
	switch typed := value.(type) {
	case json.Number:
		return typed.Int64()
	case float64:
		if typed != float64(int64(typed)) {
			return 0, errors.New("not an integer")
		}
		return int64(typed), nil
	}
	return 0, errors.New("not a number")
}

// Group answers the five tools for one repository root. Run executes one
// read verb there and returns its stdout, or an error when the executable
// cannot run; tests replace it.
type Group struct {
	Root string
	Run  func(ctx context.Context, root string, argv []string) ([]byte, error)
}

// New binds the group to root with the installed corvint-tasks.
func New(root string) Group { return Group{Root: root, Run: runInstalled} }

// Tools lists the five descriptors in name order.
func (group Group) Tools() []bridge.ToolDescriptor {
	var descriptors []bridge.ToolDescriptor
	for _, tool := range tools() {
		descriptors = append(descriptors, tool.descriptor())
	}
	slices.SortFunc(descriptors, func(left, right bridge.ToolDescriptor) int {
		if left.Name < right.Name {
			return -1
		}
		return 1
	})
	return descriptors
}

// Call validates the arguments and runs the tool's verb (TMR-V0-952).
func (group Group) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	index := slices.IndexFunc(tools(), func(tool tool) bool { return tool.name == name })
	if index < 0 {
		return nil, protocol.InvalidParams("Invalid params")
	}
	argv, refusal := tools()[index].argv(arguments)
	if refusal != nil {
		return nil, refusal
	}
	stdout, err := group.Run(ctx, group.Root, argv)
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	if err != nil {
		return Failure(name, "TASKS_UNAVAILABLE", nil)
	}
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	decoder.UseNumber()
	if decoder.Decode(&envelope) != nil || envelope["profile"] != "taskman-command-result/0" {
		return Failure(name, "TASKS_OUTPUT_UNREADABLE", nil)
	}
	if envelope["outcome"] != wire.OutcomeOK {
		return Failure(name, "CLI_REFUSED", map[string]any{"codes": envelope["codes"], "envelope": envelope})
	}
	// Ticket titles and bodies are repository-authored text (AHI-004).
	framed, frameErr := repoenvelope.Frame(string(stdout))
	if frameErr != nil {
		return Failure(name, repoenvelope.CollisionCode, nil)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": envelope,
	}, nil
}

// Failure is the closed tool-error result; extra carries a refused verb's
// codes and envelope, which can carry ticket text and so is framed.
func Failure(name, code string, extra map[string]any) (map[string]any, *protocol.RPCError) {
	value := map[string]any{"code": code, "mutates": false, "profile": ToolErrorProfile, "tool": name}
	maps.Copy(value, extra)
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	text := string(raw)
	if extra != nil {
		framed, frameErr := repoenvelope.Frame(text)
		if frameErr != nil {
			return Failure(name, repoenvelope.CollisionCode, nil)
		}
		text = framed
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": text}},
		"isError":           true,
		"structuredContent": value,
	}, nil
}

// runInstalled runs `corvint-tasks ARGV` in root with the read-only session
// marker, so a verb that would write is refused by the CLI itself (ATR-V0-212).
func runInstalled(ctx context.Context, root string, argv []string) ([]byte, error) {
	executable, err := exec.LookPath(Executable)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, argv...)
	command.Dir = root
	command.Env = append(os.Environ(), "CORVINT_TASKS_READ_ONLY=1")
	var stdout boundedBuffer
	command.Stdout = &stdout
	runErr := command.Run()
	if stdout.overflow {
		return nil, errors.New("output exceeds the bound")
	}
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return nil, runErr
	}
	return stdout.Bytes(), nil
}

type boundedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	if buffer.Len()+len(data) > maxOutputBytes {
		buffer.overflow = true
		return len(data), nil
	}
	return buffer.Buffer.Write(data)
}
