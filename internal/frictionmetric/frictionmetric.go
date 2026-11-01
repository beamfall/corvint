// Package frictionmetric computes deterministic agent-friction metrics over an
// explicit, caller-supplied session record (URE-V0-996 to URE-V0-998), so two
// context-packet arms for the same task can be compared. It is evaluation
// output only: the eval command is its sole importer, and no ranking,
// retrieval, evidence, authority or learning path may read it (URE-V0-997).
// The report carries counts, never a path or digest.
package frictionmetric

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
)

const (
	Profile           = "corvint-friction-sessions/0"
	maxDocumentBytes  = 4 << 20
	maxSessions       = 256
	maxSessionEvents  = 10000
	observed          = "OBSERVED"
	notObserved       = "NOT_OBSERVED"
	notReached        = "NOT_REACHED"
	firstRelevantRead = "first_relevant_read_ms"
)

// Metrics is the closed metric set, in report order.
var Metrics = []string{"rereads", "unplanned_reads", "retried_commands", "reverted_edits", firstRelevantRead}

type document struct {
	Profile  string    `json:"profile"`
	Sessions []session `json:"sessions"`
}

type session struct {
	ID       string   `json:"id"`
	Arm      string   `json:"arm"`
	Relevant []string `json:"relevant"`
	Labelled bool     `json:"labelled"`
	Events   []event  `json:"events"`
}

type event struct {
	Kind    string   `json:"kind"`
	TsMs    *int64   `json:"ts_ms"`
	Path    string   `json:"path"`
	Planned *bool    `json:"planned"`
	Command string   `json:"command"`
	Exit    *int     `json:"exit"`
	Before  string   `json:"before"`
	After   string   `json:"after"`
	Labels  []string `json:"labels"`
}

// result is one metric for one session: the value when observed, and the
// indices of the events the definition flagged, for the benchmark.
type result struct {
	status, reason string
	value          int64
	flagged        []int
}

// Evaluate reads one bounded friction record and returns the report.
func Evaluate(reader io.Reader) (map[string]any, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxDocumentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read friction record: %w", err)
	}
	if len(raw) > maxDocumentBytes {
		return nil, errors.New("friction record exceeds 4 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var parsed document
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("malformed friction record: %w", err)
	}
	if decoder.More() {
		return nil, errors.New("malformed friction record: trailing data")
	}
	if err := validate(parsed); err != nil {
		return nil, err
	}
	sessions := make([]any, 0, len(parsed.Sessions))
	totals := map[string]map[string]*total{}
	benchmark := map[string]*confusion{}
	labelledSessions := 0
	for _, current := range parsed.Sessions {
		arm := current.Arm
		if arm == "" {
			arm = "default"
		}
		if totals[arm] == nil {
			totals[arm] = map[string]*total{}
			for _, name := range Metrics {
				totals[arm][name] = &total{}
			}
		}
		results := compute(current)
		row := map[string]any{"id": current.ID, "arm": arm, "events": len(current.Events)}
		metrics := map[string]any{}
		for _, name := range Metrics {
			metric := results[name]
			metrics[name] = render(metric)
			if metric.status == observed {
				totals[arm][name].sessions++
				totals[arm][name].sum += metric.value
			}
		}
		row["metrics"] = metrics
		sessions = append(sessions, row)
		if current.Labelled {
			labelledSessions++
			for _, name := range Metrics {
				if benchmark[name] == nil {
					benchmark[name] = &confusion{}
				}
				benchmark[name].add(results[name].flagged, labelled(current.Events, name))
			}
		}
	}
	arms := map[string]any{}
	for arm, metrics := range totals {
		rendered := map[string]any{}
		for name, sum := range metrics {
			if sum.sessions == 0 {
				rendered[name] = map[string]any{"status": notObserved, "reason": "no-observed-session"}
			} else {
				rendered[name] = map[string]any{"status": observed, "sum": sum.sum, "sessions": sum.sessions}
			}
		}
		arms[arm] = rendered
	}
	report := map[string]any{"profile": Profile, "metrics": slices.Clone(Metrics), "sessions": sessions, "arms": arms}
	if labelledSessions == 0 {
		report["benchmark"] = map[string]any{"status": notObserved, "reason": "no-labelled-session"}
	} else {
		rendered := map[string]any{}
		for name, counts := range benchmark {
			rendered[name] = counts.render()
		}
		report["benchmark"] = map[string]any{"status": observed, "labelled_sessions": labelledSessions, "metrics": rendered}
	}
	return report, nil
}

type total struct {
	sessions int
	sum      int64
}

func validate(parsed document) error {
	if parsed.Profile != Profile {
		return fmt.Errorf("unsupported friction record profile %q", parsed.Profile)
	}
	if len(parsed.Sessions) == 0 || len(parsed.Sessions) > maxSessions {
		return fmt.Errorf("friction record must carry 1 to %d sessions", maxSessions)
	}
	seen := map[string]bool{}
	for _, current := range parsed.Sessions {
		if current.ID == "" || seen[current.ID] {
			return errors.New("friction session ids must be present and distinct")
		}
		seen[current.ID] = true
		if len(current.Events) > maxSessionEvents {
			return fmt.Errorf("friction session %q exceeds %d events", current.ID, maxSessionEvents)
		}
		if !sort.StringsAreSorted(current.Relevant) {
			return fmt.Errorf("friction session %q relevant set is not sorted", current.ID)
		}
		for index, item := range current.Events {
			switch item.Kind {
			case "read", "edit":
				if item.Path == "" {
					return fmt.Errorf("friction session %q event %d: %s without path", current.ID, index, item.Kind)
				}
			case "command":
				if item.Command == "" {
					return fmt.Errorf("friction session %q event %d: command without digest", current.ID, index)
				}
			default:
				return fmt.Errorf("friction session %q event %d: unknown kind %q", current.ID, index, item.Kind)
			}
			for _, label := range item.Labels {
				if !slices.Contains(Metrics, label) {
					return fmt.Errorf("friction session %q event %d: unknown label %q", current.ID, index, label)
				}
			}
		}
	}
	return nil
}

func compute(current session) map[string]result {
	results := map[string]result{}
	if len(current.Events) == 0 {
		for _, name := range Metrics {
			results[name] = result{status: notObserved, reason: "no-events"}
		}
		return results
	}
	results["rereads"] = rereads(current.Events)
	results["unplanned_reads"] = unplannedReads(current.Events)
	results["retried_commands"] = retriedCommands(current.Events)
	results["reverted_edits"] = revertedEdits(current.Events)
	results[firstRelevantRead] = firstRelevant(current)
	return results
}

func counted(flagged []int) result {
	return result{status: observed, value: int64(len(flagged)), flagged: flagged}
}

// rereads flags a read of a path already read with no edit of it in between.
func rereads(events []event) result {
	read := map[string]bool{}
	var flagged []int
	for index, item := range events {
		switch item.Kind {
		case "read":
			if read[item.Path] {
				flagged = append(flagged, index)
			}
			read[item.Path] = true
		case "edit":
			delete(read, item.Path)
		}
	}
	return counted(flagged)
}

func unplannedReads(events []event) result {
	var flagged []int
	for index, item := range events {
		if item.Kind != "read" {
			continue
		}
		if item.Planned == nil {
			return result{status: notObserved, reason: "read-without-planned"}
		}
		if !*item.Planned {
			flagged = append(flagged, index)
		}
	}
	return counted(flagged)
}

// retriedCommands flags a command repeating the immediately preceding
// command's digest after that command exited non-zero.
func retriedCommands(events []event) result {
	var flagged []int
	previous := -1
	for index, item := range events {
		if item.Kind != "command" {
			continue
		}
		if item.Exit == nil {
			return result{status: notObserved, reason: "command-without-exit"}
		}
		if previous >= 0 && events[previous].Command == item.Command && *events[previous].Exit != 0 {
			flagged = append(flagged, index)
		}
		previous = index
	}
	return counted(flagged)
}

// revertedEdits flags an edit that returns a path to the content an earlier
// edit of that path started from.
func revertedEdits(events []event) result {
	before := map[string]map[string]bool{}
	var flagged []int
	for index, item := range events {
		if item.Kind != "edit" {
			continue
		}
		if item.Before == "" || item.After == "" {
			return result{status: notObserved, reason: "edit-without-digests"}
		}
		if before[item.Path][item.After] {
			flagged = append(flagged, index)
		}
		if before[item.Path] == nil {
			before[item.Path] = map[string]bool{}
		}
		before[item.Path][item.Before] = true
	}
	return counted(flagged)
}

func firstRelevant(current session) result {
	if len(current.Relevant) == 0 {
		return result{status: notObserved, reason: "no-relevant-set"}
	}
	for _, item := range current.Events {
		if item.TsMs == nil {
			return result{status: notObserved, reason: "event-without-ts"}
		}
	}
	start := *current.Events[0].TsMs
	for index, item := range current.Events {
		if item.Kind == "read" {
			if _, found := slices.BinarySearch(current.Relevant, item.Path); found {
				return result{status: observed, value: *item.TsMs - start, flagged: []int{index}}
			}
		}
	}
	return result{status: notReached, reason: "relevant-path-never-read"}
}

func render(metric result) map[string]any {
	if metric.status == observed {
		return map[string]any{"status": observed, "value": metric.value}
	}
	return map[string]any{"status": metric.status, "reason": metric.reason}
}

func labelled(events []event, name string) []int {
	var indices []int
	for index, item := range events {
		if slices.Contains(item.Labels, name) {
			indices = append(indices, index)
		}
	}
	return indices
}

type confusion struct{ truePositive, falsePositive, falseNegative int }

// add scores one labelled session. A metric that was not observed flags
// nothing, so its labels count as false negatives.
func (counts *confusion) add(flagged, labels []int) {
	for _, index := range flagged {
		if slices.Contains(labels, index) {
			counts.truePositive++
		} else {
			counts.falsePositive++
		}
	}
	for _, index := range labels {
		if !slices.Contains(flagged, index) {
			counts.falseNegative++
		}
	}
}

func (counts *confusion) render() map[string]any {
	ratio := func(denominator int) any {
		if denominator == 0 {
			return map[string]any{"status": notObserved, "reason": "zero-denominator"}
		}
		return float64(counts.truePositive) / float64(denominator)
	}
	return map[string]any{
		"true_positive":  counts.truePositive,
		"false_positive": counts.falsePositive,
		"false_negative": counts.falseNegative,
		"precision":      ratio(counts.truePositive + counts.falsePositive),
		"recall":         ratio(counts.truePositive + counts.falseNegative),
	}
}
