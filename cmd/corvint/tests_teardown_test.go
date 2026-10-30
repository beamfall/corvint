package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

const teardownCLIDeclaration = `{"schema": "corvint-side-effect-declaration/0", "version": 1,
 "revision": "0123456789abcdef0123456789abcdef01234567",
 "resources": [
  {"kind": "order", "create": ["createOrder"], "delete": ["deleteOrder"]},
  {"kind": "confirmation", "create": ["createConfirmation"], "delete": ["deleteConfirmation"]}],
 "actions": [{"call": "confirmOrder", "parent": "order", "dependent": "confirmation"}]}
`

const teardownCLISpec = `test('leftover confirmation', async ({ request }) => {
  const order = await createOrder(request);
  try {
    await confirmOrder(request, order);
  } finally {
    await deleteOrder(request, order);
  }
});
`

// TestTestsTeardownCLI: `corvint tests teardown` prints the corvint-test-teardown-dependents/0
// report, exits 1 on a declared finding, joins a bound JS receipt's 409 delete and refuses bad
// arguments, declarations and receipts (TSA-V0-110, TSA-V0-117, TSA-V0-119).
func TestTestsTeardownCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	testsCommit(t, root, map[string]string{"e2e/orders.spec.ts": teardownCLISpec, ".corvint/side-effects.json": teardownCLIDeclaration, "bad.json": `{"schema":"corvint-side-effect-declaration/0","extra":1}`})
	sum := sha256.Sum256([]byte(teardownCLISpec))
	receipt := jstestprovider.Receipt{Kind: "e2e", Identity: jstestprovider.Identity{TestFileDigests: map[string]string{"/w/e2e/orders.spec.ts": hex.EncodeToString(sum[:])}},
		Tests: []jstestprovider.TestOutcome{{Name: "leftover confirmation", FullName: "leftover confirmation", State: jstestprovider.StateFailed,
			AttemptDetails: []jstestprovider.AttemptDetail{{State: jstestprovider.StateFailed, Anchor: &jstestprovider.Anchor{File: "/w/e2e/orders.spec.ts", Line: 6},
				RequestOutcomes: &jstestprovider.RequestOutcomeRecord{Assertion: "rejection-only", Requests: []jstestprovider.RequestOutcome{{Method: "DELETE", Route: "/orders/:id", Status: 409}}}}}}}}
	encoded, err := json.Marshal(map[string]any{"receipt": receipt})
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	receiptPath := filepath.Join(scratch, "receipt.json")
	if err := os.WriteFile(receiptPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errb := runCLI(t, "--root", root, "tests", "teardown", "--files", "e2e/orders.spec.ts", "--declaration", ".corvint/side-effects.json", "--receipt", receiptPath)
	var report struct {
		Schema, GraphSource string
		Findings            []struct {
			Rule, Basis, Cause     string
			ActionLine, DeleteLine int
			Recorded               []struct{ Status int }
		}
		Receipts []struct{ Bound bool }
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil || code != 1 {
		t.Fatalf("code=%d err=%s out=%s", code, errb, out)
	}
	if report.Schema != "corvint-test-teardown-dependents/0" || report.GraphSource != "declaration" || len(report.Findings) != 1 || len(report.Receipts) != 1 || !report.Receipts[0].Bound {
		t.Fatalf("report %+v", report)
	}
	if f := report.Findings[0]; f.Rule != "TEARDOWN-DEPENDENT-LEFTOVER" || f.Basis != "declared" || f.Cause != "dependent-not-removed" || f.ActionLine != 4 || f.DeleteLine != 6 || len(f.Recorded) != 1 || f.Recorded[0].Status != 409 {
		t.Errorf("finding %+v", f)
	}
	if _, again, _ := runCLI(t, "--root", root, "tests", "teardown", "--files", "e2e/orders.spec.ts", "--declaration", ".corvint/side-effects.json", "--receipt", receiptPath); again != out {
		t.Error("the report must be byte-identical for the same inputs")
	}
	garbage := filepath.Join(scratch, "garbage.json")
	if err := os.WriteFile(garbage, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		code string
	}{
		{[]string{"--files", "e2e/orders.spec.ts"}, "tests-invalid-arguments"},
		{[]string{"--declaration", ".corvint/side-effects.json"}, "tests-invalid-arguments"},
		{[]string{"--files", "e2e/orders.spec.ts", "--declaration", "a.json", "--declaration", "b.json"}, "tests-invalid-arguments"},
		{[]string{"--files", "e2e/orders.spec.ts", "--declaration", "bad.json"}, "tests-declaration-invalid"},
		{[]string{"--files", "e2e/orders.spec.ts", "--declaration", ".corvint/side-effects.json", "--receipt", garbage}, "tests-source-unavailable"},
		{append([]string{"--files", "e2e/orders.spec.ts", "--declaration", ".corvint/side-effects.json"}, strings.Split(strings.Repeat("--receipt "+receiptPath+" ", 17), " ")[:34]...), "tests-bound-exceeded"},
	} {
		code, out, errb := runCLI(t, append([]string{"--root", root, "tests", "teardown"}, c.args...)...)
		if code != 2 || out != "" || !strings.Contains(errb, `"code":"`+c.code+`"`) {
			t.Errorf("%v: code=%d out=%q err=%s", c.args, code, out, errb)
		}
	}
	if code, out, _ := runCLI(t, "tests", "--help"); code != 0 || !strings.Contains(out, "tests teardown --files") || !strings.Contains(out, "tests-declaration-invalid") {
		t.Errorf("help: code=%d", code)
	}
}
