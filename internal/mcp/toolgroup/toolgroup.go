// Package toolgroup holds the tools/call bodies of the experimental
// test-validity, source-documentation and documentation-corpus MCP servers, so
// the dedicated binaries and the unified corvint-mcp server answer every call
// of those tools through the same code (MMR-V0-013, MMR-V0-014). Each group
// keeps its own tool-error profile, so a result is the same object whichever
// server answers it.
package toolgroup

import (
	"context"
	"encoding/json"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// The tool-error profiles of the dedicated servers, kept by each group.
const (
	TestValidityErrorProfile = "corvint-test-validity-mcp-tool-error/0"
	DocsErrorProfile         = "corvint-docs-mcp-tool-error/0"
	CorpusErrorProfile       = "corvint-corpus-mcp-tool-error/0"
)

// Group is one set of tools a server lists and answers. Call receives the
// validated tools/call arguments object and returns the MCP result, or a
// JSON-RPC error for invalid parameters and cancellation.
type Group interface {
	Tools() []bridge.ToolDescriptor
	Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError)
}

// TestValidity answers corvint.test_validity for one repository root.
type TestValidity struct{ Registry *testvaliditybridge.Registry }

func (group TestValidity) Tools() []bridge.ToolDescriptor { return group.Registry.Tools() }

func (group TestValidity) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	structured, text, toolFailure, transportErr := group.Registry.Call(ctx, name, raw)
	var transport, tool *coded
	if transportErr != nil {
		transport = &coded{transportErr.Code, transportErr.Code}
	}
	if toolFailure != nil {
		tool = &coded{toolFailure.Code, toolFailure.Message}
	}
	// Test names and anchors are repository-derived text that reaches a model
	// through this block, so it alone is framed by the AHI-004 envelope.
	return answer(ctx, TestValidityErrorProfile, name, structured, text, transport, tool)
}

// Docs answers corvint.docs_draft and corvint.docs_consume for one repository root.
type Docs struct{ Registry *docsbridge.Registry }

func (group Docs) Tools() []bridge.ToolDescriptor { return group.Registry.Tools() }

func (group Docs) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	structured, text, toolFailure, transportErr := group.Registry.Call(ctx, name, raw)
	var transport, tool *coded
	if transportErr != nil {
		transport = &coded{transportErr.Code, transportErr.Code}
	}
	if toolFailure != nil {
		tool = &coded{toolFailure.Code, toolFailure.Message}
	}
	// docs_draft's Markdown and docs_consume's echoed draft reach a model
	// only through this text block, so it alone is framed by the AHI-004
	// envelope; structuredContent stays unwrapped (SDD-V0-006).
	return answer(ctx, DocsErrorProfile, name, structured, text, transport, tool)
}

type coded struct{ code, message string }

// answer maps one bridge call: invalid arguments and unknown tools are invalid
// params, cancellation is an internal error, other transport failures and
// business refusals are tool errors, and success frames the text block.
func answer(ctx context.Context, profile, name string, structured map[string]any, text string, transport, tool *coded) (map[string]any, *protocol.RPCError) {
	if transport != nil {
		if transport.code == "invalid-arguments" || transport.code == "unsupported-tool" {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if transport.code == "cancelled" || ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		return Failure(profile, name, transport.code, transport.message)
	}
	if tool != nil {
		return Failure(profile, name, tool.code, tool.message)
	}
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return Failure(profile, name, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}, nil
}

// Failure is the closed tool-error result of the named profile. It always
// sets isError: true; a refusal is never reported as success.
func Failure(profile, name, code, message string) (map[string]any, *protocol.RPCError) {
	return failureResult(map[string]any{
		"abstention": map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":       code, "error": message, "mutates": false, "profile": profile, "tool": name,
	})
}

func failureResult(value map[string]any) (map[string]any, *protocol.RPCError) {
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

// Help answers corvint.help through the shared bridge path (MCPV0-104),
// mapping its failures as every other tool's transport failure is mapped,
// with the tool-error object fail builds.
func Help(ctx context.Context, name string, raw []byte, fail func(name, code, message string) (map[string]any, *protocol.RPCError)) (map[string]any, *protocol.RPCError) {
	result, err := bridge.HelpToolResult(ctx, raw)
	if err == nil {
		return result, nil
	}
	if err.Code == "invalid-arguments" {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if err.Code == "cancelled" || ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	return fail(name, err.Code, err.Code)
}
