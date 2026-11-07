// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-2962 (V1-4283, review fix 3): a climbing token keeps rule (d) in
// every literal, printed text included. The commands `printed`, `streamed`
// and `raw`, which the withdrawn AFP-V0-2952 exemption spared, keep rule (d)
// with the library copy and the formatted, buffered and stored forms; the
// review's formatted reader (`strings.TrimSpace(fmt.Sprintf("../config%s\n",
// suffix))`) is selected by the file it reads, and so is `printed`.
func TestPrintedClimbKeepsRuleD_AFPV02962(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":           "module example.test/m\n",
		"printed/p.go":     "package main\nimport \"fmt\"\nfunc main() { F(\"x\") }\nfunc F(d string) { fmt.Printf(\"| manifest.json | %s | See ../manifest.json for the digest. |\\n\\nMore text.\\n\", d) }\n",
		"streamed/s.go":    "package main\nimport (\n\t\"fmt\"\n\tsystem \"os\"\n)\nfunc main() { fmt.Fprintln(system.Stderr, \"See ../manifest.json\\nfor the digest.\") }\n",
		"raw/r.go":         "package main\nimport \"os\"\nfunc main() { os.Stdout.WriteString(`| See ../manifest.json for the digest. |\n\nMore text.\n`) }\n",
		"printedlib/p.go":  "package printedlib\nimport \"fmt\"\nfunc F(d string) { fmt.Printf(\"| manifest.json | %s | See ../manifest.json for the digest. |\\n\\nMore text.\\n\", d) }\n",
		"prose/p.go":       "package prose\nvar Doc = \"| manifest.json | %s | See ../manifest.json for the digest. |\\n\\nMore text.\\n\"\n",
		"template/t.go":    "package template\nimport \"fmt\"\nfunc Ruby(model string) []byte { return fmt.Appendf(nil, \"require_relative \\\"../models/%s\\\"\\n\\nclass X\\nend\\n\", model) }\n",
		"formatted/f.go":   "package formatted\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\nfunc F(suffix string) []byte { b, _ := os.ReadFile(strings.TrimSpace(fmt.Sprintf(\"../config%s\\n\", suffix))); return b }\n",
		"buffered/b.go":    "package buffered\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\nfunc F(name string) string { var b strings.Builder; fmt.Fprintf(&b, \"See ../manifest.json for %s\\n\", name); return b.String() }\n",
		"oneline/o.go":     "package oneline\nimport \"fmt\"\nfunc F() { fmt.Println(\"../manifest.json\") }\n",
		"path/p.go":        "package path\nvar Manifest = \"../manifest.json\"\n",
		"script/s.go":      "package script\nimport \"fmt\"\nfunc F() { fmt.Print(\"set -e\\ncd ../fixture\\ncat ../manifest.json\\n\") }\n",
		"both/b.go":        "package both\nimport \"fmt\"\nfunc F() { fmt.Print(\"See ../manifest.json\\n\") }\nvar Path = \"../manifest.json\"\n",
		"tested/t.go":      "package tested\n",
		"tested/t_test.go": "package tested\nimport \"fmt\"\nfunc init() { fmt.Print(\"See ../../manifest.json\\n\") }\n",
		"config.go":        "package config\n",
	})
	commands := []string{"printed", "streamed", "raw"}
	kept := []string{"printed", "streamed", "raw", "printedlib", "prose", "template", "formatted", "buffered", "oneline", "path", "script", "both", "tested"}
	for _, name := range commands {
		writeFiles(t, root, map[string]string{name + "/x_test.go": "package main\n"})
	}
	for _, name := range kept[len(commands) : len(kept)-1] {
		writeFiles(t, root, map[string]string{name + "/x_test.go": "package " + name + "\n"})
	}
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]affected.Unit{}
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	for _, name := range kept {
		if unit := units["go:example.test/m/"+name]; unit.UnboundedReads == "" {
			t.Errorf("%s: a climbing literal lost rule (d): %+v", name, unit)
		}
	}
	for _, name := range commands {
		if unit := units["go:example.test/m/"+name]; !unit.LocatesRoot {
			t.Errorf("%s: a command's printed climbing literal no longer locates the root: %+v", name, unit)
		}
	}
	// The printing command and the formatted reader are selected by the
	// files their text names.
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{"manifest.json": "{}\n"})
	found := map[string]string{}
	for _, item := range affected.Select(graph, []string{"manifest.json"}).Selected {
		found[item.UnitID] = item.Witness.Kind
	}
	if found["go:example.test/m/printed"] == "" {
		t.Errorf("printed: an edit of the file its printed text names did not select it: %v", found)
	}
	reads := map[string]bool{}
	for _, item := range affected.Select(graph, []string{"config.go"}).Selected {
		reads[item.UnitID] = true
	}
	if !reads["go:example.test/m/formatted"] {
		t.Errorf("formatted: an edit of the file a formatted literal reads did not select its reader: %v", reads)
	}
}

// pipeReader is the review's case (V1-4283, review fix 2): code that
// redirects os.Stdout to an os.Pipe, prints a climbing line-broken literal
// straight to the stream, reads the pipe back, trims it and opens the result.
// With suffix ".go" it reads ../config.go.
const pipeReader = `
import (
	"fmt"
	"io"
	"os"
	"strings"
)

func Read(suffix string) []byte {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	fmt.Printf("../config%s\n", suffix)
	os.Stdout = stdout
	w.Close()
	printed, _ := io.ReadAll(r)
	b, _ := os.ReadFile(strings.TrimSpace(string(printed)))
	return b
}
`

// printedConfig is a command that prints the same literal and opens nothing
// itself.
const printedConfig = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Printf(\"../config%s\\n\", \".go\") }\n"

// capturedStreamFixture is the second review's fixture (V1-4283, review fix
// 2): the pipe reader as a library (`piped`) and as a command (`pipedmain`),
// and commands whose own tests capture the stream through os.Pipe
// (`testcaptured`), an external test package (`xtestcaptured`), a re-run
// whose output is read back (`reexec`) or a descriptor duplicated onto
// standard output (`dupped`). `quiet` is the same command whose tests
// capture nothing.
func capturedStreamFixture() map[string]string {
	captureTest := func(pkg string) string {
		return "package " + pkg + "\n" + `
import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestReads(t *testing.T) {
	r, w, _ := os.Pipe()
	os.Stdout = w
	main()
	w.Close()
	printed, _ := io.ReadAll(r)
	os.ReadFile(strings.TrimSpace(string(printed)))
}
`
	}
	return map[string]string{
		"config.go":                    "package config\n",
		"piped/p.go":                   "package piped\n" + pipeReader,
		"piped/p_test.go":              "package piped\n",
		"pipedmain/main.go":            "package main\n" + pipeReader + "\nfunc main() { Read(\".go\") }\n",
		"pipedmain/main_test.go":       "package main\n",
		"testcaptured/main.go":         printedConfig,
		"testcaptured/main_test.go":    captureTest("main"),
		"xtestcaptured/main.go":        printedConfig,
		"xtestcaptured/main_x_test.go": strings.Replace(captureTest("main_test"), "\tmain()\n", "", 1),
		"reexec/main.go":               printedConfig,
		"reexec/main_test.go":          "package main\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestReads(t *testing.T) {\n\tout, _ := exec.Command(os.Args[0]).Output()\n\tos.ReadFile(strings.TrimSpace(string(out)))\n}\n",
		"dupped/main.go":               printedConfig,
		"dupped/main_test.go":          "package main\n\nimport (\n\t\"os\"\n\t\"syscall\"\n\t\"testing\"\n)\n\nfunc TestReads(t *testing.T) {\n\tf, _ := os.Create(\"out\")\n\tsyscall.Dup2(int(f.Fd()), 1)\n\tmain()\n}\n",
		"quiet/main.go":                printedConfig,
		"quiet/main_test.go":           "package main\n",
	}
}

// AFP-V0-2962 (V1-4283, review fixes 2 and 3): the second review's pipe
// readers keep rule (d) and are selected by an edit of the file they read, as
// a library and as a command, as do commands whose tests capture their output
// through a pipe, an external test package, a re-run or a duplicated
// descriptor. `quiet`, whose tests capture nothing, keeps rule (d) too: a
// process outside its directory can read its output back.
func TestCapturedStreamKeepsClimb_AFPV02962(t *testing.T) {
	root := t.TempDir()
	files := capturedStreamFixture()
	files["go.mod"] = "module example.test/m\n"
	writeFiles(t, root, files)
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]affected.Unit{}
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	for _, name := range []string{"piped", "pipedmain", "testcaptured", "xtestcaptured", "reexec", "dupped", "quiet"} {
		if unit := units["go:example.test/m/"+name]; unit.UnboundedReads == "" {
			t.Errorf("%s: a printed climbing literal lost rule (d): %+v", name, unit)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]bool{}
	for _, item := range affected.Select(graph, []string{"config.go"}).Selected {
		reads[item.UnitID] = true
	}
	for _, name := range []string{"piped", "pipedmain", "testcaptured", "quiet"} {
		if !reads["go:example.test/m/"+name] {
			t.Errorf("%s: an edit of the file the captured text names did not select its reader: %v", name, reads)
		}
	}
}

// importedCapture is the third review's case (V1-4283, review fix 3): a
// helper package that redirects os.Stdout to a pipe, runs a callback, trims
// what the callback printed and opens it. It has capture evidence and no
// climbing literal.
const importedCapture = `package capture

import (
	"io"
	"os"
	"strings"
)

// Read runs f with os.Stdout redirected and opens the path f printed.
func Read(f func()) []byte {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	f()
	os.Stdout = stdout
	w.Close()
	printed, _ := io.ReadAll(r)
	b, _ := os.ReadFile(strings.TrimSpace(string(printed)))
	return b
}
`

// importedCaptureFixture is the review's exact example: the command `tool`
// prints "../config%s\n" and its test calls capture.Read(main), so the test
// process opens ../config.go, the repository's config.go.
func importedCaptureFixture(module string) map[string]string {
	return map[string]string{
		"config.go":               "package config\n",
		"capture/capture.go":      importedCapture,
		"capture/capture_test.go": "package capture\n",
		"tool/main.go":            printedConfig,
		"tool/main_test.go":       "package main\n\nimport (\n\t\"testing\"\n\n\t\"" + module + "/capture\"\n)\n\nfunc TestReads(t *testing.T) { capture.Read(main) }\n",
	}
}

// AFP-V0-2962 (V1-4283, review fix 3): a climbing token in printed text keeps
// rule (d). The review's imported helper captures the command's stream in the
// command's own test process; the command keeps rule (d) and an edit of
// config.go selects it.
func TestImportedCaptureKeepsClimb_AFPV02962(t *testing.T) {
	root := t.TempDir()
	files := importedCaptureFixture("example.test/m")
	files["go.mod"] = "module example.test/m\n"
	writeFiles(t, root, files)
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]affected.Unit{}
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	if unit := units["go:example.test/m/tool"]; unit.UnboundedReads == "" || !unit.LocatesRoot {
		t.Errorf("tool: a printed climbing literal an imported helper reads back lost rule (d): %+v", unit)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]bool{}
	for _, item := range affected.Select(graph, []string{"config.go"}).Selected {
		reads[item.UnitID] = true
	}
	if !reads["go:example.test/m/tool"] {
		t.Errorf("tool: an edit of the file its captured text names did not select it: %v", reads)
	}
}

// AFP-V0-2962 (V1-4283, review fix 3): printed text also leaves the process.
// A make target that runs a command and opens the path it printed reads that
// file, though no package of the command's process captures the stream and
// no Go package imports the target. The command keeps rule (d), so the target
// is selected on an unrelated change, as every runner of a root locator is
// (AFP-V0-2951). A proof over the command's import closure cannot see this
// consumer, which is why the exemption is withdrawn, not narrowed.
func TestPrintedTextReachesItsRunner_AFPV02962(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"config.go":          "package config\n",
		"elsewhere/data.txt": "x\n",
		"quiet/main.go":      printedConfig,
		"quiet/main_test.go": "package main\n",
		"Makefile":           "read-printed:\n\tcd quiet && cat \"$$(go run .)\"\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"config.go", "elsewhere/data.txt"} {
		found := map[string]string{}
		for _, item := range affected.Select(graph, []string{dirty}).Selected {
			found[item.UnitID] = item.Witness.Kind
		}
		for _, id := range []string{"go:example.test/m/quiet", "driver:make:Makefile#read-printed"} {
			if found[id] != affected.WitnessUnboundedReader {
				t.Errorf("%s: %s: a runner of a command whose printed text it opens was not selected as an unbounded reader: %v", dirty, id, found)
			}
		}
	}
}
