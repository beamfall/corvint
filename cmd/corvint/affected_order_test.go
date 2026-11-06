// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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

// AFP-V0-2487: past the bound the most recently modified receipts are kept,
// ties broken by name, so a fresh unsharded test-run.json is never dropped for
// older shard names that sort before it; the kept receipts list in name order.
func TestFailFastOrderKeepsTheMostRecentReceipts_AFPV02487(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	start := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	stamp := func(name string, at time.Time) {
		t.Helper()
		if err := os.Chtimes(filepath.Join(root, ".git", "corvint", name), at, at); err != nil {
			t.Fatal(err)
		}
	}
	names := []string{"test-run.json"}
	for shard := 1; shard <= maxFailFastReceipts; shard++ {
		names = append(names, "test-run."+strconv.Itoa(shard)+".json")
	}
	for index, name := range names {
		writeFailFastReceipt(t, root, name, sessiontest.Outcome{Outcome: sessiontest.OutcomePass, Unit: "go:x"})
		stamp(name, start.Add(time.Duration(index)*time.Second))
	}
	// test-run.json is the newest; test-run.9.json the oldest.
	stamp("test-run.json", start.Add(time.Hour))
	stamp("test-run.9.json", start.Add(-time.Hour))
	read := func() []string {
		t.Helper()
		receipts, unlisted := readFailFastReceipts(filepath.Join(root, ".git"))
		if unlisted != "" {
			t.Fatalf("unlisted %q", unlisted)
		}
		var paths []string
		for _, receipt := range receipts {
			paths = append(paths, receipt.Path)
		}
		return paths
	}
	got := read()
	if len(got) != maxFailFastReceipts || !slices.Contains(got, "corvint/test-run.json") || slices.Contains(got, "corvint/test-run.9.json") || !slices.IsSorted(got) {
		t.Fatalf("kept %v: want the %d newest in name order, test-run.json in, test-run.9.json out", got, maxFailFastReceipts)
	}

	// Equal times break by name: the name sorting last is the one left out.
	for _, name := range names {
		stamp(name, start)
	}
	if got := read(); len(got) != maxFailFastReceipts || slices.Contains(got, "corvint/test-run.json") || !slices.Contains(got, "corvint/test-run.9.json") {
		t.Fatalf("tied: kept %v", got)
	}
}

// AFP-V0-2487: a receipt directory that exists but cannot be listed is
// UNREADABLE with a detail, never NOT_OBSERVED, and the order stays plan order.
func TestFailFastOrderUnlistableDirectoryIsUnreadable_AFPV02487(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not deny a listing here")
	}
	const core, mid = "go:example.com/owned/x/a/core", "go:example.com/owned/x/b/mid"
	root, _ := testSelectionFixture(t, "")
	appendFile(t, filepath.Join(root, "x", "a", "core", "core.go"), "\n// edited\n")
	writeFailFastReceipt(t, root, "test-run.json", sessiontest.Outcome{Outcome: sessiontest.OutcomeFail, Unit: mid})
	directory := filepath.Join(root, ".git", "corvint")
	if err := os.Chmod(directory, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o755) })
	receipt, _, stderr, code := runAffectedCLI(t, root, "--fail-fast-order")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	order := receipt["order"].(map[string]any)
	failures := order["failures"].(map[string]any)
	if failures["state"] != "UNREADABLE" || !strings.Contains(failures["detail"].(string), "corvint/") || !reflect.DeepEqual(failures["receipts"], []any{}) {
		t.Fatalf("failures %v", failures)
	}
	if got := orderedUnits(order); !reflect.DeepEqual(got, []string{core, mid}) {
		t.Fatalf("order %v, want plan order", got)
	}

	// Negative control: an absent directory is NOT_OBSERVED with no detail.
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code = runAffectedCLI(t, root, "--fail-fast-order")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	failures = receipt["order"].(map[string]any)["failures"].(map[string]any)
	if _, detail := failures["detail"]; failures["state"] != "NOT_OBSERVED" || detail {
		t.Fatalf("absent directory: failures %v", failures)
	}
}

func orderedUnits(order map[string]any) []string {
	var units []string
	for _, raw := range order["units"].([]any) {
		units = append(units, raw.(map[string]any)["unitId"].(string))
	}
	return units
}

// TSP-V0-7305: content probes are limited to the 64 newest mtime candidates.
func TestFailFastContentTimeSurvivesTimestampTies(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < maxFailFastReceipts+1; i++ {
		name := fmt.Sprintf("test-run.%02d.json", i)
		body, err := sessiontest.Encode(sessiontest.Receipt{Profile: sessiontest.ReceiptProfile, CompletedAt: "2026-11-04T00:00:00.123456789Z"})
		if err != nil {
			t.Fatal(err)
		}
		writeFailFastFile(t, root, name, body)
		file := filepath.Join(root, ".git", "corvint", name)
		if err := os.Chtimes(file, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	saved := readFailFastContentTime
	t.Cleanup(func() { readFailFastContentTime = saved })
	var probes []string
	readFailFastContentTime = func(name string) (time.Time, bool) {
		probes = append(probes, filepath.Base(name))
		stamp, ok := saved(name)
		if !ok || stamp.Format(time.RFC3339Nano) != "2026-11-04T00:00:00.123456789Z" {
			t.Fatalf("content time %s %v", stamp, ok)
		}
		return stamp, ok
	}
	receipts, detail := readFailFastReceipts(filepath.Join(root, ".git"))
	if detail != "" || len(receipts) != maxFailFastReceipts || len(probes) != maxFailFastReceipts || slices.Contains(probes, "test-run.64.json") {
		t.Fatalf("receipts=%d probes=%v detail=%q", len(receipts), probes, detail)
	}
}

// TSP-V0-7305: repeated metadata errors cannot grow the diagnostic beyond 4096 bytes.
func TestFailFastMetadataDetailIsBounded(t *testing.T) {
	root := t.TempDir()
	writeFailFastReceipt(t, root, "test-run.0.json")
	saved := readFailFastDirectory
	t.Cleanup(func() { readFailFastDirectory = saved })
	readFailFastDirectory = func(dir string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		repeated := make([]os.DirEntry, 1000)
		for i := range repeated {
			repeated[i] = failingInfoEntry{entries[0]}
		}
		return repeated, nil
	}
	_, detail := readFailFastReceipts(filepath.Join(root, ".git"))
	if len(detail) != 4096 {
		t.Fatalf("detail length=%d, want 4096", len(detail))
	}
}

type failingInfoEntry struct{ fs.DirEntry }

func (e failingInfoEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrPermission }

// TSP-V0-7305: an Info failure is disclosed even when it falls outside the retained 64.
func TestFailFastDroppedInfoFailureIsDisclosed(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < maxFailFastReceipts+1; i++ {
		writeFailFastReceipt(t, root, fmt.Sprintf("test-run.%d.json", i))
	}
	saved := readFailFastDirectory
	t.Cleanup(func() { readFailFastDirectory = saved })
	readFailFastDirectory = func(dir string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(dir)
		for i, e := range entries {
			if e.Name() == "test-run.0.json" {
				entries[i] = failingInfoEntry{e}
			}
		}
		return entries, err
	}
	_, detail := readFailFastReceipts(filepath.Join(root, ".git"))
	if !strings.Contains(detail, "test-run.0.json") {
		t.Fatalf("missing disclosure: %q", detail)
	}
}

// TSP-V0-7305 (V1-4472): completion times decide retention within the bounded
// pool, including the unsharded run when all filesystem mtimes tie.
func TestFailFastCompletionTimeDecidesRetention(t *testing.T) {
	root := t.TempDir()
	start := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i <= maxFailFastReceipts; i++ {
		name := fmt.Sprintf("test-run.%02d.json", i)
		completed := start.Add(time.Duration(i) * time.Second)
		if i == maxFailFastReceipts {
			name = "test-run.json"
		}
		body, err := sessiontest.Encode(sessiontest.Receipt{Profile: sessiontest.ReceiptProfile, CompletedAt: completed.Format(time.RFC3339Nano)})
		if err != nil {
			t.Fatal(err)
		}
		writeFailFastFile(t, root, name, body)
		if err := os.Chtimes(filepath.Join(root, ".git", "corvint", name), start, start); err != nil {
			t.Fatal(err)
		}
	}
	saved := readFailFastContentTime
	t.Cleanup(func() { readFailFastContentTime = saved })
	probes := 0
	readFailFastContentTime = func(name string) (time.Time, bool) {
		probes++
		return saved(name)
	}
	receipts, detail := readFailFastReceipts(filepath.Join(root, ".git"))
	var paths []string
	for _, receipt := range receipts {
		if receipt.State != affectedOrderObserved {
			t.Fatalf("not canonical: %+v", receipt)
		}
		paths = append(paths, receipt.Path)
	}
	if detail != "" || len(paths) != maxFailFastReceipts || probes != maxFailFastReceipts+1 || !slices.IsSorted(paths) || !slices.Contains(paths, "corvint/test-run.json") || slices.Contains(paths, "corvint/test-run.00.json") {
		t.Fatalf("retained=%v probes=%d detail=%q", paths, probes, detail)
	}
}
