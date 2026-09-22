// Command cem-trial is the external agent-harness dispatcher for the CEM
// reviewer trial in docs/specs/cem-reviewer-trial-v0.md. It selects changes
// from a local Git history, presents each change's source hunks to one agent
// twice — once bare (control) and once with a cem/0.1 map, status worklist,
// and report (treatment) — and scores how much of the change's own withheld
// test-or-spec evidence each arm cited. It downloads nothing beyond the local
// history it is given and writes only its own output tree.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	profile            = "corvint-cem-trial/0"
	notObserved        = "NOT_OBSERVED"
	maxTasksFile       = 16 << 20
	maxPatchBytes      = 64 << 10
	maxReplyBytes      = 1 << 20
	maxOutputBytes     = 8 << 20
	maxErrorBytes      = 64 << 10
	maxAgentErrorBytes = 2 << 10
	maxArtefactSize    = 256 << 10
	defaultTimeout     = 5 * time.Minute
	corvintTimeout     = 2 * time.Minute
	truncatedMarker    = "\n[TRUNCATED]"
	mapRelative        = "change.cem.json"
	reportRelative     = "cem-report.md"
)

var armNames = []string{"control", "treatment"}

// skeleton is the one prompt both arms receive. Only the third field differs:
// it is empty for control and carries the treatment prologue and its three
// artefacts for treatment, so removing that block from a treatment prompt
// reproduces the control prompt byte for byte (CRT-V0-005, CRT-V0-010).
const skeleton = `You are reviewing one proposed change to a repository. Your working directory holds
that repository checked out at the change's base revision, with its history. The change
itself is not committed anywhere you can reach.

For each numbered hunk of the change, name the existing evidence in the repository at this
base revision that justifies it: a specification, a decision record, a test that claims the
behaviour, an implementation, a call site, a dependency, or an incident record. Name the
hunks you cannot justify instead of guessing.

## Hunks

%s

## Patch

` + "```diff\n%s\n```" + `
%s
## Reply

End your reply with one fenced JSON block and nothing after it:

` + "```json\n" +
	`{"citations":[{"hunk":"N","path":"...","lines":"S:E","relation":"specification|decision|test-claim|implementation|call-site|dependency|incident","confidence":"certain|likely|unsure"}],"unknown":["N"]}` +
	"\n```" + `

Every "path" is a repository-relative path that exists at the base revision, every "lines" a
one-based inclusive line range in that file. List in "unknown" the number of every hunk you
could not justify.
`

// treatmentPrologue is the only declared difference between the arms. It is
// inserted whole, so the control prompt is this block removed.
const treatmentPrologue = `
## Change Evidence Map

A Change Evidence Map has already been built over the exact patch bytes above. It lists the
same hunks with a stable identifier and an explicit disposition, and it is the worklist you
are completing: every hunk whose disposition is ` + "`unknown`" + ` still needs evidence.

### Map

` + "```json\n%s\n```" + `

### Status worklist

` + "```json\n%s\n```" + `

### Report

` + "```\n%s\n```" + `
`

type options struct {
	tasks        string
	history      string
	corvintGo    string
	arms         []string
	repeats      int
	agent        string
	agentCommand string
	model        string
	effort       string
	timeout      time.Duration
	workers      int
	output       string
	seed         string
	resume       bool
	reuse        string
}

type agentResult struct {
	reply      string
	exitCode   any
	tokens     any
	toolCalls  any
	stderrTail string
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
		fmt.Fprintln(stderr, "cem-trial: usage: cem-trial select|run|score [flags]")
		return 2
	}
	var document *report
	var output string
	var err error
	switch arguments[0] {
	case "select":
		return selectChanges(ctx, arguments[1:], stdout, stderr)
	case "run":
		document, output, err = runTrial(ctx, arguments[1:])
		if err == nil && output != "" {
			defer os.Remove(checkpointPath(output))
		}
	case "score":
		document, output, err = rescore(arguments[1:])
	default:
		err = fmt.Errorf("unknown command %q", arguments[0])
	}
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial:", err)
		return 2
	}
	return write(document, output, stdout, stderr)
}

func write(document *report, output string, stdout, stderr io.Writer) int {
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial: cannot encode the report")
		return 2
	}
	encoded = append(encoded, '\n')
	if output == "" {
		_, err = stdout.Write(encoded)
	} else {
		err = os.WriteFile(output, encoded, 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "cem-trial: cannot write the report")
		return 2
	}
	return 0
}

func parseRunOptions(arguments []string) (options, error) {
	var configuration options
	var arms string
	flags := flag.NewFlagSet("cem-trial run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&configuration.tasks, "tasks", "", "task manifest JSON written by select")
	flags.StringVar(&configuration.history, "history", "", "local Git repository holding the changes")
	flags.StringVar(&configuration.corvintGo, "corvint", "", "corvint executable (required by the treatment arm)")
	flags.StringVar(&arms, "arms", "control,treatment", "comma-separated arms to run")
	flags.IntVar(&configuration.repeats, "repeats", 1, "invocations per change and arm")
	flags.StringVar(&configuration.agent, "agent", "codex", "agent kind: codex or script")
	flags.StringVar(&configuration.agentCommand, "agent-command", "", "script agent: executable given the prompt as its argument, replying on stdout")
	flags.StringVar(&configuration.model, "model", "", "model id passed to codex -m (required for the codex agent)")
	flags.StringVar(&configuration.effort, "effort", "", "codex model_reasoning_effort override")
	flags.DurationVar(&configuration.timeout, "timeout", defaultTimeout, "per-invocation wall-clock limit")
	flags.IntVar(&configuration.workers, "workers", 1, "concurrent changes (1 = sequential)")
	flags.StringVar(&configuration.output, "output", "", "report path (default stdout)")
	flags.StringVar(&configuration.seed, "seed", "", "seed for the per-change arm order")
	flags.BoolVar(&configuration.resume, "resume", false, "reuse OUTPUT.partial.json, rerunning only lanes that never finished")
	flags.StringVar(&configuration.reuse, "reuse", "", "reuse the finished lanes of a prior report with byte-identical prompts")
	if err := flags.Parse(arguments); err != nil {
		return configuration, err
	}
	if configuration.tasks == "" || configuration.history == "" || flags.NArg() != 0 {
		return configuration, errors.New("usage: cem-trial run --tasks FILE --history DIR [--corvint PATH] [flags]")
	}
	configuration.arms = splitList(arms)
	for _, name := range configuration.arms {
		if name != "control" && name != "treatment" {
			return configuration, fmt.Errorf("unknown arm %q", name)
		}
	}
	if len(configuration.arms) == 0 || configuration.repeats < 1 {
		return configuration, errors.New("--arms must name at least one arm and --repeats at least 1")
	}
	if contains(configuration.arms, "treatment") && configuration.corvintGo == "" {
		return configuration, errors.New("--corvint is required by the treatment arm")
	}
	if configuration.agent == "codex" && configuration.model == "" {
		return configuration, errors.New("--model is required for the codex agent")
	}
	if configuration.agent == "script" && configuration.agentCommand == "" {
		return configuration, errors.New("--agent-command is required for the script agent")
	}
	if configuration.resume && configuration.output == "" {
		return configuration, errors.New("--resume needs --output to find the checkpoint")
	}
	if configuration.corvintGo != "" {
		absolute, err := filepath.Abs(configuration.corvintGo)
		if err != nil {
			return configuration, err
		}
		configuration.corvintGo = absolute
	}
	return configuration, nil
}

func runTrial(ctx context.Context, arguments []string) (*report, string, error) {
	configuration, err := parseRunOptions(arguments)
	if err != nil {
		return nil, "", err
	}
	set, raw, err := readManifest(configuration.tasks)
	if err != nil {
		return nil, "", err
	}
	runner, err := newAgent(configuration)
	if err != nil {
		return nil, "", err
	}
	identity, err := runner.identity(ctx)
	if err != nil {
		return nil, "", err
	}
	document := &report{
		Profile: profile, Partition: set.Partition, Pilot: set.Partition == "pilot",
		Repository: set.Repository, SelectionRule: set.SelectionRule, Population: set.Population,
		Pairs: len(set.Tasks), Seed: configuration.seed, Model: modelOrNot(configuration),
		Effort: valueOrNot(configuration.effort), ManifestSHA256: digestOf(string(raw)),
		SkeletonSHA256: digestOf(skeleton), Arms: configuration.arms, Repeats: configuration.repeats,
		Prologues: prologueIdentity(configuration.arms), Agent: identity,
		CorvintGo: corvintIdentity(ctx, configuration), Changes: changeMetadata(set), Lanes: []*lane{},
	}
	prior, err := loadReuse(configuration)
	if err != nil {
		return nil, "", err
	}
	if configuration.resume {
		if checkpoint, err := loadReport(checkpointPath(configuration.output)); err == nil {
			prior = prior.merge(checkpoint)
		}
	}
	base := filepath.Dir(configuration.tasks)
	lanes := plan(configuration, set)
	for _, item := range lanes {
		document.Lanes = append(document.Lanes, item.records...)
	}
	saver := newCheckpointer(document, configuration.output)
	invokePending(ctx, configuration, runner, base, lanes, prior, saver)
	finalize(document)
	return document, configuration.output, nil
}

// changePlan is one change's lanes, in the seeded arm order.
type changePlan struct {
	task    task
	records []*lane
}

// plan fixes every lane and the per-change arm order before the first
// invocation: the order is a seeded shuffle recorded on each lane.
func plan(configuration options, set *manifest) []changePlan {
	plans := make([]changePlan, 0, len(set.Tasks))
	for _, item := range set.Tasks {
		order := shuffledArms(configuration.seed, item.ID, configuration.arms)
		records := make([]*lane, 0, len(order)*configuration.repeats)
		for _, name := range order {
			for repeat := 1; repeat <= configuration.repeats; repeat++ {
				records = append(records, &lane{
					ID: item.ID, Arm: name, Repeat: repeat, ArmOrder: order,
					ReplyState: "ABSENT", Citations: []citation{}, Unknown: []string{},
					WallMs: notObserved, Tokens: notObserved, ExitCode: notObserved,
				})
			}
		}
		plans = append(plans, changePlan{task: item, records: records})
	}
	return plans
}

// shuffledArms orders the arms by a Fisher-Yates shuffle over a stream
// derived from the seed and the change id, so the order is reproducible and
// independent between changes.
func shuffledArms(seed, id string, arms []string) []string {
	order := append([]string(nil), arms...)
	stream := seededStream(seed + "\x00" + id)
	for index := len(order) - 1; index > 0; index-- {
		pick := int(stream() % uint64(index+1))
		order[index], order[pick] = order[pick], order[index]
	}
	return order
}

// seededStream is a counter-mode SHA-256 stream: deterministic, seed-derived,
// and independent of the standard library's generator version.
func seededStream(seed string) func() uint64 {
	counter := 0
	return func() uint64 {
		sum := sha256.Sum256([]byte(seed + "\x00" + strconv.Itoa(counter)))
		counter++
		value := uint64(0)
		for _, b := range sum[:8] {
			value = value<<8 | uint64(b)
		}
		return value >> 1
	}
}

// invokePending runs the planned changes, at most --workers at a time. Each
// change owns one prep clone at its base revision and one disjoint lane clone
// per arm, so no state crosses arms.
func invokePending(ctx context.Context, configuration options, runner agent, base string, plans []changePlan, prior *reuseSource, saver *checkpointer) {
	saver.save()
	slots := make(chan struct{}, maxInt(configuration.workers, 1))
	var group sync.WaitGroup
	for _, item := range plans {
		slots <- struct{}{}
		group.Add(1)
		go func(item changePlan) {
			defer group.Done()
			defer func() { <-slots }()
			runChange(ctx, configuration, runner, base, item, prior, saver)
			saver.save()
		}(item)
	}
	group.Wait()
}

func runChange(ctx context.Context, configuration options, runner agent, base string, item changePlan, prior *reuseSource, saver *checkpointer) {
	patch, err := readPatch(base, item.task)
	if err != nil {
		failLanes(item.records, err, saver)
		return
	}
	prep, err := cloneAtBase(ctx, configuration.history, item.task.BaseCommit)
	if err != nil {
		failLanes(item.records, err, saver)
		return
	}
	defer os.RemoveAll(prep)
	if resolvable(ctx, prep, item.task.Commit) {
		failLanes(item.records, fmt.Errorf("the clone resolves %s: the change leaked into its own base", item.task.Commit), saver)
		return
	}
	artefacts, err := buildArtefacts(ctx, configuration, prep, patch)
	if err != nil && contains(configuration.arms, "treatment") {
		failLanes(item.records, err, saver)
		return
	}
	prompts := map[string]string{
		"control":   buildPrompt("control", item.task, patch, artefacts),
		"treatment": buildPrompt("treatment", item.task, patch, artefacts),
	}
	for _, record := range item.records {
		record.PromptSHA256 = digestOf(prompts[record.Arm])
		record.PromptBytes = len(prompts[record.Arm])
		if prior.apply(record) {
			continue
		}
		invokeLane(ctx, configuration, runner, prep, prompts[record.Arm], patch, item.task, record, saver)
		saver.save()
	}
}

func failLanes(records []*lane, err error, saver *checkpointer) {
	for _, record := range records {
		assign(saver.lock(), func() { record.Error = err.Error() })
	}
}

func invokeLane(ctx context.Context, configuration options, runner agent, prep, prompt, patch string, item task, record *lane, saver *checkpointer) {
	if err := ctx.Err(); err != nil {
		assign(saver.lock(), func() { record.Error = err.Error() })
		return
	}
	root, err := laneClone(ctx, prep)
	if err != nil {
		assign(saver.lock(), func() { record.Error = err.Error() })
		return
	}
	defer os.RemoveAll(root)
	started := time.Now()
	result, runErr := runner.run(ctx, root, prompt, configuration.timeout)
	wall := time.Since(started).Milliseconds()
	assign(saver.lock(), func() {
		record.WallMs = wall
		if runErr != nil {
			record.Error = runErr.Error()
			return
		}
		record.ExitCode, record.Tokens, record.ToolCalls = result.exitCode, result.tokens, result.toolCalls
		record.Reply, record.ReplyTruncated = truncate(result.reply, maxReplyBytes)
		if code, ok := result.exitCode.(int); ok && code != 0 {
			record.Error = agentExitError(code, result.stderrTail)
		}
	})
	if runErr != nil || record.Error != "" || record.Arm != "treatment" || configuration.corvintGo == "" {
		return
	}
	citations, _ := extractCitations(record.Reply)
	state := applyCitations(ctx, configuration, root, patch, item, citations)
	assign(saver.lock(), func() { record.CEM = state })
}

// buildArtefacts produces the treatment arm's three artefacts from the
// withheld patch bytes alone: the map from `cem begin`, the `cem status`
// worklist, and `cem report`. `cem prepare` is never used, so nothing at or
// after the change can reach the map (CRT-V0-005).
func buildArtefacts(ctx context.Context, configuration options, root, patch string) (map[string]string, error) {
	if configuration.corvintGo == "" {
		return map[string]string{}, nil
	}
	scratch, err := os.MkdirTemp("", "corvint-cem-trial-patch-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	patchPath := filepath.Join(scratch, "change.patch")
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		return nil, err
	}
	begun, err := corvint(ctx, configuration, root, "cem", "begin", "--patch", patchPath, "--output", mapRelative)
	if err != nil {
		return nil, fmt.Errorf("cem begin: %w", err)
	}
	status, err := corvint(ctx, configuration, root, "cem", "status", "--map", mapRelative, "--patch", patchPath)
	if err != nil {
		return nil, fmt.Errorf("cem status: %w", err)
	}
	if _, err := corvint(ctx, configuration, root, "cem", "report", "--map", mapRelative, "--patch", patchPath, "--output", reportRelative); err != nil {
		return nil, fmt.Errorf("cem report: %w", err)
	}
	rendered, err := os.ReadFile(filepath.Join(root, reportRelative))
	if err != nil {
		return nil, err
	}
	document, err := os.ReadFile(filepath.Join(root, mapRelative))
	if err != nil {
		return nil, err
	}
	// The prep clone is cloned into every lane: leave neither artefact behind.
	_ = os.Remove(filepath.Join(root, mapRelative))
	_ = os.Remove(filepath.Join(root, reportRelative))
	_ = begun
	body, _ := truncate(string(rendered), maxArtefactSize)
	mapText, _ := truncate(string(document), maxArtefactSize)
	statusText, _ := truncate(string(status), maxArtefactSize)
	return map[string]string{"map": strings.TrimSpace(mapText), "status": strings.TrimSpace(statusText), "report": strings.TrimSpace(body)}, nil
}

// applyCitations replays the treatment reply into the map the arm was given
// and records what `cem status` and `cem verify` then say, beside the
// harness's own independent replay of every citation.
func applyCitations(ctx context.Context, configuration options, root, patch string, item task, citations []citation) map[string]any {
	state := map[string]any{"counts": map[string]any{}, "state": notObserved, "verify_ok": notObserved, "replay_ok": notObserved, "cited": len(citations)}
	scratch, err := os.MkdirTemp("", "corvint-cem-trial-cite-")
	if err != nil {
		return state
	}
	defer os.RemoveAll(scratch)
	patchPath := filepath.Join(scratch, "change.patch")
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		return state
	}
	if _, err := corvint(ctx, configuration, root, "cem", "begin", "--patch", patchPath, "--output", mapRelative); err != nil {
		return state
	}
	accepted := 0
	for _, one := range citations {
		if _, err := corvint(ctx, configuration, root, "cem", "cite", "--map", mapRelative,
			"--hunk", one.Hunk, "--evidence-path", one.Path, "--lines", one.Lines, "--relation", one.Relation); err == nil {
			accepted++
		}
	}
	state["accepted"] = accepted
	if raw, err := corvint(ctx, configuration, root, "cem", "status", "--map", mapRelative, "--patch", patchPath); err == nil {
		var envelope struct {
			Counts map[string]any `json:"counts"`
			State  string         `json:"state"`
		}
		if json.Unmarshal([]byte(raw), &envelope) == nil {
			state["counts"], state["state"] = envelope.Counts, envelope.State
		}
	}
	raw, err := corvint(ctx, configuration, root, "cem", "verify", "--map", mapRelative, "--patch", patchPath)
	state["verify_ok"] = err == nil && verifyOK(raw)
	state["replay_ok"] = replayCitations(ctx, root, item.BaseCommit, citations)
	return state
}

func verifyOK(raw string) bool {
	var envelope struct {
		OK bool `json:"ok"`
	}
	return json.Unmarshal([]byte(raw), &envelope) == nil && envelope.OK
}

func corvint(ctx context.Context, configuration options, root string, arguments ...string) (string, error) {
	stdout, exitCode, _, err := runCommand(ctx, root, corvintTimeout, configuration.corvintGo, append([]string{"--root", root}, arguments...)...)
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		return string(stdout), fmt.Errorf("corvint exited %d: %s", exitCode, strings.TrimSpace(string(stdout)))
	}
	return string(stdout), nil
}

func corvintIdentity(ctx context.Context, configuration options) map[string]any {
	if configuration.corvintGo == "" {
		return map[string]any{"version": notObserved, "sha256": notObserved}
	}
	identity := map[string]any{"path": configuration.corvintGo, "version": notObserved, "sha256": notObserved}
	if data, err := os.ReadFile(configuration.corvintGo); err == nil {
		sum := sha256.Sum256(data)
		identity["sha256"] = hex.EncodeToString(sum[:])
	}
	if stdout, exitCode, _, err := runCommand(ctx, "", corvintTimeout, configuration.corvintGo, "--version"); err == nil && exitCode == 0 {
		identity["version"] = strings.TrimSpace(string(stdout))
	}
	return identity
}

// buildPrompt renders the one skeleton. The control arm's third field is
// empty, so the treatment prompt with its prologue block removed is the
// control prompt byte for byte.
func buildPrompt(arm string, item task, patch string, artefacts map[string]string) string {
	extra := ""
	if arm == "treatment" {
		extra = fmt.Sprintf(treatmentPrologue, artefacts["map"], artefacts["status"], artefacts["report"])
	}
	return fmt.Sprintf(skeleton, hunkList(patch), patch, extra)
}

// hunkList numbers the patch's hunks in the order `cem begin` reads them, so
// both arms name the same hunk by the same number.
func hunkList(patch string) string {
	var out strings.Builder
	number, path := 0, ""
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			path = strings.TrimPrefix(line, "+++ b/")
		}
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		number++
		fmt.Fprintf(&out, "%d. %s %s\n", number, path, strings.TrimSpace(line))
	}
	if number == 0 {
		return "(none)"
	}
	return strings.TrimRight(out.String(), "\n")
}

type citation struct {
	Hunk       string `json:"hunk"`
	Path       string `json:"path"`
	Lines      string `json:"lines"`
	Relation   string `json:"relation"`
	Confidence string `json:"confidence"`
}

var fencePattern = regexp.MustCompile("(?s)```(?:[a-zA-Z0-9]*)\\n(.*?)```")

// extractCitations finds the reply block: the last fenced block naming
// "citations", or a reply that is itself one JSON object. ABSENT when there
// is none, MALFORMED when it does not parse; both score as cited nothing.
func extractCitations(reply string) ([]citation, string) {
	block, found := citationsBlock(reply)
	if !found {
		return nil, "ABSENT"
	}
	var envelope struct {
		Citations []citation `json:"citations"`
		Unknown   []string   `json:"unknown"`
	}
	if err := json.Unmarshal([]byte(block), &envelope); err != nil || envelope.Citations == nil {
		return nil, "MALFORMED"
	}
	return envelope.Citations, "PRESENT"
}

func extractUnknown(reply string) []string {
	block, found := citationsBlock(reply)
	if !found {
		return []string{}
	}
	var envelope struct {
		Unknown []string `json:"unknown"`
	}
	if json.Unmarshal([]byte(block), &envelope) != nil || envelope.Unknown == nil {
		return []string{}
	}
	return envelope.Unknown
}

func citationsBlock(reply string) (string, bool) {
	matches := fencePattern.FindAllStringSubmatch(reply, -1)
	for index := len(matches) - 1; index >= 0; index-- {
		if strings.Contains(matches[index][1], `"citations"`) {
			return matches[index][1], true
		}
	}
	trimmed := strings.TrimSpace(reply)
	if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"citations"`) {
		return trimmed, true
	}
	return "", false
}

// cloneAtBase builds a repository holding the base commit and its history and
// nothing after it: a fetch of exactly that commit, never a shared clone,
// which would carry the change's own objects. `.corvint/` is scrubbed.
func cloneAtBase(ctx context.Context, history, base string) (string, error) {
	root, err := os.MkdirTemp("", "corvint-cem-trial-")
	if err != nil {
		return "", err
	}
	fail := func(step string, err error) (string, error) {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("git %s: %w", step, err)
	}
	source, err := filepath.Abs(history)
	if err != nil {
		return fail("abs", err)
	}
	for _, arguments := range [][]string{
		{"init", "-q"},
		{"fetch", "-q", "--no-tags", source, base},
		{"checkout", "-q", "--detach", "FETCH_HEAD"},
	} {
		if _, err := git(ctx, root, arguments...); err != nil {
			return fail(arguments[0], err)
		}
	}
	head, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil || head != base {
		return fail("rev-parse", fmt.Errorf("HEAD %q is not %s", head, base))
	}
	_ = os.RemoveAll(filepath.Join(root, ".corvint"))
	return root, nil
}

// laneClone gives one arm its own working copy of the prep clone. It is a
// plain local clone, never `--shared`: an alternates file would make the
// repository one corvint refuses, and the copied objects stop at the base
// commit either way.
func laneClone(ctx context.Context, prep string) (string, error) {
	root, err := os.MkdirTemp("", "corvint-cem-trial-lane-")
	if err != nil {
		return "", err
	}
	if _, err := git(ctx, root, "clone", "-q", "--no-checkout", prep, root); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("git clone: %w", err)
	}
	if _, err := git(ctx, root, "checkout", "-q", "--detach", "HEAD"); err != nil {
		_ = os.RemoveAll(root)
		return "", fmt.Errorf("git checkout: %w", err)
	}
	_ = os.RemoveAll(filepath.Join(root, ".corvint"))
	return root, nil
}

func resolvable(ctx context.Context, root, commit string) bool {
	_, err := git(ctx, root, "cat-file", "-e", commit+"^{commit}")
	return err == nil
}

func git(ctx context.Context, root string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-c", "core.quotepath=false"}, arguments...)...)
	command.Dir = root
	command.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=cem-trial", "GIT_AUTHOR_EMAIL=trial@localhost",
		"GIT_COMMITTER_NAME=cem-trial", "GIT_COMMITTER_EMAIL=trial@localhost",
	)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
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

func gitOutput(ctx context.Context, repo string, arguments ...string) (string, error) {
	raw, err := gitBytes(ctx, repo, arguments...)
	return string(raw), err
}

// codexAgent runs `codex exec` read-only inside the lane clone with stdin
// closed and reads the reply from --output-last-message.
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
	scratch, err := os.MkdirTemp("", "corvint-cem-trial-reply-")
	if err != nil {
		return agentResult{}, err
	}
	defer os.RemoveAll(scratch)
	replyPath := filepath.Join(scratch, "reply.txt")
	stdout, exitCode, stderrTail, err := runCommand(ctx, root, timeout, "codex", runner.arguments(root, replyPath, prompt)...)
	if err != nil {
		return agentResult{}, err
	}
	events := parseCodexEvents(stdout)
	reply, readErr := os.ReadFile(replyPath)
	if readErr != nil {
		reply = []byte(events.lastMessage)
	}
	return agentResult{
		reply: string(reply), exitCode: exitCode, tokens: events.tokens,
		toolCalls: events.toolCalls, stderrTail: stderrTail,
	}, nil
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
		"kind": "codex", "version": strings.TrimSpace(string(version)),
		"command": strings.Join(runner.arguments("ROOT", "REPLY", "PROMPT"), " "), "effort": effort,
	}, nil
}

type codexEvents struct {
	lastMessage string
	tokens      any
	toolCalls   int
}

var toolItemTypes = map[string]bool{"command_execution": true, "file_change": true, "mcp_tool_call": true, "web_search": true}

func parseCodexEvents(stdout []byte) codexEvents {
	result := codexEvents{tokens: notObserved}
	for _, line := range bytes.Split(stdout, []byte{'\n'}) {
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
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
		}
		if event.Type == "turn.completed" && len(event.Usage) > 0 {
			result.tokens = event.Usage
		}
	}
	return result
}

// scriptAgent is any executable taking the prompt as its only argument and
// replying on stdout: the fake agent that proves the pipeline.
type scriptAgent struct{ command string }

func (runner scriptAgent) run(ctx context.Context, root, prompt string, timeout time.Duration) (agentResult, error) {
	stdout, exitCode, stderrTail, err := runCommand(ctx, root, timeout, runner.command, prompt)
	if err != nil {
		return agentResult{}, err
	}
	return agentResult{
		reply: string(stdout), exitCode: exitCode, tokens: notObserved,
		toolCalls: notObserved, stderrTail: stderrTail,
	}, nil
}

func (runner scriptAgent) identity(context.Context) (map[string]any, error) {
	data, err := os.ReadFile(runner.command)
	if err != nil {
		return nil, fmt.Errorf("agent command: %w", err)
	}
	sum := sha256.Sum256(data)
	return map[string]any{"kind": "script", "command": runner.command, "sha256": hex.EncodeToString(sum[:])}, nil
}

func newAgent(configuration options) (agent, error) {
	switch configuration.agent {
	case "codex":
		return codexAgent{model: configuration.model, effort: configuration.effort}, nil
	case "script":
		absolute, err := filepath.Abs(configuration.agentCommand)
		if err != nil {
			return nil, err
		}
		return scriptAgent{command: absolute}, nil
	}
	return nil, fmt.Errorf("unknown agent %q", configuration.agent)
}

// runCommand runs one invocation with stdin closed, bounded output, and the
// given timeout; a non-zero exit is returned as the exit code with stdout and
// the bounded stderr, so the caller can say why the invocation failed.
func runCommand(ctx context.Context, root string, timeout time.Duration, name string, arguments ...string) ([]byte, int, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, "", fmt.Errorf("%s: %v after %s", name, err, timeout)
	}
	if timeout <= 0 {
		return nil, 0, "", fmt.Errorf("%s: %v after %s", name, context.DeadlineExceeded, timeout)
	}
	directory, err := filepath.Abs(root)
	if err != nil {
		return nil, 0, "", fmt.Errorf("%s: %v", name, err)
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
				return nil, 0, "", fmt.Errorf("%s: %v", name, err)
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
		return nil, 0, "", fmt.Errorf("%s: %v after %s", name, context.DeadlineExceeded, timeout)
	}
	if observation.Cancelled {
		return nil, 0, "", fmt.Errorf("%s: %v after %s", name, ctx.Err(), timeout)
	}
	stderr := strings.TrimSpace(string(observation.Stderr))
	if observation.Err != nil {
		return nil, 0, "", fmt.Errorf("%s: %v: %s", name, observation.Err, stderr)
	}
	if !observation.ExitObserved || !observation.WaitCompleted ||
		!observation.PipesDrained || !observation.OwnedProcessGroupCleanup {
		return nil, 0, "", fmt.Errorf("%s: invocation cleanup or exit observation is incomplete", name)
	}
	return observation.Stdout, observation.ExitStatus, stderr, nil
}

func truncate(text string, limit int) (string, bool) {
	if len(text) <= limit {
		return text, false
	}
	return text[:limit] + truncatedMarker, true
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

// readManifest reads the frozen set and refuses a patch whose bytes no longer
// match the digest the manifest recorded at selection time (CRT-V0-001).
func readManifest(path string) (*manifest, []byte, error) {
	raw, err := readBounded(path, maxTasksFile)
	if err != nil {
		return nil, nil, err
	}
	var set manifest
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	if set.Partition == "" || len(set.Tasks) == 0 {
		return nil, nil, fmt.Errorf("%s: a manifest needs a partition and at least one change", path)
	}
	seen := map[string]bool{}
	for _, item := range set.Tasks {
		if seen[item.ID] {
			return nil, nil, fmt.Errorf("%s: duplicate change id %q", path, item.ID)
		}
		seen[item.ID] = true
	}
	base := filepath.Dir(path)
	for _, item := range set.Tasks {
		patch, err := readPatch(base, item)
		if err != nil {
			return nil, nil, err
		}
		if digestOf(patch) != item.PatchSHA256 {
			return nil, nil, fmt.Errorf("%s: the patch bytes no longer match patch_sha256", item.ID)
		}
	}
	return &set, raw, nil
}

func readPatch(base string, item task) (string, error) {
	path := item.PatchPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, filepath.FromSlash(path))
	}
	data, err := readBounded(path, maxPatchBytes)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// changeMetadata carries the gold and the stem baseline into the report, so
// the scorer never needs the manifest again.
func changeMetadata(set *manifest) []changeMeta {
	changes := make([]changeMeta, 0, len(set.Tasks))
	for _, item := range set.Tasks {
		changes = append(changes, changeMeta{
			ID: item.ID, Commit: item.Commit, BaseCommit: item.BaseCommit,
			PatchSHA256: item.PatchSHA256, HunkCount: item.HunkCount,
			Gold: item.Gold.Paths, StemBaseline: item.StemBaseline,
		})
	}
	return changes
}

func prologueIdentity(arms []string) map[string]map[string]any {
	result := map[string]map[string]any{}
	for _, name := range arms {
		text := ""
		if name == "treatment" {
			text = treatmentPrologue
		}
		result[name] = map[string]any{"sha256": digestOf(text), "text": text}
	}
	return result
}

func digestOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func splitList(value string) []string {
	parts := []string{}
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func modelOrNot(configuration options) string {
	if configuration.agent == "script" {
		return notObserved
	}
	return configuration.model
}

func valueOrNot(value string) string {
	if value == "" {
		return notObserved
	}
	return value
}

func sortedStrings(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// --- checkpointing and reuse ---

func checkpointPath(output string) string { return output + ".partial.json" }

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

func assign(guard *sync.Mutex, write func()) {
	if guard != nil {
		guard.Lock()
		defer guard.Unlock()
	}
	write()
}

// reuseSource is a prior report's lanes keyed by change, arm, and repeat. A
// reply is reused only for a byte-identical prompt, and only for a lane that
// actually finished, so --resume reruns exactly what never completed.
type reuseSource struct {
	identity string
	lanes    map[string]*lane
}

func loadReuse(configuration options) (*reuseSource, error) {
	if configuration.reuse == "" {
		return nil, nil
	}
	document, err := loadReport(configuration.reuse)
	if err != nil {
		return nil, err
	}
	if document.Profile != profile || document.Model != modelOrNot(configuration) {
		return nil, fmt.Errorf("%s: a reused report must share the profile and model of this run", configuration.reuse)
	}
	return newReuseSource(document), nil
}

func newReuseSource(document *report) *reuseSource {
	source := &reuseSource{identity: document.ManifestSHA256, lanes: map[string]*lane{}}
	for _, item := range document.Lanes {
		source.lanes[laneKey(item)] = item
	}
	return source
}

func (source *reuseSource) merge(document *report) *reuseSource {
	other := newReuseSource(document)
	if source == nil {
		return other
	}
	for key, item := range other.lanes {
		if _, present := source.lanes[key]; !present {
			source.lanes[key] = item
		}
	}
	return source
}

func laneKey(item *lane) string {
	return item.ID + "\x00" + item.Arm + "\x00" + strconv.Itoa(item.Repeat)
}

func (source *reuseSource) apply(record *lane) bool {
	if source == nil {
		return false
	}
	prior := source.lanes[laneKey(record)]
	if prior == nil || laneFailed(prior) || prior.PromptSHA256 != record.PromptSHA256 || !observedNumber(prior.WallMs) {
		return false
	}
	record.WallMs, record.ExitCode, record.Tokens, record.ToolCalls = prior.WallMs, prior.ExitCode, prior.Tokens, prior.ToolCalls
	record.Reply, record.ReplyTruncated, record.CEM = prior.Reply, prior.ReplyTruncated, prior.CEM
	record.ReusedFrom = source.identity
	return true
}

func observedNumber(value any) bool {
	switch value.(type) {
	case int64, float64, int:
		return true
	}
	return false
}

func loadReport(path string) (*report, error) {
	data, err := readBounded(path, maxOutputBytes)
	if err != nil {
		return nil, err
	}
	var document report
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &document, nil
}

// agentExitError describes a lane whose agent process exited non-zero: the
// lane produced no observation, so it is errored rather than a total miss.
func agentExitError(code int, stderrTail string) string {
	message := fmt.Sprintf("agent exited %d", code)
	if stderrTail == "" {
		return message + " with no stderr"
	}
	if len(stderrTail) > maxAgentErrorBytes {
		stderrTail = stderrTail[len(stderrTail)-maxAgentErrorBytes:]
	}
	return message + ": " + stderrTail
}
