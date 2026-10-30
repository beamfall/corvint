package golang_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// A `_test.go` file moved from one package to a new one is absent from the
// checkout at its base path. Its base package, the package still observed in
// that directory, is selected by rule (a) and the path raises no
// UNINDEXED_SOURCE_PATH (AFP-V0-620, V1-2973). A test path present on disk
// but unindexed, or a deleted one whose directory holds no package, keeps the
// entry because its package cannot be named.
func TestMovedGoTestIsAttributedToItsBasePackage_AFPV0620(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                          "module example.test/m\n",
		"provider/provider.go":            "package provider\n\nfunc Value() int { return 1 }\n",
		"provider/provider_test.go":       "package provider\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"provider/interlock/lock.go":      "package interlock\n",
		"provider/interlock/lock_test.go": "package interlock\n\nimport \"testing\"\n\nfunc TestLock(t *testing.T) {}\n",
		"other/other.go":                  "package other\n",
		"other/other_test.go":             "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {}\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	// provider/interlock_test.go moved to provider/interlock/lock_test.go.
	plan := affected.Select(graph, []string{"provider/interlock/lock.go", "provider/interlock/lock_test.go", "provider/interlock_test.go"})
	if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("scope=%s unknown=%v, want BOUNDED with none", plan.Scope, plan.Unknown)
	}
	if got, want := fmt.Sprint(plan.SelectedTests()), "[provider/interlock/lock_test.go provider/provider_test.go]"; got != want {
		t.Fatalf("selected tests = %s, want %s (the moved test's base package and its new one)", got, want)
	}
	for _, selection := range plan.Selected {
		if selection.UnitID == "go:example.test/m/provider" && selection.Witness.DirtyPath != "provider/interlock_test.go" {
			t.Fatalf("base package witness = %+v", selection.Witness)
		}
	}

	// A test file on disk that the graph does not index is new or excluded,
	// not deleted.
	writeFiles(t, root, map[string]string{"provider/new_test.go": "package provider\n"})
	plan = affected.Select(graph, []string{"provider/new_test.go"})
	if want := []affected.Unknown{{Reason: affected.UnknownUnindexedSourcePath, Detail: "provider/new_test.go"}}; !reflect.DeepEqual(plan.Unknown, want) {
		t.Fatalf("present unindexed test unknown=%v, want %v", plan.Unknown, want)
	}
	// A deleted test file whose directory holds no observed package cannot be
	// attributed to a base package.
	plan = affected.Select(graph, []string{"gone/gone_test.go"})
	if want := []affected.Unknown{{Reason: affected.UnknownUnindexedSourcePath, Detail: "gone/gone_test.go"}}; !reflect.DeepEqual(plan.Unknown, want) {
		t.Fatalf("packageless deleted test unknown=%v, want %v", plan.Unknown, want)
	}
	// A deleted non-test Go file keeps the entry: only a test file is confined
	// to its own package's test binary.
	plan = affected.Select(graph, []string{"provider/removed.go"})
	if want := []affected.Unknown{{Reason: affected.UnknownUnindexedSourcePath, Detail: "provider/removed.go"}}; !reflect.DeepEqual(plan.Unknown, want) {
		t.Fatalf("deleted source unknown=%v, want %v", plan.Unknown, want)
	}
}
