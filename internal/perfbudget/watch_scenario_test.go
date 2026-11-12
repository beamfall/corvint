// SPDX-License-Identifier: Apache-2.0
package perfbudget

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// PRB-V0-1014: the two watch scenarios end the set with enforced ceilings and one advisory
// p95 target; the fixture edit the first answers for is written and restored cleanly; the
// records-free self index decodes for the repository's HEAD; and a self root that cannot be
// resolved records the scenario's error instead of measuring something else.
func TestPRBV01014WatchScenarios(t *testing.T) {
	if !slices.Equal(ScenarioIDs[9:], []string{"watch-replan", "watch-replan-self"}) {
		t.Fatalf("scenario order %v", ScenarioIDs)
	}
	b, err := LoadBudgets("budgets.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range b.Scenarios[10:] {
		if len(s.Targets) != 1 || s.Targets[0].Status != StatusAdvisory || s.Targets[0].Limit != 1000 || !strings.Contains(s.Targets[0].Reason, "PRB-V0-1014") {
			t.Errorf("%s targets %+v", s.ID, s.Targets)
		}
		for _, c := range s.Ceilings {
			if !strings.Contains(c.Reason, "PRB-V0-1014") {
				t.Errorf("%s ceiling %s reason %q names no requirement", s.ID, c.Metric, c.Reason)
			}
		}
	}
	if testing.Short() {
		t.Skip("generates a Git repository")
	}
	ctx := context.Background()
	code := filepath.Join(t.TempDir(), "code-app")
	if err := generateCode(ctx, code, Smoke); err != nil {
		t.Fatal(err)
	}
	f := &Fixture{Code: code, GoPath: codeGoPath}
	if err := writeCodeEdit(f); err != nil {
		t.Fatal(err)
	}
	status, err := gitOutput(ctx, code, "status", "--porcelain")
	if err != nil || strings.TrimSpace(status) != "M "+codeGoPath {
		t.Fatalf("edit status %q (%v)", status, err)
	}
	if err := restoreCodeEdit(ctx, f); err != nil {
		t.Fatal(err)
	}
	if status, err = gitOutput(ctx, code, "status", "--porcelain"); err != nil || status != "" {
		t.Fatalf("restore left %q (%v)", status, err)
	}
	index := filepath.Join(t.TempDir(), "self.idx")
	if err := recordsFreeIndex(ctx, code, index); err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Open(index)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	head, _ := gitOutput(ctx, code, "rev-parse", "HEAD")
	if ix.Commit != head || len(ix.Files) != 0 {
		t.Fatalf("records-free index commit %s files %d, want %s and none", ix.Commit, len(ix.Files), head)
	}
	t.Chdir(t.TempDir())
	m, err := measureSelfWatch(ctx, "watch-replan-self", Options{Corvint: "corvint"}, 1)
	if err != nil || !strings.Contains(m.Error, "self root") || len(m.Samples) != 0 {
		t.Fatalf("unresolvable self root: %+v (%v)", m, err)
	}
	for _, arg := range m.Command {
		if filepath.IsAbs(arg) {
			t.Fatalf("self command carries a host path: %v", m.Command)
		}
	}
	if _, err := os.Stat(code); err != nil {
		t.Fatal(err)
	}
}
