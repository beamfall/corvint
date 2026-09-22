// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// This file is the ONLY place this suite touches the implementation. It is
// deliberately tiny and deliberately does not name any internal/frontier
// symbol.
//
// Why the seam is shaped this way. CF-V0-028 requires an independent consumer;
// this suite was authored before internal/frontier exported an entry point, and
// guessing that signature would have made the vectors hostage to the guess.
// So the vectors and fixtures are DATA (vectors/*.json, fixtures/*/case.json)
// and this package declares the contract it needs. When the implementation
// lands, add ONE file to this directory:
//
//	// binding_impl.go
//	package main
//
//	import "github.com/Beamfall/corvint/internal/frontier"
//
//	func init() { RegisterRunner(theRunner{}) }
//
// Nothing else in this package changes. Until that file exists, every
// implementation-dependent test skips with an explicit reason, which is the
// correct state for an independent vector suite authored ahead of its producer.
// Once it exists, a case still skips when the bound runner cannot MATERIALIZE
// its declared universe — but with the specific reason that universe is out of
// reach, which capabilities.go owns.

// Outcome is what one fixture invocation produced. It is expressed in wire
// terms — exact bytes and an exit code — rather than in implementation types,
// so the binding can be written against any entry-point signature.
type Outcome struct {
	// Stdout is the complete Frontier document bytes, or empty on operational
	// failure (CF-V0-022 forbids stdout there).
	Stdout []byte
	// Stderr is the complete error envelope bytes, or empty on success.
	Stderr []byte
	// ExitCode is 0 valid empty, 1 valid open, 2 operational failure.
	ExitCode int
	// Symbols maps the symbolic identifiers a fixture uses — placeholder hunk
	// IDs, claim IDs — onto the real identifiers the materialized universe
	// produced. A fixture cannot name a content-addressed ID it never saw, so
	// the expectation names a symbol and the runner reports what it bound the
	// symbol to. Anything absent from this map is compared literally, which is
	// what keeps obligation IDs (real, caller-authored, and named verbatim in
	// the fixture) honest.
	Symbols map[string]string
}

// Runner materializes a fixture's declared universe and invokes the
// implementation on it.
//
// A runner is not required to be able to build every declared universe. The
// matrix contains rows that no caller of the public entry point can produce —
// an injected timeout, a forged TCQ authority class, a 2,048-hunk result — and
// a runner that pretended otherwise would turn honest skips into harness
// failures. `Supports` declares what this runner can build; a case whose
// RequiredCapabilities are not all supported skips with the specific reason
// that capability is out of reach.
type Runner interface {
	// Run materializes one case's declared universe and invokes the
	// implementation on it. The fixture is passed alongside the case because a
	// few matrix rows are about a different seam — a candidate document to be
	// refused rather than a universe to be computed — and the fixture is where
	// that is declared.
	Run(f Fixture, c Case) (Outcome, error)
	Supports(capability Capability) bool
}

// Codecer is the narrower seam the CF-V0-019 vectors need: an implementation
// that exposes only its canonical codec can still be checked against every
// codec vector without a full Frontier invocation.
type Codecer interface {
	// CodecCanonicalize parses raw bytes under CF-V0-019 and returns the exact
	// canonical serialization. It returns an error for every input the clause
	// names invalid.
	CodecCanonicalize(raw []byte) ([]byte, error)
}

// Identifier is the seam the CF-V0-006 and CF-V0-007 vectors need.
type Identifier interface {
	// UniverseID computes the CF-V0-006 identity from the exact preimage codec
	// bytes recorded in the vector.
	UniverseID(preimageCodec []byte) (string, error)
	// ItemID computes the CF-V0-007 identity from the exact preimage codec bytes
	// recorded in the vector.
	ItemID(preimageCodec []byte) (string, error)
}

var (
	registeredRunner     Runner
	registeredCodecer    Codecer
	registeredIdentifier Identifier
)

// RegisterRunner binds a fixture runner. Call from an init function in the
// binding file.
func RegisterRunner(r Runner) { registeredRunner = r }

// RegisterCodecer binds the implementation's canonical codec.
func RegisterCodecer(c Codecer) { registeredCodecer = c }

// RegisterIdentifier binds the implementation's identity derivations.
func RegisterIdentifier(i Identifier) { registeredIdentifier = i }

// unboundReason is the skip message every implementation-dependent test prints.
// It names what to do rather than merely reporting absence.
const unboundReason = "internal/frontier has not bound this seam yet: " +
	"add a binding_impl.go to conformance/frontier-v0 calling RegisterRunner/RegisterCodecer/" +
	"RegisterIdentifier. The vectors and fixtures in this directory are complete and are the " +
	"expectation; they are authored from docs/specs/change-frontier-v0.md and must not be " +
	"regenerated from the implementation (CF-V0-028)."
