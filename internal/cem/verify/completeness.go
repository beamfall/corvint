package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// The completeness profile (docs/specs/cem-completeness-profile-v0.md,
// protocol/cem-completeness-1/README.md) is versioned apart from the stable
// reference-integrity verifier, which it neither calls nor changes. It judges
// whether a cem/1.0 map cites every run, retry and required test of an
// independently supplied run inventory (CCP-V0-001).
const (
	CompletenessProfile       = "cem-completeness/1"
	CompletenessResultProfile = "cem-completeness-result/1"
	RunInventoryProfile       = "cem-run-inventory/1"

	CompletenessPass        = "PASS"
	CompletenessRefuse      = "REFUSE"
	CompletenessNotObserved = "NOT_OBSERVED"

	InventoryIndependent  = "INDEPENDENT"
	InventorySelfAuthored = "SELF_AUTHORED"

	// MaxRunInventoryBytes bounds the inventory document (CCP-V0-003).
	MaxRunInventoryBytes = 4 << 20
	maxInventoryRecords  = 4096
)

// CompletenessOptions are the caller-supplied inputs of one completeness
// check. Inventory nil means no inventory was supplied. MapProducer is the
// identity the caller knows produced the map; Target is the target revision
// the caller verifies the map against (CCP-V0-002).
type CompletenessOptions struct {
	ArtifactRoot string
	Inventory    []byte
	MapProducer  string
	Target       string
}

// CompletenessResult is the closed cem-completeness-result/1 document
// (CCP-V0-008). ReferenceIntegrity is always NOT_EVALUATED: a PASS means
// completeness only and is meaningful beside a stable ACCEPT over the same
// MapSha256.
type CompletenessResult struct {
	Profile             string               `json:"profile"`
	CompletenessProfile string               `json:"completenessProfile"`
	Outcome             string               `json:"outcome"`
	MapSha256           string               `json:"mapSha256"`
	ReferenceIntegrity  string               `json:"referenceIntegrity"`
	Inventory           *InventoryProvenance `json:"inventory"`
	Missing             []CompletenessEntry  `json:"missing"`
	NotObserved         []CompletenessEntry  `json:"notObserved"`
}

// InventoryProvenance carries who produced the inventory and whether the
// verifier may treat it as independent of the map producer (CCP-V0-004).
// Identities are caller-asserted strings; Authentication stays NOT_OBSERVED.
type InventoryProvenance struct {
	Sha256         string `json:"sha256"`
	Producer       string `json:"producer"`
	MapProducer    string `json:"mapProducer"`
	Independence   string `json:"independence"`
	Authentication string `json:"authentication"`
}

// CompletenessEntry is one explicit missing-entry refusal or one reason the
// check could not pass (CCP-V0-006, CCP-V0-007). It carries no free text, so
// two implementations produce identical entries.
type CompletenessEntry struct {
	Reason          string `json:"reason"`
	Runner          string `json:"runner,omitempty"`
	Project         string `json:"project,omitempty"`
	Test            string `json:"test,omitempty"`
	ReceiptSha256   string `json:"receiptSha256,omitempty"`
	Attempt         int    `json:"attempt,omitempty"`
	TerminalOutcome string `json:"terminalOutcome,omitempty"`
	Source          string `json:"source,omitempty"`
}

// runInventory is the cem-run-inventory/1 document (CCP-V0-003).
type runInventory struct {
	Profile        string          `json:"profile"`
	Producer       string          `json:"producer"`
	BaseRevision   string          `json:"baseRevision"`
	TargetRevision string          `json:"targetRevision"`
	RequiredTests  []inventoryTest `json:"requiredTests"`
	Runs           []inventoryRun  `json:"runs"`
}
type inventoryTest struct {
	Runner  string `json:"runner"`
	Project string `json:"project"`
	ID      string `json:"id"`
}
type inventoryRun struct {
	Runner        string             `json:"runner"`
	Project       string             `json:"project"`
	Attempt       int                `json:"attempt"`
	ReceiptSha256 string             `json:"receiptSha256"`
	InputFiles    map[string]string  `json:"inputFiles"`
	Outcomes      []inventoryOutcome `json:"outcomes"`
}
type inventoryOutcome struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// terminalStates is the closed corvint-test-runner state set an inventory
// outcome may name; every state other than PASSED is non-passing.
var terminalStates = map[string]bool{"PASSED": true, "FAILED": true, "SKIPPED": true, "FLAKY": true, "TIMED_OUT": true, "INTERRUPTED": true, "UNKNOWN": true, "NOT_REPORTED": true}

// citedRun is one runnerReceipts row of the map as the runner adapter read it.
// Readable is false for an opaque or undecodable row (CCP-V0-005).
type citedRun struct {
	ReceiptSha256 string
	Opaque        bool
	Readable      bool
	Complete      bool
	Runner        string
	Project       string
	Selection     []string
	InputFiles    map[string]string
	Outcomes      map[string]string
}

// Completeness checks the cem/1.0 map raw against the supplied run inventory
// under the cem-completeness/1 profile. It reads only the map, the declared
// artifacts under ArtifactRoot and the inventory; it never runs the
// reference-integrity verifier (CCP-V0-001).
func Completeness(ctx context.Context, raw []byte, o CompletenessOptions) CompletenessResult {
	r := newCompletenessResult(raw)
	if o.Inventory == nil {
		r.NotObserved = append(r.NotObserved, CompletenessEntry{Reason: "inventory-not-supplied"})
		return r.finish()
	}
	inv, provenance, err := decodeRunInventory(o.Inventory, o.MapProducer)
	r.Inventory = provenance
	if err != nil {
		r.Missing = append(r.Missing, CompletenessEntry{Reason: "inventory-invalid"})
		return r.finish()
	}
	document, err := wire.ParseStable(raw)
	if err != nil {
		r.Missing = append(r.Missing, CompletenessEntry{Reason: "map-invalid"})
		return r.finish()
	}
	_, contents, err := stableArtifacts(ctx, o.ArtifactRoot, document.Artifacts)
	if err != nil {
		r.Missing = append(r.Missing, CompletenessEntry{Reason: "artifact-unavailable"})
		return r.finish()
	}
	runs := make([]citedRun, 0, len(document.Receipts))
	for _, row := range document.Receipts {
		runs = append(runs, adaptRunnerReceipt(row.Sha256, row.Profile, contents[row.Sha256], contents[row.PlanSha256]))
	}
	judgeCompleteness(&r, document.Change.BaseRevision, o.Target, runs, inv)
	return r.finish()
}

func newCompletenessResult(raw []byte) CompletenessResult {
	return CompletenessResult{Profile: CompletenessResultProfile, CompletenessProfile: CompletenessProfile, MapSha256: stableDigest(raw), ReferenceIntegrity: "NOT_EVALUATED", Missing: []CompletenessEntry{}, NotObserved: []CompletenessEntry{}}
}

// finish sorts the entries and derives the outcome: any missing entry
// refuses; otherwise PASS needs an independent inventory and no not-observed
// reason (CCP-V0-007).
func (r CompletenessResult) finish() CompletenessResult {
	if r.Inventory != nil {
		switch r.Inventory.Independence {
		case InventorySelfAuthored:
			r.NotObserved = append(r.NotObserved, CompletenessEntry{Reason: "inventory-self-authored"})
		case CompletenessNotObserved:
			r.NotObserved = append(r.NotObserved, CompletenessEntry{Reason: "inventory-independence-not-observed"})
		}
	}
	sortCompletenessEntries(r.Missing)
	sortCompletenessEntries(r.NotObserved)
	switch {
	case len(r.Missing) > 0:
		r.Outcome = CompletenessRefuse
	case len(r.NotObserved) > 0:
		r.Outcome = CompletenessNotObserved
	default:
		r.Outcome = CompletenessPass
	}
	return r
}

func sortCompletenessEntries(entries []CompletenessEntry) {
	slices.SortFunc(entries, func(a, b CompletenessEntry) int {
		for _, c := range [][2]string{{a.Reason, b.Reason}, {a.Runner, b.Runner}, {a.Project, b.Project}, {a.Test, b.Test}, {a.ReceiptSha256, b.ReceiptSha256}, {a.Source, b.Source}, {a.TerminalOutcome, b.TerminalOutcome}} {
			if n := strings.Compare(c[0], c[1]); n != 0 {
				return n
			}
		}
		return a.Attempt - b.Attempt
	})
}

// decodeRunInventory strictly decodes and validates the inventory and derives
// its provenance. An inventory whose producer is the map producer is
// SELF_AUTHORED; with no map producer supplied, independence is NOT_OBSERVED
// (CCP-V0-004). The inventory never states its own independence.
func decodeRunInventory(raw []byte, mapProducer string) (runInventory, *InventoryProvenance, error) {
	provenance := &InventoryProvenance{Sha256: stableDigest(raw), MapProducer: mapProducer, Independence: CompletenessNotObserved, Authentication: CompletenessNotObserved}
	var inv runInventory
	if len(raw) > MaxRunInventoryBytes {
		return inv, provenance, errors.New("inventory bound")
	}
	if _, err := wire.Parse(raw); err != nil {
		return inv, provenance, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&inv); err != nil {
		return inv, provenance, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return inv, provenance, errors.New("trailing data")
	}
	if err := validateRunInventory(inv); err != nil {
		return inv, provenance, err
	}
	provenance.Producer = inv.Producer
	switch {
	case mapProducer == "":
	case inv.Producer == mapProducer:
		provenance.Independence = InventorySelfAuthored
	default:
		provenance.Independence = InventoryIndependent
	}
	return inv, provenance, nil
}

func validateRunInventory(inv runInventory) error {
	if inv.Profile != RunInventoryProfile || inv.Producer == "" || !inventoryRevision(inv.BaseRevision) || !inventoryRevision(inv.TargetRevision) {
		return errors.New("inventory header")
	}
	if inv.RequiredTests == nil || inv.Runs == nil || len(inv.RequiredTests)+len(inv.Runs) > maxInventoryRecords {
		return errors.New("inventory lists")
	}
	required := map[inventoryTest]bool{}
	for _, t := range inv.RequiredTests {
		if t.Runner == "" || t.Project == "" || t.ID == "" || required[t] {
			return fmt.Errorf("required test %q", t.ID)
		}
		required[t] = true
	}
	receipts := map[string]bool{}
	attempts := map[string]bool{}
	for _, run := range inv.Runs {
		key := run.Runner + "\x00" + run.Project + "\x00" + fmt.Sprint(run.Attempt)
		if run.Runner == "" || run.Project == "" || run.Attempt < 1 || !validDigestHex(run.ReceiptSha256) || receipts[run.ReceiptSha256] || attempts[key] || run.InputFiles == nil || len(run.Outcomes) == 0 {
			return fmt.Errorf("run %s", run.ReceiptSha256)
		}
		receipts[run.ReceiptSha256], attempts[key] = true, true
		for path, digest := range run.InputFiles {
			if path == "" || !validDigestHex(digest) {
				return fmt.Errorf("run %s input %q", run.ReceiptSha256, path)
			}
		}
		ids := map[string]bool{}
		for _, outcome := range run.Outcomes {
			if outcome.ID == "" || ids[outcome.ID] || !terminalStates[outcome.State] {
				return fmt.Errorf("run %s outcome %q", run.ReceiptSha256, outcome.ID)
			}
			ids[outcome.ID] = true
		}
	}
	return nil
}

func inventoryRevision(s string) bool { return (len(s) == 40 || len(s) == 64) && lowerHexOnly(s) }
func validDigestHex(s string) bool    { return len(s) == 64 && lowerHexOnly(s) }
func lowerHexOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
	}
	return true
}

// adaptRunnerReceipt is the runner adapter: it reads a row labelled
// corvint-test-runner-receipt/0 whose receipt and plan decode under the
// corvint-test-runner receipt and plan profiles. An opaque row is never read
// as a runner receipt (CEM-V1-310); an undecodable one is not readable
// (CCP-V0-005).
func adaptRunnerReceipt(digest, label string, receiptBytes, planBytes []byte) citedRun {
	run := citedRun{ReceiptSha256: digest}
	if label != wire.StableRunnerReceiptProfile {
		run.Opaque = true
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
	if json.Unmarshal(planBytes, &plan) != nil || json.Unmarshal(receiptBytes, &receipt) != nil || plan.Profile != "corvint-test-runner-plan/0" || receipt.Profile != wire.StableRunnerReceiptProfile || plan.Request.Runner == "" || plan.Request.Runner != receipt.Observation.Runner {
		return run
	}
	run.Readable, run.Complete = true, receipt.Observation.Complete
	run.Runner, run.Project, run.Selection = plan.Request.Runner, plan.Request.Project, plan.Request.ExpectedTests
	run.InputFiles = plan.Request.InputFiles
	if run.InputFiles == nil {
		run.InputFiles = map[string]string{}
	}
	run.Outcomes = map[string]string{}
	for _, t := range receipt.Observation.Tests {
		run.Outcomes[t.ID] = t.State
	}
	return run
}

// judgeCompleteness compares the cited runs with the inventory (CCP-V0-006).
func judgeCompleteness(r *CompletenessResult, base, target string, runs []citedRun, inv runInventory) {
	missing := func(e CompletenessEntry) { r.Missing = append(r.Missing, e) }
	unobserved := func(e CompletenessEntry) { r.NotObserved = append(r.NotObserved, e) }
	if inv.BaseRevision != base {
		missing(CompletenessEntry{Reason: "source-identity-mismatch", Source: "baseRevision"})
	}
	switch {
	case target == "":
		unobserved(CompletenessEntry{Reason: "target-not-supplied"})
	case inv.TargetRevision != target:
		missing(CompletenessEntry{Reason: "source-identity-mismatch", Source: "targetRevision"})
	}
	cited := map[string]citedRun{}
	unreadable := false
	for _, run := range runs {
		cited[run.ReceiptSha256] = run
		switch {
		case run.Opaque:
			unreadable = true
			unobserved(CompletenessEntry{Reason: "receipt-opaque", ReceiptSha256: run.ReceiptSha256})
		case !run.Readable:
			unreadable = true
			unobserved(CompletenessEntry{Reason: "receipt-not-readable", ReceiptSha256: run.ReceiptSha256})
		case !run.Complete:
			unobserved(CompletenessEntry{Reason: "receipt-incomplete", Runner: run.Runner, Project: run.Project, ReceiptSha256: run.ReceiptSha256})
		}
	}
	listed := map[string]bool{}
	for _, want := range inv.Runs {
		listed[want.ReceiptSha256] = true
		at := CompletenessEntry{Runner: want.Runner, Project: want.Project, ReceiptSha256: want.ReceiptSha256, Attempt: want.Attempt}
		got, ok := cited[want.ReceiptSha256]
		if !ok {
			failed := false
			for _, outcome := range want.Outcomes {
				if outcome.State != "PASSED" {
					failed = true
					e := at
					e.Reason, e.Test, e.TerminalOutcome = "failed-run-omitted", outcome.ID, outcome.State
					missing(e)
				}
			}
			if !failed {
				e := at
				e.Reason = "receipt-missing"
				missing(e)
			}
			continue
		}
		if !got.Readable {
			continue
		}
		if got.Runner != want.Runner || got.Project != want.Project {
			e := at
			e.Reason = "run-identity-mismatch"
			missing(e)
			continue
		}
		for _, path := range unionKeys(want.InputFiles, got.InputFiles) {
			if want.InputFiles[path] != got.InputFiles[path] {
				e := at
				e.Reason, e.Source = "source-identity-mismatch", "inputFiles:"+path
				missing(e)
			}
		}
		states := map[string]string{}
		for _, outcome := range want.Outcomes {
			states[outcome.ID] = outcome.State
		}
		for _, id := range unionKeys(states, got.Outcomes) {
			if states[id] != got.Outcomes[id] {
				e := at
				e.Reason, e.Test, e.TerminalOutcome = "terminal-outcome-mismatch", id, states[id]
				missing(e)
			}
		}
	}
	for _, run := range runs {
		if !listed[run.ReceiptSha256] {
			missing(CompletenessEntry{Reason: "run-not-in-inventory", Runner: run.Runner, Project: run.Project, ReceiptSha256: run.ReceiptSha256})
		}
	}
	for _, req := range inv.RequiredTests {
		selected, recorded := false, false
		for _, run := range runs {
			if !run.Readable || run.Runner != req.Runner || run.Project != req.Project {
				continue
			}
			if slices.Contains(run.Selection, req.ID) {
				selected = true
			}
			if _, ok := run.Outcomes[req.ID]; ok {
				recorded = true
			}
		}
		at := CompletenessEntry{Runner: req.Runner, Project: req.Project, Test: req.ID}
		switch {
		case selected && recorded:
		case unreadable:
			at.Reason = "selection-not-observed"
			unobserved(at)
		case !selected:
			at.Reason = "selection-narrowed"
			missing(at)
		default:
			at.Reason = "receipt-missing"
			missing(at)
		}
	}
}

func unionKeys(a, b map[string]string) []string {
	keys := make([]string, 0, len(a)+len(b))
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
