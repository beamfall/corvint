package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// generate builds an unseen change-task set from a local Git history
// (CWT-V0-012): every non-merge commit after --since that touched between
// --min-files and --max-files text files becomes one co-change task whose
// subject is the largest modified non-test source file and whose gold is the
// other files the same commit touched. The snapshot at the parent commit is
// written in the corpus shape `run` reads, so the set needs no download and a
// repository younger than any model's training data cannot be memorised.
func generate(arguments []string, stdout, stderr io.Writer) int {
	options, err := parseGenerateArguments(arguments)
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial generate:", err)
		return 2
	}
	document, err := generateManifest(context.Background(), options)
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial generate:", err)
		return 2
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial generate:", err)
		return 2
	}
	target := filepath.Join(options.output, "tasks.json")
	if err := os.WriteFile(target, append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintln(stderr, "cw-trial generate:", err)
		return 2
	}
	fmt.Fprintf(stdout, "generated %d tasks from %d candidate commits into %s\n", len(document.Tasks), document.Population, target)
	return 0
}

type generateOptions struct {
	repo, name, since, output string
	limit, minFiles, maxFiles int
}

type generatedManifest struct {
	Name         string         `json:"name"`
	Partition    string         `json:"partition"`
	Source       string         `json:"source"`
	Repository   string         `json:"repository"`
	Since        string         `json:"since"`
	Population   int            `json:"population"`
	NovelGold    int            `json:"novel_gold_paths"`
	Counterparts int            `json:"counterpart_gold_paths"`
	GoldPaths    int            `json:"gold_paths"`
	SamplingRule string         `json:"sampling_rule"`
	Tasks        []task         `json:"tasks"`
	Snapshots    map[string]int `json:"snapshots"`
	Kinds        map[string]int `json:"per_kind_counts"`
	Languages    map[string]int `json:"per_language_counts"`
}

const (
	generateMaxDiffBytes  = 8 << 10
	generateMaxFileBytes  = 1 << 20
	generateMaxTaskChars  = 32_000
	generateDefaultLimit  = 40
	generateDefaultMin    = 2
	generateDefaultMax    = 6
	generateSamplingRule  = "non-merge commits after --since, oldest first, touching between --min-files and --max-files text files with at least one modified non-test source file; subject = the modified non-test source file with the largest diff; gold = every other touched file, kind test-file or source-file by path, plus the subject's same-directory test counterpart by stem when the parent tree holds one (kind test-file, counted under counterpart_gold_paths); select index floor(i*N/n) for i in 0..n-1 over the candidates in commit order"
	generateCandidateKind = "change"
)

func parseGenerateArguments(arguments []string) (generateOptions, error) {
	options := generateOptions{}
	flags := flag.NewFlagSet("cw-trial generate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.repo, "repo", "", "local Git repository to read")
	flags.StringVar(&options.name, "name", "", "repository name as OWNER/NAME for the corpus directory")
	flags.StringVar(&options.since, "since", "", "only commits after this date (git --since syntax)")
	flags.StringVar(&options.output, "output", "", "directory receiving tasks.json and corpus/")
	flags.IntVar(&options.limit, "limit", generateDefaultLimit, "maximum tasks")
	flags.IntVar(&options.minFiles, "min-files", generateDefaultMin, "minimum touched text files per commit")
	flags.IntVar(&options.maxFiles, "max-files", generateDefaultMax, "maximum touched text files per commit")
	if err := flags.Parse(arguments); err != nil {
		return options, err
	}
	if options.repo == "" || options.name == "" || options.since == "" || options.output == "" || flags.NArg() != 0 {
		return options, errors.New("usage: cw-trial generate --repo DIR --name OWNER/NAME --since DATE --output DIR [--limit N] [--min-files N] [--max-files N]")
	}
	if strings.Count(options.name, "/") != 1 || options.limit < 1 || options.minFiles < 2 || options.maxFiles < options.minFiles {
		return options, errors.New("--name must be OWNER/NAME, --limit at least 1, --min-files at least 2 and not above --max-files")
	}
	return options, nil
}

type touchedFile struct {
	path, status string
	diffBytes    int
}

type candidateCommit struct {
	commit, parent, message string
	files                   []touchedFile
}

func generateManifest(ctx context.Context, options generateOptions) (*generatedManifest, error) {
	candidates, err := candidateCommits(ctx, options)
	if err != nil {
		return nil, err
	}
	selected := evenlySpaced(candidates, options.limit)
	document := &generatedManifest{
		Name: "unseen-" + strings.ReplaceAll(options.name, "/", "-"), Partition: "unseen",
		Source:     "cw-trial generate over the local history of " + options.name + " (co-change tasks, CWT-V0-012)",
		Repository: options.name, Since: options.since, Population: len(candidates), SamplingRule: generateSamplingRule,
		Snapshots: map[string]int{}, Kinds: map[string]int{}, Languages: map[string]int{},
	}
	for _, candidate := range selected {
		item, novel, counterparts, err := generateTask(ctx, options, candidate)
		if err != nil {
			return nil, err
		}
		if _, written := document.Snapshots[candidate.parent]; !written {
			count, err := writeSnapshot(ctx, options, candidate.parent)
			if err != nil {
				return nil, err
			}
			document.Snapshots[candidate.parent] = count
		}
		document.Tasks = append(document.Tasks, item)
		document.NovelGold += novel
		document.Counterparts += counterparts
		for _, values := range item.Gold {
			document.GoldPaths += len(values)
		}
		document.Kinds[item.Kind]++
		document.Languages[strings.TrimPrefix(path.Ext(item.ChangedFile), ".")]++
	}
	if len(document.Tasks) == 0 {
		return nil, errors.New("no commit after --since qualifies")
	}
	return document, nil
}

// candidateCommits lists qualifying commits oldest first.
func candidateCommits(ctx context.Context, options generateOptions) ([]candidateCommit, error) {
	listing, err := gitOutput(ctx, options.repo, "log", "--no-merges", "--reverse", "--since="+options.since, "--format=%H %P")
	if err != nil {
		return nil, err
	}
	candidates := make([]candidateCommit, 0)
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		candidate, ok, err := inspectCommit(ctx, options, fields[0], fields[1])
		if err != nil {
			return nil, err
		}
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, nil
}

func inspectCommit(ctx context.Context, options generateOptions, commit, parent string) (candidateCommit, bool, error) {
	status, err := gitOutput(ctx, options.repo, "diff-tree", "--no-commit-id", "--name-status", "-r", "--diff-filter=AM", parent, commit)
	if err != nil {
		return candidateCommit{}, false, err
	}
	files := make([]touchedFile, 0)
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !textPath(fields[1]) {
			continue
		}
		files = append(files, touchedFile{path: fields[1], status: fields[0]})
	}
	if len(files) < options.minFiles || len(files) > options.maxFiles {
		return candidateCommit{}, false, nil
	}
	subjectFound := false
	for index := range files {
		if files[index].status != "M" || isTestPath(files[index].path) || !sourcePath(files[index].path) {
			continue
		}
		diff, err := gitOutput(ctx, options.repo, "diff", "--no-color", parent, commit, "--", files[index].path)
		if err != nil {
			return candidateCommit{}, false, err
		}
		files[index].diffBytes = len(diff)
		subjectFound = true
	}
	if !subjectFound {
		return candidateCommit{}, false, nil
	}
	message, err := gitOutput(ctx, options.repo, "log", "-1", "--format=%B", commit)
	if err != nil {
		return candidateCommit{}, false, err
	}
	return candidateCommit{commit: commit, parent: parent, message: strings.TrimSpace(message), files: files}, true, nil
}

func generateTask(ctx context.Context, options generateOptions, candidate candidateCommit) (task, int, int, error) {
	subject := candidate.files[0]
	for _, file := range candidate.files {
		if file.diffBytes > subject.diffBytes {
			subject = file
		}
	}
	gold := map[string][]string{}
	novel := 0
	for _, file := range candidate.files {
		if file.path == subject.path {
			continue
		}
		kind := "source-file"
		if isTestPath(file.path) {
			kind = "test-file"
		}
		gold[kind] = append(gold[kind], file.path)
		if file.status == "A" {
			novel++
		}
	}
	counterparts := 0
	for _, counterpart := range testCounterparts(ctx, options, candidate.parent, subject.path) {
		if !slices.Contains(gold["test-file"], counterpart) {
			gold["test-file"] = append(gold["test-file"], counterpart)
			counterparts++
		}
	}
	for kind := range gold {
		sort.Strings(gold[kind])
	}
	diff, err := gitOutput(ctx, options.repo, "diff", "--no-color", candidate.parent, candidate.commit, "--", subject.path)
	if err != nil {
		return task{}, 0, 0, err
	}
	diff, _ = truncate(diffBody(diff), generateMaxDiffBytes)
	text := "The file below was changed with the intent described. Which other file(s) in this repository must change alongside it?\n\nChanged file: " + subject.path +
		"\n\nIntent:\n" + candidate.message + "\n\nDiff of the changed file:\n" + diff +
		"\n\nReport each path as a claim of kind test-file for a test file and source-file otherwise."
	text, _ = truncate(text, generateMaxTaskChars)
	return task{
		ID: "cochange:" + candidate.commit[:12], Kind: generateCandidateKind, Repo: options.name,
		BaseCommit: candidate.parent, ChangedFile: subject.path, Text: text, Gold: gold,
		Source: map[string]any{"commit": candidate.commit, "touched_files": len(candidate.files), "novel_gold_paths": novel, "counterpart_gold_paths": counterparts},
	}, novel, counterparts, nil
}

// testCounterparts lists the test files in the subject's directory of the
// parent tree that share its stem: the files a reviewer opens whether or not
// the commit touched them.
func testCounterparts(ctx context.Context, options generateOptions, parent, subject string) []string {
	directory := path.Dir(subject)
	listing, err := gitOutput(ctx, options.repo, "ls-tree", "--name-only", parent, "--", directory+"/")
	if err != nil {
		return nil
	}
	stem := testStem(subject)
	found := make([]string, 0, 1)
	for _, candidate := range strings.Split(listing, "\n") {
		if candidate == "" || candidate == subject || !isTestPath(candidate) || testStem(candidate) != stem {
			continue
		}
		found = append(found, candidate)
	}
	sort.Strings(found)
	return found
}

// testStem is the lower-case base name without its extension and without a
// test marker, so `cache.go`, `cache_test.go`, `test_cache.py`, and
// `cache.spec.ts` share one stem.
func testStem(value string) string {
	name := strings.ToLower(strings.TrimSuffix(path.Base(value), path.Ext(value)))
	name = strings.TrimPrefix(name, "test_")
	for _, marker := range []string{"_test", ".test", ".spec", "_spec"} {
		name = strings.TrimSuffix(name, marker)
	}
	return name
}

// diffBody drops the diff header so the task carries hunks, as the released
// edit2ripple tasks do.
func diffBody(diff string) string {
	if index := strings.Index(diff, "\n@@"); index >= 0 {
		return diff[index+1:]
	}
	return diff
}

// writeSnapshot writes corpus/OWNER__NAME/PARENT.chunks.jsonl with one file
// row per text file of the parent tree, whole and untruncated, read from a
// single `git archive` of the revision.
func writeSnapshot(ctx context.Context, options generateOptions, revision string) (int, error) {
	archived, err := gitBytes(ctx, options.repo, "archive", "--format=tar", revision)
	if err != nil {
		return 0, err
	}
	directory := filepath.Join(options.output, "corpus", strings.ReplaceAll(options.name, "/", "__"))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return 0, err
	}
	file, err := os.Create(filepath.Join(directory, revision+".chunks.jsonl"))
	if err != nil {
		return 0, err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	reader := tar.NewReader(bytes.NewReader(archived))
	written := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		if header.Typeflag != tar.TypeReg || !textPath(header.Name) || header.Size > generateMaxFileBytes {
			continue
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return 0, err
		}
		if isBinary(content) {
			continue
		}
		row := map[string]any{"kind": "file", "path": header.Name, "repo": options.name, "base_commit": revision, "text": string(content)}
		encoded, err := json.Marshal(row)
		if err != nil {
			return 0, err
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return 0, err
		}
		written++
	}
	if err := writer.Flush(); err != nil {
		return 0, err
	}
	return written, nil
}

// evenlySpaced keeps the released sets' rule: index floor(i*N/n).
func evenlySpaced(candidates []candidateCommit, limit int) []candidateCommit {
	if len(candidates) <= limit {
		return candidates
	}
	selected := make([]candidateCommit, 0, limit)
	for index := 0; index < limit; index++ {
		selected = append(selected, candidates[index*len(candidates)/limit])
	}
	return selected
}

var generateSkippedSuffixes = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".pdf": true, ".zip": true,
	".gz": true, ".tar": true, ".bin": true, ".wasm": true, ".woff": true, ".woff2": true, ".ttf": true,
	".lock": true, ".sum": true, ".jsonl": true,
}

var generateSourceSuffixes = map[string]bool{
	".go": true, ".py": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".rs": true,
	".java": true, ".kt": true, ".swift": true, ".rb": true, ".c": true, ".cc": true, ".cpp": true, ".h": true,
	".cs": true, ".sh": true,
}

func textPath(value string) bool {
	if strings.Contains(value, "/testdata/") || strings.HasPrefix(value, ".") {
		return false
	}
	return !generateSkippedSuffixes[strings.ToLower(path.Ext(value))]
}

func sourcePath(value string) bool {
	return generateSourceSuffixes[strings.ToLower(path.Ext(value))]
}

// isTestPath mirrors the index's test rule for gold kinds.
func isTestPath(value string) bool {
	name := strings.ToLower(path.Base(value))
	wrapped := "/" + strings.ToLower(value) + "/"
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.py") ||
		strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.Contains(wrapped, "/test/") || strings.Contains(wrapped, "/tests/")
}

func gitOutput(ctx context.Context, repo string, arguments ...string) (string, error) {
	raw, err := gitBytes(ctx, repo, arguments...)
	return string(raw), err
}

func gitBytes(ctx context.Context, repo string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repo, "-c", "core.quotepath=false"}, arguments...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
