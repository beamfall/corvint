package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// TestContextCLIMatchesMCPReceipt pins LQP-V0-014 for V1-0480: the CLI
// context packet and the MCP corvint.context receipt derive from the same
// facts, with LSP off and with a gopls that fails. The CLI calls
// lspevidence.Attach directly rather than through the bridge, so this test,
// not shared code, is what holds the two surfaces equal.
func TestContextCLIMatchesMCPReceipt(t *testing.T) {
	root := evidenceSummaryRepository(t)
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "gopls"), []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CORVINT_CONTEXT_LSP", "")
	for _, lsp := range []bool{false, true} {
		arguments := []string{"--root", root, "context", "--task", evidenceSummaryTask, "--subject", "cache/demux.go", "--limit", "6"}
		call := map[string]any{"task": evidenceSummaryTask, "subject": "cache/demux.go", "limit": 6}
		registry, _ := bridge.NewTaskReview(root)
		if lsp {
			arguments = append(arguments, "--lsp", "gopls")
			call["lsp"] = "gopls"
			registry, _ = bridge.NewTaskReviewLSP(root)
		}
		if registry == nil {
			t.Fatal("bridge registry unavailable")
		}
		code, stdout, stderr := runContextCommand(t, arguments...)
		if code != 0 {
			t.Fatalf("lsp=%v: exit %d: %s", lsp, code, stderr)
		}
		encoded, _ := json.Marshal(call)
		result, failure := registry.Call(context.Background(), bridge.ToolContext, encoded)
		if failure != nil {
			t.Fatalf("lsp=%v: MCP call failed: %v", lsp, failure)
		}
		receipt, _ := json.Marshal(result.Receipt)
		cli := decodeObject(t, stdout)
		if lsp {
			if _, ok := cli["external"]; !ok {
				t.Fatalf("the gopls packet carries no external section: %s", stdout)
			}
		}
		if mcp := decodeObject(t, receipt); !reflect.DeepEqual(cli, mcp) {
			t.Fatalf("lsp=%v: CLI packet and MCP receipt differ:\ncli %s\nmcp %s", lsp, stdout, receipt)
		}
	}
}
