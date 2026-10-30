package wire

import (
	"bytes"
	"cmp"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"slices"
)

// PairedEvidenceProfile names the additive paired-range document of
// CAX-V0-030. A cem/1.0 map binds one repository's base and target and takes
// no paired change (CEM-V1-648); this separate document, bound to the map's
// change, names a second repository's range by its two pinned commits and
// holds citations resolved at that range's head. A cem/1.0 consumer that never
// reads it loses nothing it relied on.
const PairedEvidenceProfile = "cem-paired-evidence/1"

// MaxPairedEvidenceBytes bounds one paired-evidence document.
const MaxPairedEvidenceBytes = MaxMapBytes

// PairedEvidence is one cem-paired-evidence/1 document. Members are in
// canonical (sorted) order so EncodePairedEvidence is deterministic.
type PairedEvidence struct {
	Change    PairedChange     `json:"change"`
	Citations []PairedCitation `json:"citations"`
	Profile   string           `json:"profile"`
	Range     PairedRange      `json:"range"`
}

// PairedChange binds the document to one cem/1.0 map's change.
type PairedChange struct {
	BaseRevision string `json:"baseRevision"`
	PatchSha256  string `json:"patchSha256"`
}

// PairedRange pins the second repository's range by its full commit IDs.
type PairedRange struct {
	BaseRevision string `json:"baseRevision"`
	HeadRevision string `json:"headRevision"`
	Name         string `json:"name"`
}

// PairedCitation is one span of a regular blob at the range head, cited for
// one hunk of the map.
type PairedCitation struct {
	BlobOid    string     `json:"blobOid"`
	HunkID     string     `json:"hunkId"`
	Path       string     `json:"path"`
	Relation   string     `json:"relation"`
	Span       PairedSpan `json:"span"`
	SpanSha256 string     `json:"spanSha256"`
}

// PairedSpan is a zero-based, end-exclusive byte span.
type PairedSpan struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

func comparePairedCitations(a, b PairedCitation) int {
	return cmp.Or(cmp.Compare(a.HunkID, b.HunkID), cmp.Compare(a.Path, b.Path),
		cmp.Compare(a.Span.Start, b.Span.Start), cmp.Compare(a.Span.End, b.Span.End),
		cmp.Compare(a.Relation, b.Relation))
}

// ValidPairedRangeName reports whether name is 1..64 bytes of lowercase
// letters, digits, '.', '_' or '-', starting with a letter or digit.
func ValidPairedRangeName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for index, c := range []byte(name) {
		alnum := c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
		if !alnum && (index == 0 || c != '.' && c != '_' && c != '-') {
			return false
		}
	}
	return true
}

// EncodePairedEvidence sorts the citations and writes the canonical bytes:
// sorted members, no HTML escaping, no insignificant whitespace, one trailing
// newline.
func EncodePairedEvidence(document PairedEvidence) ([]byte, error) {
	document.Citations = slices.Clone(document.Citations)
	slices.SortFunc(document.Citations, comparePairedCitations)
	if document.Citations == nil {
		document.Citations = []PairedCitation{}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ParsePairedEvidence reads a cem-paired-evidence/1 document (CAX-V0-031):
// closed and canonical bytes, the profile name, full lowercase commit and
// blob IDs, a SHA-256 patch and span digest, a valid range name, a base that
// differs from the head, and at least one citation, each with a valid path, a
// registered relation, a non-empty span within MaxCitationSpanBytes, sorted
// and unique. Any failure refuses the whole document.
func ParsePairedEvidence(document []byte) (*PairedEvidence, error) {
	if len(document) > MaxPairedEvidenceBytes {
		return nil, errors.New("paired evidence: document exceeds its byte bound")
	}
	var parsed PairedEvidence
	if err := jsonv2.Unmarshal(document, &parsed, jsonv2.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("paired evidence: %w", err)
	}
	canonical, err := EncodePairedEvidence(parsed)
	if err != nil || !bytes.Equal(canonical, document) {
		return nil, errors.New("paired evidence: document is not in canonical form or lacks a member")
	}
	if parsed.Profile != PairedEvidenceProfile {
		return nil, fmt.Errorf("paired evidence: profile %q is not %s", parsed.Profile, PairedEvidenceProfile)
	}
	if !IsGitOid(parsed.Change.BaseRevision) || !IsSha256(parsed.Change.PatchSha256) {
		return nil, errors.New("paired evidence: change needs a full baseRevision and a SHA-256 patchSha256")
	}
	pin := parsed.Range
	if !ValidPairedRangeName(pin.Name) || !IsGitOid(pin.BaseRevision) || !IsGitOid(pin.HeadRevision) || pin.BaseRevision == pin.HeadRevision {
		return nil, errors.New("paired evidence: range needs a valid name and two distinct full commit IDs")
	}
	if len(parsed.Citations) == 0 {
		return nil, errors.New("paired evidence: a document holds at least one citation")
	}
	for index, citation := range parsed.Citations {
		if index > 0 && comparePairedCitations(parsed.Citations[index-1], citation) == 0 {
			return nil, fmt.Errorf("paired evidence: citation %d repeats the previous one", index)
		}
		if err := ValidatePath(citation.Path); err != nil {
			return nil, fmt.Errorf("paired evidence: citation %d: %w", index, err)
		}
		if !IsGitOid(citation.BlobOid) || !IsSha256(citation.SpanSha256) || citation.HunkID == "" || !Relations[citation.Relation] {
			return nil, fmt.Errorf("paired evidence: citation %d needs a blob ID, a span digest, a hunk ID and a registered relation", index)
		}
		if citation.Span.Start < 0 || citation.Span.End <= citation.Span.Start || citation.Span.End-citation.Span.Start > MaxCitationSpanBytes {
			return nil, fmt.Errorf("paired evidence: citation %d span is empty, negative or longer than %d bytes", index, MaxCitationSpanBytes)
		}
	}
	return &parsed, nil
}

// BindPairedEvidence checks that a parsed document names the change of the
// exact cem/1.0 map bytes supplied and that every citation names one of its
// hunks (CAX-V0-031).
func BindPairedEvidence(document *PairedEvidence, mapBytes []byte) (*StableMap, error) {
	stable, err := ParseStable(mapBytes)
	if err != nil {
		return nil, fmt.Errorf("paired evidence: the named map is not cem/1.0: %w", err)
	}
	if document.Change.BaseRevision != stable.Change.BaseRevision || document.Change.PatchSha256 != stable.Change.PatchSha256 {
		return nil, errors.New("paired evidence: change does not name the supplied map's baseRevision and patchSha256")
	}
	hunks := map[string]bool{}
	for _, hunk := range stable.Change.Hunks {
		hunks[hunk.ID] = true
	}
	for _, citation := range document.Citations {
		if !hunks[citation.HunkID] {
			return nil, fmt.Errorf("paired evidence: citation names hunk %q the map does not hold", citation.HunkID)
		}
	}
	return stable, nil
}
