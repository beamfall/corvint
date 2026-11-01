package golang_test

import (
	"context"
	"fmt"
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
		"marked/m_test.go":     "package marked\n\nvar fixture = map[string]string{\n\t\"Makefile\": \"all:\\n\", " + marker + "\n}\n",
		"raw/r.go":             "package raw\n",
		"raw/r_test.go":        "package raw\n\nvar fixture = `x\nMakefile` " + marker + "\n",
		"shared/s.go":          "package shared\n",
		"shared/s_test.go":     "package shared\n\nvar fixture = \"Makefile\" " + marker + "\n\nvar real = \"Makefile\"\n",
		"elsewhere/e.go":       "package elsewhere\n",
		"elsewhere/e_test.go":  "package elsewhere\n\n" + marker + "\nvar fixture = \"Makefile\"\n",
		"source/s.go":          "package source\n\nvar fixture = \"Makefile\" " + marker + "\n",
		"source/s_test.go":     "package source\n",
		"climb/c.go":           "package climb\n",
		"climb/c_test.go":      "package climb\n\nvar fixture = \"../..\" " + marker + "\n",
		"unmarked/u.go":        "package unmarked\n",
		"unmarked/u_test.go":   "package unmarked\n\nvar fixture = \"Makefile\"\n",
		"neighbour/n.go":       "package neighbour\n",
		"neighbour/n_test.go":  "package neighbour\n\nvar fixture = \"go.mod\" " + marker + "\n",
		"neighbour/go_test.go": "package neighbour\n\nvar real = \"go.mod\"\n",
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
		t.Fatalf("Makefile selects %s, want %s", got, want)
	}
	// A marker exempts per file: another test file's unmarked literal of the
	// same package still reads the path.
	if got := selected("go.mod"); got != "["+m+"climb<-UNBOUNDED_READER "+m+"neighbour<-PATH_LITERAL_READER]" { //corvint:fixture-path
		t.Fatalf("go.mod selects %s", got)
	}
	// The AFP-V0-021 lexicon itself is unchanged: FilePathTokens, which
	// tools/test-read-scopes-check applies the marker to, still returns the
	// marked literal's token.
	tokens, err := golang.FilePathTokens("m_test.go", []byte("package marked\n\nvar fixture = \"Makefile\" "+marker+"\n"), "")
	if err != nil || fmt.Sprint(tokens) != "[Makefile]" {
		t.Fatalf("FilePathTokens = %v, %v", tokens, err)
	}
}
