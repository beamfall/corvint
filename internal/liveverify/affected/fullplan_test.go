// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TestFullplanReplay_AZU_V0_9920_9921 ships in fullplan_replay_test.go with
// the .github/cishards/test-commands.json Python runner it reads
// (claude/ci-1107x, batch-1107x).

// Declared source-shaped output still selects its readers, while a real
// indexed source covered by the declaration keeps its source owner.
func TestFullplanEvidenceReaders_AZU_V0_9921(t *testing.T) {
	root := t.TempDir()
	writeOwnershipFixture(t, root, map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27.3\n",
		affected.NonSourcePathsPath: `{"profile":"corvint-non-source-paths/0","entries":[{"path":".agent-evidence/","reason":"retained output"},{"path":"live/","reason":"output alongside indexed source"}]}`,
		".agent-evidence/env.sh":    "export EXAMPLE=1\n",
		"literal/literal_test.go":   "package literal\nimport \"testing\"\nvar Path = \".agent-evidence/env.sh\"\nfunc TestLiteral(t *testing.T) {}\n",
		"scoped/scoped_test.go":     "package scoped\nimport \"testing\"\nfunc TestScoped(t *testing.T) {}\n",
		affected.ReadScopesPath:     `{"profile":"corvint-test-read-scopes/0","packages":{"scoped":[".agent-evidence/"]}}`,
		"live/live.go":              "package live\n",
		"live/live_test.go":         "package live\nimport \"testing\"\nfunc TestLive(t *testing.T) {}\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), shell.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{".agent-evidence/env.sh"})
	for _, test := range []string{"literal/literal_test.go", "scoped/scoped_test.go"} {
		if !slices.Contains(plan.SelectedTests(), test) {
			t.Fatalf("lost reader %s: %+v", test, plan)
		}
	}
	if len(plan.NonSource) != 1 || plan.NonSource[0].Class != affected.ClassDeclaredNonSource {
		t.Fatalf("output classification: %+v", plan)
	}
	plan = affected.Select(graph, []string{"live/live.go"})
	if len(plan.NonSource) != 0 || !slices.Contains(plan.SelectedTests(), "live/live_test.go") {
		t.Fatalf("indexed source lost ownership: %+v", plan)
	}
}
