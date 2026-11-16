// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// AFL-V1-002 criterion 3, "an ingested report": the committed Playwright JSON report of the
// AFU-V1-039 acceptance fixture goes through `flows ingest`, once to a test-run-evidence/0 file
// and once with --record into the run evidence store, and `flows status` reads each: every linked
// title shows the run's own per-title outcome (granularity test, authority INGESTED, fresh at
// HEAD), and the variation with no test link has none.
func TestAFLV1002PlaywrightReportThroughIngestIntoStatus_V15623(t *testing.T) {
	root, evidence := newAcceptanceFixture(t)
	head := shopGit(t, root, "rev-parse", "HEAD")
	want := map[string][3]string{
		"checkout/checkout.happy":      {"checkout.spec.ts > pays", "chromium", "passed"},
		"orders-api/orders-api.create": {"orders-api.spec.ts > creates", "api", "passed"},
		"profile/profile.save":         {"profile.spec.ts > saves", "chromium", "passed"},
	}
	check := func(name string, report appflows.StatusReport) {
		t.Helper()
		for at, w := range want {
			flow, variation, _ := strings.Cut(at, "/")
			tests := statusVariation(t, report, flow, variation).Tests
			if len(tests) != 1 || tests[0].TestKey != w[0] || tests[0].Project != w[1] || tests[0].Result != w[2] ||
				tests[0].Granularity != "test" || tests[0].Authority != appflows.AuthorityIngested || tests[0].Stale || tests[0].SourceCommit != head {
				t.Errorf("%s: %s: %+v", name, at, tests)
			}
		}
		if returns := statusVariation(t, report, "returns", "returns.basic"); len(returns.Tests) != 0 || returns.Code != appflows.GapNoTest {
			t.Errorf("%s: returns.basic %+v", name, returns)
		}
	}
	fromFile, _ := decodeStatus(t, root, "--no-store", "--evidence", evidence)
	check("--evidence of the ingested file", fromFile)
	if last := fromFile.ResultSources[len(fromFile.ResultSources)-1]; last.Kind != appflows.SourceEvidence || last.Records != 3 {
		t.Fatalf("result_sources %+v", fromFile.ResultSources)
	}
	report, err := filepath.Abs(filepath.Join(acceptanceFixture, "playwright-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	code, _, diagnostic := runFlowsCLI(root, append([]string{"ingest", "--format", "playwright-json", "--from", report, "--record"}, acceptanceIngestHeader(t, root, head)...)...)
	if code != 0 {
		t.Fatalf("ingest --record %d %s", code, diagnostic)
	}
	fromStore, _ := decodeStatus(t, root)
	check("the store after ingest --record", fromStore)
	if fromStore.ResultSources[0] != (appflows.ResultSource{Kind: appflows.SourceStore, State: appflows.SourceRead, Records: 3}) {
		t.Fatalf("result_sources %+v", fromStore.ResultSources)
	}
}
