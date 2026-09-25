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
	if err := checkSpec03Output(document, options.MapPath, options.Output); err != nil {
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
		witness := exported.Judge(budgeted, document.Hunks[candidate.index], candidate.tests, int(bounds.MaxMutants), bounds.wallTime)
		witness.Detail = boundedDetail(witness.Detail)
		witnesses[candidate.index] = witness
	}
	return witnesses
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
