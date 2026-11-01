package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TAW-V0-015: --tool-profile tasks lists the AGENT tools by default and the
// store-host tools under --tasks-role OPERATOR; --tasks-role is closed,
// given once, and needs the tasks group; a call runs the verb in process.
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
	agent := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "tasks"}))
	if got := tasks(agent); !slices.Equal(got, []string{"tasks_work_block", "tasks_work_deliver", "tasks_work_finding", "tasks_work_start"}) || len(agent) != len(code)+4 {
		t.Fatalf("agent tools %q", agent)
	}
	operator := tasks(toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "tasks", "--tasks-role", "OPERATOR"})))
	if len(operator) != 6 || !slices.Contains(operator, "tasks_work_close_merged") || !slices.Contains(operator, "tasks_work_release_sync") {
		t.Fatalf("operator tools %q", operator)
	}
	for _, refusal := range []struct {
		arguments []string
		reason    string
	}{
		{[]string{"--root", root, "--tasks-role", "OWNER"}, tasksRoleReasonUnpaired},
		{[]string{"--root", root, "--tool-profile", "docs", "--tasks-role", "OWNER"}, tasksRoleReasonUnpaired},
		{[]string{"--root", root, "--tool-profile", "tasks", "--tasks-role", "owner"}, tasksRoleReasonValue},
		{[]string{"--root", root, "--tool-profile", "tasks", "--tasks-role", "AGENT", "--tasks-role", "OWNER"}, tasksRoleReasonRepeated},
		{[]string{"--root", root, "--tool-profile", "tasks", "--tasks-role"}, tasksRoleReasonRepeated},
	} {
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), refusal.arguments, bytes.NewReader(nil), &stdout, &stderr); exit != 2 || stdout.Len() != 0 ||
			stderr.String() != "corvint-mcp: invalid arguments: "+refusal.reason+"\n" {
			t.Fatalf("%q exit=%d stdout=%q stderr=%q", refusal.arguments, exit, stdout.String(), stderr.String())
		}
	}
	// A root without a store copy: work start checks the form only and says so.
	session := startPipeSession(t, []string{"--root", root, "--tool-profile", "tasks"})
	result := resultOf(t, session.send(toolCall(1, "tasks_work_start", map[string]any{"tickets": []any{"V1-0001"}})))
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
