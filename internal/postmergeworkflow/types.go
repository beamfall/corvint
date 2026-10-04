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

// NativeProfile is the experimental intake/refusal route, not whole-workflow qualification.
const NativeProfile = "postmerge-replay/1"
const NativeManifestProfile = "postmerge-runtime-manifest/1"

type NativePolicy struct {
	Profile        string           `json:"profile"`
	Connector      connector.Policy `json:"connector"`
	Manifest       string           `json:"manifest"`
	ManifestSHA256 string           `json:"manifest_sha256"`
}
type Implementation struct {
	SourceCommit     string `json:"source_commit"`
	SourceTree       string `json:"source_tree"`
	ExecutableSHA256 string `json:"executable_sha256"`
}
type NativeManifest struct {
	Profile        string         `json:"profile"`
	Mode           string         `json:"mode"`
	Implementation Implementation `json:"implementation"`
	Product        struct {
		Base  string `json:"base"`
		Merge string `json:"merge"`
		Tree  string `json:"tree"`
	} `json:"product"`
	FixtureSHA256   string `json:"fixture_sha256"`
	ReaderCandidate struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"reader_candidate"`
	RetainedOutputRoot string `json:"retained_output_root"`
}

// ArtifactRef describes exact private bytes; it does not attest native execution.
type ArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type NativeStage struct {
	Name          string        `json:"name"`
	Disposition   string        `json:"disposition"`
	NativeProfile string        `json:"native_profile"`
	Inputs        []ArtifactRef `json:"inputs"`
	Output        *ArtifactRef  `json:"output"`
	Reasons       []string      `json:"reasons"`
}
type NativeReport struct {
	Profile                 string            `json:"profile"`
	Binding                 connector.Binding `json:"binding"`
	FixtureSHA256           string            `json:"fixture_sha256"`
	PolicySHA256            string            `json:"policy_sha256"`
	ManifestSHA256          string            `json:"manifest_sha256"`
	Implementation          Implementation    `json:"implementation"`
	Status                  string            `json:"status"`
	Reasons                 []string          `json:"reasons"`
	Stages                  []NativeStage     `json:"stages"`
	ComparisonStatus        string            `json:"comparison_status"`
	GeneratedMismatches     []Mismatch        `json:"generated_mismatches"`
	HumanVerifiedMismatches []Mismatch        `json:"human_verified_mismatches"`
	WorkflowQualification   string            `json:"workflow_qualification"`
}
