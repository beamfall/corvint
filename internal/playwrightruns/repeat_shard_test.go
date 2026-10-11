package playwrightruns

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// runsFixture reads one report under testdata/runs. The real ones were produced by
// @playwright/test 1.61.1 (see docs/build-log/2026-10-11-runs-repeat-each-and-shards.md).
func runsFixture(t *testing.T, name string) *Run {
	t.Helper()
	run, err := ReadFile(filepath.Join("testdata", "runs", name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return run
}

func runsFixtures(t *testing.T, names ...string) []*Run {
	t.Helper()
	out := []*Run{}
	for _, n := range names {
		out = append(out, runsFixture(t, n))
	}
	return out
}

func loadedFixtures(t *testing.T, names ...string) []Loaded {
	t.Helper()
	out := []Loaded{}
	for _, run := range runsFixtures(t, names...) {
		out = append(out, Loaded{Run: run})
	}
	return out
}

func compact(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// singleRepeatCases are unsharded single-repeat inputs whose documents must stay byte-identical to
// the output before PWR-V0-011..030 (goldens produced at 645304a2fb71065f9061d8d8d866600d703e84c6).
func singleRepeatCases(t *testing.T) map[string][]byte {
	declared := identity("app-1", "cfg-1", blobsA)
	ambiguous := reportJSON(t, "1.63.0", declared, cartTest(false, fail("Error: expect(page).toHaveURL(expected) failed", fixtureRoot+"/cart.spec.ts", 4)), cartTest(false, pass()))
	failed := reportJSON(t, "1.63.0", declared, cartTest(true, fail("Error: boom", fixtureRoot+"/pages/cart.ts", 7)))
	passed := reportJSON(t, "1.63.0", declared, cartTest(true, pass()))
	synthetic := []*Run{mustParse(t, ambiguous), mustParse(t, failed), mustParse(t, passed)}
	return map[string][]byte{
		"golden-stability-real.json":      compact(t, Stability(runsFixtures(t, "whole.json", "merged.json", "whole.json"), 3)),
		"golden-stability-synthetic.json": compact(t, Stability(synthetic, 2)),
		"golden-compare-real.json":        compact(t, Compare(loadedFixtures(t, "whole.json", "whole.json"), loadedFixtures(t, "merged.json"))),
		"golden-compare-synthetic.json": compact(t, Compare([]Loaded{{Run: mustParse(t, failed)}, {Run: mustParse(t, passed)}},
			[]Loaded{{Run: mustParse(t, ambiguous)}, {Run: mustParse(t, passed)}})),
	}
}

func TestSingleRepeatOutputByteIdentical(t *testing.T) {
	for name, got := range singleRepeatCases(t) {
		want, err := os.ReadFile(filepath.Join("testdata", "runs", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed:\n got %s\nwant %s", name, got, want)
		}
	}
}
