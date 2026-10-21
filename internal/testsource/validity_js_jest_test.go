package testsource

import (
	"context"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// TestJestValidityTestFile: a JS/TS file under `__tests__` is a test file whatever its name, for
// test-validity and for lint and the other readers that share IsTestFile (TSA-V0-388, V1-2058).
func TestJestValidityTestFile(t *testing.T) {
	for name, want := range map[string]bool{
		"src/__tests__/cart.ts":         true,
		"__tests__/deep/cart.tsx":       true,
		"src/cart.test.ts":              true,
		"src/__tests__/fixture.json":    false,
		"src/tests/cart.ts":             false,
		"src/__tests__x/cart.ts":        false,
		"spec/models/cart_spec.rb":      false,
		"packages/a/__tests__/util.mjs": true,
	} {
		if got := IsTestFile(name); got != want {
			t.Errorf("IsTestFile(%q) = %v, want %v", name, got, want)
		}
	}
	if _, reason := JSExampleSources("src/__tests__/cart.ts", "it('a', () => { expect(1).toBe(1); });\n"); reason != "" {
		t.Errorf("a __tests__ file abstained with %q", reason)
	}
}

const jestNeverRunsSpec = `import { load } from '../cart';

describe('cart', () => {
  it('then not awaited', () => {
    load().then((cart) => { expect(cart.items).toHaveLength(2); });
  });
  it('catch not awaited', () => {
    load()
      .then(() => undefined)
      .catch((err) => expect(err).toBeTruthy());
  });
  it('awaited', async () => {
    await load().then((cart) => { expect(cart.items).toHaveLength(2); });
  });
  it('returned', () => {
    return load().then((cart) => { expect(cart.items).toHaveLength(2); });
  });
  it('concise', () => load().then((cart) => expect(cart.items).toHaveLength(2)));
  it('done callback', (done) => {
    load().then((cart) => { expect(cart.items).toHaveLength(2); done(); });
  });
  it('flushed later', async () => {
    load().then((cart) => { expect(cart.items).toHaveLength(2); });
    await flushPromises();
  });
  it('inside an awaited executor', async () => {
    await new Promise((resolve) => {
      load().then((cart) => { expect(cart.items).toHaveLength(2); resolve(); });
    });
  });
  it('passed on', () => {
    expect(load().then((cart) => cart.items)).resolves.toHaveLength(2);
  });
  it('no assertion in callback', () => {
    load().then((cart) => cart.items);
    expect(1).toBe(2);
  });
});
`

// TestJestAssertionNotAwaited: an assertion in a then, catch or finally callback of a promise
// statement that the parameterless example neither awaits nor returns, with nothing awaited
// after it, gives an advisory JS-ASSERTION-NOT-AWAITED; every awaited, returned, concise, done,
// flushed, nested or passed-on form gives none (TSA-V0-389).
func TestJestAssertionNotAwaited(t *testing.T) {
	name := "src/__tests__/cart.ts"
	got := map[int][]string{}
	for line, rows := range jsBrief(t, name, jestNeverRunsSpec) {
		kept := []string{}
		for _, row := range rows[1:] {
			if len(row) > len(testvalidity.RuleJSAssertionNotAwaited) && row[:len(testvalidity.RuleJSAssertionNotAwaited)] == testvalidity.RuleJSAssertionNotAwaited {
				kept = append(kept, row)
			}
		}
		got[line] = kept
	}
	want := map[int][]string{
		4:  {"JS-ASSERTION-NOT-AWAITED advisory src/__tests__/cart.ts:5 src/__tests__/cart.ts:4"},
		7:  {"JS-ASSERTION-NOT-AWAITED advisory src/__tests__/cart.ts:10 src/__tests__/cart.ts:7"},
		12: {}, 15: {}, 18: {}, 19: {}, 22: {}, 26: {}, 31: {}, 34: {},
	}
	wantJS(t, got, want)
}

// TestJestValidityAxes: `tests validity` on a Jest file under `__tests__` lists every example
// with the TSA-V0-081 hygiene and strength: skipped forms are INELIGIBLE, an assertion-free
// example is NOT_MEASURED no-expectation-in-example-body, and a never-executed assertion carries
// the advisory JS-ASSERTION-NOT-AWAITED (TSA-V0-388..390).
func TestJestValidityAxes(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{
		"src/__tests__/cart.ts": `import { total, load } from '../cart';
describe('cart', () => {
  it('totals', () => { expect(total([1, 2])).toBe(3); });
  xit('x skipped', () => { expect(total([])).toBe(0); });
  xtest('xtest skipped', () => { expect(total([])).toBe(0); });
  test.skip('member skipped', () => { expect(total([])).toBe(0); });
  it('asserts nothing', () => { total([1]); });
  it('never runs', () => {
    load().then((cart) => { expect(total(cart)).toBe(3); });
  });
});
xdescribe('skipped describe', () => {
  it('inside', () => { expect(total([1])).toBe(1); });
});
`,
	})
	report, err := SourceValidity(OpenRepo(context.Background(), root), SourceValidityOptions{Files: []string{"src/__tests__/cart.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Files[0].Uncertain {
		t.Fatalf("file abstained: %s", report.Files[0].Reason)
	}
	type row struct{ test, hygiene, reason, strength string }
	got := []row{}
	for _, test := range report.Tests {
		r := row{test.Test, test.Hygiene.State, test.Hygiene.Reason, "-"}
		if test.Strength != nil {
			r.strength = test.Strength.Reason
		}
		got = append(got, r)
	}
	path := "src/__tests__/cart.ts:"
	want := []row{
		{path + "3", testvalidity.HygieneEligible, hygieneReasonSourceRead, testvalidity.StrengthReasonBodyExpectations},
		{path + "4", testvalidity.HygieneIneligible, testvalidity.RuleJSSkipped, testvalidity.StrengthReasonBodyExpectations},
		{path + "5", testvalidity.HygieneIneligible, testvalidity.RuleJSSkipped, testvalidity.StrengthReasonBodyExpectations},
		{path + "6", testvalidity.HygieneIneligible, testvalidity.RuleJSSkipped, testvalidity.StrengthReasonBodyExpectations},
		{path + "7", testvalidity.HygieneEligible, hygieneReasonSourceRead, testvalidity.StrengthReasonNoBodyExpectation},
		{path + "8", testvalidity.HygieneEligible, hygieneReasonSourceRead, testvalidity.StrengthReasonBodyExpectations},
		{path + "13", testvalidity.HygieneIneligible, testvalidity.RuleJSSkipped, testvalidity.StrengthReasonBodyExpectations},
	}
	if !reflect.DeepEqual(got, want) || !report.Refused() {
		t.Errorf("tests\n%v\nwant\n%v", got, want)
	}
	found := false
	for _, f := range report.Files[0].Findings {
		if f.Rule == testvalidity.RuleJSAssertionNotAwaited {
			found = f.Test == path+"8" && f.Location == path+"9" && f.Severity == testvalidity.SeverityAdvisory
		}
	}
	if !found {
		t.Errorf("no advisory JS-ASSERTION-NOT-AWAITED for %s8 at line 9: %+v", path, report.Files[0].Findings)
	}
}

// TestJestTestsDirectoryFileIsLinted: lint shares IsTestFile, so a Jest file under `__tests__`
// with no `.test`/`.spec` stem is linted at its changed lines (TSA-V0-010, TSA-V0-388, V1-2058).
func TestJestTestsDirectoryFileIsLinted(t *testing.T) {
	root := t.TempDir()
	base := commitFiles(t, root, map[string]string{"README.md": "x\n"})
	commitFiles(t, root, map[string]string{"src/__tests__/legacy.ts": oldSpec})
	report, err := Lint(OpenRepo(context.Background(), root), LintOptions{Base: base, Revision: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.Files, []string{"src/__tests__/legacy.ts"}) || len(report.Findings) == 0 {
		t.Fatalf("__tests__ file not linted: files=%v findings=%v", report.Files, report.Findings)
	}
}
