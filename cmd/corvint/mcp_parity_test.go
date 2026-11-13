// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// cliOutput runs one corvint command in process and returns its stdout.
func cliOutput(t *testing.T, arguments ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("%q exit %d: %s", arguments, code, &stderr)
	}
	return stdout.Bytes()
}

// MTT-V1-009 (PROD-TEST-009): corvint.help returns, as its receipt text, the
// exact bytes of corvint help guide, corvint help --json COMMAND and
// corvint help --json.
func TestMTTV1009HelpToolMatchesCLI(t *testing.T) {
	for _, tc := range []struct {
		arguments string
		cli       []string
	}{
		{`{}`, []string{"help", "guide"}},
		{`{"topic":"command","command":"impact"}`, []string{"help", "--json", "impact"}},
		{`{"topic":"catalog"}`, []string{"help", "--json"}},
	} {
		result, failure := bridge.HelpToolResult(context.Background(), []byte(tc.arguments))
		if failure != nil {
			t.Fatalf("%s: %v", tc.arguments, failure)
		}
		structured, _ := result["structuredContent"].(map[string]any)
		receipt, _ := structured["receipt"].(map[string]any)
		if text, _ := receipt["text"].(string); text != string(cliOutput(t, tc.cli...)) {
			t.Fatalf("%s: help tool text differs from corvint %s", tc.arguments, strings.Join(tc.cli, " "))
		}
	}
}

// MTT-V1-009 (PROD-TEST-009): corvint.impact returns, as its receipt, the
// context record corvint impact prints for the same path, revision and limit;
// the CLI adds only its ok, tool and mutates envelope members.
func TestMTTV1009ImpactToolMatchesCLI(t *testing.T) {
	root := impactCLIRepository(t)
	registry, err := bridge.New(root)
	if err != nil {
		t.Fatal(err)
	}
	result, failure := registry.Call(context.Background(), bridge.ToolImpact, []byte(`{"paths":["pkg/main.go"],"limit":3}`))
	if failure != nil || result.State != "READY" {
		t.Fatalf("impact tool: %+v %v", result, failure)
	}
	var cli map[string]any
	if err := json.Unmarshal(cliOutput(t, "--root", root, "impact", "--limit", "3", "pkg/main.go"), &cli); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	var tool map[string]any
	if err := json.Unmarshal(encoded, &tool); err != nil {
		t.Fatal(err)
	}
	if cli["tool"] != "impact" || !reflect.DeepEqual(tool, cli["context"]) {
		t.Fatalf("impact tool receipt differs from the CLI:\ntool %s\ncli  %v", encoded, cli)
	}
}

// MTT-V1-009 (PROD-TEST-009): corvint.test_evidence, served by corvint mcp,
// returns the document corvint test-evidence prints for the same path.
func TestMTTV1009TestEvidenceToolMatchesCLI(t *testing.T) {
	root := affectedFixtureRepository(t)
	responses := mcpAffectedChildWith(t, root, []string{"--tool-profile", "test-evidence"},
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.test_evidence","arguments":{"path":"core/core.go"}}}`,
	)
	result, _ := responses["1"]["result"].(map[string]any)
	if result["isError"] != false {
		t.Fatalf("test_evidence: %#v", responses["1"])
	}
	var cli map[string]any
	if err := json.Unmarshal(cliOutput(t, "--root", root, "test-evidence", "core/core.go"), &cli); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result["structuredContent"], cli) {
		t.Fatalf("test_evidence differs from the CLI:\ntool %#v\ncli  %#v", result["structuredContent"], cli)
	}
}
