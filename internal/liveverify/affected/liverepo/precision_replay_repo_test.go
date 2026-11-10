// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/testsupport"
)

func affprecFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func affprecGraph(t *testing.T, root string, plugins ...affected.Language) *affected.Graph {
	t.Helper()
	g, err := affected.Build(context.Background(), root, plugins...)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func affprecIDs(p affected.Plan) []string {
	var ids []string
	for _, s := range p.Selected {
		ids = append(ids, s.UnitID)
	}
	slices.Sort(ids)
	return ids
}
func affprecRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AFP-V0-9003 / V1-4497 / V1-4143 / V1-4535: replay the reported change kinds
// against the live repository graph. Counts and residual frontiers are measured,
// never erased. The fixture pairs each change kind with its true reader.
func TestPrecisionReplay_AFPV09003(t *testing.T) {
	testsupport.RequireLiveTier(t)
	g := affprecGraph(t, repositoryRoot(t), languages.All()...)
	sets := map[string][]string{
		"heavy-only":       {"script/heavy.sh"},
		"test-only":        {"internal/cliapp/help_test.go", "internal/versionline/versionline_test.go", "internal/liveverify/affected/graphcache_test.go"},
		"declaration-spec": {".corvint/shell-foreign-callers.json", ".corvint/test-read-scopes.json", "docs/specs/affected-plan-v0.md"},
	}
	for name, paths := range sets {
		p := affected.Select(g, paths)
		languageUnits, floorUnits, makeTargets, ciJobs := 0, 0, 0, 0
		for _, s := range p.Selected {
			switch s.Kind {
			case affected.SelectionKindDriverTarget:
				makeTargets++
			case affected.SelectionKindCIJob:
				ciJobs++
			default:
				languageUnits++
				if s.Witness.Kind == affected.WitnessUnboundedReader {
					floorUnits++
				}
			}
		}
		t.Logf("%s selected=%d language=%d make=%d ci=%d scope=%s unknown=%+v", name, len(p.Selected), languageUnits, makeTargets, ciJobs, p.Scope, p.Unknown)
		if name == "heavy-only" {
			// The historical 86-unit report predates the driver claimant.
			// Keep its language-unit bound on the full integrated graph;
			// AZU-V0-001's required drivers are additional selections. The
			// bound counts the units beyond the rule (d) floor, which every
			// change selects and the AFP-V0-025 ratchet
			// (.corvint/unbounded-readers.json) holds unit by unit: AFP-V0-2991
			// raised that floor when rule (d) began to follow execution edges.
			// The selbatch merge (2026-11-06) measured 80 before and 95 after
			// in total: AFP-V0-2203 round 8 retains 14 Go path-literal readers
			// whose lone tokens placement used to drop (`"script"` names the
			// script/ directory at any depth), and AZU-V0-9920 indexes
			// .github/cishards, which executes the built binary.
			// The batch-1107x integration of selbatch, mbrescue, mbapproved,
			// selgo and unbrd measured 123 language units, 94 beyond a 29-unit
			// floor (unbrd alone: 105 units, inside the earlier bound of 86):
			// each lane's new packages and readers of script/ add units.
			if beyond := languageUnits - floorUnits; beyond >= 100 {
				t.Errorf("heavy-only selected %d language units beyond the %d-unit rule (d) floor, want below 100 (94 measured at batch-1107x)", beyond, floorUnits)
			}
			selected := affprecIDs(p)
			reached := g.ReachedUnitIDs(paths)
			requiredDrivers := 0
			for _, id := range g.UnitIDs() {
				unit, _ := g.Unit(id)
				if !strings.HasPrefix(id, affected.DriverNamespace+":") ||
					(!slices.Contains(unit.PathTokens, "script/heavy.sh") && !slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation)) {
					continue
				}
				// Pattern rules are reached templates, not runnable targets.
				_, target, _ := strings.Cut(id, "#")
				if target == "*" || strings.Contains(target, "%") {
					if !slices.Contains(reached, id) {
						t.Errorf("heavy-only did not reach driver template %s", id)
					}
					continue
				}
				requiredDrivers++
				if !slices.Contains(selected, id) {
					t.Errorf("heavy-only omitted required driver %s", id)
				}
			}
			if requiredDrivers == 0 || makeTargets == 0 || ciJobs == 0 {
				t.Errorf("heavy-only lost driver coverage: required=%d make=%d ci=%d", requiredDrivers, makeTargets, ciJobs)
			}
		}
	}
	files := map[string]string{
		"go.mod":                 "module example.test/m\n",
		"testreader/r_test.go":   "package testreader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../changed/c_test.go\")}\n",
		"changed/c_test.go":      "package changed\n",
		"scriptreader/r_test.go": "package scriptreader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../script/heavy.sh\")}\n",
		"datareader/r_test.go":   "package datareader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../.corvint/data.json\")}\n",
		"specreader/r_test.go":   "package specreader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../docs/specs/example.md\")}\n",
	}
	fixture := affprecGraph(t, affprecFiles(t, files), golang.New())
	for path, id := range map[string]string{"changed/c_test.go": "testreader", "script/heavy.sh": "scriptreader", ".corvint/data.json": "datareader", "docs/specs/example.md": "specreader"} {
		if !slices.Contains(affprecIDs(affected.Select(fixture, []string{path})), "go:example.test/m/"+id) {
			t.Errorf("true reader %s of %s omitted", id, path)
		}
	}
}
