package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// PairedCiteOptions cite one span of a second repository's range head for one
// hunk of a cem/1.0 map into a cem-paired-evidence/1 document (CAX-V0-032).
type PairedCiteOptions struct {
	MapPath      string
	Output       string // repository-relative paired-evidence document
	Range        string // NAME=BASE..HEAD
	Root         string // absolute path of the second repository
	Hunk         string
	EvidencePath string
	Bytes        string
	Lines        string
	Relation     string
}

// PairedStatusOptions re-verify a cem-paired-evidence/1 document (CAX-V0-033).
type PairedStatusOptions struct {
	MapPath string
	Paired  string
	Root    string
}

// parsePairedRange splits NAME=BASE..HEAD.
func parsePairedRange(text string) (name, base, head string, err error) {
	name, revisions, found := strings.Cut(text, "=")
	base, head, dotted := strings.Cut(revisions, "..")
	if !found || !dotted || !wire.ValidPairedRangeName(name) || base == "" || head == "" {
		return "", "", "", invalidArguments("--paired-range must be NAME=BASE..HEAD with a lowercase name")
	}
	return name, base, head, nil
}

// openPairedRepository opens the second repository named by an absolute
// --paired-root under its own Git budget.
func openPairedRepository(root string) (*gitauth.Repository, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, invalidArguments("--paired-root must be an absolute path")
	}
	return gitauth.Open(root, gitrun.NewDefaultBudget())
}

// resolvePairedRange resolves both ends of a paired range to full commit IDs
// and requires base to be an ancestor of head; either end failing to resolve
// refuses the command with that end named (CAX-V0-032).
func resolvePairedRange(ctx context.Context, repository *gitauth.Repository, name, base, head string) (wire.PairedRange, error) {
	resolved := wire.PairedRange{Name: name}
	for _, end := range []struct {
		label, revision string
		into            *string
	}{{"base", base, &resolved.BaseRevision}, {"head", head, &resolved.HeadRevision}} {
		oid, err := repository.Resolve(ctx, end.revision)
		if err != nil {
			code := cemcode.CodeOf(err)
			if code == "" {
				code = cemcode.RepositoryObjectUnavailable
			}
			return wire.PairedRange{}, cemcode.New(code, "paired range %s: %s %q does not resolve: %v", name, end.label, end.revision, err)
		}
		*end.into = oid
	}
	if resolved.BaseRevision == resolved.HeadRevision {
		return wire.PairedRange{}, invalidArguments("paired range %s: base and head are the same commit", name)
	}
	ancestor, err := repository.IsAncestor(ctx, resolved.BaseRevision, resolved.HeadRevision)
	if err != nil {
		return wire.PairedRange{}, err
	}
	if !ancestor {
		return wire.PairedRange{}, invalidArguments("paired range %s: base is not an ancestor of head", name)
	}
	return resolved, nil
}

// checkPairedOutput refuses a document path that is absolute or names the map
// or the map's lock or backup name.
func checkPairedOutput(mapRelative, outputRelative string) error {
	if outputRelative == "" || filepath.IsAbs(outputRelative) {
		return invalidArguments("--paired-output must be a repository-relative path")
	}
	if strings.EqualFold(path.Clean(filepath.ToSlash(outputRelative)), path.Clean(filepath.ToSlash(mapRelative))) {
		return invalidArguments("--paired-output must not name map %s", mapRelative)
	}
	if err := checkMapSidecarAliasing(mapRelative, outputRelative); err != nil {
		return invalidArguments("--paired-output must not select the lock or backup name of map %s", mapRelative)
	}
	return nil
}

// readPairedMap reads a repository-relative cem/1.0 map without a lock: the
// paired profile never writes the map.
func (s *Session) readPairedMap(relative string) ([]byte, *wire.StableMap, error) {
	if filepath.IsAbs(relative) {
		return nil, nil, invalidArguments("map path must be repository-relative")
	}
	raw, err := s.readMapBytes(relative)
	if err != nil {
		return nil, nil, err
	}
	document, err := wire.ParseStable(raw)
	if err != nil {
		return nil, nil, err
	}
	return raw, document, nil
}

// CitePaired records one citation of a regular blob at the paired range's
// head in the cem-paired-evidence/1 document --paired-output names, creating
// it or appending to one bound to the same change and range. The map is read,
// never written (CAX-V0-032).
func (s *Session) CitePaired(ctx context.Context, options PairedCiteOptions) (map[string]any, error) {
	name, base, head, err := parsePairedRange(options.Range)
	if err != nil {
		return nil, err
	}
	if err := checkPairedOutput(options.MapPath, options.Output); err != nil {
		return nil, err
	}
	if err := wire.ValidatePath(options.EvidencePath); err != nil {
		return nil, err
	}
	if (options.Bytes == "") == (options.Lines == "") {
		return nil, invalidArguments("exactly one of --bytes and --lines is required")
	}
	if !wire.Relations[options.Relation] {
		return nil, invalidArguments("--relation must be a registered relation")
	}
	_, stable, err := s.readPairedMap(options.MapPath)
	if err != nil {
		return nil, err
	}
	index, err := resolveHunk(stableMechanics(stable), options.Hunk)
	if err != nil {
		return nil, err
	}
	repository, err := openPairedRepository(options.Root)
	if err != nil {
		return nil, err
	}
	pin, err := resolvePairedRange(ctx, repository, name, base, head)
	if err != nil {
		return nil, err
	}
	entry, exists, err := repository.LookupTreeEntry(ctx, pin.HeadRevision, options.EvidencePath)
	if err != nil {
		return nil, err
	}
	if !exists || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return nil, cemcode.New(cemcode.MissingEvidence, "evidence path is not a regular file at the paired range head")
	}
	blob, err := repository.BlobBytes(ctx, entry.OID)
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
	digest := sha256.Sum256(blob[span.Start:span.End])
	citation := wire.PairedCitation{
		BlobOid: entry.OID, HunkID: stable.Change.Hunks[index].ID, Path: options.EvidencePath,
		Relation: options.Relation, Span: wire.PairedSpan{Start: span.Start, End: span.End},
		SpanSha256: hex.EncodeToString(digest[:]),
	}
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	relative := path.Clean(filepath.ToSlash(options.Output))
	unlock, err := s.workRoot.Lock(relative)
	if err != nil {
		return nil, err
	}
	defer unlock()
	document := wire.PairedEvidence{
		Change:  wire.PairedChange{BaseRevision: stable.Change.BaseRevision, PatchSha256: stable.Change.PatchSha256},
		Profile: wire.PairedEvidenceProfile, Range: pin,
	}
	if _, statErr := os.Lstat(filepath.Join(s.workRoot.Path(), filepath.FromSlash(relative))); !errors.Is(statErr, fs.ErrNotExist) {
		raw, err := s.workRoot.ReadBounded(relative, wire.MaxPairedEvidenceBytes, cemcode.InvalidArguments)
		if err != nil {
			return nil, err
		}
		existing, err := wire.ParsePairedEvidence(raw)
		if err != nil {
			return nil, cemcode.New(cemcode.InvalidArguments, "--paired-output: %v", err)
		}
		if existing.Change != document.Change || existing.Range != document.Range {
			return nil, invalidArguments("--paired-output records a different change or paired range")
		}
		document.Citations = existing.Citations
	}
	added := true
	for _, recorded := range document.Citations {
		added = added && recorded != citation
	}
	if added {
		document.Citations = append(document.Citations, citation)
	}
	encoded, err := wire.EncodePairedEvidence(document)
	if err != nil {
		return nil, err
	}
	if _, err := wire.ParsePairedEvidence(encoded); err != nil {
		return nil, err
	}
	if err := publish.Publish(publish.Output{Root: s.workRoot, Relative: relative, Data: encoded}); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-cite-paired", "pairedEvidence": relative,
		"profile": wire.PairedEvidenceProfile, "hunkId": citation.HunkID, "added": added,
		"range": map[string]any{"name": pin.Name, "baseRevision": pin.BaseRevision, "headRevision": pin.HeadRevision},
	}, nil
}

// PairedStatus re-verifies a cem-paired-evidence/1 document against the map
// and the second repository: the document binds the map's change, both pinned
// commits still resolve to themselves with base an ancestor of head, and each
// citation's path is still the same regular blob at head with the same span
// digest. Any failure refuses the whole document (CAX-V0-033).
func (s *Session) PairedStatus(ctx context.Context, options PairedStatusOptions) (map[string]any, error) {
	if options.Paired == "" || filepath.IsAbs(options.Paired) {
		return nil, invalidArguments("--paired must be a repository-relative path")
	}
	mapBytes, _, err := s.readPairedMap(options.MapPath)
	if err != nil {
		return nil, err
	}
	raw, err := s.workRoot.ReadBounded(path.Clean(filepath.ToSlash(options.Paired)), wire.MaxPairedEvidenceBytes, cemcode.InvalidArguments)
	if err != nil {
		return nil, err
	}
	document, err := wire.ParsePairedEvidence(raw)
	if err != nil {
		return nil, cemcode.New(cemcode.InvalidField, "--paired: %v", err)
	}
	stable, err := wire.BindPairedEvidence(document, mapBytes)
	if err != nil {
		return nil, cemcode.New(cemcode.InvalidField, "--paired: %v", err)
	}
	repository, err := openPairedRepository(options.Root)
	if err != nil {
		return nil, err
	}
	pin := document.Range
	resolved, err := resolvePairedRange(ctx, repository, pin.Name, pin.BaseRevision, pin.HeadRevision)
	if err != nil {
		return nil, err
	}
	if resolved != pin {
		return nil, cemcode.New(cemcode.EvidenceDrift, "paired range %s no longer resolves to its pinned commits", pin.Name)
	}
	for _, citation := range document.Citations {
		entry, exists, err := repository.LookupTreeEntry(ctx, pin.HeadRevision, citation.Path)
		if err != nil {
			return nil, err
		}
		if !exists || entry.Type != "blob" || entry.OID != citation.BlobOid {
			return nil, cemcode.New(cemcode.EvidenceDrift, "paired citation %s is not blob %s at the range head", citation.Path, citation.BlobOid)
		}
		blob, err := repository.BlobBytes(ctx, entry.OID)
		if err != nil {
			return nil, err
		}
		if citation.Span.End > int64(len(blob)) {
			return nil, cemcode.New(cemcode.EvidenceDrift, "paired citation %s span is outside its blob", citation.Path)
		}
		digest := sha256.Sum256(blob[citation.Span.Start:citation.Span.End])
		if hex.EncodeToString(digest[:]) != citation.SpanSha256 {
			return nil, cemcode.New(cemcode.EvidenceDrift, "paired citation %s span digest differs", citation.Path)
		}
	}
	cited := map[string]bool{}
	for _, citation := range document.Citations {
		cited[citation.HunkID] = true
	}
	var unknown, unknownCited int64
	for _, hunk := range stable.Change.Hunks {
		if hunk.Disposition == "unknown" {
			unknown++
			if cited[hunk.ID] {
				unknownCited++
			}
		}
	}
	documentDigest := sha256.Sum256(raw)
	return map[string]any{
		"ok": true, "mutates": false, "tool": "cem-paired-status", "profile": wire.PairedEvidenceProfile,
		"evidenceSha256": hex.EncodeToString(documentDigest[:]),
		"range":          map[string]any{"name": pin.Name, "baseRevision": pin.BaseRevision, "headRevision": pin.HeadRevision},
		"counts": map[string]any{
			"citations": len(document.Citations), "pairedHunks": len(cited), "total": len(stable.Change.Hunks),
			"unknown": unknown, "unknownWithPairedEvidence": unknownCited,
		},
	}, nil
}
