// Command retrieval-bench runs Agent Retrieval Bench samples (arXiv 2607.24882)
// against `corvint query`, `corvint context`, `corvint impact`, and `corvint affected` beside
// a deterministic grep baseline and reports recall, MRR, file F1, and
// selective success with confidence intervals. It
// reads the bench's JSONL and corpus snapshots, never downloads anything, and
// writes only the report and temporary snapshot copies it removes.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	profile           = "corvint-retrieval-bench/0"
	maxSampleBytes    = 4 << 20
	maxSamplesFile    = 256 << 20
	maxChunkLineBytes = 64 << 20
	maxQueryChars     = 2_000 // gokernel.MaxQueryCharacters: corvint query's own bound
	maxGrepFileBytes  = 1 << 20
	queryTimeout      = 2 * time.Minute
	binaryProbeBytes  = 8 << 10
	defaultLimit      = 20
	maxLimit          = 50     // corvint query and impact both refuse a larger limit; affected takes none
	dirtyMarker       = "\n\n" // a dirty edit in every language
	maxOutputBytes    = 8 << 20
	maxErrorBytes     = 64 << 10
	verdictNoGoldWord = "no_gold"
)

var tokenPattern = regexp.MustCompile(`[A-Za-z0-9]+`)
var camelPattern = regexp.MustCompile(`([a-z])([A-Z])`)

// sample is one bench row. query, gold, and metadata keep the bench's own
// shape; the accessors below replicate the bench's baseline.py rules.
type sample struct {
	ID         string         `json:"id"`
	TaskType   string         `json:"task_type"`
	Repo       string         `json:"repo"`
	BaseCommit string         `json:"base_commit"`
	Query      map[string]any `json:"query"`
	Gold       map[string]any `json:"gold"`
	Metadata   map[string]any `json:"metadata"`
}

// options is one invocation.
type options struct {
	samples          string
	corpus           string
	snapshots        map[string]string
	corvintGo        string
	limit            int
	maxSamples       int
	taskTypes        map[string]bool
	output           string
	arms             map[string]bool
	summaries        []string
	snapshotLatency  bool
	registrationPath string
	contextPackets   string
	captureInfo      *os.FileInfo
}

// arm is one retriever's answer for one sample: a ranked file list and
// whether it abstained. Query abstains on OUT_OF_SCOPE; context abstains on
// NO_CANDIDATES; grep abstains when no file matches any query term; impact abstains on OUT_OF_SCOPE or when nothing
// but the changed file itself is ranked; affected abstains when its plan
// selects no test beyond the changed and given files.
type arm struct {
	Ranked      []string `json:"ranked"`
	Abstained   bool     `json:"abstained"`
	State       string   `json:"state,omitempty"`
	TopScore    float64  `json:"top_score,omitempty"`
	PacketBytes int      `json:"packet_bytes,omitempty"`
	Error       string   `json:"error,omitempty"`
	WallMillis  float64  `json:"wall_ms,omitempty"`
	CacheState  string   `json:"cache_state,omitempty"`
	ColdMillis  float64  `json:"cold_wall_ms,omitempty"`
}

type sampleReport struct {
	ID             string             `json:"id"`
	TaskType       string             `json:"task_type"`
	Repo           string             `json:"repo"`
	BaseCommit     string             `json:"base_commit"`
	Stratum        string             `json:"stratum"`
	Partition      string             `json:"partition"`
	Gold           []string           `json:"gold"`
	Given          []string           `json:"given"`
	QueryChars     int                `json:"query_chars"`
	QueryTruncated bool               `json:"query_truncated"`
	Arms           map[string]arm     `json:"arms"`
	Metrics        map[string]metrics `json:"metrics"`
}

// metrics is one arm's score on one sample, or a mean over a group.
type metrics map[string]float64

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	configuration, err := parseOptions(arguments)
	if err != nil {
		fmt.Fprintln(stderr, "retrieval-bench:", err)
		return 2
	}
	if configuration.contextPackets != "" {
		configuration.captureInfo = new(os.FileInfo)
	}
	var report map[string]any
	if len(configuration.summaries) > 0 {
		report, err = resummarize(configuration)
	} else {
		report, err = bench(ctx, configuration, runCorvint, runContext, runImpact, runAffected)
	}
	if err != nil {
		fmt.Fprintln(stderr, "retrieval-bench:", err)
		return 2
	}
	if err := validateRegistrationOutput(configuration); err != nil {
		fmt.Fprintln(stderr, "retrieval-bench:", err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(report)
	if err != nil {
		fmt.Fprintln(stderr, "retrieval-bench: cannot encode the report")
		return 2
	}
	encoded = append(encoded, '\n')
	if configuration.output == "" {
		_, err = stdout.Write(encoded)
	} else if configuration.contextPackets != "" {
		err = writeCaptureReport(configuration.output, encoded, *configuration.captureInfo)
	} else {
		err = os.WriteFile(configuration.output, encoded, 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "retrieval-bench: cannot write the report")
		return 2
	}
	return 0
}

type stringsFlag []string

func (values *stringsFlag) String() string { return strings.Join(*values, ",") }
func (values *stringsFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func parseOptions(arguments []string) (options, error) {
	flags := flag.NewFlagSet("retrieval-bench", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var snapshots, taskTypes, summaries, armList stringsFlag
	configuration := options{snapshots: map[string]string{}, taskTypes: map[string]bool{}}
	flags.StringVar(&configuration.samples, "samples", "", "bench JSONL file (required)")
	flags.StringVar(&configuration.corpus, "corpus", "", "directory of corpus snapshots named after repo and base commit")
	flags.Var(&snapshots, "snapshot", "OWNER/NAME@COMMIT=PATH snapshot override (repeatable)")
	flags.StringVar(&configuration.corvintGo, "corvint", "corvint", "corvint executable")
	flags.IntVar(&configuration.limit, "limit", defaultLimit, "ranked files per arm (k)")
	flags.IntVar(&configuration.maxSamples, "max-samples", 0, "stop after N samples (0 = all)")
	flags.Var(&taskTypes, "task-type", "keep only this task_type (repeatable)")
	flags.BoolVar(&configuration.snapshotLatency, "snapshot-latency", false, "measure matched cold and observed-hit context latency")
	flags.StringVar(&configuration.registrationPath, "registration", "", "exclusive pre-run registration path (default OUTPUT.registration.json)")
	flags.StringVar(&configuration.contextPackets, "context-packets", "", "exclusive private context diagnostic JSONL path")
	flags.StringVar(&configuration.output, "output", "", "report path (default stdout)")
	flags.Var(&armList, "arms", "arm to run: corvint, context, grep, grep-ident, bm25, impact, affected (repeatable or comma-separated; default all)")
	flags.Var(&summaries, "summarize", "existing report to re-summarize, merging arms by sample id (repeatable; runs nothing)")
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, errors.New("unexpected positional arguments")
	}
	configuration.summaries = summaries
	if len(summaries) > 0 && (configuration.samples != "" || configuration.corpus != "" || len(snapshots) > 0) {
		return options{}, errors.New("--summarize takes no --samples, --corpus, or --snapshot")
	}
	if len(summaries) == 0 && configuration.samples == "" {
		return options{}, errors.New("--samples is required")
	}
	configuration.arms = allArms()
	if len(armList) > 0 {
		selected, err := selectArms(strings.Split(strings.Join(armList, ","), ","))
		if err != nil {
			return options{}, err
		}
		configuration.arms = selected
	}
	if configuration.snapshotLatency && !configuration.arms["context"] {
		return options{}, errors.New("--snapshot-latency requires the context arm")
	}
	if len(summaries) == 0 && configuration.registrationPath == "" && configuration.output != "" {
		configuration.registrationPath = configuration.output + ".registration.json"
	}
	if configuration.snapshotLatency && configuration.registrationPath == "" {
		return options{}, errors.New("--snapshot-latency requires --registration or --output")
	}
	if configuration.contextPackets != "" && (len(summaries) > 0 || !configuration.arms["context"] || configuration.registrationPath == "") {
		return options{}, errors.New("--context-packets requires context arm, --registration or --output, and no --summarize")
	}
	if configuration.limit < 1 || configuration.limit > maxLimit {
		return options{}, fmt.Errorf("--limit must be within 1..%d", maxLimit)
	}
	if configuration.maxSamples < 0 {
		return options{}, errors.New("--max-samples must not be negative")
	}
	for _, entry := range snapshots {
		key, path, found := strings.Cut(entry, "=")
		repo, commit, keyed := strings.Cut(key, "@")
		if !found || !keyed || repo == "" || commit == "" || path == "" {
			return options{}, errors.New("--snapshot must be OWNER/NAME@COMMIT=PATH")
		}
		configuration.snapshots[key] = path
	}
	for _, taskType := range taskTypes {
		configuration.taskTypes[taskType] = true
	}
	if len(summaries) == 0 && configuration.corpus == "" && len(configuration.snapshots) == 0 {
		return options{}, errors.New("--corpus or --snapshot is required")
	}
	return configuration, nil
}

// retriever is a task-text Corvint arm behind a seam so the harness is testable without
// the binary: root is a Git worktree at the sample's base commit.
type retriever func(ctx context.Context, corvintGo, root, task string, limit int) (arm, error)

// impactRetriever is the impact arm behind the same seam: changed is the
// query's changed_file, a path relative to root.
type impactRetriever func(ctx context.Context, corvintGo, root, changed string, limit int) (arm, error)

// affectedRetriever is the affected arm behind the same seam: the arm dirties
// changed inside root for the call and restores it before returning.
type affectedRetriever func(ctx context.Context, corvintGo, root, changed string, limit int) (arm, error)

func bench(ctx context.Context, configuration options, corvint retriever, contextPacket retriever, impact impactRetriever, affected affectedRetriever) (map[string]any, error) {
	if err := validateRegistrationOutput(configuration); err != nil {
		return nil, err
	}
	samples, samplesDigest, skipped, err := readSamples(configuration)
	if err != nil {
		return nil, err
	}
	expected, err := planContextCapture(configuration, samples)
	if err != nil {
		return nil, err
	}
	identity, registered, err := registerBeforeRun(ctx, configuration, samples, samplesDigest)
	if err != nil {
		return nil, err
	}
	capture, err := openContextCapture(configuration, expected, registered)
	if err != nil {
		return nil, err
	}
	if capture != nil {
		defer capture.file.Close()
	}
	workspaces := newWorkspaces()
	defer workspaces.close()
	reports := make([]sampleReport, 0, len(samples))
	for ordinal, item := range samples {
		sampleCtx := ctx
		if capture != nil {
			sampleCtx = context.WithValue(ctx, captureScopeKey{}, &captureScope{capture, ordinal})
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		root, err := workspaces.open(ctx, configuration, item)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.ID, err)
		}
		if err := cleanWorkspace(ctx, root); err != nil {
			return nil, err
		}
		var cold arm
		if configuration.snapshotLatency {
			cold, err = prepareSnapshotSample(sampleCtx, configuration, item, root, contextPacket)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", item.ID, err)
			}
		}
		judged := judge(sampleCtx, configuration, corvint, contextPacket, impact, affected, item, root)
		if capture != nil && capture.err != nil {
			return nil, capture.err
		}
		if configuration.snapshotLatency {
			warm := judged.Arms["context"]
			if err := validateSnapshotPair(cold, warm); err != nil {
				return nil, fmt.Errorf("%s: %w", item.ID, err)
			}
			warm.ColdMillis = cold.WallMillis
			judged.Arms["context"] = warm
		}
		reports = append(reports, judged)
		if err := cleanWorkspace(ctx, root); err != nil {
			return nil, err
		}
	}
	if err := verifyRunIdentity(ctx, configuration, identity, registered); err != nil {
		return nil, err
	}
	if capture != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := capture.finish(); err != nil {
		return nil, err
	}
	return map[string]any{
		"profile":        profile,
		"samples_sha256": samplesDigest,
		"samples":        len(reports),
		"skipped":        skipped,
		"limit":          configuration.limit,
		"corvint":        identity,
		"registration":   registered,
		"arms":           summarize(reports, configuration.limit),
		"paired":         paired(reports),
		"latency":        latency(reports),
		"details":        reports,
	}, nil
}

// readSamples returns the kept samples, the file digest, and the count of
// samples skipped for the reason the bench's selective evaluator skips them:
// no gold and no `no_gold` label.
func readSamples(configuration options) ([]sample, string, map[string]int, error) {
	data, err := readBounded(configuration.samples, maxSamplesFile)
	if err != nil {
		return nil, "", nil, err
	}
	digest := sha256.Sum256(data)
	samples := make([]sample, 0, 64)
	skipped := map[string]int{"no_gold_unlabeled": 0}
	for number, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if len(line) > maxSampleBytes {
			return nil, "", nil, fmt.Errorf("sample on line %d exceeds %d bytes", number+1, maxSampleBytes)
		}
		var item sample
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, "", nil, fmt.Errorf("sample on line %d: %w", number+1, err)
		}
		if item.ID == "" || item.Repo == "" || item.BaseCommit == "" || item.TaskType == "" {
			return nil, "", nil, fmt.Errorf("sample on line %d lacks id, repo, base_commit, or task_type", number+1)
		}
		if len(configuration.taskTypes) > 0 && !configuration.taskTypes[item.TaskType] {
			continue
		}
		if stratum(item) == "positive" && len(goldFiles(item)) == 0 {
			skipped["no_gold_unlabeled"]++
			continue
		}
		samples = append(samples, item)
		if configuration.maxSamples > 0 && len(samples) == configuration.maxSamples {
			break
		}
	}
	return samples, hex.EncodeToString(digest[:]), skipped, nil
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	return data, nil
}

// queryText is the bench's own query_text_for_eval, byte for byte:
// json.dumps(query or {}, ensure_ascii=False, sort_keys=True), which puts a
// space after every comma and colon. Both arms see it; Corvint sees at most
// maxQueryChars of it.
func queryText(item sample) string {
	buffer := &bytes.Buffer{}
	pythonJSON(buffer, item.Query)
	return buffer.String()
}

func pythonJSON(buffer *bytes.Buffer, value any) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		buffer.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				buffer.WriteString(", ")
			}
			pythonJSON(buffer, key)
			buffer.WriteString(": ")
			pythonJSON(buffer, typed[key])
		}
		buffer.WriteByte('}')
	case []any:
		buffer.WriteByte('[')
		for index, element := range typed {
			if index > 0 {
				buffer.WriteString(", ")
			}
			pythonJSON(buffer, element)
		}
		buffer.WriteByte(']')
	case string:
		encoder := json.NewEncoder(buffer)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(typed)
		buffer.Truncate(buffer.Len() - 1)
	case float64:
		if typed == math.Trunc(typed) && math.Abs(typed) < 1e16 {
			fmt.Fprintf(buffer, "%.1f", typed)
			return
		}
		fmt.Fprint(buffer, typed)
	case bool:
		if typed {
			buffer.WriteString("true")
			return
		}
		buffer.WriteString("false")
	default:
		buffer.WriteString("null")
	}
}

func truncateQuery(text string) (string, bool) {
	if utf8.RuneCountInString(text) <= maxQueryChars {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:maxQueryChars]), true
}

// goldFiles replicates baseline.target_gold_files.
func goldFiles(item sample) []string {
	if item.Gold[verdictNoGoldWord] == true {
		return nil
	}
	if files := paths(item.Gold["files"]); len(files) > 0 {
		return files
	}
	switch item.TaskType {
	case "code2test":
		return paths(item.Gold["related_tests"])
	case "comment2context":
		context := paths(item.Gold["must_context_files"])
		if len(context) == 0 {
			context = paths(item.Gold["context_files"])
		}
		if len(context) > 0 {
			return context
		}
	}
	if root := paths(item.Gold["root_cause_files"]); len(root) > 0 {
		return root
	}
	return paths(item.Gold["related_tests"])
}

// givenFiles replicates baseline.given_files: context the agent already sees,
// excluded from candidates and never a success.
func givenFiles(item sample) []string {
	given := paths(item.Gold["given_files"])
	if len(given) == 0 && item.TaskType == "comment2context" {
		for _, key := range []string{"given_file", "path"} {
			if value, _ := item.Query[key].(string); value != "" {
				given = []string{value}
				break
			}
		}
	}
	return given
}

func hardNegatives(item sample) []string { return paths(item.Gold["negative_distractors"]) }

// stratum: positive, natural no-gold (metadata.organic), or counterfactual
// no-gold, which the bench reports separately.
func stratum(item sample) string {
	if item.Gold[verdictNoGoldWord] != true {
		return "positive"
	}
	if item.Metadata["organic"] == true {
		return "natural_no_gold"
	}
	return "counterfactual_no_gold"
}

// paths reads a list of paths given as strings or as objects with a path key,
// deduplicated in order.
func paths(value any) []string {
	items, _ := value.([]any)
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		path := ""
		switch typed := item.(type) {
		case string:
			path = typed
		case map[string]any:
			path, _ = typed["path"].(string)
		}
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}

// workspaces materializes each snapshot once as a Git worktree at the base
// commit: in place when the snapshot already is one, otherwise a temporary
// copy with a single commit, removed on close.
type workspaces struct {
	roots     map[string]string
	temporary []string
}

func newWorkspaces() *workspaces { return &workspaces{roots: map[string]string{}} }

func (space *workspaces) close() {
	for _, directory := range space.temporary {
		_ = os.RemoveAll(directory)
	}
}

func (space *workspaces) open(ctx context.Context, configuration options, item sample) (string, error) {
	key := item.Repo + "@" + item.BaseCommit
	if root, ready := space.roots[key]; ready {
		return root, nil
	}
	snapshot, err := resolveSnapshot(configuration, item)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(snapshot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		root, err := materialize(ctx, snapshot, writeChunkRows)
		if err != nil {
			return "", err
		}
		space.temporary = append(space.temporary, root)
		space.roots[key] = root
		return root, nil
	}
	if _, err := os.Stat(filepath.Join(snapshot, ".git")); err == nil {
		if err := checkWorktree(ctx, snapshot, item.BaseCommit); err != nil {
			return "", err
		}
	}
	root, err := materialize(ctx, snapshot, copyTree)
	if err != nil {
		return "", err
	}
	space.temporary = append(space.temporary, root)
	space.roots[key] = root
	return root, nil
}

// resolveSnapshot prefers an explicit --snapshot, then the release's own
// chunk file (`OWNER__NAME/COMMIT.chunks.jsonl`), then the one corpus
// directory whose name carries the repository name and the base commit.
func resolveSnapshot(configuration options, item sample) (string, error) {
	if path, explicit := configuration.snapshots[item.Repo+"@"+item.BaseCommit]; explicit {
		return path, nil
	}
	if configuration.corpus == "" {
		return "", fmt.Errorf("no snapshot for %s@%s", item.Repo, item.BaseCommit)
	}
	chunks := filepath.Join(configuration.corpus, strings.ReplaceAll(item.Repo, "/", "__"), item.BaseCommit+".chunks.jsonl")
	if info, err := os.Stat(chunks); err == nil && !info.IsDir() {
		return chunks, nil
	}
	entries, err := os.ReadDir(configuration.corpus)
	if err != nil {
		return "", err
	}
	name := item.Repo[strings.LastIndex(item.Repo, "/")+1:]
	commit := item.BaseCommit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	matches := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), name) && strings.Contains(entry.Name(), commit) {
			matches = append(matches, filepath.Join(configuration.corpus, entry.Name()))
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("%d corpus entries match %s@%s under %s", len(matches), item.Repo, item.BaseCommit, configuration.corpus)
	}
	return matches[0], nil
}

// checkWorktree admits a Git worktree only at the base commit and clean; the
// run still copies it, so no arm can ever write into the caller's snapshot.
func checkWorktree(ctx context.Context, snapshot, base string) error {
	head, err := git(ctx, snapshot, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("%s: %w", snapshot, err)
	}
	if head != base {
		return fmt.Errorf("%s is at %s, not %s", snapshot, head, base)
	}
	status, err := git(ctx, snapshot, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("%s: %w", snapshot, err)
	}
	if status != "" {
		return fmt.Errorf("%s is at %s but its worktree is not clean", snapshot, base)
	}
	return nil
}

// materialize builds a Git worktree in a temporary directory from a snapshot
// tree (copyTree) or from a release chunk file (writeChunkRows).
func materialize(ctx context.Context, snapshot string, write func(source, destination string) error) (string, error) {
	root, err := os.MkdirTemp("", "corvint-retrieval-bench-")
	if err != nil {
		return "", err
	}
	if err := write(snapshot, root); err != nil {
		_ = os.RemoveAll(root)
		return "", err
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "--allow-empty", "-m", "snapshot"}} {
		if _, err := git(ctx, root, arguments...); err != nil {
			_ = os.RemoveAll(root)
			return "", fmt.Errorf("git %s: %w", arguments[0], err)
		}
	}
	return root, nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		if relative == filepath.Join(".corvint", "index") || relative == ".git" {
			if !entry.IsDir() {
				return nil
			}
			return filepath.SkipDir
		}
		if entry.IsDir() && entry.Name() == ".git" && relative != "." {
			return filepath.SkipDir
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

// writeChunkRows rebuilds the snapshot the bench evaluated over from its
// chunk file: every `kind: file` row carries one file's full text. Symbol
// rows are ignored, as is any row whose path could leave the destination.
func writeChunkRows(source, destination string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), maxChunkLineBytes)
	written := 0
	for scanner.Scan() {
		var row struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return fmt.Errorf("%s: %w", source, err)
		}
		if row.Kind != "file" || !safeRelative(row.Path) {
			continue
		}
		target := filepath.Join(destination, filepath.FromSlash(row.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(row.Text), 0o644); err != nil {
			return err
		}
		written++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	if written == 0 {
		return fmt.Errorf("%s holds no file rows", source)
	}
	return nil
}

func safeRelative(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." || part == ".git" {
			return false
		}
	}
	return true
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func git(ctx context.Context, root string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks"}, arguments...)...)
	command.Dir = root
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=retrieval-bench", "GIT_AUTHOR_EMAIL=bench@localhost",
		"GIT_COMMITTER_NAME=retrieval-bench", "GIT_COMMITTER_EMAIL=bench@localhost",
	)
	ownProcessGroup(command)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// judge runs all five arms on one sample and scores them.
func judge(ctx context.Context, configuration options, corvint retriever, contextPacket retriever, impact impactRetriever, affected affectedRetriever, item sample, root string) sampleReport {
	full := queryText(item)
	task, truncated := truncateQuery(full)
	given := givenFiles(item)
	report := sampleReport{
		ID: item.ID, TaskType: item.TaskType, Repo: item.Repo, BaseCommit: item.BaseCommit,
		Stratum: stratum(item), Partition: partition(item.Repo), Gold: goldFiles(item), Given: given,
		QueryChars: utf8.RuneCountInString(full), QueryTruncated: truncated,
		Arms: map[string]arm{}, Metrics: map[string]metrics{},
	}
	timed := func(name string, produce func() arm) {
		if !configuration.arms[name] {
			return
		}
		started := time.Now()
		answer := produce()
		answer.WallMillis = round(float64(time.Since(started).Nanoseconds()) / 1e6)
		report.Arms[name] = answer
	}
	timed("corvint", func() arm {
		state := cacheState(root)
		corvintArm, err := corvint(ctx, configuration.corvintGo, root, task, configuration.limit)
		if err != nil {
			corvintArm = arm{Error: err.Error()}
		}
		corvintArm.Ranked = without(corvintArm.Ranked, given, configuration.limit)
		corvintArm.CacheState = state
		return corvintArm
	})
	phase := "ordinary"
	if configuration.snapshotLatency {
		phase = "hit"
	}
	contextCtx, observation := beginContextCapture(ctx, item, phase)
	timed("context", func() arm {
		state := cacheState(root)
		contextArm, err := contextPacket(contextCtx, configuration.corvintGo, root, full, configuration.limit)
		if err != nil {
			contextArm = arm{Error: err.Error()}
		}
		contextArm.Ranked = without(contextArm.Ranked, given, configuration.limit)
		if !configuration.snapshotLatency {
			contextArm.CacheState = state
		}
		return contextArm
	})
	_ = flushContextCapture(ctx, observation)
	timed("grep", func() arm { return runGrep(root, full, given, configuration.limit) })
	timed("grep-ident", func() arm {
		return runGrepIdent(corpusFor(root), queryIdentifiers(item.Query), given, configuration.limit)
	})
	timed("bm25:all", func() arm { return runBM25(corpusFor(root), terms(full), given, configuration.limit) })
	timed("bm25:ident", func() arm {
		return runBM25(corpusFor(root), identifierTerms(queryIdentifiers(item.Query)), given, configuration.limit)
	})
	timed("impact", func() arm {
		state := cacheState(root)
		answer := judgeImpact(ctx, configuration, impact, item, root, given)
		answer.CacheState = state
		return answer
	})
	timed("affected", func() arm {
		state := cacheState(root)
		answer := judgeAffected(ctx, configuration, affected, item, root, given)
		answer.CacheState = state
		return answer
	})
	for name, answer := range report.Arms {
		report.Metrics[name] = score(answer, report.Gold, hardNegatives(item), report.Stratum, configuration.limit)
	}
	return report
}

// judgeImpact runs the impact arm on one sample. It needs the query's
// changed_file; the changed file is the query's own subject and is removed
// from the ranking with the given files. A refusal is an error, never an
// abstention.
func judgeImpact(ctx context.Context, configuration options, impact impactRetriever, item sample, root string, given []string) arm {
	changed := changedFile(item)
	if changed == "" {
		return arm{Error: "no changed_file in query"}
	}
	answer, err := impact(ctx, configuration.corvintGo, root, changed, configuration.limit)
	if err != nil {
		return arm{Error: err.Error()}
	}
	answer.Ranked = without(answer.Ranked, append([]string{changed}, given...), configuration.limit)
	answer.Abstained = answer.State == "OUT_OF_SCOPE" || len(answer.Ranked) == 0
	return answer
}

// judgeAffected runs the affected arm on one sample. It needs the query's
// changed_file, which is removed from the ranking with the given files; a
// plan that selects nothing else is an abstention, and a failed invocation
// or a copy left dirty is an error.
func judgeAffected(ctx context.Context, configuration options, affected affectedRetriever, item sample, root string, given []string) arm {
	changed := changedFile(item)
	if changed == "" {
		return arm{Error: "no changed_file in query"}
	}
	answer, err := affected(ctx, configuration.corvintGo, root, changed, configuration.limit)
	if err != nil {
		return arm{Error: err.Error()}
	}
	answer.Ranked = without(answer.Ranked, append([]string{changed}, given...), configuration.limit)
	answer.Abstained = len(answer.Ranked) == 0
	return answer
}

// changedFile is the query's string `changed_file`, empty when absent.
func changedFile(item sample) string {
	changed, _ := item.Query["changed_file"].(string)
	return changed
}

func without(ranked, given []string, limit int) []string {
	excluded := map[string]bool{}
	for _, path := range given {
		excluded[path] = true
	}
	result := make([]string, 0, len(ranked))
	for _, path := range ranked {
		if excluded[path] {
			continue
		}
		result = append(result, path)
		if len(result) == limit {
			break
		}
	}
	return result
}

// runCorvint is the real Corvint arm: one `corvint query`, ranked by each
// result's own path in packet order, distinct.
func runCorvint(ctx context.Context, corvintGo, root, task string, limit int) (arm, error) {
	stdout, err := runCorvintGo(ctx, corvintGo, "--root", root, "query", "--task", task, "--limit", fmt.Sprint(limit))
	if err != nil {
		return arm{}, fmt.Errorf("corvint query: %w", err)
	}
	return parsePacket(stdout)
}

// runContext is the real context arm: one `corvint context` over the bench's
// full query text, ranked by task-context result IDs in packet order, distinct.
func runContext(ctx context.Context, corvintGo, root, task string, limit int) (arm, error) {
	if observation, _ := ctx.Value(captureObservationKey{}).(*captureObservation); observation != nil {
		observation.task = task
	}
	stdout, diagnostic, err := runCorvintGoObserved(ctx, corvintGo, true, "--root", root, "context", "--task", task, "--limit", fmt.Sprint(limit))
	if err != nil {
		return arm{}, fmt.Errorf("corvint context: %w", err)
	}
	answer, err := parseContextPacket(stdout)
	if observation, _ := ctx.Value(captureObservationKey{}).(*captureObservation); observation != nil {
		observation.parseErr = err
	}
	answer.CacheState = observedSnapshotState(diagnostic)
	return answer, err
}

// runCorvintGo runs one corvint invocation under the query timeout with bounded
// output, in its own process group so a cancelled run leaves no Git child
// behind to race the workspace cleanup.
func runCorvintGo(ctx context.Context, corvintGo string, arguments ...string) ([]byte, error) {
	stdout, _, err := runCorvintGoObserved(ctx, corvintGo, false, arguments...)
	return stdout, err
}

func runCorvintGoObserved(ctx context.Context, corvintGo string, observe bool, arguments ...string) (output []byte, diagnostic string, runErr error) {
	runCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, corvintGo, arguments...)
	if observe {
		command.Env = append(os.Environ(), "CORVINT_BENCH_SNAPSHOT_TRACE=1")
	}
	stdout, stderr := &boundedBuffer{limit: maxOutputBytes}, &boundedBuffer{limit: maxErrorBytes}
	if observation, _ := ctx.Value(captureObservationKey{}).(*captureObservation); observe && observation != nil {
		defer func() {
			observation.observed = true
			observation.stdout, observation.stderr = output, stderr.Bytes()
			observation.stderrTruncated = stderr.overflow
			observation.transportErr = runErr
		}()
	}
	command.Stdout, command.Stderr = stdout, stderr
	ownProcessGroup(command)
	if err := command.Run(); err != nil {
		return nil, stderr.String(), fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.overflow {
		return nil, stderr.String(), fmt.Errorf("output exceeds %d bytes", maxOutputBytes)
	}
	return stdout.Bytes(), stderr.String(), nil
}

// boundedBuffer keeps the first limit bytes and remembers that more arrived.
// It deliberately does not embed bytes.Buffer: io.Copy would take the
// embedded ReadFrom and bypass the bound.
type boundedBuffer struct {
	data     []byte
	limit    int
	overflow bool
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	room := buffer.limit - len(buffer.data)
	if len(data) > room {
		buffer.overflow = true
		data = data[:max(0, room)]
	}
	buffer.data = append(buffer.data, data...)
	return written, nil
}

func (buffer *boundedBuffer) Bytes() []byte  { return buffer.data }
func (buffer *boundedBuffer) String() string { return string(buffer.data) }

// runImpact is the real impact arm: one `corvint impact CHANGED`, ranked by
// every evidence path in packet order, distinct. The command refuses paths
// without a reverse-import rule and repositories without a Go module; a
// refusal is returned as an error.
func runImpact(ctx context.Context, corvintGo, root, changed string, limit int) (arm, error) {
	stdout, err := runCorvintGo(ctx, corvintGo, "--root", root, "impact", changed, "--limit", fmt.Sprint(limit))
	if err != nil {
		return arm{}, fmt.Errorf("corvint impact: %w", err)
	}
	return parseImpactPacket(stdout)
}

// runAffected is the real affected arm: the changed file is dirtied in the
// workspace copy, one `corvint affected` is run (it takes no path: the
// change is whatever is dirty), and the file is restored byte for byte before
// the copy is checked clean. The ranking is every selected test in plan
// order, distinct; the limit is applied by judgeAffected.
func runAffected(ctx context.Context, corvintGo, root, changed string, limit int) (arm, error) {
	stdout, err := withDirtyFile(ctx, root, changed, func() ([]byte, error) {
		return runCorvintGo(ctx, corvintGo, "--root", root, "affected")
	})
	if err != nil {
		return arm{}, fmt.Errorf("corvint affected: %w", err)
	}
	return parseAffectedPlan(stdout)
}

// withDirtyFile appends the dirty marker to path under root, runs body, and
// restores the file's exact previous bytes whether or not body failed; the
// copy must then be clean under `git status --porcelain`, or the call fails.
func withDirtyFile(ctx context.Context, root, path string, body func() ([]byte, error)) (output []byte, err error) {
	target := filepath.Join(root, filepath.FromSlash(path))
	original, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, restoreFile(ctx, root, target, original)) }()
	if err := os.WriteFile(target, dirtied(original), 0o644); err != nil {
		return nil, err
	}
	return body()
}

func dirtied(original []byte) []byte {
	dirty := make([]byte, 0, len(original)+len(dirtyMarker))
	dirty = append(dirty, original...)
	return append(dirty, dirtyMarker...)
}

// restoreFile writes original back to target and verifies that root's
// worktree is clean again.
func restoreFile(ctx context.Context, root, target string, original []byte) error {
	if err := os.WriteFile(target, original, 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", target, err)
	}
	status, err := git(ctx, root, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if status != "" {
		return fmt.Errorf("workspace copy left dirty: %s", status)
	}
	return nil
}

// plan is the part of an affected-plan/0 document the affected arm reads.
type plan struct {
	Plan struct {
		Scope    string `json:"scope"`
		Selected []struct {
			Tests []string `json:"tests"`
		} `json:"selected"`
	} `json:"plan"`
}

// parseAffectedPlan ranks an affected plan by every selected test in plan
// order, distinct, and keeps the plan's scope as the arm's state. Abstention
// is settled by judgeAffected once the changed file is removed.
func parseAffectedPlan(data []byte) (arm, error) {
	var document plan
	if err := json.Unmarshal(data, &document); err != nil {
		return arm{}, fmt.Errorf("corvint affected: unreadable plan: %w", err)
	}
	seen := map[string]bool{}
	ranked := make([]string, 0, len(document.Plan.Selected))
	for _, selection := range document.Plan.Selected {
		for _, test := range selection.Tests {
			if seen[test] {
				continue
			}
			seen[test] = true
			ranked = append(ranked, test)
		}
	}
	return arm{Ranked: ranked, State: document.Plan.Scope, PacketBytes: len(data)}, nil
}

// packet is the part of a query or impact packet the arms read.
type packet struct {
	Context struct {
		State   string `json:"state"`
		Results []struct {
			Evidence []struct {
				Path string `json:"path"`
			} `json:"evidence"`
		} `json:"results"`
	} `json:"context"`
}

func decodePacket(command string, data []byte) (packet, error) {
	var envelope packet
	if err := json.Unmarshal(data, &envelope); err != nil {
		return packet{}, fmt.Errorf("corvint %s: unreadable packet: %w", command, err)
	}
	return envelope, nil
}

// parseImpactPacket ranks an impact packet by every evidence path in packet
// order, distinct. Abstention is settled by judgeImpact once the changed file
// is removed.
func parseImpactPacket(data []byte) (arm, error) {
	envelope, err := decodePacket("impact", data)
	if err != nil {
		return arm{}, err
	}
	seen := map[string]bool{}
	ranked := make([]string, 0, len(envelope.Context.Results))
	for _, result := range envelope.Context.Results {
		for _, evidence := range result.Evidence {
			if seen[evidence.Path] {
				continue
			}
			seen[evidence.Path] = true
			ranked = append(ranked, evidence.Path)
		}
	}
	return arm{Ranked: ranked, State: envelope.Context.State, PacketBytes: len(data)}, nil
}

// parsePacket ranks a query packet by each result's own path, its first
// evidence row, distinct.
func parsePacket(data []byte) (arm, error) {
	envelope, err := decodePacket("query", data)
	if err != nil {
		return arm{}, err
	}
	seen := map[string]bool{}
	ranked := make([]string, 0, len(envelope.Context.Results))
	for _, result := range envelope.Context.Results {
		if len(result.Evidence) == 0 || seen[result.Evidence[0].Path] {
			continue
		}
		seen[result.Evidence[0].Path] = true
		ranked = append(ranked, result.Evidence[0].Path)
	}
	state := envelope.Context.State
	return arm{Ranked: ranked, State: state, Abstained: state == "OUT_OF_SCOPE" || len(ranked) == 0, PacketBytes: len(data)}, nil
}

// parseContextPacket ranks a task-context packet by each result ID, in packet
// order and distinct. Unlike query and impact, context emits its packet at the
// top level and uses NO_CANDIDATES for an empty result set; a packet whose
// `coverage.answerability.verdict` is `unsupported-conjunction` (TCP-V0-016)
// withheld its slot rows and is an abstention even when the reserved
// instruction rows remain.
func parseContextPacket(data []byte) (arm, error) {
	var envelope struct {
		State   string `json:"state"`
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
		Coverage struct {
			Answerability struct {
				Verdict string `json:"verdict"`
			} `json:"answerability"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return arm{}, fmt.Errorf("corvint context: unreadable packet: %w", err)
	}
	seen := map[string]bool{}
	ranked := make([]string, 0, len(envelope.Results))
	for _, result := range envelope.Results {
		if result.ID == "" || seen[result.ID] {
			continue
		}
		seen[result.ID] = true
		ranked = append(ranked, result.ID)
	}
	state := envelope.State
	withheld := envelope.Coverage.Answerability.Verdict == "unsupported-conjunction"
	if withheld {
		state = state + "/" + envelope.Coverage.Answerability.Verdict
	}
	return arm{Ranked: ranked, State: state, Abstained: state == "NO_CANDIDATES" || withheld || len(ranked) == 0, PacketBytes: len(data)}, nil
}

// tokenize replicates the bench's tokenizer: camelCase split, alphanumeric
// runs, lowercased.
func tokenize(text string) []string {
	spaced := camelPattern.ReplaceAllString(text, "$1 $2")
	tokens := tokenPattern.FindAllString(spaced, -1)
	for index := range tokens {
		tokens[index] = strings.ToLower(tokens[index])
	}
	return tokens
}

func terms(text string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, 32)
	for _, token := range tokenize(text) {
		if len(token) < 2 || seen[token] {
			continue
		}
		seen[token] = true
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

type grepHit struct {
	path        string
	distinct    int
	occurrences int
}

// runGrep is the baseline: every text file scored by the distinct query terms
// it contains (a path match counts double), ties broken by occurrences then
// path. No index, no learning, no abstention beyond an empty match.
func runGrep(root, text string, given []string, limit int) arm {
	wanted := terms(text)
	excluded := map[string]bool{}
	for _, path := range given {
		excluded[path] = true
	}
	hits := make([]grepHit, 0, 256)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".context-corvint" || entry.Name() == ".corvint" {
				return filepath.SkipDir
			}
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		relative = filepath.ToSlash(relative)
		if excluded[relative] || !entry.Type().IsRegular() {
			return nil
		}
		if hit, scored := scoreFile(path, relative, wanted); scored {
			hits = append(hits, hit)
		}
		return nil
	})
	sort.Slice(hits, func(left, right int) bool {
		if hits[left].distinct != hits[right].distinct {
			return hits[left].distinct > hits[right].distinct
		}
		if hits[left].occurrences != hits[right].occurrences {
			return hits[left].occurrences > hits[right].occurrences
		}
		return hits[left].path < hits[right].path
	})
	ranked := make([]string, 0, limit)
	for index := 0; index < len(hits) && index < limit; index++ {
		ranked = append(ranked, hits[index].path)
	}
	top := 0.0
	if len(hits) > 0 {
		top = float64(hits[0].distinct)
	}
	return arm{Ranked: ranked, Abstained: len(ranked) == 0, TopScore: top}
}

func scoreFile(path, relative string, wanted []string) (grepHit, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxGrepFileBytes {
		return grepHit{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil || isBinary(data) {
		return grepHit{}, false
	}
	content := strings.ToLower(string(data))
	pathTerms := map[string]bool{}
	for _, token := range tokenize(relative) {
		pathTerms[token] = true
	}
	hit := grepHit{path: relative}
	for _, term := range wanted {
		count := strings.Count(content, term)
		inPath := pathTerms[term]
		if count == 0 && !inPath {
			continue
		}
		hit.distinct++
		if inPath {
			hit.distinct++
		}
		hit.occurrences += count
	}
	return hit, hit.distinct > 0
}

func isBinary(data []byte) bool {
	probe := data
	if len(probe) > binaryProbeBytes {
		probe = probe[:binaryProbeBytes]
	}
	return bytes.IndexByte(probe, 0) >= 0
}

// score is the bench's sample_metrics for a positive sample, plus the
// selective verdict every sample carries: a no-gold sample succeeds by
// abstaining, a positive one by answering with a gold file in the top k.
func score(answer arm, gold, negatives []string, stratum string, limit int) metrics {
	result := metrics{"abstained": boolean(answer.Abstained)}
	if stratum != "positive" {
		result["selective_success"] = boolean(answer.Abstained)
		return result
	}
	goldSet := set(gold)
	hitsAt := func(k int) int {
		hits := 0
		for index, path := range answer.Ranked {
			if index < k && goldSet[path] {
				hits++
			}
		}
		return hits
	}
	for _, k := range []int{5, 10, 20} {
		if k <= limit {
			result[fmt.Sprintf("recall@%d", k)] = ratio(hitsAt(k), len(gold))
		}
	}
	hits := hitsAt(limit)
	precision := ratio(hits, min(limit, len(answer.Ranked)))
	recall := ratio(hits, len(gold))
	result["mrr@k"] = reciprocalRank(answer.Ranked, goldSet)
	result["precision@k"] = precision
	result["f1@k"] = fScore(precision, recall)
	result["hit@k"] = boolean(hits > 0)
	result["hard_negative_hits@k"] = float64(len(intersection(answer.Ranked[:min(limit, len(answer.Ranked))], negatives)))
	result["selective_success"] = boolean(!answer.Abstained && hits > 0)
	return result
}

func reciprocalRank(ranked []string, gold map[string]bool) float64 {
	for index, path := range ranked {
		if gold[path] {
			return 1 / float64(index+1)
		}
	}
	return 0
}

func fScore(precision, recall float64) float64 {
	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

func ratio(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}

func boolean(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func set(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func intersection(left, right []string) []string {
	wanted := set(right)
	result := make([]string, 0)
	for _, value := range left {
		if wanted[value] {
			result = append(result, value)
		}
	}
	return result
}

// summarize averages each arm's metrics over every sample, per task type,
// and per stratum, and attaches Wilson 95% intervals to the two rates the bet
// is judged on: hit@k over positives and selective success over everything.
func summarize(reports []sampleReport, limit int) map[string]any {
	arms := map[string]any{}
	for _, name := range presentArms(reports) {
		groups := map[string][]metrics{}
		for _, report := range reports {
			sampleMetrics, ran := report.Metrics[name]
			if !ran {
				continue
			}
			groups["all"] = append(groups["all"], sampleMetrics)
			groups["task:"+report.TaskType] = append(groups["task:"+report.TaskType], sampleMetrics)
			groups["stratum:"+report.Stratum] = append(groups["stratum:"+report.Stratum], sampleMetrics)
			groups["fold:"+report.Partition] = append(groups["fold:"+report.Partition], sampleMetrics)
		}
		summary := map[string]any{}
		for group, members := range groups {
			summary[group] = mean(members)
		}
		summary["intervals"] = map[string]any{
			fmt.Sprintf("hit@%d", limit): wilson(groups["stratum:positive"], "hit@k"),
			"selective_success":          wilson(groups["all"], "selective_success"),
		}
		summary["errors"] = errorCount(reports, name)
		arms[name] = summary
	}
	return arms
}

func mean(members []metrics) map[string]any {
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, member := range members {
		for key, value := range member {
			sums[key] += value
			counts[key]++
		}
	}
	result := map[string]any{"n": len(members), "positives": counts["hit@k"]}
	for key, total := range sums {
		result[key] = round(total / float64(counts[key]))
	}
	return result
}

// wilson is the 95% Wilson score interval of a rate over n samples.
func wilson(members []metrics, key string) map[string]any {
	n, successes := 0, 0.0
	for _, member := range members {
		value, present := member[key]
		if !present {
			continue
		}
		n++
		successes += value
	}
	if n == 0 {
		return map[string]any{"n": 0}
	}
	const z = 1.959964
	p := successes / float64(n)
	denominator := 1 + z*z/float64(n)
	centre := (p + z*z/(2*float64(n))) / denominator
	margin := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n))) / denominator
	return map[string]any{"n": n, "rate": round(p), "low": round(centre - margin), "high": round(centre + margin)}
}

func errorCount(reports []sampleReport, name string) int {
	count := 0
	for _, report := range reports {
		if report.Arms[name].Error != "" {
			count++
		}
	}
	return count
}

func round(value float64) float64 { return math.Round(value*1e6) / 1e6 }

// corvintIdentity binds the report to the binary it measured.
func corvintIdentity(ctx context.Context, corvintGo string) (map[string]any, error) {
	resolved, err := exec.LookPath(corvintGo)
	if err != nil {
		return nil, fmt.Errorf("corvint executable: %w", err)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	version, err := runCorvintGo(ctx, resolved, "--version")
	if err != nil {
		return nil, fmt.Errorf("corvint --version: %w", err)
	}
	return map[string]any{"version": strings.TrimSpace(string(version)), "sha256": hex.EncodeToString(digest[:])}, nil
}
