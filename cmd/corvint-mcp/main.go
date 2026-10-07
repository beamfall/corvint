package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/rootalias"
)

const (
	serverName    = "corvint-mcp"
	serverVersion = "0.1.0-experimental"
	toolError     = "corvint-mcp-tool-error/0"

	// toolErrorReasonClass is the tool-error object under the closed
	// --error-profile reason-class selector (MCPV0-027, decision 0383).
	toolErrorReasonClass = "corvint-mcp-tool-error/1"
	errorProfileReason   = "reason-class"
	unclassified         = "unclassified"

	// toolProfileTaskReview and toolProfileFlows are the values of the closed
	// --tool-profile selector (MCPV0-026, decision 0374, amended by AFU-V1-034).
	toolProfileTaskReview    = "task-review"
	toolProfileTaskReviewLSP = "task-review-lsp"
	toolProfileFlows         = "flows"

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
	arguments, protocolVersion, protocolOK := protocol.ExtractVersionArgument(arguments)
	arguments, profile, profileOK := extractToolProfile(arguments)
	arguments, reasonClass, errorProfileOK := extractErrorProfile(arguments)
	roots, versionOnly, ok := parseArguments(arguments)
	if !ok || !protocolOK || !profileOK || !errorProfileOK {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: invalid arguments")
		return 2
	}
	if versionOnly {
		_, _ = fmt.Fprintln(stdout, serverName+" "+serverVersion)
		return 0
	}
	git, err := gitstatus.Pin()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "corvint-mcp: git unavailable")
		return 2
	}
	// The CEM seams spawn Git through their own runner; pin it to the same path.
	gitrun.PinBinary(git)
	handler := &toolHandler{reasonClass: reasonClass}
	if len(roots) == 1 && roots[0].alias == "" {
		registry, registryErr := profileRegistries[profile](roots[0].root)
		if registryErr != nil {
			_, _ = fmt.Fprintln(stderr, "corvint-mcp: repository unavailable")
			return 2
		}
		handler.registry = registry
	} else {
		var refusal string
		handler.repositories, handler.aliases, refusal = openRepositories(roots, profileRegistries[profile])
		if refusal != "" {
			_, _ = fmt.Fprintln(stderr, "corvint-mcp: "+refusal)
			return 2
		}
	}
	instance, err := server.New(server.Config{
		ProtocolVersion: protocolVersion,
		Name:            serverName, Version: serverVersion,
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

// profileRegistries binds each --tool-profile value; the empty value is the
// default V0 registry.
var profileRegistries = map[string]func(string) (*bridge.Registry, *bridge.Error){
	"": bridge.New, toolProfileTaskReview: bridge.NewTaskReview, toolProfileTaskReviewLSP: bridge.NewTaskReviewLSP, toolProfileFlows: bridge.NewFlows,
}

// extractToolProfile removes the optional, closed descendant-profile selector
// (MCPV0-026). A missing, duplicate, or unknown value, or the selector beside
// --version, fails before repository startup; omission keeps the V0 tools.
func extractToolProfile(arguments []string) (remaining []string, profile string, ok bool) {
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--tool-profile" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if profile != "" || index+1 == len(arguments) || arguments[index+1] == "" || profileRegistries[arguments[index+1]] == nil {
			return nil, "", false
		}
		profile = arguments[index+1]
		index++
	}
	if profile != "" && slices.Contains(remaining, "--version") {
		return nil, "", false
	}
	return remaining, profile, true
}

// extractErrorProfile removes the optional, closed tool-error selector
// (MCPV0-027) under the same rules as extractToolProfile; omission keeps
// corvint-mcp-tool-error/0.
func extractErrorProfile(arguments []string) (remaining []string, reasonClass bool, ok bool) {
	remaining = make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		if arguments[index] != "--error-profile" {
			remaining = append(remaining, arguments[index])
			continue
		}
		if reasonClass || index+1 == len(arguments) || arguments[index+1] != errorProfileReason {
			return nil, false, false
		}
		reasonClass = true
		index++
	}
	if reasonClass && slices.Contains(remaining, "--version") {
		return nil, false, false
	}
	return remaining, reasonClass, true
}

// parseArguments reads `--version` alone, one plain `--root ABSOLUTE_ROOT`
// (MCPV0-001), or 1 to maxRoots `--root ALIAS=ABSOLUTE_ROOT` declarations with
// distinct aliases (MMR-V0-001, MMR-V0-002). A plain root never shares argv
// with another root.
func parseArguments(arguments []string) (roots []rootDeclaration, versionOnly bool, ok bool) {
	if len(arguments) == 1 && arguments[0] == "--version" {
		return nil, true, true
	}
	if len(arguments) == 0 || len(arguments)%2 != 0 || len(arguments) > 2*maxRoots {
		return nil, false, false
	}
	aliases := make(map[string]bool, len(arguments)/2)
	for index := 0; index < len(arguments); index += 2 {
		if arguments[index] != "--root" || arguments[index+1] == "" {
			return nil, false, false
		}
		alias, root, aliased := rootalias.Split(arguments[index+1])
		if (!aliased && len(arguments) != 2) || (aliased && (root == "" || aliases[alias])) {
			return nil, false, false
		}
		aliases[alias] = true
		roots = append(roots, rootDeclaration{alias: alias, root: root})
	}
	return roots, false, true
}

type toolHandler struct {
	registry    *bridge.Registry
	reasonClass bool
	// repositories and aliases bind the declared roots in multi-root mode
	// (MMR-V0); both are nil under the single-root contract.
	repositories map[string]*bridge.Registry
	aliases      []string
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
		var tools []bridge.ToolDescriptor
		if handler.repositories != nil {
			tools = handler.multiRootTools()
		} else {
			tools = handler.registry.Tools()
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
	registry := handler.registry
	if handler.repositories != nil {
		var routed bool
		if registry, arguments, routed = handler.route(name, arguments); !routed {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if registry == nil {
			return handler.statusAll(ctx)
		}
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	result, bridgeErr := registry.Call(ctx, name, raw)
	if bridgeErr != nil {
		if bridgeErr.Code == "invalid-arguments" || bridgeErr.Code == "unsupported-tool" {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if bridgeErr.Code == "cancelled" || ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		return handler.toolFailure(name, bridgeErr.Code, bridgeErr.ReasonClass)
	}
	structured, text, err := structuredResult(result)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return handler.toolFailure(name, repoenvelope.CollisionCode, "")
	}
	response := map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}
	// Flow intents carry repository-authored step text, which reaches the
	// caller only inside the envelope (AFU-V1-035).
	if bridge.EnvelopeOnly(name) {
		delete(response, "structuredContent")
	}
	return response, nil
}

// toolFailure is the closed tool-error object: profile /0 by default, or /1
// with a required reasonClass under the reason-class selector (MCPV0-028).
func (handler *toolHandler) toolFailure(name, code, reasonClass string) (map[string]any, *protocol.RPCError) {
	value := handler.toolErrorObject(name, code, reasonClass)
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

func (handler *toolHandler) toolErrorObject(name, code, reasonClass string) map[string]any {
	value := map[string]any{
		"abstention": map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":       code, "mutates": false, "profile": toolError, "tool": name,
	}
	if handler.reasonClass {
		value["profile"] = toolErrorReasonClass
		value["reasonClass"] = cmp.Or(reasonClass, unclassified)
	}
	return value
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
