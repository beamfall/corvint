package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
)

// Learned slot weights (learned-trace-admission-v0, LTA-V0-009..012). The
// live context path reads only the admitted file below; it never opens the
// self-observation or unplanned-read ledger (AGENTS.md invariant 4).
const (
	// SlotWeightsPath is the admitted learned slot-weight trace, beside the
	// local trace store.
	SlotWeightsPath     = ".context-corvint/slot-weights.json"
	SlotWeightMin       = -2
	SlotWeightMax       = 2
	maxSlotWeightsBytes = 4096
	slotWeightsSchema   = 1
)

// LearnableSlots is TCP-V0-004's slot order without the reserved relations:
// the closed set a learned weight may reorder.
var LearnableSlots = []string{"pair", "mentioned", "definition", "reverse-import", "reference", "cochange", "sibling", "test", "lexical"}

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

// ValidateSlotWeights refuses an unknown relation or an out-of-range weight.
func ValidateSlotWeights(weights SlotWeights) error {
	for relation, weight := range weights {
		if !slices.Contains(LearnableSlots, relation) {
			return fmt.Errorf("slot weight names an unknown relation: %q", relation)
		}
		if weight < SlotWeightMin || weight > SlotWeightMax {
			return fmt.Errorf("slot weight for %s is outside %d..%d", relation, SlotWeightMin, SlotWeightMax)
		}
	}
	return nil
}

// LoadAdmittedSlotWeights reads the admitted trace under root. An absent file
// is nil with no error (the default order); a symlink, oversized, malformed
// or out-of-range file fails closed with the rollback command named.
func LoadAdmittedSlotWeights(root string) (*AdmittedSlotWeights, error) {
	path := filepath.Join(root, filepath.FromSlash(SlotWeightsPath))
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, slotWeightsRefusal(err.Error())
	}
	if !info.Mode().IsRegular() {
		return nil, slotWeightsRefusal("not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, slotWeightsRefusal(err.Error())
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxSlotWeightsBytes+1))
	if err != nil {
		return nil, slotWeightsRefusal(err.Error())
	}
	if len(raw) > maxSlotWeightsBytes {
		return nil, slotWeightsRefusal(fmt.Sprintf("exceeds %d bytes", maxSlotWeightsBytes))
	}
	decoded, err := DecodeSlotWeightsFile(raw)
	if err != nil {
		return nil, slotWeightsRefusal(err.Error())
	}
	digest := sha256.Sum256(raw)
	return &AdmittedSlotWeights{Weights: decoded.Weights, SHA256: "sha256:" + hex.EncodeToString(digest[:])}, nil
}

// DecodeSlotWeightsFile strictly decodes and validates admitted-file bytes.
func DecodeSlotWeightsFile(raw []byte) (SlotWeightsFile, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var decoded SlotWeightsFile
	if err := decoder.Decode(&decoded); err != nil {
		return SlotWeightsFile{}, err
	}
	if decoded.SchemaVersion != slotWeightsSchema {
		return SlotWeightsFile{}, fmt.Errorf("schemaVersion must be %d", slotWeightsSchema)
	}
	return decoded, ValidateSlotWeights(decoded.Weights)
}

func slotWeightsRefusal(reason string) error {
	return &Error{Message: fmt.Sprintf("admitted slot weights %s are unusable (%s); run `corvint eval --reset-slot-weights` to restore the default order", SlotWeightsPath, reason)}
}

// TaskContextWeighted is TaskContext under an admitted learned trace. A nil
// trace is TaskContext exactly; otherwise the packet discloses the trace.
func TaskContextWeighted(ctx context.Context, index *Index, task, subject string, limit int, admitted *AdmittedSlotWeights) (map[string]any, error) {
	if admitted == nil {
		return TaskContext(ctx, index, task, subject, limit)
	}
	packet, err := taskContext(ctx, index, task, subject, limit, admitted.Weights)
	if err != nil {
		return nil, err
	}
	weights := map[string]any{}
	for relation, weight := range admitted.Weights {
		weights[relation] = weight
	}
	packet["learned_slot_weights"] = map[string]any{"path": SlotWeightsPath, "sha256": admitted.SHA256, "weights": weights}
	return packet, nil
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
