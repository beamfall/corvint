// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-9930: comparisons do not consume repository contents; a real read
// in the same binding, a returned path and a dependent retain rule (d).
func TestLocatorComparisonSelection_AZU_V0_9930(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"guard/g.go":         "package guard\nimport system \"os\"\nfunc F() { cwd, err := system.Getwd(); if err != nil || cwd != \"/product\" { return } }\n",
		"reader/r.go":        "package reader\nimport \"os\"\nfunc F() { cwd, _ := os.Getwd(); if cwd != \"/product\" { return }; os.ReadDir(cwd) }\n",
		"escape/e.go":        "package escape\nimport \"os\"\nfunc F() string { cwd, _ := os.Getwd(); return cwd }\n",
		"dependent/d.go":     "package dependent\nimport _ \"example.test/m/reader\"\n",
		"elsewhere/data.txt": "x\n",
	})
	for _, name := range []string{"guard", "reader", "escape", "dependent"} {
		writeFiles(t, root, map[string]string{name + "/x_test.go": "package " + name + "\n"})
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"elsewhere/data.txt"})
	found := map[string]bool{}
	for _, selected := range plan.Selected {
		found[selected.UnitID] = true
	}
	if found["go:example.test/m/guard"] {
		t.Fatal("comparison-only guard selected for unrelated data")
	}
	for _, name := range []string{"reader", "escape", "dependent"} {
		if !found["go:example.test/m/"+name] {
			t.Errorf("real reader %s dropped", name)
		}
	}
}

// AFP-V0-2950 (V1-4283): locating the working directory is not a read. A
// Getwd result joined with literal plain components, into a path only a read
// at or below it consumes, is bounded by those literals, which rule (c)
// already names, so the package is selected by the file it names and by
// nothing else; every escaping use of the root or of the joined path keeps
// rule (d).
func TestBoundedJoinLocatorIsNoRead_AFPV02950(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"docs/guide.md":      "guide\n",
		"docs/other.md":      "other\n",
		"elsewhere/data.txt": "x\n",
		"joiner/j.go":        "package joiner\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() []byte { root, err := os.Getwd(); if err != nil { return nil }; b, _ := os.ReadFile(filepath.Join(root, \"docs\", \"guide.md\")); return b }\n",
		"lister/l.go":        "package lister\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() int { root, err := os.Getwd(); if err != nil { return 0 }; entries, _ := os.ReadDir(filepath.Join(root, \"docs\")); return len(entries) }\n",
		"aliased/a.go":       "package aliased\nimport (\n\tsystem \"os\"\n\tfp \"path/filepath\"\n)\nfunc F() bool { var cwd string; var err error; cwd, err = system.Getwd(); if err != nil { return false }; _, err = system.Stat(fp.Join(cwd, \"docs/guide.md\")); return err == nil }\n",
		"rebound/r.go":       "package rebound\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F(given string) []byte { root := given; if root == \"\" { var err error; root, err = os.Getwd(); if err != nil { return nil } }; b, _ := os.ReadFile(filepath.Join(root, \"docs\", \"guide.md\")); return b }\n",
		"bound/b.go":         "package bound\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() int { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); if docs == \"/\" { return 0 }; var guide = filepath.Join(cwd, \"docs\", \"guide.md\"); os.Stat(guide); entries, _ := os.ReadDir(docs); return len(entries) }\n",
		"widened/w.go":       "package widened\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() []byte { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); b, _ := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(docs)), \"config.go\")); return b }\n",
		"joinreturn/j.go":    "package joinreturn\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() string { cwd, _ := os.Getwd(); return filepath.Join(cwd, \"docs\") }\n",
		"opener/o.go":        "package opener\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() string { cwd, _ := os.Getwd(); f, _ := os.Open(filepath.Join(cwd, \"docs\")); return filepath.Dir(f.Name()) }\n",
		"rejoined/r.go":      "package rejoined\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() int { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); docs = filepath.Dir(docs); entries, _ := os.ReadDir(docs); return len(entries) }\n",
		"handed/h.go":        "package handed\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc G(string) {}\nfunc F() { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); G(docs) }\n",
		"returner/r.go":      "package returner\nimport \"os\"\nfunc F() string { cwd, _ := os.Getwd(); return cwd }\n",
		"copier/c.go":        "package copier\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() string { cwd, _ := os.Getwd(); other := cwd; return filepath.Join(other, \"docs\") }\n",
		"climber/c.go":       "package climber\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() string { cwd, _ := os.Getwd(); return filepath.Join(cwd, \"..\", \"docs\") }\n",
		"parent/p.go":        "package parent\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() string { cwd, _ := os.Getwd(); return filepath.Dir(cwd) }\n",
		"computed/c.go":      "package computed\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F(name string) string { cwd, _ := os.Getwd(); return filepath.Join(cwd, name) }\n",
		"second/s.go":        "package second\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() string { cwd, _ := os.Getwd(); return filepath.Join(\"prefix\", cwd, \"docs\") }\n",
		"dotted/d.go":        "package dotted\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() string { cwd, _ := os.Getwd(); return filepath.Join(cwd, \".\") }\n",
		"global/g.go":        "package global\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nvar root string\nfunc F() string { var err error; root, err = os.Getwd(); if err != nil { return \"\" }; return filepath.Join(root, \"docs\") }\n",
		"passer/p.go":        "package passer\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc G(s string) string { return s }\nfunc F() string { cwd, _ := os.Getwd(); return filepath.Join(G(cwd), \"docs\") }\n",
		"dependent/d.go":     "package dependent\nimport _ \"example.test/m/returner\"\n",
		"joinerdep/d.go":     "package joinerdep\nimport _ \"example.test/m/joiner\"\n",
	})
	bounded := []string{"joiner", "aliased", "rebound", "lister", "bound"}
	// widened is the review's case: a bounded join climbed with filepath.Dir
	// reads a file the join does not name; joinreturn, opener, rejoined and
	// handed hand the joined path to a consumer that could widen it.
	escaping := []string{"returner", "copier", "climber", "parent", "computed", "second", "dotted", "global", "passer", "dependent", "widened", "joinreturn", "opener", "rejoined", "handed"}
	for _, name := range append(append([]string{"joinerdep"}, bounded...), escaping...) {
		writeFiles(t, root, map[string]string{name + "/x_test.go": "package " + name + "\n"})
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	selected := func(dirty string) map[string]bool {
		found := map[string]bool{}
		for _, item := range affected.Select(graph, []string{dirty}).Selected {
			found[item.UnitID] = true
		}
		return found
	}
	unrelated := selected("elsewhere/data.txt")
	for _, name := range append([]string{"joinerdep"}, bounded...) {
		if unrelated["go:example.test/m/"+name] {
			t.Errorf("%s: a literal join of the located root was selected for unrelated data", name)
		}
	}
	for _, name := range escaping {
		if !unrelated["go:example.test/m/"+name] {
			t.Errorf("%s: an escaping root locator was dropped", name)
		}
	}
	named := selected("docs/guide.md")
	for _, name := range bounded {
		if !named["go:example.test/m/"+name] {
			t.Errorf("%s: the file the join names did not select its reader", name)
		}
	}
	sibling := selected("docs/other.md")
	if sibling["go:example.test/m/joiner"] {
		t.Errorf("joiner: a sibling of the joined file selected its reader")
	}
	if !sibling["go:example.test/m/lister"] {
		// The join names the directory, so anything at or below it is read.
		t.Errorf("lister: a change below the joined directory did not select the reader")
	}
	// The file the widened read reaches selects its reader.
	if !selected("config.go")["go:example.test/m/widened"] {
		t.Errorf("widened: the file a climbed join reads did not select its reader")
	}
}

// AFP-V0-2951 (V1-4283): a command's non-test root locator keeps the
// command's own reason and selection but reaches no runner: nothing compiles
// a main package, and a runner's build or run of it is its own recipe. A
// library locator still reaches its importer.
func TestCommandLocatorReachesNoRunner_AFPV02951(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"elsewhere/data.txt": "x\n",
		"cmd/tool/main.go":   "package main\nimport \"os\"\nfunc main() { cwd, _ := os.Getwd(); run(cwd) }\nfunc run(string) {}\n",
		"cmd/tool/x_test.go": "package main\n",
		"lib/lib.go":         "package lib\nimport \"os\"\nfunc Root() string { cwd, _ := os.Getwd(); return cwd }\n",
		"lib/x_test.go":      "package lib\n",
		"user/u.go":          "package user\nimport _ \"example.test/m/lib\"\n",
		"user/x_test.go":     "package user\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]affected.Unit{}
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	tool, lib := units["go:example.test/m/cmd/tool"], units["go:example.test/m/lib"]
	if tool.UnboundedReads == "" || tool.LocatesRoot {
		t.Errorf("command: want its own unbounded reason without locatesRoot, got %+v", tool)
	}
	if lib.UnboundedReads == "" || !lib.LocatesRoot {
		t.Errorf("library: want an unbounded reason with locatesRoot, got %+v", lib)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), runner{command: "go:example.test/m/cmd/tool"})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, item := range affected.Select(graph, []string{"elsewhere/data.txt"}).Selected {
		found[item.UnitID] = item.Witness.Kind
	}
	if found["go:example.test/m/cmd/tool"] != affected.WitnessUnboundedReader {
		t.Errorf("command not selected on an unrelated change: %v", found)
	}
	if kind, ok := found["driver:make:Makefile#build-tool"]; ok {
		t.Errorf("the command's runner was selected as %s on an unrelated change", kind)
	}
	if found["go:example.test/m/user"] != affected.WitnessUnboundedReader {
		t.Errorf("the library locator's importer was dropped: %v", found)
	}
}

// runner is a one-unit plugin standing for a make target that builds and runs
// a Go command, with the import edge the driver plugin gives such a recipe.
type runner struct{ command string }

func (runner) Name() string     { return "driver" }
func (runner) Owns(string) bool { return false }
func (r runner) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{Units: []affected.Unit{{ID: "driver:make:Makefile#build-tool", Sources: []string{}, Tests: []string{}, Imports: []string{r.command}}}}, nil
}
