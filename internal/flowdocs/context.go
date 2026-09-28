package flowdocs

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

type contextParagraph struct {
	key, section, text string
	anchors            []doccorpus.Anchor
}

func contextParagraphs(m Manifest, f Flow) []contextParagraph {
	if m.Corpus == nil {
		return nil
	}
	result := []contextParagraph{}
	ids := map[string]bool{}
	matched := func(e doccorpus.Evidence) bool {
		for _, a := range e.Anchors {
			if a.Path == f.Path && a.Revision == m.Source.Revision {
				for _, p := range f.Paragraphs {
					if a.Start <= p.Anchor.Start && a.End >= p.Anchor.Start {
						return true
					}
				}
			}
		}
		return false
	}
	add := func(key, section, text string, e doccorpus.Evidence, extra []doccorpus.Anchor) {
		anchors := append([]doccorpus.Anchor{}, e.Anchors...)
		anchors = append(anchors, extra...)
		text += fmt.Sprintf(" Original trust %s; freshness %s; derivation %s. Source-span overlap is a locator, not a semantic flow join.", e.Trust, e.Freshness, e.Derivation)
		for _, a := range anchors {
			text += fmt.Sprintf(" Citation %s:%d-%d @ %s; span %s.", a.Path, a.Start, a.End, a.Revision, a.SpanSHA256)
		}
		result = append(result, contextParagraph{key: key, section: section, text: text, anchors: anchors})
	}
	for _, s := range m.Corpus.Subjects {
		if matched(s.Evidence) {
			ids[s.ID] = true
		}
	}
	for _, j := range m.Corpus.Journeys {
		if !ids[j.Subject] {
			continue
		}
		add(j.ID, "functional-overview", fmt.Sprintf("Imported journey %s, status %s; preconditions %s; cleanup %s.", j.ID, j.Status, strings.Join(j.Preconditions, ", "), j.Cleanup), j.Evidence, nil)
		for i, step := range j.Steps {
			add(j.ID+":"+step.ID, "functional-overview", fmt.Sprintf("Journey %s step %d: %s; operation %s; expected %s; observation %s.", j.ID, i+1, step.Action, step.Operation, step.Expected, step.Observation), step.Evidence, j.Evidence.Anchors)
		}
	}
	for _, r := range m.Corpus.Relations {
		if !ids[r.From] && !ids[r.To] {
			continue
		}
		d := m.Corpus.Details[r.ID]
		if d.TestLink != nil {
			x := d.TestLink
			add(r.ID, "technical-deep-dive", fmt.Sprintf("Original test join %s: role %s, confidence %s, target %s, file %s, title %s, project %s.", r.ID, x.Role, x.JoinConfidence, r.To, x.File, x.Title, x.Project), r.Evidence, nil)
		}
	}
	for _, s := range m.Corpus.Subjects {
		if !ids[s.ID] {
			continue
		}
		d := m.Corpus.Details[s.ID]
		if d.Coverage != nil {
			x := d.Coverage
			add(s.ID, "functional-overview", fmt.Sprintf("Imported coverage %s: %s; rule %s; numerator %s; denominator %s.", s.ID, x.Definition, x.Rule, strings.Join(x.Numerator, ", "), strings.Join(x.Denominator, ", ")), s.Evidence, nil)
		}
		if d.Ticket != nil {
			x := d.Ticket
			text := fmt.Sprintf("Historical ticket %s (%s).", x.ExternalID, x.Status)
			for _, h := range x.History {
				text += fmt.Sprintf(" %s at %s: %s.", h.At, h.Revision, h.Summary)
			}
			add(s.ID, "technical-deep-dive", text, s.Evidence, nil)
		}
		if d.IntentComparison != nil {
			raw, _ := Encode(d.IntentComparison)
			add(s.ID, "technical-deep-dive", "Imported intent comparison "+s.ID+": "+string(raw)+". No generated intent accepted.", s.Evidence, nil)
		}
	}
	return result
}
