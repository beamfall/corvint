// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// This is the single binding file adapter.go describes. It exists so the
// independently authored vectors in this directory are actually run against
// `internal/frontier`, which is the only thing that turns CF-V0-028 from an
// aspiration into a check.
//
// The binding deliberately hands the implementation TYPED values and lets it
// build its own preimage, rather than hashing the vector's preimage bytes
// directly. Hashing the recorded bytes would only prove SHA-256 works. Going
// through the typed API means the implementation reconstructs the CF-V0-006
// and CF-V0-007 preimages from scratch — key names, key order, nesting, and
// the canonical decimal form for both span offsets — and any difference in any
// of those changes the digest and fails the vector.

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/wp3codec"
)

func init() {
	RegisterCodecer(implCodec{})
	RegisterIdentifier(implIdentity{})
}

// implCodec routes the CF-V0-019 vectors through the shared WP3 codec. Parse
// then Encode is exactly the clause's own "parse, reserialize, and require byte
// equality before hashing" discipline, so an input the clause names invalid must
// fail in Parse rather than survive into a digest.
type implCodec struct{}

func (implCodec) CodecCanonicalize(raw []byte) ([]byte, error) {
	value, err := wp3codec.Parse(raw)
	if err != nil {
		return nil, err
	}
	return wp3codec.Encode(value)
}

// implIdentity converts a vector's recorded preimage back into the typed inputs
// the implementation accepts.
type implIdentity struct{}

// universePreimage mirrors the CF-V0-006 preimage shape. Offsets are canonical
// decimal STRINGS in the wire preimage — the codec forbids JSON numbers — so
// they are decoded as strings here and converted, never read as numbers.
type universePreimage struct {
	BaseRevision string `json:"baseRevision"`
	ExcludedPath string `json:"excludedPath"`
	Intent       struct {
		BlobOID string `json:"blobOid"`
		Path    string `json:"path"`
		Span    struct {
			End   string `json:"end"`
			Start string `json:"start"`
		} `json:"span"`
		SpanSHA256 string `json:"spanSha256"`
	} `json:"intent"`
	ObjectFormat   string `json:"objectFormat"`
	PatchSHA256    string `json:"patchSha256"`
	TargetRevision string `json:"targetRevision"`
}

type itemPreimage struct {
	Kind       string `json:"kind"`
	SubjectID  string `json:"subjectId"`
	UniverseID string `json:"universeId"`
}

func (implIdentity) UniverseID(preimageCodec []byte) (string, error) {
	var decoded universePreimage
	if err := json.Unmarshal(preimageCodec, &decoded); err != nil {
		return "", fmt.Errorf("decode universe preimage: %w", err)
	}
	// The excluded path is frozen by CF-V0-006 and is not a Scope field, so a
	// vector carrying a different one would silently pass a check that never
	// looked at it. Refuse instead.
	if decoded.ExcludedPath != frontier.ExcludedPath {
		return "", fmt.Errorf("vector excludedPath %q is not the frozen CF-V0-006 path %q",
			decoded.ExcludedPath, frontier.ExcludedPath)
	}
	start, err := strconv.ParseInt(decoded.Intent.Span.Start, 10, 64)
	if err != nil {
		return "", fmt.Errorf("intent span start is not canonical decimal: %w", err)
	}
	end, err := strconv.ParseInt(decoded.Intent.Span.End, 10, 64)
	if err != nil {
		return "", fmt.Errorf("intent span end is not canonical decimal: %w", err)
	}
	return frontier.UniverseID(frontier.Scope{
		BaseRevision:     decoded.BaseRevision,
		IntentBlobOID:    decoded.Intent.BlobOID,
		IntentPath:       decoded.Intent.Path,
		IntentSpan:       frontier.Span{Start: start, End: end},
		IntentSpanSHA256: decoded.Intent.SpanSHA256,
		ObjectFormat:     decoded.ObjectFormat,
		PatchSHA256:      decoded.PatchSHA256,
		TargetRevision:   decoded.TargetRevision,
	})
}

func (implIdentity) ItemID(preimageCodec []byte) (string, error) {
	var decoded itemPreimage
	if err := json.Unmarshal(preimageCodec, &decoded); err != nil {
		return "", fmt.Errorf("decode item preimage: %w", err)
	}
	return frontier.ItemID(decoded.Kind, decoded.SubjectID, decoded.UniverseID)
}
