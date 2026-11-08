package linecov

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func gitOut(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Diff returns the changed paths between two commits with their blob OIDs and
// old-side hunks. A rename is a delete and an add.
func Diff(ctx context.Context, root, from, to string) ([]FileDiff, error) {
	out, err := gitOut(ctx, root, "diff", "--raw", "-p", "-U0", "--no-renames", "--no-ext-diff", "--no-abbrev", "--full-index", "--no-color", from, to)
	if err != nil {
		return nil, err
	}
	diffs, err := parseDiff(out)
	if err != nil {
		return nil, err
	}
	// Read immutable head blobs as well as the caller's old-side reader.
	newDiffs := append([]FileDiff(nil), diffs...)
	for i := range newDiffs {
		newDiffs[i].OldOID = newDiffs[i].NewOID
	}
	readNew, err := BlobReader(ctx, root, newDiffs)
	if err != nil {
		return nil, err
	}
	for i := range diffs {
		if diffs[i].NewOID != "" && strings.HasSuffix(diffs[i].Path, ".go") {
			diffs[i].NewSource, err = readNew(diffs[i].Path)
			if err != nil {
				return nil, err
			}
		}
	}
	return diffs, nil
}

const zeroOID = "0000000000000000000000000000000000000000"

func parseDiff(out []byte) ([]FileDiff, error) {
	var diffs []FileDiff
	byPath := map[string]int{}
	var cur *FileDiff
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, ":"):
			meta, p, ok := strings.Cut(line, "\t")
			f := strings.Fields(meta)
			if !ok || len(f) != 5 || strings.HasPrefix(p, `"`) {
				return nil, fmt.Errorf("linecov: unsupported diff entry %q", line)
			}
			d := FileDiff{Path: p, OldOID: f[2], NewOID: f[3]}
			if d.OldOID == zeroOID {
				d.OldOID = ""
			}
			if d.NewOID == zeroOID {
				d.NewOID = ""
			}
			byPath[p] = len(diffs)
			diffs = append(diffs, d)
		case strings.HasPrefix(line, "diff --git "):
			cur = nil
		case strings.HasPrefix(line, "--- a/"):
			if i, ok := byPath[strings.TrimPrefix(line, "--- a/")]; ok {
				cur = &diffs[i]
			}
		case strings.HasPrefix(line, "+++ b/") && cur == nil:
			if i, ok := byPath[strings.TrimPrefix(line, "+++ b/")]; ok {
				cur = &diffs[i]
			}
		case strings.HasPrefix(line, "@@ -") && cur != nil:
			span, _, _ := strings.Cut(strings.TrimPrefix(line, "@@ -"), " ")
			startText, countText, hasCount := strings.Cut(span, ",")
			start, err := strconv.Atoi(startText)
			count := 1
			if err == nil && hasCount {
				count, err = strconv.Atoi(countText)
			}
			if err != nil {
				return nil, fmt.Errorf("linecov: malformed hunk %q", line)
			}
			cur.Hunks = append(cur.Hunks, Hunk{Start: start, End: start + count - 1})
			parts := strings.Fields(line)
			if len(parts) < 3 || !strings.HasPrefix(parts[2], "+") {
				return nil, fmt.Errorf("linecov: malformed new hunk %q", line)
			}
			newStart, newCount, hasNewCount := strings.Cut(strings.TrimPrefix(parts[2], "+"), ",")
			ns, err := strconv.Atoi(newStart)
			nc := 1
			if err == nil && hasNewCount {
				nc, err = strconv.Atoi(newCount)
			}
			if err != nil {
				return nil, fmt.Errorf("linecov: malformed new hunk %q", line)
			}
			cur.NewHunks = append(cur.NewHunks, Hunk{Start: ns, End: ns + nc - 1})
		}
	}
	return diffs, nil
}

// Reader returns a reader of paths at rev.
func Reader(ctx context.Context, root, rev string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		return gitOut(ctx, root, "show", rev+":"+p)
	}
}

// OIDs maps every path at rev to its blob OID.
func OIDs(ctx context.Context, root, rev string) (map[string]string, error) {
	out, err := gitOut(ctx, root, "ls-tree", "-r", "-z", "--full-tree", rev)
	if err != nil {
		return nil, err
	}
	oids := map[string]string{}
	for _, entry := range bytes.Split(out, []byte{0}) {
		meta, p, ok := bytes.Cut(entry, []byte{'\t'})
		f := strings.Fields(string(meta))
		if ok && len(f) == 3 && f[1] == "blob" {
			oids[string(p)] = f[2]
		}
	}
	return oids, nil
}

// HeadTests lists the top-level tests each package directory declares in the
// working tree under any build constraint: CI may run with tags (-race) or on
// a platform the selector does not, and an extra name only keeps a test. A
// directory with a _test.go file but no named test lists an empty, non-nil
// slice, as HeadTestNames does (GLS-V0-008).
func HeadTests(root string, dirs []string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
				continue
			}
			if out[dir] == nil {
				out[dir] = []string{} // a test binary, even without a named test
			}
			full := filepath.Join(root, filepath.FromSlash(dir))
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filepath.Join(full, name), nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, fmt.Errorf("linecov: parse %s/%s: %w", dir, name, err)
			}
			for _, d := range f.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok {
					if test, ok := topLevelTest(fn); ok {
						out[dir] = append(out[dir], test)
					}
				}
			}
		}
		sort.Strings(out[dir])
		out[dir] = slices.Compact(out[dir])
	}
	return out, nil
}

// BlobReader reads, in one `git cat-file --batch` process, the old-side blob
// of every Go path of diffs that has one, and returns a reader of them by
// path; a path it did not read is an error.
func BlobReader(ctx context.Context, root string, diffs []FileDiff) (func(string) ([]byte, error), error) {
	var oids []string
	paths := map[string]string{}
	for _, d := range diffs {
		if d.OldOID != "" && strings.HasSuffix(d.Path, ".go") {
			if _, ok := paths[d.Path]; !ok {
				oids = append(oids, d.OldOID)
				paths[d.Path] = d.OldOID
			}
		}
	}
	blobs := map[string][]byte{}
	if len(oids) > 0 {
		cmd := exec.CommandContext(ctx, "git", "-C", root, "cat-file", "--batch")
		cmd.Stdin = strings.NewReader(strings.Join(oids, "\n") + "\n")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git cat-file: %v: %s", err, strings.TrimSpace(stderr.String()))
		}
		for range oids {
			header, rest, ok := bytes.Cut(out, []byte{'\n'})
			f := strings.Fields(string(header))
			if !ok || len(f) != 3 || f[1] != "blob" {
				return nil, fmt.Errorf("git cat-file: unexpected header %q", header)
			}
			size, err := strconv.Atoi(f[2])
			if err != nil || size < 0 || size+1 > len(rest) {
				return nil, fmt.Errorf("git cat-file: bad size in %q", header)
			}
			blobs[f[0]] = rest[:size]
			out = rest[size+1:]
		}
	}
	return func(p string) ([]byte, error) {
		oid, ok := paths[p]
		if !ok {
			return nil, fmt.Errorf("linecov: no old blob of %s was read", p)
		}
		b, ok := blobs[oid]
		if !ok {
			return nil, fmt.Errorf("linecov: blob %s of %s missing", oid, p)
		}
		return b, nil
	}, nil
}
