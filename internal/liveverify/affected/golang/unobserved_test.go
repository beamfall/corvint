package golang_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestNestedModuleChangeIsANestedModuleUnknown_AFPV0191 pins V1-0922's chosen
// behaviour: a change inside an unlisted nested module, whose files are never
// units, reports a LANGUAGE_FRONTIER naming that module directory instead of
// UNINDEXED_SOURCE_PATH, and the plan stays UNKNOWN. A new file of the
// observed root module is still UNINDEXED_SOURCE_PATH.
func TestNestedModuleChangeIsANestedModuleUnknown_AFPV0191(t *testing.T) {
	root := nestedFixture(t, "module example.test/nested\n\ngo 1.26\n", map[string]string{
		"sibling/deep/go.mod": "module example.test/deep\n\ngo 1.26\n",
		"sibling/deep/d.go":   "package deep\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"nested", "sibling", "sibling/deep"}; !reflect.DeepEqual(result.Unobserved, want) {
		t.Fatalf("unobserved = %v, want %v", result.Unobserved, want)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for dirty, want := range map[string]affected.Unknown{
		"nested/n.go":       {Reason: affected.UnknownLanguageFrontier, Detail: "go:unobserved-module:nested:nested/n.go"},
		"nested/new.go":     {Reason: affected.UnknownLanguageFrontier, Detail: "go:unobserved-module:nested:nested/new.go"},
		"sibling/deep/d.go": {Reason: affected.UnknownLanguageFrontier, Detail: "go:unobserved-module:sibling/deep:sibling/deep/d.go"},
		"a/new.go":          {Reason: affected.UnknownUnindexedSourcePath, Detail: "a/new.go"},
	} {
		plan := affected.Select(graph, []string{dirty})
		if plan.Scope != affected.ScopeUnknown || !reflect.DeepEqual(plan.Unknown, []affected.Unknown{want}) {
			t.Errorf("%s: scope = %s unknown = %+v, want UNKNOWN with %+v", dirty, plan.Scope, plan.Unknown, want)
		}
	}
}

// TestUnobservedLeavesOutDirectoriesHoldingObservedModules_AFPV0191 pins that
// the root, and a nested module directory that contains an observed workspace
// module, are never unobserved: an unindexed file there may belong to an
// observed module, so it stays UNINDEXED_SOURCE_PATH.
func TestUnobservedLeavesOutDirectoriesHoldingObservedModules_AFPV0191(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.work":               "go 1.26\n\nuse ./outer/inner\n",
		"go.mod":                "module example.test/root\n\ngo 1.26\n",
		"r.go":                  "package root\n",
		"outer/go.mod":          "module example.test/outer\n\ngo 1.26\n",
		"outer/o.go":            "package outer\n",
		"outer/inner/go.mod":    "module example.test/inner\n\ngo 1.26\n",
		"outer/inner/i.go":      "package inner\n",
		"outer/inner/i_test.go": "package inner\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Unobserved) != 0 {
		t.Fatalf("unobserved = %v, want none", result.Unobserved)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"outer/inner/new.go"})
	if want := []affected.Unknown{{Reason: affected.UnknownUnindexedSourcePath, Detail: "outer/inner/new.go"}}; !reflect.DeepEqual(plan.Unknown, want) {
		t.Fatalf("unknown = %+v, want %+v", plan.Unknown, want)
	}
}
