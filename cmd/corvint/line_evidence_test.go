// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/lineevidence"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testinventory"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
	"github.com/Beamfall/corvint/internal/testrunner/killswitch"
	"slices"
)

func TestLEVV1001RefreshAndConsumers(t *testing.T) {
	root := inventoryRepository(t, map[string]string{
		"go.mod":       "module example.com/lev\n\ngo 1.27.4\n",
		"calc.go":      "package lev\nfunc Add(a,b int)int{return a+b}\n",
		"calc_test.go": "package lev\nimport \"testing\"\nfunc TestAdd(t *testing.T){if Add(1,2)!=3{t.Fatal(\"sum\")}}\n",
	})
	code, out, diagnostic := inventoryCLI(t, "--root", root, "line-evidence", "refresh", "--runner", "go")
	if code != 0 {
		t.Fatalf("refresh %d: %s %s", code, out, diagnostic)
	}
	var refreshed []lineevidence.Refreshed
	if err := json.Unmarshal([]byte(out), &refreshed); err != nil || len(refreshed) != 1 {
		t.Fatalf("%s: %v", out, err)
	}
	name := refreshed[0].Path
	before, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic = inventoryCLI(t, "--root", root, "line-evidence", "refresh", "--runner", "go")
	if code != 0 {
		t.Fatalf("second refresh %d: %s %s", code, out, diagnostic)
	}
	again, _ := os.ReadFile(name)
	if !bytes.Equal(before, again) {
		t.Fatal("same tree produced different index bytes")
	}
	ix, err := linecov.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	if ix.RunnerFamily != "go" || ix.Config != "go.mod" {
		t.Fatalf("identity %+v", ix)
	}
	found := false
	for _, test := range ix.Tests {
		if test.Name == "go:.::TestAdd" {
			found = true
		}
	}
	if !found {
		t.Fatal("canonical ID missing")
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for _, staged := range []bool{true, false} {
		if staged {
			writeInventoryFiles(t, root, map[string]string{"calc.go": "package lev\nfunc Add(a,b int)int{return a+b+1}\n"})
			affectedGit(t, root, "add", "calc.go")
		} else {
			writeInventoryFiles(t, root, map[string]string{"extra.go": "package lev\nfunc Extra(){}\n"})
		}
		_, raw, stderr, code := runAffectedCLI(t, root, "--base", base)
		if code != 0 {
			t.Fatalf("affected %d %s", code, stderr)
		}
		reason := lineevidence.Unqualified
		if !staged {
			reason = lineevidence.Stale
		}
		if !bytes.Contains(raw, []byte(`"tier":"file"`)) || !bytes.Contains(raw, []byte(reason)) {
			t.Fatalf("affected missing file reason: %s", raw)
		}
		_, summary, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
		if code != 0 {
			t.Fatalf("summary %d %s", code, stderr)
		}
		responses := mcpAffectedChild(t, root, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"`+base+`"}}}`)
		checkMCPAffectedAnswer(t, responses["1"], summary)
		code, out, stderr = inventoryCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--accept-stale-binary")
		if code != 0 && code != 3 {
			t.Fatalf("test %d: %s %s", code, out, stderr)
		}
		if !strings.Contains(stderr, reason) {
			t.Fatalf("test lost reason: %s", stderr)
		}
	}
	code, out, diagnostic = inventoryCLI(t, "--root", root, "watch", "--index", name, "--once", "--dry-run")
	if code != 0 || !strings.Contains(out, lineevidence.Stale) {
		t.Fatalf("watch %d %s %s", code, out, diagnostic)
	}
	after, _ := os.ReadFile(name)
	if !bytes.Equal(before, after) {
		t.Fatal("consumer modified private evidence")
	}
}

func TestLEVV1006PytestFileTier(t *testing.T) {
	root := inventoryRepository(t, map[string]string{"pytest.ini": "[pytest]\n", "app.py": "def add(a,b): return a+b\n", "test_app.py": "from app import add\ndef test_add(): assert add(1,2)==3\n"})
	if err := os.MkdirAll(filepath.Join(root, ".git", "corvint", "line-evidence"), 0700); err != nil {
		t.Fatal(err)
	}

	writeInventoryFiles(t, root, map[string]string{"app.py": "def add(a,b): return a+b+1\n"})
	_, raw, stderr, code := runAffectedCLI(t, root)
	if code != 0 || !bytes.Contains(raw, []byte(`"tier":"file"`)) || !bytes.Contains(raw, []byte("pytest-file-level")) {
		t.Fatalf("%d %s %s", code, raw, stderr)
	}
}

func TestLEVV1012VitestWatch(t *testing.T) {
	root := inventoryRepository(t, map[string]string{
		"vitest.config.ts": "export default {}\n", "package.json": "{\"type\":\"module\"}\n",
		"sum.js": "export const sum=(a,b)=>a+b;\n", "sum.test.js": "import {sum} from './sum.js'; test('sum',()=>sum(1,2));\n",
	})
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	oids, err := linecov.OIDs(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build(".", head, "unavailable-runtime", oids, []linecov.Record{{Package: "sum.test.js", Name: "vitest:sum.test.js::sum", Profile: []byte("mode: atomic\n./sum.js:1.1,1.33 1 1\n")}})
	if err != nil {
		t.Fatal(err)
	}
	ix.Language, ix.RunnerFamily, ix.Config = "js", "vitest", "vitest.config.ts"
	ix.Command = []string{"node_modules/.bin/vitest", "run"}
	d := testinventory.Declared{Key: "vitest@.", Runner: "vitest", Root: ".", Config: "vitest.config.ts"}
	name, err := lineevidence.Write(filepath.Join(root, ".git", "corvint", "line-evidence"), d, ix)
	if err != nil {
		t.Fatal(err)
	}
	writeInventoryFiles(t, root, map[string]string{"sum.js": "export const sum=(a,b)=>a+b+1;\n"})
	code, out, stderr := inventoryCLI(t, "--root", root, "watch", "--index", name, "--once", "--dry-run")
	if code != 0 || !strings.Contains(out, lineevidence.Stale) || strings.Contains(stderr, "watch-index-language") {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	for _, cause := range []string{"unavailable-commit", "unknown-language"} {
		t.Run(cause, func(t *testing.T) {
			stale := *ix
			if cause == "unavailable-commit" {
				stale.Commit = strings.Repeat("a", 40)
			} else {
				stale.Language = "future-language"
			}
			if _, err := lineevidence.Write(filepath.Dir(name), d, &stale); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := inventoryCLI(t, "--root", root, "watch", "--index", name, "--once", "--dry-run")
			if code != 0 || !strings.Contains(out, lineevidence.Stale) || !strings.Contains(out, `"tier":"full"`) {
				t.Fatalf("%d %s %s", code, out, stderr)
			}
			after, err := os.ReadFile(name)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("watch mutated evidence: %v", err)
			}
		})
	}
}

func TestLEVV1008ExperimentalProfile(t *testing.T) {
	root := inventoryRepository(t, map[string]string{"README.md": "fixture\n"})
	_, _, stderr, code := runAffectedCLI(t, root, "--selection-profile", e2eLineExperimental)
	if code != 0 {
		t.Fatalf("%d %s", code, stderr)
	}
	code, out, stderr := inventoryCLI(t, "line-evidence", "refresh", "--help")
	if code != 0 || !strings.Contains(out, "Example:") || !strings.Contains(out, "--runner") {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
}

// levLanguage supplies a file plan independent of coverage: A is retained, and
// B is not statically reached. Evidence may only add B and rank its canonical ID.
type levLanguage struct{}

func (levLanguage) Name() string          { return "typescript" }
func (levLanguage) Owns(name string) bool { return strings.HasSuffix(name, ".js") }
func (levLanguage) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{Units: []affected.Unit{
		{ID: "typescript:app", Sources: []string{"app.js"}},
		{ID: "typescript:playwright:a.spec.js", Tests: []string{"a.spec.js"}},
		{ID: "typescript:playwright:b.spec.js", Tests: []string{"b.spec.js"}},
	}}, nil
}

func TestLEVV1007PlaywrightOnlyAddsAndRanks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX version-probe fixture")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node unavailable")
	}
	root := inventoryRepository(t, map[string]string{".gitignore": "node_modules/\n", "playwright.config.js": "export default {}\n", "app.js": "export const value=1;\n", "a.spec.js": "// retained by accepted flow\n", "b.spec.js": "// reached only by evidence\n"})
	writeInventoryFiles(t, root, map[string]string{"node_modules/.bin/playwright": "#!/bin/sh\necho 'Version 1.56.0'\n"})
	if err := os.Chmod(filepath.Join(root, "node_modules/.bin/playwright"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	git := testinventory.Git{Executable: "git", Root: root}
	head, err := git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := testinventory.NewWorkTree(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := tree.Files()
	d := testinventory.Declared{Key: "playwright@.", Runner: "playwright", Root: ".", Config: "playwright.config.js"}
	native, err := lineevidence.NativeIdentity(ctx, root, d)
	if err != nil {
		t.Fatal(err)
	}
	command := lineevidence.Command(d)
	ix, err := linecov.Build(".", head, lineevidence.Binding(d, files, command, native), files, []linecov.Record{{Package: "b.spec.js", Name: "playwright:b.spec.js::chromium::covers", Profile: []byte("mode: atomic\n./app.js:1.1,1.22 1 1\n")}})
	if err != nil {
		t.Fatal(err)
	}
	ix.Language, ix.RunnerFamily, ix.Config, ix.Command = "js", d.Runner, d.Config, command
	store, err := lineevidence.Store(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store, 0700); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := ix.Encode(&encoded); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, lineevidence.Filename(d)), encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	writeInventoryFiles(t, root, map[string]string{"app.js": "export const value=2;\n"})
	graph, err := affected.Build(ctx, root, levLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Plan{Dirty: []string{"app.js"}, Selected: []affected.Selection{{UnitID: "typescript:playwright:a.spec.js", Tests: []string{"a.spec.js"}, Flows: []string{"accepted-flow"}}}, Excluded: []affected.Exclusion{{UnitID: "typescript:playwright:b.spec.js"}}}
	applyLineEvidence(ctx, affectedPlanInputs{gitExecutable: "git", repository: root, tree: root, revision: head}, graph, &plan)
	if len(plan.Selected) != 2 || len(plan.Excluded) != 0 {
		t.Fatalf("%+v", plan)
	}
	if plan.Selected[0].UnitID != "typescript:playwright:a.spec.js" || plan.Selected[0].Tier != "flow" || plan.Selected[1].Tier != "full" || len(plan.Selected[1].Rank) != 1 {
		t.Fatalf("%+v", plan.Selected)
	}
	for _, s := range plan.Selected {
		if s.Exact == nil || *s.Exact || s.Reason != lineevidence.Unqualified {
			t.Fatalf("%+v", s)
		}
	}
}

func TestLEVV1003StaleInstanceExecution(t *testing.T) {
	root := inventoryRepository(t, map[string]string{
		"go.mod":              "module example.com/lev\n\ngo 1.27.4\n",
		"calc.go":             "package lev\nfunc Add(a,b int)int{return a+b}\n",
		"calc_test.go":        "package lev\nimport \"testing\"\nfunc TestAdd(t *testing.T){Add(1,2)}\n",
		"other/other_test.go": "package other\nimport \"testing\"\nfunc TestOther(t *testing.T){}\n",
	})
	ctx := context.Background()
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	oids, err := linecov.OIDs(ctx, root, head)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("example.com/lev", head, "different-runner", oids, []linecov.Record{{Package: ".", Name: "go:.::TestAdd", Profile: []byte("mode: atomic\nexample.com/lev/calc.go:2.1,2.32 1 1\n")}})
	if err != nil {
		t.Fatal(err)
	}
	ix.Language, ix.RunnerFamily, ix.Config, ix.Command = "go", "go", "go.mod", []string{"go", "test", "./..."}
	d := testinventory.Declared{Key: "go@.", Runner: "go", Root: ".", Config: "go.mod"}
	if _, err := lineevidence.Write(filepath.Join(root, ".git", "corvint", "line-evidence"), d, ix); err != nil {
		t.Fatal(err)
	}
	writeInventoryFiles(t, root, map[string]string{"calc.go": "package lev\nfunc Add(a,b int)int{return a+b+1}\n"})
	input, _ := testRunInput(ctx, root, testOptions{base: head}, sessiontest.SourceIdentity{}, nil)
	if input.FullInstances[d.Key] != lineevidence.Stale {
		t.Fatalf("missing full instance: %+v", input.FullInstances)
	}
	if len(input.GoSlices) != 2 {
		t.Fatalf("lost packages: %+v", input.GoSlices)
	}
	for _, slice := range input.GoSlices {
		if slice.Pattern != "" {
			t.Fatalf("stale evidence narrowed: %+v", slice)
		}
	}
	testInventoryScope(ctx, root, &input, killswitch.Switch{})
	if input.FullInstances[d.Key] != lineevidence.Stale {
		t.Fatalf("inventory lost full instance: %+v", input.FullInstances)
	}
}

func TestLEVV1002NestedInstance(t *testing.T) {
	root := testinventory.Declared{Key: "go@.", Runner: "go", Root: "."}
	nested := testinventory.Declared{Key: "go@a", Runner: "go", Root: "a"}
	unit := affected.Unit{ID: "go:example.com/nested", Tests: []string{"a/calc_test.go"}}
	for _, declarations := range [][]testinventory.Declared{{root, nested}, {nested, root}} {
		got, ok := lineInstance(unit, declarations)
		if !ok || got.Key != nested.Key {
			t.Fatalf("one-character root lost ownership: %+v", got)
		}
	}
}

// Every unit already has a flow link: no added unlinked unit can accidentally
// preserve the instance fallback for the linked units.
func TestLEVV1003StalePlaywrightExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX runner fixture")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node unavailable")
	}
	root := inventoryRepository(t, map[string]string{
		".gitignore": "node_modules/\n", "playwright.config.js": "export default {}\n",
		"app.js": "export const value=1;\n", "a.spec.js": "// flow A\n", "b.spec.js": "// flow B\n",
	})
	writeInventoryFiles(t, root, map[string]string{"node_modules/.bin/playwright": "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'Version 1.56.0'; exit; fi\nprintf '%s\\n' \"$@\"\n"})
	if err := os.Chmod(filepath.Join(root, "node_modules/.bin/playwright"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	git := testinventory.Git{Executable: "git", Root: root}
	head, err := git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := testinventory.NewWorkTree(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := tree.Files()
	d := testinventory.Declared{Key: "playwright@.", Runner: "playwright", Root: ".", Config: "playwright.config.js"}
	native, err := lineevidence.NativeIdentity(ctx, root, d)
	if err != nil {
		t.Fatal(err)
	}
	command := lineevidence.Command(d)
	ix, err := linecov.Build(".", head, lineevidence.Binding(d, files, command, native), files, []linecov.Record{{Package: "a.spec.js", Name: "playwright:a.spec.js::chromium::covers", Profile: []byte("mode: atomic\n./app.js:1.1,1.22 1 1\n")}})
	if err != nil {
		t.Fatal(err)
	}
	ix.Language, ix.RunnerFamily, ix.Config, ix.Command = "js", d.Runner, d.Config, command
	store, err := lineevidence.Store(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store, 0700); err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(ctx, root, levLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	writeInventoryFiles(t, root, map[string]string{"app.js": "export const value=2;\n"})
	for _, cause := range []string{"identity", "unavailable-commit", "unknown-language"} {
		t.Run(cause, func(t *testing.T) {
			stale := *ix
			switch cause {
			case "identity":
				stale.Runner = "different"
			case "unavailable-commit":
				stale.Commit = strings.Repeat("a", 40)
			case "unknown-language":
				stale.Language = "future-language"
			}
			var encoded bytes.Buffer
			if err := stale.Encode(&encoded); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(store, lineevidence.Filename(d)), encoded.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			plan := affected.Plan{Dirty: []string{"app.js"}, Selected: []affected.Selection{
				{UnitID: "typescript:playwright:a.spec.js", Tests: []string{"a.spec.js"}, Flows: []string{"flow-a"}},
				{UnitID: "typescript:playwright:b.spec.js", Tests: []string{"b.spec.js"}, Flows: []string{"flow-b"}},
			}}
			applyLineEvidence(ctx, affectedPlanInputs{gitExecutable: "git", repository: root, tree: root, revision: head}, graph, &plan)
			if len(plan.Selected) != 2 {
				t.Fatalf("lost selection: %+v", plan)
			}
			for _, selection := range plan.Selected {
				if selection.Tier != "full" || selection.Reason != lineevidence.Stale {
					t.Fatalf("stale flow: %+v", selection)
				}
			}
			// Execution must not infer the fallback from presentation labels, including
			// a label a future producer might add.
			for i := range plan.Selected {
				plan.Selected[i].Tier = "future-tier"
			}
			input := sessiontest.Input{ShardCount: 1, RunnerProfile: sessiontest.ProfileLocal,
				Declared: sessiontest.BuiltInDeclared(), Instances: []contract.Instance{{Runner: d.Runner, Root: d.Root, Config: d.Config}}}
			input.Declared.Runners = []sessiontest.RunnerCommand{{Key: d.Runner, Argv: command}}
			testPlanScope(&input, affectedReceipt{Plan: plan}, gateselect.Receipt{}, graph, root, nil)
			if input.FullInstances[d.Key] != lineevidence.Stale {
				t.Fatalf("fallback lost: %+v", input.FullInstances)
			}
			manifest, err := sessiontest.Build(input)
			if err != nil {
				t.Fatal(err)
			}
			runs := sessiontest.Invocations(manifest, input.Declared, 0, root, "")
			if len(runs) != 1 || !slices.Equal(runs[0].Argv, command) {
				t.Fatalf("filtered execution: %+v; manifest %+v", runs, manifest)
			}
			run := exec.CommandContext(ctx, runs[0].Argv[0], runs[0].Argv[1:]...)
			run.Dir = root
			out, err := run.CombinedOutput()
			if err != nil || string(out) != "test\n" {
				t.Fatalf("execution %v: %s", err, out)
			}
		})
	}
}

func TestLEVV1003PlaywrightFlowAllowlist(t *testing.T) {
	for _, tc := range []struct {
		tier   lineevidence.Tier
		reason string
		flow   bool
	}{
		{lineevidence.File, lineevidence.Unqualified, true},
		{lineevidence.File, lineevidence.Stale, true},
		{lineevidence.Full, lineevidence.Absent, true},
		{lineevidence.Full, lineevidence.Damaged, true},
		{lineevidence.Full, lineevidence.Stale, false},
		{lineevidence.File, "future-reason", false},
		{"future-tier", lineevidence.Unqualified, false},
	} {
		if got := playwrightLineFlow(lineevidence.Verdict{Tier: tc.tier, Reason: tc.reason}); got != tc.flow {
			t.Errorf("%s/%s: flow=%t want %t", tc.tier, tc.reason, got, tc.flow)
		}
	}
}

// LEV-V1-003: a root edit still executes a stale, unobserved nested module.
// A future evidence language is not a certificate for omitting that suite.
func TestLEVV1003StaleNestedModuleManifest(t *testing.T) {
	root := inventoryRepository(t, map[string]string{
		"go.mod":               "module example.com/lev\n\ngo 1.27.4\n",
		"calc.go":              "package lev\nfunc Add(a,b int)int{return a+b}\n",
		"calc_test.go":         "package lev\nimport \"testing\"\nfunc TestAdd(t *testing.T){Add(1,2)}\n",
		"nested/go.mod":        "module example.com/nested\n\ngo 1.27.4\n",
		"nested/other_test.go": "package nested\nimport \"testing\"\nfunc TestOther(t *testing.T){}\n",
	})
	ctx := context.Background()
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	d := testinventory.Declared{Key: "go@nested", Runner: "go", Root: "nested", Config: "nested/go.mod"}
	ix, err := linecov.Build("example.com/nested", head, "different-runner", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ix.RunnerFamily, ix.Config, ix.Command = "go", d.Config, lineevidence.Command(d)
	writeInventoryFiles(t, root, map[string]string{"calc.go": "package lev\nfunc Add(a,b int)int{return a+b+1}\n"})
	for _, language := range []string{"go", "future-language"} {
		t.Run(language, func(t *testing.T) {
			ix.Language = language
			if _, err := lineevidence.Write(filepath.Join(root, ".git", "corvint", "line-evidence"), d, ix); err != nil {
				t.Fatal(err)
			}
			for _, profile := range []string{sessiontest.ProfileLocal, sessiontest.ProfileCI} {
				input, _ := testRunInput(ctx, root, testOptions{base: head}, sessiontest.SourceIdentity{}, nil)
				input.RunnerProfile, input.ShardCount = profile, 1
				testInventoryScope(ctx, root, &input, killswitch.Switch{})
				if input.FullInstances[d.Key] != lineevidence.Stale {
					t.Fatalf("fallback lost: %+v", input)
				}
				manifest, err := sessiontest.Build(input)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, invocation := range sessiontest.Invocations(manifest, input.Declared, 0, root, "") {
					if invocation.Dir == "nested" {
						found = true
						if !slices.Contains(invocation.Argv, "./...") || slices.Contains(invocation.Argv, "-run") {
							t.Fatalf("narrowed module: %+v", invocation)
						}
					}
				}
				if profile == sessiontest.ProfileLocal && !found {
					t.Fatalf("missing module: %+v", manifest)
				}
				if profile == sessiontest.ProfileCI && (found || !slices.Contains(manifest.Checks, sessiontest.ModuleCheck("nested"))) {
					t.Fatalf("CI obligation: %+v", manifest)
				}
			}
		})
	}
}

func TestLEVV1003FileVerdictAllowlist(t *testing.T) {
	for _, tc := range []struct {
		tier    lineevidence.Tier
		reason  string
		bounded bool
	}{
		{lineevidence.File, lineevidence.Absent, true},
		{lineevidence.File, lineevidence.Damaged, true},
		{lineevidence.File, lineevidence.Unqualified, true},
		{lineevidence.File, lineevidence.Stale, true},
		{lineevidence.Full, lineevidence.Stale, false},
		{lineevidence.File, "future-reason", false},
		{"future-tier", lineevidence.Unqualified, false},
	} {
		if got := lineEvidenceFileBounded(lineevidence.Verdict{Tier: tc.tier, Reason: tc.reason}); got != tc.bounded {
			t.Fatalf("%+v: bounded=%t", tc, got)
		}
	}
}

// LEV-V1-007/012: exercise the complete summary producer, not only plan.Selected.
func TestLEVV1007EvidenceSummaryProjection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX version-probe fixture")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node unavailable")
	}
	root, base := flowPlanCLIRepo(t, flowPlanCLIDeclaration, flowPlanCLIConfig)
	baseline, _, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
	if code != 0 {
		t.Fatalf("baseline %d: %s", code, stderr)
	}
	writeInventoryFiles(t, root, map[string]string{
		".git/info/exclude":            "node_modules/\n",
		"node_modules/.bin/playwright": "#!/bin/sh\necho 'Version 1.56.0'\n",
	})
	if err := os.Chmod(filepath.Join(root, "node_modules/.bin/playwright"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	git := testinventory.Git{Executable: "git", Root: root}
	tree, err := testinventory.NewWorkTree(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := tree.Files()
	all := testinventory.Detect(files, tree.Read)
	var d testinventory.Declared
	for _, instance := range all {
		if instance.Runner == "playwright" {
			d = instance
		}
	}
	if d.Key == "" {
		t.Fatal("no Playwright instance")
	}
	native, err := lineevidence.NativeIdentity(ctx, root, d)
	if err != nil {
		t.Fatal(err)
	}
	command := lineevidence.CommandForTree(d, files, all)
	oids, err := linecov.OIDs(ctx, root, base)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build(".", base, lineevidence.Binding(d, files, command, native), oids, []linecov.Record{{
		Package: "e2e/search.spec.ts", Name: "playwright:e2e/search.spec.ts::chromium::finds",
		Profile: []byte("mode: atomic\n./src/checkout.ts:4.1,4.23 1 1\n"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	ix.Language, ix.RunnerFamily, ix.Config, ix.Command = "js", d.Runner, d.Config, command
	store, err := lineevidence.Store(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	for _, cause := range []string{"fresh", "identity", "unknown-language"} {
		t.Run(cause, func(t *testing.T) {
			index := *ix
			if cause == "identity" {
				index.Runner = "different"
			}
			if cause == "unknown-language" {
				index.Language = "future-language"
			}
			var encoded bytes.Buffer
			if err := index.Encode(&encoded); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(store, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(store, lineevidence.Filename(d)), encoded.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			summary, raw, stderr, code := runAffectedCLI(t, root, "--summary", "--base", base)
			if code != 0 {
				t.Fatalf("summary %d: %s", code, stderr)
			}
			scope := "BOUNDED"
			if cause != "fresh" {
				scope = "FULL"
			}
			want := map[string]any{
				"scope": scope, "inventory": float64(3),
				"selected": []any{
					map[string]any{"path": "e2e/checkout.spec.ts", "project": "chromium"},
					map[string]any{"path": "e2e/orphan.spec.ts", "project": "chromium"},
					map[string]any{"path": "e2e/search.spec.ts", "project": "chromium"},
				},
				"variations": baseline["e2e"].(map[string]any)["variations"],
			}
			if cause != "fresh" {
				want["unknown"] = []any{affected.UnknownE2EPlanUnbounded}
			}
			if summary["profile"] != "affected-summary/1" || !reflect.DeepEqual(summary["e2e"], want) {
				t.Fatalf("summary: %s\nwant e2e: %#v", raw, want)
			}
			full, _, stderr, code := runAffectedCLI(t, root, "--base", base)
			if code != 0 {
				t.Fatalf("plan %d: %s", code, stderr)
			}
			plan := full["plan"].(map[string]any)
			for _, item := range plan["selected"].([]any) {
				selection := item.(map[string]any)
				if kind, ok := selection["testKind"].(string); !ok || kind == "" {
					t.Fatalf("missing test kind: %+v", selection)
				}
			}
			if cause == "fresh" {
				responses := mcpAffectedChild(t, root, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"corvint.affected","arguments":{"base":"`+base+`"}}}`)
				checkMCPAffectedAnswer(t, responses["1"], raw)
			}
		})
	}
}
