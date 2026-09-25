package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/sim"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// BeginOptions configure the low-level exact-patch producer entry.
type BeginOptions struct {
	PatchPath string // out-of-band patch input; absolute or root-relative
	Base      string // revision; empty means HEAD
	Output    string // required root-relative map output
}

// Begin builds a cem/0.1 candidate from exact out-of-band patch bytes.
func (s *Session) Begin(ctx context.Context, options BeginOptions) (map[string]any, error) {
	if options.PatchPath == "" {
		return nil, invalidArguments("--patch is required")
	}
	if options.Output == "" || filepath.IsAbs(options.Output) {
		return nil, invalidArguments("--output must be a repository-relative path")
	}
	patchBytes, err := s.readPatchInput(options.PatchPath)
	if err != nil {
		return nil, err
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return nil, err
	}
	baseArg := options.Base
	if baseArg == "" {
		baseArg = "HEAD"
	}
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	base, err := s.repository.Resolve(ctx, baseArg)
	if err != nil {
		return nil, err
	}
	document := buildCandidate(wire.Spec01, base, patchBytes, parsed)
	encoded := encodeMap(document)
	if err := publish.Publish(publish.Output{Root: s.workRoot, Relative: options.Output, Data: encoded}); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-begin",
		"map":          filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.Output)),
		"baseRevision": base, "patchSha256": document.PatchSha256, "hunks": documentHunks(document),
	}, nil
}

func (s *Session) readPatchInput(path string) ([]byte, error) {
	if filepath.IsAbs(path) {
		return publish.ReadBoundedFile(path, patch.MaxPatchBytes, cemcode.PatchUnavailable)
	}
	return s.workRoot.ReadBounded(path, patch.MaxPatchBytes, cemcode.PatchUnavailable)
}

// PrepareOptions configure candidate generation.
type PrepareOptions struct {
	Base    string // required independent producer baseline
	Target  string // required code revision
	MapPath string // must be empty or the frozen default
	Cache   string // optional root-relative convenience-cache selection
	Replace bool
}

// checkCacheAliasing rejects a convenience-cache selection that would alias
// or shadow the frozen map path: publishing the pair must never leave patch
// bytes at the map location. Comparison is case-folded because the worktree
// may sit on a case-insensitive filesystem; publish.PublishPair additionally
// refuses physical target aliasing the string comparison cannot see.
func checkCacheAliasing(cache string) error {
	if cache == "" {
		return nil
	}
	cleaned := path.Clean(cache)
	if strings.EqualFold(cleaned, wire.ExcludedCEMPath) ||
		foldHasPrefix(cleaned, wire.ExcludedCEMPath+"/") ||
		foldHasPrefix(wire.ExcludedCEMPath, cleaned+"/") {
		return invalidArguments("--patch cache must not select the map path %s", wire.ExcludedCEMPath)
	}
	// The map's lock and interrupted-publication backup names are not free
	// either: release would remove a cache at the lock path, and a later
	// prepare would restore a cache at a backup name as the map.
	if strings.EqualFold(cleaned, wire.ExcludedCEMPath+".lock") ||
		foldHasPrefix(cleaned, wire.ExcludedCEMPath+".bak-") {
		return invalidArguments("--patch cache must not select the lock or backup name of the map path %s", wire.ExcludedCEMPath)
	}
	return nil
}

func foldHasPrefix(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

// Prepare derives the canonical patch and writes or resumes the candidate map.
// It ignores target-side sidecar state entirely: candidate generation precedes
// the commit that creates the final revision.
func (s *Session) Prepare(ctx context.Context, options PrepareOptions) (map[string]any, error) {
	if options.Base == "" || options.Target == "" {
		return nil, invalidArguments("prepare requires --base and --target")
	}
	if options.MapPath != "" && options.MapPath != wire.ExcludedCEMPath {
		return nil, invalidArguments("cem prepare requires the fixed map path")
	}
	if options.Cache != "" && filepath.IsAbs(options.Cache) {
		return nil, invalidArguments("--patch must select a repository-relative cache path")
	}
	if err := checkCacheAliasing(options.Cache); err != nil {
		return nil, err
	}
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	defer s.repository.BeginObjectSession()()
	base, err := s.repository.Resolve(ctx, options.Base)
	if err != nil {
		return nil, err
	}
	target, err := s.repository.Resolve(ctx, options.Target)
	if err != nil {
		return nil, err
	}
	if err := s.checkBaseSidecar(ctx, base); err != nil {
		return nil, err
	}
	patchBytes, err := s.repository.CanonicalDiff(ctx, base, target)
	if err != nil {
		return nil, err
	}
	if len(patchBytes) == 0 {
		return nil, cemcode.New(cemcode.GitDiffFailed, "base and target trees are identical after the sidecar exclusion")
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return nil, err
	}
	// The map lock spans resume inspection through publication, so an
	// interrupted pair's backup is restored before the map counts as missing
	// and no concurrent cite or mark lands between read and replace.
	unlock, err := s.workRoot.Lock(wire.ExcludedCEMPath)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := s.workRoot.RecoverPairBackup(wire.ExcludedCEMPath); err != nil {
		return nil, err
	}
	resumed, document, err := s.resumeCandidate(ctx, base, patchBytes, options.Replace)
	if err != nil {
		return nil, err
	}
	if document == nil {
		document = buildCandidate(wire.Spec02, base, patchBytes, parsed)
	}
	cacheRoot, cacheRelative := s.gitRoot, defaultPatchRelative
	if options.Cache != "" {
		cacheRoot, cacheRelative = s.workRoot, options.Cache
	}
	cacheOutput := publish.Output{Root: cacheRoot, Relative: cacheRelative, Data: patchBytes}
	if resumed {
		if err := s.workRoot.RemoveValidatedPairBackup(wire.ExcludedCEMPath); err != nil {
			return nil, err
		}
		// A resumed candidate is never rewritten; only the currently selected
		// convenience cache is refreshed.
		if err := publish.Publish(cacheOutput); err != nil {
			return nil, err
		}
	} else {
		mapOutput := publish.Output{Root: s.workRoot, Relative: wire.ExcludedCEMPath, Data: encodeMap(document)}
		if err := publish.PublishPair(mapOutput, cacheOutput); err != nil {
			return nil, err
		}
	}
	_, work := countsAndWorklist(document)
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-prepare",
		"baseRevision": base, "targetRevision": target,
		"map":     filepath.Join(s.workRoot.Path(), filepath.FromSlash(wire.ExcludedCEMPath)),
		"patch":   filepath.Join(cacheRoot.Path(), filepath.FromSlash(cacheRelative)),
		"resumed": resumed, "worklist": work, "nextHunk": firstOpen(work),
		"nextActions": []any{nextAction(document, base, options.Cache)},
	}, nil
}

// nextAction renders the profile-correct verification action. A resumed 0.1
// action reflects only the current invocation's cache selection; a 0.2 action
// is always canonical with literal HEAD, run after committing the candidate.
// argv[0] is the contract command name `corvint`, not the `corvint` build name
// (CEM-CB-016, decision 0122): frozen cli-parity-v0 stdout pins it.
func nextAction(document *wire.Map, base, cacheSelection string) []any {
	action := []any{"corvint", "cem", "status", "--map", wire.ExcludedCEMPath}
	if wire.Canonical(document.Spec) {
		return append(action, "--expected-base", base, "--target", "HEAD",
			"--max-unknown", "0", "--max-mechanical", "0")
	}
	if cacheSelection != "" {
		action = append(action, "--patch", cacheSelection)
	}
	return action
}

func (s *Session) checkBaseSidecar(ctx context.Context, base string) error {
	entry, exists, err := s.repository.LookupTreeEntry(ctx, base, wire.ExcludedCEMPath)
	if err != nil {
		return err
	}
	if exists && (entry.Type != "blob" || entry.Mode != "100644") {
		return cemcode.New(cemcode.ExcludedPathNotFile, "base-side %s is not a regular 100644 blob", wire.ExcludedCEMPath)
	}
	return nil
}

// resumeCandidate inspects existing bytes at the frozen map path. An unsafe
// existing map — unreadable, unparseable, or outdated — is never overwritten
// without an explicit replace.
func (s *Session) resumeCandidate(ctx context.Context, base string, patchBytes []byte, replace bool) (bool, *wire.Map, error) {
	// Absence is decided from the file system error itself, never from error
	// text: a message embeds the repository path, which may contain any word.
	if _, err := os.Lstat(filepath.Join(s.workRoot.Path(), filepath.FromSlash(wire.ExcludedCEMPath))); errors.Is(err, fs.ErrNotExist) {
		return false, nil, nil
	}
	raw, err := s.workRoot.ReadBounded(wire.ExcludedCEMPath, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil {
		if replace {
			return false, nil, nil
		}
		return false, nil, err
	}
	if replace {
		return false, nil, nil
	}
	document, err := wire.ParseMap(raw)
	if err != nil {
		return false, nil, cemcode.New(cemcode.MapUnavailable,
			"existing map at %s is not a valid CEM document; pass --replace to regenerate: %v", wire.ExcludedCEMPath, err).
			Guided("the existing map is not a valid CEM document; pass --replace to regenerate")
	}
	digest := sha256.Sum256(patchBytes)
	if document.BaseRevision != base || document.PatchSha256 != hex.EncodeToString(digest[:]) {
		return false, nil, cemcode.New(cemcode.MapUnavailable,
			"existing map at %s records a different base or patch; pass --replace to regenerate", wire.ExcludedCEMPath).
			Guided("the existing map records a different base or patch; pass --replace to regenerate")
	}
	if err := verify.Candidate(ctx, s.repository, document, patchBytes); err != nil {
		return false, nil, err
	}
	return true, document, nil
}

// CiteOptions attach one evidence span to one hunk.
type CiteOptions struct {
	MapPath      string
	Hunk         string
	EvidencePath string
	Bytes        string // START:END zero-based byte span
	Lines        string // START:END one-based inclusive line span
	Relation     string
	Output       string // optional root-relative output; empty rewrites in place
}

// Cite compiles the requested span into the normative byte span and records
// the typed evidence reference, preserving the input profile.
func (s *Session) Cite(ctx context.Context, options CiteOptions) (map[string]any, error) {
	document, unlock, err := s.lockedMapInput(options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	defer unlock()
	// The oracle resolves the evidence span before the hunk selector, so an
	// invocation whose evidence is absent reports that rather than the selector.
	if err := wire.ValidatePath(options.EvidencePath); err != nil {
		return nil, err
	}
	if (options.Bytes == "") == (options.Lines == "") {
		return nil, invalidArguments("exactly one of --bytes and --lines is required")
	}
	if !wire.Relations[options.Relation] {
		return nil, invalidArguments("--relation must be a registered relation")
	}
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	entry, exists, err := s.repository.LookupTreeEntry(ctx, document.BaseRevision, options.EvidencePath)
	if err != nil {
		return nil, err
	}
	if !exists || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return nil, cemcode.New(cemcode.MissingEvidence, "evidence path is absent at baseRevision")
	}
	blob, err := s.repository.BlobBytes(ctx, entry.OID)
	if err != nil {
		return nil, err
	}
	span, err := evidenceSpan(blob, options.Bytes, options.Lines)
	if err != nil {
		return nil, err
	}
	if span.End-span.Start > maxCitationSpanBytes {
		return nil, invalidArguments("evidence spans are limited to %d bytes", maxCitationSpanBytes)
	}
	index, err := resolveHunk(document, options.Hunk)
	if err != nil {
		return nil, err
	}
	if err := s.requireCiteSpanStable(ctx, document, options.EvidencePath, entry.OID, blob, span); err != nil {
		return nil, err
	}
	spanDigest := sha256.Sum256(blob[span.Start:span.End])
	record := wire.Evidence{
		BlobOid: entry.OID, Path: options.EvidencePath, Span: span,
		SpanSha256: hex.EncodeToString(spanDigest[:]),
	}
	record.ID = wire.EvidenceIdentity(record.BlobOid, record.Path, record.Span, record.SpanSha256)
	if err := attachEvidence(document, index, record, options.Relation); err != nil {
		return nil, err
	}
	output, err := s.writeMap(document, options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-cite", "map": output,
		"hunkId": document.Hunks[index].ID, "evidenceId": record.ID,
	}, nil
}

func (s *Session) requireCiteSpanStable(ctx context.Context, document *wire.Map, evidencePath, baseBlobOID string, baseBlob []byte, span wire.Span) error {
	changedByDisplayPath := false
	for _, hunk := range document.Hunks {
		changedByDisplayPath = changedByDisplayPath || hunk.Path == evidencePath
	}
	patchBytes, err := s.citePatch(ctx, document)
	if err != nil {
		return err
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return err
	}
	for _, group := range parsed.Groups {
		if group.OldPath == nil || *group.OldPath != evidencePath {
			continue
		}
		patchedBlob, err := sim.ApplyHunks(group, baseBlob)
		if err != nil {
			return err
		}
		classification, _ := verify.ClassifySpan(
			group.NewPath != nil && *group.NewPath == evidencePath,
			bytes.Equal(baseBlob, patchedBlob),
			patchedBlob,
			baseBlob[span.Start:span.End],
			span,
		)
		if classification == verify.DriftStable || classification == verify.DriftRelocated {
			return nil
		}
		if classification == verify.DriftStale || classification == verify.DriftDeleted {
			// Decision 0165: removed base intent is named by its base blob pin
			// for an unknown-plan detail; it never enters the map as evidence.
			return cemcode.New(cemcode.CiteSpanNotStable,
				"evidence span in %q would be %s after applying its CEM hunk group; removed base intent pin removed-intent.%s.%d-%d",
				evidencePath, classification, baseBlobOID, span.Start, span.End)
		}
		return cemcode.New(cemcode.CiteSpanNotStable,
			"evidence span in %q would be %s after applying its CEM hunk group", evidencePath, classification)
	}
	if changedByDisplayPath {
		return cemcode.New(cemcode.PatchDigestMismatch, "CEM patch has no hunk group for changed evidence path %q", evidencePath)
	}
	return nil
}

func (s *Session) citePatch(ctx context.Context, document *wire.Map) ([]byte, error) {
	cached, cacheErr := s.gitRoot.ReadBounded(defaultPatchRelative, patch.MaxPatchBytes, cemcode.PatchUnavailable)
	if cacheErr == nil && patchMatches(document, cached) {
		return cached, nil
	}
	derived, err := s.repository.CanonicalDiff(ctx, document.BaseRevision, "HEAD")
	if err != nil {
		return nil, err
	}
	if patchMatches(document, derived) {
		return derived, nil
	}
	if cacheErr != nil {
		return nil, cacheErr
	}
	return nil, cemcode.New(cemcode.PatchDigestMismatch,
		"neither the cached nor current canonical patch matches the CEM patch digest")
}

func patchMatches(document *wire.Map, patchBytes []byte) bool {
	digest := sha256.Sum256(patchBytes)
	return hex.EncodeToString(digest[:]) == document.PatchSha256
}

func attachEvidence(document *wire.Map, index int, record wire.Evidence, relation string) error {
	present := false
	for _, existing := range document.Evidence {
		if existing.ID == record.ID {
			present = true
		}
	}
	if !present {
		if len(document.Evidence) >= wire.MaxEvidence {
			return invalidArguments("evidence limit of %d records exceeded", wire.MaxEvidence)
		}
		document.Evidence = append(document.Evidence, record)
	}
	hunk := &document.Hunks[index]
	for _, basis := range hunk.Basis {
		if basis.EvidenceID == record.ID && basis.Relation == relation {
			return nil
		}
	}
	if len(hunk.Basis) >= wire.MaxBases {
		return invalidArguments("basis limit of %d references exceeded", wire.MaxBases)
	}
	hunk.Basis = append(hunk.Basis, wire.Basis{EvidenceID: record.ID, Relation: relation})
	hunk.Disposition, hunk.Reason = "supported", "evidence-backed"
	return nil
}

// evidenceSpan compiles a byte or line selection into the normative byte span.
// Line compilation applies the frozen LF-tokenizer record bound.
func evidenceSpan(blob []byte, byteSpan, lineSpan string) (wire.Span, error) {
	if byteSpan != "" {
		start, end, err := parsePair(byteSpan)
		if err != nil {
			return wire.Span{}, err
		}
		if start < 0 {
			return wire.Span{}, cemcode.New(cemcode.InvalidSpan, "evidence span must be non-negative")
		}
		if end <= start {
			return wire.Span{}, cemcode.New(cemcode.InvalidSpan, "evidence span must be non-empty and end-exclusive")
		}
		if end > int64(len(blob)) {
			return wire.Span{}, cemcode.New(cemcode.SpanOutOfRange, "evidence span exceeds its blob")
		}
		return wire.Span{Start: start, End: end}, nil
	}
	startLine, endLine, err := parsePair(lineSpan)
	if err != nil {
		return wire.Span{}, err
	}
	if startLine < 1 || endLine < startLine {
		return wire.Span{}, cemcode.New(cemcode.InvalidLineRange, "line range must be one-based, inclusive, and ordered")
	}
	records, err := patch.SplitLF(blob)
	if err != nil {
		return wire.Span{}, err
	}
	if endLine > int64(len(records)) {
		return wire.Span{}, cemcode.New(cemcode.LineRangeOutOfRange, "evidence line range exceeds its blob")
	}
	start := int64(0)
	for _, record := range records[:startLine-1] {
		start += int64(len(record))
	}
	end := start
	for _, record := range records[startLine-1 : endLine] {
		end += int64(len(record))
	}
	return wire.Span{Start: start, End: end}, nil
}

func parsePair(text string) (int64, int64, error) {
	left, right, found := strings.Cut(text, ":")
	if !found {
		return 0, 0, invalidArguments("span must be START:END")
	}
	start, startErr := strconv.ParseInt(left, 10, 64)
	end, endErr := strconv.ParseInt(right, 10, 64)
	if startErr != nil || endErr != nil {
		return 0, 0, invalidArguments("span must be START:END")
	}
	return start, end, nil
}

// MarkOptions set one hunk's explicit unknown or mechanical disposition.
type MarkOptions struct {
	MapPath     string
	Hunk        string
	Disposition string
	Reason      string
	Output      string
}

// Mark records an explicit unknown or mechanical disposition and prunes
// evidence left uncited, preserving the input profile. The one profile change
// is additive (CEM-SM-006): a structural reason on a canonical map declares
// cem/0.3, the vocabulary the map now uses; a cem/0.1 map cannot carry one.
func (s *Session) Mark(ctx context.Context, options MarkOptions) (map[string]any, error) {
	document, unlock, err := s.lockedMapInput(options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	defer unlock()
	index, err := resolveHunk(document, options.Hunk)
	if err != nil {
		return nil, err
	}
	valid := (options.Disposition == "unknown" &&
		(options.Reason == "no-evidence" || options.Reason == "insufficient-evidence" || options.Reason == "conflicting-evidence")) ||
		(options.Disposition == "mechanical" && wire.MechanicalReason(wire.Spec03, options.Reason))
	if !valid {
		return nil, invalidArguments("mark accepts unknown or mechanical dispositions with their registered reasons")
	}
	if wire.StructuralReasons[options.Reason] {
		if !wire.Canonical(document.Spec) {
			return nil, invalidArguments("structural mechanical reasons require a canonical (cem/0.2 or cem/0.3) map")
		}
		if err := checkSpec03Output(document, options.MapPath, options.Output); err != nil {
			return nil, err
		}
		document.Spec = wire.Spec03
	}
	hunk := &document.Hunks[index]
	hunk.Disposition, hunk.Reason, hunk.Basis = options.Disposition, options.Reason, nil
	pruneEvidence(document)
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	output, err := s.writeMap(document, options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-mark", "map": output,
		"hunkId": hunk.ID, "disposition": options.Disposition, "reason": options.Reason,
	}, nil
}

func pruneEvidence(document *wire.Map) {
	cited := map[string]bool{}
	for _, hunk := range document.Hunks {
		for _, basis := range hunk.Basis {
			cited[basis.EvidenceID] = true
		}
	}
	retained := document.Evidence[:0]
	for _, record := range document.Evidence {
		if cited[record.ID] {
			retained = append(retained, record)
		}
	}
	document.Evidence = retained
}

var lockMap = (*publish.Root).Lock

// lockedMapInput reads the input once for unchanged map-validation precedence,
// then locks the effective output and re-reads the input under it. The caller
// must release the lock.
func (s *Session) lockedMapInput(inputRelative, outputRelative string) (*wire.Map, func(), error) {
	if _, _, err := s.readMapInput(inputRelative); err != nil {
		return nil, nil, err
	}
	effectiveOutput, err := mapOutputRelative(inputRelative, outputRelative)
	if err != nil {
		return nil, nil, err
	}
	unlock, err := lockMap(s.workRoot, effectiveOutput)
	if err != nil {
		return nil, nil, err
	}
	_, document, err := s.readMapInput(inputRelative)
	if err != nil {
		unlock()
		return nil, nil, err
	}
	return document, unlock, nil
}

func (s *Session) writeMap(document *wire.Map, inputRelative, outputRelative string) (string, error) {
	relative, err := mapOutputRelative(inputRelative, outputRelative)
	if err != nil {
		return "", err
	}
	encoded := encodeMap(document)
	if err := publish.Publish(publish.Output{Root: s.workRoot, Relative: relative, Data: encoded}); err != nil {
		return "", err
	}
	return filepath.Join(s.workRoot.Path(), filepath.FromSlash(relative)), nil
}

// checkSpec03Output refuses a cem/0.3 write that would replace a cem/0.2 input
// map in place or land on the Core sidecar path, which frontier and OCM read as
// cem/0.2 only (CEM-SM-006, V1-0335).
func checkSpec03Output(document *wire.Map, inputRelative, outputRelative string) error {
	output, err := mapOutputRelative(inputRelative, outputRelative)
	if err != nil {
		return err
	}
	output = path.Clean(output)
	if strings.EqualFold(output, wire.ExcludedCEMPath) {
		return invalidArguments("a cem/0.3 map must not be written to %s, which frontier and OCM read as cem/0.2; pass --output PATH", wire.ExcludedCEMPath)
	}
	if document.Spec == wire.Spec02 && strings.EqualFold(output, path.Clean(inputRelative)) {
		return invalidArguments("upgrading a cem/0.2 map to cem/0.3 needs --output naming a different path")
	}
	return nil
}

func mapOutputRelative(inputRelative, outputRelative string) (string, error) {
	if outputRelative == "" {
		return inputRelative, nil
	}
	if filepath.IsAbs(outputRelative) {
		return "", invalidArguments("--output must be a repository-relative path")
	}
	if err := checkMapSidecarAliasing(inputRelative, outputRelative); err != nil {
		return "", err
	}
	return outputRelative, nil
}

func checkMapSidecarAliasing(mapRelative, outputRelative string) error {
	mapRelative = path.Clean(mapRelative)
	outputRelative = path.Clean(outputRelative)
	if strings.EqualFold(outputRelative, mapRelative+".lock") ||
		foldHasPrefix(outputRelative, mapRelative+".bak-") {
		return invalidArguments("--output must not select the lock or backup name of input map %s", mapRelative)
	}
	return nil
}
