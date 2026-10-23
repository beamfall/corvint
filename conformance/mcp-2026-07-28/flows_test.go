package mcp20260728

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/flowcoverage/testfixture"
)

var flowsArguments = []string{"--tool-profile", "flows"}
var flowsToolNames = []string{"corvint.flows.coverage", "corvint.flows.gaps", "corvint.flows.impact", "corvint.flows.map", "corvint.flows.navigate", "corvint.help", "corvint.impact", "corvint.query", "corvint.status"}

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

// V1-0280: corvint.flows.coverage, the fifth flows tool, needs a committed denominator and run
// receipts, so it runs against the flowcoverage fixture: compiled stdio traffic returns its page
// only inside the untrusted envelope, writes nothing, and (with the pinned schema) matches the
// official CallToolResult definition.
func TestAFUV1034FlowsCoverageCompiledConformance(t *testing.T) {
	root := testfixture.Repository(t)
	head := gitOutput(t, root, "rev-parse", "HEAD")
	before := treeDigest(t, root)
	coverage := flowCall("coverage", map[string]any{"denominator": "denominator.json", "receipts": "runs.json", "offset": 0, "limit": 1})
	client := startServerWithArguments(t, root, flowsArguments)
	defer client.close(t)
	if got := listedToolNames(t, client, requestMeta()); !reflect.DeepEqual(got, flowsToolNames) {
		t.Fatalf("catalogue %v", got)
	}
	result := successResult(t, client.call(t, 1, "tools/call", coverage))
	if result["isError"] == true || result["structuredContent"] != nil {
		t.Fatalf("invalid coverage envelope: %s", canonicalJSON(result))
	}
	text := object(t, result["content"].([]any)[0])["text"].(string)
	if !strings.HasPrefix(text, "BEGIN CORVINT REPOSITORY DATA\n") || !strings.HasSuffix(strings.TrimRight(text, "\n"), "\nEND CORVINT REPOSITORY DATA") {
		t.Fatalf("unframed coverage: %s", text)
	}
	assertCoveragePage(t, text, head)
	assertErrorCode(t, client.call(t, 2, "tools/call", flowCall("coverage", map[string]any{"denominator": "../escape", "receipts": "runs.json"})), -32602)
	if after := treeDigest(t, root); after != before {
		t.Fatal("coverage read wrote under repository or .git")
	}
	if os.Getenv(officialSchemaEnv) == "" {
		return
	}
	schema := loadOfficialSchema(t)
	checkOfficialExchanges(t, schema, client, 10, []schemaExchange{
		{"CallToolRequest", "CallToolResultResponse", "tools/call", coverage},
		{"CallToolRequest", "JSONRPCErrorResponse", "tools/call", flowCall("coverage", map[string]any{"denominator": "../escape", "receipts": "runs.json"})},
	})
}

// assertCoveragePage decodes the one JSON document inside the untrusted envelope and requires the
// fixture's exact first page: a READY, non-abstaining bridge result bound to HEAD whose
// application-flow-coverage/1 receipt carries the single uncovered "search" row and closed
// pagination (offset 0, total 1, no next page).
func assertCoveragePage(t *testing.T, text, head string) {
	t.Helper()
	var document string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "{") {
			if document != "" {
				t.Fatalf("coverage envelope holds more than one document: %s", text)
			}
			document = line
		}
	}
	var page struct {
		Abstention struct {
			Active bool   `json:"active"`
			Reason string `json:"reason"`
		} `json:"abstention"`
		Mutates bool `json:"mutates"`
		Receipt struct {
			Gaps       int    `json:"gaps"`
			NextOffset *int   `json:"next_offset"`
			Offset     int    `json:"offset"`
			Passed     bool   `json:"passed"`
			ReportSHA  string `json:"report_sha256"`
			Revision   string `json:"revision"`
			Schema     string `json:"schema"`
			Total      int    `json:"total"`
			Rows       []struct {
				DocumentedID string `json:"documented_id"`
				IntentFlowID string `json:"intent_flow_id"`
				Status       string `json:"status"`
			} `json:"rows"`
		} `json:"receipt"`
		Repository struct {
			CommitRevision string `json:"commitRevision"`
		} `json:"repository"`
		Schema string `json:"schema"`
		State  string `json:"state"`
		Tool   string `json:"tool"`
	}
	decoder := json.NewDecoder(strings.NewReader(document))
	if err := decoder.Decode(&page); err != nil {
		t.Fatalf("coverage document is not JSON: %v: %s", err, document)
	}
	receipt := page.Receipt
	if page.Schema != "corvint-mcp-bridge-result/0" || page.Tool != "corvint.flows.coverage" || page.State != "READY" || page.Abstention.Active || page.Abstention.Reason != "NONE" || page.Mutates || page.Repository.CommitRevision != head {
		t.Fatalf("coverage bridge result is not a READY HEAD-bound read: %s", document)
	}
	if receipt.Schema != "application-flow-coverage/1" || receipt.Revision != head || len(receipt.ReportSHA) != 64 {
		t.Fatalf("coverage receipt schema/revision/report digest: %s", document)
	}
	if receipt.Offset != 0 || receipt.Total != 1 || receipt.NextOffset != nil || len(receipt.Rows) != 1 || receipt.Gaps != 1 || receipt.Passed {
		t.Fatalf("coverage pagination or totals differ from the fixture: %s", document)
	}
	if row := receipt.Rows[0]; row.DocumentedID != "search" || row.IntentFlowID != "search" || row.Status != "MISSING_TEST" {
		t.Fatalf("coverage row differs from the fixture's uncovered intent: %s", document)
	}
}
