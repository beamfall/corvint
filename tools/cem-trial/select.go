package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// selectionRule is frozen verbatim in every manifest: the harness recomputes
// it and refuses a manifest whose rule string or patch digests have moved
// (CRT-V0-001).
const selectionRule = "non-merge commits since --since, oldest first, touching between 3 and 12 text files, " +
	"with at least one modified non-test source file (.go|.ts|.tsx|.swift|.py) and at least one test-or-spec file " +
	"(_test.go, *.test.ts, *.test.tsx, *Tests.swift, _test.py, docs/specs/**); the presented patch is the source-file " +
	"hunks alone, at most 64 KiB, over which `cem begin` parses between 3 and 20 hunks; a change is rejected when any " +
	"source hunk contains a gold path literal; select index floor(i*N/n) for i in 0..n-1 over the candidates in commit order"

const (
	selectMinFiles = 3
	selectMaxFiles = 12
	selectMinHunks = 3
	selectMaxHunks = 20
)

type manifest struct {
	Partition     string `json:"partition"`
	Seed          string `json:"seed"`
	SelectionRule string `json:"selection_rule"`
	Population    int    `json:"population"`
	Repository    string `json:"repository"`
	Since         string `json:"since"`
	Pairs         int    `json:"pairs"`
	Tasks         []task `json:"tasks"`
}

type task struct {
	ID           string   `json:"id"`
	Commit       string   `json:"commit"`
	BaseCommit   string   `json:"base_commit"`
	PatchSHA256  string   `json:"patch_sha256"`
	PatchPath    string   `json:"patch_path"`
	SourceFiles  []string `json:"source_files"`
	Gold         gold     `json:"gold"`
	HunkCount    int      `json:"hunk_count"`
	StemBaseline []string `json:"stem_baseline"`
}

type gold struct {
	Paths []string `json:"paths"`
	Spans []span   `json:"spans"`
}

type span struct {
	Path  string `json:"path"`
	Lines string `json:"lines"`
}

type selectOptions struct {
	repo, name, since, output, seed, partition, corvintGo string
	exclude                                               []string
	limit                                                 int
}

// excludeError reports an --exclude file that cannot name the changes a prior
// partition already observed, so a held-out selection never silently overlaps
// the pilot (CRT-V0-009).
type excludeError struct {
	path string
	err  error
}

func (e *excludeError) Error() string { return fmt.Sprintf("--exclude %s: %v", e.path, e.err) }

func (e *excludeError) Unwrap() error { return e.err }

// poolError reports that the rule and --exclude together left no change to
// select, so an empty manifest is never written and called a selection
// (CRT-V0-009).
type poolError struct{ matched, excluded int }

func (e *poolError) Error() string {
	return fmt.Sprintf("no candidate change remains: %d matched the rule, %d excluded", e.matched, e.excluded)
}

// candidate is one commit that passed the cheap half of the rule.
type candidate struct {
	commit, parent string
	sourceFiles    []string
	goldPaths      []string
	patch          string
	hunks          int
}

func parseSelectOptions(arguments []string) (selectOptions, error) {
	config := selectOptions{}
	flags := flag.NewFlagSet("cem-trial select", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&config.repo, "repo", "", "local Git repository to read (never written)")
	flags.StringVar(&config.name, "name", "", "repository name recorded in the manifest")
	flags.StringVar(&config.since, "since", "2026-05-01", "only commits after this date (git --since syntax)")
	flags.StringVar(&config.output, "output", "", "directory receiving tasks.json and patches/")
	flags.StringVar(&config.seed, "seed", "", "seed recorded in the manifest and used for the arm order")
	flags.StringVar(&config.partition, "partition", "pilot", "partition: pilot or heldout")
	flags.StringVar(&config.corvintGo, "corvint", "", "corvint executable used to check `cem begin` parses each patch")
	exclude := ""
	flags.StringVar(&exclude, "exclude", "", "comma-separated prior tasks.json files whose changes are dropped before selection")
	flags.IntVar(&config.limit, "limit", 30, "number of changes to select")
	if err := flags.Parse(arguments); err != nil {
		return config, err
	}
	if config.repo == "" || config.output == "" || config.seed == "" || config.corvintGo == "" || flags.NArg() != 0 {
		return config, errors.New("usage: cem-trial select --repo DIR --output DIR --seed S --corvint PATH [--name N] [--since DATE] [--partition P] [--exclude FILES] [--limit N]")
	}
	config.exclude = splitList(exclude)
	if flagProvided(flags, "exclude") && len(config.exclude) == 0 {
		return config, errors.New("--exclude names no manifest")
	}
	if config.limit < 1 {
		return config, errors.New("--limit must be at least 1")
	}
	if config.name == "" {
		config.name = filepath.Base(config.repo)
	}
	absolute, err := filepath.Abs(config.corvintGo)
	if err != nil {
		return config, err
	}
	config.corvintGo = absolute
	return config, nil
}

// flagProvided reports whether the caller wrote the flag, so an explicitly
// empty --exclude is told apart from no --exclude at all (CRT-V0-009).
func flagProvided(flags *flag.FlagSet, name string) bool {
	provided := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == name {
			provided = true
		}
	})
	return provided
}

func selectChanges(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	config, err := parseSelectOptions(arguments)
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	excluded, err := excludedCommits(config.exclude)
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	population, err := candidateCommits(ctx, config)
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	candidates, dropped := withoutExcluded(population, excluded)
	if err := inertExclusion(config.exclude, len(excluded), dropped); err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	if len(candidates) == 0 {
		fmt.Fprintln(stderr, "cem-trial:", &poolError{matched: len(population), excluded: dropped})
		return 2
	}
	if len(candidates) < config.limit {
		fmt.Fprintf(stderr, "cem-trial: warning: %d candidates remain after exclusion, short of --limit %d\n",
			len(candidates), config.limit)
	}
	chosen, err := validated(ctx, config, candidates)
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	set := &manifest{
		Partition: config.partition, Seed: config.seed, SelectionRule: selectionRule,
		Population: len(candidates), Repository: config.name, Since: config.since,
		Pairs: len(chosen), Tasks: []task{},
	}
	if err := os.MkdirAll(filepath.Join(config.output, "patches"), 0o755); err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	for index, item := range chosen {
		id := fmt.Sprintf("c%02d-%s", index+1, item.commit[:12])
		relative := path.Join("patches", id+".patch")
		if err := os.WriteFile(filepath.Join(config.output, filepath.FromSlash(relative)), []byte(item.patch), 0o644); err != nil {
			fmt.Fprintln(stderr, "cem-trial:", err)
			return 2
		}
		spans, err := goldSpans(ctx, config.repo, item.commit, item.goldPaths)
		if err != nil {
			fmt.Fprintln(stderr, "cem-trial:", err)
			return 2
		}
		set.Tasks = append(set.Tasks, task{
			ID: id, Commit: item.commit, BaseCommit: item.parent,
			PatchSHA256: digestOf(item.patch), PatchPath: relative,
			SourceFiles: item.sourceFiles,
			Gold:        gold{Paths: item.goldPaths, Spans: spans},
			HunkCount:   item.hunks, StemBaseline: stemBaseline(item.sourceFiles),
		})
	}
	encoded, err := gokernel.CanonicalJSON(set)
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	if err := os.WriteFile(filepath.Join(config.output, "tasks.json"), append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	printSelection(stdout, config, set, dropped)
	return 0
}

func printSelection(stdout io.Writer, config selectOptions, set *manifest, excluded int) {
	fmt.Fprintf(stdout, "population %d candidates, excluded %d, selected %d, partition %s, seed %s\n",
		set.Population, excluded, len(set.Tasks), set.Partition, set.Seed)
	for _, item := range set.Tasks {
		fmt.Fprintf(stdout, "%s %s base=%s hunks=%d source=%d gold=%d spans=%d stem=%d\n",
			item.ID, item.Commit[:12], item.BaseCommit[:12], item.HunkCount,
			len(item.SourceFiles), len(item.Gold.Paths), len(item.Gold.Spans), len(item.StemBaseline))
		fmt.Fprintf(stdout, "    gold: %s\n", strings.Join(item.Gold.Paths, ", "))
		fmt.Fprintf(stdout, "    stem: %s\n", strings.Join(item.StemBaseline, ", "))
	}
	fmt.Fprintf(stdout, "manifest %s\n", filepath.Join(config.output, "tasks.json"))
}

// candidateCommits applies every part of the rule that needs no corvint: the
// file bounds, the source and gold requirement, the patch bound, the hunk
// bounds, and the non-circularity check.
func candidateCommits(ctx context.Context, config selectOptions) ([]candidate, error) {
	raw, err := gitOutput(ctx, config.repo, "log", "--no-merges", "--since="+config.since, "--format=%H %P")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	candidates := []candidate{}
	for index := len(lines) - 1; index >= 0; index-- { // oldest first
		fields := strings.Fields(lines[index])
		if len(fields) < 2 {
			continue
		}
		item, ok, err := inspectCommit(ctx, config.repo, fields[0], fields[1])
		if err != nil {
			return nil, err
		}
		if ok {
			candidates = append(candidates, item)
		}
	}
	return candidates, nil
}

// excludedCommits reads the change identifiers of every prior manifest named
// by --exclude. A file that cannot be read, cannot be parsed, or names no
// change is refused rather than silently excluding nothing (CRT-V0-009).
func excludedCommits(paths []string) (map[string]bool, error) {
	excluded := map[string]bool{}
	for _, item := range paths {
		raw, err := readBounded(item, maxTasksFile)
		if err != nil {
			return nil, &excludeError{path: item, err: err}
		}
		var prior manifest
		if err := json.Unmarshal(raw, &prior); err != nil {
			return nil, &excludeError{path: item, err: err}
		}
		named := 0
		for _, change := range prior.Tasks {
			if !objectName(change.Commit) {
				return nil, &excludeError{path: item, err: fmt.Errorf("%q is not a 40-character lowercase hex commit", change.Commit)}
			}
			excluded[change.Commit] = true
			named++
		}
		if named == 0 {
			return nil, &excludeError{path: item, err: errors.New("names no change")}
		}
	}
	return excluded, nil
}

// objectName is the one identifier shape an exclude manifest may carry: the
// full lowercase object name `select` itself records. An abbreviated or
// uppercase form would compare unequal to every candidate and drop nothing.
func objectName(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		hex := '0' <= character && character <= '9' || 'a' <= character && character <= 'f'
		if !hex {
			return false
		}
	}
	return true
}

// inertExclusion refuses an --exclude that bit nothing: a manifest from a
// different or rebased repository would otherwise leave the held-out pool
// identical to the pilot's while the run still exits 0 (CRT-V0-009).
func inertExclusion(paths []string, named, dropped int) error {
	if len(paths) == 0 || dropped > 0 {
		return nil
	}
	return &excludeError{
		path: strings.Join(paths, ","),
		err:  fmt.Errorf("none of its %d changes is among the candidates", named),
	}
}

// withoutExcluded drops every candidate a prior partition already observed,
// leaving the survivors in the same commit order the rule selects over, and
// reports how many were dropped (CRT-V0-009).
func withoutExcluded(candidates []candidate, excluded map[string]bool) ([]candidate, int) {
	kept := make([]candidate, 0, len(candidates))
	for _, item := range candidates {
		if !excluded[item.commit] {
			kept = append(kept, item)
		}
	}
	return kept, len(candidates) - len(kept)
}

func inspectCommit(ctx context.Context, repo, commit, parent string) (candidate, bool, error) {
	raw, err := gitOutput(ctx, repo, "show", "--name-status", "--format=", commit)
	if err != nil {
		return candidate{}, false, err
	}
	textFiles, sourceFiles, goldPaths := 0, []string{}, []string{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		status, file := fields[0], fields[len(fields)-1]
		if !textPath(file) {
			continue
		}
		textFiles++
		if strings.HasPrefix(status, "M") && sourcePath(file) && !goldPath(file) {
			sourceFiles = append(sourceFiles, file)
		}
		if goldPath(file) {
			goldPaths = append(goldPaths, file)
		}
	}
	if textFiles < selectMinFiles || textFiles > selectMaxFiles || len(sourceFiles) == 0 || len(goldPaths) == 0 {
		return candidate{}, false, nil
	}
	sort.Strings(sourceFiles)
	sort.Strings(goldPaths)
	patch, err := gitOutput(ctx, repo, append([]string{"diff", parent, commit, "--"}, sourceFiles...)...)
	if err != nil {
		return candidate{}, false, err
	}
	if len(patch) == 0 || len(patch) > maxPatchBytes {
		return candidate{}, false, nil
	}
	hunks := strings.Count(patch, "\n@@ ")
	if strings.HasPrefix(patch, "@@ ") {
		hunks++
	}
	if hunks < selectMinHunks || hunks > selectMaxHunks {
		return candidate{}, false, nil
	}
	if leaksGold(patch, goldPaths) {
		return candidate{}, false, nil
	}
	return candidate{commit: commit, parent: parent, sourceFiles: sourceFiles, goldPaths: goldPaths, patch: patch, hunks: hunks}, true, nil
}

// leaksGold rejects a change whose presented hunks name a gold path, which
// would make the answer readable from the question (CRT-V0-004).
func leaksGold(patch string, goldPaths []string) bool {
	body := hunkBody(patch)
	for _, item := range goldPaths {
		if strings.Contains(body, item) || strings.Contains(body, path.Base(item)) {
			return true
		}
	}
	return false
}

// hunkBody is the patch without its file headers, so the `+++ b/PATH` lines a
// source-only patch carries cannot themselves count as a leak.
func hunkBody(patch string) string {
	var out strings.Builder
	inside := false
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "@@ ") {
			inside = true
		}
		if strings.HasPrefix(line, "diff --git ") {
			inside = false
		}
		if inside {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// validated takes the evenly spaced selection and keeps only the changes
// `cem begin` actually parses into the declared hunk bounds, refilling from
// the remaining candidates until the selection is stable (CRT-V0-001).
func validated(ctx context.Context, config selectOptions, candidates []candidate) ([]candidate, error) {
	pool := append([]candidate(nil), candidates...)
	for attempt := 0; attempt < len(candidates)+1; attempt++ {
		selection := evenlySpaced(pool, config.limit)
		rejected := map[string]bool{}
		accepted := make([]candidate, 0, len(selection))
		for _, item := range selection {
			hunks, err := beginHunks(ctx, config, item)
			if err != nil || hunks < selectMinHunks || hunks > selectMaxHunks {
				rejected[item.commit] = true
				continue
			}
			item.hunks = hunks
			accepted = append(accepted, item)
		}
		if len(rejected) == 0 {
			return accepted, nil
		}
		kept := pool[:0]
		for _, item := range pool {
			if !rejected[item.commit] {
				kept = append(kept, item)
			}
		}
		pool = append([]candidate(nil), kept...)
		if len(pool) == 0 {
			return accepted, nil
		}
	}
	return nil, errors.New("selection did not stabilise")
}

// beginHunks reports how many hunks `cem begin` parses over the withheld
// patch, in a clone that holds the base commit and nothing after it.
func beginHunks(ctx context.Context, config selectOptions, item candidate) (int, error) {
	root, err := cloneAtBase(ctx, config.repo, item.parent)
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(root)
	scratch, err := os.MkdirTemp("", "corvint-cem-trial-select-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(scratch)
	patchPath := filepath.Join(scratch, "change.patch")
	if err := os.WriteFile(patchPath, []byte(item.patch), 0o644); err != nil {
		return 0, err
	}
	raw, err := corvint(ctx, options{corvintGo: config.corvintGo}, root, "cem", "begin", "--patch", patchPath, "--output", mapRelative)
	if err != nil {
		return 0, err
	}
	var envelope struct {
		Hunks []any `json:"hunks"`
		OK    bool  `json:"ok"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || !envelope.OK {
		return 0, fmt.Errorf("cem begin refused %s", item.commit)
	}
	return len(envelope.Hunks), nil
}

// evenlySpaced keeps the released sets' rule: index floor(i*N/n).
func evenlySpaced(candidates []candidate, limit int) []candidate {
	if len(candidates) <= limit {
		return append([]candidate(nil), candidates...)
	}
	selected := make([]candidate, 0, limit)
	for index := 0; index < limit; index++ {
		selected = append(selected, candidates[index*len(candidates)/limit])
	}
	return selected
}

var skippedSuffixes = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".pdf": true, ".zip": true,
	".gz": true, ".tar": true, ".bin": true, ".wasm": true, ".woff": true, ".woff2": true, ".ttf": true,
	".lock": true, ".sum": true, ".jsonl": true,
}

var selectSourceSuffixes = map[string]bool{".go": true, ".ts": true, ".tsx": true, ".swift": true, ".py": true}

func textPath(value string) bool {
	if strings.Contains(value, "/testdata/") || strings.HasPrefix(value, ".") {
		return false
	}
	return !skippedSuffixes[strings.ToLower(path.Ext(value))]
}

func sourcePath(value string) bool {
	return selectSourceSuffixes[strings.ToLower(path.Ext(value))]
}

// goldPath is the frozen test-or-spec rule: the gold of a change is exactly
// the set of these paths the change itself touched.
func goldPath(value string) bool {
	name := path.Base(value)
	switch {
	case strings.HasSuffix(name, "_test.go"), strings.HasSuffix(name, "_test.py"):
		return true
	case strings.HasSuffix(name, ".test.ts"), strings.HasSuffix(name, ".test.tsx"):
		return true
	case strings.HasSuffix(name, "Tests.swift"):
		return true
	case strings.HasPrefix(value, "docs/specs/"):
		return true
	}
	return false
}

// isTestPath mirrors the index's broader test rule, used only by the stem
// baseline, never by the gold.
func isTestPath(value string) bool {
	name := strings.ToLower(path.Base(value))
	wrapped := "/" + strings.ToLower(value) + "/"
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.py") ||
		strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") ||
		strings.Contains(wrapped, "/test/") || strings.Contains(wrapped, "/tests/")
}
