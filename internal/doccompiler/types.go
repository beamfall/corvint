package doccompiler

import (
	"time"
)

const (
	Profile              = "corvint-human-documentation-compiler/0"
	PlanProfile          = "corvint-human-documentation-plan/0"
	ReceiptProfile       = "corvint-human-documentation-receipt/0"
	MaterialProfile      = "mkdocs-material/qualified-p0"
	defaultCommandBytes  = 256 << 10
	defaultConfigBytes   = 1 << 20
	defaultSourceBytes   = 8 << 20
	defaultCorpusBytes   = 64 << 20
	defaultOutputBytes   = 128 << 20
	defaultOutputFiles   = 10_000
	defaultBuildDuration = 2 * time.Minute
)

// ExperimentalPatchPlanProfile names the experimental PatchPlan that Plan emits
// and Build consumes. It is not the HDCV0-027 wire (decision 0240).
const ExperimentalPatchPlanProfile = "corvint-human-documentation-experimental-patch-plan/0"

// DiscoverOptions names a project-owned environment explicitly. No PATH lookup
// or dependency installation is performed.
type DiscoverOptions struct {
	ProjectRoot               string
	MkDocsPath                string
	PythonPath                string
	ConfigPath                string
	ProjectLockPath           string
	ExpectedProjectLockSHA256 string
	ExpectedMkDocsVersion     string
	ExpectedMaterialVersion   string
	ExpectedMarkdownVersion   string
	ExpectedPyMdownVersion    string
	MaxCommandBytes           int
	MaxConfigBytes            int64
	TrustAttestation          EnvironmentTrustAttestation
}

type FilePin struct {
	Path         string `json:"path"`
	ResolvedPath string `json:"resolved_path,omitempty"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}

type Toolchain struct {
	MkDocs            FilePin        `json:"mkdocs"`
	Python            FilePin        `json:"python"`
	ProjectLock       FilePin        `json:"project_lock"`
	MkDocsVersion     string         `json:"mkdocs_version"`
	MaterialVersion   string         `json:"material_version"`
	MarkdownVersion   string         `json:"markdown_version"`
	PyMdownVersion    string         `json:"pymdown_extensions_version,omitempty"`
	PythonVersion     string         `json:"python_version"`
	EnvironmentSHA256 string         `json:"environment_sha256"`
	DistributionCount int            `json:"distribution_count"`
	Distributions     []Distribution `json:"distributions"`
}

type Distribution struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type OptionDigest struct {
	Name          string `json:"name"`
	OptionsSHA256 string `json:"options_sha256"`
}

type ConfigObservations struct {
	Authority                string         `json:"authority"`
	ParserStatus             string         `json:"parser_status"`
	DocsDir                  string         `json:"docs_dir"`
	SiteDir                  string         `json:"site_dir"`
	NavConfigured            bool           `json:"nav_configured"`
	NavMode                  string         `json:"nav_mode"`
	NavOwner                 string         `json:"nav_owner"`
	NavSHA256                string         `json:"nav_sha256"`
	SiteURL                  string         `json:"site_url,omitempty"`
	UseDirectoryURLs         bool           `json:"use_directory_urls"`
	ThemeName                string         `json:"theme_name,omitempty"`
	ThemeOptionsSHA256       string         `json:"theme_options_sha256"`
	ThemeFeatures            []string       `json:"theme_features"`
	ThemeCustomDir           string         `json:"theme_custom_dir,omitempty"`
	PluginsConfigured        bool           `json:"plugins_configured"`
	Plugins                  []string       `json:"plugins"`
	PluginOptions            []OptionDigest `json:"plugin_options"`
	SearchStatus             string         `json:"search_status"`
	MarkdownExtensions       []string       `json:"markdown_extensions"`
	MarkdownExtensionOptions []OptionDigest `json:"markdown_extension_options"`
	ExtraCSS                 []string       `json:"extra_css"`
	ExtraJavaScript          []string       `json:"extra_javascript"`
	Hooks                    []string       `json:"hooks"`
	InheritsConfig           bool           `json:"inherits_config"`
	ValidationConfigured     bool           `json:"validation_configured"`
	ValidationSHA256         string         `json:"validation_sha256"`
	Validation               map[string]any `json:"validation"`
	PrivacySettings          []string       `json:"privacy_settings"`
	OfflineRelated           []string       `json:"offline_related"`
	RemoteAssets             []string       `json:"remote_assets"`
	Uncertainty              []string       `json:"uncertainty"`
}

type Environment struct {
	ProjectRoot   string             `json:"-"`
	Toolchain     Toolchain          `json:"toolchain"`
	Config        FilePin            `json:"config"`
	Observations  ConfigObservations `json:"observations"`
	Qualification string             `json:"qualification"`
}

type Evidence struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Revision   string `json:"revision"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Authority  string `json:"authority"`
	Confidence string `json:"confidence"`
}

type Claim struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Text        string     `json:"text"`
	Uncertainty string     `json:"uncertainty,omitempty"`
	Evidence    []Evidence `json:"evidence"`
}

type DocumentProposal struct {
	Path string
	// Markdown must be empty until prose can be admitted clause by clause.
	Markdown string
	Claims   []Claim
}

type NavEntry struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

type PlanOptions struct {
	Documents []DocumentProposal
	Nav       []NavEntry
}

type SourcePin struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type BytePatch struct {
	StartByte         int    `json:"start_byte"`
	EndByte           int    `json:"end_byte"`
	OriginalSHA256    string `json:"original_sha256"`
	ReplacementSHA256 string `json:"replacement_sha256"`
	Replacement       string `json:"replacement"`
}

type DocumentPatch struct {
	Path             string    `json:"path"`
	Operation        string    `json:"operation"`
	ReviewRequired   bool      `json:"review_required"`
	CurrentSHA256    string    `json:"current_sha256,omitempty"`
	ProposedSHA256   string    `json:"proposed_sha256"`
	Patch            BytePatch `json:"patch"`
	RenderedMarkdown string    `json:"-"`
	Claims           []Claim   `json:"claims"`
}

type NavPatch struct {
	ConfigPath        string     `json:"config_path"`
	ConfigSHA256      string     `json:"config_sha256"`
	Operation         string     `json:"operation"`
	ReviewRequired    bool       `json:"review_required"`
	CurrentNavSHA256  string     `json:"current_nav_sha256"`
	ProposedNavSHA256 string     `json:"proposed_nav_sha256"`
	Patch             BytePatch  `json:"patch"`
	Entries           []NavEntry `json:"entries"`
}

type PatchPlan struct {
	Profile      string          `json:"profile"`
	ConfigSHA256 string          `json:"config_sha256"`
	Documents    []DocumentPatch `json:"documents"`
	Nav          NavPatch        `json:"nav"`
	Evidence     []Evidence      `json:"evidence"`
	Sources      []SourcePin     `json:"sources"`
	PlanSHA256   string          `json:"plan_sha256"`
}

type BuildOptions struct {
	OutputParent     string
	Timeout          time.Duration
	MaxStdoutBytes   int
	MaxStderrBytes   int
	MaxSourceBytes   int64
	MaxCorpusBytes   int64
	MaxOutputBytes   int64
	MaxOutputFiles   int
	TrustAttestation EnvironmentTrustAttestation
}

type EnvironmentTrustAttestation struct {
	Profile           string `json:"profile"`
	EnvironmentSHA256 string `json:"environment_sha256"`
	Authority         string `json:"authority"`
	Revision          string `json:"revision"`
}

type BuildResult struct {
	BuildStrictStatus       string   `json:"build_strict_status"`
	OfflineQualification    string   `json:"offline_qualification"`
	Argv                    []string `json:"argv"`
	OutputRoot              string   `json:"-"`
	OutputSHA256            string   `json:"output_sha256,omitempty"`
	OutputBytes             int64    `json:"output_bytes"`
	OutputFiles             int      `json:"output_files"`
	Stdout                  string   `json:"stdout,omitempty"`
	Stderr                  string   `json:"stderr,omitempty"`
	DocumentCandidatesBuilt bool     `json:"document_candidates_built"`
	NavCandidateBuilt       bool     `json:"nav_candidate_built"`
	OfflineEnforcement      string   `json:"offline_enforcement"`
	ProcessContainment      string   `json:"process_containment"`
	Uncertainty             []string `json:"uncertainty"`
}

type OfflineAttestation struct {
	Profile           string `json:"profile"`
	BuildSHA256       string `json:"build_sha256"`
	Authority         string `json:"authority"`
	Method            string `json:"method"`
	AttestationSHA256 string `json:"attestation_sha256"`
}

type Compiler struct{}

func New() *Compiler { return &Compiler{} }

// Plan currently admits only UNKNOWN claims with an explicit uncertainty frontier
// and no Evidence. Working-tree source pins are patch preconditions, not immutable
// evidence or acceptance authority. SUPPORTED and CONFLICTED require a future
// immutable-source and authority verifier.
func (compiler *Compiler) Plan(environment Environment, options PlanOptions) (PatchPlan, error) {
	return plan(environment, options)
}
