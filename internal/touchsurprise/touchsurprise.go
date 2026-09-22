// Package touchsurprise measures, for one completed task, how much of the
// change the task-context packet never named: the touch-set surprise. It is
// read-only and compares committed revisions only (AGENTS.md invariant 4).
package touchsurprise

import (
	"context"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// DefaultLimit is the packet and impact result bound when --limit is absent.
const DefaultLimit = 20

// MaxLimit is contextindex's own ceiling for both TaskContext and Impact.
const MaxLimit = 50

// maxImpactPaths is contextindex.Impact's own path bound; a larger packet is
// expanded over its first paths in path order, and the receipt says so.
const maxImpactPaths = 100

// Options is one parsed `surprise` invocation.
type Options struct {
	Root, Task, Subject string
	Base, Target        string
	Limit               int
}

// predictedSets records which predicted path the packet named and which only
// the impact expansion of those paths reached (TSS-V0-002).
type predictedSets struct {
	packet, impact, all []string
	impactTruncated     bool
}

// Render writes one canonical JSON touch-set surprise receipt. It reads the
// index and Git only; nothing in the repository or the trace state is written.
func Render(ctx context.Context, index *contextindex.Index, options Options, stdout io.Writer) error {
	if err := requireCleanWorktree(ctx, options.Root); err != nil {
		return err
	}
	base, err := resolveCommit(ctx, options.Root, options.Base)
	if err != nil {
		return err
	}
	target, err := resolveCommit(ctx, options.Root, options.Target)
	if err != nil {
		return err
	}
	predicted, err := predictedPaths(ctx, index, options)
	if err != nil {
		return err
	}
	actual, err := changedPaths(ctx, options.Root, base, target)
	if err != nil {
		return err
	}
	report := Compute(predicted.all, actual)
	tracked, err := trackedPaths(ctx, options.Root, target)
	if err != nil {
		return err
	}
	encoded, err := gokernel.CanonicalJSON(receipt(index, options, base, target, predicted, report, tracked))
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(encoded, '\n'))
	return err
}

// predictedPaths is the packet's own paths unioned with the impact expansion of
// those paths. Impact reads only tracked sources, so the packet paths it cannot
// accept (excluded or untracked rows) are still predicted, just unexpanded.
func predictedPaths(ctx context.Context, index *contextindex.Index, options Options) (predictedSets, error) {
	packet, err := contextindex.TaskContext(ctx, index, options.Task, options.Subject, options.Limit)
	if err != nil {
		return predictedSets{}, err
	}
	sets := predictedSets{packet: receiptPaths(packet)}
	seeds := sourcePaths(index, sets.packet)
	if len(seeds) > maxImpactPaths {
		seeds, sets.impactTruncated = seeds[:maxImpactPaths], true
	}
	if len(seeds) != 0 {
		expansion, err := contextindex.Impact(index, seeds, options.Limit)
		if err != nil {
			return predictedSets{}, err
		}
		// A feature or scenario row names a ledger record, not a path, so only
		// ids that are tracked sources count as impact predictions.
		sets.impact = subtract(sourcePaths(index, receiptPaths(expansion)), sets.packet)
	}
	sets.all = Compute(append(append([]string{}, sets.packet...), sets.impact...), nil).Predicted
	return sets, nil
}

// receiptPaths reads the `id` of each result row. Context rows always name a
// repository path there; impact rows may name a ledger record instead, which
// predictedPaths filters out.
func receiptPaths(rendered map[string]any) []string {
	rows, _ := rendered["results"].([]any)
	paths := make([]string, 0, len(rows))
	for _, raw := range rows {
		row, isMap := raw.(map[string]any)
		if !isMap {
			continue
		}
		if value, isString := row["id"].(string); isString && value != "" {
			paths = append(paths, value)
		}
	}
	return paths
}

func sourcePaths(index *contextindex.Index, paths []string) []string {
	seeds := make([]string, 0, len(paths))
	for _, value := range paths {
		if _, tracked := index.Sources[value]; tracked {
			seeds = append(seeds, value)
		}
	}
	sort.Strings(seeds)
	return seeds
}

func subtract(paths, removed []string) []string {
	set := pathSet(removed)
	remaining := make([]string, 0, len(paths))
	for _, value := range paths {
		if _, present := set[value]; !present {
			remaining = append(remaining, value)
		}
	}
	sort.Strings(remaining)
	return remaining
}

// missRow classifies one actual-only path. `test_path` and `doc_path` are
// declared heuristics over the path spelling, never an extractor claim.
func missRow(value string, tracked map[string]struct{}) map[string]any {
	_, present := tracked[value]
	return map[string]any{
		"path":              value,
		"tracked_at_target": present,
		"test_path":         isTestPath(value),
		"doc_path":          isDocPath(value),
	}
}

func isTestPath(value string) bool {
	base := path.Base(value)
	if strings.HasSuffix(base, "_test.go") || strings.HasPrefix(base, "test_") {
		return true
	}
	if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasSuffix(base, "_spec.rb") {
		return true
	}
	for _, segment := range strings.Split(path.Dir(value), "/") {
		if segment == "test" || segment == "tests" || segment == "testdata" || segment == "conformance" {
			return true
		}
	}
	return false
}

func isDocPath(value string) bool {
	for _, suffix := range []string{".md", ".mdx", ".rst", ".txt", ".adoc"} {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return strings.HasPrefix(value, "docs/")
}

func receipt(
	index *contextindex.Index, options Options, base, target string,
	predicted predictedSets, report Report, tracked map[string]struct{},
) map[string]any {
	misses := make([]any, 0, len(report.ActualOnly))
	for _, value := range report.ActualOnly {
		misses = append(misses, missRow(value, tracked))
	}
	return map[string]any{
		"tool": "surprise", "ok": true, "mutates": false, "schema_version": 1,
		"revision": index.Revision,
		"request": map[string]any{
			"task_chars": len(options.Task), "subject": subjectValue(options.Subject), "limit": options.Limit,
			"base": options.Base, "target": options.Target,
		},
		"range": map[string]any{"base": base, "target": target},
		"predicted": map[string]any{
			"paths": report.Predicted, "from_packet": predicted.packet, "from_impact": predicted.impact,
			"impact_seeds_truncated": predicted.impactTruncated,
		},
		"actual": map[string]any{"paths": report.Actual},
		"surprise": map[string]any{
			"predicted_only": report.PredictedOnly, "actual_only": report.ActualOnly,
			"intersection": report.Intersection, "symmetric_difference": report.SymmetricDifference,
			"surprise": report.Surprise, "empty_actual": report.EmptyActual,
		},
		"misses": misses,
		"uncertainty": []any{
			"`test_path` and `doc_path` are heuristics over the path spelling, not an extractor claim.",
			"The actual set is the committed range alone; no CEM document is consulted in V0.",
		},
	}
}

func subjectValue(subject string) any {
	if subject == "" {
		return nil
	}
	return subject
}
