package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/rootalias"
)

type compiler struct {
	// derivation bounds the declarations one compile derives, derived counts the declarations
	// it attempted to derive and outputBytes is the encoded size of a compile the output byte
	// bound refused (DCS-V1-484, DCS-V1-568, DCS-V1-569).
	derivation        derivationPlan
	derived           int
	outputBytes       int64
	providerRecords   map[string]ProviderRecord
	restrictedIDs     map[string]bool
	restrictedBytes   int
	adoptionProviders map[string]bool
	restrictedTokens  []string
	root              string
	manifest          Manifest
	indexes           map[string]*contextindex.Index
	sources           map[string]contextindex.Source
	anchorLines       map[string]*anchorLines
	pinned            *pinnedInputs
	artifact          *Artifact
	declarations      []CapabilityDeclaration
	testKnowledge     []providerKnowledge
	flowRelations     []providerFlowRelations
	claimAudits       []providerClaimAudits
}

func inputKey(rev, p string) string { return rev + ":" + p }

// inScope reports whether path p is scope s or lies beneath it, without building s+"/"
// (DCR-V1-450).
func inScope(p, s string) bool {
	return len(p) >= len(s) && p[:len(s)] == s && (len(p) == len(s) || p[len(s)] == '/')
}

// scopeSet holds a manifest's scope paths by revision. The scopes holding a path are its
// ancestors, so they are found by walking the path's prefixes instead of comparing the path with
// every scope; the walk visits exactly the scopes inScope admits, so every caller's result is the
// one a scope-by-path comparison gave (DCR-V1-450).
type scopeSet map[string]map[string]struct{}

// newScopeSet collects the scopes whose revision member admits; a nil member admits all.
func newScopeSet(scopes []Scope, member func(string) bool) scopeSet {
	set := scopeSet{}
	for _, scope := range scopes {
		if member != nil && !member(scope.Revision) {
			continue
		}
		if set[scope.Revision] == nil {
			set[scope.Revision] = map[string]struct{}{}
		}
		set[scope.Revision][scope.Path] = struct{}{}
	}
	return set
}

// each calls visit once for every scope of revision that holds p, deepest first.
func (set scopeSet) each(revision, p string, visit func(scope string)) {
	paths := set[revision]
	if len(paths) == 0 {
		return
	}
	for candidate := p; ; {
		if _, ok := paths[candidate]; ok {
			visit(candidate)
		}
		i := strings.LastIndexByte(candidate, '/')
		if i < 0 {
			return
		}
		candidate = candidate[:i]
	}
}

// holds reports whether any scope of revision holds p.
func (set scopeSet) holds(revision, p string) bool {
	paths := set[revision]
	if len(paths) == 0 {
		return false
	}
	for candidate := p; ; {
		if _, ok := paths[candidate]; ok {
			return true
		}
		i := strings.LastIndexByte(candidate, '/')
		if i < 0 {
			return false
		}
		candidate = candidate[:i]
	}
}

// Inventory creates an explicit corvint-corpus-input/1 manifest candidate for one scope; it grants
// no authority.
func Inventory(ctx context.Context, root, revision, scope, timestamp string) (Manifest, error) {
	return InventoryScopes(ctx, root, revision, []string{scope}, ManifestSchema, timestamp)
}

// InventoryScopes creates one manifest candidate of the given schema over every scope (DCP-V1-160..163).
// Scopes are emitted in byte order; a duplicate or nested scope is refused, and the inventory is bounded
// by the schema's record limit.
// It pins generated output like any other input: a caller such as flowdocs inventories a provider
// record and then changes its purpose. `docs corpus manifest` uses ManifestInventory instead.
func InventoryScopes(ctx context.Context, root, revision string, scopes []string, schema, timestamp string) (Manifest, error) {
	m, _, err := inventoryScopes(ctx, root, revision, scopes, schema, timestamp, "primary")
	return m, err
}

// ManifestInventory is InventoryScopes for `docs corpus manifest`: a scope holding generated
// output is refused naming every such input, since every input is pinned as a source and so the
// manifest could never build (DCP-V1-608). The pinned manifest is still returned with the refusal.
func ManifestInventory(ctx context.Context, root, revision string, scopes []string, schema, timestamp string) (Manifest, error) {
	m, refused, err := inventoryScopes(ctx, root, revision, scopes, schema, timestamp, "primary")
	if err == nil && len(refused) > 0 {
		err = generatedRefusal(refused)
	}
	return m, err
}

// inventoryScopes is InventoryScopes returning, rather than refusing, the generated inputs it
// pinned, each labelled with label: "primary", or a workspace alias (DCP-V1-608).
func inventoryScopes(ctx context.Context, root, revision string, scopes []string, schema, timestamp, label string) (_ Manifest, refused []generatedInput, err error) {
	defer func() { err = gitCapacity(err, "while inventorying the scope") }()
	scopes, err = inventoryScopeSet(scopes, schema)
	if err != nil {
		return Manifest{}, nil, err
	}
	ctx = gitstatus.WithIsolation(ctx)
	// A further repository's unusable checkout, such as a shallow clone, is refused naming its
	// alias, so the operator can tell which checkout of the workspace to repair (DCI-V1-004).
	named := func(err error) error {
		if label == "primary" {
			return err
		}
		return namedRefusal("repository "+label, err)
	}
	index, err := contextindex.BuildRevisionContext(ctx, root, revision)
	if err != nil {
		return Manifest{}, nil, named(err)
	}
	id, err := contextindex.CorpusRepositoryID(ctx, root, revision)
	if err != nil {
		return Manifest{}, nil, named(err)
	}
	m := Manifest{Schema: schema, Repository: Repository{id, revision}, BuiltAt: timestamp, Profile: Profile{ID: "native", Revision: "1", Title: "Repository evidence", Format: "markdown", Groups: []string{}}, Scopes: []Scope{}, Inputs: []Input{}, Providers: []Provider{{ID: "native", Kind: "native", Version: "1", Revision: revision}}, MergeRule: "none"}
	for _, scope := range scopes {
		m.Scopes = append(m.Scopes, Scope{scope, revision})
	}
	auth, err := gitauth.Open(root, newGitBudget(ctx))
	if err != nil {
		return m, nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	names := []string{}
	counts := map[string]int{}
	for _, scope := range scopes {
		counts[scope] = 0
	}
	for p := range index.Tracked {
		if scope, ok := owningScope(p, counts); ok {
			counts[scope]++
			names = append(names, p)
		}
	}
	for p := range index.Skipped {
		if _, ok := owningScope(p, counts); ok {
			return m, nil, fail("scope includes unsupported tree entry")
		}
	}
	sort.Strings(names)
	if err := checkInventoryBound(m, scopes, counts, len(names)); err != nil {
		return m, nil, err
	}
	loader, err := stageInputs(ctx, auth, map[string]*contextindex.Index{revision: index}, map[string][]string{revision: names}, MaxBytes)
	if err != nil {
		return m, nil, err
	}
	// An over-bound input is never read: it is pinned by blob and size as an over-bound input, which
	// the build reports as a gap, so one file no longer refuses its whole scope (DCI-V1-024). The
	// body read covers only the rest.
	within := names[:0:0]
	for _, p := range names {
		if blob, size, over := loader.oversize(revision, p); over {
			m.OverBoundInputs = append(m.OverBoundInputs, OverBoundInput{Path: p, Revision: revision, Blob: blob, Bytes: size, Provider: "native"})
			continue
		}
		within = append(within, p)
	}
	names = within
	for _, p := range names {
		if _, _, err := loader.admit(revision, p); err != nil {
			return m, nil, err
		}
	}
	if err := loader.read(ctx); err != nil {
		return m, nil, err
	}
	for _, p := range names {
		source, err := loader.source(revision, p)
		if err != nil {
			return m, nil, err
		}
		if check := generatedCheck(p, source.Data); check != "" {
			refused = append(refused, generatedInput{label, p, revision, check})
		}
		m.Inputs = append(m.Inputs, Input{Path: p, Revision: revision, Blob: source.BlobHash, SHA256: Digest(source.Data), Provider: "native", Purpose: "source"})
	}
	// A pinned generated input could never build, and dropping it would leave it undeclared in
	// its scope, so the caller refuses naming every one (DCP-V1-608).
	return m, refused, nil
}

// inventoryScopeSet sorts the requested scopes and refuses an unknown schema, an invalid path, or a
// duplicate or nested scope: every input then belongs to exactly one scope (DCP-V1-161).
func inventoryScopeSet(requested []string, schema string) ([]string, error) {
	if schema != ManifestSchema && schema != ManifestSchemaV2 {
		return nil, fail("unsupported manifest schema")
	}
	if len(requested) == 0 {
		return nil, fail("invalid scope")
	}
	scopes := append([]string(nil), requested...)
	sort.Strings(scopes)
	for i, scope := range scopes {
		if !validPath(scope) {
			return nil, fail("invalid scope")
		}
		if i > 0 && scopes[i-1] == scope {
			return nil, fail("duplicate scope " + scope)
		}
	}
	// The refusal names the byte-least containing scope and, under it, the byte-least contained
	// one, as comparing every pair in sorted order did (DCR-V1-450).
	set := map[string]struct{}{}
	for _, scope := range scopes {
		set[scope] = struct{}{}
	}
	outer, inner := "", ""
	for _, scope := range scopes {
		for i := strings.LastIndexByte(scope, '/'); i >= 0; i = strings.LastIndexByte(scope[:i], '/') {
			if _, ok := set[scope[:i]]; ok && (inner == "" || scope[:i] < outer) {
				outer, inner = scope[:i], scope
			}
		}
	}
	if inner != "" {
		return nil, fail("overlapping scopes: " + outer + " contains " + inner)
	}
	return scopes, nil
}

// owningScope names the requested scope holding p by walking p's ancestors; scopes holds one key
// per requested scope.
func owningScope(p string, scopes map[string]int) (string, bool) {
	for candidate := p; ; {
		if _, ok := scopes[candidate]; ok {
			return candidate, true
		}
		i := strings.LastIndexByte(candidate, '/')
		if i < 0 {
			return "", false
		}
		candidate = candidate[:i]
	}
}

// checkInventoryBound refuses an empty scope or an inventory over the manifest schema's record limit,
// naming the bound, the scopes and the file count (DCP-V1-162).
func checkInventoryBound(m Manifest, scopes []string, counts map[string]int, total int) error {
	for _, scope := range scopes {
		if counts[scope] == 0 {
			return fail("empty scope: " + scope + " holds no tracked files")
		}
	}
	if limit := m.recordLimit(); total > limit {
		message := fmt.Sprintf("oversized scope: %s holds %d tracked files, over the %s bound of %d records", strings.Join(scopes, ", "), total, m.Schema, limit)
		if m.Schema == ManifestSchema {
			message += fmt.Sprintf("; --schema 2 admits %d", MaxCorpusRecords)
		}
		return fail(message)
	}
	return nil
}

// Build compiles only original immutable inputs. No output is written to disk.
func Build(ctx context.Context, root string, m Manifest) (*Artifact, error) {
	return build(ctx, root, m, nil)
}

// build compiles m and, when the compile derived declarations (DCS-V1-482) and was then refused
// by a corpus bound or check, compiles it again with fewer, so derivation never refuses a
// manifest the compiler built before it (DCS-V1-484). A refusal by the output byte bound is
// retried with the prefix of the declarations that fits under it (DCS-V1-569); any other
// refusal, or a second one, is retried without derived declarations. The record bound needs no
// retry: the first compile derives only what it admits (DCS-V1-568). So a build compiles at
// most three times. Each retry restores the incremental reuse state the refused compile
// recorded, and reads the inputs that compile pinned instead of pinning them again; the
// retry's closing pin still re-reads every input (DCS-V1-572).
func build(ctx context.Context, root string, m Manifest, reuse *incrementalBuild) (*Artifact, error) {
	var stats IncrementalStats
	shards := 0
	if reuse != nil {
		stats = IncrementalStats{slices.Clone(reuse.stats.Compiled), slices.Clone(reuse.stats.Reused), slices.Clone(reuse.stats.Fallback), reuse.stats.FallbackReason}
		shards = len(reuse.cache.Shards)
	}
	var plan derivationPlan
	var pinned *pinnedInputs
	for {
		a, result, err := compileOnce(ctx, root, m, reuse, plan, pinned)
		if plan.none || !retryWithoutDerived(result.derived, err) {
			return a, err
		}
		if reuse != nil {
			*reuse.stats = IncrementalStats{slices.Clone(stats.Compiled), slices.Clone(stats.Reused), slices.Clone(stats.Fallback), stats.FallbackReason}
			reuse.cache.Shards = reuse.cache.Shards[:shards]
		}
		plan, pinned = retryPlan(plan, result, err), result.pinned
	}
}

// derivationPlan bounds the declarations one compile derives (DCS-V1-484, DCS-V1-568,
// DCS-V1-569). The zero plan derives every declaration the record bound admits. The retry of
// a compile the output byte bound refused carries that compile's encoded size, and derives the
// prefix whose estimated bytes fit under the bound; the retry of any other refusal derives none.
type derivationPlan struct {
	none        bool
	refusal     string
	attempted   int
	outputBytes int64
}

// compileResult is what a refused compile tells its retry: the declarations it attempted to
// derive, when the output byte bound refused it, its encoded size and, when the refusal came
// after its opening pin and before its closing one, the inputs it pinned.
type compileResult struct {
	derived     int
	outputBytes int64
	pinned      *pinnedInputs
}

func (c *compiler) result() compileResult { return compileResult{c.derived, c.outputBytes, c.pinned} }

// pinnedInputs is what a compile's opening pin read: the index of each revision and every
// declared input's bytes, verified against the manifest's blob and digest, with the anchor line
// tables built from them. The compile reads no other repository state, so a retry that starts
// from them compiles what a retry that pinned again would (DCS-V1-572).
type pinnedInputs struct {
	indexes     map[string]*contextindex.Index
	sources     map[string]contextindex.Source
	anchorLines map[string]*anchorLines
}

// retryPlan is the derivation of the compile after a refused one: the byte-bounded prefix
// after a first refusal by the output byte bound, and no derived declarations otherwise.
func retryPlan(plan derivationPlan, result compileResult, err error) derivationPlan {
	if plan.outputBytes == 0 && result.outputBytes > 0 {
		return derivationPlan{attempted: result.derived, outputBytes: result.outputBytes}
	}
	return derivationPlan{none: true, refusal: err.Error(), attempted: result.derived}
}

// retryWithoutDerived reports whether a compile that attempted derived declarations failed
// with a corpus refusal (a bound or check), the only failure a retry with fewer declarations
// answers; a Git, capacity or input-availability error is returned as it is (DCS-V1-484).
func retryWithoutDerived(derived int, err error) bool {
	var refused *Error
	return err != nil && derived > 0 && errors.As(err, &refused) && refused.Code == "corpus-refused"
}

func compileOnce(ctx context.Context, root string, m Manifest, reuse *incrementalBuild, plan derivationPlan, pinned *pinnedInputs) (*Artifact, compileResult, error) {
	builds.Add(1)
	ctx = gitstatus.WithIsolation(ctx)
	if _, err := Encode(m); err != nil {
		return nil, compileResult{}, err
	}
	if err := validateManifest(m); err != nil {
		return nil, compileResult{}, err
	}
	c := &compiler{root: root, manifest: m, derivation: plan, indexes: map[string]*contextindex.Index{}, sources: map[string]contextindex.Source{}, anchorLines: map[string]*anchorLines{}}
	if pinned != nil {
		c.indexes, c.sources, c.anchorLines = pinned.indexes, maps.Clone(pinned.sources), pinned.anchorLines
	} else if err := c.pin(ctx); err != nil {
		return nil, c.result(), err
	}
	c.pinned = &pinnedInputs{c.indexes, maps.Clone(c.sources), c.anchorLines}
	manifestSHA256, err := hashValue(m)
	if err != nil {
		return nil, c.result(), err
	}
	profileSHA256, err := hashValue(m.Profile)
	if err != nil {
		return nil, c.result(), err
	}
	c.artifact = &Artifact{Schema: Schema, Builder: currentBuilder(), Manifest: m, ManifestSHA256: manifestSHA256, ProfileSHA256: profileSHA256, Tree: c.indexes[m.Repository.Revision].Revision, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []Observation{}, Capabilities: []Capability{}, Gaps: []Gap{}}
	if m.Schema == ManifestSchemaV2 {
		c.artifact.Schema = SchemaV2
	}
	if err := c.preloadProviders(); err != nil {
		return nil, c.result(), err
	}
	for _, p := range m.Providers {
		if p.Kind == "native" {
			if err := c.native(p); err != nil {
				return nil, c.result(), err
			}
			if err := c.nativeImports(p); err != nil {
				return nil, c.result(), err
			}
			if err := c.nativeParagraphs(p); err != nil {
				return nil, c.result(), err
			}
			continue
		}
		if err := c.importRecordsIncremental(p, reuse); err != nil {
			return nil, c.result(), err
		}
	}
	if err := c.derivedDeclarations(); err != nil {
		return nil, c.result(), err
	}
	if err := c.repositoryInventory(); err != nil {
		return nil, c.result(), err
	}
	if err := c.validateRecords(); err != nil {
		return nil, c.result(), err
	}
	c.compileBehaviors()
	c.capabilities()
	c.aggregateImportParity()
	if err := c.validateAdoptionJoins(); err != nil {
		return nil, c.result(), err
	}
	if err := c.joinTestKnowledge(c.testKnowledge); err != nil {
		return nil, c.result(), err
	}
	if err := c.joinFlowRelations(c.flowRelations); err != nil {
		return nil, c.result(), err
	}
	if err := c.joinClaimAudits(c.claimAudits); err != nil {
		return nil, c.result(), err
	}
	if err := c.checkClaimMentions(); err != nil {
		return nil, c.result(), err
	}
	if err := validateAlternateIDsAfterJoins(c.artifact); err != nil {
		return nil, c.result(), err
	}
	if m.Schema == ManifestSchemaV2 {
		if err := validateReadableRecords(c.artifact); err != nil {
			return nil, c.result(), err
		}
	}
	if err := c.guardRestrictedOutput(); err != nil {
		return nil, c.result(), err
	}
	a := c.artifact
	sort.Slice(a.Subjects, func(i, j int) bool { return a.Subjects[i].ID < a.Subjects[j].ID })
	sort.Slice(a.Claims, func(i, j int) bool { return a.Claims[i].ID < a.Claims[j].ID })
	sort.Slice(a.Relations, func(i, j int) bool { return a.Relations[i].ID < a.Relations[j].ID })
	sort.Slice(a.Journeys, func(i, j int) bool { return a.Journeys[i].ID < a.Journeys[j].ID })
	sort.Slice(a.Observations, func(i, j int) bool { return a.Observations[i].Link.ID < a.Observations[j].Link.ID })
	sort.Slice(a.SuppliedGaps, func(i, j int) bool { return a.SuppliedGaps[i].ID < a.SuppliedGaps[j].ID })
	sort.Slice(a.StabilityEvidence, func(i, j int) bool { return a.StabilityEvidence[i].ID < a.StabilityEvidence[j].ID })
	sort.Slice(a.Gaps, func(i, j int) bool {
		return a.Gaps[i].Subject+"\x00"+a.Gaps[i].Kind+"\x00"+a.Gaps[i].Reason < a.Gaps[j].Subject+"\x00"+a.Gaps[j].Kind+"\x00"+a.Gaps[j].Reason
	})
	digest, size, err := hashSized(a)
	if err != nil {
		// size is set only when the output byte bound refused the artifact (DCS-V1-569).
		c.outputBytes = size
		return nil, c.result(), err
	}
	a.SHA256 = digest
	if _, err := Encode(a); err != nil {
		// The digest field adds at most its own bytes and its member syntax.
		c.outputBytes = size + int64(len(digest)) + 16
		return nil, c.result(), err
	}
	// Re-read immutable input identities after compilation; mutable object stores
	// must not let a changing input be reported as a successful pinned build. A refusal
	// here hands its retry nothing pinned.
	c.pinned = nil
	if err := c.pin(ctx); err != nil {
		return nil, c.result(), err
	}
	return a, c.result(), nil
}

func validateManifest(m Manifest) error {
	if m.BehaviorRepositories != nil && !validBehaviorRepositories(m.BehaviorRepositories) {
		return fail("invalid expected behavior repository set")
	}
	if (m.Schema != ManifestSchema && m.Schema != ManifestSchemaV2) || !wire.IsGitOid(m.Repository.ID) || !wire.IsGitOid(m.Repository.Revision) {
		return fail("invalid manifest identity")
	}
	if err := validateAdditionalRepositories(m); err != nil {
		return err
	}
	if err := validateRepositoryInventory(m); err != nil {
		return err
	}
	if err := validateProductionScope(m); err != nil {
		return err
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
	// An explicit empty list is refused: encoding would drop it (DCI-V1-230).
	if m.OverBoundInputs != nil && len(m.OverBoundInputs) == 0 {
		return fail("invalid over-bound input list: an explicit list is not empty")
	}
	// A scope may hold only over-bound inputs: it builds with their gaps (DCI-V1-024).
	if len(m.Inputs)+len(m.OverBoundInputs) == 0 || len(m.Scopes) == 0 || len(m.Providers) == 0 {
		return fail("manifest record bound exceeded")
	}
	if len(m.Inputs) > m.recordLimit() {
		return capacityExceeded("manifest input count", len(m.Inputs), m.recordLimit())
	}
	if len(m.Scopes) > m.recordLimit() {
		return capacityExceeded("manifest scope count", len(m.Scopes), m.recordLimit())
	}
	if len(m.Providers) > 16 {
		return capacityExceeded("manifest provider count", len(m.Providers), 16)
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
		if p.Kind == "native" && (p.Version != "1" || p.Record != "" || p.Shards != nil || p.Revision != m.Repository.Revision) {
			return fail("native provider revision mismatch")
		}
		if p.Derive != nil {
			if err := validDerivationFilter(m, p); err != nil {
				return err
			}
		}
		if p.Kind == "records" {
			if p.Shards != nil {
				if m.Schema != ManifestSchemaV2 || p.Record != "" || len(p.Shards) == 0 || len(p.Shards) > 128 || !sort.StringsAreSorted(p.Shards) {
					return fail("invalid provider shard inventory")
				}
				for i, path := range p.Shards {
					if !validPath(path) || i > 0 && path == p.Shards[i-1] {
						return fail("invalid or duplicate provider shard path")
					}
				}
			} else if !validPath(p.Record) {
				return fail("provider record path required")
			}
		}
	}
	revisions := map[string]bool{m.Repository.Revision: true}
	for _, r := range m.AdditionalRepositories {
		for _, revision := range r.pinned() {
			revisions[revision] = true
		}
	}
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
	scoped := newScopeSet(m.Scopes, nil)
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
		if rule := validInputAsOf(in); rule != "" {
			return fail("invalid input as_of: " + rule)
		}
		if in.Purpose == "provider" && (!providerHasPath(p, in.Path) || p.Revision != in.Revision) {
			return fail("provider artifact revision mismatch")
		}
		if p.Kind == "native" && (!m.nativeRevision(in.Revision) || in.Purpose != "source") {
			return fail("native input revision or purpose mismatch")
		}
		key := inputKey(in.Revision, in.Path)
		if inputs[key] && m.MergeRule != "disjoint-union" {
			return fail("multiply claimed input without merge rule")
		}
		inputs[key] = true
		if !scoped.holds(in.Revision, in.Path) {
			return fail("input outside declared scopes")
		}
		revisions[in.Revision] = true
	}
	// An over-bound input is a native scope file over the per-input bound that no input also
	// declares, listed once (DCI-V1-024).
	if len(m.OverBoundInputs) > m.recordLimit() {
		return capacityExceeded("manifest over-bound input count", len(m.OverBoundInputs), m.recordLimit())
	}
	for _, in := range m.OverBoundInputs {
		key := inputKey(in.Revision, in.Path)
		if !validPath(in.Path) || !wire.IsGitOid(in.Revision) || !wire.IsGitOid(in.Blob) || in.Bytes <= MaxBytes || providers[in.Provider].Kind != "native" || !m.nativeRevision(in.Revision) || !scoped.holds(in.Revision, in.Path) || inputs[key] {
			return fail("invalid over-bound input: " + in.Path)
		}
		inputs[key] = true
	}
	if len(revisions) > 8 {
		return fail("revision bound exceeded")
	}
	return nil
}

// validateAdditionalRepositories admits a further repository only by explicit root-commit
// identity, one pinned revision and a clean relative checkout locator, in identity order, distinct
// from the primary repository and from each other (DCP-V1-175).
func validateAdditionalRepositories(m Manifest) error {
	// An explicit empty list is refused: encoding would drop it (DCI-V1-230).
	if m.AdditionalRepositories != nil && len(m.AdditionalRepositories) == 0 {
		return fail("invalid additional repository declaration: an explicit list is not empty")
	}
	if len(m.AdditionalRepositories) > maxAdditionalRepositories {
		return capacityExceeded("additional repository count", len(m.AdditionalRepositories), maxAdditionalRepositories)
	}
	revisions, roots, aliases := map[string]bool{m.Repository.Revision: true}, map[string]bool{}, map[string]bool{}
	for i, r := range m.AdditionalRepositories {
		// DCP-V1-600: an alias is a valid workspace alias, distinct across repositories.
		if r.Alias != "" && (!rootalias.Valid(r.Alias) || aliases[r.Alias]) {
			return fail("invalid additional repository declaration")
		}
		aliases[r.Alias] = r.Alias != ""
		if !wire.IsGitOid(r.ID) || !wire.IsGitOid(r.Revision) || r.ID == m.Repository.ID || revisions[r.Revision] || roots[r.Root] || i > 0 && r.ID <= m.AdditionalRepositories[i-1].ID || !validRepositoryRoot(r.Root) {
			return fail("invalid additional repository declaration")
		}
		revisions[r.Revision], roots[r.Root] = true, true
		// DCP-V1-308: further pinned revisions of the same repository, sorted and distinct from
		// every other repository's revisions.
		if r.Revisions != nil && len(r.Revisions) == 0 {
			return fail("invalid additional repository declaration")
		}
		for j, revision := range r.Revisions {
			if !wire.IsGitOid(revision) || revisions[revision] || j > 0 && revision <= r.Revisions[j-1] {
				return fail("invalid additional repository declaration")
			}
			revisions[revision] = true
		}
	}
	if len(revisions) > maxPinnedRevisions {
		return capacityExceeded("pinned repository revision count", len(revisions), maxPinnedRevisions)
	}
	return nil
}

// maxPinnedRevisions is the eight-revision bound every pinned repository revision counts toward.
const maxPinnedRevisions = 8

// pinned lists the repository's declared current revision followed by its further pinned ones.
func (r AdditionalRepository) pinned() []string {
	return append([]string{r.Revision}, r.Revisions...)
}

// maxAdditionalRepositories keeps every repository's pinned revision inside the eight-revision bound.
const maxAdditionalRepositories = 7

// validRepositoryRoot admits a clean relative checkout path other than the primary root itself; it
// may climb out of the primary root, because a sibling checkout is the common layout.
func validRepositoryRoot(p string) bool {
	return p != "" && p != "." && len(p) <= 1024 && utf8.ValidString(p) && !strings.ContainsAny(p, "\\\x00\r\n\t") && !path.IsAbs(p) && path.Clean(p) == p
}

// additionalRepository returns the declared additional repository pinned at revision, whether
// it is the repository's current revision or a further pinned one (DCP-V1-308).
func (m Manifest) additionalRepository(revision string) (AdditionalRepository, bool) {
	for _, r := range m.AdditionalRepositories {
		if slices.Contains(r.pinned(), revision) {
			return r, true
		}
	}
	return AdditionalRepository{}, false
}

// revisionRepository names the repository identity and checkout directory holding revision: an
// additional repository's pinned revision resolves to its own checkout (DCP-V1-176); every other
// revision belongs to the primary repository at root.
func (m Manifest) revisionRepository(root, revision string) (string, string) {
	if r, ok := m.additionalRepository(revision); ok {
		return r.ID, filepath.Join(root, filepath.FromSlash(r.Root))
	}
	return m.Repository.ID, root
}

// nativeRevision reports whether native inputs may be pinned at revision: the primary revision, or
// the declared revision of an aliased additional repository (DCP-V1-600).
func (m Manifest) nativeRevision(revision string) bool {
	if revision == m.Repository.Revision {
		return true
	}
	for _, r := range m.AdditionalRepositories {
		if r.Alias != "" && r.Revision == revision {
			return true
		}
	}
	return false
}

// nativeQualifier is the repository qualifier of a native record ID for an input at revision:
// empty for the primary repository, so a single-repository corpus keeps its bytes, and the alias
// followed by ':' for an aliased additional repository (DCP-V1-601).
func (m Manifest) nativeQualifier(revision string) string {
	for _, r := range m.AdditionalRepositories {
		if r.Alias != "" && r.Revision == revision {
			return r.Alias + ":"
		}
	}
	return ""
}

// RevisionRoot is the checkout directory whose object store holds revision (DCP-V1-176).
func (m Manifest) RevisionRoot(root, revision string) string {
	_, dir := m.revisionRepository(root, revision)
	return dir
}

// checkoutUnavailable refuses a checkout gitauth.Open refused, keeping the checkout's name and
// the underlying refusal's code and message, such as the object-alternates fix, so each cause is
// told apart (DCI-V1-025).
func checkoutUnavailable(name string, err error) *Error {
	return &Error{Code: "corpus-input-unavailable", Message: name + ": " + err.Error()}
}

func (c *compiler) pin(ctx context.Context) (err error) {
	pins.Add(1)
	defer func() { err = gitCapacity(err, fmt.Sprintf("while pinning %d inputs", len(c.manifest.Inputs))) }()
	revisions := map[string]bool{c.manifest.Repository.Revision: true}
	for _, r := range c.manifest.AdditionalRepositories {
		for _, revision := range r.pinned() {
			revisions[revision] = true
		}
	}
	for _, s := range c.manifest.Scopes {
		revisions[s.Revision] = true
	}
	// Each repository is read only from its own declared checkout, primary first (DCP-V1-176).
	dirs, seen := []string{c.root}, map[string]bool{filepath.Clean(c.root): true}
	for _, r := range c.manifest.AdditionalRepositories {
		dir := c.manifest.RevisionRoot(c.root, r.Revision)
		if seen[filepath.Clean(dir)] {
			return fail("invalid additional repository declaration")
		}
		seen[filepath.Clean(dir)] = true
		dirs = append(dirs, dir)
	}
	sessions := map[string]*gitauth.Repository{}
	for i, dir := range dirs {
		auth, err := gitauth.Open(dir, newGitBudget(ctx))
		if err != nil {
			if i > 0 {
				return checkoutUnavailable("additional repository checkout unavailable: "+c.manifest.AdditionalRepositories[i-1].ID, err)
			}
			return err
		}
		release := auth.BeginObjectSession()
		defer release()
		sessions[dir] = auth
	}
	indexes := map[string]*contextindex.Index{}
	for rev := range revisions {
		want, dir := c.manifest.revisionRepository(c.root, rev)
		id, err := contextindex.CorpusRepositoryID(ctx, dir, rev)
		if err != nil {
			if want != c.manifest.Repository.ID {
				return namedRefusal("additional repository "+want, err)
			}
			return err
		}
		if id != want {
			return fail("repository identity mismatch")
		}
		index, err := contextindex.BuildRevisionContext(ctx, dir, rev)
		if err != nil {
			return err
		}
		indexes[rev] = index
	}
	declared := map[string]map[string]bool{}
	declare := func(revision, path string) {
		if declared[revision] == nil {
			declared[revision] = map[string]bool{}
		}
		declared[revision][path] = true
	}
	for _, in := range c.manifest.Inputs {
		declare(in.Revision, in.Path)
	}
	for _, in := range c.manifest.OverBoundInputs {
		declare(in.Revision, in.Path)
	}
	// Each tracked and skipped path is charged to the scopes holding it, then the scopes are
	// checked in manifest order with the precedence a per-scope pass had: an undeclared input,
	// then an unsupported entry, then an empty scope (DCR-V1-450).
	type scopeTally struct {
		count                   int
		undeclared, unsupported bool
	}
	scopes := newScopeSet(c.manifest.Scopes, nil)
	tallies := map[string]map[string]*scopeTally{}
	for rev, paths := range scopes {
		tallies[rev] = make(map[string]*scopeTally, len(paths))
		for p := range paths {
			tallies[rev][p] = &scopeTally{}
		}
		for p := range indexes[rev].Tracked {
			scopes.each(rev, p, func(scope string) {
				tally := tallies[rev][scope]
				tally.count++
				tally.undeclared = tally.undeclared || !declared[rev][p]
			})
		}
		for p := range indexes[rev].Skipped {
			scopes.each(rev, p, func(scope string) { tallies[rev][scope].unsupported = true })
		}
	}
	for _, s := range c.manifest.Scopes {
		tally := tallies[s.Revision][s.Path]
		switch {
		case tally.undeclared:
			return fail("undeclared input in scope")
		case tally.unsupported:
			return fail("scope contains unsupported tree entry")
		case tally.count == 0:
			return fail("declared scope missing")
		}
	}
	// Admit every input from the index or its Git header before any body is
	// read, so a capacity refusal precedes the expensive work (DCP-V1-157).
	paths := map[string]map[string][]string{}
	for _, in := range c.manifest.Inputs {
		dir := c.manifest.RevisionRoot(c.root, in.Revision)
		if paths[dir] == nil {
			paths[dir] = map[string][]string{}
		}
		paths[dir][in.Revision] = append(paths[dir][in.Revision], in.Path)
	}
	loaders := map[string]*inputLoader{}
	for _, dir := range dirs {
		if paths[dir] == nil {
			continue
		}
		loader, err := stageInputs(ctx, sessions[dir], indexes, paths[dir], encodingLimit(c.manifest))
		if err != nil {
			return err
		}
		loaders[dir] = loader
	}
	loaderFor := func(revision string) *inputLoader { return loaders[c.manifest.RevisionRoot(c.root, revision)] }
	// An over-bound input is checked from its Git header only, under the inventory's bound: still
	// the pinned blob, still that size and still over the bound, so a file that shrank or changed
	// is refused (DCI-V1-024).
	over := map[string]map[string][]string{}
	for _, in := range c.manifest.OverBoundInputs {
		dir := c.manifest.RevisionRoot(c.root, in.Revision)
		if over[dir] == nil {
			over[dir] = map[string][]string{}
		}
		over[dir][in.Revision] = append(over[dir][in.Revision], in.Path)
	}
	headers := map[string]*inputLoader{}
	for _, dir := range dirs {
		if over[dir] != nil {
			if headers[dir], err = stageInputs(ctx, sessions[dir], indexes, over[dir], MaxBytes); err != nil {
				return err
			}
		}
	}
	for _, in := range c.manifest.OverBoundInputs {
		loader := headers[c.manifest.RevisionRoot(c.root, in.Revision)]
		if blob, size, over := loader.oversize(in.Revision, in.Path); blob != in.Blob || size != in.Bytes || !over {
			return fail("over-bound input changed after pinning: " + in.Path)
		}
	}
	var charge inputCharge
	charged := map[string]bool{}
	for _, in := range c.manifest.Inputs {
		size, _, err := loaderFor(in.Revision).admit(in.Revision, in.Path)
		if err != nil {
			return err
		}
		if !charged[inputKey(in.Revision, in.Path)] {
			charged[inputKey(in.Revision, in.Path)] = true
			charge.add(in, size)
		}
	}
	if err := charge.check(); err != nil {
		return err
	}
	for _, dir := range dirs {
		if loader := loaders[dir]; loader != nil {
			if err := loader.read(ctx); err != nil {
				return err
			}
		}
	}
	var refused []generatedInput
	for _, in := range c.manifest.Inputs {
		source, err := loaderFor(in.Revision).source(in.Revision, in.Path)
		if err != nil {
			return err
		}
		if source.BlobHash != in.Blob || Digest(source.Data) != in.SHA256 {
			return fail("input changed after pinning")
		}
		if in.Purpose == "source" {
			if check := generatedCheck(in.Path, source.Data); check != "" {
				refused = append(refused, generatedInput{c.manifest.repositoryLabel(in.Revision), in.Path, in.Revision, check})
			}
		}
		c.sources[inputKey(in.Revision, in.Path)] = source
	}
	if len(refused) > 0 {
		return generatedRefusal(refused)
	}
	c.indexes = indexes
	return nil
}

// generatedInput is one source input a generated-output check matched (DCP-V1-607).
type generatedInput struct {
	repository, path, revision, check string
}

// generatedRefusal names every generated source input in manifest order: its repository label,
// path, revision and the check that matched, so the operator can narrow the scope (DCP-V1-607).
func generatedRefusal(inputs []generatedInput) error {
	parts := make([]string, len(inputs))
	for i, in := range inputs {
		parts[i] = fmt.Sprintf("repository=%s path=%s revision=%s check=%s", in.repository, in.path, in.revision, in.check)
	}
	return fail(fmt.Sprintf("generated output cannot be source input (%d inputs; narrow the scope to exclude them): %s", len(inputs), strings.Join(parts, "; ")))
}

// repositoryLabel names the repository holding revision in a refusal: an additional repository's
// alias, "primary" for the primary repository, else the additional repository's identity.
func (m Manifest) repositoryLabel(revision string) string {
	if revision == m.Repository.Revision {
		return "primary"
	}
	if q := m.nativeQualifier(revision); q != "" {
		return strings.TrimSuffix(q, ":")
	}
	for _, r := range m.AdditionalRepositories {
		if slices.Contains(r.pinned(), revision) {
			return r.ID
		}
	}
	return "primary"
}

// generatedCheck names the first generated-output check that matches a source input, or "":
// corpus-marker, corpus-schema, code-generated-comment, then the native generated-path and
// generated-header rules (DCP-V1-607). With no content the native rule can match only by path.
func generatedCheck(path string, data []byte) string {
	if check := generated(data); check != "" {
		return check
	}
	switch {
	case contextindex.CorpusGeneratedSource(path, nil):
		return "generated-path"
	case contextindex.CorpusGeneratedSource(path, data):
		return "generated-header"
	}
	return ""
}

func generated(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if bytes.Contains(trimmed, []byte("<!-- corvint-corpus")) {
		return "corpus-marker"
	}
	var object map[string]any
	if len(trimmed) > 0 && trimmed[0] == '{' && json.Unmarshal(trimmed, &object) == nil {
		schema, _ := object["schema"].(string)
		if strings.HasPrefix(schema, "corvint-corpus-") || schema == Schema || schema == SchemaV2 {
			return "corpus-schema"
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "// Code generated ") || strings.HasPrefix(l, "# Code generated ") {
			return "code-generated-comment"
		}
	}
	return ""
}
func identifier(id string) bool {
	return len(id) > 0 && len(id) <= 1024 && !strings.ContainsAny(id, "\x00\r\n\t")
}

func (c *compiler) native(provider Provider) error {
	// A native-scoped derivation filter drops the declarations of this provider's own inputs it
	// does not select; their file subjects and excerpts stay (DCS-V1-590).
	filter := provider.Derive
	if filter != nil && !filter.Native {
		filter = nil
	}
	filtered, filteredDeclarations, excluded := 0, 0, []string{}
	// Each over-bound scope file is reported, never silently absent: its path, pinned blob and size
	// against the bound (DCI-V1-024).
	for _, in := range c.manifest.OverBoundInputs {
		if in.Provider == provider.ID {
			reason := fmt.Sprintf("input is %d bytes, over the %d-byte per-input bound; pinned at blob %s and not read", in.Bytes, MaxBytes, in.Blob)
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{provider.ID + ":file:" + c.manifest.nativeQualifier(in.Revision) + in.Path, "input-over-bound", reason})
		}
	}
	for _, in := range c.manifest.Inputs {
		if len(c.artifact.Subjects) > c.manifest.recordLimit() || len(c.artifact.Claims) > c.manifest.recordLimit() {
			return fail(c.boundMessage("native record bound exceeded", in.Path, 1))
		}
		if in.Provider != provider.ID {
			continue
		}
		// Each input reads its own repository's index; an aliased additional repository's
		// records are keyed by alias plus path (DCP-V1-601).
		index, q := c.indexes[in.Revision], c.manifest.nativeQualifier(in.Revision)
		if reason := c.nativeExclusion(in); reason != "" {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{provider.ID + ":file:" + q + in.Path, "not-collected", reason})
			continue
		}
		source := c.sources[inputKey(in.Revision, in.Path)]
		text, valid, loaded := source.Text()
		id := provider.ID + ":file:" + q + in.Path
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
			reason := "empty source has no non-vacuous anchor"
			if strings.TrimSpace(text) != "" {
				reason = fmt.Sprintf("no line within the %d-byte anchor bound", maxSpanBytes)
			}
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "not-collected", reason})
			continue
		}
		anchor, err := c.anchor(in.Revision, in.Path, line, line, "syntax", "source", "explicit input inventory")
		if err != nil {
			return err
		}
		ev := Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{"file presence and exact excerpt only; behavior unknown"}}
		c.artifact.Subjects = append(c.artifact.Subjects, Subject{id, kind, in.Path, provider.ID, ev, nil})
		c.artifact.Claims = append(c.artifact.Claims, Claim{id + ":excerpt", id, lineText(text, line), provider.ID, ev})
		if !contextindex.ImpactPathAdmitted(in.Path) {
			c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "unsupported", "native analyzer unavailable; text only"})
		}
		for _, u := range index.Unparsed {
			if u.Path == in.Path {
				c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "not-collected", u.Reason})
			}
		}
		var symbols []contextindex.Symbol
		for _, s := range index.Symbols {
			if s.Path == in.Path {
				symbols = append(symbols, s)
			}
		}
		if _, bad := index.UnparsedPaths()[in.Path]; bad {
			symbols = nil
		} else if _, indexed := index.Sources[in.Path]; indexed {
			symbols = withClassMembers(symbols, in.Path, text)
			if err := c.moduleObjectClaims(provider, in, id, text); err != nil {
				return err
			}
		}
		if len(symbols) > 0 && filter != nil {
			sk, repository := "symbol", c.manifest.repositoryQualifier(in.Revision)
			if isTestPath(in.Path) {
				sk = "test"
			}
			if !filter.selects(repository, in.Path, sk) {
				if filtered++; len(excluded) < 16 {
					excluded = append(excluded, repository+in.Path)
				}
				filteredDeclarations += len(symbols)
				symbols = nil
			}
		}
		var lines []string
		if len(symbols) > 0 {
			lines = strings.Split(text, "\n")
		}
		for _, s := range symbols {
			if len(c.artifact.Subjects) >= c.manifest.recordLimit() || len(c.artifact.Claims) >= c.manifest.recordLimit() {
				return fail(c.boundMessage("native record bound exceeded", in.Path, s.Line))
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
			sid := fmt.Sprintf("%s:symbol:%s%s:%d:%s", provider.ID, q, in.Path, s.Line, s.Name)
			c.artifact.Subjects = append(c.artifact.Subjects, Subject{sid, sk, s.Name, provider.ID, se, nil})
			c.artifact.Claims = append(c.artifact.Claims, Claim{sid + ":declaration", sid, lineAt(lines, s.Line), provider.ID, se})
		}
	}
	if filtered > 0 {
		names := strings.Join(excluded, ", ")
		if more := filtered - len(excluded); more > 0 {
			names += fmt.Sprintf(" and %d more", more)
		}
		reason := fmt.Sprintf("native derivation filter (paths [%s]; kinds [%s]) excluded %d declarations of %d native-owned inputs: %s", strings.Join(filter.Paths, " "), strings.Join(filter.Kinds, " "), filteredDeclarations, filtered, names)
		c.artifact.Gaps = append(c.artifact.Gaps, Gap{provider.ID + ":native-derivation-filter", "not-collected", reason})
	}
	return nil
}

// repositoryQualifier names the repository of revision in a derivation filter entry: empty for
// the primary repository, else the additional repository's ID and a colon (DCS-V1-573).
func (m Manifest) repositoryQualifier(revision string) string {
	if revision == m.Repository.Revision {
		return ""
	}
	for _, r := range m.AdditionalRepositories {
		if slices.Contains(r.pinned(), revision) {
			return r.ID + ":"
		}
	}
	return ""
}

// derivedDeclarations gives the first native provider a declaration subject, and its
// declaration claim, for each symbol the native index holds in a source input that only
// non-native providers claim, so an identifier declared there is searchable whoever produced the
// input's records (DCS-V1-482, DCS-V1-483). Inputs at the primary revision and at each
// additional repository's declared revision are derived; an additional repository's
// declaration IDs name the repository after the kind (DCS-V1-549). IDs and evidence follow the
// native provider's own declarations; no file subject or excerpt is added. It runs after every
// provider. It derives at most what the corpus record bound admits (DCS-V1-568) and, on the
// retry of a compile the output byte bound refused, the prefix that fits under it
// (DCS-V1-569); on the retry of any other refusal it derives nothing (DCS-V1-484). Declarations
// are kept in derivation order: first the inputs a non-native provider's subject or claim anchors,
// then the rest, each in manifest input order and symbol order (DCS-V1-575); the kept ones are
// added in manifest input order, so a build that drops none keeps its bytes. One gap names each
// bound that dropped any, the counts and the first declaration dropped.
func (c *compiler) derivedDeclarations() error {
	var provider Provider
	native := map[string]bool{}
	for _, p := range c.manifest.Providers {
		if p.Kind == "native" {
			if len(native) == 0 {
				provider = p
			}
			native[p.ID] = true
		}
	}
	if len(native) == 0 {
		return nil
	}
	// qualifier names the repository of each derivable revision inside a derived ID: empty for
	// the primary revision, so a single-repository artifact keeps its bytes.
	qualifier := map[string]string{c.manifest.Repository.Revision: ""}
	for _, r := range c.manifest.AdditionalRepositories {
		qualifier[r.Revision] = r.ID + ":"
	}
	owned := map[string]bool{}
	for _, in := range c.manifest.Inputs {
		if native[in.Provider] {
			owned[inputKey(in.Revision, in.Path)] = true
		}
	}
	symbols := map[string][]contextindex.Symbol{}
	unparsed := map[string]map[string]string{}
	for revision := range qualifier {
		index := c.indexes[revision]
		if index == nil {
			continue
		}
		for _, s := range index.Symbols {
			symbols[inputKey(revision, s.Path)] = append(symbols[inputKey(revision, s.Path)], s)
		}
		unparsed[revision] = index.UnparsedPaths()
	}
	type declaration struct {
		subject Subject
		claim   Claim
		at      string
		// input and symbol are the declaration's manifest input order.
		input, symbol int
	}
	// candidate is a source input whose declarations derive, in manifest input order.
	type candidate struct {
		in         Input
		repository string
		kind       string
		text       string
		declared   []contextindex.Symbol
	}
	var candidates []candidate
	// The inputs the derivation filter excludes, the first of them by name (DCS-V1-573).
	var excluded []string
	filtered, filteredDeclarations := 0, 0
	for _, in := range c.manifest.Inputs {
		key := inputKey(in.Revision, in.Path)
		repository, derivable := qualifier[in.Revision]
		if in.Purpose != "source" || !derivable || owned[key] {
			continue
		}
		// An indexed web source with no top-level declaration may still declare class members.
		indexedWeb := false
		if index := c.indexes[in.Revision]; index != nil && contextindex.WebSource(in.Path) {
			_, indexedWeb = index.Sources[in.Path]
		}
		if len(symbols[key]) == 0 && !indexedWeb {
			continue
		}
		owned[key] = true
		if _, bad := unparsed[in.Revision][in.Path]; bad || c.nativeExclusion(in) != "" {
			continue
		}
		text, valid, loaded := c.sources[key].Text()
		if !valid || !loaded {
			continue
		}
		declared := withClassMembers(symbols[key], in.Path, text)
		kind := "symbol"
		if isTestPath(in.Path) {
			kind = "test"
		}
		if len(declared) > 0 && !provider.Derive.selects(repository, in.Path, kind) {
			if filtered++; len(excluded) < 16 {
				excluded = append(excluded, repository+in.Path)
			}
			filteredDeclarations += len(declared)
			continue
		}
		candidates = append(candidates, candidate{in, repository, kind, text, declared})
	}
	// DCS-V1-575: an input a non-native provider's subject or claim anchors derives first, so the
	// declarations the caller's records point at survive the bounds; the order inside each group
	// is manifest input order.
	anchored := map[string]bool{}
	for _, s := range c.artifact.Subjects {
		if !native[s.Provider] {
			for _, a := range s.Evidence.Anchors {
				anchored[inputKey(a.Revision, a.Path)] = true
			}
		}
	}
	for _, cl := range c.artifact.Claims {
		if !native[cl.Provider] {
			for _, a := range cl.Evidence.Anchors {
				anchored[inputKey(a.Revision, a.Path)] = true
			}
		}
	}
	order := make([]int, 0, len(candidates))
	for _, first := range []bool{true, false} {
		for i, cand := range candidates {
			if anchored[inputKey(cand.in.Revision, cand.in.Path)] == first {
				order = append(order, i)
			}
		}
	}
	var pending []declaration
	total, firstDropped, recordBound := 0, "", false
	budget := c.manifest.recordLimit() - max(len(c.artifact.Subjects), len(c.artifact.Claims))
	for _, i := range order {
		in, repository, kind := candidates[i].in, candidates[i].repository, candidates[i].kind
		var lines []string
		for n, s := range candidates[i].declared {
			total++
			if c.derivation.none || len(pending) >= budget {
				recordBound = recordBound || !c.derivation.none
				if firstDropped == "" {
					firstDropped = fmt.Sprintf("%s%s:%d", repository, in.Path, s.Line)
				}
				continue
			}
			// Counted before anchoring, so a refused first declaration is retried too.
			c.derived++
			a, err := c.anchor(in.Revision, in.Path, s.Line, s.Line, "syntax", "source", "native indexed declaration")
			if err != nil {
				return err
			}
			a.Symbol = s.Name
			if lines == nil {
				lines = strings.Split(candidates[i].text, "\n")
			}
			ev := Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{a}, Limitations: []string{"declaration only; build applicability and behavioral coverage unknown"}}
			id := fmt.Sprintf("%s:symbol:%s%s:%d:%s", provider.ID, repository, in.Path, s.Line, s.Name)
			pending = append(pending, declaration{Subject{id, kind, s.Name, provider.ID, ev, nil}, Claim{id + ":declaration", id, lineAt(lines, s.Line), provider.ID, ev}, fmt.Sprintf("%s%s:%d", repository, in.Path, s.Line), i, n})
		}
	}
	kept := len(pending)
	var bounds []string
	if recordBound {
		bounds = append(bounds, fmt.Sprintf("record bound %d records (%s) reached", c.manifest.recordLimit(), c.manifest.Schema))
	}
	if plan := c.derivation; plan.outputBytes > 0 {
		// The refused compile derived plan.attempted declarations, the same ones in the same
		// order, and exceeded the bound by over bytes; keep the prefix whose estimated bytes
		// leave that much and a reserve for the bytes the estimate misses.
		limit := int64(encodingLimit(c.artifact))
		budget := -(plan.outputBytes - limit) - derivedByteReserve
		for _, d := range pending {
			budget += declarationBytes(d.subject, d.claim)
		}
		kept = 0
		for _, d := range pending {
			if budget -= declarationBytes(d.subject, d.claim); budget < 0 {
				break
			}
			kept++
		}
		bounds = append(bounds, fmt.Sprintf("output byte bound %d bytes (%s) exceeded at %d bytes with %d derived", limit, c.artifact.Schema, plan.outputBytes, plan.attempted))
		c.derived = kept
	}
	if c.derivation.none {
		bounds = append(bounds, fmt.Sprintf("the compile with %d derived was refused (%s)", c.derivation.attempted, c.derivation.refusal))
	}
	if kept < len(pending) {
		firstDropped = pending[kept].at
	}
	// The kept declarations are added in manifest input order and symbol order, whatever order
	// derived them.
	added := slices.Clone(pending[:kept])
	slices.SortStableFunc(added, func(x, y declaration) int {
		if x.input != y.input {
			return x.input - y.input
		}
		return x.symbol - y.symbol
	})
	for _, d := range added {
		c.artifact.Subjects = append(c.artifact.Subjects, d.subject)
		c.artifact.Claims = append(c.artifact.Claims, d.claim)
	}
	if kept < total {
		reason := fmt.Sprintf("derived declaration bound: %s; derived %d of %d declarations of source inputs other providers claim, %d dropped from %s onward in derivation order (inputs records anchor first, then manifest input order)", strings.Join(bounds, "; "), kept, total, total-kept, firstDropped)
		c.artifact.Gaps = append(c.artifact.Gaps, Gap{provider.ID + ":derived-declarations", "not-collected", reason})
	}
	if filtered > 0 {
		f := provider.Derive
		names := strings.Join(excluded, ", ")
		if more := filtered - len(excluded); more > 0 {
			names += fmt.Sprintf(" and %d more", more)
		}
		reason := fmt.Sprintf("derivation filter (paths [%s]; kinds [%s]) excluded %d declarations of %d source inputs: %s", strings.Join(f.Paths, " "), strings.Join(f.Kinds, " "), filteredDeclarations, filtered, names)
		c.artifact.Gaps = append(c.artifact.Gaps, Gap{provider.ID + ":derivation-filter", "not-collected", reason})
	}
	return nil
}

// validDerivationFilter refuses a derivation filter on a records provider, an empty one, an
// unknown kind, an entry that names no canonical path or no declared additional repository, and
// unsorted or duplicate entries (DCS-V1-573).
func validDerivationFilter(m Manifest, p Provider) error {
	f := p.Derive
	if p.Kind != "native" {
		return fail("derivation filter on a records provider")
	}
	// Only the first native provider derives declarations (DCS-V1-482).
	for _, first := range m.Providers {
		if first.Kind == "native" && first.ID != p.ID {
			return fail("derivation filter on a native provider that derives nothing")
		} else if first.Kind == "native" {
			break
		}
	}
	if len(f.Paths) == 0 && len(f.Kinds) == 0 {
		return fail("vacuous derivation filter")
	}
	// An explicit empty list is refused: encoding would drop it (DCI-V1-230).
	if f.Paths != nil && len(f.Paths) == 0 || f.Kinds != nil && len(f.Kinds) == 0 {
		return fail("invalid derivation filter: an explicit paths or kinds list is not empty")
	}
	if len(f.Paths) > 64 {
		return capacityExceeded("derivation filter path count", len(f.Paths), 64)
	}
	repositories := map[string]bool{}
	for _, r := range m.AdditionalRepositories {
		repositories[r.ID] = true
	}
	for i, entry := range f.Paths {
		// A colon always ends a repository qualifier, so a misspelled one is refused rather
		// than read as a primary path that selects nothing.
		path := entry
		if repository, rest, ok := strings.Cut(entry, ":"); ok {
			if !repositories[repository] {
				return fail("derivation filter path names an undeclared repository")
			}
			path = rest
		}
		if !validPath(strings.TrimSuffix(path, "/")) || i > 0 && entry <= f.Paths[i-1] {
			return fail("invalid, unsorted or duplicate derivation filter path")
		}
	}
	for i, kind := range f.Kinds {
		if kind != "module" && kind != "symbol" && kind != "test" || i > 0 && kind <= f.Kinds[i-1] {
			return fail("invalid, unsorted or duplicate derivation filter kind")
		}
	}
	return nil
}

// selects reports whether the filter derives the declarations of the source input path, of
// declaration kind kind, in the repository whose ID qualifier is repository (DCS-V1-573).
func (f *DerivationFilter) selects(repository, path, kind string) bool {
	if f == nil {
		return true
	}
	if len(f.Kinds) > 0 && !slices.Contains(f.Kinds, kind) {
		return false
	}
	if len(f.Paths) == 0 {
		return true
	}
	qualified := repository + path
	for _, entry := range f.Paths {
		if entry == qualified || strings.HasSuffix(entry, "/") && strings.HasPrefix(qualified, entry) {
			return true
		}
	}
	return false
}

// withClassMembers adds the class methods contextindex.WebClassMembers names in a web source
// to that path's indexed symbols, in line order, so the methods get the declaration subject
// and claim every other symbol gets (DCS-V1-571). A symbol already indexed at the same line
// and name is kept once, and a source the member walk cannot finish adds none. It adds the
// top-level keys of exported object literals contextindex.WebExportedObjectKeys names the
// same way (DCS-V1-583).
func withClassMembers(symbols []contextindex.Symbol, path, text string) []contextindex.Symbol {
	members, ok := contextindex.WebClassMembers(path, text)
	if !ok {
		members = nil
	}
	if keys, ok := contextindex.WebExportedObjectKeys(path, text); ok {
		members = append(members, keys...)
	}
	if len(members) == 0 {
		return symbols
	}
	type at struct {
		line int
		name string
	}
	seen := map[at]bool{}
	for _, s := range symbols {
		seen[at{s.Line, s.Name}] = true
	}
	out := slices.Clip(symbols)
	for _, m := range members {
		// A getter and setter of one name on one line are one declaration.
		if !seen[at{m.Line, m.Name}] {
			seen[at{m.Line, m.Name}] = true
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, b contextindex.Symbol) int { return a.Line - b.Line })
	return out
}

// derivedByteReserve is the slack a byte-bounded derivation retry leaves under the output byte
// bound for the bytes declarationBytes does not count, such as the bound gap itself
// (DCS-V1-569).
const derivedByteReserve = 4096

// declarationBytes is a lower bound on the encoded bytes one derived declaration adds to the
// artifact: its subject and claim, their quoted IDs in the subjects and claims capability
// records, the array separators, and for a test subject its unobserved test gap with its
// separator and the shortest quoted gap offset in the gaps capability record. The retry keeps
// a prefix only while the bytes it drops reach the overshoot, so an overestimate could keep
// too many and be refused again (DCS-V1-569).
func declarationBytes(s Subject, claim Claim) int64 {
	size := func(value any) int64 {
		n, _ := EncodeTo(io.Discard, value)
		return n
	}
	n := size(s) + size(claim) + int64(len(s.ID)+len(claim.ID)) + 8
	if s.Kind == "test" {
		n += size(Gap{s.ID, "not-collected", "test declaration has no retained observation"}) + 1 + 4
	}
	return n
}

// firstContentLine is the first non-blank line whose span fits the excerpt bound, so one
// oversized leading line no longer refuses the build (DCP-V1-146).
func firstContentLine(text string) int {
	for n, line := range strings.SplitAfter(text, "\n") {
		if strings.TrimSpace(line) != "" && len(line) <= maxSpanBytes {
			return n + 1
		}
	}
	return 0
}
func lineText(text string, n int) string {
	return lineAt(strings.Split(text, "\n"), n)
}

// lineAt is lineText over the text already split at "\n", so a loop over one input's
// declarations splits it once (V1-2613).
func lineAt(lines []string, n int) string {
	if n < 1 || n > len(lines) {
		return ""
	}
	return lines[n-1]
}
func isTestPath(p string) bool {
	return strings.HasSuffix(p, "_test.go") || strings.Contains(p, ".test.") || strings.Contains(p, ".spec.") || strings.HasPrefix(path.Base(p), "test_") || strings.HasSuffix(p, "_test.py") || strings.HasSuffix(p, "_spec.rb")
}

// maxSpanBytes bounds one evidence excerpt.
const maxSpanBytes = 64 << 10

func span(data []byte, start, end int) ([]byte, error) {
	if start < 1 || end < start {
		return nil, fail("invalid anchor span")
	}
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if end > len(lines) {
		return nil, fail("anchor span out of range")
	}
	s := bytes.Join(lines[start-1:end], nil)
	if len(s) > maxSpanBytes || len(bytes.TrimSpace(s)) == 0 {
		return nil, fail("vacuous or oversized anchor")
	}
	return s, nil
}

// located names the input span a refusal concerns (DCP-V1-146).
func located(err error, p string, start, end int) error {
	var e *Error
	if !errors.As(err, &e) {
		return err
	}
	return &Error{Code: e.Code, Message: fmt.Sprintf("%s at %s:%d-%d", e.Message, p, start, end)}
}

// paragraphParts splits lines start..end (1-based, inclusive) into consecutive line-aligned
// spans of at most maxSpanBytes, so an oversized paragraph such as a long table stays
// verbatim instead of refusing the build. A single line past the bound remains its own
// span, which the caller records as a not-collected gap (DCP-V1-146).
func paragraphParts(lines []string, start, end int) [][2]int {
	var parts [][2]int
	from, size := start, 0
	for i := start; i <= end; i++ {
		if size > 0 && size+len(lines[i-1]) > maxSpanBytes {
			parts = append(parts, [2]int{from, i - 1})
			from, size = i, 0
		}
		size += len(lines[i-1])
	}
	return append(parts, [2]int{from, end})
}

func (c *compiler) anchor(revision, p string, start, end int, authority, kind, reason string) (Anchor, error) {
	source, ok := c.sources[inputKey(revision, p)]
	if !ok {
		return Anchor{}, fail("anchor input was not declared")
	}
	lines := c.anchorLines[inputKey(revision, p)]
	if lines == nil {
		lines = newAnchorLines(source.Data)
		if c.anchorLines == nil {
			c.anchorLines = map[string]*anchorLines{}
		}
		c.anchorLines[inputKey(revision, p)] = lines
	}
	data, err := lines.span(source.Data, start, end)
	if err != nil {
		return Anchor{}, located(err, p, start, end)
	}
	repository, _ := c.manifest.revisionRepository(c.root, revision)
	return Anchor{Repository: repository, Revision: revision, Path: p, Blob: source.BlobHash, SHA256: lines.sha256, Start: start, End: end, SpanSHA256: Digest(data), Authority: authority, Kind: kind, Reason: reason}, nil
}

// anchorLines is what every anchor into one pinned input shares: the input's digest and the
// offset of each line, so anchoring a declaration costs its own line rather than a digest and
// a split of the whole input (V1-2613). span selects the bytes span() selects.
type anchorLines struct {
	sha256 string
	// starts holds the offset of each line span() would split, then len(data).
	starts []int
}

func newAnchorLines(data []byte) *anchorLines {
	starts := make([]int, 1, bytes.Count(data, []byte{'\n'})+2)
	for i, b := range data {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &anchorLines{sha256: Digest(data), starts: append(starts, len(data))}
}

func (l *anchorLines) span(data []byte, start, end int) ([]byte, error) {
	if start < 1 || end < start {
		return nil, fail("invalid anchor span")
	}
	if end > len(l.starts)-1 {
		return nil, fail("anchor span out of range")
	}
	s := data[l.starts[start-1]:l.starts[end]]
	if len(s) > maxSpanBytes || len(bytes.TrimSpace(s)) == 0 {
		return nil, fail("vacuous or oversized anchor")
	}
	return s, nil
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
	if err := rederivedEqual(rebuilt, data); err != nil {
		return nil, err
	}
	return rebuilt, nil
}

// rederivedEqual compares the canonical encoding of rebuilt with data while the encoder writes,
// so a read never holds a second artifact-sized copy and its growth slack (DCR-V1-414). The
// output bound and the refusal are the ones encoding into a buffer applied.
func rederivedEqual(rebuilt *Artifact, data []byte) error {
	c := encodingComparer{want: data}
	if _, err := EncodeTo(&c, rebuilt); err != nil {
		return err
	}
	if c.n+1 > int64(encodingLimit(rebuilt)) {
		return fail("output bound exceeded")
	}
	if !c.equal() {
		return fail("artifact cannot be rederived from current builder and pinned inputs")
	}
	return nil
}

func (c *compiler) nativeImports(provider Provider) error {
	available := map[string]bool{}
	for _, s := range c.artifact.Subjects {
		available[s.ID] = true
	}
	packages := map[string]bool{}
	for _, in := range c.manifest.Inputs {
		if in.Provider != provider.ID || !strings.HasSuffix(in.Path, ".go") {
			continue
		}
		index, q := c.indexes[in.Revision], c.manifest.nativeQualifier(in.Revision)
		from := provider.ID + ":file:" + q + in.Path
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
				c.artifact.Subjects = append(c.artifact.Subjects, Subject{to, "package", imported, provider.ID, ev, nil})
				packages[to] = true
			}
			c.artifact.Relations = append(c.artifact.Relations, Relation{provider.ID + ":imports:" + q + in.Path + ":" + imported, from, to, "imports", provider.ID, ev})
			if len(c.artifact.Subjects) > c.manifest.recordLimit() || len(c.artifact.Relations) > c.manifest.recordLimit() {
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
		id := provider.ID + ":file:" + c.manifest.nativeQualifier(in.Revision) + in.Path
		if firstContentLine(text) == 0 {
			continue // no file subject; native recorded the gap
		}
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
			for _, part := range paragraphParts(lines, start, i) {
				if part[0] == part[1] && len(lines[part[0]-1]) > maxSpanBytes {
					c.artifact.Gaps = append(c.artifact.Gaps, Gap{id, "not-collected", fmt.Sprintf("paragraph line %d exceeds the %d-byte anchor bound", part[0], maxSpanBytes)})
					continue
				}
				anchor, err := c.anchor(in.Revision, in.Path, part[0], part[1], "source-document", "source", "verbatim source paragraph")
				if err != nil {
					return err
				}
				excerpt := strings.Join(lines[part[0]-1:part[1]], "")
				ev := Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{anchor}, Limitations: []string{"verbatim source excerpt; quoted prose is not accepted intent or verified behavior"}}
				c.artifact.Claims = append(c.artifact.Claims, Claim{fmt.Sprintf("%s:paragraph:%d", id, part[0]), id, excerpt, provider.ID, ev})
				if len(c.artifact.Claims) > c.manifest.recordLimit() {
					return fail(c.boundMessage("native paragraph bound exceeded", in.Path, part[0]))
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

// boundMessage locates a record-bound refusal and, under /1, names the manifest that raises the
// bound (DCP-V1-146).
func (c *compiler) boundMessage(prefix, path string, line int) string {
	message := fmt.Sprintf("%s: more than %d records at %s:%d", prefix, c.manifest.recordLimit(), path, line)
	if c.manifest.Schema != ManifestSchemaV2 {
		message += "; a " + ManifestSchemaV2 + " manifest raises the bound"
	}
	return message
}
