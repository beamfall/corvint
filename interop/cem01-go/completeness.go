package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

// Portable reader of the cem-completeness/1 profile
// (protocol/cem-completeness-1/README.md). It is versioned apart from the
// stable reference-integrity reader, which it neither calls nor changes, and
// must produce the same closed result as the native verifier for every public
// vector (CCP-V0-009).
const (
	completenessProfile       = "cem-completeness/1"
	completenessResultProfile = "cem-completeness-result/1"
	runInventoryProfile       = "cem-run-inventory/1"
	maxRunInventoryBytes      = 4 << 20
	maxInventoryRecords       = 4096
)

type completenessEntry struct {
	Reason          string `json:"reason"`
	Runner          string `json:"runner,omitempty"`
	Project         string `json:"project,omitempty"`
	Test            string `json:"test,omitempty"`
	ReceiptSHA256   string `json:"receiptSha256,omitempty"`
	Attempt         int    `json:"attempt,omitempty"`
	TerminalOutcome string `json:"terminalOutcome,omitempty"`
	Source          string `json:"source,omitempty"`
}

type inventoryProvenance struct {
	SHA256         string `json:"sha256"`
	Producer       string `json:"producer"`
	MapProducer    string `json:"mapProducer"`
	Independence   string `json:"independence"`
	Authentication string `json:"authentication"`
}

type completenessResult struct {
	Profile             string               `json:"profile"`
	CompletenessProfile string               `json:"completenessProfile"`
	Outcome             string               `json:"outcome"`
	MapSHA256           string               `json:"mapSha256"`
	ReferenceIntegrity  string               `json:"referenceIntegrity"`
	Inventory           *inventoryProvenance `json:"inventory"`
	Missing             []completenessEntry  `json:"missing"`
	NotObserved         []completenessEntry  `json:"notObserved"`
}

type runInventory struct {
	Profile        string `json:"profile"`
	Producer       string `json:"producer"`
	BaseRevision   string `json:"baseRevision"`
	TargetRevision string `json:"targetRevision"`
	RequiredTests  []struct {
		Runner  string `json:"runner"`
		Project string `json:"project"`
		ID      string `json:"id"`
	} `json:"requiredTests"`
	Runs []inventoryRun `json:"runs"`
}

type inventoryRun struct {
	Runner        string            `json:"runner"`
	Project       string            `json:"project"`
	Attempt       int               `json:"attempt"`
	ReceiptSHA256 string            `json:"receiptSha256"`
	InputFiles    map[string]string `json:"inputFiles"`
	Outcomes      []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	} `json:"outcomes"`
}

var inventoryStates = map[string]bool{"PASSED": true, "FAILED": true, "SKIPPED": true, "FLAKY": true, "TIMED_OUT": true, "INTERRUPTED": true, "UNKNOWN": true, "NOT_REPORTED": true}

// readCompleteness is one cited runnerReceipts row as the runner adapter read
// it; readable is false for an opaque or undecodable row.
type readCompleteness struct {
	digest, runner, project    string
	opaque, readable, complete bool
	selection                  []string
	inputs, outcomes           map[string]string
}

// verifyCompleteness checks the map against the inventory. inventory nil means
// none was supplied; mapProducer and target are caller-asserted.
func verifyCompleteness(ctx context.Context, raw []byte, root string, inventory []byte, mapProducer, target string) completenessResult {
	r := completenessResult{Profile: completenessResultProfile, CompletenessProfile: completenessProfile, MapSHA256: shaHex(raw), ReferenceIntegrity: "NOT_EVALUATED", Missing: []completenessEntry{}, NotObserved: []completenessEntry{}}
	if inventory == nil {
		r.NotObserved = append(r.NotObserved, completenessEntry{Reason: "inventory-not-supplied"})
		return finishCompleteness(r)
	}
	r.Inventory = &inventoryProvenance{SHA256: shaHex(inventory), MapProducer: mapProducer, Independence: "NOT_OBSERVED", Authentication: "NOT_OBSERVED"}
	inv, err := decodeRunInventory(inventory)
	if err != nil {
		r.Missing = append(r.Missing, completenessEntry{Reason: "inventory-invalid"})
		return finishCompleteness(r)
	}
	r.Inventory.Producer = inv.Producer
	if mapProducer != "" {
		r.Inventory.Independence = "INDEPENDENT"
		if inv.Producer == mapProducer {
			r.Inventory.Independence = "SELF_AUTHORED"
		}
	}
	m, e := decodeStable(raw)
	if e != nil {
		r.Missing = append(r.Missing, completenessEntry{Reason: "map-invalid"})
		return finishCompleteness(r)
	}
	_, contents, e := checkStableArtifactContents(ctx, root, m.Artifacts, readStableArtifact)
	if e != nil {
		r.Missing = append(r.Missing, completenessEntry{Reason: "artifact-unavailable"})
		return finishCompleteness(r)
	}
	runs := []readCompleteness{}
	for _, row := range m.RunnerReceipts {
		runs = append(runs, adaptCompletenessReceipt(row, contents[row.SHA256], contents[row.PlanSHA256]))
	}
	judgeInventory(&r, m.BaseRevision, target, runs, inv)
	return finishCompleteness(r)
}

func finishCompleteness(r completenessResult) completenessResult {
	if r.Inventory != nil {
		switch r.Inventory.Independence {
		case "SELF_AUTHORED":
			r.NotObserved = append(r.NotObserved, completenessEntry{Reason: "inventory-self-authored"})
		case "NOT_OBSERVED":
			r.NotObserved = append(r.NotObserved, completenessEntry{Reason: "inventory-independence-not-observed"})
		}
	}
	for _, entries := range [][]completenessEntry{r.Missing, r.NotObserved} {
		slices.SortFunc(entries, func(a, b completenessEntry) int {
			if n := slices.Compare([]string{a.Reason, a.Runner, a.Project, a.Test, a.ReceiptSHA256, a.Source, a.TerminalOutcome}, []string{b.Reason, b.Runner, b.Project, b.Test, b.ReceiptSHA256, b.Source, b.TerminalOutcome}); n != 0 {
				return n
			}
			return a.Attempt - b.Attempt
		})
	}
	r.Outcome = "PASS"
	if len(r.NotObserved) > 0 {
		r.Outcome = "NOT_OBSERVED"
	}
	if len(r.Missing) > 0 {
		r.Outcome = "REFUSE"
	}
	return r
}

func decodeRunInventory(raw []byte) (runInventory, error) {
	var inv runInventory
	if len(raw) > maxRunInventoryBytes || strictJSON(raw) != nil {
		return inv, errors.New("inventory json")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&inv); err != nil {
		return inv, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return inv, errors.New("trailing")
	}
	revision := func(s string) bool { return (len(s) == 40 || len(s) == 64) && lowerHex(s) }
	if inv.Profile != runInventoryProfile || inv.Producer == "" || !revision(inv.BaseRevision) || !revision(inv.TargetRevision) || inv.RequiredTests == nil || inv.Runs == nil || len(inv.RequiredTests)+len(inv.Runs) > maxInventoryRecords {
		return inv, errors.New("inventory header")
	}
	seen := map[string]bool{}
	for _, t := range inv.RequiredTests {
		key := "t\x00" + t.Runner + "\x00" + t.Project + "\x00" + t.ID
		if t.Runner == "" || t.Project == "" || t.ID == "" || seen[key] {
			return inv, errors.New("required test")
		}
		seen[key] = true
	}
	for _, run := range inv.Runs {
		attempt := fmt.Sprintf("a\x00%s\x00%s\x00%d", run.Runner, run.Project, run.Attempt)
		receipt := "r\x00" + run.ReceiptSHA256
		if run.Runner == "" || run.Project == "" || run.Attempt < 1 || !validDigest(run.ReceiptSHA256) || seen[receipt] || seen[attempt] || run.InputFiles == nil || len(run.Outcomes) == 0 {
			return inv, errors.New("run")
		}
		seen[receipt], seen[attempt] = true, true
		for path, digest := range run.InputFiles {
			if path == "" || !validDigest(digest) {
				return inv, errors.New("run input")
			}
		}
		ids := map[string]bool{}
		for _, o := range run.Outcomes {
			if o.ID == "" || ids[o.ID] || !inventoryStates[o.State] {
				return inv, errors.New("run outcome")
			}
			ids[o.ID] = true
		}
	}
	return inv, nil
}

// adaptCompletenessReceipt is the runner adapter: only a row labelled
// corvint-test-runner-receipt/0 is read as a runner receipt (CEM-V1-310).
func adaptCompletenessReceipt(row stableRunner, receiptBytes, planBytes []byte) readCompleteness {
	run := readCompleteness{digest: row.SHA256}
	if row.Profile != "corvint-test-runner-receipt/0" {
		run.opaque = true
		return run
	}
	var plan struct {
		Profile string `json:"profile"`
		Request struct {
			Runner        string            `json:"runner"`
			Project       string            `json:"project"`
			ExpectedTests []string          `json:"expectedTests"`
			InputFiles    map[string]string `json:"inputFiles"`
		} `json:"request"`
	}
	var receipt struct {
		Profile     string `json:"profile"`
		Observation struct {
			Runner string `json:"runner"`
			Tests  []struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"tests"`
			Complete bool `json:"complete"`
		} `json:"observation"`
	}
	if json.Unmarshal(planBytes, &plan) != nil || json.Unmarshal(receiptBytes, &receipt) != nil || plan.Profile != "corvint-test-runner-plan/0" || receipt.Profile != "corvint-test-runner-receipt/0" || plan.Request.Runner == "" || plan.Request.Runner != receipt.Observation.Runner {
		return run
	}
	run.readable, run.complete = true, receipt.Observation.Complete
	run.runner, run.project, run.selection = plan.Request.Runner, plan.Request.Project, plan.Request.ExpectedTests
	run.inputs = map[string]string{}
	for k, v := range plan.Request.InputFiles {
		run.inputs[k] = v
	}
	run.outcomes = map[string]string{}
	for _, t := range receipt.Observation.Tests {
		run.outcomes[t.ID] = t.State
	}
	return run
}

func judgeInventory(r *completenessResult, base, target string, runs []readCompleteness, inv runInventory) {
	refuse := func(e completenessEntry) { r.Missing = append(r.Missing, e) }
	unobserved := func(e completenessEntry) { r.NotObserved = append(r.NotObserved, e) }
	if inv.BaseRevision != base {
		refuse(completenessEntry{Reason: "source-identity-mismatch", Source: "baseRevision"})
	}
	if target == "" {
		unobserved(completenessEntry{Reason: "target-not-supplied"})
	} else if inv.TargetRevision != target {
		refuse(completenessEntry{Reason: "source-identity-mismatch", Source: "targetRevision"})
	}
	cited := map[string]readCompleteness{}
	anyUnreadable := false
	for _, run := range runs {
		cited[run.digest] = run
		if run.opaque {
			anyUnreadable = true
			unobserved(completenessEntry{Reason: "receipt-opaque", ReceiptSHA256: run.digest})
		} else if !run.readable {
			anyUnreadable = true
			unobserved(completenessEntry{Reason: "receipt-not-readable", ReceiptSHA256: run.digest})
		} else if !run.complete {
			unobserved(completenessEntry{Reason: "receipt-incomplete", Runner: run.runner, Project: run.project, ReceiptSHA256: run.digest})
		}
	}
	listed := map[string]bool{}
	for _, want := range inv.Runs {
		listed[want.ReceiptSHA256] = true
		entry := func(reason string) completenessEntry {
			return completenessEntry{Reason: reason, Runner: want.Runner, Project: want.Project, ReceiptSHA256: want.ReceiptSHA256, Attempt: want.Attempt}
		}
		got, ok := cited[want.ReceiptSHA256]
		if !ok {
			failed := false
			for _, o := range want.Outcomes {
				if o.State != "PASSED" {
					failed = true
					e := entry("failed-run-omitted")
					e.Test, e.TerminalOutcome = o.ID, o.State
					refuse(e)
				}
			}
			if !failed {
				refuse(entry("receipt-missing"))
			}
			continue
		}
		if !got.readable {
			continue
		}
		if got.runner != want.Runner || got.project != want.Project {
			refuse(entry("run-identity-mismatch"))
			continue
		}
		for _, path := range sortedUnion(want.InputFiles, got.inputs) {
			if want.InputFiles[path] != got.inputs[path] {
				e := entry("source-identity-mismatch")
				e.Source = "inputFiles:" + path
				refuse(e)
			}
		}
		states := map[string]string{}
		for _, o := range want.Outcomes {
			states[o.ID] = o.State
		}
		for _, id := range sortedUnion(states, got.outcomes) {
			if states[id] != got.outcomes[id] {
				e := entry("terminal-outcome-mismatch")
				e.Test, e.TerminalOutcome = id, states[id]
				refuse(e)
			}
		}
	}
	for _, run := range runs {
		if !listed[run.digest] {
			refuse(completenessEntry{Reason: "run-not-in-inventory", Runner: run.runner, Project: run.project, ReceiptSHA256: run.digest})
		}
	}
	for _, req := range inv.RequiredTests {
		selected, recorded := false, false
		for _, run := range runs {
			if run.readable && run.runner == req.Runner && run.project == req.Project {
				selected = selected || slices.Contains(run.selection, req.ID)
				_, ok := run.outcomes[req.ID]
				recorded = recorded || ok
			}
		}
		e := completenessEntry{Runner: req.Runner, Project: req.Project, Test: req.ID}
		switch {
		case selected && recorded:
			continue
		case anyUnreadable:
			e.Reason = "selection-not-observed"
			unobserved(e)
		case !selected:
			e.Reason = "selection-narrowed"
			refuse(e)
		default:
			e.Reason = "receipt-missing"
			refuse(e)
		}
	}
}

func sortedUnion(a, b map[string]string) []string {
	keys := []string{}
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}
