package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// affectedSelectedVia maps each selected unit to its witness chain.
func affectedSelectedVia(t *testing.T, receipt map[string]any) map[string][]string {
	t.Helper()
	via := map[string][]string{}
	for _, item := range receipt["plan"].(map[string]any)["selected"].([]any) {
		selection := item.(map[string]any)
		via[selection["unitId"].(string)] = anyStrings(selection["witness"].(map[string]any)["via"])
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
			if !bytes.Equal(extractJSONObject(t, ranged, "plan"), extractJSONObject(t, listed, "plan")) {
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
	for _, want := range []string{"stop at direct importers (one hop", "a READY receipt is evidence, not a closed test selection", "corvint affected --path PATH..."} {
		if !strings.Contains(impact, want) {
			t.Fatalf("impact help lacks %q", want)
		}
	}
	affectedText := strings.Join(strings.Fields(helpText("affected")), " ")
	for _, want := range []string{"affected [--full] --path PATH [--path PATH ...]", "a test two importers away is selected", "path_list", "unsupported-affected-mixed-worktree"} {
		if !strings.Contains(affectedText, want) {
			t.Fatalf("affected help lacks %q", want)
		}
	}
}
