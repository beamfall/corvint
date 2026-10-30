package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// AHI-373: `corvint mcp` hands every later argument to the server before any other parser reads
// it, and serves under the process's one CORVINT_CPUPROFILE bracket instead of opening a second.
func TestAHI373MCPVerbOwnsItsArgumentsAndProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"mcp", "--corpus=data.json"}, strings.NewReader(""), &stdout, &stderr); code != 2 ||
		stderr.String() != "corvint-mcp: invalid arguments\n" {
		t.Fatalf("mcp --corpus=data.json: exit %d stderr %q; the server, not the corpus parser, must refuse it", code, &stderr)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "cpu.out")
	lookup := func(name string) (string, bool) {
		if name == "CORVINT_CPUPROFILE" {
			return profile, true
		}
		return "", false
	}
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}` + "\n"
	stdout.Reset()
	stderr.Reset()
	ctx := context.Background()
	if code := runProfiledContext(ctx, ctx, []string{"mcp", "--root", root, "--protocol-version", "2025-11-25"}, strings.NewReader(initialize), &stdout, &stderr, lookup); code != 0 ||
		!strings.Contains(stdout.String(), `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("profiled mcp: exit %d stdout %q stderr %q", code, &stdout, &stderr)
	}
}
