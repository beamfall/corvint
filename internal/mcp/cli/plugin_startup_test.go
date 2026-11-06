// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MCPV0-8300: replay the plugin's real argv/environment with no launch worktree.
// Older root-fatal servers close before initialize; these calls must still answer.
func TestMCPV08300PluginUnavailableLaunchHandshake(t *testing.T) {
	raw, err := os.ReadFile("../../../integrations/claude-code/plugins/corvint/.mcp.json")
	if err != nil {
		t.Fatal(err)
	}
	var plugin struct {
		MCPServers map[string]struct {
			Command string
			Args    []string
			Env     map[string]string
		}
	}
	if err = json.Unmarshal(raw, &plugin); err != nil {
		t.Fatal(err)
	}
	declared := plugin.MCPServers["corvint"]
	if declared.Command != "corvint" || len(declared.Args) == 0 || declared.Args[0] != "mcp" {
		t.Fatalf("unexpected plugin command: %+v", declared)
	}
	for key, value := range declared.Env {
		t.Setenv(key, value)
	}
	launch := t.TempDir()
	t.Chdir(launch)
	for _, tc := range []struct{ name, root, reason string }{
		{"deleted spare", filepath.Join(launch, "spare-gone"), "--root does not exist"},
		{"existing spare", launch, "--root is not inside a Git worktree"},
		{"unexpanded project", "${CLAUDE_PROJECT_DIR}", "--root must be an absolute path"},
		{"empty project", "", "--root must be an absolute path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := make([]string, 0, len(declared.Args))
			for _, arg := range declared.Args[1:] {
				// legacySession supplies the compatibility flag itself.
				args = append(args, strings.ReplaceAll(arg, "${CLAUDE_PROJECT_DIR}", tc.root))
			}
			for i := 0; i < len(args); i++ {
				if args[i] == "--protocol-version" {
					args = append(args[:i], args[i+2:]...)
					break
				}
			}
			exit, responses, stderr := legacySession(t, args,
				map[string]any{"method": "tools/list"},
				map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.status", "arguments": map[string]any{}}})
			if exit != 0 || responses[0]["result"] == nil {
				t.Fatalf("early close: exit=%d stderr=%q initialize=%#v", exit, stderr, responses[0])
			}
			if len(unavailableListed(t, responses[1])) == 0 {
				t.Fatal("empty plugin tool list")
			}
			status, _ := responses[2]["result"].(map[string]any)
			content, _ := status["structuredContent"].(map[string]any)
			if status["isError"] != true || content["code"] != "invalid-root" {
				t.Fatalf("status=%#v", status)
			}
			if !strings.Contains(stderr, "corvint-mcp: repository unavailable: "+tc.reason+";") || strings.Contains(stderr, launch) {
				t.Fatalf("missing fixed path-free cause: %q", stderr)
			}
		})
	}
}
