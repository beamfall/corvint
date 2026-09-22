// Package verify composes the CEM seams into the frozen repository-conformant
// verifier: exact-patch (cem/0.1) and canonical (cem/0.2) verification with
// the frozen validation precedence, mechanical byte proofs, and same-path
// evidence drift.
package verify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/sim"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// DriftStatus enumerates the frozen same-path drift states.
type DriftStatus string

const (
	DriftStable    DriftStatus = "stable"
	DriftRelocated DriftStatus = "relocated"
	DriftStale     DriftStatus = "stale"
	DriftAmbiguous DriftStatus = "ambiguous"
	DriftDeleted   DriftStatus = "deleted"
)

// DriftItem reports one evidence record's target drift.
type DriftItem struct {
	EvidenceID    string
	Status        DriftStatus
	TargetBlobOid string     // empty for deleted
	TargetSpan    *wire.Span // exact target range for stable and relocated
}

// ClassifySpan applies the frozen same-path drift classifier to facts gathered
// by either a verifier or a producer simulating the target.
func ClassifySpan(targetExists, unchanged bool, targetData, needle []byte, original wire.Span) (DriftStatus, *wire.Span) {
	if !targetExists {
		return DriftDeleted, nil
	}
	if unchanged {
		span := original
		return DriftStable, &span
	}
	matches, firstOffset := countOverlapping(targetData, needle)
	switch matches {
	case 0:
		return DriftStale, nil
	case 1:
		return DriftRelocated, &wire.Span{Start: firstOffset, End: firstOffset + int64(len(needle))}
	default:
		return DriftAmbiguous, nil
	}
}

// Outcome reports one accepted verification.
type Outcome struct {
	BaseRevision   string
	TargetRevision string // empty when no target was checked
	Drift          []DriftItem
}

// ExactOptions configure 0.1 exact-patch verification.
type ExactOptions struct {
	// ExpectedBase, when non-empty, independently resolves and checks the
	// map's baseRevision at its historical position after digest validation.
	ExpectedBase string
	// Target, when non-empty, runs the inherited evidence drift check.
	Target string
}

// Exact verifies a cem/0.1 map against exact caller-supplied patch bytes.
func Exact(ctx context.Context, repository *gitauth.Repository, document *wire.Map, patchBytes []byte, options ExactOptions) (*Outcome, error) {
	if document.Spec != wire.Spec01 {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "exact-patch verification accepts cem/0.1 only")
	}
	if err := checkPatchDigest(document, patchBytes); err != nil {
		return nil, err
	}
	if options.ExpectedBase != "" {
		if err := checkExpectedBase(ctx, repository, document, options.ExpectedBase); err != nil {
			return nil, err
		}
	}
	return inherited(ctx, repository, document, patchBytes, options.Target)
}

// CanonicalOptions configure 0.2 canonical verification.
type CanonicalOptions struct {
	ExpectedBase string // required independent producer baseline
	Target       string // required caller target revision
	// RawMapBytes is the exact bounded raw cem/0.2 input being verified, used
	// for the target-side sidecar artifact binding.
	RawMapBytes []byte
}

// Canonical verifies a cem/0.2 map with independent base and target authority,
// deriving the canonical patch itself. It follows the frozen stage-7/8/9
// precedence: expected-base resolution and equality, target resolution, the
// base-side historical-sidecar check, the target-side raw artifact comparison,
// canonical derivation, inherited verification, then drift.
//
// The returned bound flag reports whether independent repository derivation
// and target binding both completed (CEM-CB-014): it is false for every
// failure before the canonical patch was derived, and true from the digest
// check onward, so callers claim "canonical" assurance only when it was
// actually established.
func Canonical(ctx context.Context, repository *gitauth.Repository, document *wire.Map, options CanonicalOptions) (*Outcome, bool, error) {
	if document.Spec != wire.Spec02 {
		return nil, false, cemcode.New(cemcode.UnsupportedSpec, "canonical verification accepts cem/0.2 only")
	}
	if options.ExpectedBase == "" {
		return nil, false, cemcode.New(cemcode.ExpectedBaseRequired, "canonical verification requires an independent expected base")
	}
	if options.Target == "" {
		return nil, false, cemcode.New(cemcode.TargetRequired, "canonical verification requires an independent target")
	}
	defer repository.BeginObjectSession()()
	if err := checkExpectedBase(ctx, repository, document, options.ExpectedBase); err != nil {
		return nil, false, err
	}
	targetOID, err := repository.Resolve(ctx, options.Target)
	if err != nil {
		return nil, false, err
	}
	if err := checkBaseSidecar(ctx, repository, document.BaseRevision); err != nil {
		return nil, false, err
	}
	if err := checkTargetSidecar(ctx, repository, targetOID, options.RawMapBytes); err != nil {
		return nil, false, err
	}
	patchBytes, err := repository.CanonicalDiff(ctx, document.BaseRevision, targetOID)
	if err != nil {
		return nil, false, err
	}
	if err := checkPatchDigest(document, patchBytes); err != nil {
		return nil, true, err
	}
	outcome, err := inherited(ctx, repository, document, patchBytes, targetOID)
	return outcome, true, err
}

func checkPatchDigest(document *wire.Map, patchBytes []byte) error {
	digest := sha256.Sum256(patchBytes)
	if hex.EncodeToString(digest[:]) != document.PatchSha256 {
		return cemcode.New(cemcode.PatchDigestMismatch, "patch bytes do not match patchSha256")
	}
	return nil
}

func checkExpectedBase(ctx context.Context, repository *gitauth.Repository, document *wire.Map, expected string) error {
	resolved, err := repository.Resolve(ctx, expected)
	if err != nil {
		return err
	}
	if resolved != document.BaseRevision {
		return cemcode.New(cemcode.BaseRevisionMismatch, "map baseRevision does not equal the independent expected base")
	}
	return nil
}

// checkBaseSidecar applies CEM-CB-008: an absent base-side sidecar is fine; a
// present one must be exactly one regular 100644 blob.
func checkBaseSidecar(ctx context.Context, repository *gitauth.Repository, baseOID string) error {
	entry, exists, err := repository.LookupTreeEntry(ctx, baseOID, wire.ExcludedCEMPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if entry.Type != "blob" || entry.Mode != "100644" {
		return cemcode.New(cemcode.ExcludedPathNotFile, "base-side %s is not a regular 100644 blob", wire.ExcludedCEMPath)
	}
	return nil
}

// checkTargetSidecar applies CEM-CB-009: a present target-side sidecar must be
// a regular 100644 blob whose raw bytes equal the verified CEM input.
func checkTargetSidecar(ctx context.Context, repository *gitauth.Repository, targetOID string, rawMap []byte) error {
	entry, exists, err := repository.LookupTreeEntry(ctx, targetOID, wire.ExcludedCEMPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if entry.Type != "blob" || entry.Mode != "100644" {
		return cemcode.New(cemcode.ExcludedArtifactMismatch, "target-side %s is not a regular 100644 blob", wire.ExcludedCEMPath)
	}
	stored, err := repository.BlobBytes(ctx, entry.OID)
	if err != nil {
		return err
	}
	if !bytes.Equal(stored, rawMap) {
		return cemcode.New(cemcode.ExcludedArtifactMismatch, "target-side %s bytes differ from the verified CEM input", wire.ExcludedCEMPath)
	}
	return nil
}

// inherited runs the shared CEM verification: parse, base simulation, hunk
// coverage, evidence resolution, mechanical proofs, then optional drift.
func inherited(ctx context.Context, repository *gitauth.Repository, document *wire.Map, patchBytes []byte, target string) (*Outcome, error) {
	baseOID, err := repository.Resolve(ctx, document.BaseRevision)
	if err != nil {
		return nil, err
	}
	if baseOID != document.BaseRevision {
		return nil, cemcode.New(cemcode.BaseRevisionMismatch, "baseRevision does not resolve to itself as a full commit OID")
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return nil, err
	}
	source := &baseSource{ctx: ctx, repository: repository, base: baseOID}
	if err := sim.Simulate(parsed, source); err != nil {
		return nil, err
	}
	if err := checkHunkCoverage(document, parsed); err != nil {
		return nil, err
	}
	if err := checkEvidence(ctx, repository, document, baseOID); err != nil {
		return nil, err
	}
	if err := checkMechanical(document, parsed); err != nil {
		return nil, err
	}
	outcome := &Outcome{BaseRevision: baseOID}
	if target != "" {
		if err := checkDrift(ctx, repository, document, baseOID, target, outcome); err != nil {
			// A drift rejection keeps its context: the populated outcome rides
			// beside the error so envelopes never lose the per-item rows.
			return outcome, err
		}
	}
	return outcome, nil
}

// Candidate validates a freshly derived or resumed candidate map against the
// canonical patch bytes without target-side artifact binding: prepare is the
// preceding candidate phase and must ignore inherited target sidecar state.
func Candidate(ctx context.Context, repository *gitauth.Repository, document *wire.Map, patchBytes []byte) error {
	if err := checkPatchDigest(document, patchBytes); err != nil {
		return err
	}
	_, err := inherited(ctx, repository, document, patchBytes, "")
	return err
}

// baseSource reads base blobs through Git object identity with tree-mode
// enforcement, never through the worktree.
type baseSource struct {
	ctx        context.Context
	repository *gitauth.Repository
	base       string
}

func (s *baseSource) BaseBlob(path string) ([]byte, string, bool, error) {
	entry, exists, err := s.repository.LookupTreeEntry(s.ctx, s.base, path)
	if err != nil || !exists {
		return nil, "", false, err
	}
	if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return nil, "", false, cemcode.New(cemcode.BaseMismatch,
			"base path %q is not a regular blob", path)
	}
	data, err := s.repository.BlobBytes(s.ctx, entry.OID)
	if err != nil {
		return nil, "", false, err
	}
	return data, entry.Mode, true, nil
}

// checkHunkCoverage requires every parsed hunk exactly once and nothing else,
// with ranges and display path matching the recomputed identity.
func checkHunkCoverage(document *wire.Map, parsed *patch.Patch) error {
	derived := map[string]*patch.Hunk{}
	for _, hunk := range parsed.Hunks {
		derived[hunk.ID] = hunk
	}
	seen := map[string]bool{}
	for _, mapped := range document.Hunks {
		match, ok := derived[mapped.ID]
		if !ok {
			return cemcode.New(cemcode.InvalidField, "map hunk %s is not derived from the patch", mapped.ID)
		}
		if seen[mapped.ID] {
			return cemcode.New(cemcode.InvalidField, "map hunk %s appears more than once", mapped.ID)
		}
		seen[mapped.ID] = true
		if mapped.Path != match.DisplayPath || mapped.OldRange != match.OldRange || mapped.NewRange != match.NewRange {
			return cemcode.New(cemcode.InvalidField, "map hunk %s disagrees with the derived hunk", mapped.ID)
		}
	}
	for _, hunk := range parsed.Hunks {
		if !seen[hunk.ID] {
			return cemcode.New(cemcode.UncitedHunk, "patch hunk %s is missing from the map", hunk.ID)
		}
	}
	return nil
}

// checkEvidence resolves every evidence record through the base tree and
// recomputes its span digest and identity.
func checkEvidence(ctx context.Context, repository *gitauth.Repository, document *wire.Map, baseOID string) error {
	for _, record := range document.Evidence {
		entry, exists, err := repository.LookupTreeEntry(ctx, baseOID, record.Path)
		if err != nil {
			return err
		}
		if !exists || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return cemcode.New(cemcode.EvidenceUnavailable,
				"evidence path %q does not resolve to a regular blob at the base", record.Path)
		}
		if entry.OID != record.BlobOid {
			return cemcode.New(cemcode.EvidenceUnavailable,
				"evidence path %q does not resolve to the declared blob", record.Path)
		}
		data, err := repository.BlobBytes(ctx, entry.OID)
		if err != nil {
			return err
		}
		if record.Span.Start >= record.Span.End || record.Span.End > int64(len(data)) {
			return cemcode.New(cemcode.EvidenceUnavailable,
				"evidence span for %q is empty or out of bounds", record.Path)
		}
		spanBytes := data[record.Span.Start:record.Span.End]
		digest := sha256.Sum256(spanBytes)
		if hex.EncodeToString(digest[:]) != record.SpanSha256 {
			return cemcode.New(cemcode.FabricatedEvidenceID,
				"evidence span digest for %q does not match", record.Path)
		}
		derived := wire.EvidenceIdentity(record.BlobOid, record.Path, record.Span, record.SpanSha256)
		if derived != record.ID {
			return cemcode.New(cemcode.FabricatedEvidenceID,
				"evidence ID for %q does not recompute", record.Path)
		}
	}
	return nil
}

// checkMechanical proves every mechanical claim directly from removed and
// added bytes.
func checkMechanical(document *wire.Map, parsed *patch.Patch) error {
	derived := map[string]*patch.Hunk{}
	for _, hunk := range parsed.Hunks {
		derived[hunk.ID] = hunk
	}
	for _, mapped := range document.Hunks {
		if mapped.Disposition != "mechanical" {
			continue
		}
		if !mechanicalProven(derived[mapped.ID], mapped.Reason) {
			return cemcode.New(cemcode.UnprovenMechanical,
				"hunk %s does not prove %s", mapped.ID, mapped.Reason)
		}
	}
	return nil
}

func mechanicalProven(hunk *patch.Hunk, reason string) bool {
	var removed, added []byte
	for _, line := range hunk.Body {
		if line.Prefix == '-' {
			removed = append(removed, line.OldPayload()...)
		}
		if line.Prefix == '+' {
			added = append(added, line.NewPayload()...)
		}
	}
	if len(removed) == 0 || len(added) == 0 || bytes.Equal(removed, added) {
		return false
	}
	switch reason {
	case "line-ending-only":
		crlf := []byte("\r\n")
		lf := []byte("\n")
		return bytes.Equal(bytes.ReplaceAll(removed, crlf, lf), bytes.ReplaceAll(added, crlf, lf))
	case "whitespace-only":
		return whitespaceOnlyProven(removed, added)
	default:
		return false
	}
}

func whitespaceOnlyProven(removed, added []byte) bool {
	if bytes.Count(removed, []byte{'\n'}) != bytes.Count(added, []byte{'\n'}) {
		return false
	}
	removedRecords, err := patch.SplitLF(removed)
	if err != nil {
		return false
	}
	addedRecords, err := patch.SplitLF(added)
	if err != nil {
		return false
	}
	if len(removedRecords) != len(addedRecords) {
		return false
	}
	for index := range removedRecords {
		if !bytes.Equal(stripHorizontal(removedRecords[index]), stripHorizontal(addedRecords[index])) {
			return false
		}
	}
	return true
}

func stripHorizontal(record []byte) []byte {
	out := make([]byte, 0, len(record))
	for _, c := range record {
		switch c {
		case 0x09, 0x0d, 0x0c, 0x0b, 0x20:
		default:
			out = append(out, c)
		}
	}
	return out
}

// checkDrift applies the frozen same-path drift algorithm and fails the target
// check on any stale, ambiguous, or deleted item.
func checkDrift(ctx context.Context, repository *gitauth.Repository, document *wire.Map, baseOID, target string, outcome *Outcome) error {
	targetOID, err := repository.Resolve(ctx, target)
	if err != nil {
		return err
	}
	outcome.TargetRevision = targetOID
	rejected := false
	for _, record := range document.Evidence {
		item, err := driftItem(ctx, repository, record, baseOID, targetOID)
		if err != nil {
			return err
		}
		outcome.Drift = append(outcome.Drift, item)
		if item.Status == DriftStale || item.Status == DriftAmbiguous || item.Status == DriftDeleted {
			rejected = true
		}
	}
	if rejected {
		return cemcode.New(cemcode.EvidenceDrift, "target evidence drift: stale, ambiguous, or deleted items")
	}
	return nil
}

func driftItem(ctx context.Context, repository *gitauth.Repository, record wire.Evidence, baseOID, targetOID string) (DriftItem, error) {
	item := DriftItem{EvidenceID: record.ID}
	entry, exists, err := repository.LookupTreeEntry(ctx, targetOID, record.Path)
	if err != nil {
		return item, err
	}
	if !exists {
		item.Status, item.TargetSpan = ClassifySpan(false, false, nil, nil, record.Span)
		return item, nil
	}
	item.TargetBlobOid = entry.OID
	if entry.OID == record.BlobOid {
		item.Status, item.TargetSpan = ClassifySpan(true, true, nil, nil, record.Span)
		return item, nil
	}
	if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		item.Status, item.TargetSpan = ClassifySpan(false, false, nil, nil, record.Span)
		item.TargetBlobOid = ""
		return item, nil
	}
	baseData, err := repository.BlobBytes(ctx, record.BlobOid)
	if err != nil {
		return item, err
	}
	needle := baseData[record.Span.Start:record.Span.End]
	targetData, err := repository.BlobBytes(ctx, entry.OID)
	if err != nil {
		return item, err
	}
	item.Status, item.TargetSpan = ClassifySpan(true, false, targetData, needle, record.Span)
	return item, nil
}

// countOverlapping counts matches resuming one byte after each match.
func countOverlapping(haystack, needle []byte) (int, int64) {
	count := 0
	first := int64(-1)
	offset := 0
	for offset <= len(haystack)-len(needle) {
		index := bytes.Index(haystack[offset:], needle)
		if index < 0 {
			break
		}
		if count == 0 {
			first = int64(offset + index)
		}
		count++
		offset += index + 1
	}
	return count, first
}
