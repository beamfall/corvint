package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
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

// hunkCandidate is one hunk the run may mutate: a Go source hunk whose basis
// cites at least one _test.go file as a test claim.
type hunkCandidate struct {
	index int
	tests []string
}

// Discriminate records a mutation discrimination witness on every hunk
// (TCQ-V0-055..058). It reuses the prove --mutate runner (mutate.Open and
// Export.Judge with Complete set) on the map's changed hunks only, pins the run
// to the resolved --target tree revision and to the digest of the selected
// test files, and never fails the build: a survived mutant is a visible
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
	document, unlock, err := s.lockedMapInput(options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if !wire.Canonical(document.Spec) {
		return nil, invalidArguments("discrimination witnesses require a canonical (cem/0.2 or cem/0.3) map")
	}
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	target, err := s.pinnedTarget(ctx, document, options.Target)
	if err != nil {
		return nil, err
	}
	candidates := discriminationCandidates(document)
	selected := candidates
	if len(selected) > int(bounds.MaxHunks) {
		selected = selected[:bounds.MaxHunks]
	}
	selection := selectionDigest(selected)
	witnesses := s.judgeHunks(ctx, document, target, selected, bounds)
	for index := range document.Hunks {
		witness, judged := witnesses[index]
		if !judged {
			witness = notRun(unjudgedReason(index, document.Hunks[index], candidates, bounds.MaxHunks))
		}
		witness.TreeRevision, witness.SelectionSha256, witness.Bounds = target, selection, bounds.DiscriminationBounds
		document.Hunks[index].Discriminates = &witness
	}
	document.Spec = wire.Spec03
	output, err := s.writeMap(document, options.MapPath, options.Output)
	if err != nil {
		return nil, err
	}
	counts := discriminationStates(document)
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-discriminate", "map": output,
		"treeRevision": target, "selectionSha256": selection,
		"discriminates": counts[wire.DiscriminationDiscriminates],
		"survived":      counts[wire.DiscriminationSurvived],
		"notRun":        counts[wire.DiscriminationNotRun],
		"bounds": map[string]any{
			"maxHunks": bounds.MaxHunks, "maxMutants": bounds.MaxMutants, "wallTimeSeconds": bounds.WallTimeSeconds,
		},
	}, nil
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
// test claim.
func discriminationCandidates(document *wire.Map) []hunkCandidate {
	paths := map[string]string{}
	for _, record := range document.Evidence {
		paths[record.ID] = record.Path
	}
	var candidates []hunkCandidate
	for index, hunk := range document.Hunks {
		if !isGoSource(hunk.Path) {
			continue
		}
		tests := claimedTests(hunk, paths)
		if len(tests) == 0 {
			continue
		}
		candidates = append(candidates, hunkCandidate{index: index, tests: tests})
	}
	return candidates
}

func isGoSource(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

// claimedTests returns the sorted unique _test.go paths a hunk's test-claim
// basis cites.
func claimedTests(hunk wire.Hunk, paths map[string]string) []string {
	seen := map[string]bool{}
	var tests []string
	for _, item := range hunk.Basis {
		path := paths[item.EvidenceID]
		if item.Relation != "test-claim" || !strings.HasSuffix(path, "_test.go") || seen[path] {
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
func (s *Session) judgeHunks(ctx context.Context, document *wire.Map, target string, selected []hunkCandidate, bounds discriminationBounds) map[int]wire.DiscriminationWitness {
	witnesses := map[int]wire.DiscriminationWitness{}
	if len(selected) == 0 {
		return witnesses
	}
	budgeted, cancel := context.WithTimeout(ctx, bounds.wallTime)
	defer cancel()
	exported, err := openRunner(budgeted, s.root, target)
	if err != nil {
		for _, candidate := range selected {
			witnesses[candidate.index] = notRun("mutation runner unavailable: " + err.Error())
		}
		return witnesses
	}
	defer exported.Close()
	for _, candidate := range selected {
		if budgeted.Err() != nil {
			witnesses[candidate.index] = notRun("wall time exhausted")
			continue
		}
		witnesses[candidate.index] = judgeHunk(budgeted, exported, document.Hunks[candidate.index], candidate.tests, bounds)
	}
	return witnesses
}

func openRunner(ctx context.Context, root, target string) (*mutate.Export, error) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	return mutate.Open(ctx, mutate.Request{Root: root, Git: gitExecutable, Revision: target})
}

// judgeHunk runs every mutant of one hunk against each cited test file and
// folds the reports: a mutant survives only when every cited test lets it
// live, and any report the runner could not complete makes the hunk not-run.
func judgeHunk(ctx context.Context, exported *mutate.Export, hunk wire.Hunk, tests []string, bounds discriminationBounds) wire.DiscriminationWitness {
	if hunk.NewRange.Count == 0 {
		return notRun("hunk adds no lines")
	}
	span := mutate.LineSpan{Start: int(hunk.NewRange.Start), End: int(hunk.NewRange.Start + hunk.NewRange.Count - 1)}
	var reports []mutate.Report
	for _, test := range tests {
		report, err := exported.Judge(ctx, mutate.Request{
			ChangedPath: hunk.Path, TestPath: test, Lines: []mutate.LineSpan{span},
			MaxMutants: int(bounds.MaxMutants), Budget: bounds.wallTime, Complete: true,
		})
		if err != nil {
			return notRun("mutation runner failed for " + test + ": " + err.Error())
		}
		if report.Verdict != mutate.Killed && report.Verdict != mutate.Survived {
			return notRun(strings.ToLower(string(report.Verdict)) + ": " + report.Detail)
		}
		reports = append(reports, report)
	}
	return foldReports(hunk.Path, reports)
}

// foldReports intersects survivors across the cited tests of one hunk. The
// mutant plan is a function of the changed file and lines, so every report
// judged the same mutants and the first report's counts describe the set.
func foldReports(path string, reports []mutate.Report) wire.DiscriminationWitness {
	survivors := reports[0].Survivors
	for _, report := range reports[1:] {
		survivors = survivedBoth(survivors, report.Survivors)
	}
	witness := wire.DiscriminationWitness{
		Mutants: int64(reports[0].Mutants), Survived: int64(len(survivors)),
		Survivors: []wire.SurvivingMutant{}, State: wire.DiscriminationDiscriminates,
	}
	witness.Killed = witness.Mutants - int64(reports[0].Uncompilable) - witness.Survived
	for _, mutant := range survivors {
		witness.Survivors = append(witness.Survivors, wire.SurvivingMutant{
			Operator: mutant.Operator, Line: int64(mutant.Line),
			Description: fmt.Sprintf("%s at %s:%d (bytes %d..%d) passed every cited test", mutant.Operator, path, mutant.Line, mutant.Start, mutant.End),
		})
	}
	if witness.Survived > 0 {
		witness.State = wire.DiscriminationSurvived
	}
	if witness.Killed == 0 && witness.Survived == 0 {
		return notRun("no mutant compiled")
	}
	return witness
}

func survivedBoth(first, second []mutate.Survivor) []mutate.Survivor {
	keep := map[mutate.Survivor]bool{}
	for _, mutant := range second {
		keep[mutant] = true
	}
	var both []mutate.Survivor
	for _, mutant := range first {
		if keep[mutant] {
			both = append(both, mutant)
		}
	}
	return both
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
// hunk limit, it is not Go source, or nothing cites a test file for it.
func unjudgedReason(index int, hunk wire.Hunk, candidates []hunkCandidate, maxHunks int64) string {
	for _, candidate := range candidates {
		if candidate.index == index {
			return fmt.Sprintf("hunk limit %d reached", maxHunks)
		}
	}
	if !isGoSource(hunk.Path) {
		return "not a Go source file"
	}
	return "no test claim cites a _test.go file"
}

func discriminationStates(document *wire.Map) map[string]int {
	counts := map[string]int{}
	for _, hunk := range document.Hunks {
		counts[hunk.Discriminates.State]++
	}
	return counts
}
