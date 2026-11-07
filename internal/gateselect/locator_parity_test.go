// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// plannerUnbounded lists the planner's rule (d) directories of the fixture.
func plannerUnbounded(t *testing.T, root string) []string {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	planner := []string{}
	for _, id := range graph.UnboundedReaders() {
		if rest, ok := strings.CutPrefix(id, "go:"+fixtureModule+"/"); ok {
			planner = append(planner, rest)
		}
	}
	sort.Strings(planner)
	return planner
}

// agree holds this index's rule (d) set, cold and warm, equal to want and to
// the planner's.
func agree(t *testing.T, root string, want []string) *repositoryIndex {
	t.Helper()
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	unresolved := sortedKeys(index.unresolved())
	if !reflect.DeepEqual(unresolved, want) {
		t.Errorf("gateselect rule (d) = %v, want %v", unresolved, want)
	}
	if planner := plannerUnbounded(t, root); !reflect.DeepEqual(unresolved, planner) {
		t.Errorf("gateselect rule (d) = %v, planner rule (d) = %v", unresolved, planner)
	}
	var body []byte
	for _, state := range []string{"cold", "warm"} {
		memoized, err := indexRepositoryMemo(root, fixtureModule, memoryScanMemo("program", &body))
		if err != nil {
			t.Fatal(err)
		}
		if got := sortedKeys(memoized.unresolved()); !reflect.DeepEqual(got, want) {
			t.Errorf("%s memo rule (d) = %v, want %v", state, got, want)
		}
	}
	return index
}

// TestBoundedJoinLocatorParity_AFPV02950 holds this index's rule (d) and the
// planner's equal over os.Getwd results (V1-4283): a result joined with
// literal plain components into a path only a read at or below it consumes
// (`joiner`, an aliased `=` form `aliased`, a rebinding `rebound`, a
// directory listing `lister`, a bound joined path `bound`) is no unbounded
// read and contributes the joined path as a root-anchored token; a result
// that escapes (`returner`, `copier`, `passer`), climbs (`climber`,
// `parent`), is joined with a computed, second-position, `.` or glob argument
// (`computed`, `second`, `dotted`, `globbed`) or lands in a package-level
// variable (`global`) stays one, and so does a joined path that a consumer
// could widen (`widened`, `joinreturn`, `opener`, `rejoined`, `handed`). A
// warm scan memo keeps the same set and tokens.
func TestBoundedJoinLocatorParity_AFPV02950(t *testing.T) {
	src := func(name, body string) string {
		return "package " + name + "\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\n\n" + body + "\n"
	}
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"joiner/joiner.go":      src("joiner", "func F() []byte { root, err := os.Getwd(); if err != nil { return nil }; b, _ := os.ReadFile(filepath.Join(root, \"docs\", \"guide.md\")); return b }"),
		"joiner/joiner_test.go": "package joiner\n",
		"aliased/aliased.go":    "package aliased\n\nimport (\n\tsystem \"os\"\n\tfp \"path/filepath\"\n)\n\nfunc F() bool { var cwd string; var err error; cwd, err = system.Getwd(); if err != nil { return false }; _, err = system.Stat(fp.Join(cwd, \"docs/guide.md\")); return err == nil }\n",
		"aliased/a_test.go":     "package aliased\n",
		"rebound/rebound.go":    src("rebound", "func F(given string) []byte { root := given; if root == \"\" { var err error; root, err = os.Getwd(); if err != nil { return nil } }; b, _ := os.ReadFile(filepath.Join(root, \"docs\", \"guide.md\")); return b }"),
		"bound/bound.go":        src("bound", "func F() int { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); if docs == \"/\" { return 0 }; var guide = filepath.Join(cwd, \"docs\", \"guide.md\"); os.Stat(guide); entries, _ := os.ReadDir(docs); return len(entries) }"),
		"bound/b_test.go":       "package bound\n",
		"widened/widened.go":    src("widened", "func F() []byte { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); b, _ := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(docs)), \"config.go\")); return b }"),
		"widened/w_test.go":     "package widened\n",
		"joinreturn/j.go":       src("joinreturn", "func F() string { cwd, _ := os.Getwd(); return filepath.Join(cwd, \"docs\") }"),
		"joinreturn/j_test.go":  "package joinreturn\n",
		"opener/opener.go":      src("opener", "func F() string { cwd, _ := os.Getwd(); f, _ := os.Open(filepath.Join(cwd, \"docs\")); return filepath.Dir(f.Name()) }"),
		"opener/o_test.go":      "package opener\n",
		"rejoined/rejoined.go":  src("rejoined", "func F() int { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); docs = filepath.Dir(docs); entries, _ := os.ReadDir(docs); return len(entries) }"),
		"rejoined/r_test.go":    "package rejoined\n",
		"handed/handed.go":      src("handed", "func G(string) {}\nfunc F() { cwd, _ := os.Getwd(); docs := filepath.Join(cwd, \"docs\"); G(docs) }"),
		"handed/h_test.go":      "package handed\n",
		"rebound/r_test.go":     "package rebound\n",
		"lister/lister.go":      src("lister", "func F() int { root, err := os.Getwd(); if err != nil { return 0 }; entries, _ := os.ReadDir(filepath.Join(root, \"docs\")); return len(entries) }"),
		"lister/l_test.go":      "package lister\n",
		"returner/returner.go":  "package returner\n\nimport \"os\"\n\nfunc F() string { cwd, _ := os.Getwd(); return cwd }\n",
		"returner/r_test.go":    "package returner\n",
		"copier/copier.go":      src("copier", "func F() string { cwd, _ := os.Getwd(); other := cwd; return filepath.Join(other, \"docs\") }"),
		"copier/c_test.go":      "package copier\n",
		"passer/passer.go":      src("passer", "func G(s string) string { return s }\nfunc F() string { cwd, _ := os.Getwd(); return filepath.Join(G(cwd), \"docs\") }"),
		"passer/p_test.go":      "package passer\n",
		"climber/climber.go":    src("climber", "func F() string { cwd, _ := os.Getwd(); return filepath.Join(cwd, \"..\", \"docs\") }"),
		"climber/c_test.go":     "package climber\n",
		"parent/parent.go":      src("parent", "func F() string { cwd, _ := os.Getwd(); return filepath.Dir(cwd) }"),
		"parent/p_test.go":      "package parent\n",
		"computed/computed.go":  src("computed", "func F(name string) string { cwd, _ := os.Getwd(); return filepath.Join(cwd, name) }"),
		"computed/c_test.go":    "package computed\n",
		"second/second.go":      src("second", "func F() string { cwd, _ := os.Getwd(); return filepath.Join(\"prefix\", cwd, \"docs\") }"),
		"second/s_test.go":      "package second\n",
		"dotted/dotted.go":      src("dotted", "func F() string { cwd, _ := os.Getwd(); return filepath.Join(cwd, \".\") }"),
		"dotted/d_test.go":      "package dotted\n",
		"globbed/globbed.go":    src("globbed", "func F() string { cwd, _ := os.Getwd(); return filepath.Join(cwd, \"docs\", \"*.md\") }"),
		"globbed/g_test.go":     "package globbed\n",
		"global/global.go":      src("global", "var root string\n\nfunc F() string { var err error; root, err = os.Getwd(); if err != nil { return \"\" }; return filepath.Join(root, \"docs\") }"),
		"global/g_test.go":      "package global\n",
	})
	// widened is the review's case (a bounded join climbed with
	// filepath.Dir); joinreturn, opener, rejoined and handed hand the joined
	// path to a consumer that could widen it.
	want := []string{"climber", "computed", "copier", "dotted", "global", "globbed", "handed", "joinreturn", "opener", "parent", "passer", "rejoined", "returner", "second", "widened"}
	index := agree(t, root, want)
	for directory, token := range map[string]string{"joiner": "/docs/guide.md", "aliased": "/docs/guide.md", "rebound": "/docs/guide.md", "lister": "/docs", "bound": "/docs"} {
		if holders := index.holders[token]; !containsString(holders, directory) {
			t.Errorf("%s: the joined path %q is not a token of the package: holders %v", directory, token, holders)
		}
	}
	// The anchored token places the joined path: the listing reads below
	// docs and the file join reads its file. (This index's rule (c) is the
	// broader fast-tier lexicon, AFP-V0-765, so a lone "docs" token may name
	// a sibling here that the planner's AFP-V0-072 narrowing does not.)
	for _, dirty := range []string{"docs/guide.md", "docs/other.md"} {
		if readers := index.readers(dirty); !containsString(readers, "lister") {
			t.Errorf("%s: lister is not a reader: %v", dirty, readers)
		}
	}
	if readers := index.readers("docs/guide.md"); !containsString(readers, "joiner") {
		t.Errorf("docs/guide.md: joiner is not a reader: %v", readers)
	}
}

// TestLineBrokenClimbParity_AFPV02952 holds this index's rule (d) and the
// planner's equal over climbing literals (V1-4283): a non-test file's `..`
// token held only by line-broken literals without a directory-changing word is
// text (`prose`, `template`, `raw`); a single-line path (`path`), a script
// with `cd` (`script`), a token both forms hold (`both`) and a test file
// (`tested`) keep rule (d). A warm scan memo keeps the same set.
func TestLineBrokenClimbParity_AFPV02952(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"prose/prose.go":        "package prose\n\nvar Doc = \"| manifest.json | %s | See ../manifest.json for the digest. |\\n\\nMore text.\\n\"\n",
		"prose/p_test.go":       "package prose\n",
		"template/template.go":  "package template\n\nimport \"fmt\"\n\nfunc Ruby(model string) []byte { return fmt.Appendf(nil, \"require_relative \\\"../models/%s\\\"\\n\\nclass X\\nend\\n\", model) }\n",
		"template/t_test.go":    "package template\n",
		"raw/raw.go":            "package raw\n\nvar Doc = `| manifest.json | See ../manifest.json for the digest. |\n\nMore text.\n`\n",
		"raw/r_test.go":         "package raw\n",
		"path/path.go":          "package path\n\nvar Manifest = \"../manifest.json\"\n",
		"path/p_test.go":        "package path\n",
		"script/script.go":      "package script\n\nvar Script = \"set -e\\ncd ../fixture\\ncat ../manifest.json\\n\"\n",
		"script/s_test.go":      "package script\n",
		"both/both.go":          "package both\n\nvar Doc = \"See ../manifest.json\\n\"\nvar Path = \"../manifest.json\"\n",
		"both/b_test.go":        "package both\n",
		"tested/tested.go":      "package tested\n",
		"tested/tested_test.go": "package tested\n\nvar Doc = \"See ../../manifest.json\\n\"\n",
	})
	agree(t, root, []string{"both", "path", "script", "tested"})
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
