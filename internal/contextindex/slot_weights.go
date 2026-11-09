package contextindex

import (
	"encoding/json"
	"sort"
)

// Learned slot weights (learned-trace-admission-v0, LTA-V0-009..012). The
// live context path reads only the admitted file below; it never opens the
// self-observation or unplanned-read ledger (AGENTS.md invariant 4).
const (
	// SlotWeightsPath is the admitted learned slot-weight trace, beside the
	// local trace store.
	SlotWeightsPath = ".context-corvint/slot-weights.json"
	SlotWeightMin   = -2
	SlotWeightMax   = 2
)

// LearnableSlots is TCP-V0-004's slot order without the reserved relations:
// the closed set a learned weight may reorder.
var LearnableSlots = []string{"pair", "mentioned", "definition", "reverse-import", "reference", "cochange", "sibling", "test", "lexical", "documentation"}

// SlotWeights maps a learnable relation to an integer in
// [SlotWeightMin, SlotWeightMax]; an absent relation weighs zero.
type SlotWeights map[string]int

// AdmittedSlotWeights is one gate-admitted learned slot-weight trace and the
// sha256 of the exact file bytes that carried it.
type AdmittedSlotWeights struct {
	Weights SlotWeights
	SHA256  string
}

// SlotWeightsFile is the admitted file's wire shape.
type SlotWeightsFile struct {
	SchemaVersion int             `json:"schemaVersion"`
	Weights       SlotWeights     `json:"weights"`
	Evaluation    json.RawMessage `json:"evaluation"`
}

// orderBySlotWeight stably moves rows of a higher-weighted relation ahead,
// before corroboration and truncation (LTA-V0-011). Without weights the slot
// order is unchanged.
func orderBySlotWeight(rows []contextRow, weights SlotWeights) []contextRow {
	if len(weights) == 0 {
		return rows
	}
	sort.SliceStable(rows, func(left, right int) bool {
		return weights[rows[left].kind] > weights[rows[right].kind]
	})
	return rows
}
