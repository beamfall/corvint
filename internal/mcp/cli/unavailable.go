package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
)

// invalidRoot is the tool-error code every repository call of an unavailable
// server answers (MCPV0-950).
const invalidRoot = "invalid-root"

// profileTools is each --tool-profile value's root-free tool surface, the
// tools its registry in profileRegistries advertises (MCPV0-950).
var profileTools = map[string]bridge.Profile{
	"": {}, toolProfileTaskReview: {TaskReview: true}, toolProfileTaskReviewLSP: {LSP: true},
	toolProfileFlows: {Flows: true}, toolProfileImportGraph: {ImportGraph: true},
}

// unavailableGroup lists a per-root group's tools for a server whose single
// root is unusable and refuses each call invalid-root under the group's own
// tool-error profile (MCPV0-950, MMR-V0-013).
type unavailableGroup struct {
	tools   func() []bridge.ToolDescriptor
	profile string
	reason  string
}

func (group unavailableGroup) Tools() []bridge.ToolDescriptor { return group.tools() }

func (group unavailableGroup) Call(_ context.Context, name string, _ map[string]any) (map[string]any, *protocol.RPCError) {
	return toolgroup.Failure(group.profile, name, invalidRoot, group.reason)
}

// startUnavailable binds the handler to no repository (MCPV0-950): the server
// still answers initialize and tools/list with the profile's and groups' tools,
// corvint.help still reads the binary, and every other call is an isError
// invalid-root result, so no repository evidence is ever returned. stderr names
// the fixed root failure reason once.
func (handler *toolHandler) startUnavailable(profile string, groups []string, reason string, stderr io.Writer) {
	_, _ = fmt.Fprintf(stderr, "corvint-mcp: repository unavailable: %s; serving tools/list, and every repository tool call answers %s\n", reason, invalidRoot)
	unbound := profileTools[profile]
	handler.unavailable, handler.unbound = reason, &unbound
	handler.groups = map[string][]toolgroup.Group{}
	handler.groupTools = map[string]int{}
	for _, name := range groups {
		var group unavailableGroup
		switch name {
		case toolProfileTestValidity:
			group = unavailableGroup{(&testvaliditybridge.Registry{}).Tools, toolgroup.TestValidityErrorProfile, reason}
		case toolProfileDocs:
			group = unavailableGroup{(&docsbridge.Registry{}).Tools, toolgroup.DocsErrorProfile, reason}
		default:
			continue
		}
		for _, tool := range group.Tools() {
			handler.groupTools[tool.Name] = len(handler.groups[""])
		}
		handler.groups[""] = append(handler.groups[""], group)
	}
}

// answerUnavailable answers a code-tool call of an unavailable server.
func (handler *toolHandler) answerUnavailable(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	if name == bridge.ToolHelp { // MCPV0-100: help reads only the binary.
		raw, err := json.Marshal(arguments)
		if err != nil {
			return nil, protocol.InvalidParams("Invalid params")
		}
		return toolgroup.Help(ctx, name, raw, func(name, code, _ string) (map[string]any, *protocol.RPCError) {
			return handler.toolFailure(name, code, "")
		})
	}
	if !slices.ContainsFunc(handler.unbound.Tools(), func(tool bridge.ToolDescriptor) bool { return tool.Name == name }) {
		return nil, protocol.InvalidParams("Invalid params")
	}
	return handler.toolFailure(name, invalidRoot, "")
}
