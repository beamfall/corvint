package doccorpus

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// eightSectionFlow builds an adoption corpus whose flow has eight paragraphs in
// each of its eight sections, two flow claims and one tested relation.
func eightSectionFlow(t *testing.T) (string, Manifest, *Artifact) {
	t.Helper()
	root, m := adoptionFixture(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		flow := p.Details["adapter:flow"].Flow
		for i := range flow.Sections {
			for j := range 8 {
				id := fmt.Sprintf("adapter:p-%d-%d", i, j)
				p.Subjects = append(p.Subjects, Subject{id, "paragraph", fmt.Sprintf("Paragraph %d of section %s describes one documented step", j, flow.Sections[i].Name), "adapter", ev, nil})
				p.Details[id] = RecordDetails{Paragraph: &ParagraphDetails{Flow: "adapter:flow", Section: flow.Sections[i].Name, StableID: fmt.Sprintf("adapter:s-%d-%d", i, j)}}
				flow.Sections[i].Paragraphs = append(flow.Sections[i].Paragraphs, id)
			}
		}
		for i := range 2 {
			id := fmt.Sprintf("adapter:flow-claim-%d", i)
			p.Claims = append(p.Claims, Claim{id, "adapter:flow", fmt.Sprintf("Flow claim %d", i), "adapter", ev})
			p.Details[id] = RecordDetails{ClaimKind: "behavior"}
		}
	})
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	return root, m, a
}

func flowReceipt(t *testing.T, a *Artifact, q Request) (Receipt, []byte) {
	t.Helper()
	q.Operation, q.ID = "flow", "adapter:flow"
	if q.Limit == 0 {
		q.Limit = MaxResults
	}
	r, err := Query(a, q, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	return r, out
}

func resultIDs(r Receipt) []string {
	ids := []string{}
	for _, v := range r.Results {
		ids = append(ids, recordID(v))
	}
	return ids
}

func TestDCPV1170FlowSectionFilter(t *testing.T) {
	t.Parallel()
	_, _, a := eightSectionFlow(t)
	all, allBytes := flowReceipt(t, a, Request{})
	if all.FlowSelection != nil {
		t.Fatal("unfiltered flow carries a flow selection")
	}
	b, bBytes := flowReceipt(t, a, Request{Sections: []string{"entry-points"}})
	want := []string{"adapter:flow"}
	for j := range 8 {
		want = append(want, fmt.Sprintf("adapter:p-1-%d", j))
	}
	if !slices.Equal(resultIDs(b), want) {
		t.Fatalf("section B results %v", resultIDs(b))
	}
	if 4*len(bBytes) > len(allBytes) {
		t.Fatalf("section B payload %d bytes is more than a quarter of the unfiltered %d", len(bBytes), len(allBytes))
	}
	t.Logf("unfiltered %d bytes, section B %d bytes", len(allBytes), len(bBytes))
	two, _ := flowReceipt(t, a, Request{Sections: []string{"entities", "readme"}})
	if ids := resultIDs(two); len(ids) != 18 || ids[1] != "adapter:p-0-0" || ids[16] != "adapter:p-5-7" || ids[17] != "adapter:paragraph" {
		t.Fatalf("two sections: %v", ids)
	}
	miss, _ := flowReceipt(t, a, Request{Sections: []string{"readme", "B"}})
	if miss.State != "empty" || miss.Miss != "none-found" || len(miss.Results) != 0 || miss.FlowSelection == nil || !slices.Equal(miss.FlowSelection.Unknown, []string{"B"}) {
		t.Fatalf("unknown section not a typed none-found: %+v", miss)
	}
	names := []string{}
	for _, s := range miss.FlowSelection.Sections {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, flowSections) {
		t.Fatalf("none-found does not list the available sections: %v", names)
	}
}

func TestDCPV1171FlowIncludeFilter(t *testing.T) {
	t.Parallel()
	_, _, a := eightSectionFlow(t)
	claims, _ := flowReceipt(t, a, Request{Include: []string{"claims"}})
	if !slices.Equal(resultIDs(claims), []string{"adapter:flow", "adapter:flow-claim-0", "adapter:flow-claim-1"}) {
		t.Fatalf("include claims: %v", resultIDs(claims))
	}
	rel, _ := flowReceipt(t, a, Request{Include: []string{"relations", "paragraphs"}, Sections: []string{"readme"}})
	want := []string{"adapter:flow"}
	for j := range 8 {
		want = append(want, fmt.Sprintf("adapter:p-0-%d", j))
	}
	if !slices.Equal(resultIDs(rel), append(want, "adapter:paragraph", "adapter:test-join")) {
		t.Fatalf("include relations+paragraphs of readme: %v", resultIDs(rel))
	}
	for _, q := range []Request{
		{Operation: "flow", ID: "adapter:flow", Include: []string{"subjects"}},
		{Operation: "flow", ID: "adapter:flow", Include: []string{"claims", "claims"}},
		{Operation: "flow", ID: "adapter:flow", Include: []string{}},
		{Operation: "flow", ID: "adapter:flow", Sections: []string{}},
		{Operation: "flow", ID: "adapter:flow", Sections: []string{"readme"}, Include: []string{"claims"}},
		{Operation: "get", ID: "adapter:flow", Sections: []string{"readme"}},
		{Operation: "concept", Query: "Count", Include: []string{"claims"}},
	} {
		var e *Error
		if _, err := Query(a, q, "fresh", nil); !errors.As(err, &e) || e.Code != "corpus-refused" {
			t.Fatalf("filter %+v admitted: %v", q, err)
		}
	}
}

func TestDCPV1172FlowSelectionReceipt(t *testing.T) {
	t.Parallel()
	_, _, a := eightSectionFlow(t)
	all, allBytes := flowReceipt(t, a, Request{})
	// DCP-V1-172: an unfiltered answer is the whole flow with no selection member.
	ids := resultIDs(all)
	if len(ids) != 1+2+1+65 || ids[0] != "adapter:flow" || ids[1] != "adapter:flow-claim-0" || ids[68] != "adapter:test-join" || strings.Contains(string(allBytes), "flow_selection") {
		t.Fatalf("unfiltered flow changed: %v", ids)
	}
	b, bBytes := flowReceipt(t, a, Request{Sections: []string{"entry-points"}})
	s := b.FlowSelection
	if s == nil || len(s.Sections) != 8 || s.Sections[0].Paragraphs != 9 || s.Sections[1].Paragraphs != 8 || !slices.Equal(s.Include, []string{"paragraphs"}) || !slices.Equal(s.Requested, []string{"entry-points"}) {
		t.Fatalf("selection: %+v", s)
	}
	if s.Counts != (FlowKindCounts{Claims: FlowKindCount{2, 0}, Relations: FlowKindCount{1, 0}, TestKnowledge: FlowKindCount{0, 0}, Paragraphs: FlowKindCount{65, 8}}) {
		t.Fatalf("counts: %+v", s.Counts)
	}
	if !strings.Contains(string(bBytes), `"flow_selection":{"sections":[{"name":"readme","paragraphs":9}`) {
		t.Fatalf("selection not encoded: %s", bBytes)
	}
	every, _ := flowReceipt(t, a, Request{Include: FlowRecordKinds})
	if !slices.Equal(resultIDs(every), ids) || every.FlowSelection.Counts.Claims.Selected != 2 || every.FlowSelection.Counts.Paragraphs.Selected != 65 {
		t.Fatal("include of every kind differs from the unfiltered answer")
	}
}

func TestDCPV1173FlowFilterPagination(t *testing.T) {
	t.Parallel()
	_, _, a := eightSectionFlow(t)
	q := Request{Sections: []string{"entities", "exceptions"}, Include: []string{"claims", "paragraphs"}}
	full, _ := flowReceipt(t, a, q)
	want := resultIDs(full)
	if len(want) != 1+2+16 {
		t.Fatalf("filtered answer: %v", want)
	}
	got := []string{}
	for offset := 0; ; {
		q.Limit, q.Offset = 4, offset
		page, _ := flowReceipt(t, a, q)
		again, _ := flowReceipt(t, a, q)
		if !slices.Equal(resultIDs(page), resultIDs(again)) {
			t.Fatal("filtered page is not deterministic")
		}
		if page.FlowSelection == nil || page.FlowSelection.Counts.Paragraphs.Selected != 16 {
			t.Fatalf("page selection counts: %+v", page.FlowSelection)
		}
		got = append(got, resultIDs(page)...)
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	if !slices.Equal(got, want) {
		t.Fatalf("pages %v != filtered answer %v", got, want)
	}
	q.Offset = 19
	past, _ := flowReceipt(t, a, q)
	if past.Miss != "page-exhausted" {
		t.Fatalf("offset past the filtered answer: %s", past.Miss)
	}
}
