package evalrepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
)

// Comparability additions for the offline retrieval harness
// (docs/specs/retrieval-eval-comparability-v0.md). None of them changes the
// report Evaluate returns; they are a separate block for the benchmark runner.

// Purpose names why a run reads a corpus (REC-V0-004).
type Purpose string

const (
	PurposeScore Purpose = "score"
	PurposeTune  Purpose = "tune"
)

const (
	SplitRule          = "atlas-eval-split-v0"
	SplitDev           = "dev"
	SplitHeldout       = "heldout"
	heldoutModulus     = 5
	splitSchemaVersion = 1
	// HeldoutProvenance labels every row of the v0 manifest: each row had been
	// observed during development before it was assigned (REC-V0-007).
	HeldoutProvenance     = "previously-observed"
	OutcomeNotRecorded    = "not-recorded"
	outcomeRecorded       = "recorded"
	budgetYieldDefinition = "recall@B counts a must_include selector as hit when the cumulative serialized weighted bytes of the packet's results, in packet order, through its first occurrence are at most B; area is the arithmetic mean of unrounded recall@B over the ladder"
)

// BudgetLadderBytes is the fixed recall@budget ladder (REC-V0-005).
var BudgetLadderBytes = []int{512, 1024, 2048, 4096, 8192, 16384}

// CaseSplit assigns a row to dev or heldout from its ID alone (REC-V0-001).
func CaseSplit(id string) string {
	digest := sha256.Sum256([]byte(SplitRule + "\x00" + id))
	if binary.BigEndian.Uint64(digest[:8])%heldoutModulus == 0 {
		return SplitHeldout
	}
	return SplitDev
}

// ErrHeldoutRowInTuning is returned when a tuning run reaches a held-out row.
type ErrHeldoutRowInTuning struct{ ID string }

func (err ErrHeldoutRowInTuning) Error() string {
	return fmt.Sprintf("tuning purpose refuses to read held-out row: %s", err.ID)
}

func casesForPurpose(cases []goldenCase, purpose Purpose) ([]goldenCase, int, error) {
	if purpose == PurposeScore {
		return cases, 0, nil
	}
	if purpose != PurposeTune {
		return nil, 0, fmt.Errorf("unknown evaluation purpose: %q", purpose)
	}
	scored := make([]goldenCase, 0, len(cases))
	for _, item := range cases {
		if item.Split == SplitDev {
			scored = append(scored, item)
		}
	}
	return scored, len(cases) - len(scored), nil
}

func requireReadable(purpose Purpose, cases []goldenCase) error {
	if purpose != PurposeTune {
		return nil
	}
	for _, item := range cases {
		if item.Split != SplitDev {
			return ErrHeldoutRowInTuning{ID: item.ID}
		}
	}
	return nil
}

// comparability builds the REC-V0 block from the baseline arm's case scores.
func (counts *counters) comparability(purpose Purpose, withheld int) map[string]any {
	splits := map[string]any{"full": splitMetrics(counts.caseScores, "")}
	if purpose == PurposeScore {
		splits[SplitDev] = splitMetrics(counts.caseScores, SplitDev)
		splits[SplitHeldout] = splitMetrics(counts.caseScores, SplitHeldout)
	}
	return map[string]any{
		"purpose": string(purpose), "split_rule": SplitRule, "heldout_modulus": heldoutModulus,
		"heldout_provenance": HeldoutProvenance, "heldout_rows_withheld": withheld, "splits": splits,
	}
}

// splitCountKeys are the additive counts behind every split metric.
var splitCountKeys = []string{
	"cases", "must_read_hits", "must_read_total", "critical_evidence_misses", "critical_evidence_total",
	"relevant_result_bytes", "total_result_bytes", "must_exclude_violations", "top5_task_hits", "top5_task_total",
	"abstention_hits", "abstention_cases", "epistemic_state_hits", "epistemic_state_cases",
}

func splitMetrics(scores []caseScore, split string) map[string]any {
	counts := map[string]int{}
	hitsAtBudget := make([]int, len(BudgetLadderBytes))
	for _, score := range scores {
		if split != "" && score.split != split {
			continue
		}
		addCaseCounts(counts, score)
		addHitsAtBudget(hitsAtBudget, score.mustHitBytes)
	}
	return renderSplit(counts, hitsAtBudget)
}

func addCaseCounts(counts map[string]int, score caseScore) {
	increments := []int{
		1, score.mustHit, score.mustTotal, score.criticalMisses, score.criticalTotal,
		score.relevantBytes, score.totalResultBytes, score.excludeViolations,
		boolCount(score.top5Hit), boolCount(score.top5Applicable),
		boolCount(score.abstentionHit), boolCount(score.abstentionApplicable),
		boolCount(score.epistemicHit), boolCount(score.epistemicApplicable),
	}
	for position, key := range splitCountKeys {
		counts[key] += increments[position]
	}
}

// MergeSplitMetrics sums one split's metrics across corpora and recomputes
// its ratios and budget yield from the summed counts.
func MergeSplitMetrics(parts []map[string]any) map[string]any {
	counts := map[string]int{}
	hitsAtBudget := make([]int, len(BudgetLadderBytes))
	for _, part := range parts {
		for _, key := range splitCountKeys {
			counts[key] += integer(part[key], 0)
		}
		yield, _ := part["budget_yield"].(map[string]any)
		hits, _ := yield["hits_at_budget"].([]any)
		for position := range min(len(hits), len(hitsAtBudget)) {
			hitsAtBudget[position] += integer(hits[position], 0)
		}
	}
	return renderSplit(counts, hitsAtBudget)
}

func renderSplit(counts map[string]int, hitsAtBudget []int) map[string]any {
	metrics := map[string]any{}
	for _, key := range splitCountKeys {
		metrics[key] = counts[key]
	}
	metrics["recall"] = observedRatio(counts["must_read_hits"], counts["must_read_total"])
	metrics["serialized_result_byte_weighted_precision"] = observedRatio(counts["relevant_result_bytes"], counts["total_result_bytes"])
	metrics["top5_task_success"] = observedRatio(counts["top5_task_hits"], counts["top5_task_total"])
	metrics["abstention_accuracy"] = observedRatio(counts["abstention_hits"], counts["abstention_cases"])
	metrics["epistemic_state_accuracy"] = observedRatio(counts["epistemic_state_hits"], counts["epistemic_state_cases"])
	metrics["budget_yield"] = BudgetYield(hitsAtBudget, counts["must_read_total"])
	return metrics
}

func addHitsAtBudget(hits []int, mustHitBytes []int) {
	for _, cumulative := range mustHitBytes {
		for position, budget := range BudgetLadderBytes {
			hits[position] += boolCount(cumulative <= budget)
		}
	}
}

// BudgetYield reports recall@budget over BudgetLadderBytes and its area (REC-V0-005).
func BudgetYield(hitsAtBudget []int, mustTotal int) map[string]any {
	recalls := make([]any, len(hitsAtBudget))
	hits := make([]any, len(hitsAtBudget))
	hitSum := 0
	for position, value := range hitsAtBudget {
		recalls[position], hits[position] = observedRatio(value, mustTotal), value
		hitSum += value
	}
	ladder := make([]any, len(BudgetLadderBytes))
	for position, budget := range BudgetLadderBytes {
		ladder[position] = budget
	}
	return map[string]any{
		"ladder_bytes": ladder, "must_read_total": mustTotal, "hits_at_budget": hits,
		"recall_at_budget": recalls, "area": observedRatio(hitSum, mustTotal*len(BudgetLadderBytes)),
		"definition": budgetYieldDefinition,
	}
}

// observedRatio abstains (null) rather than inventing a ratio for an empty split.
func observedRatio(numerator, denominator int) any {
	if denominator == 0 {
		return nil
	}
	return ratio(numerator, denominator, 0)
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

// NotRecordedOutcome is the explicit absent downstream outcome (REC-V0-006).
func NotRecordedOutcome() map[string]any { return map[string]any{"state": OutcomeNotRecorded} }

// ValidateDownstreamOutcome accepts exactly {"state":"not-recorded"} or a
// recorded passed/failed outcome naming its evidence (REC-V0-006).
func ValidateDownstreamOutcome(value any) error {
	slot, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("downstream_outcome must be an object")
	}
	switch slot["state"] {
	case OutcomeNotRecorded:
		if len(slot) != 1 {
			return fmt.Errorf("a not-recorded downstream_outcome carries no other field")
		}
		return nil
	case outcomeRecorded:
		outcome, _ := slot["outcome"].(string)
		evidence, _ := slot["evidence"].(string)
		if len(slot) != 3 || (outcome != "passed" && outcome != "failed") || evidence == "" {
			return fmt.Errorf("a recorded downstream_outcome needs outcome passed|failed and non-empty evidence only")
		}
		return nil
	default:
		return fmt.Errorf("downstream_outcome state must be %q or %q", OutcomeNotRecorded, outcomeRecorded)
	}
}

// SplitManifest is the committed partition manifest (REC-V0-002).
type SplitManifest struct {
	SchemaVersion     int           `json:"schema_version"`
	Rule              string        `json:"rule"`
	HeldoutModulus    int           `json:"heldout_modulus"`
	HeldoutProvenance string        `json:"heldout_provenance"`
	Corpora           []SplitCorpus `json:"corpora"`
}

type SplitCorpus struct {
	Cases string     `json:"cases"`
	Rows  []SplitRow `json:"rows"`
}

type SplitRow struct {
	ID        string `json:"id"`
	Split     string `json:"split"`
	RowSHA256 string `json:"row_sha256"`
}

// SplitRows derives one corpus's manifest rows from its bytes, in corpus order.
func SplitRows(raw []byte) ([]SplitRow, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var payload struct {
		Cases []map[string]any `json:"cases"`
	}
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid retrieval golden: %v", err)
	}
	rows := make([]SplitRow, 0, len(payload.Cases))
	for _, item := range payload.Cases {
		id := stringValue(item["id"])
		if id == "" {
			return nil, fmt.Errorf("retrieval golden row has no id")
		}
		encoded, err := canonicalJSON(item)
		if err != nil {
			return nil, err
		}
		rows = append(rows, SplitRow{ID: id, Split: CaseSplit(id), RowSHA256: fmt.Sprintf("%x", sha256.Sum256(encoded))})
	}
	return rows, nil
}

// NewSplitManifest builds the manifest for corpora given as path -> bytes.
func NewSplitManifest(corpora map[string][]byte) (SplitManifest, error) {
	paths := make([]string, 0, len(corpora))
	for path := range corpora {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	manifest := SplitManifest{
		SchemaVersion: splitSchemaVersion, Rule: SplitRule, HeldoutModulus: heldoutModulus,
		HeldoutProvenance: HeldoutProvenance, Corpora: make([]SplitCorpus, 0, len(paths)),
	}
	for _, path := range paths {
		rows, err := SplitRows(corpora[path])
		if err != nil {
			return SplitManifest{}, fmt.Errorf("%s: %w", path, err)
		}
		manifest.Corpora = append(manifest.Corpora, SplitCorpus{Cases: path, Rows: rows})
	}
	return manifest, nil
}

// VerifyCorpus refuses a corpus whose rows differ from the committed manifest
// in ID, order, assignment or content digest. It reports false when the
// manifest does not register the corpus at all.
func (manifest SplitManifest) VerifyCorpus(path string, raw []byte) (bool, error) {
	if manifest.SchemaVersion != splitSchemaVersion || manifest.Rule != SplitRule || manifest.HeldoutModulus != heldoutModulus {
		return false, fmt.Errorf("split manifest does not declare rule %s modulus %d", SplitRule, heldoutModulus)
	}
	for _, corpus := range manifest.Corpora {
		if corpus.Cases != path {
			continue
		}
		rows, err := SplitRows(raw)
		if err != nil {
			return true, err
		}
		if len(rows) != len(corpus.Rows) {
			return true, fmt.Errorf("split manifest row count differs for %s: %d != %d", path, len(corpus.Rows), len(rows))
		}
		for position, row := range rows {
			if row != corpus.Rows[position] {
				return true, fmt.Errorf("split manifest row %d differs for %s: %s", position, path, row.ID)
			}
		}
		return true, nil
	}
	return false, nil
}
