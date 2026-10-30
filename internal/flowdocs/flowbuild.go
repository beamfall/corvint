package flowdocs

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

// memberDecl is one unambiguous in-scope declaration and its source.
type memberDecl struct {
	path, ext, text string
	source          contextindex.Source
	d               declaration
	a               doccorpus.Anchor
}

// flowBuilder emits one flow's paragraphs, claims and calls into the manifest
// and provider. A declaration flow has one member; a declared inventory flow
// aggregates every resolved member under its own identity.
type flowBuilder struct {
	m     *Manifest
	p     *doccorpus.ProviderRecord
	f     Flow
	calls map[string]bool
}

func (b *flowBuilder) add(section, key, body string, binding doccorpus.Anchor) string {
	pid := identity("paragraph", b.f.ID+":"+section+":"+key)
	b.f.Paragraphs = append(b.f.Paragraphs, Paragraph{ID: pid, Section: section, Text: body, Anchor: binding})
	b.p.Subjects = append(b.p.Subjects, doccorpus.Subject{ID: pid, Kind: "paragraph", Name: body, Provider: b.p.ID, Evidence: evidence(binding)})
	b.p.Details[pid] = doccorpus.RecordDetails{Paragraph: &doccorpus.ParagraphDetails{Flow: b.f.ID, Section: section, StableID: pid}}
	return pid
}

// summaries adds one summary paragraph per section; override replaces the
// default body of a section.
func (b *flowBuilder) summaries(override map[string]string, a doccorpus.Anchor) {
	for _, section := range sections {
		body := map[string]string{
			"readme":              "Generated source documentation. Priority: unassigned. Confidence: lexical candidate. Trust: generated; accepted intent not established.",
			"flow-diagram":        "Static source inventory only; sequence and runtime ordering are unresolved.",
			"functional-overview": "Accepted variations: unknown. Runtime covered variations: unknown. Denominator: no accepted variation inventory supplied by this generator.",
			"entities":            "Runtime entities and persistence effects are unresolved; lexical calls are listed separately.",
			"exceptions":          "Error reachability, rescue behavior and completeness are unresolved.",
			"related-flows":       "Relations are static lexical candidates; computed dispatch remains unresolved.",
		}[section]
		if text, ok := override[section]; ok {
			body = text
		}
		b.add(section, "summary", body, a)
	}
}

// member adds one declaration's static calls, claims and imported corpus
// context. prefix keeps claim identities distinct across members; from is the
// first paragraph that locates imported corpus context for this member.
func (b *flowBuilder) member(prefix string, md memberDecl, from int) error {
	s, d, a := md.source, md.d, md.a
	language := "web"
	if md.ext == ".rb" {
		language = "ruby"
	}
	addCall := func(name string, line int) {
		key := md.path + ":" + name + ":" + fmt.Sprint(line)
		if !b.calls[key] {
			b.calls[key] = true
			b.f.Calls = append(b.f.Calls, Call{Name: name, Anchor: anchor(b.m.Source, s, line)})
		}
	}
	if md.ext != ".html" {
		codeLines := contextindex.FlowCodeLines(language, md.text)
		for line := d.line - 1; line < d.end; line++ {
			code := withoutDeclarationName(language, codeLines[line])
			for _, match := range call.FindAllStringSubmatchIndex(code, -1) {
				name := code[match[2]:match[3]]
				if !controlKeyword[name] || memberAccess(code, match[2]) {
					addCall(name, line+1)
				}
			}
		}
		for _, obs := range d.observations {
			if obs.kind != "behavior" {
				_, name, _ := strings.Cut(obs.key, ":")
				if name != "throw" {
					addCall(name, obs.line)
				}
			}
		}
	}
	counts := map[string]int{}
	for _, obs := range d.observations {
		counts[obs.key]++
	}
	for _, obs := range d.observations {
		if counts[obs.key] > 1 {
			b.m.Gaps = append(b.m.Gaps, "duplicate lexical claim identity unresolved")
			continue
		}
		binding := anchor(b.m.Source, s, obs.line)
		if obs.end > obs.line {
			binding.End = obs.end
			lines := strings.SplitAfter(string(s.Data), "\n")
			binding.SpanSHA256 = doccorpus.Digest([]byte(strings.Join(lines[obs.line-1:obs.end], "")))
		}
		key := prefix + obs.key
		pid := b.add(obs.section, key, obs.text, binding)
		cid := identity("claim", b.f.ID+":"+key)
		ev := evidence(binding)
		if a.Start != binding.Start {
			ev.Anchors = append(ev.Anchors, a)
		}
		b.p.Claims = append(b.p.Claims, doccorpus.Claim{ID: cid, Subject: b.f.ID, Text: obs.text, Provider: b.p.ID, Evidence: ev})
		b.p.Details[cid] = doccorpus.RecordDetails{ClaimKind: obs.kind}
		b.p.Relations = append(b.p.Relations, doccorpus.Relation{ID: identity("relation", pid+cid), From: pid, To: b.f.ID, Type: "documents", Provider: b.p.ID, Evidence: evidence(binding)})
	}
	for _, item := range contextParagraphs(*b.m, Flow{Path: md.path, Paragraphs: b.f.Paragraphs[from:]}) {
		if len(item.anchors) > 64 {
			return fail("imported paragraph anchor bound exceeded")
		}
		binding := a
		if len(item.anchors) > 0 {
			binding = item.anchors[0]
		}
		b.add(item.section, "imported:"+prefix+item.key, item.text, binding)
		b.f.Paragraphs[len(b.f.Paragraphs)-1].Imported = true
		b.f.Paragraphs[len(b.f.Paragraphs)-1].Bindings = item.anchors
		last := len(b.p.Subjects) - 1
		b.p.Subjects[last].Evidence.Anchors = item.anchors
		if len(item.anchors) == 0 {
			b.p.Subjects[last].Evidence.State = "unknown"
			b.p.Subjects[last].Evidence.Unknown = "original imported record has no source binding"
		}
	}
	return nil
}

// finish records the flow's section index and flow subject, bound at a.
func (b *flowBuilder) finish(a doccorpus.Anchor) {
	detail := doccorpus.FlowDetails{Actors: []string{}, Entrypoints: []string{b.f.ID}, Variations: []string{}, Sections: []doccorpus.FlowSection{}}
	for _, section := range sections {
		item := doccorpus.FlowSection{Name: section, Paragraphs: []string{}}
		for _, para := range b.f.Paragraphs {
			if para.Section == section {
				item.Paragraphs = append(item.Paragraphs, para.ID)
			}
		}
		detail.Sections = append(detail.Sections, item)
	}
	b.p.Subjects = append(b.p.Subjects, doccorpus.Subject{ID: b.f.ID, Kind: "flow", Name: b.f.Name, Provider: b.p.ID, Evidence: evidence(a)})
	b.p.Details[b.f.ID] = doccorpus.RecordDetails{Flow: &detail}
	b.m.Flows = append(b.m.Flows, b.f)
}
