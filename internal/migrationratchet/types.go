// Package migrationratchet compares immutable migration evidence snapshots.
// A passing receipt is an incremental no-regression result, not proof of
// behavioral adequacy or migration completeness.
package migrationratchet

const (
	ProfileSchema  = "corvint-migration-evidence-ratchet/1"
	SnapshotSchema = "corvint-migration-evidence-snapshot/1"
	ReceiptSchema  = "corvint-migration-evidence-ratchet-receipt/1"
	MaxRecords     = 4096
	MaxBytes       = 4 << 20
)

type Profile struct {
	Schema        string             `json:"schema"`
	EvaluatedOn   string             `json:"evaluated_on"`
	Baseline      Snapshot           `json:"baseline"`
	Candidate     Snapshot           `json:"candidate"`
	MigrationRule *ComparabilityRule `json:"migration_rule,omitempty"`
}

type Snapshot struct {
	Schema         string            `json:"schema"`
	Repository     RepositoryBinding `json:"repository"`
	Policy         Policy            `json:"policy"`
	Provider       ProviderBinding   `json:"provider"`
	Complete       bool              `json:"complete"`
	Fresh          bool              `json:"fresh"`
	Records        []Record          `json:"records"`
	Exceptions     []Exception       `json:"exceptions"`
	ArtifactSHA256 string            `json:"artifact_sha256"`
}

type RepositoryBinding struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Tree     string `json:"tree"`
}

type ProviderBinding struct {
	ID     string `json:"id"`
	Schema string `json:"schema"`
	SHA256 string `json:"sha256"`
}

type Policy struct {
	ID                                  string      `json:"id"`
	SHA256                              string      `json:"sha256"`
	States                              []StateRule `json:"states"`
	ForbidNewLegacy                     bool        `json:"forbid_new_legacy"`
	RequireContractForNewOrChangedTests bool        `json:"require_contract_for_new_or_changed_tests"`
	TerminalStatesCannotRegress         bool        `json:"terminal_states_cannot_regress"`
	InvalidateEvidenceOnContentChange   bool        `json:"invalidate_evidence_on_content_change"`
	UnresolvedDenominatorCannotGrow     bool        `json:"unresolved_denominator_cannot_grow"`
}

type StateRule struct {
	Name       string `json:"name"`
	Rank       int    `json:"rank"`
	Terminal   bool   `json:"terminal"`
	Unresolved bool   `json:"unresolved"`
}

type Record struct {
	Kind          string `json:"kind"`
	ID            string `json:"id"`
	SHA256        string `json:"sha256"`
	ContentSHA256 string `json:"content_sha256"`
	State         string `json:"state"`
	Links         []Link `json:"links"`
}

type Link struct {
	Relation        string `json:"relation"`
	TargetKind      string `json:"target_kind"`
	TargetID        string `json:"target_id"`
	TargetSHA256    string `json:"target_sha256"`
	ReverseRelation string `json:"reverse_relation"`
}

type Exception struct {
	ID        string         `json:"id"`
	SHA256    string         `json:"sha256"`
	Scope     ExceptionScope `json:"scope"`
	Owner     string         `json:"owner"`
	Reviewers []string       `json:"reviewers"`
	Reason    string         `json:"reason"`
	ExpiresOn string         `json:"expires_on"`
}

type ExceptionScope struct {
	Delta       string `json:"delta"`
	Kind        string `json:"kind"`
	Identity    string `json:"identity"`
	DeltaSHA256 string `json:"delta_sha256"`
}

type ComparabilityRule struct {
	ID                      string            `json:"id"`
	SHA256                  string            `json:"sha256"`
	BaselineArtifactSHA256  string            `json:"baseline_artifact_sha256"`
	CandidateArtifactSHA256 string            `json:"candidate_artifact_sha256"`
	Owner                   string            `json:"owner"`
	Reviewers               []string          `json:"reviewers"`
	Reason                  string            `json:"reason"`
	Allows                  []string          `json:"allows"`
	Mappings                []IdentityMapping `json:"mappings"`
	Selections              []RecordSelection `json:"selections"`
}

type IdentityMapping struct {
	Kind        string `json:"kind"`
	BaselineID  string `json:"baseline_id"`
	CandidateID string `json:"candidate_id"`
}

type RecordSelection struct {
	Side   string `json:"side"`
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
}

type Binding struct {
	Schema         string            `json:"schema"`
	Repository     RepositoryBinding `json:"repository"`
	PolicyID       string            `json:"policy_id"`
	PolicySHA256   string            `json:"policy_sha256"`
	Provider       ProviderBinding   `json:"provider"`
	ArtifactSHA256 string            `json:"artifact_sha256"`
}

type Denominator struct {
	Total      int            `json:"total"`
	Unresolved int            `json:"unresolved"`
	ByKind     map[string]int `json:"by_kind"`
	ByState    map[string]int `json:"by_state"`
}

type Delta struct {
	Kind        string   `json:"kind"`
	BaselineID  string   `json:"baseline_id,omitempty"`
	CandidateID string   `json:"candidate_id,omitempty"`
	Code        string   `json:"code"`
	Details     []string `json:"details,omitempty"`
	ExceptedBy  string   `json:"excepted_by,omitempty"`
}

type StateDelta struct {
	Kind        string `json:"kind"`
	BaselineID  string `json:"baseline_id"`
	CandidateID string `json:"candidate_id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Direction   string `json:"direction"`
	ExceptedBy  string `json:"excepted_by,omitempty"`
}

type IdentityDelta struct {
	Kind        string   `json:"kind"`
	BaselineID  string   `json:"baseline_id,omitempty"`
	CandidateID string   `json:"candidate_id,omitempty"`
	Changes     []string `json:"changes"`
}

type Receipt struct {
	Schema               string          `json:"schema"`
	ProfileSHA256        string          `json:"profile_sha256"`
	EvaluatedOn          string          `json:"evaluated_on"`
	Baseline             Binding         `json:"baseline"`
	Candidate            Binding         `json:"candidate"`
	BaselineDenominator  Denominator     `json:"baseline_denominator"`
	CandidateDenominator Denominator     `json:"candidate_denominator"`
	Additions            []Delta         `json:"additions"`
	Removals             []Delta         `json:"removals"`
	ContentChanges       []Delta         `json:"content_changes"`
	StateChanges         []StateDelta    `json:"state_changes"`
	StaleEvidence        []Delta         `json:"stale_evidence"`
	BrokenReverseLinks   []Delta         `json:"broken_reverse_links"`
	Unknowns             []Delta         `json:"unknowns"`
	Identities           []IdentityDelta `json:"identities"`
	PolicyFailures       []string        `json:"policy_failures"`
	Verdict              string          `json:"verdict"`
	Limitations          []string        `json:"limitations"`
}
