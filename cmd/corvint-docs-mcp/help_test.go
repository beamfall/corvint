package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// MCPV0-101: corvint-docs-mcp advertises bridge.HelpDescriptor() and answers
// corvint.help on the wire, under both protocol versions, with exactly the
// result the shared bridge path builds (MCPV0-104): the embedded
// `corvint help --json` and `corvint help guide` bytes in structuredContent and
// in the enveloped text block. cmd/corvint's TestMCPV0105CompanionHelpMatchesTheCLI
// binds those bytes to the CLI's stdout.
func TestMCPV0101HelpToolAnswersOnTheWire(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"2026-07-28", "2025-11-25"} {
		root := bareGitRoot(t)
		meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}},`
		input := ""
		if version == "2025-11-25" {
			meta = ""
			input = `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"test"}}}` + "\n" +
				`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
		}
		input += `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{` + strings.TrimSuffix(meta, ",") + `}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{` + meta + `"name":"corvint.help","arguments":{"topic":"catalog"}}}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{` + meta + `"name":"corvint.help","arguments":{}}}` + "\n" +
			`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{` + meta + `"name":"corvint.help","arguments":{"topic":"COMMAND"}}}` + "\n"
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), append([]string{"--root", root}, "--protocol-version", version), strings.NewReader(input), &stdout, &stderr); exit != 0 {
			t.Fatalf("%s exit=%d stderr=%q", version, exit, stderr.String())
		}
		responses := map[float64]map[string]any{}
		for _, line := range strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n") {
			var response map[string]any
			if err := json.Unmarshal([]byte(line), &response); err != nil {
				t.Fatal(err)
			}
			id, _ := response["id"].(float64)
			responses[id] = response
		}
		listed, _ := responses[1]["result"].(map[string]any)
		wantDescriptor := marshalHelp(t, bridge.WithOutputSchemas([]bridge.ToolDescriptor{bridge.HelpDescriptor()})[0]) // MCPV0-953
		found := false
		for _, tool := range listed["tools"].([]any) {
			if tool.(map[string]any)["name"] == bridge.ToolHelp {
				found = marshalHelp(t, tool) == wantDescriptor
			}
		}
		if !found {
			t.Fatalf("%s: tools/list lacks the shared corvint.help descriptor", version)
		}
		for id, call := range map[float64]struct{ arguments, text string }{
			2: {`{"topic":"catalog"}`, string(capcatalog.Snapshot())},
			3: {`{}`, capcatalog.Guide()}, // MCPV0-953: the guide is the default
		} {
			want, err := bridge.HelpToolResult(context.Background(), []byte(call.arguments))
			if err != nil {
				t.Fatal(err)
			}
			got, _ := responses[id]["result"].(map[string]any)
			// corvint.help carries no corvint/timing, only the transport _meta (MCPV0-104).
			transport, _ := got["_meta"].(map[string]any)
			if _, timed := transport["corvint/timing"]; timed {
				t.Fatalf("%s %s: help result carries corvint/timing", version, call.arguments)
			}
			// The transport adds these two members to every tool's result.
			delete(got, "_meta")
			delete(got, "resultType")
			if marshalHelp(t, got) != marshalHelp(t, want) {
				t.Fatalf("%s %s: wire result differs from the shared bridge result", version, call.arguments)
			}
			assertHelpText(t, got, call.text)
		}
		if failure, _ := responses[4]["error"].(map[string]any); failure["code"] != float64(-32602) {
			t.Fatalf("%s: unknown topic = %v, want -32602", version, responses[4])
		}
	}
}

// MCPV0-101: corvint.help reads only the binary, so it still answers after
// the repository root is gone, while the same valid docs_draft request that
// succeeded before the removal now refuses with repository-unavailable.
func TestMCPV0101HelpToolReadsNoRepositoryState(t *testing.T) {
	t.Parallel()
	root := docsFixtureRoot(t)
	registry, err := docsbridge.New(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := &toolHandler{registry: registry}
	draft := map[string]any{"name": docsbridge.ToolDraft, "arguments": map[string]any{"source": "owner.md", "package": "cache"}}
	if own, rpcErr := handler.call(context.Background(), draft); rpcErr != nil || own["isError"] != false {
		t.Fatalf("%s failed with its repository: rpc=%v result=%v", docsbridge.ToolDraft, rpcErr, own)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	result, rpcErr := handler.call(context.Background(), map[string]any{"name": bridge.ToolHelp, "arguments": map[string]any{"topic": "guide"}})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	assertHelpText(t, result, capcatalog.Guide())
	assertOwnToolRefused(t, handler, draft, "repository-unavailable")
	if _, rpcErr := handler.call(context.Background(), map[string]any{"name": bridge.ToolHelp, "arguments": map[string]any{"topic": nil}}); rpcErr == nil {
		t.Fatal("a null topic was accepted")
	}
}

// assertOwnToolRefused checks that the server's own tool, given a request it
// answered before the repository was removed, now refuses with code.
func assertOwnToolRefused(t *testing.T, handler *toolHandler, params map[string]any, code string) {
	t.Helper()
	own, rpcErr := handler.call(context.Background(), params)
	if rpcErr != nil {
		t.Fatalf("%s: rpc error %v, want tool failure %s", params["name"], rpcErr, code)
	}
	structured, _ := own["structuredContent"].(map[string]any)
	if own["isError"] != true || structured["code"] != code {
		t.Fatalf("%s answered without its repository: isError=%v code=%v, want %s", params["name"], own["isError"], structured["code"], code)
	}
}

// marshalHelp is value's JSON with object members in key order, so a typed
// descriptor and its decoded wire form compare equal.
func marshalHelp(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	var decoded any
	if err == nil {
		err = json.Unmarshal(raw, &decoded)
	}
	if err == nil {
		raw, err = json.Marshal(decoded)
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// assertHelpText checks the receipt text in structuredContent and inside the
// envelope-framed text block.
func assertHelpText(t *testing.T, result map[string]any, want string) {
	t.Helper()
	structured, _ := result["structuredContent"].(map[string]any)
	receipt, _ := structured["receipt"].(map[string]any)
	content, _ := result["content"].([]any)
	if result["isError"] != false || structured["authorityClass"] != "CORVINT_BINARY" || receipt["text"] != want || len(content) != 1 {
		t.Fatalf("result differs from the embedded help bytes: isError=%v authority=%v", result["isError"], structured["authorityClass"])
	}
	framed, _ := content[0].(map[string]any)["text"].(string)
	begin := strings.Index(framed, "\n{")
	end := strings.LastIndex(framed, "\n"+repoenvelope.Terminator)
	var text struct {
		Receipt struct {
			Text string `json:"text"`
		} `json:"receipt"`
	}
	if begin < 0 || end < begin || json.Unmarshal([]byte(framed[begin+1:end]), &text) != nil || text.Receipt.Text != want {
		t.Fatal("text block differs from the embedded help bytes")
	}
}

// bareGitRoot is a clean absolute root whose .git directory is empty: enough
// for the registry to start, and any repository read would fail.
func bareGitRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
