package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// This is a source-graph qualification corpus, not a recorded consumer run.
// Nine independent feature cohorts each have thirteen files and four cases.
// The oracle uses cohort membership, never the adapter's graph or exclusions.
func qualificationCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", qualificationConfig)
	write(t, root, "support/shared.ts", `export { test } from "@playwright/test"`)
	write(t, root, "setup/global.setup.ts", `import { test } from "@playwright/test"; test("setup", () => {})`)
	write(t, root, "setup/global.teardown.ts", `import { test } from "@playwright/test"; test("cleanup", () => {})`)
	for cohort := range 9 {
		write(t, root, fmt.Sprintf("support/helper%d.ts", cohort), `export const value = 1`)
		write(t, root, fmt.Sprintf("support/page%d.ts", cohort), fmt.Sprintf(`export { value } from "./helper%d"`, cohort))
		write(t, root, fmt.Sprintf("support/scenario%d.ts", cohort), fmt.Sprintf(`export { value } from "./page%d"`, cohort))
	}
	for file := range 117 {
		body := fmt.Sprintf("import { test } from '../support/shared'\nimport { value } from '../support/scenario%d'\n", file/13)
		for caseIndex, tags := range []string{"@angular @react", "@angular", "@react", "@chromium"} {
			body += fmt.Sprintf("test('%s case%d', () => { if (value !== 1) throw new Error('wrong') })\n", tags, caseIndex)
		}
		write(t, root, fmt.Sprintf("tests/case%03d.spec.ts", file), body)
	}
	return root
}

const qualificationConfig = `import { defineConfig, devices } from "@playwright/test"
export default defineConfig({ projects: [
  { name: "setup", testDir: "setup", testMatch: "**/*.setup.ts", teardown: "cleanup" },
  { name: "cleanup", testDir: "setup", testMatch: "**/*.teardown.ts" },
  { name: "chromium", testDir: "tests", testMatch: "**/*.spec.ts", dependencies: ["setup"], use: { ...devices["Desktop Chrome"] } },
  { name: "angular", testDir: "tests", testMatch: "**/*.spec.ts", dependencies: ["setup"], grep: /@angular/, metadata: { framework: "angular" }, use: { browserName: "chromium" } },
  { name: "react", testDir: "tests", testMatch: "**/*.spec.ts", dependencies: ["setup"], grep: /@react/, metadata: { framework: "react" }, use: { browserName: "chromium" } }
] })`

func qualificationOracle(first, count int) []string {
	ids := []string{"typescript:playwright:setup:setup/global.setup.ts", "typescript:playwright:cleanup:setup/global.teardown.ts"}
	for file := first; file < first+count; file++ {
		for _, project := range []string{"angular", "chromium", "react"} {
			ids = append(ids, fmt.Sprintf("typescript:playwright:%s:tests/case%03d.spec.ts", project, file))
		}
	}
	slices.Sort(ids)
	return ids
}

func TestPlaywrightQualification(t *testing.T) {
	root := qualificationCorpus(t)
	qualifyCorpusManifest(t, root)
	t.Run("TJAA-V0-017 independently enumerated zero unsafe narrowing", func(t *testing.T) {
		for _, row := range []struct {
			path         string
			first, count int
		}{
			{"support/page0.ts", 0, 13},
			{"support/scenario4.ts", 52, 13},
			{"support/helper8.ts", 104, 13},
			{"tests/case037.spec.ts", 37, 1},
			{"support/shared.ts", 0, 117},
			{"setup/global.setup.ts", 0, 117},
			{"playwright.config.ts", 0, 117},
		} {
			t.Run(row.path, func(t *testing.T) {
				plan := qualifySelection(t, root, row.path)
				want := qualificationOracle(row.first, row.count)
				if got := playwrightSelectionIDs(plan); !slices.Equal(got, want) {
					t.Fatalf("unsafe/mismatched selection: got %v want %v", got, want)
				}
				if plan.Scope != affected.ScopeBounded || plan.Fallback != PlaywrightFallbackNone {
					t.Fatalf("static corpus not bounded: %+v", plan.Unknown)
				}
				if len(plan.Selected)+len(plan.Excluded) != 353 {
					t.Fatal("project universe differs from independent 117*3+2 oracle")
				}
				t.Logf("required=%d selected=%d unsafe-narrowing=0", len(want), len(plan.Selected))
			})
		}
	})
	t.Run("TJAA-V0-015 unknown frontier cannot exclude required units", func(t *testing.T) {
		for _, row := range []struct {
			name, path, body, reason string
			projectsUnknown          bool
		}{
			{"dynamic-import", "support/page0.ts", `export const value = import(process.env.PAGE)`, FrontierDynamicImport, false},
			{"unknown-membership", "playwright.config.ts", `export default { projects: [{ name: "chromium", testMatch: matcher }, { name: "angular" }, { name: "react" }, { name: "setup" }, { name: "cleanup" }] }`, PlaywrightUnknownProjectMembership, false},
			{"dynamic-projects", "playwright.config.ts", `export default { projects: loadProjects() }`, PlaywrightUnknownProjectSet, true},
		} {
			t.Run(row.name, func(t *testing.T) {
				unknownRoot := qualificationCorpus(t)
				write(t, unknownRoot, row.path, row.body)
				plan := qualifySelection(t, unknownRoot, "support/page0.ts")
				if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Excluded) != 0 || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, row.reason) {
					t.Fatalf("unknown frontier lost: %+v", plan)
				}
				if row.projectsUnknown || row.reason == PlaywrightUnknownProjectMembership {
					if len(plan.Selected) != 0 {
						t.Fatal("unknown project set emitted runnable approximation")
					}
					if len(plan.FallbackArgv) != 4 {
						t.Fatal("complete configuration fallback missing")
					}
					return
				}
				got := playwrightSelectionIDs(plan)
				for _, id := range qualificationOracle(0, 117) {
					if !slices.Contains(got, id) {
						t.Fatalf("unsafe narrowing omitted %s", id)
					}
				}
			})
		}
	})
	t.Run("TJAA-V0-014 provider pass cannot discharge execution frontier", func(t *testing.T) {
		plan := qualifySelection(t, root, "tests/case037.spec.ts")
		before, _ := plan.Canonical()
		// The retained provider has no project field (#19). Exercise one reporter
		// observation without asserting cross-project association or runtime proof.
		outcomes, infrastructure, err := jstestprovider.ParsePlaywrightJSON([]byte(`{"suites":[{"specs":[{"title":"case0","file":"tests/case037.spec.ts","line":3,"tests":[{"status":"expected","results":[{"status":"passed","retry":0}]}]}]}]}`))
		if err != nil || infrastructure != nil || len(outcomes) != 1 {
			t.Fatalf("provider: %v %v %v", outcomes, infrastructure, err)
		}
		projection := jstestprovider.ToTestProjection(outcomes[0])
		if projection.Execution.State != testvalidity.ExecutionPassed || projection.Strength.State != testvalidity.StrengthNotMeasured || projection.Freshness.State != testvalidity.FreshnessUnknown {
			t.Fatalf("green observation invented freshness/strength: %+v", projection)
		}
		after, _ := plan.Canonical()
		if !bytes.Equal(before, after) || !hasPlaywrightUnknown(plan, PlaywrightAxisExecution, PlaywrightUnknownExternalApplication) {
			t.Fatal("provider composition changed selection authority")
		}
		qualifyRetainedProvider(t, root, plan, outcomes)
	})
}

func qualifyCorpusManifest(t *testing.T, root string) {
	t.Helper()
	manifest, err := os.ReadFile("testdata/playwright-qualification.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(manifest)), "\n")
	if len(rows) != 118 || rows[0] != "path\tcohort\tcases" {
		t.Fatal("qualification manifest must contain 117 files")
	}
	files, err := filepath.Glob(filepath.Join(root, "tests", "*.spec.ts"))
	if err != nil || len(files) != 117 {
		t.Fatalf("corpus inventory=%d error=%v", len(files), err)
	}
	seen := map[string]bool{}
	cases := 0
	for _, row := range rows[1:] {
		fields := strings.Split(row, "\t")
		if len(fields) != 3 || seen[fields[0]] {
			t.Fatalf("invalid manifest row %q", row)
		}
		seen[fields[0]] = true
		body, err := os.ReadFile(filepath.Join(root, fields[0]))
		if err != nil {
			t.Fatal(err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil || strings.Count(string(body), "\ntest(") != count {
			t.Fatalf("case inventory mismatch: %s", row)
		}
		if !strings.Contains(string(body), "../support/scenario"+fields[1]+"'") {
			t.Fatalf("cohort mismatch: %s", row)
		}
		cases += count
	}
	if cases != 468 {
		t.Fatalf("cases=%d want=468", cases)
	}
}

func qualifyRetainedProvider(t *testing.T, root string, plan PlaywrightPlan, outcomes []jstestprovider.TestOutcome) {
	t.Helper()
	file := "tests/case037.spec.ts"
	source, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(root, plan.Config.Path))
	if err != nil {
		t.Fatal(err)
	}
	identity := jstestprovider.Identity{TestFileDigests: map[string]string{file: fmt.Sprintf("%x", sha256.Sum256(source))}, ConfigFile: plan.Config.Path, ConfigDigest: plan.Config.SHA256}
	for _, row := range []struct {
		name, freshness, reason  string
		mismatch, stale, missing bool
	}{
		{"matching-e2e", testvalidity.FreshnessUnknown, "retained-app-build-identity-unverifiable", false, false, false},
		{"source-mismatch", testvalidity.FreshnessStale, "retained-digest-mismatch", true, false, false},
		{"stale-build", testvalidity.FreshnessStale, "retained-digest-mismatch", false, true, false},
		{"missing-bound-source", testvalidity.FreshnessUnknown, "retained-bound-source-unreadable", false, false, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			receipt := jstestprovider.Receipt{Kind: "e2e", Identity: identity, Tests: outcomes, StaleAppBuild: row.stale}
			body, err := json.Marshal(map[string]any{"receipt": receipt})
			if err != nil {
				t.Fatal(err)
			}
			input, err := testvaliditydoc.Decode(body)
			if err != nil {
				t.Fatal(err)
			}
			document := testvaliditydoc.ProjectPinned(input, func(path string) ([]byte, bool) {
				if row.missing {
					return nil, false
				}
				if row.mismatch {
					return []byte("changed"), true
				}
				if path == file {
					return source, true
				}
				if path == plan.Config.Path {
					return config, true
				}
				return nil, false
			})
			if document.Run.Freshness.State != row.freshness || document.Run.Freshness.Reason != row.reason {
				t.Fatalf("freshness=%+v", document.Run.Freshness)
			}
			if document.Tests[0].Projection.Strength.State != testvalidity.StrengthNotMeasured {
				t.Fatal("retained evidence invented strength")
			}
		})
	}
	if _, err := testvaliditydoc.Decode([]byte(`{"receipt":{"kind":"e2e"},"profile":"corvint-go-live-session-event/0"}`)); err == nil {
		t.Fatal("ambiguous retained envelope admitted")
	}
}

func qualifySelection(t *testing.T, root, dirty string) PlaywrightPlan {
	t.Helper()
	units := []PlaywrightDiscoveryUnit{}
	for _, id := range qualificationOracle(0, 117) {
		parts := strings.SplitN(strings.TrimPrefix(id, "typescript:playwright:"), ":", 2)
		units = append(units, PlaywrightDiscoveryUnit{Project: parts[0], Test: parts[1]})
	}
	receipt := discoveryFixtureBytes(t, root, units)
	first, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{dirty}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{dirty}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := first.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("fixed corpus produced different canonical bytes")
	}
	return first
}
