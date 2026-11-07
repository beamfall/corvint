// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// runnerTest is a test file of package name that runs the command at the
// import path with `go run`, trims what it prints and opens that path.
func runnerTest(name, command string) string {
	return "package " + name + "\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
		"func TestOpensPrinted(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"run\", \"" + command + "\").Output()\n" +
		"\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n\tif _, err := os.ReadFile(strings.TrimSpace(string(out))); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
}

// selectedKinds returns each selected unit's witness, by unit id, for one
// dirty path of a graph built over root.
func selectedKinds(t *testing.T, root, dirty string) map[string]affected.Witness {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]affected.Witness{}
	for _, item := range affected.Select(graph, []string{dirty}).Selected {
		found[item.UnitID] = item.Witness
	}
	return found
}

// TestRuleDReachesGoRunRunner_AFPV02990 is the review's case (V1-4983): the
// command `tool` prints "../config%s\n", a climbing literal that keeps rule
// (d) in its non-test file (AFP-V0-2962), and a test in a separate package
// runs it through exec.Command("go", "run", "example.test/m/tool"), trims the
// output and opens that path. An edit of config.go must select the runner as
// an unbounded reader whose cause is the command, not only the command.
func TestRuleDReachesGoRunRunner_AFPV02990(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                "module example.test/m\n",
		"config.go":             "package m\n",
		"tool/main.go":          "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Printf(\"../config%s\\n\", \".go\") }\n",
		"tool/main_test.go":     "package main\n",
		"runner/runner.go":      "package runner\n",
		"runner/runner_test.go": runnerTest("runner", "example.test/m/tool"),
	})
	found := selectedKinds(t, root, "config.go")
	if found["go:example.test/m/tool"].Kind == "" {
		t.Errorf("tool: the command was not selected by an edit of config.go: %v", found)
	}
	witness := found["go:example.test/m/runner"]
	if witness.Kind != affected.WitnessUnboundedReader || witness.Cause == nil || witness.Cause.Unit != "go:example.test/m/tool" {
		t.Errorf("runner: a test that runs the command and opens what it prints was not selected through the command's rule (d) read: %+v", witness)
	}
}

// TestRuleDReachesGetwdRunner_AFPV02990 is the round-3 friction case
// (V1-4983): `tool` calls os.Getwd and hands the root on, so it locates the
// root (AFP-V0-2950 keeps it), and `runner`'s test runs it with `go run`. An
// unrelated edit must select the runner as it selects the command.
func TestRuleDReachesGetwdRunner_AFPV02990(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"elsewhere/data.txt": "x\n",
		"tool/main.go":       "package main\n\nimport \"os\"\n\nfunc main() { cwd, _ := os.Getwd(); run(cwd) }\n\nfunc run(string) {}\n",
		"tool/main_test.go":  "package main\n",
		"runner/r_test.go":   runnerTest("runner", "example.test/m/tool"),
	})
	found := selectedKinds(t, root, "elsewhere/data.txt")
	if found["go:example.test/m/tool"].Kind != affected.WitnessUnboundedReader {
		t.Errorf("tool: the root-locating command was not selected on an unrelated change: %v", found)
	}
	witness := found["go:example.test/m/runner"]
	if witness.Kind != affected.WitnessUnboundedReader || witness.Cause == nil || witness.Cause.Unit != "go:example.test/m/tool" {
		t.Errorf("runner: a test that runs the root-locating command was not selected on an unrelated change: %+v", witness)
	}
}

// TestRuntimeUncertaintyFollowsExecutionEdges_AFPV02991 widens the two cases
// (AFP-V0-2991, AFP-V0-2992): the rule (d) closure follows an execution edge
// as it follows an import, transitively through a command or library whose
// non-test code runs the locator, and adds a unit whose tests alone run a
// reached command one level past it, as it adds a test-only importer. A test
// file's run edge makes no importer of its package a runner, and a command
// that locates nothing reaches its runners only when its build changes. A
// test-less command that reads an unowned path reaches the tests that run it
// (AFP-V0-495 traverses its runners as it traverses its importers).
func TestRuntimeUncertaintyFollowsExecutionEdges_AFPV02991(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"elsewhere/data.txt": "x\n",
		"data/report.txt":    "report\n",
		"tool/main.go":       "package main\n\nimport \"os\"\n\nfunc main() { cwd, _ := os.Getwd(); run(cwd) }\n\nfunc run(string) {}\n",
		"tool/main_test.go":  "package main\n",
		// A command whose own code runs the locator, and a test that runs it.
		"wrapper/main.go":      "package main\n\nimport \"os/exec\"\n\nfunc main() { _ = exec.Command(\"go\", \"run\", \"example.test/m/tool\").Run() }\n",
		"wrapper/main_test.go": "package main\n",
		"outer/o_test.go":      runnerTest("outer", "example.test/m/wrapper"),
		// A library whose code runs the locator, and an importer's test.
		"harness/h.go":       "package harness\n\nimport \"os/exec\"\n\nfunc Run() error { return exec.Command(\"go\", \"run\", \"example.test/m/tool\").Run() }\n",
		"harness/h_test.go":  "package harness\n",
		"consumer/c.go":      "package consumer\n\nimport _ \"example.test/m/harness\"\n",
		"consumer/c_test.go": "package consumer\n",
		// A package whose tests alone run the locator, and its importer.
		"testonly/t.go":      "package testonly\n",
		"testonly/t_test.go": runnerTest("testonly", "example.test/m/tool"),
		"beyond/b.go":        "package beyond\n\nimport _ \"example.test/m/testonly\"\n",
		"beyond/b_test.go":   "package beyond\n",
		// A command that locates nothing.
		"quiet/main.go":         "package main\n\nfunc main() {}\n",
		"quiet/main_test.go":    "package main\n",
		"quietrunner/q_test.go": runnerTest("quietrunner", "example.test/m/quiet"),
		// A test-less command that reads an unowned path, and its runner.
		"reporter/main.go":         "package main\n\nimport \"os\"\n\nfunc main() { b, _ := os.ReadFile(\"data/report.txt\"); os.Stdout.Write(b) }\n",
		"reporterrunner/r_test.go": runnerTest("reporterrunner", "example.test/m/reporter"),
	})
	found := selectedKinds(t, root, "elsewhere/data.txt")
	for _, id := range []string{"tool", "wrapper", "outer", "harness", "consumer", "testonly"} {
		witness := found["go:example.test/m/"+id]
		if witness.Kind != affected.WitnessUnboundedReader || witness.Cause == nil || witness.Cause.Unit != "go:example.test/m/tool" {
			t.Errorf("%s: a unit that runs the root locator's binary, directly or through code that runs it, was not selected on an unrelated change: %+v", id, witness)
		}
	}
	for _, id := range []string{"beyond", "quiet", "quietrunner", "reporter", "reporterrunner"} {
		if witness, ok := found["go:example.test/m/"+id]; ok {
			t.Errorf("%s: selected on an unrelated change as %+v; no code it runs locates the root", id, witness)
		}
	}
	if witness := selectedKinds(t, root, "quiet/main.go")["go:example.test/m/quietrunner"]; witness.Kind != affected.WitnessBinaryExec {
		t.Errorf("quietrunner: an edit of the command it runs did not select it as BINARY_EXEC: %+v", witness)
	}
	report := selectedKinds(t, root, "data/report.txt")
	if witness := report["go:example.test/m/reporterrunner"]; witness.Kind == "" {
		t.Errorf("reporterrunner: a test that runs the test-less reader of data/report.txt was not selected by its edit: %v", report)
	}
}

// TestExecsSplitCodeFromTestRuns_AFPV02991 pins the Go plugin's testOnlyExecs
// (AFP-V0-2991): a command named only in a test file is a test-only run, one
// named in a non-test file is not (even when a test also names it), and a
// declared edge (.corvint/test-binary-execs.json) is never test-only.
func TestExecsSplitCodeFromTestRuns_AFPV02991(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                          "module example.test/m\n",
		"tool/main.go":                    "package main\n\nfunc main() {}\n",
		"harness/h.go":                    "package harness\n\nimport \"os/exec\"\n\nfunc Run() error { return exec.Command(\"go\", \"run\", \"example.test/m/tool\").Run() }\n",
		"harness/h_test.go":               runnerTest("harness", "example.test/m/tool"),
		"testonly/t.go":                   "package testonly\n",
		"testonly/t_test.go":              runnerTest("testonly", "example.test/m/tool"),
		"declared/d.go":                   "package declared\n",
		"declared/d_test.go":              "package declared\n",
		".corvint/test-binary-execs.json": "{\"profile\":\"corvint-test-binary-execs/0\",\"packages\":{\"declared\":[\"tool\"]}}\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]affected.Unit{}
	for _, unit := range result.Units {
		units[strings.TrimPrefix(unit.ID, "go:example.test/m/")] = unit
	}
	const tool = "go:example.test/m/tool"
	for name, testOnly := range map[string]bool{"harness": false, "testonly": true, "declared": false} {
		unit := units[name]
		if !slices.Equal(unit.Execs, []string{tool}) {
			t.Errorf("%s: execs %v, want [%s]", name, unit.Execs, tool)
		}
		if got := slices.Equal(unit.TestOnlyExecs, []string{tool}); got != testOnly || !testOnly && len(unit.TestOnlyExecs) != 0 {
			t.Errorf("%s: testOnlyExecs %v, want test-only %v", name, unit.TestOnlyExecs, testOnly)
		}
	}
}

// TestBuildReachWalksCodeRunners_AFPV02993: a change to a library the command
// `tool` imports rebuilds tool. A unit whose code runs tool (`harness`) runs
// the changed binary, so its importer (`consumer`) and the runner of a
// command whose code runs tool (`outer`, through `wrapper`) are selected as
// an importer of a changed package is; a unit whose tests alone run tool
// (`testonly`) is selected one edge past it and its importer (`beyond`) is
// not (AFP-V0-037 keeps that edge terminal).
func TestBuildReachWalksCodeRunners_AFPV02993(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":               "module example.test/m\n",
		"lib/lib.go":           "package lib\n",
		"tool/main.go":         "package main\n\nimport _ \"example.test/m/lib\"\n\nfunc main() {}\n",
		"tool/main_test.go":    "package main\n",
		"wrapper/main.go":      "package main\n\nimport \"os/exec\"\n\nfunc main() { _ = exec.Command(\"go\", \"run\", \"example.test/m/tool\").Run() }\n",
		"wrapper/main_test.go": "package main\n",
		"outer/o_test.go":      runnerTest("outer", "example.test/m/wrapper"),
		"harness/h.go":         "package harness\n\nimport \"os/exec\"\n\nfunc Run() error { return exec.Command(\"go\", \"run\", \"example.test/m/tool\").Run() }\n",
		"harness/h_test.go":    "package harness\n",
		"consumer/c.go":        "package consumer\n\nimport _ \"example.test/m/harness\"\n",
		"consumer/c_test.go":   "package consumer\n",
		"testonly/t.go":        "package testonly\n",
		"testonly/t_test.go":   runnerTest("testonly", "example.test/m/tool"),
		"beyond/b.go":          "package beyond\n\nimport _ \"example.test/m/testonly\"\n",
		"beyond/b_test.go":     "package beyond\n",
	})
	found := selectedKinds(t, root, "lib/lib.go")
	for id, kind := range map[string]string{"tool": affected.WitnessDependency, "harness": affected.WitnessBinaryExec, "consumer": affected.WitnessDependency, "wrapper": affected.WitnessBinaryExec, "outer": affected.WitnessBinaryExec, "testonly": affected.WitnessBinaryExec} {
		if got := found["go:example.test/m/"+id].Kind; got != kind {
			t.Errorf("%s: selected as %q on a change tool's build reaches, want %s", id, got, kind)
		}
	}
	if witness, ok := found["go:example.test/m/beyond"]; ok {
		t.Errorf("beyond: selected as %+v; it imports a package whose tests alone run tool", witness)
	}
}

// TestTestlessReaderRunnersKeepReadScopes_AFPV02992: the runners of a
// test-less command that reads an unowned path are selected by its edit,
// except a runner whose declared read scope (AFP-V0-023) excludes the path:
// its tests read nothing there, through any binary they run (AFP-V0-161).
func TestTestlessReaderRunnersKeepReadScopes_AFPV02992(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                         "module example.test/m\n",
		"data/report.txt":                "report\n",
		"reporter/main.go":               "package main\n\nimport \"os\"\n\nfunc main() { b, _ := os.ReadFile(\"data/report.txt\"); os.Stdout.Write(b) }\n",
		"open/o_test.go":                 runnerTest("open", "example.test/m/reporter"),
		"scoped/s_test.go":               runnerTest("scoped", "example.test/m/reporter"),
		"wide/w_test.go":                 runnerTest("wide", "example.test/m/reporter"),
		".corvint/test-read-scopes.json": "{\"profile\":\"corvint-test-read-scopes/0\",\"packages\":{\"scoped\":[\"go.mod\"],\"wide\":[\"data/\"]}}\n",
	})
	found := selectedKinds(t, root, "data/report.txt")
	for _, id := range []string{"open", "wide"} {
		if found["go:example.test/m/"+id].Kind == "" {
			t.Errorf("%s: a runner of the test-less reader was not selected by an edit of what it reads: %v", id, found)
		}
	}
	if witness, ok := found["go:example.test/m/scoped"]; ok {
		t.Errorf("scoped: selected as %+v though its declared read scope excludes data/report.txt", witness)
	}
}
