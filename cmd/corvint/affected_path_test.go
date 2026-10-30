package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// affectedSelectedVia maps each selected unit to its witness chain, rebuilt
// from plan.viaLinks in a compact receipt (AFP-V0-640).
func affectedSelectedVia(t *testing.T, receipt map[string]any) map[string][]string {
	t.Helper()
	plan := receipt["plan"].(map[string]any)
	links := map[string]string{}
	if raw, ok := plan["viaLinks"].(map[string]any); ok {
		for unit, previous := range raw {
			links[unit] = previous.(string)
		}
	}
	via := map[string][]string{}
	for _, item := range plan["selected"].([]any) {
		selection := item.(map[string]any)
		witness := selection["witness"].(map[string]any)
		if chain, listed := witness["via"]; listed {
			via[selection["unitId"].(string)] = anyStrings(chain)
			continue
		}
		chain, ok := expandAffectedVia(links, selection["unitId"].(string), int(witness["viaLength"].(float64)))
		if !ok {
			t.Fatalf("witness chain of %v does not resolve through plan.viaLinks %v", selection["unitId"], links)
		}
		via[selection["unitId"].(string)] = chain
	}
	return via
}

// AFP-V0-094/095: `--path` plans the named paths at a clean HEAD and walks
// reverse dependencies transitively, so a test two importers away from the
// changed path is selected, which one-hop impact evidence omits (GitHub #1301).
func TestAFPV0094PathListSelectsTwoHopImportersAtHead(t *testing.T) {
	t.Parallel()
	t.Run("two-hop importer through an untested intermediate", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		affectedGit(t, root, "rm", "-q", "mid/mid_test.go")
		affectedGit(t, root, "commit", "-qm", "intermediate package without tests")
		head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		receipt, _, stderr, code := runAffectedCLI(t, root, "--full", "--path", "core/core.go")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		via := affectedSelectedVia(t, receipt)
		leaf := via["go:example.com/fixture/leaf"]
		if !slices.Equal(leaf, []string{"go:example.com/fixture/core", "go:example.com/fixture/mid", "go:example.com/fixture/leaf"}) {
			t.Fatalf("the two-hop importer leaf must be selected through mid: %v", via)
		}
		if _, ok := via["go:example.com/fixture/mid"]; ok {
			t.Fatalf("mid declares no test and must not be selected: %v", via)
		}
		plan := receipt["plan"].(map[string]any)
		if plan["scope"] != "BOUNDED" || !slices.Equal(anyStrings(plan["dirty"]), []string{"core/core.go"}) {
			t.Fatalf("plan=%v", plan)
		}
		if !slices.Equal(anyStrings(receipt["path_list"]), []string{"core/core.go"}) || receipt["revision"] != head {
			t.Fatalf("path_list=%v revision=%v head=%s", receipt["path_list"], receipt["revision"], head)
		}
		rangeMember := receipt["range"].(map[string]any)
		if rangeMember["base"] != "" || len(anyStrings(rangeMember["paths"])) != 0 {
			t.Fatalf("--path joins no committed range: %v", rangeMember)
		}
		if _, ok := receipt["warning"]; ok {
			t.Fatalf("a clean --path receipt carries no mixed warning: %v", receipt["warning"])
		}
	})
	t.Run("plan equals the --base plan of the same committed change", func(t *testing.T) {
		root := affectedFixtureRepository(t)
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		corePath := filepath.Join(root, "core", "core.go")
		body, err := os.ReadFile(corePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corePath, append(body, []byte("\n// committed edit\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, root, "commit", "-qam", "edit core")
		for _, full := range [][]string{nil, {"--full"}} {
			_, ranged, stderr, code := runAffectedCLI(t, root, append(slices.Clone(full), "--base", base)...)
			if code != 0 {
				t.Fatalf("--base exit %d: %s", code, stderr)
			}
			pathReceipt, listed, stderr, code := runAffectedCLI(t, root, append(slices.Clone(full), "--path", "core/core.go", "--path", "core/core.go")...)
			if code != 0 {
				t.Fatalf("--path exit %d: %s", code, stderr)
			}
			// --path mode has no measured-from tree, so it records no additive
			// change (ADN-V0-009); the plans are otherwise identical.
			if !reflect.DeepEqual(planWithoutGoAdditive(t, extractJSONObject(t, ranged, "plan")), planWithoutGoAdditive(t, extractJSONObject(t, listed, "plan"))) {
				t.Fatalf("plans differ (full=%v):\n%s\n%s", full, ranged, listed)
			}
			if !slices.Equal(anyStrings(pathReceipt["path_list"]), []string{"core/core.go"}) {
				t.Fatalf("a repeated --path collapses: %v", pathReceipt["path_list"])
			}
		}
		_, plain, _, _ := runAffectedCLI(t, root)
		if bytes.Contains(plain, []byte(`"path_list"`)) {
			t.Fatalf("a receipt without --path must omit path_list: %s", plain)
		}
	})
}

// planWithoutGoAdditive decodes an encoded plan without its goAdditive member.
func planWithoutGoAdditive(t *testing.T, plan []byte) map[string]any {
	t.Helper()
	var members map[string]any
	if err := json.Unmarshal(plan, &members); err != nil {
		t.Fatal(err)
	}
	delete(members, "goAdditive")
	return members
}

// AFP-V0-095: a path list never yields a BOUNDED plan its graph cannot
// justify; an unowned or unindexed path keeps the existing UNKNOWN vocabulary.
func TestAFPV0095PathListKeepsUnknownScope(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	receipt, _, stderr, code := runAffectedCLI(t, root, "--path", "notes/scratch.dat", "--path", "core/core.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	plan := receipt["plan"].(map[string]any)
	if plan["scope"] != "UNKNOWN" {
		t.Fatalf("an unowned listed path must leave scope UNKNOWN: %v", plan)
	}
	found := false
	for _, item := range plan["unknown"].([]any) {
		entry := item.(map[string]any)
		found = found || (entry["reason"] == "UNOWNED_DIRTY_PATH" && entry["detail"] == "notes/scratch.dat")
	}
	if !found {
		t.Fatalf("expected UNOWNED_DIRTY_PATH for notes/scratch.dat: %v", plan["unknown"])
	}
	if !slices.Equal(anyStrings(receipt["path_list"]), []string{"core/core.go", "notes/scratch.dat"}) {
		t.Fatalf("path_list=%v", receipt["path_list"])
	}
}

// AFP-V0-094: a dirty worktree and every unsupported combination refuse with
// exit 2 and no stdout document.
func TestAFPV0094PathListRefusals(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for _, tc := range []struct {
		name    string
		options []string
		code    string
	}{
		{"absolute", []string{"--path", "/etc/passwd"}, "invalid-arguments"},
		{"parent", []string{"--path", "../core/core.go"}, "invalid-arguments"},
		{"empty", []string{"--path="}, "invalid-arguments"},
		{"missing value", []string{"--path"}, "invalid-arguments"},
		{"base", []string{"--path", "core/core.go", "--base", head}, "invalid-arguments"},
		{"snapshot", []string{"--path", "core/core.go", "--snapshot", "receipt.json"}, "invalid-arguments"},
		{"provider", []string{"--path", "core/core.go", "--provider", "record.json"}, "invalid-arguments"},
		{"playwright", []string{"--path", "core/core.go", "--playwright-config", "playwright.config.ts"}, "invalid-arguments"},
	} {
		receipt, stdout, stderr, code := runAffectedCLI(t, root, tc.options...)
		if code != 2 || len(stdout) != 0 || receipt != nil || !strings.Contains(stderr, tc.code) {
			t.Fatalf("%s: exit %d stdout=%s stderr=%s", tc.name, code, stdout, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "core", "core.go"), []byte("package core\n\nconst Value = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, root)
	_, stdout, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go")
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") || !strings.Contains(stderr, head) {
		t.Fatalf("dirty worktree: exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("a refused --path run must not write")
	}
}

// AFP-V0-096: impact help states its one-hop closure and that READY is not a
// closed selection; affected help documents --path.
func TestAFPV0096HelpStatesImpactClosureDepth(t *testing.T) {
	t.Parallel()
	impact := strings.Join(strings.Fields(helpText("impact")), " ")
	for _, want := range []string{"stop at direct importers (one hop", "a READY receipt is evidence, not a closed test selection", "corvint affected --path PATH [PATH ...]"} {
		if !strings.Contains(impact, want) {
			t.Fatalf("impact help lacks %q", want)
		}
	}
	affectedText := strings.Join(strings.Fields(helpText("affected")), " ")
	for _, want := range []string{"affected [--full] --path PATH [PATH ...] [--path PATH ...]", "a test two importers away is selected", "path_list", "unsupported-affected-mixed-worktree"} {
		if !strings.Contains(affectedText, want) {
			t.Fatalf("affected help lacks %q", want)
		}
	}
}

// AFP-V0-094: an ignored file absent from HEAD is not dirty, but the disk
// graph reads it, so a --path plan over it would not be HEAD's; it refuses
// whether the ignored file is listed or only read by the graph.
func TestAFPV0094PathListRefusesIgnoredFilesTheGraphReads(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("core/generated.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".gitignore")
	affectedGit(t, root, "commit", "-qm", "ignore generated source")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "core", "generated.go"), []byte("package core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, root)
	for _, listed := range []string{"core/generated.go", "core/core.go"} {
		receipt, stdout, stderr, code := runAffectedCLI(t, root, "--path", listed)
		if code != 2 || len(stdout) != 0 || receipt != nil || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") ||
			!strings.Contains(stderr, "core/generated.go") || !strings.Contains(stderr, head) {
			t.Fatalf("--path %s over an ignored graph file: exit %d stdout=%s stderr=%s", listed, code, stdout, stderr)
		}
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("a refused --path run must not write")
	}
}

// AFP-V0-094 (V1-2086): an ignored file the graph reaches through a unit's
// testdata, a go:embed directory or a declared read scope refuses a --path
// plan too; an ignored file no unit reads does not.
func TestAFPV0094PathListRefusesIgnoredFilesUnitsRead(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name, ignore, file string
		committed          map[string]string
		refuses            bool
	}{
		{name: "testdata", ignore: "core/testdata/generated.json", file: "core/testdata/generated.json", refuses: true},
		{name: "embed", ignore: "core/assets/extra.txt", file: "core/assets/extra.txt", refuses: true, committed: map[string]string{
			"core/assets/kept.txt": "kept\n",
			"core/embed.go":        "package core\n\nimport \"embed\"\n\n//go:embed assets\nvar Assets embed.FS\n",
		}},
		{name: "read scope", ignore: "docs/generated.md", file: "docs/generated.md", refuses: true, committed: map[string]string{
			".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"core":["docs/"]}}`,
		}},
		// A read-scoped unit reads its own package subtree even with no entries.
		{name: "read scope package subtree", ignore: "core/fixtures/generated.json", file: "core/fixtures/generated.json", refuses: true, committed: map[string]string{
			".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"core":[]}}`,
		}},
		{name: "unread", ignore: "build/out.bin", file: "build/out.bin", refuses: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			root := affectedFixtureRepository(t)
			files := map[string]string{".gitignore": testCase.ignore + "\n"}
			for name, body := range testCase.committed {
				files[name] = body
			}
			for name, body := range files {
				target := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			affectedGit(t, root, "add", ".")
			affectedGit(t, root, "commit", "-qm", "ignore "+testCase.name)
			head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			target := filepath.Join(root, filepath.FromSlash(testCase.file))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("absent from HEAD\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			receipt, stdout, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go")
			if !testCase.refuses {
				if code != 0 || receipt == nil {
					t.Fatalf("an ignored file no unit reads must not refuse: exit %d stderr=%s", code, stderr)
				}
				return
			}
			if code != 2 || len(stdout) != 0 || receipt != nil || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") ||
				!strings.Contains(stderr, path.Dir(testCase.file)) || !strings.Contains(stderr, head) {
				t.Fatalf("--path over an ignored %s file: exit %d stdout=%s stderr=%s", testCase.name, code, stdout, stderr)
			}
		})
	}
}

// AFP-V0-094: the separated form never takes an option-looking token as its
// value, so `--path --full` cannot swallow the next option; a dash-prefixed
// file name is still reachable as `--path=VALUE`.
func TestAFPV0094PathListRejectsOptionLookingSeparatedValue(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	for _, options := range [][]string{{"--path", "--path=core/core.go"}, {"--path", "--full"}, {"--path", "-x"}} {
		receipt, stdout, stderr, code := runAffectedCLI(t, root, options...)
		if code != 2 || len(stdout) != 0 || receipt != nil || !strings.Contains(stderr, "invalid-arguments") {
			t.Fatalf("%v: exit %d stdout=%s stderr=%s", options, code, stdout, stderr)
		}
	}
	receipt, _, stderr, code := runAffectedCLI(t, root, "--path=-dash.txt")
	if code != 0 || !slices.Equal(anyStrings(receipt["path_list"]), []string{"-dash.txt"}) {
		t.Fatalf("--path=-dash.txt: exit %d stderr=%s receipt=%v", code, stderr, receipt)
	}
}
