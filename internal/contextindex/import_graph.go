package contextindex

import (
	"context"
	"encoding/json"
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

// ImportGraphRecord is one independently revision-bound graph-imports/1 record.
// An edge is a syntax observation, never a claim of runtime dependency closure.
type ImportGraphRecord struct {
	Schema   string         `json:"schema"`
	Revision string         `json:"revision"`
	Type     string         `json:"type"`
	From     string         `json:"from,omitempty"`
	To       string         `json:"to,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
	Unknown  map[string]any `json:"unknown,omitempty"`
}

// ImportGraphKinds validates the closed, optional comma-separated edge filter.
// Uncertainty records are never filtered out.
func ImportGraphKinds(value string) (map[string]bool, error) {
	kinds := map[string]bool{}
	if value == "" {
		return kinds, nil
	}
	for _, kind := range strings.Split(value, ",") {
		if !slices.Contains(importGraphKindNames, kind) || kinds[kind] {
			return nil, &Error{Code: "graph-invalid-kinds", Message: "kinds must be unique " + strings.Join(importGraphKindNames, ",") + " values"}
		}
		kinds[kind] = true
	}
	return kinds, nil
}

// BuildImportGraph pins an existing full commit and reads only Git objects:
// neither HEAD nor any worktree file, so an unborn or moved HEAD and a dirty
// checkout cannot change or block the graph (NGI-V0-304). It writes no index
// snapshot.
func BuildImportGraph(ctx context.Context, root, revision string) (*Index, error) {
	ctx, cancel := withGitDeadline(ctx)
	defer cancel()
	raw, err := git(ctx, root, maxIdentityBytes, nil, "rev-parse", "--show-object-format")
	if err != nil {
		return nil, err
	}
	format := strings.TrimSuffix(string(raw), "\n")
	if format != "sha1" && format != "sha256" {
		return nil, &Error{Message: "unsupported Git object format: " + format}
	}
	if !validObjectID(revision, format) {
		return nil, &Error{Code: "graph-invalid-revision", Message: "graph imports requires a full immutable commit ID"}
	}
	kind, err := git(ctx, root, maxIdentityBytes, nil, "cat-file", "-t", revision)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &Error{Code: "graph-unknown-revision", Message: "graph revision is not available"}
	}
	if string(kind) != "commit\n" {
		return nil, &Error{Code: "graph-invalid-revision", Message: "graph revision must identify a commit"}
	}
	tree, err := git(ctx, root, maxIdentityBytes, nil, "rev-parse", revision+"^{tree}")
	if err != nil {
		return nil, err
	}
	// History completeness is not observed, so it is never claimed.
	identity := repositoryIdentity{objectFormat: format, commitRevision: revision, treeRevision: strings.TrimSpace(string(tree)), historyCut: true}
	if !validObjectID(identity.treeRevision, format) {
		return nil, &Error{Message: "Git returned an invalid object identity"}
	}
	index, err := buildEvidence(withObjectOnlyPins(ctx), root, identity, nil, "")
	if err != nil {
		return nil, err
	}
	index.compile()
	return index, nil
}

// objectOnlyPinsKey marks a build whose sources come only from Git objects,
// never from a worktree copy that hashes to the same blob.
type objectOnlyPinsKey struct{}

func withObjectOnlyPins(ctx context.Context) context.Context {
	return context.WithValue(ctx, objectOnlyPinsKey{}, true)
}

func objectOnlyPins(ctx context.Context) bool {
	only, _ := ctx.Value(objectOnlyPinsKey{}).(bool)
	return only
}

// WalkImportGraph streams in target-path, importer-path, specifier order. It
// deliberately shares impact's resolvers and statement anchors; registry
// consumers are not relabelled as imports, and Angular resource and Zeitwerk
// constant edges keep their own kinds and authority (NGI-V0-306, NGI-V0-307).
func WalkImportGraph(ctx context.Context, index *Index, kinds map[string]bool, emit func(ImportGraphRecord) error) error {
	write := func(record ImportGraphRecord) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		record.Schema, record.Revision = "graph-imports/1", index.CommitRevision
		return emit(record)
	}
	if err := write(ImportGraphRecord{Type: "scope", Evidence: map[string]any{"tree": index.Revision, "scope": "indexed-syntax-only"}}); err != nil {
		return err
	}
	unknown := func(row map[string]any) error { return write(ImportGraphRecord{Type: "uncertainty", Unknown: row}) }
	web := buildWebImportGraph(index)
	imports := graphImportLookup(index)
	registries := loadRegistryDialect(index)
	ruby := loadRubyAutoload(index)
	extras := buildGraphExtras(index, ruby)
	typeKinds := map[string]map[string]bool{}
	for _, target := range sortedSourcePaths(index) {
		if err := ctx.Err(); err != nil {
			return err
		}
		importers := imports(target)
		if strings.HasSuffix(target, ".go") && (index.Module == "" || !strings.Contains(index.Module, "/")) {
			importers = nil
			if err := unknown(map[string]any{"path": target, "language": "go", "code": "go-module-unavailable"}); err != nil {
				return err
			}
		}
		importers = sortImporters(append(importers, web.importers[target]...))
		others := sortGraphEdges(extras.edges[target])
		var previous importer
		for n, imp := range importers {
			// Convention and resource edges merge in importer, specifier order;
			// an import precedes them on a tie.
			for len(others) > 0 && (others[0].from < imp.path || others[0].from == imp.path && others[0].specifier < imp.imported) {
				if err := writeGraphEdge(write, kinds, target, others[0]); err != nil {
					return err
				}
				others = others[1:]
			}
			if n > 0 && imp == previous {
				continue
			}
			previous = imp
			source := index.Sources[imp.path]
			line, ok := importEvidenceLine(source, imp.imported)
			if !ok {
				if err := unknown(map[string]any{"path": imp.path, "code": "import-evidence-unavailable", "target": target}); err != nil {
					return err
				}
				continue
			}
			kind := "import"
			if webTypeOnlyImport(source, imp.imported, typeKinds) {
				kind = "type-import"
			}
			if len(kinds) > 0 && !kinds[kind] {
				continue
			}
			if err := write(ImportGraphRecord{Type: "edge", From: imp.path, To: target, Kind: kind,
				Evidence: map[string]any{"path": imp.path, "line": line, "blob_hash": source.BlobHash, "specifier": imp.imported, "authority": "syntax"}}); err != nil {
				return err
			}
		}
		for _, edge := range others {
			if err := writeGraphEdge(write, kinds, target, edge); err != nil {
				return err
			}
		}
		extra := map[string][]any{}
		language := NonGoImpactLanguage(target)
		if language != "" && registries.declared() {
			_, extra[target] = registries.impact(index, target, importers)
		}
		if language == "ruby" {
			_, rows := ruby.impact(index, target)
			extra[target] = append(extra[target], rows...)
			if err := unknown(map[string]any{"path": target, "language": language, "code": "ruby-autoload-not-import"}); err != nil {
				return err
			}
		}
		for _, row := range nonGoImpactUnknowns(index, []string{target}, web.unresolved, extra, ruby) {
			if err := unknown(row.(map[string]any)); err != nil {
				return err
			}
		}
		if language != "" && language != "ruby" {
			for _, code := range []string{"framework-resources-unresolved", "string-keyed-di-unresolved"} {
				if err := unknown(map[string]any{"path": target, "language": language, "code": code}); err != nil {
					return err
				}
			}
		}
		for _, row := range extras.sortedUnknowns(target) {
			if err := unknown(row); err != nil {
				return err
			}
		}
		if language == "" && !ImpactRuleNamed(target) {
			// Admission makes a path searchable, not resolvable: every indexed
			// path is admitted by suffix, and any without a rule says so.
			if err := unknown(map[string]any{"path": target, "language": strings.TrimPrefix(path.Ext(target), "."), "code": "reverse-import-rule-unavailable"}); err != nil {
				return err
			}
		}
	}
	// Index omissions remain visible, without presenting excluded files as an
	// empty dependency graph. Sort the records independently of extractor order.
	rows := make([]string, 0, len(index.Exclusions)+len(index.Unparsed))
	for _, exclusion := range index.Exclusions {
		data, _ := json.Marshal(map[string]any{"path": exclusion.Path, "code": "index-excluded", "reason": exclusion.Reason})
		rows = append(rows, string(data))
	}
	for _, source := range index.Unparsed {
		data, _ := json.Marshal(map[string]any{"path": source.Path, "code": source.Reason, "facts": source.Facts})
		rows = append(rows, string(data))
	}
	for _, data := range slices.Compact(slices.Sorted(slices.Values(rows))) {
		var row map[string]any
		_ = json.Unmarshal([]byte(data), &row)
		if err := unknown(row); err != nil {
			return err
		}
	}
	return nil
}

// graphImportLookup indexes specifiers once rather than rescanning all imports
// for every target. It applies the same three rules as reverseImporters, using
// their shared spelling helpers, including cross-language and self-edge behavior.
func graphImportLookup(index *Index) func(string) []importer {
	bySpecifier := map[string][]importer{}
	byWebStem := map[string][]importer{}
	profile := projectprofile.ByID(index.ProfileID)
	for from, imports := range index.Imports {
		for specifier := range imports {
			imp := importer{from, specifier}
			bySpecifier[specifier] = append(bySpecifier[specifier], imp)
			if webSpecifierAdmitted(specifier, profile) {
				stem := webImportTarget(from, specifier, profile)
				byWebStem[stem] = append(byWebStem[stem], imp)
				stem = strings.TrimRight(stem, "/") + "/index"
				byWebStem[stem] = append(byWebStem[stem], imp)
			}
		}
	}
	return func(target string) []importer {
		found := []importer{}
		add := func(specifier string) {
			for _, imp := range bySpecifier[specifier] {
				if imp.path != target {
					found = append(found, imp)
				}
			}
		}
		switch {
		case strings.HasSuffix(target, goImpactSuffix):
			add(goPackageImportPath(index.Module, target))
		case strings.HasSuffix(target, pythonImpactSuffix):
			for candidate := range PythonImportCandidates(target) {
				add(candidate)
			}
		case webSuffixes[strings.ToLower(pythonPathSuffix(target))]:
			found = append(found, byWebStem[pythonWithoutSuffix(target)]...)
		}
		return found
	}
}
