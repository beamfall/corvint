// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-2952 (V1-4283): a climbing token a non-test file holds only inside
// line-broken literals that reach only a standard output or error stream is
// text the program prints, not a path it opens. A line break alone proves
// nothing: formatted, buffered or stored text keeps rule (d), and the review's
// formatted reader (`strings.TrimSpace(fmt.Sprintf("../config%s\n", suffix))`)
// is selected by the file it reads.
func TestLineBrokenClimbIsText_AFPV02952(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":           "module example.test/m\n",
		"printed/p.go":     "package printed\nimport \"fmt\"\nfunc F(d string) { fmt.Printf(\"| manifest.json | %s | See ../manifest.json for the digest. |\\n\\nMore text.\\n\", d) }\n",
		"streamed/s.go":    "package streamed\nimport (\n\t\"fmt\"\n\tsystem \"os\"\n)\nfunc F() { fmt.Fprintln(system.Stderr, \"See ../manifest.json\\nfor the digest.\") }\n",
		"raw/r.go":         "package raw\nimport \"os\"\nfunc F() { os.Stdout.WriteString(`| See ../manifest.json for the digest. |\n\nMore text.\n`) }\n",
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
	kept := []string{"prose", "template", "formatted", "buffered", "oneline", "path", "script", "both", "tested"}
	for _, name := range append(exempt[:len(exempt):len(exempt)], kept[:len(kept)-1]...) {
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
