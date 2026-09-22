// Package worksource qualifies the read-only source shared by the work observer
// and the repository-owned producer. It does not authorize execution.
package worksource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// PlatformPath is the fixed VPO-V0-022 executable search path.
func PlatformPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin", nil
	case "linux":
		return "/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin", nil
	default:
		return "", errors.New("unsupported work source platform")
	}
}

func newSource() (*Source, error) { return newSourceWithScratch("") }

func newSourceWithScratch(scratch string) (*Source, error) {
	fixed, err := PlatformPath()
	if err != nil {
		return nil, err
	}
	// An explicitly rebound caller is not the ordinary worktree this profile admits.
	for _, name := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE"} {
		if _, present := os.LookupEnv(name); present {
			return nil, fmt.Errorf("unsupported ambient %s", name)
		}
	}
	var executable string
	for _, directory := range filepath.SplitList(fixed) {
		candidate := filepath.Join(directory, "git")
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			executable, err = filepath.EvalSymlinks(candidate)
			break
		}
	}
	if executable == "" || err != nil {
		return nil, errors.New("fixed-path Git unavailable")
	}
	if runtime.GOOS == "darwin" {
		executable = bypassAppleGitShim(executable, appleDeveloperLink)
	}
	// Ordinary acquisition never consults ambient TMPDIR. The explicit scratch
	// seam is only for a private parent already owned by the observer/script.
	parent := "/tmp"
	if scratch != "" {
		parent, err = filepath.EvalSymlinks(scratch)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Lstat(parent)
		if statErr != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
			return nil, errors.New("source scratch is not a private directory")
		}
	}
	temporary, err := os.MkdirTemp(parent, "corvint-work-source-")
	if err != nil {
		return nil, err
	}
	source := &Source{GitPath: executable, temporary: temporary, objects: make(map[string]object)}
	for _, name := range []string{"home", "tmp"} {
		if err := os.Mkdir(filepath.Join(temporary, name), 0700); err != nil {
			source.Close()
			return nil, err
		}
	}
	source.GitEnvironment = []string{"PATH=" + fixed, "LANG=C", "LC_ALL=C", "TZ=UTC", "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "HOME=" + filepath.Join(temporary, "home"), "TMPDIR=" + filepath.Join(temporary, "tmp")}
	return source, nil
}

// appleDeveloperLink is the root-owned xcode-select choice the shim reads when
// DEVELOPER_DIR is absent, which the sanitized environment guarantees.
const appleDeveloperLink = "/var/db/xcode_select_link"

// bypassAppleGitShim replaces Apple's /usr/bin/git shim with the developer
// directory's real Git. Under a fresh HOME the shim can re-resolve its toolchain
// cache and warn on stderr, which the qualified boundary refuses. Without a
// regular executable developer Git the shim stays, still under stderr rejection.
func bypassAppleGitShim(executable, link string) string {
	if executable != "/usr/bin/git" {
		return executable
	}
	developer, err := os.Readlink(link)
	if err != nil || !filepath.IsAbs(developer) {
		return executable
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(developer, "usr", "bin", "git"))
	if err != nil {
		return executable
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return executable
	}
	return resolved
}

func (source *Source) Close() error {
	if source == nil || source.temporary == "" {
		return nil
	}
	return os.RemoveAll(source.temporary)
}

// ResolveRoot resolves an ordinary repository using the same closed Git boundary.
func ResolveRoot(ctx context.Context, directory string) (string, error) {
	return ResolveRootWithScratch(ctx, directory, "")
}

// ResolveRootWithScratch uses an explicitly owned scratch parent; it never reads
// ambient TMPDIR. The caller retains cleanup ownership of that parent.
func ResolveRootWithScratch(ctx context.Context, directory, scratch string) (string, error) {
	if err := ValidateScratchLocation(ctx, directory, scratch); err != nil {
		return "", err
	}
	source, err := newSourceWithScratch(scratch)
	if err != nil {
		return "", err
	}
	defer source.Close()
	source.Root = directory
	raw, err := source.Git(ctx, 8192, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(strings.TrimSuffix(string(raw), "\n"))
}

type limitedOutput struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	overflow bool
}

// These finite liveness guards also cap resource retention if repository inputs
// make Git or a pipe holder hang. They are variables only for deterministic tests.
var (
	gitCommandTimeout   = 10 * time.Minute
	gitCommandWaitDelay = time.Minute
)

func (output *limitedOutput) Write(raw []byte) (int, error) {
	if len(raw) > output.limit-output.buffer.Len() {
		output.overflow = true
		output.cancel()
		return 0, errors.New("Git output limit")
	}
	return output.buffer.Write(raw)
}

// Git runs fixed, caller-owned arguments with bounded output and pipe ownership.
// Callers must not pass a mutating operation against the qualified source.
func (source *Source) Git(ctx context.Context, limit int, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()
	if len(arguments) > 0 && arguments[0] == "status" {
		temporary := ""
		for _, value := range source.GitEnvironment {
			if strings.HasPrefix(value, "TMPDIR=") {
				temporary = strings.TrimPrefix(value, "TMPDIR=")
			}
		}
		run := func(ctx context.Context, root string, outputLimit int, args ...string) ([]byte, error) {
			private := *source
			private.Root, private.GitDir = root, ""
			return private.gitRaw(ctx, outputLimit, args...)
		}
		raw, err := gitstatus.StatusIn(ctx, source.Root, temporary, limit, run, arguments...)
		if err != nil {
			return nil, fmt.Errorf("qualified Git acquisition failed (status): %w", err)
		}
		return raw, nil
	}
	return source.gitRaw(ctx, limit, arguments...)
}

func (source *Source) gitRaw(ctx context.Context, limit int, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	args := []string{"--no-optional-locks", "--no-replace-objects", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.excludesFile=", "-c", "core.hooksPath=" + os.DevNull, "-c", "credential.helper=", "-c", "submodule.recurse=false", "-c", "diff.external=", "-c", "core.quotePath=false"}
	if source.GitDir != "" {
		args = append(args, "--git-dir="+source.GitDir, "--work-tree="+source.Root)
	}
	args = append(args, arguments...)
	command := exec.CommandContext(ctx, source.GitPath, args...)
	command.Dir = source.Root
	command.Env = append([]string(nil), source.GitEnvironment...)
	if gitstatus.Isolated(ctx) {
		command.Env = append(command.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(source.Root))
	}
	command.WaitDelay = gitCommandWaitDelay
	setupGitProcess(command)
	stdout := &limitedOutput{limit: limit, cancel: cancel}
	stderr := &limitedOutput{limit: 64 << 10, cancel: cancel}
	command.Stdout, command.Stderr = stdout, stderr
	defer cleanupGitProcess(command)
	err := command.Run()
	if err != nil || ctx.Err() != nil || stdout.overflow || stderr.overflow || stderr.buffer.Len() != 0 {
		return nil, fmt.Errorf("qualified Git acquisition failed (%s): %w", strings.Join(arguments, " "), errors.Join(err, ctx.Err(), errors.New(stderr.buffer.String())))
	}
	return bytes.Clone(stdout.buffer.Bytes()), nil
}
