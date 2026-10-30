package store

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// headLayout is where a claim root's HEAD and refs live, observed with Git
// before any lock (PSR-V0-224). The lock-held claim observer resolves the
// root's HEAD from these files alone, so it can confirm a memoized base
// without running Git.
type headLayout struct {
	root, gitDir, commonDir string
	route                   headRoute
	// read reads a HEAD or ref file; nil reads with readRefFile. Tests
	// interpose on it to move refs between two reads.
	read func(path string) ([]byte, bool)
}

// headRoute is how root reaches its Git directory: its .git entry (a
// directory, or a file naming a linked worktree's admin directory) and that
// directory's gitdir and commondir files. A layout serves only while the
// route is the one Git resolved when it was observed (PSR-V0-224).
type headRoute struct {
	dir                       os.FileInfo
	dotGit, gitdir, commondir string
}

// absentFile stands for a routing file that does not exist; a file that
// exists never reads as it, because readRefFile refuses a NUL byte.
const absentFile = "\x00"

const (
	maxRefFile     = 4 << 10
	maxPackedRefs  = 8 << 20
	maxSymrefDepth = 5
)

// headQuery reports whether a Git query resolves the root's HEAD commit, the
// one mutable query PSR-V0-224 binds to the ref files instead of the store
// snapshot.
func headQuery(args []string) bool {
	return len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" &&
		args[2] == "--end-of-options" && args[3] == "HEAD^{commit}"
}

// observeHeadLayout asks Git where root's HEAD and common refs live. It
// reports false for a layout the file resolver does not read: a failed
// query, a relative path, or a reftable ref store.
func observeHeadLayout(root string) (headLayout, bool) {
	before, ok := readRoute(root, "")
	if !ok {
		return headLayout{}, false
	}
	out, err := gitOutput(root, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	if err != nil {
		return headLayout{}, false
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), "\n")
	if len(lines) != 2 || !filepath.IsAbs(lines[0]) || !filepath.IsAbs(lines[1]) {
		return headLayout{}, false
	}
	l := headLayout{root: root, gitDir: filepath.Clean(lines[0]), commonDir: filepath.Clean(lines[1])}
	if _, err := os.Lstat(filepath.Join(l.commonDir, "reftable")); !errors.Is(err, fs.ErrNotExist) {
		return headLayout{}, false
	}
	// The route read after the query must be the one read before it, and a
	// .git directory must be the Git directory itself, so the route names
	// the layout Git reported.
	if l.route, ok = readRoute(root, l.gitDir); !ok || !sameRoute(before, l.route, false) {
		return headLayout{}, false
	}
	if l.route.dir != nil {
		if info, err := os.Stat(l.gitDir); err != nil || !os.SameFile(info, l.route.dir) {
			return headLayout{}, false
		}
	}
	return l, true
}

// readRoute reads root's .git entry and, when gitDir is set, gitDir's
// gitdir and commondir files. A root whose .git is missing (a subdirectory
// of a checkout) or is neither a directory nor a regular file has no route.
func readRoute(root, gitDir string) (headRoute, bool) {
	dot := filepath.Join(root, ".git")
	info, err := os.Lstat(dot)
	if err != nil {
		return headRoute{}, false
	}
	r := headRoute{dotGit: absentFile, gitdir: absentFile, commondir: absentFile}
	switch {
	case info.IsDir():
		r.dir = info
	case info.Mode().IsRegular():
		b, ok := readRefFile(dot)
		if !ok {
			return headRoute{}, false
		}
		r.dotGit = string(b)
	default:
		return headRoute{}, false
	}
	if gitDir == "" {
		return r, true
	}
	for _, f := range []struct {
		name string
		into *string
	}{{"gitdir", &r.gitdir}, {"commondir", &r.commondir}} {
		path := filepath.Join(gitDir, f.name)
		if b, ok := readRefFile(path); ok {
			*f.into = string(b)
		} else if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			return headRoute{}, false
		}
	}
	return r, true
}

// sameRoute reports whether two routes are the same; with files false it
// compares only the .git entry.
func sameRoute(a, b headRoute, files bool) bool {
	if (a.dir == nil) != (b.dir == nil) || a.dir != nil && !os.SameFile(a.dir, b.dir) || a.dotGit != b.dotGit {
		return false
	}
	return !files || a.gitdir == b.gitdir && a.commondir == b.commondir
}

// head resolves HEAD to the object ID its ref chain names, reading only the
// root's routing files, the HEAD file, loose ref files and packed-refs. It
// reports false for anything it does not resolve exactly as Git's files
// backend does (a symlink, an unborn branch, an unusual ref name, an
// oversized or malformed file), for a root no longer routed to the observed
// Git directory, and for a HEAD or symbolic ref that changed while the
// chain was read; the caller then treats the memoized base as unconfirmed.
func (l headLayout) head() (string, bool) {
	if !l.routed() {
		return "", false
	}
	oid, chain, ok := l.resolve()
	if !ok {
		return "", false
	}
	for _, link := range chain {
		if b, present := l.readFile(link.path); !present || !bytes.Equal(b, link.b) {
			return "", false
		}
	}
	return oid, l.routed()
}

// routed reports whether root still reaches the observed Git directory.
func (l headLayout) routed() bool {
	r, ok := readRoute(l.root, l.gitDir)
	return ok && sameRoute(l.route, r, true)
}

func (l headLayout) readFile(path string) ([]byte, bool) {
	if l.read != nil {
		return l.read(path)
	}
	return readRefFile(path)
}

type refLink struct {
	path string
	b    []byte
}

// resolve follows HEAD's chain once and returns the object ID with every
// symbolic file it read (HEAD itself when symbolic, and each symref).
func (l headLayout) resolve() (string, []refLink, bool) {
	path := filepath.Join(l.gitDir, "HEAD")
	b, present := l.readFile(path)
	if !present {
		return "", nil, false
	}
	var chain []refLink
	for depth := 0; ; depth++ {
		s := strings.TrimSpace(string(b))
		name, symbolic := strings.CutPrefix(s, "ref: ")
		if !symbolic {
			oid, err := wire.ParseOID("HEAD", s)
			return oid, chain, err == nil
		}
		chain = append(chain, refLink{path, b})
		if depth == maxSymrefDepth || !plainRefName(name) {
			return "", nil, false
		}
		dir := l.commonDir
		for _, own := range []string{"refs/bisect/", "refs/worktree/", "refs/rewritten/"} {
			if strings.HasPrefix(name, own) {
				dir = l.gitDir
			}
		}
		path = filepath.Join(dir, filepath.FromSlash(name))
		if b, present = l.readFile(path); present {
			continue
		}
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			return "", nil, false
		}
		oid, ok := l.packed(name)
		return oid, chain, ok
	}
}

// packed looks name up in packed-refs. A peeled line (^OID) follows its ref
// and is not a ref itself.
func (l headLayout) packed(name string) (string, bool) {
	f, err := os.Open(filepath.Join(l.commonDir, "packed-refs"))
	if err != nil {
		return "", false
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() > maxPackedRefs {
		return "", false
	}
	scan := bufio.NewScanner(io.LimitReader(f, maxPackedRefs))
	scan.Buffer(make([]byte, 0, 4096), maxRefFile)
	for scan.Scan() {
		line := scan.Text()
		oid, ref, ok := strings.Cut(line, " ")
		if !ok || ref != name {
			continue
		}
		oid, err := wire.ParseOID("packed-refs", oid)
		return oid, err == nil
	}
	return "", false
}

// readRefFile reads a regular file of at most maxRefFile bytes; anything
// else, including a symlink, reads as absent.
func readRefFile(path string) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxRefFile {
		return nil, false
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) > maxRefFile || bytes.IndexByte(b, 0) >= 0 {
		return nil, false
	}
	return b, true
}

// plainRefName accepts a ref name under refs/ whose components the files
// backend stores as plain path segments.
func plainRefName(name string) bool {
	rest, ok := strings.CutPrefix(name, "refs/")
	if !ok || rest == "" {
		return false
	}
	for _, part := range strings.Split(rest, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") ||
			strings.HasSuffix(part, ".") || strings.Contains(part, "..") {
			return false
		}
		for _, r := range part {
			if r < 0x21 || r == 0x7f || strings.ContainsRune(`~^:?*[\@{`, r) || r > 0x7e {
				return false
			}
		}
	}
	return true
}
