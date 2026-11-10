// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// MCPV0-8300: replay a host's launch argv with no launch worktree. Older
// root-fatal servers close before initialize; these calls must still answer.
func TestMCPV08300UnavailableLaunchHandshake(t *testing.T) {
	declared := []string{"--root", "${CLAUDE_PROJECT_DIR}", "--protocol-version", "2025-11-25", "--tool-profile", "test-validity,docs"}
	launch := t.TempDir()
	t.Chdir(launch)
	for _, tc := range []struct{ name, root, reason string }{
		{"deleted spare", filepath.Join(launch, "spare-gone"), "--root does not exist"},
		{"existing spare", launch, "--root is not inside a Git worktree"},
		{"unexpanded project", "${CLAUDE_PROJECT_DIR}", "--root must be an absolute path"},
		{"empty project", "", "--root must be an absolute path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := make([]string, 0, len(declared))
			for _, arg := range declared {
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
				t.Fatal("empty tool list")
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
