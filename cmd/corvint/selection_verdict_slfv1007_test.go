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

// slfv1007Repository is a committed Vitest repository and the base before a
// change that yields unknown NO_SELECTABLE_TEST (src/lonely.ts, which no test
// imports) and a TypeScript DISCOVERY frontier (a Vitest config whose include
// is computed). It has no Go module: a Go universe would make the Go
// selector fall back for a change no Go package reads, and test run FULL
// (TestSelectionVerdictUnownedPathParity_SLFV1007 covers the Go side).
func slfv1007Repository(t *testing.T) (string, string) {
	t.Helper()
	return slfv1007Commit(t, map[string]string{
		"package.json":     `{"name":"fx","devDependencies":{"vitest":"1.0.0"}}` + "\n",
		"vitest.config.ts": "export default { test: { include: [process.env.SUITE] } }\n",
		"src/math.ts":      "export function add(a: number, b: number) { return a + b }\n",
		"src/math.test.ts": "import { add } from \"./math\"\nimport { test } from \"vitest\"\ntest(\"add\", () => { add(1, 2) })\n",
		"src/lonely.ts":    "export const lonely = 1\n",
	}, map[string]string{"src/lonely.ts": "export const lonely = 2\n"})
}

// slfv1007Commit commits files as the base, then commits change on top and
// returns the root and the base.
func slfv1007Commit(t *testing.T, files, change map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range files {
		write(name, body)
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for name, body := range change {
		write(name, body)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "change")
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

// slfv1007Surfaces runs every surface that prints the verdict on one plan:
// `corvint affected` (the affected-plan/1 document and its --summary), the
// `corvint test` plan step (its dry-run document, its stderr and its default
// summary) and the MCP plan tool. It fails unless all of them print the same
// verdict, and returns it with the test step's dry-run scope.
func slfv1007Surfaces(t *testing.T, root, base string) (sessiontest.SelectionVerdict, string) {
	t.Helper()
	plan, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("affected exit %d: %s", code, stderr)
	}
	affectedVerdict := slfv1007Verdict(t, "affected", plan["selectionVerdict"])
	for _, entry := range affectedVerdict.Unknown {
		if strings.TrimSpace(entry.Hint) == "" {
			t.Fatalf("affected entry with an empty hint: %+v", entry)
		}
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
	words := testSelectionWords(affectedVerdict)
	lines := []string{"corvint-test: SELECTION " + words}
	for _, entry := range affectedVerdict.Unknown {
		lines = append(lines, "corvint-test: EFFECT "+entry.Effect+" "+entry.Reason+" "+entry.Detail+": "+entry.Hint)
	}
	if !strings.Contains(testStderr, strings.Join(lines, "\n")+"\n") {
		t.Fatalf("test stderr does not print the verdict %q:\n%s", lines, testStderr)
	}

	// The default local dry run is the bounded summary, which drops the
	// SELECTION and EFFECT stderr lines but keeps the verdict in its budget.
	code, summaryOut, summaryErr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run")
	if code != 0 {
		t.Fatalf("test --dry-run (summary) exit %d: %s", code, summaryErr)
	}
	if !strings.Contains(summaryOut, "\n  selection: "+words+"\n") || len(summaryOut)+len(summaryErr) > testSummaryLimit {
		t.Fatalf("test --dry-run summary does not print the verdict %q within %d bytes:\n%s%s", words, testSummaryLimit, summaryOut, summaryErr)
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
	scope, _ := dryRun["scope"].(string)
	if (scope == sessiontest.ScopeFull) != (affectedVerdict.Scope == sessiontest.VerdictFull) {
		t.Fatalf("test runs %s but every surface prints %+v", scope, affectedVerdict)
	}
	return affectedVerdict, scope
}

// SLF-V1-007 scenario "Same plan, same verdict": on a change that yields
// unknown NO_SELECTABLE_TEST and a TypeScript DISCOVERY frontier, every
// surface reports the same effect for each unknown and the same tier for
// each runner, each with a non-empty hint.
func TestSelectionVerdictParityAcrossSurfaces_SLFV1007(t *testing.T) {
	root, base := slfv1007Repository(t)
	verdict, _ := slfv1007Surfaces(t, root, base)
	want := sessiontest.SelectionVerdict{Scope: sessiontest.VerdictNarrowed, Runners: []sessiontest.RunnerVerdict{{Runner: "typescript", Tier: "full"}}}
	reasons := map[string]bool{}
	for _, entry := range verdict.Unknown {
		reasons[entry.Reason] = true
		if entry.Effect != sessiontest.EffectAddsSuite+"typescript" {
			t.Fatalf("entry %+v", entry)
		}
	}
	if !reasons["NO_SELECTABLE_TEST"] || !reasons["LANGUAGE_FRONTIER"] || len(verdict.Unknown) != 2 ||
		verdict.Scope != want.Scope || !reflect.DeepEqual(verdict.Runners, want.Runners) {
		t.Fatalf("the fixture plan is not the scenario's: %+v", verdict)
	}
}

// SLF-V1-007: an UNOWNED_DIRTY_PATH inside a Go package directory is
// attributed by the enclosing package's Go rule, so `corvint test` keeps the
// run narrowed; `corvint affected`, its summary and the MCP tool read the
// attribution from the same Go selector and print the same ignored effect.
// One outside every Go package is attributed by none, and every surface
// prints forces-full while test runs FULL.
func TestSelectionVerdictUnownedPathParity_SLFV1007(t *testing.T) {
	files := map[string]string{
		"go.mod":          "module example.test/m\n\ngo 1.22\n",
		"lib/lib.go":      "package lib\n\nfunc L() int { return 1 }\n",
		"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) {\n\tif L() != 1 {\n\t\tt.Fatal(L())\n\t}\n}\n",
		"lib/notes.txt":   "one\n",
		"notes/plan.txt":  "one\n",
	}
	for _, test := range []struct {
		name, path, effect, scope string
	}{
		{"attributed by the enclosing package", "lib/notes.txt", sessiontest.EffectIgnored, sessiontest.ScopeNarrowed},
		{"attributed by no Go rule", "notes/plan.txt", sessiontest.EffectForcesFull, sessiontest.ScopeFull},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, base := slfv1007Commit(t, files, map[string]string{test.path: "two\n"})
			verdict, scope := slfv1007Surfaces(t, root, base)
			found := false
			for _, entry := range verdict.Unknown {
				if entry.Reason == "UNOWNED_DIRTY_PATH" && entry.Detail == test.path {
					found = entry.Effect == test.effect
				}
			}
			if !found || scope != test.scope {
				t.Fatalf("verdict %+v with test scope %s, want %s for %s and test %s", verdict, scope, test.effect, test.path, test.scope)
			}
		})
	}
}
