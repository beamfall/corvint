package cli

import (
	"path/filepath"
	"reflect"
	"testing"
)

// MCPV0-952: every -32602 a corvint-mcp tools/call or tools/list answers
// carries data.reason and the offending field, and the allowed values or rule
// when the listed schema states them.
func TestMCPV0952InvalidParamsCarryReasonAndField(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	call := func(params map[string]any) map[string]any {
		return map[string]any{"method": "tools/call", "params": params}
	}
	cases := []struct {
		name    string
		request map[string]any
		want    map[string]any
	}{
		{"unknown tool", call(map[string]any{"name": "corvint.nope", "arguments": map[string]any{}}),
			map[string]any{"reason": "unknown-tool", "field": "name", "allowed": []any{"corvint.help", "corvint.impact", "corvint.query", "corvint.status"}}},
		{"bad enum", call(map[string]any{"name": "corvint.help", "arguments": map[string]any{"topic": "everything"}}),
			map[string]any{"reason": "invalid-enum", "field": "arguments.topic", "allowed": []any{"guide", "command", "catalog"}}},
		{"pattern violation", call(map[string]any{"name": "corvint.query", "arguments": map[string]any{"task": "   "}}),
			map[string]any{"reason": "pattern-mismatch", "field": "arguments.task", "rule": "^[ -~]*[!-~][ -~]*$"}},
		{"unknown argument", call(map[string]any{"name": "corvint.status", "arguments": map[string]any{"verbose": true}}),
			map[string]any{"reason": "unknown-argument", "field": "arguments.verbose"}},
		{"missing argument", call(map[string]any{"name": "corvint.query", "arguments": map[string]any{}}),
			map[string]any{"reason": "missing-argument", "field": "arguments.task"}},
		{"wrong type", call(map[string]any{"name": "corvint.impact", "arguments": map[string]any{"paths": "main.go"}}),
			map[string]any{"reason": "invalid-type", "field": "arguments.paths", "rule": "array"}},
		{"out of range", call(map[string]any{"name": "corvint.impact", "arguments": map[string]any{"paths": []any{"main.go"}, "limit": 0}}),
			map[string]any{"reason": "out-of-range", "field": "arguments.limit", "rule": "minimum 1"}},
		{"duplicate item", call(map[string]any{"name": "corvint.impact", "arguments": map[string]any{"paths": []any{"main.go", "main.go"}}}),
			map[string]any{"reason": "duplicate-item", "field": "arguments.paths[1]"}},
		{"arguments not an object", call(map[string]any{"name": "corvint.status", "arguments": []any{}}),
			map[string]any{"reason": "invalid-type", "field": "arguments", "rule": "object"}},
		{"missing name", call(map[string]any{"arguments": map[string]any{}}),
			map[string]any{"reason": "missing-tool-name", "field": "name"}},
		{"unknown call param", call(map[string]any{"name": "corvint.status", "stream": true}),
			map[string]any{"reason": "unknown-param", "field": "stream", "allowed": []any{"_meta", "arguments", "name"}}},
		{"list cursor", map[string]any{"method": "tools/list", "params": map[string]any{"cursor": "next"}},
			map[string]any{"reason": "invalid-cursor", "field": "cursor"}},
		{"list unknown param", map[string]any{"method": "tools/list", "params": map[string]any{"page": 2}},
			map[string]any{"reason": "unknown-param", "field": "page"}},
	}
	requests := make([]map[string]any, len(cases))
	for index, test := range cases {
		requests[index] = test.request
	}
	exit, responses, stderr := legacySession(t, []string{"--root", root}, requests...)
	if exit != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr)
	}
	for index, test := range cases {
		failure, _ := responses[float64(index+1)]["error"].(map[string]any)
		if failure["code"] != float64(-32602) || failure["message"] != "Invalid params" || !reflect.DeepEqual(failure["data"], test.want) {
			t.Errorf("%s: error=%#v, want data %#v", test.name, failure, test.want)
		}
	}
}

// MCPV0-952: the protocol-level refusals name their field too.
func TestMCPV0952LegacyInitializeInvalidParamsCarryField(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	gitInit(t, root)
	exit, responses, _ := legacySession(t, []string{"--root", root},
		map[string]any{"method": "ping", "params": map[string]any{"x": 1}})
	failure, _ := responses[1]["error"].(map[string]any)
	if exit != 0 || failure["code"] != float64(-32602) || !reflect.DeepEqual(failure["data"], map[string]any{"reason": "unknown-param", "field": "params"}) {
		t.Fatalf("ping error=%#v", failure)
	}
}
