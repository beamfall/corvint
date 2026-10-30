package tcq

// Error is one bounded operational failure. Per TCQ-V0-042 an operational error
// emits no partial artifact and no unverified path, OID, digest, selector, count,
// command argument, XML value, or derived identity — so the code alone is the
// payload and Message never carries input-derived text.
type Error struct {
	Code string
}

func (err *Error) Error() string { return err.Code }

func fail(code string) error { return &Error{Code: code} }

// The exact operational vocabulary TCQ adds on top of the inherited CEM/OCM,
// repository, and Git codes (TCQ-V0-042).
const (
	CodeInvalidInput               = "invalid-tcq-input"
	CodeUnsupportedCEMProfile      = "unsupported-cem-profile"
	CodeUnsupportedOCMProfile      = "unsupported-ocm-profile"
	CodeUnsupportedClaimExtractor  = "unsupported-claim-extractor"
	CodeInvalidCommand             = "invalid-command"
	CodeNoncanonicalCommand        = "noncanonical-command"
	CodeCommandTargetMismatch      = "command-target-mismatch"
	CodeCommandCwdUnavailable      = "command-cwd-unavailable"
	CodeInvalidObservation         = "invalid-observation"
	CodeNoncanonicalObservation    = "noncanonical-observation"
	CodeObservationTargetMismatch  = "observation-target-mismatch"
	CodeObservationCommandMismatch = "observation-command-mismatch"
	CodeReportDigestMismatch       = "report-digest-mismatch"
	CodeInvalidJUnit               = "invalid-junit"
	CodeReportCommandInconsistent  = "report-command-inconsistent"
	CodeInvalidTCQ                 = "invalid-tcq"
	CodeNoncanonicalTCQ            = "noncanonical-tcq"
	CodeResourceExhausted          = "tcq-resource-exhausted"

	// Inherited codes TCQ re-raises at its own stages (TCQ-V0-042 step 4/5).
	CodeExpectedBaseRequired = "expected-base-required"
	CodeTargetRequired       = "target-required"
	CodeBaseRevisionMismatch = "base-revision-mismatch"
	CodeTargetMismatch       = "target-mismatch"
	CodeObjectUnavailable    = "repository-object-unavailable"
	CodeNoncanonicalMap      = "noncanonical-map"
)
