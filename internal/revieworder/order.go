// Package revieworder provides an experimental, pure review-priority projection
// (FPK-V0-053..056, proposed pending owner acceptance for V1-0408).
// Repository-history authorship and caller-reported coverage confer no authority.
package revieworder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gitnotes"
	"github.com/Beamfall/corvint/internal/witness"
)

// Value is a factor's determination, never a trust or correctness verdict.
type Value string

const (
	Yes     Value = "yes"
	No      Value = "no"
	Unknown Value = "unknown"
)

// Citation identifies already-read evidence or explicitly explains its absence.
// Object is a Git note blob OID or a SHA-256 digest for CEM/witness inputs.
type Citation struct {
	Source   string
	Revision string
	Object   string
	Path     string
	HunkID   string
	Detail   string
	Trust    string
}

type Factor struct {
	Value    Value
	Citation Citation
}

// Note is the original, already-read Git AI note, pinned to its target commit.
// A nil Note means absent/unavailable, never human authorship.
type Note struct {
	Commit  string
	BlobOID string
	Body    []byte
}

// CEM holds an already-read cem/1.0 map (the Change of wire.ParseStable; a
// retired cem/0.x map never qualifies, CEM-V1-211) and its caller-established
// verification binding.
// Verified must come from verification against the same patch and target, not
// merely successful JSON parsing. Coverage is still a caller-reported observation.
type CEM struct {
	Map            *wire.Map
	TargetRevision string
	MapSha256      string
	Verified       bool
}

// Inputs owns no repository handle. Commits and digests must be full identities;
// Hunks and PatchSha256 come from the same already-read parsed patch.
type Inputs struct {
	BaseRevision   string
	TargetRevision string
	PatchSha256    string
	Hunks          []patch.Hunk
	Note           *Note
	Witness        *witness.Report
	CEM            *CEM
}

// Hunk is a ranked row. Ordinal is one-based original patch order and the final
// tie-break, so even identical duplicate inputs remain deterministic.
type Hunk struct {
	ID          string
	Path        string
	OldRange    wire.Range
	NewRange    wire.Range
	Ordinal     int
	AIAuthored  Factor
	Unwitnessed Factor
	Uncovered   Factor
}

// Rank performs no I/O and does not mutate inputs. Priority is lexicographic:
// AIAuthored, Unwitnessed, Uncovered, each yes > unknown > no; then path, new
// range start/count, old range start/count, ID and original ordinal ascending.
func Rank(in Inputs) []Hunk {
	rows := make([]Hunk, 0, len(in.Hunks))
	authors, noteReason := readNote(in.Note, in.TargetRevision)
	witnessDigest := ""
	if in.Witness != nil {
		raw, _ := json.Marshal(in.Witness)
		sum := sha256.Sum256(raw)
		witnessDigest = hex.EncodeToString(sum[:])
	}
	for i, h := range in.Hunks {
		lines := addedLines(h)
		authored, aiLines := authorship(in, h, lines, authors, noteReason)
		coverageLines := lines
		if len(aiLines) != 0 {
			coverageLines = aiLines
		}
		rows = append(rows, Hunk{
			ID: h.ID, Path: h.DisplayPath, OldRange: h.OldRange, NewRange: h.NewRange, Ordinal: i + 1,
			AIAuthored: authored, Unwitnessed: witnessing(in, h, witnessDigest),
			Uncovered: coverage(in, h, coverageLines),
		})
	}
	priority := map[Value]int{Yes: 0, Unknown: 1, No: 2}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		av := []Value{a.AIAuthored.Value, a.Unwitnessed.Value, a.Uncovered.Value}
		bv := []Value{b.AIAuthored.Value, b.Unwitnessed.Value, b.Uncovered.Value}
		for k := range av {
			if av[k] != bv[k] {
				return priority[av[k]] < priority[bv[k]]
			}
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		an := []int64{a.NewRange.Start, a.NewRange.Count, a.OldRange.Start, a.OldRange.Count}
		bn := []int64{b.NewRange.Start, b.NewRange.Count, b.OldRange.Start, b.OldRange.Count}
		for k := range an {
			if an[k] != bn[k] {
				return an[k] < bn[k]
			}
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Ordinal < b.Ordinal
	})
	return rows
}

func addedLines(h patch.Hunk) []int64 {
	lines := []int64{}
	line := h.NewRange.Start
	for _, body := range h.Body {
		switch body.Prefix {
		case '+':
			lines = append(lines, line)
			line++
		case ' ':
			line++
		}
	}
	return lines
}

func factor(source, revision, object string, h patch.Hunk, trust, reason string) Factor {
	return Factor{Value: Unknown, Citation: Citation{
		Source: source, Revision: revision, Object: object, Path: h.DisplayPath,
		HunkID: h.ID, Trust: trust, Detail: reason,
	}}
}

func authorship(in Inputs, h patch.Hunk, lines []int64, authors []attribution, reason string) (Factor, []int64) {
	object := ""
	if in.Note != nil {
		object = in.Note.BlobOID
	}
	f := factor(gitnotes.ForeignRef, in.TargetRevision, object, h, gitnotes.Trust, reason)
	ai := []int64{}
	if reason != "" {
		return f, ai
	}
	if len(lines) == 0 {
		f.Citation.Detail = "no-added-lines"
		return f, ai
	}
	incomplete := false
	for _, line := range lines {
		value := lineAuthor(authors, h.DisplayPath, line)
		if value == Unknown {
			incomplete = true
		}
		if value == Yes {
			ai = append(ai, line)
		}
	}
	if len(ai) != 0 {
		f.Value, f.Citation.Detail = Yes, "ai-attested-added-lines"
		if incomplete {
			f.Citation.Detail += ";some-line-authorship-unknown"
		}
		return f, ai
	}
	if incomplete {
		f.Citation.Detail = "missing-or-conflicting-line-authorship"
		return f, ai
	}
	f.Value, f.Citation.Detail = No, "explicit-human-added-lines"
	return f, ai
}

func witnessing(in Inputs, h patch.Hunk, digest string) Factor {
	f := factor(witness.Profile, in.TargetRevision, digest, h, "obligation-report", "witness-not-supplied")
	r := in.Witness
	if r == nil {
		return f
	}
	if !commitID(in.BaseRevision) || !commitID(in.TargetRevision) || r.Profile != witness.Profile ||
		r.Range.Base != in.BaseRevision || r.Range.Head != in.TargetRevision {
		f.Citation.Detail = "witness-binding-mismatch"
		return f
	}
	var match *witness.Obligation
	for i := range r.Obligations {
		o := &r.Obligations[i]
		if o.Kind != witness.KindChange || o.Path != h.DisplayPath {
			continue
		}
		if match != nil {
			f.Citation.Detail = "ambiguous-change-obligation"
			return f
		}
		match = o
	}
	if match == nil {
		f.Citation.Detail = "change-obligation-missing"
		return f
	}
	f.Citation.Detail = "path-obligation:" + match.ID + ":" + string(match.Verdict)
	if match.Verdict == witness.Unproven {
		f.Value = Yes
	}
	if match.Verdict == witness.Closed {
		for _, w := range match.Witnesses {
			if w.HunkID == h.ID && w.Closing {
				f.Value = No
				break
			}
		}
		if f.Value == Unknown {
			f.Citation.Detail = "closed-path-without-hunk-witness"
		}
	}
	return f
}

func coverage(in Inputs, h patch.Hunk, lines []int64) Factor {
	f := factor("cem/coverage", in.TargetRevision, "", h, "caller-reported", "cem-not-supplied")
	c := in.CEM
	if c == nil || c.Map == nil {
		return f
	}
	f.Citation.Object = c.MapSha256
	if !c.Verified || !commitID(in.BaseRevision) || !commitID(in.TargetRevision) ||
		!digestID(c.MapSha256) || !digestID(in.PatchSha256) || c.Map.Spec != wire.StableSpec ||
		c.Map.BaseRevision != in.BaseRevision || c.TargetRevision != in.TargetRevision || c.Map.PatchSha256 != in.PatchSha256 {
		f.Citation.Detail = "cem-binding-unverified-or-mismatched"
		return f
	}
	var match *wire.Hunk
	for i := range c.Map.Hunks {
		m := &c.Map.Hunks[i]
		if m.ID != h.ID {
			continue
		}
		if match != nil {
			f.Citation.Detail = "ambiguous-cem-hunk"
			return f
		}
		match = m
	}
	if match == nil {
		f.Citation.Detail = "cem-hunk-missing"
		return f
	}
	if match.Path != h.DisplayPath || match.NewRange != h.NewRange || match.OldRange != h.OldRange {
		f.Citation.Detail = "cem-hunk-mismatch"
		return f
	}
	if len(lines) == 0 {
		f.Citation.Detail = "no-added-lines"
		return f
	}
	w := match.Coverage
	if w == nil {
		f.Citation.Detail = "coverage-witness-missing"
		return f
	}
	if !digestID(w.ProfileSha256) || w.TestRun == "" ||
		(w.Mode != "set" && w.Mode != "count" && w.Mode != "atomic") ||
		(w.State != wire.CoverageCovered && w.State != wire.CoverageUncovered) ||
		(w.State == wire.CoverageCovered && len(w.Covered) == 0) ||
		(w.State == wire.CoverageUncovered && len(w.Covered) != 0) {
		f.Citation.Detail = "coverage-witness-invalid"
		return f
	}
	for _, r := range w.Covered {
		if r.Start < h.NewRange.Start || r.Count <= 0 ||
			r.Start-h.NewRange.Start >= h.NewRange.Count ||
			r.Count > h.NewRange.Count-(r.Start-h.NewRange.Start) {
			f.Citation.Detail = "coverage-range-invalid"
			return f
		}
	}
	f.Citation.Detail = "coverage-profile:" + w.ProfileSha256 + ";test-run:" + w.TestRun
	f.Value = No
	for _, line := range lines {
		covered := false
		for _, r := range w.Covered {
			if line >= r.Start && line-r.Start < r.Count {
				covered = true
				break
			}
		}
		if !covered {
			f.Value = Yes
			break
		}
	}
	return f
}

func commitID(s string) bool { return (len(s) == 40 || len(s) == 64) && hexID(s) }
func digestID(s string) bool { return len(s) == 64 && hexID(s) }
func hexID(s string) bool {
	for _, c := range s {
		if !('0' <= c && c <= '9') && !('a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}
