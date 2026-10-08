package dynamic

import (
	"encoding/json"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// Synthetic profile-owned Mocha reports: the live counterparts are in
// TestMochaActualSelectionQualification (TRE-V0-024).
func TestMochaSelectionReconciliation(t *testing.T) {
	row := func(file, title string) map[string]any {
		return map[string]any{"ID": file + "::" + title, "Name": title, "File": file, "State": tr.Passed, "Attempts": []map[string]string{{"State": tr.Passed}}}
	}
	a, b := row("/r/tests/a.test.cjs", "a"), row("/r/other/b.test.cjs", "b")
	for _, tc := range []struct {
		name      string
		rows      []map[string]any
		selectors []string
		expected  []string
		codes     []string
	}{
		{"exact", []map[string]any{a}, []string{"tests/a.test.cjs"}, []string{"/r/tests/a.test.cjs::a"}, nil},
		{"absolute selector", []map[string]any{a}, []string{"/r/tests/a.test.cjs"}, nil, nil},
		{"no selectors keeps legacy inventory", []map[string]any{a, b}, nil, nil, nil},
		{"configured spec broadens", []map[string]any{a, b}, []string{"tests/a.test.cjs"}, []string{"/r/tests/a.test.cjs::a"}, []string{"unselected-test-file", "unexpected-observed-test"}},
		{"no-match selector", []map[string]any{a}, []string{"tests/a.test.cjs", "tests/missing.test.cjs"}, nil, []string{"selector-without-tests"}},
		{"directory selector stays lexical", []map[string]any{a}, []string{"tests"}, nil, []string{"unselected-test-file", "selector-without-tests"}},
		{"expected subset is not exact", []map[string]any{a, row("/r/tests/a.test.cjs", "a2")}, []string{"tests/a.test.cjs"}, []string{"/r/tests/a.test.cjs::a"}, []string{"unexpected-observed-test"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, e := json.Marshal(map[string]any{"Profile": "corvint-mocha/0", "Complete": true, "Count": len(tc.rows), "Tests": tc.rows, "Problems": []any{}})
			if e != nil {
				t.Fatal(e)
			}
			in := tr.Input{Runner: "mocha", SourceRoot: "/r", Selectors: tc.selectors, Expected: tc.expected, Reports: map[string][]byte{"mocha-1.json": report}}
			o, e := Parse(in)
			if e != nil {
				t.Fatal(e)
			}
			o = tr.Normalize(in, o)
			got := map[string]bool{}
			for _, p := range o.Problems {
				got[p.Code] = true
			}
			if o.Complete != (len(tc.codes) == 0) || len(got) != len(tc.codes) {
				t.Fatalf("complete=%v problems=%+v", o.Complete, o.Problems)
			}
			for _, c := range tc.codes {
				if !got[c] {
					t.Fatalf("missing %s in %+v", c, o.Problems)
				}
			}
			if !o.Complete {
				for _, x := range o.Tests {
					if x.State != tr.Unknown {
						t.Fatalf("resolved state survived: %+v", x)
					}
				}
			}
		})
	}
}

func TestMochaSelectionLeavesOtherRunnersUnchanged(t *testing.T) {
	report := []byte(`{"Profile":"corvint-cypress/0","Complete":true,"Count":1,"Tests":[{"ID":"/r/b.cy.js::b","Name":"b","File":"/r/b.cy.js","State":"PASSED","Attempts":[{"State":"PASSED"}]}],"Problems":[]}`)
	in := tr.Input{Runner: "cypress", SourceRoot: "/r", Selectors: []string{"a.cy.js"}, Reports: map[string][]byte{"cypress-1.json": report}}
	o, e := Parse(in)
	if e != nil || !o.Complete {
		t.Fatalf("%+v %v", o, e)
	}
}
