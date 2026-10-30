package affected_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// batch1028cUnowned is every UNOWNED_DIRTY_PATH of batch 1028c
// (c6c75440bf2c..5cc0d42e84ff) before AFP-V0-602: eight sealed lane CEMs, the
// read-scope declaration and an embedded catalog.
var batch1028cUnowned = []string{
	".corvint/changes/06cf00ac5b39e1df734ea0fd63dbe42cfa6d078f.cem.json",
	".corvint/changes/3807936c3293f04ffc5c3c9618990757fd3c4174.cem.json",
	".corvint/changes/56acf444fd6a27b31eec10ac5e210472a547a6f2.cem.json",
	".corvint/changes/57f4c2d938fbfa9cdd0f21854703cf312c867da0.cem.json",
	".corvint/changes/65d29e1efd7440bf88c711bdbe46719c82956ab6.cem.json",
	".corvint/changes/89561333fdd4d1fbad9776df5aeacef3978e76f1.cem.json",
	".corvint/changes/8d7b44188cb086aeb5c32182fe4fab18890973d9.cem.json",
	".corvint/changes/e9eec84100408553dc6fde3399aa8e48e0cc36a1.cem.json",
	".corvint/test-read-scopes.json",
	"internal/capcatalog/catalog.json",
}

// batch1028aDeclared is the part of batch 1028a's (73f47a93b3af..f7f73626af4a)
// UNOWNED_DIRTY_PATH set whose test readers the reader rules name: a sealed
// CEM, the two reader declarations, a cmd/corvint golden, the use-case ledger
// and receipts, the host compatibility matrix, embedded catalog files and the
// line-citation inventory.
var batch1028aDeclared = []string{
	".corvint/changes/ebda995079c5c3e402cf94ac0158d01ee15b5d63.cem.json",
	".corvint/test-read-scopes.json",
	".corvint/unbounded-readers.json",
	"cmd/corvint/testdata/flows/gaps.golden.json",
	"conformance/use-cases-v0/ledger.json",
	"conformance/use-cases-v0/receipts/UC-CHANGE-CONSEQUENCE/implementation.json",
	"conformance/use-cases-v0/receipts/UC-EVIDENCE-CARRYING-COMPLETION/implementation.json",
	"conformance/use-cases-v0/receipts/UC-TASK-ORIENTATION/implementation.json",
	"integrations/compatibility.json",
	"internal/capcatalog/catalog.json",
	"internal/capcatalog/trees/corvint-test.json",
	"internal/capcatalog/trees/corvint.json",
	"script/line-citation-unpinned-inventory.txt",
}

// batch1028aUnowned is the rest of that set, which must stay UNKNOWN: CI
// driver inputs, which no declaration may cover, and the opencode package's
// npm manifest and README, which no declaration covers.
var batch1028aUnowned = []string{
	".github/forward-rows/narrow.jq",
	".github/workflows/ci.yml",
	"integrations/opencode/README.md",
	"integrations/opencode/package.json",
}

// docsOnly is a documentation-only change (V1-2726): a spec and a build-log
// entry, both declared documentation (AFP-V0-160).
var docsOnly = []string{
	"docs/build-log/2026-10-25-v1-2708-selection-precision.md",
	"docs/specs/affected-plan-v0.md",
}

// frontierFloor is the LANGUAGE_FRONTIER detail set of a plain Go source edit
// in the package that embeds the catalog: the entries of the Go and shell
// readers that rule (d) selects for every dirty path, which any plan carries.
// Since the shell foreign-caller declaration (TSP-V0-005) the floor is empty.
// The replayed CEM and spec paths are still named by undeclared non-test shell
// scripts (check-line-citations.sh, dogfood-seal.sh and others), path-literal
// readers whose shell:foreign-invocation-unmodelled entry bears; that one
// entry is the measured residual, and no other may appear.
func frontierFloor(t *testing.T, graph *affected.Graph) map[string]bool {
	t.Helper()
	floor := map[string]bool{}
	for _, entry := range affected.Select(graph, []string{"internal/capcatalog/capcatalog.go"}).Unknown {
		if entry.Reason == affected.UnknownLanguageFrontier {
			floor[entry.Detail] = true
		}
	}
	return floor
}

// TestBatchReplayDeclaredRuntimeReaders_V12941 replays the batch path sets
// that kept every CI shard FULL against this repository's graph with every
// plugin (AFP-V0-602; V1-2941, V1-2726), and a documentation-only change.
// Each declared path is DECLARED_RUNTIME_READERS (or DECLARED_DOCUMENTATION)
// and raises no UNOWNED_DIRTY_PATH. No plugin with no selected unit
// contributes a LANGUAGE_FRONTIER entry, so none takes part only because a
// path was unowned; the 1028c and documentation sets add no entry beyond the
// floor every Go source edit carries. The undeclared remainder of batch 1028a
// stays UNOWNED_DIRTY_PATH, path by path.
func TestBatchReplayDeclaredRuntimeReaders_V12941(t *testing.T) {
	graph, err := affected.Build(context.Background(), repositoryRoot(t), languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	floor := frontierFloor(t, graph)
	for name, dirty := range map[string][]string{"1028c": batch1028cUnowned, "1028a": batch1028aDeclared, "docs": docsOnly} {
		plan := affected.Select(graph, dirty)
		class := affected.ClassDeclaredRuntimeReaders
		if name == "docs" {
			class = affected.ClassDeclaredDocumentation
		}
		var declared []string
		for _, group := range plan.NonSource {
			if group.Class == class {
				declared = append(declared, group.Paths...)
			}
		}
		slices.Sort(declared)
		if !slices.Equal(declared, dirty) {
			t.Errorf("%s: %s %v, want %v", name, class, declared, dirty)
		}
		selected := map[string]bool{}
		for _, selection := range plan.Selected {
			plugin, _, _ := strings.Cut(selection.UnitID, ":")
			selected[plugin] = true
		}
		for _, entry := range plan.Unknown {
			switch entry.Reason {
			case affected.UnknownUnownedDirtyPath:
				t.Errorf("%s: UNOWNED_DIRTY_PATH %s", name, entry.Detail)
			case affected.UnknownLanguageFrontier:
				plugin, _, _ := strings.Cut(entry.Detail, ":")
				if !selected[plugin] {
					t.Errorf("%s: LANGUAGE_FRONTIER %s from %s, which has no selected unit", name, entry.Detail, plugin)
				}
				if name != "1028a" && !floor[entry.Detail] && entry.Detail != shell.FrontierForeignInvocation {
					t.Errorf("%s: LANGUAGE_FRONTIER %s beyond a Go source edit's floor", name, entry.Detail)
				}
			default:
				t.Errorf("%s: %s %s", name, entry.Reason, entry.Detail)
			}
		}
	}
	plan := affected.Select(graph, append(slices.Clone(batch1028aDeclared), batch1028aUnowned...))
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("1028a with undeclared paths: scope %s", plan.Scope)
	}
	var unowned []string
	for _, entry := range plan.Unknown {
		if entry.Reason == affected.UnknownUnownedDirtyPath {
			unowned = append(unowned, entry.Detail)
		}
	}
	slices.Sort(unowned)
	if !slices.Equal(unowned, batch1028aUnowned) {
		t.Fatalf("1028a unowned %v, want %v", unowned, batch1028aUnowned)
	}
}
