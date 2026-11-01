package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestContextCLIMatchesMCPReceipt pins LQP-V0-014 for V1-0480: the CLI
// context packet and the MCP corvint.context receipt derive from the same
// facts with LSP off, with a gopls that fails, and with a scripted gopls that
// completes a dialogue whose answers repeat a path and name one outside the
// root, so relations, provenance and omission counts are compared too. The CLI
// calls lspevidence.Attach directly rather than through the bridge, so this
// test, not shared code, is what holds the two surfaces equal.
func TestContextCLIMatchesMCPReceipt(t *testing.T) {
	root := evidenceSummaryRepository(t)
	failing := filepath.Join(t.TempDir(), "gopls")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	completing := testsupport.FakeGoplsExecutable(t, "cache/join.go,cache/join.go,"+filepath.Join(t.TempDir(), "z.go"))
	path := os.Getenv("PATH")
	t.Setenv("CORVINT_CONTEXT_LSP", "")
	for _, c := range []struct{ name, gopls string }{{"off", ""}, {"failing", failing}, {"completing", completing}} {
		arguments := []string{"--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go", "--limit", "6"}
		call := map[string]any{"task": evidenceSummaryTask, "subject": "cache/demux.go", "limit": 6}
		registry, _ := bridge.NewTaskReview(root)
		if c.gopls != "" {
			t.Setenv("PATH", filepath.Dir(c.gopls)+string(os.PathListSeparator)+path)
			arguments = append(arguments, "--lsp", "gopls")
			call["lsp"] = "gopls"
			registry, _ = bridge.NewTaskReviewLSP(root)
		}
		if registry == nil {
			t.Fatal("bridge registry unavailable")
		}
		code, stdout, stderr := runContextCommand(t, arguments...)
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", c.name, code, stderr)
		}
		encoded, _ := json.Marshal(call)
		result, failure := registry.Call(context.Background(), bridge.ToolContext, encoded)
		if failure != nil {
			t.Fatalf("%s: MCP call failed: %v", c.name, failure)
		}
		receipt, _ := json.Marshal(result.Receipt)
		cli := decodeObject(t, stdout)
		external, _ := cli["external"].(map[string]any)
		switch c.name {
		case "failing":
			if external == nil {
				t.Fatalf("the gopls packet carries no external section: %s", stdout)
			}
		case "completing":
			relations, _ := external["path_relations"].([]any)
			query, _ := external["query"].(map[string]any)
			if outside, _ := query["outside_repository"].(float64); len(relations) == 0 || outside < 1 {
				t.Fatalf("the completed dialogue produced no relations or outside count: %s", stdout)
			}
		}
		if mcp := decodeObject(t, receipt); !reflect.DeepEqual(cli, mcp) {
			t.Fatalf("%s: CLI packet and MCP receipt differ:\ncli %s\nmcp %s", c.name, stdout, receipt)
		}
	}
}
