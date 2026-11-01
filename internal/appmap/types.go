// Package appmap compiles the revision-pinned application map (docs/specs/application-map-v0.md,
// AMAP-V0): a screen graph from declared router definitions, AFU-V1 flow navigation steps and the
// E2E suite's import graph, and serves capped projections over it.
package appmap

import "github.com/Beamfall/corvint/internal/contextindex"

// Wire identities (AMAP-V0-001, AMAP-V0-009, AMAP-V0-011).
const (
	ManifestSchema = "application-map-manifest/0"
	MapSchema      = "application-map/0"
	ScreenSchema   = "application-map-screen/0"
	FlowSchema     = "application-map-flow/0"
	FindSchema     = "application-map-find/0"
	ScaffoldSchema = "application-map-scaffold/0"

	// BuilderVersion names the map compiler (AMAP-V0-215). Bump it in the same change as any
	// change to what BuildRoots derives from the same inputs, so `--from` rebuilds a map an
	// earlier compiler wrote instead of carrying it forward. A binary built from a VCS checkout
	// also binds its revision (builderIdentity), so an unbumped compiler change still rebuilds.
	BuilderVersion = "appmap-builder/2"

	DialectUIRouter = "ui-router-states/0"
	DialectRails    = "rails-routes/0"
	DialectReact    = "react-router/0"

	StatusResolved = "resolved"
	StatusUnknown  = "UNKNOWN"

	Fresh        = "FRESH"
	Stale        = "STALE"
	FreshUnknown = "UNKNOWN"
)

// Input bounds, applied before decode or expansion (AMAP-V0-015).
const (
	maxManifestBytes = 256 << 10
	maxRouterFiles   = 64
	maxRouterBytes   = 4 << 20
	maxStates        = 20000
	maxTestFiles     = 20000
	maxTestFileBytes = 1 << 20
	maxTestBytes     = 128 << 20
	maxImportHops    = 8
	maxMapBytes      = 64 << 20
)

// Manifest is the closed application-map-manifest/0 document, committed and read at the evaluated
// revision (AMAP-V0-001).
type Manifest struct {
	Schema     string `json:"schema"`
	App        string `json:"app"`
	HashPrefix string `json:"hash_prefix,omitempty"`
	// BasePath is the router basename every application URL carries, such as "/web"
	// (AMAP-V0-191); empty serves the application at the origin root.
	BasePath          string             `json:"base_path,omitempty"`
	Routers           []RouterFile       `json:"routers"`
	Flows             string             `json:"flows,omitempty"`
	Tests             TestLayout         `json:"tests"`
	PageObjectScreens []PageObjectScreen `json:"page_object_screens,omitempty"`
	// DIConstants scopes the AngularJS `.constant(...)` registrations a router's injected
	// parameters resolve through (AMAP-V0-021); empty resolves none.
	DIConstants []string `json:"di_constants,omitempty"`
	// DIClosureLimit raises the files whose import graph may reach an injected table that one
	// examined closure may hold (AMAP-V0-369); 0 keeps the default of 256.
	DIClosureLimit int `json:"di_closure_limit,omitempty"`
	// RouterGlobs declares the discovery scope compared with explicit routers (AMAP-V0-045).
	RouterGlobs []string `json:"router_globs,omitempty"`
	// Registries declares string-key registries and their accessors for non-Go syntax impact
	// (NGI-V0-007); the application map itself does not read them.
	Registries []contextindex.RegistryDeclaration `json:"registries,omitempty"`
}

// RouterFile names one router definition file and its dialect. Mount, for a React file, names
// the splat route of another declared router that renders this file's descendant routes
// (AMAP-V0-192).
type RouterFile struct {
	Path    string       `json:"path"`
	Dialect string       `json:"dialect"`
	Mount   *RouterMount `json:"mount,omitempty"`
}

// RouterMount is the declared splat route (`path`, as that router writes its full path) of the
// declared React router file `router` under which a mounted file's routes are matched.
type RouterMount struct {
	Router string `json:"router"`
	Path   string `json:"path"`
}

// TestLayout declares the E2E test root and the role of each directory under it. Repo, when set,
// names the operator-declared root (`--repo ALIAS=ABSOLUTE_ROOT`) that holds the tests
// (AMAP-V0-017); empty is --root.
type TestLayout struct {
	Repo        string   `json:"repo,omitempty"`
	Root        string   `json:"root"`
	Specs       []string `json:"specs"`
	PageObjects []string `json:"page_objects,omitempty"`
	Workflows   []string `json:"workflows,omitempty"`
	Scenarios   []string `json:"scenarios,omitempty"`
	// WebRoot is the dev-server root browser-context root-relative specifiers resolve against
	// (AMAP-V0-782); empty leaves them unknown.
	WebRoot string `json:"web_root,omitempty"`
}

// PageObjectScreen declares the router state a page object models.
type PageObjectScreen struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

// Anchor pins one element to committed bytes: path, line span, blob and span digest at the map's
// revision (AMAP-V0-009). Repo names the aliased root the bytes were read from; empty is --root
// (AMAP-V0-018).
type Anchor struct {
	Repo       string `json:"repo,omitempty"`
	Path       string `json:"path"`
	Start      int    `json:"start_line"`
	End        int    `json:"end_line"`
	Blob       string `json:"blob"`
	SpanSHA256 string `json:"span_sha256"`
}

// Map is the application-map/0 artifact.
type Map struct {
	Schema   string `json:"schema"`
	App      string `json:"app"`
	Revision string `json:"revision"`
	// Roots pins every aliased root the map read and what it supplied; absent for a single-root
	// map (AMAP-V0-018).
	Roots []RootPin `json:"roots,omitempty"`
	// HashPrefix is the manifest's hash-routing prefix, kept so projections can print a loadable URL.
	HashPrefix string `json:"hash_prefix"`
	// BasePath is the manifest's router basename (AMAP-V0-191); absent when empty.
	BasePath    string       `json:"base_path,omitempty"`
	Manifest    Anchor       `json:"manifest"`
	Screens     []Screen     `json:"screens"`
	Edges       []Edge       `json:"edges"`
	Flows       []Flow       `json:"flows"`
	Files       []TestFile   `json:"files"`
	Unknowns    []Unknown    `json:"unknowns"`
	AppCoverage *AppCoverage `json:"app_coverage,omitempty"`
	// Builder is the builder identity that wrote the map (AMAP-V0-215). It is provenance, not
	// content: the digest excludes it, so maps written before it existed keep their digests.
	Builder string `json:"builder,omitempty"`
	Digest  string `json:"digest"`
}

// RootPin is one aliased root of a map: its alias, the commit read, and the manifest inputs it
// supplied ("manifest", "tests").
type RootPin struct {
	Repo     string   `json:"repo"`
	Revision string   `json:"revision"`
	Inputs   []string `json:"inputs"`
	// Stated marks a revision the operator named with --repo-revision instead of the root's HEAD
	// (AMAP-V0-200); absent for a HEAD pin, so earlier maps keep their bytes.
	Stated bool `json:"stated,omitempty"`
}

// Screen is one router state: a node of the screen graph (AMAP-V0-002, AMAP-V0-003, AMAP-V0-006).
type Screen struct {
	// Entry records static entry constraints; absent in older maps (AMAP-V0-053/058).
	Entry *EntryDeclaration `json:"entry,omitempty"`
	// SplatNonEmpty preserves Rails required-glob semantics while sharing collision keys (AMAP-V0-046).
	SplatNonEmpty   bool     `json:"splat_nonempty,omitempty"`
	ID              string   `json:"id"`
	State           string   `json:"state"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason,omitempty"`
	Template        string   `json:"template,omitempty"`
	Key             string   `json:"key,omitempty"`
	Parent          string   `json:"parent,omitempty"`
	Abstract        bool     `json:"abstract,omitempty"`
	Params          []string `json:"params"`
	Query           []string `json:"query"`
	Permissions     []string `json:"permissions"`
	PermissionsFrom string   `json:"permissions_from,omitempty"`
	Flags           []string `json:"flags"`
	FlagsFrom       string   `json:"flags_from,omitempty"`
	Anchor          Anchor   `json:"anchor"`
	// NameFrom and ParentFrom anchor a state name or parent read through a constant table
	// (AMAP-V0-016): the declaring line, then the import binding when the table is imported;
	// absent for a literal.
	NameFrom      []Anchor      `json:"name_from,omitempty"`
	ParentFrom    []Anchor      `json:"parent_from,omitempty"`
	PageObjects   []string      `json:"page_objects"`
	Workflows     []string      `json:"workflows"`
	Scenarios     []string      `json:"scenarios"`
	Specs         []Attribution `json:"specs"`
	Flows         []string      `json:"flows"`
	Steps         []string      `json:"steps"`
	Preconditions []Requirement `json:"preconditions"`
}

// Attribution says why a spec covers a screen: an import chain to a bound page object or a
// literal goto (AMAP-V0-005).
type Attribution struct {
	File  string   `json:"file"`
	Basis string   `json:"basis"`
	Via   []string `json:"via"`
}

// Requirement is one data precondition and the flow that declares it.
type Requirement struct {
	Flow string `json:"flow"`
	Text string `json:"text"`
}

// Edge is one navigation between screens with its witness (AMAP-V0-004, AMAP-V0-008).
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Basis  string `json:"basis"`
	Source string `json:"source"`
	Anchor Anchor `json:"anchor"`
	// Effect classifies a test-sequence step (ANL-V0-035): navigation-only, state-effect or
	// unknown, with the deciding call and the page-object method spans read; absent on other
	// edges and in older maps.
	Effect     string   `json:"effect,omitempty"`
	EffectRef  string   `json:"effect_ref,omitempty"`
	EffectCall string   `json:"effect_call,omitempty"`
	EffectFrom []Anchor `json:"effect_from,omitempty"`
}

// Flow is one AFU-V1 flow intent projected onto the screen graph.
type Flow struct {
	ID            string    `json:"id"`
	FlowID        string    `json:"flow_id"`
	Preconditions []string  `json:"preconditions"`
	Outcomes      []Outcome `json:"outcomes"`
	Steps         []Step    `json:"steps"`
	Anchor        Anchor    `json:"anchor"`
}

// Outcome is one declared expected outcome of a flow.
type Outcome struct {
	ID       string `json:"id"`
	Behavior string `json:"behavior"`
	Matcher  string `json:"matcher"`
	Locator  string `json:"locator"`
	Value    string `json:"value"`
}

// Step is one flow step: its screen, locator with strength, and the page-object methods that
// already perform an identical selector (reuse points).
type Step struct {
	ID       string    `json:"id"`
	Action   string    `json:"action"`
	Screen   string    `json:"screen,omitempty"`
	Status   string    `json:"status"`
	Reason   string    `json:"reason,omitempty"`
	Selector *Selector `json:"selector,omitempty"`
	Reuse    []string  `json:"reuse"`
	// Tests are the test keys the flow intent's declared `test` links bind to this step
	// (RVN-V0-002); inferred links never bind.
	Tests []string `json:"tests,omitempty"`
}

// Selector is one locator with its strength class (AMAP-V0-007). Its ID is content-addressed, so
// the same selector keeps its ID across files and revisions.
type Selector struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	Name     string `json:"name,omitempty"`
	Strength string `json:"strength"`
	Line     int    `json:"line,omitempty"`
}

// ClassExportDefault marks a page-object class that is only a default export: no named import of
// it can bind, so the scaffold never generates one (V1-0974).
const ClassExportDefault = "default"

// TestFile is one analysed E2E source file.
type TestFile struct {
	ID          string     `json:"id"`
	Path        string     `json:"path"`
	Role        string     `json:"role"`
	Class       string     `json:"class,omitempty"`
	Screen      string     `json:"screen,omitempty"`
	ScreenBasis string     `json:"screen_basis,omitempty"`
	Join        string     `json:"join"`
	Anchor      Anchor     `json:"anchor"`
	Imports     []Import   `json:"imports"`
	Methods     []Method   `json:"methods"`
	Gotos       []Goto     `json:"gotos"`
	Tests       []string   `json:"tests"`
	Assertions  int        `json:"assertions"`
	Selectors   []Selector `json:"selectors"`
	// ClassExport is "default" when Class is declared `export default class`; it is omitted
	// otherwise, so a named class reads as before (V1-0974).
	ClassExport string `json:"class_export,omitempty"`
	// HelperGotos are a page object's literal URLs passed to a one-hop navigation helper: learned
	// remedy candidates, never a screen binding (AMAP-V0-342).
	HelperGotos []Goto `json:"helper_gotos,omitempty"`
}

// Import is one static import statement and its resolution against the committed test tree.
type Import struct {
	Line      int      `json:"line"`
	Statement string   `json:"statement"`
	Specifier string   `json:"specifier"`
	Status    string   `json:"status"`
	Resolved  string   `json:"resolved,omitempty"`
	Names     []string `json:"names"`
}

// Method is one page-object or workflow method with the selectors it uses.
type Method struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Anchor Anchor `json:"anchor"`
	// Callable is true only for a public instance method whose header literally declares no
	// parameters; the scaffold calls nothing else (AMAP-V0-013).
	Callable  bool       `json:"callable"`
	Selectors []Selector `json:"selectors"`
}

// Goto is one literal navigation in test source and the screen it names.
type Goto struct {
	Line   int    `json:"line"`
	URL    string `json:"url"`
	Screen string `json:"screen,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Unknown is one input the compiler could not resolve; it is reported, never guessed.
type Unknown struct {
	Repo   string `json:"repo,omitempty"`
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	// Detail names the failed hop (and binding, AMAP-V0-196) of a di-constant refusal
	// (AMAP-V0-072, AMAP-V0-201), the candidate paths of an ambiguous re-export, why an injected
	// state name or parent refused (AMAP-V0-202), or the specifier of an unresolved test import
	// (AMAP-V0-195); absent otherwise.
	Detail string `json:"detail,omitempty"`
}

// Import statuses.
const (
	importResolved   = "resolved"
	importExternal   = "external"
	importUnresolved = "unresolved"
	importBrowser    = "browser-context" // runs in the page, not a Node import of the test file (AMAP-V0-781)
)

// File roles.
const (
	roleSpec       = "spec"
	rolePageObject = "page-object"
	roleWorkflow   = "workflow"
	roleScenario   = "scenario"
	roleSupport    = "support"
)
