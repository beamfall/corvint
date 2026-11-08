// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"os"
	"path/filepath"
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
		"rebound/r.go":       "package rebound\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F(given string) []byte { root := \"\"; if root == \"\" { var err error; root, err = os.Getwd(); if err != nil { return nil } }; b, _ := os.ReadFile(filepath.Join(root, \"docs\", \"guide.md\")); return b }\n",
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

// AFP-V0-2951 (V1-4283, review fix): a command's non-test root locator
// reaches every unit that runs the command, and a rule (d) chain skips only
// the build-only edges of `go vet` and `go list`, which run no code and leave
// no binary. The regression is run-tool: the command's runtime read of the
// located root is the only path from the changed file to the target that runs
// it. `go build` is no build-only edge: the target that depends on it runs the
// binary.
func TestCommandLocatorReachesItsRunners_AFPV02951(t *testing.T) {
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
		"Makefile": "run-tool:\n\tgo run ./cmd/tool\n" +
			"build-tool:\n\tgo build -o bin/tool ./cmd/tool\n" +
			"check: build-tool\n\tbin/tool check\n" +
			"vet-and-run:\n\tgo vet ./cmd/tool\n\tgo run ./cmd/tool\n" +
			"vet-tool:\n\tgo vet ./cmd/tool\n" +
			"vet-all:\n\tgo vet ./...\n" +
			"list-all:\n\tgo list ./...\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]affected.Unit{}
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	for _, id := range []string{"go:example.test/m/cmd/tool", "go:example.test/m/lib"} {
		if unit := units[id]; unit.UnboundedReads == "" || !unit.LocatesRoot {
			t.Errorf("%s: want an unbounded reason with locatesRoot, got %+v", id, unit)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	selected := func(dirty string) map[string]string {
		found := map[string]string{}
		for _, item := range affected.Select(graph, []string{dirty}).Selected {
			found[item.UnitID] = runnerWitness(item.Witness)
		}
		return found
	}
	found := selected("elsewhere/data.txt")
	for _, id := range []string{"go:example.test/m/cmd/tool", "go:example.test/m/user", "driver:make:Makefile#run-tool", "driver:make:Makefile#build-tool", "driver:make:Makefile#check", "driver:make:Makefile#vet-and-run"} {
		if found[id] != affected.WitnessUnboundedReader {
			t.Errorf("%s: a unit that runs a root locator was not selected on an unrelated change: %v", id, found)
		}
	}
	for _, id := range []string{"driver:make:Makefile#vet-tool", "driver:make:Makefile#vet-all", "driver:make:Makefile#list-all"} {
		if kind, ok := found[id]; ok {
			t.Errorf("%s: a build-only runner was selected as %s on an unrelated change", id, kind)
		}
	}
	// A build-only edge is still a dependency edge: an edit of the code it
	// vets selects it.
	if _, ok := selected("cmd/tool/main.go")["driver:make:Makefile#vet-tool"]; !ok {
		t.Errorf("vet-tool: an edit of the package it vets did not select it")
	}
}

// AFP-V0-2956 and AFP-V0-2957 (V1-4283, review fix 2): the review's case on a
// real Makefile. `go list ./lib | xargs -I PKG go test PKG` runs the listed
// package's tests, so lib's non-test root locator reaches the target on an
// unrelated change, as it does through a substitution and through a vet run
// whose output a shell executes. A `go list` whose output feeds no command
// keeps its build-only edge.
func TestListingOutputReachesItsRunner_AFPV02956(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"elsewhere/data.txt": "x\n",
		"lib/lib.go":         "package lib\nimport \"os\"\nfunc Root() string { cwd, _ := os.Getwd(); return cwd }\n",
		"lib/x_test.go":      "package lib\n",
		"Makefile": "list-run:\n\tgo list ./lib | xargs -I PKG go test PKG\n" +
			"list-subst:\n\tgo test `go list ./lib`\n" +
			"list-loop:\n\tgo list ./lib | while read -r p; do go test \"$$p\"; done\n" +
			"list-only:\n\tgo list ./lib > pkgs.txt\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, item := range affected.Select(graph, []string{"elsewhere/data.txt"}).Selected {
		found[item.UnitID] = item.Witness.Kind
	}
	for _, id := range []string{"driver:make:Makefile#list-run", "driver:make:Makefile#list-subst", "driver:make:Makefile#list-loop"} {
		if found[id] != affected.WitnessUnboundedReader {
			t.Errorf("%s: a target that runs what go list printed was not selected through lib's root locator: %v", id, found)
		}
	}
	if kind, ok := found["driver:make:Makefile#list-only"]; ok {
		t.Errorf("list-only: a go list whose output feeds no command was selected as %s on an unrelated change", kind)
	}
}

// AFP-V0-2950: a conditional fallback does not constrain an arbitrary
// initializer. The nested changed directory is absent from the reader's text.
func TestUnprovenLocatorInitializerSelection_AFPV02950(t *testing.T) {
	for _, initializer := range []string{`os.Getenv("DATA_ROOT")`, `mystery()`} {
		t.Run(initializer, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.mod":                 "module example.test/m\n",
				"reader/r.go":            "package reader\nimport (\"os\"; \"path/filepath\")\nfunc F() { root := " + initializer + `; var err error; if root == "" { root, err = os.Getwd() }; _ = err; os.ReadFile(filepath.Join(root, "config.go")) }`,
				"reader/r_test.go":       "package reader\n",
				"other/nested/config.go": "package nested\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			for _, selected := range affected.Select(graph, []string{"other/nested/config.go"}).Selected {
				if selected.UnitID == "go:example.test/m/reader" {
					return
				}
			}
			t.Fatal("reader of arbitrary DATA_ROOT/config.go was omitted")
		})
	}
}

// AFP-V0-2956: arbitrary formatted output may be a program, buffered to a
// named file and executed without a pipe, expansion or feeder.
func TestBufferedListingProgramSelection_AFPV02956(t *testing.T) {
	for _, format := range []string{`go test {{.ImportPath}}`, `{{printf "%s %s" "go test" .ImportPath}}`} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.mod":               "module example.test/m\n",
				"lib/lib.go":           "package lib\nimport \"os\"\nfunc Root() string { cwd, _ := os.Getwd(); return cwd }\n",
				"lib/x_test.go":        "package lib\n",
				"runtime/settings.env": "MODE=fast\n",
				"Makefile":             "check:\n\tgo list -f '" + format + "' ./lib > /tmp/check; sh /tmp/check\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
			if err != nil {
				t.Fatal(err)
			}
			for _, selected := range affected.Select(graph, []string{"runtime/settings.env"}).Selected {
				if selected.UnitID == "driver:make:Makefile#check" {
					return
				}
			}
			t.Fatal("buffered program's runner was omitted for a runtime input change")
		})
	}
}

// AFP-V0-2956: a generated program need not execute the listed package.
// Unknown output options must retain every owner too, without interpreting
// their output or relying on an eventual consumer to be visibly computed.
func TestBufferedListingOtherPackageSelection_AFPV02956(t *testing.T) {
	for _, tc := range []struct {
		name, options string
		selected      bool
	}{
		{"reported-template", `-f 'go test ./{{printf "%s%s" "rea" "der"}}'`, true},
		{"unknown-option", `-future-format program`, true},
		{"default-paths", "", false},
		{"directory-paths", `-f '{{.Dir}}'`, false},
		{"import-paths", `-f '{{.ImportPath}}'`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.mod":               "module example.test/m\n",
				"lib/lib.go":           "package lib\n",
				"lib/x_test.go":        "package lib\n",
				"reader/r.go":          "package reader\nimport (\"os\"; \"path/filepath\")\nfunc Read() { os.ReadFile(filepath.Join(os.Getenv(\"DATA_ROOT\"), \"settings.env\")) }\n",
				"reader/r_test.go":     "package reader\n",
				"runtime/settings.env": "MODE=fast\n",
				"Makefile":             "check:\n\tgo list " + tc.options + " ./lib > /tmp/check\n\tsh /tmp/check\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
			if err != nil {
				t.Fatal(err)
			}
			found := map[string]bool{}
			for _, selected := range affected.Select(graph, []string{"runtime/settings.env"}).Selected {
				found[selected.UnitID] = true
			}
			if !found["go:example.test/m/reader"] {
				t.Error("runtime configuration change omitted its unbounded reader")
			}
			if got := found["driver:make:Makefile#check"]; got != tc.selected {
				t.Errorf("runtime configuration change selected driver = %v, want %v", got, tc.selected)
			}
		})
	}
}

// TestBoundedJoinThroughDirectorySymlinkSelectsReader_AFPV02950: a bounded
// join through a directory symlink reads the link target, so the join keeps
// its last component as a literal and a change to the target still selects
// the reader, while a sibling of the joined file still does not.
func TestBoundedJoinThroughDirectorySymlinkSelectsReader_AFPV02950(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":           "module example.test/m\n",
		"shared/config.go": "package shared\n",
		"shared/other.go":  "package shared\n",
		"shared/x_test.go": "package shared\n",
		"reader/r.go":      "package reader\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\nfunc F() []byte { root, err := os.Getwd(); if err != nil { return nil }; b, _ := os.ReadFile(filepath.Join(root, \"alias\", \"config.go\")); return b }\n",
		"reader/x_test.go": "package reader\n",
	})
	if err := os.Symlink("../shared", filepath.Join(root, "reader", "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	selects := func(dirty string) bool {
		for _, item := range affected.Select(graph, []string{dirty}).Selected {
			if item.UnitID == "go:example.test/m/reader" {
				return true
			}
		}
		return false
	}
	if !selects("shared/config.go") {
		t.Error("a change to the symlink target did not select the joined path's reader")
	}
	if selects("shared/other.go") {
		t.Error("a sibling of the joined file selected its reader")
	}
}

// runnerWitness reads a driver target's whole-tree read as the unbounded
// reader the runner assertions expect: AFP-V0-9401 records a modelled
// `go run` as the anchored glob "/*", which selects the target on every
// change exactly as AFP-V0-2951 and AFP-V0-2962 require.
func runnerWitness(witness affected.Witness) string {
	if witness.Kind == affected.WitnessPathGlobReader && witness.Glob == "/*" {
		return affected.WitnessUnboundedReader
	}
	return witness.Kind
}
