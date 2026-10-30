package worktreeimpact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	Profile             = "corvint-working-tree-impact/0"
	maxPaths            = 100
	maxPathCharacters   = 1_024
	maxFileBytes        = 1_000_000
	maxAggregateBytes   = 64_000_000
	maxLimit            = 50
	maxUncertaintyItems = 3
)

type observedFile struct {
	path, packageName string
	imports           []string
	data              []byte
	mode              os.FileMode
	identity          platformFileIdentity
}

type candidate struct {
	score, order int
	key          string
	value        map[string]any
}

func Compile(ctx context.Context, index *contextindex.Index, paths []string, limit int) (map[string]any, error) {
	cleaned, err := validateRequest(index, paths, limit)
	if err != nil {
		return nil, err
	}
	root, err := openRepositoryRoot(index.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	observed := make([]observedFile, 0, len(cleaned))
	aggregateBytes := 0
	for _, value := range cleaned {
		file, err := readStableTarget(root, value, nil)
		if err != nil {
			return nil, err
		}
		aggregateBytes += len(file.data)
		if aggregateBytes > maxAggregateBytes {
			return nil, failure("working-tree-impact-too-large", "working-tree impact targets exceed 64000000-byte aggregate bound")
		}
		observed = append(observed, file)
	}
	if err := confirmIndexSnapshot(ctx, index, cleaned); err != nil {
		return nil, err
	}
	if err := confirmRepositoryRoot(root, index.Root); err != nil {
		return nil, err
	}
	return compileReceipt(index, observed, cleaned, limit)
}

func validateRequest(index *contextindex.Index, values []string, limit int) ([]string, error) {
	if index == nil {
		if err := validatePathSuffixes(values); err != nil {
			return nil, err
		}
		return nil, missingIndexRefusal()
	}
	if limit < 1 || limit > maxLimit {
		return nil, failure("invalid-working-tree-impact-limit", "working-tree impact limit must be an integer from 1 to 50")
	}
	if len(values) == 0 || len(values) > maxPaths {
		return nil, failure("invalid-working-tree-impact-paths", "working-tree impact requires 1 to 100 paths")
	}
	if err := validatePathSuffixes(values); err != nil {
		return nil, err
	}
	if index.Module == "" || !strings.Contains(index.Module, "/") {
		return nil, moduleRefusal(index.Module)
	}
	dirty := stringSet(index.DirtyPaths)
	tracked := make(map[string]struct{}, len(index.Sources)+len(index.Exclusions))
	for value := range index.Sources {
		tracked[value] = struct{}{}
	}
	for _, exclusion := range index.Exclusions {
		tracked[exclusion.Path] = struct{}{}
	}
	cleanedSet := make(map[string]struct{}, len(values))
	for _, value := range values {
		cleaned, err := cleanPath(value)
		if err != nil {
			return nil, err
		}
		if _, ok := tracked[cleaned]; ok {
			return nil, trackedPathRefusal(cleaned, index.Revision)
		}
		if _, ok := dirty[cleaned]; !ok {
			return nil, unobservedPathRefusal(cleaned, index.Revision)
		}
		cleanedSet[cleaned] = struct{}{}
	}
	cleaned := sortedKeys(cleanedSet)
	return cleaned, nil
}

func validatePathSuffixes(values []string) error {
	for _, value := range values {
		if !strings.HasSuffix(value, ".go") {
			return rawSuffixRefusal(value)
		}
	}
	return nil
}

func cleanPath(value string) (string, error) {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxPathCharacters {
		return "", failure("invalid-working-tree-impact-path", "working-tree impact path must be valid UTF-8 with 1 to 1024 characters")
	}
	if strings.ContainsAny(value, "\\\x00") || path.IsAbs(value) || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
		return "", failure("invalid-working-tree-impact-path", fmt.Sprintf("working-tree impact path must be normalized and repository-relative: %q", value))
	}
	if !strings.HasSuffix(value, ".go") {
		return "", cleanSuffixRefusal(value)
	}
	if path.Dir(value) == "." {
		return "", rootPathRefusal(value)
	}
	return value, nil
}

func openRepositoryRoot(name string) (*os.Root, error) {
	info, err := os.Lstat(name)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, failure("unsafe-working-tree-impact-root", "working-tree impact repository root must be a non-symlink directory")
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, failure("unsafe-working-tree-impact-root", "cannot open contained working-tree root")
	}
	opened, err := root.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
		root.Close()
		return nil, failure("stale-working-tree-impact", "working-tree repository root changed while opening")
	}
	return root, nil
}

func confirmRepositoryRoot(root *os.Root, name string) error {
	pinned, err := root.Stat(".")
	if err != nil || !pinned.IsDir() {
		return failure("stale-working-tree-impact", "cannot confirm pinned working-tree repository root")
	}
	current, err := os.Lstat(name)
	if err != nil || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(pinned, current) {
		return failure("stale-working-tree-impact", "working-tree repository root changed during impact")
	}
	return nil
}

func readStableTarget(root *os.Root, value string, afterFirst func()) (observedFile, error) {
	first, err := readTargetOnce(root, value)
	if err != nil {
		return observedFile{}, err
	}
	if afterFirst != nil {
		afterFirst()
	}
	second, err := readTargetOnce(root, value)
	if err != nil {
		return observedFile{}, err
	}
	if first.identity != second.identity || first.mode != second.mode || !bytes.Equal(first.data, second.data) {
		return observedFile{}, failure("stale-working-tree-impact", "working-tree target changed between bounded reads: "+value)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), value, first.data, parser.AllErrors)
	if err != nil {
		return observedFile{}, failure("invalid-working-tree-impact-source", "working-tree target is not valid Go syntax: "+value)
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return observedFile{}, failure("invalid-working-tree-impact-source", "working-tree target contains an invalid Go import: "+value)
		}
		imports = append(imports, imported)
	}
	sort.Strings(imports)
	first.packageName = parsed.Name.Name
	first.imports = imports
	return first, nil
}

func readTargetOnce(root *os.Root, value string) (observedFile, error) {
	leaf, err := validateComponents(root, value)
	if err != nil {
		return observedFile{}, err
	}
	file, err := openTarget(root, value)
	if err != nil {
		return observedFile{}, failure("unsafe-working-tree-impact-file", "cannot open contained working-tree target: "+value)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(leaf, opened) {
		return observedFile{}, failure("stale-working-tree-impact", "working-tree target identity changed while opening: "+value)
	}
	identity, err := identityFrom(opened)
	if err != nil {
		return observedFile{}, failure("unsafe-working-tree-impact-file", err.Error()+": "+value)
	}
	if opened.Size() > maxFileBytes {
		return observedFile{}, failure("working-tree-impact-too-large", "working-tree target exceeds 1000000-byte bound: "+value)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil || len(data) > maxFileBytes {
		return observedFile{}, failure("working-tree-impact-too-large", "working-tree target exceeds 1000000-byte bound: "+value)
	}
	afterOpen, err := file.Stat()
	if err != nil {
		return observedFile{}, failure("stale-working-tree-impact", "cannot confirm working-tree target identity: "+value)
	}
	afterIdentity, identityErr := identityFrom(afterOpen)
	if identityErr != nil || identity != afterIdentity || !os.SameFile(opened, afterOpen) || opened.Mode() != afterOpen.Mode() || opened.Size() != afterOpen.Size() {
		return observedFile{}, failure("stale-working-tree-impact", "working-tree target changed during read: "+value)
	}
	leafAfter, err := validateComponents(root, value)
	if err != nil {
		return observedFile{}, failure("stale-working-tree-impact", "working-tree target path changed during read: "+value)
	}
	leafIdentity, leafIdentityErr := identityFrom(leafAfter)
	if leafIdentityErr != nil || identity != leafIdentity || !os.SameFile(opened, leafAfter) {
		return observedFile{}, failure("stale-working-tree-impact", "working-tree target path changed during read: "+value)
	}
	return observedFile{path: value, data: data, mode: opened.Mode(), identity: identity}, nil
}

func validateComponents(root *os.Root, value string) (os.FileInfo, error) {
	parts := strings.Split(value, "/")
	prefix := ""
	var leaf os.FileInfo
	for position, part := range parts {
		if prefix == "" {
			prefix = part
		} else {
			prefix += "/" + part
		}
		info, err := root.Lstat(prefix)
		if err != nil {
			return nil, failure("unsafe-working-tree-impact-file", "working-tree target path cannot be resolved safely: "+value)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, failure("unsafe-working-tree-impact-file", "working-tree target path contains a symlink: "+value)
		}
		if position != len(parts)-1 && !info.IsDir() {
			return nil, failure("unsafe-working-tree-impact-file", "working-tree target parent is not a directory: "+value)
		}
		leaf = info
	}
	if leaf == nil || !leaf.Mode().IsRegular() {
		return nil, failure("unsafe-working-tree-impact-file", "working-tree target must be a regular file: "+value)
	}
	return leaf, nil
}

func confirmIndexSnapshot(ctx context.Context, before *contextindex.Index, targets []string) error {
	after, err := contextindex.Build(ctx, before.Root)
	if err != nil {
		return failure("stale-working-tree-impact", "cannot confirm repository identity after working-tree reads")
	}
	if before.Root != after.Root || before.ObjectFormat != after.ObjectFormat || before.CommitRevision != after.CommitRevision ||
		before.Revision != after.Revision || before.ProfileID != after.ProfileID || before.Module != after.Module ||
		before.StatusSHA256 != after.StatusSHA256 ||
		!equalStrings(before.DirtyPaths, after.DirtyPaths) {
		return failure("stale-working-tree-impact", "repository revision or mixed-worktree status changed during working-tree impact")
	}
	dirty := stringSet(after.DirtyPaths)
	for _, value := range targets {
		if _, tracked := after.Sources[value]; tracked {
			return failure("stale-working-tree-impact", "working-tree target became revision-authoritative during impact: "+value)
		}
		if _, ok := dirty[value]; !ok {
			return failure("stale-working-tree-impact", "working-tree target left captured mixed status during impact: "+value)
		}
	}
	return nil
}

func compileReceipt(index *contextindex.Index, observed []observedFile, paths []string, limit int) (map[string]any, error) {
	allCandidates := collectCandidates(index, observed)
	candidates := allCandidates
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	results := make([]any, len(candidates))
	includedTargets := make(map[string]struct{})
	authoritative := 0
	for position, item := range candidates {
		results[position] = item.value
		if item.value["kind"] == "working-tree-path" {
			includedTargets[item.value["id"].(string)] = struct{}{}
		} else {
			authoritative++
		}
	}
	critical, missing := make([]any, 0, len(paths)), make([]any, 0)
	for _, value := range paths {
		selector := "working-tree-path:" + value
		critical = append(critical, selector)
		if _, ok := includedTargets[value]; !ok {
			missing = append(missing, selector)
		}
	}
	state := "WORKTREE_EVIDENCE"
	if len(missing) != 0 {
		state = "PARTIAL"
	}
	dirtyDigest, err := digestValue(index.DirtyPaths)
	if err != nil {
		return nil, failure("working-tree-impact-output-failed", "cannot bind mixed-worktree status")
	}
	uncertainty := []any{
		"target bytes are stable local observations, not immutable Git revision evidence",
		"impact relationships are inferred only from the captured revision index",
		"working-tree evidence may become stale immediately after the final observation",
	}
	receipt := map[string]any{
		"schema_version": 1,
		"profile":        Profile,
		"mode":           "impact",
		"state":          state,
		"revision":       index.Revision,
		"request": map[string]any{
			"paths": paths, "limit": limit, "authority_profile": "working-tree-untracked",
		},
		"authority": map[string]any{
			"revision": map[string]any{
				"commit": index.CommitRevision, "tree": index.Revision, "object_format": index.ObjectFormat,
				"source": "immutable-git-tree",
			},
			"targets": map[string]any{
				"source": "contained-stable-twice-read-working-tree", "revision_membership": "absent",
				"file_type": "regular", "link_count": 1,
			},
			"mixed_worktree": map[string]any{
				"dirty_path_count": len(index.DirtyPaths), "dirty_paths_sha256": dirtyDigest,
				"status_sha256": index.StatusSHA256,
			},
		},
		"module": map[string]any{"path": index.Module, "source": "captured-revision-go.mod"},
		"freshness": map[string]any{
			"state": "mixed-worktree", "scope": "git+working-tree", "revision": index.Revision,
			"dirty_path_count": len(index.DirtyPaths), "dirty_paths_sha256": dirtyDigest,
			"status_sha256": index.StatusSHA256,
		},
		"results":      results,
		"verification": verificationCommands(observed),
		"coverage": map[string]any{
			"requested_results": len(allCandidates), "included_results": len(results),
			"omitted_results":                len(allCandidates) - len(results),
			"revision_authoritative_results": authoritative, "working_tree_observed_results": len(includedTargets),
			"critical": critical, "critical_missing": missing, "packet_bytes": 0, "within_budget": true,
			"uncertainty": uncertainty[:maxUncertaintyItems],
		},
	}
	if err := stabilizePacketBytes(receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

func collectCandidates(index *contextindex.Index, observed []observedFile) []candidate {
	items := make([]candidate, 0)
	seen := make(map[string]struct{})
	for _, file := range observed {
		packageDirectory := path.Dir(file.path)
		packageImport := index.Module + "/" + packageDirectory
		digest := sha256.Sum256(file.data)
		identityDigest := sha256.Sum256([]byte(file.identity.preimage(file.mode, int64(len(file.data)))))
		direct := map[string]any{
			"kind": "working-tree-path", "id": file.path, "score": 1000,
			"summary": "direct revision-absent Go path observed in the working tree",
			"package": map[string]any{
				"directory": packageDirectory, "import_path": packageImport, "name": file.packageName, "imports": file.imports,
			},
			"evidence": []any{map[string]any{
				"path": file.path, "line": 1, "sha256": fmt.Sprintf("%x", digest),
				"identity_sha256": fmt.Sprintf("%x", identityDigest), "bytes": len(file.data),
				"mode": fmt.Sprintf("%04o", uint32(file.mode.Perm())), "link_count": 1,
				"reason": "requested revision-absent path", "confidence": "observed", "source": "working-tree-stable-read",
			}},
		}
		items = append(items, candidate{score: 1000, order: 0, key: "working-tree-path:" + file.path, value: direct})
		for sourcePath, source := range index.Sources {
			if path.Dir(sourcePath) != packageDirectory || !strings.HasSuffix(sourcePath, ".go") {
				continue
			}
			parsedSource, err := parser.ParseFile(token.NewFileSet(), sourcePath, source.Data, parser.PackageClauseOnly)
			if err != nil || parsedSource.Name.Name != file.packageName {
				continue
			}
			kind, score := "revision-package-source", 700
			if strings.HasSuffix(sourcePath, "_test.go") {
				kind, score = "revision-package-test", 825
			}
			key := kind + ":" + sourcePath
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, candidate{score: score, order: 1, key: key, value: revisionResult(kind, sourcePath, source.BlobHash, score, "same captured-revision package as "+file.path)})
		}
		for importerPath, imports := range index.Imports {
			if _, ok := imports[packageImport]; !ok {
				continue
			}
			source, ok := index.Sources[importerPath]
			if !ok {
				continue
			}
			key := "revision-reverse-import:" + importerPath
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, candidate{score: 650, order: 2, key: key, value: revisionResult("revision-reverse-import", importerPath, source.BlobHash, 650, "imports "+packageImport)})
		}
	}
	sort.SliceStable(items, func(left, right int) bool {
		if items[left].score != items[right].score {
			return items[left].score > items[right].score
		}
		if items[left].order != items[right].order {
			return items[left].order < items[right].order
		}
		return items[left].key < items[right].key
	})
	return items
}

func revisionResult(kind, value, blob string, score int, reason string) map[string]any {
	return map[string]any{
		"kind": kind, "id": value, "score": score, "summary": reason,
		"evidence": []any{map[string]any{
			"path": value, "line": 1, "blob": blob, "reason": reason,
			"confidence": "authoritative", "source": "git-tree",
		}},
	}
}

func verificationCommands(observed []observedFile) []any {
	directories := make(map[string]struct{})
	for _, file := range observed {
		directories[path.Dir(file.path)] = struct{}{}
	}
	ordered := sortedKeys(directories)
	result := make([]any, 0, len(ordered))
	for _, directory := range ordered {
		result = append(result, map[string]any{
			"command": "go test ./" + directory + "/...", "reason": "candidate verification for observed package", "authority": "proposal",
		})
	}
	return result
}

func stabilizePacketBytes(receipt map[string]any) error {
	coverage := receipt["coverage"].(map[string]any)
	for attempts := 0; attempts < 12; attempts++ {
		encoded, err := contextindex.CanonicalJSON(receipt)
		if err != nil {
			return failure("working-tree-impact-output-failed", "cannot encode working-tree impact receipt")
		}
		if coverage["packet_bytes"] == len(encoded) {
			return nil
		}
		coverage["packet_bytes"] = len(encoded)
	}
	return failure("working-tree-impact-output-failed", "cannot stabilize working-tree impact receipt size")
}

func digestValue(value any) (string, error) {
	encoded, err := contextindex.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", digest), nil
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for position := range left {
		if left[position] != right[position] {
			return false
		}
	}
	return true
}

func failure(code, message string) error {
	return &gokernel.Error{Code: code, Message: message}
}
