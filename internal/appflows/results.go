// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/runevidence"
	"github.com/Beamfall/corvint/internal/testid"
)

// The kinds and states of one result source a flows status report read (AFL-V1-002).
const (
	SourceStore    = "run-evidence-store"
	SourceReceipt  = "receipt"
	SourceEvidence = "evidence"

	SourceRead    = "read"
	SourceAbsent  = "absent"
	SourceInStore = "in-store"
	SourceSkipped = "not-read"

	// DefaultReceiptLabel names the receipt `corvint test` writes by default; the report prints
	// the label, not the host's absolute Git directory, so it stays byte-identical across clones.
	DefaultReceiptLabel = "<git-dir>/corvint/test-run.json"
)

// ResultSource is one place a flows status report looked for test results (AFL-V1-002): the run
// evidence store, the default `corvint test` receipt, or a named receipt or evidence file. Records
// is how many result records it gave the report.
type ResultSource struct {
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	State   string `json:"state"`
	Records int    `json:"records"`
}

// ResultInputs are the result sources a caller names: test-run-evidence/0 files written by
// `flows ingest`, corvint-test-run/0 receipts written by `corvint test`, and whether to skip the
// run evidence store (and with it the default receipt).
type ResultInputs struct {
	Evidence []string
	Receipts []string
	NoStore  bool
	// Resolve maps a named file to the path to open; nil opens the name as given. The report
	// always prints the name as given, so a caller that confines names to the root (the MCP
	// tools) and one that passes them through (the command) print the same bytes.
	Resolve func(name string) (string, error)
}

func (in ResultInputs) open(name string) (string, error) {
	if in.Resolve == nil {
		return name, nil
	}
	return in.Resolve(name)
}

// Results are the records a flows report joins, in the order the newest-record rule reads them,
// with what the store read gave and every source the report looked at.
type Results struct {
	Records []TestRunEvidence
	Store   *RunEvidenceRead
	Sources []ResultSource
}

// ReadResults reads the only two writers a flows status result may come from (AFL-V1-002): the run
// evidence store (which holds the receipts `corvint test` appends and the documents `flows ingest
// --record` appends, RVD-V1-004, RVD-V1-005), the default receipt `corvint test` wrote when the
// store does not already hold it, then each named receipt and each named evidence file. The
// default receipt is read first, so a receipt the store never took cannot hide a newer stored
// run. A named file that is missing, malformed, oversized or of another profile refuses the whole
// read (AFL-V1-006); an absent default receipt is listed, not refused. Nothing is written.
func ReadResults(ctx context.Context, root string, in ResultInputs) (Results, error) {
	var out Results
	var fallback []TestRunEvidence
	if !in.NoStore {
		stored, read, err := ReadStoredRunEvidence(ctx, root)
		if err != nil {
			return Results{}, err
		}
		out.Store = read
		out.Records = stored
		out.Sources = append(out.Sources, ResultSource{Kind: SourceStore, State: SourceRead, Records: len(stored)})
		source, records, err := defaultReceipt(ctx, root, read)
		if err != nil {
			return Results{}, err
		}
		out.Sources = append(out.Sources, source)
		fallback = records
	} else {
		out.Sources = append(out.Sources,
			ResultSource{Kind: SourceStore, State: SourceSkipped},
			ResultSource{Kind: SourceReceipt, Path: DefaultReceiptLabel, State: SourceSkipped})
	}
	out.Records = append(fallback, out.Records...)
	for _, name := range in.Receipts {
		file, err := in.open(name)
		if err != nil {
			return Results{}, err
		}
		raw, err := ReadFile(file)
		if err != nil {
			return Results{}, errors.New("receipt " + name + ": " + err.Error())
		}
		records, err := receiptRecords(name, raw)
		if err != nil {
			return Results{}, err
		}
		if len(out.Records)+len(records) > maxRunRecords {
			return Results{}, boundError(BoundRecords)
		}
		out.Records = append(out.Records, records...)
		out.Sources = append(out.Sources, ResultSource{Kind: SourceReceipt, Path: name, State: SourceRead, Records: len(records)})
	}
	for _, name := range in.Evidence {
		file, err := in.open(name)
		if err != nil {
			return Results{}, err
		}
		records, err := ReadRunEvidence([]string{file})
		if err != nil {
			return Results{}, err
		}
		if len(out.Records)+len(records) > maxRunRecords {
			return Results{}, boundError(BoundRecords)
		}
		out.Records = append(out.Records, records...)
		out.Sources = append(out.Sources, ResultSource{Kind: SourceEvidence, Path: name, State: SourceRead, Records: len(records)})
	}
	return out, nil
}

// defaultReceipt reads <git-dir>/corvint/test-run.json, the receipt `corvint test` writes when no
// --receipt is given. It is absent when no such file exists, and in-store when the store already
// holds the same bytes (the usual case: `corvint test` appends every receipt it writes).
func defaultReceipt(ctx context.Context, root string, read *RunEvidenceRead) (ResultSource, []TestRunEvidence, error) {
	source := ResultSource{Kind: SourceReceipt, Path: DefaultReceiptLabel, State: SourceAbsent}
	out, err := git(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return source, nil, errors.New("the default receipt cannot be located: the Git directory is unreadable")
	}
	name := filepath.Join(strings.TrimSpace(string(out)), "corvint", "test-run.json")
	if _, err := os.Lstat(name); errors.Is(err, os.ErrNotExist) {
		return source, nil, nil
	}
	raw, err := ReadFile(name)
	if err != nil {
		return source, nil, errors.New("the default receipt " + DefaultReceiptLabel + " is unreadable: " + err.Error())
	}
	if read != nil && read.ids[runevidence.ID(raw)] {
		source.State = SourceInStore
		return source, nil, nil
	}
	records, err := receiptRecords(DefaultReceiptLabel, raw)
	if err != nil {
		return source, nil, err
	}
	source.State, source.Records = SourceRead, len(records)
	return source, records, nil
}

// receiptRecords reads one corvint-test-run/0 receipt written by `corvint test` as result
// records, one per unit outcome, each marked with its row's granularity, which is file: a receipt
// unit is a test file or a package, never one test title (AFL-V1-002, RVD-V1-006). Any other
// profile, a malformed receipt or one over the bound is refused.
func receiptRecords(name string, raw []byte) ([]TestRunEvidence, error) {
	var probe struct {
		Profile *string `json:"profile"`
	}
	if json.Unmarshal(bytes.TrimSpace(raw), &probe) != nil || probe.Profile == nil {
		return nil, errors.New("receipt " + name + " is not a " + runevidence.ProfileReceipt + " document")
	}
	if *probe.Profile != runevidence.ProfileReceipt {
		return nil, errors.New("receipt " + name + " has profile " + *probe.Profile + ", not " + runevidence.ProfileReceipt)
	}
	parsed, err := runevidence.Parse(raw)
	if err != nil {
		return nil, errors.New("receipt " + name + ": " + err.Error())
	}
	if len(parsed.Rows) > maxRunRecords {
		return nil, boundError(BoundRecords)
	}
	return rowRecords(runevidence.ID(raw), parsed.Rows), nil
}

// rowRecords converts normalised receipt rows into LOCALLY_OBSERVED result records that keep each
// row's granularity; they never carry the run header a verified record needs.
func rowRecords(id string, rows []runevidence.Row) []TestRunEvidence {
	records := make([]TestRunEvidence, 0, len(rows))
	for _, row := range rows {
		r := TestRunEvidence{
			Schema: RunEvidenceSchema, Authority: AuthorityLocallyObserved, RunID: id,
			Runner:  RunRunner{Name: row.Instance, Version: row.RunnerVersion},
			Source:  RunSource{Commit: row.Commit, Tree: row.Tree, Clean: row.Clean},
			TestKey: row.TestID, Project: row.Project, Attempts: []RunAttempt{}, Cleanup: "not-declared",
			NegativeControls: []NegativeControl{}, granularity: row.Granularity,
		}
		if row.Outcome != "not-run" {
			attempt := RunAttempt{Ordinal: 1, Outcome: row.Outcome, AssertionAnchors: []RunAnchor{}, Attachments: []RunAttachment{}}
			// A row with no durationMs is NOT_MEASURED: DurationMS stays zero and the attempt is
			// marked, never read as a measured 0 ms.
			if row.DurationMs != nil {
				attempt.DurationMS = *row.DurationMs
			} else {
				attempt.durationNotMeasured = true
			}
			r.Attempts = append(r.Attempts, attempt)
		}
		records = append(records, r)
	}
	return records
}

// testTarget is one declared test link target of a variation: its key and, when the link names
// one, the test file.
type testTarget struct{ key, path string }

// declaredTestTargets is declaredTargets for test links, keeping each key's file path (the
// smallest one when several links name the key) so a file-level result can be joined to it.
func declaredTestTargets(links []EvaluatedLink, from string) []testTarget {
	paths := map[string]string{}
	for _, l := range links {
		if l.ReviewState == ReviewInferred || l.Target.Type != "test" || l.From != from {
			continue
		}
		key := l.Target.TestKey + l.Target.Assertion
		if old, seen := paths[key]; !seen || old == "" || (l.Target.Path != "" && l.Target.Path < old) {
			paths[key] = l.Target.Path
		}
	}
	targets := []testTarget{}
	for _, key := range declaredTargets(links, "test", from) {
		targets = append(targets, testTarget{key: key, path: paths[key]})
	}
	return targets
}

// matchesTarget reports whether record r names target under project, and at which granularity:
// test when the record's key is the target's own, file when a file-level record names the target's
// test file (by path or by its canonical file-level test ID) or its key. A file-level record with
// no project covers every project of the file (AFL-V1-002).
func matchesTarget(r *TestRunEvidence, target testTarget, project string) (string, bool) {
	file := r.granularity == runevidence.GranularityFile
	projectOK := project == "" || r.Project == project || file && r.Project == ""
	if !projectOK {
		return "", false
	}
	if r.TestKey == target.key {
		if file {
			return runevidence.GranularityFile, true
		}
		return runevidence.GranularityTest, true
	}
	if !file || target.path == "" {
		return "", false
	}
	if r.TestKey == target.path {
		return runevidence.GranularityFile, true
	}
	id, err := testid.Parse(r.TestKey)
	if err == nil && id.IsFileLevel() && id.File == target.path {
		return runevidence.GranularityFile, true
	}
	return "", false
}
