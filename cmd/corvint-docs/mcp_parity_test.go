// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/flowcoverage/testfixture"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// counterFlow is one committed flow intent with one variation.
const counterFlow = `{"schema":"application-flow-intent/1","flow_id":"counter","revision":1,"kind":"ui","actor":"caller","preconditions":[],"steps":[{"step_id":"click","action":"click counter"}],"outcomes":[{"outcome_id":"one","behavior":"counter is one","matcher":"toHaveText","locator":"#count","value":"1"}],"variations":[{"variation_id":"happy","preconditions":[],"steps":["click"],"observable_facts":[],"outcomes":["one"],"projects":["chromium"]}],"links":[]}`

// flowsParityRepository is a committed repository with flows/counter.json.
func flowsParityRepository(t *testing.T) string {
	t.Helper()
	root := taskContextRepository(t)
	if err := os.MkdirAll(filepath.Join(root, "flows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "flows", "counter.json"), []byte(counterFlow+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "add", "flows")
	cemGit(t, root, "commit", "-qm", "declare counter flow")
	return root
}

// flowsCLIRecord runs `corvint-docs --root ROOT flows ARGS...` and decodes its
// JSON record; flows coverage exits 1 when its page reports a gap, which is
// its verdict, not a failure.
func flowsCLIRecord(t *testing.T, root string, arguments ...string) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), append([]string{"--root", root, "flows"}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 0 && (code != 1 || arguments[0] != "coverage") {
		t.Fatalf("flows %q exit %d: %s", arguments, code, &stderr)
	}
	var record map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &record); err != nil {
		t.Fatalf("flows %q: %v\n%s", arguments, err, &stdout)
	}
	return record
}

// flowsToolRecord calls one corvint mcp flow tool through the flows registry
// and returns its receipt.
func flowsToolRecord(t *testing.T, root, tool, arguments string) map[string]any {
	t.Helper()
	registry, err := bridge.NewFlows(root)
	if err != nil {
		t.Fatal(err)
	}
	result, failure := registry.Call(context.Background(), tool, []byte(arguments))
	if failure != nil || result.State != "READY" {
		t.Fatalf("%s %s: %+v %v", tool, arguments, result, failure)
	}
	encoded, err := json.Marshal(result.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

// MTT-V1-009 (PROD-TEST-009): each corvint mcp flow-report tool returns, as
// its receipt, the record `corvint-docs flows VERB` prints on the same
// fixture and inputs.
func TestMTTV1009FlowsToolsMatchCLI(t *testing.T) {
	root := flowsParityRepository(t)
	head := strings.TrimSpace(cemGit(t, root, "rev-parse", "HEAD"))
	for _, tc := range []struct {
		tool, arguments string
		cli             []string
	}{
		{bridge.ToolFlowsMap, `{"flows":"flows"}`, []string{"map", "--flows", "flows"}},
		{bridge.ToolFlowsGaps, `{"flows":"flows"}`, []string{"gaps", "--flows", "flows"}},
		{bridge.ToolFlowsImpact, `{"flows":"flows","base":"` + head + `"}`, []string{"impact", "--flows", "flows", "--base", head}},
		{bridge.ToolFlowsNavigate, `{"flows":"flows"}`, []string{"navigate", "--flows", "flows"}},
		{bridge.ToolFlowsStatus, `{"flows":"flows"}`, []string{"status", "--flows", "flows"}},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			if tool, cli := flowsToolRecord(t, root, tc.tool, tc.arguments), flowsCLIRecord(t, root, tc.cli...); !reflect.DeepEqual(tool, cli) {
				t.Fatalf("%s differs from corvint-docs flows %s:\ntool %v\ncli  %v", tc.tool, tc.cli[0], tool, cli)
			}
		})
	}
	t.Run(bridge.ToolFlowsCoverage, func(t *testing.T) {
		root := testfixture.Repository(t)
		tool := flowsToolRecord(t, root, bridge.ToolFlowsCoverage, `{"denominator":"denominator.json","receipts":"runs.json","offset":0,"limit":1}`)
		cli := flowsCLIRecord(t, root, "coverage", "--denominator", "denominator.json", "--receipts", "runs.json", "--offset", "0", "--limit", "1")
		if !reflect.DeepEqual(tool, cli) {
			t.Fatalf("coverage differs from the CLI:\ntool %v\ncli  %v", tool, cli)
		}
	})
}

// MTT-V1-009 (PROD-DOCS-010): corvint.docs_consume, answered through the
// docs-tool verb both documentation servers run, returns the record
// `corvint-docs docs consume` prints for the same draft and task (the CLI
// adds only its line's newline).
func TestMTTV1009DocsConsumeToolMatchesCLI(t *testing.T) {
	root := docsRepository(t)
	code, draft, stderr := runDocsFixture(t, root, "draft", "")
	if code != 0 {
		t.Fatalf("draft exit %d: %s", code, stderr)
	}
	code, consumed, stderr := runDocsFixture(t, root, "consume", draft, "--task", "Split")
	if code != 0 {
		t.Fatalf("consume exit %d: %s", code, stderr)
	}
	arguments, err := json.Marshal(map[string]string{"source": "owner.md", "package": "cache", "task": "Split", "draft": draft})
	if err != nil {
		t.Fatal(err)
	}
	code, result, stderr := runDocsToolVerb(t, root, docsenvelope.ToolConsume, string(arguments))
	if code != 0 || result.Failure != nil || result.Error != nil {
		t.Fatalf("docs-tool consume: exit %d %+v %s", code, result, stderr)
	}
	var tool, cli map[string]any
	if json.Unmarshal([]byte(result.Text), &tool) != nil || json.Unmarshal([]byte(consumed), &cli) != nil || !reflect.DeepEqual(tool, cli) || !reflect.DeepEqual(result.Structured, cli) {
		t.Fatalf("consume tool text differs from the CLI:\ntool %q\ncli  %q", result.Text, consumed)
	}
}
