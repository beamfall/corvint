// Package doccorpus compiles the proposed DCP-V1 documentation evidence profile.
// Identity validation does not establish semantic truth or accepted intent.
package doccorpus

import "github.com/Beamfall/corvint/internal/testvaliditydoc"

const (
	Schema         = "corvint-evidence-corpus/1"
	ManifestSchema = "corvint-corpus-input/1"
	ProviderSchema = "corvint-corpus-provider/1"
	ReceiptSchema  = "corvint-corpus-receipt/1"
	MaxBytes       = 4 << 20
	MaxRecords     = 4096
	MaxResults     = 256
)

type Repository struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}
type Profile struct {
	ID       string   `json:"id"`
	Revision string   `json:"revision"`
	Title    string   `json:"title"`
	Format   string   `json:"format"`
	Groups   []string `json:"groups"`
}
type Input struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
	Blob     string `json:"blob"`
	SHA256   string `json:"sha256"`
	Provider string `json:"provider"`
	Purpose  string `json:"purpose"`
}
type Scope struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
}
type Provider struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Version  string `json:"version"`
	Revision string `json:"revision"`
	Record   string `json:"record"`
}
type Manifest struct {
	Schema     string     `json:"schema"`
	Repository Repository `json:"repository"`
	BuiltAt    string     `json:"built_at"`
	Profile    Profile    `json:"profile"`
	Scopes     []Scope    `json:"scopes"`
	Inputs     []Input    `json:"inputs"`
	Providers  []Provider `json:"providers"`
	MergeRule  string     `json:"merge_rule"`
}
type Builder struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	Engine   string `json:"engine"`
}
type Anchor struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	SHA256     string `json:"sha256"`
	Start      int    `json:"start_line"`
	End        int    `json:"end_line"`
	SpanSHA256 string `json:"span_sha256"`
	Symbol     string `json:"symbol"`
	Region     string `json:"region"`
	Authority  string `json:"authority"`
	Kind       string `json:"evidence_kind"`
	Reason     string `json:"reason"`
}
type Evidence struct {
	ReportedTrust string   `json:"reported_trust"`
	Derivation    string   `json:"derivation"`
	Trust         string   `json:"trust"`
	State         string   `json:"state"`
	Freshness     string   `json:"freshness"`
	Anchors       []Anchor `json:"anchors"`
	Unknown       string   `json:"unknown"`
	Limitations   []string `json:"limitations"`
}
type Subject struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	Evidence Evidence `json:"evidence"`
}
type Claim struct {
	ID       string   `json:"id"`
	Subject  string   `json:"subject"`
	Text     string   `json:"text"`
	Provider string   `json:"provider"`
	Evidence Evidence `json:"evidence"`
}
type Relation struct {
	ID       string   `json:"id"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Type     string   `json:"type"`
	Provider string   `json:"provider"`
	Evidence Evidence `json:"evidence"`
}
type Step struct {
	ID          string   `json:"id"`
	Action      string   `json:"action"`
	Operation   string   `json:"operation"`
	Expected    string   `json:"expected"`
	Observation string   `json:"observation"`
	Evidence    Evidence `json:"evidence"`
}
type Journey struct {
	ID            string   `json:"id"`
	Subject       string   `json:"subject"`
	Provider      string   `json:"provider"`
	Status        string   `json:"status"`
	Preconditions []string `json:"preconditions"`
	Cleanup       string   `json:"cleanup"`
	Steps         []Step   `json:"steps"`
	Evidence      Evidence `json:"evidence"`
}

// ObservationLink refers to a retained native receipt by immutable input path.
// Its package/name join is exact, and StepEvidence proves only the provider's
// recorded expected observation, not an independently assessed assertion.
type ObservationLink struct {
	TestID         string            `json:"test_id,omitempty"`
	Project        string            `json:"project,omitempty"`
	StepInput      string            `json:"step_input"`
	StepRevision   string            `json:"step_revision"`
	SourcePaths    map[string]string `json:"source_paths"`
	ID             string            `json:"id"`
	Subject        string            `json:"subject"`
	Input          string            `json:"input"`
	InputRevision  string            `json:"input_revision"`
	SourceRevision string            `json:"source_revision"`
	RunID          string            `json:"run_id"`
	Package        string            `json:"package"`
	Test           string            `json:"test"`
	StepEvidence   []Anchor          `json:"step_evidence"`
}
type Observation struct {
	Trust       string                   `json:"trust"`
	Limitations []string                 `json:"limitations"`
	Link        ObservationLink          `json:"link"`
	InputSHA256 string                   `json:"input_sha256"`
	Document    testvaliditydoc.Document `json:"document"`
}
type CapabilityDeclaration struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}
type ProviderRecord struct {
	BehaviorContracts *BehaviorRegistry       `json:"behavior_contracts,omitempty"`
	Schema            string                  `json:"schema"`
	ID                string                  `json:"id"`
	Version           string                  `json:"version"`
	Source            Repository              `json:"source"`
	Subjects          []Subject               `json:"subjects"`
	Claims            []Claim                 `json:"claims"`
	Relations         []Relation              `json:"relations"`
	Journeys          []Journey               `json:"journeys"`
	Observations      []ObservationLink       `json:"observations"`
	Capabilities      []CapabilityDeclaration `json:"capabilities"`
}
type Capability struct {
	Name        string   `json:"name"`
	State       string   `json:"state"`
	Reason      string   `json:"reason"`
	Records     []string `json:"records"`
	Providers   []string `json:"providers"`
	Tools       []string `json:"tools"`
	Count       int      `json:"count"`
	Denominator int      `json:"denominator"`
	Rule        string   `json:"rule"`
}
type Gap struct {
	Subject string `json:"subject"`
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
}
type Artifact struct {
	BehaviorContracts []BehaviorReport `json:"behavior_contracts,omitempty"`
	Schema            string           `json:"schema"`
	Builder           Builder          `json:"builder"`
	Manifest          Manifest         `json:"manifest"`
	ManifestSHA256    string           `json:"manifest_sha256"`
	ProfileSHA256     string           `json:"profile_sha256"`
	Tree              string           `json:"tree"`
	Subjects          []Subject        `json:"subjects"`
	Claims            []Claim          `json:"claims"`
	Relations         []Relation       `json:"relations"`
	Journeys          []Journey        `json:"journeys"`
	Observations      []Observation    `json:"observations"`
	Capabilities      []Capability     `json:"capabilities"`
	Gaps              []Gap            `json:"gaps"`
	SHA256            string           `json:"sha256"`
}
type Request struct {
	Operation string `json:"operation"`
	Query     string `json:"query"`
	ID        string `json:"id"`
	Path      string `json:"path"`
	Limit     int    `json:"limit"`
}
type Receipt struct {
	Schema         string       `json:"schema"`
	Operation      string       `json:"operation"`
	ArtifactSHA256 string       `json:"artifact_sha256"`
	Repository     Repository   `json:"repository"`
	Tree           string       `json:"tree"`
	Trust          string       `json:"trust"`
	Freshness      string       `json:"freshness"`
	State          string       `json:"state"`
	Miss           string       `json:"miss"`
	Results        []any        `json:"results"`
	Citations      []Anchor     `json:"citations"`
	Capabilities   []Capability `json:"capabilities"`
	Limitations    []string     `json:"limitations"`
	Omitted        int          `json:"omitted"`
}
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string  { return e.Code + ": " + e.Message }
func fail(message string) error { return &Error{Code: "corpus-refused", Message: message} }

// JourneyRun is independently retained step observation data. A native passing
// test alone cannot populate these ordered expected/observed records.
type JourneyRun struct {
	Schema         string            `json:"schema"`
	RunSHA256      string            `json:"run_sha256"`
	SourceRevision string            `json:"source_revision"`
	Package        string            `json:"package"`
	Test           string            `json:"test"`
	Cleanup        string            `json:"cleanup"`
	Steps          []StepObservation `json:"steps"`
}
type StepObservation struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Operation string `json:"operation"`
	Expected  string `json:"expected"`
	Observed  string `json:"observed"`
	Passed    bool   `json:"passed"`
}
