package testvaliditydoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// recordedExternal reads the qualified corvint-playwright-external/3 document of one recorded
// CCE side (CCE-V0-101): a complete external run whose tests all executed.
func recordedExternal(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "changee2e", "testdata", "recorded", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var side struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal(raw, &side); err != nil {
		t.Fatal(err)
	}
	return []byte(side.Receipt)
}

// PWP-V0-153, PWP-V0-154: a complete external receipt names why its run has no execution state
// (each test's axis is the verdict, and DCP-V1-007 keeps the run unpromoted) and why freshness is
// unknown, instead of no-input-supplied and no-execution-report.
func TestPWPV0153CompleteExternalReceiptNamesRunAndFreshness(t *testing.T) {
	for _, c := range []struct{ side, test string }{
		{"preserves.head", testvalidity.ExecutionPassed},
		{"regression.head", testvalidity.ExecutionFailed},
	} {
		input, err := Decode(recordedExternal(t, c.side))
		if err != nil {
			t.Fatalf("%s: %v", c.side, err)
		}
		doc := Project(input)
		if doc.Run.Execution.State != testvalidity.StateUnsupported || doc.Run.Execution.Reason != ReasonExternalRunPerTest {
			t.Errorf("%s: run execution %+v", c.side, doc.Run.Execution)
		}
		if doc.Run.Freshness.State != testvalidity.FreshnessUnknown || doc.Run.Freshness.Reason != ReasonExternalCurrencyUnverified {
			t.Errorf("%s: run freshness %+v", c.side, doc.Run.Freshness)
		}
		for _, test := range doc.Tests {
			if test.Projection.Execution.State != c.test || test.Projection.Freshness.State != testvalidity.FreshnessUnknown || test.Projection.Freshness.Reason != ReasonExternalCurrencyUnverified {
				t.Errorf("%s: test %s projection %+v", c.side, test.Name, test.Projection)
			}
		}
	}
	// A stale build stays stale, and a run without tests keeps its own reason.
	stale := testvalidity.Project(testvalidity.Input{Execution: &testvalidity.ExecutionFacts{Currency: testvalidity.FreshnessStale}})
	if got := externalRunProjection(stale, []Test{{}}); got.Freshness.State != testvalidity.FreshnessStale {
		t.Errorf("stale: %+v", got.Freshness)
	}
	if got := externalRunProjection(testvalidity.Project(testvalidity.Input{}), nil); got.Execution.Reason != "no-input-supplied" || got.Freshness.Reason != ReasonExternalCurrencyUnverified {
		t.Errorf("no tests: %+v", got)
	}
}
