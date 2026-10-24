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
	gitDir, commonDir string
}

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
	out, err := gitOutput(root, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	if err != nil {
		return headLayout{}, false
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), "\n")
	if len(lines) != 2 || !filepath.IsAbs(lines[0]) || !filepath.IsAbs(lines[1]) {
		return headLayout{}, false
	}
	l := headLayout{gitDir: filepath.Clean(lines[0]), commonDir: filepath.Clean(lines[1])}
	if _, err := os.Lstat(filepath.Join(l.commonDir, "reftable")); !errors.Is(err, fs.ErrNotExist) {
		return headLayout{}, false
	}
	return l, true
}

// head resolves HEAD to the object ID its ref chain names, reading only the
// HEAD file, loose ref files and packed-refs. It reports false for anything
// it does not resolve exactly as Git's files backend does (a symlink, an
// unborn branch, an unusual ref name, an oversized or malformed file); the
// caller then treats the memoized base as unconfirmed.
func (l headLayout) head() (string, bool) {
	b, present := readRefFile(filepath.Join(l.gitDir, "HEAD"))
	if !present {
		return "", false
	}
	for depth := 0; ; depth++ {
		s := strings.TrimSpace(string(b))
		name, symbolic := strings.CutPrefix(s, "ref: ")
		if !symbolic {
			oid, err := wire.ParseOID("HEAD", s)
			return oid, err == nil
		}
		if depth == maxSymrefDepth || !plainRefName(name) {
			return "", false
		}
		dir := l.commonDir
		for _, own := range []string{"refs/bisect/", "refs/worktree/", "refs/rewritten/"} {
			if strings.HasPrefix(name, own) {
				dir = l.gitDir
			}
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if b, present = readRefFile(path); present {
			continue
		}
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			return "", false
		}
		return l.packed(name)
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
