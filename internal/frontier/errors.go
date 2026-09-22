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
// the pinned CEM 0.2 / OCM 0.1 verifier is admitted to surface. CF-V0-022
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

// lrfPassthroughCodes is the closed LRF allowlist. CF-V0-022 admits exactly
// one explicit LRF code; `invalid-lrf-request` is deliberately absent, because
// Frontier builds the LRF request itself and a rejection of it is a Frontier
// internal error, not a caller-visible LRF outcome.
var lrfPassthroughCodes = closedSet("unsupported-lrf-context")

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

// translateVerifier maps one shared-verifier failure. CF-V0-021 gives native
// CEM/OCM precedence priority over every later Frontier check, so an admitted
// code is emitted exactly as the verifier declared it.
func translateVerifier(err error) *Error {
	var owned *Error
	if errors.As(err, &owned) {
		return owned
	}
	if interrupted := translateInterruption(err); interrupted != nil {
		return interrupted
	}
	code := upstreamCode(err)
	if verifierCodes[code] {
		return fail(code, "inherited verifier code")
	}
	return fail(CodeInternalError, "unadmitted verifier outcome")
}

// translateLRF maps one LRF recomputation failure. The complete bound document
// is handled by the caller, because a bound result is a normal LRF return
// value rather than an error.
func translateLRF(err error) *Error {
	if interrupted := translateInterruption(err); interrupted != nil {
		return interrupted
	}
	code := upstreamCode(err)
	if lrfPassthroughCodes[code] {
		return fail(code, "inherited lrf code")
	}
	return fail(CodeInternalError, "unadmitted lrf outcome")
}

// translateTCQ maps one TCQ recomputation failure under the TCQ-V0-042 closed
// allowlist.
func translateTCQ(err error) *Error {
	if interrupted := translateInterruption(err); interrupted != nil {
		return interrupted
	}
	code := upstreamCode(err)
	if code == "tcq-resource-exhausted" {
		return fail(CodeResourceExhausted, "tcq resource exhaustion")
	}
	if tcqPassthroughCodes[code] {
		return fail(code, "inherited tcq code")
	}
	return fail(CodeInternalError, "unadmitted tcq outcome")
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
