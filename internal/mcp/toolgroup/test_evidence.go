// SPDX-License-Identifier: Apache-2.0
package toolgroup

import (
	"context"
	"encoding/json"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/testevidencedoc"
)

const TestEvidenceTool = "corvint.test_evidence"
const TestEvidenceErrorProfile = "corvint-test-evidence-mcp-tool-error/1"

// TestEvidence is an opt-in group so existing profiles keep their frozen tools.
type TestEvidence struct {
	Root    string
	Binding *rootbind.Binding
}

func (g TestEvidence) Tools() []bridge.ToolDescriptor {
	props := map[string]any{}
	for _, s := range []string{"path", "lines", "test", "explain", "revision", "base", "index"} {
		props[s] = map[string]any{"type": "string", "minLength": 1, "maxLength": 4096}
	}
	return []bridge.ToolDescriptor{{Name: TestEvidenceTool, Description: "Read existing test evidence for path (optional lines START:END), test, or explain with base; revision defaults to HEAD. Missing inventory/context abstains UNKNOWN; unqualified runners are FULL. Same projection as corvint test-evidence; never collects or writes.", Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}, InputSchema: map[string]any{"type": "object", "properties": props, "additionalProperties": false}}}
}
func (g TestEvidence) Call(ctx context.Context, name string, args map[string]any) (map[string]any, *protocol.RPCError) {
	if name != TestEvidenceTool {
		return nil, protocol.InvalidParams("Invalid params")
	}
	argv := []string{}
	// Fixed ordering keeps JSON map iteration from changing validation.
	for key, value := range args {
		if _, ok := value.(string); !ok || key != "path" && key != "lines" && key != "test" && key != "explain" && key != "revision" && key != "base" && key != "index" {
			return nil, protocol.InvalidParams("Invalid params")
		}
	}
	for _, key := range []string{"revision", "base", "index", "test", "explain", "lines"} {
		if v, ok := args[key].(string); ok {
			argv = append(argv, "--"+key, v)
		}
	}
	if v, ok := args["path"].(string); ok {
		// The terminator keeps a path that starts with "-" from parsing as an option.
		argv = append(argv, "--", v)
	}
	q, _, err := testevidencedoc.Parse(argv)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if g.Binding == nil || !g.Binding.Same(g.Root) {
		return Failure(TestEvidenceErrorProfile, name, "invalid-root", "repository unavailable")
	}
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	d := testevidencedoc.Read(ctx, g.Root, q)
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	var structured map[string]any
	if json.Unmarshal(raw, &structured) != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	return answer(ctx, TestEvidenceErrorProfile, name, structured, string(raw), nil, nil)
}
