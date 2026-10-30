package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/coverprofile"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CoverOptions attach one local coverprofile to every hunk of a map.
type CoverOptions struct {
	MapPath      string
	Coverprofile string
	TestRun      string
	Output       string
	// StatesOutput, when set, also writes the additive cem-coverage-states/1
	// document (CAX-V0-010) at this repository-relative path; empty writes
	// nothing beyond the map, exactly as before.
	StatesOutput string
}

// Cover records a coverage witness on every hunk (TCQ-V0-051..054). The
// coverprofile is one explicit operator-named local file: Corvint neither
// discovers profiles nor runs tests here. Covered ranges are the hunk's added
// lines that a block with a positive count reaches (diff-cover semantics); a
// hunk none reach carries an explicit uncovered witness rather than none.
// A cem/1.0 producer map carries the witness vocabulary (CEM-V1-144).
func (s *Session) Cover(ctx context.Context, options CoverOptions) (map[string]any, error) {
	if err := wire.ValidateTestRun(options.TestRun); err != nil {
		return nil, invalidArguments("--test-run %s", strings.TrimPrefix(err.Error(), "coverage "))
	}
	if err := checkStatesOutput(options); err != nil {
		return nil, err
	}
	document, unlock, err := s.lockedMapInput("cover", options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	defer unlock()
	raw, err := s.readCoverprofile(options.Coverprofile)
	if err != nil {
		return nil, err
	}
	mode, blocks, err := coverprofile.Parse(raw)
	if err != nil {
		return nil, cemcode.New(cemcode.InvalidArguments, "coverprofile: %s", err.Error())
	}
	digest := sha256.Sum256(raw)
	profileSha256 := hex.EncodeToString(digest[:])
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	patchBytes, err := s.citePatch(ctx, document)
	if err != nil {
		return nil, err
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		return nil, err
	}
	coveredLines := coveredIntervalsByPath(blocks)
	covered, uncovered := 0, 0
	// notInstrumented names the uncovered hunks whose path no profile block
	// names at all, apart from an instrumented hunk no block reaches; the map
	// witness stays uncovered for both (TCQ-V0-091).
	notInstrumented := []string{}
	instrumented := profilePaths(blocks)
	states := make([]wire.CoverageStateHunk, 0, len(document.Hunks))
	for index := range document.Hunks {
		if ctx.Err() != nil {
			return nil, cemcode.New(cemcode.GitCancelled, "coverage witness recording cancelled")
		}
		hunk := &document.Hunks[index]
		lines, err := profileLinesFor(coveredLines, hunk.Path)
		if err != nil {
			return nil, err
		}
		ranges := coveredRanges(addedLines(parsed, hunk.ID), lines)
		witness := wire.CoverageWitness{
			ProfileSha256: profileSha256, TestRun: options.TestRun, Mode: string(mode),
			State: wire.CoverageUncovered, Covered: ranges,
		}
		state := wire.CoverageUncovered
		if len(ranges) > 0 {
			witness.State = wire.CoverageCovered
			state = wire.CoverageCovered
			covered++
		} else {
			uncovered++
			if !profileNames(instrumented, hunk.Path) {
				notInstrumented = append(notInstrumented, hunk.ID)
				state = wire.CoverageNotInstrumented
			}
		}
		hunk.Coverage = &witness
		states = append(states, wire.CoverageStateHunk{ID: hunk.ID, State: state})
	}
	output, err := s.writeMap(document, options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"ok": true, "mutates": true, "tool": "cem-cover", "map": output,
		"profileSha256": profileSha256, "testRun": options.TestRun, "mode": string(mode),
		"covered": covered, "uncovered": uncovered,
		"notInstrumented": len(notInstrumented), "notInstrumentedHunks": notInstrumented,
	}
	if options.StatesOutput != "" {
		statesPath, err := s.writeCoverageStates(output, options, profileSha256, states)
		if err != nil {
			return nil, err
		}
		result["coverageStates"] = statesPath
	}
	return result, nil
}

// checkStatesOutput refuses a --states-output that is absolute or names the
// input map, the output map, or either map's lock or backup name, before any
// write.
func checkStatesOutput(options CoverOptions) error {
	if options.StatesOutput == "" {
		return nil
	}
	if filepath.IsAbs(options.StatesOutput) {
		return invalidArguments("--states-output must be a repository-relative path")
	}
	for _, mapRelative := range []string{options.MapPath, options.Output} {
		if mapRelative == "" {
			continue
		}
		if strings.EqualFold(filepath.ToSlash(filepath.Clean(options.StatesOutput)), filepath.ToSlash(filepath.Clean(mapRelative))) {
			return invalidArguments("--states-output must not name map %s", mapRelative)
		}
		if err := checkMapSidecarAliasing(mapRelative, options.StatesOutput); err != nil {
			return invalidArguments("--states-output must not select the lock or backup name of map %s", mapRelative)
		}
	}
	return nil
}

// writeCoverageStates publishes the cem-coverage-states/1 document bound to
// the exact map bytes just written (CAX-V0-010), and validates it before
// returning so no document the reader would refuse is ever written.
func (s *Session) writeCoverageStates(mapPath string, options CoverOptions, profileSha256 string, states []wire.CoverageStateHunk) (string, error) {
	mapBytes, err := os.ReadFile(mapPath)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(mapBytes)
	encoded, err := wire.EncodeCoverageStates(wire.CoverageStates{
		Hunks: states, MapSha256: hex.EncodeToString(digest[:]), Profile: wire.CoverageStatesProfile,
		ProfileSha256: profileSha256, TestRun: options.TestRun,
	})
	if err != nil {
		return "", err
	}
	if _, err := wire.ValidateCoverageStates(encoded, mapBytes); err != nil {
		return "", err
	}
	relative := filepath.ToSlash(filepath.Clean(options.StatesOutput))
	if err := publish.Publish(publish.Output{Root: s.workRoot, Relative: relative, Data: encoded}); err != nil {
		return "", err
	}
	return filepath.Join(s.workRoot.Path(), filepath.FromSlash(relative)), nil
}

// profilePaths is the set of every profile path a block names, whatever its
// count.
func profilePaths(blocks []coverprofile.Block) map[string]bool {
	paths := map[string]bool{}
	for _, block := range blocks {
		paths[block.ProfilePath] = true
	}
	return paths
}

// profileNames reports whether any profile path matches hunkPath under the
// profileLinesFor rule.
func profileNames(paths map[string]bool, hunkPath string) bool {
	for profilePath := range paths {
		if profilePath == hunkPath || strings.HasSuffix(profilePath, "/"+hunkPath) {
			return true
		}
	}
	return false
}

func (s *Session) readCoverprofile(path string) ([]byte, error) {
	if path == "" {
		return nil, invalidArguments("--coverprofile is required")
	}
	bound := int(coverprofile.MaxBytes)
	if filepath.IsAbs(path) {
		return publish.ReadBoundedFile(path, bound, cemcode.InvalidArguments)
	}
	return s.workRoot.ReadBounded(path, bound, cemcode.InvalidArguments)
}

// lineInterval is one inclusive run of covered lines.
type lineInterval struct{ start, end int64 }

// coveredIntervalsByPath keys every profile path a block names, whatever its
// count, to the sorted, merged line intervals its positive-count blocks span
// (TCQ-V0-097). Coverage stays as intervals: a block may span up to 2^32-1
// lines, so expanding it line by line is unbounded work. Keying zero-count
// paths too lets profileLinesFor see every candidate path when it resolves an
// ambiguous hunk path. A profile path is the Go import path plus file name, so
// it is matched to a hunk path by suffix in profileLinesFor.
func coveredIntervalsByPath(blocks []coverprofile.Block) map[string][]lineInterval {
	byPath := map[string][]lineInterval{}
	for _, block := range blocks {
		intervals := byPath[block.ProfilePath]
		if block.Count != 0 && block.StartLine <= block.EndLine {
			intervals = append(intervals, lineInterval{int64(block.StartLine), int64(block.EndLine)})
		}
		byPath[block.ProfilePath] = intervals
	}
	for path, intervals := range byPath {
		sort.Slice(intervals, func(i, j int) bool { return intervals[i].start < intervals[j].start })
		merged := intervals[:0]
		for _, next := range intervals {
			last := len(merged) - 1
			if last >= 0 && next.start <= merged[last].end+1 {
				merged[last].end = max(merged[last].end, next.end)
				continue
			}
			merged = append(merged, next)
		}
		byPath[path] = merged
	}
	return byPath
}

// profileLinesFor resolves a hunk path to at most one profile path: the same
// path, or a profile path ending in "/"+hunk path. Every profile path counts,
// including one whose blocks all have a zero count (TCQ-V0-097): two distinct
// matches make the witness ambiguous, and Corvint refuses rather than guessing.
func profileLinesFor(lines map[string][]lineInterval, hunkPath string) ([]lineInterval, error) {
	var matches []string
	for profilePath := range lines {
		if profilePath == hunkPath || strings.HasSuffix(profilePath, "/"+hunkPath) {
			matches = append(matches, profilePath)
		}
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return nil, cemcode.New(cemcode.InvalidArguments,
			"coverprofile paths %s all match hunk path %s", strings.Join(matches, ", "), hunkPath)
	}
	if len(matches) == 0 {
		return nil, nil
	}
	return lines[matches[0]], nil
}

// addedLines lists the one-based new-side lines a hunk adds, in order.
func addedLines(parsed *patch.Patch, hunkID string) []int64 {
	for _, hunk := range parsed.Hunks {
		if hunk.ID != hunkID {
			continue
		}
		var added []int64
		line := hunk.NewRange.Start
		for _, body := range hunk.Body {
			if body.Prefix == '+' {
				added = append(added, line)
			}
			if body.Prefix != '-' {
				line++
			}
		}
		return added
	}
	return nil
}

// coveredRanges folds the added lines the sorted, merged covered intervals
// contain into ascending, non-adjacent ranges, intersecting each added line
// with the intervals by binary search; it returns a non-nil empty slice so the
// witness always carries the covered key.
func coveredRanges(added []int64, covered []lineInterval) []wire.Range {
	ranges := []wire.Range{}
	for _, line := range added {
		at := sort.Search(len(covered), func(i int) bool { return covered[i].end >= line })
		if at == len(covered) || covered[at].start > line {
			continue
		}
		last := len(ranges) - 1
		if last >= 0 && ranges[last].Start+ranges[last].Count == line {
			ranges[last].Count++
			continue
		}
		ranges = append(ranges, wire.Range{Start: line, Count: 1})
	}
	return ranges
}
