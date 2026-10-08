package typescript

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
)

// mochaSelectors derives exact Mocha file selectors from a plan. It refuses,
// with the retained reason, whenever the plan is not a bounded selection of
// Mocha-owned units, so uncertainty never becomes a narrowed command.
func mochaSelectors(plan affected.Plan) ([]string, string) {
	if plan.Scope != affected.ScopeBounded {
		return nil, "plan scope " + plan.Scope
	}
	if len(plan.Selected) == 0 {
		return nil, "no selected test unit"
	}
	var out []string
	for _, s := range plan.Selected {
		relative, ok := strings.CutPrefix(s.UnitID, "typescript:"+runnerMocha+":")
		if !ok {
			return nil, "selected unit is not Mocha-owned: " + s.UnitID
		}
		out = append(out, relative)
	}
	sort.Strings(out)
	return out, ""
}

type mochaScenario struct {
	name      string
	files     map[string]string
	change    map[string]string
	selectors []string // executed even when the plan refuses, to prove the receipt still refuses
	expected  []string
	scope     string
	refusal   string
	complete  bool
	problems  []string
}

// Opt in with independently selected local tools. This proves local tuples,
// not dependency closure, runtime authority, or a general exact-discovery claim.
func TestMochaActualSelectionQualification(t *testing.T) {
	node, cli, output := os.Getenv("CORVINT_MOCHA_NODE"), os.Getenv("CORVINT_MOCHA_CLI"), os.Getenv("CORVINT_MOCHA_EVIDENCE")
	if node == "" || cli == "" || output == "" {
		t.Skip("requires explicitly selected native Mocha tools and evidence directory")
	}
	if !filepath.IsAbs(node) || !filepath.IsAbs(cli) || !filepath.IsAbs(output) {
		t.Fatal("absolute tools and evidence directory required")
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	digest := func(p string) string {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return tr.Digest(b)
	}
	launcher := filepath.Join(output, "mocha-launcher.cjs")
	modules := filepath.Dir(filepath.Dir(filepath.Dir(cli)))
	launch := fmt.Sprintf("#!%s\nprocess.env.NODE_PATH=%q;require('node:module').Module._initPaths();require(%q);\n", node, modules, cli)
	if e := os.WriteFile(launcher, []byte(launch), 0700); e != nil {
		t.Fatal(e)
	}
	base := map[string]string{
		"package.json":             `{"devDependencies":{"mocha":"12.0.3"}}`,
		"src/value.cjs":            "module.exports=1;\n",
		"src/orphan.cjs":           "module.exports=0;\n",
		"tests/related.test.cjs":   `const {it}=require("mocha");const value=require("../src/value.cjs");it("related",()=>{if(value!==2)throw Error("wrong value")});`,
		"tests/unrelated.test.cjs": `const {it}=require("mocha");it("unrelated",()=>{throw Error("must remain excluded")});`,
	}
	with := func(extra map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	changed := map[string]string{"src/value.cjs": "module.exports=2;\n"}
	related := []string{"tests/related.test.cjs::related"}
	scenarios := []mochaScenario{
		{name: "exact", files: base, change: changed, scope: affected.ScopeBounded, expected: related, complete: true},
		{name: "no-match-selector", files: base, change: changed, scope: affected.ScopeBounded, selectors: []string{"tests/missing.test.cjs", "tests/related.test.cjs"}, expected: related, problems: []string{"selector-without-tests"}},
		{name: "zero-test-selector", files: with(map[string]string{"tests/empty.test.cjs": `const {it}=require("mocha");`}), change: changed, scope: affected.ScopeBounded, selectors: []string{"tests/empty.test.cjs"}, problems: []string{"no-tests", "selector-without-tests"}},
		{name: "untested-change", files: base, change: map[string]string{"src/orphan.cjs": "module.exports=3;\n"}, scope: affected.ScopeUnknown, refusal: "plan scope " + affected.ScopeUnknown},
		{name: "configured-spec", files: with(map[string]string{".mocharc.json": `{"spec":["other/*.test.cjs"]}`, "other/extra.test.cjs": `const {it}=require("mocha");it("extra",()=>{})`}), change: changed, scope: affected.ScopeUnknown, refusal: "plan scope " + affected.ScopeUnknown, selectors: []string{"tests/related.test.cjs"}, expected: related, problems: []string{"unexpected-observed-test", "unselected-test-file"}},
		{name: "ambiguous-runner", files: with(map[string]string{".mocharc.json": `{}`, "jest.config.js": "module.exports={}"}), change: changed, scope: affected.ScopeUnknown, refusal: "plan scope " + affected.ScopeUnknown},
	}
	results := map[string]any{"mochaCliSha256": digest(cli), "nodeSha256": digest(node), "launcherSha256": digest(launcher)}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			dir := filepath.Join(output, sc.name)
			root := filepath.Join(dir, "repository")
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			for name, body := range sc.files {
				write(t, root, name, body)
			}
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
				cmd.Dir = root
				cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
				b, e := cmd.CombinedOutput()
				if e != nil {
					t.Fatalf("git %v: %s %v", args, b, e)
				}
				return strings.TrimSpace(string(b))
			}
			git("init", "-b", "main")
			git("config", "user.name", "Mocha Qualification")
			git("config", "user.email", "mocha@example.invalid")
			git("add", ".")
			git("commit", "-m", "base fixture")
			baseSHA := git("rev-parse", "HEAD")
			for name, body := range sc.change {
				write(t, root, name, body)
			}
			git("add", ".")
			git("commit", "-m", "changed source")
			target := git("rev-parse", "HEAD")
			graph, e := affected.Build(root, New())
			if e != nil {
				t.Fatal(e)
			}
			plan := affected.Select(graph, strings.Fields(git("diff", "--name-only", baseSHA, target)))
			selectors, refusal := mochaSelectors(plan)
			record := map[string]any{"base": baseSHA, "target": target, "plan": plan, "planSelectors": selectors, "planRefusal": refusal}
			defer func() { results[sc.name] = record }()
			if plan.Scope != sc.scope || refusal != sc.refusal {
				t.Fatalf("plan scope=%s refusal=%q: %+v", plan.Scope, refusal, plan)
			}
			if sc.name == "exact" && (len(selectors) != 1 || selectors[0] != "tests/related.test.cjs" || len(plan.Excluded) != 1) {
				t.Fatalf("exact selection %+v", plan)
			}
			if sc.name == "ambiguous-runner" && (len(plan.Selected) != 1 || plan.Selected[0].UnitID != unitID(runnerUnknown, "tests/related.test.cjs")) {
				t.Fatalf("ambiguous runner identity %+v", plan.Selected)
			}
			run := selectors
			if sc.selectors != nil {
				run = sc.selectors
			}
			if run == nil {
				return
			}
			inputs := map[string]string{}
			for name := range sc.files {
				inputs[name] = digest(filepath.Join(root, name))
			}
			var expected []string
			for _, id := range sc.expected {
				expected = append(expected, filepath.Join(root, id))
			}
			req := tr.Request{Runner: "mocha", Root: root, Target: target, Executable: launcher, ExecutableSha256: digest(launcher), Selectors: run, InputFiles: inputs, ExpectedTests: expected, Tools: map[string]tr.Tool{"node": {Executable: node, Sha256: digest(node)}}, ReportDir: filepath.Join(dir, "reports"), TimeoutSeconds: 60}
			inv, e := dynamic.Build(req)
			if e != nil {
				t.Fatal(e)
			}
			execution, e := tr.Execute(t.Context(), req, inv)
			if e != nil {
				t.Fatal(e)
			}
			parsed, e := dynamic.Parse(execution.Input)
			if e != nil {
				t.Fatal(e)
			}
			observation := tr.Normalize(execution.Input, parsed)
			record["request"], record["invocation"], record["execution"], record["observation"] = req, inv, execution, observation
			if git("status", "--porcelain") != "" || git("rev-parse", "HEAD") != target {
				t.Fatal("fixture changed during execution")
			}
			codes := map[string]bool{}
			for _, p := range observation.Problems {
				codes[p.Code] = true
			}
			if observation.Complete != sc.complete {
				t.Fatalf("complete=%v problems=%+v tests=%+v stderr=%s", observation.Complete, observation.Problems, observation.Tests, execution.Input.Stderr)
			}
			for _, code := range sc.problems {
				if !codes[code] {
					t.Fatalf("missing problem %s in %+v", code, observation.Problems)
				}
			}
			if !sc.complete {
				return
			}
			// Exact reconciliation: selected files, expected identities and
			// observed identities name the same tests.
			files, ids := map[string]bool{}, map[string]bool{}
			for _, x := range observation.Tests {
				files[x.File], ids[x.ID] = true, true
				if x.State != tr.Passed {
					t.Fatalf("state %+v", x)
				}
			}
			if len(files) != len(run) || len(ids) != len(expected) {
				t.Fatalf("observed files=%v ids=%v", files, ids)
			}
			for _, s := range run {
				if !files[filepath.Join(root, s)] {
					t.Fatalf("selected file %s not observed", s)
				}
			}
			for _, id := range expected {
				if !ids[id] {
					t.Fatalf("expected test %s not observed", id)
				}
			}
		})
	}
	results["limits"] = []string{"locally observed fixtures only", "dependency closure NOT_OBSERVED", "configured Mocha discovery remains UNKNOWN", "ESM, TypeScript loaders, parallel mode and watch mode NOT_RUN"}
	raw, e := json.MarshalIndent(results, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(output, "qualification.json"), append(raw, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
