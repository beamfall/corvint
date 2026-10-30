// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

func listTools(t *testing.T, handler *toolHandler) []bridge.ToolDescriptor {
	t.Helper()
	result, failure := handler.Handle(context.Background(), protocol.Request{Method: "tools/list", Params: map[string]any{}}, nil)
	if failure != nil {
		t.Fatalf("tools/list: %#v", failure)
	}
	return result["tools"].([]bridge.ToolDescriptor)
}

// TestMCPV0114FollowRefToolsNotReadOnly (V1-3030): under --follow-ref every
// tool a call binds to the ref advertises readOnlyHint false and states the
// checkout and reindex in its description; corvint.help, which never binds
// the ref, and the whole list without --follow-ref keep the read-only
// annotations and descriptions.
func TestMCPV0114FollowRefToolsNotReadOnly(t *testing.T) {
	_, lane := followFixture(t)
	following := followHandler(t, lane, "feature")
	plain := &toolHandler{registry: following.registry}
	before, after := listTools(t, plain), listTools(t, following)
	if len(before) != len(after) || len(before) == 0 {
		t.Fatalf("tool counts %d and %d", len(before), len(after))
	}
	readOnly := bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	for i, tool := range before {
		if tool.Annotations != readOnly {
			t.Fatalf("%s without --follow-ref: %+v", tool.Name, tool.Annotations)
		}
		bound := after[i]
		if bound.Name != tool.Name || !reflect.DeepEqual(bound.InputSchema, tool.InputSchema) {
			t.Fatalf("follow-ref changed %s beyond its annotations and description", tool.Name)
		}
		if tool.Name == bridge.ToolHelp {
			if !reflect.DeepEqual(bound, tool) {
				t.Fatalf("corvint.help changed under --follow-ref: %+v", bound)
			}
			continue
		}
		if bound.Annotations.ReadOnlyHint || bound.Annotations.DestructiveHint || !bound.Annotations.IdempotentHint || bound.Annotations.OpenWorldHint {
			t.Fatalf("%s under --follow-ref: %+v", tool.Name, bound.Annotations)
		}
		if !strings.HasPrefix(bound.Description, tool.Description) || !strings.Contains(bound.Description, "git checkout --detach") || !strings.Contains(bound.Description, "reindex") {
			t.Fatalf("%s description does not state the checkout: %q", tool.Name, bound.Description)
		}
	}
	// The list a later plain handler serves is unchanged by the follow-ref list.
	if again := listTools(t, plain); !reflect.DeepEqual(again, before) {
		t.Fatal("the follow-ref list altered the registry's descriptors")
	}
}
