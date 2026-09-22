// Package necessity answers one read-only question about a task-context
// packet: which of the files the packet included does the packet actually
// depend on? Each included path is removed from a copy of the index and the
// packet is recompiled; a path whose removal costs the packet an anchor, grows
// its missing-critical set, or moves it off a resolved answerability verdict is
// `load-bearing`, and every other included path is `supporting`.
//
// The label is a counterfactual under the current ranker at one immutable
// revision, never a claim about relevance to a human reader. The original index
// is never mutated, nothing is written to the repository, and a packet that
// abstains is emitted unchanged with no labels (AGENTS.md invariants 1, 2, 4).
package necessity

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Error is a necessity failure. An abstention is not an Error: it is a
// successful answer carrying no labels.
type Error struct{ Message string }

func (err *Error) Error() string { return err.Message }

const (
	// DefaultLimit bounds both the packet and the counterfactual budget when
	// no --limit is given.
	DefaultLimit = 12
	// MaxReruns is the hard ceiling on counterfactual recompiles for one
	// invocation, whatever --limit asks for.
	MaxReruns = 12

	labelLoadBearing = "load-bearing"
	labelSupporting  = "supporting"
	labelUnlabelled  = "unlabelled"

	// Disclosure is fixed text emitted with every labelled answer.
	Disclosure = "Labels are counterfactual under the current ranker at this revision, not proof of relevance: " +
		"`load-bearing` says only that this packet lost an anchor, gained a missing critical selector, or left a " +
		"resolved verdict when the file was removed from the index, and `supporting` does not say the file is " +
		"unnecessary to a reader."
)

// Options is one necessity request.
type Options struct {
	Root, Task, Subject string
	Limit               int
}

// Render compiles one labelled packet and writes it as a single canonical
// JSON line.
func Render(ctx context.Context, options Options, stdout io.Writer) error {
	packet, err := Resolve(ctx, options)
	if err != nil {
		return err
	}
	encoded, err := gokernel.CanonicalJSON(packet)
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(encoded, '\n'))
	return err
}

// Resolve loads the index the context verb loads, compiles the packet, and
// attaches the necessity block without emitting it.
func Resolve(ctx context.Context, options Options) (map[string]any, error) {
	if options.Limit < 1 {
		options.Limit = DefaultLimit
	}
	index, err := loadIndex(ctx, options)
	if err != nil {
		return nil, err
	}
	packet, err := contextindex.TaskContext(ctx, index, options.Task, options.Subject, options.Limit)
	if err != nil {
		return nil, err
	}
	packet["necessity"] = label(ctx, index, packet, options)
	return packet, nil
}

func loadIndex(ctx context.Context, options Options) (*contextindex.Index, error) {
	index, hit, opening, err := contextindex.LoadContextSnapshot(ctx, options.Root)
	if err != nil {
		return nil, err
	}
	if hit {
		return index, nil
	}
	return contextindex.BuildContextObserved(ctx, options.Root, options.Subject, opening)
}

// shape is the part of a packet a removal is compared on.
type shape struct {
	state, verdict string
	anchors        []string
	missing        int
}

// label runs the bounded counterfactual sweep and returns the necessity block.
func label(ctx context.Context, index *contextindex.Index, packet map[string]any, options Options) map[string]any {
	baseline := readShape(packet)
	paths := includedPaths(packet)
	block := map[string]any{
		"baseline": map[string]any{
			"state": baseline.state, "verdict": baseline.verdict,
			"anchors": len(baseline.anchors), "critical_missing": baseline.missing,
		},
		"disclosure": Disclosure,
	}
	if abstains(baseline) || len(paths) == 0 {
		block["abstained"] = true
		block["reruns"] = 0
		block["labels"] = []any{}
		return block
	}
	budget := min(options.Limit, MaxReruns)
	labels, reruns := make([]any, 0, len(paths)), 0
	for _, path := range paths {
		if reruns >= budget {
			labels = append(labels, map[string]any{
				"path": path, "label": labelUnlabelled, "change": "not recompiled", "reason": "budget",
			})
			continue
		}
		reruns++
		labels = append(labels, labelOne(ctx, index, options, baseline, path))
	}
	block["abstained"] = false
	block["reruns"] = reruns
	block["budget"] = budget
	block["labels"] = labels
	return block
}

// labelOne recompiles the packet without one path and reads the difference.
func labelOne(ctx context.Context, index *contextindex.Index, options Options, baseline shape, path string) map[string]any {
	reduced, err := contextindex.TaskContext(ctx, without(index, path), options.Task, options.Subject, options.Limit)
	if err != nil {
		return map[string]any{"path": path, "label": labelUnlabelled, "change": "recompile refused", "reason": "error"}
	}
	change := difference(baseline, readShape(reduced))
	if change == "" {
		return map[string]any{"path": path, "label": labelSupporting, "change": "no anchor, critical, or verdict change", "reason": "counterfactual"}
	}
	return map[string]any{"path": path, "label": labelLoadBearing, "change": change, "reason": "counterfactual"}
}

// difference is the short deterministic string naming the first degradation
// the removal caused, or "" when the packet held its shape.
func difference(baseline, reduced shape) string {
	if lost := lostAnchors(baseline.anchors, reduced.anchors); len(lost) > 0 {
		return "anchor lost: " + strings.Join(lost, ", ")
	}
	if reduced.missing > baseline.missing {
		return fmt.Sprintf("critical_missing %d -> %d", baseline.missing, reduced.missing)
	}
	if reduced.state != baseline.state {
		return "state " + baseline.state + " -> " + reduced.state
	}
	if reduced.verdict != baseline.verdict && abstains(reduced) {
		return "verdict " + baseline.verdict + " -> " + reduced.verdict
	}
	return ""
}

func lostAnchors(baseline, reduced []string) []string {
	lost := make([]string, 0)
	for _, anchor := range baseline {
		if !slices.Contains(reduced, anchor) {
			lost = append(lost, anchor)
		}
	}
	return lost
}

// without returns a shallow copy of the index whose read-side tables no longer
// carry path. Every mutated map is copied first: the original is never touched.
func without(index *contextindex.Index, path string) *contextindex.Index {
	reduced := *index
	reduced.Sources = dropSource(index.Sources, path)
	reduced.Tracked = dropSet(index.Tracked, path)
	reduced.Documents = dropRecord(index.Documents, path)
	reduced.Features = dropRecord(index.Features, path)
	reduced.Scenarios = dropRecord(index.Scenarios, path)
	reduced.Markers = dropMarkers(index.Markers, path)
	reduced.Symbols = slices.DeleteFunc(slices.Clone(index.Symbols), func(symbol contextindex.Symbol) bool {
		return symbol.Path == path
	})
	// The term table is derived from the tables above; a stale one would
	// answer for the removed source.
	reduced.Vocabulary = nil
	return &reduced
}

func dropSource(table map[string]contextindex.Source, path string) map[string]contextindex.Source {
	copied := make(map[string]contextindex.Source, len(table))
	for key, value := range table {
		if key != path {
			copied[key] = value
		}
	}
	return copied
}

func dropRecord(table map[string]contextindex.Record, path string) map[string]contextindex.Record {
	copied := make(map[string]contextindex.Record, len(table))
	for key, value := range table {
		if value.Path != path {
			copied[key] = value
		}
	}
	return copied
}

func dropMarkers(table map[string][]contextindex.Marker, path string) map[string][]contextindex.Marker {
	copied := make(map[string][]contextindex.Marker, len(table))
	for key, value := range table {
		kept := slices.DeleteFunc(slices.Clone(value), func(marker contextindex.Marker) bool {
			return marker.Path == path
		})
		if len(kept) != 0 {
			copied[key] = kept
		}
	}
	return copied
}

func dropSet(table map[string]struct{}, path string) map[string]struct{} {
	copied := make(map[string]struct{}, len(table))
	for key := range table {
		if key != path {
			copied[key] = struct{}{}
		}
	}
	return copied
}

// abstains reports the packet states that carry no answer to label.
func abstains(current shape) bool {
	return current.state != "READY" || current.verdict == "unsupported-conjunction"
}

// includedPaths is the packet's result ids in packet order.
func includedPaths(packet map[string]any) []string {
	results, _ := packet["results"].([]any)
	paths := make([]string, 0, len(results))
	for _, item := range results {
		row, _ := item.(map[string]any)
		if id, ok := row["id"].(string); ok {
			paths = append(paths, id)
		}
	}
	return paths
}

// readShape extracts the compared members from one compiled packet.
func readShape(packet map[string]any) shape {
	current := shape{state: stringMember(packet, "state")}
	coverage, _ := packet["coverage"].(map[string]any)
	if coverage == nil {
		return current
	}
	answerability, _ := coverage["answerability"].(map[string]any)
	current.verdict = stringMember(answerability, "verdict")
	critical, _ := coverage["critical"].([]any)
	for _, item := range critical {
		selector, _ := item.(map[string]any)
		current.anchors = append(current.anchors, stringMember(selector, "relation")+" "+stringMember(selector, "path"))
	}
	missing, _ := coverage["critical_missing"].([]any)
	current.missing = len(missing)
	return current
}

func stringMember(table map[string]any, name string) string {
	value, _ := table[name].(string)
	return value
}
