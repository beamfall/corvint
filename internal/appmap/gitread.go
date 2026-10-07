package appmap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// blobEntry is one regular committed file at the evaluated revision.
type blobEntry struct {
	repo string // the alias of the root read; empty is --root
	path string
	oid  string
	size int
}

// repo reads committed objects only: never the working tree, never the network (AMAP-V0-015).
// alias names an operator-declared second root (AMAP-V0-016); empty is --root.
type repo struct {
	ctx   context.Context
	root  string
	alias string
}

func rootUnavailable(alias, why string) error {
	return &gokernel.Error{Code: "appmap-root-unavailable", Message: fmt.Sprintf("root %s %s", alias, why)}
}

// openRoot binds a manifest alias to its operator-declared root and pins that root's HEAD commit
// (AMAP-V0-016). The root keeps the MCPV0-001 bounds the multi-root MCP server applies
// (MMR-V0-002), must be the top of a Git worktree, and must not be --root itself (MMR-V0-003).
func openRoot(ctx context.Context, primary, alias, root string) (repo, string, error) {
	if root == "" {
		return repo{}, "", rootUnavailable(alias, "is not declared: pass --repo "+alias+"=ABSOLUTE_ROOT")
	}
	if !rootalias.ValidRoot(root) {
		return repo{}, "", rootUnavailable(alias, "must be an absolute, clean path")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return repo{}, "", rootUnavailable(alias, "is unreadable")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return repo{}, "", rootUnavailable(alias, "is not a directory")
	}
	if p, err := os.Stat(primary); err == nil && os.SameFile(info, p) {
		return repo{}, "", rootUnavailable(alias, "is the --root repository")
	}
	r := repo{ctx: ctx, root: resolved, alias: alias}
	top, err := r.git(4096, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return repo{}, "", rootUnavailable(alias, "is not a Git worktree")
	}
	if t, err := os.Stat(strings.TrimSpace(string(top))); err != nil || !os.SameFile(info, t) {
		return repo{}, "", rootUnavailable(alias, "is not the top of a Git worktree")
	}
	rev, err := r.resolve("HEAD")
	if err != nil {
		return repo{}, "", rootUnavailable(alias, "has no HEAD commit")
	}
	return r, rev, nil
}

// dirty reports whether the worktree differs from HEAD under any of paths: a staged, unstaged or
// untracked change (ignored files do not count). It writes nothing: --no-optional-locks keeps
// status from refreshing the index.
func (r repo) dirty(paths []string) (bool, error) {
	args := append([]string{"--literal-pathspecs", "status", "--porcelain=v1", "-z", "--untracked-files=normal", "--"}, paths...)
	out, err := r.git(1<<20, nil, args...)
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

func (r repo) git(limit int, stdin []byte, args ...string) ([]byte, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, errors.New("Git unavailable")
	}
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", r.root}, args...)
	o := procgroup.Run(r.ctx, procgroup.Spec{Argv: argv, Dir: r.root, Env: gokernel.SanitizedGitEnvironment(), Stdin: stdin,
		Timeout: 60 * time.Second, OutputLimit: limit})
	if o.Err != nil || o.ExitStatus != 0 {
		return nil, errors.New("application map Git source unavailable")
	}
	return o.Stdout, nil
}

// resolve names the commit a revision denotes.
func (r repo) resolve(revision string) (string, error) {
	if revision == "" || strings.HasPrefix(revision, "-") {
		return "", errors.New("invalid revision")
	}
	out, err := r.git(4096, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", errors.New("evaluated revision is not a commit")
	}
	return strings.TrimSpace(string(out)), nil
}

// tree lists the regular files under each prefix at rev. A prefix naming a file lists that file.
// More than limit entries refuses before any blob is read.
func (r repo) tree(rev string, prefixes []string, limit int) ([]blobEntry, error) {
	if len(prefixes) == 0 {
		return nil, nil
	}
	args := append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "-l", "--full-tree", rev, "--"}, prefixes...)
	out, err := r.git(64<<20, nil, args...)
	if err != nil {
		return nil, err
	}
	entries := []blobEntry{}
	for _, line := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if len(line) == 0 {
			continue
		}
		meta, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 {
			return nil, errors.New("application map tree unreadable")
		}
		if fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			continue // symlinks and submodules are never followed
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("%s: committed input unavailable", name)
		}
		entries = append(entries, blobEntry{repo: r.alias, path: name, oid: fields[2], size: size})
		if len(entries) > limit {
			return nil, bound(fmt.Sprintf("more than %d files under the declared test root", limit))
		}
	}
	return entries, nil
}

// missingDirs returns the declared directories that are not trees at rev. One
// `git cat-file --batch-check` answers them all; a name Git cannot take on one line is missing.
func (r repo) missingDirs(rev string, dirs []string) ([]string, error) {
	missing, asked := []string{}, []string{}
	var in bytes.Buffer
	for _, d := range dirs {
		if strings.ContainsAny(d, "\n\r") {
			missing = append(missing, d)
			continue
		}
		asked = append(asked, d)
		in.WriteString(rev + ":" + d + "\n")
	}
	if len(asked) == 0 {
		return missing, nil
	}
	out, err := r.git(1<<20, in.Bytes(), "cat-file", "--batch-check=%(objecttype)")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != len(asked) {
		return nil, errors.New("application map tree unreadable")
	}
	for i, d := range asked {
		if lines[i] != "tree" {
			missing = append(missing, d)
		}
	}
	return missing, nil
}

// blobs reads every entry through one `git cat-file --batch`, checking each recorded size.
func (r repo) blobs(entries []blobEntry, perFile, total int) (map[string][]byte, error) {
	out := map[string][]byte{}
	if len(entries) == 0 {
		return out, nil
	}
	sum, request := 0, &bytes.Buffer{}
	unique := map[string]bool{}
	for _, e := range entries {
		if e.size > perFile {
			return nil, bound(fmt.Sprintf("%s exceeds %d bytes", e.path, perFile))
		}
		if !unique[e.oid] {
			unique[e.oid] = true
			sum += e.size + 128
			request.WriteString(e.oid + "\n")
		}
	}
	if sum > total {
		return nil, bound(fmt.Sprintf("declared inputs exceed %d bytes", total))
	}
	raw, err := r.git(sum+1024, request.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	for len(raw) > 0 {
		header, rest, ok := bytes.Cut(raw, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			return nil, errors.New("committed input unavailable")
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size+1 > len(rest) {
			return nil, errors.New("committed input unavailable")
		}
		out[fields[0]] = rest[:size]
		raw = rest[size+1:]
	}
	for oid := range unique {
		if _, ok := out[oid]; !ok {
			return nil, errors.New("committed input unavailable")
		}
	}
	return out, nil
}

// readOne reads one committed regular file at rev.
func (r repo) readOne(rev, p string, limit int) (blobEntry, []byte, error) {
	entries, err := r.tree(rev, []string{p}, 1)
	if err != nil {
		return blobEntry{}, nil, err
	}
	for _, e := range entries {
		if e.path == p {
			data, err := r.blobs([]blobEntry{e}, limit, limit+128)
			if err != nil {
				return e, nil, err
			}
			return e, data[e.oid], nil
		}
	}
	return blobEntry{}, nil, fmt.Errorf("%s is absent at the evaluated revision", p)
}

func digest(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// spanOf anchors lines start..end of a committed file.
func spanOf(e blobEntry, data []byte, start, end int) Anchor {
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if n := len(lines); n > 0 && len(lines[n-1]) == 0 {
		lines = lines[:n-1]
	}
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if end < start {
		end = start
	}
	span := []byte{}
	if start <= len(lines) {
		span = bytes.Join(lines[start-1:end], nil)
	}
	return Anchor{Repo: e.repo, Path: e.path, Start: start, End: end, Blob: e.oid, SpanSHA256: digest(span)}
}

// wholeFile anchors every line of a committed file.
func wholeFile(e blobEntry, data []byte) Anchor {
	return spanOf(e, data, 1, bytes.Count(data, []byte{'\n'})+1)
}

// safeRelative reports whether p is a clean repository-relative slash path outside .git.
func safeRelative(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || path.Clean(p) != p || p == "." || strings.HasPrefix(p, "../") || p == ".." {
		return false
	}
	first, _, _ := strings.Cut(p, "/")
	return !strings.EqualFold(first, ".git")
}

func bound(message string) error {
	return &gokernel.Error{Code: "appmap-bound-exceeded", Message: message}
}
