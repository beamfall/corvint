// SPDX-License-Identifier: Apache-2.0
package docsserver

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
)

// repository is a committed Git worktree with one Go file.
func repository(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"}} {
		git(t, root, args...)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "init")
	return root
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// session serves arguments with the given requests after the initialize
// handshake and returns the exit code, each response by id and stderr.
func session(t *testing.T, arguments []string, requests ...map[string]any) (int, map[float64]map[string]any, string) {
	t.Helper()
	lines := []string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}
	for index, request := range requests {
		request["jsonrpc"], request["id"] = "2.0", index+1
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	var stdout, stderr bytes.Buffer
	exit := Serve(context.Background(), "test", append([]string{"--protocol-version", "2025-11-25"}, arguments...), strings.NewReader(strings.Join(lines, "\n")+"\n"), &stdout, &stderr)
	responses := map[float64]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		id, _ := response["id"].(float64)
		responses[id] = response
	}
	return exit, responses, stderr.String()
}

func listed(t *testing.T, arguments []string) []map[string]any {
	t.Helper()
	exit, responses, stderr := session(t, arguments, map[string]any{"method": "tools/list"})
	if exit != 0 {
		t.Fatalf("exit %d stderr %q", exit, stderr)
	}
	result, _ := responses[1]["result"].(map[string]any)
	var tools []map[string]any
	for _, tool := range result["tools"].([]any) {
		tools = append(tools, tool.(map[string]any))
	}
	return tools
}

// MTT-V1-004, MTT-V1-003: `corvint-docs mcp` lists corvint.help,
// docs_draft, docs_consume and every corpus tool, and registers exactly
// the table's corvint-docs mcp rows and their input properties.
func TestMTTV1004DocsServerToolList(t *testing.T) {
	root := repository(t)
	tools := listed(t, []string{"--root", root})
	want := map[string]capcatalog.MCPTool{}
	for _, row := range capcatalog.MCPToolsFor(capcatalog.ServerDocs) {
		want[row.Name] = row
	}
	var names []string
	for _, tool := range tools {
		name := tool["name"].(string)
		names = append(names, name)
		row, ok := want[name]
		if !ok {
			t.Fatalf("%s: registered with no corvint-docs mcp table row", name)
		}
		properties, _ := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		var described []string
		for _, input := range row.Inputs {
			described = append(described, input.Name)
		}
		slices.Sort(described)
		if registered := slices.Sorted(maps.Keys(properties)); !slices.Equal(registered, described) {
			t.Fatalf("%s: registered input properties %q, table describes %q", name, registered, described)
		}
	}
	if len(names) != len(want) {
		t.Fatalf("listed %q, table rows %q", names, slices.Sorted(maps.Keys(want)))
	}
	for _, name := range []string{"corvint.help", docsbridge.ToolDraft, docsbridge.ToolConsume, corpusbridge.SearchTool} {
		if !slices.Contains(names, name) {
			t.Fatalf("%s not listed: %q", name, names)
		}
	}
	for _, name := range names {
		if strings.Contains(name, "tasks") || name == "corvint.impact" || name == "corvint.affected" {
			t.Fatalf("documentation server lists %s", name)
		}
	}
}

// MTT-V1-004: help answers from the binary; a corpus call without
// --corpus-artifact is the corpus-refused tool error naming the fix, and
// with a missing artifact it is the same refusal while the server keeps
// serving; an unknown tool is invalid params.
func TestMTTV1004DocsServerAnswers(t *testing.T) {
	root := repository(t)
	exit, responses, stderr := session(t, []string{"--root", root},
		map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.help", "arguments": map[string]any{}}},
		map[string]any{"method": "tools/call", "params": map[string]any{"name": corpusbridge.SearchTool, "arguments": map[string]any{"query": "x"}}},
		map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.test_validity", "arguments": map[string]any{}}},
	)
	if exit != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", exit, stderr)
	}
	if help, _ := responses[1]["result"].(map[string]any); help["isError"] != false {
		t.Fatalf("help: %#v", responses[1])
	}
	refused, _ := responses[2]["result"].(map[string]any)
	structured, _ := refused["structuredContent"].(map[string]any)
	if refused["isError"] != true || structured["code"] != corpusRefused || !strings.Contains(structured["error"].(string), "--corpus-artifact") {
		t.Fatalf("corpus without artifact: %#v", responses[2])
	}
	if rpc, _ := responses[3]["error"].(map[string]any); rpc["code"] != float64(-32602) {
		t.Fatalf("unknown tool: %#v", responses[3])
	}

	exit, responses, stderr = session(t, []string{"--root", root, "--corpus-artifact", "missing.json"},
		map[string]any{"method": "tools/call", "params": map[string]any{"name": corpusbridge.SearchTool, "arguments": map[string]any{"query": "x"}}},
	)
	refused, _ = responses[1]["result"].(map[string]any)
	structured, _ = refused["structuredContent"].(map[string]any)
	if exit != 0 || !strings.Contains(stderr, "--corpus-artifact unavailable") || refused["isError"] != true || structured["code"] != corpusRefused {
		t.Fatalf("missing artifact: exit %d stderr %q %#v", exit, stderr, responses[1])
	}
}

// MTT-V1-004: argument errors refuse before any MCP byte with a reason.
func TestMTTV1004DocsServerRefusesArguments(t *testing.T) {
	root := repository(t)
	for _, tc := range []struct {
		arguments []string
		want      string
	}{
		{nil, "--root ABSOLUTE_ROOT is required"},
		{[]string{"--root", root, "--tool-profile", "docs"}, `unknown argument "--tool-profile"`},
		{[]string{"--root", root, "--root", root}, "--root must be given once"},
		{[]string{"--root"}, "--root needs a value"},
		{[]string{"--root", root, "--corpus-artifact", "../outside.json"}, "--corpus-artifact:"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := Serve(context.Background(), "test", tc.arguments, strings.NewReader(""), &stdout, &stderr); exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "corvint-docs mcp: invalid arguments: "+tc.want) {
			t.Fatalf("%q: exit %d stdout %q stderr %q", tc.arguments, exit, &stdout, &stderr)
		}
	}
	var stdout, stderr bytes.Buffer
	if exit := Serve(context.Background(), "test", []string{"--help"}, strings.NewReader(""), &stdout, &stderr); exit != 0 || stdout.String() != Usage {
		t.Fatalf("--help: exit %d", exit)
	}
}
