// Package cemcode registers the stable CEM error taxonomy shared by every CEM seam.
//
// Codes marked "frozen" are registered by the CEM 0.1 interop manifest
// (interop/cem-0.1/manifest.json expectedCode / conformance acceptableCodes) or by
// docs/specs/cem-0.2-canonical-binding.md CEM-CB-021 and must not be renamed.
// Codes marked "operational" report documented implementation limits and are
// distinguished from protocol invalidity per interop/cem-0.1/ALGORITHMS.md.
package cemcode

import "fmt"

// Error is a registered CEM failure: a stable machine code plus a human message.
type Error struct {
	Code     string
	Message  string
	gitExit  *GitExitDetails
	gitStart error
	guidance string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// New returns a registered error with the given stable code.
func New(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Guided attaches one bounded, source-content-free recovery line. A surface
// that collapses the registered message to a fixed text keeps this line, so a
// caller can act without inspecting the implementation.
func (e *Error) Guided(guidance string) *Error {
	e.guidance = guidance
	return e
}

// GuidanceOf returns the bounded recovery line carried by err, or "" when it
// carries none.
func GuidanceOf(err error) string {
	var registered *Error
	if !asError(err, &registered) {
		return ""
	}
	return registered.guidance
}

// GitExitDetails retains non-rendered process details for command-scoped parity adapters.
type GitExitDetails struct {
	ExitCode int
	Stderr   []byte
}

// NewGitExitFailure preserves the registered error bytes and attaches bounded process details.
func NewGitExitFailure(message string, exitCode int, stderr []byte) *Error {
	return &Error{
		Code: GitExitFailure, Message: message,
		gitExit: &GitExitDetails{ExitCode: exitCode, Stderr: append([]byte(nil), stderr...)},
	}
}

// NewGitStartFailure preserves the registered error bytes and attaches the process start error.
func NewGitStartFailure(message string, startError error) *Error {
	return &Error{Code: GitStartFailed, Message: message, gitStart: startError}
}

// GitExitFailureDetails returns a defensive copy of a registered Git exit failure.
func GitExitFailureDetails(err error) (GitExitDetails, bool) {
	var registered *Error
	if !asError(err, &registered) || registered.gitExit == nil {
		return GitExitDetails{}, false
	}
	details := *registered.gitExit
	details.Stderr = append([]byte(nil), details.Stderr...)
	return details, true
}

// GitStartFailureDetails returns the process start error without changing rendered bytes.
func GitStartFailureDetails(err error) (error, bool) {
	var registered *Error
	if !asError(err, &registered) || registered.gitStart == nil {
		return nil, false
	}
	return registered.gitStart, true
}

// CodeOf returns the registered code carried by err, or "" when err carries none.
// MessageOf returns the registered message WITHOUT the code prefix that
// Error() adds. The wire envelope carries code and message in separate fields,
// so using Error() there printed the code twice.
func MessageOf(err error) string {
	var registered *Error
	if ok := asError(err, &registered); ok {
		return registered.Message
	}
	return err.Error()
}

func CodeOf(err error) string {
	var registered *Error
	if ok := asError(err, &registered); ok {
		return registered.Code
	}
	return ""
}

func asError(err error, target **Error) bool {
	for err != nil {
		if e, ok := err.(*Error); ok {
			*target = e
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

// Frozen wire/verification codes.
const (
	InvalidJSON             = "invalid-json"              // malformed UTF-8/JSON, duplicate key, bad integer
	NotAnObject             = "not-an-object"             // root JSON value is not an object
	MissingField            = "missing-field"             // frozen: cem-0.2 cases.json missing-excluded-path
	UnknownField            = "unknown-field"             // frozen: cem-0.2 cases.json surplus-field
	UnsupportedSpec         = "unsupported-spec"          // frozen: CEM-CB-021
	InvalidField            = "invalid-field"             // structurally invalid wire value
	InvalidExcludedPath     = "invalid-excluded-path"     // frozen: CEM-CB-021
	PathTraversal           = "path-traversal"            // frozen: conformance cases.json
	PatchDigestMismatch     = "patch-digest-mismatch"     // frozen: conformance cases.json, CEM-CB-021
	BaseRevisionMismatch    = "base-revision-mismatch"    // frozen: CEM-CB-021
	FabricatedEvidenceID    = "fabricated-evidence-id"    // frozen: conformance cases.json
	UncitedHunk             = "uncited-hunk"              // frozen: conformance cases.json
	UnsupportedWithoutBasis = "unsupported-without-basis" // frozen: conformance cases.json
	UnprovenMechanical      = "unproven-mechanical"       // frozen: interop manifest expectedCode
	OrphanEvidence          = "orphan-evidence"           // uncited or duplicate evidence records
	EvidenceUnavailable     = "evidence-unavailable"      // evidence path/blob/span does not resolve
	EvidenceDrift           = "evidence-drift"            // stale, ambiguous, or deleted target evidence
)

// Frozen patch-parser codes.
const (
	BinaryPatch          = "binary-patch"           // binary marker, NUL, empty, or oversized input
	MalformedPatch       = "malformed-patch"        // state-machine grammar violation
	ExtraHunkPayload     = "extra-hunk-payload"     // frozen: conformance cases.json referenceCode
	DiffMetadataMismatch = "diff-metadata-mismatch" // frozen: interop manifest expectedCode
	UnsupportedTreeMode  = "unsupported-tree-mode"  // frozen: interop manifest expectedCode
	TooManyLines         = "too-many-lines"         // frozen: interop manifest expectedCode
	BaseMismatch         = "base-mismatch"          // simulated old bytes disagree with the base tree
)

// Frozen CLI/workflow codes (CEM-CB-021).
const (
	InvalidArguments            = "invalid-arguments"
	ExpectedBaseRequired        = "expected-base-required"
	TargetRequired              = "target-required"
	ExcludedPathNotFile         = "excluded-path-not-file"
	ExcludedArtifactMismatch    = "excluded-artifact-mismatch"
	UnsupportedObjectAlternates = "unsupported-object-alternates"
	RepositoryObjectUnavailable = "repository-object-unavailable"
	// GitReadFailed, UnknownHunkID, and MissingEvidence are the oracle's codes
	// for a revision that will not resolve, a hunk selector that names no mapped
	// hunk, and an evidence path absent at the base. The candidate had invented
	// its own code for each.
	GitReadFailed   = "git-read-failed"
	UnknownHunkID   = "unknown-hunk-id"
	MissingEvidence = "missing-evidence"
	// The four span codes the oracle distinguishes. The candidate had collapsed
	// all of them, plus four parse-time argument errors, into invalid-arguments.
	InvalidSpan         = "invalid-span"
	SpanOutOfRange      = "span-out-of-range"
	InvalidLineRange    = "invalid-line-range"
	LineRangeOutOfRange = "line-range-out-of-range"
	CiteSpanNotStable   = "cite-span-not-stable"
	GitDiffFailed       = "git-diff-failed"
	GitDiffTimeout      = "git-diff-timeout"
)

// Typed bounded-runner codes.
const (
	GitCancelled      = "git-cancelled"       // caller context cancelled before completion
	GitTimeout        = "git-timeout"         // per-operation or budget deadline elapsed
	GitOutputExceeded = "git-output-exceeded" // stdout or stderr exceeded its byte bound
	GitBudgetExceeded = "git-budget-exceeded" // operation-count budget exhausted
	GitStartFailed    = "git-start-failed"    // process could not start
	GitExitFailure    = "git-exit-failure"    // process exited non-zero
)

// Operational refusal codes (documented limits, not protocol invalidity).
const (
	UnsupportedRepositoryAttributes = "unsupported-repository-attributes" // non-empty $GIT_DIR/info/attributes
	MapUnavailable                  = "map-unavailable"                   // bounded map read failed
	PatchUnavailable                = "patch-unavailable"                 // bounded patch read failed
	PublishFailed                   = "publish-failed"                    // transactional output publication failed
	MapLocked                       = "map-locked"                        // update-lock wait expired behind another map update
)
