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
	return parseDiff(out)
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
// a platform the selector does not, and an extra name only keeps a test.
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
