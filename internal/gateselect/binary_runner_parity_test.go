// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"slices"
	"strings"
	"testing"
)

// runnerTest is a test file of package name that runs the command at the
// import path with `go run`, trims what it prints and opens that path.
func runnerTest(name, command string) string {
	return "package " + name + "\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
		"func TestOpensPrinted(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"run\", \"" + command + "\").Output()\n" +
		"\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := os.ReadFile(strings.TrimSpace(string(out))); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
}

// selects reports whether the CI selector's verdict for one dirty path runs
// the fixture package in directory.
func selects(t *testing.T, root, dirty, directory string) bool {
	t.Helper()
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{dirty}
	got := verdict(plan, root)
	return strings.Contains(got+" ", " "+fixtureModule+"/"+directory+" ")
}

// TestGoRunRunnerParity_AFPV02990 holds this index's rule (d) and the
// planner's equal on the review's case (V1-4983): `tool` prints
// "../config%s\n" and a separate package's test runs it through
// exec.Command("go", "run", ".../tool"), trims the output and opens that
// path. The runner is a rule (d) unit with the command, cold and warm, and an
// edit of config.go selects it.
func TestGoRunRunnerParity_AFPV02990(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"config.go":             "package fixture\n",
		"tool/main.go":          "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Printf(\"../config%s\\n\", \".go\") }\n",
		"tool/main_test.go":     "package main\n",
		"runner/runner.go":      "package runner\n",
		"runner/runner_test.go": runnerTest("runner", fixtureModule+"/tool"),
	})
	agree(t, root, []string{"runner", "tool"})
	if !selects(t, root, "config.go", "runner") {
		t.Errorf("an edit of config.go did not select runner, whose test runs tool and opens what it prints")
	}
}

// TestGetwdRunnerParity_AFPV02990 is the round-3 friction case (V1-4983):
// `tool` calls os.Getwd and hands the root on, and `runner`'s test runs it
// with `go run`. Both analyzers make the runner a rule (d) unit, and an
// unrelated edit selects it.
func TestGetwdRunnerParity_AFPV02990(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":             "module example.com/fixture\n\ngo 1.22\n",
		"elsewhere/data.txt": "x\n",
		"tool/main.go":       "package main\n\nimport \"os\"\n\nfunc main() { cwd, _ := os.Getwd(); run(cwd) }\n\nfunc run(string) {}\n",
		"tool/main_test.go":  "package main\n",
		"runner/r_test.go":   runnerTest("runner", fixtureModule+"/tool"),
	})
	agree(t, root, []string{"runner", "tool"})
	if !selects(t, root, "elsewhere/data.txt", "runner") {
		t.Errorf("an unrelated edit did not select runner, whose test runs the root-locating tool")
	}
}

// TestExecutionEdgeParity_AFPV02992 holds the two analyzers' rule (d) equal
// over execution edges (AFP-V0-2991, AFP-V0-2992): a command (`wrapper`) or
// library (`harness`) whose non-test code runs the locator is followed like
// an importer, so its runner (`outer`) and importer (`consumer`) are reached;
// a package whose tests alone run it (`testonly`, `climber` by a climbing
// `go build ../../tool`) is added one level past it and its importer
// (`beyond`) is not; a declared runner (`declared`, .corvint/test-binary-execs.json)
// is a runner too; a command that locates nothing (`quiet`) reaches no runner.
func TestExecutionEdgeParity_AFPV02992(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":                          "module example.com/fixture\n\ngo 1.22\n",
		"elsewhere/data.txt":              "x\n",
		"tool/main.go":                    "package main\n\nimport \"os\"\n\nfunc main() { cwd, _ := os.Getwd(); run(cwd) }\n\nfunc run(string) {}\n",
		"tool/main_test.go":               "package main\n",
		"wrapper/main.go":                 "package main\n\nimport \"os/exec\"\n\nfunc main() { _ = exec.Command(\"go\", \"run\", \"example.com/fixture/tool\").Run() }\n",
		"wrapper/main_test.go":            "package main\n",
		"outer/o_test.go":                 runnerTest("outer", fixtureModule+"/wrapper"),
		"harness/h.go":                    "package harness\n\nimport \"os/exec\"\n\nfunc Run() error { return exec.Command(\"go\", \"run\", \"example.com/fixture/tool\").Run() }\n",
		"harness/h_test.go":               "package harness\n",
		"consumer/c.go":                   "package consumer\n\nimport _ \"example.com/fixture/harness\"\n",
		"consumer/c_test.go":              "package consumer\n",
		"testonly/t.go":                   "package testonly\n",
		"testonly/t_test.go":              runnerTest("testonly", fixtureModule+"/tool"),
		"beyond/b.go":                     "package beyond\n\nimport _ \"example.com/fixture/testonly\"\n",
		"beyond/b_test.go":                "package beyond\n",
		"deep/sub/climber_test.go":        "package sub\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestBuild(t *testing.T) { _ = exec.Command(\"go\", \"build\", \"../../tool\").Run() }\n",
		"declared/d.go":                   "package declared\n",
		"declared/d_test.go":              "package declared\n\nimport (\n\t\"flag\"\n\t\"os/exec\"\n\t\"testing\"\n)\n\nvar binary = flag.String(\"binary\", \"\", \"\")\n\nfunc TestRuns(t *testing.T) { _ = exec.Command(*binary).Run() }\n",
		".corvint/test-binary-execs.json": "{\"profile\":\"corvint-test-binary-execs/0\",\"packages\":{\"declared\":[\"tool\"]}}\n",
		"quiet/main.go":                   "package main\n\nfunc main() {}\n",
		"quiet/main_test.go":              "package main\n",
		"quietrunner/q_test.go":           runnerTest("quietrunner", fixtureModule+"/quiet"),
	})
	agree(t, root, []string{"consumer", "declared", "deep/sub", "harness", "outer", "testonly", "tool", "wrapper"})
	for _, directory := range []string{"outer", "consumer", "testonly", "deep/sub", "declared"} {
		if !selects(t, root, "elsewhere/data.txt", directory) {
			t.Errorf("%s: an unrelated edit did not select a package that runs the root locator's binary", directory)
		}
	}
	for _, directory := range []string{"beyond", "quietrunner"} {
		if selects(t, root, "elsewhere/data.txt", directory) {
			t.Errorf("%s: an unrelated edit selected a package that runs no root locator", directory)
		}
	}
}

// TestBinaryExecsDeclarationWidens_AFPV02994: a binary-execs declaration the
// selector cannot apply (a member naming no command, a malformed body) makes
// every package a rule (d) unit, since the selector cannot tell which package
// runs which command (AFP-V0-2994); the planner raises the module-level
// go:test-binary-execs-invalid frontier for the same declaration, which keeps
// the CI tier on its full run.
func TestBinaryExecsDeclarationWidens_AFPV02994(t *testing.T) {
	for name, body := range map[string]string{
		"not a command": "{\"profile\":\"corvint-test-binary-execs/0\",\"packages\":{\"runner\":[\"lib\"]}}\n",
		"malformed":     "{\"profile\":\"corvint-test-binary-execs/0\",\"packages\":{\"runner\":[]}}\n",
		"other profile": "{\"profile\":\"corvint-test-binary-execs/1\",\"packages\":{}}\n",
	} {
		root := writeFixture(t, map[string]string{
			"go.mod":                          "module example.com/fixture\n\ngo 1.22\n",
			"lib/lib.go":                      "package lib\n",
			"lib/lib_test.go":                 "package lib\n",
			"runner/runner.go":                "package runner\n",
			"runner/runner_test.go":           "package runner\n",
			".corvint/test-binary-execs.json": body,
		})
		index, err := indexRepository(root, fixtureModule)
		if err != nil {
			t.Fatal(err)
		}
		if got := sortedKeys(index.unresolved()); !slices.Equal(got, []string{"lib", "runner"}) {
			t.Errorf("%s: rule (d) = %v, want every package", name, got)
		}
		var plan Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Plan.Dirty = []string{"lib/lib.go"}
		if got := verdict(plan, root); !strings.Contains(got+" ", " example.com/fixture/runner ") {
			t.Errorf("%s: an edit of lib did not select runner under an unusable declaration: %q", name, got)
		}
	}
}

// TestRunnerEdgesNeverNarrowerThanThePlanner_AFPV02994: where the selector
// cannot resolve a literal as the plugin does it widens. A test that names
// `../tool` from deep/sub resolves, for the plugin, to deep/tool, which is
// no command, so the planner records no edge; the selector also tries the
// literal's component run, `tool`, and keeps the package as a rule (d) unit.
// Its set holds the planner's and more, never less.
func TestRunnerEdgesNeverNarrowerThanThePlanner_AFPV02994(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"tool/main.go":          "package main\n\nimport \"os\"\n\nfunc main() { cwd, _ := os.Getwd(); run(cwd) }\n\nfunc run(string) {}\n",
		"tool/main_test.go":     "package main\n",
		"deep/sub/near_test.go": "package sub\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestBuild(t *testing.T) { _ = exec.Command(\"go\", \"build\", \"../tool\").Run() }\n",
		"runner/r_test.go":      runnerTest("runner", fixtureModule+"/tool"),
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	selector := sortedKeys(index.unresolved())
	planner := plannerUnbounded(t, root)
	for _, directory := range planner {
		if !slices.Contains(selector, directory) {
			t.Errorf("planner rule (d) unit %s missing from the selector's %v", directory, selector)
		}
	}
	if !slices.Contains(selector, "deep/sub") || slices.Contains(planner, "deep/sub") {
		t.Errorf("selector %v, planner %v: want deep/sub in the selector's set only", selector, planner)
	}
}
