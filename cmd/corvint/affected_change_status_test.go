// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// affectedSelectedShellTests lists the shell test files a compact affected
// receipt selects, one per selected shell unit.
func affectedSelectedShellTests(t *testing.T, receipt map[string]any) []string {
	t.Helper()
	var tests []string
	plan, _ := receipt["plan"].(map[string]any)
	selected, _ := plan["selected"].([]any)
	for _, raw := range selected {
		encoded, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		var selection struct {
			UnitID string `json:"unitId"`
		}
		if err := json.Unmarshal(encoded, &selection); err != nil {
			t.Fatalf("selection %s: %v", encoded, err)
		}
		if test, ok := strings.CutPrefix(selection.UnitID, "shell:"); ok {
			tests = append(tests, test)
		}
	}
	return tests
}

// AFP-V0-9960: `corvint affected --base` passes each changed path's status
// to the planner, so an edit of an existing file does not select a
// name-listing test (AFP-V0-9962) or a name-only walker (AFP-V0-9963), while
// an added file does, and a plan with no base keeps every status unknown.
func TestAffectedChangeStatusCLI_AFPV09960(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"script/a.sh":    "true\n",
		"docs/x.md":      "x\n",
		"lister_test.sh": "test \"$(ls script | wc -l)\" -eq 1\n",
		"walker_test.sh": "find . -name '*.md' | wc -l\n",
		"long_test.sh":   "ls -l script | wc -l\n",
	}
	for relative, body := range files {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	plan := func(options ...string) []string {
		t.Helper()
		receipt, _, stderr, code := runAffectedCLI(t, root, options...)
		if code != 0 {
			t.Fatalf("affected %v: exit %d: %s", options, code, stderr)
		}
		return affectedSelectedShellTests(t, receipt)
	}
	// An edit of an existing file in the listed directory and of a file the
	// walker lists, against the base.
	if err := os.WriteFile(filepath.Join(root, "script", "a.sh"), []byte("false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "x.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edited := plan("--base", base)
	for _, test := range []string{"lister_test.sh", "walker_test.sh"} {
		if slices.Contains(edited, test) {
			t.Errorf("edit with --base: %s selected: %v", test, edited)
		}
	}
	if !slices.Contains(edited, "long_test.sh") {
		t.Errorf("edit with --base: long_test.sh (ls -l, a content read) not selected: %v", edited)
	}
	// Without a base every status is unknown, which selects as before.
	unknown := plan()
	for _, test := range []string{"lister_test.sh", "walker_test.sh", "long_test.sh"} {
		if !slices.Contains(unknown, test) {
			t.Errorf("edit without --base: %s not selected: %v", test, unknown)
		}
	}
	// An untracked addition is added, which can change both listings.
	if err := os.WriteFile(filepath.Join(root, "script", "b.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	added := plan("--base", base)
	for _, test := range []string{"lister_test.sh", "walker_test.sh", "long_test.sh"} {
		if !slices.Contains(added, test) {
			t.Errorf("addition with --base: %s not selected: %v", test, added)
		}
	}
}
