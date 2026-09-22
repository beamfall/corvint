package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const (
	serverName    = "corvint-mcp"
	serverVersion = "0.1.0-experimental"
	toolError     = "corvint-mcp-tool-error/0"

	// untrustedDataPrefix and untrustedDataSuffix are the internal/repoenvelope
	// envelope the host adapters apply to repository-authored free text.
	// Repository-authored title, summary, and evidence-reason fields reach a
	// model through this "text" content block, so it is framed by the same
	// builder (AHI-004); structuredContent stays unwrapped for programmatic callers.
	untrustedDataPrefix = repoenvelope.Prefix
	untrustedDataSuffix = repoenvelope.Suffix
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root, versionOnly, ok := parseArguments(arguments)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: invalid arguments")
		return 2
	}
	if versionOnly {
		_, _ = fmt.Fprintln(stdout, serverName+" "+serverVersion)
		return 0
	}
	registry, registryErr := bridge.New(root)
	if registryErr != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: repository unavailable")
		return 2
	}
	handler := &toolHandler{registry: registry}
	instance, err := server.New(server.Config{
		Name: serverName, Version: serverVersion,
		Description:  "Local read-only Corvint context and repository evidence server.",
		Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}},
		Handler:      handler,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: startup failed")
		return 2
	}
	if err := instance.Serve(ctx, stdin, stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: transport failed")
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

type toolHandler struct{ registry *bridge.Registry }

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
	result, bridgeErr := handler.registry.Call(ctx, name, raw)
	if bridgeErr != nil {
		if bridgeErr.Code == "invalid-arguments" || bridgeErr.Code == "unsupported-tool" {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if bridgeErr.Code == "cancelled" || ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		return toolFailure(name, bridgeErr.Code)
	}
	structured, text, err := structuredResult(result)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return toolFailure(name, repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}, nil
}

func toolFailure(name, code string) (map[string]any, *protocol.RPCError) {
	value := map[string]any{
		"abstention": map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":       code, "mutates": false, "profile": toolError, "tool": name,
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

func structuredResult(result bridge.Result) (map[string]any, string, error) {
	structured, objectErr := result.Object()
	if objectErr != nil {
		return nil, "", objectErr
	}
	raw, canonicalErr := result.CanonicalJSON()
	if canonicalErr != nil {
		return nil, "", canonicalErr
	}
	return structured, string(raw), nil
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
