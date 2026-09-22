package extevidence

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// Identity states for a declared repository (EEP-V1-002).
const (
	IdentityResolved   = "resolved"
	IdentityUnresolved = "unresolved"
	IdentityAmbiguous  = "ambiguous"
)

// Binding states for a declared repository (EEP-V1-003).
const (
	BindingRoot        = "root"
	BindingCheckout    = "checkout"
	BindingUnbound     = "unbound"
	BindingMismatch    = "mismatch"
	BindingUnavailable = "unavailable"
	BindingAmbiguous   = "ambiguous"
	BindingUnresolved  = "unresolved"
)

// Freshness states V1 adds to the EEP-V0-009 set (EEP-V1-005).
const (
	FreshnessTreeMismatch       = "tree-mismatch"
	FreshnessIdentityUnresolved = "identity-unresolved"
	FreshnessIdentityAmbiguous  = "identity-ambiguous"
	FreshnessNotEvaluated       = "not-evaluated"
)

// Relation states (EEP-V1-008), in precedence order.
const (
	RelationUnresolved  = "unresolved"
	RelationStale       = "stale"
	RelationNotVerified = "not-verified"
	RelationFresh       = "fresh"
)

// Checkout is one operator-supplied `--repository ID=DIR` binding.
type Checkout struct {
	ID, Source string
}

// checkout is a resolved local checkout: its HEAD commit and root commits.
type checkout struct {
	source, dir, head, state, reason string
	roots                            map[string]struct{}
}

type repositoryState struct {
	declared                                     Repository1
	identity, binding, freshness, captured, from string
}

// bindings resolves the root checkout and every operator checkout once per
// Section so each V1 record binds against the same observations.
type bindings struct {
	root      checkout
	checkouts map[string]checkout
	order     []Checkout
}

func resolveBindings(ctx context.Context, root rootRepository, checkouts []Checkout) *bindings {
	out := &bindings{
		root:      checkout{dir: root.dir, head: root.revision, state: "resolved", roots: rootCommits(ctx, root.dir, root.revision)},
		checkouts: make(map[string]checkout, len(checkouts)),
		order:     checkouts,
	}
	for _, entry := range checkouts {
		out.checkouts[entry.ID] = resolveCheckout(ctx, root.dir, entry.Source)
	}
	return out
}

func resolveCheckout(ctx context.Context, root, source string) checkout {
	entry := checkout{source: source, state: BindingUnavailable}
	dir := source
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, filepath.FromSlash(source))
	}
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		entry.reason = "cannot read checkout: " + describe(err)
		return entry
	}
	// Git walks up to an enclosing repository, so the directory must be the
	// top level itself or a subdirectory would silently bind its parent.
	top, err := runGit(ctx, canonical, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		entry.reason = "not a Git checkout"
		return entry
	}
	topCanonical, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil || topCanonical != canonical {
		entry.reason = "not the top level of a Git checkout"
		return entry
	}
	head, err := runGit(ctx, canonical, nil, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		entry.reason = "checkout has no HEAD commit"
		return entry
	}
	entry.dir, entry.head, entry.state = canonical, strings.TrimSpace(string(head)), "resolved"
	entry.roots = rootCommits(ctx, canonical, entry.head)
	entry.reason = "checkout resolved at HEAD " + entry.head
	return entry
}

// rootCommits lists the parentless commits reachable from revision, the only
// repository identity V1 accepts as authoritative (EEP-V1-002).
func rootCommits(ctx context.Context, dir, revision string) map[string]struct{} {
	roots := make(map[string]struct{})
	output, err := runGit(ctx, dir, nil, "rev-list", "--max-parents=0", revision)
	if err != nil {
		return roots
	}
	for _, line := range strings.Fields(string(output)) {
		roots[line] = struct{}{}
	}
	return roots
}

// repositoryStates applies EEP-V1-002, -003, and -005 to one record.
func repositoryStates(ctx context.Context, record Record1, bound *bindings) (map[string]*repositoryState, string) {
	states := make(map[string]*repositoryState, len(record.Repositories))
	origins := make(map[string]int, len(record.Repositories))
	for _, declared := range record.Repositories {
		origins[declared.Origin]++
	}
	var rootCandidates []string
	for _, declared := range record.Repositories {
		state := &repositoryState{declared: declared}
		states[declared.ID] = state
		state.identity = identityOf(declared, origins)
		state.binding = bindingOf(state, bound)
		if state.binding == BindingRoot {
			rootCandidates = append(rootCandidates, declared.ID)
		}
	}
	primary := ""
	if len(rootCandidates) == 1 {
		primary = rootCandidates[0]
	}
	for _, id := range rootCandidates {
		if primary == "" {
			states[id].binding = BindingAmbiguous
		}
	}
	for _, state := range states {
		state.freshness = freshnessOf(ctx, state, bound)
	}
	return states, primary
}

func identityOf(declared Repository1, origins map[string]int) string {
	if declared.Origin == "" {
		return IdentityUnresolved
	}
	if origins[declared.Origin] > 1 {
		return IdentityAmbiguous
	}
	return IdentityResolved
}

// bindingOf binds a resolved identity to an explicit checkout when the
// operator named one, else to the root checkout when its origin is a root
// commit of the captured revision. Paths, branches, and remotes never bind.
func bindingOf(state *repositoryState, bound *bindings) string {
	if state.identity != IdentityResolved {
		return BindingUnresolved
	}
	origin := state.declared.Origin
	if explicit, named := bound.checkouts[state.declared.ID]; named {
		state.from = explicit.source
		if explicit.state != "resolved" {
			return BindingUnavailable
		}
		if _, matches := explicit.roots[origin]; !matches {
			return BindingMismatch
		}
		state.captured = explicit.head
		return BindingCheckout
	}
	if _, matches := bound.root.roots[origin]; matches {
		state.captured = bound.root.head
		return BindingRoot
	}
	return BindingUnbound
}

func freshnessOf(ctx context.Context, state *repositoryState, bound *bindings) string {
	switch state.identity {
	case IdentityUnresolved:
		return FreshnessIdentityUnresolved
	case IdentityAmbiguous:
		return FreshnessIdentityAmbiguous
	}
	dir := bound.root.dir
	switch state.binding {
	case BindingAmbiguous:
		return FreshnessIdentityAmbiguous
	case BindingCheckout:
		dir = bound.checkouts[state.declared.ID].dir
	case BindingRoot:
	default:
		return FreshnessNotEvaluated
	}
	freshness := Freshness(ctx, dir, state.captured, state.declared.Revision)
	if freshness == FreshnessRevisionUnavailable || state.declared.Tree == "" {
		return freshness
	}
	tree, err := runGit(ctx, dir, nil, "rev-parse", "--verify", "--quiet", state.declared.Revision+"^{tree}")
	if err != nil || strings.TrimSpace(string(tree)) != state.declared.Tree {
		return FreshnessTreeMismatch
	}
	return freshness
}

// view1 resolves one V1 record against its repository states and trees.
func view1(ctx context.Context, root rootRepository, record Record1, bound *bindings) *view {
	states, primary := repositoryStates(ctx, record, bound)
	v := &view{
		provider: record.Provider.ID, entities: entityMap(record.Entities), primary: primary, repositories: states, trees: map[string]tree{},
		pathToPath: record.Schema == Schema2, revision: record.Provider.Revision,
	}
	for position := range record.Relations {
		resolved, failure := v.resolve1(&record.Relations[position])
		if failure != nil {
			v.unknowns = append(v.unknowns, *failure)
			continue
		}
		v.links = append(v.links, resolved)
	}
	pinned := make(map[string][]string)
	for _, candidate := range v.links {
		for _, side := range []endpoint{candidate.from, candidate.to} {
			if side.isPath() {
				pinned[side.repository] = append(pinned[side.repository], side.path)
			}
			pinned[primary] = append(pinned[primary], v.held(side, root.changed)...)
		}
	}
	v.deleted = map[string]map[string]struct{}{}
	for id, state := range states {
		dir := root.dir
		switch state.binding {
		case BindingRoot:
			v.trees[id] = root.tree(ctx, pinned[id])
		case BindingCheckout:
			dir = bound.checkouts[id].dir
			v.trees[id] = checkoutTree(ctx, bound.checkouts[id], pinned[id])
		default:
			continue
		}
		v.deleted[id] = deletedPaths(ctx, dir, state.freshness, state.declared.Revision, v.trees[id], v.pathsIn(id))
	}
	return v
}

// held lists the changed root paths a V2 directory scope holds.
func (v *view) held(side endpoint, changed []string) []string {
	var out []string
	for _, path := range changed {
		if v.pathToPath && side.contains(endpoint{repository: v.primary, path: path}) {
			out = append(out, path)
		}
	}
	return out
}

func (v *view) resolve1(relation *Relation1) (link, *unknown) {
	keys := Relation{From: endpointKey(relation.From), To: endpointKey(relation.To), Type: relation.Type, Evidence: relation.Evidence, Rule: relation.Rule, Reference: relation.Reference}
	failure := func(state, reason string) (link, *unknown) {
		return link{}, &unknown{provider: v.provider, relation: keys, structured: relation, state: state, reason: reason}
	}
	if _, known := evidenceKinds[relation.Evidence]; !known {
		return failure(unknownExcluded, fmt.Sprintf("evidence kind %q is not declared, observed, or inferred", relation.Evidence))
	}
	from, reason := v.endpoint1(relation.From)
	if reason != "" {
		return failure(unknownUnresolved, "from: "+reason)
	}
	to, reason := v.endpoint1(relation.To)
	if reason != "" {
		return failure(unknownUnresolved, "to: "+reason)
	}
	if from.isPath() && to.isPath() && !v.pathToPath {
		return failure(unknownUnsupported, "path-to-path relations are not composed; relate each path to an entity")
	}
	return link{relation: keys, structured: relation, from: from, to: to}, nil
}

func (v *view) endpoint1(raw Endpoint1) (endpoint, string) {
	if raw.Path != "" {
		if _, declared := v.repositories[raw.Repository]; !declared {
			return endpoint{}, fmt.Sprintf("repository %q is not declared by the record", raw.Repository)
		}
		return endpoint{repository: raw.Repository, path: raw.Path, blob: raw.Blob}, checkPath(raw.Path)
	}
	if raw.Provider != v.provider {
		return endpoint{}, fmt.Sprintf("endpoint names provider %q; V1 resolves only the record's own provider", raw.Provider)
	}
	if _, declared := v.entities[raw.Entity]; !declared {
		return endpoint{}, fmt.Sprintf("entity %q is not declared by the record", raw.Entity)
	}
	return endpoint{entity: raw.Entity}, ""
}

// endpointKey is the ordering key of a structured endpoint; it is never parsed.
func endpointKey(raw Endpoint1) string {
	if raw.Path != "" {
		return "repo:" + raw.Repository + ":path:" + raw.Path
	}
	return "provider:" + raw.Provider + ":entity:" + raw.Entity
}

func endpointMap(raw Endpoint1) map[string]any {
	if raw.Path == "" {
		return map[string]any{"provider": raw.Provider, "entity": raw.Entity}
	}
	out := map[string]any{"repository": raw.Repository, "path": raw.Path}
	if raw.Blob != "" {
		out["blob"] = raw.Blob
	}
	return out
}

// label names a path endpoint in a reason; V0 keeps its bare path.
func (v *view) label(target endpoint) string {
	if v.repositories == nil {
		return target.path
	}
	return target.repository + ":" + target.path
}

// annotate adds the V1 per-endpoint repository evidence and the relation
// state to an item (EEP-V1-007, -008, -010). V0 items carry none of it.
func (v *view) annotate(candidate link, anchor endpoint) map[string]any {
	if v.repositories == nil {
		return nil
	}
	endpoints := make([]any, 0, 2)
	state, crosses := RelationFresh, false
	paths := 0
	for _, side := range []endpoint{candidate.from, candidate.to} {
		if !side.isPath() {
			endpoints = append(endpoints, map[string]any{"provider": v.provider, "entity": side.entity, "verification": VerificationUnsupported})
			continue
		}
		paths++
		repository := v.repositories[side.repository]
		verification := v.verify(side)
		endpoints = append(endpoints, repository.endpointMap(side, verification))
		state = worse(state, sideState(repository, verification))
		crosses = crosses || side.repository != v.primary
	}
	if paths == 0 {
		state = RelationNotVerified
	}
	out := map[string]any{"endpoints": endpoints, "relation_state": state, "crosses_repositories": crosses}
	if anchor.isPath() {
		out["repository"] = anchor.repository
	}
	return out
}

func (state *repositoryState) endpointMap(side endpoint, verification string) map[string]any {
	out := map[string]any{
		"repository": side.repository, "path": side.path, "verification": verification,
		"revision": state.declared.Revision, "identity": state.identity, "binding": state.binding, "freshness": state.freshness,
	}
	optional := map[string]string{"origin": state.declared.Origin, "tree": state.declared.Tree, "captured_revision": state.captured, "checkout": state.from, "blob": side.blob}
	for key, value := range optional {
		if value != "" {
			out[key] = value
		}
	}
	return out
}

var sideStates = map[string]string{
	BindingUnresolved: RelationUnresolved, BindingAmbiguous: RelationUnresolved, BindingMismatch: RelationUnresolved,
	BindingUnbound: RelationNotVerified, BindingUnavailable: RelationNotVerified,
}

var freshnessStates = map[string]string{
	FreshnessEqual: RelationFresh, FreshnessRevisionUnavailable: RelationNotVerified,
	FreshnessRepositoryAhead: RelationStale, FreshnessProviderAhead: RelationStale,
	FreshnessUnrelatedHistory: RelationStale, FreshnessTreeMismatch: RelationStale,
}

// staleVerification lists the path verification states that make a relation
// side stale (EEP-V1-008): the pinned content differs, or the path is gone.
var staleVerification = map[string]struct{}{VerificationStale: {}, VerificationMissing: {}, VerificationDeleted: {}}

func sideState(repository *repositoryState, verification string) string {
	if state, decided := sideStates[repository.binding]; decided {
		return state
	}
	if _, unverified := staleVerification[verification]; unverified {
		return RelationStale
	}
	return freshnessStates[repository.freshness]
}

var relationRank = map[string]int{RelationFresh: 0, RelationNotVerified: 1, RelationStale: 2, RelationUnresolved: 3}

func worse(a, b string) string {
	if relationRank[b] > relationRank[a] {
		return b
	}
	return a
}

// rootTree answers tracked and blob questions at the captured root revision,
// preferring blobs the index already read.
func rootTree(ctx context.Context, index *contextindex.Index, paths []string) tree {
	blobs := make(map[string]string)
	var pending []string
	for _, path := range paths {
		if _, tracked := index.Tracked[path]; !tracked {
			continue
		}
		if source, read := index.Sources[path]; read {
			blobs[path] = source.BlobHash
			continue
		}
		if _, queued := blobs[path]; !queued {
			blobs[path] = ""
			pending = append(pending, path)
		}
	}
	sort.Strings(pending)
	for path, oid := range batchBlobs(ctx, index.Root, index.CommitRevision, pending) {
		blobs[path] = oid
	}
	return tree{
		tracked: func(path string) bool { _, ok := index.Tracked[path]; return ok },
		blob: func(path string) (string, bool) {
			oid, known := blobs[path]
			return oid, known && oid != ""
		},
	}
}

// checkoutTree answers the same questions at a bound checkout's HEAD; a path
// is tracked there when it names a blob.
func checkoutTree(ctx context.Context, bound checkout, paths []string) tree {
	unique := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		unique[path] = struct{}{}
	}
	ordered := make([]string, 0, len(unique))
	for path := range unique {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	blobs := batchBlobs(ctx, bound.dir, bound.head, ordered)
	return tree{
		tracked: func(path string) bool { _, ok := blobs[path]; return ok },
		blob:    func(path string) (string, bool) { oid, ok := blobs[path]; return oid, ok },
	}
}

// checkoutRows reports every operator checkout and how many repositories it
// bound (EEP-V1-003); it is present only when a checkout was supplied.
func (b *bindings) checkoutRows(used map[string]int) []any {
	rows := make([]any, 0, len(b.order))
	for _, entry := range b.order {
		resolved := b.checkouts[entry.ID]
		row := map[string]any{"id": entry.ID, "source": entry.Source, "state": resolved.state, "reason": resolved.reason, "bound": used[entry.ID]}
		if resolved.head != "" {
			row["revision"] = resolved.head
		}
		rows = append(rows, row)
	}
	return rows
}

// ParseCheckout splits one `ID=DIR` argument (EEP-V1-003).
func ParseCheckout(value string) (Checkout, error) {
	id, dir, found := strings.Cut(value, "=")
	if !found || dir == "" || checkIdentifier("repository id", id) != nil {
		return Checkout{}, fmt.Errorf("expected ID=DIR with an identifier ID of at most %d bytes", maxIdentifier)
	}
	return Checkout{ID: id, Source: dir}, nil
}
