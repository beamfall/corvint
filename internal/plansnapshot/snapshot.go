// Package plansnapshot validates caller-owned immutable planning inputs.
package plansnapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const Schema = "corvint-planning-snapshot/0"
const MaxBytes = 512 << 10

var ErrInvalid = errors.New("unsupported-planning-snapshot")

// Receipt binds the complete committed diff and all source/configuration bytes.
// Overlay inputs are deliberately absent: unknown members fail closed.
type Receipt struct {
	Schema string   `json:"schema"`
	Commit string   `json:"commitRevision"`
	Tree   string   `json:"treeRevision"`
	Base   string   `json:"baseRevision"`
	Paths  []string `json:"changedPaths"`
	Digest string   `json:"changedPathsSha256"`
}

func Decode(raw []byte) (Receipt, error) {
	var receipt Receipt
	if len(raw) > MaxBytes {
		return receipt, ErrInvalid
	}
	if err := json.Unmarshal(raw, &receipt, json.RejectUnknownMembers(true)); err != nil {
		return receipt, ErrInvalid
	}
	if !receipt.Valid() {
		return receipt, ErrInvalid
	}
	return receipt, nil
}

func (receipt Receipt) Valid() bool {
	if receipt.Schema != Schema || !objectID(receipt.Commit) || !objectID(receipt.Base) || !objectID(receipt.Tree) || receipt.Paths == nil || len(receipt.Paths) > 4096 {
		return false
	}
	previous := ""
	for _, path := range receipt.Paths {
		if !affected.ValidRelativePath(path) || path <= previous {
			return false
		}
		previous = path
	}
	return receipt.Digest == PathDigest(receipt.Paths)
}

func PathDigest(paths []string) string {
	body, _ := gokernel.CanonicalJSON(paths)
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func objectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

type reader struct {
	root, binary string
	budget       *gitrun.Budget
}

func newReader(root string) (*reader, error) {
	binary, err := exec.LookPath("git")
	if err != nil {
		return nil, ErrInvalid
	}
	return &reader{root, binary, gitrun.NewBudget(12, 30*time.Second)}, nil
}

func (r *reader) run(ctx context.Context, input []byte, limit int, args ...string) ([]byte, error) {
	env := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LANG=C", "LC_ALL=C"}
	body, err := gitrun.Run(ctx, r.budget, gitrun.Options{Binary: r.binary, Dir: r.root, Env: env, Stdin: input, StdoutLimit: limit}, append([]string{"--no-optional-locks", "-C", r.root}, args...)...)
	if err != nil {
		return nil, ErrInvalid
	}
	return body, nil
}

func (receipt Receipt) Validate(ctx context.Context, root string) error {
	_, _, err := receipt.validatedTree(ctx, root)
	return err
}

func (receipt Receipt) validatedTree(ctx context.Context, root string) ([]string, []string, error) {
	if !receipt.Valid() {
		return nil, nil, ErrInvalid
	}
	r, err := newReader(root)
	if err != nil {
		return nil, nil, err
	}
	for _, pair := range [][2]string{{"HEAD^{commit}", receipt.Commit}, {receipt.Commit + "^{tree}", receipt.Tree}, {receipt.Base + "^{commit}", receipt.Base}} {
		body, err := r.run(ctx, nil, 1024, "rev-parse", "--verify", pair[0])
		if err != nil || string(body) != pair[1]+"\n" {
			return nil, nil, ErrInvalid
		}
	}
	body, err := r.run(ctx, nil, 8<<20, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", receipt.Base, receipt.Commit, "--")
	if err != nil {
		return nil, nil, err
	}
	paths, err := affected.DecodeNameList(body)
	if err != nil || PathDigest(paths) != receipt.Digest {
		return nil, nil, ErrInvalid
	}
	return r.treeEntries(ctx, receipt.Tree)
}

// Scope distinguishes authoritative immutable inputs from advisory selection.
func (receipt Receipt) Scope() map[string]any {
	return map[string]any{"kind": "IMMUTABLE_COMMITTED_SNAPSHOT", "status": "PLAN_ONLY", "accepting": false, "worktree": "NOT_EVIDENCE", "sourceConfigTree": receipt.Tree, "receipt": receipt}
}

// Materialize copies exact blobs to private scratch; no checkout, filters,
// archive attributes, live files, symlinks or submodules can change the inputs.
func (receipt Receipt) Materialize(ctx context.Context, root string) (string, func(), error) {
	paths, ids, err := receipt.validatedTree(ctx, root)
	if err != nil {
		return "", nil, err
	}
	r, err := newReader(root)
	if err != nil {
		return "", nil, err
	}
	input := []byte(strings.Join(ids, "\n"))
	if len(ids) > 0 {
		input = append(input, '\n')
	}
	blobs, err := r.run(ctx, input, 64<<20, "cat-file", "--batch")
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "corvint-planning-snapshot-")
	if err != nil {
		return "", nil, ErrInvalid
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	relative, relErr := filepath.Rel(resolvedRoot, resolvedDir)
	if rootErr != nil || dirErr != nil || relErr != nil || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		cleanup()
		return "", nil, ErrInvalid
	}
	if err := writeBlobs(ctx, dir, paths, ids, blobs); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}

func writeBlobs(ctx context.Context, dir string, paths, ids []string, blobs []byte) error {
	buffer := bytes.NewBuffer(blobs)
	for index, path := range paths {
		if ctx.Err() != nil {
			return ErrInvalid
		}
		header, err := buffer.ReadString('\n')
		fields := strings.Fields(header)
		if err != nil || len(fields) != 3 || fields[0] != ids[index] || fields[1] != "blob" {
			return ErrInvalid
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size >= buffer.Len() {
			return ErrInvalid
		}
		body := buffer.Next(size)
		if next, err := buffer.ReadByte(); err != nil || next != '\n' {
			return ErrInvalid
		}
		name := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return ErrInvalid
		}
		file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return ErrInvalid
		}
		_, writeErr := file.Write(body)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return ErrInvalid
		}
	}
	if buffer.Len() != 0 {
		return ErrInvalid
	}
	actual := []string{}
	err := filepath.WalkDir(dir, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dir, name)
		if err != nil {
			return err
		}
		actual = append(actual, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(actual)
	expected := append([]string{}, paths...)
	sort.Strings(expected)
	if err != nil || strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		return ErrInvalid
	}
	return nil
}

func ReadFile(name string) (Receipt, error) {
	before, err := os.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Size() > MaxBytes {
		return Receipt{}, ErrInvalid
	}
	f, err := os.Open(name)
	if err != nil {
		return Receipt{}, ErrInvalid
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !os.SameFile(before, info) || !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return Receipt{}, ErrInvalid
	}
	body, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return Receipt{}, ErrInvalid
	}
	return Decode(body)
}

func (r *reader) treeEntries(ctx context.Context, tree string) ([]string, []string, error) {
	body, err := r.run(ctx, nil, 8<<20, "ls-tree", "-r", "-z", "--full-tree", tree)
	if err != nil {
		return nil, nil, err
	}
	entries := bytes.Split(bytes.TrimSuffix(body, []byte{0}), []byte{0})
	var paths, ids []string
	for _, entry := range entries {
		if len(entry) == 0 {
			continue
		}
		header, path, ok := strings.Cut(string(entry), "\t")
		fields := strings.Fields(header)
		if !ok || len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || !objectID(fields[2]) || !affected.ValidRelativePath(path) {
			return nil, nil, ErrInvalid
		}
		for _, part := range strings.Split(path, "/") {
			if strings.EqualFold(part, ".git") {
				return nil, nil, ErrInvalid
			}
		}
		paths, ids = append(paths, path), append(ids, fields[2])
	}
	if len(paths) > 20000 {
		return nil, nil, ErrInvalid
	}
	return paths, ids, nil
}
