package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TMR-V0-950, TMR-V0-952: `--tool-profile tasks` lists the five Tasks tools
// beside the default ones and runs the installed corvint-tasks on PATH in the
// bound toplevel with CORVINT_TASKS_READ_ONLY=1; an unusable root lists them
// and refuses each invalid-root under the group's profile.
func TestTMRV0950TasksGroupRunsTheInstalledReadVerb(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake corvint-tasks is a shell script")
	}
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '{\"codes\":[],\"command\":[\"%s\"],\"items\":[{\"cwd\":\"%s\",\"readOnly\":\"%s\",\"args\":\"%s\"}],\"outcome\":\"OK\",\"page\":null,\"profile\":\"taskman-command-result/0\"}\\n' \"$1\" \"$(pwd -P)\" \"$CORVINT_TASKS_READ_ONLY\" \"$*\"\n"
	if err := execfile.WriteFile(filepath.Join(bin, "corvint-tasks"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	exit, responses, stderr := legacySession(t, []string{"--root", filepath.Join(root, "sub"), "--tool-profile", "tasks"},
		map[string]any{"method": "tools/list", "params": map[string]any{}},
		map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.tasks.roadmap", "arguments": map[string]any{"limit": 2}}})
	if exit != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	var names []string
	for _, raw := range unavailableListed(t, responses[1]) {
		tool := raw.(map[string]any)
		names = append(names, tool["name"].(string))
		annotations, _ := tool["annotations"].(map[string]any)
		if strings.HasPrefix(tool["name"].(string), "corvint.tasks.") && (tool["outputSchema"] == nil || annotations["readOnlyHint"] != true || annotations["destructiveHint"] != false) {
			t.Fatalf("%s descriptor %+v", tool["name"], tool)
		}
	}
	if strings.Join(names, ",") != "corvint.help,corvint.impact,corvint.query,corvint.status,corvint.tasks.queue_status,corvint.tasks.receipt_audit,corvint.tasks.roadmap,corvint.tasks.ticket_search,corvint.tasks.ticket_show" {
		t.Fatalf("tools %q", names)
	}
	result, _ := responses[2]["result"].(map[string]any)
	item := result["structuredContent"].(map[string]any)["items"].([]any)[0].(map[string]any)
	physical, _ := filepath.EvalSymlinks(root)
	if result["isError"] != false || item["cwd"] != physical || item["readOnly"] != "1" || item["args"] != "roadmap --limit 2" {
		t.Fatalf("roadmap %+v", result)
	}
	// An existing directory outside Git is unusable without being missing, so the
	// MCPV0-115 launch-worktree fallback does not replace it.
	_, responses, _ = legacySession(t, []string{"--root", t.TempDir(), "--tool-profile", "tasks"},
		map[string]any{"method": "tools/call", "params": map[string]any{"name": "corvint.tasks.queue_status", "arguments": map[string]any{}}})
	result, _ = responses[1]["result"].(map[string]any)
	if structured := result["structuredContent"].(map[string]any); result["isError"] != true || structured["code"] != invalidRoot || structured["profile"] != "corvint-tasks-mcp-tool-error/0" {
		t.Fatalf("unavailable %+v", result)
	}
}
