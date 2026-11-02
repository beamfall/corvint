package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// AHI-860: the server the Claude Code plugin declares completes the 2025-11-25 handshake and lists
// its tools for every project-directory shape Claude Code has been observed to substitute for
// ${CLAUDE_PROJECT_DIR} (V1-3894), so the host never records CONNECTION_CLOSED. A repository
// call answers from the repository when the substituted root is a worktree path, and from the
// launch worktree when it no longer exists (MCPV0-115) or is an existing directory outside Git
// (MCPV0-861); it answers invalid-root, never a dead connection, when the variable was not
// expanded.
func TestAHI860ClaudePluginHandshakeAcrossProjectDirShapes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "integrations", "claude-code", "plugins", "corvint", ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		t.Fatal(err)
	}
	server, ok := declared.MCPServers["corvint"]
	if !ok || server.Command != "corvint" || len(server.Args) == 0 || server.Args[0] != "mcp" {
		t.Fatalf("declared %+v, want a corvint mcp server", declared.MCPServers)
	}
	wantTools := strings.Split(server.Env["CORVINT_PLUGIN_TOOLS"], ",")
	slices.Sort(wantTools)

	repository := cliRepository(t)
	subdirectory := filepath.Join(repository, "sub")
	if err := os.Mkdir(subdirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	// A Claude desktop spare session: argv names the spare directory while the server's working
	// directory is the real project. The directory may still exist, or be deleted already.
	spare := t.TempDir()
	deletedSpare := filepath.Join(t.TempDir(), "spare-gone")

	cases := []struct {
		name        string
		projectDir  string
		statusError string
	}{
		{"repository", repository, ""},
		{"subdirectory", subdirectory, ""},
		{"deleted spare", deletedSpare, ""},
		{"existing non-worktree spare", spare, ""},
		{"unexpanded variable", "${CLAUDE_PROJECT_DIR}", "invalid-root"},
	}
	requests := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"ahi-860","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"corvint.status","arguments":{}}}`,
	}, "\n") + "\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			arguments := make([]string, len(server.Args))
			for index, argument := range server.Args {
				arguments[index] = strings.ReplaceAll(argument, "${CLAUDE_PROJECT_DIR}", tc.projectDir)
			}
			overrides := []string{"CORVINT_HELPER_PROCESS=1", "CLAUDE_PROJECT_DIR=" + repository}
			for name, value := range server.Env {
				overrides = append(overrides, name+"="+value)
			}
			command := candidateCommand(arguments...)
			command.Dir = repository
			command.Env = testEnvironment(overrides...)
			command.Stdin = strings.NewReader(requests)
			result := execute(t, command)
			if result.exit != 0 {
				t.Fatalf("exit=%d stderr=%s", result.exit, result.stderr)
			}
			responses := map[int]json.RawMessage{}
			scanner := bufio.NewScanner(bytes.NewReader(result.stdout))
			scanner.Buffer(make([]byte, 0, 1<<20), 16<<20)
			for scanner.Scan() {
				var message struct {
					ID     *int            `json:"id"`
					Result json.RawMessage `json:"result"`
					Error  json.RawMessage `json:"error"`
				}
				if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
					t.Fatalf("response %q: %v", scanner.Text(), err)
				}
				if message.ID == nil {
					continue
				}
				if message.Error != nil {
					t.Fatalf("request %d answered error %s", *message.ID, message.Error)
				}
				responses[*message.ID] = message.Result
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}

			var initialized struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if err := json.Unmarshal(responses[1], &initialized); err != nil || initialized.ProtocolVersion != "2025-11-25" {
				t.Fatalf("initialize=%s (%v), want protocolVersion 2025-11-25; stderr=%s", responses[1], err, result.stderr)
			}
			var listed struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			}
			if err := json.Unmarshal(responses[2], &listed); err != nil {
				t.Fatalf("tools/list=%s: %v", responses[2], err)
			}
			var names []string
			for _, tool := range listed.Tools {
				names = append(names, tool.Name)
			}
			slices.Sort(names)
			if !slices.Equal(names, wantTools) {
				t.Fatalf("tools/list=%q, want the plugin's declared %q", names, wantTools)
			}

			var status struct {
				IsError           bool `json:"isError"`
				StructuredContent struct {
					Code string `json:"code"`
				} `json:"structuredContent"`
				Meta map[string]struct {
					Compatible bool `json:"compatible"`
				} `json:"_meta"`
			}
			if err := json.Unmarshal(responses[3], &status); err != nil {
				t.Fatalf("corvint.status=%s: %v", responses[3], err)
			}
			if tc.statusError == "" {
				if status.IsError {
					t.Fatalf("corvint.status=%s, want a repository answer; stderr=%s", responses[3], result.stderr)
				}
				if !status.Meta["io.corvint/pluginCompatibility"].Compatible {
					t.Fatalf("corvint.status _meta=%s, want the plugin compatible", responses[3])
				}
				return
			}
			if !status.IsError || status.StructuredContent.Code != tc.statusError {
				t.Fatalf("corvint.status=%s, want isError code %q", responses[3], tc.statusError)
			}
			if !strings.Contains(string(result.stderr), "repository unavailable") {
				t.Fatalf("stderr=%q, want the repository-unavailable reason", result.stderr)
			}
		})
	}
}
