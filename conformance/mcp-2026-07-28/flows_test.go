package mcp20260728

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var flowsArguments = []string{"--tool-profile", "flows"}
var flowsToolNames = []string{"corvint.flows.coverage", "corvint.flows.gaps", "corvint.flows.impact", "corvint.flows.map", "corvint.flows.navigate", "corvint.impact", "corvint.query", "corvint.status"}

func flowsFixture(t *testing.T) string {
	t.Helper()
	root := fixtureRepository(t)
	if err := os.Mkdir(filepath.Join(root, "flows"), 0755); err != nil {
		t.Fatal(err)
	}
	raw := `{"schema":"application-flow-intent/1","flow_id":"counter","revision":1,"kind":"ui","actor":"caller","preconditions":[],"steps":[{"step_id":"click","action":"click counter"}],"outcomes":[{"outcome_id":"one","behavior":"counter is one","matcher":"toHaveText","locator":"#count","value":"1"}],"variations":[{"variation_id":"happy","preconditions":[],"steps":["click"],"observable_facts":[],"outcomes":["one"],"projects":["chromium"]}],"links":[]}`
	if err := os.WriteFile(filepath.Join(root, "flows/counter.json"), []byte(raw+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "flows")
	gitOutput(t, root, "commit", "-m", "declare counter flow")
	return root
}

func flowCall(name string, args map[string]any) map[string]any {
	return map[string]any{"_meta": requestMeta(), "name": "corvint.flows." + name, "arguments": args}
}

// AFU-V1-034 and AFU-V1-035: compiled stdio traffic preserves the opt-in registry,
// untrusted framing, read-only boundary and closed arguments on both protocols.
func TestAFUV1034FlowsCompiledConformance(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "2026-07-28", true: "2025-11-25"}[legacy], func(t *testing.T) {
			root := flowsFixture(t)
			head := gitOutput(t, root, "rev-parse", "HEAD")
			before := treeDigest(t, root)
			args := append([]string{}, flowsArguments...)
			meta := requestMeta()
			if legacy {
				args = append(args, "--protocol-version", "2025-11-25")
				meta = map[string]any{}
			}
			client := startServerWithArguments(t, root, args)
			defer client.close(t)
			if legacy {
				successResult(t, client.call(t, 1, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "conformance", "version": "test"}}))
				client.sendJSON(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
			}
			if got := listedToolNames(t, client, meta); !reflect.DeepEqual(got, flowsToolNames) {
				t.Fatalf("catalogue %v", got)
			}
			for i, verb := range []string{"map", "gaps", "navigate", "impact"} {
				params := flowCall(verb, map[string]any{"flows": "flows"})
				params["_meta"] = meta
				if verb == "impact" {
					params["arguments"].(map[string]any)["base"] = head
				}
				result := successResult(t, client.call(t, 10+i, "tools/call", params))
				if result["isError"] == true || result["structuredContent"] != nil {
					t.Fatalf("invalid flow envelope: %s", canonicalJSON(result))
				}
				content := result["content"].([]any)
				text := object(t, content[0])["text"].(string)
				if !strings.Contains(text, "BEGIN CORVINT REPOSITORY DATA") || (verb != "impact" && !strings.Contains(text, "counter")) || !strings.Contains(text, head) {
					t.Fatalf("unbound/unframed flow: %s", text)
				}
			}
			for i, args := range []map[string]any{{"flows": "../escape"}, {"flows": "flows", "unknown": true}, {"flows": "flows", "path": "pkg/value.go", "testKey": "counter"}} {
				params := flowCall("map", args)
				params["_meta"] = meta
				assertErrorCode(t, client.call(t, 30+i, "tools/call", params), -32602)
			}
			if after := treeDigest(t, root); after != before {
				t.Fatal("flow reads wrote under repository or .git")
			}
		})
	}
}

// AFU-V1-034: pinned official schema checks the new profile's actual requests and responses.
func TestAFUV1034FlowsOfficialSchema(t *testing.T) {
	schema := loadOfficialSchema(t)
	root := flowsFixture(t)
	head := gitOutput(t, root, "rev-parse", "HEAD")
	client := startServerWithArguments(t, root, flowsArguments)
	defer client.close(t)
	exchanges := []schemaExchange{{"ListToolsRequest", "ListToolsResultResponse", "tools/list", map[string]any{"_meta": requestMeta()}}}
	for _, verb := range []string{"map", "gaps", "navigate", "impact"} {
		args := map[string]any{"flows": "flows"}
		if verb == "impact" {
			args["base"] = head
		}
		exchanges = append(exchanges, schemaExchange{"CallToolRequest", "CallToolResultResponse", "tools/call", flowCall(verb, args)})
	}
	exchanges = append(exchanges, schemaExchange{"CallToolRequest", "JSONRPCErrorResponse", "tools/call", flowCall("map", map[string]any{"flows": "../escape"})})
	checkOfficialExchanges(t, schema, client, 1400, exchanges)
}
