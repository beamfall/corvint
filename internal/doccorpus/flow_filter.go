package doccorpus

import (
	"context"
	"slices"
)

// FlowRecordKinds is the closed set of record kinds a flow query can select
// with Request.Include, in the order the flow answer emits them (DCP-V1-171).
var FlowRecordKinds = []string{"claims", "relations", "test_knowledge", "paragraphs"}

// MaxFlowSections bounds the section names one flow query may request.
const MaxFlowSections = 64

// FlowSelection is the receipt member a filtered flow query carries
// (DCP-V1-172). Sections lists every section of the flow in flow order with
// its paragraph count; Counts reports, per record kind, how many records the
// flow has and how many the filter selected before limit/offset paging. An
// unfiltered flow query carries no FlowSelection, so its bytes are unchanged.
type FlowSelection struct {
	Sections  []FlowSectionCount `json:"sections"`
	Requested []string           `json:"requested_sections,omitempty"`
	Unknown   []string           `json:"unknown_sections,omitempty"`
	Include   []string           `json:"include"`
	Counts    FlowKindCounts     `json:"counts"`
}
type FlowSectionCount struct {
	Name       string `json:"name"`
	Paragraphs int    `json:"paragraphs"`
}
type FlowKindCount struct {
	Available int `json:"available"`
	Selected  int `json:"selected"`
}
type FlowKindCounts struct {
	Claims        FlowKindCount `json:"claims"`
	Relations     FlowKindCount `json:"relations"`
	TestKnowledge FlowKindCount `json:"test_knowledge"`
	Paragraphs    FlowKindCount `json:"paragraphs"`
}

// ValidFlowFilter reports whether a flow section/include filter is well formed
// (DCP-V1-170, DCP-V1-171): a nil list is absent; a present list is non-empty,
// bounded and duplicate-free; include values come from FlowRecordKinds; and a
// section filter requires paragraphs among the included kinds.
func ValidFlowFilter(sections, include []string) bool {
	if sections != nil && (len(sections) == 0 || len(sections) > MaxFlowSections) || include != nil && (len(include) == 0 || len(include) > len(FlowRecordKinds)) {
		return false
	}
	for i, s := range sections {
		if s == "" || len(s) > 1024 || slices.Contains(sections[:i], s) {
			return false
		}
	}
	for i, k := range include {
		if !slices.Contains(FlowRecordKinds, k) || slices.Contains(include[:i], k) {
			return false
		}
	}
	return sections == nil || include == nil || slices.Contains(include, "paragraphs")
}

// flowQuery answers one flow: its subject, then its claims, relations, joined
// test knowledge and section paragraphs. Without a filter the answer is the
// whole flow (DCP-V1-039); with one, the subject plus the selected kinds and
// sections (DCP-V1-170..173).
func flowQuery(ctx context.Context, a *Artifact, q Request, r *Receipt) error {
	x := a.RuntimeIndex
	s, ok := indexedSubject(a, q.ID)
	if !ok || a.Details[s.ID].Flow == nil {
		return nil
	}
	flow := a.Details[s.ID].Flow
	filtered := q.Sections != nil || q.Include != nil
	include := FlowRecordKinds
	if q.Include != nil {
		include = q.Include
	} else if q.Sections != nil {
		include = []string{"paragraphs"}
	}
	want := func(kind string) bool { return slices.Contains(include, kind) }
	knowledge := []TestKnowledgeJoin{}
	for _, j := range a.TestKnowledge {
		if j.Status == "joined" && slices.Contains(j.Flows, q.ID) {
			knowledge = append(knowledge, j)
		}
	}
	var selection *FlowSelection
	if filtered {
		selection = &FlowSelection{Sections: []FlowSectionCount{}, Requested: q.Sections, Include: []string{}}
		for _, kind := range FlowRecordKinds {
			if want(kind) {
				selection.Include = append(selection.Include, kind)
			}
		}
		names := map[string]bool{}
		for _, section := range flow.Sections {
			names[section.Name] = true
			n := 0
			for _, id := range section.Paragraphs {
				if _, ok := indexedSubject(a, id); ok {
					n++
				}
			}
			selection.Sections = append(selection.Sections, FlowSectionCount{section.Name, n})
			selection.Counts.Paragraphs.Available += n
		}
		for _, name := range q.Sections {
			if !names[name] {
				selection.Unknown = append(selection.Unknown, name)
			}
		}
		selection.Counts.Claims.Available = len(x.Claims[q.ID])
		selection.Counts.Relations.Available = len(x.Edges[q.ID])
		selection.Counts.TestKnowledge.Available = len(knowledge)
		for kind, count := range map[string]*FlowKindCount{"claims": &selection.Counts.Claims, "relations": &selection.Counts.Relations, "test_knowledge": &selection.Counts.TestKnowledge} {
			if want(kind) {
				count.Selected = count.Available
			}
		}
		r.FlowSelection = selection
		if len(selection.Unknown) > 0 {
			// DCP-V1-170: an unknown section is a typed none-found that names
			// the flow's sections; it never falls back to the whole flow.
			r.State, r.Miss = "empty", "none-found"
			r.Limitations = append(r.Limitations, "requested flow section not found; flow_selection.sections lists the available sections")
			return nil
		}
	}
	addRecord(r, s)
	if want("claims") {
		for _, i := range x.Claims[q.ID] {
			addRecord(r, a.Claims[i])
		}
	}
	if want("relations") {
		for _, i := range x.Edges[q.ID] {
			addRecord(r, a.Relations[i])
		}
	}
	if want("test_knowledge") {
		for _, j := range knowledge {
			addRecord(r, j)
		}
	}
	if !want("paragraphs") {
		return nil
	}
	for _, section := range flow.Sections {
		if q.Sections != nil && !slices.Contains(q.Sections, section.Name) {
			continue
		}
		for _, id := range section.Paragraphs {
			if err := ctx.Err(); err != nil {
				return err
			}
			if p, ok := indexedSubject(a, id); ok {
				addRecord(r, p)
				if selection != nil {
					selection.Counts.Paragraphs.Selected++
				}
			}
		}
	}
	return nil
}
