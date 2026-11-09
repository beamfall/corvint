// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	json "encoding/json/v2"
	"reflect"
	"slices"
	"testing"
)

// AFP-V0-3350, AFP-V0-3351: the raw-byte reader guard retains the embedding
// tests, including deleted targets, without rebuilding importers or binaries.
func TestTestEmbedReadersStayTestScoped_AFPV03350(t *testing.T) {
	graph := mustBuild(t, fake{name: "go", units: []Unit{
		{ID: "go:a", Sources: []string{"a/a.go"}, Tests: []string{"a/a_test.go"}, TestEmbeds: true, TestEmbedFiles: []string{"a/sub/x.go"}},
		{ID: "go:sub", Sources: []string{"a/sub/x.go"}},
		{ID: "go:user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:a"}},
		{ID: "go:runner", Sources: []string{"runner/runner.go"}, Tests: []string{"runner/runner_test.go"}, Execs: []string{"go:a"}},
	}})
	for _, dirty := range []string{"a/sub/x.go", "a/sub/deleted.go"} {
		plan := Select(graph, []string{dirty})
		if got := plan.SelectedTests(); !slices.Equal(got, []string{"a/a_test.go"}) {
			t.Fatalf("%s selected %v, want only embedding tests", dirty, got)
		}
		readers := graph.Readers([]string{dirty})
		if _, ok := readers["go:a"]; !ok || len(readers) != 1 {
			t.Fatalf("%s byte-reader guard = %v", dirty, readers)
		}
	}
}

// AFP-V0-3352: both test-embed facts are digest inputs, paths are validated,
// and serialization preserves them without setting production facts.
func TestTestEmbedFactsAreDigestedValidatedAndCached_AFPV03352(t *testing.T) {
	unit := Unit{ID: "go:cat", Sources: []string{"cat/cat.go"}, Tests: []string{"cat/cat_test.go"}}
	build := func(unit Unit) (*Graph, error) {
		return Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{unit}})
	}
	previous := mustBuild(t, fake{name: "go", units: []Unit{unit}}).Digest()
	unit.TestEmbeds = true
	for _, paths := range [][]string{nil, {"cat/data.json"}, {"cat/other.json"}} {
		unit.TestEmbedFiles = paths
		graph, err := build(unit)
		if err != nil {
			t.Fatal(err)
		}
		if graph.Digest() == previous {
			t.Fatal("test embed fact did not change graph digest")
		}
		previous = graph.Digest()
		encoded, err := json.Marshal(cachedUnit(unit), json.FormatNilSliceAsNull(true))
		if err != nil {
			t.Fatal(err)
		}
		var restored cachedUnit
		if err := json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(Unit(restored), unit) {
			t.Fatalf("cache round trip %+v, want %+v", Unit(restored), unit)
		}
	}
	unit.TestEmbedFiles = []string{"../escape.json"}
	if _, err := build(unit); err == nil {
		t.Fatal("invalid test embed path must fail Build")
	}
}

// AFP-V0-3351: V1-5176's WithGoChanges guard checks this exact index before
// accepting COMMENT_ONLY. Test embeds must be present even when the source
// owner is another package, with no production embed fact on either unit.
func TestCommentOnlyTestEmbedIndex_AFPV03351(t *testing.T) {
	graph := mustBuild(t, fake{name: "go", units: []Unit{
		{ID: "go:a", Sources: []string{"a/a.go"}, Tests: []string{"a/a_test.go"}, TestEmbeds: true, TestEmbedFiles: []string{"a/sub/x.go"}},
		{ID: "go:sub", Sources: []string{"a/sub/x.go", "a/sub/y.go"}},
	}})
	if got := graph.embedded["a/sub/x.go"]; !slices.Equal(got, []string{"go:a"}) {
		t.Fatalf("comment-only guard cannot see test embed: %v", got)
	}
	if got := graph.embedded["a/sub/y.go"]; len(got) != 0 {
		t.Fatalf("resolved embed index claimed an unmatched path: %v", got)
	}
}
