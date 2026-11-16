// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/runevidence"
	"github.com/Beamfall/corvint/internal/testid"
)

// RunEvidenceRead is what the run evidence store gave a report (RVD-V1-007, RVD-V1-010): how many
// records it read, the records it skipped and the tests they held, and the reasons it prints.
type RunEvidenceRead struct {
	Records        int      `json:"records"`
	LogUnreadable  bool     `json:"log_unreadable,omitempty"`
	CorruptRecords []string `json:"corrupt_records,omitempty"`
	CorruptTests   []string `json:"corrupt_tests,omitempty"`
	Truncated      bool     `json:"truncated,omitempty"`
	Reasons        []string `json:"reasons"`
}

// RunEvidenceStore is the run evidence store of the repository at root:
// <git-dir>/corvint/evidence (RVD-V1-001).
func RunEvidenceStore(ctx context.Context, root string) (runevidence.Store, error) {
	out, err := git(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return runevidence.Store{}, errors.New("the run evidence store cannot be located: the Git directory is unreadable")
	}
	return runevidence.Store{Dir: runevidence.Locate(strings.TrimSpace(string(out)))}, nil
}

// ReadStoredRunEvidence reads the store without writing it (RVD-V1-012) and returns its records as
// test-run-evidence/0 records in write order, oldest first, so the newest record that names a test
// is the last one a report sees (RVD-V1-007). An ingested record gives its own lines; a receipt
// gives one LOCALLY_OBSERVED record per row, which never carries the run header a verified record
// needs. At most maxRunRecords records are returned, the newest kept, with Truncated set.
func ReadStoredRunEvidence(ctx context.Context, root string) ([]TestRunEvidence, *RunEvidenceRead, error) {
	store, err := RunEvidenceStore(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	snapshot, err := store.Read()
	if err != nil {
		return nil, nil, errors.New("the run evidence store cannot be read")
	}
	read := &RunEvidenceRead{LogUnreadable: snapshot.LogUnreadable, Reasons: slices.Clone(snapshot.Reasons)}
	corrupt := func(c runevidence.Corrupt) {
		read.CorruptRecords = append(read.CorruptRecords, c.ID)
		read.CorruptTests = append(read.CorruptTests, c.Tests...)
		if !slices.Contains(read.Reasons, runevidence.ReasonCorrupt) {
			read.Reasons = append(read.Reasons, runevidence.ReasonCorrupt)
		}
	}
	for _, c := range snapshot.Corrupt {
		corrupt(c)
	}
	var batches [][]TestRunEvidence
	total := 0
	for index := len(snapshot.Records) - 1; index >= 0; index-- {
		record := snapshot.Records[index]
		converted, err := storedRecords(record)
		if err != nil {
			corrupt(runevidence.Corrupt{ID: record.Entry.ID, Tests: record.Entry.Tests})
			continue
		}
		if total+len(converted) > maxRunRecords {
			read.Truncated = true
			break
		}
		total += len(converted)
		read.Records++
		batches = append(batches, converted)
	}
	records := make([]TestRunEvidence, 0, total)
	for index := len(batches) - 1; index >= 0; index-- {
		records = append(records, batches[index]...)
	}
	slices.Sort(read.CorruptTests)
	read.CorruptTests = slices.Compact(read.CorruptTests)
	if read.Reasons == nil {
		read.Reasons = []string{}
	}
	return records, read, nil
}

// storedRecords converts one stored record.
func storedRecords(record runevidence.Record) ([]TestRunEvidence, error) {
	if record.Entry.Profile == runevidence.ProfileIngested {
		var records []TestRunEvidence
		for line := range bytes.Lines(record.Data) {
			r, err := DecodeRunEvidence(line)
			if err != nil {
				return nil, err
			}
			records = append(records, r)
		}
		return records, nil
	}
	return rowRecords(record.Entry.ID, record.Rows), nil
}

// RunEvidenceFor returns the records a report reads: the store's, unless noStore, followed by those
// of the named files, so a named file adds to the store and never replaces it (RVD-V1-007). read is
// nil with noStore.
func RunEvidenceFor(ctx context.Context, root string, files []string, noStore bool) ([]TestRunEvidence, *RunEvidenceRead, error) {
	named, err := ReadRunEvidence(files)
	if err != nil || noStore {
		return named, nil, err
	}
	return WithStoredRunEvidence(ctx, root, named)
}

// WithStoredRunEvidence prepends the store's records to records a caller already read.
func WithStoredRunEvidence(ctx context.Context, root string, records []TestRunEvidence) ([]TestRunEvidence, *RunEvidenceRead, error) {
	stored, read, err := ReadStoredRunEvidence(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	return append(stored, records...), read, nil
}

// corruptKey reports whether a test and project pair was held by a record the read skipped.
func (r *RunEvidenceRead) corruptKey(key, project string) bool {
	if r == nil {
		return false
	}
	want := key
	if project != "" {
		want = key + " @" + project
	}
	_, found := slices.BinarySearch(r.CorruptTests, want)
	return found
}

// present reports whether the read is worth printing in a report.
func (r *RunEvidenceRead) present() bool {
	return r != nil && (r.Records != 0 || len(r.CorruptRecords) != 0 || len(r.Reasons) != 0)
}

// FlowImpactResults is FlowImpactAt with each reached flow's last result per test key when evidence
// holds a record (RVD-V1-007): the stored result, its tree and whether that tree is HEAD's. With no
// evidence the report is FlowImpactAt's, byte for byte.
func FlowImpactResults(ctx context.Context, root string, set IntentSet, base string, evidence []TestRunEvidence) ([]byte, error) {
	data, err := FlowImpactAt(ctx, root, set, base)
	if err != nil || len(evidence) == 0 {
		return data, err
	}
	var report ImpactReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, errors.New("flows impact produced an unreadable report")
	}
	tree, err := git(ctx, root, "rev-parse", "--verify", "--end-of-options", report.Revision+"^{tree}")
	if err != nil {
		return nil, errors.New("evaluated revision has no tree")
	}
	at := head{commit: report.Revision, tree: strings.TrimSpace(string(tree))}
	for index := range report.Flows {
		flow := &report.Flows[index]
		for _, key := range flow.TestKeys {
			flow.LastResults = append(flow.LastResults, lastResult(evidence, key, "", at))
		}
	}
	return doccorpus.Encode(report)
}

// The kinds and states of one result source a flows status report read (AFL-V1-002).
const (
	SourceStore    = "run-evidence-store"
	SourceEvidence = "evidence"

	SourceRead    = "read"
	SourceSkipped = "not-read"
)

// ResultSource is one place a flows status report looked for test results (AFL-V1-002): the run
// evidence store or a named evidence file. Records is how many result records it gave the report;
// a store read with none is listed, so a report of only NOT_OBSERVED results names what it missed.
type ResultSource struct {
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	State   string `json:"state"`
	Records int    `json:"records"`
}

// ResultInputs are the result sources a caller names: test-run-evidence/0 files written by
// `flows ingest`, and whether to skip the run evidence store.
type ResultInputs struct {
	Evidence []string
	NoStore  bool
	// Resolve maps a named file to the path to open; nil opens the name as given. The report
	// always prints the name as given, so a caller that confines names to the root (the MCP
	// tools) and one that passes them through (the command) print the same bytes.
	Resolve func(name string) (string, error)
}

// Results are the records a flows report joins, in the order the newest-record rule reads them,
// with what the store read gave and every source the report looked at.
type Results struct {
	Records []TestRunEvidence
	Store   *RunEvidenceRead
	Sources []ResultSource
}

// ReadResults reads the only two writers a flows status result may come from (AFL-V1-002): the run
// evidence store, which holds every corvint-test-run/0 receipt `corvint test` and `corvint watch`
// write (RVD-V1-004) and every test-run-evidence/0 document `flows ingest --record` appends
// (RVD-V1-005), normalised by the store's one receipt reader (RVD-V1-006), then each named
// test-run-evidence/0 file. It opens no receipt file itself: STR-V0-011 admits no other receipt
// reader. A named file that is missing, malformed, oversized or not test-run-evidence/0 refuses
// the whole read (AFL-V1-006). Nothing is written (RVD-V1-012).
func ReadResults(ctx context.Context, root string, in ResultInputs) (Results, error) {
	var out Results
	if in.NoStore {
		out.Sources = append(out.Sources, ResultSource{Kind: SourceStore, State: SourceSkipped})
	} else {
		stored, read, err := ReadStoredRunEvidence(ctx, root)
		if err != nil {
			return Results{}, err
		}
		out.Store, out.Records = read, stored
		out.Sources = append(out.Sources, ResultSource{Kind: SourceStore, State: SourceRead, Records: len(stored)})
	}
	for _, name := range in.Evidence {
		file := name
		if in.Resolve != nil {
			var err error
			if file, err = in.Resolve(name); err != nil {
				return Results{}, err
			}
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
