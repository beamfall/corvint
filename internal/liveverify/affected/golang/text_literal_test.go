// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-2952 (V1-4283): a climbing token a non-test file holds only inside
// line-broken literals that reach only a standard output or error stream is
// text the program prints, not a path it opens. A line break alone proves
// nothing: formatted, buffered or stored text keeps rule (d), and the review's
// formatted reader (`strings.TrimSpace(fmt.Sprintf("../config%s\n", suffix))`)
// is selected by the file it reads. AFP-V0-2959: only a `package main` file's
// text is exempt; a library's printed text (`printedlib`) keeps rule (d),
// since an importer in the same process may capture the stream.
func TestLineBrokenClimbIsText_AFPV02952(t *testing.T) {
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
	exempt := []string{"printed", "streamed", "raw"}
	kept := []string{"printedlib", "prose", "template", "formatted", "buffered", "oneline", "path", "script", "both", "tested"}
	for _, name := range exempt {
		writeFiles(t, root, map[string]string{name + "/x_test.go": "package main\n"})
	}
	for _, name := range kept[:len(kept)-1] {
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
	for _, name := range exempt {
		if unit := units["go:example.test/m/"+name]; unit.UnboundedReads != "" || unit.LocatesRoot {
			t.Errorf("%s: a printed line-broken text literal made the package an unbounded reader: %+v", name, unit)
		}
	}
	for _, name := range kept {
		if unit := units["go:example.test/m/"+name]; unit.UnboundedReads == "" {
			t.Errorf("%s: a climbing literal with no output-only evidence lost rule (d): %+v", name, unit)
		}
	}
	// The exempt token stays a rule (c) path token, and the formatted
	// reader is selected by the file it reads.
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, map[string]string{"manifest.json": "{}\n"})
	found := map[string]string{}
	for _, item := range affected.Select(graph, []string{"manifest.json"}).Selected {
		found[item.UnitID] = item.Witness.Kind
	}
	if found["go:example.test/m/printed"] != affected.WitnessPathLiteralReader {
		t.Errorf("printed: the text token no longer names its file: %v", found)
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

// printedConfig is a command that prints the same literal and reads nothing.
const printedConfig = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Printf(\"../config%s\\n\", \".go\") }\n"

// capturedStreamFixture is the shared fixture of the AFP-V0-2960 tests: the
// review's pipe reader as a library (`piped`) and as a command
// (`pipedmain`), and commands whose own tests capture the stream through
// os.Pipe (`testcaptured`), an external test package (`xtestcaptured`), a
// re-run whose output is read back (`reexec`) or a descriptor duplicated
// onto standard output (`dupped`). `quiet` is the same command whose tests
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

// AFP-V0-2960 (V1-4283, review fix 2): an output proof exempts a printed
// literal only when nothing in its directory, test files included, can
// capture the stream. The review's pipe reader keeps rule (d) and is selected
// by an edit of the file it reads, as a library and as a command; a command
// whose tests capture its output through a pipe, an external test package, a
// re-run or a duplicated descriptor keeps rule (d) too. The same command
// whose tests capture nothing stays exempt.
func TestCapturedStreamKeepsClimb_AFPV02960(t *testing.T) {
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
	for _, name := range []string{"piped", "pipedmain", "testcaptured", "xtestcaptured", "reexec", "dupped"} {
		if unit := units["go:example.test/m/"+name]; unit.UnboundedReads == "" {
			t.Errorf("%s: a printed climbing literal whose stream the package can capture lost rule (d): %+v", name, unit)
		}
	}
	if unit := units["go:example.test/m/quiet"]; unit.UnboundedReads != "" {
		t.Errorf("quiet: a command whose package captures no stream lost its text exemption: %+v", unit)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]bool{}
	for _, item := range affected.Select(graph, []string{"config.go"}).Selected {
		reads[item.UnitID] = true
	}
	for _, name := range []string{"piped", "pipedmain", "testcaptured"} {
		if !reads["go:example.test/m/"+name] {
			t.Errorf("%s: an edit of the file the captured text names did not select its reader: %v", name, reads)
		}
	}
}
