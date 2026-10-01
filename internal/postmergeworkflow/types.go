// Package postmergeworkflow implements an experimental credential-free replay
// harness. Adapter observations are evidence to compare, not workflow authority.
package postmergeworkflow

import connector "github.com/Beamfall/corvint/internal/postmergeconnector"

const Profile = "postmerge-replay/0"
const MaxBytes = 4 << 20

type Label struct {
	Basis          string `json:"basis"` // generated | human-verified
	EvidenceSHA256 string `json:"evidence_sha256"`
	Human          string `json:"human"`
	Approval       string `json:"approval"`
}
type Expected struct {
	AffectedFlows []string            `json:"affected_flows"`
	Followup      bool                `json:"followup"`
	TestGaps      []string            `json:"test_gaps"`
	Defects       []connector.Finding `json:"defects"`
	Labels        map[string]Label    `json:"labels"`
}
type Fixture struct {
	Profile    string            `json:"profile"`
	Connector  connector.Fixture `json:"connector"`
	SourceItem string            `json:"source_item"`
	Expected   Expected          `json:"expected"`
}

// Policy is a separate host-owned input. The fixture cannot choose an executable,
// arguments, connector binding, runtime configuration or human label registry.
type Policy struct {
	Profile          string           `json:"profile"`
	Connector        connector.Policy `json:"connector"`
	Executable       string           `json:"executable"`
	ExecutableSHA256 string           `json:"executable_sha256"`
	Args             []string         `json:"args"`
	RuntimeSHA256    string           `json:"runtime_sha256"`
	Registry         string           `json:"registry"`
	RegistrySHA256   string           `json:"registry_sha256"`
}
type Approval struct {
	Binding     connector.Binding `json:"binding"`
	Field       string            `json:"field"`
	ValueSHA256 string            `json:"value_sha256"`
	Label       Label             `json:"label"`
}
type Registry struct {
	Profile   string     `json:"profile"`
	Approvals []Approval `json:"approvals"`
}
type Request struct {
	Profile       string            `json:"profile"`
	Binding       connector.Binding `json:"binding"`
	FixtureSHA256 string            `json:"fixture_sha256"`
	RuntimeSHA256 string            `json:"runtime_sha256"`
}
type Stage struct {
	Name           string `json:"name"`
	Status         string `json:"status"` // observed | blocked | not-applicable | deferred
	ArtifactSHA256 string `json:"artifact_sha256"`
}
type Result struct {
	Request
	AffectedFlows        []string            `json:"affected_flows"`
	DocumentationTargets []string            `json:"documentation_targets"`
	TestGaps             []string            `json:"test_gaps"`
	Defects              []connector.Finding `json:"defects"`
	Input                connector.Input     `json:"connector_input"`
	Stages               []Stage             `json:"stages"`
}
type Mismatch struct {
	Field          string `json:"field"`
	Basis          string `json:"basis"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ObservedSHA256 string `json:"observed_sha256"`
}
type Report struct {
	Profile       string            `json:"profile"`
	Binding       connector.Binding `json:"binding"`
	FixtureSHA256 string            `json:"fixture_sha256"`
	PolicySHA256  string            `json:"policy_sha256"`
	Status        string            `json:"status"` // MATCH | MISMATCH | BLOCKED
	Reasons       []string          `json:"reasons"`
	// Mismatches are reported separately by the basis of the expectation
	// they contradict; a generated expectation never counts as human-verified.
	HumanVerifiedMismatches []Mismatch `json:"human_verified_mismatches"`
	GeneratedMismatches     []Mismatch `json:"generated_mismatches"`
	DeferredStages          []string   `json:"deferred_stages"`
	RecordingSHA256         string     `json:"recording_sha256"`
	Recording               string     `json:"recording_jsonl"`
	WorkflowQualification   string     `json:"workflow_qualification"`
	Limits                  []string   `json:"limits"`
}
