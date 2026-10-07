package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"

	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const (
	serverName    = "corvint-corpus-mcp"
	serverVersion = "0.1.0-experimental"
	toolError     = "corvint-corpus-mcp-tool-error/0"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	arguments, protocolVersion, protocolOK := protocol.ExtractVersionArgument(arguments)
	root, artifact, maps, versionOnly, ok := parseArguments(arguments)
	if !ok || !protocolOK {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: invalid arguments")
		return 2
	}
	if versionOnly {
		_, _ = fmt.Fprintln(stdout, serverName+" "+serverVersion)
		return 0
	}
	registry, registryErr := corpusbridge.New(root, artifact)
	if registryErr != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: repository unavailable")
		return 2
	}
	planner, plannerErr := newMapPlanner(root, maps)
	if plannerErr != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: application map unavailable")
		return 2
	}
	handler := &toolHandler{registry: registry, planner: planner}
	instance, err := server.New(server.Config{
		ProtocolVersion: protocolVersion,
		Name:            serverName, Version: serverVersion,
		Description:  "Local read-only Corvint experimental documentation corpus server.",
		Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}},
		Handler:      handler,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: startup failed")
		return 2
	}
	if err := instance.Serve(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, "corvint-corpus-mcp: transport failed")
		return 2
	}
	return 0
}

// parseArguments accepts --version, or --root R --artifact A followed by up to eight --map FILE
// pairs naming root-relative application maps for corvint.map_plan (AMSP-V0-010).
func parseArguments(arguments []string) (root, artifact string, maps []string, versionOnly, ok bool) {
	if len(arguments) == 1 && arguments[0] == "--version" {
		return "", "", nil, true, true
	}
	if len(arguments) < 4 || len(arguments)%2 != 0 || arguments[0] != "--root" || arguments[1] == "" || arguments[2] != "--artifact" || arguments[3] == "" {
		return "", "", nil, false, false
	}
	for i := 4; i < len(arguments); i += 2 {
		if arguments[i] != "--map" || arguments[i+1] == "" {
			return "", "", nil, false, false
		}
		maps = append(maps, arguments[i+1])
	}
	if len(maps) > maxConfiguredMaps {
		return "", "", nil, false, false
	}
	return arguments[1], arguments[3], maps, false, true
}

type toolHandler struct {
	registry *corpusbridge.Registry
	planner  *mapPlanner
}

func (handler *toolHandler) Handle(ctx context.Context, request protocol.Request, _ server.Notifier) (map[string]any, *protocol.RPCError) {
	switch request.Method {
	case "tools/list":
		if !onlyKeys(request.Params, "_meta", "cursor") {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if cursor, present := request.Params["cursor"]; present {
			if value, valid := cursor.(string); !valid || value != "" {
				return nil, protocol.InvalidParams("Invalid params")
			}
		}
		tools := handler.registry.Tools()
		if handler.planner != nil {
			tools = append(tools, handler.planner.descriptor())
		}
		sort.Slice(tools, func(left, right int) bool { return tools[left].Name < tools[right].Name })
		return map[string]any{
			"cacheScope": "private", "tools": tools, "ttlMs": 300_000,
		}, nil
	case "tools/call":
		return handler.call(ctx, request.Params)
	default:
		return nil, protocol.MethodNotFound()
	}
}

func (handler *toolHandler) call(ctx context.Context, params map[string]any) (map[string]any, *protocol.RPCError) {
	if !onlyKeys(params, "_meta", "name", "arguments") {
		return nil, protocol.InvalidParams("Invalid params")
	}
	name, nameOK := params["name"].(string)
	arguments := map[string]any{}
	if rawArguments, present := params["arguments"]; present {
		var argumentsOK bool
		arguments, argumentsOK = rawArguments.(map[string]any)
		if !argumentsOK {
			return nil, protocol.InvalidParams("Invalid params")
		}
	}
	if !nameOK || name == "" {
		return nil, protocol.InvalidParams("Invalid params")
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if name == mapPlanTool && handler.planner != nil {
		return handler.callMapPlan(ctx, raw)
	}
	structured, text, toolFailure, transportErr := handler.registry.Call(ctx, name, raw)
	if transportErr != nil {
		if transportErr.Code == "invalid-arguments" || transportErr.Code == "unsupported-tool" {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if transportErr.Code == "cancelled" || ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		return toolFailureResult(name, transportErr.Code, transportErr.Code)
	}
	if toolFailure != nil {
		return toolFailureResult(name, toolFailure.Code, toolFailure.Message)
	}
	// Model-facing evidence uses the shared untrusted-data envelope; the
	// structured native receipt remains unchanged.
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return toolFailureResult(name, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}, nil
}

// callMapPlan frames the plan like every other tool result: the model-facing text in the
// untrusted-data envelope, the plan itself as structured content.
func (handler *toolHandler) callMapPlan(ctx context.Context, raw []byte) (map[string]any, *protocol.RPCError) {
	text, code, message, valid := handler.planner.call(ctx, raw)
	if !valid {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	if code != "" {
		return toolFailureResult(mapPlanTool, code, message)
	}
	if text == "" {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	var structured map[string]any
	if json.Unmarshal([]byte(text), &structured) != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return toolFailureResult(mapPlanTool, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}, nil
}

// toolFailureResult always sets isError: true. A business or transport
// refusal from the native docs compiler must never be reported as success.
func toolFailureResult(name, code, message string) (map[string]any, *protocol.RPCError) {
	value := map[string]any{
		"trust_envelope": map[string]any{"content_status": "unknown", "source_revision": "unknown", "corpus_revision": "unknown", "freshness": "unknown", "retirement": "unknown", "citations": []any{}, "limitations": []string{"operation failed; no evidence answer"}},
		"abstention":     map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":           code, "error": message, "mutates": false, "profile": toolError, "tool": name,
	}
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

func onlyKeys(params map[string]any, allowed ...string) bool {
	if params == nil {
		return false
	}
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range params {
		if _, exists := set[key]; !exists {
			return false
		}
	}
	return true
}
