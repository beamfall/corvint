// Command corvint-test-validity-mcp is a separate local stdio MCP 2026-07-28
// server for the experimental MCP test-validity profile
// (docs/specs/mcp-test-validity-profile-v0.md). It exposes one read-only
// projection tool, corvint.test_validity, returning the shared
// internal/testvalidity projection `corvint test-validity` computes, plus the
// binary-only corvint.help tool every Corvint MCP server shares (MCPV0-103). Its
// tool is outside cmd/corvint-mcp's default tool set, which MCPV0-008 freezes
// (decision 0103); `corvint-mcp --tool-profile test-validity` serves the same
// tool through the same internal/mcp/toolgroup code (MMR-V0-013), and this
// binary stays for one release as a deprecated alias (MMR-V0-016). It reuses
// the same transport (internal/mcp/server) and tool-error and cancellation
// conventions.
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
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/procprofile"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

const (
	serverName    = "corvint-test-validity-mcp"
	serverVersion = "0.1.0-experimental"
	toolError     = toolgroup.TestValidityErrorProfile
)

// usage is the --help text (MMR-V0-016). The unified corvint-mcp server serves
// the same tools; this binary stays for one release.
const usage = "usage: corvint-test-validity-mcp --root ROOT [--protocol-version 2026-07-28|2025-11-25]\n" +
	"       corvint-test-validity-mcp --version\n" +
	"A local read-only stdio MCP server for the experimental test-validity tools of the Git worktree at ROOT.\n" +
	"Deprecated alias, kept for one release: corvint-mcp --root ROOT --tool-profile test-validity\n" +
	"serves the same tools with the same results."

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	os.Exit(runProfiled(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.LookupEnv))
}

// runProfiled is run under the CORVINT_CPUPROFILE and CORVINT_MEMPROFILE bracket (PRB-V0-010):
// one profile per server process, written when the server exits and on SIGINT or SIGTERM.
func runProfiled(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer, lookup runtimeenv.Lookup) int {
	return procprofile.Run(ctx, lookup, procprofile.Reporter(stderr, "corvint-test-validity-mcp"), func() int {
		return run(ctx, arguments, stdin, stdout, stderr)
	})
}

func run(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h") { // MMR-V0-016
		_, _ = fmt.Fprintln(stdout, usage)
		return 0
	}
	arguments, protocolVersion, protocolOK := protocol.ExtractVersionArgument(arguments)
	root, versionOnly, ok := parseArguments(arguments)
	if !ok || !protocolOK {
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
		ProtocolVersion: protocolVersion,
		Name:            serverName, Version: serverVersion,
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
		tools := append(handler.registry.Tools(), bridge.HelpDescriptor()) // MCPV0-103
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
	if name == bridge.ToolHelp { // MCPV0-103: reads only the binary, so no repository check applies.
		return helpResult(ctx, name, raw)
	}
	return toolgroup.TestValidity{Registry: handler.registry}.Call(ctx, name, arguments)
}

// helpResult answers corvint.help through the shared bridge path (MCPV0-104).
func helpResult(ctx context.Context, name string, raw []byte) (map[string]any, *protocol.RPCError) {
	return toolgroup.Help(ctx, name, raw, toolFailureResult)
}

// toolFailureResult is this server's closed tool-error result; it always sets isError: true.
func toolFailureResult(name, code, message string) (map[string]any, *protocol.RPCError) {
	return toolgroup.Failure(toolError, name, code, message)
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
