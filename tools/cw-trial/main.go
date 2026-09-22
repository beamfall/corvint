// Command cw-trial is the external agent-harness dispatcher for the
// confidently-wrong trial in docs/specs/confidently-wrong-trial-v0.md. It
// runs one agent over one frozen task set under three context arms (`none`,
// `grep`, `corvint`), each inside a temporary copy of the task's repository
// snapshot, extracts the claims block every reply must end with, and scores
// task success, confidently-wrong claims, and abstention per arm with Wilson
// intervals. It downloads nothing and writes only the report and the
// temporary copies it removes.
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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	profile           = "corvint-cw-trial/0"
	notObserved       = "NOT_OBSERVED"
	notApplicable     = "NOT_APPLICABLE"
	defaultLimit      = 20
	maxLimit          = 50 // corvint query and impact refuse a larger limit
	maxTasksFile      = 16 << 20
	maxChunkLineBytes = 64 << 20
	maxGrepFileBytes  = 1 << 20
	binaryProbeBytes  = 8 << 10
	maxContextBytes   = 64 << 10 // the same context budget in every arm
	maxReplyBytes     = 1 << 20
	maxOutputBytes    = 8 << 20
	maxErrorBytes     = 64 << 10
	defaultTimeout    = 5 * time.Minute
	corvintTimeout    = 2 * time.Minute
	truncatedMarker   = "\n[TRUNCATED]"
)

var armNames = []string{"none", "grep", "corvint", "aider"}

// skeleton is the one prompt every arm receives under --access read-only;
// only the arm name, the arm's prologue, and the context body differ. Its
// digest is recorded.
const skeleton = `You are answering one task about the repository in the current working directory, which is checked out at exactly the revision the task refers to. You may read files; you cannot modify anything.

TASK
%s

CONTEXT (arm: %s)
%s

%s

ANSWER FORMAT
Reply in prose if you wish, then end your reply with exactly one fenced JSON block of this shape:
` + "```json" + `
{"claims":[{"kind":"test-file|source-file|answer","value":"...","confidence":"certain|likely|unsure","evidence":"..."}]}
` + "```" + `
- kind: test-file for the repository-relative path of a test file, source-file for the path of a source file, answer for a free-text answer.
- value: the exact repository-relative path, or the answer text. A path the task presents as its own subject (the changed or commented file) is the subject of the question, not one of its answers: do not claim it.
- confidence: certain only when you verified the claim against the repository or the supplied context; likely when you believe it but did not verify; unsure when guessing.
- evidence: the packet row id or grep hit path from the CONTEXT section, or the path of a file you opened, that supports the claim; none otherwise.
A claim marked certain that turns out false counts against you; an omitted claim does not. If you cannot answer, emit {"claims":[]}.`

// skeletonNoAccess is the prompt under --access none: the repository is not
// on disk, so a claim can rest only on the task text and the supplied
// context, and running any command is a protocol violation the report
// records. Its digest is recorded in place of the read-only skeleton's.
const skeletonNoAccess = `You are answering one task about a repository that is NOT available to you: the working directory is empty, and you must not run any command or open any file. Everything you know about the repository is the task text and the CONTEXT section below.

TASK
%s

CONTEXT (arm: %s)
%s

%s

ANSWER FORMAT
Reply in prose if you wish, then end your reply with exactly one fenced JSON block of this shape:
` + "```json" + `
{"claims":[{"kind":"test-file|source-file|answer","value":"...","confidence":"certain|likely|unsure","evidence":"..."}]}
` + "```" + `
- kind: test-file for the repository-relative path of a test file, source-file for the path of a source file, answer for a free-text answer.
- value: the exact repository-relative path, or the answer text. A path the task presents as its own subject (the changed or commented file) is the subject of the question, not one of its answers: do not claim it.
- confidence: certain only when the task text or the supplied context establishes the claim; likely when you believe it but cannot establish it; unsure when guessing.
- evidence: the packet row id or grep hit path from the CONTEXT section that supports the claim; none otherwise.
A claim marked certain that turns out false counts against you; an omitted claim does not. If you cannot answer, emit {"claims":[]}.`

// skeletons keys the prompt skeleton by the --access mode.
var skeletons = map[string]string{"read-only": skeleton, "none": skeletonNoAccess}

// prologues are the per-arm fixed texts; each digest is recorded.
var prologues = map[string]string{
	"none":    "No retrieved context is supplied. Anything you claim must come from the task text or from files you open yourself.",
	"grep":    "The lines below are the top files a term-overlap grep over the repository returns for the task text: `score` is the number of distinct task terms the file or its path contains (a path match counts double). They are ranked hits, not verified evidence. Cite a hit as evidence by its path.",
	"aider":   "The lines below are the files aider's repository map ranks highest for this task: a personalised PageRank over identifier definitions and references across the repository, seeded with the task's subject path and the code-shaped identifiers in the task text, rank 1 first. They are ranked hits, not verified evidence. Cite a hit as evidence by its path.",
	"corvint": "The JSON below is a Corvint task-context packet (`corvint context --task TEXT [--subject PATH] --limit K`). Each result row is a file to read: `id` is its path, `kind` names the relation that admitted it (pair, mentioned, definition, reverse-import, cochange, sibling, lexical), its evidence row's `reason` states that relation, and `action` says what to do with the file for this task: act on it; `lexical` rows are term matches, not verified evidence. `subject`, when present, is the task's own path and is deliberately not a result. A row's evidence `confidence` bounds what it can back: a `high` row can back `certain`; a `medium` or `low` row backs at most `likely`. `state` NO_CANDIDATES means Corvint found nothing to read; claim at most `unsure` then. Cite a row as evidence by its `id`.",
}

var (
	claimKinds       = map[string]bool{"test-file": true, "source-file": true, "answer": true}
	claimConfidences = map[string]bool{"certain": true, "likely": true, "unsure": true}
	fencePattern     = regexp.MustCompile("(?s)```[a-zA-Z]*[ \t]*\r?\n(.*?)```")
	tokenPattern     = regexp.MustCompile(`[A-Za-z0-9]+`)
	camelPattern     = regexp.MustCompile(`([a-z])([A-Z])`)
)

// task is one frozen row of the manifest. Gold maps a claim kind to the
// values the task's author marked true; a kind absent from gold is unjudged.
type task struct {
	ID          string              `json:"id"`
	Kind        string              `json:"kind"`
	Repo        string              `json:"repo"`
	BaseCommit  string              `json:"base_commit"`
	ChangedFile string              `json:"changed_file,omitempty"`
	Snapshot    string              `json:"snapshot,omitempty"`
	Text        string              `json:"text"`
	Gold        map[string][]string `json:"gold"`
	Source      any                 `json:"source,omitempty"`
}

type manifest struct {
	Partition string `json:"partition"`
	Source    string `json:"source"`
	Tasks     []task `json:"tasks"`
}

// claim is one entry of the claims block, as the agent wrote it.
type claim struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Confidence string `json:"confidence"`
	Evidence   string `json:"evidence"`
}

// judgedClaim is a claim with its verdict against gold: TRUE, FALSE,
// UNJUDGED (no gold for its kind), or INVALID (outside the grammar).
type judgedClaim struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Confidence string `json:"confidence"`
	Evidence   string `json:"evidence"`
	Verdict    string `json:"verdict"`
	Grounded   any    `json:"grounded"`
}

// armRecord is one agent invocation: what it was given, what it replied,
// and how the reply scored. Reply, Context, and the observation fields are
// raw; Claims and Metrics are derived by finalize and rebuilt by `score`.
type armRecord struct {
	PromptSHA256     string             `json:"prompt_sha256"`
	PromptBytes      int                `json:"prompt_bytes"`
	Context          string             `json:"context"`
	ContextBytes     int                `json:"context_bytes"`
	ContextTruncated bool               `json:"context_truncated"`
	ContextState     string             `json:"context_state,omitempty"`
	ContextError     string             `json:"context_error,omitempty"`
	WallMs           any                `json:"wall_ms"`
	ExitCode         any                `json:"exit_code"`
	Tokens           any                `json:"tokens"`
	ToolCalls        any                `json:"tool_calls"`
	Commands         []string           `json:"commands,omitempty"`
	Reply            string             `json:"reply"`
	ReplyTruncated   bool               `json:"reply_truncated"`
	Error            string             `json:"error,omitempty"`
	ReusedFrom       string             `json:"reused_from,omitempty"`
	BlockState       string             `json:"block_state"`
	Claims           []judgedClaim      `json:"claims"`
	Metrics          map[string]float64 `json:"metrics"`
}

type taskRecord struct {
	ID         string              `json:"id"`
	Kind       string              `json:"kind"`
	Repo       string              `json:"repo"`
	BaseCommit string              `json:"base_commit"`
	Gold       map[string][]string `json:"gold"`
	// HistoryCommits is the number of commits behind the copy's HEAD when the
	// snapshot was materialised from --history; absent for a chunk rebuild.
	HistoryCommits int `json:"history_commits,omitempty"`
	// Subject is the task's changed file, the anchor the retrieval-native
	// metrics (CWT-V0-013) exclude when they ask for gold beyond its stem.
	Subject string `json:"subject,omitempty"`
	// GoldAtBase lists the gold paths present in the materialised copy, and
	// GoldChecked says the copy was asked; a report written before the field
	// existed has neither, and `score --history` can fill them in.
	GoldAtBase  []string              `json:"gold_at_base,omitempty"`
	GoldChecked bool                  `json:"gold_checked,omitempty"`
	Arms        map[string]*armRecord `json:"arms"`
}

// report is the whole document; `score` reads one back and rebuilds every
// derived field from the raw ones.
type report struct {
	Profile        string                    `json:"profile"`
	Partial        bool                      `json:"partial,omitempty"`
	Pilot          bool                      `json:"pilot"`
	Partition      string                    `json:"partition"`
	Source         string                    `json:"source"`
	TasksSHA256    string                    `json:"tasks_sha256"`
	Tasks          int                       `json:"tasks"`
	Limit          int                       `json:"limit"`
	Model          string                    `json:"model"`
	Access         string                    `json:"access"`
	Agent          map[string]any            `json:"agent"`
	Corvint        any                       `json:"corvint"`
	Aider          any                       `json:"aider"`
	SkeletonSHA256 string                    `json:"skeleton_sha256"`
	Prologues      map[string]map[string]any `json:"prologues"`
	Arms           map[string]any            `json:"arms"`
	Details        []taskRecord              `json:"details"`
}

type options struct {
	tasks        string
	corpus       string
	corvintGo    string
	arms         []string
	agent        string
	agentCommand string
	model        string
	effort       string
	timeout      time.Duration
	limit        int
	maxTasks     int
	output       string
	access       string
	workers      int
	reuse        string
	resume       bool
	history      string
	aiderCommand string
}

// agentResult is one reply plus what the agent CLI reported about itself.
type agentResult struct {
	reply     string
	exitCode  any
	tokens    any
	toolCalls any
	commands  []string
}

type agent interface {
	run(ctx context.Context, root, prompt string, timeout time.Duration) (agentResult, error)
	identity(ctx context.Context) (map[string]any, error)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		fmt.Fprintln(stderr, "cw-trial: usage: cw-trial run|score|import-heldout [flags]")
		return 2
	}
	var document *report
	var output string
	var err error
	switch arguments[0] {
	case "run":
		document, output, err = runTrial(ctx, arguments[1:])
		if err == nil && output != "" {
			defer os.Remove(checkpointPath(output))
		}
	case "score":
		document, output, err = rescore(arguments[1:])
	case "import-heldout":
		return importHeldout(arguments[1:], stdout, stderr)
	case "generate":
		return generate(arguments[1:], stdout, stderr)
	default:
		err = fmt.Errorf("unknown command %q", arguments[0])
	}
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial:", err)
		return 2
	}
	return write(document, output, stdout, stderr)
}

func write(document *report, output string, stdout, stderr io.Writer) int {
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial: cannot encode the report")
		return 2
	}
	encoded = append(encoded, '\n')
	if output == "" {
		_, err = stdout.Write(encoded)
	} else {
		err = os.WriteFile(output, encoded, 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial: cannot write the report")
		return 2
	}
	return 0
}

func parseRunOptions(arguments []string) (options, error) {
	var configuration options
	var arms string
	flags := flag.NewFlagSet("cw-trial run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&configuration.tasks, "tasks", "", "task manifest JSON")
	flags.StringVar(&configuration.corpus, "corpus", "", "bench corpus directory holding OWNER__NAME/COMMIT.chunks.jsonl")
	flags.StringVar(&configuration.corvintGo, "corvint", "", "corvint executable (required by the corvint arm)")
	flags.StringVar(&arms, "arms", "none,grep,corvint", "comma-separated arms to run")
	flags.StringVar(&configuration.agent, "agent", "codex", "agent kind: codex or script")
	flags.StringVar(&configuration.agentCommand, "agent-command", "", "script agent: executable that receives the prompt as its argument and replies on stdout")
	flags.StringVar(&configuration.model, "model", "", "model id passed to codex -m (required for the codex agent)")
	flags.StringVar(&configuration.effort, "effort", "", "codex model_reasoning_effort override")
	flags.DurationVar(&configuration.timeout, "timeout", defaultTimeout, "per-invocation wall-clock limit")
	flags.IntVar(&configuration.limit, "limit", defaultLimit, "grep hits and corvint results per task")
	flags.IntVar(&configuration.maxTasks, "max-tasks", 0, "run at most this many tasks (0 = all)")
	flags.StringVar(&configuration.output, "output", "", "report path (default stdout)")
	flags.IntVar(&configuration.workers, "workers", 1, "concurrent invocations under --access none (1 = sequential)")
	flags.BoolVar(&configuration.resume, "resume", false, "reuse the replies checkpointed under OUTPUT.partial.json by an earlier, interrupted run of this command")
	flags.StringVar(&configuration.aiderCommand, "aider-command", "", "aider arm: caller-provided command prefix that prints rank=N PATH lines given --root DIR --task-file FILE [--subject PATH] --limit K")
	flags.StringVar(&configuration.history, "history", "", "local Git repository whose history holds the tasks' base commits: a task found there is materialised as a shared clone at its base commit, with history, instead of a one-commit rebuild")
	flags.StringVar(&configuration.reuse, "reuse", "", "prior report whose replies are reused for any arm with a byte-identical prompt (--access none only)")
	flags.StringVar(&configuration.access, "access", "read-only", "repository access during dispatch: read-only (the agent runs inside the copy) or none (contexts are produced first, the copy is removed, and the agent runs in an empty directory)")
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if configuration.tasks == "" {
		return options{}, errors.New("--tasks is required")
	}
	if skeletons[configuration.access] == "" {
		return options{}, fmt.Errorf("--access must be read-only or none, not %q", configuration.access)
	}
	if configuration.limit < 1 || configuration.limit > maxLimit {
		return options{}, fmt.Errorf("--limit must be between 1 and %d", maxLimit)
	}
	for _, name := range strings.Split(arms, ",") {
		name = strings.TrimSpace(name)
		if prologues[name] == "" {
			return options{}, fmt.Errorf("unknown arm %q", name)
		}
		configuration.arms = append(configuration.arms, name)
	}
	if len(configuration.arms) == 0 {
		return options{}, errors.New("--arms names no arm")
	}
	if configuration.workers < 1 {
		return options{}, errors.New("--workers must be at least 1")
	}
	if configuration.access != "none" && (configuration.workers > 1 || configuration.reuse != "" || configuration.resume) {
		return options{}, errors.New("--workers above 1, --reuse, and --resume need --access none")
	}
	if configuration.resume && (configuration.output == "" || configuration.reuse != "") {
		return options{}, errors.New("--resume needs --output and excludes --reuse")
	}
	if configuration.resume {
		configuration.reuse = checkpointPath(configuration.output)
		if _, err := os.Stat(configuration.reuse); err != nil {
			return options{}, fmt.Errorf("--resume: no checkpoint to resume: %w", err)
		}
	}
	return configuration, validateAgentOptions(configuration)
}

func validateAgentOptions(configuration options) error {
	if configuration.aiderCommand == "" && contains(configuration.arms, "aider") {
		return errors.New("--aider-command is required by the aider arm")
	}
	if configuration.corvintGo == "" && contains(configuration.arms, "corvint") {
		return errors.New("--corvint is required by the corvint arm")
	}
	switch configuration.agent {
	case "codex":
		if configuration.model == "" {
			return errors.New("--model is required for the codex agent")
		}
	case "script":
		if configuration.agentCommand == "" {
			return errors.New("--agent-command is required for the script agent")
		}
	default:
		return fmt.Errorf("unknown agent %q", configuration.agent)
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func newAgent(configuration options) agent {
	if configuration.agent == "script" {
		return scriptAgent{command: configuration.agentCommand}
	}
	return codexAgent{model: configuration.model, effort: configuration.effort}
}

func runTrial(ctx context.Context, arguments []string) (*report, string, error) {
	configuration, err := parseRunOptions(arguments)
	if err != nil {
		return nil, "", err
	}
	document, err := trial(ctx, configuration, newAgent(configuration))
	if err != nil {
		return nil, "", err
	}
	return document, configuration.output, nil
}

func trial(ctx context.Context, configuration options, runner agent) (*report, error) {
	if configuration.access == "" {
		configuration.access = "read-only"
	}
	tasks, digest, err := readManifest(configuration)
	if err != nil {
		return nil, err
	}
	agentIdentity, err := runner.identity(ctx)
	if err != nil {
		return nil, err
	}
	var corvint any = notObserved
	if contains(configuration.arms, "corvint") {
		if corvint, err = corvintIdentity(ctx, configuration.corvintGo); err != nil {
			return nil, err
		}
	}
	var aider any = notObserved
	if contains(configuration.arms, "aider") {
		aider = map[string]any{"command": configuration.aiderCommand}
	}
	prior, err := loadReuse(configuration)
	if err != nil {
		return nil, err
	}
	document := &report{
		Profile:        profile,
		Pilot:          tasks.Partition == "pilot",
		Partition:      tasks.Partition,
		Source:         tasks.Source,
		TasksSHA256:    digest,
		Tasks:          len(tasks.Tasks),
		Limit:          configuration.limit,
		Model:          configuration.model,
		Access:         configuration.access,
		Agent:          agentIdentity,
		Corvint:        corvint,
		Aider:          aider,
		SkeletonSHA256: digestOf(skeletons[configuration.access]),
		Prologues:      prologueIdentity(configuration.arms),
		Arms:           map[string]any{},
		Details:        make([]taskRecord, 0, len(tasks.Tasks)),
	}
	saver := newCheckpointer(document, configuration.output)
	workspaces := newWorkspaces()
	defer workspaces.close()
	pending := make([]invocation, 0, len(tasks.Tasks)*len(configuration.arms))
	for _, item := range tasks.Tasks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		root, err := workspaces.open(ctx, configuration, item)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.ID, err)
		}
		record, waiting := dispatchTask(ctx, configuration, runner, item, root, workspaces, prior, saver)
		record.HistoryCommits = workspaces.commits[root]
		document.Details = append(document.Details, record)
		pending = append(pending, waiting...)
	}
	invokePending(ctx, configuration, runner, pending, saver)
	finalize(document)
	return document, nil
}

// checkpointPath is where a run with --output keeps its partial report.
func checkpointPath(output string) string { return output + ".partial.json" }

// checkpointer writes the report so far after every finished invocation,
// marked partial and unscored, so an interrupted run keeps what it paid for
// and --resume reuses it. Its guard also serialises record writes against
// the snapshot, so concurrent workers never race the encoder.
type checkpointer struct {
	guard    sync.Mutex
	path     string
	document *report
}

func newCheckpointer(document *report, output string) *checkpointer {
	if output == "" {
		return nil
	}
	return &checkpointer{path: checkpointPath(output), document: document}
}

func (saver *checkpointer) lock() *sync.Mutex {
	if saver == nil {
		return nil
	}
	return &saver.guard
}

func (saver *checkpointer) save() {
	if saver == nil {
		return
	}
	saver.guard.Lock()
	defer saver.guard.Unlock()
	snapshot := *saver.document
	snapshot.Partial = true
	encoded, err := gokernel.CanonicalJSON(&snapshot)
	if err != nil {
		return
	}
	temporary := saver.path + ".tmp"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0o644); err != nil {
		return
	}
	_ = os.Rename(temporary, saver.path)
}

// assign runs one record write under the checkpoint guard when there is one.
func assign(guard *sync.Mutex, write func()) {
	if guard != nil {
		guard.Lock()
		defer guard.Unlock()
	}
	write()
}

func prologueIdentity(arms []string) map[string]map[string]any {
	result := map[string]map[string]any{}
	for _, name := range arms {
		result[name] = map[string]any{"sha256": digestOf(prologues[name]), "text": prologues[name]}
	}
	return result
}

func digestOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func readManifest(configuration options) (manifest, string, error) {
	data, err := readBounded(configuration.tasks, maxTasksFile)
	if err != nil {
		return manifest{}, "", err
	}
	var tasks manifest
	if err := json.Unmarshal(data, &tasks); err != nil {
		return manifest{}, "", fmt.Errorf("%s: %w", configuration.tasks, err)
	}
	if tasks.Partition == "" {
		return manifest{}, "", fmt.Errorf("%s: partition is required", configuration.tasks)
	}
	seen := map[string]bool{}
	for _, item := range tasks.Tasks {
		if err := validateTask(item); err != nil {
			return manifest{}, "", fmt.Errorf("%s: task %q: %w", configuration.tasks, item.ID, err)
		}
		if seen[item.ID] {
			return manifest{}, "", fmt.Errorf("%s: duplicate task id %q", configuration.tasks, item.ID)
		}
		seen[item.ID] = true
	}
	if configuration.maxTasks > 0 && len(tasks.Tasks) > configuration.maxTasks {
		tasks.Tasks = tasks.Tasks[:configuration.maxTasks]
	}
	sum := sha256.Sum256(data)
	return tasks, hex.EncodeToString(sum[:]), nil
}

func validateTask(item task) error {
	if item.ID == "" || item.Text == "" {
		return errors.New("id and text are required")
	}
	if item.Kind != "retrieval" && item.Kind != "change" {
		return fmt.Errorf("kind must be retrieval or change, not %q", item.Kind)
	}
	if item.Kind == "change" && item.ChangedFile == "" {
		return errors.New("a change task needs changed_file")
	}
	for kind := range item.Gold {
		if !claimKinds[kind] {
			return fmt.Errorf("gold kind %q is not a claim kind", kind)
		}
	}
	return nil
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

// workspaces materialises each snapshot once, in a temporary directory that
// is committed so every arm sees one immutable tree; the source snapshot is
// never used in place and never written.
type workspaces struct {
	roots     map[string]string
	commits   map[string]int
	temporary []string
}

func newWorkspaces() *workspaces {
	return &workspaces{roots: map[string]string{}, commits: map[string]int{}}
}

func (space *workspaces) close() {
	for _, directory := range space.temporary {
		_ = os.RemoveAll(directory)
	}
}

// discard removes one materialised copy now, so a later task on the same
// snapshot materialises afresh; used under --access none before dispatch.
func (space *workspaces) discard(root string) {
	_ = os.RemoveAll(root)
	for key, known := range space.roots {
		if known == root {
			delete(space.roots, key)
		}
	}
}

func (space *workspaces) open(ctx context.Context, configuration options, item task) (string, error) {
	key := item.Repo + "@" + item.BaseCommit + "@" + item.Snapshot
	if root, ready := space.roots[key]; ready {
		return root, nil
	}
	if configuration.history != "" && item.BaseCommit != "" && commitKnown(ctx, configuration.history, item.BaseCommit) {
		root, commits, err := materializeHistory(ctx, configuration.history, item.BaseCommit)
		if err != nil {
			return "", err
		}
		space.temporary = append(space.temporary, root)
		space.roots[key], space.commits[root] = root, commits
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
	writer := writeChunkRows
	if info.IsDir() {
		writer = copyTree
	}
	root, err := materialize(ctx, snapshot, writer)
	if err != nil {
		return "", err
	}
	space.temporary = append(space.temporary, root)
	space.roots[key] = root
	return root, nil
}

// resolveSnapshot prefers the task's explicit snapshot path, else the
// corpus chunk file `OWNER__NAME/COMMIT.chunks.jsonl`.
func resolveSnapshot(configuration options, item task) (string, error) {
	if item.Snapshot != "" {
		return item.Snapshot, nil
	}
	if configuration.corpus == "" {
		return "", errors.New("no snapshot: the task names none and --corpus is unset")
	}
	if item.Repo == "" || item.BaseCommit == "" {
		return "", errors.New("no snapshot: repo and base_commit are required to find one in --corpus")
	}
	directory := strings.ReplaceAll(item.Repo, "/", "__")
	return filepath.Join(configuration.corpus, directory, item.BaseCommit+".chunks.jsonl"), nil
}

// commitKnown reports whether the history repository holds the commit.
func commitKnown(ctx context.Context, history, commit string) bool {
	_, err := git(ctx, history, "cat-file", "-e", commit+"^{commit}")
	return err == nil
}

// materializeHistory clones the history repository into a temporary
// directory sharing its objects (read, never written) and checks the base
// commit out detached, so the copy carries every commit behind it; it returns
// the copy and that commit count. The source's .git gains no worktree entry
// and no object.
func materializeHistory(ctx context.Context, history, commit string) (string, int, error) {
	root, err := os.MkdirTemp("", "corvint-cw-trial-")
	if err != nil {
		return "", 0, err
	}
	fail := func(step string, err error) (string, int, error) {
		_ = os.RemoveAll(root)
		return "", 0, fmt.Errorf("git %s: %w", step, err)
	}
	if _, err := git(ctx, root, "clone", "-q", "--shared", "--no-checkout", history, root); err != nil {
		return fail("clone", err)
	}
	if _, err := git(ctx, root, "checkout", "-q", "--detach", commit); err != nil {
		return fail("checkout", err)
	}
	head, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(head) != commit {
		return fail("rev-parse", fmt.Errorf("HEAD %q is not %s", strings.TrimSpace(head), commit))
	}
	count, err := git(ctx, root, "rev-list", "--count", "HEAD")
	if err != nil {
		return fail("rev-list", err)
	}
	commits, err := strconv.Atoi(strings.TrimSpace(count))
	if err != nil {
		return fail("rev-list", err)
	}
	return root, commits, nil
}

func materialize(ctx context.Context, snapshot string, write func(source, destination string) error) (string, error) {
	root, err := os.MkdirTemp("", "corvint-cw-trial-")
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

// writeChunkRows rebuilds a bench snapshot from its chunk file: every
// `kind: file` row carries one file's full text, as tools/retrieval-bench
// reads it. Other rows and unsafe paths are skipped.
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
		"GIT_AUTHOR_NAME=cw-trial", "GIT_AUTHOR_EMAIL=trial@localhost",
		"GIT_COMMITTER_NAME=cw-trial", "GIT_COMMITTER_EMAIL=trial@localhost",
	)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// invocation is one prepared arm still to be run from an empty directory.
type invocation struct {
	record *armRecord
	prompt string
}

// dispatchTask prepares every arm of one task. Under read-only access each
// arm is prepared and invoked in turn inside the copy and nothing is
// returned to run later. Under no access every arm's context and prompt are
// prepared first, the copy is removed, a reply from the prior report is
// reused when its prompt is byte-identical, and the rest are returned for
// invokePending to run from empty directories, so no invocation can reach
// the repository.
func dispatchTask(ctx context.Context, configuration options, runner agent, item task, root string, space *workspaces, prior *reuseSource, saver *checkpointer) (taskRecord, []invocation) {
	record := taskRecord{ID: item.ID, Kind: item.Kind, Repo: item.Repo, BaseCommit: item.BaseCommit, Gold: item.Gold, Subject: item.ChangedFile, Arms: map[string]*armRecord{}}
	record.GoldAtBase, record.GoldChecked = goldPresent(item.Gold, func(value string) bool {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(value)))
		return err == nil && info.Mode().IsRegular()
	}), true
	prompts := map[string]string{}
	for _, name := range configuration.arms {
		record.Arms[name], prompts[name] = prepareArm(ctx, configuration, item, root, name)
		if configuration.access == "read-only" {
			invokeArm(ctx, configuration, runner, root, prompts[name], record.Arms[name], nil)
		}
	}
	if configuration.access == "read-only" {
		return record, nil
	}
	space.discard(root)
	pending := make([]invocation, 0, len(configuration.arms))
	for _, name := range configuration.arms {
		if prior.apply(item.ID, name, record.Arms[name]) {
			continue
		}
		pending = append(pending, invocation{record: record.Arms[name], prompt: prompts[name]})
	}
	return record, pending
}

// invokePending runs the prepared invocations, at most --workers at a time,
// each from its own empty directory. Records are written only by their own
// goroutine, and the report order is the manifest order, so the worker
// count changes timings and nothing else.
func invokePending(ctx context.Context, configuration options, runner agent, pending []invocation, saver *checkpointer) {
	saver.save()
	slots := make(chan struct{}, maxInt(configuration.workers, 1))
	var group sync.WaitGroup
	for _, item := range pending {
		slots <- struct{}{}
		group.Add(1)
		go func(item invocation) {
			defer group.Done()
			defer func() { <-slots }()
			invokeEmpty(ctx, configuration, runner, item, saver)
			saver.save()
		}(item)
	}
	group.Wait()
}

func invokeEmpty(ctx context.Context, configuration options, runner agent, item invocation, saver *checkpointer) {
	empty, err := os.MkdirTemp("", "corvint-cw-trial-empty-")
	if err != nil {
		assign(saver.lock(), func() { item.record.Error = err.Error() })
		return
	}
	defer os.RemoveAll(empty)
	invokeArm(ctx, configuration, runner, empty, item.prompt, item.record, saver.lock())
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

// reuseSource is a prior report's invocations keyed by task and arm; a reply
// is reused only for a byte-identical prompt under the same model and access,
// and the record says which report it came from.
type reuseSource struct {
	identity string
	records  map[string]*armRecord
}

func loadReuse(configuration options) (*reuseSource, error) {
	if configuration.reuse == "" {
		return nil, nil
	}
	data, err := readBounded(configuration.reuse, maxOutputBytes)
	if err != nil {
		return nil, err
	}
	var document report
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%s: %w", configuration.reuse, err)
	}
	if document.Profile != profile || document.Model != configuration.model || document.Access != configuration.access {
		return nil, fmt.Errorf("%s: a reused report must share the profile, model, and access of this run", configuration.reuse)
	}
	source := &reuseSource{identity: "sha256:" + digestOf(string(data)), records: map[string]*armRecord{}}
	for _, item := range document.Details {
		for name, record := range item.Arms {
			source.records[item.ID+"\x00"+name] = record
		}
	}
	return source, nil
}

// observedNumber tells a finished invocation (wall time recorded) from one a
// checkpoint captured before it ran.
func observedNumber(value any) bool {
	switch value.(type) {
	case int64, float64, int:
		return true
	}
	return false
}

func (source *reuseSource) apply(id, arm string, record *armRecord) bool {
	if source == nil {
		return false
	}
	prior := source.records[id+"\x00"+arm]
	if prior == nil || prior.Error != "" || prior.PromptSHA256 != record.PromptSHA256 || !observedNumber(prior.WallMs) {
		return false
	}
	if _, failed := failedInvocation(prior); failed {
		return false
	}
	record.WallMs, record.ExitCode, record.Tokens, record.ToolCalls, record.Commands = prior.WallMs, prior.ExitCode, prior.Tokens, prior.ToolCalls, prior.Commands
	record.Reply, record.ReplyTruncated, record.ReusedFrom = prior.Reply, prior.ReplyTruncated, source.identity
	return true
}

// prepareArm produces the arm's context and prompt inside the copy and
// returns the record with every observation field still unobserved.
func prepareArm(ctx context.Context, configuration options, item task, root, name string) (*armRecord, string) {
	record := &armRecord{WallMs: notObserved, ExitCode: notObserved, Tokens: notObserved, ToolCalls: notObserved}
	body, err := contextBody(ctx, configuration, item, root, name, record)
	if err != nil {
		record.ContextError = err.Error()
		body = "(the context producer failed: " + err.Error() + ")"
	}
	record.Context, record.ContextTruncated = truncate(body, maxContextBytes)
	record.ContextBytes = len(record.Context)
	prompt := buildPrompt(configuration.access, name, item.Text, record.Context)
	record.PromptSHA256 = digestOf(prompt)
	record.PromptBytes = len(prompt)
	return record, prompt
}

// invokeArm runs the agent once from root with the prepared prompt and
// records what it observed.
func invokeArm(ctx context.Context, configuration options, runner agent, root, prompt string, record *armRecord, guard *sync.Mutex) {
	if err := ctx.Err(); err != nil {
		assign(guard, func() { record.Error = err.Error() })
		return
	}
	started := time.Now()
	result, err := runner.run(ctx, root, prompt, configuration.timeout)
	wall := time.Since(started).Milliseconds()
	assign(guard, func() {
		record.WallMs = wall
		if err != nil {
			record.Error = err.Error()
			return
		}
		record.ExitCode, record.Tokens, record.ToolCalls, record.Commands = result.exitCode, result.tokens, result.toolCalls, result.commands
		record.Reply, record.ReplyTruncated = truncate(result.reply, maxReplyBytes)
	})
}

func buildPrompt(access, arm, text, body string) string {
	if body == "" {
		body = "(none)"
	}
	return fmt.Sprintf(skeletons[access], text, arm, prologues[arm], body)
}

func truncate(text string, limit int) (string, bool) {
	if len(text) <= limit {
		return text, false
	}
	return text[:limit] + truncatedMarker, true
}

// contextBody produces what the arm adds to the task text: nothing, the
// grep ranking, or the Corvint packet verbatim.
func contextBody(ctx context.Context, configuration options, item task, root, arm string, record *armRecord) (string, error) {
	switch arm {
	case "grep":
		return grepBody(root, item.Text, configuration.limit), nil
	case "aider":
		return aiderBody(ctx, configuration, item, root)
	case "corvint":
		packet, err := corvintPacket(ctx, configuration, item, root)
		if err != nil {
			return "", err
		}
		record.ContextState = packetState(packet)
		return string(packet), nil
	}
	return "", nil
}

func corvintPacket(ctx context.Context, configuration options, item task, root string) ([]byte, error) {
	arguments := []string{"--root", root, "context", "--task", item.Text, "--limit", fmt.Sprint(configuration.limit)}
	if item.Kind == "change" {
		arguments = append(arguments, "--subject", item.ChangedFile)
	}
	return runCorvintGo(ctx, configuration.corvintGo, arguments...)
}

// packetState reads the packet's state: `query` carries it at the top level,
// `impact` under `context`.
func packetState(data []byte) string {
	var envelope struct {
		State   string `json:"state"`
		Context struct {
			State string `json:"state"`
		} `json:"context"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "UNREADABLE"
	}
	if envelope.State != "" {
		return envelope.State
	}
	return envelope.Context.State
}

func runCorvintGo(ctx context.Context, corvintGo string, arguments ...string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, corvintTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, corvintGo, arguments...)
	stdout, stderr := &boundedBuffer{limit: maxOutputBytes}, &boundedBuffer{limit: maxErrorBytes}
	command.Stdout, command.Stderr = stdout, stderr
	command.WaitDelay = 5 * time.Second
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("corvint %s: %v: %s", arguments[2], err, strings.TrimSpace(stderr.String()))
	}
	if stdout.overflow {
		return nil, fmt.Errorf("corvint %s: output exceeds %d bytes", arguments[2], maxOutputBytes)
	}
	return stdout.Bytes(), nil
}

// boundedBuffer keeps the first limit bytes and remembers that more arrived.
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

// grepBody is tools/retrieval-bench's grep baseline reimplemented: every
// text file under 1 MiB scored by the distinct task terms it contains (a
// path match counts double), ties broken by occurrences then path.
// aiderBody runs the aider repository-map producer inside the copy with the
// same seeds the corvint arm gets (subject path, task text) and returns its
// rank listing. The producer gets twice the agent timeout: aider's first scan
// of a copy parses every file.
func aiderBody(ctx context.Context, configuration options, item task, root string) (string, error) {
	taskFile, err := os.CreateTemp("", "corvint-cw-trial-task-")
	if err != nil {
		return "", err
	}
	defer os.Remove(taskFile.Name())
	if _, err := taskFile.WriteString(item.Text); err != nil {
		return "", err
	}
	if err := taskFile.Close(); err != nil {
		return "", err
	}
	arguments := append(strings.Fields(configuration.aiderCommand), "--root", root, "--task-file", taskFile.Name(), "--limit", strconv.Itoa(configuration.limit))
	if item.Kind == "change" {
		arguments = append(arguments, "--subject", item.ChangedFile)
	}
	bounded, cancel := context.WithTimeout(ctx, 2*configuration.timeout)
	defer cancel()
	command := exec.CommandContext(bounded, arguments[0], arguments[1:]...)
	command.Dir = root
	command.Stdin = nil
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("aider map: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func grepBody(root, text string, limit int) string {
	hits := grepHits(root, text)
	if len(hits) == 0 {
		return "(no file matched any task term)"
	}
	lines := make([]string, 0, limit)
	for index := 0; index < len(hits) && index < limit; index++ {
		lines = append(lines, fmt.Sprintf("score=%d %s", hits[index].distinct, hits[index].path))
	}
	return strings.Join(lines, "\n")
}

type grepHit struct {
	path        string
	distinct    int
	occurrences int
}

func grepHits(root, text string) []grepHit {
	wanted := terms(text)
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
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		if hit, scored := scoreFile(path, filepath.ToSlash(relative), wanted); scored {
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
	return hits
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

// codexAgent runs `codex exec` read-only inside the materialised root with
// stdin closed, reads the reply from --output-last-message, and takes token
// counts from the `turn.completed` usage event when the CLI emits one.
type codexAgent struct {
	model  string
	effort string
}

func (runner codexAgent) arguments(root, replyPath, prompt string) []string {
	arguments := []string{"exec", "--json", "--ephemeral", "-s", "read-only", "--skip-git-repo-check", "-C", root, "-m", runner.model}
	if runner.effort != "" {
		arguments = append(arguments, "-c", "model_reasoning_effort="+runner.effort)
	}
	return append(arguments, "-o", replyPath, prompt)
}

func (runner codexAgent) run(ctx context.Context, root, prompt string, timeout time.Duration) (agentResult, error) {
	scratch, err := os.MkdirTemp("", "corvint-cw-trial-reply-")
	if err != nil {
		return agentResult{}, err
	}
	defer os.RemoveAll(scratch)
	replyPath := filepath.Join(scratch, "reply.txt")
	stdout, exitCode, err := runCommand(ctx, root, timeout, "codex", runner.arguments(root, replyPath, prompt)...)
	if err != nil {
		return agentResult{}, err
	}
	events := parseCodexEvents(stdout)
	reply, readErr := os.ReadFile(replyPath)
	if readErr != nil {
		reply = []byte(events.lastMessage)
	}
	return agentResult{reply: string(reply), exitCode: exitCode, tokens: events.tokens, toolCalls: events.toolCalls, commands: events.commands}, nil
}

func (runner codexAgent) identity(ctx context.Context) (map[string]any, error) {
	version, err := exec.CommandContext(ctx, "codex", "--version").Output()
	if err != nil {
		return nil, fmt.Errorf("codex --version: %w", err)
	}
	effort := any(runner.effort)
	if runner.effort == "" {
		effort = notObserved
	}
	return map[string]any{
		"kind":    "codex",
		"version": strings.TrimSpace(string(version)),
		"command": strings.Join(runner.arguments("ROOT", "REPLY", "PROMPT"), " "),
		"effort":  effort,
	}, nil
}

type codexEvents struct {
	lastMessage string
	tokens      any
	toolCalls   int
	commands    []string
}

// toolItemTypes are the codex item types that reach outside the prompt: a
// shell command, a file edit, an MCP tool, a web search. Each completed one
// is a tool call; a command's text is kept, bounded, for the audit.
var toolItemTypes = map[string]bool{"command_execution": true, "file_change": true, "mcp_tool_call": true, "web_search": true}

const (
	maxCommandBytes = 200
	maxCommands     = 50
)

func parseCodexEvents(stdout []byte) codexEvents {
	result := codexEvents{tokens: notObserved}
	for _, line := range bytes.Split(stdout, []byte{'\n'}) {
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Command string `json:"command"`
			} `json:"item"`
			Usage map[string]any `json:"usage"`
		}
		if json.Unmarshal(line, &event) != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			result.lastMessage = event.Item.Text
		}
		if event.Type == "item.completed" && toolItemTypes[event.Item.Type] {
			result.toolCalls++
			if event.Item.Command != "" && len(result.commands) < maxCommands {
				command, _ := truncate(event.Item.Command, maxCommandBytes)
				result.commands = append(result.commands, command)
			}
		}
		if event.Type == "turn.completed" && len(event.Usage) > 0 {
			result.tokens = event.Usage
		}
	}
	return result
}

// scriptAgent is any executable that takes the prompt as its only argument,
// runs in the materialised root, and replies on stdout: the fake agent that
// proves the pipeline, or an adapter for another agent CLI.
type scriptAgent struct {
	command string
}

func (runner scriptAgent) run(ctx context.Context, root, prompt string, timeout time.Duration) (agentResult, error) {
	stdout, exitCode, err := runCommand(ctx, root, timeout, runner.command, prompt)
	if err != nil {
		return agentResult{}, err
	}
	return agentResult{reply: string(stdout), exitCode: exitCode, tokens: notObserved, toolCalls: notObserved}, nil
}

func (runner scriptAgent) identity(context.Context) (map[string]any, error) {
	data, err := os.ReadFile(runner.command)
	if err != nil {
		return nil, fmt.Errorf("agent command: %w", err)
	}
	sum := sha256.Sum256(data)
	return map[string]any{"kind": "script", "command": runner.command, "sha256": hex.EncodeToString(sum[:])}, nil
}

// runCommand runs one agent invocation with stdin closed, bounded output,
// and the per-invocation timeout; a non-zero exit is returned as the exit
// code with stdout, a timeout or launch failure as an error.
func runCommand(ctx context.Context, root string, timeout time.Duration, name string, arguments ...string) ([]byte, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("%s: %v after %s", name, err, timeout)
	}
	if timeout <= 0 {
		return nil, 0, fmt.Errorf("%s: %v after %s", name, context.DeadlineExceeded, timeout)
	}
	directory, err := filepath.Abs(root)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %v", name, err)
	}
	executable := name
	if !filepath.IsAbs(executable) {
		if strings.ContainsRune(executable, filepath.Separator) {
			executable = filepath.Join(directory, executable)
		} else {
			executable, err = exec.LookPath(executable)
			if err == nil {
				executable, err = filepath.Abs(executable)
			}
			if err != nil {
				return nil, 0, fmt.Errorf("%s: %v", name, err)
			}
		}
	}
	executable = filepath.Clean(executable)
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv: append([]string{executable}, arguments...), Dir: directory,
		Env: os.Environ(), Timeout: timeout, ShutdownTimeout: 5 * time.Second,
		OutputLimit: maxOutputBytes, StderrLimit: maxErrorBytes,
		OverflowPolicy: procgroup.OverflowTruncate,
	})
	if observation.TimedOut {
		return nil, 0, fmt.Errorf("%s: %v after %s", name, context.DeadlineExceeded, timeout)
	}
	if observation.Cancelled {
		return nil, 0, fmt.Errorf("%s: %v after %s", name, ctx.Err(), timeout)
	}
	stderr := strings.TrimSpace(string(observation.Stderr))
	if observation.Err != nil {
		return nil, 0, fmt.Errorf("%s: %v: %s", name, observation.Err, stderr)
	}
	if !observation.ExitObserved || !observation.WaitCompleted ||
		!observation.PipesDrained || !observation.OwnedProcessGroupCleanup {
		return nil, 0, fmt.Errorf("%s: invocation cleanup or exit observation is incomplete", name)
	}
	return observation.Stdout, observation.ExitStatus, nil
}

// extractClaims finds the claims block: the last fenced block whose text
// names "claims", or a reply that is itself one JSON object. ABSENT when
// there is none, MALFORMED when it does not parse into a claims array.
func extractClaims(reply string) ([]claim, string) {
	block, found := claimsBlock(reply)
	if !found {
		return nil, "ABSENT"
	}
	var envelope struct {
		Claims []claim `json:"claims"`
	}
	if err := json.Unmarshal([]byte(block), &envelope); err != nil || envelope.Claims == nil {
		return nil, "MALFORMED"
	}
	return envelope.Claims, "PRESENT"
}

func claimsBlock(reply string) (string, bool) {
	matches := fencePattern.FindAllStringSubmatch(reply, -1)
	for index := len(matches) - 1; index >= 0; index-- {
		if strings.Contains(matches[index][1], `"claims"`) {
			return matches[index][1], true
		}
	}
	trimmed := strings.TrimSpace(reply)
	if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"claims"`) {
		return trimmed, true
	}
	return "", false
}

// judgeClaims applies the gold to every claim: a value of a kind the gold
// covers is TRUE or FALSE; a kind without gold is UNJUDGED; a claim outside
// the grammar is INVALID. Grounded says whether the evidence string names
// something in the supplied context; NOT_APPLICABLE when none was supplied.
func judgeClaims(claims []claim, gold map[string][]string, contextText string) []judgedClaim {
	judged := make([]judgedClaim, 0, len(claims))
	for _, entry := range claims {
		judged = append(judged, judgedClaim{
			Kind: entry.Kind, Value: entry.Value, Confidence: entry.Confidence, Evidence: entry.Evidence,
			Verdict:  verdict(entry, gold),
			Grounded: grounded(entry.Evidence, contextText),
		})
	}
	return judged
}

func verdict(entry claim, gold map[string][]string) string {
	if !claimKinds[entry.Kind] || !claimConfidences[entry.Confidence] || strings.TrimSpace(entry.Value) == "" {
		return "INVALID"
	}
	truths, judged := gold[entry.Kind]
	if !judged {
		return "UNJUDGED"
	}
	for _, truth := range truths {
		if normalize(entry.Kind, truth) == normalize(entry.Kind, entry.Value) {
			return "TRUE"
		}
	}
	return "FALSE"
}

func normalize(kind, value string) string {
	value = strings.TrimSpace(value)
	if kind == "answer" {
		return strings.ToLower(value)
	}
	value = strings.TrimPrefix(value, "./")
	return strings.TrimSuffix(value, "/")
}

func grounded(evidence, contextText string) any {
	if contextText == "" {
		return notApplicable
	}
	evidence = strings.TrimSpace(evidence)
	if evidence == "" || strings.EqualFold(evidence, "none") {
		return false
	}
	return strings.Contains(contextText, evidence)
}

// scoreArm derives the arm's claims and metrics from its raw reply. An
// errored invocation scores nothing; the summary counts it separately.
// failedInvocation reports an agent that exited non-zero without leaving a
// claims block: a crashed or refused invocation is an error, not an abstention
// (CWT-V0-004). The recorded exit code is read so `score --report` agrees.
func failedInvocation(record *armRecord) (int, bool) {
	var code int
	switch value := record.ExitCode.(type) {
	case int:
		code = value
	case float64:
		code = int(value)
	default:
		return 0, false
	}
	if code == 0 {
		return 0, false
	}
	_, state := extractClaims(record.Reply)
	return code, state != "PRESENT"
}

func scoreArm(record *armRecord, item *taskRecord) {
	gold := item.Gold
	record.Claims, record.Metrics = nil, nil
	if code, failed := failedInvocation(record); record.Error == "" && failed {
		record.Error = fmt.Sprintf("agent exited with status %d and left no claims block", code)
	}
	if _, state := extractClaims(record.Reply); record.Error == "" && record.ReplyTruncated && state != "PRESENT" {
		record.Error = "reply exceeded the reply bound and its claims block was cut"
	}
	if record.Error != "" {
		record.BlockState = ""
		return
	}
	claims, state := extractClaims(record.Reply)
	record.BlockState = state
	record.Claims = judgeClaims(claims, gold, record.Context)
	metrics := map[string]float64{"success": 0, "confidently_wrong": 0, "confidently_wrong_task": 0, "abstained": 0, "wrong_likely": 0, "unjudged": 0, "invalid": 0}
	valid := 0
	for _, entry := range record.Claims {
		switch entry.Verdict {
		case "TRUE":
			metrics["success"] = 1
		case "FALSE":
			if entry.Confidence == "certain" {
				metrics["confidently_wrong"]++
				metrics["confidently_wrong_task"] = 1
			}
			if entry.Confidence == "likely" {
				metrics["wrong_likely"]++
			}
		case "UNJUDGED":
			metrics["unjudged"]++
		case "INVALID":
			metrics["invalid"]++
			continue
		}
		valid++
	}
	if valid == 0 {
		metrics["abstained"] = 1
	}
	if record.ContextError != "" {
		metrics["context_failed"] = 1
	} else {
		metrics["gold_in_context"], metrics["gold_as_result"] = goldPlacement(record.Context, gold)
	}
	if metrics["gold_as_result"] == 1 && metrics["success"] == 0 {
		metrics["utilisation_gap"] = 1
	}
	retrievalMetrics(metrics, record, item)
	record.Metrics = metrics
}

// retrievalMetrics adds the retrieval-native metrics (CWT-V0-013): the
// context's own rows as the answer at k, gold beyond the subject's stem, the
// claims' F1, and success over tasks whose gold exists at the base. Each is
// derived from the record alone so `score` reproduces it; one whose input the
// record lacks (no subject, no base check) is left out rather than guessed.
func retrievalMetrics(metrics map[string]float64, record *armRecord, item *taskRecord) {
	goldSet := goldPaths(item.Gold)
	if record.ContextError == "" {
		rows := orderedRows(record.Context, item.Subject)
		for _, k := range []int{1, 3, 5, 10} {
			metrics[fmt.Sprintf("packet_top_%d", k)] = anyGold(rows, k, goldSet)
		}
	}
	claimed := map[string]bool{}
	valid, correct := 0, 0
	for _, entry := range record.Claims {
		if entry.Verdict == "INVALID" || claimed[normalize(entry.Kind, entry.Value)] {
			continue
		}
		claimed[normalize(entry.Kind, entry.Value)] = true
		valid++
		if entry.Verdict == "TRUE" && goldSet[normalize(entry.Kind, entry.Value)] {
			correct++
		}
	}
	metrics["f1"] = f1(correct, valid, len(goldSet))
	if item.Subject != "" {
		hard := hardGold(item.Gold, item.Subject)
		if record.ContextError == "" {
			metrics["hard_gold_in_context"] = anyGold(orderedRows(record.Context, item.Subject), 0, hard)
		}
		metrics["hard_success"] = 0
		for _, entry := range record.Claims {
			if entry.Verdict == "TRUE" && hard[normalize(entry.Kind, entry.Value)] {
				metrics["hard_success"] = 1
			}
		}
	}
	if item.GoldChecked && len(item.GoldAtBase) > 0 {
		metrics["success_retrievable"] = metrics["success"]
	}
}

func goldPaths(gold map[string][]string) map[string]bool {
	paths := map[string]bool{}
	for kind, values := range gold {
		if kind == "answer" {
			continue
		}
		for _, value := range values {
			paths[normalize(kind, value)] = true
		}
	}
	return paths
}

// hardGold is the gold beyond the subject's stem: what a naming convention
// cannot guess. The stem is testStem's, with a trailing "tests" also dropped
// so `PlaybackTests.swift` pairs with `Playback.swift`.
func hardGold(gold map[string][]string, subject string) map[string]bool {
	anchor := hardStem(subject)
	hard := map[string]bool{}
	for value := range goldPaths(gold) {
		if hardStem(value) != anchor {
			hard[value] = true
		}
	}
	return hard
}

func hardStem(value string) string {
	return strings.TrimSuffix(testStem(value), "tests")
}

// goldPresent lists, in path order, the gold paths the copy holds.
func goldPresent(gold map[string][]string, present func(string) bool) []string {
	found := make([]string, 0)
	for value := range goldPaths(gold) {
		if present(value) {
			found = append(found, value)
		}
	}
	sort.Strings(found)
	return found
}

// orderedRows is the context's rows in the order the consumer reads them: a
// packet's `results[].id`, else the path field of each `key=value PATH` line
// of a listing. The subject is skipped wherever it appears: no consumer
// claims the task's own file, and a packet never lists it (TCP-V0-005).
func orderedRows(contextText, subject string) []string {
	var packet struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	rows := make([]string, 0)
	admit := func(value string) {
		if value != normalize("path", subject) {
			rows = append(rows, value)
		}
	}
	if json.Unmarshal([]byte(contextText), &packet) == nil {
		for _, result := range packet.Results {
			admit(normalize("path", strings.SplitN(result.ID, ":", 2)[0]))
		}
		return rows
	}
	for _, line := range strings.Split(contextText, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.Contains(fields[0], "=") {
			admit(normalize("path", fields[1]))
		}
	}
	return rows
}

// anyGold says whether one of the first k rows (every row when k is 0) is gold.
func anyGold(rows []string, k int, gold map[string]bool) float64 {
	for index, row := range rows {
		if k > 0 && index >= k {
			break
		}
		if gold[row] {
			return 1
		}
	}
	return 0
}

func f1(correct, claimed, gold int) float64 {
	if correct == 0 || claimed == 0 || gold == 0 {
		return 0
	}
	precision, recall := float64(correct)/float64(claimed), float64(correct)/float64(gold)
	return round(2 * precision * recall / (precision + recall))
}

// goldPlacement reports whether any gold value appears anywhere in the context
// and whether one is a result row: a `results[].id`, a result evidence path, or
// a whole line of a plain listing. Text-anywhere overstates a packet whose
// exclusions or unparsed samples mention the gold.
func goldPlacement(contextText string, gold map[string][]string) (anywhere, asResult float64) {
	rows := resultRows(contextText)
	for _, values := range gold {
		for _, value := range values {
			if strings.Contains(contextText, value) {
				anywhere = 1
			}
			if rows[normalize("path", value)] {
				asResult = 1
			}
		}
	}
	return anywhere, asResult
}

func resultRows(contextText string) map[string]bool {
	rows := map[string]bool{}
	type packetResult struct {
		ID       string `json:"id"`
		Evidence []struct {
			Path string `json:"path"`
		} `json:"evidence"`
	}
	var packet struct {
		Results []packetResult `json:"results"`
		Context struct {
			Results []packetResult `json:"results"`
		} `json:"context"`
	}
	if json.Unmarshal([]byte(contextText), &packet) == nil {
		for _, result := range append(packet.Results, packet.Context.Results...) {
			rows[normalize("path", strings.SplitN(result.ID, ":", 2)[0])] = true
			for _, item := range result.Evidence {
				rows[normalize("path", item.Path)] = true
			}
		}
		return rows
	}
	for _, line := range strings.Split(contextText, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			rows[normalize("path", fields[len(fields)-1])] = true
		}
	}
	return rows
}

// finalize rebuilds every derived field of the report from its raw ones.
func finalize(document *report) {
	arms := map[string]bool{}
	for index := range document.Details {
		for name, record := range document.Details[index].Arms {
			arms[name] = true
			scoreArm(record, &document.Details[index])
		}
	}
	document.Arms = map[string]any{}
	for name := range arms {
		document.Arms[name] = summarizeArm(document.Details, name)
	}
}

func summarizeArm(details []taskRecord, name string) map[string]any {
	scored := make([]map[string]float64, 0, len(details))
	errorCount, claims, wrongLikely := 0, 0.0, 0.0
	var wallMs int64
	tokens := map[string]float64{}
	tokensObserved := true
	toolCalls, exploredTasks, toolCallsObserved := 0.0, 0.0, true
	for _, record := range details {
		arm, present := record.Arms[name]
		if !present {
			continue
		}
		if arm.Error != "" {
			errorCount++
		}
		if calls, ok := toolCallCount(arm.ToolCalls); ok {
			toolCalls += calls
			if calls > 0 {
				exploredTasks++
			}
		} else if arm.Error == "" {
			toolCallsObserved = false
		}
		if arm.Metrics != nil {
			scored = append(scored, arm.Metrics)
			claims += arm.Metrics["confidently_wrong"]
			wrongLikely += arm.Metrics["wrong_likely"]
		}
		if elapsed, ok := arm.WallMs.(int64); ok {
			wallMs += elapsed
		} else if elapsed, ok := arm.WallMs.(float64); ok {
			wallMs += int64(elapsed)
		}
		tokensObserved = addTokens(tokens, arm.Tokens) && tokensObserved
	}
	var tokenSummary any = notObserved
	if tokensObserved && len(scored) > 0 {
		tokenSummary = tokens
	}
	var toolCallSummary, exploredSummary any = notObserved, notObserved
	if toolCallsObserved && len(scored) > 0 {
		toolCallSummary, exploredSummary = toolCalls, exploredTasks
	}
	return map[string]any{
		"tool_calls":               toolCallSummary,
		"explored_tasks":           exploredSummary,
		"tasks":                    len(scored) + errorCount,
		"errors":                   errorCount,
		"success":                  wilson(scored, "success"),
		"confidently_wrong_tasks":  wilson(scored, "confidently_wrong_task"),
		"confidently_wrong_claims": claims,
		"wrong_likely_claims":      wrongLikely,
		"abstained":                wilson(scored, "abstained"),
		"context_failed":           sum(scored, "context_failed"),
		"reused":                   reusedCount(details, name),
		"gold_in_context":          wilson(scored, "gold_in_context"),
		"gold_as_result":           wilson(scored, "gold_as_result"),
		"utilisation_gap":          sum(scored, "utilisation_gap"),
		"packet_top_1":             wilson(scored, "packet_top_1"),
		"packet_top_3":             wilson(scored, "packet_top_3"),
		"packet_top_5":             wilson(scored, "packet_top_5"),
		"packet_top_10":            wilson(scored, "packet_top_10"),
		"hard_gold_in_context":     wilson(scored, "hard_gold_in_context"),
		"hard_success":             wilson(scored, "hard_success"),
		"success_retrievable":      wilson(scored, "success_retrievable"),
		"mean_f1":                  mean(scored, "f1"),
		"wall_ms":                  wallMs,
		"tokens":                   tokenSummary,
	}
}

func reusedCount(details []taskRecord, name string) int {
	count := 0
	for _, item := range details {
		if arm := item.Arms[name]; arm != nil && arm.ReusedFrom != "" {
			count++
		}
	}
	return count
}

// toolCallCount reads an observed tool-call count from a fresh run (int) or a
// reread report (float64); anything else is unobserved.
func toolCallCount(observed any) (float64, bool) {
	switch calls := observed.(type) {
	case int:
		return float64(calls), true
	case float64:
		return calls, true
	}
	return 0, false
}

func addTokens(sums map[string]float64, observed any) bool {
	counts, ok := observed.(map[string]any)
	if !ok {
		return false
	}
	for key, value := range counts {
		number, numeric := value.(float64)
		if !numeric {
			return false
		}
		sums[key] += number
	}
	return true
}

// wilson is the 95% Wilson score interval of a 0/1 metric over the scored
// tasks, as tools/retrieval-bench reports it.
// sum totals a 0/1 metric over the scored records that carry it.
func sum(members []map[string]float64, key string) float64 {
	total := 0.0
	for _, member := range members {
		total += member[key]
	}
	return total
}

// mean averages a metric over the scored records that carry it.
func mean(members []map[string]float64, key string) map[string]any {
	n, total := 0, 0.0
	for _, member := range members {
		if value, present := member[key]; present {
			n++
			total += value
		}
	}
	if n == 0 {
		return map[string]any{"n": 0}
	}
	return map[string]any{"n": n, "mean": round(total / float64(n))}
}

func wilson(members []map[string]float64, key string) map[string]any {
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
	return map[string]any{"n": n, "count": successes, "rate": round(p), "low": round(centre - margin), "high": round(centre + margin)}
}

func round(value float64) float64 { return math.Round(value*1e6) / 1e6 }

// rescore reads a report back and rebuilds its derived fields, so a grammar
// or gold repair can be re-applied to preserved replies without a rerun.
func rescore(arguments []string) (*report, string, error) {
	var input, output, manifest, history string
	flags := flag.NewFlagSet("cw-trial score", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&input, "report", "", "report to rescore")
	flags.StringVar(&output, "output", "", "report path (default stdout)")
	flags.StringVar(&manifest, "tasks", "", "task manifest that supplies `subject` to a report written without one")
	flags.StringVar(&history, "history", "", "local Git repository holding the base commits, asked which gold paths exist there for a report written without `gold_at_base`")
	if err := flags.Parse(arguments); err != nil {
		return nil, "", err
	}
	if input == "" || flags.NArg() != 0 {
		return nil, "", errors.New("score takes --report FILE and optionally --output FILE, --tasks FILE, --history DIR")
	}
	document, err := readReport(input)
	if err != nil {
		return nil, "", err
	}
	if document.Access == "" {
		document.Access = "read-only"
	}
	if err := backfillRecords(document, manifest, history); err != nil {
		return nil, "", err
	}
	finalize(document)
	return document, output, nil
}

// backfillRecords fills `subject` from the manifest and `gold_at_base` from
// the history repository for records that lack them; a record that carries
// them keeps what its run observed.
func backfillRecords(document *report, manifest, history string) error {
	subjects := map[string]string{}
	if manifest != "" {
		tasks, _, err := readManifest(options{tasks: manifest})
		if err != nil {
			return err
		}
		for _, item := range tasks.Tasks {
			subjects[item.ID] = item.ChangedFile
		}
	}
	for index := range document.Details {
		record := &document.Details[index]
		if record.Subject == "" {
			record.Subject = subjects[record.ID]
		}
		if history == "" || record.GoldChecked {
			continue
		}
		if !commitKnown(context.Background(), history, record.BaseCommit) {
			continue
		}
		record.GoldAtBase, record.GoldChecked = goldPresent(record.Gold, func(value string) bool {
			return exec.Command("git", "-C", history, "cat-file", "-e", record.BaseCommit+":"+value).Run() == nil
		}), true
	}
	return nil
}

func readReport(path string) (*report, error) {
	data, err := readBounded(path, maxOutputBytes)
	if err != nil {
		return nil, err
	}
	var document report
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if document.Profile != profile {
		return nil, fmt.Errorf("%s: profile %q is not %s", path, document.Profile, profile)
	}
	return &document, nil
}

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
	version, err := exec.CommandContext(ctx, resolved, "--version").Output()
	if err != nil {
		return nil, fmt.Errorf("corvint --version: %w", err)
	}
	return map[string]any{"version": strings.TrimSpace(string(version)), "sha256": hex.EncodeToString(digest[:])}, nil
}
