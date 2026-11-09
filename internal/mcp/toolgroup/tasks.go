package toolgroup

import (
	"context"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/tasksread"
)

// Tasks answers the read-only Corvint Tasks tools for one repository root
// (TMR-V0-950); every one carries the read-only annotations.
type Tasks struct{ Group tasksread.Group }

func (group Tasks) Tools() []bridge.ToolDescriptor {
	var tools []bridge.ToolDescriptor
	for _, descriptor := range group.Group.Descriptors() {
		tools = append(tools, bridge.ToolDescriptor{
			Name: descriptor.Name, Description: descriptor.Description, InputSchema: descriptor.InputSchema,
			Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false},
		})
	}
	return tools
}

func (group Tasks) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	return group.Group.Call(ctx, name, arguments)
}
