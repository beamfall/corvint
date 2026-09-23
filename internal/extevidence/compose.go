package extevidence

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Verification states for a path endpoint (EEP-V0-010).
const (
	VerificationVerified    = "verified"
	VerificationStale       = "stale"
	VerificationMissing     = "missing"
	VerificationDeleted     = "deleted"
	VerificationUnsupported = "unsupported"
	VerificationNotVerified = "not-verified"
)

// revisionKnown lists the freshness states under which the provider's
// declared revision is a commit the repository holds, so its tree can answer
// whether a path existed there (EEP-V0-010 `deleted`).
var revisionKnown = map[string]struct{}{FreshnessRepositoryAhead: {}, FreshnessProviderAhead: {}, FreshnessUnrelatedHistory: {}}

// Unknown states (EEP-V0-006, EEP-V0-007).
const (
	unknownUnresolved  = "unresolved"
	unknownExcluded    = "excluded"
	unknownUnsupported = "unsupported"
)

var verificationTypes = map[string]struct{}{"verifies": {}, "covers": {}, "asserts": {}}

var evidenceKinds = map[string]struct{}{EvidenceDeclared: {}, EvidenceObserved: {}, EvidenceInferred: {}, EvidenceGenerated: {}}

// tree answers the two repository questions composition asks about a path.
type tree struct {
	tracked func(path string) bool
	blob    func(path string) (string, bool)
}

// endpoint is a resolved path or entity. repository is empty in V0, where
// every path belongs to the one root repository.
type endpoint struct {
	repository string
	path       string
	entity     string
	blob       string
}

func (e endpoint) isPath() bool { return e.path != "" }

// isScope reports a directory scope: a V2 path ending in "/" (EEP-V2-012).
func (e endpoint) isScope() bool { return strings.HasSuffix(e.path, "/") }

// contains reports whether a directory scope holds other strictly below it in
// the same repository.
func (e endpoint) contains(other endpoint) bool {
	return e.isScope() && other.repository == e.repository && other.path != e.path && strings.HasPrefix(other.path, e.path)
}

// link is a resolved relation. For a V1 record, relation carries the ordering
// keys and structured holds the record's own endpoints for output.
type link struct {
	relation   Relation
	structured *Relation1
	from, to   endpoint
}

type item struct {
	provider    string
	entity      Entity
	path        string
	link        link
	state       string
	reason      string
	annotations map[string]any
}

type unknown struct {
	provider   string
	relation   Relation
	structured *Relation1
	state      string
	reason     string
}

// view is one loaded record in the form composition reads. primary names the
// repository the changed paths belong to; repositories is nil for V0.
type view struct {
	provider     string
	entities     map[string]Entity
	links        []link
	unknowns     []unknown
	primary      string
	trees        map[string]tree
	repositories map[string]*repositoryState
	// deleted holds, per repository, the path endpoints untracked at the
	// captured revision but tracked at the provider's declared revision.
	deleted map[string]map[string]struct{}
	// pathToPath is set only for a V2 record, the one schema that composes a
	// relation between two paths (EEP-V2-001); revision is its provider revision.
	pathToPath bool
	revision   string
}

type composition struct {
	results, downstream, verification, paths []item
	unknowns                                 []unknown
}

// compose applies EEP-V0-006, -007, -010, and -011 to one loaded record.
func compose(ctx context.Context, root rootRepository, entry provider, changed map[string]struct{}, repository tree) composition {
	return composeView(viewOf(ctx, root, entry, repository), changed)
}

func viewOf(ctx context.Context, root rootRepository, entry provider, repository tree) *view {
	record := entry.record
	v := &view{provider: record.Provider.ID, entities: entityMap(record.Entities), trees: map[string]tree{"": repository}}
	for _, relation := range record.Relations {
		resolved, failure := resolve(record, v.entities, relation)
		if failure != nil {
			v.unknowns = append(v.unknowns, *failure)
			continue
		}
		v.links = append(v.links, resolved)
	}
	gone := deletedPaths(ctx, root.dir, entry.freshness, record.Repository.Revision, repository, v.pathsIn(""))
	v.deleted = map[string]map[string]struct{}{"": gone}
	return v
}

// pathsIn lists every path endpoint the resolved relations place in one
// repository, sorted and without duplicates.
func (v *view) pathsIn(repository string) []string {
	unique := make(map[string]struct{})
	for _, candidate := range v.links {
		for _, side := range []endpoint{candidate.from, candidate.to} {
			if side.isPath() && side.repository == repository {
				unique[side.path] = struct{}{}
			}
		}
	}
	paths := make([]string, 0, len(unique))
	for path := range unique {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// deletedPaths answers which of paths are untracked at the captured revision
// yet tracked at the provider's declared revision. It asks Git once, and only
// when that revision is a known commit and some path is untracked
// (EEP-V0-010 `deleted`).
func deletedPaths(ctx context.Context, dir, freshness, revision string, current tree, paths []string) map[string]struct{} {
	if _, known := revisionKnown[freshness]; !known {
		return nil
	}
	var untracked []string
	for _, path := range paths {
		if !current.tracked(path) {
			untracked = append(untracked, path)
		}
	}
	gone := make(map[string]struct{}, len(untracked))
	for path := range batchBlobs(ctx, dir, revision, untracked) {
		gone[path] = struct{}{}
	}
	return gone
}

func entityMap(list []Entity) map[string]Entity {
	entities := make(map[string]Entity, len(list))
	for _, entity := range list {
		entities[entity.ID] = entity
	}
	return entities
}

func composeView(v *view, changed map[string]struct{}) composition {
	out := composition{unknowns: v.unknowns}
	out.results = directResults(v, changed)
	listed := entitySet(out.results)
	out.downstream = downstreamOf(v, listed)
	for id := range entitySet(out.downstream) {
		listed[id] = struct{}{}
	}
	out.verification = verificationOf(v, changed, listed)
	out.paths = pathRelationsOf(v, changed)
	sortItems(out.results)
	sortItems(out.downstream)
	sortItems(out.verification)
	sort.Slice(out.unknowns, func(i, j int) bool { return lessRelation(out.unknowns[i].relation, out.unknowns[j].relation) })
	return out
}

func resolve(record Record, entities map[string]Entity, relation Relation) (link, *unknown) {
	if _, known := evidenceKinds[relation.Evidence]; !known {
		return link{}, &unknown{
			provider: record.Provider.ID, relation: relation, state: unknownExcluded,
			reason: fmt.Sprintf("evidence kind %q is not declared, observed, inferred, or generated", relation.Evidence),
		}
	}
	from, reason := parseEndpoint(record, entities, relation.From, relation)
	if reason != "" {
		return link{}, &unknown{provider: record.Provider.ID, relation: relation, state: unknownUnresolved, reason: "from: " + reason}
	}
	to, reason := parseEndpoint(record, entities, relation.To, relation)
	if reason != "" {
		return link{}, &unknown{provider: record.Provider.ID, relation: relation, state: unknownUnresolved, reason: "to: " + reason}
	}
	return link{relation: relation, from: from, to: to}, nil
}

func parseEndpoint(record Record, entities map[string]Entity, raw string, relation Relation) (endpoint, string) {
	if path, isPath := strings.CutPrefix(raw, "path:"); isPath {
		return endpoint{path: path, blob: relation.Blob}, checkPath(path)
	}
	provider, id, found := strings.Cut(raw, ":")
	if !found {
		return endpoint{}, "endpoint has neither a path: prefix nor a provider prefix"
	}
	if provider != record.Provider.ID {
		return endpoint{}, fmt.Sprintf("endpoint names provider %q; V0 resolves only the record's own provider", provider)
	}
	if _, declared := entities[id]; !declared {
		return endpoint{}, fmt.Sprintf("entity %q is not declared by the record", id)
	}
	return endpoint{entity: id}, ""
}

func checkPath(path string) string {
	if path == "" || len(path) > maxPath {
		return fmt.Sprintf("path must be non-empty and at most %d bytes", maxPath)
	}
	if strings.HasPrefix(path, "/") {
		return "path must be repository-relative"
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return "path must not contain a parent-traversal segment"
		}
	}
	return ""
}

// missingState separates a path the provider saw and the repository since
// dropped from one that is simply absent at the captured revision.
func (v *view) missingState(target endpoint) string {
	if _, gone := v.deleted[target.repository][target.path]; gone {
		return VerificationDeleted
	}
	return VerificationMissing
}

// verify checks one path endpoint in its own repository; a repository that
// is not bound to a checkout cannot be verified (EEP-V1-007).
func (v *view) verify(target endpoint) string {
	repository, bound := v.trees[target.repository]
	if !bound {
		return VerificationNotVerified
	}
	path, pinned := target.path, target.blob
	if !repository.tracked(path) {
		return v.missingState(target)
	}
	if pinned == "" {
		return VerificationVerified
	}
	actual, known := repository.blob(path)
	if known && actual == pinned {
		return VerificationVerified
	}
	return VerificationStale
}

// pathAndEntity splits a link into its path side and entity side when it has
// exactly one of each.
func pathAndEntity(candidate link) (path endpoint, entity string, ok bool) {
	if candidate.from.isPath() && !candidate.to.isPath() {
		return candidate.from, candidate.to.entity, true
	}
	if !candidate.from.isPath() && candidate.to.isPath() {
		return candidate.to, candidate.from.entity, true
	}
	return endpoint{}, "", false
}

// isChanged reports whether a path endpoint is a requested changed path;
// changed paths belong only to the primary repository.
func (v *view) isChanged(target endpoint, changed map[string]struct{}) bool {
	if target.repository != v.primary {
		return false
	}
	_, isChanged := changed[target.path]
	return isChanged
}

func directResults(v *view, changed map[string]struct{}) []item {
	var results []item
	for _, candidate := range v.links {
		path, entity, ok := pathAndEntity(candidate)
		if !ok {
			continue
		}
		if !v.isChanged(path, changed) {
			continue
		}
		results = append(results, item{
			provider: v.provider, entity: v.entities[entity], path: path.path, link: candidate,
			state:       v.verify(path),
			reason:      fmt.Sprintf("changed path %s joined by %s relation %s", v.label(path), candidate.relation.Evidence, candidate.relation.Type),
			annotations: v.annotate(candidate, path),
		})
	}
	return results
}

func downstreamOf(v *view, results map[string]struct{}) []item {
	var downstream []item
	for _, candidate := range v.links {
		if candidate.from.isPath() || candidate.to.isPath() {
			continue
		}
		origin, next := candidate.from.entity, candidate.to.entity
		if _, isResult := results[origin]; !isResult {
			origin, next = next, origin
		}
		if _, isResult := results[origin]; !isResult {
			continue
		}
		if _, isResult := results[next]; isResult {
			continue
		}
		downstream = append(downstream, item{
			provider: v.provider, entity: v.entities[next], link: candidate, state: VerificationUnsupported,
			reason:      fmt.Sprintf("one %s relation %s from result entity %s:%s", candidate.relation.Evidence, candidate.relation.Type, v.provider, origin),
			annotations: v.annotate(candidate, endpoint{}),
		})
	}
	return downstream
}

func verificationOf(v *view, changed, listed map[string]struct{}) []item {
	var verification []item
	for _, candidate := range v.links {
		if _, verifying := verificationTypes[candidate.relation.Type]; !verifying {
			continue
		}
		path, entity, ok := pathAndEntity(candidate)
		if !ok {
			continue
		}
		if v.isChanged(path, changed) {
			continue
		}
		if _, isListed := listed[entity]; !isListed {
			continue
		}
		verification = append(verification, item{
			provider: v.provider, entity: v.entities[entity], path: path.path, link: candidate,
			state:       v.verify(path),
			reason:      fmt.Sprintf("%s relation %s from path %s to listed entity %s:%s", candidate.relation.Evidence, candidate.relation.Type, v.label(path), v.provider, entity),
			annotations: v.annotate(candidate, path),
		})
	}
	return verification
}

// pathRelationsOf lists every V2 path-to-path relation with a changed root
// path on either side (EEP-V2-004). The first changed side anchors the item.
func pathRelationsOf(v *view, changed map[string]struct{}) []item {
	var paths []item
	for _, candidate := range v.links {
		if !candidate.from.isPath() || !candidate.to.isPath() {
			continue
		}
		anchor, other := candidate.from, candidate.to
		if !v.isChanged(anchor, changed) {
			anchor, other = other, anchor
		}
		if !v.isChanged(anchor, changed) {
			continue
		}
		annotations := v.annotate(candidate, anchor)
		annotations["provider_revision"] = v.revision
		annotations["limitations"] = pathLimitations(v, candidate)
		paths = append(paths, item{
			provider: v.provider, path: anchor.path, link: candidate,
			state:       v.verify(anchor),
			reason:      fmt.Sprintf("changed path %s related by %s path-to-path relation %s to %s", v.label(anchor), candidate.relation.Evidence, candidate.relation.Type, v.label(other)),
			annotations: annotations,
		})
	}
	return paths
}

// pathLimitations names what a path-to-path item does not establish; a side
// read from a bound checkout was never checked for uncommitted changes.
func pathLimitations(v *view, candidate link) []any {
	out := []any{"not-coverage-proof", "not-executed"}
	for _, side := range []endpoint{candidate.from, candidate.to} {
		if v.repositories[side.repository].binding == BindingCheckout {
			return append(out, "checkout-worktree-not-inspected")
		}
	}
	return out
}

func entitySet(items []item) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, entry := range items {
		set[entry.entity.ID] = struct{}{}
	}
	return set
}

func sortItems(items []item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].provider != items[j].provider {
			return items[i].provider < items[j].provider
		}
		if items[i].entity.ID != items[j].entity.ID {
			return items[i].entity.ID < items[j].entity.ID
		}
		return lessRelation(items[i].link.relation, items[j].link.relation)
	})
}

func lessRelation(a, b Relation) bool {
	if a.From != b.From {
		return a.From < b.From
	}
	if a.To != b.To {
		return a.To < b.To
	}
	return a.Type < b.Type
}

func relationMap(relation Relation, structured *Relation1) map[string]any {
	if structured != nil {
		return map[string]any{
			"from": endpointMap(structured.From), "to": endpointMap(structured.To), "type": structured.Type,
			"evidence": structured.Evidence, "rule": structured.Rule, "reference": structured.Reference,
		}
	}
	out := map[string]any{
		"from": relation.From, "to": relation.To, "type": relation.Type,
		"evidence": relation.Evidence, "rule": relation.Rule, "reference": relation.Reference,
	}
	if relation.Blob != "" {
		out["blob"] = relation.Blob
	}
	return out
}

func (entry item) toMap() map[string]any {
	out := map[string]any{
		"authority":    Authority,
		"provider":     entry.provider,
		"relation":     relationMap(entry.link.relation, entry.link.structured),
		"verification": entry.state,
		"reason":       entry.reason,
	}
	// A path-to-path item names no entity (EEP-V2-004); every other item does.
	if entry.entity.ID != "" {
		out["entity"], out["kind"], out["summary"] = entry.provider+":"+entry.entity.ID, entry.entity.Kind, entry.entity.Summary
	}
	if entry.path != "" {
		out["path"] = entry.path
	}
	for key, value := range entry.annotations {
		out[key] = value
	}
	return out
}

func (entry unknown) toMap() map[string]any {
	relation := map[string]any{"from": entry.relation.From, "to": entry.relation.To, "type": entry.relation.Type, "evidence": entry.relation.Evidence}
	if entry.structured != nil {
		relation["from"], relation["to"] = endpointMap(entry.structured.From), endpointMap(entry.structured.To)
	}
	return map[string]any{
		"authority": Authority,
		"provider":  entry.provider,
		"relation":  relation,
		"state":     entry.state,
		"reason":    entry.reason,
	}
}
