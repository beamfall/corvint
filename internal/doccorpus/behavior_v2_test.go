package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

func behaviorV2Fixture(t *testing.T, edit func(*BehaviorProviderRequestV2), expectedEdit func(*Manifest)) (string, Manifest, []byte) {
	t.Helper()
	root, m := fixture(t)
	a := declaredEvidence(t, root, m, "src/readme.md", 3).Anchors[0]
	local := BehaviorRepository{m.Repository.ID, m.Repository.Revision}
	request := BehaviorProviderRequestV2{Schema: BehaviorProviderRequestSchemaV2, ID: "behavior", Version: "2", Source: m.Repository,
		Registry: BehaviorRegistry{Schema: 2, ContractID: "count", SourceRevision: m.Repository.Revision, DocumentationRevision: m.Repository.Revision,
			Repositories: []BehaviorRepository{{strings.Repeat("1", 40), strings.Repeat("8", 40)}, local, {strings.Repeat("2", 40), strings.Repeat("9", 40)}, {strings.Repeat("3", 40), strings.Repeat("7", 40)}}, Discovery: a, Manifest: a,
			Flows:     []BehaviorFlow{{ID: "count-flow", Derivation: "declared", Evidence: a, Criteria: []string{"one"}, Tests: []string{"count-test"}, RequiredPages: []string{}, NegativeControls: []string{}, OrderedEvents: []BehaviorEvent{}}},
			Behaviors: []BehaviorSource{{ID: "count-source", Evidence: a, Flows: []string{"count-flow"}}},
			Tests:     []BehaviorTest{{ID: "count-test", Project: "chromium", Title: "count", Evidence: a, Flows: []string{"count-flow"}, Criteria: []string{"one"}, Assertions: []BehaviorAssertion{}}}}}
	if edit != nil {
		edit(&request)
	}
	m.BehaviorRepositories = slices.Clone(request.Registry.Repositories)
	sort.Slice(m.BehaviorRepositories, func(i, j int) bool {
		return m.BehaviorRepositories[i].RootCommit < m.BehaviorRepositories[j].RootCommit
	})
	if expectedEdit != nil {
		expectedEdit(&m)
	}
	raw, err := Encode(request)
	if err != nil {
		t.Fatal(err)
	}
	p, err := BuildBehaviorProviderV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	again, err := BuildBehaviorProviderV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := Encode(again)
	if !bytes.Equal(data, other) {
		t.Fatal("nondeterministic /2 producer")
	}
	name := "behavior-v2.json"
	if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", name)
	git(t, root, "commit", "-qm", "/2 provider")
	rev := git(t, root, "rev-parse", "HEAD")
	blob := git(t, root, "rev-parse", rev+":"+name)
	m.Inputs = append(m.Inputs, Input{name, rev, blob, Digest(data), "behavior", "provider"})
	m.Scopes = append(m.Scopes, Scope{name, rev})
	m.Providers = append(m.Providers, Provider{ID: "behavior", Kind: "records", Version: "2", Revision: rev, Record: name})
	m.MergeRule = "disjoint-union"
	return root, m, data
}

func TestBehaviorV2ProducerCorpusRoundTrip(t *testing.T) {
	// AFU-V1-006: preserve /2 identities from producer through the corpus boundary.
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "four repositories", true: "single repository"}[single], func(t *testing.T) {
			root, m, provider := behaviorV2Fixture(t, func(r *BehaviorProviderRequestV2) {
				if single {
					r.Registry.Repositories = []BehaviorRepository{{r.Source.ID, r.Source.Revision}}
				}
			}, nil)
			before := git(t, root, "status", "--porcelain=v1", "--untracked-files=all")
			artifact, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			report := artifact.BehaviorContracts[0]
			if report.ProviderSchema != BehaviorProviderSchemaV2 || report.Registry.Revisions != (BehaviorRevisions{}) || len(report.Registry.Repositories) != len(m.BehaviorRepositories) || report.Fallback != "full-relevant-suite" || len(report.VerifiedTests)+len(report.VerifiedFlows) != 0 {
				t.Fatalf("unexpected report %+v", report)
			}
			data, err := Encode(artifact)
			if err != nil {
				t.Fatal(err)
			}
			rebuilt, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			other, _ := Encode(rebuilt)
			if !bytes.Equal(data, other) {
				t.Fatal("nondeterministic corpus")
			}
			receipt, err := ReadQuery(context.Background(), root, data, Request{Operation: "gaps", Limit: MaxResults})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, gap := range artifact.Gaps {
				found = found || gap.Kind == "runtime-unqualified"
			}
			if !found || len(receipt.Results) == 0 {
				t.Fatal("lost explicit /2 qualification gaps")
			}
			retained, err := os.ReadFile(filepath.Join(root, "behavior-v2.json"))
			if err != nil || !bytes.Equal(retained, provider) || git(t, root, "status", "--porcelain=v1", "--untracked-files=all") != before {
				t.Fatal("read mutated source")
			}
		})
	}
}

func TestBehaviorV2RepositoryGaps(t *testing.T) {
	// AFU-V1-006: missing and stale members never become successful empty evidence.
	cases := []struct {
		name, kind string
		edit       func(*BehaviorProviderRequestV2)
		expected   func(*Manifest)
	}{
		{name: "stale expected", kind: "stale-repository-member", expected: func(m *Manifest) { m.BehaviorRepositories[0].Revision = strings.Repeat("f", 40) }},
		{name: "missing member", kind: "repository-member-missing", expected: func(m *Manifest) {
			m.BehaviorRepositories = append(m.BehaviorRepositories, BehaviorRepository{strings.Repeat("f", 40), strings.Repeat("e", 40)})
		}},
		{name: "missing expectations", kind: "repository-expectation-missing", expected: func(m *Manifest) { m.BehaviorRepositories = nil }},
		{name: "stale anchor", kind: "stale-repository-anchor", edit: func(r *BehaviorProviderRequestV2) { r.Registry.Flows[0].Evidence.Revision = strings.Repeat("f", 40) }},
		{name: "foreign anchor", kind: "repository-member-missing", edit: func(r *BehaviorProviderRequestV2) { r.Registry.Flows[0].Evidence.Repository = strings.Repeat("e", 40) }},
		{name: "external source unavailable", kind: "repository-evidence-unavailable", edit: func(r *BehaviorProviderRequestV2) {
			r.Registry.Flows[0].Evidence.Repository = strings.Repeat("1", 40)
			r.Registry.Flows[0].Evidence.Revision = strings.Repeat("8", 40)
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			root, m, _ := behaviorV2Fixture(t, tt.edit, tt.expected)
			a, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, g := range a.Gaps {
				found = found || g.Kind == tt.kind
			}
			if !found || len(a.BehaviorContracts[0].VerifiedTests) > 0 || a.BehaviorContracts[0].Fallback != "full-relevant-suite" {
				t.Fatalf("missing gap %s: %+v", tt.kind, a.Gaps)
			}
		})
	}
}

func TestBehaviorV2RefusesMalformedOrConflictingRecords(t *testing.T) {
	root, m, data := behaviorV2Fixture(t, nil, nil)
	var good ProviderRecord
	if err := decode(data, &good); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*ProviderRecord){
		func(p *ProviderRecord) { p.BehaviorContracts.Schema = 3 },
		func(p *ProviderRecord) {
			p.BehaviorContracts.Repositories = append(p.BehaviorContracts.Repositories, p.BehaviorContracts.Repositories[0])
		},
		func(p *ProviderRecord) { p.BehaviorContracts.Flows[0].Evidence.Authority = "accepted-spec" },
		func(p *ProviderRecord) { p.BehaviorContracts.Flows[0].Evidence.Path = "../secret" },
		func(p *ProviderRecord) { p.BehaviorContracts.Flows[0].Evidence.Start = 0 },
		func(p *ProviderRecord) {
			p.BehaviorContracts.Tests = append(p.BehaviorContracts.Tests, p.BehaviorContracts.Tests[0])
		},
	} {
		var p ProviderRecord
		if err := decode(data, &p); err != nil {
			t.Fatal(err)
		}
		edit(&p)
		p.BehaviorContracts.ContractSHA256 = testHash(t, behaviorV2Declarations(*p.BehaviorContracts))
		if ValidateBehaviorProviderV2(p) == nil {
			t.Fatal("malformed /2 record accepted")
		}
	}
	// A correctly-shaped local anchor still has to resolve to exact immutable bytes.
	m.Inputs[0].SHA256 = strings.Repeat("0", 64)
	if _, err := Build(context.Background(), root, m); err == nil {
		t.Fatal("forged source bytes accepted")
	}
	raw, _ := Encode(BehaviorProviderRequestV2{Schema: BehaviorProviderRequestSchemaV2, ID: good.ID, Version: good.Version, Source: good.Source, Registry: *good.BehaviorContracts})
	if _, err := BuildBehaviorProviderV2(raw); err == nil {
		t.Fatal("producer silently re-signed a registry")
	}
}

// legacyV1Member reads the legacy /1 member's quoted JSON key from its single
// declaration, so tests never repeat the string (AFU-V1-056).
func legacyV1Member(t *testing.T) string {
	t.Helper()
	field, ok := reflect.TypeFor[BehaviorRevisions]().FieldByName("E2E")
	name := field.Tag.Get("json")
	if !ok || name == "" || name == "e2e" {
		t.Fatalf("legacy /1 member declaration changed: %q", name)
	}
	return `"` + name + `"`
}

func TestBehaviorProviderV1LegacyMemberDecodes(t *testing.T) {
	// AFU-V1-054: /1 keeps the legacy member on both sides of the wire and gains no alias.
	legacy := legacyV1Member(t)
	want := v1BehaviorProviderLiteral()
	data, err := Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), legacy+":") != 1 || strings.Contains(string(data), `"e2e"`) {
		t.Fatalf("/1 wire lost the legacy member: %s", data)
	}
	var decoded ProviderRecord
	if err := decode(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != BehaviorProviderSchema || decoded.BehaviorContracts.Revisions != want.BehaviorContracts.Revisions || decoded.BehaviorContracts.Revisions.E2E != want.Source {
		t.Fatalf("/1 decode lost the legacy member: %+v", decoded.BehaviorContracts.Revisions)
	}
	renamed := strings.Replace(string(data), legacy+":", `"e2e":`, 1)
	if err := decode([]byte(renamed), &ProviderRecord{}); err == nil {
		t.Fatal("/1 accepted a neutral alias of the legacy member")
	}
}

func TestBehaviorProviderV2EmitsNeutralMembersOnly(t *testing.T) {
	// AFU-V1-055: the /2 record and its corpus report carry the end-to-end
	// repository only as the neutral source and repositories members.
	legacy := legacyV1Member(t)
	root, m, provider := behaviorV2Fixture(t, nil, nil)
	source := `"source":{"id":"` + m.Repository.ID + `","revision":"` + m.Repository.Revision + `"}`
	member := `{"root_commit":"` + m.Repository.ID + `","revision":"` + m.Repository.Revision + `"}`
	if strings.Contains(string(provider), legacy) || strings.Contains(string(provider), `"revisions"`) || !strings.Contains(string(provider), source) || !strings.Contains(string(provider), member) {
		t.Fatalf("/2 provider wire: %s", provider)
	}
	artifact, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact.BehaviorContracts) != 1 || artifact.BehaviorContracts[0].ProviderSchema != BehaviorProviderSchemaV2 || strings.Contains(string(data), legacy) || strings.Contains(string(data), `"revisions"`) {
		t.Fatalf("/2 corpus report emitted a fixed /1 member: %s", data)
	}
}

func TestBehaviorProviderV2RefusesLegacyMember(t *testing.T) {
	// AFU-V1-056: a /2 request neither carries the legacy member nor a neutral alias of it.
	legacy := legacyV1Member(t)
	p := v1BehaviorProviderLiteral()
	r := *p.BehaviorContracts
	r.Revisions, r.ContractSHA256 = BehaviorRevisions{}, ""
	r.Repositories = []BehaviorRepository{{p.Source.ID, p.Source.Revision}}
	request := BehaviorProviderRequestV2{Schema: BehaviorProviderRequestSchemaV2, ID: "behavior", Version: "2", Source: p.Source, Registry: r}
	raw, err := Encode(request)
	if err != nil {
		t.Fatal(err)
	}
	built, err := BuildBehaviorProviderV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := Encode(built); strings.Contains(string(data), legacy) {
		t.Fatalf("/2 producer emitted the legacy member: %s", data)
	}
	withLegacy := request
	withLegacy.Registry.Revisions = BehaviorRevisions{E2E: p.Source}
	legacyRaw, err := Encode(withLegacy)
	if err != nil || !strings.Contains(string(legacyRaw), legacy) {
		t.Fatalf("fixture lacks the legacy member: %v", err)
	}
	if _, err := BuildBehaviorProviderV2(legacyRaw); err == nil {
		t.Fatal("/2 producer accepted the legacy member")
	}
	alias := `"e2e":{"id":"` + p.Source.ID + `","revision":"` + p.Source.Revision + `"},"repositories":`
	aliasRaw := strings.Replace(string(raw), `"repositories":`, alias, 1)
	if aliasRaw == string(raw) {
		t.Fatal("fixture lacks the repositories member")
	}
	if _, err := BuildBehaviorProviderV2([]byte(aliasRaw)); err == nil {
		t.Fatal("/2 producer accepted a neutral alias of the legacy member")
	}
}
