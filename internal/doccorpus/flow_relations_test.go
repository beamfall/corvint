package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// flowRelationFixture adds flows B, C and D to the adoption corpus and one
// flow-relation provider: A produces job:J (source anchored at src/view.ts),
// B consumes job:J, C depends-on B, C calls an external system, and an
// unreviewed D depends-on A.
func flowRelationFixture(t *testing.T, edit func(root string, m Manifest, p *ProviderRecord), raw func([]byte) []byte) (string, Manifest) {
	t.Helper()
	return flowRelationFixtureWith(t, nil, edit, raw)
}

// flowRelationFixtureWith also lets a test edit the adoption provider, for example to declare
// job or event subjects.
func flowRelationFixtureWith(t *testing.T, adopt func(*ProviderRecord), edit func(root string, m Manifest, p *ProviderRecord), raw func([]byte) []byte) (string, Manifest) {
	t.Helper()
	root, m := adoptionFixture(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		for _, id := range []string{"adapter:flow-b", "adapter:flow-c", "adapter:flow-d"} {
			sections := []FlowSection{}
			for _, name := range flowSections {
				sections = append(sections, FlowSection{Name: name, Paragraphs: []string{}})
			}
			p.Subjects = append(p.Subjects, Subject{ID: id, Kind: "flow", Name: id, Provider: p.ID, Evidence: ev})
			p.Details[id] = RecordDetails{Flow: &FlowDetails{Sections: sections, Actors: []string{"operator"}, Entrypoints: []string{"adapter:count"}, Variations: []string{}}}
		}
		if adopt != nil {
			adopt(p)
		}
	})
	rev := m.Repository.Revision
	source := tkAnchor(t, root, m, rev, "src/view.ts", 1, 1)
	record := ProviderRecord{Schema: FlowRelationSchema, ID: "fr", Version: "1", Source: m.Repository, FlowRelations: &FlowRelationSet{Revision: rev, Records: []FlowRelationRecord{
		{ID: "fr:a-produces-j", Kind: "produces", Flow: "adapter:flow", Name: "job:J", Review: "reviewed", Source: &source},
		{ID: "fr:b-consumes-j", Kind: "consumes", Flow: "adapter:flow-b", Name: "job:J", Review: "reviewed"},
		{ID: "fr:c-depends-b", Kind: "depends-on", Flow: "adapter:flow-c", Target: "adapter:flow-b", Review: "reviewed"},
		{ID: "fr:c-external", Kind: "external", Flow: "adapter:flow-c", System: "payments-gateway", Review: "reviewed"},
		{ID: "fr:d-depends-a", Kind: "depends-on", Flow: "adapter:flow-d", Target: "adapter:flow", Review: "unreviewed"},
	}}}
	if edit != nil {
		edit(root, m, &record)
	}
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if raw != nil {
		data = raw(data)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/fr.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/fr.json")
	git(t, root, "commit", "-qm", "flow relations")
	frRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "fr", Kind: "records", Version: "1", Revision: frRevision, Record: "evidence/fr.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/fr.json", frRevision})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/fr.json", Revision: frRevision, Blob: git(t, root, "rev-parse", frRevision+":evidence/fr.json"), SHA256: Digest(data), Provider: "fr", Purpose: "provider"})
	return root, m
}
func frBuild(t *testing.T, edit func(string, Manifest, *ProviderRecord), raw func([]byte) []byte) (*Artifact, error) {
	t.Helper()
	root, m := flowRelationFixture(t, edit, raw)
	return Build(context.Background(), root, m)
}
func frRecord(i int, edit func(r *FlowRelationRecord)) func(string, Manifest, *ProviderRecord) {
	return func(_ string, _ Manifest, p *ProviderRecord) { edit(&p.FlowRelations.Records[i]) }
}
func frJoin(t *testing.T, a *Artifact, id string) FlowRelationJoin {
	t.Helper()
	for _, j := range a.FlowRelations {
		if j.ID == id {
			return j
		}
	}
	t.Fatalf("flow relation %s not joined", id)
	return FlowRelationJoin{}
}
func impactFlows(t *testing.T, a *Artifact, paths []string, depth int) (*FlowTrace, []string) {
	t.Helper()
	result := Impact(a, paths, "fresh", depth)
	trace, ok := result["flow_relations"].(*FlowTrace)
	if !ok {
		t.Fatalf("impact carried no flow trace: %#v", result["flow_relations"])
	}
	return trace, result["unknowns"].([]string)
}

func TestDCPV1093FlowRelationSchemaIsClosed(t *testing.T) {
	t.Parallel()
	a, err := frBuild(t, nil, nil)
	if err != nil || len(a.FlowRelations) != 5 {
		t.Fatalf("valid flow relations not joined: %v", err)
	}
	for name, raw := range map[string]func([]byte) []byte{
		"unknown record member": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"review":"reviewed"`), []byte(`"review":"reviewed","weight":1`), 1)
		},
		"unknown set member": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"flow_relations":{`), []byte(`"flow_relations":{"complete":true,`), 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := frBuild(t, nil, raw); err == nil || !strings.Contains(err.Error(), "corpus-refused") {
				t.Fatalf("unknown member admitted: %v", err)
			}
		})
	}
	t.Run("other members under the schema", func(t *testing.T) {
		_, err := frBuild(t, func(_ string, _ Manifest, p *ProviderRecord) {
			p.Capabilities = []CapabilityDeclaration{{"subjects", "absent", "none"}}
		}, nil)
		tkRefused(t, err, "only its flow_relations member")
	})
	t.Run("duplicate id", func(t *testing.T) {
		_, err := frBuild(t, func(_ string, _ Manifest, p *ProviderRecord) {
			p.FlowRelations.Records = append(p.FlowRelations.Records, p.FlowRelations.Records[1])
		}, nil)
		tkRefused(t, err, "unique and provider-qualified")
	})
}

func TestDCPV1094FlowRelationKindFields(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		i    int
		edit func(*FlowRelationRecord)
		want string
	}{
		"unknown kind":            {1, func(r *FlowRelationRecord) { r.Kind = "calls" }, "unknown flow-relation kind or review state"},
		"unknown review":          {1, func(r *FlowRelationRecord) { r.Review = "approved" }, "unknown flow-relation kind or review state"},
		"unqualified id":          {1, func(r *FlowRelationRecord) { r.ID = "b-consumes-j" }, "provider-qualified"},
		"produces without name":   {1, func(r *FlowRelationRecord) { r.Name = "" }, "requires exactly its own"},
		"consumes with target":    {1, func(r *FlowRelationRecord) { r.Target = "adapter:flow" }, "requires exactly its own"},
		"depends-on with name":    {2, func(r *FlowRelationRecord) { r.Name = "job:J" }, "requires exactly its own"},
		"depends-on on itself":    {2, func(r *FlowRelationRecord) { r.Target = r.Flow }, "requires exactly its own"},
		"external without system": {3, func(r *FlowRelationRecord) { r.System = "" }, "requires exactly its own"},
		"external with target":    {3, func(r *FlowRelationRecord) { r.Target = "adapter:flow" }, "requires exactly its own"},
		"secret-shaped name":      {1, func(r *FlowRelationRecord) { r.Name = "AKIAABCDEFGHIJKLMNOP" }, "requires exactly its own"},
		"unpinned source":         {0, func(r *FlowRelationRecord) { r.Source.Revision = strings.Repeat("a", 40) }, "not pinned to the set revision"},
		"invalid source pin":      {0, func(r *FlowRelationRecord) { r.Source.SpanSHA256 = "x" }, "invalid flow-relation source anchor"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := frBuild(t, frRecord(c.i, c.edit), nil)
			tkRefused(t, err, c.want)
		})
	}
	t.Run("forged source binding", func(t *testing.T) {
		_, err := frBuild(t, frRecord(0, func(r *FlowRelationRecord) { r.Source.SHA256 = strings.Repeat("0", 64) }), nil)
		tkRefused(t, err, "source anchor binding mismatch")
	})
	t.Run("manifest v1", func(t *testing.T) {
		root, m := flowRelationFixture(t, nil, nil)
		m.Schema = ManifestSchema
		if _, err := Build(context.Background(), root, m); err == nil {
			t.Fatal("flow-relation provider admitted under manifest /1")
		}
	})
}

func TestDCPV1095FlowRelationJoinStatus(t *testing.T) {
	t.Parallel()
	a, err := frBuild(t, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if j := frJoin(t, a, "fr:a-produces-j"); j.Status != "joined" || !j.Traversable || len(j.Reasons) != 0 {
		t.Fatalf("reviewed anchored relation not traversable: %+v", j)
	}
	if j := frJoin(t, a, "fr:d-depends-a"); j.Status != "joined" || j.Traversable || !slices.Contains(j.Reasons, "unreviewed: declared relation awaits review") {
		t.Fatalf("unreviewed relation became traversable: %+v", j)
	}
	t.Run("unresolved flow", func(t *testing.T) {
		a, err := frBuild(t, frRecord(2, func(r *FlowRelationRecord) { r.Target = "adapter:count" }), nil)
		if err != nil {
			t.Fatal(err)
		}
		if j := frJoin(t, a, "fr:c-depends-b"); j.Status != "unresolved" || j.Traversable {
			t.Fatalf("non-flow target resolved: %+v", j)
		}
	})
	t.Run("unanchored source", func(t *testing.T) {
		a, err := frBuild(t, func(root string, m Manifest, p *ProviderRecord) {
			anchor := tkAnchor(t, root, m, m.Repository.Revision, "go.mod", 1, 1)
			p.FlowRelations.Records[0].Source = &anchor
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if j := frJoin(t, a, "fr:a-produces-j"); j.Status != "unanchored" || j.Traversable {
			t.Fatalf("undeclared source anchored: %+v", j)
		}
	})
	t.Run("stale set", func(t *testing.T) {
		a, err := frBuild(t, func(root string, m Manifest, p *ProviderRecord) {
			p.FlowRelations.Revision = git(t, root, "rev-parse", "HEAD~1")
			p.FlowRelations.Records[0].Source = nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range a.FlowRelations {
			if j.Status != "stale" || j.Traversable {
				t.Fatalf("stale relation traversable: %+v", j)
			}
		}
	})
}

func TestDCPV1096FlowRelationImpactAttribution(t *testing.T) {
	t.Parallel()
	a, err := frBuild(t, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Acceptance: A produces job:J, B consumes job:J; a change in A's anchored
	// code lists B as affected through A -> J -> B.
	trace, unknowns := impactFlows(t, a, []string{"src/view.ts"}, 0)
	if !slices.Equal(trace.Seeds, []string{"adapter:flow"}) || trace.Depth != DefaultFlowDepth || trace.Authority != "project-declared-corpus" {
		t.Fatalf("seeds or bounds: %+v", trace)
	}
	if len(trace.Reached) != 3 {
		t.Fatalf("reach: %+v", trace.Reached)
	}
	b := trace.Reached[0]
	if b.Flow != "adapter:flow-b" || b.Depth != 1 || !slices.Equal(b.Path, []string{"adapter:flow", "job:J", "adapter:flow-b"}) || !slices.Equal(b.Relations, []string{"fr:a-produces-j", "fr:b-consumes-j"}) {
		t.Fatalf("B not attributed to A->J->B: %+v", b)
	}
	c := trace.Reached[1]
	if c.Flow != "adapter:flow-c" || c.Depth != 2 || !slices.Equal(c.Path, []string{"adapter:flow", "job:J", "adapter:flow-b", "adapter:flow-c"}) {
		t.Fatalf("depends-on hop: %+v", c)
	}
	s := trace.Reached[2]
	if s.System != "payments-gateway" || s.Flow != "" || s.Depth != 3 || !slices.Equal(s.Relations, []string{"fr:a-produces-j", "fr:b-consumes-j", "fr:c-depends-b", "fr:c-external"}) {
		t.Fatalf("external system: %+v", s)
	}
	if trace.Truncated {
		t.Fatal("complete walk reported truncation")
	}
	if !slices.Contains(trace.Limitations, flowRelationLimitation) {
		t.Fatal("authority limitation dropped")
	}
	result := Impact(a, []string{"src/view.ts"}, "fresh", 0)
	if result["selection"].(map[string]any)["narrowing_allowed"] != false {
		t.Fatal("flow relations narrowed selection")
	}
	_ = unknowns
}

func TestDCPV1097FlowRelationUnreviewedAndDepth(t *testing.T) {
	t.Parallel()
	a, err := frBuild(t, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	trace, unknowns := impactFlows(t, a, []string{"src/view.ts"}, 0)
	for _, r := range trace.Reached {
		if r.Flow == "adapter:flow-d" {
			t.Fatal("unreviewed relation silently traversed")
		}
	}
	if len(trace.Withheld) != 1 || trace.Withheld[0].ID != "fr:d-depends-a" || trace.Withheld[0].At != "adapter:flow" || trace.Withheld[0].Reason != "relation is unreviewed" {
		t.Fatalf("unreviewed relation not reported: %+v", trace.Withheld)
	}
	if !slices.Contains(unknowns, "unreviewed or unjoined flow relation reported, not traversed") {
		t.Fatalf("unreviewed relation did not keep impact open: %v", unknowns)
	}
	t.Run("depth bound", func(t *testing.T) {
		trace, unknowns := impactFlows(t, a, []string{"src/view.ts"}, 1)
		if trace.Depth != 1 || len(trace.Reached) != 1 || trace.Reached[0].Flow != "adapter:flow-b" || !trace.Truncated {
			t.Fatalf("depth 1 walk: %+v", trace)
		}
		if !slices.Contains(unknowns, "flow relation traversal reached its depth bound") {
			t.Fatalf("truncation not an unknown: %v", unknowns)
		}
		deep, _ := impactFlows(t, a, []string{"src/view.ts"}, MaxFlowDepth)
		if deep.Truncated || len(deep.Reached) != 3 {
			t.Fatalf("max depth walk: %+v", deep)
		}
	})
	t.Run("reviewed after review", func(t *testing.T) {
		a, err := frBuild(t, frRecord(4, func(r *FlowRelationRecord) { r.Review = "reviewed" }), nil)
		if err != nil {
			t.Fatal(err)
		}
		trace, _ := impactFlows(t, a, []string{"src/view.ts"}, 0)
		if len(trace.Withheld) != 0 || trace.Reached[1].Flow != "adapter:flow-d" || !slices.Equal(trace.Reached[1].Relations, []string{"fr:d-depends-a"}) {
			t.Fatalf("reviewed depends-on not traversed: %+v", trace)
		}
	})
	t.Run("unreviewed consumer is withheld", func(t *testing.T) {
		a, err := frBuild(t, frRecord(1, func(r *FlowRelationRecord) { r.Review = "unreviewed" }), nil)
		if err != nil {
			t.Fatal(err)
		}
		trace, _ := impactFlows(t, a, []string{"src/view.ts"}, 0)
		if len(trace.Reached) != 0 || len(trace.Withheld) != 2 {
			t.Fatalf("unreviewed consumer traversed: %+v", trace)
		}
	})
	t.Run("no seed no reach", func(t *testing.T) {
		trace, _ := impactFlows(t, a, []string{"src/report.py"}, 0)
		if len(trace.Seeds) != 0 || len(trace.Reached) != 0 {
			t.Fatalf("unrelated path reached flows: %+v", trace)
		}
	})
}

func TestDCPV1098FlowRelationTraceAndGaps(t *testing.T) {
	t.Parallel()
	a, err := frBuild(t, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Query(a, Request{Operation: "trace", ID: "adapter:flow", Depth: 1}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	var trace *FlowTrace
	for _, x := range r.Results {
		if ft, ok := x.(FlowTrace); ok {
			trace = &ft
		}
	}
	if trace == nil || trace.Depth != 1 || len(trace.Reached) != 1 || !slices.Equal(trace.Reached[0].Path, []string{"adapter:flow", "job:J", "adapter:flow-b"}) {
		t.Fatalf("trace did not walk flow relations: %+v", r.Results)
	}
	got, err := Query(a, Request{Operation: "get", ID: "fr:d-depends-a"}, "fresh", nil)
	if err != nil || len(got.Results) != 1 {
		t.Fatalf("flow relation not retrievable: %+v %v", got, err)
	}
	for _, bad := range []Request{{Operation: "get", ID: "adapter:flow", Depth: 2}, {Operation: "trace", ID: "adapter:flow", Depth: MaxFlowDepth + 1}, {Operation: "trace", ID: "adapter:flow", Depth: -1}} {
		if _, err := Query(a, bad, "fresh", nil); err == nil || !strings.Contains(err.Error(), "flow trace depth") {
			t.Fatalf("depth %+v admitted: %v", bad, err)
		}
	}
	gaps, err := Query(a, Request{Operation: "gaps", ID: "fr:d-depends-a"}, "fresh", nil)
	if err != nil || len(gaps.Results) != 1 || gaps.Results[0].(Gap).Kind != "unreviewed-flow-relation" {
		t.Fatalf("unreviewed relation not a gap: %+v %v", gaps.Results, err)
	}
}

func TestDCPV1099FlowRelationDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	root, m := flowRelationFixture(t, nil, nil)
	first, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := Encode(first)
	y, _ := Encode(second)
	if !bytes.Equal(x, y) {
		t.Fatal("flow relation join is not deterministic")
	}
	if _, err := Open(context.Background(), root, x); err != nil {
		t.Fatal(err)
	}
	t.Run("record bound", func(t *testing.T) {
		_, err := frBuild(t, func(_ string, _ Manifest, p *ProviderRecord) {
			r := p.FlowRelations.Records[1]
			for i := len(p.FlowRelations.Records); i <= MaxFlowRelations; i++ {
				r.ID = "fr:extra-" + strconv.Itoa(i)
				p.FlowRelations.Records = append(p.FlowRelations.Records, r)
			}
		}, nil)
		tkRefused(t, err, "record bound")
	})
}

func TestDCXV1397FlowRelationNamesCheckDeclaredJobAndEventSubjects(t *testing.T) {
	t.Parallel()
	declare := func(names ...string) func(*ProviderRecord) {
		return func(p *ProviderRecord) {
			for i, name := range names {
				p.Subjects = append(p.Subjects, Subject{ID: "adapter:declared-" + string(rune('a'+i)), Kind: []string{"job", "event"}[i%2], Name: name, Provider: p.ID, Evidence: p.Subjects[0].Evidence})
			}
		}
	}
	t.Run("matched names join and name their subjects", func(t *testing.T) {
		t.Parallel()
		root, m := flowRelationFixtureWith(t, declare("job:J"), nil, nil)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"fr:a-produces-j", "fr:b-consumes-j"} {
			j := frJoin(t, a, id)
			if j.Status != "joined" || !j.Traversable || len(j.NameSubjects) != 1 || j.NameSubjects[0] != "adapter:declared-a" {
				t.Fatalf("%s: %+v", id, j)
			}
		}
		if j := frJoin(t, a, "fr:c-depends-b"); j.NameSubjects != nil {
			t.Fatalf("depends-on carries name subjects: %+v", j)
		}
	})
	t.Run("an undeclared name is unresolved, never traversed", func(t *testing.T) {
		t.Parallel()
		root, m := flowRelationFixtureWith(t, declare("job:Other", "order.created"), nil, nil)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		j := frJoin(t, a, "fr:a-produces-j")
		if j.Status != "unresolved" || j.Traversable || !slices.Contains(j.Reasons, "unresolved: no declared job or event subject is named job:J") {
			t.Fatalf("undeclared name joined: %+v", j)
		}
	})
	t.Run("without declared jobs or events the join is unchanged", func(t *testing.T) {
		t.Parallel()
		a, err := frBuild(t, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		if j := frJoin(t, a, "fr:a-produces-j"); j.Status != "joined" || j.NameSubjects != nil || strings.Contains(string(raw), "name_subjects") {
			t.Fatalf("legacy join changed: %+v", j)
		}
	})
}
