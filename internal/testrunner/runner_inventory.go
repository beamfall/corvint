// SPDX-License-Identifier: Apache-2.0

package testrunner

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// InventoryKey binds discovery to the checkout, runner configuration and target.
// In particular a host inventory is never substituted for a different target.
type InventoryKey struct {
	Checkout, Runner, ConfigDigest string
	OS, Arch                       string
}

type InventoryTest struct{ ID, Path string }

// RunnerInventory is a core-owned in-process value, not a new wire format.
// Complete must be explicit: the zero value cannot justify narrowing.
type RunnerInventory struct {
	Key      InventoryKey
	Tests    []InventoryTest
	Complete bool
	Reason   string
}

func inventoryText(s string) bool {
	return utf8.ValidString(s) && s != "" && strings.TrimSpace(s) == s && len(s) <= maxSelectionKeyBytes && strings.IndexFunc(s, unicode.IsControl) < 0
}

// CanonicalInventory validates and copies before sorting, so the same discovery
// inputs produce byte-equal inventories without mutating plugin-owned slices.
// An absent, failed, capped or unparseable discovery must carry Complete=false.
func CanonicalInventory(in RunnerInventory, expected InventoryKey) (RunnerInventory, error) {
	if !in.Complete {
		return RunnerInventory{}, fmt.Errorf("inventory-incomplete:%s", in.Reason)
	}
	if in.Reason != "" {
		return RunnerInventory{}, fmt.Errorf("inventory-conflicting-status:%s", in.Reason)
	}
	if in.Key != expected {
		return RunnerInventory{}, fmt.Errorf("inventory-key-mismatch")
	}
	for _, s := range []string{expected.Checkout, expected.Runner, expected.OS, expected.Arch} {
		if !inventoryText(s) {
			return RunnerInventory{}, fmt.Errorf("inventory-key-invalid")
		}
	}
	if b, err := hex.DecodeString(expected.ConfigDigest); err != nil || len(b) != 32 || strings.ToLower(expected.ConfigDigest) != expected.ConfigDigest {
		return RunnerInventory{}, fmt.Errorf("inventory-config-digest-invalid")
	}
	if len(in.Tests) > MaxTests {
		return RunnerInventory{}, fmt.Errorf("inventory-cap-exceeded")
	}
	out := in
	out.Tests = append([]InventoryTest{}, in.Tests...)
	seen := make(map[string]bool, len(out.Tests))
	for _, test := range out.Tests {
		if !inventoryText(test.ID) || !inventoryText(test.Path) || path.IsAbs(test.Path) || path.Clean(test.Path) != test.Path || test.Path == "." || test.Path == ".." || strings.HasPrefix(test.Path, "../") || strings.ContainsAny(test.Path, "\\:") {
			return RunnerInventory{}, fmt.Errorf("inventory-test-invalid")
		}
		if seen[test.ID] {
			return RunnerInventory{}, fmt.Errorf("inventory-test-duplicate:%s", test.ID)
		}
		seen[test.ID] = true
	}
	sort.Slice(out.Tests, func(i, j int) bool { return out.Tests[i].ID < out.Tests[j].ID })
	return out, nil
}

// InventoryBytes is deterministic internal evidence, not a protocol encoder.
func InventoryBytes(in RunnerInventory, expected InventoryKey) ([]byte, error) {
	canonical, err := CanonicalInventory(in, expected)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

// Exclusion is a core selector decision. Fresh means the core checked the
// record against this inventory's exact key; a plugin cannot supply decisions.
type Exclusion struct {
	ID    string
	Fresh bool
}

type RunSet struct {
	Full   bool
	Tests  []string
	Reason string
}

func fullRunSet(in RunnerInventory, reason string) RunSet {
	s := RunSet{Full: true, Reason: reason}
	for _, t := range in.Tests {
		s.Tests = append(s.Tests, t.ID)
	}
	return s
}

// PlanRunSet implements head inventory minus fresh exclusions plus mandates.
// Invalid inputs always widen; FULL commands omit filters even if discovery
// failed and the known test list is empty (RSP-V0-002, RSP-V0-004).
func PlanRunSet(in RunnerInventory, key InventoryKey, exclusions []Exclusion, mandated []string) RunSet {
	inv, err := CanonicalInventory(in, key)
	if err != nil {
		return RunSet{Full: true, Reason: err.Error()}
	}
	if len(exclusions) > MaxTests || len(mandated) > MaxTests {
		return fullRunSet(inv, "selection-cap-exceeded")
	}
	known := map[string]bool{}
	for _, t := range inv.Tests {
		known[t.ID] = true
	}
	excluded := map[string]bool{}
	seen := map[string]bool{}
	for _, e := range exclusions {
		if !known[e.ID] || seen[e.ID] {
			return fullRunSet(inv, "exclusion-invalid:"+e.ID)
		}
		seen[e.ID], excluded[e.ID] = true, e.Fresh
	}
	for _, id := range mandated {
		if !known[id] {
			return fullRunSet(inv, "mandate-not-in-inventory:"+id)
		}
		delete(excluded, id)
	}
	out := RunSet{Tests: []string{}}
	for _, t := range inv.Tests {
		if !excluded[t.ID] {
			out.Tests = append(out.Tests, t.ID)
		}
	}
	return out
}

// RunnerResult uses the runner-plugin outcomes, separate from the frozen
// experimental execution Observation and its uppercase states.
type RunnerResult struct{ ID, Outcome, Reason string }

type Reconciliation struct {
	Succeeded                    bool
	Full                         bool
	NeedsRequalification         bool
	Missing, Unexpected, Reasons []string
}

// ReconcileInventory checks every planned identity exactly once. Superset
// commands may add inventoried results, but can never omit a planned identity.
func ReconcileInventory(in RunnerInventory, key InventoryKey, planned RunSet, results []RunnerResult, superset bool) Reconciliation {
	out := Reconciliation{Succeeded: true}
	fail := func(reason string) {
		out.Succeeded = false
		out.Full = true
		out.NeedsRequalification = true
		out.Reasons = append(out.Reasons, reason)
	}
	inv, err := CanonicalInventory(in, key)
	if err != nil {
		fail(err.Error())
		return out
	}
	if len(results) > MaxTests || len(planned.Tests) > MaxTests {
		fail("result-cap-exceeded")
		return out
	}
	known, required := map[string]bool{}, map[string]bool{}
	for _, t := range inv.Tests {
		known[t.ID] = true
		if planned.Full {
			required[t.ID] = true
		}
	}
	for _, id := range planned.Tests {
		if !known[id] {
			fail("planned-test-not-in-inventory:" + id)
		}
		required[id] = true
	}
	counts := map[string]int{}
	terminal := map[string]bool{}
	for _, r := range results {
		counts[r.ID]++
		if counts[r.ID] > 1 {
			fail("duplicate-result:" + r.ID)
		}
		if !known[r.ID] {
			out.Unexpected = append(out.Unexpected, r.ID)
			fail("uninventoried-result:" + r.ID)
		} else if !required[r.ID] && !superset {
			fail("unplanned-result:" + r.ID)
		}
		switch r.Outcome {
		case "passed":
			terminal[r.ID] = true
		case "failed":
			terminal[r.ID] = true
			out.Succeeded = false
		case "skipped":
			if strings.TrimSpace(r.Reason) != "" {
				terminal[r.ID] = true
			} else {
				fail("skip-reason-missing:" + r.ID)
			}
		default:
			fail("nonterminal-result:" + r.ID)
		}
	}
	for id := range required {
		if !terminal[id] || counts[id] != 1 {
			out.Missing = append(out.Missing, id)
		}
	}
	sort.Strings(out.Missing)
	sort.Strings(out.Unexpected)
	if len(out.Missing) != 0 {
		fail("missing-results:" + strings.Join(out.Missing, ","))
	}
	sort.Strings(out.Reasons)
	return out
}

// InventoryAttempt retains native results, including those before divergence.
type InventoryAttempt struct {
	Plan           RunSet
	Results        []RunnerResult
	Superset       bool
	Reconciliation Reconciliation
}

type InventoryRun struct {
	Attempts             []InventoryAttempt
	Succeeded            bool
	NeedsRequalification bool
}

// RunInventoried is the core reconciliation loop. A missing result permits one
// FULL rerun in the same invocation, or fails when the caller forbids it. Its
// sticky NeedsRequalification output must be retained by the core qualification
// owner; a successful retry does not repair inventory qualification.
//
// RSP-V0-003: a failed outcome reported by ANY attempt is sticky, including one
// returned alongside a runner or report error, an otherwise rejected or
// incomplete report, or a non-canonical spelling that reconciliation refuses.
// A later passing FULL retry can complete the evidence but never turn the run
// into success.
func RunInventoried(in RunnerInventory, key InventoryKey, planned RunSet, allowRerun bool, run func(RunSet) ([]RunnerResult, bool, error)) InventoryRun {
	var out InventoryRun
	failedResult := false
	for attempt := 0; attempt < 2; attempt++ {
		runPlan := planned
		runPlan.Tests = append([]string{}, planned.Tests...)
		results, superset, err := run(runPlan)
		results = append([]RunnerResult(nil), results...)
		for _, result := range results {
			if strings.EqualFold(strings.TrimSpace(result.Outcome), "failed") {
				failedResult = true
			}
		}
		r := ReconcileInventory(in, key, planned, results, superset)
		if err != nil {
			r.Succeeded = false
			r.Full = true
			r.NeedsRequalification = true
			r.Reasons = append(r.Reasons, "runner-failed:"+err.Error())
		}
		out.Attempts = append(out.Attempts, InventoryAttempt{planned, results, superset, r})
		out.NeedsRequalification = out.NeedsRequalification || r.NeedsRequalification
		out.Succeeded = r.Succeeded && !failedResult
		if len(r.Missing) == 0 || !allowRerun || attempt == 1 {
			break
		}
		planned = fullRunSet(in, "missing-results:"+strings.Join(r.Missing, ","))
	}
	return out
}
