// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	json "encoding/json/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func commentChangeGraph(t *testing.T) *Graph {
	t.Helper()
	graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
		{ID: "go:m/lib", Sources: []string{"lib/a.go", "lib/z.go"}, Tests: []string{"lib/lib_test.go"}},
		{ID: "go:m/user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:m/lib"}},
		{ID: "go:m/tool", Sources: []string{"tool/main.go"}, Imports: []string{"go:m/lib"}},
		{ID: "go:m/runner", Tests: []string{"runner/runner_test.go"}, Execs: []string{"go:m/tool"}, TestOnlyExecs: []string{"go:m/tool"}},
		{ID: "go:m/list", Tests: []string{"list/list_test.go"}, ImportGraphReadsAll: true},
		{ID: "go:m/metadata", Tests: []string{"metadata/metadata_test.go"}, ImportGraphReadsAll: true, ImportGraphReadsFiles: true},
		{ID: "go:m/literal", Tests: []string{"literal/literal_test.go"}, PathTokens: []string{"lib/a.go"}},
		{ID: "go:m/glob", Tests: []string{"glob/glob_test.go"}, PathGlobs: []string{"lib/*.go"}},
		{ID: "go:m/scoped", Tests: []string{"scoped/scoped_test.go"}, ReadScoped: true, ReadScope: []string{"lib/"}},
		{ID: "go:m/wide", Tests: []string{"wide/wide_test.go"}, UnboundedReads: "opens computed paths"},
	}}, fake{name: DriverNamespace, units: []Unit{
		{ID: "driver:make:Makefile#build", Imports: []string{"go:m/lib"}},
		{ID: "driver:make:Makefile#read", PathTokens: []string{"lib/a.go"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// AFP-V0-3321, AFP-V0-3322: compile-only closure is removed; all byte-reader
// routes and the owning package remain. Missing classifications are unknown.
func TestCommentOnlySelection_AFP_V0_3321(t *testing.T) {
	graph := commentChangeGraph(t)
	dirty := []string{"lib/a.go"}
	baseline := Select(graph, dirty)
	full := witnesses(baseline)
	for id, kind := range map[string]string{
		"go:m/user": WitnessDependency, "go:m/runner": WitnessBinaryExec,
		"go:m/list": WitnessImportGraphReader, "driver:make:Makefile#build": WitnessDependency,
	} {
		if full[id].Kind != kind {
			t.Fatalf("baseline does not exercise %s: %+v", kind, full)
		}
	}
	classified := graph.WithGoChanges([]GoChange{{Path: dirty[0], Kind: GoCommentOnly}})
	plan := Select(classified, dirty)
	got := witnesses(plan)
	want := map[string]string{
		"go:m/lib": WitnessDirectSource, "go:m/literal": WitnessPathLiteralReader,
		"go:m/glob": WitnessPathGlobReader, "go:m/scoped": WitnessDeclaredReadScope,
		"go:m/wide": WitnessUnboundedReader, "driver:make:Makefile#read": WitnessPathLiteralReader,
		"go:m/metadata": WitnessUnboundedReader,
	}
	if len(got) != len(want) {
		t.Fatalf("selected = %+v", got)
	}
	for id, kind := range want {
		if got[id].Kind != kind {
			t.Errorf("%s = %+v; want %s", id, got[id], kind)
		}
	}
	encoded, err := json.Marshal(plan)
	if err != nil || !strings.Contains(string(encoded), `"goChanges":[{"path":"lib/a.go","kind":"COMMENT_ONLY"}]`) {
		t.Fatalf("classification missing: %s, %v", encoded, err)
	}
	if !reflect.DeepEqual(Select(graph, dirty), baseline) || !reflect.DeepEqual(Select(classified.WithGoChanges(nil), dirty), baseline) {
		t.Fatal("per-plan classification mutated the original graph or survived reset")
	}
}

// AFP-V0-3323: a sibling code edit still propagates, including when the
// package import-set comparison attributed a changed header to its first
// dirty path (the comment-only file).
func TestMixedCommentAndCodeSelection_AFP_V0_3323(t *testing.T) {
	graph := commentChangeGraph(t).WithGoChanges([]GoChange{{Path: "lib/a.go", Kind: GoCommentOnly}})
	graph = graph.WithImportSetChanges(&ImportSetChanges{Changed: map[string]string{"go:m/lib": "lib/a.go"}, Source: "lib/a.go"})
	got := witnesses(Select(graph, []string{"lib/a.go", "lib/z.go"}))
	for id, kind := range map[string]string{"go:m/user": WitnessDependency, "go:m/runner": WitnessBinaryExec, "go:m/list": WitnessImportGraphReader, "driver:make:Makefile#build": WitnessDependency} {
		if got[id].Kind != kind || got[id].DirtyPath != "lib/z.go" {
			t.Errorf("%s = %+v; want %s from lib/z.go", id, got[id], kind)
		}
	}
}

// AFP-V0-3322: a direct owner witness must not mask an independent self-read
// in a package whose only tests are its callers, including binary runners.
func TestCommentOnlyTestlessSelfReader_AFP_V0_3322(t *testing.T) {
	for _, kind := range []string{"literal", "glob", "none"} {
		t.Run(kind, func(t *testing.T) {
			lib := Unit{ID: "go:m/lib", Sources: []string{"lib/a.go"}}
			switch kind {
			case "literal":
				lib.PathTokens = []string{"lib/a.go"}
			case "glob":
				lib.PathGlobs = []string{"lib/*.go"}
			}
			graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
				lib,
				{ID: "go:m/user", Tests: []string{"user/user_test.go"}, Imports: []string{"go:m/lib"}},
				{ID: "go:m/tool", Sources: []string{"tool/main.go"}, Imports: []string{"go:m/lib"}},
				{ID: "go:m/runner", Tests: []string{"runner/runner_test.go"}, Execs: []string{"go:m/tool"}, TestOnlyExecs: []string{"go:m/tool"}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			graph = graph.WithGoChanges([]GoChange{{Path: "lib/a.go", Kind: GoCommentOnly}})
			got := witnesses(Select(graph, []string{"lib/a.go"}))
			for _, id := range []string{"go:m/user", "go:m/runner"} {
				witness, selected := got[id]
				if selected != (kind != "none") || selected && witness.DirtyPath != "lib/a.go" {
					t.Errorf("%s: selected=%v witness=%+v", id, selected, witness)
				}
			}
		})
	}
}

// AFP-V0-3324: a .go file a //go:embed may consume is never narrowed, whether
// a unit's EmbedFiles names it or it lies below an embedding package.
func TestCommentOnlyEmbeddedSource_AFP_V0_3324(t *testing.T) {
	for _, path := range []string{"lib/a.go", "lib/b.go"} {
		t.Run(path, func(t *testing.T) {
			graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
				{ID: "go:m/lib", Sources: []string{"lib/a.go", "lib/b.go"}, Embeds: true, EmbedFiles: []string{"lib/a.go"}},
				{ID: "go:m/user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:m/lib"}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph.WithGoChanges([]GoChange{{Path: path, Kind: GoCommentOnly}}), []string{path})
			if got := witnesses(plan)["go:m/user"]; got.Kind != WitnessDependency || len(plan.GoChanges) != 0 {
				t.Fatalf("embedded source narrowed: user=%+v goChanges=%+v", got, plan.GoChanges)
			}
		})
	}
}

// AFP-V0-3325: a unit whose only dirty inputs are comment-only compiles
// identically, so it reports no coverage gap; a code edit to it still does.
func TestCommentOnlyCoverageGap_AFP_V0_3325(t *testing.T) {
	graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
		{ID: "go:m/lib", Sources: []string{"lib/a.go"}},
		{ID: "go:m/user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:m/lib"}},
		{ID: "go:m/solo", Sources: []string{"solo/a.go", "solo/b.go"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	gaps := func(plan Plan) []string {
		var units []string
		for _, entry := range append(slices.Clone(plan.Unknown), plan.Uncovered...) {
			if entry.Reason == UnknownNoSelectableTest {
				units = append(units, entry.Detail)
			}
		}
		return units
	}
	classified := graph.WithGoChanges([]GoChange{{Path: "lib/a.go", Kind: GoCommentOnly}, {Path: "solo/a.go", Kind: GoCommentOnly}})
	for _, dirty := range [][]string{{"lib/a.go"}, {"solo/a.go"}} {
		if plan := Select(classified, dirty); len(gaps(plan)) != 0 || plan.Scope != ScopeBounded {
			t.Errorf("%v: comment-only gap=%v scope=%s unknown=%+v", dirty, gaps(plan), plan.Scope, plan.Unknown)
		}
	}
	if got := gaps(Select(classified, []string{"solo/a.go", "solo/b.go"})); !slices.Equal(got, []string{"go:m/solo"}) {
		t.Errorf("code edit beside a comment-only file lost its coverage gap: %v", got)
	}
}

// AFP-V0-3322: a test-less package whose go list may observe source comments
// is checked only by its callers' tests, so a comment-only edit still selects
// its importers and the runners of a binary built from it.
func TestCommentOnlyTestlessSourceReaderCallers_AFP_V0_3322(t *testing.T) {
	graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
		{ID: "go:m/lib", Sources: []string{"lib/a.go"}, Tests: []string{"lib/lib_test.go"}},
		{ID: "go:m/docs", Sources: []string{"docs/docs.go"}, ImportGraphReadsAll: true, ImportGraphReadsFiles: true},
		{ID: "go:m/user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:m/docs"}},
		{ID: "go:m/tool", Sources: []string{"tool/main.go"}, Imports: []string{"go:m/docs"}},
		{ID: "go:m/runner", Tests: []string{"runner/runner_test.go"}, Execs: []string{"go:m/tool"}, TestOnlyExecs: []string{"go:m/tool"}},
		{ID: "go:m/other", Sources: []string{"other/other.go"}, Tests: []string{"other/other_test.go"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := witnesses(Select(graph.WithGoChanges([]GoChange{{Path: "lib/a.go", Kind: GoCommentOnly}}), []string{"lib/a.go"}))
	for _, id := range []string{"go:m/user", "go:m/runner"} {
		if witness, selected := got[id]; !selected || witness.DirtyPath != "lib/a.go" {
			t.Errorf("%s: selected=%v witness=%+v", id, selected, witness)
		}
	}
	if _, selected := got["go:m/other"]; selected {
		t.Errorf("unrelated package selected: %+v", got)
	}
}
