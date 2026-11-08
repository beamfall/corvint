// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/Beamfall/corvint/internal/testevidence"
)

// EvidenceRecords exposes existing Go and JS indexes without inventing runner,
// target or trace identity (RSP-V0-005, RSP-V0-008). Such legacy records cannot
// pass Record.Validate/Fresh until recollected with the missing identities.
// Native RSP records keep their repository IDs; repository names the legacy root.
func (ix *Index) EvidenceRecords(repository string) ([]testevidence.Record, error) {
	if err := ix.checkEvidence(); err != nil {
		return nil, err
	}
	var out []testevidence.Record
	represented := map[string]bool{}
	if len(ix.Evidence) > 0 {
		// A detached copy prevents callers from mutating the immutable index.
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(ix.Evidence); err != nil {
			return nil, err
		}
		if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
			return nil, err
		}
		for _, r := range out {
			if r.Repository == repository {
				represented[r.Test] = true
			}
		}
	}
	if repository == "" {
		return nil, errors.New("linecov: evidence repository missing")
	}

	for i, t := range ix.Tests {
		if represented[t.Package+"/"+t.Name] {
			continue
		}
		r := testevidence.New(repository, t.Package+"/"+t.Name, ix.Commit)
		r.Executed.State = testevidence.Present
		r.Executed.Reason = ""
		if t.Incomplete {
			r.Executed.State = testevidence.Incomplete
			r.Executed.Reason = "legacy-record-incomplete"
		}
		for _, f := range ix.Files {
			ef := testevidence.ExecutedFile{Blob: testevidence.Blob{Path: testevidence.Path{Repository: repository, Lexical: f.Path, Resolved: f.Path}, OID: f.OID}}
			for _, b := range f.Blocks {
				// Native Go coverage includes empty bodies and clauses. Their
				// positive, zero-width positions cannot be expressed by Range.
				// Retain the blob and abstain, rather than invent covered bytes
				// or silently lose evidence. All other shapes still validate.
				if (ix.Language == "" || ix.Language == "go") && b.Start > 0 && b.StartCol > 0 && b.Start == b.End && b.StartCol == b.EndCol {
					r.Executed.State = testevidence.Incomplete
					r.Executed.Reason = "legacy-record-incomplete"
					continue
				}
				span := testevidence.Range{Start: b.Start, StartColumn: b.StartCol, End: b.End, EndColumn: b.EndCol}
				if slices.Contains(b.Tests, int32(i)) {
					ef.Executed = append(ef.Executed, span)
				} else {
					ef.Unexecuted = append(ef.Unexecuted, span)
				}
			}
			r.Executed.Items = append(r.Executed.Items, ef)
		}
		if err := r.ValidateComponents(); err != nil {
			return nil, fmt.Errorf("linecov: legacy evidence incomplete: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

func (ix *Index) checkEvidence() error {
	seen := map[testevidence.TargetKey]bool{}
	for _, r := range ix.Evidence {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("linecov: evidence incomplete: %w", err)
		}
		if r.Commit != ix.Commit {
			return errors.New("linecov: evidence commit mismatch")
		}
		k, _ := r.TargetKey()
		if seen[k] {
			return errors.New("linecov: evidence duplicate target")
		}
		seen[k] = true
	}
	return nil
}

// IndexSizeLimit is the existing immutable-index bound, strictly less than the
// larger of 1% of packed repository bytes and 64 MiB (RSP-V0-008).
func IndexSizeLimit(packedBytes int64) int64 {
	return max(64<<20, packedBytes/100)
}

// EncodeBounded admits the extended index only below the size bound. It stages
// bytes before publishing, so a cap or invalid record writes nothing to w.
func (ix *Index) EncodeBounded(w io.Writer, packedBytes int64) error {
	if packedBytes < 0 {
		return errors.New("linecov: packed size unknown")
	}
	var buf bytes.Buffer
	bound := &indexBoundWriter{w: &buf, remaining: IndexSizeLimit(packedBytes) - 1}
	if err := ix.checkEvidence(); err != nil {
		return err
	}
	if err := gob.NewEncoder(bound).Encode(ix); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

type indexBoundWriter struct {
	w         io.Writer
	remaining int64
}

func (w *indexBoundWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.remaining {
		return 0, errors.New("linecov: evidence index size cap")
	}
	n, err := w.w.Write(b)
	w.remaining -= int64(n)
	return n, err
}
