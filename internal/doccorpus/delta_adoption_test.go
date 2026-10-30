package doccorpus

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// docsAdoptionRecord is GitHub #1611's documentation pipeline: one adoption provider holding a
// flow anchored at first and last, two active paragraphs (one at each anchor), a retired
// paragraph, a retired_to and a generic relation, and a typed claim about the flow.
func docsAdoptionRecord(m Manifest, first, last Anchor) ProviderRecord {
	ev := func(anchors ...Anchor) Evidence {
		return Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: anchors, Limitations: []string{"delta adoption fixture"}}
	}
	sections := []FlowSection{}
	for _, name := range flowSections {
		sections = append(sections, FlowSection{Name: name, Paragraphs: []string{}})
	}
	sections[0].Paragraphs = []string{"docs:p1"}
	sections[3].Paragraphs = []string{"docs:p2"}
	return ProviderRecord{Schema: AdoptionProviderSchema, ID: "docs", Version: "1", Source: m.Repository,
		Subjects: []Subject{
			{"docs:flow", "flow", "Checkout flow", "docs", ev(first, last), nil},
			{"docs:p0", "paragraph", "Retired readme paragraph", "docs", ev(first), nil},
			{"docs:p1", "paragraph", "Readme paragraph", "docs", ev(first), nil},
			{"docs:p2", "paragraph", "Functional overview paragraph", "docs", ev(last), nil},
		},
		Claims: []Claim{{"docs:claim", "docs:flow", "Checkout counts the cart", "docs", ev(first)}},
		Relations: []Relation{
			{"docs:documents", "docs:p1", "docs:flow", "documents", "docs", ev(first)},
			{"docs:retired", "docs:p0", "docs:p1", "retired_to", "docs", ev(first)},
		},
		Journeys:     []Journey{},
		Observations: []ObservationLink{},
		Capabilities: []CapabilityDeclaration{{"subjects", "present", "fixture"}, {"claims", "present", "fixture"}, {"relations", "present", "fixture"}},
		Details: map[string]RecordDetails{
			"docs:flow":  {Flow: &FlowDetails{Sections: sections, Actors: []string{"shopper"}, Entrypoints: []string{}, Variations: []string{}}},
			"docs:p0":    {Paragraph: &ParagraphDetails{Flow: "docs:flow", Section: "readme", StableID: "docs:stable-0", RetiredTo: "docs:p1"}},
			"docs:p1":    {Paragraph: &ParagraphDetails{Flow: "docs:flow", Section: "readme", StableID: "docs:stable-1"}},
			"docs:p2":    {Paragraph: &ParagraphDetails{Flow: "docs:flow", Section: "functional-overview", StableID: "docs:stable-2"}},
			"docs:claim": {ClaimKind: "behavior"},
		},
	}
}

var docsAdoptionIDs = []string{"docs:flow", "docs:p0", "docs:p1", "docs:p2", "docs:claim", "docs:documents", "docs:retired"}

// withDocsAdoption declares the docs adoption provider over first and last on a /2 manifest.
func withDocsAdoption(t *testing.T, root string, m Manifest, first, last Anchor) Manifest {
	t.Helper()
	m = commitProvider(t, root, m, docsAdoptionRecord(m, first, last), "evidence/docs.json")
	m.Schema = ManifestSchemaV2
	return m
}

func subjectAnchor(t *testing.T, a *Artifact, id string) Anchor {
	t.Helper()
	for _, s := range a.Subjects {
		if s.ID == id {
			return s.Evidence.Anchors[0]
		}
	}
	t.Fatalf("fixture lacks subject %s", id)
	return Anchor{}
}

// assertSelfDeltaCarries is the DCX-V1-444 conformance check: a corpus compared with itself lists
// no unresolved entry in either format, and every docs adoption record is carry-forward.
func assertSelfDeltaCarries(t *testing.T, root string, a *Artifact) {
	t.Helper()
	ctx := context.Background()
	receipt, err := DeltaWith(ctx, root, a, a, "artifact", DeltaOptions{Limit: MaxResults})
	if err != nil {
		t.Fatal(err)
	}
	got := recordEntries(receipt)
	for _, e := range receipt.Entries {
		if e.Class == DeltaUnresolved {
			t.Errorf("self-delta listed unresolved %s %s: %s", e.RecordKind, e.ID, e.Reason)
		}
	}
	for _, id := range docsAdoptionIDs {
		if e := got[id]; e.Class != DeltaCarryForward || !e.PresentInCurrent || len(e.Anchors) == 0 {
			t.Errorf("adoption record %s not carried: %+v", id, e)
		}
	}
	if receipt.Counts[DeltaUnresolved] != 0 || receipt.NextOffset != 0 {
		t.Fatalf("self-delta counts: %+v (next %d)", receipt.Counts, receipt.NextOffset)
	}
	stream, err := DeltaStream(ctx, root, a, a, "artifact", DeltaOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var header DeltaStreamHeader
	if err := json.Unmarshal([]byte(strings.SplitN(string(stream), "\n", 2)[0]), &header); err != nil || header.Counts[DeltaUnresolved] != 0 || !reflect.DeepEqual(header.Counts, receipt.Counts) {
		t.Fatalf("self-delta stream counts: %+v %v", header.Counts, err)
	}
}

func TestDCXV1440DeltaAdoptionRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("DCX-V1-440 DCX-V1-444 a self-delta of the additional-repository fixture with flows and paragraphs has no unresolved entry", func(t *testing.T) {
		t.Parallel()
		f := newDeltaAddRepoFixture(t)
		m := withDocsAdoption(t, f.root, f.previous.Manifest, subjectAnchor(t, f.previous, "adapter:lib"), subjectAnchor(t, f.previous, "adapter:print"))
		a, err := Build(ctx, f.root, m)
		if err != nil {
			t.Fatal(err)
		}
		assertSelfDeltaCarries(t, f.root, a)
		if e := recordEntries(mustDelta(t, f.root, a, a))["docs:flow"]; len(e.Anchors) != 2 || e.Anchors[0].Repository != f.extra.ID {
			t.Fatalf("flow anchors in the additional repository: %+v", e)
		}
	})
	t.Run("DCX-V1-440 DCX-V1-444 a self-delta of the multi-revision fixture with flows and paragraphs has no unresolved entry", func(t *testing.T) {
		t.Parallel()
		root, m, _, _ := multiRevisionFixture(t)
		base, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		m = withDocsAdoption(t, root, m, subjectAnchor(t, base, "adapter:lib-old"), subjectAnchor(t, base, "adapter:lib"))
		a, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		assertSelfDeltaCarries(t, root, a)
	})
	t.Run("DCX-V1-440 DCX-V1-442 a partly changed flow is changed with its anchors and its dependents are demoted", func(t *testing.T) {
		t.Parallel()
		root, m := fixture(t)
		rev := m.Repository.Revision
		previous, err := Build(ctx, root, withDocsAdoption(t, root, m, tkAnchor(t, root, m, rev, "src/readme.md", 3, 3), tkAnchor(t, root, m, rev, "src/value.go", 3, 3)))
		if err != nil {
			t.Fatal(err)
		}
		cur := commitEdits(t, root, map[string]string{"src/value.go": "package value\n\n// Count returns one value.\nfunc Count() int { return 1 }\n"})
		cur.Schema = ManifestSchemaV2
		current, err := Build(ctx, root, cur)
		if err != nil {
			t.Fatal(err)
		}
		got := deltaEntries(t, root, previous, current)
		flow := got["docs:flow"]
		if flow.Class != DeltaChanged || len(flow.Anchors) != 2 || flow.Anchors[0].State != DeltaCarryForward || flow.Anchors[1].State != DeltaChanged || len(flow.Anchors[1].ChangedLines) == 0 {
			t.Fatalf("partly changed flow: %+v", flow)
		}
		if p2 := got["docs:p2"]; p2.Class != DeltaChanged {
			t.Fatalf("changed paragraph: %+v", p2)
		}
		for _, id := range []string{"docs:p0", "docs:p1"} {
			if e := got[id]; e.Class != DeltaUnresolved || e.Reason != "typed detail reference docs:flow is neither in the current corpus nor carried forward" || e.Anchors[0].State != DeltaCarryForward {
				t.Fatalf("paragraph of a changed flow not demoted: %+v", e)
			}
		}
		if e := got["docs:claim"]; e.Class != DeltaUnresolved || e.Reason != "claim subject docs:flow is neither in the current corpus nor carried forward" {
			t.Fatalf("claim about a changed flow not demoted: %+v", e)
		}
		if e := got["docs:retired"]; e.Class != DeltaUnresolved || !strings.Contains(e.Reason, "relation endpoint docs:p") {
			t.Fatalf("retired_to relation of demoted paragraphs: %+v", e)
		}
		record, err := CarryForward(ctx, root, previous, current, "docs")
		if err != nil {
			t.Fatal(err)
		}
		if n := len(record.Subjects) + len(record.Claims) + len(record.Relations) + len(record.Details); n != 0 {
			t.Fatalf("carry-forward re-applied a record whose dependencies changed: %+v", record)
		}
	})
	t.Run("DCX-V1-443 carry-forward re-applies adoption records with their typed details and the next build admits them", func(t *testing.T) {
		t.Parallel()
		root, m := fixture(t)
		rev := m.Repository.Revision
		previous, err := Build(ctx, root, withDocsAdoption(t, root, m, tkAnchor(t, root, m, rev, "src/readme.md", 3, 3), tkAnchor(t, root, m, rev, "src/value.go", 3, 3)))
		if err != nil {
			t.Fatal(err)
		}
		cur := commitEdits(t, root, map[string]string{"src/report.py": "def report_count():\n    return 2\n"})
		v1, err := Build(ctx, root, cur)
		if err != nil {
			t.Fatal(err)
		}
		got := deltaEntries(t, root, previous, v1)
		for _, id := range docsAdoptionIDs {
			if got[id].Class != DeltaCarryForward {
				t.Fatalf("%s not carry-forward: %+v", id, got[id])
			}
		}
		if _, err := CarryForward(ctx, root, previous, v1, "docs"); err == nil || !strings.Contains(err.Error(), ManifestSchemaV2) {
			t.Fatalf("adoption carry-forward against a /1 manifest admitted: %v", err)
		}
		cur.Schema = ManifestSchemaV2
		current, err := Build(ctx, root, cur)
		if err != nil {
			t.Fatal(err)
		}
		record, err := CarryForward(ctx, root, previous, current, "docs")
		if err != nil {
			t.Fatal(err)
		}
		if record.Schema != AdoptionProviderSchema || len(record.Subjects) != 4 || len(record.Claims) != 1 || len(record.Relations) != 2 || !reflect.DeepEqual(record.Details, previous.Details) {
			t.Fatalf("carried adoption record: %+v", record)
		}
		next, err := Build(ctx, root, commitProvider(t, root, cur, *record, "evidence/docs-carried.json"))
		if err != nil {
			t.Fatalf("carried adoption record does not build: %v", err)
		}
		if !reflect.DeepEqual(next.Details, previous.Details) {
			t.Fatalf("typed details changed across the carry: %+v", next.Details)
		}
		again, err := CarryForward(ctx, root, previous, next, "docs")
		if err != nil || len(again.Subjects)+len(again.Claims)+len(again.Relations) != 0 || again.Schema != AdoptionProviderSchema {
			t.Fatalf("second carry-forward carried again: %+v %v", again, err)
		}
	})
	t.Run("DCX-V1-441 an adoption record missing its typed details stays unresolved with the reason", func(t *testing.T) {
		t.Parallel()
		root, m := fixture(t)
		rev := m.Repository.Revision
		a, err := Build(ctx, root, withDocsAdoption(t, root, m, tkAnchor(t, root, m, rev, "src/readme.md", 3, 3), tkAnchor(t, root, m, rev, "src/value.go", 3, 3)))
		if err != nil {
			t.Fatal(err)
		}
		data, err := Encode(a)
		if err != nil {
			t.Fatal(err)
		}
		mutated, err := ParseArtifact(data)
		if err != nil {
			t.Fatal(err)
		}
		delete(mutated.Details, "docs:p2")
		for i := range mutated.Subjects {
			if mutated.Subjects[i].ID == "docs:p0" {
				mutated.Subjects[i].Kind = "unknown_kind"
			}
		}
		got := recordEntries(mustDelta(t, root, mutated, a))
		if e := got["docs:p2"]; e.Class != DeltaUnresolved || e.Reason != "adoption record kind paragraph requires typed details that the previous corpus does not hold" || e.Anchors[0].State != DeltaCarryForward {
			t.Fatalf("paragraph without typed details: %+v", e)
		}
		if e := got["docs:p0"]; e.Class != DeltaUnresolved || e.Reason != "record kind is in neither the generic nor the adoption vocabulary" {
			t.Fatalf("subject of an unknown kind: %+v", e)
		}
		if e := got["docs:p1"]; e.Class != DeltaCarryForward {
			t.Fatalf("a present dependency was demoted: %+v", e)
		}
	})
}

func TestDCXV1442DeltaExactTestJoinNeedsItsFile(t *testing.T) {
	t.Parallel()
	join := func(file string) *deltaRecord {
		return &deltaRecord{kind: "relation", entry: DeltaEntry{Class: DeltaCarryForward, ID: "docs:join"}, relation: Relation{ID: "docs:join", From: "docs:flow", To: "docs:test"}, details: &RecordDetails{TestLink: &TestLinkDetails{Role: "asserting", JoinConfidence: "exact", File: file}}}
	}
	for _, c := range []struct {
		file, class string
	}{{"src/a_test.go", DeltaCarryForward}, {"src/b_test.go", DeltaUnresolved}} {
		test := &deltaRecord{kind: "subject", entry: DeltaEntry{Class: DeltaMoved, ID: "docs:test"}, anchors: []Anchor{{Path: "src/a_test.go"}}}
		flow := &deltaRecord{kind: "subject", entry: DeltaEntry{Class: DeltaCarryForward, ID: "docs:flow"}, anchors: []Anchor{{Path: "docs/flow.md"}}}
		r := join(c.file)
		d := &deltaRun{current: &Artifact{}, records: []*deltaRecord{flow, test, r}}
		d.demoteDependencies(map[string]bool{})
		if r.entry.Class != c.class || c.class == DeltaUnresolved && r.entry.Reason != "exact test join file src/b_test.go is not anchored by test docs:test at the current revision" {
			t.Fatalf("exact join to %s: %+v", c.file, r.entry)
		}
	}
}

func mustDelta(t *testing.T, root string, previous, current *Artifact) *DeltaReceipt {
	t.Helper()
	receipt, err := Delta(context.Background(), root, previous, current, "artifact", 0, MaxResults)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

// recordEntries keys the classified record entries by ID, leaving out the uncovered entries that
// name the same subjects.
func recordEntries(receipt *DeltaReceipt) map[string]DeltaEntry {
	out := map[string]DeltaEntry{}
	for _, e := range receipt.Entries {
		if e.Class != DeltaUncovered {
			out[e.ID] = e
		}
	}
	return out
}

// TestDCXV1444SelfDeltaSharedSpanChargedOnce pins that a self-delta stays free of unresolved
// entries when more records share one large span than the 64 MiB hashing bound could hash one
// by one: each distinct verified span is charged once, and a cached digest still has to match.
func TestDCXV1444SelfDeltaSharedSpanChargedOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, m := fixture(t)
	line := strings.Repeat("x", 63) + "\n"
	m = commitEdits(t, root, map[string]string{"src/large.md": strings.Repeat(line, 1024)})
	rev := m.Repository.Revision
	shared := tkAnchor(t, root, m, rev, "src/large.md", 1, 1024)
	records := deltaHashMaxBytes/(64<<10) + 1
	subjects := make([]Subject, 0, records)
	for i := range records {
		subjects = append(subjects, Subject{fmt.Sprintf("adapter:shared-%04d", i), "document", "Shared", "adapter", deltaEvidence(shared), nil})
	}
	a, err := Build(ctx, root, commitProvider(t, root, m, deltaProviderRecord(m, "adapter", subjects, nil), "evidence/provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := DeltaWith(ctx, root, a, a, "artifact", DeltaOptions{Limit: MaxResults})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Counts[DeltaUnresolved] != 0 || receipt.Counts[DeltaCarryForward] < records {
		t.Fatalf("self-delta over one shared span: %+v", receipt.Counts)
	}
	run, err := classifyDelta(ctx, root, a, a)
	if err != nil {
		t.Fatal(err)
	}
	if run.hashed > 64<<10 {
		t.Fatalf("shared span charged %d bytes, want one span", run.hashed)
	}
	forged := subjectAnchor(t, a, "adapter:shared-0000")
	forged.SpanSHA256 = Digest([]byte("not the span"))
	if da, _, err := run.classifyAnchor(ctx, forged); err != nil || da.State == DeltaCarryForward || da.State == DeltaMoved {
		t.Fatalf("cached span digest confirmed a forged span hash: %+v %v", da, err)
	}
}
