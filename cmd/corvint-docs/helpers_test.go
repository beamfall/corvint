// SPDX-License-Identifier: Apache-2.0

package main

// Test helpers the docs tests shared with the corvint command package before the docs group
// moved here; they are copied so this package's tests run alone (CMS-V0-009).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func fixtureGitEnvironment() []string {
	return testsupport.WithoutGitIdentity(os.Environ())
}

func fixtureGitCommand(arguments ...string) *exec.Cmd {
	command := exec.Command("git", arguments...)
	command.Env = fixtureGitEnvironment()
	return command
}

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

func taskContextRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                "module example.test/ctx\n\ngo 1.27.0\n",
		"cache/demux.go":        "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go":   "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"testing/features.yaml": "features: []\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture"}} {
		command := fixtureGitCommand(arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func repositoryListing(t *testing.T, root string) string {
	t.Helper()
	command := fixtureGitCommand("status", "--porcelain", "--ignored")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relative, _ := filepath.Rel(root, path)
		hash.Write([]byte(relative))
		hash.Write([]byte{0})
		hash.Write(body)
		hash.Write([]byte{0})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func repositoryBytesDigest(t *testing.T, root string, excluded ...string) [sha256.Size]byte {
	t.Helper()
	skip := make(map[string]bool, len(excluded))
	for _, path := range excluded {
		skip[path] = true
	}
	paths := make([]string, 0)
	err := filepath.WalkDir(root, func(file string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if file != root {
			paths = append(paths, file)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	var snapshot bytes.Buffer
	for _, file := range paths {
		info, err := os.Lstat(file)
		if err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatal(err)
		}
		if skip[filepath.ToSlash(relative)] {
			continue
		}
		fmt.Fprintf(&snapshot, "%s\x00%s\x00", filepath.ToSlash(relative), info.Mode())
		switch {
		case info.Mode().IsRegular():
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Write(data)
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(file)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.WriteString(target)
		}
		snapshot.WriteByte(0)
	}
	return sha256.Sum256(snapshot.Bytes())
}

// treeSnapshot records every entry under root with its mode, content digest and mtime, so a
// read-only verb can be shown to leave the worktree byte-identical.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if file == root {
			return nil
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		info, err := os.Lstat(file)
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			snapshot[filepath.ToSlash(relative)] = fmt.Sprintf("dir %v", info.Mode())
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(file)
			if err != nil {
				return err
			}
			snapshot[filepath.ToSlash(relative)] = "link " + target
		default:
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(data)
			snapshot[filepath.ToSlash(relative)] = fmt.Sprintf("file %v %s %d", info.Mode(), hex.EncodeToString(digest[:]), info.ModTime().UnixNano())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// forbiddenHelpReader fails a test that reads stdin while answering help.
type forbiddenHelpReader struct{ reads int }

func (reader *forbiddenHelpReader) Read(_ []byte) (int, error) {
	reader.reads++
	return 0, nil
}

// stdoutBrokenPipeWriter accepts four bytes and then fails every write.
type stdoutBrokenPipeWriter struct{ bytes.Buffer }

func (w *stdoutBrokenPipeWriter) Write(raw []byte) (int, error) {
	n, _ := w.Buffer.Write(raw[:min(4, len(raw))])
	return n, io.ErrClosedPipe
}
