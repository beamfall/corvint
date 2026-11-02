package golang_test

import (
	"context"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// selectedIDs is the sorted ID list of a plan's selected units.
func selectedIDs(plan affected.Plan) []string {
	ids := make([]string, 0, len(plan.Selected))
	for _, selection := range plan.Selected {
		ids = append(ids, selection.UnitID)
	}
	slices.Sort(ids)
	return ids
}

// TestIndependentNestedModuleIsARunnableModuleFrontier_AFPV0650: a nested
// module no observed module builds is reported runnable, a dirty path inside
// it (owned or not) is the unobserved-module frontier naming the module, the
// root selection of the same change set is unchanged, and the runnable list
// survives the graph cache round trip.
func TestIndependentNestedModuleIsARunnableModuleFrontier_AFPV0650(t *testing.T) {
	root := nestedFixture(t, "module example.test/nested\n\ngo 1.26\n", map[string]string{
		"nested/README.md": "nested\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"nested", "sibling"}; !reflect.DeepEqual(result.Runnable, want) {
		t.Fatalf("runnable = %v, want %v", result.Runnable, want)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		// The graph cache pins change times only here (AFP-V0-392); a
		// restored graph carries the runnable list exactly.
		saved := affected.GraphCacheRacyWindow
		affected.GraphCacheRacyWindow = 0
		t.Cleanup(func() { affected.GraphCacheRacyWindow = saved })
		// A directory BuildCached creates is 0700 under any umask; t.TempDir is
		// 0777 &^ umask, group-writable under 0002, which the cache refuses (V1-3521).
		cacheDir := filepath.Join(t.TempDir(), "graph-cache")
		if _, outcome, err := affected.BuildCached(context.Background(), root, cacheDir, "test-identity", golang.New()); err != nil || !outcome.Stored {
			t.Fatalf("first build: %+v %v", outcome, err)
		}
		cached, outcome, err := affected.BuildCached(context.Background(), root, cacheDir, "test-identity", golang.New())
		if err != nil || !outcome.Hit {
			t.Fatalf("second build missed the cache: %+v %v", outcome, err)
		}
		if cached.Digest() != graph.Digest() {
			t.Fatal("the cached graph lost the runnable list")
		}
		graph = cached
	}
	rootOnly := affected.Select(graph, []string{"a/a.go"})
	for _, dirty := range []string{"nested/n.go", "nested/README.md", "nested/new_test.go"} {
		want := []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: "go:unobserved-module:nested:" + dirty}}
		if dirty == "nested/README.md" {
			// A path no plugin owns names the module and keeps its own
			// fail-closed UNOWNED_DIRTY_PATH: the module root does not
			// prove nothing else reads it.
			want = append(want, affected.Unknown{Reason: affected.UnknownUnownedDirtyPath, Detail: dirty})
		}
		plan := affected.Select(graph, []string{"a/a.go", dirty})
		if plan.Scope != affected.ScopeUnknown || !reflect.DeepEqual(plan.Unknown, want) {
			t.Errorf("%s: scope = %s unknown = %+v, want UNKNOWN with %+v", dirty, plan.Scope, plan.Unknown, want)
		}
		if got, want := selectedIDs(plan), selectedIDs(rootOnly); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: root selection = %v, want %v", dirty, got, want)
		}
	}
}

// TestNestedModuleTheRootBuildsOrCannotReadIsNotRunnable_AFPV0650: a nested
// module the root requires or replaces, or whose go.mod cannot be parsed, is
// not runnable, so a change inside it keeps the AFP-V0-191
// unobserved-directory frontier naming the module.
func TestNestedModuleTheRootBuildsOrCannotReadIsNotRunnable_AFPV0650(t *testing.T) {
	head := "module example.test/nested\n\ngo 1.26\n"
	rootHead := "module example.test/root\n\ngo 1.26\n\n"
	for _, testCase := range []struct {
		name     string
		manifest string
		extra    map[string]string
	}{
		{"root requires nested", head, map[string]string{"go.mod": rootHead + "require example.test/nested v0.0.0\n\nreplace example.test/nested => ./nested\n"}},
		{"root replaces nested", head, map[string]string{"go.mod": rootHead + "replace example.test/nested v0.0.0 => example.test/fork v0.0.1\n"}},
		{"unparsable manifest", "module example.test/nested\n\nrequires example.test/root v0.0.0\n", nil},
		{"nested workspace", head, map[string]string{"nested/go.work": "go 1.26\n\nuse (\n\t.\n\t..\n)\n"}},
		// Independent review, 2026-10-28: sibling compiles nested's files
		// through its directory replace, so nested's own tests alone do not
		// cover a change to them.
		{"sibling replaces nested by directory", head, map[string]string{"sibling/go.mod": "module example.test/sibling\n\ngo 1.26\n\nrequire example.test/nested v0.0.0\n\nreplace example.test/nested => ../nested\n"}},
		{"nested manifest replaces outside the repository", head, map[string]string{"sibling/go.mod": "module example.test/sibling\n\ngo 1.26\n\nreplace example.test/other => ../../other\n"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := nestedFixture(t, testCase.manifest, testCase.extra)
			result, err := golang.New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if slices.Contains(result.Runnable, "nested") {
				t.Fatalf("runnable = %v, want nested left out", result.Runnable)
			}
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			// The plugin-wide nested-module frontier stays as well; the
			// change itself is named by the module's directory frontier.
			want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: "go:unobserved-directory:nested:nested/n.go"}
			plan := affected.Select(graph, []string{"nested/n.go"})
			if plan.Scope != affected.ScopeUnknown || !slices.Contains(plan.Unknown, want) {
				t.Errorf("scope = %s unknown = %+v, want UNKNOWN with %+v", plan.Scope, plan.Unknown, want)
			}
			for _, entry := range plan.Unknown {
				if strings.Contains(entry.Detail, ":"+affected.FrontierUnobservedModule+":") {
					t.Errorf("unknown %+v names a runnable module", entry)
				}
			}
		})
	}
}
