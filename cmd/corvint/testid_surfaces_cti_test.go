// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testid"
)

// CTI-V1-009 (Go): one changed function in package p. The plan
// (`affected --summary`, the corvint.affected MCP result), the
// corvint-test-run/0 receipt's testId and `watch` name the same test by the
// same canonical ID: the plan and the receipt name package p, go:p, and the
// watch run narrows to go:p::TestPass, the ID go test -json reports for that
// test, which go:p selects (CTI-V1-008).
func TestCTIV1009GoPlanReceiptAndWatchAgree(t *testing.T) {
	t.Parallel()
	root, index := watchModule(t)
	if err := os.WriteFile(filepath.Join(root, "p", "p.go"), []byte("package p\nfunc F() int {\n return 2\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const unit = "go:example.com/m/p"
	verdict, _, stderr, code := runAffectedCLI(t, root, "--summary")
	if code != 0 {
		t.Fatalf("affected --summary exit %d: %s", code, stderr)
	}
	tiers, _ := verdict["tiers"].(map[string]any)
	ids, _ := tiers["testIds"].(map[string]any)
	planID, _ := ids[unit].(string)
	receiptID := sessiontest.UnitTestID(unit, sessiontest.GoUnitDir(root))
	if planID != "go:p" || receiptID != planID {
		t.Fatalf("plan %q (tiers %v), receipt %q; want go:p for both", planID, tiers, receiptID)
	}
	code, stdout, stderr := watchCLI(t, "", "--root", root, "watch", "--index", index, "--once", "--dry-run", "--packages", "p")
	if code != 0 {
		t.Fatalf("watch exit %d: %s", code, stderr)
	}
	events := watchEvents(t, stdout)
	if len(events) != 1 || events[0].Plan == nil || len(events[0].Plan.Run) != 1 {
		t.Fatalf("watch events %+v", events)
	}
	watchIDs := events[0].Plan.Run[0].TestIDs
	reported, err := testid.GoTestJSON(strings.NewReader(`{"Action":"run","Package":"example.com/m/p","Test":"TestPass"}`+"\n"), testid.GoModuleDir("example.com/m", "."))
	if err != nil || len(reported) != 1 {
		t.Fatalf("report: %v %v", reported, err)
	}
	if len(watchIDs) != 1 || watchIDs[0] != reported[0].ID.String() || !testid.Matches(planID, watchIDs[0]) {
		t.Fatalf("watch %v, report %q, plan %q", watchIDs, reported[0].ID, planID)
	}
}

// CTI-V1-009 (Vitest, Jest, Playwright, RSpec, Minitest, pytest): for each
// runner's unit as the plan selects it, the plan's testIds (the verdict
// `affected --summary` and corvint.affected serve) and the receipt's testId
// are the same file-level ID, and it selects the test the runner's report
// names (CTI-V1-008). corvint watch plans Go indexes only, so these runners
// have no watch output to compare. A pytest unit names a Python module, not
// a file, so neither surface invents an ID for it (CTI-V1-011).
func TestCTIV1009PlanAndReceiptAgreePerRunner(t *testing.T) {
	t.Parallel()
	report := func(t *testing.T, read func() ([]testid.Named, error)) string {
		t.Helper()
		named, err := read()
		if err != nil || len(named) == 0 {
			t.Fatalf("report: %v %v", named, err)
		}
		return named[0].ID.String()
	}
	cases := []struct {
		runner, unit, want string
		test               func(t *testing.T) string
	}{
		{testid.Vitest, "typescript:vitest:src/a.test.ts", "vitest:src/a.test.ts", func(t *testing.T) string {
			return report(t, func() ([]testid.Named, error) {
				return testid.JestJSON(testid.Vitest, strings.NewReader(`{"testResults":[{"name":"/repo/src/a.test.ts","assertionResults":[{"ancestorTitles":["a"],"title":"GET /a:b"}]}]}`), "/repo")
			})
		}},
		{testid.Jest, "typescript:jest:web/sum.test.js", "jest:web/sum.test.js", func(t *testing.T) string {
			return report(t, func() ([]testid.Named, error) {
				return testid.JestJSON(testid.Jest, strings.NewReader(`{"testResults":[{"name":"/repo/web/sum.test.js","assertionResults":[{"ancestorTitles":["sum"],"title":"adds"}]}]}`), "/repo")
			})
		}},
		{testid.Playwright, "typescript:playwright:e2e/login.spec.ts", "playwright:e2e/login.spec.ts", func(t *testing.T) string {
			return report(t, func() ([]testid.Named, error) {
				return testid.PlaywrightJSON(strings.NewReader(`{"config":{"rootDir":"/repo/e2e"},"suites":[{"title":"login.spec.ts","file":"login.spec.ts","specs":[{"title":"signs in","tests":[{"projectName":"chromium"}]}]}]}`), "/repo")
			})
		}},
		{testid.RSpec, "ruby:rspec:spec/user_spec", "rspec:spec/user_spec.rb", func(t *testing.T) string {
			id, err := testid.RSpecExample("./spec/user_spec.rb", []string{"User"}, "saves")
			if err != nil {
				t.Fatal(err)
			}
			return id.String()
		}},
		{testid.Minitest, "ruby:minitest:test/calc_test", "minitest:test/calc_test.rb", func(t *testing.T) string {
			return report(t, func() ([]testid.Named, error) {
				return testid.MinitestJSONL(strings.NewReader(`{"kind":"test","package":"test/calc_test.rb","name":"CalcTest#test_add"}` + "\n"))
			})
		}},
		{testid.Pytest, "python:tests.test_a", "", nil},
	}
	tiers := map[string]int{}
	for _, c := range cases {
		tiers[c.unit] = 0
	}
	verdict := affectedVerdictOf(affectedCompactReceipt{Tiers: &affectedTiers{Units: tiers}}, nil)
	for _, c := range cases {
		t.Run(c.runner, func(t *testing.T) {
			planID := verdict.Tiers.TestIDs[c.unit]
			// The receipt's testId is what Runner.Execute stamps on the
			// unit's outcome (TestSTRV0010CLIRunWritesReceipt and
			// TestSTRV08704CLIMixedGoAndPlaywrightRun read it back).
			receiptID := sessiontest.UnitTestID(c.unit, nil)
			if planID != c.want || receiptID != c.want {
				t.Fatalf("plan %q, receipt %q, want %q", planID, receiptID, c.want)
			}
			if c.test == nil {
				return
			}
			if test := c.test(t); !testid.Matches(planID, test) || test == planID {
				t.Fatalf("plan %q does not select the reported test %q", planID, test)
			}
		})
	}
}
