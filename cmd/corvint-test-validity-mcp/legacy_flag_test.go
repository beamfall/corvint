package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestMCPV0820DocsAndTestValidityNameLegacyFlag: started without
// --protocol-version, the server answers a 2025-11-25 initialize with -32602
// naming the flag, instead of a bare method-not-found, and keeps serving
// (V1-2513).
func TestMCPV0820DocsAndTestValidityNameLegacyFlag(t *testing.T) {
	t.Parallel()
	input := `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"claude-code","version":"2.1.311"}}}` + "\n" +
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", bareGitRoot(t)}, strings.NewReader(input), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses=%q", stdout.String())
	}
	var refused struct {
		Error struct {
			Code    int
			Message string
			Data    struct{ Flag string }
		}
	}
	first, second := lines[0], lines[1]
	if strings.Contains(first, `"tools":[`) {
		first, second = second, first
	}
	if err := json.Unmarshal([]byte(first), &refused); err != nil {
		t.Fatal(err)
	}
	if refused.Error.Code != -32602 || !strings.Contains(refused.Error.Message, "--protocol-version 2025-11-25") || refused.Error.Data.Flag != "--protocol-version 2025-11-25" {
		t.Fatalf("initialize answer=%s", first)
	}
	if !strings.Contains(second, `"tools":[`) {
		t.Fatalf("server stopped serving after the answer: %s", second)
	}
}
