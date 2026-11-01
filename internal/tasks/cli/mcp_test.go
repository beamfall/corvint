package cli_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
)

// tasksMCPSession serves `corvint-tasks mcp` over the 2025-11-25 handshake in
// root and returns each response by id.
func tasksMCPSession(t *testing.T, root string, requests ...string) map[float64]map[string]any {
	t.Helper()
	lines := []string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}
	for index, request := range requests {
		lines = append(lines, `{"jsonrpc":"2.0","id":`+string(rune('1'+index))+`,`+request+`}`)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: root, Args: []string{"mcp", "--protocol-version", "2025-11-25"}, Stdin: strings.NewReader(strings.Join(lines, "\n") + "\n"), Stdout: &stdout, Stderr: &stderr}); code != 0 {
		t.Fatalf("mcp exit %d stderr %q", code, stderr.String())
	}
	responses := map[float64]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		id, _ := response["id"].(float64)
		responses[id] = response
	}
	return responses
}

// TMR-V0-950..953: the read-only profile lists exactly five typed read-only
// tools, answers each with the CLI read verb's own envelope, pages the roadmap
// and ticket search, refuses a schema-invalid argument with -32602 data, and
// reports a CLI refusal as isError carrying the CLI codes.
func TestTMRV0950ReadOnlyTasksMCPProfile(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, first := tcdTicket(t, root, "alpha", "")
	tcdTicket(t, root, "beta", "")
	before := atm(t, root, nil, "queue", "status").stdout
	responses := tasksMCPSession(t, root,
		`"method":"tools/list","params":{}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.ticket_search","arguments":{"text":"alpha","limit":1}}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.roadmap","arguments":{"offset":1,"limit":1}}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.ticket_show","arguments":{"ticket":"`+first+`"}}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.queue_status","arguments":{}}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.ticket_show","arguments":{"ticket":"V9-9999"}}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.ticket_search","arguments":{"priority":"P9"}}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.claim","arguments":{}}`,
		`"method":"tools/call","params":{"name":"corvint.tasks.roadmap","arguments":{"limit":0}}`,
	)
	result := func(id float64) map[string]any {
		value, _ := responses[id]["result"].(map[string]any)
		return value
	}
	var names []string
	for _, raw := range result(1)["tools"].([]any) {
		tool := raw.(map[string]any)
		names = append(names, tool["name"].(string))
		schema := tool["inputSchema"].(map[string]any)
		annotations := tool["annotations"].(map[string]any)
		if schema["additionalProperties"] != false || annotations["readOnlyHint"] != true || annotations["destructiveHint"] != false || tool["outputSchema"] == nil {
			t.Fatalf("%s descriptor %+v", tool["name"], tool)
		}
	}
	if want := []string{"corvint.tasks.queue_status", "corvint.tasks.receipt_audit", "corvint.tasks.roadmap", "corvint.tasks.ticket_search", "corvint.tasks.ticket_show"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("tools %q", names)
	}
	page := func(id float64) map[string]any {
		structured := result(id)["structuredContent"].(map[string]any)
		if result(id)["isError"] != false || structured["outcome"] != "OK" || structured["profile"] != "taskman-command-result/0" {
			t.Fatalf("id %v: %+v", id, result(id))
		}
		value, _ := structured["page"].(map[string]any)
		return value
	}
	if search := page(2); search["limit"] != "1" || search["total"] != "1" {
		t.Fatalf("search page %+v", search)
	}
	if roadmap := page(3); roadmap["offset"] != "1" || roadmap["limit"] != "1" {
		t.Fatalf("roadmap page %+v", roadmap)
	}
	page(4)
	page(5)
	// The CLI's own envelope, byte for byte, is the enveloped text block.
	text := result(5)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "BEGIN CORVINT REPOSITORY DATA") || !strings.Contains(text, strings.TrimSpace(string(atm(t, root, nil, "queue", "status").stdout))[:40]) {
		t.Fatalf("queue text %q", text)
	}
	refused := result(6)["structuredContent"].(map[string]any)
	envelope, _ := refused["envelope"].(map[string]any)
	if result(6)["isError"] != true || refused["code"] != "CLI_REFUSED" || refused["profile"] != "corvint-tasks-mcp-tool-error/0" ||
		envelope["outcome"] == "OK" || !reflect.DeepEqual(refused["codes"], envelope["codes"]) {
		t.Fatalf("unknown ticket %+v", result(6))
	}
	for id, want := range map[float64]map[string]any{
		7: {"reason": "invalid-enum", "field": "arguments.priority", "allowed": []any{"P0", "P1", "P2", "P3", "P4"}},
		8: {"reason": "unknown-tool", "field": "name", "allowed": []any{"corvint.tasks.queue_status", "corvint.tasks.receipt_audit", "corvint.tasks.roadmap", "corvint.tasks.ticket_search", "corvint.tasks.ticket_show"}},
		9: {"reason": "out-of-range", "field": "arguments.limit", "rule": "minimum 1"},
	} {
		failure, _ := responses[id]["error"].(map[string]any)
		if failure["code"] != float64(-32602) || !reflect.DeepEqual(failure["data"], want) {
			t.Fatalf("id %v error %+v", id, failure)
		}
	}
	// TMR-V0-952: the profile reads only; the queue is unchanged.
	if after := atm(t, root, nil, "queue", "status").stdout; !bytes.Equal(withoutObservedAt(before), withoutObservedAt(after)) {
		t.Fatalf("queue changed:\n%s\n%s", before, after)
	}
}

func withoutObservedAt(raw []byte) []byte {
	index := bytes.Index(raw, []byte(`"observedAt":"`))
	if index < 0 {
		return raw
	}
	return append(append([]byte{}, raw[:index]...), raw[index+len(`"observedAt":"2026-11-01T00:00:00Z"`):]...)
}

// TMR-V0-950: any argument other than --protocol-version exits 2.
func TestTMRV0950InvalidArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: []string{"mcp", "--root", "/x"}, Stdout: &stdout, Stderr: &stderr}); code != 2 || stderr.String() != "corvint-tasks mcp: invalid arguments\n" || stdout.Len() != 0 {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}
