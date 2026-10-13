package frontier

import (
	"context"
	"errors"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/lrf"
)

// Frontier-owned operational codes (CF-V0-022).
const (
	CodeInvalidInput       = "invalid-frontier-input"
	CodeUnsupportedContext = "unsupported-frontier-context"
	CodeNoncanonical       = "noncanonical-frontier"
	CodeResourceExhausted  = "frontier-resource-exhausted"
	CodeInterrupted        = "frontier-interrupted"
	CodeInternalError      = "frontier-internal-error"
)

// Error is one operational Frontier failure. It carries a code and nothing
// else on purpose: CF-V0-024 forbids echoing an unverified path, OID, digest,
// ID, count, XML value, argv element, sibling canary, or exception, and a
// message field is the usual way all of those leak. The unexported detail is
// for a Go caller's debugger, is never rendered, and never reaches Error().
type Error struct {
	Code   string
	detail string
}

// Error returns the code alone, so that even an accidental %v in a caller's
// log cannot widen the privacy surface.
func (e *Error) Error() string { return e.Code }

func fail(code, detail string) *Error { return &Error{Code: code, detail: detail} }

// CodeOf extracts a Frontier code from an error chain, or "" when the error is
// not a Frontier failure.
func CodeOf(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

// upstreamCode reads the explicit code an upstream error declares, without
// prefix or message matching, which CF-V0-022 forbids.
func upstreamCode(err error) string {
	if code := cemcode.CodeOf(err); code != "" {
		return code
	}
	var lrfErr *lrf.Error
	if errors.As(err, &lrfErr) {
		return lrfErr.Code
	}
	var coded interface{ Code() string }
	if errors.As(err, &coded) {
		return coded.Code()
	}
	var coded2 interface{ ErrorCode() string }
	if errors.As(err, &coded2) {
		return coded2.ErrorCode()
	}
	return ""
}

// verifierCodes is the closed allowlist of explicit public validation errors
// the CEM workflow and OCM 0.1 verifier are admitted to surface beside the
// stable verifier's own codes. CF-V0-022
// requires closed allowlists for the exact pinned upstream profile versions so
// that an unknown future code cannot pass through as if Frontier understood
// it; anything absent here falls to `frontier-internal-error`.
var verifierCodes = closedSet(
	// cem wire and map validation
	"invalid-json", "not-an-object", "missing-field", "unknown-field",
	"unsupported-spec", "invalid-field", "invalid-excluded-path", "path-traversal",
	"patch-digest-mismatch", "base-revision-mismatch", "fabricated-evidence-id",
	"uncited-hunk", "unsupported-without-basis", "unproven-mechanical",
	"orphan-evidence", "evidence-unavailable", "evidence-drift",
	// canonical patch derivation
	"binary-patch", "malformed-patch", "extra-hunk-payload", "diff-metadata-mismatch",
	"unsupported-tree-mode", "too-many-lines", "base-mismatch",
	// workflow and repository boundary
	"invalid-arguments", "expected-base-required", "target-required",
	"excluded-path-not-file", "excluded-artifact-mismatch",
	"unsupported-object-alternates", "repository-object-unavailable",
	"unsupported-repository-attributes", "map-unavailable", "patch-unavailable",
	"git-read-failed", "unknown-hunk-id", "missing-evidence",
	"invalid-span", "span-out-of-range", "invalid-line-range", "line-range-out-of-range",
	"git-diff-failed", "git-diff-timeout",
	"git-cancelled", "git-timeout", "git-output-exceeded", "git-budget-exceeded",
	"git-start-failed", "git-exit-failure",
	// ocm map verification
	"bootstrap-intent-not-unknown", "cem-map-digest-mismatch", "cem-patch-digest-mismatch",
	"claim-not-reextractable", "claim-obligation-mismatch", "duplicate-obligation",
	"duplicate-or-unsorted-claims", "duplicate-or-unsorted-reference", "duplicate-requirement",
	"fabricated-claim-id", "intent-scope-mismatch", "invalid-cem-digest", "invalid-claim-blob",
	"invalid-claim-extractor", "invalid-claims", "invalid-integer", "invalid-intent",
	"invalid-intent-blob", "invalid-linked", "invalid-object", "invalid-obligation-id",
	"invalid-obligations", "invalid-path", "invalid-reference-array", "invalid-reference-id",
	"invalid-requirement-prefix", "invalid-requirements-section", "invalid-selector",
	"invalid-span-digest", "invalid-target-revision", "invalid-unknown", "map-too-large",
	"missing-requirements", "noncanonical-map", "obligation-order-mismatch",
	"obligation-set-mismatch", "target-mismatch", "too-many-obligations",
	"unknown-claim-reference", "unknown-hunk-reference", "unsupported-ocm-python-claims",
)

// tcqPassthroughCodes is the closed allowlist of explicit public non-resource
// TCQ validation errors admitted by TCQ-V0-042. `tcq-resource-exhausted` is
// deliberately absent: CF-V0-022 maps it to `frontier-resource-exhausted`
// rather than passing it through.
var tcqPassthroughCodes = closedSet(
	"invalid-tcq-input", "unsupported-cem-profile", "unsupported-ocm-profile",
	"unsupported-claim-extractor", "invalid-command", "noncanonical-command",
	"command-target-mismatch", "command-cwd-unavailable", "invalid-observation",
	"noncanonical-observation", "observation-target-mismatch",
	"observation-command-mismatch", "report-digest-mismatch", "invalid-junit",
	"report-command-inconsistent", "invalid-tcq", "noncanonical-tcq",
)

func closedSet(codes ...string) map[string]bool {
	set := make(map[string]bool, len(codes))
	for _, code := range codes {
		set[code] = true
	}
	return set
}

// translateInterruption separates the two caught disruptions CF-V0-022 names:
// a timeout is resource exhaustion, a cancellation is an interruption for
// which the process remains able to render.
func translateInterruption(err error) *Error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fail(CodeResourceExhausted, "caught timeout")
	case errors.Is(err, context.Canceled):
		return fail(CodeInterrupted, "caught interruption")
	}
	return nil
}

// checkContext runs between cascade stages so a deadline or cancellation is
// caught and translated rather than surfacing as a half-built result.
func checkContext(ctx context.Context) *Error {
	if ctx == nil {
		return nil
	}
	return translateInterruption(ctx.Err())
}
