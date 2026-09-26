package contextindex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/projectprofile"
	"github.com/Beamfall/corvint/internal/untrackedallowance"
)

const (
	rangeImpactProfile         = "corvint-range-impact/0"
	expandedRangeImpactProfile = "corvint-range-impact-expanded/experimental"
	maxExpandedRangePaths      = 256
	maxRangeHunks              = 10_000
	maxRangeTargetLines        = 200_000
)

var (
	rangeHunkPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
	rangeADRPattern  = regexp.MustCompile(`\bADR-([0-9]{4})\b`)
	rangePathPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

type rangeHunk struct {
	path               string
	oldStart, oldLines int
	newStart, newLines int
}

type rangeChange struct {
	status, path           string
	oldMode, newMode       string
	oldBlob, newBlob       string
	similarity             int
	sourcePath, sourceBlob string
}

// RangeImpact compiles hunk-qualified impact for one immutable base commit
// against the clean, stably captured HEAD represented by index.
func RangeImpact(ctx context.Context, index *Index, base string, limit int) (map[string]any, error) {
	return compileRangeImpact(ctx, index, base, limit, false)
}

// ExpandedRangeImpact explicitly selects the experimental fixed-256-path
// profile. All remaining validation, resource bounds and semantics are shared
// with RangeImpact; no failed default request falls back to this profile.
func ExpandedRangeImpact(ctx context.Context, index *Index, base string, limit int) (map[string]any, error) {
	return compileRangeImpact(ctx, index, base, limit, true)
}

func compileRangeImpact(ctx context.Context, index *Index, base string, limit int, expanded bool) (map[string]any, error) {
	if limit < 1 || limit > maxLimit {
		return nil, &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	if !validObjectID(base, index.ObjectFormat) {
		return nil, &Error{Code: "unsupported-impact-range", Message: "--base must be a full immutable commit object ID"}
	}
	if index.Module == "" || !strings.Contains(index.Module, "/") {
		return nil, &Error{Code: "unsupported-impact-repository", Message: "native Go impact requires a slash-qualified Go module"}
	}
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	allowedUntracked, err := verifyRangeSnapshot(ctx, index)
	if err != nil {
		return nil, err
	}
	// V1-0055: one spawn resolves both the commit and its tree; `^{commit}`
	// fails the same way `cat-file -t` did when base is not a commit object.
	resolvedRaw, err := git(ctx, index.Root, maxIdentityBytes, nil, "rev-parse", base+"^{commit}", base+"^{tree}")
	if err != nil {
		return nil, &Error{Code: "unsupported-impact-range", Message: "--base must identify an available commit object"}
	}
	resolved := strings.Split(strings.TrimSuffix(string(resolvedRaw), "\n"), "\n")
	if len(resolved) != 2 {
		return nil, &Error{Code: "unsupported-impact-range", Message: "cannot resolve the base commit tree"}
	}
	baseTree := resolved[1]
	if !validObjectID(baseTree, index.ObjectFormat) {
		return nil, &Error{Code: "unsupported-impact-range", Message: "Git returned an invalid base tree identity"}
	}
	mergeBases, err := git(ctx, index.Root, maxIdentityBytes, nil, "merge-base", "--all", base, index.CommitRevision)
	if err != nil || strings.TrimSpace(string(mergeBases)) != base {
		return nil, &Error{Code: "unsupported-impact-range", Message: "--base must be an ancestor of the captured HEAD"}
	}
	changes, err := readRangeChanges(ctx, index, baseTree)
	if err != nil {
		return nil, err
	}
	// V1-0055: built once so the per-Go-path lookups below (readRangeHunksFor,
	// the goPaths loop, qualifyRangePathEvidence) are map reads instead of a
	// linear scan of changes per path.
	changeByPath := make(map[string]rangeChange, len(changes))
	for _, change := range changes {
		changeByPath[change.path] = change
	}
	pathLimit := maxImpactPaths
	if expanded {
		pathLimit = maxExpandedRangePaths
	}
	if len(changes) > pathLimit {
		return nil, &Error{Code: "unsupported-impact-range", Message: fmt.Sprintf("impact range exceeds the %d-path bound", pathLimit)}
	}
	if err := rejectRangeBinary(ctx, index, baseTree); err != nil {
		return nil, err
	}

	goPaths := make([]string, 0, len(changes))
	omissions := make([]any, 0)
	moduleChanged := false
	for _, change := range changes {
		switch change.status {
		case "M", "A", "C", "R":
		default:
			return nil, &Error{Code: "unsupported-impact-range", Message: "impact range contains unsupported status " + change.status + " for " + change.path}
		}
		if change.status != "A" && change.oldMode != change.newMode {
			return nil, &Error{Code: "unsupported-impact-range", Message: "impact range contains a mode change for " + change.path}
		}
		if change.newMode != "100644" && change.newMode != "100755" {
			return nil, &Error{Code: "unsupported-impact-range", Message: "impact range contains a symlink, submodule, or unsupported file mode for " + change.path}
		}
		if change.path == "go.mod" || change.path == "go.sum" {
			moduleChanged = true
		}
		if !strings.HasSuffix(change.path, ".go") {
			omissions = append(omissions, map[string]any{"path": change.path, "reason": "non-Go path outside native Go range profile"})
			continue
		}
		if path.Dir(change.path) == "." {
			return nil, &Error{Code: "unsupported-impact-path", Message: "native Go range impact requires changed Go paths in a non-root package"}
		}
		source, ok := index.Sources[change.path]
		if !ok {
			return nil, &Error{Code: "unsupported-impact-range", Message: "changed Go path is not admitted by the target index: " + change.path}
		}
		if source.BlobHash != change.newBlob {
			return nil, &Error{Code: "impact-range-drift", Message: "changed Go path does not match the captured target blob: " + change.path}
		}
		if _, parseErr := parser.ParseFile(token.NewFileSet(), change.path, source.Data, parser.AllErrors); parseErr != nil {
			return nil, &Error{Code: "unsupported-impact-range", Message: "changed Go path is not valid Go syntax: " + change.path}
		}
		goPaths = append(goPaths, change.path)
	}

	hunks := make([]rangeHunk, 0)
	spans := make(map[string][]rangeHunk, len(goPaths))
	targetLineCount := 0
	pathReads := readRangeHunksFor(ctx, index, baseTree, changeByPath, goPaths)
	for position, changedPath := range goPaths {
		change := changeByPath[changedPath]
		pathHunks, hunkErr := pathReads[position].hunks, pathReads[position].err
		if hunkErr != nil {
			return nil, hunkErr
		}
		if len(pathHunks) == 0 && change.sourcePath == "" {
			return nil, &Error{Code: "unsupported-impact-range", Message: "changed Go path has no textual diff hunks: " + changedPath}
		}
		hunks = append(hunks, pathHunks...)
		for _, hunk := range pathHunks {
			targetLineCount += hunk.newLines
		}
		if len(hunks) > maxRangeHunks {
			return nil, &Error{Code: "unsupported-impact-range", Message: fmt.Sprintf("impact range exceeds the %d-hunk bound", maxRangeHunks)}
		}
		if targetLineCount > maxRangeTargetLines {
			return nil, &Error{Code: "unsupported-impact-range", Message: fmt.Sprintf("impact range exceeds the %d-target-line bound", maxRangeTargetLines)}
		}
		spans[changedPath] = pathHunks
	}
	if _, err := verifyRangeSnapshot(ctx, index); err != nil {
		return nil, err
	}

	rawResults, authorityUncertainty := compileRangeResults(index, goPaths, spans, changeByPath, callerAuthoredPaths(changes))
	rawResults = deduplicateAndSortRangeResults(rawResults)

	request := map[string]any{"base": base, "limit": limit}
	if expanded {
		request["rangeProfile"] = "expanded-256"
	}
	result, err := receipt(index, "range-impact", request, rawResults, limit, "")
	if err != nil {
		return nil, err
	}
	result["profile"] = rangeImpactProfile
	if expanded {
		result["profile"] = expandedRangeImpactProfile
	}
	hunkRows := make([]any, 0, len(hunks))
	changeRows := make([]any, 0, len(changes))
	for _, change := range changes {
		changeRows = append(changeRows, change.binding())
	}
	for _, hunk := range hunks {
		change := changeByPath[hunk.path]
		row := change.binding()
		row["oldStart"], row["oldLines"] = hunk.oldStart, hunk.oldLines
		row["newStart"], row["newLines"] = hunk.newStart, hunk.newLines
		hunkRows = append(hunkRows, row)
	}
	hunkBytes, _ := CanonicalJSON(hunkRows)
	hunkHash := sha256.New()
	hunkHash.Write([]byte("corvint-range-impact-hunks/0"))
	hunkHash.Write([]byte{0})
	hunkHash.Write(hunkBytes)
	changeBytes, _ := CanonicalJSON(changeRows)
	changeHash := sha256.New()
	changeHash.Write([]byte("corvint-range-impact-changes/0"))
	changeHash.Write([]byte{0})
	changeHash.Write(changeBytes)
	rangeBinding := map[string]any{
		"baseCommit": base, "baseTree": baseTree,
		"headCommit": index.CommitRevision, "headTree": index.Revision,
		"status": "CLEAN", "statusSha256": "sha256:" + index.StatusSHA256,
		"changedPathCount": len(changes), "changedGoPathCount": len(goPaths),
		"changesSha256": fmt.Sprintf("sha256:%x", changeHash.Sum(nil)),
		"hunkCount":     len(hunks), "hunksSha256": fmt.Sprintf("sha256:%x", hunkHash.Sum(nil)),
	}
	// A clean worktree keeps its existing bytes; a tolerated untracked set is
	// named by status and bound by count and digest (GPK-V0-060).
	if len(allowedUntracked) != 0 {
		allowance := untrackedallowance.Bind(allowedUntracked)
		rangeBinding["status"] = "UNTRACKED-ALLOWED"
		rangeBinding["untrackedAllowance"] = map[string]any{
			"rule": allowance.Rule, "count": allowance.Count, "sha256": "sha256:" + allowance.SHA256,
		}
	}
	result["range"] = rangeBinding
	omissionBytes, _ := CanonicalJSON(omissions)
	omissionHash := sha256.New()
	omissionHash.Write([]byte("corvint-range-impact-omissions/0"))
	omissionHash.Write([]byte{0})
	omissionHash.Write(omissionBytes)
	result["omissions"] = map[string]any{
		"count": len(omissions), "sha256": fmt.Sprintf("sha256:%x", omissionHash.Sum(nil)),
		"samples": omissions[:min(len(omissions), maxExclusionSamples)],
	}
	result["verification"] = rangeVerification(goPaths, index, moduleChanged)
	coverage := result["coverage"].(map[string]any)
	// The result counts are left to `receipt`, which now denominates them in
	// the same pre-truncation `rawResults` this function would restate.  They
	// were restated here only because that arithmetic was structurally zero;
	// keeping a second writer of the same three fields buys nothing and lets
	// the two drift apart.  The critical set below is a genuine override: it
	// names every changed Go path, including one the ranking ceiling dropped,
	// which a set derived from the surviving results cannot report.
	included := min(len(rawResults), limit)
	critical := make([]any, len(goPaths))
	criticalMissing := make([]any, 0)
	includedPaths := make(map[string]struct{})
	for _, item := range mapsFromAny(result["results"]) {
		if item["kind"] == "path" {
			includedPaths[stringValue(item["id"])] = struct{}{}
		}
	}
	for position, changedPath := range goPaths {
		selector := "path:" + changedPath
		critical[position] = selector
		if _, ok := includedPaths[changedPath]; !ok {
			criticalMissing = append(criticalMissing, selector)
		}
	}
	coverage["critical"] = critical
	coverage["critical_missing"] = criticalMissing
	uncertainty := []any{"feature/scenario markers and ADR citations are admitted only from target lines added or replaced by the committed diff"}
	uncertainty = append(uncertainty, authorityUncertainty...)
	if len(omissions) != 0 {
		uncertainty = append(uncertainty, fmt.Sprintf("%d non-Go changed paths are outside the native Go range profile", len(omissions)))
	}
	if len(rawResults) > included {
		uncertainty = append(uncertainty, fmt.Sprintf("%d ranked impact results are omitted by the result limit", len(rawResults)-included))
	}
	coverage["uncertainty"] = uncertainty
	if len(criticalMissing) != 0 {
		result["state"] = "BUDGETED"
	}
	if err := verifyRangeBase(ctx, index, base, baseTree); err != nil {
		return nil, err
	}
	if err := stabilizePacketBytes(result); err != nil {
		return nil, err
	}
	return result, nil
}

// verifyRangeSnapshot re-observes identity and status and returns the
// untracked paths the range tolerates. Status bytes are digest-equal to the
// captured index, so every call returns the same allowed set.
func verifyRangeSnapshot(ctx context.Context, index *Index) ([]string, error) {
	identity, err := readIdentity(ctx, index.Root)
	if err != nil {
		return nil, err
	}
	paths, statusSHA256, raw, err := readStatusSnapshotRaw(ctx, index.Root)
	if err != nil {
		return nil, err
	}
	if identity.objectFormat != index.ObjectFormat || identity.commitRevision != index.CommitRevision || identity.treeRevision != index.Revision || statusSHA256 != index.StatusSHA256 || !slicesEqual(paths, index.DirtyPaths) {
		return nil, &Error{Code: "impact-range-drift", Message: "repository identity or status changed while compiling impact range"}
	}
	return rangeAllowedUntracked(index, raw)
}

// rangeAllowedUntracked admits a dirty worktree only when every entry is an
// untracked path disjoint from the Go build over the captured tree (decision
// 0142, GPK-V0-060). Ignored paths never appear in status; anything else,
// including every tracked change, still refuses.
func rangeAllowedUntracked(index *Index, raw []byte) ([]string, error) {
	untracked, tracked, err := untrackedallowance.ParseStatus(raw)
	if err != nil || len(tracked) != 0 {
		return nil, &Error{Code: "unsupported-impact-worktree", Message: "range impact requires a clean worktree"}
	}
	trackedPaths := make([]string, 0, len(index.Tracked)+len(index.Sources)+len(index.Exclusions))
	for trackedPath := range index.Tracked {
		trackedPaths = append(trackedPaths, trackedPath)
	}
	for sourcePath := range index.Sources {
		trackedPaths = append(trackedPaths, sourcePath)
	}
	for _, exclusion := range index.Exclusions {
		trackedPaths = append(trackedPaths, exclusion.Path)
	}
	directories := untrackedallowance.GoPackageDirectories(trackedPaths)
	allowed, overlapping := untrackedallowance.Partition(untracked, func(candidate string) bool {
		return untrackedallowance.OverlapsGoBuild(candidate, directories)
	})
	if overlapping != "" {
		return nil, &Error{Code: "unsupported-impact-worktree", Message: "range impact requires a clean worktree; untracked path overlaps the Go build: " + overlapping}
	}
	return allowed, nil
}

func verifyRangeBase(ctx context.Context, index *Index, base, baseTree string) error {
	if _, err := verifyRangeSnapshot(ctx, index); err != nil {
		return err
	}
	objectType, err := git(ctx, index.Root, maxIdentityBytes, nil, "cat-file", "-t", base)
	if err != nil || string(objectType) != "commit\n" {
		return &Error{Code: "impact-range-drift", Message: "base commit became unavailable while compiling impact range"}
	}
	resolved, err := git(ctx, index.Root, maxIdentityBytes, nil, "rev-parse", base+"^{tree}")
	if err != nil || strings.TrimSuffix(string(resolved), "\n") != baseTree {
		return &Error{Code: "impact-range-drift", Message: "base tree changed while compiling impact range"}
	}
	mergeBases, err := git(ctx, index.Root, maxIdentityBytes, nil, "merge-base", "--all", base, index.CommitRevision)
	if err != nil || strings.TrimSpace(string(mergeBases)) != base {
		return &Error{Code: "impact-range-drift", Message: "base ancestry changed while compiling impact range"}
	}
	return nil
}

func (change rangeChange) binding() map[string]any {
	result := map[string]any{
		"path": change.path, "status": change.status,
		"oldMode": change.oldMode, "newMode": change.newMode,
		"baseBlob": change.oldBlob, "targetBlob": change.newBlob,
	}
	if change.sourcePath != "" {
		result["similarity"] = change.similarity
		result["sourcePath"] = change.sourcePath
		result["sourceBlob"] = change.sourceBlob
	}
	return result
}

func readRangeChanges(ctx context.Context, index *Index, baseTree string) ([]rangeChange, error) {
	raw, err := git(ctx, index.Root, maxStatusBytes, nil,
		"-c", "diff.algorithm=myers", "-c", "diff.indentHeuristic=false", "-c", "diff.renameLimit=200000",
		"diff-tree", "--no-commit-id", "--raw", "-r", "-z", "--find-renames", "--find-copies-harder", baseTree, index.Revision)
	if err != nil {
		return nil, classifyMissingObjects(ctx, index.Root, err, gitStderr(err), baseTree, index.Revision)
	}
	return parseRangeChanges(raw, index)
}

func parseRangeChanges(raw []byte, index *Index) ([]rangeChange, error) {
	fields := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	if len(raw) == 0 {
		return nil, nil
	}
	changes := make([]rangeChange, 0)
	for position := 0; position < len(fields); {
		metadata := strings.Fields(string(fields[position]))
		position++
		if len(metadata) != 5 || len(metadata[0]) != 7 || metadata[0][0] != ':' || !validObjectID(metadata[2], index.ObjectFormat) || !validObjectID(metadata[3], index.ObjectFormat) {
			return nil, &Error{Code: "unsupported-impact-range", Message: "Git raw range status is malformed"}
		}
		statusCode := metadata[4]
		if statusCode == "" {
			return nil, &Error{Code: "unsupported-impact-range", Message: "Git raw range status is malformed"}
		}
		status := statusCode[:1]
		similarity := 0
		sourcePath := ""
		sourceBlob := ""
		if status == "C" || status == "R" {
			var scoreErr error
			similarity, scoreErr = strconv.Atoi(statusCode[1:])
			if len(statusCode) != 4 || scoreErr != nil || similarity < 0 || similarity > 100 {
				return nil, &Error{Code: "unsupported-impact-range", Message: "Git copy or rename similarity is malformed"}
			}
			if position >= len(fields) || !utf8.Valid(fields[position]) {
				return nil, &Error{Code: "unsupported-impact-range", Message: "Git copy or rename source is missing"}
			}
			sourcePath = string(fields[position])
			sourceBlob = metadata[2]
			position++
			if err := validateRangePath(sourcePath); err != nil {
				return nil, err
			}
		} else if len(statusCode) != 1 {
			return nil, &Error{Code: "unsupported-impact-range", Message: "Git raw range status is malformed"}
		}
		if position >= len(fields) || !utf8.Valid(fields[position]) {
			return nil, &Error{Code: "unsupported-impact-range", Message: "Git range status is malformed"}
		}
		changedPath := string(fields[position])
		position++
		if err := validateRangePath(changedPath); err != nil {
			return nil, err
		}
		changes = append(changes, rangeChange{
			status: status, path: changedPath, oldMode: metadata[0][1:], newMode: metadata[1],
			oldBlob: metadata[2], newBlob: metadata[3], similarity: similarity,
			sourcePath: sourcePath, sourceBlob: sourceBlob,
		})
	}
	sort.Slice(changes, func(left, right int) bool { return changes[left].path < changes[right].path })
	return changes, nil
}

func validateRangePath(candidate string) error {
	if _, err := cleanImpactPath(candidate); err != nil {
		return &Error{Code: "unsupported-impact-range", Message: "Git range contains an unsupported path"}
	}
	if !rangePathPattern.MatchString(candidate) {
		return &Error{Code: "unsupported-impact-range", Message: "Git range contains a path unsafe for verification planning"}
	}
	return nil
}

func rejectRangeBinary(ctx context.Context, index *Index, baseTree string) error {
	raw, err := git(ctx, index.Root, maxStatusBytes, nil,
		"-c", "diff.algorithm=myers", "-c", "diff.indentHeuristic=false",
		"diff", "--numstat", "-z", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-indent-heuristic", baseTree, index.Revision)
	if err != nil {
		return classifyMissingObjects(ctx, index.Root, err, gitStderr(err), baseTree, index.Revision)
	}
	for _, row := range bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0}) {
		if len(row) == 0 {
			continue
		}
		fields := bytes.SplitN(row, []byte{'\t'}, 3)
		if len(fields) != 3 {
			return &Error{Code: "unsupported-impact-range", Message: "Git range numstat is malformed"}
		}
		if bytes.Equal(fields[0], []byte("-")) || bytes.Equal(fields[1], []byte("-")) {
			return &Error{Code: "unsupported-impact-range", Message: "impact range contains binary content"}
		}
	}
	return nil
}

type rangeHunkRead struct {
	hunks []rangeHunk
	err   error
}

// readRangeHunksFor runs the independent per-path hunk diffs on up to four
// Git processes. The caller judges the reads in path order, so the first
// failure it returns is the one a sequential read would have stopped at.
func readRangeHunksFor(ctx context.Context, index *Index, baseTree string, changeByPath map[string]rangeChange, goPaths []string) []rangeHunkRead {
	reads := make([]rangeHunkRead, len(goPaths))
	workers := min(4, runtime.NumCPU(), max(1, len(goPaths)))
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		go func(worker int) {
			defer pending.Done()
			for position := worker; position < len(goPaths); position += workers {
				change := changeByPath[goPaths[position]]
				reads[position].hunks, reads[position].err = readRangeHunks(ctx, index, baseTree, change)
			}
		}(worker)
	}
	pending.Wait()
	return reads
}

func readRangeHunks(ctx context.Context, index *Index, baseTree string, change rangeChange) ([]rangeHunk, error) {
	arguments := []string{
		"-c", "diff.algorithm=myers", "-c", "diff.indentHeuristic=false",
		"diff", "--unified=0", "--no-indent-heuristic", "--no-ext-diff", "--no-textconv", "--no-renames",
	}
	if change.sourcePath == "" {
		arguments = append(arguments, baseTree, index.Revision, "--", change.path)
	} else {
		arguments = append(arguments, change.sourceBlob, change.newBlob)
	}
	raw, err := git(ctx, index.Root, maxSourceBytes*2+maxGitErrorBytes, nil, arguments...)
	if err != nil {
		return nil, classifyMissingObjects(ctx, index.Root, err, gitStderr(err), baseTree, index.Revision)
	}
	result := make([]rangeHunk, 0)
	for _, line := range strings.Split(string(raw), "\n") {
		match := rangeHunkPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		oldStart, _ := strconv.Atoi(match[1])
		oldLines := 1
		if match[2] != "" {
			oldLines, _ = strconv.Atoi(match[2])
		}
		newStart, _ := strconv.Atoi(match[3])
		newLines := 1
		if match[4] != "" {
			newLines, _ = strconv.Atoi(match[4])
		}
		result = append(result, rangeHunk{path: change.path, oldStart: oldStart, oldLines: oldLines, newStart: newStart, newLines: newLines})
	}
	return result, nil
}

func lineInRangeHunks(line int, spans []rangeHunk) bool {
	for _, span := range spans {
		if span.newLines != 0 && line >= span.newStart && line < span.newStart+span.newLines {
			return true
		}
	}
	return false
}

func qualifyRangePathEvidence(results []map[string]any, index *Index, spans map[string][]rangeHunk, changeByPath map[string]rangeChange) {
	for _, result := range results {
		if result["kind"] != "path" {
			continue
		}
		changedPath := stringValue(result["id"])
		source := index.Sources[changedPath]
		hunkEvidence := make([]any, 0, min(len(spans[changedPath]), maxEvidence))
		for _, hunk := range spans[changedPath][:min(len(spans[changedPath]), maxEvidence)] {
			line := hunk.newStart
			if line < 1 {
				line = 1
			}
			hunkEvidence = append(hunkEvidence, evidence(changedPath, line, source.BlobHash,
				fmt.Sprintf("committed diff hunk -%d,%d +%d,%d", hunk.oldStart, hunk.oldLines, hunk.newStart, hunk.newLines),
				"authoritative", "git-diff-hunk"))
		}
		change := changeByPath[changedPath]
		if len(hunkEvidence) == 0 && change.sourcePath != "" {
			hunkEvidence = append(hunkEvidence, evidence(changedPath, 1, source.BlobHash,
				fmt.Sprintf("committed Git %s%03d from %s at %s", change.status, change.similarity, change.sourcePath, change.sourceBlob),
				"authoritative", "git-range-member"))
		}
		result["evidence"] = hunkEvidence
		result["summary"] = "direct committed Go diff path"
	}
}

func compileRangeResults(index *Index, goPaths []string, spans map[string][]rangeHunk, changeByPath map[string]rangeChange, authored map[string]struct{}) ([]map[string]any, []any) {
	results := make([]map[string]any, 0, len(goPaths))
	for _, changedPath := range goPaths {
		results = append(results, map[string]any{
			"kind": "path", "id": changedPath, "score": 1000,
			"summary": "direct committed Go diff path", "evidence": []any{},
		})
	}
	qualifyRangePathEvidence(results, index, spans, changeByPath)
	filtered := *index
	filtered.Markers = make(map[string][]Marker)
	for key, markers := range index.Markers {
		for _, marker := range markers {
			if lineInRangeHunks(marker.Line, spans[marker.Path]) {
				filtered.Markers[key] = append(filtered.Markers[key], marker)
			}
		}
	}
	ledgerUncertainty := make([]any, 0)
	for _, key := range sortedRangeKeys(filtered.Markers) {
		kind, identifier := splitRelation(key)
		records := filtered.Features
		if kind == "scenario" {
			records = filtered.Scenarios
		}
		if record, exists := records[identifier]; exists {
			_, selfAuthored := authored[record.Path]
			results = append(results, rangeMarkerResult(record, filtered.Markers[key], selfAuthored))
			if selfAuthored {
				ledgerUncertainty = append(ledgerUncertainty, kind+":"+identifier+
					" is declared by "+record.Path+", which the same change set authors, so its canonical-ledger authority is withheld")
			}
		}
	}
	decisions, uncertainty := citedRangeDecisions(index, spans, authored)
	return append(results, decisions...), append(ledgerUncertainty, uncertainty...)
}

// rangeMarkerResult reports one canonical ledger record a changed hunk's marker
// selects.  When the caller's own change set authors the ledger file, the record
// is caller-controlled at target HEAD: `CF-V0-031` forbids emitting
// `authoritative` confidence or a canonical authority label for it, so the
// record is still reported but its authority is withheld and named.
func rangeMarkerResult(record Record, markers []Marker, selfAuthoredLedger bool) map[string]any {
	ledgerConfidence, ledgerAuthority := "authoritative", "canonical-ledger"
	if selfAuthoredLedger {
		ledgerConfidence, ledgerAuthority = "low", UnverifiedLedgerAuthority
	}
	resultEvidence := []any{evidence(record.Path, record.Line, record.BlobHash,
		"canonical ledger record selected by exact changed-hunk marker", ledgerConfidence, ledgerAuthority)}
	for _, marker := range markers {
		if len(resultEvidence) >= maxEvidence {
			break
		}
		confidence, authority := "medium", "source-marker"
		if isTestPath(marker.Path) {
			confidence, authority = "high", "test-marker"
		}
		resultEvidence = append(resultEvidence, evidence(marker.Path, marker.Line, marker.BlobHash,
			"exact changed-hunk "+record.Kind+":"+record.ID+" marker", confidence, authority))
	}
	return map[string]any{
		"kind": record.Kind, "id": record.ID, "score": 850,
		"area":    valueOr(record.Fields["area"], ""),
		"summary": truncateRunes(pythonString(valueOr(record.Fields["summary"], "")), 500),
		"status":  valueOr(record.Fields["status"], ""), "adr": []any{},
		"applies": recordSequenceValue(record.Fields["applies"]), "evidence": resultEvidence,
	}
}

type rangeCitation struct {
	path string
	line int
}

// callerAuthoredPaths is the exact path set the caller's own change set introduces
// or modifies between the immutable base and the captured HEAD. A governing
// document inside that set is caller-controlled at target HEAD, so its declared
// status is not an independent authority over the same change set (`CF-V0-031`).
func callerAuthoredPaths(changes []rangeChange) map[string]struct{} {
	authored := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		authored[change.path] = struct{}{}
	}
	return authored
}

func citedRangeDecisions(index *Index, spans map[string][]rangeHunk, authored map[string]struct{}) ([]map[string]any, []any) {
	identifiers := make(map[string]rangeCitation)
	for _, changedPath := range sortedRangeKeys(spans) {
		pathSpans := spans[changedPath]
		text, valid, loaded := index.Sources[changedPath].Text()
		if !loaded || !valid {
			continue
		}
		for lineIndex, line := range strings.Split(text, "\n") {
			if !lineInRangeHunks(lineIndex+1, pathSpans) {
				continue
			}
			for _, match := range rangeADRPattern.FindAllStringSubmatch(line, -1) {
				if _, exists := identifiers[match[1]]; !exists {
					identifiers[match[1]] = rangeCitation{path: changedPath, line: lineIndex + 1}
				}
			}
		}
	}
	result := make([]map[string]any, 0, len(identifiers))
	uncertainty := make([]any, 0)
	identifierValues := sortedRangeKeys(identifiers)
	for _, identifier := range identifierValues {
		citation := identifiers[identifier]
		prefix := "docs/adr/" + identifier + "-"
		matches := make([]Record, 0, 1)
		for _, document := range index.Documents {
			if strings.HasPrefix(document.Path, prefix) && strings.HasSuffix(document.Path, ".md") {
				matches = append(matches, document)
			}
		}
		if len(matches) != 1 {
			uncertainty = append(uncertainty, "ADR-"+identifier+" cited by a changed hunk is missing or ambiguous at target HEAD")
			continue
		}
		document := matches[0]
		if _, selfAuthored := authored[document.Path]; selfAuthored {
			uncertainty = append(uncertainty, "ADR-"+identifier+" cited by a changed hunk is authored by the same change set and confers no authority")
			continue
		}
		status := stringValue(document.Fields["status"])
		if status != "accepted" {
			uncertainty = append(uncertainty, "ADR-"+identifier+" cited by a changed hunk is not an accepted target-HEAD authority")
			continue
		}
		result = append(result, map[string]any{
			"kind": document.Kind, "id": document.ID, "score": 900,
			"title": stringValue(document.Fields["title"]), "summary": stringValue(document.Fields["summary"]),
			"status": status, "references": []any{},
			"evidence": []any{
				evidence(citation.path, citation.line, index.Sources[citation.path].BlobHash,
					"changed diff hunk explicitly cites ADR-"+identifier, "authoritative", "git-diff-hunk"),
				evidence(document.Path, document.Line, document.BlobHash,
					"accepted ADR-"+identifier+" resolved at target HEAD", "authoritative", "accepted-contract"),
			},
		})
	}
	return result, uncertainty
}

func rangeVerification(goPaths []string, index *Index, moduleChanged bool) []any {
	commands := make([]any, 0)
	seen := make(map[string]struct{})
	for _, changedPath := range goPaths {
		parent := path.Dir(changedPath)
		command := "go test ./" + parent + "/..."
		if _, exists := seen[command]; !exists {
			seen[command] = struct{}{}
			commands = append(commands, command)
		}
	}
	if moduleChanged {
		commands = append(commands, "go mod verify")
	}
	final := projectprofile.ByID(index.ProfileID).Gate
	if _, exists := seen[final]; !exists {
		commands = append(commands, final)
	}
	return commands
}

func deduplicateAndSortRangeResults(results []map[string]any) []map[string]any {
	deduplicated := make(map[string]map[string]any)
	for _, result := range results {
		key := selector(result["kind"], result["id"])
		if existing, ok := deduplicated[key]; ok {
			mergeEvidence(existing, result)
			continue
		}
		deduplicated[key] = result
	}
	ordered := make([]map[string]any, 0, len(deduplicated))
	for _, result := range deduplicated {
		ordered = append(ordered, result)
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		leftScore, _ := ordered[left]["score"].(int)
		rightScore, _ := ordered[right]["score"].(int)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return selector(ordered[left]["kind"], ordered[left]["id"]) < selector(ordered[right]["kind"], ordered[right]["id"])
	})
	return ordered
}

func mapsFromAny(value any) []map[string]any {
	values := anySlice(value)
	result := make([]map[string]any, 0, len(values))
	for _, item := range values {
		result = append(result, item.(map[string]any))
	}
	return result
}

func sortedRangeKeys[Value any](values map[string]Value) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
