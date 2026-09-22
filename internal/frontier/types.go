package frontier

// Frozen vocabulary and shapes for `frontier/0`. Every constant below is
// authored from docs/specs/change-frontier-v0.md and is part of the wire; per
// CF-V0-029 none of it may be reinterpreted in place under this identifier.

// Profile identifiers and the single admitted policy (CF-V0-005, CF-V0-018).
const (
	Profile      = "frontier/0"
	ErrorProfile = "frontier-error/0"
	Policy       = "strict-v0"

	// ExcludedPath is the frozen CEM 0.2 sidecar exclusion. It is a literal in
	// both the universe preimage (CF-V0-006) and the emitted scope
	// (CF-V0-018), never copied from caller input.
	ExcludedPath = ".corvint/change.cem.json"
)

// Admitted upstream profiles (CF-V0-001). CEM 0.1, or OCM bound to CEM 0.1,
// is `unsupported-frontier-context`.
const (
	AdmittedCEMSpec = "cem/0.2"
	AdmittedOCMSpec = "ocm/0.1-experimental"
)

// Identity prefixes and domain separators (CF-V0-006, CF-V0-007, CF-V0-019).
const (
	universeIDPrefix = "frontier-universe:sha256:"
	itemIDPrefix     = "frontier-item:sha256:"
	documentIDPrefix = "frontier:sha256:"

	universeDomain = "corvint-frontier-universe/0"
	itemDomain     = "corvint-frontier-item/0"
	documentDomain = "corvint-frontier/0"
)

// FrontierState is the CF-V0-004 global state. There is no third state, and
// deliberately no stop-decision field: V0 has no hook authority (CF-V0-027).
const (
	StateEmpty = "EMPTY"
	StateOpen  = "OPEN"
)

// Item kinds (CF-V0-007).
const (
	KindHunkBasis    = "HUNK_BASIS"
	KindIntentChange = "INTENT_CHANGE"
	KindIntentTest   = "INTENT_TEST"
)

// kindOrder is the CF-V0-020 sort key. Items never sort by ID: identity is
// content-addressed and would scramble review order.
var kindOrder = map[string]int{KindHunkBasis: 0, KindIntentChange: 1, KindIntentTest: 2}

// Authority classes (CF-V0-017). `CALLER_REPORTED` is integrity-bound caller
// input with no independent execution authority root, which is exactly why
// CF-V0-014 forbids it from closing anything.
const (
	AuthorityNone             = "NONE"
	AuthorityProducerDeclared = "PRODUCER_DECLARED"
	AuthorityCallerReported   = "CALLER_REPORTED"
)

// Resolution classes (CF-V0-017).
const (
	ResolutionActionable        = "ACTIONABLE"
	ResolutionAuthorityRequired = "AUTHORITY_REQUIRED"
	ResolutionProfileRequired   = "PROFILE_REQUIRED"
)

// Reasons. HUNK_BASIS reasons come from CF-V0-009 and CF-V0-010, INTENT_CHANGE
// from CF-V0-011 and CF-V0-012, INTENT_TEST from the frozen CF-V0-016 table.
const (
	ReasonHunkNoEvidence           = "HUNK_NO_EVIDENCE"
	ReasonHunkInsufficientEvidence = "HUNK_INSUFFICIENT_EVIDENCE"
	ReasonHunkConflictingEvidence  = "HUNK_CONFLICTING_EVIDENCE"

	ReasonDeletionRelationRequired   = "DELETION_RELATION_REQUIRED"
	ReasonSubjectTermBoundExceeded   = "SUBJECT_TERM_BOUND_EXCEEDED"
	ReasonEvidenceSpanTooBroad       = "EVIDENCE_SPAN_TOO_BROAD"
	ReasonSelfReferentialBasis       = "SELF_REFERENTIAL_BASIS"
	ReasonInsufficientLexicalSupport = "INSUFFICIENT_LEXICAL_SUPPORT"

	ReasonObligationUnassessed           = "OBLIGATION_UNASSESSED"
	ReasonObligationNoTestClaim          = "OBLIGATION_NO_TEST_CLAIM"
	ReasonObligationInsufficientEvidence = "OBLIGATION_INSUFFICIENT_EVIDENCE"
	ReasonObligationConflictingEvidence  = "OBLIGATION_CONFLICTING_EVIDENCE"

	ReasonLexicalCandidateNonclosing = "LEXICAL_CANDIDATE_NONCLOSING"
	ReasonNoMaterialChangeWitness    = "NO_MATERIAL_CHANGE_WITNESS"

	ReasonCallerReportedNonclosing     = "CALLER_REPORTED_NONCLOSING"
	ReasonTargetCleanlinessNotAttested = "TARGET_CLEANLINESS_NOT_ATTESTED"
	ReasonCommandFailed                = "COMMAND_FAILED"
	ReasonTestError                    = "TEST_ERROR"
	ReasonTestFailed                   = "TEST_FAILED"
	ReasonTestSkipped                  = "TEST_SKIPPED"
	ReasonTestIdentityAmbiguous        = "TEST_IDENTITY_AMBIGUOUS"
	ReasonTestRowIdentityUnavailable   = "TEST_ROW_IDENTITY_UNAVAILABLE"
	ReasonTestNotMatched               = "TEST_NOT_MATCHED"
	ReasonTestClaimEmpty               = "TEST_CLAIM_EMPTY"
	ReasonTestClaimUnconditionalSkip   = "TEST_CLAIM_UNCONDITIONAL_SKIP"
	ReasonTestClaimUnassociated        = "TEST_CLAIM_UNASSOCIATED"
	ReasonTestClaimUnsupported         = "TEST_CLAIM_UNSUPPORTED"
)

// Next actions (CF-V0-009..012, CF-V0-016).
const (
	ActionSupplyHunkBasis           = "SUPPLY_HUNK_BASIS"
	ActionNarrowOrReplaceBasis      = "NARROW_OR_REPLACE_BASIS"
	ActionDefineSupportedProfile    = "DEFINE_SUPPORTED_PROFILE"
	ActionLinkObligation            = "LINK_OBLIGATION"
	ActionLinkMaterialHunk          = "LINK_MATERIAL_HUNK"
	ActionEstablishChangeWitness    = "ESTABLISH_CHANGE_WITNESS"
	ActionEstablishHarnessAuthority = "ESTABLISH_HARNESS_AUTHORITY"
	ActionRerunTestCommand          = "RERUN_TEST_COMMAND"
	ActionFixOrRerunTest            = "FIX_OR_RERUN_TEST"
	ActionDisambiguateTestIdentity  = "DISAMBIGUATE_TEST_IDENTITY"
	ActionSupplyTestObservation     = "SUPPLY_TEST_OBSERVATION"
	ActionRepairTestClaim           = "REPAIR_TEST_CLAIM"
)

// Frontier-owned limits (CF-V0-023). Counts are checked before append and byte
// arithmetic before output allocation, so exhaustion never leaves a partial
// result behind.
const (
	MaxHunkItems         = 2048
	MaxIntentChangeItems = 256
	MaxIntentTestItems   = 256
	MaxTotalItems        = 2560
	MaxRelatedIDsPerItem = 64
	MaxReasonsPerItem    = 32
	MaxOutputBytes       = 4194304
)

// Inherited raw ceilings applied at cascade step 2 (CF-V0-021). They are the
// upstream producers' own bounds, restated here only so the cheap check runs
// before any Git work.
const (
	maxCEMRawBytes = 4 << 20
	maxOCMRawBytes = 1 << 20
)

// Item is the closed CF-V0-017 shape. No field is optional and no field may be
// added under this profile; a stop-decision field in particular is refused by
// VerifyDocument as `noncanonical-frontier`.
type Item struct {
	AuthorityClass  string
	ID              string
	Kind            string
	NextAction      string
	Reasons         []string
	RelatedIDs      []string
	ResolutionClass string
	SubjectID       string
}

// Span is an intent byte span. Both offsets are emitted as canonical decimal
// strings (CF-V0-006, CF-V0-018), never as JSON numbers.
type Span struct {
	Start int64
	End   int64
}

// Scope is the CF-V0-018 `scope` block: the declared universe, restated in the
// result so a reader can rebind the identity without the original request.
type Scope struct {
	BaseRevision     string
	IntentBlobOID    string
	IntentPath       string
	IntentSpan       Span
	IntentSpanSHA256 string
	ObjectFormat     string
	PatchSHA256      string
	TargetRevision   string
}

// Inputs is the CF-V0-018 `inputs` block. Each digest binds exactly the bytes
// CF-V0-019 names: CEM and OCM hash their verified bounded raw copies, LRF
// hashes its complete canonical result bytes.
type Inputs struct {
	CEMSHA256 string
	LRFSHA256 string
	OCMSHA256 string
	TCQID     string
	TestMode  TestMode
}

// Document is one complete Frontier result. It has no stop-decision field by
// construction (CF-V0-004).
type Document struct {
	FrontierState string
	ID            string
	Inputs        Inputs
	Items         []Item
	Scope         Scope
	UniverseID    string
}

// ExitCode is the CF-V0-004 exit law: 0 valid empty, 1 valid open. Operational
// failure exits 2 and is signalled by an error, never by a Document.
func (d Document) ExitCode() int {
	if d.FrontierState == StateEmpty {
		return 0
	}
	return 1
}

// Obligation is one verified OCM 0.1 obligation row projected into Frontier.
// Fields mirror the OCM wire exactly (`ocm-v0-dogfood.md` OCM-V0-004): a
// linked obligation carries non-empty hunk and claim references, an unknown
// one carries empty references and one bounded reason.
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

// CEM dispositions consumed by CF-V0-008 and CF-V0-009.
const (
	CEMSupported  = "supported"
	CEMUnknown    = "unknown"
	CEMMechanical = "mechanical"
)

// Mechanical reasons that close a hunk once the verifier has reverified them
// (CF-V0-008). A `supported` disposition alone never closes a hunk.
var mechanicalReasons = map[string]bool{"whitespace-only": true, "line-ending-only": true}
