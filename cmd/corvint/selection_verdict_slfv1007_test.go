// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// slfv1007Repository is a committed Go and Vitest repository and the base
// before a change that yields unknown NO_SELECTABLE_TEST (src/lonely.ts, which
// no test imports) and a TypeScript DISCOVERY frontier (a Vitest config whose
// include is computed).
func slfv1007Repository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":           "module example.test/m\n\ngo 1.22\n",
		"lib/lib.go":       "package lib\n\nfunc L() int { return 1 }\n",
		"package.json":     `{"name":"fx","devDependencies":{"vitest":"1.0.0"}}` + "\n",
		"vitest.config.ts": "export default { test: { include: [process.env.SUITE] } }\n",
		"src/math.ts":      "export function add(a: number, b: number) { return a + b }\n",
		"src/math.test.ts": "import { add } from \"./math\"\nimport { test } from \"vitest\"\ntest(\"add\", () => { add(1, 2) })\n",
		"src/lonely.ts":    "export const lonely = 1\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "src", "lonely.ts"), []byte("export const lonely = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change")
	return root, base
}

// slfv1007Verdict decodes a selectionVerdict member.
func slfv1007Verdict(t *testing.T, surface string, raw any) sessiontest.SelectionVerdict {
	t.Helper()
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var verdict sessiontest.SelectionVerdict
	if raw == nil || json.Unmarshal(encoded, &verdict) != nil {
		t.Fatalf("%s: no selectionVerdict member: %s", surface, encoded)
	}
	return verdict
}

// SLF-V1-007 scenario "Same plan, same verdict": on a change that yields
// unknown NO_SELECTABLE_TEST and a TypeScript DISCOVERY frontier,
// `corvint affected` (the affected-plan/1 document and its --summary), the
// `corvint test` plan step (its dry-run document and its stderr) and the MCP
// plan tool report the same effect for each unknown and the same tier for
// each runner, each with a non-empty hint, because all of them print
// sessiontest.SelectionVerdictOf's output.
func TestSelectionVerdictParityAcrossSurfaces_SLFV1007(t *testing.T) {
	root, base := slfv1007Repository(t)

	plan, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("affected exit %d: %s", code, stderr)
	}
	affectedVerdict := slfv1007Verdict(t, "affected", plan["selectionVerdict"])
	want := sessiontest.SelectionVerdict{Scope: sessiontest.VerdictNarrowed, Runners: []sessiontest.RunnerVerdict{{Runner: "typescript", Tier: "full"}}}
	reasons := map[string]bool{}
	for _, entry := range affectedVerdict.Unknown {
		reasons[entry.Reason] = true
		if entry.Effect != sessiontest.EffectAddsSuite+"typescript" || strings.TrimSpace(entry.Hint) == "" {
			t.Fatalf("affected entry %+v", entry)
		}
	}
	if !reasons["NO_SELECTABLE_TEST"] || !reasons["LANGUAGE_FRONTIER"] || len(affectedVerdict.Unknown) != 2 ||
		affectedVerdict.Scope != want.Scope || !reflect.DeepEqual(affectedVerdict.Runners, want.Runners) {
		t.Fatalf("the fixture plan is not the scenario's: %+v", affectedVerdict)
	}

	summary, _, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
	if code != 0 {
		t.Fatalf("affected --summary exit %d: %s", code, stderr)
	}
	if got := slfv1007Verdict(t, "affected --summary", summary["selectionVerdict"]); !reflect.DeepEqual(got, affectedVerdict) {
		t.Fatalf("affected --summary %+v\nwant %+v", got, affectedVerdict)
	}

	code, stdout, testStderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--full")
	if code != 0 {
		t.Fatalf("test --dry-run exit %d: %s", code, testStderr)
	}
	var dryRun map[string]any
	if err := json.Unmarshal([]byte(stdout), &dryRun); err != nil {
		t.Fatal(err)
	}
	if got := slfv1007Verdict(t, "test --dry-run", dryRun["selectionVerdict"]); !reflect.DeepEqual(got, affectedVerdict) {
		t.Fatalf("test --dry-run %+v\nwant %+v", got, affectedVerdict)
	}
	lines := []string{"corvint-test: SELECTION narrowed typescript=full adds-suite:typescript=2"}
	for _, entry := range affectedVerdict.Unknown {
		lines = append(lines, "corvint-test: EFFECT "+entry.Effect+" "+entry.Reason+" "+entry.Detail+": "+entry.Hint)
	}
	if !strings.Contains(testStderr, strings.Join(lines, "\n")+"\n") {
		t.Fatalf("test stderr does not print the verdict %q:\n%s", lines, testStderr)
	}

	responses := mcpAffectedChild(t, root,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"`+base+`"}}}`)
	result, _ := responses["1"]["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	if result["isError"] != false || structured == nil {
		t.Fatalf("MCP result %#v", responses["1"])
	}
	if got := slfv1007Verdict(t, "MCP corvint.affected", structured["selectionVerdict"]); !reflect.DeepEqual(got, affectedVerdict) {
		t.Fatalf("MCP %+v\nwant %+v", got, affectedVerdict)
	}
}
