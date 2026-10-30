package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// DCP-V1-653..655 (V1-3080, GitHub #1977): docs_search answers with the
// compact receipt by default, in both the structured content and the framed
// text; it is exactly the compaction of the full receipt the same call returns
// with receipt "full", it carries capabilities only on request, and the new
// arguments are closed.
func TestDCPV1653SearchCompactReceiptOnTheWire(t *testing.T) {
	t.Parallel()
	root := corpusRoot(t)
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	calls := []string{
		`{"name":"corvint.docs_search","arguments":{"query":"Original"}}`,
		`{"name":"corvint.docs_search","arguments":{"query":"Original","receipt":"full"}}`,
		`{"name":"corvint.docs_search","arguments":{"query":"Original","include":["capabilities"]}}`,
		`{"name":"corvint.docs_search","arguments":{"query":"Original","receipt":"compact","include":["claims","capabilities"]}}`,
		`{"name":"corvint.docs_search","arguments":{"query":"Original","receipt":"brief"}}`,
		`{"name":"corvint.docs_search","arguments":{"query":"Original","include":["capabilities","capabilities"]}}`,
		`{"name":"corvint.docs_search","arguments":{"query":"Original","include":[]}}`,
		`{"name":"corvint.docs_get","arguments":{"id":"native:file:source.md","receipt":"full"}}`,
	}
	input := ""
	for i, call := range calls {
		input += `{"jsonrpc":"2.0","id":` + string(rune('0'+i)) + `,"method":"tools/call","params":{` + meta + `,` + strings.TrimPrefix(call, "{") + "}\n"
	}
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--root", root, "--artifact", "corpus.json"}, strings.NewReader(input), &stdout, &stderr); exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	responses := map[float64]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatal(err)
		}
		id, ok := response["id"].(float64)
		if !ok {
			t.Fatalf("response without id: %s", line)
		}
		responses[id] = response
	}
	structured := func(id float64) map[string]any {
		result, _ := responses[id]["result"].(map[string]any)
		if result == nil || result["isError"] != false {
			t.Fatalf("call %v: %v", id, responses[id])
		}
		return result["structuredContent"].(map[string]any)
	}
	full := structured(1)
	if full["receipt_view"] != nil || full["capabilities"] == nil {
		t.Fatalf("the full receipt changed: %v", full)
	}
	want, err := corpusbridge.CompactReceipt(full, false)
	if err != nil {
		t.Fatal(err)
	}
	compact := structured(0)
	if mustJSON(t, compact) != mustJSON(t, want) {
		t.Fatalf("default receipt is not the compaction of the full one:\n%s\n%s", mustJSON(t, compact), mustJSON(t, want))
	}
	encoded, _ := json.Marshal(want)
	framed, _ := repoenvelope.Frame(string(encoded))
	if text := responses[0]["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"]; text != framed {
		t.Fatalf("framed text is not the compact receipt: %v", text)
	}
	if len(mustJSON(t, compact)) >= len(mustJSON(t, full)) {
		t.Fatal("the compact receipt is not smaller")
	}
	if capabilities := structured(2)["capabilities"]; !reflect.DeepEqual(capabilities, full["capabilities"]) || structured(2)["receipt_view"] != "compact" {
		t.Fatalf("include capabilities: %v", capabilities)
	}
	if both := structured(3); both["capabilities"] == nil || both["receipt_view"] != "compact" {
		t.Fatalf("claims and capabilities: %v", both)
	}
	for id := 4.0; id <= 7; id++ {
		if errorBody, _ := responses[id]["error"].(map[string]any); errorBody == nil || errorBody["code"] != float64(-32602) {
			t.Fatalf("call %v admitted: %v", id, responses[id])
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
