package cli

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// NGI-V0-305: a separate profile preserves every pre-existing tools/list wire.
func TestImportGraphMCPProfile(t *testing.T) {
	root := makeHostileRepository(t, "graph fixture")
	want := []string{bridge.ToolGraphImports, bridge.ToolHelp, bridge.ToolImpact, bridge.ToolQuery, bridge.ToolStatus}
	for _, version := range []string{"2026-07-28", "2025-11-25"} {
		args := []string{"--root", root, "--tool-profile", "import-graph", "--protocol-version", version}
		if got := listedToolNames(t, args); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: tools=%v", version, got)
		}
	}
	for _, args := range [][]string{
		{"--root", root, "--tool-profile", "IMPORT-GRAPH"},
		{"--root", root, "--tool-profile", "import-graph", "--tool-profile", "task-review"},
	} {
		var out, diagnostic bytes.Buffer
		if code := run(t.Context(), args, bytes.NewReader(nil), &out, &diagnostic); code != 2 || out.Len() != 0 {
			t.Fatalf("%v: %d %s %s", args, code, &out, &diagnostic)
		}
	}
	registry, err := profileRegistries[toolProfileImportGraph](root)
	if err != nil {
		t.Fatal(err)
	}
	revision := gitTest(t, root, "rev-parse", "HEAD")
	handler := &toolHandler{registry: registry}
	result, failure := handler.call(t.Context(), map[string]any{"name": bridge.ToolGraphImports, "arguments": map[string]any{"revision": revision}})
	if failure != nil || result["isError"] == true {
		t.Fatalf("call: %#v %v", result, failure)
	}
	structured := result["structuredContent"].(map[string]any)
	receipt := structured["receipt"].(map[string]any)
	if receipt["revision"] != revision || receipt["schema"] != "graph-imports/1" || receipt["done"] != true {
		t.Fatalf("receipt: %#v", receipt)
	}
}
