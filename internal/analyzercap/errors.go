package analyzercap

type Reason string

const (
	RegistryUntrusted               Reason = "REGISTRY_UNTRUSTED"
	RegistrySnapshotUnavailable     Reason = "REGISTRY_SNAPSHOT_UNAVAILABLE"
	LockUnavailable                 Reason = "LOCK_UNAVAILABLE"
	LockMismatch                    Reason = "LOCK_MISMATCH"
	NoCapabilityProvider            Reason = "NO_CAPABILITY_PROVIDER"
	AmbiguousProvider               Reason = "AMBIGUOUS_PROVIDER"
	BinaryDigestMismatch            Reason = "BINARY_DIGEST_MISMATCH"
	HostPlatformUnsupported         Reason = "HOST_PLATFORM_UNSUPPORTED"
	TargetPlatformUnsupported       Reason = "TARGET_PLATFORM_UNSUPPORTED"
	ExecutableIdentityUnsafe        Reason = "EXECUTABLE_IDENTITY_UNSAFE"
	ExecutableRace                  Reason = "EXECUTABLE_RACE"
	ProfileUnavailable              Reason = "PROFILE_UNAVAILABLE"
	CapabilityProjectionUnavailable Reason = "CAPABILITY_PROJECTION_UNAVAILABLE"
	SchemeMissing                   Reason = "SCHEME_MISSING"
	SchemeInvalid                   Reason = "SCHEME_INVALID"
	SchemeUnknown                   Reason = "SCHEME_UNKNOWN"
	SchemeFutureVersion             Reason = "SCHEME_FUTURE_VERSION"
	SchemeUnsupported               Reason = "SCHEME_UNSUPPORTED"
	NoCompatibleClause              Reason = "NO_COMPATIBLE_CLAUSE"
	AmbiguousClause                 Reason = "AMBIGUOUS_CLAUSE"
	TransitiveProfileUnavailable    Reason = "TRANSITIVE_PROFILE_UNAVAILABLE"
	FailureFeatureUnknown           Reason = "FEATURE_UNKNOWN"
	ProtocolEchoMismatch            Reason = "PROTOCOL_ECHO_MISMATCH"
	InputEvidenceMismatch           Reason = "INPUT_EVIDENCE_MISMATCH"
	AdmissionRejected               Reason = "ADMISSION_REJECTED"
	ExactVerifierNotRun             Reason = "EXACT_VERIFIER_NOT_RUN"
	ExactVerifierUnsupported        Reason = "EXACT_VERIFIER_UNSUPPORTED"
	LimitExceeded                   Reason = "LIMIT_EXCEEDED"
	Timeout                         Reason = "TIMEOUT"
	Cancelled                       Reason = "CANCELLED"
	AnalyzerFailure                 Reason = "ANALYZER_FAILURE"
)

type Failure struct {
	Reason Reason
}

func (f *Failure) Error() string { return string(f.Reason) }
func fail(reason Reason) error   { return &Failure{Reason: reason} }

func validReason(reason Reason) bool {
	switch reason {
	case RegistryUntrusted, RegistrySnapshotUnavailable, LockUnavailable, LockMismatch,
		NoCapabilityProvider, AmbiguousProvider, BinaryDigestMismatch, HostPlatformUnsupported,
		TargetPlatformUnsupported, ExecutableIdentityUnsafe, ExecutableRace, ProfileUnavailable,
		CapabilityProjectionUnavailable, SchemeMissing, SchemeInvalid, SchemeUnknown,
		SchemeFutureVersion, SchemeUnsupported, NoCompatibleClause, AmbiguousClause,
		TransitiveProfileUnavailable, FailureFeatureUnknown, ProtocolEchoMismatch,
		InputEvidenceMismatch, AdmissionRejected, ExactVerifierNotRun, ExactVerifierUnsupported,
		LimitExceeded, Timeout, Cancelled, AnalyzerFailure:
		return true
	default:
		return false
	}
}
