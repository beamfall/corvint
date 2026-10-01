package doccorpus

import (
	"context"
	"sort"
)

// OperationInput is shared by CLI, MCP and indexed companions. The native
// Query function retains compatibility with older permissive pure callers.
func OperationInput(op string) string {
	switch op {
	case "search", "concept", "vocabulary":
		return "query"
	case "locate", "claims", "recommend-tests":
		return "path"
	case "get", "trace", "related", "journey", "stability", "flow", "dependencies", "navigation", "intent":
		return "id"
	case "info", "validate", "inventory", "coverage", "gaps":
		return ""
	}
	return "unsupported"
}
func typedCapability(op string) string {
	switch op {
	case "concept", "flow", "navigation", "vocabulary", "intent":
		return "subjects"
	case "claims":
		return "claims"
	case "dependencies", "recommend-tests":
		return "relations"
	}
	return ""
}

type DependencyEdge struct {
	Direction string   `json:"direction"`
	Relation  Relation `json:"relation"`
}
type NavigationSelector struct {
	ID         string `json:"id"`
	Flow       string `json:"flow"`
	Test       string `json:"test"`
	Locator    string `json:"locator"`
	Annotation Anchor `json:"annotation"`
	Trust      string `json:"trust"`
}
type TestRecommendation struct {
	State            string   `json:"state"`
	NarrowingAllowed bool     `json:"narrowing_allowed"`
	Required         string   `json:"required"`
	MissingEvidence  []string `json:"missing_evidence"`
	Paths            []string `json:"paths"`
}

func addRecord(r *Receipt, v any) {
	r.Results = append(r.Results, v)
	r.Citations = append(r.Citations, recordAnchors(v)...)
}
func indexedSubject(a *Artifact, id string) (Subject, bool) {
	ref, ok := a.RuntimeIndex.IDs[id]
	if !ok || ref.Kind != "subject" {
		return Subject{}, false
	}
	return a.Subjects[ref.Offset], true
}
func typedQuery(ctx context.Context, a *Artifact, q Request, r *Receipt) error {
	x := a.RuntimeIndex
	switch q.Operation {
	case "concept", "vocabulary":
		if err := search(ctx, a, q, r); err != nil {
			return err
		}
		selected := map[string]bool{}
		for _, v := range r.Results {
			s := v.(Subject)
			if q.Operation == "concept" || s.Kind == "business_term" {
				selected[s.ID] = true
			}
		}
		r.Results = nil
		for id := range selected {
			s, _ := indexedSubject(a, id)
			addRecord(r, s)
			for _, i := range x.Edges[id] {
				if err := ctx.Err(); err != nil {
					return err
				}
				rel := a.Relations[i]
				addRecord(r, rel)
				for _, target := range []string{rel.From, rel.To} {
					if s, ok := indexedSubject(a, target); ok && s.Kind == "flow" {
						addRecord(r, s)
					}
				}
			}
		}
		for _, s := range a.Subjects {
			if err := ctx.Err(); err != nil {
				return err
			}
			if flow := a.Details[s.ID].Flow; flow != nil {
				for _, id := range flow.Entrypoints {
					if selected[id] {
						addRecord(r, s)
						break
					}
				}
			}
		}
	case "claims":
		for _, ref := range x.Paths[q.Path] {
			if err := ctx.Err(); err != nil {
				return err
			}
			if ref.Kind == "claim" {
				addRecord(r, a.indexedRecord(ref))
			}
		}
	case "flow":
		if s, ok := indexedSubject(a, q.ID); ok && a.Details[s.ID].Flow != nil {
			addRecord(r, s)
			for _, i := range x.Claims[q.ID] {
				addRecord(r, a.Claims[i])
			}
			for _, i := range x.Edges[q.ID] {
				addRecord(r, a.Relations[i])
			}
			for _, section := range a.Details[s.ID].Flow.Sections {
				for _, id := range section.Paragraphs {
					if err := ctx.Err(); err != nil {
						return err
					}
					if p, ok := indexedSubject(a, id); ok {
						addRecord(r, p)
					}
				}
			}
		}
	case "dependencies":
		for _, direction := range []string{"upstream", "downstream"} {
			seen := map[string]bool{q.ID: true}
			edges := map[string]bool{}
			queue := []string{q.ID}
			work := 0
			for len(queue) > 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
				id := queue[0]
				queue = queue[1:]
				for _, i := range x.Edges[id] {
					work++
					if work > MaxIndexPostings {
						return fail("dependency traversal bound exceeded")
					}
					if err := ctx.Err(); err != nil {
						return err
					}
					rel := a.Relations[i]
					if rel.Type != "depends_on" {
						continue
					}
					next := ""
					if direction == "downstream" && rel.From == id {
						next = rel.To
					}
					if direction == "upstream" && rel.To == id {
						next = rel.From
					}
					if next == "" {
						continue
					}
					if !edges[rel.ID] {
						edges[rel.ID] = true
						addRecord(r, DependencyEdge{direction, rel})
					}
					if !seen[next] {
						seen[next] = true
						queue = append(queue, next)
					}
				}
			}
		}
		r.Limitations = append(r.Limitations, "only explicit directed depends_on edges; dynamic or undocumented dependencies remain unknown")
	case "recommend-tests":
		selected := map[string]bool{}
		for _, ref := range x.Paths[q.Path] {
			if err := ctx.Err(); err != nil {
				return err
			}
			switch ref.Kind {
			case "subject":
				s := a.Subjects[ref.Offset]
				selected[s.ID] = true
				addRecord(r, s)
			case "claim":
				selected[a.Claims[ref.Offset].Subject] = true
			}
		}
		for id := range selected {
			for _, i := range x.Edges[id] {
				if err := ctx.Err(); err != nil {
					return err
				}
				rel := a.Relations[i]
				if a.Details[rel.ID].TestLink != nil {
					addRecord(r, rel)
				}
			}
		}
		missing := []string{"source-to-test completeness and semantic assertion adequacy NOT_OBSERVED", "manual case completeness NOT_OBSERVED"}
		if len(selected) == 0 {
			missing = append(missing, "changed path not documented")
		}
		if r.Freshness != "fresh" {
			missing = append(missing, "fresh corpus evidence NOT_OBSERVED")
		}
		r.Selection = &TestRecommendation{State: "full-relevant-suite-required", Required: "full relevant suite and mandatory repository checks", MissingEvidence: missing, Paths: []string{q.Path}}
	case "navigation":
		selected := map[string]bool{q.ID: true}
		if flow := a.Details[q.ID].Flow; flow != nil {
			for _, id := range flow.Entrypoints {
				selected[id] = true
			}
		}
		for _, i := range x.Edges[q.ID] {
			rel := a.Relations[i]
			selected[rel.From] = true
			selected[rel.To] = true
			if rel.Type == "navigates" {
				addRecord(r, rel)
			}
		}
		for id := range selected {
			if err := ctx.Err(); err != nil {
				return err
			}
			if s, ok := indexedSubject(a, id); ok && (s.Kind == "endpoint" || s.Kind == "ui_surface" || s.Kind == "interaction" || id != q.ID) {
				addRecord(r, s)
			}
		}
		for _, report := range a.BehaviorContracts {
			for _, test := range report.Registry.Tests {
				if err := ctx.Err(); err != nil {
					return err
				}
				linked := false
				for _, flow := range test.Flows {
					linked = linked || flow == q.ID
				}
				if linked {
					for _, assertion := range test.Assertions {
						addRecord(r, NavigationSelector{ID: test.ID + ":" + assertion.ID, Flow: q.ID, Test: test.ID, Locator: assertion.Locator, Annotation: assertion.Annotation, Trust: "attributed-declaration"})
					}
				}
			}
		}
		r.Limitations = append(r.Limitations, "routes, screens and selectors are documented anchors/declarations; live UI and navigation completeness NOT_OBSERVED")
	case "intent":
		targets := map[string]bool{q.ID: true}
		for _, i := range x.Edges[q.ID] {
			if err := ctx.Err(); err != nil {
				return err
			}
			rel := a.Relations[i]
			targets[rel.From] = true
			targets[rel.To] = true
		}
		for id := range targets {
			if s, ok := indexedSubject(a, id); ok && (a.Details[s.ID].Ticket != nil || a.Details[s.ID].IntentComparison != nil) {
				addRecord(r, s)
			}
		}
		r.Limitations = append(r.Limitations, "recorded intent and observed claims are distinct attributed records; no accepted intent inferred")
	}
	sort.SliceStable(r.Results, func(i, j int) bool { return typedKey(r.Results[i]) < typedKey(r.Results[j]) })
	unique := r.Results[:0]
	last := ""
	for _, v := range r.Results {
		k := typedKey(v)
		if k != last {
			unique = append(unique, v)
			last = k
		}
	}
	r.Results = unique
	return ctx.Err()
}
func typedKey(v any) string {
	if e, ok := v.(DependencyEdge); ok {
		return e.Direction + ":" + e.Relation.ID
	}
	return recordID(v)
}
func typedGaps(ctx context.Context, a *Artifact, freshness string) ([]Gap, error) {
	out := []Gap{}
	journeys := map[string][]Journey{}
	for _, j := range a.Journeys {
		journeys[j.Subject] = append(journeys[j.Subject], j)
	}
	for _, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if s.Kind != "flow" {
			continue
		}
		tests, asserting, manual := false, false, false
		for _, i := range a.RuntimeIndex.Edges[s.ID] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			rel := a.Relations[i]
			if link := a.Details[rel.ID].TestLink; link != nil {
				tests = true
				if link.Role == "asserting" && link.JoinConfidence == "exact" && rel.Evidence.Trust == "reviewed" && rel.Evidence.State == "supported" {
					asserting = true
				}
			}
			if rel.Type == "tested_by" {
				tests = true
			}
			if rel.Type == "journey_for" {
				manual = true
			}
		}
		for _, j := range journeys[s.ID] {
			manual = true
			if j.Evidence.Freshness == "stale" || freshness == "stale" {
				out = append(out, Gap{s.ID, "stale-journey", "journey evidence is stale"})
			}
		}
		if !tests {
			out = append(out, Gap{s.ID, "no-tests", "no documented test links"})
		}
		if !asserting {
			out = append(out, Gap{s.ID, "missing-asserting-e2e-tests", "no exact reviewed asserting test link; adequacy remains unknown"})
		}
		if !manual {
			out = append(out, Gap{s.ID, "missing-manual-cases", "no documented manual disposition; non-UI disposition NOT_OBSERVED"})
		}
		if freshness == "stale" || s.Evidence.Freshness == "stale" {
			out = append(out, Gap{s.ID, "stale-anchors", "source freshness is stale"})
		}
	}
	return out, ctx.Err()
}
