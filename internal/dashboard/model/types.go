// Package model compiles already-normalized dashboard adapter results into the
// canonical corvint-dashboard-snapshot/0 wire representation.
package model

const (
	SnapshotSchema                       = "corvint-dashboard-snapshot/0"
	LimitsProfile                        = "corvint-dashboard-limits/0"
	MaxSnapshotBytes                     = 4 << 20
	MaxConfiguredArtifacts               = 10_000
	MaxAggregateInputBytes        uint64 = 256 << 20
	MaxMetricSamples                     = 100_000
	ExpectedAdapterRegistrySHA256        = "sha256:2de98344235e7f432b1b486c14fa1023f9fdb08b41632bac04d3049592cf3bb3"
)

type Validity string

const (
	ValidityValid        Validity = "VALID"
	ValidityInvalid      Validity = "INVALID"
	ValidityNotPresent   Validity = "NOT_PRESENT"
	ValidityInaccessible Validity = "INACCESSIBLE"
	ValidityUnsupported  Validity = "UNSUPPORTED"
	ValidityDisabled     Validity = "DISABLED"
	ValidityExpired      Validity = "EXPIRED"
)

type EpistemicClass string

const (
	EpistemicObserved    EpistemicClass = "OBSERVED"
	EpistemicDeclared    EpistemicClass = "DECLARED"
	EpistemicAdvisory    EpistemicClass = "ADVISORY"
	EpistemicNotObserved EpistemicClass = "NOT_OBSERVED"
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

type Completeness string

const (
	CompletenessComplete Completeness = "COMPLETE"
	CompletenessPartial  Completeness = "PARTIAL"
	CompletenessUnknown  Completeness = "UNKNOWN"
)

type Currency string

const (
	CurrencyValidatedAt Currency = "VALIDATED_AT"
	CurrencyHistorical  Currency = "HISTORICAL"
	CurrencyStale       Currency = "STALE"
	CurrencyMixed       Currency = "MIXED"
	CurrencyUnknown     Currency = "UNKNOWN"
)

type DeliveryStage string

const (
	DeliveryAccepted     DeliveryStage = "ACCEPTED"
	DeliveryValidated    DeliveryStage = "VALIDATED"
	DeliveryImplemented  DeliveryStage = "IMPLEMENTED"
	DeliveryExperimental DeliveryStage = "EXPERIMENTAL"
	DeliveryNotStarted   DeliveryStage = "NOT_STARTED"
	DeliveryFailed       DeliveryStage = "FAILED"
	DeliveryUnsupported  DeliveryStage = "UNSUPPORTED"
)

type ScanState string

const (
	ScanComplete ScanState = "COMPLETE"
	ScanPartial  ScanState = "PARTIAL"
	ScanInvalid  ScanState = "INVALID"
)

type WorktreeState string

const (
	WorktreeClean   WorktreeState = "CLEAN"
	WorktreeMixed   WorktreeState = "MIXED"
	WorktreeUnknown WorktreeState = "UNKNOWN"
)

type ScopeClass string

const (
	ScopeSingleCohort         ScopeClass = "SINGLE_COHORT"
	ScopeMultiCohortInventory ScopeClass = "MULTI_COHORT_INVENTORY"
	ScopeUnavailable          ScopeClass = "UNAVAILABLE"
)

type ClockSource string

const (
	ClockProcess ClockSource = "PROCESS"
	ClockCaller  ClockSource = "CALLER"
)

type Severity string

const (
	SeverityInfo    Severity = "INFO"
	SeverityWarning Severity = "WARNING"
	SeverityError   Severity = "ERROR"
)

type AdapterRegistration struct {
	AdapterID        string        `json:"adapterId"`
	AcceptedProfiles []string      `json:"acceptedProfiles"`
	DefaultLocation  *string       `json:"defaultLocation"`
	DeliveryStage    DeliveryStage `json:"deliveryStage"`
	IssueCodes       []string      `json:"issueCodes"`
	MaxBytes         string        `json:"maxBytes"`
	SourceKind       string        `json:"sourceKind"`
	VerifierID       string        `json:"verifierId"`
}

type CohortIdentity struct {
	AdapterID              string  `json:"adapterId"`
	Profile                string  `json:"profile"`
	RepositoryObjectFormat *string `json:"repositoryObjectFormat"`
	SourceRevision         *string `json:"sourceRevision"`
	SourceTreeRevision     *string `json:"sourceTreeRevision"`
	DirtyPathsSHA256       *string `json:"dirtyPathsSha256"`
	ProducerIdentity       *string `json:"producerIdentity"`
	SourceObservationStart *string `json:"sourceObservationStart"`
	SourceObservationEnd   *string `json:"sourceObservationEnd"`
}

type Cohort struct {
	AdapterID              string  `json:"adapterId"`
	CohortID               string  `json:"cohortId"`
	DirtyPathsSHA256       *string `json:"dirtyPathsSha256"`
	ProducerIdentity       *string `json:"producerIdentity"`
	Profile                string  `json:"profile"`
	RepositoryObjectFormat *string `json:"repositoryObjectFormat"`
	SourceObservationEnd   *string `json:"sourceObservationEnd"`
	SourceObservationStart *string `json:"sourceObservationStart"`
	SourceRevision         *string `json:"sourceRevision"`
	SourceTreeRevision     *string `json:"sourceTreeRevision"`
}

type SourceInput struct {
	AdapterID           string
	ConfiguredOrdinal   string
	Profile             string
	AuthorityClass      AuthorityClass
	ByteCount           *string
	Completeness        Completeness
	ContentSHA256       *string
	Currency            Currency
	DeliveryStage       DeliveryStage
	EpistemicClass      EpistemicClass
	Exclusions          []string
	ObservationTime     *string
	RepositoryWitnesses []RepositoryWitness
	Validity            Validity
	VerifierID          string
	Cohorts             []CohortIdentity
	Members             *[]TraceMember
	ObservationEnd      *string
	ObservationStart    *string
}

type RepositoryWitness struct {
	Kind         string  `json:"kind"`
	ObjectFormat string  `json:"objectFormat"`
	ObjectID     string  `json:"objectId"`
	ObjectType   string  `json:"objectType"`
	Revision     *string `json:"revision"`
}

type TraceMember struct {
	Revision      string `json:"revision"`
	ContentSHA256 string `json:"contentSha256"`
	ByteCount     string `json:"byteCount"`
}

type Source struct {
	AdapterID             string         `json:"adapterId"`
	AuthorityClass        AuthorityClass `json:"authorityClass"`
	ByteCount             *string        `json:"byteCount"`
	CohortIDs             []string       `json:"cohortIds"`
	Completeness          Completeness   `json:"completeness"`
	ConfiguredOrdinal     string         `json:"configuredOrdinal"`
	ContentSHA256         *string        `json:"contentSha256"`
	Currency              Currency       `json:"currency"`
	DeliveryStage         DeliveryStage  `json:"deliveryStage"`
	DisplayLabel          string         `json:"displayLabel"`
	EpistemicClass        EpistemicClass `json:"epistemicClass"`
	Exclusions            []string       `json:"exclusions"`
	ID                    string         `json:"id"`
	Members               *[]TraceMember `json:"members"`
	ObservationEnd        *string        `json:"observationEnd"`
	ObservationStart      *string        `json:"observationStart"`
	ObservationTime       *string        `json:"observationTime"`
	Profile               string         `json:"profile"`
	RepositoryReadsSHA256 *string        `json:"repositoryReadsSha256"`
	Validity              Validity       `json:"validity"`
	VerifierID            string         `json:"verifierId"`
}

type Dimension struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

type Window struct {
	End   string `json:"end"`
	Start string `json:"start"`
}

type MetricInput struct {
	AuthorityClass AuthorityClass
	CohortIDs      []string
	Completeness   Completeness
	Currency       Currency
	Denominator    *string
	DeliveryStage  DeliveryStage
	Dimensions     []Dimension
	EpistemicClass EpistemicClass
	Exclusions     []string
	Name           string
	Numerator      *string
	ScopeClass     ScopeClass
	SourceIDs      []string
	Unit           string
	Validity       Validity
	Value          *string
	Window         *Window
}

type Metric struct {
	AuthorityClass AuthorityClass `json:"authorityClass"`
	CohortIDs      []string       `json:"cohortIds"`
	Completeness   Completeness   `json:"completeness"`
	Currency       Currency       `json:"currency"`
	Denominator    *string        `json:"denominator"`
	DeliveryStage  DeliveryStage  `json:"deliveryStage"`
	Dimensions     []Dimension    `json:"dimensions"`
	EpistemicClass EpistemicClass `json:"epistemicClass"`
	Exclusions     []string       `json:"exclusions"`
	Name           string         `json:"name"`
	Numerator      *string        `json:"numerator"`
	ScopeClass     ScopeClass     `json:"scopeClass"`
	SourceIDs      []string       `json:"sourceIds"`
	Unit           string         `json:"unit"`
	Validity       Validity       `json:"validity"`
	Value          *string        `json:"value"`
	Window         *Window        `json:"window"`
}

type IssueInput struct {
	Code     string
	Severity Severity
	SourceID *string
	Observed *string
	Limit    *string
}

type Issue struct {
	ID       string   `json:"id"`
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	SourceID *string  `json:"sourceId"`
	Observed *string  `json:"observed"`
	Limit    *string  `json:"limit"`
}

type ObservationInput struct {
	ClockSource ClockSource
	End         string
	ScanState   ScanState
	Start       string
}

type Observation struct {
	AdapterRegistrySHA256     string      `json:"adapterRegistrySha256"`
	ClockSource               ClockSource `json:"clockSource"`
	ConfiguredSourceSetSHA256 string      `json:"configuredSourceSetSha256"`
	End                       string      `json:"end"`
	LimitsProfile             string      `json:"limitsProfile"`
	ScanState                 ScanState   `json:"scanState"`
	Start                     string      `json:"start"`
}

type Repository struct {
	DirtyPathCount   *string       `json:"dirtyPathCount"`
	DirtyPathsSHA256 *string       `json:"dirtyPathsSha256"`
	HeadRevision     *string       `json:"headRevision"`
	ObjectFormat     *string       `json:"objectFormat"`
	TreeRevision     *string       `json:"treeRevision"`
	WorktreeState    WorktreeState `json:"worktreeState"`
}

type Privacy struct {
	Collection      string `json:"collection"`
	OutboundNetwork string `json:"outboundNetwork"`
	PathDisclosure  string `json:"pathDisclosure"`
	RawBodies       string `json:"rawBodies"`
	ThreatBoundary  string `json:"threatBoundary"`
}

type Snapshot struct {
	Schema         string      `json:"schema"`
	GeneratedAt    string      `json:"generatedAt"`
	Observation    Observation `json:"observation"`
	Repository     Repository  `json:"repository"`
	Cohorts        []Cohort    `json:"cohorts"`
	Sources        []Source    `json:"sources"`
	Data           []Metric    `json:"data"`
	Usage          []Metric    `json:"usage"`
	Verification   []Metric    `json:"verification"`
	Frontier       []Metric    `json:"frontier"`
	Harnesses      []Metric    `json:"harnesses"`
	Beamfall       []Metric    `json:"beamfall"`
	Privacy        Privacy     `json:"privacy"`
	Issues         []Issue     `json:"issues"`
	SnapshotSHA256 *string     `json:"snapshotSha256"`
}

type Input struct {
	GeneratedAt string
	Observation ObservationInput
	Repository  Repository
	Registry    []AdapterRegistration
	Sources     []SourceInput
	Metrics     []MetricInput
	Issues      []IssueInput
}

type Error struct {
	Code string
}

func (e *Error) Error() string { return e.Code }
