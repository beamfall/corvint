package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/sim"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// BeginOptions configure the low-level exact-patch producer entry, which
// writes a cem/1.0 producer map (CEM-V1-142, CEM-V1-216).
type BeginOptions struct {
	PatchPath string // out-of-band patch input; absolute or root-relative
	Base      string // revision; empty means HEAD
	Output    string // required root-relative map output
	// Spec must be empty or cem/1.0, the only producer profile (CEM-V1-216).
	Spec string
}

// NonRegularUnknown is the only value of --non-regular (CAX-V0-020).
const NonRegularUnknown = "unknown"

func nonRegularOption(value string) (bool, error) {
	switch value {
	case "":
		return false, nil
	case NonRegularUnknown:
		return true, nil
	}
	return false, invalidArguments("--non-regular must be %s", NonRegularUnknown)
}

// NonRegularRows renders the excluded entries of CAX-V0-020 as result rows,
// each an explicit unknown that was never read as content or followed.
func NonRegularRows(entries []gitauth.NonRegularEntry) []any {
	rows := make([]any, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, map[string]any{
			"path": entry.Path, "baseMode": entry.BaseMode, "targetMode": entry.TargetMode,
			"disposition": "unknown",
		})
	}
	return rows
}

// Begin builds a cem/1.0 producer map from exact out-of-band patch bytes.
func (s *Session) Begin(ctx context.Context, options BeginOptions) (map[string]any, error) {
	if err := producerSpec("begin", options.Spec); err != nil {
		return nil, err
	}
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
	document := buildCandidate(base, patchBytes, parsed)
	encoded, err := encodeStableCandidate(document)
	if err != nil {
		return nil, err
	}
	// CEM-V1-031: the non-mutating preflight refuses a stable destination before
	// the lock creates anything; the output lock then spans the strict probe and
	// publication, so no Core writer lands between them. Core replaces only a
	// stable producer map (CEM-V1-143).
	if err := s.preflightDestination("begin", options.Output); err != nil {
		return nil, err
	}
	unlock, err := lockMap(s.workRoot, options.Output)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := s.refuseStableEntry("begin", options.Output, "replace"); err != nil {
		return nil, err
	}
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
	// Spec must be empty or cem/1.0, the only producer profile (CEM-V1-216).
	Spec string
	// NonRegular is empty or "unknown". "unknown" selects the additive
	// cem-non-regular/1 profile (CAX-V0-020): changed paths that are not
	// regular files are left out of the mapped patch and reported as
	// explicit per-path unknowns instead of refusing the whole range.
	NonRegular string
	// PathPolicy is empty or "base". "base" selects the additive
	// cem-path-policy/1 profile (CAX-V0-060): unknown hunks a base-tree
	// witness path policy declares are classified beside the result. The map
	// bytes are unchanged.
	PathPolicy string
}

// producerSpec admits a producer profile selection: empty or cem/1.0. A
// retired cem/0.x profile is refused with unsupported-spec and the migration
// fix, any other value as invalid arguments (CEM-V1-140, CEM-V1-216).
func producerSpec(tool, spec string) error {
	switch {
	case spec == "" || spec == wire.StableSpec:
		return nil
	case wire.RetiredSpec(spec):
		return cemcode.New(cemcode.UnsupportedSpec, "%s is retired; %s is the only stable wire; fix: %s", spec, wire.StableSpec, wire.RetiredSpecFix)
	}
	return invalidArguments("cem %s --spec must be %s", tool, wire.StableSpec)
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
// maxNamedModePaths bounds how many non-regular paths a tree-mode refusal
// names; the count covers the rest.
const maxNamedModePaths = 8

// treeModeRefusal names, for an unsupported-tree-mode patch, every changed
// path whose base or target entry is not a regular file, with both modes. The
// cem/1.0 patch grammar admits only regular files (protocol/cem-1.0
// ALGORITHMS.md), so the range still refuses whole, but the caller learns
// which links or gitlinks to move out of it. Only tree entries are read: a
// link is never followed (CEM-V1-655). Any other error passes through.
func (s *Session) treeModeRefusal(ctx context.Context, base, target string, err error) error {
	if cemcode.CodeOf(err) != cemcode.UnsupportedTreeMode {
		return err
	}
	named, listErr := s.repository.UnsupportedModeChanges(ctx, base, target)
	if listErr != nil || len(named) == 0 {
		return err
	}
	shown := named[:min(len(named), maxNamedModePaths)]
	message := fmt.Sprintf("%s; %d changed paths are not regular files and cem/1.0 maps only regular files: %s",
		cemcode.MessageOf(err), len(named), strings.Join(shown, ", "))
	if len(named) > len(shown) {
		message += fmt.Sprintf("; and %d more", len(named)-len(shown))
	}
	return cemcode.New(cemcode.UnsupportedTreeMode, "%s", message).
		Guided("prepare a range that does not add, change or remove these entries; a symbolic link is never followed or mapped")
}

// the commit that creates the final revision.
func (s *Session) Prepare(ctx context.Context, options PrepareOptions) (map[string]any, error) {
	if options.Base == "" || options.Target == "" {
		return nil, invalidArguments("prepare requires --base and --target")
	}
	if err := producerSpec("prepare", options.Spec); err != nil {
		return nil, err
	}
	profile, err := nonRegularOption(options.NonRegular)
	if err != nil {
		return nil, err
	}
	pathPolicy, err := pathPolicyOption(options.PathPolicy)
	if err != nil {
		return nil, err
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
	var patchBytes []byte
	var nonRegular []gitauth.NonRegularEntry
	if profile {
		patchBytes, _, nonRegular, err = s.repository.CanonicalRegularDiffWithCreateDestinations(ctx, base, target)
	} else {
		patchBytes, err = s.repository.CanonicalDiff(ctx, base, target)
	}
	if err != nil {
		return nil, err
	}
	if len(patchBytes) == 0 && len(nonRegular) != 0 {
		return nil, cemcode.New(cemcode.UnsupportedTreeMode, "every changed path is a non-regular entry; cem-non-regular/1 leaves no regular-file hunk to map")
	}
	if len(patchBytes) == 0 {
		return nil, cemcode.New(cemcode.GitDiffFailed, "base and target trees are identical after the sidecar exclusion")
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return nil, s.treeModeRefusal(ctx, base, target, err)
	}
	// The map lock spans resume inspection through publication, so an
	// interrupted pair's backup is restored before the map counts as missing
	// and no concurrent cite or mark lands between read and replace. A stable
	// sidecar or backup is refused first, before the lock or recovery touches
	// either, and again under the lock before recovery or resume cleanup
	// (CEM-V1-030). Prepare writes cem/1.0, so it downgrades no sidecar, but
	// recovery or resume cleanup still restores or removes a backup: only a
	// non-stable or stable producer backup is admitted (CEM-V1-141).
	if err := s.preflightStableSidecar(options.Replace); err != nil {
		return nil, err
	}
	unlock, err := s.workRoot.Lock(wire.ExcludedCEMPath)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := s.preflightStableSidecar(options.Replace); err != nil {
		return nil, err
	}
	if err := s.workRoot.RecoverPairBackup(wire.ExcludedCEMPath); err != nil {
		return nil, err
	}
	if options.Replace {
		if err := s.refuseStableReplace(); err != nil {
			return nil, err
		}
	}
	resumed, document, err := s.resumeStableCandidate(ctx, base, patchBytes, options.Replace)
	if err != nil {
		return nil, err
	}
	if document == nil {
		document = buildCandidate(base, patchBytes, parsed)
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
		encoded, err := encodeStableCandidate(document)
		if err != nil {
			return nil, err
		}
		mapOutput := publish.Output{Root: s.workRoot, Relative: wire.ExcludedCEMPath, Data: encoded}
		if err := publish.PublishPair(mapOutput, cacheOutput); err != nil {
			return nil, err
		}
	}
	_, work := countsAndWorklist(document)
	result := map[string]any{
		"ok": true, "mutates": true, "tool": "cem-prepare",
		"baseRevision": base, "targetRevision": target,
		"map":     filepath.Join(s.workRoot.Path(), filepath.FromSlash(wire.ExcludedCEMPath)),
		"patch":   filepath.Join(cacheRoot.Path(), filepath.FromSlash(cacheRelative)),
		"resumed": resumed, "worklist": work, "nextHunk": firstOpen(work),
		"nextActions": []any{stableNextAction(base, target, s.workRoot.Path())},
	}
	if profile {
		result["profile"] = gitauth.NonRegularProfile
		result["nonRegular"] = NonRegularRows(nonRegular)
	}
	if pathPolicy {
		// The policy is read from the resolved base only, after the map is
		// written, so its bytes never depend on it (CAX-V0-060, CAX-V0-061).
		result["pathPolicyProfile"] = PathPolicyProfile
		result["pathPolicy"], result["pathPolicyHunks"], _ = s.pathPolicyMembers(ctx, base, target, document)
		next := append(stableNextAction(base, target, s.workRoot.Path()), "--path-policy", PathPolicyBase)
		if profile {
			next = append(next, "--non-regular", NonRegularUnknown)
		}
		result["nextActions"] = []any{next}
	}
	return result, nil
}

// stableNextAction is the stable read of a prepared cem/1.0 map: the stable
// verifier needs full-OID revisions and an absolute artifact root, which a
// producer map with no artifacts never opens (CEM-V1-141, CEM-V1-146).
func stableNextAction(base, target, artifactRoot string) []any {
	return []any{"corvint", "cem", "status", "--map", wire.ExcludedCEMPath,
		"--expected-base", base, "--target", target, "--artifacts", artifactRoot,
		"--max-unknown", "0", "--max-mechanical", "0"}
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

// resumeStableCandidate inspects existing bytes at the frozen map path. An
// unsafe existing map (unreadable, unparseable, or outdated) is never
// overwritten without an explicit replace. Writing cem/1.0 downgrades nothing,
// so --replace regenerates any existing bytes; a resumed map must be a stable
// map for the same base and patch, and a retired cem/0.x map is refused with
// the guided replace recovery (CEM-V1-141, CEM-V1-216).
func (s *Session) resumeStableCandidate(ctx context.Context, base string, patchBytes []byte, replace bool) (bool, *wire.Map, error) {
	if _, err := os.Lstat(filepath.Join(s.workRoot.Path(), filepath.FromSlash(wire.ExcludedCEMPath))); errors.Is(err, fs.ErrNotExist) {
		return false, nil, nil
	}
	raw, err := s.workRoot.ReadBounded(wire.ExcludedCEMPath, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil && !replace {
		return false, nil, err
	}
	if replace {
		return false, nil, nil
	}
	stableDocument, err := wire.ParseStable(raw)
	if err != nil && cemcode.CodeOf(err) == cemcode.UnsupportedSpec && strings.Contains(cemcode.MessageOf(err), wire.RetiredSpecFix) {
		// A retired cem/0.x map records a different spec; any other bytes,
		// another unknown spec included, keep the invalid-document refusal.
		return false, nil, cemcode.New(cemcode.MapUnavailable,
			"existing map at %s records a different spec than %s; pass --replace to regenerate", wire.ExcludedCEMPath, wire.StableSpec).
			Guided("the existing map records a different spec; pass --replace to regenerate")
	}
	if err != nil {
		return false, nil, cemcode.New(cemcode.MapUnavailable,
			"existing map at %s is not a valid CEM document; pass --replace to regenerate: %v", wire.ExcludedCEMPath, err).
			Guided("the existing map is not a valid CEM document; pass --replace to regenerate")
	}
	document := stableMechanics(stableDocument)
	digest := sha256.Sum256(patchBytes)
	if document.BaseRevision != base || document.PatchSha256 != hex.EncodeToString(digest[:]) {
		return false, nil, cemcode.New(cemcode.MapUnavailable,
			"existing map at %s records a different base or patch; pass --replace to regenerate", wire.ExcludedCEMPath).
			Guided("the existing map records a different base or patch; pass --replace to regenerate")
	}
	// A resumed map re-verifies every hunk and evidence record, so its Git
	// operation bound scales with the map (DCW-V0-373).
	s.repository.ScaleBudgetForRecords(len(document.Hunks) + len(document.Evidence))
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
	// Replace drops the hunk's earlier citations before attaching this one,
	// only while ExpectedHunkDigest is the hunk's current wire.HunkDigest
	// (CEM-V1-694, CEM-V1-695).
	Replace            bool
	ExpectedHunkDigest string
}

// requireHunkDigest refuses a guarded replacement whose expected-old digest
// is not the hunk's current one, before anything is written (CEM-V1-694).
func requireHunkDigest(document *wire.Map, index int, expected string) error {
	if current := wire.DocumentHunkDigest(document, index); expected != current {
		return cemcode.New(cemcode.HunkDigestMismatch,
			"hunk %s has changed since its expected digest was read: expected %s, current %s; nothing was written, so reread the map and retry with the current digest",
			document.Hunks[index].ID, expected, current)
	}
	return nil
}

// Cite compiles the requested span into the normative byte span and records
// the typed evidence reference, preserving the input profile.
func (s *Session) Cite(ctx context.Context, options CiteOptions) (map[string]any, error) {
	document, unlock, err := s.lockedMapInput("cite", options.MapPath, options.Output)
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
	if span.End-span.Start > wire.MaxCitationSpanBytes {
		return nil, invalidArguments("evidence spans are limited to %d bytes", wire.MaxCitationSpanBytes)
	}
	index, err := resolveHunk(document, options.Hunk)
	if err != nil {
		return nil, err
	}
	if err := refuseSelfModifiedAuthority(document, options.EvidencePath); err != nil {
		return nil, err
	}
	if err := s.requireCiteSpanStable(ctx, document, options.EvidencePath, entry.OID, blob, span); err != nil {
		return nil, err
	}
	if options.Replace {
		if err := requireHunkDigest(document, index, options.ExpectedHunkDigest); err != nil {
			return nil, err
		}
		// Every check that can refuse this row has passed; only the publish
		// below can still fail, and it leaves the input map intact.
		document.Hunks[index].Basis = nil
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
	if options.Replace {
		pruneEvidence(document)
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
		// CEM-V1-322: every refusal states the rule and names the nearest
		// citable base lines, so the caller can retry without guessing.
		guidance := citeSpanGuidance(group, baseBlob, patchedBlob, span)
		if classification == verify.DriftStale || classification == verify.DriftDeleted {
			// Decision 0165: removed base intent is named by its base blob pin
			// for an unknown-plan detail; it never enters the map as evidence.
			return cemcode.New(cemcode.CiteSpanNotStable,
				"evidence span in %q would be %s after applying its CEM hunk group; removed base intent pin removed-intent.%s.%d-%d; %s",
				evidencePath, classification, baseBlobOID, span.Start, span.End, guidance)
		}
		// V1-1418, V1-1349: stability matches the span's text, not git's line
		// alignment, so name the alignment used and, for lines no hunk touches,
		// why an unchanged span is still refused (DCW-V0-063).
		needle := baseBlob[span.Start:span.End]
		occurrences := verify.SpanOccurrences(patchedBlob, needle)
		// V1-2541: name where each copy of the text lands in the changed file
		// and the narrowest wider base span whose text occurs once, which keeps
		// the cited lines (DCW-V0-334).
		duplicates := occurrenceLines(patchedBlob, needle, maxNamedOccurrences)
		if widened := uniqueWidening(group, baseBlob, patchedBlob, span); widened != "" {
			guidance = "recovery: cite the wider unique span --lines " + widened + ", which keeps these lines; " + guidance
		}
		if start, end := spanLines(baseBlob, span); spanUneditedByHunks(group, start, end) {
			return cemcode.New(cemcode.CiteSpanNotStable,
				"evidence span in %q would be %s after applying its CEM hunk group; base lines %d:%d are edited by no hunk (at most hunk context), so git's line alignment leaves them unchanged, but the stability check matches the span text, which occurs %d times in the changed file (at changed-file lines %s); %s",
				evidencePath, classification, start, end, occurrences, duplicates, guidance)
		}
		return cemcode.New(cemcode.CiteSpanNotStable,
			"evidence span in %q would be %s after applying its CEM hunk group; the stability check matches the span text, which occurs %d times in the changed file (at changed-file lines %s); %s",
			evidencePath, classification, occurrences, duplicates, guidance)
	}
	if changedByDisplayPath {
		return cemcode.New(cemcode.PatchDigestMismatch, "CEM patch has no hunk group for changed evidence path %q", evidencePath)
	}
	return nil
}

// citeSpanRule is the stability rule every cite-span-not-stable refusal states.
const citeSpanRule = "rule: a cited BASE span must survive the change, so its exact text must occur exactly once in the changed file (stable or relocated); cite base lines no hunk removes"

// citeSpanGuidance returns the rule plus the nearest stable base line spans
// before and after span (CEM-V1-322). A candidate is the nearest maximal run
// of base lines no hunk removes or splits, clipped toward span to the citation
// byte bound, and is named only when its text classifies stable or relocated
// against the patched blob, so a named span is citable as stated. At most two
// candidates are classified, so the refusal stays linear in the file.
func citeSpanGuidance(group *patch.Group, baseBlob, patchedBlob []byte, span wire.Span) string {
	targetExists := group.NewPath != nil && *group.NewPath == *group.OldPath
	if !targetExists {
		return citeSpanRule + "; the change deletes or renames this file, so no base line of it is citable"
	}
	starts := lineStarts(baseBlob)
	lines := int64(len(starts))
	edited, splitBefore := hunkLineEdits(group, lines)
	first, last := spanLines(baseBlob, span)
	unchanged := bytes.Equal(baseBlob, patchedBlob)
	bytesOf := func(start, end int64) wire.Span {
		return lineSpan(baseBlob, starts, start, end)
	}
	var found []string
	name := func(start, end int64) {
		candidate := bytesOf(start, end)
		if candidate.End <= candidate.Start || candidate.End-candidate.Start > wire.MaxCitationSpanBytes {
			return
		}
		status, _ := verify.ClassifySpan(true, unchanged, patchedBlob, baseBlob[candidate.Start:candidate.End], candidate)
		if status == verify.DriftStable || status == verify.DriftRelocated {
			found = append(found, strconv.FormatInt(start, 10)+":"+strconv.FormatInt(end, 10))
		}
	}
	// Before: the run ending at the nearest unedited line above the span,
	// clipped from its far (upper) end to the byte bound.
	end := first - 1
	for end >= 1 && edited[end] {
		end--
	}
	if end >= 1 {
		start := end
		for start > 1 && !edited[start-1] && !splitBefore[start] {
			start--
		}
		for start < end && bytesOf(start, end).End-bytesOf(start, end).Start > wire.MaxCitationSpanBytes {
			start++
		}
		name(start, end)
	}
	// After: the run starting at the nearest unedited line below the span,
	// clipped from its far (lower) end.
	start := last + 1
	for start <= lines && edited[start] {
		start++
	}
	if start <= lines {
		end := start
		for end < lines && !edited[end+1] && !splitBefore[end+1] {
			end++
		}
		for end > start && bytesOf(start, end).End-bytesOf(start, end).Start > wire.MaxCitationSpanBytes {
			end--
		}
		name(start, end)
	}
	if len(found) == 0 {
		return citeSpanRule + "; no nearby unedited base line span is stable"
	}
	return citeSpanRule + "; nearest stable base spans: --lines " + strings.Join(found, ", --lines ")
}

// hunkLineEdits marks, by one-based base line, each line a hunk of group
// removes and each line an insertion lands before (CEM-V1-322).
func hunkLineEdits(group *patch.Group, lines int64) (edited, splitBefore []bool) {
	edited = make([]bool, lines+2)
	splitBefore = make([]bool, lines+2)
	for _, hunk := range group.Hunks {
		line := hunk.OldRange.Start
		if hunk.OldRange.Count == 0 {
			line++
		}
		for _, body := range hunk.Body {
			switch body.Prefix {
			case ' ':
				line++
			case '-':
				if line >= 1 && line <= lines {
					edited[line] = true
				}
				line++
			case '+':
				if line >= 1 && line <= lines+1 {
					splitBefore[line] = true
				}
			}
		}
	}
	return edited, splitBefore
}

// lineSpan returns the bytes of one-based inclusive base lines start..end.
func lineSpan(blob []byte, starts []int64, start, end int64) wire.Span {
	span := wire.Span{Start: starts[start-1], End: int64(len(blob))}
	if end < int64(len(starts)) {
		span.End = starts[end]
	}
	return span
}

// maxNamedOccurrences bounds how many changed-file copies a refusal names.
const maxNamedOccurrences = 8

// maxWidenLines bounds how many lines uniqueWidening adds on each side, so a
// refusal stays linear in the file for each candidate it classifies.
const maxWidenLines = 16

// occurrenceLines names, as one-based inclusive changed-file line ranges, the
// first limit overlapping occurrences of needle in blob (V1-2541).
func occurrenceLines(blob, needle []byte, limit int) string {
	var named []string
	total := 0
	for offset := 0; len(needle) > 0 && offset <= len(blob)-len(needle); {
		next := bytes.Index(blob[offset:], needle)
		if next < 0 {
			break
		}
		at := offset + next
		total++
		if len(named) < limit {
			start, end := spanLines(blob, wire.Span{Start: int64(at), End: int64(at + len(needle))})
			named = append(named, strconv.FormatInt(start, 10)+":"+strconv.FormatInt(end, 10))
		}
		offset = at + 1
	}
	if total > len(named) {
		named = append(named, "+"+strconv.Itoa(total-len(named))+" more")
	}
	return strings.Join(named, ", ")
}

// uniqueWidening returns the narrowest base line span that contains span,
// adds at most maxWidenLines lines on each side, spans only lines no hunk
// removes or splits, and whose text classifies stable or relocated against
// the patched blob, so the named span cites as stated (DCW-V0-334). Every
// split of each total width is tried, so an asymmetric span qualifies; within
// a width, more lines above are tried before more below. It returns "" when
// none qualifies.
func uniqueWidening(group *patch.Group, baseBlob, patchedBlob []byte, span wire.Span) string {
	if group.NewPath == nil || *group.NewPath != *group.OldPath {
		return ""
	}
	starts := lineStarts(baseBlob)
	lines := int64(len(starts))
	edited, splitBefore := hunkLineEdits(group, lines)
	first, last := spanLines(baseBlob, span)
	unedited := func(start, end int64) bool {
		for line := start; line <= end; line++ {
			if edited[line] || (line > start && splitBefore[line]) {
				return false
			}
		}
		return true
	}
	if !unedited(first, last) {
		return ""
	}
	unchanged := bytes.Equal(baseBlob, patchedBlob)
	for width := int64(1); width <= 2*maxWidenLines; width++ {
		for above := min(width, maxWidenLines); above >= max(0, width-maxWidenLines); above-- {
			start, end := first-above, last+width-above
			if start < 1 || end > lines || !unedited(start, end) {
				continue
			}
			bytesSpan := lineSpan(baseBlob, starts, start, end)
			if bytesSpan.End-bytesSpan.Start > wire.MaxCitationSpanBytes {
				continue
			}
			status, _ := verify.ClassifySpan(true, unchanged, patchedBlob, baseBlob[bytesSpan.Start:bytesSpan.End], bytesSpan)
			if status == verify.DriftStable || status == verify.DriftRelocated {
				return strconv.FormatInt(start, 10) + ":" + strconv.FormatInt(end, 10)
			}
		}
	}
	return ""
}

// lineStarts returns the byte offset of each line of blob; a final line
// without a newline is a line.
func lineStarts(blob []byte) []int64 {
	var starts []int64
	for offset := 0; offset < len(blob); {
		starts = append(starts, int64(offset))
		next := bytes.IndexByte(blob[offset:], '\n')
		if next < 0 {
			break
		}
		offset += next + 1
	}
	return starts
}

// spanLines returns the one-based inclusive base lines a non-empty span covers.
func spanLines(blob []byte, span wire.Span) (int64, int64) {
	start := int64(bytes.Count(blob[:span.Start], []byte("\n"))) + 1
	end := int64(bytes.Count(blob[:span.End-1], []byte("\n"))) + 1
	return start, end
}

// spanUneditedByHunks reports base lines start..end that no hunk line of the
// group removes or splits with an insertion, as git's own alignment places
// them; context lines a hunk carries leave a line unchanged.
func spanUneditedByHunks(group *patch.Group, start, end int64) bool {
	for _, hunk := range group.Hunks {
		line := hunk.OldRange.Start
		if hunk.OldRange.Count == 0 {
			line++
		}
		for _, body := range hunk.Body {
			switch body.Prefix {
			case ' ':
				line++
			case '-':
				if start <= line && line <= end {
					return false
				}
				line++
			case '+':
				// An insertion before line `line` inside the span splits it.
				if start < line && line <= end {
					return false
				}
			}
		}
	}
	return true
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
	// A map prepared under cem-non-regular/1 pins the regular-only patch
	// (CAX-V0-020); the digest still decides, so this admits nothing else.
	if regular, _, excluded, err := s.repository.CanonicalRegularDiffWithCreateDestinations(ctx, document.BaseRevision, "HEAD"); err == nil && len(excluded) != 0 && patchMatches(document, regular) {
		return regular, nil
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
	// ExpectedHunkDigest, when set, refuses the mark unless it is the hunk's
	// current wire.HunkDigest (CEM-V1-695).
	ExpectedHunkDigest string
}

// Mark records an explicit unknown or mechanical disposition and prunes
// evidence left uncited. A cem/1.0 producer map admits the structural
// mechanical vocabulary and stays cem/1.0 (CEM-V1-144).
func (s *Session) Mark(ctx context.Context, options MarkOptions) (map[string]any, error) {
	document, unlock, err := s.lockedMapInput("mark", options.MapPath, options.Output)
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
		(options.Disposition == "mechanical" && wire.MechanicalReason(options.Reason))
	if !valid {
		return nil, invalidArguments("mark accepts unknown or mechanical dispositions with their registered reasons")
	}
	if options.ExpectedHunkDigest != "" {
		if err := requireHunkDigest(document, index, options.ExpectedHunkDigest); err != nil {
			return nil, err
		}
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

// lockedMapInput locks in-place updates before their first read, so a valid
// concurrent publication cannot trip the safe reader's identity check. Separate
// outputs retain input-validation precedence. The caller must release the lock.
// tool names the Core mutator in its stable refusals (CEM-V1-031).
//
// An absolute input is an out-of-tree map read under the CEM-V1-315 and
// CEM-V1-316 rules; the mutator then publishes only to the repository-relative
// --output the caller names, because Core writes a map only under the
// repository's lock and never beside an external one (CEM-V1-647).
func (s *Session) lockedMapInput(tool, inputRelative, outputRelative string) (*wire.Map, func(), error) {
	if filepath.IsAbs(inputRelative) && outputRelative == "" {
		return nil, nil, invalidArguments("cem %s reads an absolute --map outside the repository only with a repository-relative --output: Core writes a map only under the repository's lock, never in place outside it", tool)
	}
	inPlace := outputRelative == "" || strings.EqualFold(outputRelative, inputRelative) // case-aliased names alias on case-insensitive volumes
	if !inPlace || inputRelative == "" || filepath.IsAbs(inputRelative) {
		if _, err := s.readMutableMap(tool, inputRelative); err != nil {
			return nil, nil, err
		}
	}
	effectiveOutput, err := mapOutputRelative(inputRelative, outputRelative)
	if err != nil {
		return nil, nil, err
	}
	// The non-mutating preflight refuses a stable input or destination before
	// the lock creates its sibling file; the locked reads below re-check both.
	// Only a stable producer map is mutated or replaced (CEM-V1-031, CEM-V1-143).
	for _, relative := range []string{inputRelative, effectiveOutput} {
		if err := s.preflightDestination(tool, relative); err != nil {
			return nil, nil, err
		}
	}
	unlock, err := lockMap(s.workRoot, effectiveOutput)
	if err != nil {
		// Preserve read refusals (including unsafe ancestors) over publication
		// errors without admitting an unlocked read on the successful path.
		if _, readErr := s.readMutableMap(tool, inputRelative); readErr != nil {
			return nil, nil, readErr
		}
		return nil, nil, err
	}
	document, err := s.readMutableMap(tool, inputRelative)
	if err == nil && effectiveOutput != inputRelative {
		// The output lock is held, so no Core writer changes it before
		// publication; a case variant is checked too, since it may not alias.
		err = s.refuseStableEntry(tool, effectiveOutput, "replace")
	}
	if err != nil {
		unlock()
		return nil, nil, err
	}
	return document, unlock, nil
}

// readMutableMap reads a mutator's input map. A cem/1.0 producer map is read
// through the stable parser and returned as its mechanics view, which writeMap
// re-encodes as cem/1.0; any other cem/1.0 map is refused as one Core does not
// mutate; every other input, a retired cem/0.x map included, is refused by the
// stable parser (CEM-V1-031, CEM-V1-143, CEM-V1-216).
func (s *Session) readMutableMap(tool, relative string) (*wire.Map, error) {
	var raw []byte
	var err error
	if filepath.IsAbs(relative) {
		raw, _, err = s.readExternalMap(relative)
	} else {
		raw, err = s.readMapBytes(relative)
	}
	if err != nil {
		return nil, err
	}
	stableDocument, err := wire.ParseStable(raw)
	if err != nil {
		return nil, err
	}
	if !stableProducerMap(stableDocument) {
		return nil, stableMutationRefusal(tool, relative)
	}
	document := stableMechanics(stableDocument)
	if s.stableHolders == nil {
		s.stableHolders = map[*wire.Map]*wire.StableMap{}
	}
	s.stableHolders[document] = stableDocument
	return document, nil
}

// refuseStableEntry is the strict probe shared by destinations and
// interrupted-publication backups; action names what the tool would otherwise
// do to the entry. Core may replace a stable producer map, never a stable map
// carrying criteria, receipts, links, artifacts or provenance (CEM-V1-143).
func (s *Session) refuseStableEntry(tool, relative, action string) error {
	info, err := os.Lstat(filepath.Join(s.workRoot.Path(), filepath.FromSlash(relative)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err == nil && (!info.Mode().IsRegular() || info.Size() > wire.MaxMapBytes) {
		return nil
	}
	var raw []byte
	if err == nil {
		raw, err = s.workRoot.ReadBounded(relative, wire.MaxMapBytes, cemcode.MapUnavailable)
	}
	if err != nil {
		return cemcode.New(cemcode.MapUnavailable,
			"existing %s cannot be read to resolve its spec, so Core cem %s does not %s it: it may be a %s map; make it readable or remove it",
			relative, tool, action, wire.StableSpec)
	}
	if wire.DeclaresStable(raw) {
		if document, parseErr := wire.ParseStable(raw); parseErr == nil && stableProducerMap(document) {
			return nil
		}
		return stableMutationRefusal(tool, relative)
	}
	return nil
}

// refuseStableReplace guards a cem/1.0 prepare --replace under the sidecar
// lock: it regenerates a non-stable map, a stable producer map and bytes that declare
// cem/1.0 but do not parse, and refuses a valid stable map that carries
// criteria, receipts, links, artifacts or provenance, or a regular sidecar it
// cannot read, leaving it unchanged (CEM-V1-141).
func (s *Session) refuseStableReplace() error {
	err := s.refuseStableEntry("prepare", wire.ExcludedCEMPath, "replace")
	if err == nil || cemcode.CodeOf(err) == cemcode.MapUnavailable {
		return err
	}
	raw, readErr := s.workRoot.ReadBounded(wire.ExcludedCEMPath, wire.MaxMapBytes, cemcode.MapUnavailable)
	if readErr == nil {
		if _, parseErr := wire.ParseStable(raw); parseErr != nil {
			return nil
		}
	}
	return err
}

// preflightDestination is the non-mutating preflight of one input or output
// path: readable stable bytes there are refused unless they are a stable
// producer map (CEM-V1-031, CEM-V1-143).
func (s *Session) preflightDestination(tool, relative string) error {
	stable, producer := s.stableClassAt(relative)
	if stable && !producer {
		return stableMutationRefusal(tool, relative)
	}
	return nil
}

// stableClassAt reports whether readable bytes at relative declare cem/1.0
// and, if so, whether they parse as a stable producer map.
func (s *Session) stableClassAt(relative string) (stable, producer bool) {
	if relative == "" || filepath.IsAbs(relative) {
		return false, false
	}
	raw, err := s.workRoot.ReadBounded(relative, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil || !wire.DeclaresStable(raw) {
		return false, false
	}
	document, err := wire.ParseStable(raw)
	return true, err == nil && stableProducerMap(document)
}

// stableProducerMap reports whether a stable map is one Core produces: it
// carries no criterion bindings, runner receipts, criterion links, artifacts
// or provenance. A role is admitted; it names the map's citation class only
// (CEM-V1-095, CEM-V1-143).
func stableProducerMap(document *wire.StableMap) bool {
	return len(document.Criteria) == 0 && len(document.Receipts) == 0 && len(document.Links) == 0 &&
		len(document.Artifacts) == 0 && document.Provenance == nil
}

// stableMechanics returns the mutable mechanics view of a stable map, with the
// complete mechanical, coverage and discrimination vocabulary; writeMap
// re-encodes the view as cem/1.0 (CEM-V1-143).
func stableMechanics(document *wire.StableMap) *wire.Map {
	view := wire.Map(document.Change)
	return &view
}

// stableOf returns the stable holder of a mechanics view, or nil for a view
// no mutator read.
func (s *Session) stableOf(document *wire.Map) *wire.StableMap {
	return s.stableHolders[document]
}

// encodeStableCandidate wraps a freshly built candidate in an empty
// stable producer map (CEM-V1-141, CEM-V1-142).
func encodeStableCandidate(document *wire.Map) ([]byte, error) {
	return encodeStableView(&wire.StableMap{
		Criteria: []wire.StableCriterion{}, Receipts: []wire.StableReceipt{},
		Links: []wire.StableLink{}, Artifacts: []wire.StableArtifact{},
	}, document)
}

// encodeStableView writes a mechanics view back into its stable holder and
// encodes it through the stable encoder, which revalidates the document.
func encodeStableView(holder *wire.StableMap, document *wire.Map) ([]byte, error) {
	change := wire.StableChange(*document)
	change.Spec, change.ExcludedPath = wire.StableSpec, wire.ExcludedCEMPath
	holder.Change = change
	return wire.EncodeStable(holder)
}

// preflightStableSidecar refuses prepare before it locks the sidecar or
// restores an interrupted-publication backup when the sidecar, or a backup
// that recovery would restore or resume cleanup would remove, declares
// cem/1.0 (CEM-V1-030). A recognized backup gets the strict probe. Backup
// names follow publish.RecoverPairBackup: "<map>.bak-" plus sixteen lowercase
// hex digits, regular files only. Prepare replaces no sidecar here
// (refuseStableReplace guards --replace) and admits a stable producer backup;
// a stable backup carrying criteria, receipts, links, artifacts or provenance,
// or an unreadable one, is still refused (CEM-V1-141). With --replace, the
// non-mutating replace guard runs here too, so its refusal precedes the lock.
func (s *Session) preflightStableSidecar(replace bool) error {
	if replace {
		if err := s.refuseStableReplace(); err != nil {
			return err
		}
	}
	directory := path.Dir(wire.ExcludedCEMPath)
	entries, err := os.ReadDir(filepath.Join(s.workRoot.Path(), filepath.FromSlash(directory)))
	if err != nil {
		return nil
	}
	prefix := path.Base(wire.ExcludedCEMPath) + ".bak-"
	for _, entry := range entries {
		suffix, found := strings.CutPrefix(entry.Name(), prefix)
		if !found || len(suffix) != 16 || strings.Trim(suffix, "0123456789abcdef") != "" || !entry.Type().IsRegular() {
			continue
		}
		if err := s.refuseStableEntry("prepare", path.Join(directory, entry.Name()), "restore or remove"); err != nil {
			return err
		}
	}
	return nil
}

// stableMutationRefusal is the explicit refusal of a Core mutation of a stable
// map that is not a stable producer map.
// Core mutates only the producer map it writes; a bound map's criteria,
// receipts, links, artifacts and provenance belong to the candidate assembler,
// and the Core read is status or verify (CEM-V1-017, CEM-V1-143). Its code is
// not-a-producer-map, never unsupported-spec: cem/1.0 is supported (CEM-V1-342).
func stableMutationRefusal(tool, relative string) error {
	return cemcode.New(cemcode.NotAProducerMap,
		"%s is a %s map, which Core cem %s does not mutate or replace here: Core mutates only a producer map without criteria, receipts, links, artifacts or provenance; read it with cem status or cem verify and --artifacts",
		relative, wire.StableSpec, tool)
}

func (s *Session) writeMap(document *wire.Map, inputRelative, outputRelative string) (string, error) {
	relative, err := mapOutputRelative(inputRelative, outputRelative)
	if err != nil {
		return "", err
	}
	// Every mutator input is a stable producer map, re-encoded as cem/1.0
	// (CEM-V1-143).
	encoded, err := encodeStableView(s.stableOf(document), document)
	if err != nil {
		return "", err
	}
	if err := publish.Publish(publish.Output{Root: s.workRoot, Relative: relative, Data: encoded}); err != nil {
		return "", err
	}
	return filepath.Join(s.workRoot.Path(), filepath.FromSlash(relative)), nil
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
