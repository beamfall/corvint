package mcp20260728

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// taskReviewArguments selects the opt-in descendant profile
// corvint-mcp-2026-07-28-conformance/1 (MCPV0-026, decision 0374).
var taskReviewArguments = []string{"--tool-profile", "task-review"}

var (
	v0ToolNames         = []string{"corvint.impact", "corvint.query", "corvint.status"}
	taskReviewToolNames = []string{"corvint.cem.report", "corvint.context", "corvint.impact", "corvint.query", "corvint.status"}
)

func TestTaskReviewCaseInventoryIsClosed(t *testing.T) {
	type manifest struct {
		Profile          string   `json:"profile"`
		ParentProfile    string   `json:"parentProfile"`
		Selector         []string `json:"selector"`
		ProtocolVersions []string `json:"protocolVersions"`
		Tools            []string `json:"tools"`
		OfficialSchema   struct {
			ObservedSHA256   string `json:"observedSha256"`
			ValidationStatus string `json:"validationStatus"`
			ValidationReason string `json:"validationReason"`
		} `json:"officialSchema"`
		Cases []string `json:"cases"`
	}
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "conformance", "mcp-2026-07-28", "cases-task-review.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&got); err != nil {
		t.Fatal(err)
	}
	wantCases := []string{
		"selector-closed", "default-profile-unchanged", "task-review-tool-catalogue", "legacy-protocol-task-review",
		"context-cem-report-read-only", "context-cem-report-argument-rejection", "cem-map-escape-refusal",
		"cem-report-envelope-and-budget", "repository-filter-no-execution", "git-executable-pinned",
		"official-schema-traffic",
	}
	if got.Profile != "corvint-mcp-2026-07-28-conformance/1" || got.ParentProfile != "corvint-mcp-2026-07-28-conformance/0" ||
		!reflect.DeepEqual(got.Selector, taskReviewArguments) ||
		!reflect.DeepEqual(got.ProtocolVersions, []string{protocolVersion, "2025-11-25"}) ||
		!reflect.DeepEqual(got.Tools, taskReviewToolNames) ||
		got.OfficialSchema.ObservedSHA256 != officialSchemaSHA256 ||
		got.OfficialSchema.ValidationStatus != "OPT_IN" || got.OfficialSchema.ValidationReason == "" ||
		!reflect.DeepEqual(got.Cases, wantCases) {
		t.Fatalf("invalid task-review conformance manifest: %#v", got)
	}
}

// MCPV0-026: the selector is closed; a missing, unknown, differently cased or
// duplicate value, or the selector beside --version, fails before repository
// startup (the nonexistent root would otherwise report repository-unavailable).
func TestTaskReviewSelectorIsClosed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, arguments := range [][]string{
		{"--root", missing, "--tool-profile"},
		{"--root", missing, "--tool-profile", "default"},
		{"--root", missing, "--tool-profile", "TASK-REVIEW"},
		{"--root", missing, "--tool-profile", "task-review", "--tool-profile", "task-review"},
		{"--root", missing, "--tool-profile=task-review"},
		{"--version", "--tool-profile", "task-review"},
	} {
		command := exec.Command(serverBinary, arguments...)
		var stdout, stderr strings.Builder
		command.Stdout, command.Stderr = &stdout, &stderr
		var exitErr *exec.ExitError
		if err := command.Run(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 ||
			stdout.Len() != 0 || stderr.String() != "corvint-mcp: invalid arguments\n" {
			t.Fatalf("%q: err=%v stdout=%q stderr=%q", arguments, err, stdout.String(), stderr.String())
		}
	}
}

// Without the selector the server is exactly profile /0: three tools, and a
// call to either task-review tool fails like any other unknown tool.
func TestTaskReviewDefaultProfileUnchanged(t *testing.T) {
	root := fixtureRepository(t)
	head := gitOutput(t, root, "rev-parse", "HEAD")
	client := startServer(t, root)
	defer client.close(t)
	if got := listedToolNames(t, client, requestMeta()); !reflect.DeepEqual(got, v0ToolNames) {
		t.Fatalf("default tools=%v want %v", got, v0ToolNames)
	}
	for index, call := range []map[string]any{
		{"name": "corvint.context", "arguments": map[string]any{"task": "change the fixture Value"}},
		{"name": "corvint.cem.report", "arguments": map[string]any{"map": "missing.cem.json", "expectedBase": head, "target": head}},
	} {
		call["_meta"] = requestMeta()
		assertErrorCode(t, client.call(t, 800+index, "tools/call", call), -32602)
	}
}

func TestTaskReviewToolCatalogue(t *testing.T) {
	client := startServerWithArguments(t, fixtureRepository(t), taskReviewArguments)
	defer client.close(t)
	result := successResult(t, client.call(t, 810, "tools/list", map[string]any{"_meta": requestMeta()}))
	if result["resultType"] != "complete" || result["nextCursor"] != nil || result["cacheScope"] != "private" {
		t.Fatalf("tool list envelope=%s", canonicalJSON(result))
	}
	wantAnnotations := map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
	tools, _ := result["tools"].([]any)
	names := make([]string, 0, len(tools))
	for _, value := range tools {
		tool := object(t, value)
		name, _ := tool["name"].(string)
		names = append(names, name)
		schema := object(t, tool["inputSchema"])
		if schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("tool %s input schema is not closed: %s", name, canonicalJSON(schema))
		}
		if annotations := object(t, tool["annotations"]); !reflect.DeepEqual(annotations, wantAnnotations) {
			t.Fatalf("tool %s annotations=%s", name, canonicalJSON(annotations))
		}
	}
	if !reflect.DeepEqual(names, taskReviewToolNames) {
		t.Fatalf("tool order/names=%v want=%v", names, taskReviewToolNames)
	}
	assertServerInfo(t, result)
	assertErrorCode(t, client.call(t, 811, "tools/call", map[string]any{
		"_meta": requestMeta(), "name": "corvint.evidence", "arguments": map[string]any{},
	}), -32602)
}

// MCPV0-021 and MCPV0-026 compose: the explicit 2025-11-25 profile lists the
// same registry the selector chose, and a task-review call succeeds.
func TestTaskReviewLegacyProtocol(t *testing.T) {
	for _, test := range []struct {
		arguments []string
		want      []string
	}{
		{[]string{"--protocol-version", "2025-11-25"}, v0ToolNames},
		{append([]string{"--protocol-version", "2025-11-25"}, taskReviewArguments...), taskReviewToolNames},
		{append(append([]string{}, taskReviewArguments...), "--protocol-version", "2025-11-25"), taskReviewToolNames},
	} {
		client := startServerWithArguments(t, fixtureRepository(t), test.arguments)
		initialized := client.call(t, 0, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "conformance", "version": "test"}})
		if initialized["error"] != nil {
			t.Fatal(canonicalJSON(initialized))
		}
		client.sendJSON(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
		if got := listedToolNames(t, client, map[string]any{}); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%q tools=%v want %v", test.arguments, got, test.want)
		}
		context := client.call(t, 821, "tools/call", map[string]any{
			"_meta": map[string]any{}, "name": "corvint.context", "arguments": map[string]any{"task": "change the fixture Value"},
		})
		if len(test.want) == len(v0ToolNames) {
			assertErrorCode(t, context, -32602)
		} else if result := successResult(t, context); result["isError"] == true || result["resultType"] != nil {
			t.Fatalf("legacy task-review context=%s", canonicalJSON(result))
		}
		client.close(t)
	}
}

func TestTaskReviewToolsRefuseExecutableConfigAndWorktreeRedirects(t *testing.T) {
	readToolsRefuseExecutableConfigAndWorktreeRedirects(t, taskReviewArguments, []string{"corvint.context", "corvint.cem.report"})
}

func TestTaskReviewTrafficMatchesOfficialSchema(t *testing.T) {
	schema := loadOfficialSchema(t)
	root := fixtureRepository(t)
	base, target := cemFixture(t, root, ".corvint/change.cem.json", "pkg/value.go", 1)
	client := startServerWithArguments(t, root, taskReviewArguments)
	defer client.close(t)
	cem := func(mapPath string) map[string]any {
		return map[string]any{"_meta": requestMeta(), "name": "corvint.cem.report", "arguments": map[string]any{"map": mapPath, "expectedBase": base, "target": target}}
	}
	checkOfficialExchanges(t, schema, client, 830, []schemaExchange{
		{"ListToolsRequest", "ListToolsResultResponse", "tools/list", map[string]any{"_meta": requestMeta()}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.context", "arguments": map[string]any{"task": "change the fixture Value", "subject": "pkg/value.go"}}},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", cem(".corvint/change.cem.json")},
		{"CallToolRequest", "CallToolResultResponse", "tools/call", cem("missing.cem.json")},
		{"CallToolRequest", "JSONRPCErrorResponse", "tools/call", cem("../escape.cem.json")},
	})
}

func listedToolNames(t *testing.T, client *stdioClient, meta map[string]any) []string {
	t.Helper()
	result := successResult(t, client.call(t, 899, "tools/list", map[string]any{"_meta": meta}))
	tools, _ := result["tools"].([]any)
	names := make([]string, 0, len(tools))
	for _, value := range tools {
		name, _ := object(t, value)["name"].(string)
		names = append(names, name)
	}
	return names
}
