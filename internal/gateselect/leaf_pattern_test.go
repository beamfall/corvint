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

// TestLeafPackagePatternParity_AFPV02542 holds this index's rule (d) and the
// planner's equal over relative package patterns in test files (V1-4512). A
// "./..." or "./NAME/..." whose subtree holds no other package directory
// reaches what "." reaches and is no unbounded read: `leaf` (no subdirectory),
// `datum` (a data-only subdirectory) and `fixtured` (Go files only below
// testdata). A pattern that reaches another package, a nested module, the
// module's import path or a parent directory stays one: `parent`, `outer`,
// `imported` and `climber`. A leaf pattern stays one too when a file of the
// package moves the working directory the command runs in: `rooted` (a test
// that locates the repository root with runtime.Caller, sets cmd.Dir there and
// builds "./..."), `helper` (a sibling file sets Dir), `keyed` (an exec.Cmd
// literal keys Dir), `chdir` (t.Chdir), `flagged` (go -C) and `shell` (a cd
// word); `dotted` sets cmd.Dir to "." and stays bounded. A warm scan memo
// keeps the same set.
func TestLeafPackagePatternParity_AFPV02542(t *testing.T) {
	pkg := func(name string) string { return "package " + name + "\n" }
	test := func(name, literal string) string {
		return "package " + name + "\n\nimport \"testing\"\n\nvar patterns = []string{" + literal + "}\n\nfunc TestX(t *testing.T) { _ = patterns }\n"
	}
	command := func(name, statements string) string {
		return "package " + name + "\n\nimport (\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nvar root = filepath.Join(\"a\", \"b\")\n\nfunc TestX(t *testing.T) {\n\tcmd := " + statements + "\n\t_ = cmd\n}\n"
	}
	rooted := "package rooted\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"runtime\"\n\t\"testing\"\n)\n\n" +
		"func repositoryRoot() string {\n\t_, file, _, _ := runtime.Caller(0)\n\tdirectory := filepath.Dir(file)\n\tfor {\n\t\tif _, err := os.Stat(filepath.Join(directory, \"go.mod\")); err == nil {\n\t\t\treturn directory\n\t\t}\n\t\tdirectory = filepath.Dir(directory)\n\t}\n}\n\n" +
		"func TestBuild(t *testing.T) {\n\tcmd := exec.Command(\"go\", \"build\", \"./...\")\n\tcmd.Dir = repositoryRoot()\n\t_ = cmd.Run()\n}\n"
	root := writeFixture(t, map[string]string{
		"go.mod":                        "module example.com/fixture\n\ngo 1.22\n",
		"leaf/leaf.go":                  pkg("leaf"),
		"leaf/leaf_test.go":             test("leaf", `"./..."`),
		"datum/datum.go":                pkg("datum"),
		"datum/datum_test.go":           test("datum", `"./data/..."`),
		"datum/data/rows.txt":           "rows\n",
		"fixtured/fixtured.go":          pkg("fixtured"),
		"fixtured/fixtured_test.go":     test("fixtured", `"./..."`),
		"fixtured/testdata/m/m.go":      pkg("m"),
		"parent/parent.go":              pkg("parent"),
		"parent/parent_test.go":         test("parent", `"./..."`),
		"parent/sub/sub.go":             pkg("sub"),
		"outer/outer.go":                pkg("outer"),
		"outer/outer_test.go":           test("outer", `"./..."`),
		"outer/inner/go.mod":            "module example.com/inner\n\ngo 1.22\n",
		"outer/inner/inner.go":          pkg("inner"),
		"imported/imported.go":          pkg("imported"),
		"imported/imported_test.go":     test("imported", `"example.com/fixture/imported/..."`),
		"climber/climber.go":            pkg("climber"),
		"climber/climber_test.go":       test("climber", `"./../..."`),
		"deep/deep.go":                  pkg("deep"),
		"deep/deep_test.go":             test("deep", `"./data/.../x/..."`),
		"deep/data/notes.txt":           "notes\n",
		"wildcard/wildcard.go":          pkg("wildcard"),
		"wildcard/wildcard_test.go":     test("wildcard", `"./.../..."`),
		"production/production.go":      "package production\n\nvar Pattern = \"./...\"\n",
		"production/production_test.go": test("production", `""`),
		"rooted/rooted.go":              pkg("rooted"),
		"rooted/rooted_test.go":         rooted,
		"helper/helper.go":              pkg("helper"),
		"helper/helper_test.go":         test("helper", `"./..."`),
		"helper/run_test.go":            "package helper\n\nimport \"os/exec\"\n\nfunc goIn(dir string, args ...string) *exec.Cmd {\n\tcmd := exec.Command(\"go\", args...)\n\tcmd.Dir = dir\n\treturn cmd\n}\n",
		"keyed/keyed.go":                pkg("keyed"),
		"keyed/keyed_test.go":           command("keyed", `&exec.Cmd{Path: "go", Args: []string{"go", "vet", "./..."}, Dir: root}`),
		"chdir/chdir.go":                pkg("chdir"),
		"chdir/chdir_test.go":           command("chdir", `exec.Command("go", "vet", "./...")`+"\n\tt.Chdir(root)"),
		"flagged/flagged.go":            pkg("flagged"),
		"flagged/flagged_test.go":       command("flagged", `exec.Command("go", "-C", root, "vet", "./...")`),
		"shell/shell.go":                pkg("shell"),
		"shell/shell_test.go":           command("shell", `exec.Command("sh", "-c", "cd \"$ROOT\" && go vet ./...")`),
		"dotted/dotted.go":              pkg("dotted"),
		"dotted/dotted_test.go":         command("dotted", `exec.Command("go", "vet", "./...")`+"\n\tcmd.Dir = \".\"\n\t_ = filepath.Dir(root)"),
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
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
	unresolved := sortedKeys(index.unresolved())
	want := []string{"chdir", "climber", "deep", "flagged", "helper", "imported", "keyed", "outer", "parent", "rooted", "shell", "wildcard"}
	if !reflect.DeepEqual(unresolved, want) {
		t.Errorf("gateselect rule (d) = %v, want %v", unresolved, want)
	}
	if !reflect.DeepEqual(unresolved, planner) {
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
}
