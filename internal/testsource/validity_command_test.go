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

// TestSourceValidityJSExamples: a JS/TS test file gives one tests entry per example in line order,
// with the example rules' hygiene and a NOT_MEASURED strength member; an unlocated example
// abstains with no strength, and a skipped one refuses the report (TSA-V0-081).
func TestSourceValidityJSExamples(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{
		"src/cart.test.ts": `import { total } from './cart';
describe('cart', () => {
  it('totals', () => { expect(total([1, 2])).toBe(3); });
  it.skip('skipped', () => { expect(total([])).toBe(0); });
  it('asserts nothing', () => { total([1]); });
  it('by reference', handler);
  it('constant', () => { expect(1).toBe(1); });
});
`,
	})
	repo := OpenRepo(context.Background(), root)
	report, err := SourceValidity(repo, SourceValidityOptions{Files: []string{"src/cart.test.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ test, hygiene, hygieneReason, strength string }
	got := []row{}
	for _, test := range report.Tests {
		r := row{test.Test, test.Hygiene.State, test.Hygiene.Reason, "-"}
		if test.Strength != nil {
			r.strength = test.Strength.State + " " + test.Strength.Reason
		}
		got = append(got, r)
	}
	notMeasured := testvalidity.StrengthNotMeasured + " "
	want := []row{
		{"src/cart.test.ts:3", testvalidity.HygieneEligible, hygieneReasonSourceRead, notMeasured + testvalidity.StrengthReasonBodyExpectations},
		{"src/cart.test.ts:4", testvalidity.HygieneIneligible, testvalidity.RuleJSSkipped, notMeasured + testvalidity.StrengthReasonBodyExpectations},
		{"src/cart.test.ts:5", testvalidity.HygieneEligible, hygieneReasonSourceRead, notMeasured + testvalidity.StrengthReasonNoBodyExpectation},
		{"src/cart.test.ts:6", testvalidity.HygieneAbstained, testvalidity.HygieneReasonJSBodyNotLiteral, "-"},
		{"src/cart.test.ts:7", testvalidity.HygieneEligible, hygieneReasonSourceRead, notMeasured + testvalidity.StrengthReasonConstantOnly},
	}
	if !reflect.DeepEqual(got, want) || !report.Refused() {
		t.Errorf("tests\n%v\nwant\n%v", got, want)
	}
	rules := []string{}
	for _, f := range report.Files[0].Findings {
		rules = append(rules, f.Rule+"@"+f.Test)
	}
	wantRules := []string{
		testvalidity.RuleJSSkipped + "@src/cart.test.ts:4",
		testvalidity.RuleJSNoAssertion + "@src/cart.test.ts:5",
		testvalidity.RuleJSConstantExpectation + "@src/cart.test.ts:7",
	}
	if !reflect.DeepEqual(rules, wantRules) || report.Files[0].Uncertain {
		t.Errorf("findings %q, want %q", rules, wantRules)
	}
}

// TestSourceValidityQualifiedReference: `tests validity` reads a qualified reference at the
// revision through its one inclusion site, with the support set read from the same commit; each
// inclusion gets its own verdict and an unbindable one abstains with its reason (TSA-V0-089..091).
func TestSourceValidityQualifiedReference(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{
		"spec/widget_spec.rb": widgetSpec,
		widgetShared:          widgetExamples,
	})
	repo := OpenRepo(context.Background(), root)
	stub := "spec/widget_spec.rb:5>" + widgetShared + ":2"
	yielding := "spec/widget_spec.rb:11>" + widgetShared + ":2"
	missing := "spec/widget_spec.rb:3>" + widgetShared + ":2"
	report, err := SourceValidity(repo, SourceValidityOptions{Files: []string{stub, yielding, missing}})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, test := range report.Tests {
		got = append(got, test.Test+" "+test.Hygiene.State+" "+test.Hygiene.Reason)
	}
	want := []string{
		stub + " " + testvalidity.HygieneIneligible + " " + testvalidity.RuleRBBlockNeverYielded,
		yielding + " " + testvalidity.HygieneEligible + " " + hygieneReasonSourceRead,
		missing + " " + testvalidity.HygieneAbstained + " " + testvalidity.InclusionNotFound,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tests = %q\nwant %q", got, want)
	}
	if f := report.Files[2]; f.Path != missing || !f.Uncertain || f.Reason != testvalidity.InclusionNotFound {
		t.Errorf("file = %+v, want uncertain %s", f, testvalidity.InclusionNotFound)
	}
	if !report.Refused() {
		t.Error("an INELIGIBLE inclusion did not refuse the report")
	}
	if _, err := SourceValidity(repo, SourceValidityOptions{Files: []string{"spec/widget_spec.rb:5>spec/support/gone.rb:2"}}); !hasCode(err, "tests-source-unavailable") {
		t.Errorf("missing shared file: %v, want tests-source-unavailable", err)
	}
}
