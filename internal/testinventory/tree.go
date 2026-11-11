// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Tree is the repository content an inventory is judged against or listed
// from: every regular file with its Git blob object ID, and bounded reads.
// A reader never writes through it.
type Tree interface {
	// Files maps each regular file's repository-relative slash path to its
	// Git blob object ID. Symbolic links and submodules are not files.
	Files() (map[string]string, error)
	// Read returns at most limit bytes of path, an error wrapping
	// fs.ErrNotExist when it is absent and errLimit when it is longer.
	Read(path string, limit int) ([]byte, error)
}

// errLimit is wrapped by Tree.Read for a file longer than its limit.
var errLimit = errors.New("file exceeds its read limit")

// maxGitOutput bounds one Git listing a tree reads.
const maxGitOutput = 256 << 20

// MapTree is an in-memory Tree: path to content. Blob IDs are SHA-1 Git
// object IDs of the content.
type MapTree map[string]string

// Files implements Tree.
func (tree MapTree) Files() (map[string]string, error) {
	files := make(map[string]string, len(tree))
	for name, content := range tree {
		files[name] = BlobID("sha1", []byte(content))
	}
	return files, nil
}

// Read implements Tree.
func (tree MapTree) Read(name string, limit int) ([]byte, error) {
	content, ok := tree[name]
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
	}
	if len(content) > limit {
		return nil, fmt.Errorf("%s: %w", name, errLimit)
	}
	return []byte(content), nil
}

// BlobID is the Git blob object ID of content under objectFormat ("sha1" or
// "sha256").
func BlobID(objectFormat string, content []byte) string {
	var digest hash.Hash
	if objectFormat == "sha256" {
		digest = sha256.New()
	} else {
		digest = sha1.New()
	}
	fmt.Fprintf(digest, "blob %d\x00", len(content))
	digest.Write(content)
	return hex.EncodeToString(digest.Sum(nil))
}

// Git runs hermetic, lock-free Git commands in one repository: no global or
// system configuration, no optional locks, no fsmonitor or hooks, so a read
// never rewrites the index (product invariant 4).
type Git struct {
	Executable string
	Root       string
}

func (git Git) command(ctx context.Context, arguments ...string) *exec.Cmd {
	prefix := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false",
		"-c", "core.hooksPath=" + os.DevNull, "-C", git.Root}
	command := exec.CommandContext(ctx, git.Executable, append(prefix, arguments...)...)
	command.Env = []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ALLOW_PROTOCOL=", "LANG=C", "LC_ALL=C",
	}
	if _, err := os.Lstat(filepath.Join(git.Root, ".git")); err == nil {
		command.Env = append(command.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(git.Root))
	}
	return command
}

// Output runs one Git command and returns its bounded stdout.
func (git Git) Output(ctx context.Context, arguments ...string) ([]byte, error) {
	command := git.command(ctx, arguments...)
	var stderr bytes.Buffer
	command.Stderr = &limitedBuffer{buffer: &stderr, limit: 4096}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, maxGitOutput+1))
	waitErr := command.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if len(data) > maxGitOutput {
		return nil, fmt.Errorf("git %s: output exceeds %d bytes", arguments[0], maxGitOutput)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("git %s: %v: %s", arguments[0], waitErr, strings.TrimSpace(stderr.String()))
	}
	return data, nil
}

type limitedBuffer struct {
	buffer *bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.buffer.Len(); room > 0 {
		b.buffer.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

// Head is the full object ID of HEAD's commit.
func (git Git) Head(ctx context.Context) (string, error) {
	out, err := git.Output(ctx, "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	head := strings.TrimSpace(string(out))
	if !objectID(head) {
		return "", fmt.Errorf("HEAD %q is not a full object ID", head)
	}
	return head, nil
}

// ObjectFormat is the repository's object format, "sha1" or "sha256".
func (git Git) ObjectFormat(ctx context.Context) (string, error) {
	out, err := git.Output(ctx, "rev-parse", "--show-object-format")
	if err != nil {
		return "", err
	}
	switch format := strings.TrimSpace(string(out)); format {
	case "sha1", "sha256":
		return format, nil
	default:
		return "", fmt.Errorf("unknown object format %q", format)
	}
}

// requireTopLevel refuses a Root that is not the working tree's top level:
// Git prints Root's path below the top level, empty at the top level.
func (git Git) requireTopLevel(ctx context.Context) error {
	out, err := git.Output(ctx, "rev-parse", "--show-prefix")
	if err != nil {
		return err
	}
	if prefix := strings.TrimRight(string(out), "\r\n"); prefix != "" {
		return fmt.Errorf("root %s is %s below the Git top level; run from the top level", git.Root, prefix)
	}
	return nil
}

// CommitTree is the Tree of one commit, read from the object store.
type CommitTree struct {
	Git      Git
	Revision string
	ctx      context.Context
	files    map[string]string
}

// NewCommitTree lists revision's tree once.
func NewCommitTree(ctx context.Context, git Git, revision string) (*CommitTree, error) {
	out, err := git.Output(ctx, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", revision)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, record := range bytes.Split(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, name, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Fields(string(meta))
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("git ls-tree: unreadable record %q", record)
		}
		if fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755") {
			files[string(name)] = fields[2]
		}
	}
	return &CommitTree{Git: git, Revision: revision, ctx: ctx, files: files}, nil
}

// Files implements Tree.
func (tree *CommitTree) Files() (map[string]string, error) { return tree.files, nil }

// Read implements Tree.
func (tree *CommitTree) Read(name string, limit int) ([]byte, error) {
	blob, ok := tree.files[name]
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
	}
	size, err := tree.Git.Output(tree.ctx, "cat-file", "-s", blob)
	if err != nil {
		return nil, err
	}
	if n, err := strconv.Atoi(strings.TrimSpace(string(size))); err != nil || n > limit {
		return nil, fmt.Errorf("%s: %w", name, errLimit)
	}
	return tree.Git.Output(tree.ctx, "cat-file", "blob", blob)
}

// WorkTree is the working tree: tracked files with the index's blob IDs,
// except those the working tree changed, and untracked files Git does not
// ignore, both hashed from their bytes. It never runs a Git filter and never
// writes the index.
type WorkTree struct {
	Git   Git
	files map[string]string
}

// NewWorkTree lists the working tree once. It refuses a Git.Root that is
// not the repository's top level: Git lists paths from the top level and
// the tree reads them below Git.Root, so the two must be one directory.
func NewWorkTree(ctx context.Context, git Git) (*WorkTree, error) {
	if err := git.requireTopLevel(ctx); err != nil {
		return nil, err
	}
	format, err := git.ObjectFormat(ctx)
	if err != nil {
		return nil, err
	}
	staged, err := git.Output(ctx, "ls-files", "--full-name", "-s", "-z")
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, record := range bytes.Split(staged, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, name, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Fields(string(meta))
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("git ls-files: unreadable record %q", record)
		}
		if fields[2] == "0" && (fields[0] == "100644" || fields[0] == "100755") {
			files[string(name)] = fields[1]
		} else if fields[2] != "0" {
			// An unmerged path has no single index blob: hash its bytes.
			files[string(name)] = ""
		}
	}
	changed, err := git.Output(ctx, "diff-files", "--name-only", "-z", "--no-renames")
	if err != nil {
		return nil, err
	}
	untracked, err := git.Output(ctx, "ls-files", "--full-name", "-o", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var rehash []string
	for name, blob := range files {
		if blob == "" {
			rehash = append(rehash, name)
		}
	}
	for _, list := range [][]byte{changed, untracked} {
		for _, name := range bytes.Split(list, []byte{0}) {
			if len(name) != 0 {
				rehash = append(rehash, string(name))
			}
		}
	}
	for _, name := range rehash {
		delete(files, name)
		full := filepath.Join(git.Root, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		files[name] = BlobID(format, content)
	}
	return &WorkTree{Git: git, files: files}, nil
}

// Files implements Tree.
func (tree *WorkTree) Files() (map[string]string, error) { return tree.files, nil }

// Read implements Tree. It reads the file even when Git does not list it,
// so an untracked or ignored inventory is still the one on disk.
func (tree *WorkTree) Read(name string, limit int) ([]byte, error) {
	full := filepath.Join(tree.Git.Root, filepath.FromSlash(name))
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	file, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%s: %w", name, errLimit)
	}
	return data, nil
}
