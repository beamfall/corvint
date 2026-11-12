// Package toolgroup is the shared tools/call plumbing of the experimental
// test-validity, source-documentation and documentation-corpus MCP servers: the
// Group contract, the result and tool-error shapes and the root-relative path
// rule, so the dedicated binaries and the unified corvint-mcp server answer
// every call of those tools through the same code (MMR-V0-013, MMR-V0-014).
// Each product bridge defines its own Group beside its registry
// (testvaliditybridge.Group, corpusbridge.Group and corpusbridge.Artifact; the
// docs group is Docs here, because docsbridge only execs the corvint-docs app)
// and keeps its own tool-error profile, so a result is the same object
// whichever server answers it, and this package imports no product.
package toolgroup

import (
	"context"
	"encoding/json"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
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

// Docs answers corvint.docs_draft and corvint.docs_consume for one repository root.
type Docs struct{ Registry *docsbridge.Registry }

func (group Docs) Tools() []bridge.ToolDescriptor { return group.Registry.Tools() }

func (group Docs) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	structured, text, toolFailure, transportErr := group.Registry.Call(ctx, name, raw)
	var transport, tool *Coded
	if transportErr != nil {
		// DMF-V1-010: a failed corvint-docs process keeps its last stderr line.
		transport = &Coded{Code: transportErr.Code, Message: transportErr.Code}
		if transportErr.Message != "" {
			transport.Message, transport.Next = transportErr.Message, transportErr.Next
		}
	}
	if toolFailure != nil {
		tool = &Coded{Code: toolFailure.Code, Message: toolFailure.Message}
	}
	// docs_draft's Markdown and docs_consume's echoed draft reach a model
	// only through this text block, so it alone is framed by the AHI-004
	// envelope; structuredContent stays unwrapped (SDD-V0-006).
	return Answer(ctx, DocsErrorProfile, name, structured, text, transport, tool)
}

// Coded is one coded failure a bridge call reports: a transport error (Code
// doubles as its message unless the bridge kept one, such as a child's last
// stderr line) or a business refusal with its own message; Next is an optional
// next step the bridge knows (DMF-V1-010).
type Coded struct{ Code, Message, Next string }

// Answer maps one bridge call: invalid arguments and unknown tools are invalid
// params, cancellation is the cancelled tool error (DMF-V1-010), other transport
// failures and business refusals are tool errors, and success frames the text
// block.
func Answer(ctx context.Context, profile, name string, structured map[string]any, text string, transport, tool *Coded) (map[string]any, *protocol.RPCError) {
	if transport != nil {
		if transport.Code == "invalid-arguments" || transport.Code == "unsupported-tool" {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if transport.Code == CancelledCode || ctx.Err() != nil {
			return Cancelled(profile, name)
		}
		return FailureNext(profile, name, transport.Code, transport.Message, transport.Next)
	}
	if tool != nil {
		return Failure(profile, name, tool.Code, tool.Message)
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
// sets isError: true; a refusal is never reported as success. It carries
// code, the old error member, and the DMF-V1-010 message, retryable and next.
func Failure(profile, name, code, message string) (map[string]any, *protocol.RPCError) {
	return FailureNext(profile, name, code, message, "")
}

// FailureNext is Failure with a next step the caller knows, which replaces the
// code's default (DMF-V1-010).
func FailureNext(profile, name, code, message, next string) (map[string]any, *protocol.RPCError) {
	value := map[string]any{
		"abstention": map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":       code, "error": message, "mutates": false, "profile": profile, "tool": name,
	}
	if next != "" {
		value["next"] = next
	}
	return FailureResult(value)
}

// FailureResult is the isError result carrying value as both the text block
// and the structured content; a group with its own error object builds it here.
// An object with a code gains message, retryable and next (DMF-V1-010).
func FailureResult(value map[string]any) (map[string]any, *protocol.RPCError) {
	if _, coded := value["code"].(string); coded {
		value = Describe(value)
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
	if err.Code == CancelledCode || ctx.Err() != nil {
		return fail(name, CancelledCode, errorDetails[CancelledCode].message) // DMF-V1-010
	}
	return fail(name, err.Code, err.Code)
}
