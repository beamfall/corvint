package tasksread

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

const okEnvelope = `{"codes":[],"command":["ticket","search"],"items":[{"title":"Alpha"}],"outcome":"OK","page":{"limit":"1","offset":"0","total":"2","truncated":true},"profile":"taskman-command-result/0"}` + "\n"

// TMR-V0-951: exactly five closed tool descriptors (toolgroup.Tasks adds
// the read-only annotations, which TestTMRV0950TasksGroupRunsTheInstalledReadVerb checks).
func TestTMRV0951FiveReadOnlyDescriptors(t *testing.T) {
	var names []string
	for _, tool := range (Group{}).Descriptors() {
		names = append(names, tool.Name)
		if tool.InputSchema["additionalProperties"] != false || tool.InputSchema["$schema"] == nil {
			t.Fatalf("%s descriptor %+v", tool.Name, tool)
		}
	}
	if want := []string{"corvint.tasks.queue_status", "corvint.tasks.receipt_audit", "corvint.tasks.roadmap", "corvint.tasks.ticket_search", "corvint.tasks.ticket_show"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("tools %q", names)
	}
}

// TMR-V0-952, TMR-V0-953: arguments become the verb's flags in the root, an
// OK envelope is structuredContent and the enveloped text, a refusal is
// CLI_REFUSED with the CLI's codes, a run failure is TASKS_UNAVAILABLE,
// unreadable output is TASKS_OUTPUT_UNREADABLE, and a schema-invalid argument
// is -32602 with data before anything runs.
func TestTMRV0952CallsRunTheReadVerb(t *testing.T) {
	var ran [][]string
	reply, failure := []byte(okEnvelope), error(nil)
	group := Group{Root: "/repo", Run: func(_ context.Context, root string, argv []string) ([]byte, error) {
		if root != "/repo" {
			t.Fatalf("root %q", root)
		}
		ran = append(ran, argv)
		return reply, failure
	}}
	result, refusal := group.Call(context.Background(), "corvint.tasks.ticket_search", map[string]any{"text": "alpha", "limit": json.Number("1"), "offset": json.Number("0")})
	if refusal != nil || result["isError"] != false || !reflect.DeepEqual(ran[0], []string{"ticket", "search", "--limit", "1", "--offset", "0", "--text", "alpha"}) {
		t.Fatalf("search refusal=%v result=%v ran=%q", refusal, result, ran)
	}
	page := result["structuredContent"].(map[string]any)["page"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if page["total"] != "2" || !strings.Contains(text, "BEGIN CORVINT REPOSITORY DATA") || !strings.Contains(text, `"title":"Alpha"`) {
		t.Fatalf("page %v text %q", page, text)
	}
	if _, refusal := group.Call(context.Background(), "corvint.tasks.ticket_show", map[string]any{"ticket": "V1-0001"}); refusal != nil || !reflect.DeepEqual(ran[1], []string{"ticket", "show", "V1-0001"}) {
		t.Fatalf("show ran %q", ran)
	}
	reply = []byte(`{"codes":["NOT_FOUND"],"outcome":"ERROR","profile":"taskman-command-result/0"}`)
	result, _ = group.Call(context.Background(), "corvint.tasks.ticket_show", map[string]any{"ticket": "V9-9999"})
	structured := result["structuredContent"].(map[string]any)
	if result["isError"] != true || structured["code"] != "CLI_REFUSED" || !reflect.DeepEqual(structured["codes"], []any{"NOT_FOUND"}) || structured["profile"] != ToolErrorProfile {
		t.Fatalf("refused %+v", result)
	}
	for code, run := range map[string]func(){
		"TASKS_UNAVAILABLE":       func() { reply, failure = nil, errors.New("executable file not found") },
		"TASKS_OUTPUT_UNREADABLE": func() { reply, failure = []byte("not json"), nil },
	} {
		run()
		result, _ = group.Call(context.Background(), "corvint.tasks.queue_status", map[string]any{})
		if result["isError"] != true || result["structuredContent"].(map[string]any)["code"] != code {
			t.Fatalf("%s: %+v", code, result)
		}
	}
	before := len(ran)
	for arguments, want := range map[string]protocol.InvalidParamsData{
		`{"limit":0}`:            {Reason: "out-of-range", Field: "arguments.limit", Rule: "minimum 1"},
		`{"limit":1.5}`:          {Reason: "invalid-type", Field: "arguments.limit", Rule: "integer"},
		`{"priority":"high"}`:    {Reason: "pattern-mismatch", Field: "arguments.priority", Rule: `^P[0-9]$`},
		`{"page":1}`:             {Reason: "unknown-argument", Field: "arguments.page", Allowed: []any{"kind", "label", "limit", "milestone", "offset", "owner", "priority", "status", "text"}},
		`{"text":"line\nbreak"}`: {Reason: "pattern-mismatch", Field: "arguments.text", Rule: `^[^\x00-\x1f\x7f]+$`},
	} {
		var decoded map[string]any
		decoder := json.NewDecoder(strings.NewReader(arguments))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err != nil {
			t.Fatal(err)
		}
		_, refusal := group.Call(context.Background(), "corvint.tasks.ticket_search", decoded)
		if refusal == nil || refusal.Code != protocol.CodeInvalidParams || !reflect.DeepEqual(refusal.Data, want) {
			t.Fatalf("%s: %+v", arguments, refusal)
		}
	}
	if _, refusal := group.Call(context.Background(), "corvint.tasks.ticket_show", map[string]any{}); refusal == nil {
		t.Fatal("missing ticket accepted")
	}
	if len(ran) != before {
		t.Fatal("an invalid argument ran the verb")
	}
}
