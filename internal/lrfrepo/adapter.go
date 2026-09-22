// Package lrfrepo issues canonical LRF results from verified repository authority.
package lrfrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
)

const maxOCMBytes = 1 << 20

// Options are the repository-authority inputs accepted by corvint lrf.
type Options struct {
	CEMPath      string
	OCMPath      string
	PatchPath    string
	PatchGiven   bool
	ExpectedBase string
	Target       string
}

// Error is an adapter-owned structural or compatibility failure.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// CodeOf returns an LRF, CEM, or adapter error code.
func CodeOf(err error) string {
	if code := cemcode.CodeOf(err); code != "" {
		return code
	}
	for current := err; current != nil; {
		switch typed := current.(type) {
		case *Error:
			return typed.Code
		case *lrf.Error:
			return typed.Code
		case interface{ Unwrap() error }:
			current = typed.Unwrap()
		default:
			return ""
		}
	}
	return ""
}

func fail(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Evaluate verifies all repository authority and evaluates the frozen LRF projection.
func Evaluate(ctx context.Context, root string, options Options) (lrf.Result, error) {
	if options.CEMPath == "" {
		return lrf.Result{}, fail(cemcode.InvalidArguments, "--cem is required")
	}
	inputRoot, err := publish.OpenRoot(root)
	if err != nil {
		return lrf.Result{}, err
	}
	cemRaw, err := readRootInput(inputRoot, options.CEMPath, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil {
		return lrf.Result{}, err
	}
	document, err := wire.ParseMap(cemRaw)
	if err != nil {
		return lrf.Result{}, err
	}
	if document.Spec == wire.Spec02 && options.PatchGiven {
		return lrf.Result{}, fail(cemcode.InvalidArguments, "cem/0.2 lrf does not accept --patch")
	}
	if document.Spec == wire.Spec02 && options.ExpectedBase == "" {
		return lrf.Result{}, cemcode.New(cemcode.ExpectedBaseRequired, "cem/0.2 lrf requires --expected-base")
	}
	if document.Spec == wire.Spec02 && options.Target == "" {
		return lrf.Result{}, cemcode.New(cemcode.TargetRequired, "cem/0.2 lrf requires --target")
	}
	repository, err := gitauth.Open(inputRoot.Path(), gitrun.NewDefaultBudget())
	if err != nil {
		return lrf.Result{}, err
	}
	verified, err := verifyCEM(ctx, inputRoot, repository, document, cemRaw, options)
	if err != nil {
		return lrf.Result{}, err
	}
	if options.OCMPath != "" && document.Spec != wire.Spec02 {
		return lrf.Result{}, fail("unsupported-lrf-context", "OCM requires canonical cem/0.2")
	}
	ocm, err := verifyOptionalOCM(ctx, inputRoot, repository, document, cemRaw, verified, options)
	if err != nil {
		return lrf.Result{}, err
	}
	request, err := project(ctx, repository, document, cemRaw, verified, ocm)
	if err != nil {
		return lrf.Result{}, err
	}
	return lrf.Evaluate(request)
}

type verifiedCEM struct {
	base        string
	target      string
	patch       []byte
	patchSource string
	excluded    *string
}

func verifyCEM(ctx context.Context, root *publish.Root, repository *gitauth.Repository, document *wire.Map, raw []byte, options Options) (*verifiedCEM, error) {
	if document.Spec == wire.Spec02 {
		return verifyCanonicalCEM(ctx, repository, document, raw, options)
	}
	patchBytes, source, err := readExactPatch(root, repository, options)
	if err != nil {
		return nil, err
	}
	outcome, err := verify.Exact(ctx, repository, document, patchBytes, verify.ExactOptions{
		ExpectedBase: options.ExpectedBase, Target: options.Target,
	})
	if err != nil {
		return nil, err
	}
	return &verifiedCEM{outcome.BaseRevision, outcome.TargetRevision, patchBytes, source, nil}, nil
}

// verifyCanonicalCEM is the cem/0.2 half of verifyCEM: the shared canonical
// verification call in its own frozen order, then the canonical patch its
// accepted revisions derive. It is a named step rather than an inline branch
// so a bytes-only caller can reach it without a publication root — the 0.2
// path reads no filesystem input, and only the 0.1 branch below does.
func verifyCanonicalCEM(ctx context.Context, repository *gitauth.Repository, document *wire.Map, raw []byte, options Options) (*verifiedCEM, error) {
	outcome, _, err := verify.Canonical(ctx, repository, document, verify.CanonicalOptions{
		ExpectedBase: options.ExpectedBase, Target: options.Target, RawMapBytes: raw,
	})
	if err != nil {
		return nil, err
	}
	patchBytes, err := repository.CanonicalDiff(ctx, outcome.BaseRevision, outcome.TargetRevision)
	if err != nil {
		return nil, err
	}
	excluded := wire.ExcludedCEMPath
	return &verifiedCEM{outcome.BaseRevision, outcome.TargetRevision, patchBytes, "canonical-derived", &excluded}, nil
}

func readExactPatch(root *publish.Root, repository *gitauth.Repository, options Options) ([]byte, string, error) {
	if options.PatchGiven {
		if options.PatchPath == "" {
			return nil, "", fail(cemcode.InvalidArguments, "--patch requires a value")
		}
		data, err := readRootInput(root, options.PatchPath, patch.MaxPatchBytes, cemcode.PatchUnavailable)
		return data, "explicit-out-of-band", err
	}
	gitRoot, err := publish.OpenRoot(repository.GitDir)
	if err != nil {
		return nil, "", err
	}
	data, err := gitRoot.ReadBounded("corvint/change.patch", patch.MaxPatchBytes, cemcode.PatchUnavailable)
	return data, "default-out-of-band", err
}

func readRootInput(root *publish.Root, requested string, bound int, code string) ([]byte, error) {
	path := requested
	if filepath.IsAbs(path) {
		relative, err := filepath.Rel(root.Path(), filepath.Clean(path))
		if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
			return nil, fail(cemcode.InvalidArguments, "input path must remain inside repository root")
		}
		path = filepath.ToSlash(relative)
	}
	if filepath.Clean(filepath.FromSlash(path)) != filepath.FromSlash(path) {
		return nil, fail(cemcode.InvalidArguments, "input path must be normalized")
	}
	return root.ReadBounded(path, bound, code)
}

func project(ctx context.Context, repository *gitauth.Repository, document *wire.Map, cemRaw []byte, verified *verifiedCEM, ocm *verifiedOCM) (lrf.Request, error) {
	parsed, err := patch.Parse(verified.patch)
	if err != nil {
		return lrf.Request{}, err
	}
	mapped := make(map[string]wire.Hunk, len(document.Hunks))
	for _, hunk := range document.Hunks {
		mapped[hunk.ID] = hunk
	}
	hunks := make([]lrf.Hunk, 0, len(parsed.Hunks))
	for ordinal, parsedHunk := range parsed.Hunks {
		mappedHunk := mapped[parsedHunk.ID]
		added := make([]byte, 0)
		for _, line := range parsedHunk.Body {
			if line.Prefix == '+' {
				added = append(added, line.NewPayload()...)
			}
		}
		hunks = append(hunks, lrf.Hunk{
			ID: parsedHunk.ID, OldPath: parsedHunk.OldPath, NewPath: parsedHunk.NewPath,
			Added: added, Disposition: mappedHunk.Disposition, Basis: mappedHunk.Basis, Ordinal: ordinal,
		})
	}
	evidence := make([]lrf.Evidence, 0, len(document.Evidence))
	for _, record := range document.Evidence {
		entry, exists, err := repository.LookupTreeEntry(ctx, verified.base, record.Path)
		if err != nil {
			return lrf.Request{}, err
		}
		if !exists || entry.OID != record.BlobOid || entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" {
			return lrf.Request{}, cemcode.New(cemcode.EvidenceUnavailable, "verified evidence is no longer available")
		}
		blob, err := repository.BlobBytes(ctx, entry.OID)
		if err != nil {
			return lrf.Request{}, err
		}
		if record.Span.Start < 0 || record.Span.End > int64(len(blob)) || record.Span.Start >= record.Span.End {
			return lrf.Request{}, cemcode.New(cemcode.EvidenceUnavailable, "verified evidence span is unavailable")
		}
		evidence = append(evidence, lrf.Evidence{ID: record.ID, Path: record.Path, Span: append([]byte(nil), blob[record.Span.Start:record.Span.End]...)})
	}
	context := lrf.Context{
		CEMSpec: document.Spec, CEMMapSHA256: sha256Hex(cemRaw), PatchSource: verified.patchSource,
		BaseRevision: verified.base, PatchSHA256: sha256Hex(verified.patch), ExcludedPath: verified.excluded,
	}
	if verified.target != "" {
		context.TargetRevision = stringPointer(verified.target)
	}
	obligations := []lrf.Obligation(nil)
	if ocm != nil {
		context.OCMSpec = stringPointer(ocm.spec)
		context.OCMMapSHA256 = stringPointer(ocm.mapSHA256)
		context.IntentPath = stringPointer(ocm.intent.path)
		context.IntentBlobOID = stringPointer(ocm.intent.blobOID)
		context.IntentStart = intPointer(ocm.intent.start)
		context.IntentEnd = intPointer(ocm.intent.end)
		context.IntentSpanSHA256 = stringPointer(ocm.intent.spanSHA256)
		obligations = append(obligations, ocm.obligations...)
	}
	return lrf.Request{Context: context, Hunks: hunks, Evidence: evidence, Obligations: obligations}, nil
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func stringPointer(value string) *string { return &value }
func intPointer(value int64) *int64      { return &value }
