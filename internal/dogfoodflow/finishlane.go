package dogfoodflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// FinishLaneOptions are the inputs of one finish-lane run: the change passes
// take Change, whose Citations names the plan, and the check and seal take
// Check (DCW-V0-060).
type FinishLaneOptions struct {
	Change ChangeOptions
	Check  CheckOptions
	// Preflight runs only the input checks and the prepare pass, the
	// `dogfood inputs-check` pre-flight: it binds, commits and seals nothing
	// (DCW-V0-380).
	Preflight bool
}

// finishLaneSidecar is the one path the bind loop writes and commits.
const finishLaneSidecar = ".corvint/change.cem.json"

// bindSubject is the bind commit's message, as docs/DOGFOOD.md §4 writes it.
const bindSubject = "chore: bind change evidence"

type finishLane struct {
	flow
	options FinishLaneOptions
	stdout  io.Writer
	// missing keeps every row a change pass did not produce, prefixed by the
	// finish-lane step that ran it (DCW-V0-062).
	missing []string
	notes   map[string]bool
	// step is the finish-lane step running now, named if the run is
	// interrupted (DCW-V0-067).
	step string
	// clock times every step and command (DCW-V0-115).
	clock *laneClock
	// inputs are the supplied input files' content digests, taken before
	// prepare and compared after every pass (DCW-V0-378).
	inputs []laneInput
	// stageFiles are the staged route's plans of a plan over 256 rows, the
	// last one replayed by the rebind, and stages their rows (DCW-V0-446).
	stageFiles []string
	stages     [][]string
	// stage labels the running stage in kept rows and step lines.
	stage string
	// cause is the last pass's first failing row, which a stop names before
	// the rows that follow from it (DCW-V0-806).
	cause *step
}

// laneInput is one supplied input file and its content digest at run start.
type laneInput struct {
	flag, path string
	digest     [sha256.Size]byte
	present    bool
}

// FinishLane runs the docs/DOGFOOD.md §4 bind loop end to end on a committed
// change: prepare, bind with the plan, commit the sidecar, rebind, check and
// seal (DCW-V0-060). It stops at the first step that does not succeed, with that
// step's own output, and a rerun continues from the bind commit (DCW-V0-061).
func FinishLane(ctx context.Context, options FinishLaneOptions, stdout, stderr io.Writer) (int, error) {
	clock := newLaneClock()
	options.Change.Steps = clock.wrap(options.Change.Steps)
	options.Check.BaseVerifier = clock.wrap(options.Check.BaseVerifier)
	options.Check.TreeVerifier = clock.wrap(options.Check.TreeVerifier)
	if options.Check.Override != nil {
		override := clock.wrap(*options.Check.Override)
		options.Check.Override = &override
	}
	prefix := "dogfood-finish-lane"
	if options.Preflight {
		prefix = "dogfood-inputs-check"
	}
	run := &finishLane{flow: flow{ctx: ctx, prefix: prefix, root: options.Change.Root, stderr: stderr}, options: options, stdout: stdout, notes: map[string]bool{}, clock: clock}
	defer run.releaseOperation()
	code, err := boundary(ctx, run.run, func() {})
	if errors.Is(err, ErrInterrupted) {
		run.interrupted()
	}
	run.writeTimings()
	return code, err
}

// interrupted names the step an ended context stopped and every reason kept
// so far, so an interrupted run never ends silently (DCW-V0-067).
func (l *finishLane) interrupted() {
	step := l.step
	if step == "" {
		step = "start"
	}
	l.line("%s INTERRUPTED", step)
	l.reportMissing()
	l.say("%s: INTERRUPTED %s\n  fix: %s\n", l.prefix, step, l.recovery(step))
}

// recovery is the exact command sequence that resumes an interrupted run.
// Before the bind commit a rerun prepares again; from the bind commit on it
// resumes there once the sidecar, and a stopped seal's staged rename, are
// restored. A stopped pass's dogfood-report.json is never the checked one, so
// only the finished rerun's check decides it (DCW-V0-116).
func (l *finishLane) recovery(step string) string {
	base := l.base
	if base == "" {
		base = l.options.Change.Base
	}
	rerun := "rerun corvint dogfood finish-lane --base " + base + " (or make dogfood-finish BASE=" + base + ") with the same inputs"
	switch step {
	case "rebind", "check":
		return "git restore --source=HEAD --staged --worktree -- " + finishLaneSidecar + ", then " + rerun + "; it resumes from the bind commit " + l.target
	case "seal":
		return "unless HEAD is already the seal commit, git restore --source=HEAD --staged --worktree -- " + finishLaneSidecar + " .corvint/changes/" + l.target + ".cem.json; then " + rerun + "; it resumes from the bind commit " + l.target + " or reports the seal"
	default:
		return rerun + "; it prepares the map again, and only the rerun's check decides the report"
	}
}

func (l *finishLane) run() int {
	l.requireRoot()
	l.requireTrustedAncestry()
	l.gitDir = l.gitValue("rev-parse", "--absolute-git-dir")
	// One operation lock spans every step; each nested pass shares it.
	l.acquireOperation()
	l.base = l.gitValue("rev-parse", l.options.Change.Base+"^{commit}")
	l.target = l.gitValue("rev-parse", "HEAD^{commit}")
	l.requireBaseAncestor(l.base, l.target)
	if l.options.Change.Citations == "" {
		l.say("%s: REFUSE citation-plan-not-provided\n", l.prefix)
		l.say("  fix: pass --citations FILE, a TSV plan with one row per hunk of %s (docs/DOGFOOD.md §4)\n", finishLaneSidecar)
		l.exit(2)
	}
	// A plan drafted before HEAD moved is refreshed before its inputs are
	// snapshotted, so the run checks and binds the refreshed file (DCW-V0-947).
	l.refreshMovedPlan()
	l.requireVerifyInput()
	l.requireIntentsInput()
	l.snapshotInputs()
	// A sealed HEAD is admitted like its bind commit, so a rerun with another
	// base or a dirty tree is refused rather than reported as done.
	sealed := l.isSealCommit(l.target)
	bind := l.target
	if sealed {
		bind = l.gitValue("rev-parse", l.target+"^1")
	}
	resume := l.isBindCommit(bind)
	change := bind
	if resume {
		change = l.gitValue("rev-parse", bind+"^1")
	}
	if change == l.base || !l.gitSucceeds("merge-base", "--is-ancestor", l.base, change) {
		l.say("%s: REFUSE head-not-ahead-of-base\n", l.prefix)
		l.say("  fix: commit the change on a branch whose history contains %s, then rerun corvint dogfood finish-lane --base %s\n", l.base, l.base)
		l.exit(2)
	}
	l.requireCleanWorktree(resume || sealed, sealed)
	if sealed {
		l.line("seal SKIP already-sealed sealed=.corvint/changes/%s.cem.json", bind)
		return 0
	}
	l.requireOutcomeInputs()
	l.requireFreshWorktree(l.base, l.gitDir, !l.options.Preflight)
	// A plan over 256 rows runs as the staged route's plans (DCW-V0-446).
	files, stages, cleanup := l.stagePlan()
	defer cleanup()
	l.stageFiles, l.stages = files, stages
	if l.options.Preflight {
		// The pre-flight checks the inputs the bind pass reads; from the
		// bind commit on that pass has run, so there is nothing to check.
		if resume {
			l.line("inputs-check SKIP bind-commit=%s", l.target)
			return 0
		}
		// A refused prepare stops with its map in place for the next pass to
		// remove; the pre-flight leaves no map either way.
		defer func() { _ = removeFile(l.root + "/" + finishLaneSidecar) }()
		l.removePreflight()
		l.recordPreflight(l.prepare())
		l.line("inputs-check PASS nothing was bound or committed")
		return 0
	}
	if resume {
		l.removePreflight()
		l.line("prepare SKIP bind-commit=%s", l.target)
		l.line("bind SKIP bind-commit=%s", l.target)
		l.line("commit SKIP bind-commit=%s", l.target)
	} else {
		// A passing inputs-check over the same inputs already ran this exact
		// prepare pass (DCW-V0-887).
		if !l.reusePreflight() {
			l.prepare()
		}
		l.bind()
		l.commitBind()
	}
	l.settleDeferralRecord()
	l.rebind()
	return l.checkAndSeal()
}

// snapshotInputs records the content digest of every supplied input file, so a
// file another lane or a later edit rewrites mid-run is refused by name rather
// than surfacing as a drift the change itself did not cause (DCW-V0-378).
func (l *finishLane) snapshotInputs() {
	for _, input := range []struct{ flag, path string }{{"--citations", l.options.Change.Citations}, {"--intents", l.options.Change.IntentsFile}, {"--verify", l.options.Change.VerifyFile}} {
		if input.path == "" {
			continue
		}
		digest, present := fileDigest(input.path)
		l.inputs = append(l.inputs, laneInput{flag: input.flag, path: input.path, digest: digest, present: present})
	}
}

// fileDigest streams a regular file through SHA-256 in constant memory. A
// missing or non-regular path (a FIFO would block the open) is recorded as
// absent without being opened, so its validation stays with the step that
// reads it.
func fileDigest(path string) ([sha256.Size]byte, bool) {
	var digest [sha256.Size]byte
	if !isRegular(path) {
		return digest, false
	}
	file, err := os.Open(path)
	if err != nil {
		return digest, false
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return digest, false
	}
	copy(digest[:], hash.Sum(nil))
	return digest, true
}

// requireInputsUnchanged refuses the run after step when a supplied input
// file's content differs from its digest at run start, naming each changed
// file (DCW-V0-378).
func (l *finishLane) requireInputsUnchanged(step string) {
	changed := false
	for _, input := range l.inputs {
		digest, present := fileDigest(input.path)
		if digest == input.digest && present == input.present {
			continue
		}
		if !changed {
			l.say("%s: REFUSE input-changed-during-run step=%s\n", l.prefix, step)
		}
		changed = true
		l.say("  changed: %s %s\n", input.flag, input.path)
	}
	if !changed {
		return
	}
	l.say("  fix: keep this run's input files in a lane-unique directory no other lane or step rewrites (docs/DOGFOOD.md §4), then rerun corvint dogfood finish-lane --base %s\n", l.options.Change.Base)
	l.line("%s FAIL exit=2 input-changed", step)
	l.reportMissing()
	l.exit(2)
}

// requireVerifyInput refuses, before prepare runs any step, a verify input the
// bind's local-outcome row would refuse, with that row's reason (DCW-V0-092).
func (l *finishLane) requireVerifyInput() {
	verify, reason := verifyInput(l.options.Change)
	detail := ""
	if reason == "" && len(verify) > 0 {
		if err := validateVerify(verify); err != nil {
			reason, detail = verifyFailureReason(err), err.Error()
		}
	}
	if reason == "" {
		l.requireVerifyRunSelectsTest()
		return
	}
	l.say("%s: REFUSE %s\n", l.prefix, reason)
	if fix := quotedRunFix(l.options.Change.VerifyFile); reason == "unsupported-verify-syntax" && fix != "" {
		// DCW-V0-403: an unquoted -run alternation gets one line naming its
		// admitted quoted form, not the grammar remediation.
		l.say("  fix: --verify %s, then rerun corvint dogfood finish-lane --base %s\n", fix, l.options.Change.Base)
		l.exit(2)
		return
	}
	if fix := tildeFix(l.options.Change.VerifyFile); reason == "unsupported-verify-syntax" && fix != "" {
		// DCW-V0-905: a ~ path is never expanded silently; the fix gives the
		// line with the absolute home directory.
		l.say("  fix: --verify %s, then rerun corvint dogfood finish-lane --base %s\n", fix, l.options.Change.Base)
	}
	if line := verifyRefusalLine(l.options.Change.VerifyFile); detail != "" && line > 0 {
		l.say("  input: --verify %s line %d\n", l.options.Change.VerifyFile, line)
	}
	if detail != "" {
		l.say("  %s\n", oneLine(detail))
	}
	if _, text := refusedVerifyLineOf(l.options.Change); text != "" {
		if why := quotedRunRefusal(text); why != "" {
			l.say("  refused: %s\n", why)
		}
	}
	l.say("  fix: pass a readable verify file of at most %d bytes with one shell-free command per line (no globs or operators, and no quotes except around one -run alternation), then rerun corvint dogfood finish-lane --base %s\n", maxVerifyFileBytes, l.options.Change.Base)
	if reason == "unsupported-verify-syntax" {
		// DCW-V0-170, amended by DCW-V0-384 and DCW-V0-801: name the admitted
		// quoted -run alternation, the line's own quoted form when only the
		// quotes are missing, and the accepted forms for any other regex.
		for _, suggestion := range quotedRunSuggestions(l.options.Change) {
			l.say("  quoted form: %s\n", suggestion)
		}
		l.say("  accepted form: one quoted -run alternation of test names, -run 'TestA|TestB', -run '^(TestA|TestB)$' or -run '^TestA$|^TestB$' (single or double quotes, or attached as -run='TestA|TestB'), is admitted as one argv word (DCW-V0-384, DCW-V0-801), as are one quoted name such as -run '^TestA$', one name with a bracket class such as -run 'TestX88[45]_' (DCW-V0-991) and the whole package as -run '.' or -run '.*' (DCW-V0-904), and leading NAME='value' or NAME=\"value\" assignments such as GOFLAGS='-count=1 -v' go test ./pkg (DCW-V0-804); an unquoted alternation or any other quoted regex is refused; write one command per test name (go test -run TestA ./pkg, then go test -run TestB ./pkg), use a shared name prefix (-run TestA matches TestA*), or commit a script and list it as sh path/to/script.sh\n")
	}
	l.exit(2)
}

// requireVerifyRunSelectsTest refuses, before prepare runs any step, a
// verify line whose go test -run pattern selects no test function in the
// package directories it names: go test exits 0 with "no tests to run", so
// the CEM would record verification that ran nothing (DCW-V0-922).
func (l *finishLane) requireVerifyRunSelectsTest() {
	empty, found := emptyRunLineOf(l.root, l.options.Change)
	if !found {
		return
	}
	source := "--verify " + l.options.Change.VerifyFile
	if l.options.Change.VerifyFile == "" {
		source = "inline verify"
	}
	l.say("%s: REFUSE verify-run-matches-no-test\n", l.prefix)
	l.say("  input: %s line %d: %s\n", source, empty.line, oneLine(empty.text))
	l.say("  refused: -run %s matches no Test, Example or Fuzz function in the _test.go files of %s, so go test would pass having run no test (DCW-V0-922)\n", empty.pattern, empty.packages)
	l.say("  fix: name a test that exists in that package (grep -n '^func Test' in its _test.go files), or drop -run, then rerun corvint dogfood finish-lane --base %s\n", l.options.Change.Base)
	l.exit(2)
}

// requireIntentsInput refuses, before prepare runs any step, an --intents
// manifest the bind's ocm-aggregate row would refuse for its grammar, naming
// the first faulty line and, for an ordering fault, the sort that fixes it
// (DCW-V0-882). Unset intents stay with the bind pass.
func (l *finishLane) requireIntentsInput() {
	source := l.options.Change.IntentsFile
	if source == "" {
		return
	}
	_, paths, reason, issue := readIntentManifest(source)
	if reason == "" && len(paths) > maxAggregateScopes {
		reason = "intent-scope-limit"
	}
	if reason == "" {
		return
	}
	l.say("%s: REFUSE %s\n", l.prefix, reason)
	l.say("  input: --intents %s\n", source)
	if issue != "" {
		l.say("  %s\n", issue)
	}
	if strings.Contains(issue, "does not sort after") {
		l.say("  fix: LC_ALL=C sort -u -o %s %s, then rerun corvint dogfood finish-lane --base %s\n", shellWord(source), shellWord(source), l.options.Change.Base)
		l.exit(2)
	}
	l.say("  fix: %s, then rerun corvint dogfood finish-lane --base %s\n", intentScopeHint, l.options.Change.Base)
	l.exit(2)
}

// requireOutcomeInputs refuses, before prepare and before any bind or commit,
// a run without a verification command or an outcome: the rebind's
// local-outcome row records both, so without them it can only refuse
// outcome-input-not-provided after the bind commit exists (DCW-V0-800). The
// refusal names both inputs, as the rebind's fix line does.
func (l *finishLane) requireOutcomeInputs() {
	verify, _ := verifyInput(l.options.Change)
	// DCW-V0-960: the full pass's ocm-aggregate refuses missing-intent-scope
	// only after the bind commit, so the pre-flight refuses an absent manifest.
	intents := !l.options.Preflight || l.options.Change.IntentsFile != ""
	if !intents {
		l.say("%s: REFUSE intent-scope-not-provided\n", l.prefix)
		l.say("  fix: pass --intents FILE (or DOGFOOD_INTENTS_FILE; make takes INTENTS=<intents.txt>): the sorted spec paths that govern the change, or the one line #no-intent-declared when none does (DCW-V0-024); then rerun corvint dogfood inputs-check --base %s\n", l.options.Change.Base)
	}
	var missing []string
	if len(verify) == 0 {
		missing = append(missing, "--verify")
	}
	if l.options.Change.Outcome == "" {
		missing = append(missing, "--outcome")
	}
	if len(missing) == 0 {
		if !intents {
			l.say("  nothing was prepared, bound or committed\n")
			l.exit(2)
		}
		return
	}
	l.say("%s: REFUSE outcome-input-not-provided missing=%s\n", l.prefix, strings.Join(missing, ","))
	l.say("  fix: pass --verify FILE (or DOGFOOD_VERIFY_FILE, one verification command per line) and --outcome passed|failed|blocked (or DOGFOOD_OUTCOME); make dogfood-finish takes them as VERIFY=<verify.txt> OUTCOME=passed|failed|blocked; then rerun corvint dogfood finish-lane --base %s\n", l.options.Change.Base)
	l.say("  the rebind records the local outcome from both inputs; nothing was prepared, bound or committed\n")
	l.exit(2)
}

// verifyRefusalLine is the 1-based line of the verify file whose command
// alone the same validator refuses, or 0 when no single line is refused, as
// for a file over the command bound (DCW-V0-379).
func verifyRefusalLine(path string) int {
	data, err := readPrefix(path, maxVerifyFileBytes)
	if path == "" || err != nil {
		return 0
	}
	line, _ := refusedVerifyLine(data)
	return line
}

// refusedVerifyLine returns the 1-based number and text of the first line of
// data that the recorder refuses, or 0 and "" when every line is admitted.
func refusedVerifyLine(data []byte) (int, string) {
	for index, line := range strings.Split(string(data), "\n") {
		if strings.TrimLeft(line, " \t\n\v\f\r") != "" && validateVerify(verifyLineArguments(line)) != nil {
			return index + 1, line
		}
	}
	return 0, ""
}

// refusedVerifyLineOf is refusedVerifyLine over the verify file, else the
// DOGFOOD_VERIFY text.
func refusedVerifyLineOf(options ChangeOptions) (int, string) {
	text := []byte(options.Verify)
	if options.VerifyFile != "" {
		data, err := readPrefix(options.VerifyFile, maxVerifyFileBytes)
		if err != nil {
			return 0, ""
		}
		text = data
	}
	return refusedVerifyLine(text)
}

// line writes one step line to stdout.
func (l *finishLane) line(format string, values ...any) {
	_, _ = fmt.Fprintf(l.stdout, "%s: %s\n", l.prefix, fmt.Sprintf(format, values...))
}

// isBindCommit reports a one-parent commit titled as the bind commit that
// changes only the sidecar, and any deferral record the bind pass wrote
// (DCW-V0-370): a previous run, or the manual recipe, already committed it,
// so a rerun never commits it again (DCW-V0-061).
func (l *finishLane) isBindCommit(commit string) bool {
	if l.gitSucceeds("rev-parse", "--verify", "-q", commit+"^2") || !l.gitSucceeds("rev-parse", "--verify", "-q", commit+"^1") {
		return false
	}
	if l.gitValue("log", "-1", "--format=%s", commit) != bindSubject {
		return false
	}
	changed, _ := l.git(false, "diff-tree", "-r", "--no-renames", "--no-commit-id", "--name-only", commit+"^1", commit)
	sidecar := false
	for _, path := range nonEmpty(strings.Split(changed, "\n")) {
		switch {
		case path == finishLaneSidecar:
			sidecar = true
		case !strings.HasPrefix(path, workflow.DeferredIntentsDir+"/"):
			return false
		}
	}
	return sidecar
}

// requireCleanWorktree reads the worktree through Core's hardened probe, as
// the check does (DCW-V0-045). Before the bind commit only the sidecar a
// stopped run left may differ; at the bind or seal commit nothing may.
func (l *finishLane) requireCleanWorktree(committed, sealed bool) {
	repository, err := gokernel.ProbeRepositoryContext(l.ctx, l.root)
	l.checkpoint()
	if err != nil {
		l.say("%s: REFUSE worktree-status-unavailable\n  %s\n", l.prefix, oneLine(err.Error()))
		l.say("  fix: remove the repository configuration the reason names, or run finish-lane in a plain clone (git clone --no-hardlinks)\n")
		l.exit(2)
	}
	if repository.DirtyPathCount == 0 {
		return
	}
	sidecarOnly, _ := gokernel.CanonicalJSON([]string{finishLaneSidecar})
	if !committed && repository.DirtyPathCount == 1 && repository.DirtyPathsSHA == sha256Hex(sidecarOnly) {
		return
	}
	l.say("%s: REFUSE dirty-worktree\n", l.prefix)
	// Naming the paths saves a status round trip; the sample is sorted and
	// bounded, and each path is quoted so no byte reaches the terminal raw (DCW-V0-397).
	more := ""
	if extra := repository.DirtyPathCount - min(repository.DirtyPathCount, gokernel.DirtySampleSize); extra > 0 {
		more = fmt.Sprintf(" and %d more", extra)
	}
	l.say("  uncommitted: %s%s\n", repository.DirtySample, more)
	switch {
	case sealed:
		l.say("  fix: HEAD is sealed; commit or remove every uncommitted path, then rerun corvint dogfood finish-lane --base %s\n", l.base)
	case committed:
		// A seal whose commit failed leaves its staged rename; this restores
		// the bind commit's sidecar and drops the rename (DCW-V0-061).
		l.say("  fix: git restore --source=HEAD --staged --worktree -- %s .corvint/changes/%s.cem.json, commit or remove every other change, then rerun corvint dogfood finish-lane --base %s\n", finishLaneSidecar, l.target, l.base)
	default:
		l.say("  fix: commit the change (finish-lane binds only committed work) and remove every other uncommitted path, then rerun corvint dogfood finish-lane --base %s\n", l.base)
	}
	l.exit(2)
}

// pass runs one dogfood change pass in-process under this run's operation
// lock. Its stderr is kept: a stopping step prints it unchanged, an accepted
// one only its NOTE lines.
func (l *finishLane) pass(name, citations string) (int, *change, []byte) {
	l.step = name
	options := l.options.Change
	options.Citations = citations
	var captured bytes.Buffer
	run := &change{flow: flow{ctx: l.ctx, prefix: "dogfood-change", root: options.Root, stderr: &captured}, options: options, scope: passScopes[name], nested: true}
	code, err := boundary(l.ctx, run.run, run.cleanup)
	run.releaseOperation()
	l.cause = nil
	for _, row := range run.rows {
		if row.status != "PRODUCED" {
			l.missing = append(l.missing, name+l.stage+" "+row.name+" "+row.status+" "+row.reason)
		}
		if l.cause == nil && run.rowFailing(row) {
			l.cause = &row
		}
	}
	if err != nil {
		// The interrupted pass's own output and rows are kept, as a stopping
		// one's are.
		_, _ = l.stderr.Write(captured.Bytes())
		l.exit(-1)
	}
	return code, run, captured.Bytes()
}

// passScopes narrows the passes whose rows the rebind recomputes: prepare
// reads only the prepared map, bind only what the bind commit needs; the
// rebind runs every step (DCW-V0-115).
var passScopes = map[string]passScope{"prepare": passPrepare, "bind": passBind, "rebind": passFull}

// accept keeps an accepted pass's NOTE lines, each once per run. A later
// pass repeats a note with only its target or counts changed, so a note is
// keyed by its name, status and reason (DCW-V0-405).
func (l *finishLane) accept(output []byte) {
	for _, text := range textLines(output) {
		if !strings.HasPrefix(text, "dogfood-change: NOTE ") {
			continue
		}
		if key := noteKey(text); !l.notes[key] {
			l.notes[key] = true
			l.say("%s\n", text)
		}
	}
}

// noteKey is a NOTE line's first five fields: the prefix, NOTE, the step
// name, its status and its reason.
func noteKey(text string) string {
	fields := strings.Fields(text)
	if len(fields) > 5 {
		fields = fields[:5]
	}
	return strings.Join(fields, " ")
}

// stop ends the run at step: the step's own output is printed unchanged, then
// the step line and every reason kept so far (DCW-V0-061, DCW-V0-062).
func (l *finishLane) stop(name string, code int, output []byte) {
	// A changed input explains the failure better than the step's own
	// refusal, such as intent-scope-drift (DCW-V0-378).
	l.requireInputsUnchanged(name)
	for _, text := range textLines(output) {
		// A note an earlier pass printed is not repeated (DCW-V0-405).
		if !strings.HasPrefix(text, "dogfood-change: NOTE ") || !l.notes[noteKey(text)] {
			l.say("%s\n", text)
		}
	}
	if code == 0 {
		code = 1
	}
	// The first failing row is the originating refusal; an ocm-aggregate
	// intent-scope-drift after a refused ocm-prepare only follows from it
	// (DCW-V0-806).
	if l.cause != nil {
		l.line("CAUSE %s%s %s %s", name, l.stage, l.cause.name, l.cause.reason)
		if name != "prepare" && isRegular(l.root+"/"+finishLaneSidecar) {
			_, _ = fmt.Fprintf(l.stdout, "  recover: this pass left the uncommitted %s, which the next finish-lane run removes before it prepares again when it is the only uncommitted path; fix the cause above, then rerun the same command\n", finishLaneSidecar)
		}
	}
	l.line("%s FAIL exit=%d", name, code)
	l.reportMissing()
	l.exit(code)
}

func (l *finishLane) reportMissing() {
	l.line("NOT_PRODUCED rows=%d", len(l.missing))
	for _, row := range l.missing {
		_, _ = fmt.Fprintf(l.stdout, "  %s\n", row)
	}
}

// removeSidecar is the recipe's rm -f before a pass, so a pass never resumes
// a map an earlier pass or plan cited (DCW-V0-019).
func (l *finishLane) removeSidecar(name string) {
	if err := removeFile(l.root + "/" + finishLaneSidecar); err != nil {
		l.say("%s: REFUSE sidecar-not-removed\n  %s\n", l.prefix, oneLine(err.Error()))
		l.line("%s FAIL exit=2", name)
		l.exit(2)
	}
}

// prepare is pass 0: it prepares the map whose hunk IDs the plan names. Only
// a refusal or an unprepared map stops it; the rows it cannot produce without
// a plan or a committed sidecar are the bind's to decide.
func (l *finishLane) prepare() []string {
	defer l.timed("prepare")()
	if len(l.stageFiles) == 0 {
		line := fmt.Sprintf("prepare PASS hunks=%d", l.preparePass(l.options.Change.Citations, false))
		l.line("%s", line)
		return []string{line}
	}
	// Each stage's plan is prechecked against a freshly prepared map, so a
	// refused row of any stage stops before the first bind (DCW-V0-446).
	lines := []string{}
	for k, file := range l.stageFiles {
		l.stage = fmt.Sprintf(" stage=%d/%d", k+1, len(l.stageFiles))
		line := fmt.Sprintf("prepare PASS hunks=%d%s", l.preparePass(file, k == 0), l.stage)
		l.line("%s", line)
		lines = append(lines, line)
	}
	l.stage = ""
	return lines
}

// preparePass runs one prepare pass with citations and returns the prepared
// map's hunk count; first also runs the whole-plan stage checks on that map.
func (l *finishLane) preparePass(citations string, first bool) int {
	l.removeSidecar("prepare")
	// The plan is prechecked against this prepared map, so a row cem cite
	// would refuse, or a plan or manifest the bind would refuse before any
	// cite, stops here, before the bind pass (DCW-V0-335, DCW-V0-379).
	code, run, output := l.pass("prepare", citations)
	prepared := code != 2 && isRegular(l.root+"/"+finishLaneSidecar)
	for _, row := range run.rows {
		if (row.name == "cem-prepare" || row.name == citePrecheckStep || row.name == inputsPrecheckStep) && row.status != "PRODUCED" {
			prepared = false
		}
	}
	if !prepared {
		l.stop("prepare", code, output)
	}
	l.requireInputsUnchanged("prepare")
	data := readFile(l.root + "/" + finishLaneSidecar)
	if first {
		l.requireStageableMap(l.stages, data)
	}
	l.accept(output)
	l.removeSidecar("prepare")
	return len(mapHunks(data))
}

// bind cites the plan into a freshly prepared map. The only row it may leave
// is the recorder's record-index-failed, which the uncommitted sidecar causes
// and the bind commit clears (DCW-V0-015).
func (l *finishLane) bind() {
	defer l.timed("bind")()
	citations, last := l.options.Change.Citations, ""
	if n := len(l.stageFiles); n > 0 {
		for k, file := range l.stageFiles[:n-1] {
			l.stage = fmt.Sprintf(" stage=%d/%d", k+1, n)
			l.bindStage(file)
		}
		citations, l.stage = l.stageFiles[n-1], fmt.Sprintf(" stage=%d/%d", n, n)
		last = l.stage
	}
	code, run, output := l.pass("bind", citations)
	if code == 1 {
		for _, row := range run.rows {
			if run.rowFailing(row) && row != (step{"local-outcome", "NOT_PRODUCED", "record-index-failed"}) {
				l.stop("bind", code, output)
			}
		}
	} else if code != 0 {
		l.stop("bind", code, output)
	}
	l.requireInputsUnchanged("bind")
	l.accept(output)
	l.line("bind PASS%s", last)
	l.stage = ""
}

// bindStage cites one earlier stage's plan into the retained map, which cem
// prepare resumes, as the staged route's successive passes do. The pass is
// incomplete by design, since unknown hunks remain until the last stage, so
// only a refused prepare, input check or cite stops it (DCW-V0-446).
func (l *finishLane) bindStage(file string) {
	code, run, output := l.pass("bind", file)
	if code != 0 && code != 1 {
		l.stop("bind", code, output)
	}
	cited := false
	for _, row := range run.rows {
		switch row.name {
		case "cem-prepare", "cem-cite", citePrecheckStep, inputsPrecheckStep:
			if row.status != "PRODUCED" {
				l.stop("bind", code, output)
			}
			cited = cited || row.name == "cem-cite"
		}
	}
	if !cited {
		l.stop("bind", code, output)
	}
	l.requireInputsUnchanged("bind")
	l.accept(output)
	l.line("bind PASS%s", l.stage)
}

// commitBind commits only the sidecar, and the deferral record the bind pass
// wrote or replaced (DCW-V0-370), with the caller's Git identity, signing and
// hooks, as the recipe's git commit does.
func (l *finishLane) commitBind() {
	defer l.timed("commit")()
	l.step = "commit"
	paths := []string{finishLaneSidecar}
	if records, _ := l.git(true, "status", "--porcelain", "--untracked-files=all", "--", workflow.DeferredIntentsDir+"/"); strings.TrimSpace(records) != "" {
		paths = append(paths, workflow.DeferredIntentsDir+"/")
	}
	if len(paths) == 1 && l.gitSucceeds("diff", "--quiet", "HEAD", "--", finishLaneSidecar) && l.gitSucceeds("cat-file", "-e", "HEAD:"+finishLaneSidecar) {
		l.line("commit SKIP sidecar-unchanged")
		return
	}
	if l.gitPassthrough(append([]string{"add", "-A", "--"}, paths...)...) != 0 || l.gitPassthrough(append([]string{"commit", "-q", "-m", bindSubject, "--"}, paths...)...) != 0 {
		l.line("commit FAIL")
		l.reportMissing()
		l.exit(2)
	}
	l.target = l.gitValue("rev-parse", "HEAD^{commit}")
	l.line("commit PASS bind=%s", l.target)
}

// gitPassthrough runs one Git write in the caller's environment, as the
// seal's does; its output reaches stderr so stdout keeps only step lines.
func (l *finishLane) gitPassthrough(args ...string) int {
	command := exec.CommandContext(l.ctx, "git", append([]string{"-C", l.root}, args...)...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	command.Stdout, command.Stderr = l.stderr, l.stderr
	status := exitStatus(command.Run())
	l.checkpoint()
	return status
}

// bindCarriesRecord reports whether the bind commit adds or changes a
// deferral record beside the sidecar (DCW-V0-370).
func (l *finishLane) bindCarriesRecord() bool {
	changed, status := l.git(false, "diff-tree", "-r", "--no-renames", "--no-commit-id", "--name-only", l.target+"^1", l.target)
	if status != 0 {
		return false
	}
	for _, path := range nonEmpty(strings.Split(changed, "\n")) {
		if strings.HasPrefix(path, workflow.DeferredIntentsDir+"/") {
			return true
		}
	}
	return false
}

// settleDeferralRecord folds the deferral record's own hunk into the bind
// commit (DCW-V0-941, V1-3036). The record is a hunk of the map at the bind
// commit that the bind pass could not see, so the rebind would prepare and
// cite it into an uncommitted sidecar and refuse. One bind-scope pass with the
// same plan settles the map; when only the sidecar changed, the bind commit
// is amended with it, so it still changes only the sidecar and the record.
func (l *finishLane) settleDeferralRecord() {
	if !l.bindCarriesRecord() {
		return
	}
	defer l.timed("settle")()
	citations := l.options.Change.Citations
	if n := len(l.stageFiles); n > 0 {
		citations = l.stageFiles[n-1]
	}
	// The settle pass refuses its OCM rows on the uncommitted sidecar by
	// design; the rebind decides every row, so none is listed from here.
	listed := len(l.missing)
	code, run, output := l.pass("bind", citations)
	l.missing = l.missing[:listed]
	l.step = "settle"
	if code != 0 && code != 1 {
		l.stop("settle", code, output)
	}
	for _, row := range run.rows {
		if (row.name == "cem-prepare" || row.name == "cem-cite" || strings.HasPrefix(row.name, deferStep)) && row.status != "PRODUCED" {
			l.stop("settle", code, output)
		}
	}
	l.requireInputsUnchanged("settle")
	changed, _ := l.git(true, "status", "--porcelain", "--untracked-files=all")
	paths := nonEmpty(strings.Split(changed, "\n"))
	if len(paths) == 0 {
		l.line("settle SKIP map-unchanged")
		return
	}
	if len(paths) != 1 || len(paths[0]) < 4 || paths[0][3:] != finishLaneSidecar {
		l.stop("settle", 1, append(output, []byte("dogfood-finish-lane: settle changed more than the sidecar:\n"+changed)...))
	}
	if l.gitPassthrough("add", "--", finishLaneSidecar) != 0 || l.gitPassthrough("commit", "-q", "--amend", "--no-edit", "--", finishLaneSidecar) != 0 {
		l.line("settle FAIL")
		l.reportMissing()
		l.exit(2)
	}
	l.target = l.gitValue("rev-parse", "HEAD^{commit}")
	l.line("settle PASS bind=%s", l.target)
}

// rebind reruns the bind on the committed sidecar; it must complete.
func (l *finishLane) rebind() {
	defer l.timed("rebind")()
	// A staged plan's rebind replays its last stage on the fully cited map,
	// as the staged route's last-plan rerun does (DCW-V0-446).
	citations := l.options.Change.Citations
	if n := len(l.stageFiles); n > 0 {
		citations = l.stageFiles[n-1]
	}
	code, _, output := l.pass("rebind", citations)
	if code != 0 {
		l.stop("rebind", code, output)
	}
	l.requireInputsUnchanged("rebind")
	l.accept(output)
	l.line("rebind PASS")
}

// checkAndSeal runs the seal, which runs the check first, and ends with the
// check's own SUMMARY and verdict lines and the seal line.
func (l *finishLane) checkAndSeal() int {
	defer l.timed("check-seal")()
	l.step = "check"
	var verdict bytes.Buffer
	code, err := Seal(l.ctx, l.options.Check, &verdict, l.stderr)
	checked := hasLine(verdict.Bytes(), "dogfood-check: PASS")
	if err != nil {
		l.sealInterrupted(verdict.Bytes())
	}
	switch {
	case code == 0:
		l.line("check PASS")
		l.line("seal PASS seal=%s", l.gitValue("rev-parse", "HEAD^{commit}"))
		defer l.reportReview()
	case checked:
		l.line("check PASS")
		l.line("seal FAIL exit=%d", code)
	default:
		l.line("check FAIL exit=%d", code)
	}
	l.reportMissing()
	_, _ = l.stdout.Write(verdict.Bytes())
	l.reportRuns()
	return code
}

// dogfoodChangeRuns is the run log script/dogfood-change.sh appends one line
// to per invocation, under the Git directory's corvint evidence directory
// (ASD-V0-907).
const dogfoodChangeRuns = "dogfood-change-runs.tsv"

// reportRuns prints how many times dogfood-change ran in this worktree, the
// count the lane batch measures (ASD-V0-907, V1-1629), or NOT_OBSERVED when
// the log is absent or unreadable.
func (l *finishLane) reportRuns() {
	count := "NOT_OBSERVED"
	if data, err := readPrefix(filepath.Join(l.gitDir, "corvint", dogfoodChangeRuns), 1<<20); err == nil {
		count = strconv.Itoa(len(textLines(data)))
	}
	l.say("%s: RUNS dogfood-change=%s log=%s\n", l.prefix, count, filepath.Join(l.gitDir, "corvint", dogfoodChangeRuns))
}

// reportReview names the exact reviewer report the seal bound and says that
// its independent review and the native completion are still to come, so a
// review is never implied as done (DCW-V0-908, V1-3188).
func (l *finishLane) reportReview() {
	report := filepath.Join(l.root, ".corvint", "dogfood-report.json")
	digest := "NOT_OBSERVED"
	if sum, ok := fileDigest(report); ok {
		digest = fmt.Sprintf("%x", sum)
	}
	l.say("%s: REVIEW report=%s sha256=%s reviewed=NOT_OBSERVED\n", l.prefix, report, digest)
	l.say("  next: an independent review of this exact report, then the native completion; finish-lane records neither (DCW-V0-908)\n")
}

// sealInterrupted unwinds an interrupted check and seal. An interruption after
// the check passed stopped the seal, so the step is named seal; the verdict
// lines captured so far are kept (DCW-V0-067).
func (l *finishLane) sealInterrupted(verdict []byte) {
	if hasLine(verdict, "dogfood-check: PASS") {
		l.step = "seal"
	}
	_, _ = l.stdout.Write(verdict)
	l.exit(-1)
}
