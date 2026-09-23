// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// The two seams through which this suite reaches the implementation. Both are
// the real exported entry points `cmd/corvint` calls; nothing is re-implemented
// here, so a vector that passes proves the decoder the CLI ships with, not a
// double.
//
//   - DecodeStructural reaches the structural OCM parser through `ocm mark`,
//     the one producer that consumes no CEM and opens no repository: a
//     structural refusal is returned before any selector or mutation, and a
//     valid map is re-canonicalized and republished, which is what the
//     round-trip identity vectors need.
//   - Verify reaches the full verifier through `ocm status`'s reader over a
//     real repository, CEM, and target.

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// Decoded is the outcome of one structural decode.
type Decoded struct {
	// Code is the refusal code, or empty when the map parsed and was
	// republished.
	Code    string
	Message string
	// Output is the republished map bytes when the map parsed.
	Output []byte
}

// DecodeStructural writes raw as the OCM map of a bare temporary root and
// applies `ocm mark` of obligation to reason. A map that fails the structural
// parser is refused with its code; a map that parses is republished.
func DecodeStructural(raw []byte, obligation, reason string) (Decoded, error) {
	root, err := os.MkdirTemp("", "ocm-v0-vector-")
	if err != nil {
		return Decoded{}, err
	}
	defer os.RemoveAll(root)
	if err := writeFile(root, MapPath, string(raw)); err != nil {
		return Decoded{}, err
	}
	_, err = lrfrepo.MarkOCM(context.Background(), root, lrfrepo.MarkOptions{
		MapPath: MapPath, Obligation: obligation, Reason: reason,
	})
	if err != nil {
		if code := lrfrepo.CodeOf(err); code != "" {
			return Decoded{Code: code, Message: err.Error()}, nil
		}
		return Decoded{}, err
	}
	output, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(MapPath)))
	if err != nil {
		return Decoded{}, err
	}
	return Decoded{Output: output}, nil
}

// Verdict is the outcome of one full verification.
type Verdict struct {
	// Refusal is the operational refusal code when the reader returned an
	// error instead of a verdict.
	Refusal string
	State   string
	Code    string
	Counts  map[string]int
}

// Verify writes ocmRaw as the universe's scope map and reads it through the
// `ocm status` reader with the given caller target.
func Verify(u *Universe, ocmRaw []byte, target string) (Verdict, error) {
	if err := writeFile(u.Root, MapPath, string(ocmRaw)); err != nil {
		return Verdict{}, err
	}
	result, err := lrfrepo.ReadOCM(context.Background(), u.Root, lrfrepo.OCMReadOptions{
		OCMPath: MapPath, CEMPath: wire.ExcludedCEMPath,
		ExpectedBase: u.Base, Target: target,
		ExpectedBaseGiven: true, TargetGiven: true,
	})
	if err != nil {
		if code := lrfrepo.CodeOf(err); code != "" {
			return Verdict{Refusal: code}, nil
		}
		return Verdict{}, err
	}
	return verdictOf(result), nil
}

func verdictOf(result *lrfrepo.OCMReadResult) Verdict {
	verdict := Verdict{State: result.State, Counts: map[string]int{}}
	for key, value := range result.Counts {
		if count, ok := value.(int); ok {
			verdict.Counts[key] = count
		}
	}
	issues, _ := result.Verification["issues"].([]any)
	for _, issue := range issues {
		if fields, ok := issue.(map[string]any); ok {
			verdict.Code, _ = fields["code"].(string)
			break
		}
	}
	return verdict
}
