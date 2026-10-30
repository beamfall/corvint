// Package playwrightminimize plans bounded Playwright suite-interaction trials.
// The separate live profile requires explicit digest-bound execution approval.
package playwrightminimize

import (
	"context"
	"time"
)

const Schema = "corvint-playwright-suite-interaction/0"

type Outcome string

const (
	OutcomePassed Outcome = "passed"
	OutcomeFailed Outcome = "failed"
)

type FailureClass string

const (
	FailureAssertion          FailureClass = "assertion"
	FailureSynchronization    FailureClass = "synchronization"
	FailureFixture            FailureClass = "fixture"
	FailureProduct            FailureClass = "product"
	FailureInfrastructure     FailureClass = "infrastructure"
	FailureRestart            FailureClass = "restart"
	FailureResourceExhaustion FailureClass = "resource-exhaustion"
)

type TrialKind string

const (
	TrialReproduction TrialKind = "reproduce-original"
	TrialIsolation    TrialKind = "verify-isolated"
	TrialOrdered      TrialKind = "ordered-predecessor"
	TrialLoad         TrialKind = "unordered-load"
)

type FixedIdentity struct {
	TestRevision                 string `json:"test_revision"`
	ApplicationRevision          string `json:"application_revision"`
	ConfigDigest                 string `json:"config_digest"`
	Runner                       string `json:"runner"`
	RunnerVersion                string `json:"runner_version"`
	Browser                      string `json:"browser"`
	BrowserVersion               string `json:"browser_version"`
	Project                      string `json:"project"`
	FixtureSchema                string `json:"fixture_schema"`
	FixtureDigest                string `json:"fixture_digest"`
	SeedIdentity                 string `json:"seed_identity"`
	ApplicationAttestationDigest string `json:"application_attestation_digest"`
}

type WorkerTopology struct {
	Workers       int    `json:"workers"`
	FullyParallel bool   `json:"fully_parallel"`
	Shard         string `json:"shard,omitempty"`
	Policy        string `json:"policy"`
}

type RunIdentity struct {
	Fixed          FixedIdentity  `json:"fixed"`
	Order          []string       `json:"order"`
	WorkerTopology WorkerTopology `json:"worker_topology"`
}

type FailureObservation struct {
	Class          FailureClass `json:"class"`
	EvidenceDigest string       `json:"evidence_digest"`
	Summary        string       `json:"summary"`
}

type Baseline struct {
	ReceiptDigest string               `json:"receipt_digest"`
	Identity      RunIdentity          `json:"identity"`
	Outcome       Outcome              `json:"outcome"`
	Failures      []FailureObservation `json:"failures,omitempty"`
}

type Qualification struct {
	RunnerQualified          bool   `json:"runner_qualified"`
	RunnerReceiptDigest      string `json:"runner_receipt_digest,omitempty"`
	StabilityQualified       bool   `json:"stability_qualified"`
	StabilityReceiptDigest   string `json:"stability_receipt_digest,omitempty"`
	ApplicationAttested      bool   `json:"application_attested"`
	ApplicationReceiptDigest string `json:"application_receipt_digest,omitempty"`
}

type ResetPolicy struct {
	Name               string `json:"name"`
	Digest             string `json:"digest"`
	RestartApplication bool   `json:"restart_application"`
	ClearBrowserState  bool   `json:"clear_browser_state"`
}

type Limits struct {
	MaxTrials   int           `json:"max_trials"`
	WallClock   time.Duration `json:"wall_clock"`
	Repetitions int           `json:"repetitions"`
}

type Request struct {
	Target          string         `json:"target"`
	Predecessors    []string       `json:"predecessors"`
	LoadMembers     []string       `json:"load_members"`
	OriginalFailure Baseline       `json:"original_failure"`
	IsolatedPass    Baseline       `json:"isolated_pass"`
	LoadTopology    WorkerTopology `json:"load_topology"`
	ResetPolicy     ResetPolicy    `json:"reset_policy"`
	Limits          Limits         `json:"limits"`
	Qualification   Qualification  `json:"qualification"`
}

type Trial struct {
	ID          string      `json:"id"`
	Ordinal     int         `json:"ordinal"`
	CandidateID string      `json:"candidate_id"`
	Repetition  int         `json:"repetition"`
	Kind        TrialKind   `json:"kind"`
	Context     []string    `json:"context"`
	Identity    RunIdentity `json:"identity"`
	ResetPolicy ResetPolicy `json:"reset_policy"`
}

type Plan struct {
	Schema          string   `json:"schema"`
	Request         Request  `json:"request"`
	Trials          []Trial  `json:"trials"`
	OrderedComplete bool     `json:"ordered_complete"`
	LoadComplete    bool     `json:"load_complete"`
	Complete        bool     `json:"complete"`
	Blockers        []string `json:"blockers,omitempty"`
	Digest          string   `json:"digest"`
}

type EvidenceRef struct {
	Digest string `json:"digest"`
	Detail string `json:"detail"`
}

type TrialEvidence struct {
	Setup          []EvidenceRef `json:"setup"`
	PageAssertions []EvidenceRef `json:"page_assertions"`
	Retries        []EvidenceRef `json:"retries"`
	Cleanup        []EvidenceRef `json:"cleanup"`
	ServerHealth   []EvidenceRef `json:"server_health"`
	Resources      []EvidenceRef `json:"resources"`
}

type TrialReceipt struct {
	Live                          *LiveTrialEvidence   `json:"live,omitempty"`
	Digest                        string               `json:"digest"`
	TrialID                       string               `json:"trial_id"`
	Identity                      RunIdentity          `json:"identity"`
	ResetPolicy                   ResetPolicy          `json:"reset_policy"`
	ResetSucceeded                bool                 `json:"reset_succeeded"`
	CleanupSucceeded              bool                 `json:"cleanup_succeeded"`
	ApplicationAttestationStart   string               `json:"application_attestation_start"`
	ApplicationAttestationPublish string               `json:"application_attestation_publish"`
	Outcome                       Outcome              `json:"outcome"`
	Evidence                      TrialEvidence        `json:"evidence"`
	Failures                      []FailureObservation `json:"failures,omitempty"`
}

type Authorization struct {
	OperatorApproved bool   `json:"operator_approved"`
	PlanDigest       string `json:"plan_digest"`
}

type Runner interface {
	Run(context.Context, Trial) (TrialReceipt, error)
}

type RunnerFunc func(context.Context, Trial) (TrialReceipt, error)

func (f RunnerFunc) Run(ctx context.Context, trial Trial) (TrialReceipt, error) {
	return f(ctx, trial)
}

type TrialResult struct {
	Trial          Trial         `json:"trial"`
	Receipt        *TrialReceipt `json:"receipt,omitempty"`
	Valid          bool          `json:"valid"`
	InvalidReasons []string      `json:"invalid_reasons,omitempty"`
	ExecutionError string        `json:"execution_error,omitempty"`
}

type ObservedFailure struct {
	TrialID       string             `json:"trial_id"`
	ReceiptDigest string             `json:"receipt_digest"`
	Observation   FailureObservation `json:"observation"`
}

type MemberAssessment struct {
	Member string `json:"member"`
	Status string `json:"status"`
}

type Finding struct {
	Kind           TrialKind          `json:"kind"`
	Smallest       []string           `json:"smallest"`
	Members        []MemberAssessment `json:"members"`
	ReceiptDigests []string           `json:"receipt_digests"`
	SearchComplete bool               `json:"search_complete"`
	Minimality     string             `json:"minimality"`
}

type Report struct {
	Schema           string            `json:"schema"`
	PlanDigest       string            `json:"plan_digest"`
	Status           string            `json:"status"`
	Diagnosis        string            `json:"diagnosis"`
	Confident        bool              `json:"confident"`
	Blockers         []string          `json:"blockers,omitempty"`
	Trials           []TrialResult     `json:"trials"`
	ObservedFailures []ObservedFailure `json:"observed_failures"`
	Ordered          *Finding          `json:"ordered,omitempty"`
	Load             *Finding          `json:"load,omitempty"`
	Limitations      []string          `json:"limitations"`
}
