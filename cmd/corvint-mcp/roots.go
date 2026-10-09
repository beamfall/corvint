package main

import (
	"context"
	"maps"
	"slices"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/rootalias"
)

const (
	// maxRoots caps the repositories one process may declare (MMR-V0-002).
	maxRoots = rootalias.MaxRoots
	// repositoryArgument is the one tool argument multi-root mode adds (MMR-V0-004).
	repositoryArgument = "repository"
	// multiRootStatusSchema is the all-repositories status object (MMR-V0-006).
	multiRootStatusSchema = "corvint-mcp-multi-root-status/0"
)

// rootDeclaration is one --root value: a plain ABSOLUTE_ROOT, whose alias is
// empty, or ALIAS=ABSOLUTE_ROOT (MMR-V0-001), read by rootalias.Split.
type rootDeclaration struct{ alias, root string }

// openRepositories validates and pins every declared root at startup with the
// single-root MCPV0-001 rules (MMR-V0-003). A root that fails is "repository
// unavailable"; a directory declared twice, under any spelling, is refused as an
// invalid argument. The returned aliases are sorted.
func openRepositories(roots []rootDeclaration, open func(string) (*bridge.Registry, *bridge.Error)) (map[string]*bridge.Registry, []string, string) {
	repositories := make(map[string]*bridge.Registry, len(roots))
	for _, declared := range roots {
		registry, err := open(declared.root)
		if err != nil {
			return nil, nil, "repository unavailable"
		}
		for _, other := range repositories {
			if registry.SharesRoot(other) {
				return nil, nil, "invalid arguments"
			}
		}
		repositories[declared.alias] = registry
	}
	return repositories, slices.Sorted(maps.Keys(repositories)), ""
}

// multiRootTools adds the required repository alias enum to every tool, and
// the optional one to corvint.status, whose omission reports every binding
// (MMR-V0-004, MMR-V0-006). Every registry has the same profile, so the first
// alias's descriptors stand for all of them; each call builds fresh schemas.
func (handler *toolHandler) multiRootTools() []bridge.ToolDescriptor {
	enum := make([]any, len(handler.aliases))
	for index, alias := range handler.aliases {
		enum[index] = alias
	}
	tools := handler.repositories[handler.aliases[0]].Tools()
	for _, tool := range tools {
		tool.InputSchema["properties"].(map[string]any)[repositoryArgument] = map[string]any{"type": "string", "enum": enum}
		if tool.Name != bridge.ToolStatus {
			required, _ := tool.InputSchema["required"].([]any)
			tool.InputSchema["required"] = append(required, repositoryArgument)
		}
	}
	return tools
}

// route binds a multi-root call to the registry its declared alias names and
// removes the selector before the bridge's closed decode (MMR-V0-005). A
// missing, non-string, or undeclared alias is not ok; a path is never an alias.
// A nil registry with ok means corvint.status without an alias, which, like
// the closed status input, admits no other argument.
func (handler *toolHandler) route(name string, arguments map[string]any) (*bridge.Registry, map[string]any, bool) {
	value, present := arguments[repositoryArgument]
	if !present {
		return nil, nil, name == bridge.ToolStatus && len(arguments) == 0
	}
	alias, isString := value.(string)
	registry := handler.repositories[alias]
	if !isString || registry == nil {
		return nil, nil, false
	}
	remaining := maps.Clone(arguments)
	delete(remaining, repositoryArgument)
	return registry, remaining, true
}

// statusAll reports every declared binding in alias order (MMR-V0-006). Each
// entry's result is exactly the structuredContent of a corvint.status call
// naming that alias, probed from that alias's registry alone.
func (handler *toolHandler) statusAll(ctx context.Context) (map[string]any, *protocol.RPCError) {
	entries := make([]any, 0, len(handler.aliases))
	for _, alias := range handler.aliases {
		result, bridgeErr := handler.repositories[alias].Call(ctx, bridge.ToolStatus, []byte("{}"))
		var entry map[string]any
		if bridgeErr != nil {
			if bridgeErr.Code == "cancelled" || ctx.Err() != nil {
				return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
			}
			entry = handler.toolErrorObject(bridge.ToolStatus, bridgeErr.Code, bridgeErr.ReasonClass)
		} else {
			object, objectErr := result.Object()
			if objectErr != nil {
				return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
			}
			entry = object
		}
		entries = append(entries, map[string]any{repositoryArgument: alias, "result": entry})
	}
	value := map[string]any{
		"schema": multiRootStatusSchema, "tool": bridge.ToolStatus, "mutates": false, "repositories": entries,
	}
	raw, err := gokernel.CanonicalJSON(value)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(string(raw))
	if frameErr != nil {
		return handler.toolFailure(bridge.ToolStatus, repoenvelope.CollisionCode, "")
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": value,
	}, nil
}
