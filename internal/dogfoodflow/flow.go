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
		command.Env = append(os.Environ(), "LC_ALL=C")
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
	ctx             context.Context
	prefix          string
	root            string
	stderr          io.Writer
	gitDir          string
	base            string
	target          string
	anchorObserved  bool
	anchorMergeBase string
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

// git runs one Git command in the repository and returns its stdout and exit
// status; its stderr reaches the caller unless quiet.
func (f *flow) git(quiet bool, args ...string) (string, int) {
	var stdout bytes.Buffer
	command := exec.CommandContext(f.ctx, "git", append([]string{"-C", f.root}, args...)...)
	command.Env = append(os.Environ(), "LC_ALL=C", "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=")
	command.Stdout = &stdout
	if !quiet {
		command.Stderr = f.stderr
	}
	status := exitStatus(command.Run())
	f.checkpoint()
	return stdout.String(), status
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
		f.refuse("base-not-anchored")
	}
}

// isSealCommit reports a one-parent commit that only renames that parent's
// CEM, unchanged, to .corvint/changes/<parent>.cem.json (DOGFOOD-013).
func (f *flow) isSealCommit(commit string) bool {
	parent, status := f.git(true, "rev-parse", "--verify", "-q", commit+"^1")
	if status != 0 {
		return false
	}
	parent = chomp(parent)
	if f.gitSucceeds("rev-parse", "--verify", "-q", commit+"^2") {
		return false
	}
	renamed, _ := f.git(false, "diff-tree", "-r", "-M", "--no-commit-id", "--name-status", parent, commit)
	return chomp(renamed) == "R100\t.corvint/change.cem.json\t.corvint/changes/"+parent+".cem.json"
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

// impactAbstentions are the impact refusals the daily path keeps as typed,
// visible scope limits of the native Go profile rather than failed steps
// (DCW-V0-025); each stays its own row reason.
var impactAbstentions = map[string]bool{
	"unsupported-impact-range":      true,
	"unsupported-impact-repository": true,
	"unsupported-impact-path":       true,
}
