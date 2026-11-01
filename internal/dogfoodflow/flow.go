// Package dogfoodflow runs the daily dogfood change, check and seal steps of
// docs/DOGFOOD.md inside the Corvint binary, so any Git repository can run them
// without Corvint's scripts, VERSION file or source tree (DCW-V0-020 and
// DCW-V0-021). The report, evidence and message bytes are those the former
// script/dogfood-change.sh and script/dogfood-check.sh produced.
package dogfoodflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// ErrInterrupted reports that the context ended before the flow finished; the
// flow wrote no further report and removed its private temporary state.
var ErrInterrupted = errors.New("dogfood flow interrupted")

// Command runs one public Corvint command against root, like
// localcompletion.PublicCommand.
type Command func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int

// Runner is one Corvint executable identity: Path is recorded as the step argv[0]
// and hashed as a verifier identity, Run executes a command with it.
type Runner struct {
	Path string
	Run  Command
}

// Exec runs every command as the executable at path. On cancellation the child
// receives SIGTERM and is waited for, as the former scripts' traps did.
func Exec(path string) Runner {
	return Runner{Path: path, Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		command := exec.CommandContext(ctx, path, append([]string{"--root", root}, args...)...)
		command.Env = childEnvironment(ctx)
		command.Stdout, command.Stderr = nullable(stdout), nullable(stderr)
		command.Cancel = func() error {
			if err := command.Process.Signal(syscall.SIGTERM); err != nil {
				return command.Process.Kill()
			}
			return nil
		}
		return exitStatus(command.Run())
	}}
}

// stableReuseKey carries the private run directory whose path Exec passes to
// every child as lrfrepo.StableReuseEnv (OCM-V0-027).
type stableReuseKey struct{}

// withStableReuse returns ctx naming dir as this run's stable verdict store.
func withStableReuse(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, stableReuseKey{}, dir)
}

// childEnvironment is the caller's environment with LC_ALL=C, without any
// inherited stable verdict store, plus this run's own store when it has one:
// a verdict is reused only within the run that made it.
func childEnvironment(ctx context.Context) []string {
	prefix := lrfrepo.StableReuseEnv + "="
	environment := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, prefix) {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, "LC_ALL=C")
	if dir, _ := ctx.Value(stableReuseKey{}).(string); dir != "" {
		environment = append(environment, prefix+dir)
	}
	return environment
}

func nullable(writer io.Writer) io.Writer {
	if writer == io.Discard {
		return nil
	}
	return writer
}

// exitStatus maps a finished child to the status Bash reported for it.
func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return 127
	}
	if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return exit.ExitCode()
}

// stop unwinds a flow to its boundary: code is the exit status, or -1 when the
// context ended.
type stop struct{ code int }

// flow holds the state both the change and the check resolve first.
type flow struct {
	operationUnlock func()
	ctx             context.Context
	prefix          string
	root            string
	stderr          io.Writer
	gitDir          string
	base            string
	target          string
	anchorObserved  bool
	anchorMergeBase string
	// gitOutputLimit overrides maxGitOutputBytes when positive (tests only).
	gitOutputLimit int
	// replays memoizes each regeneration replay by commit (DCW-V0-401).
	replays     map[string]regeneration
	replayCount int
}

// boundary runs body, then cleanup, and converts an unwind into the result.
func boundary(ctx context.Context, body func() int, cleanup func()) (code int, err error) {
	defer func() {
		cleanup()
		recovered := recover()
		if recovered == nil {
			return
		}
		unwound, ok := recovered.(stop)
		if !ok {
			panic(recovered)
		}
		code, err = unwound.code, nil
		if unwound.code < 0 {
			code, err = 0, ErrInterrupted
		}
	}()
	code = body()
	if ctx.Err() != nil {
		return 0, ErrInterrupted
	}
	return code, nil
}

func (f *flow) exit(code int) {
	panic(stop{code})
}

func (f *flow) checkpoint() {
	if err := dogfoodoperation.Check(f.ctx); err != nil {
		f.refuse(err.Error())
	}
	if f.ctx.Err() != nil {
		panic(stop{-1})
	}
}

func (f *flow) refuse(reason string) {
	f.say("%s: REFUSE %s\n", f.prefix, reason)
	f.exit(2)
}

func (f *flow) say(format string, values ...any) {
	_, _ = fmt.Fprintf(f.stderr, format, values...)
}

// maxGitOutputBytes caps the stdout every Git read of the flow keeps
// (DCW-V0-064). A read whose output would exceed it keeps none of it: git
// refuses the flow with gitOutputOverBound, and boundedGit returns the read
// as unbounded so its caller records NOT_OBSERVED.
const maxGitOutputBytes = 16 << 20

// gitOutputOverBound is the refusal reason of a Git read over maxGitOutputBytes.
const gitOutputOverBound = "git-output-over-bound"

// git runs one Git read in the repository under Core's sanitized Git
// environment, with no credential helper and no transport, and returns its
// stdout and exit status; its stderr reaches the caller unless quiet. As in
// Core's hardened reader, a repository-configured fsmonitor hook or untracked
// cache never runs or writes. An output over maxGitOutputBytes refuses the
// flow, so no caller reads a truncated or unbounded result.
func (f *flow) git(quiet bool, args ...string) (string, int) {
	f.checkpoint()
	output, status, bounded := f.readGit(f.gitCommand(quiet, args...))
	if !bounded {
		f.refuse(gitOutputOverBound)
	}
	return output, status
}

// boundedGit is the quiet git read whose overflow its caller reports itself
// (NOT_OBSERVED) instead of refusing the flow.
func (f *flow) boundedGit(args ...string) (string, int, bool) {
	f.checkpoint()
	return f.readGit(f.gitCommand(true, args...))
}

func (f *flow) gitCommand(quiet bool, args ...string) *exec.Cmd {
	command := exec.CommandContext(f.ctx, "git", append([]string{"--no-optional-locks", "-C", f.root, "-c", "credential.helper=",
		"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false"}, args...)...)
	command.Env = gokernel.SanitizedGitEnvironment()
	if !quiet {
		command.Stderr = f.stderr
	}
	return command
}

// readGit is the flow's one Git stdout reader: it keeps at most the flow's
// output limit (maxGitOutputBytes unless a test lowers gitOutputLimit), and
// on overflow stops reading, discards what it kept and reports false.
func (f *flow) readGit(command *exec.Cmd) (string, int, bool) {
	limit := f.gitOutputLimit
	if limit <= 0 {
		limit = maxGitOutputBytes
	}
	stdout := &cappedOutput{limit: limit}
	command.Stdout = stdout
	status := exitStatus(command.Run())
	f.checkpoint()
	if stdout.overflow {
		return "", status, false
	}
	return stdout.text.String(), status, true
}

// cappedOutput keeps at most limit bytes and refuses the write that exceeds it.
type cappedOutput struct {
	text     strings.Builder
	limit    int
	overflow bool
}

func (output *cappedOutput) Write(data []byte) (int, error) {
	if output.text.Len()+len(data) > output.limit {
		output.overflow = true
		return 0, errors.New("git output exceeds its bound")
	}
	return output.text.Write(data)
}

// gitValue is a Git command substitution whose failure exits 2.
func (f *flow) gitValue(args ...string) string {
	output, status := f.git(false, args...)
	if status != 0 {
		f.exit(2)
	}
	return chomp(output)
}

func (f *flow) gitSucceeds(args ...string) bool {
	_, status := f.git(true, args...)
	return status == 0
}

// requireRoot refuses a root below the worktree top level, where the private
// .corvint paths would not be the repository's (DCW-V0-021).
func (f *flow) requireRoot() {
	if f.gitValue("rev-parse", "--show-prefix") != "" {
		f.refuse("not-repository-root")
	}
}

// resolveAnchor enforces the optional corvint.dogfood.anchor contract.
func (f *flow) resolveAnchor() {
	configured, status := f.git(false, "config", "--local", "--get-all", "corvint.dogfood.anchor")
	configured = chomp(configured)
	if status == 1 {
		return
	}
	if status != 0 || configured == "" {
		f.refuse("anchor-ref-unavailable")
	}
	if strings.Contains(configured, "\n") {
		f.refuse("multiple-anchor-refs")
	}
	merges, status := f.git(false, "merge-base", "--all", configured, f.target)
	merges = chomp(merges)
	if status != 0 {
		f.refuse("anchor-ref-unavailable")
	}
	if merges == "" || strings.Contains(merges, "\n") {
		f.refuse("anchor-merge-base-ambiguous")
	}
	f.anchorObserved = true
	f.anchorMergeBase = merges
	if f.base != f.anchorMergeBase {
		// DCW-V0-171: name the anchored base, so a lane that merged its
		// mainline forward past an unmoved BASE learns the base to rerun with.
		f.say("%s: REFUSE base-not-anchored\n", f.prefix)
		f.say("  fix: BASE %s is not git merge-base %s HEAD; rerun with BASE=%s, which binds only the commits HEAD adds to the anchor (docs/DOGFOOD.md \"Produce the CEM\", DCW-V0-171)\n", f.base, configured, f.anchorMergeBase)
		f.exit(2)
	}
}

// maxMergedForwardScan bounds the first-parent merges mergedForward reads.
const maxMergedForwardScan = 64

// mergedForward returns the newest merge on HEAD's first-parent path from
// BASE whose merged parent descends from BASE and is not a seal commit: the
// shape of a lane that merged its mainline forward past an unmoved BASE
// (DCW-V0-171). A merged seal commit is a sealed lane or stacked predecessor,
// which DCW-V0-042 and DCW-V0-113 route instead. Empty when none is found.
func (f *flow) mergedForward() (merge, parent string) {
	merges, status := f.git(true, "rev-list", "--first-parent", "--merges", fmt.Sprintf("--max-count=%d", maxMergedForwardScan), f.base+".."+f.target)
	if status != 0 {
		return "", ""
	}
	for _, candidate := range nonEmpty(strings.Split(chomp(merges), "\n")) {
		merged, status := f.git(true, "rev-parse", "--verify", "-q", candidate+"^2")
		merged = chomp(merged)
		if status != 0 || merged == f.base || !f.gitSucceeds("merge-base", "--is-ancestor", f.base, merged) || f.isSealCommit(merged) {
			continue
		}
		return candidate, merged
	}
	return "", ""
}

// noteMergedForward prints the DCW-V0-171 note before the expensive passes
// when no anchor is configured and BASE..HEAD merged a descendant of BASE.
// No implicit anchor is selected, so it is a note, not a refusal.
func (f *flow) noteMergedForward() {
	if f.anchorObserved {
		return
	}
	merge, merged := f.mergedForward()
	if merge == "" {
		return
	}
	f.say("%s: NOTE merged-forward merge=%s merged=%s\n", f.prefix, merge, merged)
	f.say("  fix: if %s is a mainline commit (git merge-base --is-ancestor %s origin/main), BASE %s..HEAD binds the mainline's commits as this change's and can exceed the bounds; rerun dogfood-change, dogfood-check and dogfood-seal with BASE=%s (git merge-base origin/main HEAD after the last merge); to refuse this before the passes run, set git config --local corvint.dogfood.anchor refs/remotes/origin/main (docs/DOGFOOD.md \"Produce the CEM\", DCW-V0-171)\n", merged, merged, f.base, merged)
}

// isSealCommit reports a one-parent commit that only renames that parent's
// CEM, unchanged, to .corvint/changes/<parent>.cem.json (DOGFOOD-013).
func (f *flow) isSealCommit(commit string) bool {
	_, err := ResolveSealBinding(f.ctx, f.root, commit)
	return err == nil
}

// inheritedCEM is the unsealed .corvint/change.cem.json BASE tracks, if any:
// its blob, the commit that last wrote it, and the sealed path that archives
// it under that commit's name (DCW-V0-043). path is empty when no commit
// reachable from BASE wrote it.
type inheritedCEM struct{ blob, writer, path string }

func (f *flow) inheritedCEM() inheritedCEM {
	blob := f.gitRevision(f.base + ":.corvint/change.cem.json")
	if blob == "" {
		return inheritedCEM{}
	}
	writer, _ := f.git(true, "log", "-1", "--format=%H", f.base, "--", ".corvint/change.cem.json")
	inherited := inheritedCEM{blob: blob, writer: chomp(writer)}
	if fullRevision.MatchString(inherited.writer) {
		inherited.path = ".corvint/changes/" + inherited.writer + ".cem.json"
	}
	return inherited
}

// archives reports whether target keeps the inherited CEM's blob at its
// sealed path.
func (f *flow) archives(inherited inheritedCEM, target string) bool {
	return inherited.path != "" && f.gitRevision(target+":"+inherited.path) == inherited.blob
}

// archiveRecovery is the exact command that archives the inherited CEM in its
// own commit, byte for byte, whatever the worktree's sidecar now holds.
func (inherited inheritedCEM) archiveRecovery() string {
	path := inherited.path
	return fmt.Sprintf("mkdir -p .corvint/changes && git cat-file blob %s > %s && git add %s && git commit -q -m \"chore: archive inherited change evidence\" -- %s",
		inherited.blob, path, path, path)
}

var codePattern = regexp.MustCompile(`^.*"code"[[:space:]]*:[[:space:]]*"([A-Za-z0-9_-]*)".*$`)

// failureReason names a failed step by the first "code" its stderr reports.
func failureReason(stderr []byte, status int) string {
	reason, _ := firstCapture(codePattern, stderr)
	if reason == "" && status == 1 {
		return "not-ready"
	}
	if reason == "" {
		return "exit-" + strconv.Itoa(status)
	}
	return reason
}

// firstCapture is `sed -n 's/RE/\1/p' | head -1`: the capture on the first
// matching line.
func firstCapture(pattern *regexp.Regexp, data []byte) (string, bool) {
	for _, line := range textLines(data) {
		if match := pattern.FindStringSubmatch(line); match != nil {
			return match[1], true
		}
	}
	return "", false
}

// allCaptures is `$(sed -n 's/RE/\1/p')`: every matching line's capture.
func allCaptures(pattern *regexp.Regexp, data []byte) string {
	captures := []string{}
	for _, line := range textLines(data) {
		if match := pattern.FindStringSubmatch(line); match != nil {
			captures = append(captures, match[1])
		}
	}
	return chomp(strings.Join(captures, "\n"))
}

// textLines splits like sed, awk and rg: a final line without LF still counts.
func textLines(data []byte) []string {
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" && len(data) == 0 {
		return nil
	}
	return strings.Split(text, "\n")
}

// readLines splits like `while IFS= read -r`: only LF-terminated lines count.
func readLines(data []byte) []string {
	text := string(data)
	end := strings.LastIndexByte(text, '\n')
	if end < 0 {
		return nil
	}
	return strings.Split(text[:end], "\n")
}

// hasLine reports whether any line equals value, like an anchored rg -q.
func hasLine(data []byte, value string) bool {
	for _, line := range textLines(data) {
		if line == value {
			return true
		}
	}
	return false
}

// firstLine is `sed -n 1p`: the first line, LF-terminated, or nothing.
func firstLine(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	line, _, _ := bytes.Cut(data, []byte("\n"))
	return append(line[:len(line):len(line)], '\n')
}

// requireBaseAncestor refuses a BASE that head does not contain before any
// patch is read (DCW-V0-946): BASE..head would then carry the reverse of every
// BASE commit head lacks, so a reseal would bind a revert of main. The refusal
// names their merge-base, the BASE head actually branched from.
func (f *flow) requireBaseAncestor(base, head string) {
	if f.gitSucceeds("merge-base", "--is-ancestor", base, head) {
		return
	}
	merge, status := f.git(true, "merge-base", base, head)
	merge = chomp(merge)
	if status != 0 || merge == "" {
		merge = "none"
	}
	f.say("%s: REFUSE base-not-ancestor base=%s merge-base=%s\n", f.prefix, base, merge)
	f.say("  HEAD %s does not contain BASE, so BASE..HEAD would carry the reverse of every BASE commit HEAD lacks and a reseal would bind a revert of them; nothing was read or written (DCW-V0-946)\n", head)
	if merge == "none" {
		f.say("  fix: BASE and HEAD share no history; pass a BASE that HEAD contains, then rerun\n")
	} else {
		f.say("  fix: rerun with BASE=%s, the commit HEAD branched from, or rebase this branch onto %s first (git rebase %s), then rerun with BASE=%s\n", merge, base, base, base)
	}
	f.exit(2)
}

func chomp(value string) string {
	return strings.TrimRight(value, "\n")
}

func readFile(name string) []byte {
	data, _ := os.ReadFile(name)
	return data
}

// readPrefix is `head -c limit`.
func readPrefix(name string, limit int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit))
}

func isRegular(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsRegular()
}

func isSymlink(name string) bool {
	info, err := os.Lstat(name)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func exists(name string) bool {
	_, err := os.Lstat(name)
	return err == nil
}

func hasContent(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Size() > 0
}

// removeFile is `rm -f`: absence succeeds and a directory fails.
func removeFile(name string) error {
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("is a directory")
	}
	return os.Remove(name)
}

// writePrivate is `(umask 077; ... > name)`.
func writePrivate(name string, data []byte) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	return errors.Join(err, file.Close())
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(name string) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// hasForbiddenControl reports a byte outside TAB, LF, printable ASCII and 0x80-0xff.
func hasForbiddenControl(data []byte) bool {
	for _, value := range data {
		if value != '\t' && value != '\n' && (value < 0x20 || value == 0x7f) {
			return true
		}
	}
	return false
}

// anyLine reports whether any line matches pattern, like `rg -q`.
func anyLine(pattern *regexp.Regexp, data []byte) bool {
	for _, line := range textLines(data) {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

var impactRefusal = regexp.MustCompile(`^\{"code": "([a-z-]+)", "error": "[A-Za-z0-9 ._/:()-]+", "ok": false\}$`)

// impactSnapshotMissNotice is the one diagnostic stderr line `corvint impact`
// writes before its result when no index snapshot matches the tree and the
// in-memory build took at least a second: the IDX-SNAP-V0-115 form, or the
// IDX-SNAP-V0-102 form an older binary writes, never a mix of the two.
var impactSnapshotMissNotice = regexp.MustCompile(`^corvint impact: no index snapshot matches this tree, so this call built the full index in memory \([0-9]+ ms\); run \["corvint","--root","(?:[^"\\]|\\.)*","index"(?:\] once per checkout to reuse it|,"--if-stale"\] once per committed tree, which every worktree of this repository then reuses)$`)

// withoutImpactSnapshotNotice is an impact refusal's stderr without its one
// leading IDX-SNAP-V0-102 snapshot-miss notice, when the stderr is exactly that
// notice and one more LF-terminated line; any other stderr is returned whole.
// The notice is diagnostic and never evidence, and its duration differs on
// every run, so the abstention shape and the check rerun compare the stderr
// without it (DCW-V0-377).
func withoutImpactSnapshotNotice(stderr []byte) []byte {
	if bytes.Count(stderr, []byte("\n")) != 2 || stderr[len(stderr)-1] != '\n' {
		return stderr
	}
	first, rest, _ := bytes.Cut(stderr, []byte("\n"))
	if !impactSnapshotMissNotice.Match(first) {
		return stderr
	}
	return rest
}

// impactAbstentionStderr reports whether an impact refusal's stderr has the
// admitted abstention shape: exactly one line, optionally preceded by exactly
// one snapshot-miss notice (DCW-V0-025, DCW-V0-377).
func impactAbstentionStderr(stderr []byte) bool {
	return bytes.Count(withoutImpactSnapshotNotice(stderr), []byte("\n")) == 1
}

// impactAbstentions are the impact refusals the daily path keeps as typed,
// visible scope limits of the native Go profile rather than failed steps
// (DCW-V0-025); each stays its own row reason.
var impactAbstentions = map[string]bool{
	"unsupported-impact-range":      true,
	"unsupported-impact-repository": true,
	"unsupported-impact-path":       true,
}

// impactDeadlineEnvelope is the exact refusal `corvint impact` writes when one
// of its Git phases overruns the contextindex 30-second Git deadline, which has
// no override (DCW-V0-075).
const impactDeadlineEnvelope = "{\"error\": \"Git repository index exceeded its deadline\", \"ok\": false}\n"

// impactDeadlineReason is the visible, non-blocking row reason of an impact
// step whose one bounded retry also overran that deadline (DCW-V0-075).
const impactDeadlineReason = "impact-deadline-exceeded"

// impactDeadlineOverrun reports an impact run that refused with exactly the
// deadline envelope, exit 2 and empty stdout.
func impactDeadlineOverrun(status int, output, errorFile string) bool {
	info, err := os.Lstat(output)
	emptyStdout := err == nil && info.Mode().IsRegular() && info.Size() == 0
	return status == 2 && emptyStdout && string(readFile(errorFile)) == impactDeadlineEnvelope
}

func (f *flow) acquireOperation() {
	if f.operationUnlock != nil {
		return
	}
	ctx, unlock, err := dogfoodoperation.Acquire(f.ctx, f.gitDir)
	if err != nil {
		f.refuse(err.Error())
	}
	f.ctx, f.operationUnlock = ctx, unlock
}
func (f *flow) releaseOperation() {
	if f.operationUnlock != nil {
		f.operationUnlock()
		f.operationUnlock = nil
	}
}
