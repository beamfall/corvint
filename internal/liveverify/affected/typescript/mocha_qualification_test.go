package typescript

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
)

// Opt in with independently selected local tools. This proves one local tuple,
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
	root := filepath.Join(output, "repository")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	write(t, root, "package.json", `{"devDependencies":{"mocha":"12.0.3"}}`)
	write(t, root, "src/value.cjs", "module.exports=1;\n")
	write(t, root, "tests/related.test.cjs", `const {it}=require("mocha");const value=require("../src/value.cjs");it("related",()=>{if(value!==2)throw Error("wrong value")});`)
	write(t, root, "tests/unrelated.test.cjs", `const {it}=require("mocha");it("unrelated",()=>{throw Error("must remain excluded")});`)
	git := func(args ...string) string {
		t.Helper()
		// GIT_CONFIG_GLOBAL hides the host's maintenance settings; a detached
		// `git maintenance` outliving a commit would race the evidence cleanup (V1-0662).
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
	base := git("rev-parse", "HEAD")
	write(t, root, "src/value.cjs", "module.exports=2;\n")
	git("add", "src/value.cjs")
	git("commit", "-m", "changed source")
	target := git("rev-parse", "HEAD")
	graph, e := affected.Build(root, New())
	if e != nil {
		t.Fatal(e)
	}
	plan := affected.Select(graph, strings.Fields(git("diff", "--name-only", base, target)))
	if plan.Scope != affected.ScopeBounded || len(plan.SelectedTests()) != 1 || plan.SelectedTests()[0] != "tests/related.test.cjs" || len(plan.Excluded) != 1 {
		t.Fatalf("selection %+v", plan)
	}
	launcher := filepath.Join(output, "mocha-launcher.cjs")
	modules := filepath.Dir(filepath.Dir(filepath.Dir(cli)))
	launch := fmt.Sprintf("#!%s\nprocess.env.NODE_PATH=%q;require('node:module').Module._initPaths();require(%q);\n", node, modules, cli)
	if e = os.WriteFile(launcher, []byte(launch), 0700); e != nil {
		t.Fatal(e)
	}
	digest := func(p string) string {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return tr.Digest(b)
	}
	inputs := map[string]string{}
	for _, f := range []string{"package.json", "src/value.cjs", "tests/related.test.cjs", "tests/unrelated.test.cjs"} {
		inputs[f] = digest(filepath.Join(root, f))
	}
	req := tr.Request{Runner: "mocha", Root: root, Executable: launcher, ExecutableSha256: digest(launcher), Selectors: plan.SelectedTests(), InputFiles: inputs, ExpectedTests: []string{filepath.Join(root, "tests/related.test.cjs") + "::related"}, Tools: map[string]tr.Tool{"node": {Executable: node, Sha256: digest(node)}}, ReportDir: filepath.Join(output, "reports"), TimeoutSeconds: 60}
	inv, e := dynamic.Build(req)
	if e != nil {
		t.Fatal(e)
	}
	execution, e := tr.Execute(t.Context(), req, inv)
	if e != nil {
		t.Fatal(e)
	}
	observation, e := dynamic.Parse(execution.Input)
	if e != nil {
		t.Fatal(e)
	}
	result := map[string]any{"base": base, "target": target, "plan": plan, "request": req, "invocation": inv, "execution": execution, "observation": observation, "mochaCliSha256": digest(cli), "limits": []string{"one locally observed no-config fixture only", "dependency closure NOT_OBSERVED", "configured Mocha discovery remains UNKNOWN"}}
	raw, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(output, "qualification.json"), append(raw, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	if !observation.Complete || len(observation.Tests) != 1 || observation.Tests[0].State != tr.Passed || observation.Tests[0].File != filepath.Join(root, "tests/related.test.cjs") {
		t.Fatalf("observation %+v", observation)
	}
	if git("status", "--porcelain") != "" || git("rev-parse", "HEAD") != target {
		t.Fatal("fixture changed during execution")
	}
}
