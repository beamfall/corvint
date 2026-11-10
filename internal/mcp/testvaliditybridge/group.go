// SPDX-License-Identifier: Apache-2.0

package testvaliditybridge

import (
	"context"
	"encoding/json"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
)

// Group answers corvint.test_validity for one repository root as a
// toolgroup.Group, so corvint-test-validity-mcp and the unified corvint-mcp
// server return the same object (MMR-V0-013).
type Group struct{ Registry *Registry }

func (group Group) Tools() []bridge.ToolDescriptor { return group.Registry.Tools() }

func (group Group) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	structured, text, toolFailure, transportErr := group.Registry.Call(ctx, name, raw)
	var transport, tool *toolgroup.Coded
	if transportErr != nil {
		transport = &toolgroup.Coded{Code: transportErr.Code, Message: transportErr.Code}
	}
	if toolFailure != nil {
		tool = &toolgroup.Coded{Code: toolFailure.Code, Message: toolFailure.Message}
	}
	// Test names and anchors are repository-derived text that reaches a model
	// through this block, so it alone is framed by the AHI-004 envelope.
	return toolgroup.Answer(ctx, toolgroup.TestValidityErrorProfile, name, structured, text, transport, tool)
}
