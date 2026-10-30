package main

// Equal-budget text windows (CEP-V0-008), outcome separation (CEP-V0-009)
// and experiment kinds (CEP-V0-010). A ranked path list says nothing about
// how much text an agent must read, so a packet's bytes and a baseline's
// whole files are not comparable context. Under --budget-bytes every arm,
// Corvint packets and lexical baselines alike, is consumed through the same
// text window per ranked file until the same byte budget is spent; recall is
// then read at equal context. The historical metrics are unchanged.

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	defaultWindowBytes = 4096
	maxBudgetBytes     = 1 << 24
	tokenBudgetState   = "NOT_MEASURED: needs the bench's chunk tokenizer"
)

// validateBudget checks the budget flags. A zero budget turns the windows
// off, so a run without the flag keeps its report bytes.
func validateBudget(configuration options) error {
	if configuration.budgetBytes < 0 || configuration.budgetBytes > maxBudgetBytes {
		return errors.New("--budget-bytes must be within 0..16777216")
	}
	if configuration.windowBytes < 1 || configuration.windowBytes > maxBudgetBytes {
		return errors.New("--window-bytes must be within 1..16777216")
	}
	if configuration.budgetBytes > 0 && len(configuration.summaries) > 0 {
		return errors.New("--budget-bytes needs the snapshot copies; it cannot be combined with --summarize")
	}
	return nil
}

// budgetBlock is the report's description of the shared budget, or nil when
// windows are off.
func budgetBlock(configuration options) map[string]any {
	if configuration.budgetBytes == 0 {
		return nil
	}
	return map[string]any{
		"unit": "bytes", "budget_bytes": configuration.budgetBytes, "window_bytes": configuration.windowBytes,
		"window": "first window_bytes of each ranked regular file, in ranking order, whole windows only",
		"tokens": tokenBudgetState,
	}
}

// budgetMetrics consumes the ranking through identical text windows: each
// ranked regular file in the copy costs min(size, window) bytes, windows are
// taken whole in ranking order, and the walk stops at the first window that
// would exceed the budget. A ranked path that is not a regular file in the
// copy supplies no text and costs nothing, and the copy is read through
// os.Root so no ranked path escapes it. An errored or abstained arm delivers
// nothing.
func budgetMetrics(result metrics, answer arm, gold []string, root string, positive bool, configuration options) {
	used, files, hits := 0, 0, 0
	goldSet := set(gold)
	tree, err := os.OpenRoot(root)
	if err == nil {
		defer tree.Close()
	}
	if err == nil && answer.Error == "" && !answer.Abstained {
		for _, path := range answer.Ranked {
			if !safeRelative(path) {
				continue
			}
			info, err := tree.Lstat(filepath.FromSlash(path))
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			window := int(min(info.Size(), int64(configuration.windowBytes)))
			if used+window > configuration.budgetBytes {
				break
			}
			used += window
			files++
			if goldSet[path] {
				hits++
			}
		}
	}
	result["budget_bytes_used"] = float64(used)
	result["budget_files"] = float64(files)
	if positive {
		result["budget_recall"] = ratio(hits, len(gold))
		result["budget_hit"] = boolean(hits > 0)
	}
}

// outcomes separates what an arm did from how well it ranked: coverage is
// the share of samples answered without error, abstention and errors are
// their own rates, and answered-only precision averages precision@k over the
// positive samples the arm answered, so an abstention or an error can never
// raise or lower it. The historical means, which score an error as zero and
// an abstention by its empty ranking, are kept beside it.
func outcomes(members []sampleReport, name string) map[string]any {
	answered, abstained, errored, answeredPositives := 0, 0, 0, 0
	precision := 0.0
	for _, member := range members {
		answer := member.Arms[name]
		switch {
		case answer.Error != "":
			errored++
		case answer.Abstained:
			abstained++
		default:
			answered++
			if member.Stratum == "positive" {
				answeredPositives++
				precision += member.Metrics[name]["precision@k"]
			}
		}
	}
	n := len(members)
	block := map[string]any{
		"n": n, "answered": answered, "abstained": abstained, "errors": errored,
		"coverage": nullableRatio(answered, n), "abstention_rate": nullableRatio(abstained, n),
		"error_rate": nullableRatio(errored, n), "answered_positives": answeredPositives,
		"answered_precision@k": nil,
	}
	if answeredPositives > 0 {
		block["answered_precision@k"] = round(precision / float64(answeredPositives))
	}
	return block
}

func nullableRatio(part, total int) any {
	if total == 0 {
		return nil
	}
	return round(float64(part) / float64(total))
}

// experiment names what this report measures. Retrieval is the only kind
// this tool produces; oracle-context utility and voluntary tool adoption are
// separate experiments with their own arms and graders, and a downstream
// SWE-bench Pro outcome is graded only by the official pinned grader, never
// by a reimplementation here.
func experiment() map[string]any {
	return map[string]any{
		"kind":              "retrieval",
		"oracle_context":    "NOT_RUN: separate experiment kind, not produced by retrieval-bench",
		"tool_adoption":     "NOT_RUN: separate experiment kind, not produced by retrieval-bench",
		"downstream_grader": "NOT_RUN: SWE-bench Pro outcomes are graded only by the official pinned grader; retrieval-bench grades none",
	}
}
