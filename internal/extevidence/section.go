package extevidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// Provider states (EEP-V0-005).
const (
	StateLoaded      = "loaded"
	StateUnavailable = "unavailable"
	StateInvalid     = "invalid"
	StateUnsupported = "unsupported"
)

// UntrustedTextFields names the provider-authored free text in the section (EEP-V0-013).
var UntrustedTextFields = []string{
	"external.results[].summary", "external.results[].relation.rule", "external.results[].relation.reference",
	"external.downstream[].summary", "external.downstream[].relation.rule", "external.downstream[].relation.reference",
	"external.verification[].summary", "external.verification[].relation.rule", "external.verification[].relation.reference",
}

type provider struct {
	source, sha256, state, reason, freshness string
	record                                   Record
	record1                                  *Record1
	view                                     *view
}

// Section reads every selected record and returns the `external` member of
// the impact receipt. It never fails: every problem is a structured entry.
// checkouts bind V1 repositories to local directories (EEP-V1-003).
func Section(ctx context.Context, index *contextindex.Index, sources []string, checkouts []Checkout, changedPaths []string, limit int) map[string]any {
	root := indexRoot(index)
	providers, repository, bound := loadAll(ctx, root, sources, checkouts)
	changed := make(map[string]struct{}, len(changedPaths))
	for _, path := range changedPaths {
		changed[path] = struct{}{}
	}
	var merged composition
	for _, entry := range providers {
		if entry.state != StateLoaded {
			continue
		}
		part := composeView(entry.viewOf(ctx, root, repository), changed)
		merged.results = append(merged.results, part.results...)
		merged.downstream = append(merged.downstream, part.downstream...)
		merged.verification = append(merged.verification, part.verification...)
		merged.paths = append(merged.paths, part.paths...)
		merged.unknowns = append(merged.unknowns, part.unknowns...)
	}
	section := assemble(providers, merged, limit)
	if hasPathProfile(providers) {
		addPathRelations(section, merged.paths, limit)
	}
	if bound != nil && len(checkouts) != 0 {
		section["checkouts"] = bound.checkoutRows(checkoutUse(providers))
	}
	return section
}

// rootRepository is the repository the changed paths belong to: its
// directory, the commit records are judged against, and how to answer tracked
// and blob questions there. impact reads it from the index, affected from HEAD.
// A HEAD tree knows only the paths it was asked about, so every V0 path
// endpoint is requested (allPaths), not only the pinned ones.
type rootRepository struct {
	dir, revision string
	allPaths      bool
	tree          func(ctx context.Context, paths []string) tree
	// changed lists the root paths a V2 directory scope may hold, so the
	// tree can answer for them (EEP-V2-012).
	changed []string
	// selecting marks an `affected` invocation, which needs declared or
	// observed evidence rather than every kind the record uses (EEP-TR-013).
	selecting bool
}

// unsupported is the Core-authored reason a declared capability set omits
// what this invocation requires, or empty when it declares enough or nothing
// (EEP-TR-013). Only the first missing capability is named.
func (root rootRepository) unsupported(id, schema string, declared *Capabilities, used []string) string {
	if declared == nil {
		return ""
	}
	if declared.Schemas != nil && !slices.Contains(declared.Schemas, schema) {
		return fmt.Sprintf("provider %s declares capabilities without schema %s", id, schema)
	}
	if declared.EvidenceKinds == nil {
		return ""
	}
	if root.selecting {
		qualifying := []string{EvidenceDeclared, EvidenceObserved}
		if slices.ContainsFunc(qualifying, func(kind string) bool { return slices.Contains(declared.EvidenceKinds, kind) }) {
			return ""
		}
		return fmt.Sprintf("provider %s declares capabilities without evidence kind %s or %s", id, EvidenceDeclared, EvidenceObserved)
	}
	for _, kind := range used {
		if !slices.Contains(declared.EvidenceKinds, kind) {
			return fmt.Sprintf("provider %s declares capabilities without evidence kind %s", id, kind)
		}
	}
	return ""
}

func indexRoot(index *contextindex.Index) rootRepository {
	return rootRepository{dir: index.Root, revision: index.CommitRevision, tree: func(ctx context.Context, paths []string) tree {
		return rootTree(ctx, index, paths)
	}}
}

// headRoot answers at a commit without an index: a path is tracked when it
// names a blob there, exactly as for a bound checkout.
func headRoot(dir, revision string) rootRepository {
	return rootRepository{dir: dir, revision: revision, allPaths: true, tree: func(ctx context.Context, paths []string) tree {
		return checkoutTree(ctx, checkout{dir: dir, head: revision}, paths)
	}}
}

// loadAll decodes every source and binds every V1 record once.
func loadAll(ctx context.Context, root rootRepository, sources []string, checkouts []Checkout) ([]provider, tree, *bindings) {
	providers := make([]provider, 0, len(sources))
	for _, source := range sources {
		providers = append(providers, load(ctx, root, source))
	}
	repository := repositoryTree(ctx, root, providers)
	return providers, repository, bindV1(ctx, root, providers, checkouts)
}

// bindV1 resolves bindings only when a V1 record loaded or a checkout was
// named, so a V0-only run makes no additional Git call.
func bindV1(ctx context.Context, root rootRepository, providers []provider, checkouts []Checkout) *bindings {
	needed := len(checkouts) != 0
	for _, entry := range providers {
		needed = needed || entry.record1 != nil
	}
	if !needed {
		return nil
	}
	bound := resolveBindings(ctx, root, checkouts)
	for position := range providers {
		if providers[position].record1 != nil {
			providers[position].view = view1(ctx, root, *providers[position].record1, bound)
		}
	}
	return bound
}

func (entry provider) viewOf(ctx context.Context, root rootRepository, repository tree) *view {
	if entry.view != nil {
		return entry.view
	}
	return viewOf(ctx, root, entry, repository)
}

func checkoutUse(providers []provider) map[string]int {
	used := make(map[string]int)
	for _, entry := range providers {
		if entry.view == nil {
			continue
		}
		for id, state := range entry.view.repositories {
			if state.binding == BindingCheckout {
				used[id]++
			}
		}
	}
	return used
}

func load(ctx context.Context, root rootRepository, source string) provider {
	if encoded, ok := strings.CutPrefix(source, mcpSourcePrefix); ok {
		return loadMCP(ctx, root, encoded)
	}
	if argv, isCommand := commandArgv(source); isCommand {
		return loadCommand(ctx, root, argv)
	}
	entry := provider{source: source, state: StateUnavailable}
	resolved := source
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root.dir, filepath.FromSlash(source))
	}
	info, err := os.Stat(resolved)
	if err != nil {
		entry.reason = "cannot read record: " + describe(err)
		return entry
	}
	if info.Size() > MaxRecordBytes {
		entry.state, entry.reason = StateInvalid, fmt.Sprintf("record exceeds %d bytes", MaxRecordBytes)
		return entry
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		entry.reason = "cannot read record: " + describe(err)
		return entry
	}
	return decodeRecord(ctx, root, entry, data)
}

// decodeRecord is the one decode every transport shares: the same bytes give
// the same state, reason, record, and freshness whatever carried them
// (EEP-TR-005).
func decodeRecord(ctx context.Context, root rootRepository, entry provider, data []byte) provider {
	digest := sha256.Sum256(data)
	entry.sha256 = hex.EncodeToString(digest[:])
	if schema := declaredSchema(data); schema == Schema1 || schema == Schema2 {
		record, err := Decode1(data)
		if err != nil {
			entry.state, entry.reason = StateInvalid, err.Error()
			return entry
		}
		used := make([]string, 0, len(record.Relations))
		for _, relation := range record.Relations {
			used = append(used, relation.Evidence)
		}
		if reason := root.unsupported(record.Provider.ID, record.Schema, record.Capabilities, used); reason != "" {
			entry.state, entry.reason = StateUnsupported, reason
			return entry
		}
		entry.record1, entry.state = &record, StateLoaded
		entry.reason = "record decoded; freshness by Git ancestry per declared repository"
		return entry
	}
	record, err := Decode(data)
	if err != nil {
		entry.state, entry.reason = StateInvalid, err.Error()
		return entry
	}
	used := make([]string, 0, len(record.Relations))
	for _, relation := range record.Relations {
		used = append(used, relation.Evidence)
	}
	if reason := root.unsupported(record.Provider.ID, record.Schema, record.Capabilities, used); reason != "" {
		entry.state, entry.reason = StateUnsupported, reason
		return entry
	}
	entry.record, entry.state = record, StateLoaded
	entry.freshness = Freshness(ctx, root.dir, root.revision, record.Repository.Revision)
	entry.reason = "record decoded; freshness by Git ancestry against " + root.revision
	return entry
}

// declaredSchema reads only the schema member so a V1 record takes the V1
// decoder; anything else keeps the unchanged V0 path (EEP-V1-011).
func declaredSchema(data []byte) string {
	var probe struct {
		Schema string `json:"schema"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return ""
	}
	return probe.Schema
}

// describe strips the file path from an OS error so the reason stays bounded
// and repeats only what the caller already passed.
func describe(err error) string {
	if pathError, ok := err.(*os.PathError); ok {
		return pathError.Err.Error()
	}
	return err.Error()
}

// repositoryTree answers tracked and blob questions for every pinned path the
// loaded records name, resolving blobs the index did not read in one Git call.
func repositoryTree(ctx context.Context, root rootRepository, providers []provider) tree {
	var pinned []string
	for _, entry := range providers {
		if entry.state != StateLoaded {
			continue
		}
		for _, relation := range entry.record.Relations {
			for _, raw := range []string{relation.From, relation.To} {
				path, isPath := strings.CutPrefix(raw, "path:")
				if !isPath || (relation.Blob == "" && !root.allPaths) || checkPath(path) != "" {
					continue
				}
				pinned = append(pinned, path)
			}
		}
	}
	return root.tree(ctx, pinned)
}

func batchBlobs(ctx context.Context, root, revision string, paths []string) map[string]string {
	resolved := make(map[string]string, len(paths))
	if len(paths) == 0 {
		return resolved
	}
	var request strings.Builder
	for _, path := range paths {
		request.WriteString(revision + ":" + path + "\n")
	}
	output, err := runGit(ctx, root, []byte(request.String()), "cat-file", "--batch-check")
	if err != nil {
		return resolved
	}
	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	for position, line := range lines {
		fields := strings.Fields(line)
		if position < len(paths) && len(fields) == 3 && fields[1] == "blob" {
			resolved[paths[position]] = fields[0]
		}
	}
	return resolved
}

func assemble(providers []provider, merged composition, limit int) map[string]any {
	rows := providerRows(providers)
	sortItems(merged.results)
	sortItems(merged.downstream)
	sortItems(merged.verification)
	sort.SliceStable(merged.unknowns, func(i, j int) bool {
		if merged.unknowns[i].provider != merged.unknowns[j].provider {
			return merged.unknowns[i].provider < merged.unknowns[j].provider
		}
		return lessRelation(merged.unknowns[i].relation, merged.unknowns[j].relation)
	})
	results, omittedResults := boundItems(merged.results, limit)
	downstream, omittedDownstream := boundItems(merged.downstream, limit)
	verification, omittedVerification := boundItems(merged.verification, limit)
	unknowns := make([]any, 0, min(len(merged.unknowns), limit))
	for _, entry := range merged.unknowns[:min(len(merged.unknowns), limit)] {
		unknowns = append(unknowns, entry.toMap())
	}
	fields := make([]any, 0, len(UntrustedTextFields))
	for _, field := range UntrustedTextFields {
		fields = append(fields, field)
	}
	return map[string]any{
		"schema_version":        1,
		"authority":             Authority,
		"providers":             rows,
		"results":               results,
		"downstream":            downstream,
		"verification":          verification,
		"unknowns":              unknowns,
		"omitted":               map[string]any{"results": omittedResults, "downstream": omittedDownstream, "verification": omittedVerification, "unknowns": max(len(merged.unknowns)-limit, 0)},
		"untrusted_text_fields": fields,
	}
}

// providerRows reports every selected record, loaded or not (EEP-V0-005).
func providerRows(providers []provider) []any {
	rows := make([]any, 0, len(providers))
	for _, entry := range providers {
		if entry.view != nil {
			rows = append(rows, entry.row1())
			continue
		}
		rows = append(rows, map[string]any{
			"source": entry.source, "sha256": entry.sha256, "state": entry.state, "reason": entry.reason,
			"id": entry.record.Provider.ID, "revision": entry.record.Provider.Revision,
			"repository_revision": entry.record.Repository.Revision, "freshness": entry.freshness,
			"entities": len(entry.record.Entities), "relations": len(entry.record.Relations),
		})
	}
	return rows
}

// row1 is a loaded V1 provider row: freshness is per repository, never one
// value for the record (EEP-V1-005).
func (entry provider) row1() map[string]any {
	record := entry.record1
	repositories := make([]any, 0, len(record.Repositories))
	for _, declared := range record.Repositories {
		state := entry.view.repositories[declared.ID]
		row := map[string]any{"id": declared.ID, "revision": declared.Revision, "identity": state.identity, "binding": state.binding, "freshness": state.freshness}
		optional := map[string]string{"origin": declared.Origin, "remote": declared.Remote, "tree": declared.Tree, "role": declared.Role, "captured_revision": state.captured, "checkout": state.from}
		for key, value := range optional {
			if value != "" {
				row[key] = value
			}
		}
		repositories = append(repositories, row)
	}
	return map[string]any{
		"source": entry.source, "sha256": entry.sha256, "state": entry.state, "reason": entry.reason,
		"schema": record.Schema, "id": record.Provider.ID, "revision": record.Provider.Revision,
		"root_repository": entry.view.primary, "repositories": repositories,
		"entities": len(record.Entities), "relations": len(record.Relations),
	}
}

// hasPathProfile reports whether any V2 record loaded; only then does the
// section gain path_relations, so V0 and V1 bytes are unchanged (EEP-V2-005).
func hasPathProfile(providers []provider) bool {
	for _, entry := range providers {
		if entry.view != nil && entry.view.pathToPath {
			return true
		}
	}
	return false
}

// addPathRelations adds the bounded, sorted path-to-path items, their omission
// count, and their untrusted text fields (EEP-V2-004, EEP-V2-005).
func addPathRelations(section map[string]any, paths []item, limit int) {
	sort.Slice(paths, func(i, j int) bool { return pathItemKey(paths[i]) < pathItemKey(paths[j]) })
	kept, omitted := boundItems(paths, limit)
	section["path_relations"] = kept
	section["omitted"].(map[string]any)["path_relations"] = omitted
	section["untrusted_text_fields"] = append(section["untrusted_text_fields"].([]any), "external.path_relations[].relation.rule", "external.path_relations[].relation.reference")
}

// pathItemKey is a total order over path-to-path items: every relation member
// and the provider, so record order never reaches the output (EEP-V2-005).
func pathItemKey(entry item) string {
	relation := entry.link.structured
	return strings.Join([]string{
		endpointKey(relation.From), endpointKey(relation.To), relation.Type, relation.Evidence,
		relation.Rule, relation.Reference, relation.From.Blob, relation.To.Blob, entry.provider,
	}, "\x00")
}

func boundItems(items []item, limit int) ([]any, int) {
	kept := min(len(items), max(limit, 0))
	out := make([]any, 0, kept)
	for _, entry := range items[:kept] {
		out = append(out, entry.toMap())
	}
	return out, len(items) - kept
}
