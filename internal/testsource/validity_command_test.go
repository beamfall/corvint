package testsource

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const serialSpec = `test.describe('alpha', () => {
  test.describe.configure({ mode: 'serial' });
  test('alpha-01 existing', async () => { expect(1).toBe(1); });
  test('alpha-02 new row', { tag: '@isolated' }, async () => { expect(2).toBe(2); });
});
`

// TestSourceValidityReport: the `tests validity` reading reports ISOLATED-IN-SERIAL and
// MEMOIZED-BLOCK-EXPECTATION findings and the RSpec example rules over committed files; a test
// with an invalid finding is INELIGIBLE and refuses the report, one with only advisory findings
// stays ELIGIBLE (TSA-V0-071, V1-1787).
func TestSourceValidityReport(t *testing.T) {
	root := t.TempDir()
	head := commitFiles(t, root, map[string]string{
		"e2e/alpha.spec.ts":                 serialSpec,
		"spec/services/widget_sync_spec.rb": memoizedSpec,
		"spec/models/widget_spec.rb":        hygieneSpec,
		"spec/models/clean_spec.rb":         "RSpec.describe Clean do\n  it 'works' do\n    expect(1).to eq(1)\n  end\nend\n",
	})
	repo := OpenRepo(context.Background(), root)
	report, err := SourceValidity(repo, SourceValidityOptions{Revision: "HEAD", Files: []string{
		"e2e/alpha.spec.ts", "spec/services/widget_sync_spec.rb", "spec/models/widget_spec.rb", "spec/models/clean_spec.rb", "e2e/alpha.spec.ts",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != SourceValiditySchema || report.Revision != head || len(report.Files) != 4 || !report.Refused() {
		t.Fatalf("report header %+v", report)
	}
	hygiene := map[string]testvalidity.Axis{}
	for _, test := range report.Tests {
		hygiene[test.Test] = test.Hygiene
	}
	if h := hygiene["e2e/alpha.spec.ts:4"]; h.State != testvalidity.HygieneEligible || h.Reason != hygieneReasonSourceRead {
		t.Errorf("isolated-in-serial test %+v", h)
	}
	if h := hygiene["spec/services/widget_sync_spec.rb:12"]; h.State != testvalidity.HygieneIneligible || h.Reason != "memoized-block-expectation" {
		t.Errorf("memoized test %+v", h)
	}
	if h := hygiene["spec/models/widget_spec.rb:6"]; h.State != testvalidity.HygieneIneligible || h.Reason != testvalidity.RuleRSpecSkipped {
		t.Errorf("skipped test %+v", h)
	}
	rules := []string{}
	for _, f := range report.Files {
		for _, finding := range f.Findings {
			rules = append(rules, finding.Rule)
		}
	}
	joined := strings.Join(rules, ",")
	for _, want := range []string{testvalidity.RuleIsolatedInSerial, testvalidity.RuleMemoizedBlockExpectation, testvalidity.RuleRSpecSkipped, testvalidity.RuleRSpecFocused} {
		if !strings.Contains(joined, want) {
			t.Errorf("rules %q lack %s", joined, want)
		}
	}
	if clean := report.Files[3]; clean.Uncertain || len(clean.Findings) != 0 {
		t.Errorf("clean file %+v", clean)
	}
	again, _ := SourceValidity(repo, SourceValidityOptions{Revision: "HEAD", Files: []string{"e2e/alpha.spec.ts", "spec/services/widget_sync_spec.rb", "spec/models/widget_spec.rb", "spec/models/clean_spec.rb"}})
	if !reflect.DeepEqual(again, report) {
		t.Errorf("a repeated or deduplicated reading differs")
	}

	clean, err := SourceValidity(repo, SourceValidityOptions{Files: []string{"spec/models/clean_spec.rb"}})
	if err != nil || clean.Refused() || len(clean.Tests) != 0 {
		t.Errorf("clean report %+v %v", clean, err)
	}
}

// TestSourceValidityRefusals: no file, an invalid path, an uncommitted file and too many files are
// typed refusals (TSA-V0-071).
func TestSourceValidityRefusals(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"spec/a_spec.rb": "RSpec.describe A do\nend\n"})
	repo := OpenRepo(context.Background(), root)
	many := make([]string, MaxValidityFiles+1)
	for i := range many {
		many[i] = "spec/" + strings.Repeat("x", i+1) + "_spec.rb"
	}
	for _, c := range []struct {
		files []string
		code  string
	}{
		{nil, "tests-invalid-arguments"},
		{[]string{"../a_spec.rb"}, "tests-invalid-arguments"},
		{[]string{".git/config"}, "tests-invalid-arguments"},
		{[]string{"spec/missing_spec.rb"}, "tests-source-unavailable"},
		{many, "tests-bound-exceeded"},
	} {
		if _, err := SourceValidity(repo, SourceValidityOptions{Files: c.files}); !hasCode(err, c.code) {
			t.Errorf("%v: %v, want %s", c.files, err, c.code)
		}
	}
}
