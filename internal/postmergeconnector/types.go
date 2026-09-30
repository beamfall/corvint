// Package postmergeconnector is an experimental, local reference companion.
// It grants no authority to raw context, request records, or connector effects.
package postmergeconnector

const Profile = "postmerge-connector/0"
const MaxBytes = 4 << 20

type Binding struct {
	Forge      string `json:"forge"`
	Repository string `json:"repository"`
	Change     string `json:"change"`
	Base       string `json:"base"`
	Merge      string `json:"merge"`
}
type ChangedFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
type Change struct {
	Binding   Binding       `json:"binding"`
	Author    string        `json:"author"`
	CreatedAt string        `json:"created_at"`
	MergedAt  string        `json:"merged_at"`
	Files     []ChangedFile `json:"files"`
	Title     string        `json:"title"`
	Body      string        `json:"body"`
}
type Item struct {
	ID     string `json:"id"`
	Level  string `json:"level"`
	Parent string `json:"parent"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// LocalTracker and LocalForge are different reference connector kinds. They
// never inspect credentials or contact a remote service.
type LocalTracker struct {
	Items []Item `json:"items"`
}
type LocalForge struct {
	Change Change `json:"change"`
}

func (LocalTracker) MinimumScopes() []string { return []string{} }
func (LocalForge) MinimumScopes() []string   { return []string{} }

type Fixture struct {
	Profile string       `json:"profile"`
	Tracker LocalTracker `json:"tracker"`
	Forge   LocalForge   `json:"forge"`
}
type Policy struct {
	Profile        string   `json:"profile"`
	Expected       Binding  `json:"expected"`
	HierarchyLevel string   `json:"hierarchy_level"`
	AllowedClasses []string `json:"allowed_classes"`
	MaxFindings    int      `json:"max_findings"`
	URLOrigins     []string `json:"url_origins"`
}
type Counts struct {
	Docs   int `json:"docs"`
	Tests  int `json:"tests"`
	Corpus int `json:"corpus"`
}
type Finding struct {
	Class          string `json:"class"`
	Classification string `json:"classification"` // ordinary | security | unknown
	Path           string `json:"path"`
	Line           int    `json:"line"`
}
type Draft struct {
	Kind     string `json:"kind"` // docs | tests | corpus
	Revision string `json:"revision"`
	URL      string `json:"url"`
}
type Input struct {
	Binding    Binding   `json:"binding"`
	SourceItem string    `json:"source_item"`
	Counts     Counts    `json:"counts"`
	Findings   []Finding `json:"findings"`
	Drafts     []Draft   `json:"drafts"`
}
type Intake struct {
	Profile   string `json:"profile"`
	Change    Change `json:"change"`
	Hierarchy []Item `json:"hierarchy"`
}

// Request contains only the fixed body and typed interpolations. These bytes
// are observations for a trusted deterministic writer, never agent write tools.
type Request struct {
	Profile   string `json:"profile"`
	Key       string `json:"key"`
	Operation string `json:"operation"`
	Source    string `json:"source"`
	Parent    string `json:"parent"`
	Route     string `json:"route"`
	Branch    string `json:"branch"`
	Draft     bool   `json:"draft"`
	Body      string `json:"body"`
}
type Plan struct {
	Profile  string    `json:"profile"`
	Input    Input     `json:"input"`
	Requests []Request `json:"requests"`
	Digest   string    `json:"digest"`
}
type State struct {
	Profile string             `json:"profile"`
	Tracker map[string]Request `json:"tracker"`
	Forge   map[string]Request `json:"forge"`
}

// Writer is implemented by deterministic host steps. Hosts alone supply any
// credentials to their writer; raw readers and authoring steps cannot receive them.
type Writer interface{ Upsert(Request) error }
