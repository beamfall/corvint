package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TAW-V0-015: --tool-profile tasks-work lists the AGENT tools by default and the
// store-host tools under --tasks-role OPERATOR; --tasks-role is closed,
// given once, and needs the tasks-work group; a call runs the corvint-tasks
// binary found on PATH and fails as a tool error when there is none
// (TAW-V0-025).
func TestTAWV0015TasksGroupProfile(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	code := toolNames(listedTools(t, []string{"--root", root}))
	tasks := func(names []string) []string {
		var out []string
		for _, name := range names {
			if strings.HasPrefix(name, "tasks_work_") {
				out = append(out, name)
			}
		}
		return out
	}
	agent := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "tasks-work"}))
	if got := tasks(agent); !slices.Equal(got, []string{"tasks_work_block", "tasks_work_deliver", "tasks_work_finding", "tasks_work_start"}) || len(agent) != len(code)+4 {
		t.Fatalf("agent tools %q", agent)
	}
	operator := tasks(toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "tasks-work", "--tasks-role", "OPERATOR"})))
	if len(operator) != 6 || !slices.Contains(operator, "tasks_work_close_merged") || !slices.Contains(operator, "tasks_work_release_sync") {
		t.Fatalf("operator tools %q", operator)
	}
	for _, refusal := range []struct {
		arguments []string
		reason    string
	}{
		{[]string{"--root", root, "--tasks-role", "OWNER"}, tasksRoleReasonUnpaired},
		{[]string{"--root", root, "--tool-profile", "docs", "--tasks-role", "OWNER"}, tasksRoleReasonUnpaired},
		{[]string{"--root", root, "--tool-profile", "tasks-work", "--tasks-role", "owner"}, tasksRoleReasonValue},
		{[]string{"--root", root, "--tool-profile", "tasks-work", "--tasks-role", "AGENT", "--tasks-role", "OWNER"}, tasksRoleReasonRepeated},
		{[]string{"--root", root, "--tool-profile", "tasks-work", "--tasks-role"}, tasksRoleReasonRepeated},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), refusal.arguments, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || stdout.Len() != 0 ||
			stderr.String() != "corvint-mcp: invalid arguments: "+refusal.reason+"\n" {
			t.Fatalf("%q exit=%d stdout=%q stderr=%q", refusal.arguments, exit, stdout.String(), stderr.String())
		}
	}
	if runtime.GOOS == "windows" {
		return // corvint-tasks refuses Windows (CTS-V0-025)
	}
	binaries := t.TempDir() // a corvint-tasks built from this tree
	if output, err := exec.Command("go", "build", "-buildvcs=false", "-o", binaries, "../../../cmd/corvint-tasks").CombinedOutput(); err != nil {
		t.Fatalf("go build corvint-tasks: %v: %s", err, output)
	}
	// Without corvint-tasks beside the executable or on PATH (which keeps
	// only git), a call is the TASKS_BINARY_UNAVAILABLE tool error, never an
	// in-process run (TAW-V0-025).
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitOnly := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(gitOnly, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", gitOnly)
	unavailable := startPipeSession(t, []string{"--root", root, "--tool-profile", "tasks-work"})
	result := resultOf(t, unavailable.send(toolCall(1, "tasks_work_start", map[string]any{"tickets": []any{"V1-0001"}})))
	if result["isError"] != true || !strings.Contains(mustJSONText(t, result["structuredContent"]), `"TASKS_BINARY_UNAVAILABLE"`) {
		t.Fatalf("tasks_work_start without corvint-tasks: %v", result)
	}
	if exit := unavailable.close(); exit != 0 {
		t.Fatalf("exit %d: %s", exit, unavailable.stderr.String())
	}
	// With the built corvint-tasks on PATH, a root without a store copy: work start checks the form only and says so.
	t.Setenv("PATH", binaries+string(os.PathListSeparator)+gitOnly)
	session := startPipeSession(t, []string{"--root", root, "--tool-profile", "tasks-work"})
	result = resultOf(t, session.send(toolCall(1, "tasks_work_start", map[string]any{"tickets": []any{"V1-0001"}})))
	envelope, _ := result["structuredContent"].(map[string]any)
	if result["isError"] != false || !strings.Contains(mustJSONText(t, envelope), `"command":["work","start"]`) || !strings.Contains(mustJSONText(t, envelope), "STORE_NOT_PRESENT") {
		t.Fatalf("tasks_work_start: %v", result)
	}
	refused := session.send(toolCall(2, "tasks_work_close_merged", map[string]any{"integration": "main"}))
	if refused["error"] == nil {
		t.Fatalf("an AGENT session called a store-host tool: %v", refused)
	}
	if exit := session.close(); exit != 0 {
		t.Fatalf("exit %d: %s", exit, session.stderr.String())
	}
}

func mustJSONText(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
