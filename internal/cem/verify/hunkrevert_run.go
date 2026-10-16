package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// hunkRevertOutputLimit bounds one go test -json stream; a larger stream is an
// infrastructure failure, never a partial verdict.
const hunkRevertOutputLimit = 32 << 20

// goTestProfileEnvironment and goTestProfileArgv are the fixed go-test runner
// profile of internal/testrunner/native (GOTOOLCHAIN=local, no proxy, no
// checksum database; test -json -count=1 -run ^(names)$ <one package>). A
// parity test pins them to native.Build so the two cannot drift.
var goTestProfileEnvironment = map[string]string{"GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off"}

func goTestProfileArgv(project string, names []string) []string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = regexp.QuoteMeta(name)
	}
	argv := []string{"test", "-json", "-count=1"}
	if len(names) > 0 {
		argv = append(argv, "-run", "^("+strings.Join(quoted, "|")+")$")
	}
	return append(argv, project)
}

// runOutcome is one decoded go test run.
type runOutcome struct {
	states        map[string]string // top-level test name -> pass, fail or skip
	buildFailed   bool
	packageFailed bool
	timedOut      bool
	err           error
}

// passed returns the names that passed in this run, in order.
func (o runOutcome) passed(names []string) []string {
	var out []string
	for _, name := range names {
		if o.states[name] == "pass" {
			out = append(out, name)
		}
	}
	return out
}

// notRunReason names why no named test passed at the target: a test that
// fails, or a package that does not build, without the revert is
// fails-without-revert (CEM-V1-089).
func (o runOutcome) notRunReason(names []string) string {
	switch {
	case o.timedOut:
		return RevertTimeout
	case o.err != nil:
		return RevertInfrastructure
	case o.buildFailed || o.packageFailed && len(o.states) == 0:
		return RevertFailsWithoutRevert
	}
	for _, name := range names {
		if o.states[name] == "fail" {
			return RevertFailsWithoutRevert
		}
	}
	for _, name := range names {
		if o.states[name] == "skip" {
			return RevertSkipped
		}
	}
	return RevertNotObserved
}

// goTestRunner executes the fixed profile in dir of the disposable copy, in
// its own process group, and decodes the -json stream itself. repository is
// the caller's repository, which no Go cache may point into.
func goTestRunner(repository string) testRunner {
	return func(ctx context.Context, tree, dir string, names []string) runOutcome {
		return goTestRun(ctx, repository, tree, dir, names)
	}
}

func goTestRun(ctx context.Context, repository, tree, dir string, names []string) runOutcome {
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return runOutcome{err: err}
	}
	command := exec.CommandContext(ctx, goBinary, goTestProfileArgv(".", names)...)
	command.Dir = filepath.Join(tree, filepath.FromSlash(dir))
	command.Env = goTestEnvironment(filepath.Dir(tree), repository)
	stdout := &cappedBuffer{limit: hunkRevertOutputLimit}
	command.Stdout = stdout
	command.Stderr = &cappedBuffer{limit: 64 << 10}
	groupreap.Contain(command)
	// The bound detects a descendant holding the output pipes, not a slow reader.
	command.WaitDelay = 10 * time.Second
	runErr := groupreap.Run(command)
	if ctx.Err() != nil {
		return runOutcome{timedOut: true}
	}
	if stdout.over {
		return runOutcome{err: errors.New("go test output exceeded its bound")}
	}
	outcome := decodeGoTest(stdout.data.Bytes())
	if outcome.err == nil && len(outcome.states) == 0 && !outcome.buildFailed && !outcome.packageFailed && runErr != nil {
		outcome.err = runErr
	}
	return outcome
}

// goTestEnvironment is the run's complete environment: the profile's three
// pins, a fixed locale and zone, GOWORK=off so no workspace outside the copy
// applies, GOENV=off so no user go env file redirects a cache or adds flags, a
// temporary directory inside the disposable workspace, and only the host
// variables go needs to find itself and its caches. A cache variable that
// points into the caller's repository is moved into the workspace, so a run
// never writes there.
func goTestEnvironment(workspace, repository string) []string {
	var environment []string
	for _, name := range []string{"PATH", "HOME", "GOROOT", "GOPATH", "GOMODCACHE", "GOCACHE", "SystemRoot", "USERPROFILE", "LOCALAPPDATA", "APPDATA"} {
		value, ok := os.LookupEnv(name)
		if !ok {
			continue
		}
		if name == "GOPATH" || name == "GOMODCACHE" || name == "GOCACHE" {
			for _, entry := range filepath.SplitList(value) {
				if insideDirectory(repository, entry) {
					value = filepath.Join(workspace, strings.ToLower(name))
					break
				}
			}
		}
		environment = append(environment, name+"="+value)
	}
	for _, name := range []string{"GOPROXY", "GOSUMDB", "GOTOOLCHAIN"} {
		environment = append(environment, name+"="+goTestProfileEnvironment[name])
	}
	tmp := filepath.Join(workspace, "tmp")
	return append(environment, "GOWORK=off", "GOENV=off", "LANG=C.UTF-8", "TZ=UTC", "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)
}

// insideDirectory reports whether candidate is directory or lies below it,
// comparing absolute, cleaned paths; a relative candidate counts as inside.
func insideDirectory(directory, candidate string) bool {
	if !filepath.IsAbs(candidate) {
		return true
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return true
	}
	relative, err := filepath.Rel(root, filepath.Clean(candidate))
	return err != nil || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// disposableWorkspace creates the disposable export directory both
// --hunk-revert and --guarding-tests use, under the system temporary
// directory, and refuses one that resolves inside the repository's worktree
// or Git directory (CEM-V1-250): a repository-local TMPDIR would otherwise
// place the export, and the tests' own writes, inside the repository the
// read command must leave untouched (product invariant 4).
func disposableWorkspace(ctx context.Context, repository, prefix string) (string, error) {
	temp, err := filepath.Abs(os.TempDir())
	if err == nil {
		temp, err = filepath.EvalSymlinks(temp)
	}
	if err != nil {
		return "", fmt.Errorf("temporary directory: %w", err)
	}
	// --show-cdup prints the worktree top relative to repository (an empty
	// line at the top), then the absolute common Git directory follows.
	output, err := gitOutput(ctx, repository, nil, "rev-parse", "--show-cdup", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) != 2 {
		return "", fmt.Errorf("repository roots: unexpected git rev-parse output %q", output)
	}
	top, err := filepath.Abs(filepath.Join(repository, lines[0]))
	if err != nil {
		return "", fmt.Errorf("repository roots: %w", err)
	}
	var roots []os.FileInfo
	for _, root := range []string{top, lines[1]} {
		info, err := os.Stat(root)
		if err != nil {
			return "", fmt.Errorf("repository roots: %w", err)
		}
		roots = append(roots, info)
	}
	// Containment is decided by file identity along the temporary
	// directory's ancestors, not by comparing path text, so a case alias on
	// a case-insensitive volume or a second spelling of a root is refused
	// too, and a root on another volume is never mistaken for an ancestor.
	for directory := temp; ; directory = filepath.Dir(directory) {
		if info, err := os.Stat(directory); err == nil {
			for _, root := range roots {
				if os.SameFile(root, info) {
					return "", fmt.Errorf("temporary directory %s lies inside the repository at %s", temp, directory)
				}
			}
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	return os.MkdirTemp(temp, prefix)
}

// timedOutPanic is the go test binary's own -timeout panic.
const timedOutPanic = "panic: test timed out after "

// decodeGoTest decodes a go test -json stream. Only a test-level terminal
// event sets a state; a build-fail event or a FailedBuild package marks the
// build failed; an undecodable stream is an error, never a verdict.
func decodeGoTest(stream []byte) runOutcome {
	outcome := runOutcome{states: map[string]string{}}
	decoder := json.NewDecoder(bytes.NewReader(stream))
	for {
		var event struct {
			Action, Package, ImportPath, Test, Output, FailedBuild string
		}
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			return outcome
		} else if err != nil {
			return runOutcome{err: err}
		}
		switch event.Action {
		case "start", "run", "pause", "cont", "output", "build-output", "bench":
		case "build-fail":
			outcome.buildFailed = true
		case "pass", "fail", "skip":
			if event.Test == "" {
				if event.Action == "fail" {
					outcome.packageFailed = true
				}
			} else if !strings.Contains(event.Test, "/") {
				outcome.states[event.Test] = event.Action
			}
		default:
			return runOutcome{err: fmt.Errorf("unknown go test action %q", event.Action)}
		}
		if event.FailedBuild != "" {
			outcome.buildFailed = true
		}
		if strings.Contains(event.Output, timedOutPanic) {
			outcome.timedOut = true
		}
	}
}

type cappedBuffer struct {
	data  bytes.Buffer
	limit int
	over  bool
}

func (b *cappedBuffer) Write(chunk []byte) (int, error) {
	if room := b.limit - b.data.Len(); room < len(chunk) {
		b.over = true
		if room > 0 {
			b.data.Write(chunk[:room])
		}
		return len(chunk), nil
	}
	return b.data.Write(chunk)
}

// exportTree writes the target commit's tree into directory from raw objects:
// git ls-tree lists it and git cat-file --batch streams the blobs, with no
// attribute, conversion or optional lock, so the caller's worktree, index and
// refs are only read. A symlink, a submodule or a path escaping the copy is
// refused rather than dropped, so the copy never silently differs.
func exportTree(ctx context.Context, repository, target, directory, tmp string) error {
	for _, dir := range []string{directory, tmp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	listing, err := gitOutput(ctx, repository, nil, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", target)
	if err != nil {
		return err
	}
	type file struct {
		id, target string
		mode       os.FileMode
	}
	var files []file
	var request bytes.Buffer
	if len(listing) == 0 {
		return errors.New("target tree is empty")
	}
	for _, entry := range bytes.Split(bytes.TrimSuffix(listing, []byte{0}), []byte{0}) {
		meta, name, ok := strings.Cut(string(entry), "\t")
		fields := strings.Fields(meta)
		cleaned := path.Clean(name)
		if !ok || len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") ||
			cleaned != name || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("target tree entry %q is outside the copy profile", name)
		}
		destination := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if fields[0] == "100755" {
			mode = 0o755
		}
		files = append(files, file{id: fields[2], target: destination, mode: mode})
		request.WriteString(fields[2] + "\n")
	}
	blobs, err := gitOutput(ctx, repository, request.Bytes(), "cat-file", "--batch")
	if err != nil {
		return err
	}
	for _, f := range files {
		header, rest, found := bytes.Cut(blobs, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !found || len(fields) != 3 || fields[0] != f.id || fields[1] != "blob" {
			return fmt.Errorf("git cat-file answered %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size >= len(rest) || rest[size] != '\n' {
			return fmt.Errorf("git cat-file answered %q", header)
		}
		data := rest[:size]
		blobs = rest[size+1:]
		out, err := os.OpenFile(f.target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.mode)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// gitOutput runs one read-only git command against repository with a
// sanitized environment and no optional locks.
func gitOutput(ctx context.Context, repository string, stdin []byte, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repository, "--no-optional-locks"}, args...)...)
	var environment []string
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE", "HOME"} {
		if value, ok := os.LookupEnv(name); ok {
			environment = append(environment, name+"="+value)
		}
	}
	command.Env = append(environment, "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_ASKPASS=")
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	stdout := &cappedBuffer{limit: 1 << 30}
	stderr := &cappedBuffer{limit: 64 << 10}
	command.Stdout, command.Stderr = stdout, stderr
	groupreap.Contain(command)
	command.WaitDelay = 10 * time.Second
	if err := groupreap.Run(command); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.data.String()))
	}
	if stdout.over {
		return nil, fmt.Errorf("git %s output exceeded its bound", args[0])
	}
	return stdout.data.Bytes(), nil
}
