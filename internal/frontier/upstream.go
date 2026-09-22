// Package frontier implements Change Frontier V0 (`frontier/0`), the deterministic
// composition of canonical CEM 0.2, OCM 0.1, Lexical Relevance Floor V0, and Test
// Claim Qualification V0 specified in docs/specs/change-frontier-v0.md.
//
// V0 is a local frontier preview and review queue. Per CF-V0-004 it has no
// stop-decision field and no hook authority, and per CF-V0-027 harness integration
// waits for a separately accepted harness-authority relation under a NEW profile
// identifier. Nothing in this package may be wired into the agent harness.
package frontier

// The declarations below are the frozen seam between this package and the upstream
// producers it consumes. They are authored from the specification clauses cited on
// each type — never transcribed from either runtime — as CF-V0-031 requires of any
// expectation over an authority-conferring surface.

// TCQResult is the `tcq/0` document Frontier recomputes under CF-V0-002. Its shape is
// the reference vector in docs/specs/test-claim-qualification-v0.md:375. Frontier
// reads only the fields declared here; the remaining document fields are bound by
// TCQID and never re-derived.
type TCQResult struct {
	// ID is the `tcq:sha256:` identity recorded in the Frontier `inputs` block
	// (CF-V0-018) and bound in both static and dynamic modes (CF-V0-003).
	ID string
	// Claims holds exactly one result per selected `(obligationId, claimId)` edge
	// (TCQ-V0-003). CF-V0-015 requires every selected edge to be evaluated.
	Claims []TCQClaimResult
}

// TCQClaimResult is one selected claim edge, per the reference vector at
// docs/specs/test-claim-qualification-v0.md:391.
type TCQClaimResult struct {
	ObligationID string
	ClaimID      string
	// Reasons are the TCQ diagnostics Frontier maps to `INTENT_TEST` reasons under
	// the exact CF-V0-016 table. The consumed vocabulary is closed:
	// target-cleanliness-not-attested, command-failed, test-error, test-failed,
	// test-skipped, execution-identity-ambiguous, repeated-test-rows,
	// row-identity-unavailable, test-not-matched, empty-body, unconditional-skip,
	// claim-association-missing, claim-association-ambiguous, python-offset-mismatch,
	// unparseable-test-unit, unsupported-anchor-profile, unsupported-python-grammar.
	// An unrecognised diagnostic is a closed-allowlist failure (CF-V0-022), never a
	// silently dropped reason.
	Reasons []string
	// Relation is the TCQ relation when one was emitted (`test-report-matched-v0`).
	// CF-V0-014 freezes it as non-closing: it maps to CALLER_REPORTED_NONCLOSING and
	// never removes an INTENT_TEST item, whatever the rows or exit status say.
	Relation string
	// AuthorityClass is `CALLER_REPORTED` for every TCQ V0 edge (CF-V0-015). It is
	// carried rather than assumed so a future producer cannot silently upgrade it.
	AuthorityClass string
}

// TestMode is the CF-V0-003 all-or-none dynamic-input discipline.
type TestMode string

const (
	// TestModeStatic is recorded when the (command, observation, report) tuple is
	// entirely absent.
	TestModeStatic TestMode = "STATIC"
	// TestModeDynamicCallerReported is recorded when all three are present. It does
	// not upgrade authority: CF-V0-015 keeps every edge CALLER_REPORTED in both modes.
	TestModeDynamicCallerReported TestMode = "DYNAMIC_CALLER_REPORTED"
)

// TCQRecomputer recomputes TCQ from verified inputs and target Git objects.
// CF-V0-002 forbids accepting a caller-supplied TCQ result as authority or as a
// shortcut, so Frontier holds this seam rather than a decoded document.
type TCQRecomputer interface {
	Recompute(request TCQRequest) (TCQResult, error)
}

// TCQRequest carries the verified bytes and revisions TCQ recomputes from. The
// dynamic tuple is all-or-none (CF-V0-003); a partial combination is
// `invalid-frontier-input` and is rejected before this call.
type TCQRequest struct {
	// CEMBytes is the canonical `cem/0.2` artifact the OCM binds. TCQ-V0-001
	// requires it alongside the OCM, and TCQ-V0-002 refuses `cem/0.1`
	// operationally, so the producer cannot source it for itself: an artifact it
	// fetched independently would not be the one this invocation verified.
	CEMBytes       []byte
	OCMBytes       []byte
	BaseRevision   string
	TargetRevision string
	Mode           TestMode
	Command        []byte
	Observation    []byte
	JUnitReport    []byte
}
