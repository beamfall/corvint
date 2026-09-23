// Package slotlearn turns the local read ledgers into negative labels for
// context slot order and proposes bounded slot-weight traces that only the
// frozen held-out gate may admit (learned-trace-admission-v0, LTA-V0-009..012).
// It runs only from the explicit `corvint eval --learn-slot-weights` step; the
// live context path never imports it.
package slotlearn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/evalrepo"
	"github.com/Beamfall/corvint/internal/observations"
	"github.com/Beamfall/corvint/internal/unplannedread"
)

const (
	// MaxLabelPaths bounds the distinct negative-label paths kept.
	MaxLabelPaths = 256
	// MaxProposals bounds the slot-weight traces the gate scores.
	MaxProposals = 4

	ReasonNoLabels      = "no negative labels"
	ReasonNoImprovement = "no held-out improvement"
	ReasonNotRequested  = "improved; --admit not given"
	ReasonAdmitted      = "admitted"
)

// Labels are the negative labels: repository paths an agent needed that the
// packet did not supply, each mapped to the slot that should have served it.
type Labels struct {
	Paths          []string
	UnplannedPaths int
	ObservedMisses int
	PlannedReads   int
	SkippedRows    int
	Truncated      bool
	Classes        map[string]int
}

// ReadLabels reads both ledgers through their bounded readers. Unplanned
// reads and observed misses are labels; planned re-reads are disclosed as a
// count only, because the packet already carried those paths.
func ReadLabels(root string) (Labels, error) {
	unplanned, err := unplannedread.Read(root)
	if err != nil {
		return Labels{}, err
	}
	observed, err := observations.Read(root)
	if err != nil {
		return Labels{}, err
	}
	distinct := map[string]struct{}{}
	for _, entry := range unplanned.TopPaths {
		distinct[entry.Path] = struct{}{}
	}
	for path := range observed.Misses {
		distinct[path] = struct{}{}
	}
	paths := make([]string, 0, len(distinct))
	for path := range distinct {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	labels := Labels{
		UnplannedPaths: len(unplanned.TopPaths), ObservedMisses: len(observed.Misses),
		PlannedReads: unplanned.Planned, SkippedRows: unplanned.SkippedRows,
		Truncated: len(paths) > MaxLabelPaths, Paths: paths[:min(MaxLabelPaths, len(paths))],
		Classes: map[string]int{},
	}
	for _, path := range labels.Paths {
		labels.Classes[ServingSlot(path)]++
	}
	return labels, nil
}

// ServingSlot is the closed table from a missed path to the slot that serves
// that kind of path: tests, prose (the packet's `documentation` relation,
// by the same suffixes), other docs/ files (lexical), or a code definition.
func ServingSlot(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := lower[strings.LastIndex(lower, "/")+1:]
	switch {
	case strings.HasSuffix(base, "_test.go"), strings.HasPrefix(base, "test_"),
		strings.Contains(base, ".test."), strings.Contains(base, ".spec."),
		strings.Contains("/"+lower, "/test/"), strings.Contains("/"+lower, "/tests/"):
		return "test"
	case strings.HasSuffix(base, ".md"), strings.HasSuffix(base, ".mdx"), strings.HasSuffix(base, ".rst"),
		strings.HasSuffix(base, ".txt"):
		return "documentation"
	case strings.HasPrefix(lower, "docs/"):
		return "lexical"
	}
	return "definition"
}

// Propose raises the most-missed serving slots by one and then two steps,
// most frequent class first, at most MaxProposals traces.
func Propose(classes map[string]int) []contextindex.SlotWeights {
	names := make([]string, 0, len(classes))
	for name := range classes {
		names = append(names, name)
	}
	sort.Slice(names, func(left, right int) bool {
		if classes[names[left]] != classes[names[right]] {
			return classes[names[left]] > classes[names[right]]
		}
		return names[left] < names[right]
	})
	proposals := []contextindex.SlotWeights{}
	for _, name := range names {
		proposals = append(proposals, contextindex.SlotWeights{name: 1}, contextindex.SlotWeights{name: contextindex.SlotWeightMax})
	}
	return proposals[:min(MaxProposals, len(proposals))]
}

// Learn reads the labels, scores the proposals on the frozen held-out gate
// and, only with admit and an improved proposal, writes the admitted trace.
func Learn(ctx context.Context, root, goldenPath string, admit bool) (map[string]any, bool, error) {
	labels, err := ReadLabels(root)
	if err != nil {
		return nil, false, err
	}
	result := map[string]any{"labels": labelsValue(labels)}
	if len(labels.Paths) == 0 {
		result["decision"] = map[string]any{"admitted": false, "reason": ReasonNoLabels}
		return result, false, nil
	}
	proposals := Propose(labels.Classes)
	report, best, err := evalrepo.EvaluateSlotWeights(ctx, root, goldenPath, proposals)
	if err != nil {
		return nil, false, err
	}
	result["evaluation"] = report
	decision := map[string]any{"admitted": false, "reason": ReasonNoImprovement}
	result["decision"] = decision
	if best < 0 {
		return result, false, nil
	}
	decision["weights"] = proposals[best]
	decision["reason"] = ReasonNotRequested
	if !admit {
		return result, false, nil
	}
	chosen := report["proposals"].([]any)[best].(map[string]any)
	evaluation := map[string]any{
		"goldens_sha256": report["goldens_sha256"], "revision": report["revision"],
		"heldout_cases": report["heldout_cases"], "baseline": report["baseline"], "arm": chosen["arm"],
		"delta": chosen["delta"],
	}
	if err := writeAdmitted(root, proposals[best], evaluation); err != nil {
		return nil, false, err
	}
	decision["admitted"], decision["reason"], decision["path"] = true, ReasonAdmitted, contextindex.SlotWeightsPath
	return result, true, nil
}

func labelsValue(labels Labels) map[string]any {
	classes := map[string]any{}
	for name, count := range labels.Classes {
		classes[name] = count
	}
	return map[string]any{
		"label_paths": len(labels.Paths), "unplanned_paths": labels.UnplannedPaths,
		"observed_misses": labels.ObservedMisses, "planned_reads": labels.PlannedReads,
		"skipped_rows": labels.SkippedRows, "truncated": labels.Truncated, "classes": classes,
		"max_label_paths": MaxLabelPaths,
	}
}

// writeAdmitted replaces the admitted trace atomically, refusing a symlinked
// store directory, and re-decodes the bytes it wrote through the live loader.
func writeAdmitted(root string, weights contextindex.SlotWeights, evaluation map[string]any) error {
	path := filepath.Join(root, filepath.FromSlash(contextindex.SlotWeightsPath))
	dir := filepath.Dir(path)
	if info, err := os.Lstat(dir); err == nil && !info.IsDir() {
		return fmt.Errorf("%s is not a directory", filepath.ToSlash(filepath.Dir(contextindex.SlotWeightsPath)))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	weightValue := map[string]any{}
	for relation, weight := range weights {
		weightValue[relation] = weight
	}
	raw, err := evalrepo.Encode(map[string]any{"schemaVersion": 1, "weights": weightValue, "evaluation": evaluation})
	if err != nil {
		return err
	}
	if _, err := contextindex.DecodeSlotWeightsFile(raw); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".slot-weights-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), path)
}

// Reset removes the admitted trace so the context packet returns to the
// default slot order. An absent trace is not an error.
func Reset(root string) (bool, error) {
	path := filepath.Join(root, filepath.FromSlash(contextindex.SlotWeightsPath))
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("%s is a directory", contextindex.SlotWeightsPath)
	}
	return true, os.Remove(path)
}
