package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// NGI-V0-305: paginated MCP JSONL reconstructs the uncapped CLI stream exactly.
func TestGraphImportsCLIMCPParity(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 66; i++ {
		body := "import { value } from './f00';\n"
		if i == 0 {
			body = "export const value = 1;\n"
		}
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%02d.ts", i)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cemGit(t, root, "init", "-q")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "graph fixture")
	revision := cemGit(t, root, "rev-parse", "HEAD")
	registry, callErr := bridge.NewImportGraph(root)
	if callErr != nil {
		t.Fatal(callErr)
	}
	for _, kinds := range []string{"", "import", "type-import"} {
		args := []string{"--root", root, "graph", "imports", "--revision", revision, "--format", "jsonl"}
		if kinds != "" {
			args = append(args, "--kinds", kinds)
		}
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, &stderr)
		}
		var reconstructed bytes.Buffer
		offset := int64(0)
		for pages := 0; ; pages++ {
			if pages > 10 {
				t.Fatal("pagination did not terminate")
			}
			request := map[string]any{"revision": revision, "offset": offset}
			if kinds != "" {
				request["kinds"] = kinds
			}
			raw, _ := json.Marshal(request)
			result, err := registry.Call(t.Context(), bridge.ToolGraphImports, raw)
			if err != nil || result.State != "READY" {
				t.Fatalf("result: %+v, error %v", result, err)
			}
			if _, err := result.CanonicalJSON(); err != nil {
				t.Fatal(err)
			}
			reconstructed.WriteString(result.Receipt["jsonl"].(string))
			if result.Receipt["done"] == true {
				if kinds == "" && pages == 0 {
					t.Fatal("fixture never reached second page")
				}
				break
			}
			next := result.Receipt["nextOffset"].(int64)
			if next <= offset {
				t.Fatal("page made no progress")
			}
			offset = next
		}
		if !bytes.Equal(stdout.Bytes(), reconstructed.Bytes()) {
			t.Fatal("MCP pages differ from CLI JSONL")
		}
	}
	if status := cemGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("graph mutated repository: %s", status)
	}
}

// NGI-V0-304: public help and fail-closed CLI argument/revision admission.
func TestGraphImportsCLIAdmission(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.ts"), []byte("export const a = 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "fixture")
	revision := cemGit(t, root, "rev-parse", "HEAD")
	for _, item := range []struct {
		args []string
		code string
	}{
		{[]string{"--revision", "HEAD"}, "graph-invalid-revision"},
		{[]string{"--revision", strings.Repeat("1", 40)}, "graph-unknown-revision"},
		{nil, "graph-invalid-revision"},
		{[]string{"--revision", revision, "--revision", revision}, "graph-invalid-arguments"},
		{[]string{"--revision", revision, "--kinds", "unknown"}, "graph-invalid-kinds"},
		{[]string{"--revision", revision, "--format", "json"}, "graph-invalid-arguments"},
	} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"--root", root, "graph", "imports"}, item.args...)
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), item.code) {
			t.Fatalf("%v: exit %d, stdout=%s stderr=%s", args, code, &stdout, &stderr)
		}
	}
	for _, args := range [][]string{{"graph", "--help"}, {"graph", "imports", "--help"}, {"help", "graph"}, {"help", "graph", "imports"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "--revision") {
			t.Fatalf("help %v: %d %s %s", args, code, &stdout, &stderr)
		}
	}
}
