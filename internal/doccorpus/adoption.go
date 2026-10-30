package doccorpus

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// RecordDetails preserves typed provider vocabulary beside normalized records.
// The normalized record's evidence axes govern every detail; none grants authority.
type RecordDetails struct {
	ClaimKind        string                   `json:"claim_kind,omitempty"`
	Flow             *FlowDetails             `json:"flow,omitempty"`
	Paragraph        *ParagraphDetails        `json:"paragraph,omitempty"`
	TestLink         *TestLinkDetails         `json:"test_link,omitempty"`
	Coverage         *CoverageDetails         `json:"coverage,omitempty"`
	Ticket           *TicketDetails           `json:"ticket,omitempty"`
	IntentComparison *IntentComparisonDetails `json:"intent_comparison,omitempty"`
	Status           *SuppliedStatusDetails   `json:"status,omitempty"`
	TestRun          *SuppliedTestRunDetails  `json:"test_run,omitempty"`
	LegacyCase       *LegacyCaseLinkDetails   `json:"legacy_case,omitempty"`
	MigrationMapping *MigrationMappingDetails `json:"migration_mapping,omitempty"`
	// CoverageLedger, FlowCompleteness and UnresolvedConcept are supplied
	// claim details about the claim's subject (DCP-V1-324 to DCP-V1-326).
	CoverageLedger    *CoverageLedgerDetails    `json:"coverage_ledger,omitempty"`
	FlowCompleteness  *FlowCompletenessDetails  `json:"flow_completeness,omitempty"`
	UnresolvedConcept *UnresolvedConceptDetails `json:"unresolved_concept,omitempty"`
}

// CoverageLedgerDetails is a producer's coverage-ledger status for the claim's
// subject (DCP-V1-324). It is supplied, not derived, and grants no trust.
type CoverageLedgerDetails struct {
	Scope           string   `json:"scope"`
	LedgerStatus    string   `json:"ledger_status"`
	BrowserEvidence string   `json:"browser_evidence,omitempty"`
	Journeys        *int     `json:"journeys,omitempty"`
	OrderedSteps    *int     `json:"ordered_steps,omitempty"`
	Verification    string   `json:"verification,omitempty"`
	Limitations     []string `json:"limitations,omitempty"`
}

// FlowCompletenessDetails is a producer's coverage and audit completeness
// flags for the claim's subject, with the revision and path that back them
// (DCP-V1-325). A true flag is supplied, never derived, and grants no trust.
type FlowCompletenessDetails struct {
	TargetStatus     string           `json:"target_status"`
	CoverageComplete CompletenessFlag `json:"coverage_complete"`
	AuditComplete    CompletenessFlag `json:"audit_complete"`
	Revision         string           `json:"revision"`
	Path             string           `json:"path"`
}

// CompletenessFlag is one supplied completeness boolean and its reason. Value
// is a pointer so an omitted or null value is refused rather than read as
// false (DCP-V1-325).
type CompletenessFlag struct {
	Value  *bool  `json:"value"`
	Reason string `json:"reason"`
}

// UnresolvedConceptDetails is a concept the producer anchored and reviewed but
// has not yet placed in the named targets (DCP-V1-326). It is not a gap.
type UnresolvedConceptDetails struct {
	Concept  string   `json:"concept"`
	Unplaced []string `json:"unplaced"`
	Reason   string   `json:"reason"`
}
type FlowDetails struct {
	Sections    []FlowSection `json:"sections"`
	Actors      []string      `json:"actors"`
	Entrypoints []string      `json:"entrypoints"`
	Variations  []string      `json:"variations"`
}
type FlowSection struct {
	Name       string   `json:"name"`
	Paragraphs []string `json:"paragraphs"`
}
type ParagraphDetails struct {
	Flow      string `json:"flow"`
	Section   string `json:"section"`
	StableID  string `json:"stable_id"`
	RetiredTo string `json:"retired_to,omitempty"`
}
type TestLinkDetails struct {
	Role           string `json:"role"`
	JoinConfidence string `json:"join_confidence"`
	File           string `json:"file"`
	Title          string `json:"title"`
	Project        string `json:"project"`
}
type CoverageDetails struct {
	Definition  string   `json:"definition"`
	Denominator []string `json:"denominator"`
	Numerator   []string `json:"numerator"`
	Rule        string   `json:"rule"`
}
type TicketDetails struct {
	ExternalID string          `json:"external_id"`
	Status     string          `json:"status"`
	History    []TicketHistory `json:"history"`
}
type TicketHistory struct {
	At       string `json:"at"`
	Revision string `json:"revision"`
	Summary  string `json:"summary"`
}
type IntentComparisonDetails struct {
	ReportedIntent []string `json:"reported_intent"`
	ObservedClaims []string `json:"observed_claims"`
	Missing        []string `json:"missing"`
	Conflicts      []string `json:"conflicts"`
}

// SuppliedStatusDetails is a producer's readiness or proof status for the
// subject of the claim it details (DCP-V1-187, DCP-V1-300). It is supplied,
// not derived: a "proven" state grants no trust and changes no
// compiler-computed result. A qualifier keeps a provisional or
// classification-only outcome apart from the plain state.
type SuppliedStatusDetails struct {
	State      string   `json:"state"`
	Qualifier  string   `json:"qualifier,omitempty"`
	Reason     string   `json:"reason"`
	Supporting []string `json:"supporting"`
}

// SuppliedTestRunDetails is a producer's typed receipt of one test run of the
// claim's subject (DCP-V1-210). It is supplied, not derived: Origin must be
// "supplied", and a "passed" outcome grants no trust and changes no
// compiler-computed result.
type SuppliedTestRunDetails struct {
	Outcome  string `json:"outcome"`
	Run      string `json:"run"`
	Origin   string `json:"origin"`
	Revision string `json:"revision,omitempty"`
	At       string `json:"at,omitempty"`
	// Optional run-ledger fields (DCP-V1-303), each from a closed set. They are
	// supplied, never derived, and grant no trust.
	Retries        *int                  `json:"retries,omitempty"`
	FlakeVerdict   string                `json:"flake_verdict,omitempty"`
	Cleanup        string                `json:"cleanup,omitempty"`
	ControlOutcome string                `json:"control_outcome,omitempty"`
	Validity       []SuppliedRunValidity `json:"validity,omitempty"`
}

// SuppliedRunValidity is one supplied validity axis of a test run (DCP-V1-303).
type SuppliedRunValidity struct {
	Axis   string `json:"axis"`
	Status string `json:"status"`
}

// MigrationMappingDetails types a migration_mapping relation between two
// inventory units of a code-migration ledger (DCP-V1-306). A one-sided
// outcome names the same unit at both ends. It is supplied, not derived.
type MigrationMappingDetails struct {
	Outcome string `json:"outcome"`
	Basis   string `json:"basis"`
	Run     string `json:"run,omitempty"`
}

// LegacyCaseLinkDetails names the external test-management case a
// manual_case subject stands for, as a case export prints its ID (DCP-V1-212).
// The corpus never reads the export, so the link is supplied, not verified.
type LegacyCaseLinkDetails struct {
	ID string `json:"id"`
}

// RestrictedFinding exists only in the local input. Its identity, original
// detail path and body are never admitted to an artifact or diagnostic.
type RestrictedFinding struct {
	ID              string `json:"id"`
	Severity        string `json:"severity"`
	LocalDetailPath string `json:"local_detail_path"`
	Detail          string `json:"detail"`
}
type RestrictedSummary struct {
	Provider   string         `json:"provider"`
	Total      int            `json:"total"`
	BySeverity map[string]int `json:"by_severity"`
}
type ImportParity struct {
	Provider   string   `json:"provider"`
	SourceKind string   `json:"source_kind"`
	RecordsIn  int      `json:"records_in"`
	Admitted   int      `json:"admitted"`
	Dropped    int      `json:"dropped"`
	Reasons    []string `json:"reasons"`
}

var flowSections = []string{"readme", "entry-points", "flow-diagram", "functional-overview", "technical-deep-dive", "entities", "exceptions", "related-flows"}
var adoptionSubjectKinds = words("flow paragraph variation coverage_definition ticket intent_comparison inventory manual_case")
var adoptionRelationKinds = words("asserts navigates retired_to manual_case_for migration_mapping")

// suppliedStatusStates is the closed producer status vocabulary (DCP-V1-187,
// widened by DCP-V1-300); suppliedStatusQualifiers is its closed optional
// qualifier set.
var (
	SuppliedStatusStates     = []string{"blocked", "ready", "proven", "partial", "not-demonstrable", "disputed", "obsolete", "not-established", "unknown"}
	SuppliedStatusQualifiers = []string{"provisional", "classification-only"}
	suppliedStatusStates     = words(strings.Join(SuppliedStatusStates, " "))
	suppliedStatusQualifiers = words(strings.Join(SuppliedStatusQualifiers, " "))
)

// ValidStatusFilter reports whether a claims status filter names a supplied
// state, an absent or supplied qualifier, and a run of at most 1,024 bytes
// (DCP-V1-318).
func ValidStatusFilter(state, qualifier, run string) bool {
	return suppliedStatusStates[state] && (qualifier == "" || suppliedStatusQualifiers[qualifier]) && len(run) <= maxSuppliedRunIdentifier
}

// Closed optional test-run ledger vocabularies (DCP-V1-303).
var (
	suppliedFlakeVerdicts  = words("stable flaky unqualified unknown")
	suppliedRunCleanups    = words("passed failed partial not-run unknown")
	suppliedValidityAxes   = words("revision environment isolation oracle determinism association")
	suppliedValidityStatus = words("passed failed unknown not-applicable")
)

// Closed migration mapping vocabularies (DCP-V1-306). The one-sided outcomes
// name the same unit at both ends of the relation; every other outcome joins
// two distinct units.
var (
	migrationOutcomes = words("unchanged modified moved renamed copied added removed ambiguous unresolved")
	migrationOneSided = words("added removed unresolved")
	migrationBases    = words("stable_id region_hash content_hash path symbol declared unknown")
)

const (
	maxSuppliedRunRetries    = 1 << 20
	maxSuppliedRunIdentifier = 1024
)

// Closed supplied coverage-detail vocabularies and bounds (DCP-V1-324 to
// DCP-V1-326).
var (
	suppliedLedgerScopes        = words("ui non_ui")
	suppliedLedgerStatuses      = words("confirmed partial not-browser")
	suppliedLedgerVerifications = words("verified partial unverified unknown")
	suppliedUnplacedTargets     = words("ui-location flow")
)

const (
	maxSuppliedLabel           = 128
	maxSuppliedLedgerCount     = 1 << 20
	maxSuppliedLedgerLimits    = 64
	maxSuppliedConcept         = 1024
	maxSuppliedLimitationBytes = 4096
)

// suppliedTestRunOutcomes is the closed test-run outcome vocabulary (DCP-V1-210).
var suppliedTestRunOutcomes = words("passed failed skipped errored")

// ClaimKinds is the closed claim kind vocabulary a claims receipt labels and
// a claims kind filter names (DCS-V1-591, DCS-V1-592): the supplied record
// kinds, the supplied ClaimKind values, and "unspecified" for a claim whose
// details name neither.
var ClaimKinds = []string{"behavior", "constraint", "side_effect", "integration", "permission", "status", "test_run", "coverage_ledger", "flow_completeness", "unresolved_concept", "unspecified"}

// ValidClaimKinds reports whether kinds is absent, or a bounded list of
// distinct claim kinds.
func ValidClaimKinds(kinds []string) bool {
	if kinds == nil {
		return true
	}
	if len(kinds) == 0 || len(kinds) > len(ClaimKinds) {
		return false
	}
	for i, k := range kinds {
		if !slices.Contains(ClaimKinds, k) || slices.Contains(kinds[:i], k) {
			return false
		}
	}
	return true
}

// claimParityKind is the import_parity claim kind of a claim's typed detail,
// shared by cold import and warm reuse so both count the same kinds.
func claimParityKind(d RecordDetails) string {
	switch {
	case d.Status != nil:
		return "status"
	case d.TestRun != nil:
		return "test_run"
	case d.CoverageLedger != nil:
		return "coverage_ledger"
	case d.FlowCompleteness != nil:
		return "flow_completeness"
	case d.UnresolvedConcept != nil:
		return "unresolved_concept"
	case d.ClaimKind != "":
		return d.ClaimKind
	}
	return "unspecified"
}

func (c *compiler) importAdoption(p ProviderRecord) error {
	if c.adoptionProviders == nil {
		c.adoptionProviders = map[string]bool{}
	}
	c.adoptionProviders[p.ID] = true
	if c.artifact.Details == nil {
		c.artifact.Details = map[string]RecordDetails{}
	}
	for id, d := range p.Details {
		c.artifact.Details[id] = d
	}
	counts := map[string]int{}
	for _, s := range p.Subjects {
		counts["subject/"+s.Kind]++
	}
	for _, r := range p.Claims {
		counts["claim/"+claimParityKind(p.Details[r.ID])]++
	}
	for _, r := range p.Relations {
		counts["relation/"+r.Type]++
	}
	counts["journey"] = len(p.Journeys)
	counts["observation"] = len(p.Observations)
	counts["typed_detail"] = len(p.Details)
	if len(p.Gaps) > 0 {
		counts["supplied_gap"] = len(p.Gaps)
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c.artifact.ImportParity = append(c.artifact.ImportParity, ImportParity{Provider: p.ID, SourceKind: k, RecordsIn: counts[k], Admitted: counts[k], Reasons: []string{}})
	}

	return nil
}

// validateAdoption preserves the original detail/ownership predicates for both
// cold imports and source-bound warm correspondence, without producing records.
func (c *compiler) validateAdoption(p ProviderRecord) error {
	kinds := map[string]string{}
	relations := map[string]Relation{}
	for _, s := range p.Subjects {
		if s.Kind == "finding" {
			return fail("restricted findings require their separate local-only family")
		}
		kinds[s.ID] = s.Kind
	}
	for _, claim := range p.Claims {
		kinds[claim.ID] = "claim"
	}
	for _, rel := range p.Relations {
		kinds[rel.ID] = "relation"
		relations[rel.ID] = rel
	}
	if len(p.Details) > MaxCorpusRecords-len(c.artifact.Details) {
		return fail("typed detail bound exceeded")
	}
	for id, d := range p.Details {
		if !strings.HasPrefix(id, p.ID+":") || kinds[id] == "" {
			return fail("typed detail has no owned normalized record")
		}
		if _, exists := c.artifact.Details[id]; exists {
			return fail("duplicate typed detail")
		}
		if err := validateRecordDetails(kinds[id], d, relations[id]); err != nil {
			return err
		}
	}
	for _, s := range p.Subjects {
		if typedSubjectNeedsDetails(s.Kind) {
			if _, ok := p.Details[s.ID]; !ok {
				return fail("typed subject requires details")
			}
		}
	}
	for _, r := range p.Relations {
		if typedRelationNeedsDetails(r.Type) {
			if _, ok := p.Details[r.ID]; !ok && r.Type == "migration_mapping" {
				return fail("migration mapping requires typed details")
			} else if !ok {
				return fail("test relationship requires typed join details")
			}
		}
	}
	return nil
}

// typedSubjectNeedsDetails and typedRelationNeedsDetails name the adoption kinds an adoption
// provider must give typed details; the delta applies the same rule (DCX-V1-441).
func typedSubjectNeedsDetails(kind string) bool {
	return adoptionSubjectKinds[kind] && kind != "variation" && kind != "inventory" && kind != "manual_case"
}

func typedRelationNeedsDetails(kind string) bool {
	return adoptionRelationKinds[kind] && kind != "retired_to" && kind != "manual_case_for"
}

func validateRecordDetails(kind string, d RecordDetails, rel Relation) error {
	raw, err := Encode(d)
	if err != nil || len(raw) > 1<<20 {
		return fail("typed detail byte bound exceeded")
	}
	count := 0
	if d.ClaimKind != "" {
		count++
		if kind != "claim" || !words("behavior constraint side_effect integration permission")[d.ClaimKind] {
			return fail("invalid typed claim")
		}
	}
	if d.Flow != nil {
		count++
		if kind != "flow" || len(d.Flow.Sections) != len(flowSections) || !boundedUnique(d.Flow.Actors, 4096) || !boundedUnique(d.Flow.Entrypoints, 4096) || !boundedUnique(d.Flow.Variations, MaxCorpusRecords) {
			return fail("invalid flow detail")
		}
		for i, section := range d.Flow.Sections {
			if section.Name != flowSections[i] || !boundedUnique(section.Paragraphs, MaxCorpusRecords) {
				return fail("flow sections must use the complete canonical order")
			}
		}
	}
	if d.Paragraph != nil {
		count++
		p := d.Paragraph
		if kind != "paragraph" || !identifier(p.StableID) || !identifier(p.Flow) || !words(strings.Join(flowSections, " "))[p.Section] || p.RetiredTo != "" && !identifier(p.RetiredTo) {
			return fail("invalid stable paragraph detail")
		}
	}
	if d.TestLink != nil {
		count++
		x := d.TestLink
		if kind != "relation" || !words("asserting navigating")[x.Role] || (rel.Type == "asserts") != (x.Role == "asserting") || !words("asserts navigates")[rel.Type] || !words("exact declared heuristic unknown")[x.JoinConfidence] || !validPath(x.File) || !textOK(x.Title) || !textOK(x.Project) {
			return fail("invalid typed test join")
		}
	}
	if d.Coverage != nil {
		count++
		x := d.Coverage
		if kind != "coverage_definition" || !textOK(x.Definition) || x.Rule != "explicit-membership" || !boundedUnique(x.Denominator, MaxCorpusRecords) || !boundedUnique(x.Numerator, MaxCorpusRecords) {
			return fail("invalid reproducible coverage definition")
		}
		denom := map[string]bool{}
		for _, id := range x.Denominator {
			denom[id] = true
		}
		for _, id := range x.Numerator {
			if !denom[id] {
				return fail("coverage numerator outside denominator")
			}
		}
	}
	if d.Ticket != nil {
		count++
		x := d.Ticket
		if kind != "ticket" || !textOK(x.ExternalID) || !textOK(x.Status) || len(x.History) > 256 {
			return fail("invalid historical ticket")
		}
		for _, h := range x.History {
			if _, err := time.Parse(time.RFC3339, h.At); err != nil || !wire.IsGitOid(h.Revision) || !textOK(h.Summary) {
				return fail("invalid ticket history")
			}
		}
	}
	if d.IntentComparison != nil {
		count++
		x := d.IntentComparison
		if kind != "intent_comparison" || !boundedUnique(x.ReportedIntent, MaxCorpusRecords) || !boundedUnique(x.ObservedClaims, MaxCorpusRecords) || !boundedUnique(x.Missing, MaxCorpusRecords) || !boundedUnique(x.Conflicts, MaxCorpusRecords) {
			return fail("invalid intent comparison")
		}
	}
	if d.Status != nil {
		count++
		x := d.Status
		if kind != "claim" || !suppliedStatusStates[x.State] || x.Qualifier != "" && !suppliedStatusQualifiers[x.Qualifier] || !textOK(x.Reason) || !boundedUnique(x.Supporting, 256) {
			return fail("invalid supplied status")
		}
	}
	if d.TestRun != nil {
		count++
		x := d.TestRun
		if kind != "claim" || !suppliedTestRunOutcomes[x.Outcome] || !textOK(x.Run) || len(x.Run) > maxSuppliedRunIdentifier || x.Origin != "supplied" || x.Revision != "" && !wire.IsGitOid(x.Revision) {
			return fail("invalid supplied test run")
		}
		if !validRunLedger(x) {
			return fail("invalid supplied test run ledger field")
		}
		if _, err := time.Parse(time.RFC3339, x.At); x.At != "" && err != nil {
			return fail("invalid supplied test run time")
		}
	}
	if d.LegacyCase != nil {
		count++
		if id, ok := legacyCaseID(d.LegacyCase.ID); kind != "manual_case" || !ok || id != d.LegacyCase.ID {
			return fail("invalid legacy case link")
		}
	}
	if d.MigrationMapping != nil {
		count++
		x := d.MigrationMapping
		if kind != "relation" || rel.Type != "migration_mapping" || !migrationOutcomes[x.Outcome] || !migrationBases[x.Basis] || x.Run != "" && (!textOK(x.Run) || len(x.Run) > maxSuppliedRunIdentifier) || migrationOneSided[x.Outcome] != (rel.From == rel.To) {
			return fail("invalid migration mapping")
		}
	}
	if d.CoverageLedger != nil {
		count++
		x := d.CoverageLedger
		if kind != "claim" || !suppliedLedgerScopes[x.Scope] || !suppliedLedgerStatuses[x.LedgerStatus] || x.BrowserEvidence != "" && !suppliedLabel(x.BrowserEvidence) || !ledgerCount(x.Journeys) || !ledgerCount(x.OrderedSteps) || x.Verification != "" && !suppliedLedgerVerifications[x.Verification] || x.Limitations != nil && (len(x.Limitations) == 0 || !boundedUnique(x.Limitations, maxSuppliedLedgerLimits)) {
			return fail("invalid supplied coverage ledger")
		}
		for _, l := range x.Limitations {
			if len(l) > maxSuppliedLimitationBytes {
				return fail("invalid supplied coverage ledger")
			}
		}
	}
	if d.FlowCompleteness != nil {
		count++
		x := d.FlowCompleteness
		if kind != "claim" || !suppliedLabel(x.TargetStatus) || x.CoverageComplete.Value == nil || x.AuditComplete.Value == nil || !textOK(x.CoverageComplete.Reason) || !textOK(x.AuditComplete.Reason) || !wire.IsGitOid(x.Revision) || !validPath(x.Path) {
			return fail("invalid supplied flow completeness")
		}
	}
	if d.UnresolvedConcept != nil {
		count++
		x := d.UnresolvedConcept
		if kind != "claim" || !textOK(x.Concept) || len(x.Concept) > maxSuppliedConcept || !textOK(x.Reason) || len(x.Unplaced) == 0 || !boundedUnique(x.Unplaced, len(suppliedUnplacedTargets)) {
			return fail("invalid supplied unresolved concept")
		}
		for _, t := range x.Unplaced {
			if !suppliedUnplacedTargets[t] {
				return fail("invalid supplied unresolved concept")
			}
		}
	}
	if count != 1 {
		return fail("typed detail must name exactly one record family")
	}
	return nil
}

// suppliedLabel admits a supplied open-vocabulary label of at most 128 bytes.
func suppliedLabel(v string) bool { return textOK(v) && len(v) <= maxSuppliedLabel }

// ledgerCount admits an absent count or one in [0, 2^20].
func ledgerCount(n *int) bool { return n == nil || *n >= 0 && *n <= maxSuppliedLedgerCount }

// validRunLedger admits the optional test-run ledger fields only from their
// closed sets (DCP-V1-303): retries in [0, 2^20], each validity axis at most
// once, and no empty validity list.
func validRunLedger(x *SuppliedTestRunDetails) bool {
	if x.Retries != nil && (*x.Retries < 0 || *x.Retries > maxSuppliedRunRetries) {
		return false
	}
	if x.FlakeVerdict != "" && !suppliedFlakeVerdicts[x.FlakeVerdict] || x.Cleanup != "" && !suppliedRunCleanups[x.Cleanup] || x.ControlOutcome != "" && !suppliedTestRunOutcomes[x.ControlOutcome] {
		return false
	}
	if x.Validity != nil && len(x.Validity) == 0 {
		return false
	}
	axes := map[string]bool{}
	for _, v := range x.Validity {
		if !suppliedValidityAxes[v.Axis] || !suppliedValidityStatus[v.Status] || axes[v.Axis] {
			return false
		}
		axes[v.Axis] = true
	}
	return true
}
func boundedUnique(values []string, limit int) bool {
	if len(values) > limit {
		return false
	}
	seen := map[string]bool{}
	for _, v := range values {
		if !textOK(v) || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

func (c *compiler) validateAdoptionJoins() error {
	kinds := map[string]string{}
	subjects := map[string]Subject{}
	relations := map[string]Relation{}
	for _, s := range c.artifact.Subjects {
		kinds[s.ID] = s.Kind
		subjects[s.ID] = s
	}
	anchored := map[string]bool{}
	for _, x := range c.artifact.Claims {
		kinds[x.ID] = "claim"
		anchored[x.ID] = len(x.Evidence.Anchors) > 0
	}
	for _, x := range c.artifact.Relations {
		kinds[x.ID] = "relation"
		relations[x.ID] = x
	}
	require := func(ids []string, kind string) bool {
		for _, id := range ids {
			if kinds[id] == "" || kind != "" && kinds[id] != kind {
				return false
			}
		}
		return true
	}
	stable := map[string]bool{}
	listed := map[string]bool{}
	for _, d := range c.artifact.Details {
		if d.Flow != nil {
			for _, section := range d.Flow.Sections {
				for _, id := range section.Paragraphs {
					listed[id] = true
				}
			}
		}
	}
	for id, d := range c.artifact.Details {
		if d.Flow != nil {
			for _, section := range d.Flow.Sections {
				for _, paragraph := range section.Paragraphs {
					p := c.artifact.Details[paragraph].Paragraph
					if p == nil || p.Flow != id || p.Section != section.Name {
						return fail("flow section paragraph join mismatch")
					}
				}
			}
			if !require(d.Flow.Variations, "variation") || !require(d.Flow.Entrypoints, "") {
				return fail("flow inventory join missing")
			}
		}
		if p := d.Paragraph; p != nil {
			if stable[p.StableID] || kinds[p.Flow] != "flow" || p.RetiredTo == id || p.RetiredTo != "" && kinds[p.RetiredTo] != "paragraph" {
				return fail("paragraph identity or retirement join mismatch")
			}
			stable[p.StableID] = true
			if p.RetiredTo == "" && !listed[id] {
				return fail("active paragraph absent from flow section")
			}
		}
		if d.Coverage != nil && !require(d.Coverage.Denominator, "") {
			return fail("coverage denominator join missing")
		}
		if x := d.TestLink; x != nil {
			r := relations[id]
			target := subjects[r.To]
			if target.Kind != "test" {
				return fail("test join target is not a test")
			}
			if x.JoinConfidence == "exact" {
				found := false
				for _, a := range target.Evidence.Anchors {
					found = found || a.Path == x.File
				}
				if !found {
					return fail("exact test join has no file anchor")
				}
			}
		}
		// DCP-V1-326: an unresolved concept is anchored in source.
		if d.UnresolvedConcept != nil && !anchored[id] {
			return fail("supplied unresolved concept has no anchor")
		}
		if x := d.Status; x != nil && !require(x.Supporting, "") {
			return fail("supplied status supporting record missing")
		}
		if x := d.IntentComparison; x != nil {
			if !require(x.ReportedIntent, "") || !require(x.ObservedClaims, "claim") || !require(x.Missing, "") || !require(x.Conflicts, "") {
				return fail("intent comparison join missing")
			}
		}
	}
	// A legacy case ID names at most one manual case subject (DCP-V1-212).
	cases := map[string]bool{}
	for _, d := range c.artifact.Details {
		if x := d.LegacyCase; x != nil {
			if cases[x.ID] {
				return fail("legacy case linked by more than one manual case")
			}
			cases[x.ID] = true
		}
	}
	// A manual case link joins an imported case subject to any subject (DCP-V1-189).
	// A migration mapping joins two inventory units (DCP-V1-306).
	for _, r := range c.artifact.Relations {
		if r.Type == "manual_case_for" && kinds[r.From] != "manual_case" {
			return fail("manual case link source is not a manual case")
		}
		if r.Type == "migration_mapping" && (kinds[r.From] != "inventory" || kinds[r.To] != "inventory") {
			return fail("migration mapping must join inventory subjects")
		}
	}
	// Each retirement edge is visited once, including long imported histories.
	visited := map[string]uint8{}
	for id, d := range c.artifact.Details {
		if d.Paragraph == nil || visited[id] == 2 {
			continue
		}
		path := []string{}
		for next := id; next != ""; {
			if visited[next] == 1 {
				return fail("paragraph retirement cycle")
			}
			if visited[next] == 2 {
				break
			}
			p := c.artifact.Details[next].Paragraph
			if p == nil {
				return fail("paragraph retirement target missing")
			}
			visited[next] = 1
			path = append(path, next)
			next = p.RetiredTo
		}
		for _, id := range path {
			visited[id] = 2
		}
	}

	return nil
}

func (c *compiler) guardRestrictedOutput() error {
	if len(c.restrictedTokens) == 0 {
		return nil
	}
	// Scan one encoded public projection with a trie-based replacer. This also
	// covers manifest paths, map keys, gaps and diagnostic strings across providers.
	raw, err := Encode(c.artifact)
	if err != nil {
		return err
	}
	pairs := []string{}
	seen := map[string]bool{}
	for _, value := range c.restrictedTokens {
		encoded, err := Encode(value)
		if err != nil {
			return fail("restricted token exceeds bound")
		}
		token := string(encoded[1 : len(encoded)-2])
		if !seen[token] {
			seen[token] = true
			pairs = append(pairs, token, "")
		}
	}
	if strings.NewReplacer(pairs...).Replace(string(raw)) != string(raw) {
		return fail("restricted input referenced by public corpus")
	}
	return nil
}

func (c *compiler) registerRestricted(p ProviderRecord) error {
	if len(p.RestrictedFindings) > 1024 {
		return fail("restricted input count exceeded")
	}
	summary := RestrictedSummary{Provider: p.ID, BySeverity: map[string]int{}}
	if c.restrictedIDs == nil {
		c.restrictedIDs = map[string]bool{}
	}
	for _, r := range p.RestrictedFindings {
		if !textOK(r.ID) || !validPath(r.LocalDetailPath) || !textOK(r.Detail) || !words("critical high medium low informational unknown")[r.Severity] {
			return fail("invalid restricted input")
		}
		c.restrictedBytes += len(r.ID) + len(r.LocalDetailPath) + len(r.Detail)
		if c.restrictedBytes > 1<<20 {
			return fail("restricted input byte bound exceeded")
		}
		if c.restrictedIDs[r.ID] {
			return fail("duplicate restricted input identity")
		}
		c.restrictedIDs[r.ID] = true
		c.restrictedTokens = append(c.restrictedTokens, r.ID, r.LocalDetailPath, r.Detail)
		summary.Total++
		summary.BySeverity[r.Severity]++
	}
	if summary.Total > 0 {
		c.artifact.RestrictedSummaries = append(c.artifact.RestrictedSummaries, summary)
	}
	if summary.Total > 0 {
		c.artifact.ImportParity = append(c.artifact.ImportParity, ImportParity{Provider: p.ID, SourceKind: "restricted_finding", RecordsIn: summary.Total, Dropped: summary.Total, Reasons: []string{"restricted-local-only"}})
	}
	return nil
}
