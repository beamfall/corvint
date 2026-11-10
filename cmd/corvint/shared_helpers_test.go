package main

// Test fixture helpers shared by kept tests. They used to live in test files of
// command families retired by decision 0594.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cemGit runs git on the fixture with the host's config files masked rather
// than HOME moved: macOS's git shim keys its toolchain cache on HOME, and a
// foreign HOME under a sandbox costs a full re-resolution plus a cache warning
// on stderr per call. Only stdout is returned, so such a warning can never
// leak into a revision or blob the test hands back to the CLI.
func cemGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := fixtureGitCommand(append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = dir
	command.Env = append(fixtureGitEnvironment(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s%s", args, err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func cemWrite(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cemRepo builds a byte-deterministic two-commit fixture repository.
func cemRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "init", "-q", "-b", "main")
	cemWrite(t, root, "docs/rule.txt", "the frozen rule\nsecond line\n")
	cemWrite(t, root, "src/app.txt", "alpha\nbeta\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "base")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "src/app.txt", "alpha\nBETA\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "target")
	target := cemGit(t, root, "rev-parse", "HEAD")
	return root, base, target
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func materializationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := fixtureGitCommandContext(ctx, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", root}, args...)...)
	command.Env = append(fixtureGitEnvironment(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	command.WaitDelay = 10 * time.Second
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, raw)
	}
	return strings.TrimSpace(string(raw))
}

func materializationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	materializationGit(t, root, "init", "-q")
	// A commit must not spawn detached auto maintenance: it outlives the fixture
	// command and its transient .git writes race the manifests.
	materializationGit(t, root, "config", "maintenance.auto", "false")
	materializationGit(t, root, "config", "gc.auto", "0")
	for name, raw := range map[string]string{"tracked": "pinned bytes\n", ".gitignore": "ignored\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "fixture")
	return root
}

func candidateCommand(arguments ...string) *exec.Cmd {
	command := exec.Command(os.Args[0], append([]string{"-test.run=^TestCandidateHelperProcess$", "--"}, arguments...)...)
	command.Env = append(os.Environ(), "CORVINT_HELPER_PROCESS=1")
	return command
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
