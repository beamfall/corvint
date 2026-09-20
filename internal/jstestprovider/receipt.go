// Package jstestprovider is an experimental IPR-08 provider slice: one JS/TS
// unit adapter (Vitest) and one E2E adapter (Playwright) that each produce a
// receipt binding test/config/app-build/environment identity to per-test
// execution outcomes, then feed those facts through
// internal/testvalidity.Project (see ../testvalidity/projection.go). This
// package carries no qualification authority: a receipt states what was
// observed, never a "valid" verdict.
package jstestprovider

import "encoding/json"

// ExecutionState is the per-test outcome vocabulary this provider maps every
// reporter's own status field onto. It intentionally does not reuse
// testvalidity.Execution* verbatim: those are the shared LPCV/GLTP axis
// states, while this is the finer-grained JS/TS runner vocabulary the
// roadmap acceptance criteria name explicitly (flaky/retried, timedOut,
// interrupted, infrastructure) that a caller then folds into the shared axis
// via ToExecutionFacts.
type ExecutionState string

const (
	StatePassed         ExecutionState = "passed"
	StateFailed         ExecutionState = "failed"
	StateSkipped        ExecutionState = "skipped"
	StateFlaky          ExecutionState = "flaky"
	StateTimedOut       ExecutionState = "timedOut"
	StateInterrupted    ExecutionState = "interrupted"
	StateInfrastructure ExecutionState = "infrastructure"
)

// Anchor is one file:line source anchor taken from a reporter's own location
// field (Playwright) or parsed from its failure stack trace (Vitest, which
// does not emit a structured location field as of 5.0.0 - see
// evidence/ipr-08-runner-selection.md section 4/5 and vitest_test.go).
type Anchor struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// FailureArtifact is a bounded pointer to a failure artifact (trace,
// screenshot, error-context) - the path the reporter recorded, never the
// artifact's own content, which stays wherever the runner wrote it.
type FailureArtifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// TestOutcome is one reporter test/assertion result, normalized to this
// package's ExecutionState vocabulary.
type TestOutcome struct {
	ID             string            `json:"id,omitempty"`
	Project        *ProjectIdentity  `json:"project,omitempty"`
	Attempts       []Attempt         `json:"attempts,omitempty"`
	Name           string            `json:"name"`
	FullName       string            `json:"fullName"`
	State          ExecutionState    `json:"state"`
	Retries        int               `json:"retries"`
	DurationMS     float64           `json:"durationMs"`
	Anchor         *Anchor           `json:"anchor,omitempty"`
	FailureMessage string            `json:"failureMessage,omitempty"`
	Artifacts      []FailureArtifact `json:"artifacts,omitempty"`
}

// InfrastructureFailure records a run-level failure that never produced a
// per-test result: a bad command/config, a missing browser, a run the
// reporter itself reports as having found no tests. It is distinct from a
// per-test "failed" outcome (a real assertion or workflow failure) and from
// StateInfrastructure applied to an individual outcome whose message pattern
// still matches a known infrastructure cause (e.g. a missing browser
// executable surfacing inside an otherwise well-formed per-test result).
type InfrastructureFailure struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

// Identity is every binding fact a receipt commits to before evidence
// emits, per AGENTS.md invariant 1 and the roadmap acceptance line "bind
// test/configuration/application build and environment identities."
type Identity struct {
	ConfigInputDigests map[string]string `json:"configInputDigests,omitempty"`
	TestFileDigests    map[string]string `json:"testFileDigests"`
	ConfigFile         string            `json:"configFile"`
	ConfigDigest       string            `json:"configDigest"`
	PackageDigest      string            `json:"packageDigest"`
	NodeVersion        string            `json:"nodeVersion"`
	RunnerName         string            `json:"runnerName"`
	RunnerVersion      string            `json:"runnerVersion"`
	// Environment holds only the declared keys a caller asked to bind, never
	// the full process environment (AGENTS.md invariant 4 spirit: bounded,
	// explicit inputs, not incidental host state).
	Environment map[string]string `json:"environment"`
	Argv        []string          `json:"argv"`
}

// AppBuildIdentity is the served app's content identity: a digest of the
// configured build output directory, or "unknown" when no build directory
// was configured (Vitest unit runs, or an E2E run with no app dir bound).
type AppBuildIdentity struct {
	Digest  string `json:"digest"`
	Unknown bool   `json:"unknown"`
	Reason  string `json:"reason,omitempty"`
}

// Receipt is the one JS/TS provider output shape: identity, app-build
// freshness, per-test outcomes, and an optional run-level infrastructure
// failure. It carries no boolean "valid" summary; ToInput below projects it
// through the shared testvalidity axes instead.
type Receipt struct {
	Profile                 string                         `json:"profile,omitempty"`
	External                *ExternalLifecycle             `json:"external,omitempty"`
	ApplicationAttestation  *ApplicationAttestationReceipt `json:"applicationAttestation,omitempty"`
	TestRepositoryAtStart   *ApplicationRepositoryIdentity `json:"testRepositoryAtStart,omitempty"`
	TestRepositoryAtPublish *ApplicationRepositoryIdentity `json:"testRepositoryAtPublish,omitempty"`
	Kind                    string                         `json:"kind"` // "unit" | "e2e"
	Identity                Identity                       `json:"identity"`
	AppBuildAtStart         AppBuildIdentity               `json:"appBuildAtStart"`
	AppBuildAtPublish       AppBuildIdentity               `json:"appBuildAtPublish"`
	StaleAppBuild           bool                           `json:"staleAppBuild"`
	Tests                   []TestOutcome                  `json:"tests"`
	Infrastructure          *InfrastructureFailure         `json:"infrastructure,omitempty"`
	Cancelled               bool                           `json:"cancelled"`
	ServerDescendantsGone   *bool                          `json:"serverDescendantsGone,omitempty"`
}

// ProjectIdentity binds the resolved runtime configuration, not a device label
// inferred from the project name. Empty device labels remain explicit unknowns.
type ProjectIdentity struct {
	Name         string          `json:"name"`
	Browser      string          `json:"browser"`
	Device       string          `json:"device"`
	Use          json.RawMessage `json:"use"`
	ConfigDigest string          `json:"configDigest"`
}

type Attempt struct {
	State       ExecutionState `json:"state"`
	Retry       int            `json:"retry"`
	FailureKind string         `json:"failureKind"`
}

type ExternalLifecycle struct {
	ReadyURL              string `json:"readyUrl"`
	DeclaredAppIdentity   string `json:"declaredAppIdentity"`
	Ownership             string `json:"ownership"`
	CleanupResponsibility string `json:"cleanupResponsibility"`
	ServerDescendants     string `json:"serverDescendants"`
	ReadyAtStart          bool   `json:"readyAtStart"`
	ReadyAtPublish        bool   `json:"readyAtPublish"`
	RunnerDescendantsGone bool   `json:"runnerDescendantsGone"`
	InputsUnchanged       bool   `json:"inputsUnchanged"`
	ConfigOverride        string `json:"configOverride"`
}

type ApplicationAttestationReceipt struct {
	Provider    ApplicationAttestationProviderIdentity `json:"provider"`
	Expectation ApplicationAttestationExpectation      `json:"expectation"`
	Before      *ApplicationAttestationObservation     `json:"before,omitempty"`
	After       *ApplicationAttestationObservation     `json:"after,omitempty"`
	Failures    []string                               `json:"failures"`
}

type ApplicationAttestationProviderIdentity struct {
	Profile          string            `json:"profile"`
	Argv             []string          `json:"argv"`
	ExecutablePath   string            `json:"executablePath"`
	ExecutableDigest string            `json:"executableDigest"`
	ConfigPath       string            `json:"configPath"`
	ConfigDigest     string            `json:"configDigest"`
	Environment      map[string]string `json:"environment"`
}

type ApplicationAttestationExpectation struct {
	Repository    ApplicationRepositoryExpectation `json:"repository"`
	Build         ApplicationArtifactIdentity      `json:"build"`
	Configuration ApplicationArtifactIdentity      `json:"configuration"`
	InstanceKind  string                           `json:"instanceKind"`
}

type ApplicationRepositoryExpectation struct {
	RootCommit  string `json:"rootCommit"`
	Revision    string `json:"revision"`
	Tree        string `json:"tree"`
	DirtyPolicy string `json:"dirtyPolicy"`
	DirtyDigest string `json:"dirtyDigest,omitempty"`
}

type ApplicationAttestationObservation struct {
	OutputDigest string                 `json:"outputDigest"`
	Attestation  ApplicationAttestation `json:"attestation"`
}

type ApplicationAttestation struct {
	Profile       string                        `json:"profile"`
	Repository    ApplicationRepositoryIdentity `json:"repository"`
	Build         ApplicationArtifactIdentity   `json:"build"`
	Configuration ApplicationArtifactIdentity   `json:"configuration"`
	Instance      ApplicationInstanceIdentity   `json:"instance"`
	Health        ApplicationHealth             `json:"health"`
}

type ApplicationRepositoryIdentity struct {
	RootCommit  string `json:"rootCommit"`
	Revision    string `json:"revision"`
	Tree        string `json:"tree"`
	DirtyState  string `json:"dirtyState"`
	DirtyDigest string `json:"dirtyDigest,omitempty"`
}

type ApplicationArtifactIdentity struct {
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
}

type ApplicationInstanceIdentity struct {
	Kind            string `json:"kind"`
	ID              string `json:"id"`
	StartGeneration string `json:"startGeneration"`
}

type ApplicationHealth struct {
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}
