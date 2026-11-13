// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

// MTT-V1-003: the retiring corvint-test-validity-mcp's real tools/list
// response, read off the wire, registers exactly its table rows: its own
// corvint.test_validity and the shared corvint.help.
func TestMTTV1003TestValidityServerRegistersExactlyTheTable(t *testing.T) {
	t.Parallel()
	root := bareGitRoot(t)
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root}, strings.NewReader(input), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	var response struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &response); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	if len(response.Result.Tools) == 0 {
		t.Fatalf("tools/list listed nothing: %s", stdout.String())
	}
	if problems := capcatalog.MCPDrift(capcatalog.MCPToolsFor(capcatalog.ServerTestValidity), response.Result.Tools); len(problems) != 0 {
		t.Fatalf("corvint-test-validity-mcp drifts from the table:\n%s", strings.Join(problems, "\n"))
	}
}
