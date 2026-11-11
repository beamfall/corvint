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
	records := make([]TestRunEvidence, 0, len(record.Rows))
	for _, row := range record.Rows {
		r := TestRunEvidence{
			Schema: RunEvidenceSchema, Authority: AuthorityLocallyObserved, RunID: record.Entry.ID,
			Runner:  RunRunner{Name: row.Instance, Version: row.RunnerVersion},
			Source:  RunSource{Commit: row.Commit, Tree: row.Tree, Clean: row.Clean},
			TestKey: row.TestID, Project: row.Project, Attempts: []RunAttempt{}, Cleanup: "not-declared",
			NegativeControls: []NegativeControl{},
		}
		if row.Outcome != "not-run" {
			attempt := RunAttempt{Ordinal: 1, Outcome: row.Outcome, AssertionAnchors: []RunAnchor{}, Attachments: []RunAttachment{}}
			if row.DurationMs != nil {
				attempt.DurationMS = *row.DurationMs
			}
			r.Attempts = append(r.Attempts, attempt)
		}
		records = append(records, r)
	}
	return records, nil
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
