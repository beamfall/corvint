// Package doccorpus compiles the proposed DCP-V1 documentation evidence profile.
// Identity validation does not establish semantic truth or accepted intent.
package doccorpus

import "github.com/Beamfall/corvint/internal/testvaliditydoc"

const (
	Schema         = "corvint-evidence-corpus/1"
	ManifestSchema = "corvint-corpus-input/1"
	ProviderSchema = "corvint-corpus-provider/1"
	ReceiptSchema  = "corvint-corpus-receipt/2"
	MaxBytes       = 4 << 20
	MaxRecords     = 4096
	MaxResults     = 256
)

type Repository struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

// AdditionalRepository is one further repository a manifest names explicitly (DCP-V1-175): its
// root-commit identity, its one pinned revision and the local checkout holding its objects, as a
// clean path relative to the primary repository root. The locator grants no identity; the build
// verifies both against the checkout and never fetches or discovers a repository.
//
// Revisions optionally pins further, older commits of the same repository, read from the same
// checkout (DCP-V1-308); Revision stays the declared current one that freshness compares.
type AdditionalRepository struct {
	ID        string   `json:"id"`
	Revision  string   `json:"revision"`
	Revisions []string `json:"revisions,omitempty"`
	Root      string   `json:"root"`
	// Alias is the workspace alias (WSD-V0) the repository was declared under; it qualifies the
	// native provider's record IDs for this repository (DCP-V1-600). Omitted when empty.
	Alias string `json:"alias,omitempty"`
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
	// AsOf is a compiled input's declared upstream position (DCP-V1-574). It
	// is a pointer so an explicit but empty declaration is refused, not dropped.
	AsOf *InputAsOf `json:"as_of,omitzero"`
}
type Scope struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
}
type Provider struct {
	Shards []string `json:"shards,omitempty"`
	// Derive restricts the declarations a native provider derives; absent, it derives every
	// declaration the bounds admit and the manifest's bytes are unchanged (DCS-V1-573).
	Derive   *DerivationFilter `json:"derive,omitempty"`
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Version  string            `json:"version"`
	Revision string            `json:"revision"`
	Record   string            `json:"record"`
}

// DerivationFilter selects the source inputs whose declarations a native provider derives
// (DCS-V1-573). An input is selected when Paths is empty or names it, and Kinds is empty or
// holds its declaration kind. A path entry is "[repository-id:]path", exact or, ending in "/",
// a directory prefix; the repository qualifier names an additional repository, its absence the
// primary one. Kinds are "module", "symbol" and "test"; "module" names the file subject every
// input keeps, so a filter whose kinds are only "module" derives no declaration. Both lists are
// sorted and duplicate-free. Native also applies the filter to the declarations of the provider's
// own inputs (DCS-V1-590); absent, they keep every declaration and the bytes are unchanged.
type DerivationFilter struct {
	Paths  []string `json:"paths,omitempty"`
	Kinds  []string `json:"kinds,omitempty"`
	Native bool     `json:"native,omitempty"`
}
type Manifest struct {
	// AdditionalRepositories is absent from a single-repository manifest, whose bytes are
	// unchanged (DCP-V1-175).
	AdditionalRepositories []AdditionalRepository `json:"additional_repositories,omitempty"`
	BehaviorRepositories   []BehaviorRepository   `json:"behavior_repositories,omitempty"`
	BehaviorRevisions      *BehaviorRevisions     `json:"behavior_revisions,omitempty"`
	Schema                 string                 `json:"schema"`
	Repository             Repository             `json:"repository"`
	BuiltAt                string                 `json:"built_at"`
	Profile                Profile                `json:"profile"`
	Scopes                 []Scope                `json:"scopes"`
	Inputs                 []Input                `json:"inputs"`
	Providers              []Provider             `json:"providers"`
	MergeRule              string                 `json:"merge_rule"`
	// RepositoryInventory and ProductionScope are absent from a manifest that does not declare
	// them, whose bytes are unchanged (DCP-V1-700, DCP-V1-720). An empty declaration `{}` is a
	// declaration and is kept: omitempty would drop it from the encoded manifest the artifact
	// embeds, while the build still derived its inventory (DCI-V1-005).
	RepositoryInventory *RepositoryInventory `json:"repository_inventory,omitzero"`
	ProductionScope     *ProductionScope     `json:"production_scope,omitempty"`
	// OverBoundInputs are the tracked files in a declared scope that are over the per-input
	// bound: each is pinned by blob and size, never read, and reported as an input-over-bound gap,
	// so the rest of the scope still builds (DCI-V1-024). Absent when there are none, so the
	// manifest's bytes are unchanged.
	OverBoundInputs []OverBoundInput `json:"over_bound_inputs,omitempty"`
}

// OverBoundInput is one scope file over the per-input bound, pinned without being read
// (DCI-V1-024).
type OverBoundInput struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
	Blob     string `json:"blob"`
	Bytes    int    `json:"bytes"`
	Provider string `json:"provider"`
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
	// AlternateIDs are a supplied subject's producer identifiers, each resolved
	// exactly to this subject by the id readers (DCP-V1-329, DCP-V1-330).
	AlternateIDs []string `json:"alternate_ids,omitempty"`
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
	Details            map[string]RecordDetails `json:"details,omitempty"`
	RestrictedFindings []RestrictedFinding      `json:"restricted_findings,omitempty"`
	BehaviorContracts  *BehaviorRegistry        `json:"behavior_contracts,omitempty"`
	TestKnowledge      *TestKnowledgeSet        `json:"test_knowledge,omitempty"`
	FlowRelations      *FlowRelationSet         `json:"flow_relations,omitempty"`
	ClaimAudits        *ClaimAuditSet           `json:"claim_audits,omitempty"`
	// Gaps are producer-supplied gap claims (DCP-V1-185); adoption schema only.
	Gaps         []SuppliedGap           `json:"gaps,omitempty"`
	Schema       string                  `json:"schema"`
	ID           string                  `json:"id"`
	Version      string                  `json:"version"`
	Source       Repository              `json:"source"`
	Subjects     []Subject               `json:"subjects"`
	Claims       []Claim                 `json:"claims"`
	Relations    []Relation              `json:"relations"`
	Journeys     []Journey               `json:"journeys"`
	Observations []ObservationLink       `json:"observations"`
	Capabilities []CapabilityDeclaration `json:"capabilities"`
}
type Capability struct {
	RecordsOmitted int      `json:"records_omitted,omitempty"`
	Name           string   `json:"name"`
	State          string   `json:"state"`
	Reason         string   `json:"reason"`
	Records        []string `json:"records"`
	Providers      []string `json:"providers"`
	Tools          []string `json:"tools"`
	Count          int      `json:"count"`
	Denominator    int      `json:"denominator"`
	Rule           string   `json:"rule"`
}
type Gap struct {
	Subject string `json:"subject"`
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
}

// SuppliedGap is a producer's gap claim (DCP-V1-185, DCP-V1-186). It is
// attributed provider data, never a compiler result: the compiler sets Origin
// to "supplied" and its evidence keeps the provider trust ceiling.
type SuppliedGap struct {
	ID       string   `json:"id"`
	Subject  string   `json:"subject"`
	Kind     string   `json:"kind"`
	Reason   string   `json:"reason"`
	Origin   string   `json:"origin"`
	Provider string   `json:"provider"`
	Evidence Evidence `json:"evidence"`
}
type Artifact struct {
	// RuntimeIndex is derived resident state, never evidence or serialized authority.
	RuntimeIndex        *QueryIndex              `json:"-"`
	Details             map[string]RecordDetails `json:"details,omitempty"`
	ImportParity        []ImportParity           `json:"import_parity,omitempty"`
	RestrictedSummaries []RestrictedSummary      `json:"restricted_summaries,omitempty"`
	BehaviorContracts   []BehaviorReport         `json:"behavior_contracts,omitempty"`
	StabilityEvidence   []StabilityReport        `json:"stability_evidence,omitempty"`
	TestKnowledge       []TestKnowledgeJoin      `json:"test_knowledge,omitempty"`
	FlowRelations       []FlowRelationJoin       `json:"flow_relations,omitempty"`
	ClaimAudits         []ClaimAuditJoin         `json:"claim_audits,omitempty"`
	// ClaimMentions is the build-time mention check of every non-native claim with a code-like
	// token, sorted by claim ID (DCX-V1-341).
	ClaimMentions  []ClaimMentionCheck `json:"claim_mentions,omitempty"`
	SuppliedGaps   []SuppliedGap       `json:"supplied_gaps,omitempty"`
	Schema         string              `json:"schema"`
	Builder        Builder             `json:"builder"`
	Manifest       Manifest            `json:"manifest"`
	ManifestSHA256 string              `json:"manifest_sha256"`
	ProfileSHA256  string              `json:"profile_sha256"`
	Tree           string              `json:"tree"`
	Subjects       []Subject           `json:"subjects"`
	Claims         []Claim             `json:"claims"`
	Relations      []Relation          `json:"relations"`
	Journeys       []Journey           `json:"journeys"`
	Observations   []Observation       `json:"observations"`
	Capabilities   []Capability        `json:"capabilities"`
	Gaps           []Gap               `json:"gaps"`
	SHA256         string              `json:"sha256"`

	// validation is how a reader admitted the artifact (DCR-V1-413); never serialized.
	validation string
	// deltaUnverified is set only by OpenDeltaCurrent when the artifact could not be rederived
	// because a declared additional checkout is unavailable; the delta reports it (DCX-V1-425).
	deltaUnverified []string
	// claimTerms hands the claim-term derivation a read made while decoding to the first query
	// index derivation, instead of tokenizing again (DCR-V1-434); never serialized. It is set
	// before the read returns and never reassigned, so a copy of the artifact shares it.
	claimTerms *claimTermHandoff
}
type Request struct {
	Retirement *RetirementPolicy `json:"retirement,omitempty"`
	Offset     int               `json:"offset,omitempty"`
	Operation  string            `json:"operation"`
	Query      string            `json:"query"`
	ID         string            `json:"id"`
	Path       string            `json:"path"`
	Limit      int               `json:"limit"`
	Depth      int               `json:"depth,omitempty"`
	// Sections and Include filter a flow query (DCP-V1-170, DCP-V1-171);
	// nil means absent.
	Sections []string `json:"sections,omitempty"`
	// Include is also ["claims"] on a search, which then ranks claim hits with
	// subject hits (DCP-V1-328).
	Include []string `json:"include,omitempty"`
	// Repository optionally restricts locate and recommend-tests to one declared repository's
	// root-commit identity (DCP-V1-310), or a declared repository's alias (DCP-V1-602); with
	// Path it also names the repository of a trace or gaps path (DCP-V1-602).
	Repository string `json:"repository,omitempty"`
	// Kinds filters a gaps query to the named gap kinds (DCP-V1-284), and a
	// claims query to the named claim kinds (DCS-V1-592); nil means absent.
	Kinds []string `json:"kinds,omitempty"`
	// ExcludeKinds drops the named claim kinds from a claims query
	// (DCS-V1-592); nil means absent.
	ExcludeKinds []string `json:"exclude_kinds,omitempty"`
	// State, Qualifier and Run select a claims query's supplied status claims
	// (DCP-V1-318); an empty State means absent.
	State     string `json:"state,omitempty"`
	Qualifier string `json:"qualifier,omitempty"`
	Run       string `json:"run,omitempty"`
	// CallerRevision is the caller's checkout commit, compared with the corpus
	// source revision (DCP-V1-332); empty means absent.
	CallerRevision string `json:"caller_revision,omitempty"`
}
type Receipt struct {
	Envelope  *TrustEnvelope      `json:"trust_envelope,omitempty"`
	Meaning   string              `json:"meaning,omitempty"`
	Selection *TestRecommendation `json:"test_selection,omitempty"`
	// FlowSelection is present only on a filtered flow query (DCP-V1-172).
	FlowSelection *FlowSelection `json:"flow_selection,omitempty"`
	// KindTotals counts, per requested kind, the gaps a kind-filtered gaps query matched before
	// paging; present only with a kind filter (DCP-V1-284).
	KindTotals     map[string]int           `json:"kind_totals,omitempty"`
	Offset         int                      `json:"offset,omitempty"`
	NextOffset     *int                     `json:"next_offset,omitempty"`
	Details        map[string]RecordDetails `json:"details,omitempty"`
	Schema         string                   `json:"schema"`
	Operation      string                   `json:"operation"`
	ArtifactSHA256 string                   `json:"artifact_sha256"`
	Repository     Repository               `json:"repository"`
	Tree           string                   `json:"tree"`
	Trust          string                   `json:"trust"`
	Freshness      string                   `json:"freshness"`
	State          string                   `json:"state"`
	Miss           string                   `json:"miss"`
	Results        []any                    `json:"results"`
	Citations      []Anchor                 `json:"citations"`
	Capabilities   []Capability             `json:"capabilities"`
	Limitations    []string                 `json:"limitations"`
	Omitted        int                      `json:"omitted"`
	// ClaimAudits shows each visible claim's audit verdicts, present only when the corpus
	// holds claim-audit records (DCP-V1-263).
	ClaimAudits map[string][]ClaimAuditView `json:"claim_audits,omitempty"`
	// ClaimMentions serves each visible claim's mention check, present only when the corpus
	// stores one for it (DCX-V1-343).
	ClaimMentions map[string]ClaimMentionCheck `json:"claim_mentions,omitempty"`
	// SymbolAnchors is the live re-resolution of each cited symbol anchor,
	// present only from a live reader (DCP-V1-141).
	SymbolAnchors []SymbolAnchorCheck `json:"symbol_anchors,omitempty"`
	// ResultRepositories names the repository of each locate or recommend-tests result when the
	// artifact spans more than one repository (DCP-V1-311); absent otherwise.
	ResultRepositories map[string]string `json:"result_repositories,omitempty"`
	// TextOrigins marks each visible record's provider-supplied free text as
	// untrusted, with its byte cap; absent when no visible record carries
	// supplied text (DCP-V1-322).
	TextOrigins map[string]map[string]TextOrigin `json:"text_origins,omitempty"`
	// ResultKinds tags each visible result of a search that includes claims with
	// its record kind (DCP-V1-328).
	ResultKinds map[string]string `json:"result_kinds,omitempty"`
	// ResultRanks shows each visible result's place in a ranked search or
	// claim-text query: its position before paging, its tie group, its tier and
	// its normalized score (DCS-V1-479, DCS-V1-480). Absent for every other
	// operation, so their receipts keep their bytes.
	ResultRanks map[string]ResultRank `json:"result_ranks,omitempty"`
	// ClaimKinds labels each visible claim of a claims receipt with its claim
	// kind, read from the claim's typed details (DCS-V1-591); absent for every
	// other operation, so their receipts keep their bytes.
	ClaimKinds map[string]string `json:"claim_kinds,omitempty"`
	// ResultLocations lists each visible result's line anchors on a locate
	// query (DCP-V1-651); absent for every other operation.
	ResultLocations map[string][]ResultLocation `json:"result_locations,omitempty"`
	// ranks holds the rank of every ranked result before paging; the visible
	// page copies its own entries into ResultRanks.
	ranks map[string]ResultRank
	// matchClaim finds the claim whose lines a visible search result matched
	// (DCS-V1-581); it is evaluated for the visible page only.
	matchClaim func(Subject) (Claim, bool)
	// UnmatchedTerms lists the search query words that no record of the corpus
	// holds, in query order (DCS-V1-582). Absent when every word matched and
	// for every other operation, so their receipts keep their bytes.
	UnmatchedTerms []string `json:"unmatched_terms,omitempty"`
	// SearchQuality is an additive, versioned weak-match explanation. It never
	// suppresses hits or changes the existing receipt fields (DCP-V1-761).
	SearchQuality *SearchQuality `json:"search_quality,omitempty"`
	// AliasResolution records that the requested id was a subject's alternate
	// id (DCP-V1-330).
	AliasResolution *AliasResolution `json:"alias_resolution,omitempty"`
	// PathResolution records the file subject a trace or gaps path named (DCP-V1-602).
	PathResolution *PathResolution `json:"path_resolution,omitempty"`
}

// SearchQuality describes query coverage, not answer correctness (DCP-V1-761).
type SearchQuality struct {
	Schema             string   `json:"schema"`
	Outcome            string   `json:"outcome"`
	Reason             string   `json:"reason"`
	CorpusSubjects     int      `json:"corpus_subjects"`
	RareIDFThreshold   float64  `json:"rare_idf_threshold"`
	RareUnmatchedTerms []string `json:"rare_unmatched_terms"`
	// UnmatchedIDFShare is set only for reason unmatched-terms-carry-most-idf-mass: the share of
	// the query's distinct-word IDF mass its absent words carry, rounded to 4 places (DCI-V1-020).
	UnmatchedIDFShare float64 `json:"unmatched_idf_share,omitempty"`
}

// PathResolution names the repository and path a trace or gaps request gave and the native file
// subject ID it resolved to (DCP-V1-602).
type PathResolution struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	ID         string `json:"id"`
}

// ResultRank is one ranked result's place in its ranking (DCS-V1-479). Rank
// is the 1-based position before offset and limit. Results that share Group
// tie on every ranking key the scorer computes, and only the specificity and
// natural ID tie-breaks order them. Tier is exact, identifier, path-phrase,
// phrase, id-phrase, named or terms (DCS-V1-544). Score is the normalized term
// score in [0, 1], rounded to six decimals. Terms counts the distinct query
// words a result holds (DCS-V1-480, DCS-V1-540). MatchClaim and MatchAnchor
// name a subject's claim holding the most query words and its first anchor,
// when that anchor is not the subject's own first anchor, such as a
// document's matching paragraph (DCS-V1-581).
type ResultRank struct {
	Rank        int     `json:"rank"`
	Group       int     `json:"group"`
	Tier        string  `json:"tier"`
	Score       float64 `json:"score"`
	Terms       int     `json:"terms,omitempty"`
	MatchClaim  string  `json:"match_claim,omitempty"`
	MatchAnchor *Anchor `json:"match_anchor,omitempty"`
}

// LocateQueryKinds are the subject kinds a locate query ranks (DCP-V1-650).
var LocateQueryKinds = []string{"module", "symbol"}

// ResultLocation is one line anchor of a locate query result (DCP-V1-651).
type ResultLocation struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Start      int    `json:"start_line"`
	End        int    `json:"end_line"`
	Symbol     string `json:"symbol,omitempty"`
}

// TextOrigin is the origin and byte cap of one free-text field (DCP-V1-322).
type TextOrigin struct {
	Origin   string `json:"origin"`
	MaxBytes int    `json:"max_bytes"`
}

// AliasResolution names the alternate id a request used and the subject it
// resolved to (DCP-V1-330).
type AliasResolution struct {
	AlternateID string `json:"alternate_id"`
	ID          string `json:"id"`
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
