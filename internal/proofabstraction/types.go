// Package proofabstraction implements the experimental exact proof-projection
// relation described by PPA-V0. It proves only non-strengthening relative to a
// supplied source envelope; it does not establish source truth.
package proofabstraction

const (
	EnvelopeProfile    = "corvint-proof-state/0-experimental"
	RequestProfile     = "corvint-proof-preserving-abstraction-request/0-experimental"
	CertificateProfile = "corvint-proof-preserving-abstraction-certificate/0-experimental"
	AlgorithmProfile   = "corvint-exact-proof-projection/0"
	CertificateClaim   = "RELATIVE_NON_STRENGTHENING"

	MaxArtifactBytes = 4 << 20
	MaxClaims        = 2_048
	MaxEvidence      = 4_096
	MaxFrontier      = 4_096
	MaxClaimEvidence = 64
	MaxTokenBytes    = 256
)

type Verdict string

const (
	VerdictProved     Verdict = "PROVED"
	VerdictRefuted    Verdict = "REFUTED"
	VerdictConflicted Verdict = "CONFLICTED"
	VerdictUnknown    Verdict = "UNKNOWN"
)

type AuthorityClass string

const (
	AuthorityRepositoryAccepted AuthorityClass = "REPOSITORY_ACCEPTED"
	AuthorityOwningVerifier     AuthorityClass = "OWNING_VERIFIER"
	AuthorityProviderQualified  AuthorityClass = "PROVIDER_QUALIFIED"
	AuthorityAdapterQualified   AuthorityClass = "ADAPTER_QUALIFIED"
	AuthorityCallerReported     AuthorityClass = "CALLER_REPORTED"
	AuthorityAdvisory           AuthorityClass = "ADVISORY"
	AuthorityNone               AuthorityClass = "NONE"
)

type EvidenceClass string

const (
	EvidenceDeclared    EvidenceClass = "DECLARED"
	EvidenceImplemented EvidenceClass = "IMPLEMENTED"
	EvidenceVerified    EvidenceClass = "VERIFIED"
	EvidenceObserved    EvidenceClass = "OBSERVED"
	EvidenceInferred    EvidenceClass = "INFERRED"
)

type Completeness string

const (
	CompletenessComplete Completeness = "COMPLETE"
	CompletenessPartial  Completeness = "PARTIAL"
	CompletenessUnknown  Completeness = "UNKNOWN"
)

type FrontierKind string

const (
	FrontierSourceUnknown   FrontierKind = "SOURCE_UNKNOWN"
	FrontierSourceConflict  FrontierKind = "SOURCE_CONFLICT"
	FrontierSourceExclusion FrontierKind = "SOURCE_EXCLUSION"
	FrontierAbstractedClaim FrontierKind = "ABSTRACTED_CLAIM"
)

type Recovery string

const (
	RecoverySourceVerifier     Recovery = "SOURCE_VERIFIER"
	RecoveryLoadSourceEnvelope Recovery = "LOAD_SOURCE_ENVELOPE"
)

type ActionKind string

const (
	ActionRetain          ActionKind = "RETAIN"
	ActionDemoteToUnknown ActionKind = "DEMOTE_TO_UNKNOWN"
	ActionOmit            ActionKind = "OMIT"
)

type Reason string

const (
	ReasonAudience Reason = "AUDIENCE"
	ReasonBudget   Reason = "BUDGET"
)

type Relation string

const (
	RelationIdentical        Relation = "IDENTICAL"
	RelationDemotedToUnknown Relation = "DEMOTED_TO_UNKNOWN"
)

type EvidenceState string

const (
	EvidencePreserved EvidenceState = "PRESERVED"
	EvidenceOmitted   EvidenceState = "OMITTED"
)

type CompletenessRelation string

const (
	CompletenessEqual             CompletenessRelation = "EQUAL"
	CompletenessWeakenedToPartial CompletenessRelation = "WEAKENED_TO_PARTIAL"
)

type Repository struct {
	ObjectFormat string `json:"objectFormat"`
	Revision     string `json:"revision"`
	Tree         string `json:"tree"`
}

type Evidence struct {
	ID            string `json:"id,omitempty"`
	SourceProfile string `json:"sourceProfile"`
	ExternalID    string `json:"externalId"`
	ContentSHA256 string `json:"contentSha256"`
}

type Claim struct {
	ID              string          `json:"id,omitempty"`
	StatementSHA256 string          `json:"statementSha256"`
	ScopeSHA256     string          `json:"scopeSha256"`
	Critical        bool            `json:"critical"`
	Verdict         Verdict         `json:"verdict"`
	AuthorityClass  AuthorityClass  `json:"authorityClass"`
	EvidenceClasses []EvidenceClass `json:"evidenceClasses"`
	Evidence        []string        `json:"evidence"`
	CounterEvidence []string        `json:"counterEvidence"`
	UnknownReasons  []string        `json:"unknownReasons"`
}

type Frontier struct {
	ID               string       `json:"id,omitempty"`
	ClaimID          string       `json:"claimId"`
	Kind             FrontierKind `json:"kind"`
	Reason           string       `json:"reason"`
	SourceEnvelopeID *string      `json:"sourceEnvelopeId"`
	Recovery         Recovery     `json:"recovery"`
}

type Envelope struct {
	Profile       string       `json:"profile"`
	Repository    Repository   `json:"repository"`
	PolicySHA256  string       `json:"policySha256"`
	PurposeSHA256 string       `json:"purposeSha256"`
	Completeness  Completeness `json:"completeness"`
	Claims        []Claim      `json:"claims"`
	Evidence      []Evidence   `json:"evidence"`
	Frontier      []Frontier   `json:"frontier"`
	EnvelopeID    string       `json:"envelopeId,omitempty"`
}

type Action struct {
	ClaimID string     `json:"claimId"`
	Action  ActionKind `json:"action"`
	Reason  *Reason    `json:"reason"`
}

type Request struct {
	Profile          string   `json:"profile"`
	SourceEnvelopeID string   `json:"sourceEnvelopeId"`
	PurposeSHA256    string   `json:"purposeSha256"`
	Actions          []Action `json:"actions"`
}

type ClaimRelation struct {
	SourceClaimID      string   `json:"sourceClaimId"`
	DestinationClaimID string   `json:"destinationClaimId"`
	Relation           Relation `json:"relation"`
	Reason             *Reason  `json:"reason"`
}

type OmittedClaim struct {
	SourceClaimID string  `json:"sourceClaimId"`
	SourceVerdict Verdict `json:"sourceVerdict"`
	Reason        Reason  `json:"reason"`
	FrontierID    string  `json:"frontierId"`
}

type EvidenceAccounting struct {
	EvidenceID     string        `json:"evidenceId"`
	State          EvidenceState `json:"state"`
	SourceClaimIDs []string      `json:"sourceClaimIds"`
}

type FrontierAccounting struct {
	Preserved []string `json:"preserved"`
	Added     []string `json:"added"`
}

type Certificate struct {
	Profile               string               `json:"profile"`
	Algorithm             string               `json:"algorithm"`
	Claim                 string               `json:"claim"`
	SourceEnvelopeID      string               `json:"sourceEnvelopeId"`
	DestinationEnvelopeID string               `json:"destinationEnvelopeId"`
	RequestSHA256         string               `json:"requestSha256"`
	Repository            Repository           `json:"repository"`
	PolicySHA256          string               `json:"policySha256"`
	PurposeSHA256         string               `json:"purposeSha256"`
	CompletenessRelation  CompletenessRelation `json:"completenessRelation"`
	ClaimRelations        []ClaimRelation      `json:"claimRelations"`
	OmittedClaims         []OmittedClaim       `json:"omittedClaims"`
	EvidenceAccounting    []EvidenceAccounting `json:"evidenceAccounting"`
	FrontierAccounting    FrontierAccounting   `json:"frontierAccounting"`
	CertificateID         string               `json:"certificateId,omitempty"`
}
