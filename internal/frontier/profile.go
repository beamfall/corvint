// Package frontier holds the frontier-stable-view/0 projection (V1-1304), the
// stable successor of Change Frontier V0. The frontier/0 engine, which
// composed cem/0.2, lrf/0 and tcq/0 into a frontier document, was deleted with
// the cem/0.x wire (decision 0525 §1, CEM-V1-217): a map that does not declare
// cem/1.0 is refused before it is read, and a cem/1.0 map yields the stable
// view, whose frontier state is always NOT_PRODUCED.
//
// The view has no stop-decision field and no hook authority (CF-V0-004,
// CF-V0-027). Nothing in this package may be wired into the agent harness.
package frontier

// ErrorProfile is the bounded stderr envelope profile (CF-V0-022). It names
// the operational failure of the stable view as it named frontier/0's.
const ErrorProfile = "frontier-error/0"

// ExcludedPath is the committed change-map sidecar path every CEM producer
// excludes from its own patch.
const ExcludedPath = ".corvint/change.cem.json"

// AdmittedOCMSpec is the one OCM profile the stable view admits; the OCM must
// also bind cem/1.0.
const AdmittedOCMSpec = "ocm/0.1-experimental"

// MaxOutputBytes bounds the encoded view (CF-V0-023).
const MaxOutputBytes = 4194304

// Inherited raw ceilings, checked before any Git work and reported with the
// upstream producers' own codes.
const (
	maxCEMRawBytes = 4 << 20
	maxOCMRawBytes = 1 << 20
)

// TestMode is the CF-V0-003 all-or-none dynamic-input discipline.
type TestMode string

const (
	// TestModeStatic is recorded when the (command, observation, report) tuple
	// is entirely absent.
	TestModeStatic TestMode = "STATIC"
	// TestModeDynamicCallerReported is recorded when all three are present. It
	// does not upgrade authority: every edge stays CALLER_REPORTED.
	TestModeDynamicCallerReported TestMode = "DYNAMIC_CALLER_REPORTED"
)

// Obligation is one verified OCM obligation row, carried field for field
// (OCM-V0-004): a linked obligation carries non-empty hunk and claim
// references, an unknown one empty references and one bounded reason.
type Obligation struct {
	ID          string
	Disposition string
	Reason      string
	HunkIDs     []string
	ClaimIDs    []string
}

// OCM dispositions (OCM-V0-004).
const (
	OCMLinked  = "linked"
	OCMUnknown = "unknown"
)

// TCQClaimResult is one selected tcq-stable/0 claim edge as the stable
// upstream carries it.
type TCQClaimResult struct {
	ObligationID   string
	ClaimID        string
	Reasons        []string
	Relation       string
	AuthorityClass string
}
