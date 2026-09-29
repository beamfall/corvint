package flowdocs

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

const Schema = "corvint-flow-documentation/1"
const Version = "1"
const MaxFlows = 256
const MaxBytes = 64 << 20
const MaxManifestBytes = 256 << 20
const MaxOutputBytes = MaxManifestBytes + MaxBytes

var sections = []string{"readme", "entry-points", "flow-diagram", "functional-overview", "technical-deep-dive", "entities", "exceptions", "related-flows"}

type Options struct {
	Revision, Scope string
	Corpus          []byte
}
type Paragraph struct {
	ID, Section, Text string
	Anchor            doccorpus.Anchor
	Bindings          []doccorpus.Anchor
	Imported          bool
}
type Call struct {
	Name   string
	Anchor doccorpus.Anchor
}

type Flow struct {
	ID, Name, Path string
	Paragraphs     []Paragraph
	Calls          []Call
}
type Output struct{ Path, SHA256 string }
type Manifest struct {
	OpaqueGitlinks []OpaqueGitlink          `json:"opaque_gitlinks"`
	Schema         string                   `json:"schema"`
	Version        string                   `json:"version"`
	Source         doccorpus.Repository     `json:"source"`
	Scope          string                   `json:"scope"`
	Flows          []Flow                   `json:"flows"`
	Provider       doccorpus.ProviderRecord `json:"provider"`
	Corpus         *doccorpus.Artifact      `json:"corpus,omitempty"`
	Gaps           []string                 `json:"gaps"`
	Outputs        []Output                 `json:"outputs"`
}
type Result struct {
	Manifest Manifest
	Files    map[string][]byte
}

func encodingLimit(v any) int {
	switch v.(type) {
	case Manifest, *Manifest:
		return MaxManifestBytes
	case doccorpus.Manifest, *doccorpus.Manifest:
		return doccorpus.MaxCorpusBytes
	}
	return MaxBytes
}
func Encode(v any) ([]byte, error) {
	b, e := json.Marshal(v, json.Deterministic(true))
	if len(b) > encodingLimit(v) {
		return nil, fail("encoded value exceeds profile byte bound")
	}
	return b, e
}

func validScope(s string) bool {
	return s != "" && s != "." && path.Clean(s) == s && !strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "../") && !strings.ContainsAny(s, "\\\x00\r\n")
}

func Generate(ctx context.Context, root string, o Options) (*Result, error) {
	if !validScope(o.Scope) {
		return nil, fail("scope must be a confined repository path")
	}
	index, err := contextindex.BuildRevisionContext(ctx, root, o.Revision)
	if err != nil {
		return nil, err
	}
	// HTML is intentionally outside native indexing; read only explicitly scoped
	// regular tracked template blobs through the existing immutable Git authority.
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	for name := range index.Tracked {
		if path.Ext(name) != ".html" || !(name == o.Scope || strings.HasPrefix(name, o.Scope+"/")) {
			continue
		}
		entry, ok, e := auth.LookupTreeEntry(ctx, o.Revision, name)
		if e != nil {
			return nil, e
		}
		if !ok || entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" {
			continue
		}
		raw, e := auth.BlobBytes(ctx, entry.OID)
		if e != nil {
			return nil, e
		}
		if len(raw) > 4<<20 {
			return nil, fail("template exceeds source bound")
		}
		index.Sources[name] = contextindex.Source{Path: name, BlobHash: entry.OID, Data: raw, Mode: entry.Mode}
	}
	repository, err := contextindex.CorpusRepositoryID(ctx, root, o.Revision)
	if err != nil {
		return nil, err
	}
	m := Manifest{Schema: Schema, Version: Version, Source: doccorpus.Repository{ID: repository, Revision: o.Revision}, Scope: o.Scope, Flows: []Flow{}, Outputs: []Output{}, Gaps: []string{"Source-only lexical candidates: dynamic dispatch, metaprogramming, regex literals, interpolation, unsupported templates and runtime order are unresolved.", "Accepted variation denominator and runtime coverage are unknown unless separately supplied by revalidated corpus evidence.", "Gitlinks are opaque superproject identities; their contents are not inspected."}}
	m.OpaqueGitlinks, err = opaqueGitlinks(ctx, auth, index, o.Revision, o.Scope)
	if err != nil {
		return nil, err
	}
	if len(o.Corpus) > 0 {
		m.Corpus, err = doccorpus.Open(ctx, root, o.Corpus)
		if err != nil {
			return nil, err
		}
		if m.Corpus.Manifest.Repository != m.Source {
			return nil, fail("corpus source must match generation source; cross-revision joins unresolved")
		}
	}
	p := doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "flowdocs", Version: Version, Source: m.Source, Subjects: []doccorpus.Subject{}, Claims: []doccorpus.Claim{}, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{}, Details: map[string]doccorpus.RecordDetails{}}
	for _, link := range m.OpaqueGitlinks {
		text := "Opaque gitlink " + link.Path + " at " + link.Commit + "; nested contents outside coverage"
		m.Gaps = append(m.Gaps, text)
		p.Subjects = append(p.Subjects, doccorpus.Subject{ID: identity("gitlink", link.Path), Kind: "module", Name: text, Provider: p.ID, Evidence: doccorpus.Evidence{Derivation: "source-derived", Trust: "generated", State: "unknown", Freshness: "unknown", Anchors: []doccorpus.Anchor{}, Unknown: text, Limitations: []string{"Superproject commit identity only; no submodule source or runtime evidence"}}})
	}

	for _, name := range []string{"subjects", "claims", "relations"} {
		p.Capabilities = append(p.Capabilities, doccorpus.CapabilityDeclaration{Name: name, State: "present", Reason: "bounded source-derived inventory, not runtime completeness"})
	}
	for _, name := range []string{"journeys", "observations"} {
		p.Capabilities = append(p.Capabilities, doccorpus.CapabilityDeclaration{Name: name, State: "not-collected", Reason: "source generation does not execute applications; retain original corpus witnesses separately"})
	}
	names := []string{}
	for name := range index.Sources {
		if name == o.Scope || strings.HasPrefix(name, o.Scope+"/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		s := index.Sources[name]
		ext := path.Ext(name)
		if ext != ".rb" && ext != ".js" && ext != ".jsx" && ext != ".ts" && ext != ".tsx" && ext != ".html" {
			continue
		}
		if s.Mode == "120000" {
			m.Gaps = append(m.Gaps, "symbolic link source excluded")
			continue
		}
		text, valid, loaded := s.Text()
		if !valid || !loaded {
			m.Gaps = append(m.Gaps, "unloaded or non-text source excluded")
			continue
		}
		s.Data = []byte(text)
		ds, gaps := extract(s)
		m.Gaps = append(m.Gaps, gaps...)
		duplicate := map[string]int{}
		for _, d := range ds {
			duplicate[d.name]++
		}
		for _, d := range ds {
			if duplicate[d.name] > 1 {
				m.Gaps = append(m.Gaps, "duplicate logical declaration unresolved")
				continue
			}
			if len(m.Flows) >= MaxFlows {
				return nil, fail("flow limit exceeds 256; narrow scope")
			}
			id := identity("flow", name+":"+d.name)
			a := anchor(m.Source, s, d.line)
			for _, symbol := range index.Symbols {
				if symbol.Path == name && symbol.Line == d.line {
					a.Symbol = symbol.Name
					break
				}
			}
			f := Flow{ID: id, Name: d.name, Path: name, Paragraphs: []Paragraph{}}
			language := "web"
			if ext == ".rb" {
				language = "ruby"
			}
			codeLines := contextindex.FlowCodeLines(language, text)
			callKeys := map[string]bool{}
			addCall := func(name string, line int) {
				key := name + ":" + fmt.Sprint(line)
				if !callKeys[key] {
					callKeys[key] = true
					f.Calls = append(f.Calls, Call{Name: name, Anchor: anchor(m.Source, s, line)})
				}
			}
			if ext != ".html" {
				for line := d.line - 1; line < d.end; line++ {
					for _, match := range call.FindAllStringSubmatch(withoutDeclarationName(language, codeLines[line]), -1) {
						addCall(match[1], line+1)
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

			detail := doccorpus.FlowDetails{Actors: []string{}, Entrypoints: []string{id}, Variations: []string{}, Sections: []doccorpus.FlowSection{}}
			add := func(section, key, body string, binding doccorpus.Anchor) string {
				pid := identity("paragraph", id+":"+section+":"+key)
				f.Paragraphs = append(f.Paragraphs, Paragraph{ID: pid, Section: section, Text: body, Anchor: binding})
				p.Subjects = append(p.Subjects, doccorpus.Subject{ID: pid, Kind: "paragraph", Name: body, Provider: p.ID, Evidence: evidence(binding)})
				p.Details[pid] = doccorpus.RecordDetails{Paragraph: &doccorpus.ParagraphDetails{Flow: id, Section: section, StableID: pid}}
				return pid
			}
			for _, section := range sections {
				body := map[string]string{
					"readme":              "Generated source documentation. Priority: unassigned. Confidence: lexical candidate. Trust: generated; accepted intent not established.",
					"entry-points":        "Source declares " + d.name + ". Entry dispatch and callers are unresolved.",
					"flow-diagram":        "Static source inventory only; sequence and runtime ordering are unresolved.",
					"functional-overview": "Accepted variations: unknown. Runtime covered variations: unknown. Denominator: no accepted variation inventory supplied by this generator.",
					"technical-deep-dive": "Source declares " + d.name + "; generated call observations do not establish executed behavior.",
					"entities":            "Runtime entities and persistence effects are unresolved; lexical calls are listed separately.",
					"exceptions":          "Error reachability, rescue behavior and completeness are unresolved.",
					"related-flows":       "Relations are static lexical candidates; computed dispatch remains unresolved.",
				}[section]
				add(section, "summary", body, a)
			}
			counts := map[string]int{}
			for _, obs := range d.observations {
				counts[obs.key]++
			}
			for _, obs := range d.observations {
				if counts[obs.key] > 1 {
					m.Gaps = append(m.Gaps, "duplicate lexical claim identity unresolved")
					continue
				}
				binding := anchor(m.Source, s, obs.line)
				if obs.end > obs.line {
					binding.End = obs.end
					lines := strings.SplitAfter(string(s.Data), "\n")
					binding.SpanSHA256 = doccorpus.Digest([]byte(strings.Join(lines[obs.line-1:obs.end], "")))
				}
				pid := add(obs.section, obs.key, obs.text, binding)
				cid := identity("claim", id+":"+obs.key)
				ev := evidence(binding)
				if a.Start != binding.Start {
					ev.Anchors = append(ev.Anchors, a)
				}
				p.Claims = append(p.Claims, doccorpus.Claim{ID: cid, Subject: id, Text: obs.text, Provider: p.ID, Evidence: ev})
				p.Details[cid] = doccorpus.RecordDetails{ClaimKind: obs.kind}
				p.Relations = append(p.Relations, doccorpus.Relation{ID: identity("relation", pid+cid), From: pid, To: id, Type: "documents", Provider: p.ID, Evidence: evidence(binding)})
			}
			for _, item := range contextParagraphs(m, f) {
				if len(item.anchors) > 64 {
					return nil, fail("imported paragraph anchor bound exceeded")
				}
				binding := a
				if len(item.anchors) > 0 {
					binding = item.anchors[0]
				}
				add(item.section, "imported:"+item.key, item.text, binding)
				f.Paragraphs[len(f.Paragraphs)-1].Imported = true
				f.Paragraphs[len(f.Paragraphs)-1].Bindings = item.anchors
				last := len(p.Subjects) - 1
				p.Subjects[last].Evidence.Anchors = item.anchors
				if len(item.anchors) == 0 {
					p.Subjects[last].Evidence.State = "unknown"
					p.Subjects[last].Evidence.Unknown = "original imported record has no source binding"
				}
			}

			for _, section := range sections {
				item := doccorpus.FlowSection{Name: section, Paragraphs: []string{}}
				for _, para := range f.Paragraphs {
					if para.Section == section {
						item.Paragraphs = append(item.Paragraphs, para.ID)
					}
				}
				detail.Sections = append(detail.Sections, item)
			}
			p.Subjects = append(p.Subjects, doccorpus.Subject{ID: id, Kind: "flow", Name: d.name, Provider: p.ID, Evidence: evidence(a)})
			p.Details[id] = doccorpus.RecordDetails{Flow: &detail}
			m.Flows = append(m.Flows, f)
		}
	}
	if len(m.Flows) == 0 {
		m.Gaps = append(m.Gaps, "no unambiguous supported declarations; empty inventory is not behavioral coverage")
	}
	// Shared source files are an explicit structural relation, not inferred calls.
	for i, f := range m.Flows {
		for j := i + 1; j < len(m.Flows); j++ {
			g := m.Flows[j]
			if f.Path == g.Path {
				p.Relations = append(p.Relations, doccorpus.Relation{ID: identity("relation", f.ID+g.ID), From: f.ID, To: g.ID, Type: "related_to", Provider: p.ID, Evidence: evidence(f.Paragraphs[0].Anchor)})
			}
		}
	}
	// Exact same-file lexical name joins are source dependencies only.
	byName := map[string][]string{}
	for _, f := range m.Flows {
		short := f.Name
		if at := strings.LastIndex(short, "#"); at >= 0 {
			short = short[at+1:]
		}
		byName[f.Path+":"+short] = append(byName[f.Path+":"+short], f.ID)
	}
	for _, f := range m.Flows {
		seen := map[string]bool{}
		for _, c := range f.Calls {
			targets := byName[f.Path+":"+c.Name]
			if len(targets) != 1 || targets[0] == f.ID || seen[targets[0]] {
				continue
			}
			seen[targets[0]] = true
			p.Relations = append(p.Relations, doccorpus.Relation{ID: identity("dependency", f.ID+targets[0]), From: f.ID, To: targets[0], Type: "depends_on", Provider: p.ID, Evidence: evidence(c.Anchor)})
		}
	}
	m.Provider = p
	sort.Strings(m.Gaps)
	m.Gaps = unique(m.Gaps)
	files := render(m)
	raw, err := Encode(p)
	if err != nil {
		return nil, err
	}
	files["provider.json"] = raw
	names = nil
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		total += len(files[name])
		m.Outputs = append(m.Outputs, Output{Path: name, SHA256: doccorpus.Digest(files[name])})
	}
	if total > MaxBytes {
		return nil, fail("output exceeds 64 MiB")
	}
	manifest, err := Encode(m)
	if err != nil {
		return nil, err
	}
	files["generation.json"] = manifest
	return &Result{Manifest: m, Files: files}, nil
}
func unique(s []string) []string {
	r := []string{}
	for _, v := range s {
		if len(r) == 0 || r[len(r)-1] != v {
			r = append(r, v)
		}
	}
	return r
}

// Open distrusts carried IDs, prose, bindings and output digests. Every byte is
// rederived from the pinned source and revalidated optional corpus.
func Open(ctx context.Context, root string, raw []byte) (*Result, error) {
	if len(raw) > MaxManifestBytes {
		return nil, fail("manifest exceeds 256 MiB bound")
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m, json.RejectUnknownMembers(true)); err != nil {
		return nil, fail("invalid closed generation manifest")
	}
	if m.Schema != Schema || m.Version != Version {
		return nil, fail("unsupported generator version")
	}
	var corpus []byte
	var err error
	if m.Corpus != nil {
		corpus, err = doccorpus.Encode(m.Corpus)
		if err != nil {
			return nil, err
		}
	}
	r, err := Generate(ctx, root, Options{Revision: m.Source.Revision, Scope: m.Scope, Corpus: corpus})
	if err != nil {
		return nil, err
	}
	expected, err := Encode(r.Manifest)
	if err != nil {
		return nil, err
	}
	actual, err := Encode(m)
	if err != nil {
		return nil, err
	}
	if string(expected) != string(actual) {
		return nil, fail("generation manifest differs from immutable rederivation")
	}
	return r, nil
}

// Finalize is read-only: after the caller commits provider.json, it binds that
// later provider commit while preserving the original source revision.
func Finalize(ctx context.Context, root string, r *Result, revision, providerPath string) (doccorpus.Manifest, error) {
	m, err := sourceInventory(ctx, root, r.Manifest)
	if err != nil {
		return m, err
	}
	if r.Manifest.Corpus != nil {
		sourceInventory := m
		m = r.Manifest.Corpus.Manifest
		existing := map[string]bool{}
		for _, in := range m.Inputs {
			existing[in.Revision+":"+in.Path] = true
		}
		hasNative := false
		for _, p := range m.Providers {
			hasNative = hasNative || p.ID == "native" && p.Kind == "native"
		}
		if !hasNative {
			m.Providers = append(m.Providers, sourceInventory.Providers[0])
		}
		for _, in := range sourceInventory.Inputs {
			if !existing[in.Revision+":"+in.Path] {
				m.Inputs = append(m.Inputs, in)
			}
		}
		scopeSet := map[doccorpus.Scope]bool{}
		for _, scope := range m.Scopes {
			scopeSet[scope] = true
		}
		for _, scope := range sourceInventory.Scopes {
			if !scopeSet[scope] {
				m.Scopes = append(m.Scopes, scope)
				scopeSet[scope] = true
			}
		}

	}
	inventory, err := doccorpus.Inventory(ctx, root, revision, providerPath, "2000-01-01T00:00:00Z")
	if err != nil {
		return m, err
	}
	if len(inventory.Inputs) != 1 || inventory.Inputs[0].SHA256 != doccorpus.Digest(r.Files["provider.json"]) {
		return m, fail("committed provider differs from generated provider")
	}
	for _, p := range m.Providers {
		if p.ID == "flowdocs" {
			return m, fail("corpus already contains flowdocs provider")
		}
	}
	m.Schema = doccorpus.ManifestSchemaV2
	m.MergeRule = "disjoint-union"
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "flowdocs", Kind: "records", Version: Version, Revision: revision, Record: providerPath})
	in := inventory.Inputs[0]
	in.Provider = "flowdocs"
	in.Purpose = "provider"
	m.Inputs = append(m.Inputs, in)
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: providerPath, Revision: revision})
	if _, err = doccorpus.Build(ctx, root, m); err != nil {
		return m, err
	}
	return m, nil
}
