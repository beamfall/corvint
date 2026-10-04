package doccorpus

import (
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
var adoptionSubjectKinds = words("flow paragraph variation coverage_definition ticket intent_comparison inventory")
var adoptionRelationKinds = words("asserts navigates retired_to")

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
		kind := p.Details[r.ID].ClaimKind
		if kind == "" {
			kind = "unspecified"
		}
		counts["claim/"+kind]++
	}
	for _, r := range p.Relations {
		counts["relation/"+r.Type]++
	}
	counts["journey"] = len(p.Journeys)
	counts["observation"] = len(p.Observations)
	counts["typed_detail"] = len(p.Details)
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
		if adoptionSubjectKinds[s.Kind] && s.Kind != "variation" && s.Kind != "inventory" {
			if _, ok := p.Details[s.ID]; !ok {
				return fail("typed subject requires details")
			}
		}
	}
	for _, r := range p.Relations {
		if adoptionRelationKinds[r.Type] && r.Type != "retired_to" {
			if _, ok := p.Details[r.ID]; !ok {
				return fail("test relationship requires typed join details")
			}
		}
	}
	return nil
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
	if count != 1 {
		return fail("typed detail must name exactly one record family")
	}
	return nil
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
	for _, x := range c.artifact.Claims {
		kinds[x.ID] = "claim"
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
		if x := d.IntentComparison; x != nil {
			if !require(x.ReportedIntent, "") || !require(x.ObservedClaims, "claim") || !require(x.Missing, "") || !require(x.Conflicts, "") {
				return fail("intent comparison join missing")
			}
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
