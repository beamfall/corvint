// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9010 / AFP-V0-2200: an environment source must resolve to the
// unshadowed standard os import. Spelling and unknown producers prove nothing.
func TestReaderEnvironmentIdentity_AFP_V0_9010(t *testing.T) {
	const custom = `type environment struct{}
func (environment) Environ() []string { return append(realos.Environ(), "\x47OFLAGS=-json") }
`
	for _, test := range []struct {
		name, imports, helper, setup, env string
		unproven                          bool
	}{
		{"standard import", `"os"`, "", "", `append(os.Environ(), "EXTRA=1")`, false},
		{"aliased standard import", `realos "os"`, "", "", `append(realos.Environ(), "EXTRA=1")`, false},
		{"literal environment", "", "", "", `[]string{"EXTRA=1"}`, false},
		{"custom receiver", `realos "os"`, custom + "var os environment\n", "", `append(os.Environ(), "EXTRA=1")`, true},
		{"shadowed standard import", `realos "os"`, "", `_ = realos.Environ
realos := struct{ Environ func() []string }{func() []string {
return []string{"\x47OFLAGS=-json"}
}}`, `append(realos.Environ(), "EXTRA=1")`, true},
		{"unknown receiver form", `realos "os"`, custom + "func unknownEnvironment() environment { return environment{} }\n", "", `append(unknownEnvironment().Environ(), "EXTRA=1")`, true},
	} {
		for _, changedDir := range []string{"lib", "base"} {
			t.Run(test.name+"/"+changedDir, func(t *testing.T) {
				root := importGraphRepository(t)
				existing := "package " + changedDir + "\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n"
				added := "package " + changedDir + "\nimport \"testing\"\nfunc TestB(t *testing.T) {}\n"
				writeFiles(t, root, map[string]string{
					changedDir + "/a_test.go": existing,
					"probe/probe_test.go": "package probe\nimport (\"os/exec\"; \"testing\"; " + test.imports + ")\n" + test.helper +
						"func TestList(t *testing.T) {\n" + test.setup + "\n" +
						`c := exec.Command("go", "list", "-deps", "./cmd/tool")` + "\n" +
						"c.Env = " + test.env + "\n_, _ = c.Output()\n}\n",
				})
				graph, err := affected.Build(context.Background(), root, golang.New())
				if err != nil {
					t.Fatal(err)
				}
				reader := importGraphModule + "probe"
				unit, ok := graph.Unit(reader)
				if !ok || unit.ImportGraphReadsFiles != test.unproven || unit.ImportGraphReadsAll != test.unproven {
					t.Fatalf("environment identity: %+v, want unproven=%v", unit, test.unproven)
				}
				base := fstest.MapFS{
					"go.mod":                  {Data: []byte("module example.test/g\n")},
					changedDir + "/lib.go":    {Data: []byte("package " + changedDir + "\n")},
					changedDir + "/a_test.go": {Data: []byte(existing)},
				}
				head := fstest.MapFS{"go.mod": base["go.mod"], changedDir + "/lib.go": base[changedDir+"/lib.go"], changedDir + "/a_test.go": base[changedDir+"/a_test.go"], changedDir + "/b_test.go": {Data: []byte(added)}}
				dirty := []string{changedDir + "/b_test.go"}
				changes := golang.ImportSetRecords(base, head, graph, dirty)
				if len(changes.Changed) != 0 || changes.Module != "" || changes.Source != dirty[0] {
					t.Fatalf("addition must keep the folded import set: %+v", changes)
				}
				selected := func(changes *affected.ImportSetChanges, id string) affected.Witness {
					for _, selection := range affected.Select(graph.WithImportSetChanges(changes), dirty).Selected {
						if selection.UnitID == id {
							return selection.Witness
						}
					}
					return affected.Witness{}
				}
				witness := selected(changes, reader)
				if test.unproven {
					want := affected.WitnessImportGraphReader
					if changedDir == "lib" {
						want = affected.WitnessBinaryExec
					}
					if witness.Kind != want || witness.DirtyPath != dirty[0] {
						t.Fatalf("unproven environment missed kept-import addition: %+v", witness)
					}
				} else if witness.Kind != "" {
					t.Fatalf("proven environment selected: %+v", witness)
				}
				if witness := selected(changes, importGraphModule+"guard"); witness.Kind != "" {
					t.Fatalf("proven import-only control selected: %+v", witness)
				}
				// The unrelated package has no competing binary edge: removing the
				// source witness must lose selection there.
				if witness := selected(&affected.ImportSetChanges{Changed: changes.Changed}, reader); changedDir == "base" && witness.Kind != "" {
					t.Fatalf("reader selected without source observation: %+v", witness)
				}
			})
		}
	}
}
