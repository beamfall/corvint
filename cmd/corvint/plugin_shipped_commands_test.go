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
