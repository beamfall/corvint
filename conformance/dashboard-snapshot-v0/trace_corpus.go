package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxTraceRows       = 1_000
	maxTraceRowBytes   = 256 << 10
	maxTraceStoreBytes = 16 << 20
)

var traceIDRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

var traceSecretRE = regexp.MustCompile(`(?i)[a-z0-9_.-]*(?:api[_-]?key|access[_-]?key|authorization|password|private[_-]?key|secret|token)[a-z0-9_.-]*\s*[:=]\s*\S+|-----BEGIN [A-Z ]*PRIVATE KEY-----|-----BEGIN PGP PRIVATE KEY BLOCK-----|[a-z][a-z0-9+.-]*://[^\s/:]+:[^\s/@]+@|gh[pousr]_[a-z0-9]{20,}|github_pat_[a-z0-9_]{20,}|glpat-[a-z0-9_-]{20,}|xox[a-z]-[a-z0-9-]{10,}|(?:akia|asia)[a-z0-9]{16}|aiza[a-z0-9_-]{30,}|npm_[a-z0-9]{20,}|\bsk-[a-z0-9_-]{20,}|(?:sk|rk)_(?:live|test)_[a-z0-9]{16,}|pypi-ageichlwasi5vcmc[a-z0-9_-]{20,}|sg\.[a-z0-9_-]{16,}\.[a-z0-9_-]{32,}|sk[0-9a-f]{32}|bearer\s+[a-z0-9][a-z0-9_.~+/-]{15,}|[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}`)
var traceSafeCommandRE = regexp.MustCompile(`^[A-Za-z0-9_./:@=+, -]+$`)
var generatedTracePathRE = regexp.MustCompile(`(?i)(?:^|/)(?:generated|gen)(?:/|$)|(?:\.gen\.|_generated\.)|(?:^|/)docs/api/(?:openapi\.json|reference\.md|llms(?:-full)?\.txt)$`)

type traceCorpusResult struct {
	files      int
	rows       int
	outcomes   map[string]map[string]int
	members    []traceCorpusMember
	sources    []traceCorpusSource
	repository gitSemanticEvidence
}

type traceCorpusSource struct {
	configuredOrdinal string
	files             int
	rows              int
	outcomes          map[string]map[string]int
	members           []traceCorpusMember
	terminal          map[string]int
	overrideCode      string
	overrideLimit     string
}

type traceCorpusMember struct {
	revision string
	digest   string
	bytes    int
}

func validateTraceCorpus(rootPath string, manifest traceManifest) (traceCorpusResult, error) {
	authority, err := newClosedGitAuthority(rootPath)
	if err != nil {
		return traceCorpusResult{}, err
	}
	return validateTraceCorpusWithAuthorityAndHook(rootPath, manifest, authority, nil)
}

type traceRepositoryAuthority interface {
	repositoryIdentity() (string, string, error)
	qualifyCommit(revision, head string) error
	qualifyPaths(revision string, relative []string) error
	finish() error
}

func validateTraceCorpusWithAuthority(rootPath string, manifest traceManifest, authority traceRepositoryAuthority) (traceCorpusResult, error) {
	return validateTraceCorpusWithAuthorityAndHook(rootPath, manifest, authority, nil)
}

type traceScanHook func(configuredOrdinal string, attempt int) error

func validateTraceCorpusWithAuthorityAndHook(rootPath string, manifest traceManifest, authority traceRepositoryAuthority, hook traceScanHook) (traceCorpusResult, error) {
	if rootPath == "" || !filepath.IsAbs(rootPath) {
		return traceCorpusResult{}, reject(rejectPrivacyText)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return traceCorpusResult{}, reject(rejectJSON)
	}
	defer root.Close()
	format, head, err := authority.repositoryIdentity()
	if err != nil || format != "sha1" && format != "sha256" {
		return traceCorpusResult{}, reject(rejectIdentity)
	}
	result := traceCorpusResult{outcomes: make(map[string]map[string]int)}
	traceIDs := make(map[string]struct{})
	for _, source := range manifest.configuredSources {
		if source.adapterID != "local-trace-v1" {
			continue
		}
		scanned, nextTraceIDs, err := scanTraceSource(root, source, format, head, authority, traceIDs, hook)
		if err != nil {
			return traceCorpusResult{}, err
		}
		traceIDs = nextTraceIDs
		result.sources = append(result.sources, scanned)
		result.files += scanned.files
		result.rows += scanned.rows
		for revision, outcomes := range scanned.outcomes {
			if result.outcomes[revision] == nil {
				result.outcomes[revision] = make(map[string]int)
			}
			for outcome, count := range outcomes {
				result.outcomes[revision][outcome] += count
			}
		}
		result.members = append(result.members, scanned.members...)
	}
	if err := authority.finish(); err != nil {
		return traceCorpusResult{}, err
	}
	if provider, ok := authority.(interface{ semanticEvidence() gitSemanticEvidence }); ok {
		result.repository = provider.semanticEvidence()
	}
	return result, nil
}

type traceDirectoryEntry struct {
	name string
	info os.FileInfo
}

type traceDirectorySnapshot struct {
	directory os.FileInfo
	binding   os.FileInfo
	entries   []traceDirectoryEntry
	overBound bool
}

func snapshotTraceDirectory(root *os.Root, relative string) (traceDirectorySnapshot, error) {
	if err := verifyDirectoryComponents(root, relative); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return traceDirectorySnapshot{}, reject(rejectJSON)
		}
		return traceDirectorySnapshot{}, err
	}
	binding, err := root.Lstat(relative)
	if err != nil || binding.Mode()&os.ModeSymlink != 0 || !binding.IsDir() {
		return traceDirectorySnapshot{}, reject(rejectPrivacyText)
	}
	directory, err := root.Open(relative)
	if err != nil {
		return traceDirectorySnapshot{}, reject(rejectJSON)
	}
	defer directory.Close()
	directoryInfo, err := directory.Stat()
	if err != nil || !os.SameFile(binding, directoryInfo) {
		return traceDirectorySnapshot{}, reject(rejectIdentity)
	}
	rows, readErr := directory.ReadDir(1_001)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return traceDirectorySnapshot{}, reject(rejectJSON)
	}
	result := traceDirectorySnapshot{directory: directoryInfo, binding: binding, overBound: len(rows) > 1_000}
	for _, row := range rows {
		info, statErr := root.Lstat(relative + "/" + row.Name())
		if statErr != nil {
			return traceDirectorySnapshot{}, reject(rejectIdentity)
		}
		result.entries = append(result.entries, traceDirectoryEntry{name: row.Name(), info: info})
	}
	sort.Slice(result.entries, func(left, right int) bool { return result.entries[left].name < result.entries[right].name })
	return result, nil
}

func sameTraceDirectorySnapshot(left, right traceDirectorySnapshot) bool {
	if left.overBound != right.overBound || !sameTraceFileIdentity(left.directory, right.directory) || !sameTraceFileIdentity(left.binding, right.binding) || len(left.entries) != len(right.entries) {
		return false
	}
	for index := range left.entries {
		if left.entries[index].name != right.entries[index].name || !sameTraceFileIdentity(left.entries[index].info, right.entries[index].info) {
			return false
		}
	}
	return true
}

func sameTraceFileIdentity(left, right os.FileInfo) bool {
	return os.SameFile(left, right) && left.Mode() == right.Mode() && left.Size() == right.Size() && left.ModTime().Equal(right.ModTime())
}

func scanTraceSource(root *os.Root, source traceConfiguredSource, format, head string, authority traceRepositoryAuthority, committedTraceIDs map[string]struct{}, hook traceScanHook) (traceCorpusSource, map[string]struct{}, error) {
	for attempt := 0; attempt < 2; attempt++ {
		before, err := snapshotTraceDirectory(root, source.relativePath)
		if err != nil {
			return traceCorpusSource{}, committedTraceIDs, err
		}
		candidateTraceIDs := cloneStringSet(committedTraceIDs)
		result := traceCorpusSource{configuredOrdinal: source.configuredOrdinal, outcomes: make(map[string]map[string]int), terminal: make(map[string]int)}
		totalBytes := 0
		if before.overBound {
			result.overrideCode, result.overrideLimit = "TRACE_STORE_BOUND", "1000"
		} else {
			for _, entry := range before.entries {
				name := entry.name
				if !traceFilename(format, name) {
					continue
				}
				result.files++
				if entry.info.Mode()&os.ModeSymlink != 0 {
					result.terminal["SOURCE_SYMLINK"]++
					continue
				}
				if !entry.info.Mode().IsRegular() {
					result.terminal["SOURCE_SPECIAL_FILE"]++
					continue
				}
				revision := strings.TrimSuffix(name, ".jsonl")
				raw, readErr := stableRootRead(root, source.relativePath+"/"+name)
				if readErr != nil {
					result.terminal[traceReadTerminal(readErr)]++
					continue
				}
				totalBytes += len(raw)
				if totalBytes > maxTraceStoreBytes {
					result.overrideCode, result.overrideLimit = "TRACE_STORE_BOUND", "16777216"
					break
				}
				candidateOutcomes := make(map[string]int)
				candidateIDs := cloneStringSet(candidateTraceIDs)
				candidateRows := 0
				candidateCode := ""
				scanner := bufio.NewScanner(bytes.NewReader(raw))
				scanner.Buffer(make([]byte, 0, 4096), maxTraceRowBytes)
				for scanner.Scan() {
					candidateRows++
					result.rows++
					if result.rows > maxTraceRows {
						result.overrideCode, result.overrideLimit = "TRACE_STORE_BOUND", "1000"
						break
					}
					trace, rowErr := validateTraceRow(scanner.Bytes(), revision, authority)
					if rowErr != nil {
						candidateCode = traceRowTerminal(rowErr)
						break
					}
					traceID := trace["trace_id"].(string)
					if _, duplicate := candidateIDs[traceID]; duplicate {
						candidateCode = "VERIFIER_REJECTED"
						break
					}
					candidateIDs[traceID] = struct{}{}
					candidateOutcomes[trace["outcome"].(string)]++
				}
				if result.overrideCode != "" {
					break
				}
				if scanErr := scanner.Err(); scanErr != nil {
					result.overrideCode, result.overrideLimit = "TRACE_STORE_BOUND", "262144"
					break
				}
				if candidateRows == 0 && candidateCode == "" {
					candidateCode = "SOURCE_INVALID_SCHEMA"
				}
				if candidateCode == "" {
					if qualifyErr := authority.qualifyCommit(revision, head); qualifyErr != nil {
						candidateCode = traceAuthorityTerminal(qualifyErr)
					}
				}
				if candidateCode != "" {
					result.terminal[candidateCode]++
					continue
				}
				candidateTraceIDs = candidateIDs
				memberDigest := sha256.Sum256(raw)
				result.members = append(result.members, traceCorpusMember{revision: revision, digest: "sha256:" + hex.EncodeToString(memberDigest[:]), bytes: len(raw)})
				result.outcomes[revision] = candidateOutcomes
			}
		}
		if hook != nil {
			if err := hook(source.configuredOrdinal, attempt); err != nil {
				return traceCorpusSource{}, committedTraceIDs, err
			}
		}
		after, err := snapshotTraceDirectory(root, source.relativePath)
		if err == nil && sameTraceDirectorySnapshot(before, after) {
			if result.overrideCode != "" {
				result.members = nil
				result.outcomes = map[string]map[string]int{}
				result.terminal = map[string]int{}
			}
			return result, candidateTraceIDs, nil
		}
		if attempt == 1 {
			return traceCorpusSource{configuredOrdinal: source.configuredOrdinal, outcomes: map[string]map[string]int{}, terminal: map[string]int{}, overrideCode: "STORE_CHANGED"}, committedTraceIDs, nil
		}
	}
	panic("unreachable")
}

func cloneStringSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for key := range source {
		result[key] = struct{}{}
	}
	return result
}

func traceReadTerminal(err error) string {
	if rejectionCode(err) == rejectSize {
		return "SOURCE_OVERSIZED"
	}
	if rejectionCode(err) == rejectIdentity {
		return "SOURCE_CHANGED_DURING_READ"
	}
	return "SOURCE_INACCESSIBLE"
}

func traceRowTerminal(err error) string {
	switch rejectionCode(err) {
	case rejectIdentity:
		return "SOURCE_INVALID_IDENTITY"
	case rejectPrivacyText, rejectEnum, rejectOrdering:
		return "VERIFIER_REJECTED"
	default:
		return "SOURCE_INVALID_SCHEMA"
	}
}

func traceAuthorityTerminal(err error) string {
	if rejectionCode(err) == rejectSize {
		return "TRACE_ANCESTRY_BOUND"
	}
	return "SOURCE_INVALID_IDENTITY"
}

func verifyDirectoryComponents(root *os.Root, relative string) error {
	components := strings.Split(relative, "/")
	for index := range components {
		name := strings.Join(components[:index+1], "/")
		info, err := root.Lstat(name)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return reject(rejectPrivacyText)
		}
	}
	return nil
}

func traceFilename(format, name string) bool {
	if !strings.HasSuffix(name, ".jsonl") {
		return false
	}
	revision := strings.TrimSuffix(name, ".jsonl")
	if format == "sha1" {
		return gitSHA1RE.MatchString(revision)
	}
	return gitSHA256RE.MatchString(revision)
}

func stableRootRead(root *os.Root, relative string) ([]byte, error) {
	before, err := root.Lstat(relative)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() > maxTraceStoreBytes {
		return nil, reject(rejectPrivacyText)
	}
	read := func() ([]byte, error) {
		file, err := root.Open(relative)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(io.LimitReader(file, maxTraceStoreBytes+1))
	}
	first, err := read()
	if err != nil || len(first) > maxTraceStoreBytes {
		return nil, reject(rejectSize)
	}
	second, err := read()
	after, statErr := root.Lstat(relative)
	if err != nil || statErr != nil || !bytes.Equal(first, second) || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, reject(rejectIdentity)
	}
	return first, nil
}

func validateTraceRow(raw []byte, revision string, authority traceRepositoryAuthority) (map[string]any, error) {
	if len(raw) == 0 || len(raw) > maxTraceRowBytes || !utf8.Valid(raw) {
		return nil, reject(rejectJSON)
	}
	value, err := decodeTrace(raw)
	if err != nil {
		return nil, err
	}
	trace, err := object(value)
	if err != nil {
		return nil, err
	}
	if err := exactFields(trace, "schema_version", "revision", "trace_id", "task", "opened_paths", "changed_paths", "verification", "outcome"); err != nil {
		return nil, err
	}
	if trace["schema_version"] != json.Number("1") || trace["revision"] != revision {
		return nil, reject(rejectIdentity)
	}
	if _, err := enum(trace["outcome"], set("passed", "failed", "blocked")); err != nil {
		return nil, err
	}
	task, err := stringValue(trace["task"])
	if err != nil || strings.TrimSpace(task) == "" || strings.TrimSpace(task) != task || utf8.RuneCountInString(task) > 2_000 || traceSecretRE.MatchString(task) {
		return nil, reject(rejectPrivacyText)
	}
	allPaths := make(map[string]struct{})
	for _, field := range []string{"opened_paths", "changed_paths"} {
		paths, err := traceStringSet(trace[field], 200)
		if err != nil {
			return nil, err
		}
		for _, item := range paths {
			if !safeTraceRelativePath(item) || traceSecretRE.MatchString(item) || forbiddenTracePath(item) {
				return nil, reject(rejectIdentity)
			}
			allPaths[item] = struct{}{}
		}
	}
	qualifiedPaths := make([]string, 0, len(allPaths))
	for item := range allPaths {
		qualifiedPaths = append(qualifiedPaths, item)
	}
	sort.Strings(qualifiedPaths)
	if err := authority.qualifyPaths(revision, qualifiedPaths); err != nil {
		return nil, reject(rejectIdentity)
	}
	commands, err := traceStringSet(trace["verification"], 50)
	if err != nil {
		return nil, err
	}
	for _, command := range commands {
		if strings.TrimSpace(command) != command || utf8.RuneCountInString(command) > 512 || traceSecretRE.MatchString(command) || !traceSafeCommandRE.MatchString(command) {
			return nil, reject(rejectPrivacyText)
		}
	}
	claimed, err := stringValue(trace["trace_id"])
	if err != nil || !traceIDRE.MatchString(claimed) {
		return nil, reject(rejectIdentity)
	}
	basisBytes, err := canonical(cloneWithout(trace, "trace_id"), false)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(basisBytes)
	if claimed != hex.EncodeToString(digest[:]) {
		return nil, reject(rejectIdentity)
	}
	return trace, nil
}

func forbiddenTracePath(value string) bool {
	for _, component := range strings.Split(value, "/") {
		switch component {
		case ".git", "vendor", "node_modules", "app-dist", "dist", "build", "coverage", ".next", ".cache", "target":
			return true
		}
	}
	return strings.HasPrefix(value, "internal/store/migrate/") || strings.HasPrefix(value, "internal/conformance/testdata/") || generatedTracePathRE.MatchString(value)
}

func traceStringSet(value any, maximum int) ([]string, error) {
	items, err := array(value)
	if err != nil || len(items) > maximum {
		return nil, reject(rejectSize)
	}
	result := make([]string, len(items))
	for index, item := range items {
		text, err := stringValue(item)
		if err != nil || text == "" || index > 0 && result[index-1] >= text {
			return nil, reject(rejectOrdering)
		}
		result[index] = text
	}
	return result, nil
}

func decodeTrace(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeTraceValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, reject(rejectJSON)
	}
	return value, nil
}

func decodeTraceValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, reject(rejectJSON)
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, reject(rejectJSON)
	}
	if number, ok := token.(json.Number); ok {
		if !decimalRE.MatchString(string(number)) {
			return nil, reject(rejectDecimal)
		}
		return number, nil
	}
	if delimiter, ok := token.(json.Delim); ok {
		switch delimiter {
		case '{':
			result := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, reject(rejectJSON)
				}
				key := keyToken.(string)
				if _, exists := result[key]; exists {
					return nil, reject(rejectDuplicate)
				}
				field, err := decodeTraceValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				result[key] = field
			}
			if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
				return nil, reject(rejectJSON)
			}
			return result, nil
		case '[':
			result := make([]any, 0)
			for decoder.More() {
				item, err := decodeTraceValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, item)
			}
			if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
				return nil, reject(rejectJSON)
			}
			return result, nil
		}
	}
	return token, nil
}
