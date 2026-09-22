package proofabstraction

import "errors"

// Code is a stable verifier failure classification.
type Code string

const (
	Noncanonical             Code = "NONCANONICAL"
	LimitExceeded            Code = "LIMIT_EXCEEDED"
	SourceIDMismatch         Code = "SOURCE_ID_MISMATCH"
	SnapshotChanged          Code = "SNAPSHOT_CHANGED"
	PolicyChanged            Code = "POLICY_CHANGED"
	ClaimAdded               Code = "CLAIM_ADDED"
	ClaimIdentityChanged     Code = "CLAIM_IDENTITY_CHANGED"
	VerdictStrengthened      Code = "VERDICT_STRENGTHENED"
	AuthorityChanged         Code = "AUTHORITY_CHANGED"
	CriticalClaimWeakened    Code = "CRITICAL_CLAIM_WEAKENED"
	ConflictHidden           Code = "CONFLICT_HIDDEN"
	UnknownHidden            Code = "UNKNOWN_HIDDEN"
	EvidenceAdded            Code = "EVIDENCE_ADDED"
	EvidenceMutated          Code = "EVIDENCE_MUTATED"
	FrontierRemoved          Code = "FRONTIER_REMOVED"
	OmissionUnaccounted      Code = "OMISSION_UNACCOUNTED"
	CompletenessStrengthened Code = "COMPLETENESS_STRENGTHENED"
	CertificateMismatch      Code = "CERTIFICATE_MISMATCH"
)

// Error reports one fail-closed protocol or non-strengthening violation.
type Error struct {
	Code Code
}

func (e *Error) Error() string { return string(e.Code) }

func fail(code Code) error { return &Error{Code: code} }

// ErrorCode returns a stable failure code and false for unrelated errors.
func ErrorCode(err error) (Code, bool) {
	var target *Error
	if !errors.As(err, &target) {
		return "", false
	}
	return target.Code, true
}
