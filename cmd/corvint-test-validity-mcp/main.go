// Command corvint-test-validity-mcp is a separate local stdio MCP 2026-07-28
// server for the experimental MCP test-validity profile
// (docs/specs/mcp-test-validity-profile-v0.md). It exposes exactly one
// read-only tool, corvint.test_validity, returning the shared
// internal/testvalidity projection `corvint test-validity` computes. It is
// intentionally not part of cmd/corvint-mcp, whose tool set MCPV0-008 freezes
// (decision 0103), nor of cmd/corvint-docs-mcp. It reuses the same transport
// (internal/mcp/server) and tool-error and cancellation conventions.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const (
	serverName    = "corvint-test-validity-mcp"
	serverVersion = "0.1.0-experimental"
	toolError     = "corvint-test-validity-mcp-tool-error/0"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root, versionOnly, ok := parseArguments(arguments)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "corvint-test-validity-mcp: invalid arguments")
		return 2
	}
	if versionOnly {
		_, _ = fmt.Fprintln(stdout, serverName+" "+serverVersion)
		return 0
	}
	registry, registryErr := testvaliditybridge.New(root)
	if registryErr != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-test-validity-mcp: repository unavailable")
		return 2
	}
	handler := &toolHandler{registry: registry}
	instance, err := server.New(server.Config{
		Name: serverName, Version: serverVersion,
		Description:  "Local read-only Corvint experimental test-validity projection server.",
		Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}},
		Handler:      handler,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-test-validity-mcp: startup failed")
		return 2
	}
	if err := instance.Serve(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, "corvint-test-validity-mcp: transport failed")
		return 2
	}
	return 0
}

func parseArguments(arguments []string) (root string, versionOnly bool, ok bool) {
	if len(arguments) == 1 && arguments[0] == "--version" {
		return "", true, true
	}
	if len(arguments) != 2 || arguments[0] != "--root" || arguments[1] == "" {
		return "", false, false
	}
	return arguments[1], false, true
}

type toolHandler struct{ registry *testvaliditybridge.Registry }

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
	// Test names and anchors are repository-derived text that reaches a model
	// through this block, so it alone is framed by the AHI-004 envelope;
	// structuredContent stays unwrapped, as in corvint-docs-mcp.
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

// toolFailureResult always sets isError: true. A refusal must never be
// reported as success.
func toolFailureResult(name, code, message string) (map[string]any, *protocol.RPCError) {
	value := map[string]any{
		"abstention": map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":       code, "error": message, "mutates": false, "profile": toolError, "tool": name,
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
