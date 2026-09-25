package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// AFU-V1-034: each flows MCP tool returns, as its receipt, the same document its CLI verb writes.
func TestAFUV1034FlowsToolsMatchCLIVerbs(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the repository probe is qualified only on Darwin and Linux")
	}
	shop := newShopFixture(t)
	shop.root = resolvedRoot(t, shop.root)
	registry := flowsRegistry(t, shop.root)
	// Impact runs before an evidence copy makes the worktree differ from HEAD.
	assertFlowsParity(t, shop.root, registry, bridge.ToolFlowsImpact, map[string]any{"flows": "flows", "base": shop.b}, "impact", "--flows", "flows", "--base", shop.b)
	evidence := copyIntoRoot(t, shop.root, shop.evidence, "runs/runs.jsonl")
	assertFlowsParity(t, shop.root, registry, bridge.ToolFlowsMap, map[string]any{"flows": "flows", "evidence": []any{"runs/runs.jsonl"}}, "map", "--flows", "flows", "--evidence", evidence)
	assertFlowsParity(t, shop.root, registry, bridge.ToolFlowsMap, map[string]any{"flows": "flows", "path": "profile/profile.go"}, "map", "--flows", "flows", "--path", "profile/profile.go")
	assertFlowsParity(t, shop.root, registry, bridge.ToolFlowsMap, map[string]any{"flows": "flows", "testKey": "wishlist.spec.ts > adds"}, "map", "--flows", "flows", "--test-key", "wishlist.spec.ts > adds")
	assertFlowsParity(t, shop.root, registry, bridge.ToolFlowsGaps, map[string]any{"flows": "flows", "evidence": []any{"runs/runs.jsonl"}}, "gaps", "--flows", "flows", "--evidence", evidence)

	nav := newNavFixture(t)
	nav.root = resolvedRoot(t, nav.root)
	registry = flowsRegistry(t, nav.root)
	evidence = copyIntoRoot(t, nav.root, nav.evidence, "runs/runs.jsonl")
	traffic := copyIntoRoot(t, nav.root, nav.traffic, "runs/traffic.jsonl")
	files := map[string]any{"flows": "flows", "evidence": []any{"runs/runs.jsonl"}, "traffic": []any{"runs/traffic.jsonl"}}
	assertFlowsParity(t, nav.root, registry, bridge.ToolFlowsNavigate, files, "navigate", "--flows", "flows", "--evidence", evidence, "--traffic", traffic)
	packet := map[string]any{"goal": "checkout", "maxEffect": "write-irreversible"}
	for key, value := range files {
		packet[key] = value
	}
	assertFlowsParity(t, nav.root, registry, bridge.ToolFlowsNavigate, packet, "navigate", "--flows", "flows", "--evidence", evidence, "--traffic", traffic, "--goal", "checkout", "--max-effect", "write-irreversible")

	// The CLI's argument refusals are the tools' invalid-arguments; a verb refusal is flows-refused.
	for name, arguments := range map[string]map[string]any{
		"map path and testKey":      {"flows": "flows", "path": "a", "testKey": "b"},
		"map lookup with evidence":  {"flows": "flows", "path": "a", "evidence": []any{"runs/runs.jsonl"}},
		"navigate maxEffect only":   {"flows": "flows", "maxEffect": "read"},
		"navigate unknown effect":   {"flows": "flows", "goal": "checkout", "maxEffect": "delete"},
		"navigate absolute traffic": {"flows": "flows", "traffic": []any{traffic}},
		"navigate git metadata":     {"flows": ".git"},
		"navigate unknown member":   {"flows": "flows", "root": "/"},
	} {
		raw, _ := json.Marshal(arguments)
		tool := bridge.ToolFlowsNavigate
		if name[:3] == "map" {
			tool = bridge.ToolFlowsMap
		}
		if _, err := registry.Call(context.Background(), tool, raw); err == nil || err.Code != "invalid-arguments" {
			t.Errorf("%s: %#v", name, err)
		}
	}
	if _, err := registry.Call(context.Background(), bridge.ToolFlowsNavigate, []byte(`{"flows":"flows","goal":"nowhere"}`)); err == nil || err.Code != "flows-refused" {
		t.Errorf("unknown goal: %#v", err)
	}
	if _, err := registry.Call(context.Background(), bridge.ToolFlowsImpact, []byte(`{"flows":"flows","base":"`+string(bytes.Repeat([]byte("0"), 40))+`"}`)); err == nil || err.Code != "flows-refused" {
		t.Errorf("unknown base: %#v", err)
	}
}

func resolvedRoot(t *testing.T, root string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func flowsRegistry(t *testing.T, root string) *bridge.Registry {
	t.Helper()
	registry, err := bridge.NewFlows(root)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func copyIntoRoot(t *testing.T, root, source, relative string) string {
	t.Helper()
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, root, map[string]string{relative: string(raw)})
	return filepath.Join(root, filepath.FromSlash(relative))
}

func assertFlowsParity(t *testing.T, root string, registry *bridge.Registry, tool string, arguments map[string]any, cli ...string) {
	t.Helper()
	code, out, diagnostic := runFlowsCLI(root, cli...)
	if code != 0 {
		t.Fatalf("%v exited %d: %s", cli, code, diagnostic)
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	result, callErr := registry.Call(context.Background(), tool, raw)
	if callErr != nil || result.State != "READY" {
		t.Fatalf("%s %s: %#v %#v", tool, raw, callErr, result.Abstention)
	}
	receipt, err := json.Marshal(result.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodeNumbers(t, receipt), decodeNumbers(t, []byte(out))) {
		t.Errorf("%s %s receipt differs from `flows %v`:\n%s\n%s", tool, raw, cli, receipt, out)
	}
}

func decodeNumbers(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
