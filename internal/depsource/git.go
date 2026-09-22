package depsource

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
)

const maxGitOutputBytes = 8 << 20

// sanitizedGitEnvironment mirrors internal/contextindex/git.go: a closed
// environment so a repository read never consults ambient Git configuration,
// credential helpers, or a lazy fetch.
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

func git(ctx context.Context, root string, arguments ...string) ([]byte, error) {
	commandArguments := []string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false",
		"-C", root,
	}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, "git", commandArguments...)
	command.Env = sanitizedGitEnvironment()
	stdout := &cappedBuffer{limit: maxGitOutputBytes}
	stderr := &bytes.Buffer{}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return nil, &Error{Message: "git " + arguments[0] + " failed: " + firstLine(stderr.String())}
	}
	if stdout.overflow {
		return nil, &Error{Message: "git " + arguments[0] + " emitted more than the bounded read limit"}
	}
	return stdout.buffer.Bytes(), nil
}

// cappedBuffer consumes all command output so the child can exit normally,
// but retains no more than limit bytes and records whether output exceeded it.
type cappedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (writer *cappedBuffer) Write(content []byte) (int, error) {
	written := len(content)
	remaining := writer.limit - writer.buffer.Len()
	if remaining <= 0 {
		writer.overflow = writer.overflow || written > 0
		return written, nil
	}
	if len(content) > remaining {
		writer.overflow = true
		content = content[:remaining]
	}
	_, _ = writer.buffer.Write(content)
	return written, nil
}

func firstLine(value string) string {
	trimmed := strings.TrimSpace(value)
	if index := strings.IndexByte(trimmed, '\n'); index >= 0 {
		return trimmed[:index]
	}
	if trimmed == "" {
		return "no diagnostic"
	}
	return trimmed
}

// committedRevision is the commit whose tree every go.mod, go.sum and
// local-replace blob in one answer is read from. The dirty worktree is never
// an evidence source (AGENTS.md invariant 1).
func committedRevision(ctx context.Context, root string) (string, error) {
	raw, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	revision := strings.TrimSpace(string(raw))
	if revision == "" {
		return "", &Error{Message: "repository has no committed revision"}
	}
	return revision, nil
}

func showBlob(ctx context.Context, root, revision, path string) ([]byte, error) {
	return git(ctx, root, "show", revision+":"+path)
}

// blobAbsent separates a tree that genuinely carries no such path from a Git
// failure. Only the first is an abstention; the second is a repository failure
// and must reach the caller as an error.
func blobAbsent(ctx context.Context, root, revision, path string) bool {
	raw, err := git(ctx, root, "ls-tree", "--name-only", revision, "--", path)
	return err == nil && strings.TrimSpace(string(raw)) == ""
}

type treeBlob struct {
	Path string
	OID  string
	Size int64
}
