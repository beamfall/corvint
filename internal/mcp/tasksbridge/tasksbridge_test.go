package tasksbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/tasksbridge"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
)

const createPayload = `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],` +
	`"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,` +
	`"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS",` +
	`"kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2",` +
	`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,` +
	`"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,` +
	`"supersedes":null,"title":"TITLE"}`

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// tasks runs corvint-tasks in process with the legacy (unset) class, as the
// store owner sets the fixture up.
func tasks(t *testing.T, root string, args ...string) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cli.Run(cli.Env{Cwd: root, Args: args, Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr})
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope["outcome"] != "OK" {
		t.Fatalf("%v: %s %s", args, stdout.String(), stderr.String())
	}
	return envelope
}

func firstItem(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	items, _ := envelope["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("no items: %v", envelope)
	}
	return items[0].(map[string]any)
}

// store is an initialized fixture store in a Git repository with two
// tickets in milestone m1 and a release v1 holding the first.
func store(t *testing.T) (root string, tickets []string) {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv(cli.RoleEnv, "")
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	git(t, r.Root, "init", "-q", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "base")
	tasks(t, r.Root, "init")
	for _, title := range []string{"bridge deliver", "bridge block"} {
		payload := strings.Replace(strings.Replace(createPayload, "TITLE", title, 1), `"milestone":null`, `"milestone":"m1"`, 1)
		created := tasks(t, r.Root, "ticket", "create", "--request-id", "create-"+strings.ReplaceAll(title, " ", "-"), "--issued-at", "2026-11-01T09:00:00Z", "--payload", payload)
		tickets = append(tickets, firstItem(t, created)["ticketId"].(string))
	}
	release := `{"acceptanceCriteria":["release criterion"],"predecessorReleaseIds":[],"requiredGates":[],"ticketIds":["` + tickets[0] + `"],"title":"Version 1","version":"v1"}`
	tasks(t, r.Root, "release", "create", "--request-id", "rel", "--target", "v1", "--issued-at", "2026-11-01T09:01:00Z", "--payload", release)
	return r.Root, tickets
}

func names(group tasksbridge.Group) []string {
	var out []string
	for _, tool := range group.Tools() {
		out = append(out, tool.Name)
	}
	return out
}

// TestTAWV0015_TasksToolsPerRole: AGENT lists the lane verbs and finding,
// OPERATOR and OWNER add the store-host verbs, every schema is closed, and a
// tool the class does not list, or an argument outside the schema, is
// invalid params (TAW-V0-015).
func TestTAWV0015_TasksToolsPerRole(t *testing.T) {
	agent := []string{tasksbridge.ToolStart, tasksbridge.ToolDeliver, tasksbridge.ToolBlock, tasksbridge.ToolFinding}
	host := append(slices.Clone(agent), tasksbridge.ToolCloseMerged, tasksbridge.ToolReleaseSync)
	for class, want := range map[string][]string{cli.ClassAgent: agent, cli.ClassOperator: host, cli.ClassOwner: host} {
		group := tasksbridge.Group{Root: t.TempDir(), Class: class}
		if got := names(group); !slices.Equal(got, want) {
			t.Fatalf("%s tools %q, want %q", class, got, want)
		}
		for _, tool := range group.Tools() {
			if tool.InputSchema["additionalProperties"] != false || tool.Description == "" || tool.Annotations.DestructiveHint {
				t.Fatalf("%s: schema or annotations: %+v", tool.Name, tool)
			}
			lane := tool.Name == tasksbridge.ToolStart || tool.Name == tasksbridge.ToolDeliver || tool.Name == tasksbridge.ToolBlock
			if tool.Annotations.ReadOnlyHint != lane || !tool.Annotations.IdempotentHint {
				t.Fatalf("%s: annotations %+v", tool.Name, tool.Annotations)
			}
		}
	}
	agentGroup := tasksbridge.Group{Root: t.TempDir(), Class: cli.ClassAgent}
	for name, arguments := range map[string]map[string]any{
		tasksbridge.ToolCloseMerged: {"integration": "main"},
		tasksbridge.ToolReleaseSync: {"release": "v1", "milestone": "m1", "requestId": "r"},
		"tasks_work_unknown":        {},
		tasksbridge.ToolStart:       {"tickets": []any{"V1-0001"}, "extra": true},
		tasksbridge.ToolDeliver:     {"tickets": []any{}},
		tasksbridge.ToolFinding:     {"payload": map[string]any{}},
	} {
		if result, rpcErr := agentGroup.Call(context.Background(), name, arguments); rpcErr == nil {
			t.Fatalf("%s %v: want invalid params, got %v", name, arguments, result)
		}
	}
}

// call runs one tool and returns its envelope; the text block is the framed
// envelope and isError follows the outcome.
func call(t *testing.T, group tasksbridge.Group, name string, arguments map[string]any) map[string]any {
	t.Helper()
	result, rpcErr := group.Call(context.Background(), name, arguments)
	if rpcErr != nil {
		t.Fatalf("%s: %v", name, rpcErr)
	}
	envelope := result["structuredContent"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.HasPrefix(text, repoenvelope.Prefix) || result["isError"] != (envelope["outcome"] != "OK") {
		t.Fatalf("%s: framing or isError: %v", name, result)
	}
	if envelope["outcome"] != "OK" {
		t.Fatalf("%s: %v", name, envelope)
	}
	return envelope
}

// TestTAWV0015_CallsEachTool calls every tasks tool over a fixture store: the
// lane verbs return trailers, a delivered trailer on main is completed by
// close-merged, a finding is filed and release-sync adds the milestone
// member (TAW-V0-015).
func TestTAWV0015_CallsEachTool(t *testing.T) {
	root, tickets := store(t)
	local := func(id string) string { return id[strings.LastIndex(id, ":")+1:] }
	agent := tasksbridge.Group{Root: root, Class: cli.ClassAgent}
	operator := tasksbridge.Group{Root: root, Class: cli.ClassOperator}

	call(t, agent, tasksbridge.ToolStart, map[string]any{"tickets": []any{local(tickets[0])}, "lane": "bridge"})
	delivered := call(t, agent, tasksbridge.ToolDeliver, map[string]any{"tickets": []any{local(tickets[0])}, "lane": "bridge"})
	blocked := call(t, agent, tasksbridge.ToolBlock, map[string]any{"tickets": []any{local(tickets[1])}, "reason": "needs a fixture owned by another lane"})
	var message []string
	for _, envelope := range []map[string]any{delivered, blocked} {
		for _, line := range firstItem(t, envelope)["trailers"].([]any) {
			message = append(message, line.(string))
		}
	}
	if len(message) != 6 || message[0] != "Corvint-Ticket: "+local(tickets[0]) {
		t.Fatalf("trailers %q", message)
	}
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m",
		"deliver\n\n"+strings.Join(message[:3], "\n"))
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m",
		"block\n\n"+strings.Join(message[3:], "\n"))

	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.Replace(createPayload, "TITLE", "bridge finding about cursor files", 1)), &payload); err != nil {
		t.Fatal(err)
	}
	filed := call(t, agent, tasksbridge.ToolFinding, map[string]any{"payload": payload, "requestId": "finding-1", "issuedAt": "2026-11-01T09:02:00Z"})
	if !strings.Contains(mustJSON(t, filed), `"CREATED"`) {
		t.Fatalf("finding: %v", filed)
	}

	merged := call(t, operator, tasksbridge.ToolCloseMerged, map[string]any{"integration": "main"})
	text := mustJSON(t, merged)
	if !strings.Contains(text, `"COMPLETED"`) || !strings.Contains(text, `"BLOCKED"`) {
		t.Fatalf("close-merged: %s", text)
	}
	synced := call(t, operator, tasksbridge.ToolReleaseSync, map[string]any{"release": "v1", "milestone": "m1", "requestId": "sync-1", "issuedAt": "2026-11-01T09:03:00Z"})
	if !strings.Contains(mustJSON(t, synced), `"ADDED"`) {
		t.Fatalf("release-sync: %v", synced)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
