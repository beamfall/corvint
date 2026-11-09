package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Flows fixtures shared with root tests (affected, impact, MCP parity and the remaining flows
// dispatch/help tests). The handler-level copies live in internal/command/flows.

func runFlowsCLI(root string, args ...string) (int, string, string) {
	var out, diagnostic bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root, "flows"}, args...), bytes.NewReader(nil), &out, &diagnostic)
	return code, out.String(), diagnostic.String()
}

// appmapCLIRepo commits the internal/appmap fixture application into a fresh repository.
func appmapCLIRepo(t *testing.T) string {
	t.Helper()
	return appmapCLIRepoFrom(t, "fixture")
}

// appmapCLIRepoFrom commits one internal/appmap/testdata fixture into a fresh repository.
func appmapCLIRepoFrom(t *testing.T, fixture string) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join("..", "..", "internal", "appmap", "testdata", fixture)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		cmd := fixtureGitCommand(append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		cmd.Dir, cmd.Env = root, append(fixtureGitEnvironment(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

func decodeNumbers(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
func shopGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := fixtureGitCommand(append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false"}, args...)...)
	c.Dir = root
	date := "2026-01-01T00:00:00Z"
	c.Env = append(fixtureGitEnvironment(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func shopWrite(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func shopCommit(t *testing.T, root, message string) string {
	t.Helper()
	shopGit(t, root, "add", ".")
	shopGit(t, root, "commit", "-qm", message)
	return shopGit(t, root, "rev-parse", "HEAD")
}
