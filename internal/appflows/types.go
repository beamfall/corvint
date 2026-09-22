// Package appflows composes experimental, caller-reported application behavior evidence.
package appflows

import "encoding/json"

const (
	IntentProfile   = "application-flow-intent/0"
	EvidenceProfile = "application-flow-evidence/0"
	ReportProfile   = "application-flow-report/0"
	MaxBytes        = 5 << 20
	MaxSources      = 128
)

type Manifest struct {
	Profile       string     `json:"profile"`
	Application   string     `json:"application"`
	Origin        string     `json:"origin"`
	Sources       []string   `json:"sources"`
	Tests         []string   `json:"tests"`
	BackendSource string     `json:"backendSource"`
	Fixture       string     `json:"fixture"`
	IdentityPath  string     `json:"identityPath"`
	ResetPath     string     `json:"resetPath"`
	Server        []string   `json:"server"`
	Scenarios     []Scenario `json:"scenarios"`
}

type Scenario struct {
	ID      string   `json:"id"`
	Role    string   `json:"role"`
	Path    string   `json:"path"`
	Basis   string   `json:"basis"`
	Actions []Action `json:"actions"`
	Checks  []Check  `json:"checks"`
}

type Action struct {
	Kind     string `json:"kind"`
	Selector string `json:"selector,omitempty"`
	Value    string `json:"value,omitempty"`
}

type Check struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Selector string          `json:"selector,omitempty"`
	Path     string          `json:"path,omitempty"`
	Pointer  string          `json:"pointer,omitempty"`
	Want     json.RawMessage `json:"want"`
}

type Binding struct {
	Tree           string            `json:"tree"`
	ManifestDigest string            `json:"manifestDigest"`
	Sources        map[string]string `json:"sources"`
	FrontendDigest string            `json:"frontendDigest"`
	BackendDigest  string            `json:"backendDigest"`
	Fixture        string            `json:"fixture"`
}

type Source struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// Input is passed over stdin to the optional parser/observer; sources never execute in scan mode.
type Input struct {
	Manifest Manifest `json:"manifest"`
	Binding  Binding  `json:"binding"`
	Sources  []Source `json:"sources"`
	Root     string   `json:"root"`
	Observe  bool     `json:"observe"`
	RunID    string   `json:"runId"`
}

type Anchor struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Digest string `json:"digest"`
}

type Test struct {
	ID         string      `json:"id"`
	Anchor     Anchor      `json:"anchor"`
	Route      string      `json:"route"`
	Actions    []Action    `json:"actions"`
	Assertions []Assertion `json:"assertions"`
	Complete   bool        `json:"complete"`
}

type Assertion struct {
	Kind       string `json:"kind"`
	Selector   string `json:"selector"`
	WantDigest string `json:"wantDigest"`
	Anchor     Anchor `json:"anchor"`
}

type Control struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Selector  string `json:"selector"`
	State     string `json:"state"`
	Exercised bool   `json:"exercised"`
}

type CheckResult struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Layer   string `json:"layer"`
}

type Run struct {
	Scenario        string        `json:"scenario"`
	Outcome         string        `json:"outcome"`
	Checks          []CheckResult `json:"checks"`
	States          []string      `json:"states"`
	IdentityMatched bool          `json:"identityMatched"`
}

type Evidence struct {
	Profile           string    `json:"profile"`
	Authority         string    `json:"authority"`
	Binding           Binding   `json:"binding"`
	RunID             string    `json:"runId"`
	Mode              string    `json:"mode"`
	Parser            string    `json:"parser"`
	ProviderDigest    string    `json:"providerDigest"`
	NodeVersion       string    `json:"nodeVersion"`
	PlaywrightVersion string    `json:"playwrightVersion"`
	Browser           string    `json:"browser"`
	Tests             []Test    `json:"tests"`
	InventoryComplete bool      `json:"inventoryComplete"`
	Controls          []Control `json:"controls"`
	Runs              []Run     `json:"runs"`
	Gaps              []string  `json:"gaps"`
	Cleanup           bool      `json:"cleanup"`
	CleanupScope      string    `json:"cleanupScope"`
	BrowserClosed     bool      `json:"browserClosed"`
	ServerExited      bool      `json:"serverExited"`
}

type Coverage struct {
	Check      string   `json:"check"`
	State      string   `json:"state"`
	Tests      []string `json:"tests"`
	Assertions []Anchor `json:"assertions"`
}

type Flow struct {
	ID           string        `json:"id"`
	Role         string        `json:"role"`
	RoleIdentity string        `json:"roleIdentity"`
	Basis        string        `json:"basis"`
	Coverage     []Coverage    `json:"coverage"`
	Runtime      string        `json:"runtime"`
	Checks       []CheckResult `json:"checks"`
	Gaps         []string      `json:"gaps"`
	Next         string        `json:"next"`
}

type Report struct {
	Profile        string      `json:"profile"`
	Authority      string      `json:"authority"`
	Application    string      `json:"application"`
	Binding        Binding     `json:"binding"`
	Freshness      string      `json:"freshness"`
	RunID          string      `json:"runId"`
	ProviderDigest string      `json:"providerDigest"`
	Flows          []Flow      `json:"flows"`
	Candidates     []Candidate `json:"candidates"`
	Discovered     []Control   `json:"discovered"`
	Frontier       []string    `json:"frontier"`
	Complete       bool        `json:"complete"`
}

// Candidate is an inferred multi-step journey, independent of the supplied intent inventory.
type Candidate struct {
	ID        string   `json:"id"`
	Basis     string   `json:"basis"`
	Route     string   `json:"route"`
	Actions   []Action `json:"actions"`
	Source    Anchor   `json:"source"`
	Freshness string   `json:"freshness"`
}
