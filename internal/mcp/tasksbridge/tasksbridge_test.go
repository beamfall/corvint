package tasksbridge_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/mcp/tasksbridge"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fakeEnv turns this test binary into a fake corvint-tasks: it prints one
// envelope naming what it was run with, so the child-process contract is
// checked without the real binary (TAW-V0-025).
const fakeEnv = "TASKSBRIDGE_FAKE_TASKS"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeEnv); mode != "" {
		os.Exit(fakeTasks(mode))
	}
	os.Exit(m.Run())
}

// fakeTasks answers one run: "garbage" prints no envelope, "exit=N" prints
// a REFUSED envelope and exits N, anything else prints OK and exits 0.
func fakeTasks(mode string) int {
	if mode == "garbage" {
		_, _ = os.Stdout.WriteString("not an envelope\n")
		return 0
	}
	stdin, _ := io.ReadAll(os.Stdin)
	cwd, _ := os.Getwd()
	outcome, code := "OK", 0
	if value, ok := strings.CutPrefix(mode, "exit="); ok {
		outcome, code = "REFUSED", 3
		if n, err := strconv.Atoi(value); err == nil {
			code = n
		}
	}
	raw, _ := json.Marshal(map[string]any{"outcome": outcome, "args": os.Args[1:], "cwd": cwd, "role": os.Getenv(wire.RoleEnv), "stdin": string(stdin)})
	_, _ = os.Stdout.Write(append(raw, '\n'))
	return code
}

// The fixture queue and policy files (internal/tasks/fixture QueueBytes and
// PolicyBytes, which this package may not import).
const (
	queueFile  = `{"canonicalWriter":"NATIVE","executionCutover":null,"fixture":true,"foreignAdapterId":null,"importMapSha256":null,"intentBranch":"main","nextSerial":"2","prefix":"AT","profile":"taskman-queue/0","queueId":"queue:acme:main","repositoryAuthorityId":"repo:acme","schemaVersion":"0","writeBarrier":{"reason":"NONE","since":null}}` + "\n"
	policyFile = `{"allowEmptyObligationsKinds":["CHORE"],"budgets":{"lane":{"cacheCreationTokens":"1000000","cacheReadTokens":"100000000","inputTokens":"2000000","outputTokens":"400000","turns":"400","wallClockMinutes":"90"},"requireEnforcedFields":["turns","wallClockMinutes"],"ticketMultiplier":"4"},"capacity":{"classes":[],"maxActiveAttempts":"1","maxWorkersTotal":"4"},"cemRequired":false,"docsLane":{"required":false},"environment":{"allowedEnvKeys":[]},"gates":[{"argv":["make","verify"],"cwd":"WORKTREE","env":[],"evidence":[],"expected":{"exitCode":"0","reducer":null},"gateId":"verify","inputs":[],"kind":"COMMAND","required":true,"reusable":true,"sharedResource":null,"timeoutSeconds":"1800"}],"integrationRequiredKinds":[],"ocmRequired":false,"policyVersion":"1","profile":"taskman-policy/0","retention":{"evidenceDays":"90"},"retries":{"admissionsPerRevision":"3","gateRerunOnStale":"1","malformedReviewRetry":"1","reconcileAttempts":"3","repairRounds":"2"},"reviewLane":{"required":true},"roles":{},"runtimes":[],"serialFallback":"BLOCK"}` + "\n"
)

const createPayload = `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],` +
	`"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,` +
	`"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS",` +
	`"kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2",` +
	`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,` +
	`"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,` +
	`"supersedes":null,"title":"TITLE"}`

// skipUnsupported skips where Tasks refuses to run (CTS-V0-025).
func skipUnsupported(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("corvint-tasks refuses Windows")
	}
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// buildTasks builds cmd/corvint-tasks into a temporary directory.
func buildTasks(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	build := exec.Command("go", "build", "-buildvcs=false", "-o", dir, "../../../cmd/corvint-tasks")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build corvint-tasks: %v: %s", err, output)
	}
	return filepath.Join(dir, tasksbridge.BinaryName)
}

// tasks runs corvint-tasks with the legacy (unset) class, as the store
// owner sets the fixture up.
func tasks(t *testing.T, binary, root string, args ...string) map[string]any {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Dir = root
	command.Env = append(os.Environ(), wire.RoleEnv+"=")
	stdout, err := command.Output()
	var envelope map[string]any
	if jsonErr := json.Unmarshal(stdout, &envelope); jsonErr != nil || envelope["outcome"] != "OK" {
		t.Fatalf("%v: %v %s", args, err, stdout)
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

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// store is an initialized fixture store in a Git repository with two
// tickets in milestone m1 and a release v1 holding the first.
func store(t *testing.T, binary string) (root string, tickets []string) {
	t.Helper()
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv(wire.RoleEnv, "")
	root = filepath.Join(resolvedTempDir(t), "repo")
	if err := os.MkdirAll(filepath.Join(root, ".taskman"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"queue.json": queueFile, "policy.json": policyFile} {
		if err := os.WriteFile(filepath.Join(root, ".taskman", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "base")
	tasks(t, binary, root, "init")
	for _, title := range []string{"bridge deliver", "bridge block"} {
		payload := strings.Replace(strings.Replace(createPayload, "TITLE", title, 1), `"milestone":null`, `"milestone":"m1"`, 1)
		created := tasks(t, binary, root, "ticket", "create", "--request-id", "create-"+strings.ReplaceAll(title, " ", "-"), "--issued-at", "2026-11-01T09:00:00Z", "--payload", payload)
		tickets = append(tickets, firstItem(t, created)["ticketId"].(string))
	}
	release := `{"acceptanceCriteria":["release criterion"],"predecessorReleaseIds":[],"requiredGates":[],"ticketIds":["` + tickets[0] + `"],"title":"Version 1","version":"v1"}`
	tasks(t, binary, root, "release", "create", "--request-id", "rel", "--target", "v1", "--issued-at", "2026-11-01T09:01:00Z", "--payload", release)
	return root, tickets
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
	for class, want := range map[string][]string{wire.ClassAgent: agent, wire.ClassOperator: host, wire.ClassOwner: host} {
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
	agentGroup := tasksbridge.Group{Root: t.TempDir(), Class: wire.ClassAgent}
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

// TestTAWV0015_CallsEachTool calls every tasks tool through a corvint-tasks
// built from this tree over a fixture store: the lane verbs return
// trailers, a delivered trailer on main is completed by close-merged, a
// finding is filed and release-sync adds the milestone member (TAW-V0-015,
// TAW-V0-025).
func TestTAWV0015_CallsEachTool(t *testing.T) {
	skipUnsupported(t)
	binary := buildTasks(t)
	root, tickets := store(t, binary)
	local := func(id string) string { return id[strings.LastIndex(id, ":")+1:] }
	agent := tasksbridge.Group{Root: root, Class: wire.ClassAgent, Binary: binary}
	operator := tasksbridge.Group{Root: root, Class: wire.ClassOperator, Binary: binary}

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

// toolError runs one tool that must fail as a tool error and returns its
// structured content.
func toolError(t *testing.T, group tasksbridge.Group, name string, arguments map[string]any) map[string]any {
	t.Helper()
	result, rpcErr := group.Call(context.Background(), name, arguments)
	if rpcErr != nil || result["isError"] != true {
		t.Fatalf("%s: want a tool error, got %v %v", name, result, rpcErr)
	}
	return result["structuredContent"].(map[string]any)
}

// TestTAWV0025_ChildProcessContract: a call runs the resolved binary in the
// root with the verb's argv, the payload on stdin and the group's class in
// CORVINT_TASKS_ROLE, whatever the caller's environment says; a nonzero
// exit is a tool error carrying the envelope, and output without one is
// TASKS_ENVELOPE_UNREADABLE.
func TestTAWV0025_ChildProcessContract(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := resolvedTempDir(t)
	group := tasksbridge.Group{Root: root, Class: wire.ClassAgent, Binary: self}
	t.Setenv(wire.RoleEnv, wire.ClassOwner)
	t.Setenv(fakeEnv, "ok")
	payload := map[string]any{"title": "a <finding>"}
	ran := call(t, group, tasksbridge.ToolFinding, map[string]any{"payload": payload, "requestId": "r1"})
	if got := ran["args"].([]any); mustJSON(t, got) != `["work","finding","--payload","-","--request-id","r1"]` {
		t.Fatalf("argv %v", got)
	}
	if ran["cwd"] != root || ran["role"] != wire.ClassAgent || ran["stdin"] != "{\"title\":\"a <finding>\"}\n" {
		t.Fatalf("child context %v", ran)
	}

	t.Setenv(fakeEnv, "exit=3")
	refused := toolError(t, group, tasksbridge.ToolStart, map[string]any{"tickets": []any{"V1-0001"}, "lane": "l", "allowUnknown": true})
	if refused["outcome"] != "REFUSED" || mustJSON(t, refused["args"]) != `["work","start","V1-0001","--lane","l","--allow-unknown"]` {
		t.Fatalf("refused run %v", refused)
	}

	t.Setenv(fakeEnv, "garbage")
	if got := toolError(t, group, tasksbridge.ToolStart, map[string]any{"tickets": []any{"V1-0001"}}); got["code"] != tasksbridge.CodeEnvelopeUnreadable || got["profile"] != tasksbridge.ErrorProfile {
		t.Fatalf("unreadable envelope %v", got)
	}
}

// TestTAWV0025_BinaryUnavailable: with no corvint-tasks found, or one that
// cannot be started, a valid call is the TASKS_BINARY_UNAVAILABLE tool
// error; invalid arguments stay invalid params.
func TestTAWV0025_BinaryUnavailable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), tasksbridge.BinaryName)
	for _, binary := range []string{"", missing} {
		group := tasksbridge.Group{Root: t.TempDir(), Class: wire.ClassAgent, Binary: binary}
		got := toolError(t, group, tasksbridge.ToolStart, map[string]any{"tickets": []any{"V1-0001"}})
		if got["code"] != tasksbridge.CodeBinaryUnavailable || got["tool"] != tasksbridge.ToolStart {
			t.Fatalf("binary %q: %v", binary, got)
		}
		if _, rpcErr := group.Call(context.Background(), tasksbridge.ToolStart, map[string]any{}); rpcErr == nil {
			t.Fatalf("binary %q: empty arguments accepted", binary)
		}
	}
}

// TestTAWV0025_Locate: corvint-tasks beside the resolved running executable
// wins over PATH, PATH is the fallback, and neither yields "".
func TestTAWV0025_Locate(t *testing.T) {
	skipUnsupported(t)
	install := resolvedTempDir(t)
	executable := filepath.Join(install, "corvint-mcp")
	sibling := filepath.Join(install, tasksbridge.BinaryName)
	for _, path := range []string{executable, sibling} {
		if err := execfile.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(resolvedTempDir(t), "corvint-mcp")
	if err := os.Symlink(executable, link); err != nil {
		t.Fatal(err)
	}
	onPath := func(string) (string, error) { return "/path/corvint-tasks", nil }
	absent := func(string) (string, error) { return "", exec.ErrNotFound }
	bare := filepath.Join(resolvedTempDir(t), "corvint-mcp")
	for _, c := range []struct {
		executable string
		lookPath   func(string) (string, error)
		want       string
	}{
		{executable, absent, sibling},
		{link, onPath, sibling},
		{bare, onPath, "/path/corvint-tasks"},
		{"", onPath, "/path/corvint-tasks"},
		{bare, absent, ""},
	} {
		if got := tasksbridge.LocateFrom(c.executable, c.lookPath); got != c.want {
			t.Fatalf("locate(%q) = %q, want %q", c.executable, got, c.want)
		}
	}
	if got := tasksbridge.Locate(); got != "" && filepath.Base(got) != tasksbridge.BinaryName {
		t.Fatalf("Locate named another binary: %q", got)
	}
}
