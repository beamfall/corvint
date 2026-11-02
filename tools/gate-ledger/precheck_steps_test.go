// SPDX-License-Identifier: Apache-2.0
package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// precheckLedgerNames is every ledger NAME `script/batch-precheck.sh` keys a
// step under (`run_one`: the target of `make T`, go-format-check for a gofmt
// step, batch-precheck for any other command). A new batch-precheck step must
// be added here, so its input classification is decided, not inherited.
var precheckLedgerNames = []string{
	"ai-coauthor-trailers-check", "bash32-check", "batch-precheck", "binary-blobs-check",
	"capabilities-check", "capability-catalog-check", "decision-numbers-check",
	"declaration-compat-check", "documentation-paths-check", "error-code-ownership-check",
	"fixed-tmp-paths-check", "flow-guide-check", "go-format-check", "go-version-pins-check",
	"host-adapter-node-test", "host-package-versions-check", "host-package-versions-test",
	"lane-scripts-test", "legal-file-pins-check", "license-headers-check", "line-citations-check",
	"mkfifo-build-tags-check", "requirement-definitions-check", "selection-baseline-check",
	"shell-foreign-callers-check", "shell-test-registration-check", "spec-requirements-check",
	"test-commands-check", "test-read-scopes-check", "traceability-tests-check",
	"traceability-tests-test", "unbounded-readers-check", "use-case-receipts-check",
	"use-case-receipts-test",
}

// TestPrecheckLedgerNamesAreComplete_GLV0785 ties the table to its sources:
// every DOC_GATE_TARGETS entry in Makefile and every EXTRA_TARGETS entry in
// script/batch-precheck.sh is listed, so no batch-precheck make step escapes
// the classification below.
func TestPrecheckLedgerNamesAreComplete_GLV0785(t *testing.T) {
	listed := map[string]bool{}
	for _, name := range precheckLedgerNames {
		listed[name] = true
	}
	sources := map[string]*regexp.Regexp{
		filepath.Join("..", "..", "Makefile"):                    regexp.MustCompile(`(?m)^DOC_GATE_TARGETS := (.*)$`),
		filepath.Join("..", "..", "script", "batch-precheck.sh"): regexp.MustCompile(`(?m)^EXTRA_TARGETS=\(([^)]*)\)`),
	}
	for file, re := range sources {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		m := re.FindSubmatch(data)
		if m == nil {
			t.Fatalf("%s: no target list", file)
		}
		for _, target := range strings.Fields(string(m[1])) {
			if !listed[target] {
				t.Errorf("%s names %s, which precheckLedgerNames does not classify", file, target)
			}
		}
	}
}

// inScopeSample is one path an edit to which a scope entry covers.
func inScopeSample(entry string) (string, bool) {
	switch {
	case entry == treeScope || entry == neverScope || strings.HasPrefix(entry, "!"):
		return "", false
	case entry == "@go":
		return "sample.go", true
	case strings.HasPrefix(entry, "*."):
		return "sample" + strings.TrimPrefix(entry, "*"), true
	case entry == "@root":
		return "ROOT-SAMPLE", true
	case strings.HasPrefix(entry, "@"):
		return "", false
	case strings.Contains(entry, "*"):
		return strings.Replace(entry, "*", "sample", 1), true
	case strings.HasSuffix(entry, "/"):
		return entry + "sample.txt", true
	}
	return entry, true
}

// TestEveryPrecheckStepDeclaresOrKeysOnTheTree_GLV0785 replays V1-4160's
// acceptance for every batch-precheck ledger step, as the step runs there
// (`NAME+SALT`). An undeclared step keys on the whole tree, so no input
// dimension is outside its key: a CEM-only change, a build-log edit and a
// source edit each change it. A declared step keys exactly on its scope plus
// the gate tooling: an edit under each declared entry changes its key, and a
// CEM-only change outside every entry does not. A `@test:` or `@deps:` step's
// closure is proven by TestRunStepKeysTestScopeOnPackageClosure and
// TestRunStepKeysDepsAndAllScope_GLV0787; a `never` step always runs.
func TestEveryPrecheckStepDeclaresOrKeysOnTheTree_GLV0785(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the fixture repository is driven through git")
	root := fixtureRepository(t)
	key := func(step string) string {
		t.Helper()
		l := open(io.Discard, io.Discard)
		k, reason := l.key(step, nil)
		if reason != "" {
			t.Fatalf("%s: %s", step, reason)
		}
		return k
	}
	edit := func(file string) {
		t.Helper()
		name := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		body, _ := os.ReadFile(name)
		if err := os.WriteFile(name, append(body, 'x'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names := append([]string(nil), precheckLedgerNames...)
	sort.Strings(names)
	for _, name := range names {
		step := name + "+salt"
		scope, declared := scopes[name]
		switch {
		case declared && len(scope) == 1 && scope[0] == neverScope:
			continue
		case declared && len(scope) > 0 && (strings.HasPrefix(scope[0], testScopePrefix) || strings.HasPrefix(scope[0], depsScopePrefix)):
			// A package-closure scope needs a Go package; its fixture tests cover it.
			continue
		case !declared || (len(scope) == 1 && scope[0] == treeScope):
			for _, file := range []string{".corvint/changes/" + name + ".cem.json", "docs/build-log/" + name + ".md", "main.go"} { //corvint:fixture-path
				before := key(step)
				edit(file)
				if key(step) == before {
					t.Errorf("%s: tree-keyed, but an edit to %s kept its key", name, file)
				}
			}
		default:
			cem := ".corvint/changes/" + name + ".cem.json" //corvint:fixture-path
			coversCEM := inScope(cem, scope)
			before := key(step)
			edit(cem)
			if after := key(step); !coversCEM && after != before {
				t.Errorf("%s: a CEM-only change outside its scope %q changed its key", name, scope)
			}
			for _, entry := range scope {
				sample, ok := inScopeSample(entry)
				if !ok {
					continue
				}
				before := key(step)
				edit(sample)
				if key(step) == before {
					t.Errorf("%s: an edit to %s under its declared entry %q kept its key", name, sample, entry)
				}
			}
		}
	}
}
