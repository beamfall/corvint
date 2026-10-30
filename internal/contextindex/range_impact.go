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
	// maxRangeCopyProbePathspecBytes bounds the argument list of the lower
	// bound's copy-source probe; a larger range defers to the exact diff.
	maxRangeCopyProbePathspecBytes = 256 << 10
	maxRangeHunks                  = 10_000
	maxRangeTargetLines            = 200_000
)

var (
	rangeHunkPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
	rangeADRPattern  = regexp.MustCompile(`\bADR-([0-9]{4})\b`)
	// rangePathPattern admits '@' and '+' (V1-2796, ERI-V0-263): neither is
	// Git pathspec magic or a glob, nor a POSIX shell metacharacter, so a
	// planned command naming the path needs no quoting. A Go package
	// directory with '@' is refused separately, since a go test pattern reads
	// it as a version query.
	rangePathPattern = regexp.MustCompile(`^[A-Za-z0-9._/@+-]+$`)
)

// rangePathRule names the characters rangePathPattern admits, for refusals.
const rangePathRule = "allowed characters are ASCII letters, digits, '.', '_', '/', '-', '@' and '+'"

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
	// refusal is the range path validator's verdict on the source path, else
	// on path; nil when both validate (ERI-V0-264).
	refusal *Error
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

// PerPathRangeImpact is RangeImpact for a caller that reports every changed
// path on its own (the witness report, ERI-V0-264). A changed path the default
// profile refuses for itself alone -- an unsafe path, a deletion or other
// unsupported status, a mode change, an unsupported file mode, binary
// content, a root-package or '@'-directory Go path, a path the target index
// did not admit, invalid Go syntax, or no textual hunk -- is named under
// refusedPaths with its rule and left out of the analysis instead of refusing
// the range. Every repository-level refusal, bound and drift check is
// unchanged. With no refused path the receipt is byte-identical to
// RangeImpact's. The receipt plans no verification for a refused path, so it
// is not a verification plan for the whole range.
func PerPathRangeImpact(ctx context.Context, index *Index, base string, limit int) (map[string]any, error) {
	return compileRangeImpactMode(ctx, index, base, limit, false, true, false)
}

// ExpandedPerPathRangeImpact is PerPathRangeImpact under the experimental
// fixed-256-path profile (ERI-V0-305).
func ExpandedPerPathRangeImpact(ctx context.Context, index *Index, base string, limit int) (map[string]any, error) {
	return compileRangeImpactMode(ctx, index, base, limit, true, true, false)
}

// CommitPinnedPerPathRangeImpact is PerPathRangeImpact over an index built at
// an explicit commit by BuildRevisionContext, which need not be checked out
// (ERI-V0-265). It reads only the Git object store: no worktree, status or
// untracked-path check applies, and the drift checks instead re-verify that
// the pinned commit still names the tree the index read and that base is
// still its ancestor. The range binding's status is COMMIT-PINNED and carries
// no status digest; every other field, bound and refusal is PerPathRangeImpact's.
func CommitPinnedPerPathRangeImpact(ctx context.Context, index *Index, base string, limit int) (map[string]any, error) {
	return compileRangeImpactMode(ctx, index, base, limit, false, true, true)
}

func compileRangeImpact(ctx context.Context, index *Index, base string, limit int, expanded bool) (map[string]any, error) {
	return compileRangeImpactMode(ctx, index, base, limit, expanded, false, false)
}

// maxPerPathRangeRecords bounds the exact copy-detecting diff the per-path
// profile reads; a range past it still refuses whole, while one past the path
// bound alone refuses its later paths each on their own (ERI-V0-285).
const maxPerPathRangeRecords = 1_000

func compileRangeImpactMode(ctx context.Context, index *Index, base string, limit int, expanded, perPath, pinned bool) (map[string]any, error) {
	if limit < 1 || limit > maxLimit {
		return nil, &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	if !validObjectID(base, index.ObjectFormat) {
		return nil, &Error{Code: "unsupported-impact-range", Message: "--base must be a full immutable commit object ID"}
	}
	// Preserve the historical no-module refusal before worktree validation in
	// repositories outside the new language capability (including first-pass CEMs).
	if (index.Module == "" || !strings.Contains(index.Module, "/")) && !hasNonGoImpactSource(index) {
		return nil, &Error{Code: "unsupported-impact-repository", Message: "native Go impact requires a slash-qualified Go module"}
	}
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	verify := func() ([]string, error) {
		if pinned {
			return nil, verifyPinnedRange(ctx, index)
		}
		return verifyRangeSnapshot(ctx, index)
	}
	allowedUntracked, err := verify()
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
	pathLimit := maxImpactPaths
	if expanded {
		pathLimit = maxExpandedRangePaths
	}
	// V1-1881: copy detection over every base-tree file dominates an
	// over-bound refusal, so a sound lower bound refuses first, with the same
	// error, when the exact diff below could only refuse the same way.
	// Per path, an unsafe path refuses only itself, so only the bound
	// refusal is proved early.
	if perPath {
		if raw, err := git(ctx, index.Root, maxStatusBytes, nil, "diff-tree", "--no-commit-id", "--raw", "-r", "-z", "--no-renames", baseTree, index.Revision); err == nil &&
			rangeCertainlyExceeds(ctx, index, baseTree, raw, maxPerPathRangeRecords) {
			return nil, rangePathBoundError(maxPerPathRangeRecords)
		}
	} else if refusal := rangeCertainRefusal(ctx, index, baseTree, pathLimit); refusal != nil {
		return nil, refusal
	}
	var changes []rangeChange
	if perPath {
		changes, err = readRangeChangeRecords(ctx, index, baseTree)
	} else {
		changes, err = readRangeChangesFor(ctx, index, baseTree)
	}
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
	// Per path, the first pathLimit changed paths in path order are analysed
	// and every later one is refused alone as PATH_BOUND (ERI-V0-285).
	pastBound := make(map[string]bool)
	if len(changes) > pathLimit {
		if !perPath {
			return nil, rangePathBoundError(pathLimit)
		}
		if len(changes) > maxPerPathRangeRecords {
			return nil, rangePathBoundError(maxPerPathRangeRecords)
		}
		ordered := make([]string, 0, len(changes))
		for _, change := range changes {
			ordered = append(ordered, change.path)
		}
		sort.Strings(ordered)
		for _, changedPath := range ordered[pathLimit:] {
			pastBound[changedPath] = true
		}
	}
	binaryPaths, err := rangeBinaryPaths(ctx, index, baseTree)
	if err != nil {
		return nil, err
	}
	if len(binaryPaths) != 0 && !perPath {
		return nil, rangeBinaryError(sortedRangeKeys(binaryPaths)[0])
	}
	// Per path, a binary path no change carries -- as its path or as a rename
	// source -- would otherwise vanish, so it still refuses the range.
	for _, binaryPath := range sortedRangeKeys(binaryPaths) {
		carried := false
		for _, change := range changes {
			if change.path == binaryPath || (change.status == "R" && change.sourcePath == binaryPath) {
				carried = true
				break
			}
		}
		if !carried {
			return nil, rangeBinaryError(binaryPath)
		}
	}

	admittedPaths := make([]string, 0, len(changes))
	goPaths := make([]string, 0, len(changes))
	omissions := make([]any, 0)
	// refusedPaths names, per path, what a per-path caller would otherwise
	// have refused the whole range for (ERI-V0-264).
	refusedPaths := make([]any, 0)
	refuse := func(changedPath, rule string, refusal *Error) error {
		if !perPath {
			return refusal
		}
		refusedPaths = append(refusedPaths, map[string]any{"path": changedPath, "rule": rule, "detail": refusal.Message})
		return nil
	}
	moduleChanged := false
	for _, change := range changes {
		var rule string
		var refusal *Error
		switch {
		case pastBound[change.path]:
			rule, refusal = "PATH_BOUND", &Error{Code: "unsupported-impact-range", Message: fmt.Sprintf("impact range exceeds the %d-path bound and %s sorts after the first %d changed paths", pathLimit, change.path, pathLimit)}
		case change.refusal != nil:
			rule, refusal = "UNSAFE_PATH", change.refusal
		case change.status == "D":
			rule, refusal = "DELETED", &Error{Code: "unsupported-impact-range", Message: "impact range contains unsupported status D for " + change.path}
		case change.status != "M" && change.status != "A" && change.status != "C" && change.status != "R":
			rule, refusal = "UNSUPPORTED_STATUS", &Error{Code: "unsupported-impact-range", Message: "impact range contains unsupported status " + change.status + " for " + change.path}
		case change.status != "A" && change.oldMode != change.newMode:
			rule, refusal = "MODE_CHANGE", &Error{Code: "unsupported-impact-range", Message: "impact range contains a mode change for " + change.path}
		case change.newMode != "100644" && change.newMode != "100755":
			rule, refusal = "UNSUPPORTED_FILE_MODE", &Error{Code: "unsupported-impact-range", Message: "impact range contains a symlink, submodule, or unsupported file mode for " + change.path}
		case binaryPaths[change.path] || (change.status == "R" && binaryPaths[change.sourcePath]):
			rule, refusal = "BINARY_CONTENT", rangeBinaryError(change.path)
		}
		if refusal != nil {
			if err := refuse(change.path, rule, refusal); err != nil {
				return nil, err
			}
			continue
		}
		if change.path == "go.mod" || change.path == "go.sum" {
			moduleChanged = true
		}
		if !MCPImpactPathAdmitted(change.path) {
			omissions = append(omissions, map[string]any{"path": change.path, "reason": "non-Go path outside native Go range profile"})
			continue
		}
		isGo := strings.HasSuffix(change.path, ".go")
		pathKind := "changed syntax path"
		if isGo {
			pathKind = "changed Go path"
		}
		if isGo && (index.Module == "" || !strings.Contains(index.Module, "/")) {
			return nil, &Error{Code: "unsupported-impact-repository", Message: "native Go impact requires a slash-qualified Go module"}
		}
		if isGo && path.Dir(change.path) == "." {
			if err := refuse(change.path, "ROOT_GO_PACKAGE", &Error{Code: "unsupported-impact-path", Message: "native Go range impact requires changed Go paths in a non-root package"}); err != nil {
				return nil, err
			}
			continue
		}
		if isGo && strings.Contains(path.Dir(change.path), "@") {
			if err := refuse(change.path, "GO_PACKAGE_DIRECTORY_AT", &Error{Code: "unsupported-impact-path",
				Message: "changed Go path's package directory contains '@', which a go test pattern reads as a version query: " + change.path}); err != nil {
				return nil, err
			}
			continue
		}
		source, ok := index.Sources[change.path]
		if !ok {
			if err := refuse(change.path, "NOT_INDEXED", &Error{Code: "unsupported-impact-range", Message: pathKind + " is not admitted by the target index: " + change.path}); err != nil {
				return nil, err
			}
			continue
		}
		if source.BlobHash != change.newBlob {
			return nil, &Error{Code: "impact-range-drift", Message: pathKind + " does not match the captured target blob: " + change.path}
		}
		if isGo {
			if _, parseErr := parser.ParseFile(token.NewFileSet(), change.path, source.Data, parser.AllErrors); parseErr != nil {
				if err := refuse(change.path, "INVALID_GO_SYNTAX", &Error{Code: "unsupported-impact-range", Message: "changed Go path is not valid Go syntax: " + change.path}); err != nil {
					return nil, err
				}
				continue
			}
			goPaths = append(goPaths, change.path)
		}
		admittedPaths = append(admittedPaths, change.path)
	}

	hunks := make([]rangeHunk, 0)
	spans := make(map[string][]rangeHunk, len(admittedPaths))
	targetLineCount := 0
	pathReads := readRangeHunksFor(ctx, index, baseTree, changeByPath, admittedPaths)
	hunkless := make(map[string]bool)
	for position, changedPath := range admittedPaths {
		change := changeByPath[changedPath]
		pathHunks, hunkErr := pathReads[position].hunks, pathReads[position].err
		if hunkErr != nil {
			return nil, hunkErr
		}
		if len(pathHunks) == 0 && change.sourcePath == "" {
			pathKind := "changed syntax path"
			if strings.HasSuffix(changedPath, ".go") {
				pathKind = "changed Go path"
			}
			if err := refuse(changedPath, "NO_TEXT_HUNKS", &Error{Code: "unsupported-impact-range", Message: pathKind + " has no textual diff hunks: " + changedPath}); err != nil {
				return nil, err
			}
			hunkless[changedPath] = true
			continue
		}
		pathLines := 0
		for _, hunk := range pathHunks {
			pathLines += hunk.newLines
		}
		// Per path, a path whose hunks would cross a range bound is refused
		// alone and the later paths still fit what remains (ERI-V0-285).
		if perPath && (len(hunks)+len(pathHunks) > maxRangeHunks || targetLineCount+pathLines > maxRangeTargetLines) {
			rule, message := "HUNK_BOUND", fmt.Sprintf("impact range exceeds the %d-hunk bound at %s", maxRangeHunks, changedPath)
			if len(hunks)+len(pathHunks) <= maxRangeHunks {
				rule, message = "TARGET_LINE_BOUND", fmt.Sprintf("impact range exceeds the %d-target-line bound at %s", maxRangeTargetLines, changedPath)
			}
			if err := refuse(changedPath, rule, &Error{Code: "unsupported-impact-range", Message: message}); err != nil {
				return nil, err
			}
			hunkless[changedPath] = true
			continue
		}
		hunks = append(hunks, pathHunks...)
		targetLineCount += pathLines
		if len(hunks) > maxRangeHunks {
			return nil, &Error{Code: "unsupported-impact-range", Message: fmt.Sprintf("impact range exceeds the %d-hunk bound", maxRangeHunks)}
		}
		if targetLineCount > maxRangeTargetLines {
			return nil, &Error{Code: "unsupported-impact-range", Message: fmt.Sprintf("impact range exceeds the %d-target-line bound", maxRangeTargetLines)}
		}
		spans[changedPath] = pathHunks
	}
	if len(hunkless) != 0 {
		admittedPaths = removeRangePaths(admittedPaths, hunkless)
		goPaths = removeRangePaths(goPaths, hunkless)
	}
	if _, err := verify(); err != nil {
		return nil, err
	}

	rawResults, authorityUncertainty := compileRangeResults(index, admittedPaths, spans, changeByPath, callerAuthoredPaths(changes))
	rubyConstants, rubyUnknowns, rawResults := rangeRubyAutoload(index, admittedPaths, rawResults)

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
	if pinned {
		rangeBinding["status"] = "COMMIT-PINNED"
		delete(rangeBinding, "statusSha256")
	}
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
	setVerification(result, index, rangeVerification(goPaths, admittedPaths, index, moduleChanged))
	coverage := result["coverage"].(map[string]any)
	// The result counts are left to `receipt`, which now denominates them in
	// the same pre-truncation `rawResults` this function would restate.  They
	// were restated here only because that arithmetic was structurally zero;
	// keeping a second writer of the same three fields buys nothing and lets
	// the two drift apart.  The critical set below is a genuine override: it
	// names every changed Go path, including one the ranking ceiling dropped,
	// which a set derived from the surviving results cannot report.
	included := min(len(rawResults), limit)
	critical := make([]any, len(admittedPaths))
	criticalMissing := make([]any, 0)
	includedPaths := make(map[string]struct{})
	for _, item := range mapsFromAny(result["results"]) {
		if item["kind"] == "path" {
			includedPaths[stringValue(item["id"])] = struct{}{}
		}
	}
	for position, changedPath := range admittedPaths {
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
	// A Ruby range whose plan names no language command says so with the line
	// path impact reports for the same files; a range with no admitted Ruby
	// path never evaluates it and keeps its bytes (NGI-V0-059).
	if rangeHasRubyPath(admittedPaths) && lacksLanguageVerification(index, result) {
		uncertainty = append(uncertainty, unknownLanguageVerification)
	}
	uncertainty = append(uncertainty, rubyAdvisoryUncertainty(mapsFromAny(result["results"]))...)
	if len(omissions) != 0 {
		uncertainty = append(uncertainty, fmt.Sprintf("%d non-Go changed paths are outside the native Go range profile", len(omissions)))
	}
	if len(refusedPaths) != 0 {
		result["refusedPaths"] = refusedPaths
		uncertainty = append(uncertainty, fmt.Sprintf("%d changed paths were refused per path and are not analysed; this receipt plans no verification for them", len(refusedPaths)))
	}
	if len(rawResults) > included {
		uncertainty = append(uncertainty, fmt.Sprintf("%d ranked impact results are omitted by the result limit", len(rawResults)-included))
	}
	if line := evidenceTruncatedDisclosure(mapsFromAny(result["results"])); line != "" {
		uncertainty = append(uncertainty, line)
	}
	coverage["uncertainty"] = uncertainty
	if len(criticalMissing) != 0 {
		result["state"] = "BUDGETED"
	}
	if err := verifyRangeBase(ctx, index, base, baseTree, verify); err != nil {
		return nil, err
	}
	if err := attachNonGoImpactUnknowns(result, index, admittedPaths, 0, rubyUnknowns, rubyConstants); err != nil {
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

// verifyPinnedRange is the commit-pinned drift check (ERI-V0-265). The index
// was built from objects at index.CommitRevision, so the only state that can
// drift is the object store itself: the commit must still resolve to itself
// as a commit and to the tree the index read. Any Git failure refuses.
func verifyPinnedRange(ctx context.Context, index *Index) error {
	if !validObjectID(index.CommitRevision, index.ObjectFormat) || !validObjectID(index.Revision, index.ObjectFormat) {
		return &Error{Code: "unsupported-impact-range", Message: "a commit-pinned range needs a full head commit and tree identity"}
	}
	resolved, err := git(ctx, index.Root, maxIdentityBytes, nil, "rev-parse", index.CommitRevision+"^{commit}", index.CommitRevision+"^{tree}")
	if err != nil || string(resolved) != index.CommitRevision+"\n"+index.Revision+"\n" {
		return &Error{Code: "impact-range-drift", Message: "the pinned head commit or its tree is unavailable or changed while compiling impact range"}
	}
	return nil
}

func verifyRangeBase(ctx context.Context, index *Index, base, baseTree string, verify func() ([]string, error)) error {
	if _, err := verify(); err != nil {
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

func rangePathBoundError(pathLimit int) error {
	return &Error{Code: "unsupported-impact-range", Message: fmt.Sprintf("impact range exceeds the %d-path bound", pathLimit)}
}

// readRangeChangesFor is readRangeChanges; a test replaces it to prove the
// lower-bound refusal never reaches the copy-detecting diff (V1-1881).
var readRangeChangesFor = readRangeChanges

// rangeCertainRefusal returns the refusal the exact range diff
// (readRangeChanges) would end in when it is proved from one rename-free diff,
// or nil when it is not: the path refusal (ERI-V0-262, V1-2221) and otherwise
// the bound refusal (ERI-V0-060). The exact parse refuses before it counts, so
// the path refusal is tried first.
func rangeCertainRefusal(ctx context.Context, index *Index, baseTree string, pathLimit int) error {
	raw, err := git(ctx, index.Root, maxStatusBytes, nil, "diff-tree", "--no-commit-id", "--raw", "-r", "-z", "--no-renames", baseTree, index.Revision)
	if err != nil {
		return nil
	}
	if refusal := rangeCertainPathRefusal(ctx, index, baseTree, raw); refusal != nil {
		return refusal
	}
	if rangeCertainlyExceeds(ctx, index, baseTree, raw, pathLimit) {
		return rangePathBoundError(pathLimit)
	}
	return nil
}

// rangeCertainPathRefusal returns the path refusal the exact range diff must
// end in, without its copy detection (ERI-V0-262, V1-2221). raw is the
// rename-free diff. Every changed path there reaches the exact parse, as a
// changed path or as a rename or copy source, and the exact parse validates
// each; so one changed path that fails validateRangePath proves the exact
// diff refuses. The exact parse refuses with its smallest failing path's
// refusal (ERI-V0-264), so the answer is the smallest failing changed path's,
// and, when there are additions, it is proved only when no base-tree path,
// any of which may be a copy source, fails and sorts below it. The exact diff
// itself must also complete: the repository is not a partial clone, whose
// missing blobs it would report instead, and its output, which can only add
// one source path and a score to each addition, stays within maxStatusBytes.
// A path that is not UTF-8, any other doubt, or a Git failure answers nil.
func rangeCertainPathRefusal(ctx context.Context, index *Index, baseTree string, raw []byte) error {
	fields := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	if len(raw) == 0 || len(fields)%2 != 0 {
		return nil
	}
	var refusal error
	smallest := ""
	additions := 0
	for position := 0; position < len(fields); position += 2 {
		metadata := strings.Fields(string(fields[position]))
		if len(metadata) != 5 || len(metadata[0]) != 7 || metadata[0][0] != ':' || !validObjectID(metadata[2], index.ObjectFormat) ||
			!validObjectID(metadata[3], index.ObjectFormat) || len(metadata[4]) != 1 || !utf8.Valid(fields[position+1]) {
			return nil
		}
		if metadata[4] == "A" {
			additions++
		}
		if changedPath := string(fields[position+1]); refusal == nil || changedPath < smallest {
			if err := validateRangePath(changedPath); err != nil {
				refusal, smallest = err, changedPath
			}
		}
	}
	if refusal == nil || mayLackObjects(ctx, index.Root) {
		return nil
	}
	if additions == 0 {
		return refusal
	}
	listing, err := git(ctx, index.Root, maxStatusBytes, nil, "ls-tree", "-r", "-z", "--name-only", baseTree)
	if err != nil {
		return nil
	}
	longest := 0
	for _, candidate := range bytes.Split(bytes.TrimSuffix(listing, []byte{0}), []byte{0}) {
		if !utf8.Valid(candidate) {
			return nil
		}
		longest = max(longest, len(candidate))
		if string(candidate) < smallest && validateRangePath(string(candidate)) != nil {
			return nil
		}
	}
	// A copy or rename record adds "NNN" to the status and "\0source" to an
	// addition's record; nothing else grows.
	if len(raw)+additions*(longest+4) > maxStatusBytes {
		return nil
	}
	return refusal
}

// rangeCertainlyExceeds reports whether the exact range diff (readRangeChanges)
// must hold more than pathLimit entries and must parse without an earlier
// refusal, so refusing on the bound now returns the bytes it would return
// (ERI-V0-060). Without rename or copy detection the diff reads no blobs.
// Detection never adds an entry: a copy turns an addition into C and a rename
// merges one deletion and one addition into R, so the exact count is at least
// total - min(additions, deletions). The exact parse validates every changed
// path and every rename or copy source. Every changed path validates here; a
// copy source may also be any unchanged base-tree path, so each unchanged path
// that fails validation is offered as the only copy source for the additions,
// and any copy found defers to the exact diff (a source below the similarity
// threshold alone cannot pass it among more candidates). A partial clone may
// lack the blobs detection reads, and the exact diff then reports the missing
// object instead. Any doubt or Git failure answers false, and the exact path
// decides as before.
// raw is the rename-free diff rangeCertainRefusal read.
func rangeCertainlyExceeds(ctx context.Context, index *Index, baseTree string, raw []byte, pathLimit int) bool {
	fields := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	if len(raw) == 0 || len(fields)%2 != 0 {
		return false
	}
	additions := make([]string, 0)
	deletions := 0
	for position := 0; position < len(fields); position += 2 {
		metadata := strings.Fields(string(fields[position]))
		changedPath := string(fields[position+1])
		if len(metadata) != 5 || len(metadata[4]) != 1 || validateRangePath(changedPath) != nil {
			return false
		}
		switch metadata[4] {
		case "A":
			additions = append(additions, changedPath)
		case "D":
			deletions++
		}
	}
	total := len(fields) / 2
	if total-min(len(additions), deletions) <= pathLimit {
		return false
	}
	if mayLackObjects(ctx, index.Root) {
		return false
	}
	listing, err := git(ctx, index.Root, maxStatusBytes, nil, "ls-tree", "-r", "-z", "--name-only", baseTree)
	if err != nil {
		return false
	}
	unsafeSources := make([]string, 0)
	for _, candidate := range bytes.Split(bytes.TrimSuffix(listing, []byte{0}), []byte{0}) {
		if len(candidate) != 0 && validateRangePath(string(candidate)) != nil {
			unsafeSources = append(unsafeSources, string(candidate))
		}
	}
	if len(unsafeSources) == 0 {
		return true
	}
	if len(additions) == 0 {
		return true
	}
	pathspecBytes := 0
	for _, candidate := range append(additions, unsafeSources...) {
		pathspecBytes += len(candidate) + 1
	}
	if pathspecBytes > maxRangeCopyProbePathspecBytes {
		return false
	}
	arguments := append([]string{"--literal-pathspecs", "-c", "diff.renameLimit=200000",
		"diff-tree", "--no-commit-id", "--raw", "-r", "-z", "--find-copies-harder", baseTree, index.Revision, "--"}, additions...)
	copies, err := git(ctx, index.Root, maxStatusBytes, nil, append(arguments, unsafeSources...)...)
	if err != nil {
		return false
	}
	for _, field := range bytes.Split(copies, []byte{0}) {
		if metadata := strings.Fields(string(field)); len(metadata) == 5 && strings.HasPrefix(metadata[0], ":") && metadata[4][0] != 'A' {
			return false
		}
	}
	return true
}

// mayLackObjects reports whether the repository may be a partial clone, whose
// missing blobs the exact diff names as repository-object-unavailable with its
// recovery. Git treats a remote as a promisor through extensions.partialclone,
// remote.<name>.promisor or remote.<name>.partialclonefilter, so any of them,
// and any doubt or Git failure, answers true.
func mayLackObjects(ctx context.Context, root string) bool {
	partial, err := git(ctx, root, maxIdentityBytes, nil, "config", "--default", "", "--get", "extensions.partialclone")
	if err != nil || strings.TrimSpace(string(partial)) != "" {
		return true
	}
	names, err := git(ctx, root, maxStatusBytes, nil, "config", "-z", "--name-only", "--list")
	if err != nil {
		return true
	}
	for _, name := range bytes.Split(names, []byte{0}) {
		key := strings.ToLower(string(name))
		if !strings.HasPrefix(key, "remote.") {
			continue
		}
		if strings.HasSuffix(key, ".partialclonefilter") {
			return true
		}
		if strings.HasSuffix(key, ".promisor") {
			value, err := git(ctx, root, maxIdentityBytes, nil, "config", "--type=bool", "--default", "false", "--get", string(name))
			if err != nil || strings.TrimSpace(string(value)) != "false" {
				return true
			}
		}
	}
	return false
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

// parseRangeChanges parses the exact range diff and refuses when any changed
// path or rename or copy source fails validateRangePath, with the refusal of
// the smallest such path, so the refusal names one path independent of record
// order (ERI-V0-264).
func parseRangeChanges(raw []byte, index *Index) ([]rangeChange, error) {
	changes, err := parseRangeChangeRecords(raw, index)
	if err != nil {
		return nil, err
	}
	var refusal error
	smallest := ""
	for _, change := range changes {
		if change.refusal == nil {
			continue
		}
		for _, candidate := range []string{change.sourcePath, change.path} {
			if candidate == "" || (refusal != nil && candidate >= smallest) {
				continue
			}
			if err := validateRangePath(candidate); err != nil {
				refusal, smallest = err, candidate
			}
		}
	}
	if refusal != nil {
		return nil, refusal
	}
	return changes, nil
}

// readRangeChangeRecords is readRangeChanges for the per-path profile: a path
// that fails validateRangePath annotates its change instead of refusing.
func readRangeChangeRecords(ctx context.Context, index *Index, baseTree string) ([]rangeChange, error) {
	raw, err := git(ctx, index.Root, maxStatusBytes, nil,
		"-c", "diff.algorithm=myers", "-c", "diff.indentHeuristic=false", "-c", "diff.renameLimit=200000",
		"diff-tree", "--no-commit-id", "--raw", "-r", "-z", "--find-renames", "--find-copies-harder", baseTree, index.Revision)
	if err != nil {
		return nil, classifyMissingObjects(ctx, index.Root, err, gitStderr(err), baseTree, index.Revision)
	}
	return parseRangeChangeRecords(raw, index)
}

// parseRangeChangeRecords parses the exact range diff. A malformed record
// refuses; a path that fails validateRangePath sets its change's refusal.
func parseRangeChangeRecords(raw []byte, index *Index) ([]rangeChange, error) {
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
		} else if len(statusCode) != 1 {
			return nil, &Error{Code: "unsupported-impact-range", Message: "Git raw range status is malformed"}
		}
		if position >= len(fields) || !utf8.Valid(fields[position]) {
			return nil, &Error{Code: "unsupported-impact-range", Message: "Git range status is malformed"}
		}
		changedPath := string(fields[position])
		position++
		var refusal *Error
		for _, candidate := range []string{sourcePath, changedPath} {
			if candidate == "" || refusal != nil {
				continue
			}
			if err := validateRangePath(candidate); err != nil {
				refusal = err.(*Error)
			}
		}
		changes = append(changes, rangeChange{
			status: status, path: changedPath, oldMode: metadata[0][1:], newMode: metadata[1],
			oldBlob: metadata[2], newBlob: metadata[3], similarity: similarity,
			sourcePath: sourcePath, sourceBlob: sourceBlob, refusal: refusal,
		})
	}
	sort.Slice(changes, func(left, right int) bool { return changes[left].path < changes[right].path })
	return changes, nil
}

// validateRangePath refuses a path the range profile cannot plan for, naming
// the path and the rule it fails (ERI-V0-263).
func validateRangePath(candidate string) error {
	if _, err := cleanImpactPath(candidate); err != nil {
		return &Error{Code: "unsupported-impact-range", Message: "Git range contains an unsupported path (it must be a non-empty, normalized, repository-relative path within the path-length bound): " + strconv.Quote(candidate)}
	}
	if !rangePathPattern.MatchString(candidate) {
		return &Error{Code: "unsupported-impact-range", Message: "Git range contains a path unsafe for verification planning (" + rangePathRule + "): " + strconv.Quote(candidate)}
	}
	return nil
}

func rangeBinaryError(changedPath string) *Error {
	return &Error{Code: "unsupported-impact-range", Message: "impact range contains binary content: " + strconv.Quote(changedPath)}
}

// rangeBinaryPaths returns the rename-free diff's paths with binary content.
func rangeBinaryPaths(ctx context.Context, index *Index, baseTree string) (map[string]bool, error) {
	raw, err := git(ctx, index.Root, maxStatusBytes, nil,
		"-c", "diff.algorithm=myers", "-c", "diff.indentHeuristic=false",
		"diff", "--numstat", "-z", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-indent-heuristic", baseTree, index.Revision)
	if err != nil {
		return nil, classifyMissingObjects(ctx, index.Root, err, gitStderr(err), baseTree, index.Revision)
	}
	binary := make(map[string]bool)
	for _, row := range bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0}) {
		if len(row) == 0 {
			continue
		}
		fields := bytes.SplitN(row, []byte{'\t'}, 3)
		if len(fields) != 3 {
			return nil, &Error{Code: "unsupported-impact-range", Message: "Git range numstat is malformed"}
		}
		if bytes.Equal(fields[0], []byte("-")) || bytes.Equal(fields[1], []byte("-")) {
			binary[string(fields[2])] = true
		}
	}
	return binary, nil
}

func removeRangePaths(paths []string, removed map[string]bool) []string {
	kept := make([]string, 0, len(paths))
	for _, candidate := range paths {
		if !removed[candidate] {
			kept = append(kept, candidate)
		}
	}
	return kept
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
		noteEvidenceTruncated(result, len(spans[changedPath])-maxEvidence)
		result["summary"] = "direct committed Go diff path"
		if NonGoImpactLanguage(changedPath) != "" {
			result["summary"] = "direct committed syntax diff path"
		}
	}
}

func compileRangeResults(index *Index, goPaths []string, spans map[string][]rangeHunk, changeByPath map[string]rangeChange, authored map[string]struct{}) ([]map[string]any, []any) {
	if rubyImpactRequested(goPaths) {
		index = index.withRubyTests() // NGI-V0-063
	}
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
			results = append(results, rangeMarkerResult(index, record, filtered.Markers[key], selfAuthored))
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
func rangeMarkerResult(index *Index, record Record, markers []Marker, selfAuthoredLedger bool) map[string]any {
	ledgerConfidence, ledgerAuthority := "authoritative", "canonical-ledger"
	if selfAuthoredLedger {
		ledgerConfidence, ledgerAuthority = "low", UnverifiedLedgerAuthority
	}
	resultEvidence := []any{evidence(record.Path, record.Line, record.BlobHash,
		"canonical ledger record selected by exact changed-hunk marker", ledgerConfidence, ledgerAuthority)}
	truncated := 0
	for _, marker := range markers {
		if len(resultEvidence) >= maxEvidence {
			truncated = len(markers) - (len(resultEvidence) - 1)
			break
		}
		confidence, authority := "medium", "source-marker"
		if index.isTest(marker.Path) {
			confidence, authority = "high", "test-marker"
		}
		resultEvidence = append(resultEvidence, evidence(marker.Path, marker.Line, marker.BlobHash,
			"exact changed-hunk "+record.Kind+":"+record.ID+" marker", confidence, authority))
	}
	result := map[string]any{
		"kind": record.Kind, "id": record.ID, "score": 850,
		"area":    valueOr(record.Fields["area"], ""),
		"summary": truncateRunes(pythonString(valueOr(record.Fields["summary"], "")), 500),
		"status":  valueOr(record.Fields["status"], ""), "adr": []any{},
		"applies": recordSequenceValue(record.Fields["applies"]), "evidence": resultEvidence,
	}
	noteEvidenceTruncated(result, truncated)
	return result
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

func rangeVerification(goPaths, admittedPaths []string, index *Index, moduleChanged bool) []any {
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
	// A range with a Ruby path names the repository's Ruby command once, after
	// the Go package commands; a range without one reads nothing and keeps its
	// bytes (NGI-V0-051).
	if rangeHasRubyPath(admittedPaths) {
		if command := rubyVerificationCommand(index); command != "" {
			seen[command] = struct{}{}
			commands = append(commands, command)
		}
	}
	if moduleChanged {
		commands = append(commands, "go mod verify")
	}
	// The range plan closes with the same gates as a path plan over the same
	// tree: declared commands first, else the profile's or Makefile's gate
	// (NGI-V0-062).
	for _, final := range closingGates(index, projectprofile.ByID(index.ProfileID)) {
		if _, exists := seen[final]; !exists {
			seen[final] = struct{}{}
			commands = append(commands, final)
		}
	}
	return commands
}

// rangeRubyAutoload attaches the path impact's Ruby autoload-convention
// reverse references, both hops, to each admitted Ruby path (NGI-V0-037), and
// returns rawResults with them deduplicated and sorted. The rows are ordered
// by score so a path two changed paths reach keeps its highest-ranked row; a
// range with no Ruby path reads nothing and keeps its bytes.
func rangeRubyAutoload(index *Index, admittedPaths []string, rawResults []map[string]any) (*rubyAutoload, map[string][]any, []map[string]any) {
	if !rubyImpactRequested(admittedPaths) {
		return nil, nil, deduplicateAndSortRangeResults(rawResults)
	}
	autoload := loadRubyAutoload(index)
	unknowns := map[string][]any{}
	ranked := []rankedResult{}
	for _, changedPath := range admittedPaths {
		if NonGoImpactLanguage(changedPath) != "ruby" {
			continue
		}
		rows, pathUnknowns := autoload.impact(index, changedPath)
		ranked = append(ranked, rows...)
		unknowns[changedPath] = pathUnknowns
	}
	sort.SliceStable(ranked, func(left, right int) bool { return ranked[left].score > ranked[right].score })
	for _, row := range ranked {
		rawResults = append(rawResults, row.result)
	}
	return autoload, unknowns, deduplicateAndSortRangeResults(rawResults)
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
