package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/update"
)

// AHI-373: every executable an installed host plugin launches from PATH ships in the Core
// release archive, so a release user never meets an ENOENT server or hook.
func TestAHI373PluginCommandsShipInCoreRelease(t *testing.T) {
	type hook struct {
		Command string `json:"command"`
	}
	var commands []string
	read := func(path string, into any) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "integrations", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	for _, path := range []string{"claude-code/plugins/corvint/hooks/hooks.json", "codex/plugins/corvint/hooks/hooks.json"} {
		var declared struct {
			Hooks map[string][]struct {
				Hooks []hook `json:"hooks"`
			} `json:"hooks"`
		}
		read(path, &declared)
		for _, groups := range declared.Hooks {
			for _, group := range groups {
				for _, h := range group.Hooks {
					commands = append(commands, strings.Fields(h.Command)[0])
				}
			}
		}
	}
	for _, path := range []string{"claude-code/plugins/corvint/.mcp.json", "codex/plugins/corvint/.mcp.json"} {
		var declared struct {
			MCPServers map[string]struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"mcpServers"`
		}
		read(path, &declared)
		for name, server := range declared.MCPServers {
			command := server.Command
			if command == "/bin/sh" {
				// The Codex launcher binds the worktree in sh, then execs the server.
				script := server.Args[len(server.Args)-1]
				_, after, found := strings.Cut(script, "exec ")
				if !found {
					t.Fatalf("%s %s: sh launcher without exec: %q", path, name, script)
				}
				command = strings.Fields(after)[0]
			}
			commands = append(commands, command)
		}
	}
	if len(commands) < 4 {
		t.Fatalf("read only %d plugin commands", len(commands))
	}
	for _, command := range commands {
		if !slices.Contains(update.CoreExecutables, command) {
			t.Errorf("plugin launches %q, which the Core release archive %v does not ship", command, update.CoreExecutables)
		}
	}
}

// AHI-820: the Claude Code plugin's corvint server serves the test-validity and docs tool groups
// over the legacy handshake Claude Code speaks, so their tools are reachable in-session (V1-2513).
func TestAHI820ClaudePluginDeclaresTestValidityAndDocsGroups(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "integrations", "claude-code", "plugins", "corvint", ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		t.Fatal(err)
	}
	server := declared.MCPServers["corvint"]
	want := []string{"mcp", "--root", "${CLAUDE_PROJECT_DIR}", "--protocol-version", "2025-11-25", "--tool-profile", "test-validity,docs"}
	if len(declared.MCPServers) != 1 || server.Command != "corvint" || !slices.Equal(server.Args, want) {
		t.Fatalf("declared %+v, want one corvint server with args %q", declared.MCPServers, want)
	}
	root := taskContextRepository(t)
	args := slices.Clone(server.Args)
	args[slices.Index(args, "${CLAUDE_PROJECT_DIR}")] = root
	input := `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"claude-code","version":"2.1.311"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	var stdout, stderr strings.Builder
	if exit := runContext(t.Context(), args, strings.NewReader(input), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	var listed struct {
		ID     int
		Result struct{ Tools []struct{ Name string } }
	}
	var names []string
	for line := range strings.Lines(stdout.String()) {
		if err := json.Unmarshal([]byte(line), &listed); err == nil && listed.ID == 1 {
			for _, tool := range listed.Result.Tools {
				names = append(names, tool.Name)
			}
		}
	}
	for _, name := range []string{"corvint.status", "corvint.test_validity", "corvint.docs_draft", "corvint.docs_consume"} {
		if !slices.Contains(names, name) {
			t.Fatalf("tools/list %q lacks %s: %s", names, name, stdout.String())
		}
	}
}
