package golang_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-551 (V1-4173, V1-3392): a test-file literal on a line carrying the
// ASD-V0-895 fixture marker names no read, so a token only such literals of a
// test file hold selects no PATH_LITERAL_READER. A token an unmarked literal of
// the same file also holds, a marker in a non-test file, and a marker on
// another line exempt nothing; a marked parent climb still reaches the root.
func TestAFPV0551FixtureMarkedLiteralsNameNoRead(t *testing.T) {
	marker := "//" + "corvint:fixture-path"
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":               "module example.test/m\n", //corvint:fixture-path
		"marked/m.go":          "package marked\n",
		"marked/m_test.go":     "package marked\n\nvar fixture = map[string]string{\n\t\"Makefile\": \"all:\\n\", " + marker + "\n}\n", //corvint:fixture-path
		"raw/r.go":             "package raw\n",
		"raw/r_test.go":        "package raw\n\nvar fixture = `x\nMakefile` " + marker + "\n", //corvint:fixture-path
		"shared/s.go":          "package shared\n",
		"shared/s_test.go":     "package shared\n\nvar fixture = \"Makefile\" " + marker + "\n\nvar real = \"Makefile\"\n", //corvint:fixture-path
		"elsewhere/e.go":       "package elsewhere\n",
		"elsewhere/e_test.go":  "package elsewhere\n\n" + marker + "\nvar fixture = \"Makefile\"\n", //corvint:fixture-path
		"source/s.go":          "package source\n\nvar fixture = \"Makefile\" " + marker + "\n",     //corvint:fixture-path
		"source/s_test.go":     "package source\n",
		"climb/c.go":           "package climb\n",
		"climb/c_test.go":      "package climb\n\nvar fixture = \"../..\" " + marker + "\n",
		"unmarked/u.go":        "package unmarked\n",
		"unmarked/u_test.go":   "package unmarked\n\nvar fixture = \"Makefile\"\n", //corvint:fixture-path
		"neighbour/n.go":       "package neighbour\n",
		"neighbour/n_test.go":  "package neighbour\n\nvar fixture = \"go.mod\" " + marker + "\n", //corvint:fixture-path
		"neighbour/go_test.go": "package neighbour\n\nvar real = \"go.mod\"\n",                   //corvint:fixture-path
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	selected := func(dirty ...string) string {
		plan := affected.Select(graph, dirty)
		ids := make([]string, 0, len(plan.Selected))
		for _, selection := range plan.Selected {
			ids = append(ids, selection.UnitID+"<-"+selection.Witness.Kind)
		}
		return fmt.Sprint(ids)
	}
	const m = "go:example.test/m/"
	want := "[" + m + "climb<-UNBOUNDED_READER " + m + "elsewhere<-PATH_LITERAL_READER " + m + "shared<-PATH_LITERAL_READER " + m + "source<-PATH_LITERAL_READER " + m + "unmarked<-PATH_LITERAL_READER]"
	if got := selected("Makefile"); got != want { //corvint:fixture-path
		t.Fatalf("Makefile selects %s, want %s", got, want) //corvint:fixture-path
	}
	// A marker exempts per file: another test file's unmarked literal of the
	// same package still reads the path.
	if got := selected("go.mod"); got != "["+m+"climb<-UNBOUNDED_READER "+m+"neighbour<-PATH_LITERAL_READER]" { //corvint:fixture-path
		t.Fatalf("go.mod selects %s", got)
	}
	// The AFP-V0-021 lexicon itself is unchanged: FilePathTokens, which
	// tools/test-read-scopes-check applies the marker to, still returns the
	// marked literal's token.
	tokens, err := golang.FilePathTokens("m_test.go", []byte("package marked\n\nvar fixture = \"Makefile\" "+marker+"\n"), "") //corvint:fixture-path
	if err != nil || fmt.Sprint(tokens) != "[Makefile]" {                                                                      //corvint:fixture-path
		t.Fatalf("FilePathTokens = %v, %v", tokens, err)
	}
}

// AFP-V0-551: Go's scanner removes carriage returns from comment text, so
// both spellings of a fixture marker must exempt the same literal.
func TestAFPV0551FixtureMarkerCarriageReturnParity(t *testing.T) {
	var selections [][]affected.Selection
	for _, carriageReturn := range []string{"", "\r"} {
		marker := "//" + "corvint:fixture-" + carriageReturn + "path"
		root := t.TempDir()
		writeFiles(t, root, map[string]string{
			"go.mod":           "module example.test/m\n", //corvint:fixture-path
			"marked/m.go":      "package marked\n",
			"marked/m_test.go": "package marked\n\nvar fixture = \"Makefile\" " + marker + "\n", //corvint:fixture-path
			"reader/r.go":      "package reader\n",
			"reader/r_test.go": "package reader\n\nvar real = \"Makefile\"\n", //corvint:fixture-path
		})
		graph, err := affected.Build(context.Background(), root, golang.New())
		if err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{"Makefile"}) //corvint:fixture-path
		if len(plan.Selected) != 1 || plan.Selected[0].UnitID != "go:example.test/m/reader" || plan.Selected[0].Witness.Kind != "PATH_LITERAL_READER" {
			t.Errorf("marker %q selects %+v; want only the unmarked reader", marker, plan.Selected)
		}
		selections = append(selections, plan.Selected)
	}
	if !reflect.DeepEqual(selections[0], selections[1]) {
		t.Errorf("fixture marker selection differs with a carriage return: plain %+v, carriage return %+v", selections[0], selections[1])
	}
}

// AFP-V0-831 (V1-3509, V1-4258): the dependent nested-module reader honours
// the marker too: a README.md token only a marked _test.go literal holds no
// longer makes the module a reader of a README edit, while an unmarked one, or
// a marked literal in a non-test file, still does.
func TestNestedModuleHonoursFixtureMarker_AFP_V0_831(t *testing.T) {
	marker := "//" + "corvint:fixture-path"
	manifest := "module example.test/nested\n\ngo 1.26\n\nrequire example.test/root v0.0.0\n\nreplace example.test/root => ../\n"
	for _, test := range []struct {
		name, file, body string
		reads            bool
	}{
		{"marked test literal", "nested/tree_test.go", "package nested\n\nvar entry = \"README.md\" " + marker + "\n", false},
		{"unmarked test literal", "nested/tree_test.go", "package nested\n\nvar entry = \"README.md\"\n", true},
		{"marked non-test literal", "nested/tree.go", "package nested\n\nvar entry = \"README.md\" " + marker + "\n", true},
	} {
		root := nestedFixture(t, manifest, map[string]string{
			"nested/n.go": "package nested\n",
			test.file:     test.body,
			"README.md":   "# root\n", //corvint:fixture-path
		})
		graph, err := affected.Build(context.Background(), root, golang.New())
		if err != nil {
			t.Fatalf("%s: build: %v", test.name, err)
		}
		bears := false
		for _, unknown := range affected.Select(graph, []string{"README.md"}).Unknown { //corvint:fixture-path
			bears = bears || unknown.Detail == golang.FrontierNestedModule
		}
		if bears != test.reads {
			t.Errorf("%s: nested frontier bears %v on a README edit, want %v", test.name, bears, test.reads)
		}
	}
}
