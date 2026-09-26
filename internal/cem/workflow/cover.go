package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
}

// Cover records a coverage witness on every hunk (TCQ-V0-051..054). The
// coverprofile is one explicit operator-named local file: Corvint neither
// discovers profiles nor runs tests here. Covered ranges are the hunk's added
// lines that a block with a positive count reaches (diff-cover semantics); a
// hunk none reach carries an explicit uncovered witness rather than none.
// Witnesses are a cem/0.3 field, so a canonical map is upgraded like a
// structural reason in Mark (CEM-SM-006).
func (s *Session) Cover(ctx context.Context, options CoverOptions) (map[string]any, error) {
	if err := wire.ValidateTestRun(options.TestRun); err != nil {
		return nil, invalidArguments("--test-run %s", strings.TrimPrefix(err.Error(), "coverage "))
	}
	document, unlock, err := s.lockedMapInput(options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if !wire.Canonical(document.Spec) {
		return nil, invalidArguments("coverage witnesses require a canonical (cem/0.2 or cem/0.3) map")
	}
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
	coveredLines := coveredLinesByPath(blocks)
	covered, uncovered := 0, 0
	for index := range document.Hunks {
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
		if len(ranges) > 0 {
			witness.State = wire.CoverageCovered
			covered++
		} else {
			uncovered++
		}
		hunk.Coverage = &witness
	}
	if err := checkSpec03Output(document, options.MapPath, options.Output); err != nil {
		return nil, err
	}
	document.Spec = wire.Spec03
	output, err := s.writeMap(document, options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-cover", "map": output,
		"profileSha256": profileSha256, "testRun": options.TestRun, "mode": string(mode),
		"covered": covered, "uncovered": uncovered,
	}, nil
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

// coveredLinesByPath collects, per profile path, every line a block with a
// positive count spans. A profile path is the Go import path plus file name,
// so it is matched to a hunk path by suffix in profileLinesFor.
func coveredLinesByPath(blocks []coverprofile.Block) map[string]map[int64]bool {
	lines := map[string]map[int64]bool{}
	for _, block := range blocks {
		if block.Count == 0 {
			continue
		}
		set := lines[block.ProfilePath]
		if set == nil {
			set = map[int64]bool{}
			lines[block.ProfilePath] = set
		}
		for line := int64(block.StartLine); line <= int64(block.EndLine); line++ {
			set[line] = true
		}
	}
	return lines
}

// profileLinesFor resolves a hunk path to at most one profile path: the same
// path, or a profile path ending in "/"+hunk path. Two distinct matches make the
// witness ambiguous, and Corvint refuses rather than guessing.
func profileLinesFor(lines map[string]map[int64]bool, hunkPath string) (map[int64]bool, error) {
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

// coveredRanges folds the covered added lines into ascending, non-adjacent
// ranges; it returns a non-nil empty slice so the witness always carries the
// covered key.
func coveredRanges(added []int64, covered map[int64]bool) []wire.Range {
	ranges := []wire.Range{}
	for _, line := range added {
		if !covered[line] {
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
