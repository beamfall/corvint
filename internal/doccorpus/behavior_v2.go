package doccorpus

import (
	"slices"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const BehaviorProviderRequestSchemaV2 = "corvint-behavior-provider-request/2"

type BehaviorProviderRequestV2 struct {
	Schema   string           `json:"schema"`
	ID       string           `json:"provider_id"`
	Version  string           `json:"provider_version"`
	Source   Repository       `json:"source"`
	Registry BehaviorRegistry `json:"registry"`
}

// BuildBehaviorProviderV2 emits declarations only. Repository identities and
// anchors remain caller claims until the corpus boundary checks its inputs.
func BuildBehaviorProviderV2(raw []byte) (ProviderRecord, error) {
	var request BehaviorProviderRequestV2
	if err := decode(raw, &request); err != nil {
		return ProviderRecord{}, err
	}
	if request.Schema != BehaviorProviderRequestSchemaV2 || request.Registry.ContractSHA256 != "" {
		return ProviderRecord{}, fail("invalid /2 producer request or pre-signed registry")
	}
	r := request.Registry
	sort.Slice(r.Repositories, func(i, j int) bool { return r.Repositories[i].RootCommit < r.Repositories[j].RootCommit })
	declarations := behaviorV2Declarations(r)
	digest, err := hashValue(declarations)
	if err != nil {
		return ProviderRecord{}, err
	}
	r.ContractSHA256 = digest
	p := ProviderRecord{Schema: BehaviorProviderSchemaV2, ID: request.ID, Version: request.Version, Source: request.Source,
		BehaviorContracts: &r, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}, Capabilities: []CapabilityDeclaration{}}
	if err := ValidateBehaviorProviderV2(p); err != nil {
		return ProviderRecord{}, err
	}
	return p, nil
}

func behaviorV2Declarations(r BehaviorRegistry) BehaviorRegistry {
	r.ContractSHA256 = ""
	r.Tests = slices.Clone(r.Tests)
	for i := range r.Tests {
		r.Tests[i].Runtime = nil
	}
	r.Legacy = slices.Clone(r.Legacy)
	for i := range r.Legacy {
		r.Legacy[i].Runtime = nil
	}
	return r
}

type behaviorV2Anchor struct {
	subject string
	anchor  Anchor
}

func behaviorV2Anchors(r *BehaviorRegistry) []behaviorV2Anchor {
	out := []behaviorV2Anchor{{r.ContractID, r.Discovery}, {r.ContractID, r.Manifest}}
	for _, f := range r.Flows {
		out = append(out, behaviorV2Anchor{f.ID, f.Evidence})
		if f.MissingReview != nil {
			out = append(out, behaviorV2Anchor{f.ID, *f.MissingReview})
		}
	}
	for _, b := range r.Behaviors {
		out = append(out, behaviorV2Anchor{b.ID, b.Evidence})
	}
	for _, t := range r.Tests {
		out = append(out, behaviorV2Anchor{t.ID, t.Evidence})
		for _, a := range t.Assertions {
			out = append(out, behaviorV2Anchor{t.ID, a.Annotation})
		}
		if t.Runtime != nil {
			out = append(out, behaviorV2Anchor{t.ID, t.Runtime.Evidence})
		}
	}
	return out
}

func validateBehaviorV2Declarations(p ProviderRecord) error {
	r := p.BehaviorContracts
	if !identifier(p.ID) || !textOK(p.Version) || r.Schema != 2 || !textOK(r.ContractID) || !wire.IsGitOid(r.DocumentationRevision) {
		return fail("invalid behavior /2 declaration identity")
	}
	// /2 currently admits the behavior registry only. The independent /1 legacy
	// runtime and stability profiles require their own multi-repository contracts.
	if len(r.Legacy) > 0 || len(p.Subjects)+len(p.Claims)+len(p.Relations)+len(p.Journeys)+len(p.Observations)+len(p.Capabilities) > 0 {
		return fail("unsupported behavior /2 record family")
	}
	if len(r.Flows)+len(r.Behaviors)+len(r.Tests) > MaxRecords {
		return fail("behavior /2 record bound exceeded")
	}
	eventCount := 0
	ids := []string{}
	for _, f := range r.Flows {
		ids = append(ids, f.ID)
		eventCount += len(f.OrderedEvents)
		if eventCount > MaxRecords {
			return fail("behavior /2 event bound exceeded")
		}
		for _, event := range f.OrderedEvents {
			if !textOK(event.ID) || !textOK(event.Kind) || event.Sequence < 0 {
				return fail("invalid behavior /2 event")
			}
		}
		if !words("generated source-derived declared imported")[f.Derivation] || !uniqueIdentities(f.Criteria) || !uniqueIdentities(f.Tests) || !uniqueIdentities(f.RequiredPages) || !uniqueIdentities(f.NegativeControls) || len(f.OrderedEvents) > MaxRecords {
			return fail("invalid behavior /2 flow")
		}
	}
	for _, b := range r.Behaviors {
		ids = append(ids, b.ID)
		if !uniqueIdentities(b.Flows) {
			return fail("invalid behavior /2 source links")
		}
	}
	anchors := 2 + len(r.Flows)*2 + len(r.Behaviors)
	for _, t := range r.Tests {
		ids = append(ids, t.ID)
		anchors += 2 + len(t.Assertions)
		if anchors > MaxRecords || !textOK(t.Project) || !textOK(t.Title) || !uniqueIdentities(t.Flows) || !uniqueIdentities(t.Criteria) || !uniqueIdentities(t.Fixtures) || !uniqueIdentities(t.Roles) || len(t.Legacy) > 0 {
			return fail("invalid behavior /2 test")
		}
		aids := []string{}
		for _, a := range t.Assertions {
			aids = append(aids, a.ID)
			if !textOK(a.Behavior) || !textOK(a.Criterion) || !textOK(a.Matcher) || !textOK(a.Locator) || !textOK(a.Value) {
				return fail("invalid behavior /2 assertion")
			}
		}
		if !uniqueIdentities(aids) {
			return fail("duplicate behavior /2 assertion")
		}
		if t.Runtime != nil && !textOK(t.Runtime.Observation) {
			return fail("invalid behavior /2 runtime reference")
		}
	}
	if !uniqueIdentities(ids) {
		return fail("duplicate behavior /2 identity")
	}
	if anchors > MaxRecords {
		return fail("behavior /2 anchor bound exceeded")
	}
	for _, row := range behaviorV2Anchors(r) {
		a := row.anchor
		if !wire.IsGitOid(a.Repository) || !wire.IsGitOid(a.Revision) || !validPath(a.Path) || !wire.IsGitOid(a.Blob) || !wire.IsSha256(a.SHA256) || !wire.IsSha256(a.SpanSHA256) || a.Start < 1 || a.End < a.Start || a.End-a.Start >= MaxRecords || a.Authority != "external-provider" || !words("source declared observed review imported")[a.Kind] || !textOK(a.Reason) {
			return fail("invalid behavior /2 anchor")
		}
	}
	return nil
}

// importBehaviorV2 retains the /2 wire and checks only evidence available at
// this repository boundary. It never runs the fixed-triplet /1 reconciler.
func (c *compiler) importBehaviorV2(p ProviderRecord) error {
	if err := ValidateBehaviorProviderV2(p); err != nil {
		return err
	}
	r := p.BehaviorContracts
	expected := map[string]string{}
	for _, repo := range c.manifest.BehaviorRepositories {
		expected[repo.RootCommit] = repo.Revision
	}
	members := map[string]string{}
	for _, repo := range r.Repositories {
		members[repo.RootCommit] = repo.Revision
		if revision, ok := expected[repo.RootCommit]; !ok {
			c.behaviorGap(repo.RootCommit, "repository-expectation-missing", "/2 member has no independently declared expected revision")
		} else if revision != repo.Revision {
			c.behaviorGap(repo.RootCommit, "stale-repository-member", "/2 member revision differs from the manifest expectation")
		}
		if repo.RootCommit != c.manifest.Repository.ID {
			c.behaviorGap(repo.RootCommit, "repository-evidence-unavailable", "external repository bytes and live revision were not observed by this local corpus")
		}
	}
	for _, repo := range c.manifest.BehaviorRepositories {
		if _, ok := members[repo.RootCommit]; !ok {
			c.behaviorGap(repo.RootCommit, "repository-member-missing", "expected repository is absent from the /2 registry")
		}
	}
	docsPresent := false
	for _, repo := range r.Repositories {
		docsPresent = docsPresent || repo.Revision == r.DocumentationRevision
	}
	if !docsPresent {
		c.behaviorGap(p.ID, "repository-member-missing", "documentation revision has no /2 repository member")
	}
	for _, row := range behaviorV2Anchors(r) {
		a := row.anchor
		revision, ok := members[a.Repository]
		if !ok {
			c.behaviorGap(row.subject, "repository-member-missing", "anchor repository is absent from /2 members")
			continue
		}
		if a.Revision != revision {
			c.behaviorGap(row.subject, "stale-repository-anchor", "anchor revision differs from its /2 member")
			continue
		}
		if a.Repository != c.manifest.Repository.ID {
			continue
		}
		if err := c.checkAnchor(a, true); err != nil {
			return err
		}
	}
	flows, tests, behaviors := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, f := range r.Flows {
		flows[f.ID] = true
	}
	for _, t := range r.Tests {
		tests[t.ID] = true
	}
	for _, b := range r.Behaviors {
		behaviors[b.ID] = true
	}
	for _, f := range r.Flows {
		for _, id := range f.Tests {
			if !tests[id] {
				c.behaviorGap(f.ID, "unreviewed-join", "flow names an unavailable test")
			}
		}
	}
	for _, b := range r.Behaviors {
		for _, id := range b.Flows {
			if !flows[id] {
				c.behaviorGap(b.ID, "unreviewed-join", "source behavior names an unavailable flow")
			}
		}
	}
	for _, t := range r.Tests {
		for _, id := range t.Flows {
			if !flows[id] {
				c.behaviorGap(t.ID, "unreviewed-join", "test names an unavailable flow")
			}
		}
		for _, a := range t.Assertions {
			if !behaviors[a.Behavior] {
				c.behaviorGap(t.ID, "unreviewed-join", "assertion names an unavailable source behavior")
			}
		}
	}
	c.behaviorGap(p.ID, "runtime-unqualified", "/2 declarations do not qualify discovery, migration or runtime witnesses; full relevant suite remains required")
	c.artifact.BehaviorContracts = append(c.artifact.BehaviorContracts, BehaviorReport{ProviderSchema: BehaviorProviderSchemaV2, Provider: p.ID, Registry: *r,
		VerifiedTests: []string{}, LinkedFlows: []string{}, LinkedBehaviors: []string{}, VerifiedFlows: []string{}, LegacyRuntimeParity: []string{}, Fallback: "full-relevant-suite",
		Limitations: []string{"/2 repository membership is declared; external bytes and live freshness remain unobserved", "local anchor validation is structural only", "no runtime, semantic adequacy, Core or narrowing authority"}})
	return nil
}
