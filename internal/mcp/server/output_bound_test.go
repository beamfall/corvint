package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// MCPV0-042 (V1-1999): a tools/call result over the message bound is the tool
// failure output-bound-exceeded naming the bound, under both protocol paths,
// never a generic -32603; another oversized result stays -32603.
func TestMCPV0042OversizedToolResultIsTypedRefusal(t *testing.T) {
	oversized := HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": strings.Repeat("x", protocol.MaxMessageBytes)}}}, nil
	})
	bound := fmt.Sprintf("exceeds the %d-byte MCP message bound", protocol.MaxMessageBytes)
	check := func(t *testing.T, response map[string]any) {
		t.Helper()
		result, ok := response["result"].(map[string]any)
		if !ok || result["isError"] != true {
			t.Fatalf("want a tool failure result, got %v", response["error"])
		}
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.HasPrefix(text, "output-bound-exceeded: tool result of ") || !strings.Contains(text, bound) {
			t.Fatalf("refusal text %q", text)
		}
	}

	var output bytes.Buffer
	call := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"t","arguments":{},` + requestMeta + `}}` + "\n"
	if err := newTestServer(t, oversized).Serve(context.Background(), strings.NewReader(call), &output); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatal(err)
	}
	check(t, response)
	if response["result"].(map[string]any)["resultType"] != "complete" {
		t.Fatal("refusal skipped result decoration")
	}

	results := legacyExchange(t, legacyServer(t, oversized), legacyInitialize, legacyInitializedNotification,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	check(t, results[1])
	assertErrorCode(t, results[2], protocol.CodeInternalError)
}
