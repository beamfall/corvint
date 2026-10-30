package testsource

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// TestEitherOrOutcomeForms: each either-or form inside a test is one invalid finding on its line,
// naming the accepted values (TSA-V0-160, TSA-V0-161).
func TestEitherOrOutcomeForms(t *testing.T) {
	src := `import { test, expect } from '@playwright/test';
test('denies the stranger', async ({ request }) => {
  const res = await request.get('/admin');
  expect([401, 403]).toContain(res.status());
  expect(res.status() === 401 || res.status() === 403).toBe(true);
  expect(res.status()).toBeOneOf([401, 403]);
  expect(String(res.status())).toMatch(/^(401|403)$/);
  expect(['denied', 'forbidden'].includes(state)).toBeTruthy();
  assert.oneOf(code, [200, 204]);
  expect(String(code)).toMatch(/^4[01]3$/);
});
`
	r := ValidityFindings("e2e/admin.spec.ts", src)
	wantFindings(t, "forms", r,
		"EITHER-OR-OUTCOME invalid e2e/admin.spec.ts:4 e2e/admin.spec.ts:4 denies the stranger",
		"EITHER-OR-OUTCOME invalid e2e/admin.spec.ts:5 e2e/admin.spec.ts:5 denies the stranger",
		"EITHER-OR-OUTCOME invalid e2e/admin.spec.ts:6 e2e/admin.spec.ts:6 denies the stranger",
		"EITHER-OR-OUTCOME invalid e2e/admin.spec.ts:7 e2e/admin.spec.ts:7 denies the stranger",
		"EITHER-OR-OUTCOME invalid e2e/admin.spec.ts:8 e2e/admin.spec.ts:8 denies the stranger",
		"EITHER-OR-OUTCOME invalid e2e/admin.spec.ts:9 e2e/admin.spec.ts:9 denies the stranger",
		"EITHER-OR-OUTCOME invalid e2e/admin.spec.ts:10 e2e/admin.spec.ts:10 denies the stranger",
	)
	for _, f := range r.Findings {
		if f.Test != "e2e/admin.spec.ts:2" || !strings.Contains(f.Message, testvalidity.ValidityAllowMarker+" EITHER-OR-OUTCOME") {
			t.Errorf("finding %+v", f)
		}
	}
	if !strings.Contains(r.Findings[0].Message, "any of 401, 403") {
		t.Errorf("values not named: %s", r.Findings[0].Message)
	}
}

// TestEitherOrOutcomeNegatives: one value, a non-literal set, a negated chain, an unanchored word
// alternation, two different subjects, and an assertion outside any test are not reported.
func TestEitherOrOutcomeNegatives(t *testing.T) {
	src := `import { test, expect } from '@playwright/test';
function helper(code) { expect([401, 403]).toContain(code); }
test('single values', async ({ request }) => {
  const res = await request.get('/admin');
  expect([401]).toContain(res.status());
  expect(allowed).toContain(res.status());
  expect(res.status()).not.toBeOneOf([401, 403]);
  expect(message).toMatch(/error|warning/);
  expect(a === 1 || b === 2).toBe(true);
  expect(res.status()).toBe(401);
  expect([401, 401]).toContain(res.status());
});
`
	wantFindings(t, "negatives", ValidityFindings("e2e/single.spec.ts", src))
}

// TestEitherOrOutcomeAnnotation: a justification annotation with a reason, on the line or alone on
// the line above, exempts the assertion; one without a reason or naming another rule does not
// (TSA-V0-163).
func TestEitherOrOutcomeAnnotation(t *testing.T) {
	src := `test('upstream picks', async () => {
  // corvint-validity-allow EITHER-OR-OUTCOME: the upstream gateway answers 502 or 504 at random
  expect([502, 504]).toContain(status);
  expect([502, 504]).toContain(other); // corvint-validity-allow EITHER-OR-OUTCOME: same gateway
  // corvint-validity-allow EITHER-OR-OUTCOME:
  expect([401, 403]).toContain(status);
  // corvint-validity-allow VACUOUS-EMPTY-EQUALITY: wrong rule
  expect([401, 403]).toContain(status);
});
`
	wantFindings(t, "annotation", ValidityFindings("e2e/gw.spec.ts", src),
		"EITHER-OR-OUTCOME invalid e2e/gw.spec.ts:6 e2e/gw.spec.ts:6 upstream picks",
		"EITHER-OR-OUTCOME invalid e2e/gw.spec.ts:8 e2e/gw.spec.ts:8 upstream picks",
	)
}

// TestEitherOrOutcomeContract: "X or Y" and enumerated "X/Y" expected values in a behaviour contract
// are reported on their line; a single value, a date, prose or an annotated line is not
// (TSA-V0-162).
func TestEitherOrOutcomeContract(t *testing.T) {
	md := `# Admin access

- expected status: 401 or 403
- **Expected outcome**: DENIED/FORBIDDEN
- expected status: 200
- expected date: 2026/10/19
- expected message: the user sees an error or a warning banner
<!-- corvint-validity-allow EITHER-OR-OUTCOME: the gateway is non-deterministic -->
- expected status: 502 or 504
`
	wantFindings(t, "markdown", ValidityFindings("docs/contracts/admin.md", md),
		"EITHER-OR-OUTCOME invalid docs/contracts/admin.md:3 docs/contracts/admin.md:3 expected status",
		"EITHER-OR-OUTCOME invalid docs/contracts/admin.md:4 docs/contracts/admin.md:4 Expected outcome",
	)
	yml := "rows:\n  - id: r1\n    expected_status: 401 or 403\n"
	wantFindings(t, "yaml", ValidityFindings("contracts/admin.yml", yml),
		"EITHER-OR-OUTCOME invalid contracts/admin.yml:3 contracts/admin.yml:3 expected_status",
	)
	wantFindings(t, "other file", ValidityFindings("docs/notes.txt", "expected status: 401 or 403\n"))
}

// TestEitherOrOutcomeRefusesProof: the invalid finding makes the test INELIGIBLE, so the source
// validity report refuses (TSA-V0-160, TSA-V0-051).
func TestEitherOrOutcomeRefusesProof(t *testing.T) {
	src := `test('denies', async () => {
  expect([401, 403]).toContain(status);
});
test('allows', async () => {
  expect(status).toBe(200);
});
`
	_, tests := readSourceValidity("e2e/a.spec.ts", src, testvalidity.RubySupport{})
	if len(tests) != 2 || tests[0].Hygiene.State != testvalidity.HygieneIneligible || tests[0].Hygiene.Reason != testvalidity.RuleEitherOrOutcome || tests[1].Hygiene.State != testvalidity.HygieneEligible {
		t.Fatalf("tests %+v", tests)
	}
	if !(SourceValidityReport{Tests: tests}).Refused() {
		t.Error("report not refused")
	}
}

// TestEitherOrOutcomeReviewCases: equal numbers written differently are one outcome, and `^A|B$`
// anchors only its ends, so neither is reported; parenthesized equalities on one operand are; a
// contract value is exempted only by an annotation comment, not by data on the line above
// (TSA-V0-161, TSA-V0-163).
func TestEitherOrOutcomeReviewCases(t *testing.T) {
	src := `test('cases', async () => {
  expect([200, 200.0]).toContain(status);
  expect(message).toMatch(/^BEGIN|END$/);
  expect((status === 401) || (status === 403)).toBe(true);
});
`
	wantFindings(t, "js", ValidityFindings("e2e/cases.spec.ts", src),
		"EITHER-OR-OUTCOME invalid e2e/cases.spec.ts:4 e2e/cases.spec.ts:4 cases",
	)
	yml := "note: \"corvint-validity-allow EITHER-OR-OUTCOME: example\"\nexpected_status: 401 or 403\n# corvint-validity-allow EITHER-OR-OUTCOME: gateway\nexpected_code: 502 or 504\n"
	wantFindings(t, "yaml", ValidityFindings("contracts/a.yml", yml),
		"EITHER-OR-OUTCOME invalid contracts/a.yml:2 contracts/a.yml:2 expected_status",
	)
}
