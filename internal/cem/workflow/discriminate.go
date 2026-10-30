package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// DiscriminateOptions run bounded hunk mutation against the tests a map's
// hunks cite. MaxHunks, MaxMutants, and WallTime are the operator's text;
// empty means the default.
type DiscriminateOptions struct {
	MapPath    string
	Target     string
	MaxHunks   string
	MaxMutants string
	WallTime   string
	Output     string
	// SameChangeTests is the operator's SELECTOR=PATH[,SELECTOR=PATH...]
	// list of test files the change itself adds or edits, each judged against
	// the selected hunk by execution only (TCQ-V0-071). SELECTOR is the
	// one-based ordinal or full hunk ID cite and mark accept (TCQ-V0-093).
	SameChangeTests string
}

const (
	defaultMaxHunks   = 8
	defaultMaxMutants = 8
	defaultWallTime   = 10 * time.Minute
)

// discriminationBounds are the run's parsed bounds and the wall-time budget
// they name.
type discriminationBounds struct {
	wire.DiscriminationBounds
	wallTime time.Duration
}

// hunkCandidate is one hunk the run may mutate: a Go or Ruby source hunk whose
// basis cites at least one test file of its language as a test claim, or that
// the operator named a same-change test for (TCQ-V0-071).
type hunkCandidate struct {
	index int
	tests []string
	// sameChange lists the tests among tests that only --same-change-test
	// named; the witness detail names them.
	sameChange []string
}

// maxSameChangeTests bounds the --same-change-test list.
const maxSameChangeTests = 64

// Discriminate records a mutation discrimination witness on every hunk
// (TCQ-V0-055..058). It reuses the prove --mutate runner through the
// installed OpenHunkJudge (internal/cemdiscriminate) on the map's changed
// hunks only, pins the run to the resolved --target tree revision and to the
// digest of the selected test files, and never fails the build: a survived mutant is a visible
// downgrade in the report, and a hunk the bounds or the host leave unjudged
// carries an explicit not-run witness with its reason.
func (s *Session) Discriminate(ctx context.Context, options DiscriminateOptions) (map[string]any, error) {
	bounds, err := parseDiscriminationBounds(options)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.Target) == "" {
		return nil, invalidArguments("--target is required")
	}
	document, unlock, err := s.lockedMapInput("discriminate", options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	target, err := s.pinnedTarget(ctx, document, options.Target)
	if err != nil {
		return nil, err
	}
	sameChange, err := parseSameChangeTests(options.SameChangeTests, document)
	if err != nil {
		return nil, err
	}
	candidates := discriminationCandidates(document, sameChange)
	selected := candidates
	if len(selected) > int(bounds.MaxHunks) {
		selected = selected[:bounds.MaxHunks]
	}
	selection := selectionDigest(selected)
	witnesses, unavailable, unavailableHunks := s.judgeHunks(ctx, document, target, selected, bounds)
	for index := range document.Hunks {
		witness, judged := witnesses[index]
		if !judged {
			witness = notRun(unjudgedReason(index, document.Hunks[index], candidates, bounds.MaxHunks))
		}
		witness.TreeRevision, witness.SelectionSha256, witness.Bounds = target, selection, bounds.DiscriminationBounds
		document.Hunks[index].Discriminates = &witness
	}
	output, err := s.writeMap(document, options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	counts := discriminationStates(document)
	result := map[string]any{
		"ok": true, "mutates": true, "tool": "cem-discriminate", "map": output,
		"treeRevision": target, "selectionSha256": selection,
		"discriminates": counts[wire.DiscriminationDiscriminates],
		"survived":      counts[wire.DiscriminationSurvived],
		"notRun":        counts[wire.DiscriminationNotRun],
		"bounds": map[string]any{
			"maxHunks": bounds.MaxHunks, "maxMutants": bounds.MaxMutants, "wallTimeSeconds": bounds.WallTimeSeconds,
		},
	}
	// TCQ-V0-092: a runner that could not open names why apart from the
	// per-hunk not-run details, so the run is not read as an ordinary not-run.
	// TCQ-V0-094: a runner arm that could not start on some hunks names its
	// cause too, and counts the hunks it left not-run.
	if unavailable != "" {
		result["runnerUnavailable"] = unavailable
		result["runnerUnavailableHunks"] = unavailableHunks
	}
	return result, nil
}

// SandboxUnpackSymlink is the runnerUnavailable reason when the exported copy
// refused a tracked symlink: one that would resolve outside it, whose link
// text is out of bounds, or that the host could not create; the runner's
// refusal carries the same text (internal/liveverify/mutate.SymlinkRefusal).
const SandboxUnpackSymlink = "sandbox-unpack-symlink"

// runnerUnavailableReason classifies why the mutation runner did not open.
func runnerUnavailableReason(err error) string {
	if strings.Contains(err.Error(), SandboxUnpackSymlink) {
		return SandboxUnpackSymlink
	}
	return "runner-unavailable"
}

func parseDiscriminationBounds(options DiscriminateOptions) (discriminationBounds, error) {
	maxHunks, err := parseBound(options.MaxHunks, "--max-hunks", defaultMaxHunks)
	if err != nil {
		return discriminationBounds{}, err
	}
	maxMutants, err := parseBound(options.MaxMutants, "--max-mutants", defaultMaxMutants)
	if err != nil {
		return discriminationBounds{}, err
	}
	wallTime, err := parseWallTime(options.WallTime)
	if err != nil {
		return discriminationBounds{}, err
	}
	return discriminationBounds{
		DiscriminationBounds: wire.DiscriminationBounds{
			MaxHunks: maxHunks, MaxMutants: maxMutants, WallTimeSeconds: int64(wallTime / time.Second),
		},
		wallTime: wallTime,
	}, nil
}

func parseBound(text, name string, fallback int64) (int64, error) {
	if text == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 1 {
		return 0, invalidArguments("%s must be a positive integer", name)
	}
	return value, nil
}

// parseWallTime accepts a Go duration of whole seconds, at least one, so the
// witness records exactly the bound the run enforced.
func parseWallTime(text string) (time.Duration, error) {
	if text == "" {
		return defaultWallTime, nil
	}
	value, err := time.ParseDuration(text)
	if err != nil || value < time.Second || value%time.Second != 0 {
		return 0, invalidArguments("--wall-time must be a duration of whole seconds, at least 1s")
	}
	return value, nil
}

// pinnedTarget resolves --target and requires the canonical diff from the
// map's base to it to carry the map's patch digest, so the witness names the
// tree the mutated hunks were taken from.
func (s *Session) pinnedTarget(ctx context.Context, document *wire.Map, targetArg string) (string, error) {
	target, err := s.repository.Resolve(ctx, targetArg)
	if err != nil {
		return "", err
	}
	patchBytes, err := s.repository.CanonicalDiff(ctx, document.BaseRevision, target)
	if err != nil {
		return "", err
	}
	if !patchMatches(document, patchBytes) {
		return "", cemcode.New(cemcode.PatchDigestMismatch,
			"the canonical patch from the map base to --target does not match the CEM patch digest")
	}
	return target, nil
}

// discriminationCandidates lists, in map order, the hunks that can be
// mutated: Go source (not a test file) whose basis cites a _test.go file as a
// test claim, or Ruby source (not a spec file) whose basis cites a _spec.rb
// file (TCQ-V0-066), or either that sameChange names a test for (TCQ-V0-071).
func discriminationCandidates(document *wire.Map, sameChange map[int][]string) []hunkCandidate {
	paths := map[string]string{}
	for _, record := range document.Evidence {
		paths[record.ID] = record.Path
	}
	var candidates []hunkCandidate
	for index, hunk := range document.Hunks {
		if runnerTestSuffix(hunk.Path) == "" {
			continue
		}
		tests := claimedTests(hunk, paths)
		claimed := map[string]bool{}
		for _, test := range tests {
			claimed[test] = true
		}
		var added []string
		for _, test := range sameChange[index] {
			if !claimed[test] {
				claimed[test] = true
				added = append(added, test)
				tests = append(tests, test)
			}
		}
		if len(tests) == 0 {
			continue
		}
		sort.Strings(tests)
		sort.Strings(added)
		candidates = append(candidates, hunkCandidate{index: index, tests: tests, sameChange: added})
	}
	return candidates
}

// parseSameChangeTests reads --same-change-test: comma-separated SELECTOR=PATH
// pairs naming a test file the change itself adds or edits for one runner
// hunk (TCQ-V0-071). SELECTOR resolves as cite --hunk does: a canonical
// one-based ordinal or a full hunk ID (TCQ-V0-093). The hunk must be in the map and judged by a runner; the
// path must be a test file of that runner's language that the map's own
// hunks add lines to, so it is changed by the bound patch and present at the
// target. The test is never evidence: it can only take part in the run.
func parseSameChangeTests(text string, document *wire.Map) (map[int][]string, error) {
	if text == "" {
		return nil, nil
	}
	present := map[string]bool{}
	for _, hunk := range document.Hunks {
		if hunk.NewRange.Count > 0 {
			present[hunk.Path] = true
		}
	}
	pairs := strings.Split(text, ",")
	if len(pairs) > maxSameChangeTests {
		return nil, invalidArguments("--same-change-test names more than %d tests", maxSameChangeTests)
	}
	out := map[int][]string{}
	for _, pair := range pairs {
		selector, path, ok := strings.Cut(pair, "=")
		index, err := 0, error(nil)
		if ok && selector != "" {
			index, err = resolveHunk(document, selector)
		}
		if !ok || selector == "" || err != nil {
			return nil, invalidArguments("--same-change-test %q must be SELECTOR=PATH, SELECTOR a one-based hunk ordinal or a hunk ID of the map", pair)
		}
		id := document.Hunks[index].ID
		suffix := runnerTestSuffix(document.Hunks[index].Path)
		if suffix == "" {
			return nil, invalidArguments("--same-change-test hunk %s is not Go or Ruby source", id)
		}
		if wire.ValidatePath(path) != nil || !strings.HasSuffix(path, suffix) {
			return nil, invalidArguments("--same-change-test path %q must be a repository-relative %s file", path, suffix)
		}
		if !present[path] {
			return nil, invalidArguments("--same-change-test path %q must be a test file the change adds lines to", path)
		}
		out[index] = append(out[index], path)
	}
	return out, nil
}

// runnerTestSuffix names the test files the mutation runner judges a source
// path against: _test.go for Go source, _spec.rb for Ruby source through the
// RSpec arm (TCQ-V0-066), and "" for a test file or any other language. It is
// the candidate side of the runner contract and must agree with the
// compiled-in declarations in internal/cemdiscriminate (TCQ-V0-067).
func runnerTestSuffix(path string) string {
	switch {
	case strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_spec.rb"):
		return ""
	case strings.HasSuffix(path, ".go"):
		return "_test.go"
	case strings.HasSuffix(path, ".rb"):
		return "_spec.rb"
	}
	return ""
}

// claimedTests returns the sorted unique test paths of the hunk's language
// that its test-claim basis cites.
func claimedTests(hunk wire.Hunk, paths map[string]string) []string {
	suffix := runnerTestSuffix(hunk.Path)
	seen := map[string]bool{}
	var tests []string
	for _, item := range hunk.Basis {
		path := paths[item.EvidenceID]
		if item.Relation != "test-claim" || suffix == "" || !strings.HasSuffix(path, suffix) || seen[path] {
			continue
		}
		seen[path] = true
		tests = append(tests, path)
	}
	sort.Strings(tests)
	return tests
}

// selectionDigest is the SHA-256 of the sorted unique selected test paths,
// each terminated by a newline: the run's test selection, derived by Corvint.
func selectionDigest(selected []hunkCandidate) string {
	seen := map[string]bool{}
	var paths []string
	for _, candidate := range selected {
		for _, path := range candidate.tests {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	sort.Strings(paths)
	digest := sha256.Sum256([]byte(strings.Join(paths, "\n") + "\n"))
	return hex.EncodeToString(digest[:])
}

// judgeHunks runs the selected hunks in order under one wall-time budget and
// one exported copy of the target tree. Every selected hunk gets a witness:
// a runner that cannot open, a spent budget, or a runner failure yields
// not-run with the reason rather than an error.
func (s *Session) judgeHunks(ctx context.Context, document *wire.Map, target string, selected []hunkCandidate, bounds discriminationBounds) (map[int]wire.DiscriminationWitness, string, int) {
	witnesses := map[int]wire.DiscriminationWitness{}
	if len(selected) == 0 {
		return witnesses, "", 0
	}
	budgeted, cancel := context.WithTimeout(ctx, bounds.wallTime)
	defer cancel()
	exported, err := openRunner(budgeted, s.root, target)
	if err != nil {
		for _, candidate := range selected {
			witnesses[candidate.index] = notRun("mutation runner unavailable: " + err.Error())
		}
		return witnesses, runnerUnavailableReason(err), len(selected)
	}
	defer exported.Close()
	reporter, _ := exported.(interface{ RunnerUnavailable() string })
	unavailable, unavailableHunks := "", 0
	for _, candidate := range selected {
		if budgeted.Err() != nil {
			witnesses[candidate.index] = notRun("wall time exhausted")
			continue
		}
		witness := exported.Judge(budgeted, document.Hunks[candidate.index], candidate.tests, int(bounds.MaxMutants), bounds.wallTime)
		witness.Detail = boundedDetail(sameChangeDetail(candidate.sameChange, witness.Detail))
		witnesses[candidate.index] = witness
		if reporter == nil {
			continue
		}
		if cause := reporter.RunnerUnavailable(); cause != "" {
			unavailableHunks++
			if unavailable == "" {
				unavailable = cause
			}
		}
	}
	return witnesses, unavailable, unavailableHunks
}

// sameChangeDetail names the same-change tests a run took part in, so a
// reader sees that the test and the code arrived together (TCQ-V0-071).
func sameChangeDetail(sameChange []string, detail string) string {
	if len(sameChange) == 0 {
		return detail
	}
	named := "same-change test (execution only): " + strings.Join(sameChange, ", ")
	if detail == "" {
		return named
	}
	return named + "; " + detail
}

// HunkJudge judges one hunk's mutants against its cited test files on one
// exported copy of the target tree. Its not-run details are bounded here.
type HunkJudge interface {
	Judge(ctx context.Context, hunk wire.Hunk, tests []string, maxMutants int, wallTime time.Duration) wire.DiscriminationWitness
	Close()
}

// OpenHunkJudge serves `cem discriminate` (internal/cemdiscriminate). The
// binary installs it, so the CEM seams' dependency closure stays the standard
// library and internal/cem; an uninstalled runner is an unavailable runner,
// so every selected hunk carries a not-run witness.
var OpenHunkJudge func(ctx context.Context, root, target string) (HunkJudge, error)

func openRunner(ctx context.Context, root, target string) (HunkJudge, error) {
	if OpenHunkJudge == nil {
		return nil, errors.New("no mutation runner is installed")
	}
	return OpenHunkJudge(ctx, root, target)
}

func notRun(detail string) wire.DiscriminationWitness {
	return wire.DiscriminationWitness{
		Survivors: []wire.SurvivingMutant{}, State: wire.DiscriminationNotRun, Detail: boundedDetail(detail),
	}
}

// boundedDetail keeps a runner message inside the wire bound and free of
// control characters so the witness it lands in stays valid.
func boundedDetail(detail string) string {
	cleaned := strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return ' '
		}
		return character
	}, detail)
	if len(cleaned) > wire.MaxDiscriminationTextBytes {
		cleaned = strings.ToValidUTF8(cleaned[:wire.MaxDiscriminationTextBytes-3], "") + "..."
	}
	return cleaned
}

// unjudgedReason names why a hunk carries no run: it was a candidate past the
// hunk limit, it is not Go or Ruby source, or nothing cites a test file of its
// language for it.
func unjudgedReason(index int, hunk wire.Hunk, candidates []hunkCandidate, maxHunks int64) string {
	for _, candidate := range candidates {
		if candidate.index == index {
			return fmt.Sprintf("hunk limit %d reached", maxHunks)
		}
	}
	suffix := runnerTestSuffix(hunk.Path)
	if suffix == "" {
		return "not a Go or Ruby source file"
	}
	return "no test claim cites a " + suffix + " file"
}

func discriminationStates(document *wire.Map) map[string]int {
	counts := map[string]int{}
	for _, hunk := range document.Hunks {
		counts[hunk.Discriminates.State]++
	}
	return counts
}
