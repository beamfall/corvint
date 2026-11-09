// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-3325: appending a comment to lib/a.go must not hide the coverage
// gap from deleting lib/b.go in a package with no tests or callers. Other
// structural inputs must also prevent the all-comment-only exemption.
func TestCommentOnlyStructuralCoverageGap_AFP_V0_3325(t *testing.T) {
	base := fstest.MapFS{
		"go.mod":   {Data: []byte("module example.test/m\n")},
		"lib/a.go": {Data: []byte("package lib\nconst A = 1\n")},
		"lib/b.go": {Data: []byte("package lib\nconst B = 2\n")},
	}
	for _, change := range []string{"delete", "rename", "add", "data", "nested-data"} {
		t.Run(change, func(t *testing.T) {
			head := maps.Clone(base)
			head["lib/a.go"] = &fstest.MapFile{Data: append(slices.Clone(base["lib/a.go"].Data), []byte("// appended comment\n")...)}
			dirty := []string{"lib/a.go"}
			switch change {
			case "delete", "rename":
				delete(head, "lib/b.go")
				dirty = append(dirty, "lib/b.go")
				if change == "rename" {
					head["lib/c.go"] = base["lib/b.go"]
					dirty = append(dirty, "lib/c.go")
				}
			case "add":
				head["lib/c.go"] = &fstest.MapFile{Data: []byte("package lib\nconst C = 3\n")}
				dirty = append(dirty, "lib/c.go")
			case "data", "nested-data":
				path := "lib/data.txt"
				if change == "nested-data" {
					path = "lib/testdata/data.txt"
				}
				head[path] = &fstest.MapFile{Data: []byte("changed bytes\n")}
				dirty = append(dirty, path)
			}
			graph, err := affected.BuildFS(head, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			changes := golang.GoChangeRecords(base, head, dirty)
			if !slices.Equal(changes, []affected.GoChange{{Path: "lib/a.go", Kind: affected.GoCommentOnly}}) {
				t.Fatalf("fixture classification = %+v", changes)
			}
			for _, classified := range []bool{false, true} {
				candidate := graph
				if classified {
					candidate = graph.WithGoChanges(changes)
				}
				plan := affected.Select(candidate, dirty)
				gap := affected.Unknown{Reason: affected.UnknownNoSelectableTest, Detail: "go:example.test/m/lib"}
				if !slices.Contains(plan.Unknown, gap) && !slices.Contains(plan.Uncovered, gap) {
					t.Fatalf("classified=%v: structural change lost NO_SELECTABLE_TEST: unknown=%+v uncovered=%+v", classified, plan.Unknown, plan.Uncovered)
				}
			}
		})
	}
}

// AFP-V0-3322: reader imports the tested lib and reads lib/a.go, but has no
// tests itself. Consumer's tests must still check those bytes through reader
// when a comment-only edit removes lib/a.go from compilation seeds.
func TestCommentOnlyTestlessReaderOfTestedOwner_AFP_V0_3322(t *testing.T) {
	base := fstest.MapFS{
		"go.mod":                    {Data: []byte("module example.test/m\n")},
		"lib/a.go":                  {Data: []byte("package lib\nconst A = 1\n")},
		"lib/a_test.go":             {Data: []byte("package lib\nimport \"testing\"\nfunc TestA(t *testing.T) { if A != 1 { t.Fatal(A) } }\n")},
		"reader/reader.go":          {Data: []byte("package reader\nimport (\"os\"; \"example.test/m/lib\")\nfunc Read() ([]byte, error) { _ = lib.A; return os.ReadFile(\"lib/a.go\") }\n")},
		"consumer/consumer.go":      {Data: []byte("package consumer\nimport \"example.test/m/reader\"\nfunc Read() ([]byte, error) { return reader.Read() }\n")},
		"consumer/consumer_test.go": {Data: []byte("package consumer\nimport \"testing\"\nfunc TestRead(t *testing.T) { if _, err := Read(); err != nil { t.Fatal(err) } }\n")},
	}
	head := maps.Clone(base)
	head["lib/a.go"] = &fstest.MapFile{Data: append(slices.Clone(base["lib/a.go"].Data), []byte("// appended comment\n")...)}
	graph, err := affected.BuildFS(head, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"lib/a.go"}
	changes := golang.GoChangeRecords(base, head, dirty)
	if !slices.Equal(changes, []affected.GoChange{{Path: dirty[0], Kind: affected.GoCommentOnly}}) {
		t.Fatalf("fixture classification = %+v", changes)
	}
	const consumer = "go:example.test/m/consumer"
	if !slices.Contains(affprecIDs(affected.Select(graph, dirty)), consumer) {
		t.Fatal("baseline does not reach consumer through compilation")
	}
	plan := affected.Select(graph.WithGoChanges(changes), dirty)
	for _, selection := range plan.Selected {
		if selection.UnitID == consumer {
			if selection.Witness.DirtyPath != dirty[0] || !slices.Equal(selection.Witness.Via, []string{"go:example.test/m/reader", consumer}) {
				t.Fatalf("consumer must be reached through its independent reader: %+v", selection.Witness)
			}
			return
		}
	}
	t.Fatalf("comment-only source read lost consumer tests: %+v", plan.Selected)
}
