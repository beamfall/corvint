// SPDX-License-Identifier: Apache-2.0

// Package runevidence is the private, append-only store of test run results under
// <git-dir>/corvint/evidence/ (docs/specs/run-evidence-v1.md, RVD-V1-001 to RVD-V1-012). Each
// record is the exact bytes of one run document, named by their SHA-256, and listed by one line of
// log.jsonl. `corvint test` and `flows ingest --record` append; the reports read. The package has
// no Corvint imports beyond the standard library, so the writer (cmd/corvint) and the readers
// (internal/appflows, internal/testevidencedoc) share it without a cycle.
package runevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// The two run documents a record can hold (RVD-V1-006, design decision 3).
const (
	ProfileReceipt  = "corvint-test-run/0"
	ProfileIngested = "test-run-evidence/0"
)

// Row granularities: a row names one test, or one file or package whose tests the runner reported
// only together (RVD-V1-006).
const (
	GranularityTest = "test"
	GranularityFile = "file"
)

// Row is one test's result in one stored run (RVD-V1-006). Outcome uses the test-run-evidence/0
// classification vocabulary: passed, failed, flaky, timedOut, skipped, interrupted or not-run;
// NativeOutcome keeps the document's own word.
type Row struct {
	TestID              string `json:"testId"`
	Project             string `json:"project,omitempty"`
	Slice               string `json:"slice,omitempty"`
	Granularity         string `json:"granularity"`
	Outcome             string `json:"outcome"`
	NativeOutcome       string `json:"nativeOutcome"`
	Retries             int    `json:"retries,omitempty"`
	Instance            string `json:"instance"`
	RunnerVersion       string `json:"runnerVersion,omitempty"`
	Tree                string `json:"tree"`
	Commit              string `json:"commit"`
	Clean               bool   `json:"clean"`
	DurationMs          *int64 `json:"durationMs,omitempty"`
	DurationSource      string `json:"durationSource,omitempty"`
	DurationGranularity string `json:"durationGranularity,omitempty"`
}

// Key is the retention key of the row's test: its ID, and its project when it has one.
func (r Row) Key() string {
	if r.Project == "" {
		return r.TestID
	}
	return r.TestID + " @" + r.Project
}

// Parsed is a run document normalised to rows, with the header its log line carries.
type Parsed struct {
	Profile   string
	Tree      string
	Commit    string
	Instances []string
	Rows      []Row
}

// Tests is the sorted, unique retention keys of the parsed rows.
func (p Parsed) Tests() []string {
	keys := make([]string, 0, len(p.Rows))
	for _, row := range p.Rows {
		keys = append(keys, row.Key())
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// receiptDocument is the part of a corvint-test-run/0 receipt a row needs. It is read leniently:
// a receipt from a newer Corvint with more members still normalises (D25 add-only rule), and
// testId and the duration members are optional (CTI-V1-012, PTD-V1-001).
type receiptDocument struct {
	Profile string `json:"profile"`
	Source  struct {
		Head  string            `json:"head"`
		Tree  string            `json:"tree"`
		Paths []json.RawMessage `json:"paths"`
	} `json:"source"`
	Invocations []struct {
		Adapter  string `json:"adapter"`
		Version  string `json:"version"`
		Outcomes []struct {
			Outcome             string `json:"outcome"`
			Slice               string `json:"slice"`
			Unit                string `json:"unit"`
			TestID              string `json:"testId"`
			DurationMs          *int64 `json:"durationMs"`
			DurationSource      string `json:"durationSource"`
			DurationGranularity string `json:"durationGranularity"`
		} `json:"outcomes"`
	} `json:"invocations"`
}

// ingestedRecord is the part of one test-run-evidence/0 line a row needs.
type ingestedRecord struct {
	Schema    string `json:"schema"`
	Authority string `json:"authority"`
	Runner    struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"runner"`
	Source struct {
		Commit string `json:"commit"`
		Tree   string `json:"tree"`
		Clean  bool   `json:"clean"`
	} `json:"source"`
	TestKey  string `json:"test_key"`
	Project  string `json:"project"`
	Attempts []struct {
		Outcome string `json:"outcome"`
	} `json:"attempts"`
}

// receiptOutcomes maps a corvint-test-run/0 unit outcome onto the classification vocabulary.
var receiptOutcomes = map[string]string{
	"PASS": "passed", "NO_TESTS": "skipped", "FAIL": "failed", "BUILD_FAILED": "failed",
	"TIMED_OUT": "timedOut", "NOT_RUN": "not-run",
}

var errNotRunDocument = errors.New("not a corvint-test-run/0 receipt or test-run-evidence/0 document")

// Parse normalises one run document to rows (RVD-V1-006): a corvint-test-run/0 receipt gives one
// row per unit outcome, marked file because a receipt unit is a package or a test file; a
// test-run-evidence/0 document gives one row per test with its final classification and retry
// count. Anything else is refused.
func Parse(data []byte) (Parsed, error) {
	if len(data) > MaxRecordBytes {
		return Parsed{}, fmt.Errorf("record exceeds %d bytes", MaxRecordBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Parsed{}, errNotRunDocument
	}
	var probe struct {
		Profile string `json:"profile"`
	}
	if json.Unmarshal(trimmed, &probe) == nil && probe.Profile == ProfileReceipt {
		return parseReceipt(trimmed)
	}
	return parseIngested(data)
}

func parseReceipt(data []byte) (Parsed, error) {
	var doc receiptDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return Parsed{}, err
	}
	if doc.Source.Head == "" || doc.Source.Tree == "" {
		return Parsed{}, errors.New("receipt names no source head and tree")
	}
	parsed := Parsed{Profile: ProfileReceipt, Tree: doc.Source.Tree, Commit: doc.Source.Head, Rows: []Row{}}
	clean := len(doc.Source.Paths) == 0
	for _, invocation := range doc.Invocations {
		for _, outcome := range invocation.Outcomes {
			if outcome.Unit == "" {
				return Parsed{}, errors.New("receipt outcome names no unit")
			}
			id := outcome.TestID
			if id == "" {
				id = outcome.Unit
			}
			outcomeWord, known := receiptOutcomes[outcome.Outcome]
			if !known {
				outcomeWord = "failed"
			}
			row := Row{
				TestID: id, Granularity: GranularityFile, Outcome: outcomeWord, NativeOutcome: outcome.Outcome,
				Instance: receiptInstance(invocation.Adapter, outcome.Unit), RunnerVersion: invocation.Version,
				Tree: doc.Source.Tree, Commit: doc.Source.Head, Clean: clean,
				DurationMs: outcome.DurationMs, DurationSource: outcome.DurationSource, DurationGranularity: outcome.DurationGranularity,
			}
			if outcome.Slice != "" && outcome.Slice != "whole" {
				row.Slice = outcome.Slice
			}
			parsed.Rows = append(parsed.Rows, row)
			parsed.Instances = append(parsed.Instances, row.Instance)
		}
	}
	slices.Sort(parsed.Instances)
	parsed.Instances = slices.Compact(parsed.Instances)
	if parsed.Instances == nil {
		parsed.Instances = []string{}
	}
	return parsed, nil
}

// receiptInstance names the runner instance a receipt row ran under. Until the test inventory's
// instance IDs exist on this tree (TIV-V1-003) it is the adapter and the runner it launched: "go",
// "declared:<command>" or "provider:<runner>".
func receiptInstance(adapter, unit string) string {
	switch adapter {
	case "declared":
		return unit
	case "provider":
		if rest, ok := strings.CutPrefix(unit, "typescript:"); ok {
			if runner, _, ok := strings.Cut(rest, ":"); ok {
				return "provider:" + runner
			}
		}
		return "provider"
	}
	return adapter
}

func parseIngested(data []byte) (Parsed, error) {
	parsed := Parsed{Profile: ProfileIngested, Rows: []Row{}}
	decoder := json.NewDecoder(bytes.NewReader(data))
	seen := map[string]bool{}
	for {
		var record ingestedRecord
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Parsed{}, errNotRunDocument
		}
		if record.Schema != ProfileIngested || record.TestKey == "" {
			return Parsed{}, errNotRunDocument
		}
		if record.Authority == "STATIC" {
			// A source mapping alone is no run (AFU-V1-014): it gives no row.
			continue
		}
		if parsed.Tree == "" {
			parsed.Tree, parsed.Commit = record.Source.Tree, record.Source.Commit
		}
		outcomes := make([]string, 0, len(record.Attempts))
		for _, attempt := range record.Attempts {
			outcomes = append(outcomes, attempt.Outcome)
		}
		row := Row{
			TestID: record.TestKey, Project: record.Project, Granularity: GranularityTest, Outcome: classify(outcomes),
			Instance: record.Runner.Name, RunnerVersion: record.Runner.Version,
			Tree: record.Source.Tree, Commit: record.Source.Commit, Clean: record.Source.Clean,
		}
		if len(outcomes) != 0 {
			row.NativeOutcome = outcomes[len(outcomes)-1]
			row.Retries = len(outcomes) - 1
		}
		if seen[row.Key()] {
			return Parsed{}, fmt.Errorf("test %q appears twice in one run", row.Key())
		}
		seen[row.Key()] = true
		parsed.Rows = append(parsed.Rows, row)
		parsed.Instances = append(parsed.Instances, row.Instance)
	}
	if len(parsed.Rows) == 0 && parsed.Tree == "" {
		return Parsed{}, errNotRunDocument
	}
	slices.Sort(parsed.Instances)
	parsed.Instances = slices.Compact(parsed.Instances)
	if parsed.Instances == nil {
		parsed.Instances = []string{}
	}
	return parsed, nil
}

// classify is the AFU-V1-013 per-test result of an attempt list: the last attempt's outcome,
// except that a pass after a failed or timed-out attempt is flaky; no attempt is not-run.
func classify(outcomes []string) string {
	if len(outcomes) == 0 {
		return "not-run"
	}
	last := outcomes[len(outcomes)-1]
	if last == "passed" && slices.ContainsFunc(outcomes[:len(outcomes)-1], func(o string) bool { return o == "failed" || o == "timedOut" }) {
		return "flaky"
	}
	return last
}
