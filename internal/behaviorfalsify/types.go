// Package behaviorfalsify runs explicitly approved, bounded falsification
// controls for one exact browser-behavior criterion.
package behaviorfalsify

const (
	PlanSchema       = "corvint-browser-behavior-falsification-plan/0"
	InvocationSchema = "corvint-browser-behavior-control-invocation/0"
	HookSchema       = "corvint-browser-behavior-control-run/0"
	ReportSchema     = "corvint-browser-behavior-falsification-report/0"
	MarkerName       = ".corvint-disposable-browser-fixture"
)

type Status string

const (
	StatusKilled               Status = "killed"
	StatusSurvived             Status = "survived"
	StatusNotSupported         Status = "not_supported"
	StatusNotRun               Status = "not_run"
	StatusInfrastructureFailed Status = "infrastructure_failed"
	StatusInvalidControl       Status = "invalid_control"
)

type ControlKind string

const (
	WrongLocator          ControlKind = "wrong-locator"
	WrongExpectedValue    ControlKind = "wrong-expected-value"
	OmittedAssertion      ControlKind = "omitted-assertion"
	OmittedEvent          ControlKind = "omitted-event"
	ReorderedEvent        ControlKind = "reordered-event"
	WrongProject          ControlKind = "wrong-project"
	SuppressedPersistence ControlKind = "suppressed-persistence"
	OppositeBranch        ControlKind = "opposite-branch"
	ChangedFixtureValue   ControlKind = "changed-fixture-value"
)

type TargetIdentity struct {
	ContractID            string `json:"contract_id"`
	ContractSHA256        string `json:"contract_sha256"`
	CriterionID           string `json:"criterion_id"`
	AssertionID           string `json:"assertion_id"`
	ApplicationRevision   string `json:"application_revision"`
	TestRevision          string `json:"test_revision"`
	DocumentationRevision string `json:"documentation_revision"`
	ContractFile          string `json:"contract_file"`
	TestID                string `json:"test_id"`
	TestFile              string `json:"test_file"`
	TestLine              int    `json:"test_line"`
	TestTitle             string `json:"test_title"`
	Project               string `json:"project"`
}

type RunnerIdentity struct {
	Runner            string `json:"runner"`
	RunnerVersion     string `json:"runner_version"`
	Browser           string `json:"browser"`
	BrowserVersion    string `json:"browser_version"`
	ConfigFile        string `json:"config_file"`
	ConfigSHA256      string `json:"config_sha256"`
	EnvironmentSHA256 string `json:"environment_sha256"`
}

type RepositoryRoots struct {
	Application   string `json:"application"`
	Test          string `json:"test"`
	Documentation string `json:"documentation"`
}

type Command struct {
	Path             string `json:"path"`
	ExecutableSHA256 string `json:"executable_sha256"`
}

type ControlSpec struct {
	ID                string            `json:"id"`
	Kind              ControlKind       `json:"kind"`
	Disposition       string            `json:"disposition"`
	Definition        map[string]string `json:"definition"`
	Hook              *Command          `json:"hook,omitempty"`
	UnrelatedCriteria []string          `json:"unrelated_criteria"`
	RequiredSetup     []string          `json:"required_setup"`
}

type Request struct {
	Target           TargetIdentity  `json:"target"`
	Runner           RunnerIdentity  `json:"runner"`
	Controls         []ControlSpec   `json:"controls"`
	Cleanup          Command         `json:"cleanup"`
	DisposableRoot   string          `json:"disposable_root"`
	Repositories     RepositoryRoots `json:"repositories"`
	MarkerSHA256     string          `json:"marker_sha256"`
	DeclaredEnvKeys  []string        `json:"declared_env_keys"`
	Attempts         int             `json:"attempts"`
	TimeoutSeconds   int             `json:"timeout_seconds"`
	WallClockSeconds int             `json:"wall_clock_seconds"`
	ExternalState    string          `json:"external_state"`
}

type PlannedControl struct {
	ControlSpec
	Ordinal            int    `json:"ordinal"`
	PerturbationSHA256 string `json:"perturbation_sha256"`
}

type Plan struct {
	Schema          string           `json:"schema"`
	Request         Request          `json:"request"`
	Controls        []PlannedControl `json:"planned_controls"`
	WorkspaceSHA256 string           `json:"workspace_sha256"`
	Digest          string           `json:"digest"`
}

type Invocation struct {
	Schema     string         `json:"schema"`
	PlanDigest string         `json:"plan_digest"`
	Attempt    int            `json:"attempt"`
	Target     TargetIdentity `json:"target"`
	Runner     RunnerIdentity `json:"runner"`
	Control    PlannedControl `json:"control"`
}

type CriterionObservation struct {
	CriterionID string `json:"criterion_id"`
	AssertionID string `json:"assertion_id,omitempty"`
	State       string `json:"state"`
	FailureKind string `json:"failure_kind,omitempty"`
}

type SetupObservation struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type InfrastructureFailure struct {
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

type HookReceipt struct {
	Schema              string                 `json:"schema"`
	PlanDigest          string                 `json:"plan_digest"`
	PerturbationSHA256  string                 `json:"perturbation_sha256"`
	Target              TargetIdentity         `json:"target"`
	Runner              RunnerIdentity         `json:"runner"`
	Attempt             int                    `json:"attempt"`
	Retry               int                    `json:"retry"`
	NativeReceiptSHA256 string                 `json:"native_receipt_sha256"`
	TestOutcome         string                 `json:"test_outcome"`
	TargetObservation   CriterionObservation   `json:"target_observation"`
	Unrelated           []CriterionObservation `json:"unrelated"`
	Setup               []SetupObservation     `json:"setup"`
	Infrastructure      *InfrastructureFailure `json:"infrastructure,omitempty"`
	Artifacts           []Artifact             `json:"artifacts"`
}

type ProcessEvidence struct {
	Exit            int    `json:"exit"`
	Started         bool   `json:"started"`
	Completed       bool   `json:"completed"`
	Cancelled       bool   `json:"cancelled"`
	TimedOut        bool   `json:"timed_out"`
	Overflow        bool   `json:"overflow"`
	OwnedCleanup    bool   `json:"owned_cleanup"`
	DescendantsGone bool   `json:"descendants_gone"`
	StdoutSHA256    string `json:"stdout_sha256"`
	StderrSHA256    string `json:"stderr_sha256"`
	Error           string `json:"error,omitempty"`
}

type AttemptResult struct {
	Attempt           int             `json:"attempt"`
	Status            Status          `json:"status"`
	Reasons           []string        `json:"reasons"`
	Receipt           *HookReceipt    `json:"receipt,omitempty"`
	HookProcess       ProcessEvidence `json:"hook_process"`
	CleanupProcess    ProcessEvidence `json:"cleanup_process"`
	WorkspaceBefore   string          `json:"workspace_before"`
	WorkspaceAfter    string          `json:"workspace_after"`
	RetainedArtifacts []Artifact      `json:"retained_artifacts"`
}

type ControlResult struct {
	ID       string          `json:"id"`
	Kind     ControlKind     `json:"kind"`
	Ordinal  int             `json:"ordinal"`
	Status   Status          `json:"status"`
	Reasons  []string        `json:"reasons"`
	Attempts []AttemptResult `json:"attempts"`
}

type MutationScore struct {
	Defined     bool `json:"defined"`
	Killed      int  `json:"killed"`
	Denominator int  `json:"denominator"`
}

type Report struct {
	Schema             string          `json:"schema"`
	PlanDigest         string          `json:"plan_digest"`
	Results            []ControlResult `json:"results"`
	Counts             map[Status]int  `json:"counts"`
	Requested          int             `json:"requested"`
	Supported          int             `json:"supported"`
	Executed           int             `json:"executed"`
	CompleteVocabulary bool            `json:"complete_vocabulary"`
	MutationScore      MutationScore   `json:"mutation_score"`
	CoverageGaps       []string        `json:"coverage_gaps"`
	Fallback           string          `json:"fallback"`
	Limitations        []string        `json:"limitations"`
	Digest             string          `json:"digest"`
}

func cloneDefinition(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
