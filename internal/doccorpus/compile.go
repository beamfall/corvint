package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

type compiler struct {
	root         string
	manifest     Manifest
	indexes      map[string]*contextindex.Index
	sources      map[string]contextindex.Source
	artifact     *Artifact
	declarations []CapabilityDeclaration
}

func inputKey(rev, p string) string { return rev + ":" + p }
func inScope(p, s string) bool      { return p == s || strings.HasPrefix(p, s+"/") }

// Inventory creates an explicit manifest candidate; it grants no authority.
func Inventory(ctx context.Context, root, revision, scope, timestamp string) (Manifest, error) {
	if !validPath(scope) {
		return Manifest{}, fail("invalid scope")
	}
	ctx = gitstatus.WithIsolation(ctx)
	index, err := contextindex.BuildRevisionContext(ctx, root, revision)
	if err != nil {
		return Manifest{}, err
	}
	id, err := contextindex.CorpusRepositoryID(ctx, root, revision)
	if err != nil {
		return Manifest{}, err
	}
	m := Manifest{Schema: ManifestSchema, Repository: Repository{id, revision}, BuiltAt: timestamp, Profile: Profile{ID: "native", Revision: "1", Title: "Repository evidence", Format: "markdown", Groups: []string{}}, Scopes: []Scope{{scope, revision}}, Inputs: []Input{}, Providers: []Provider{{ID: "native", Kind: "native", Version: "1", Revision: revision}}, MergeRule: "none"}
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return m, err
	}
	release := auth.BeginObjectSession()
	defer release()
	names := []string{}
	for p := range index.Tracked {
		if inScope(p, scope) {
			names = append(names, p)
		}
	}
	for p := range index.Skipped {
		if inScope(p, scope) {
			return m, fail("scope includes unsupported tree entry")
		}
	}
	sort.Strings(names)
	if len(names) == 0 || len(names) > MaxRecords {
		return m, fail("empty or oversized scope")
	}
	for _, p := range names {
		source, err := loadInput(ctx, auth, index, p)
		if err != nil {
			return m, err
		}
		m.Inputs = append(m.Inputs, Input{Path: p, Revision: revision, Blob: source.BlobHash, SHA256: Digest(source.Data), Provider: "native", Purpose: "source"})
	}
	return m, nil
}

// Build compiles only original immutable inputs. No output is written to disk.
func Build(ctx context.Context, root string, m Manifest) (*Artifact, error) {
	ctx = gitstatus.WithIsolation(ctx)
	if _, err := Encode(m); err != nil {
		return nil, err
	}
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	c := &compiler{root: root, manifest: m, indexes: map[string]*contextindex.Index{}, sources: map[string]contextindex.Source{}}
	if err := c.pin(ctx); err != nil {
		return nil, err
	}
	c.artifact = &Artifact{Schema: Schema, Builder: currentBuilder(), Manifest: m, ManifestSHA256: hashValue(m), ProfileSHA256: hashValue(m.Profile), Tree: c.indexes[m.Repository.Revision].Revision, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []Observation{}, Capabilities: []Capability{}, Gaps: []Gap{}}
	for _, p := range m.Providers {
		if p.Kind == "native" {
			if err := c.native(p); err != nil {
				return nil, err
			}
			if err := c.nativeImports(p); err != nil {
				return nil, err
			}
			if err := c.nativeParagraphs(p); err != nil {
				return nil, err
			}
			continue
		}
		if err := c.importRecords(p); err != nil {
			return nil, err
		}
	}
	if err := c.validateRecords(); err != nil {
		return nil, err
	}
	c.compileBehaviors()
	c.capabilities()
	a := c.artifact
	sort.Slice(a.Subjects, func(i, j int) bool { return a.Subjects[i].ID < a.Subjects[j].ID })
	sort.Slice(a.Claims, func(i, j int) bool { return a.Claims[i].ID < a.Claims[j].ID })
	sort.Slice(a.Relations, func(i, j int) bool { return a.Relations[i].ID < a.Relations[j].ID })
	sort.Slice(a.Journeys, func(i, j int) bool { return a.Journeys[i].ID < a.Journeys[j].ID })
	sort.Slice(a.Observations, func(i, j int) bool { return a.Observations[i].Link.ID < a.Observations[j].Link.ID })
	sort.Slice(a.Gaps, func(i, j int) bool {
		return a.Gaps[i].Subject+"\x00"+a.Gaps[i].Kind+"\x00"+a.Gaps[i].Reason < a.Gaps[j].Subject+"\x00"+a.Gaps[j].Kind+"\x00"+a.Gaps[j].Reason
	})
	a.SHA256 = hashValue(a)
	if _, err := Encode(a); err != nil {
		return nil, err
	}
	// Re-read immutable input identities after compilation; mutable object stores
	// must not let a changing input be reported as a successful pinned build.
	if err := c.pin(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func validateManifest(m Manifest) error {
	if m.Schema != ManifestSchema || !wire.IsGitOid(m.Repository.ID) || !wire.IsGitOid(m.Repository.Revision) {
		return fail("invalid manifest identity")
	}
	if _, err := time.Parse(time.RFC3339, m.BuiltAt); err != nil {
		return fail("build timestamp must be explicit RFC3339")
	}
	if !textOK(m.Profile.ID) || !textOK(m.Profile.Revision) || !textOK(m.Profile.Title) {
		return fail("invalid profile identity")
	}
	if m.Profile.Format != "markdown" && m.Profile.Format != "json" {
		return fail("unsupported render profile")
	}
	if len(m.Profile.Groups) > 32 {
		return fail("profile group bound exceeded")
	}
	if len(m.Inputs) == 0 || len(m.Inputs) > MaxRecords || len(m.Scopes) == 0 || len(m.Scopes) > MaxRecords || len(m.Providers) == 0 || len(m.Providers) > 16 {
		return fail("manifest record bound exceeded")
	}
	if m.MergeRule != "none" && m.MergeRule != "disjoint-union" {
		return fail("unknown merge rule")
	}
	providers := map[string]Provider{}
	for _, p := range m.Providers {
		if !identifier(p.ID) || !textOK(p.Version) || !wire.IsGitOid(p.Revision) {
			return fail("invalid provider identity")
		}
		if _, ok := providers[p.ID]; ok {
			return fail("duplicate provider")
		}
		providers[p.ID] = p
		if p.Kind != "native" && p.Kind != "records" {
			return &Error{Code: "corpus-provider-unavailable", Message: "named provider is unavailable"}
		}
		if p.Kind == "native" && (p.Version != "1" || p.Record != "" || p.Revision != m.Repository.Revision) {
			return fail("native provider revision mismatch")
		}
		if p.Kind == "records" && !validPath(p.Record) {
			return fail("provider record path required")
		}
	}
	revisions := map[string]bool{m.Repository.Revision: true}
	scopes := map[string]bool{}
	for _, s := range m.Scopes {
		if !validPath(s.Path) || !wire.IsGitOid(s.Revision) {
			return fail("invalid scope")
		}
		key := inputKey(s.Revision, s.Path)
		if scopes[key] {
			return fail("duplicate scope")
		}
		scopes[key] = true
		revisions[s.Revision] = true
	}
	inputs := map[string]bool{}
	for _, in := range m.Inputs {
		if !validPath(in.Path) || !wire.IsGitOid(in.Revision) || !wire.IsGitOid(in.Blob) || !wire.IsSha256(in.SHA256) {
			return fail("invalid input binding")
		}
		p, ok := providers[in.Provider]
		if !ok {
			return &Error{Code: "corpus-provider-unavailable", Message: "input provider was not declared"}
		}
		if in.Purpose != "source" && in.Purpose != "provider" && in.Purpose != "observation" && in.Purpose != "evidence" {
			return fail("invalid input purpose")
		}
		if in.Purpose == "provider" && (p.Record != in.Path || p.Revision != in.Revision) {
			return fail("provider artifact revision mismatch")
		}
		if p.Kind == "native" && (in.Revision != m.Repository.Revision || in.Purpose != "source") {
			return fail("native input revision or purpose mismatch")
		}
		key := inputKey(in.Revision, in.Path)
		if inputs[key] && m.MergeRule != "disjoint-union" {
			return fail("multiply claimed input without merge rule")
		}
		inputs[key] = true
		admitted := false
		for _, s := range m.Scopes {
			if s.Revision == in.Revision && inScope(in.Path, s.Path) {
				admitted = true
			}
		}
		if !admitted {
			return fail("input outside declared scopes")
		}
		revisions[in.Revision] = true
	}
	if len(revisions) > 8 {
		return fail("revision bound exceeded")
	}
	return nil
}
func (c *compiler) pin(ctx context.Context) error {
	auth, err := gitauth.Open(c.root, gitrun.NewDefaultBudget())
	if err != nil {
		return err
	}
	release := auth.BeginObjectSession()
	defer release()
	revisions := map[string]bool{c.manifest.Repository.Revision: true}
	for _, s := range c.manifest.Scopes {
		revisions[s.Revision] = true
	}
	indexes := map[string]*contextindex.Index{}
	for rev := range revisions {
		id, err := contextindex.CorpusRepositoryID(ctx, c.root, rev)
		if err != nil {
			return err
		}
		if id != c.manifest.Repository.ID {
			return fail("repository identity mismatch")
		}
		index, err := contextindex.BuildRevisionContext(ctx, c.root, rev)
		if err != nil {
			return err
		}
		indexes[rev] = index
	}
	declared := map[string]bool{}
	for _, in := range c.manifest.Inputs {
		declared[inputKey(in.Revision, in.Path)] = true
	}
	for _, s := range c.manifest.Scopes {
		count := 0
		for p := range indexes[s.Revision].Tracked {
			if inScope(p, s.Path) {
				count++
				if !declared[inputKey(s.Revision, p)] {
					return fail("undeclared input in scope")
				}
			}
		}
		for p := range indexes[s.Revision].Skipped {
			if inScope(p, s.Path) {
				return fail("scope contains unsupported tree entry")
			}
		}
		if count == 0 {
			return fail("declared scope missing")
		}
	}
	for _, in := range c.manifest.Inputs {
		source, err := loadInput(ctx, auth, indexes[in.Revision], in.Path)
		if err != nil {
			return err
		}
		if source.BlobHash != in.Blob || Digest(source.Data) != in.SHA256 {
			return fail("input changed after pinning")
		}
		if in.Purpose == "source" && (generated(source.Data) || contextindex.CorpusGeneratedSource(in.Path, source.Data)) {
			return fail("generated output cannot be source input")
		}
		c.sources[inputKey(in.Revision, in.Path)] = source
	}
	c.indexes = indexes
	return nil
}
func loadInput(ctx context.Context, auth *gitauth.Repository, index *contextindex.Index, p string) (contextindex.Source, error) {
	if source, ok := index.Sources[p]; ok {
		text, valid, loaded := source.Text()
		if loaded && valid {
			source.Data = []byte(text)
			return source, nil
		}
	}
	entry, ok, err := auth.LookupTreeEntry(ctx, index.CommitRevision, p)
	if err != nil {
		return contextindex.Source{}, err
	}
	if !ok {
		return contextindex.Source{}, &Error{Code: "corpus-input-unavailable", Message: "declared input missing"}
	}
	if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return contextindex.Source{}, fail("input is not a regular Git blob")
	}
	data, err := auth.BlobBytes(ctx, entry.OID)
	if err != nil {
		return contextindex.Source{}, err
	}
	if len(data) > MaxBytes {
		return contextindex.Source{}, fail("input bound exceeded")
	}
	return contextindex.Source{Path: p, BlobHash: entry.OID, Data: data, Mode: entry.Mode}, nil
}
func generated(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if bytes.Contains(trimmed, []byte("<!-- corvint-corpus")) {
		return true
	}
	var object map[string]any
	if len(trimmed) > 0 && trimmed[0] == '{' && json.Unmarshal(trimmed, &object) == nil {
		schema, _ := object["schema"].(string)
		if strings.HasPrefix(schema, "corvint-corpus-") {
			return true
		}
		if schema == Schema {
			return true
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "// Code generated ") || strings.HasPrefix(l, "# Code generated ") {
			return true
		}
	}
	return false
}
func identifier(id string) bool {
	return len(id) > 0 && len(id) <= 1024 && !strings.ContainsAny(id, "\x00\r\n\t")
}

func (c *compiler) native(provider Provider) error {
	index := c.indexes[c.manifest.Repository.Revision]
	for _, in := range c.manifest.Inputs {
		if len(c.artifact.Subjects) > MaxRecords || len(c.artifact.Claims) > MaxRecords {
			return fail("native record bound exceeded")
		}
		if in.Provider != provider.ID {
			continue
		}
		if reason := c.nativeExclusion(in); reason != "" {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{provider.ID + ":file:" + in.Path, "not-collected", reason})
			continue
		}
		source := c.sources[inputKey(in.Revision, in.Path)]
		text, valid, loaded := source.Text()
		id := provider.ID + ":file:" + in.Path
		if !valid || !loaded {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "not-collected", "input is not supported UTF-8 text"})
			continue
		}
		kind := "document"
		if path.Ext(in.Path) != ".md" {
			kind = "module"
		}
		line := firstContentLine(text)
		if line == 0 {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "not-collected", "empty source has no non-vacuous anchor"})
			continue
		}
		anchor, err := c.anchor(in.Revision, in.Path, line, line, "syntax", "source", "explicit input inventory")
		if err != nil {
			return err
		}
		ev := Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{"file presence and exact excerpt only; behavior unknown"}}
		c.artifact.Subjects = append(c.artifact.Subjects, Subject{id, kind, in.Path, provider.ID, ev})
		c.artifact.Claims = append(c.artifact.Claims, Claim{id + ":excerpt", id, lineText(text, line), provider.ID, ev})
		if !contextindex.ImpactPathAdmitted(in.Path) {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "unsupported", "native analyzer unavailable; text only"})
		}
		for _, u := range index.Unparsed {
			if u.Path == in.Path {
				c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "not-collected", u.Reason})
			}
		}
		for _, s := range index.Symbols {
			if len(c.artifact.Subjects) >= MaxRecords || len(c.artifact.Claims) >= MaxRecords {
				return fail("native record bound exceeded")
			}
			if s.Path != in.Path {
				continue
			}
			if _, bad := index.UnparsedPaths()[in.Path]; bad {
				continue
			}
			a, err := c.anchor(in.Revision, in.Path, s.Line, s.Line, "syntax", "source", "native indexed declaration")
			if err != nil {
				return err
			}
			a.Symbol = s.Name
			se := ev
			se.Anchors = []Anchor{a}
			se.Limitations = []string{"declaration only; build applicability and behavioral coverage unknown"}
			sk := "symbol"
			if isTestPath(in.Path) {
				sk = "test"
			}
			sid := fmt.Sprintf("%s:symbol:%s:%d:%s", provider.ID, in.Path, s.Line, s.Name)
			c.artifact.Subjects = append(c.artifact.Subjects, Subject{sid, sk, s.Name, provider.ID, se})
			c.artifact.Claims = append(c.artifact.Claims, Claim{sid + ":declaration", sid, lineText(text, s.Line), provider.ID, se})
		}
	}
	return nil
}
func firstContentLine(text string) int {
	for n, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return n + 1
		}
	}
	return 0
}
func lineText(text string, n int) string {
	lines := strings.Split(text, "\n")
	if n < 1 || n > len(lines) {
		return ""
	}
	return lines[n-1]
}
func isTestPath(p string) bool {
	return strings.HasSuffix(p, "_test.go") || strings.Contains(p, ".test.") || strings.Contains(p, ".spec.") || strings.HasPrefix(path.Base(p), "test_") || strings.HasSuffix(p, "_test.py") || strings.HasSuffix(p, "_spec.rb")
}
func span(data []byte, start, end int) ([]byte, error) {
	if start < 1 || end < start {
		return nil, fail("invalid anchor span")
	}
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if end > len(lines) {
		return nil, fail("anchor span out of range")
	}
	s := bytes.Join(lines[start-1:end], nil)
	if len(s) > 64<<10 || len(bytes.TrimSpace(s)) == 0 {
		return nil, fail("vacuous or oversized anchor")
	}
	return s, nil
}
func (c *compiler) anchor(revision, p string, start, end int, authority, kind, reason string) (Anchor, error) {
	source, ok := c.sources[inputKey(revision, p)]
	if !ok {
		return Anchor{}, fail("anchor input was not declared")
	}
	data, err := span(source.Data, start, end)
	if err != nil {
		return Anchor{}, err
	}
	return Anchor{Repository: c.manifest.Repository.ID, Revision: revision, Path: p, Blob: source.BlobHash, SHA256: Digest(source.Data), Start: start, End: end, SpanSHA256: Digest(data), Authority: authority, Kind: kind, Reason: reason}, nil
}

// Open reconstructs all derived records before allowing a query.
func Open(ctx context.Context, root string, data []byte) (*Artifact, error) {
	a, err := ParseArtifact(data)
	if err != nil {
		return nil, err
	}
	rebuilt, err := Build(ctx, root, a.Manifest)
	if err != nil {
		return nil, err
	}
	canonical, err := Encode(rebuilt)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fail("artifact cannot be rederived from current builder and pinned inputs")
	}
	return rebuilt, nil
}

func (c *compiler) nativeImports(provider Provider) error {
	index := c.indexes[c.manifest.Repository.Revision]
	available := map[string]bool{}
	for _, s := range c.artifact.Subjects {
		available[s.ID] = true
	}
	packages := map[string]bool{}
	for _, in := range c.manifest.Inputs {
		if in.Provider != provider.ID || !strings.HasSuffix(in.Path, ".go") {
			continue
		}
		from := provider.ID + ":file:" + in.Path
		if !available[from] {
			continue
		}
		imports := []string{}
		for imported := range index.Imports[in.Path] {
			imports = append(imports, imported)
		}
		sort.Strings(imports)
		for _, imported := range imports {
			line, ok := contextindex.ImportAnchorLine(index.Sources[in.Path], imported)
			if !ok {
				c.artifact.Gaps = append(c.artifact.Gaps, Gap{from, "not-collected", "import statement anchor unavailable"})
				continue
			}
			anchor, err := c.anchor(in.Revision, in.Path, line, line, "syntax", "source", "native Go import declaration")
			if err != nil {
				return err
			}
			ev := Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{"syntactic import only; runtime applicability and dependency contents unknown"}}
			to := provider.ID + ":package:" + imported
			if !packages[to] {
				c.artifact.Subjects = append(c.artifact.Subjects, Subject{to, "package", imported, provider.ID, ev})
				packages[to] = true
			}
			c.artifact.Relations = append(c.artifact.Relations, Relation{provider.ID + ":imports:" + in.Path + ":" + imported, from, to, "imports", provider.ID, ev})
			if len(c.artifact.Subjects) > MaxRecords || len(c.artifact.Relations) > MaxRecords {
				return fail("native relation bound exceeded")
			}
		}
	}
	return nil
}
func (c *compiler) nativeParagraphs(provider Provider) error {
	for _, in := range c.manifest.Inputs {
		if in.Provider != provider.ID || !strings.HasSuffix(in.Path, ".md") {
			continue
		}
		if c.nativeExclusion(in) != "" {
			continue
		}
		source := c.sources[inputKey(in.Revision, in.Path)]
		text, valid, loaded := source.Text()
		if !valid || !loaded {
			continue
		}
		id := provider.ID + ":file:" + in.Path
		lines := strings.SplitAfter(text, "\n")
		start := 0
		for i := 0; i <= len(lines); i++ {
			if i < len(lines) && strings.TrimSpace(lines[i]) != "" {
				if start == 0 {
					start = i + 1
				}
				continue
			}
			if start == 0 {
				continue
			}
			end := i
			if end >= start {
				anchor, err := c.anchor(in.Revision, in.Path, start, end, "source-document", "source", "verbatim source paragraph")
				if err != nil {
					return err
				}
				excerpt := strings.Join(lines[start-1:end], "")
				ev := Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{"verbatim source excerpt; quoted prose is not accepted intent or verified behavior"}}
				c.artifact.Claims = append(c.artifact.Claims, Claim{fmt.Sprintf("%s:paragraph:%d", id, start), id, excerpt, provider.ID, ev})
				if len(c.artifact.Claims) > MaxRecords {
					return fail("native paragraph bound exceeded")
				}
			}
			start = 0
		}
	}
	return nil
}

func (c *compiler) nativeExclusion(in Input) string {
	if reason := contextindex.ForbiddenPathReason(in.Path); reason != "" {
		return reason
	}
	for _, exclusion := range c.indexes[in.Revision].Exclusions {
		if exclusion.Path == in.Path {
			return exclusion.Reason
		}
	}
	return ""
}
