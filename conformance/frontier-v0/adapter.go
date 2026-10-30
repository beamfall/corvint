// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// This file is the ONLY place this suite touches the implementation. It is
// deliberately tiny and deliberately does not name any internal/frontier
// symbol.
//
// frontier/0 was deleted with the cem/0.x wire (decision 0525 §1, CEM-V1-217),
// and with it the fixture runner and the CF-V0-006/007 identity seam: there is
// no frontier/0 entry point or identity derivation left to bind. The CF-V0-019
// codec survives as the shared WP3 codec, so the codec seam stays bound
// (binding_impl.go) and the hand-derived codec vectors still run against it.

// Codecer is the seam the CF-V0-019 vectors need: an implementation that
// exposes its canonical codec can be checked against every codec vector.
type Codecer interface {
	// CodecCanonicalize parses raw bytes under CF-V0-019 and returns the exact
	// canonical serialization. It returns an error for every input the clause
	// names invalid.
	CodecCanonicalize(raw []byte) ([]byte, error)
}

var registeredCodecer Codecer

// RegisterCodecer binds the implementation's canonical codec.
func RegisterCodecer(c Codecer) { registeredCodecer = c }

// unboundReason is the skip message the codec implementation test prints when
// no codec is bound.
const unboundReason = "no implementation codec is bound: add a binding_impl.go calling RegisterCodecer. " +
	"The vectors in this directory are authored from docs/specs/change-frontier-v0.md and must not be " +
	"regenerated from the implementation (CF-V0-028)."
