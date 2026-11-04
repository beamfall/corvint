// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

func writeFailFastReceipt(t *testing.T, root, name string, outcomes ...sessiontest.Outcome) {
	t.Helper()
	body, err := sessiontest.Encode(sessiontest.Receipt{Profile: sessiontest.ReceiptProfile, Revision: "fixture",
		Invocations: []sessiontest.Invocation{{Adapter: sessiontest.AdapterGo, Outcomes: outcomes}}})
	if err != nil {
		t.Fatal(err)
	}
	writeFailFastFile(t, root, name, body)
}

func writeFailFastFile(t *testing.T, root, name string, body []byte) {
	t.Helper()
	path := filepath.Join(root, ".git", "corvint", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// AFP-V0-2487: --fail-fast-order puts a unit a recent local receipt records as
// failed first, keeps plan order otherwise, discloses its key and every
// receipt it read, and changes no other member; without receipts the state is
// NOT_OBSERVED, and the flag refuses a document it does not order.
func TestFailFastOrderPutsRecentFailuresFirst_AFPV02487(t *testing.T) {
	t.Parallel()
	const core, mid = "go:example.com/owned/x/a/core", "go:example.com/owned/x/b/mid"
	root, _ := testSelectionFixture(t, "")
	appendFile(t, filepath.Join(root, "x", "a", "core", "core.go"), "\n// edited\n")

	// No receipt: NOT_OBSERVED, plan order.
	receipt, _, stderr, code := runAffectedCLI(t, root, "--fail-fast-order")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	order := receipt["order"].(map[string]any)
	failures := order["failures"].(map[string]any)
	if failures["state"] != "NOT_OBSERVED" || !reflect.DeepEqual(failures["receipts"], []any{}) {
		t.Fatalf("failures without a receipt: %v", failures)
	}
	if got := orderedUnits(order); !reflect.DeepEqual(got, []string{core, mid}) {
		t.Fatalf("order without a receipt %v, want plan order", got)
	}

	writeFailFastReceipt(t, root, "test-run.json", sessiontest.Outcome{Outcome: sessiontest.OutcomePass, Unit: core})
	writeFailFastReceipt(t, root, "test-run.2.json", sessiontest.Outcome{Outcome: sessiontest.OutcomeFail, Unit: mid},
		sessiontest.Outcome{Outcome: sessiontest.OutcomeTimedOut, Unit: "go:example.com/owned/x/unselected"})
	writeFailFastFile(t, root, "test-run.3.json", []byte(`{"profile": "corvint-test-run/0"}`))
	writeFailFastReceipt(t, root, "test-run.old.json", sessiontest.Outcome{Outcome: sessiontest.OutcomeFail, Unit: core})

	plain, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	ordered, _, stderr, code := runAffectedCLI(t, root, "--fail-fast-order")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	order = ordered["order"].(map[string]any)
	delete(ordered, "order")
	if !reflect.DeepEqual(ordered, plain) {
		t.Fatal("--fail-fast-order changed a member other than order")
	}
	if !reflect.DeepEqual(order["key"], []any{"recent-local-failure", "plan-order"}) {
		t.Fatalf("order key %v", order["key"])
	}
	if got := orderedUnits(order); !reflect.DeepEqual(got, []string{mid, core}) {
		t.Fatalf("order %v, want the recent failure first", got)
	}
	units := order["units"].([]any)
	if first := units[0].(map[string]any); first["recentFailure"] != true || first["planIndex"] != float64(1) {
		t.Fatalf("first unit %v", first)
	}
	failures = order["failures"].(map[string]any)
	if failures["state"] != "OBSERVED" {
		t.Fatalf("failures state %v", failures["state"])
	}
	var read []string
	for _, raw := range failures["receipts"].([]any) {
		entry := raw.(map[string]any)
		read = append(read, entry["path"].(string)+"="+entry["state"].(string))
	}
	// test-run.old.json is no receipt name `corvint test` writes.
	if want := []string{"corvint/test-run.2.json=OBSERVED", "corvint/test-run.3.json=UNREADABLE", "corvint/test-run.json=OBSERVED"}; !reflect.DeepEqual(read, want) {
		t.Fatalf("receipts read %v, want %v", read, want)
	}

	for _, combination := range [][]string{
		{"--fail-fast-order", "--full"}, {"--fail-fast-order", "--summary"}, {"--fail-fast-order", "--format", "text"},
		{"--fail-fast-order", "--fail-fast-order"}, {"--fail-fast-order=yes"},
	} {
		if _, _, stderr, code := runAffectedCLI(t, root, combination...); code != 2 || !strings.Contains(stderr, "--fail-fast-order") {
			t.Fatalf("%v: exit %d, stderr %s", combination, code, stderr)
		}
	}
}

func orderedUnits(order map[string]any) []string {
	var units []string
	for _, raw := range order["units"].([]any) {
		units = append(units, raw.(map[string]any)["unitId"].(string))
	}
	return units
}
