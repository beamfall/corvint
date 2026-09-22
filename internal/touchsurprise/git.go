package touchsurprise

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const maxGitOutputBytes = 8 << 20

// boundedWriter is the bounded sink internal/contextindex/git.go uses, kept
// here so no Git read of this verb can grow without limit.
type boundedWriter struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (writer *boundedWriter) Write(payload []byte) (int, error) {
	if writer.buffer.Len()+len(payload) > writer.limit {
		writer.exceeded = true
		return len(payload), nil
	}
	return writer.buffer.Write(payload)
}

// sanitizedGitEnvironment mirrors internal/contextindex/git.go: Git runs with a
// closed environment, no system or global configuration, and no prompt, so the
// committed range this verb reads never depends on ambient Git settings.
func sanitizedGitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

// git runs one read-only Git command under the sanitized environment and the
// same configuration overrides the index builder uses.
func git(ctx context.Context, root, code string, arguments ...string) ([]byte, error) {
	commandArguments := append([]string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false",
		"-C", root,
	}, arguments...)
	command := exec.CommandContext(ctx, "git", commandArguments...)
	command.Env = sanitizedGitEnvironment()
	stdout, stderr := &boundedWriter{limit: maxGitOutputBytes}, &boundedWriter{limit: 8 << 10}
	command.Stdout, command.Stderr = stdout, stderr
	err := command.Run()
	if err != nil {
		return nil, &gokernel.Error{
			Code:    code,
			Message: "git " + strings.Join(arguments, " ") + " failed: " + failureDetail(stderr.buffer.String(), err),
		}
	}
	if stdout.exceeded {
		return nil, &gokernel.Error{Code: code, Message: "git " + strings.Join(arguments, " ") + " output exceeds its byte limit"}
	}
	return stdout.buffer.Bytes(), nil
}

func failureDetail(stderr string, err error) string {
	if trimmed := strings.TrimSpace(strings.SplitN(stderr, "\n", 2)[0]); trimmed != "" {
		return trimmed
	}
	return err.Error()
}

// resolveCommit pins one requested revision to its commit object, so every
// reported path is bound to immutable Git content (invariant 1).
func resolveCommit(ctx context.Context, root, revision string) (string, error) {
	output, err := git(ctx, root, "unsupported-surprise-revision", "rev-parse", "--verify", "--quiet", revision+"^{commit}")
	if err != nil {
		return "", revisionRefusal(revision)
	}
	resolved := strings.TrimSpace(string(output))
	if resolved == "" {
		return "", revisionRefusal(revision)
	}
	return resolved, nil
}

// requireCleanWorktree refuses a comparison over uncommitted edits: the actual
// set is committed evidence only.
func requireCleanWorktree(ctx context.Context, root string) error {
	output, err := git(ctx, root, "unsupported-surprise-git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if status := strings.TrimSpace(string(output)); status != "" {
		return dirtyWorktreeRefusal(strings.Count(status, "\n") + 1)
	}
	return nil
}

func changedPaths(ctx context.Context, root, base, target string) ([]string, error) {
	output, err := git(ctx, root, "unsupported-surprise-git", "diff", "-z", "--name-only", "--no-renames", base, target, "--")
	if err != nil {
		return nil, err
	}
	return splitNUL(output), nil
}

func trackedPaths(ctx context.Context, root, target string) (map[string]struct{}, error) {
	output, err := git(ctx, root, "unsupported-surprise-git", "ls-tree", "-r", "-z", "--name-only", target, "--")
	if err != nil {
		return nil, err
	}
	tracked := make(map[string]struct{})
	for _, path := range splitNUL(output) {
		tracked[path] = struct{}{}
	}
	return tracked, nil
}

// splitNUL reads `-z` output literally: newline output C-quotes a non-ASCII or
// quote-bearing path, and trimming would rewrite a path with edge whitespace, so
// either would report a spelling no index row or changed file carries.
func splitNUL(output []byte) []string {
	paths := make([]string, 0)
	for _, path := range strings.Split(string(output), "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
