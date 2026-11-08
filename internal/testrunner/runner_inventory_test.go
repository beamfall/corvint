// SPDX-License-Identifier: Apache-2.0

package testrunner

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func inventoryFixture() RunnerInventory {
	return RunnerInventory{Key: InventoryKey{Checkout: "head", Runner: "fixture", ConfigDigest: strings.Repeat("a", 64), OS: "linux", Arch: "amd64"}, Complete: true,
		Tests: []InventoryTest{{"new", "tests/new.go"}, {"old", "tests/old.go"}, {"stale", "tests/stale.go"}}}
}

// RSP-V0-002: repeat discovery has a canonical byte representation, keyed by target.
func TestRSPV0002CanonicalInventory(t *testing.T) {
	in := inventoryFixture()
	first, err := InventoryBytes(in, in.Key)
	if err != nil {
		t.Fatal(err)
	}
	in.Tests[0], in.Tests[2] = in.Tests[2], in.Tests[0]
	second, err := InventoryBytes(in, in.Key)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("unstable inventory: %v", err)
	}
	if in.Tests[0].ID != "stale" {
		t.Fatal("canonicalization mutated input")
	}
	in.Tests = nil
	if _, err := InventoryBytes(in, in.Key); err != nil {
		t.Fatalf("explicit complete empty inventory: %v", err)
	}
}

// RSP-V0-002: failed, truncated, malformed, capped and wrong-target inputs are FULL.
func TestRSPV0002InvalidInventoryFull(t *testing.T) {
	for name, mutate := range map[string]func(*RunnerInventory){
		"missing":      func(i *RunnerInventory) { *i = RunnerInventory{} },
		"truncated":    func(i *RunnerInventory) { i.Complete = false; i.Reason = "truncated" },
		"unparseable":  func(i *RunnerInventory) { i.Complete = false; i.Reason = "unparseable" },
		"conflict":     func(i *RunnerInventory) { i.Reason = "failed" },
		"target-os":    func(i *RunnerInventory) { i.Key.OS = "windows" },
		"target-arch":  func(i *RunnerInventory) { i.Key.Arch = "arm64" },
		"checkout":     func(i *RunnerInventory) { i.Key.Checkout = "other" },
		"runner":       func(i *RunnerInventory) { i.Key.Runner = "other" },
		"config":       func(i *RunnerInventory) { i.Key.ConfigDigest = strings.Repeat("b", 64) },
		"invalid-utf8": func(i *RunnerInventory) { i.Tests[0].ID = string([]byte{255}) },
		"duplicate":    func(i *RunnerInventory) { i.Tests[1].ID = i.Tests[0].ID },
		"missing-path": func(i *RunnerInventory) { i.Tests[0].Path = "" },
		"escape":       func(i *RunnerInventory) { i.Tests[0].Path = "../outside" },
		"windows-path": func(i *RunnerInventory) { i.Tests[0].Path = "C:/outside" },
		"cap":          func(i *RunnerInventory) { i.Tests = make([]InventoryTest, MaxTests+1) },
	} {
		t.Run(name, func(t *testing.T) {
			in := inventoryFixture()
			key := in.Key
			mutate(&in)
			plan := PlanRunSet(in, key, []Exclusion{{"old", true}}, nil)
			if !plan.Full || plan.Reason == "" {
				t.Fatalf("narrowed bad inventory: %+v", plan)
			}
			if _, err := InventoryBytes(in, key); err == nil {
				t.Fatal("invalid evidence admitted")
			}
		})
	}
	for _, key := range []InventoryKey{{}, {Checkout: "head", Runner: "fixture", ConfigDigest: "bad", OS: "linux", Arch: "amd64"}} {
		in := inventoryFixture()
		in.Key = key
		if !PlanRunSet(in, key, nil, nil).Full {
			t.Fatal("invalid key admitted")
		}
	}
}

// RSP-V0-004: only a fresh exclusion subtracts a test; mandates add it back.
func TestRSPV0004ExclusionRunSet(t *testing.T) {
	in := inventoryFixture()
	exclusions := []Exclusion{{"old", true}, {"stale", false}}
	plan := PlanRunSet(in, in.Key, exclusions, nil)
	if plan.Full || !reflect.DeepEqual(plan.Tests, []string{"new", "stale"}) {
		t.Fatalf("%+v", plan)
	}
	plan = PlanRunSet(in, in.Key, exclusions, []string{"old"})
	if plan.Full || !reflect.DeepEqual(plan.Tests, []string{"new", "old", "stale"}) {
		t.Fatalf("mandate lost: %+v", plan)
	}
}

// RSP-V0-004: unknown exclusions/mandates and caps cannot narrow discovery.
func TestRSPV0004InvalidSelectionFull(t *testing.T) {
	in := inventoryFixture()
	for _, plan := range []RunSet{
		PlanRunSet(in, in.Key, []Exclusion{{"not-in-head", true}}, nil),
		PlanRunSet(in, in.Key, []Exclusion{{"old", true}, {"old", false}}, nil),
		PlanRunSet(in, in.Key, make([]Exclusion, MaxTests+1), nil),
		PlanRunSet(in, in.Key, nil, []string{"unknown-mandate"}),
		PlanRunSet(in, in.Key, nil, make([]string, MaxTests+1)),
	} {
		if !plan.Full || plan.Reason == "" || len(plan.Tests) != len(in.Tests) {
			t.Fatalf("%+v", plan)
		}
	}
}

func terminalResults() []RunnerResult {
	return []RunnerResult{{ID: "new", Outcome: "passed"}, {ID: "old", Outcome: "passed"}, {ID: "stale", Outcome: "skipped", Reason: "not supported"}}
}

// RSP-V0-003: a selected or FULL run accounts for each planned terminal result.
func TestRSPV0003TerminalResults(t *testing.T) {
	in := inventoryFixture()
	for _, plan := range []RunSet{{Tests: []string{"new"}}, {Full: true}} {
		results := terminalResults()
		if !plan.Full {
			results = results[:1]
		}
		r := ReconcileInventory(in, in.Key, plan, results, false)
		if !r.Succeeded || r.Full || r.NeedsRequalification {
			t.Fatalf("%+v", r)
		}
	}
	results := terminalResults()
	results[0].Outcome = "failed"
	r := ReconcileInventory(in, in.Key, RunSet{Full: true}, results, false)
	if r.Succeeded || r.NeedsRequalification || len(r.Missing) != 0 {
		t.Fatalf("failure is terminal: %+v", r)
	}
}

// RSP-V0-003: unreported FULL identities, extras, duplicates and nonterminal
// reports are divergence, and skipped without a reason is a missing result.
func TestRSPV0003Divergence(t *testing.T) {
	in := inventoryFixture()
	for name, results := range map[string][]RunnerResult{
		"missing-full":    terminalResults()[:2],
		"extra":           append(terminalResults(), RunnerResult{ID: "extra", Outcome: "passed"}),
		"duplicate":       append(terminalResults(), terminalResults()[0]),
		"unknown-outcome": {{ID: "new", Outcome: "unknown"}},
		"skip-no-reason":  {{ID: "new", Outcome: "skipped"}},
		"capped":          make([]RunnerResult, MaxTests+1),
	} {
		t.Run(name, func(t *testing.T) {
			r := ReconcileInventory(in, in.Key, RunSet{Full: true}, results, false)
			if r.Succeeded || !r.Full || !r.NeedsRequalification || len(r.Reasons) == 0 {
				t.Fatalf("%+v", r)
			}
			if name == "missing-full" && !reflect.DeepEqual(r.Missing, []string{"stale"}) {
				t.Fatalf("%+v", r)
			}
			if name == "extra" && !reflect.DeepEqual(r.Unexpected, []string{"extra"}) {
				t.Fatalf("%+v", r)
			}
		})
	}
}

// RSP-V0-003: missing results rerun FULL once, preserving the original report;
// callers that forbid the retry get a failed run with named missing identities.
func TestRSPV0003FullRetry(t *testing.T) {
	in := inventoryFixture()
	for _, allow := range []bool{true, false} {
		calls := 0
		out := RunInventoried(in, in.Key, RunSet{Tests: []string{"new", "old"}}, allow, func(plan RunSet) ([]RunnerResult, bool, error) {
			calls++
			if calls == 1 {
				return terminalResults()[:1], false, nil
			}
			if !plan.Full {
				t.Fatal("retry narrowed")
			}
			return terminalResults(), false, nil
		})
		if out.Succeeded != allow || !out.NeedsRequalification || !reflect.DeepEqual(out.Attempts[0].Reconciliation.Missing, []string{"old"}) || len(out.Attempts[0].Results) != 1 {
			t.Fatalf("%+v", out)
		}
		if allow && calls != 2 || !allow && calls != 1 {
			t.Fatalf("calls=%d", calls)
		}
	}
	out := RunInventoried(in, in.Key, RunSet{Full: true}, true, func(RunSet) ([]RunnerResult, bool, error) { return nil, false, errors.New("capture-failed") })
	if out.Succeeded || len(out.Attempts) != 2 {
		t.Fatalf("unbounded retry or success: %+v", out)
	}
}

// RSP-V0-004: a reported superset is allowed, a subset is never complete.
func TestRSPV0004SupersetNeverSubset(t *testing.T) {
	in := inventoryFixture()
	plan := RunSet{Tests: []string{"new", "old"}}
	if r := ReconcileInventory(in, in.Key, plan, terminalResults(), true); !r.Succeeded {
		t.Fatalf("superset: %+v", r)
	}
	if r := ReconcileInventory(in, in.Key, plan, terminalResults(), false); r.Succeeded {
		t.Fatal("unreported superset accepted")
	}
	if r := ReconcileInventory(in, in.Key, plan, terminalResults()[:1], true); r.Succeeded || !reflect.DeepEqual(r.Missing, []string{"old"}) {
		t.Fatalf("subset: %+v", r)
	}
}

// RSP-V0-003: a reported failed test stands even if a missing-result retry passes.
func TestRSPV0003RetryPreservesFailure(t *testing.T) {
	in := inventoryFixture()
	calls := 0
	out := RunInventoried(in, in.Key, RunSet{Full: true}, true, func(RunSet) ([]RunnerResult, bool, error) {
		calls++
		if calls == 1 {
			return []RunnerResult{{ID: "new", Outcome: "failed"}}, false, nil
		}
		return terminalResults(), false, nil
	})
	if out.Succeeded || len(out.Attempts) != 2 || !out.NeedsRequalification || out.Attempts[0].Results[0].Outcome != "failed" {
		t.Fatalf("reported failure lost: %+v", out)
	}
}

// RSP-V0-003: a failed row from a rejected or incomplete report stays sticky,
// even when reconciliation refuses its spelling and the FULL retry passes.
func TestRSPV0003RejectedReportFailureStaysSticky(t *testing.T) {
	in := inventoryFixture()
	for _, outcome := range []string{"failed", " FAILED "} {
		calls := 0
		out := RunInventoried(in, in.Key, RunSet{Full: true}, true, func(plan RunSet) ([]RunnerResult, bool, error) {
			calls++
			if calls == 1 {
				return []RunnerResult{{ID: "new", Outcome: outcome}}, false, errors.New("report-rejected:count-mismatch")
			}
			if !plan.Full {
				t.Fatal("retry narrowed")
			}
			return terminalResults(), false, nil
		})
		if out.Succeeded || calls != 2 || len(out.Attempts) != 2 || !out.Attempts[1].Reconciliation.Succeeded || out.Attempts[0].Results[0].Outcome != outcome {
			t.Fatalf("%q: rejected-report failure erased by passing retry: %+v", outcome, out)
		}
	}
}

// RSP-V0-003: plugin-owned slices cannot change the planned identities or
// overwrite results retained from an earlier attempt.
func TestRSPV0003RetainsCorePlanAndResults(t *testing.T) {
	in := inventoryFixture()
	shared := []RunnerResult{{ID: "new", Outcome: "passed"}}
	calls := 0
	out := RunInventoried(in, in.Key, RunSet{Tests: []string{"new", "old"}}, true, func(plan RunSet) ([]RunnerResult, bool, error) {
		calls++
		if calls == 1 {
			plan.Tests[1] = "new"
			return shared, false, nil
		}
		shared[0] = RunnerResult{ID: "mutated", Outcome: "failed"}
		return terminalResults(), false, nil
	})
	if !out.Succeeded || len(out.Attempts) != 2 || !reflect.DeepEqual(out.Attempts[0].Reconciliation.Missing, []string{"old"}) || out.Attempts[0].Results[0].ID != "new" {
		t.Fatalf("plugin mutated core evidence: %+v", out)
	}
}
