package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// gh1811Bundle is a generated bundle over affected.MaxSourceBytes, like the reporter's
// public/frontend/admin/main.js.
const gh1811BundlePath = "public/frontend/admin/main.js"

func gh1811Bundle() string {
	return "var x = 1;\n" + strings.Repeat("// pad\n", affected.MaxSourceBytes/7+1)
}

func gh1811Listing(t *testing.T, root string) string {
	t.Helper()
	return e2eCorpusListing(t, root, []string{"cart.spec.ts", "checkout.spec.ts", "profile.spec.ts", "search.spec.ts"})
}

// GitHub #1811 (AFP-V0-444): one over-bound file is a per-path unknown, never a whole-run
// failure. Discovery still binds the sources; e2e-safe still narrows a change that does not touch
// the file and fails closed for one that does.
func TestAffectedOverBoundSourceIsPerPath_GH1811(t *testing.T) {
	corpus := loadE2ECorpus(t)
	for _, touch := range []bool{false, true} {
		name := "untouched"
		if touch {
			name = "touched"
		}
		t.Run(name, func(t *testing.T) {
			c := corpus.find(t, "search-source")
			bundle := gh1811Bundle()
			c.BaseChanges = map[string]*string{gh1811BundlePath: &bundle}
			if touch {
				changed := bundle + "var y = 2;\n"
				c.Changes = map[string]*string{gh1811BundlePath: &changed}
				for path, body := range corpus.find(t, "search-source").Changes {
					c.Changes[path] = body
				}
			}
			root, base, _ := buildE2ECase(t, corpus, c)
			code, produced, stderr := runAffectedDiscoveryProducer(t, root, gh1811Listing(t, root))
			if code != 0 {
				t.Fatalf("discovery aborted on an over-bound file: exit=%d stderr=%s", code, stderr)
			}
			shopWrite(t, root, map[string]string{"discovery.json": string(produced)})
			got := runE2ESelection(t, root, base)
			if got.Discovery.State != "MATCHED" {
				t.Fatalf("discovery receipt did not bind: %+v", got.Discovery)
			}
			if !touch && (got.State != "narrow-selection-allowed" || len(got.OmittedTests) == 0) {
				t.Fatalf("a change that does not touch the over-bound file did not narrow: %+v", got)
			}
			if touch && (got.State != "full-relevant-suite-required" || len(got.OmittedTests) != 0) {
				t.Fatalf("a change touching the over-bound file narrowed: %+v", got)
			}
			receipt, _, stderr, code := runAffectedArguments(t, root, "--base", base)
			if code != 0 {
				t.Fatalf("affected exited %d: %s", code, stderr)
			}
			raw, _ := json.Marshal(receipt)
			if !strings.Contains(string(raw), typescript.FrontierOverBoundSource+":"+gh1811BundlePath) {
				t.Fatalf("plan does not name the over-bound path: %s", raw)
			}
		})
	}
}

// GitHub #1811 (AFP-V0-444): without a declaration the Playwright plan names the bound and the
// remedy, as an informational frontier when the file is outside the closure (GitHub #1878); with .corvint/generated-paths.json the walk excludes the declared path, the plan records
// the declaration, and discovery and e2e-safe still bind and narrow.
func TestAffectedGeneratedPathsDeclaration_GH1811(t *testing.T) {
	corpus := loadE2ECorpus(t)
	for _, declared := range []bool{false, true} {
		name := "undeclared"
		if declared {
			name = "declared"
		}
		t.Run(name, func(t *testing.T) {
			c := corpus.find(t, "search-source")
			bundle := gh1811Bundle()
			c.BaseChanges = map[string]*string{gh1811BundlePath: &bundle}
			if declared {
				declaration := `{"profile":"corvint-generated-paths/0","paths":["public/frontend/"]}`
				c.BaseChanges[affected.GeneratedPathsPath] = &declaration
			}
			shop := "export const origin = \"http://localhost:3002\";\n"
			c.Changes = map[string]*string{"e2e/pages/shop.ts": &shop}
			root, base, _ := buildE2ECase(t, corpus, c)
			receipt, _, stderr, code := runAffectedArguments(t, root, "--base", base, "--playwright-config", "playwright.config.ts")
			if code != 0 {
				t.Fatalf("playwright plan exited %d: %s", code, stderr)
			}
			raw, _ := json.Marshal(receipt["plan"])
			var plan typescript.PlaywrightPlan
			if err := json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			overBound := false
			for _, unknown := range append(slices.Clone(plan.Unknown), plan.Informational...) {
				if unknown.Reason == typescript.FrontierOverBoundSource {
					overBound = true
					if unknown.Axis != typescript.PlaywrightAxisInformational || !strings.Contains(unknown.Detail, gh1811BundlePath) || !strings.Contains(unknown.Detail, "4194304") || !strings.Contains(unknown.Detail, affected.GeneratedPathsPath) {
						t.Fatalf("over-bound frontier is not informational naming the path, the bound and the remedy: %+v", unknown)
					}
				}
			}
			if declared {
				if overBound || plan.Generated == nil || plan.Generated.Path != affected.GeneratedPathsPath || len(plan.Generated.Paths) != 1 || plan.Generated.Paths[0] != "public/frontend/" || len(plan.Generated.SHA256) != 64 {
					t.Fatalf("declared exclusion is not visible in the plan: %s", raw)
				}
				if plan.Narrowed == nil || plan.Narrowed.State != typescript.PlaywrightNarrowed || len(plan.Narrowed.Candidates) == 0 {
					t.Fatalf("declared exclusion did not let the test-side change narrow: %s", raw)
				}
			} else if !overBound || plan.Generated != nil || plan.Narrowed == nil || plan.Narrowed.State != typescript.PlaywrightNarrowed {
				// The bundle sits outside the Playwright closure and no dirty path reaches it, so it
				// cannot change the selection: it is reported, never widening (TJAA-V0-108, GitHub #1878).
				t.Fatalf("undeclared over-bound file outside the closure is not a named informational frontier: %s", raw)
			}
			code, produced, stderr := runAffectedDiscoveryProducer(t, root, gh1811Listing(t, root))
			if code != 0 {
				t.Fatalf("discovery exited %d: %s", code, stderr)
			}
			shopWrite(t, root, map[string]string{"discovery.json": string(produced)})
			if got := runE2ESelection(t, root, base); got.Discovery.State != "MATCHED" {
				t.Fatalf("e2e-safe verifier disagrees with the producer's source binding: %+v", got.Discovery)
			}
		})
	}
}
