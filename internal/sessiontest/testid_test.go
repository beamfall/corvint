// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
)

// CTI-V1-009, CTI-V1-012: a corvint-test-run/0 receipt written before testId
// existed still reads strictly and joins each outcome at its unit's
// granularity; a receipt with testId reads the same IDs from the field; and
// stamping the old receipt gives exactly the new receipt's bytes, so the
// field is the only change and no stored record is rewritten.
func TestCTIV1012ReceiptWithAndWithoutTestID(t *testing.T) {
	t.Parallel()
	before, err := ReadReceipt(filepath.Join("testdata", "receipt-before-testid.json"))
	if err != nil {
		t.Fatal(err)
	}
	with, err := ReadReceipt(filepath.Join("testdata", "receipt-with-testid.json"))
	if err != nil {
		t.Fatal(err)
	}
	goDir := testid.GoModuleDir("example.test/m", ".")
	want := map[string]struct {
		id          string
		granularity testid.Granularity
	}{
		"go:example.test/m/a":             {"go:a", testid.GranularityPackage},
		"typescript:vitest:src/a.test.ts": {"vitest:src/a.test.ts", testid.GranularityFile},
		"ruby:rspec:spec/user_spec":       {"rspec:spec/user_spec.rb", testid.GranularityFile},
		"python:pkg.mod":                  {"", ""},
	}
	for index, invocation := range before.Invocations {
		old, stamped := invocation.Outcomes[0], with.Invocations[index].Outcomes[0]
		expect := want[old.Unit]
		if old.TestID != "" || stamped.TestID != expect.id || stamped.Unit != old.Unit || stamped.Outcome != old.Outcome {
			t.Errorf("outcome %d: before %+v, with %+v, want testId %q", index, old, stamped, expect.id)
		}
		for name, outcome := range map[string]Outcome{"before": old, "with": stamped} {
			id, ok := outcome.ReadTestID(goDir)
			if got := map[bool]string{true: id.String(), false: ""}[ok]; got != expect.id || (ok && id.Granularity() != expect.granularity) {
				t.Errorf("%s %s reads %q (%s), want %q (%s)", name, outcome.Unit, got, id.Granularity(), expect.id, expect.granularity)
			}
		}
	}
	stampTestIDs(&before, goDir)
	encoded, err := encodeBounded(before, MaxDocumentBytes)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "receipt-with-testid.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(raw) {
		t.Fatalf("stamped receipt\n%s\nwant\n%s", encoded, raw)
	}
	if forged := (Outcome{TestID: "not an id", Unit: "go:example.test/m/a"}); func() bool { _, ok := forged.ReadTestID(goDir); return ok }() {
		t.Error("an unparsable testId was read as an ID instead of refused")
	}
}

// CTI-V1-009: GoUnitDir reads the root module path, so a root Go universe
// unit's testId is its repository-relative package directory; a root with
// no go.mod gives no Go testId rather than a guessed one.
func TestCTIV1009GoUnitDirFromRootModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if GoUnitDir(root) != nil || GoUnitDir("") != nil {
		t.Fatal("a root without go.mod resolved Go units")
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/m\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goDir := GoUnitDir(root)
	if got := UnitTestID("go:example.test/m/internal/a", goDir); got != "go:internal/a" {
		t.Errorf("UnitTestID = %q", got)
	}
	if got := UnitTestID(ModuleUnitPrefix+"tools/x", goDir); got != "" {
		t.Errorf("a nested-module unit got testId %q", got)
	}
}
