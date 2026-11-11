// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

// APK-V0-9970 (V1-5254): on the APK flows fixture the --summary verdict carries
// the reduced e2e member (scope, inventory count, selected files, distinct
// flow and variation pairs) and --format text prints its lines; the twin
// without a flows declaration gets no e2e member and no e2e line.
func TestAPKV09970SummaryNamesE2ESelections_V15254(t *testing.T) {
	t.Parallel()
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, flowPlanCLIConfig)
	verdict, _, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
	if code != 0 {
		t.Fatalf("--summary exit %d: %s", code, stderr)
	}
	want := map[string]any{
		"scope":     "BOUNDED",
		"inventory": float64(3),
		"selected": []any{
			map[string]any{"path": "e2e/checkout.spec.ts", "project": "chromium"},
			map[string]any{"path": "e2e/orphan.spec.ts", "project": "chromium"},
		},
		"variations": []any{
			map[string]any{"flow": "checkout", "variation": "checkout.guest"},
			map[string]any{"flow": "checkout", "variation": "checkout.happy"},
		},
	}
	if verdict["profile"] != "affected-summary/1" || !reflect.DeepEqual(verdict["e2e"], want) {
		t.Fatalf("verdict e2e = %#v\nwant %#v", verdict["e2e"], want)
	}
	var text, textErr bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--summary", "--base", base, "--format", "text"}, strings.NewReader(""), &text, &textErr); code != 0 {
		t.Fatalf("--summary --format text exit %d: %s", code, textErr.String())
	}
	wantText := "e2e-scope BOUNDED inventory=3 selected=2\n" +
		"e2e-variation checkout checkout.guest\n" +
		"e2e-variation checkout checkout.happy\n" +
		"e2e e2e/checkout.spec.ts project chromium\n" +
		"e2e e2e/orphan.spec.ts project chromium\n"
	if !strings.HasSuffix(text.String(), wantText) {
		t.Fatalf("text verdict lacks the e2e lines after the tiers:\n%s\nwant suffix:\n%s", text.String(), wantText)
	}

	twin, twinBase := flowPlanCLIRepo(t, "", flowPlanCLIConfig)
	bare, raw, stderr, code := runAffectedCLI(t, twin, "--summary", "--base", twinBase)
	if code != 0 {
		t.Fatalf("twin --summary exit %d: %s", code, stderr)
	}
	if _, present := bare["e2e"]; present || bytes.Contains(raw, []byte(`"e2e"`)) {
		t.Fatalf("the twin without a declaration carries e2e: %s", raw)
	}
	text.Reset()
	textErr.Reset()
	if code := runContext(context.Background(), []string{"--root", twin, "affected", "--summary", "--base", twinBase, "--format", "text"}, strings.NewReader(""), &text, &textErr); code != 0 || strings.Contains(text.String(), "e2e") {
		t.Fatalf("twin text verdict exit %d: %s%s", code, text.String(), textErr.String())
	}
}

// MMR-V0-1118, APK-V0-9970 (V1-5254): on a repository that declares its flows,
// corvint.affected answers the exact `corvint affected --summary` bytes for a
// base and for the dirty worktree, and the base answer names the flow and
// variation selections of the plan.
func TestV15254MCPAffectedCarriesFlowSelections(t *testing.T) {
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, flowPlanCLIConfig)
	_, ranged, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
	if code != 0 {
		t.Fatalf("--summary --base exit %d: %s", code, stderr)
	}
	responses := mcpAffectedChild(t, root,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"`+base+`"}}}`,
	)
	checkMCPAffectedAnswer(t, responses["1"], ranged)
	shopWrite(t, root, map[string]string{"src/search.ts": "export function find(q: string): string {\n  return q + \"!\";\n}\n"})
	_, dirty, stderr, code := runAffectedCLI(t, root, "--summary")
	if code != 0 {
		t.Fatalf("--summary exit %d: %s", code, stderr)
	}
	dirtyResponses := mcpAffectedChild(t, root,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint.affected","arguments":{}}}`,
	)
	checkMCPAffectedAnswer(t, dirtyResponses["2"], dirty)
	structured := responses["1"]["result"].(map[string]any)["structuredContent"].(map[string]any)
	e2e, _ := structured["e2e"].(map[string]any)
	wantVariations := []any{
		map[string]any{"flow": "checkout", "variation": "checkout.guest"},
		map[string]any{"flow": "checkout", "variation": "checkout.happy"},
	}
	if e2e == nil || !reflect.DeepEqual(e2e["variations"], wantVariations) {
		t.Fatalf("MCP answer lacks the flow and variation selections: %#v", structured)
	}
	if _, present := dirtyResponses["2"]["result"].(map[string]any)["structuredContent"].(map[string]any)["e2e"]; !present {
		t.Fatalf("dirty-worktree MCP answer lacks e2e: %#v", dirtyResponses["2"])
	}
}
