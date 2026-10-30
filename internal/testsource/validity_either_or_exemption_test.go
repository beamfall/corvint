package testsource

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// TestEitherOrOutcomeExemptionRetainsReason: TSA-V0-870 (V1-1930 AC2). Every line a justified
// annotation exempts is reported as an exemption naming the rule, the exempted `path:line` and the
// annotation's own reason, in JS/TS (the line above and the same line), a Markdown and a YAML
// contract comment, and a JSON sibling member; a reasonless or other-rule annotation exempts
// nothing and records nothing. The file record carries the same exemptions in its JSON.
func TestEitherOrOutcomeExemptionRetainsReason(t *testing.T) {
	exempt := func(location, reason string) testvalidity.RuleExemption {
		return testvalidity.RuleExemption{Rule: testvalidity.RuleEitherOrOutcome, Location: location, Reason: reason}
	}
	js := `test('upstream picks', async () => {
  // corvint-validity-allow EITHER-OR-OUTCOME: the upstream gateway answers 502 or 504 at random
  expect([502, 504]).toContain(status);
  expect([502, 504]).toContain(other); /* corvint-validity-allow EITHER-OR-OUTCOME: same gateway */
  // corvint-validity-allow EITHER-OR-OUTCOME:
  expect([401, 403]).toContain(status);
  // corvint-validity-allow VACUOUS-EMPTY-EQUALITY: wrong rule
  expect([401, 403]).toContain(status);
});
`
	md := "<!-- corvint-validity-allow EITHER-OR-OUTCOME: the gateway is non-deterministic -->\n- expected status: 502 or 504\n- expected status: 401 or 403\n"
	yml := "# corvint-validity-allow EITHER-OR-OUTCOME: load balancer picks\nexpected_code: 502 or 504\n"
	jsonContract := `{
  "rows": [
    {
      "expected_status": "502 or 504",
      "corvint-validity-allow": {"expected_status": "EITHER-OR-OUTCOME: the gateway is non-deterministic"}
    },
    {
      "expected_status": "401 or 403",
      "corvint-validity-allow": {"expected_status": "EITHER-OR-OUTCOME:"}
    }
  ]
}
`
	for _, c := range []struct {
		name, text string
		findings   int
		want       []testvalidity.RuleExemption
	}{
		{"e2e/gw.spec.ts", js, 2, []testvalidity.RuleExemption{exempt("e2e/gw.spec.ts:3", "the upstream gateway answers 502 or 504 at random"), exempt("e2e/gw.spec.ts:4", "same gateway")}},
		{"docs/contracts/gw.md", md, 1, []testvalidity.RuleExemption{exempt("docs/contracts/gw.md:2", "the gateway is non-deterministic")}},
		{"contracts/gw.yml", yml, 0, []testvalidity.RuleExemption{exempt("contracts/gw.yml:2", "load balancer picks")}},
		{"contracts/gw.json", jsonContract, 1, []testvalidity.RuleExemption{exempt("contracts/gw.json:4", "the gateway is non-deterministic")}},
	} {
		report := ValidityFindings(c.name, c.text)
		if len(report.Findings) != c.findings || !reflect.DeepEqual(report.Exemptions, c.want) {
			t.Errorf("%s: findings=%+v exemptions=%+v, want %d findings and %+v", c.name, report.Findings, report.Exemptions, c.findings, c.want)
		}
		file, _ := readSourceValidity(c.name, c.text, testvalidity.RubySupport{})
		raw, err := json.Marshal(file)
		if err != nil || !reflect.DeepEqual(file.Exemptions, c.want) || !strings.Contains(string(raw), `"exemptions":[{"rule":"EITHER-OR-OUTCOME","location":"`+c.want[0].Location+`","reason":"`+c.want[0].Reason+`"}`) {
			t.Errorf("%s: file record lost the exemption reason: %s %v", c.name, raw, err)
		}
	}
	if report := ValidityFindings("e2e/plain.spec.ts", "test('x', async () => {\n  expect([401, 403]).toContain(status);\n});\n"); report.Exemptions != nil {
		t.Errorf("unannotated file recorded exemptions: %+v", report.Exemptions)
	}
}

// TestEitherOrOutcomeProposesNoValue: TSA-V0-164 (V1-1930 AC3). Without a receipt observing the
// operand, the finding names every accepted value and proposes none of them: no receipt records
// assertion operand values (LPCV-V0-120 records operand sizes only), so no reader can know which
// value a passing run returned.
func TestEitherOrOutcomeProposesNoValue(t *testing.T) {
	src := `test('denies', async () => {
  expect([401, 403]).toContain(status);
  expect(code).toMatch(/^(DENIED|FORBIDDEN)$/);
});
`
	report := ValidityFindings("e2e/deny.spec.ts", src)
	if len(report.Findings) != 2 {
		t.Fatalf("findings %+v", report.Findings)
	}
	for i, values := range [][]string{{"401", "403"}, {"DENIED", "FORBIDDEN"}} {
		message := report.Findings[i].Message
		if !strings.Contains(message, "accepts any of "+strings.Join(values, ", ")+",") || !strings.Contains(message, "assert the one correct value") {
			t.Errorf("finding %d does not name every accepted value: %s", i, message)
		}
		for _, guess := range []string{"propos", "observed", "returned " + values[0], "returned " + values[1], "use " + values[0], "use " + values[1]} {
			if strings.Contains(message, guess) {
				t.Errorf("finding %d guesses a value (%q): %s", i, guess, message)
			}
		}
	}
}
