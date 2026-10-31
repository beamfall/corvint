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

// Version is the generator version a manifest records. Version 2 (V1-0461)
// binds each flow's static call inventory and drops bare control keywords
// from it, so a version 1 manifest can no longer be rederived byte for byte.
// Version 3 (V1-0461, AFU-V1-043) stops treating member-call arrows and
// non-route HTTP-verb member calls as web declarations, so version 2
// manifests can no longer be rederived byte for byte either. Version 4
// (V1-2415, AFU-V1-043) passes over a generic call's type-argument list
// instead of masking the rest of the file as JSX text, so a version 3
// manifest of such a file can no longer be rederived byte for byte.
const Version = "4"

// predecessorVersions are generator versions whose manifests are refused with
// StaleGeneratorCode rather than the generic unsupported-version failure.
var predecessorVersions = map[string]bool{"1": true, "2": true, "3": true}

// StaleGeneratorCode is the typed refusal of a manifest an older generator
// wrote: it must be regenerated, not compared (AFU-V1-045).
const StaleGeneratorCode = "flowdocs-generator-stale"

// CorpusRepositoryMismatchCode refuses a --corpus whose primary repository or revision is not the
// generation source, before the corpus is rederived against --root (AFU-V1-091).
const CorpusRepositoryMismatchCode = "flowdocs-corpus-repository-mismatch"

const FlowLimitCode = "flowdocs-flow-limit-exceeded"
const MaxFlows = 256
const MaxBytes = 64 << 20
const MaxManifestBytes = 256 << 20
const MaxOutputBytes = MaxManifestBytes + MaxBytes

var sections = []string{"readme", "entry-points", "flow-diagram", "functional-overview", "technical-deep-dive", "entities", "exceptions", "related-flows"}

type Options struct {
	Revision, Scope string
	Corpus          []byte
	// Flows is an optional declared flow inventory (AFU-V1-101).
	Flows []byte
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
	Inventory      *InventoryReport         `json:"inventory,omitempty"`
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

// flowSourceExt is the closed AFU-V1-043 source set. ES module (.mjs) and
// CommonJS (.cjs) files are JavaScript and take the same lexical path as .js;
// before V1-0461 they were skipped with no gap, so a module-only scope
// reported zero flows without saying why.
var flowSourceExt = map[string]bool{
	".rb": true, ".js": true, ".mjs": true, ".cjs": true, ".jsx": true, ".ts": true, ".tsx": true, ".html": true,
}

// controlKeyword names syntax that the lexical call pattern matches before a
// parenthesis (`if (`, `function (`, `return (`) but that is never a call.
// Listing them as static calls made every diagram open with "call if".
var controlKeyword = map[string]bool{
	"if": true, "elsif": true, "unless": true, "while": true, "until": true, "for": true, "switch": true,
	"case": true, "when": true, "catch": true, "function": true, "return": true, "typeof": true,
	"await": true, "yield": true, "void": true, "delete": true, "in": true, "of": true, "do": true,
	"else": true, "and": true, "or": true, "not": true,
}

// memberAccess reports whether the identifier at start is a property access
// (`promise.catch(`, `iterator.return(`, `a?.delete(`): such a name is a real
// call even when it spells a control keyword.
func memberAccess(code string, start int) bool {
	before := strings.TrimRight(code[:start], " \t")
	return strings.HasSuffix(before, ".")
}

func Generate(ctx context.Context, root string, o Options) (*Result, error) {
	if !validScope(o.Scope) {
		return nil, fail("scope must be a confined repository path")
	}
	inv, err := scopedInventory(o.Flows, o.Scope)
	if err != nil {
		return nil, err
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
		if err = corpusSourceMatches(o.Corpus, m.Source); err != nil {
			return nil, err
		}
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
	decls := []memberDecl{}
	ambiguous := map[string]bool{}
	candidates := 0
	for _, name := range names {
		s := index.Sources[name]
		ext := path.Ext(name)
		if !flowSourceExt[ext] {
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
				ambiguous[name+":"+d.name] = true
				continue
			}
			if inv == nil {
				if candidates++; candidates > MaxFlows {
					continue
				}
			}
			a := anchor(m.Source, s, d.line)
			for _, symbol := range index.Symbols {
				if symbol.Path == name && symbol.Line == d.line {
					a.Symbol = symbol.Name
					break
				}
			}
			md := memberDecl{path: name, ext: ext, text: text, source: s, d: d, a: a}
			// With a declared inventory every declaration is a member
			// candidate, never a flow of its own (AFU-V1-102).
			if inv != nil {
				if len(decls) >= MaxInventoryDeclarations {
					return nil, fail("in-scope declarations exceed 16384; narrow scope")
				}
				decls = append(decls, md)
				continue
			}
			b := &flowBuilder{m: &m, p: &p, f: Flow{ID: identity("flow", name+":"+d.name), Name: d.name, Path: name, Paragraphs: []Paragraph{}}, calls: map[string]bool{}}
			b.summaries(map[string]string{
				"entry-points":        "Source declares " + d.name + ". Entry dispatch and callers are unresolved.",
				"technical-deep-dive": "Source declares " + d.name + "; generated call observations do not establish executed behavior.",
			}, a)
			if err := b.member("", md, 0); err != nil {
				return nil, err
			}
			b.finish(a)
		}
	}
	if candidates > MaxFlows {
		return nil, &doccorpus.Error{Code: FlowLimitCode, Message: fmt.Sprintf("scope %q has %d flow candidates, over the MaxFlows limit of %d; narrow --scope", o.Scope, candidates, MaxFlows)}
	}
	if inv != nil {
		if m.Inventory, err = buildInventory(&m, &p, inv, decls, ambiguous); err != nil {
			return nil, err
		}
	}
	if len(m.Flows) == 0 {
		m.Gaps = append(m.Gaps, "no unambiguous supported declarations; empty inventory is not behavioral coverage")
	}
	// Inventory flows span files and carry caller titles, so the same-file
	// and same-name joins below do not apply to them (AFU-V1-102).
	related := m.Flows
	if inv != nil {
		related = nil
		m.Gaps = append(m.Gaps, "Declared inventory flows: structural and lexical relations between flows are not derived.")
	}
	// Shared source files are an explicit structural relation, not inferred calls.
	for i, f := range related {
		for j := i + 1; j < len(related); j++ {
			g := related[j]
			if f.Path == g.Path {
				p.Relations = append(p.Relations, doccorpus.Relation{ID: identity("relation", f.ID+g.ID), From: f.ID, To: g.ID, Type: "related_to", Provider: p.ID, Evidence: evidence(f.Paragraphs[0].Anchor)})
			}
		}
	}
	// Exact same-file lexical name joins are source dependencies only.
	byName := map[string][]string{}
	for _, f := range related {
		short := f.Name
		if at := strings.LastIndex(short, "#"); at >= 0 {
			short = short[at+1:]
		}
		byName[f.Path+":"+short] = append(byName[f.Path+":"+short], f.ID)
	}
	for _, f := range related {
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
	if m.Schema == Schema && predecessorVersions[m.Version] {
		return nil, &doccorpus.Error{Code: StaleGeneratorCode, Message: "generation manifest was written by flow-docs generator version " + m.Version + "; regenerate it with docs flows generate (generator version " + Version + ") before check or finalize"}
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
	flows, err := m.inventoryInput()
	if err != nil {
		return nil, err
	}
	r, err := Generate(ctx, root, Options{Revision: m.Source.Revision, Scope: m.Scope, Corpus: corpus, Flows: flows})
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
	a, err := doccorpus.Build(ctx, root, m)
	if err != nil {
		return m, err
	}
	return m, doccorpus.AdmitIndex(ctx, a) // DCP-V1-196: refuse an index the reads would refuse
}

// corpusSourceMatches refuses a corpus whose primary repository is not the generation source
// before doccorpus.Open rederives it: Open resolves additional repository roots and primary
// revisions against --root, so a mismatched corpus otherwise fails with a misleading checkout or
// object error. A corpus that declares the generated repository only as an additional repository
// is named as such and refused too (AFU-V1-091).
func corpusSourceMatches(data []byte, source doccorpus.Repository) error {
	a, err := doccorpus.ParseArtifact(data)
	if err != nil {
		return err
	}
	primary := a.Manifest.Repository
	if primary == source {
		return nil
	}
	message := "corpus primary repository is " + primary.ID + " at " + primary.Revision + "; generation repository is " + source.ID + " at " + source.Revision
	for _, r := range a.Manifest.AdditionalRepositories {
		if r.ID == source.ID {
			message += "; the corpus declares the generation repository only as an additional repository at " + r.Revision + ", which generate does not accept"
		}
	}
	return &doccorpus.Error{Code: CorpusRepositoryMismatchCode, Message: message + "; build the corpus with the generation repository and revision as its primary"}
}
