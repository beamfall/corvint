// Package testsource is the static, read-only analysis of JavaScript and TypeScript test sources
// at a committed revision: `corvint tests lint` and `corvint tests duplicates`
// (docs/specs/test-source-analysis-v0.md, TSA-V0). It reads committed Git objects only, never the
// working tree or the network, and every read is bounded.
package testsource

import (
	"context"
	"errors"
	"path"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/testdiscovery"
)

// MaxFiles is testdiscovery.MaxFiles, kept under its old name.
const MaxFiles = testdiscovery.MaxFiles

// MaxFileBytes is testdiscovery.MaxFileBytes, kept under its old name.
const MaxFileBytes = testdiscovery.MaxFileBytes

// maxTotalBytes is testdiscovery.MaxTotalBytes, kept under its old name.
const maxTotalBytes = testdiscovery.MaxTotalBytes

// maxDiffBytes is testdiscovery.MaxDiffBytes, kept under its old name.
const maxDiffBytes = testdiscovery.MaxDiffBytes

// Error is testdiscovery.Error, kept under its old name.
type Error = testdiscovery.Error

// refuse is testdiscovery.Refuse, kept under its old name.
func refuse(code, format string, args ...any) error {
	return testdiscovery.Refuse(code, format, args...)
}

// Repo is testdiscovery.Repo, kept under its old name.
type Repo = testdiscovery.Repo

// OpenRepo is testdiscovery.OpenRepo, kept under its old name.
func OpenRepo(ctx context.Context, root string) Repo { return testdiscovery.OpenRepo(ctx, root) }

// entry is testdiscovery.Entry, kept under its old name.
type entry = testdiscovery.Entry

// readBudget is what remains of one command's file and byte bounds; every read of the command
// draws on the same budget, so the aggregate never exceeds MaxFiles files or maxTotalBytes bytes
// (TSA-V0-012, TSA-V0-030).
type readBudget struct{ files, bytes int }

func newReadBudget() *readBudget { return &readBudget{files: MaxFiles, bytes: maxTotalBytes} }

// readFiles reads the kept committed files at rev, keyed by path, within a fresh budget.
func readFiles(r Repo, rev string, keep func(string) bool) (map[string]string, map[string]string, error) {
	return readFilesWithin(r, rev, keep, newReadBudget())
}

// readFilesWithin reads the kept committed files at rev and charges them to budget.
func readFilesWithin(r Repo, rev string, keep func(string) bool, budget *readBudget) (map[string]string, map[string]string, error) {
	entries, err := r.Tree(rev, keep, budget.files)
	if err != nil {
		return nil, nil, err
	}
	data, used, err := r.Blobs(entries, budget.bytes)
	if err != nil {
		return nil, nil, err
	}
	budget.files -= len(entries)
	budget.bytes -= used
	texts, oids := map[string]string{}, map[string]string{}
	for _, e := range entries {
		texts[e.Path], oids[e.Path] = string(data[e.Oid]), e.Oid
	}
	return texts, oids, nil
}

// lineChanges is what a diff changed in one file: the new-side lines it adds or changes, and the
// new-side lines a deletion-only hunk follows (a deletion between line n and n+1 is recorded as n).
type lineChanges struct {
	lines, deletedAfter map[int]bool
}

// touches reports whether the change adds or changes a line in [start, end], or deletes lines
// strictly inside it (TSA-V0-012, TSA-V0-021).
func (c *lineChanges) touches(start, end int) bool {
	if c == nil {
		return false
	}
	for line := start; line <= end; line++ {
		if c.lines[line] || (line < end && c.deletedAfter[line]) {
			return true
		}
	}
	return false
}

// diffArgs makes every changed-path read independent of Git attributes, repository diff
// configuration and external drivers: textual hunks always, fixed a/ and b/ prefixes (TSA-V0-010).
var diffArgs = []string{"diff", "--no-ext-diff", "--no-color", "--no-renames", "--no-textconv", "--text",
	"--src-prefix=a/", "--dst-prefix=b/", "--diff-filter=AM"}

// changedLines returns, for every file added or modified from base to rev that keep accepts, what
// the diff changed. Paths come from a NUL-delimited inventory; every patch header must decode to an
// inventory path. A rename is a delete and an add.
func changedLines(r Repo, base, rev string, keep func(string) bool) (map[string]*lineChanges, error) {
	inventory, err := r.Git(maxDiffBytes, nil, append(append([]string{}, diffArgs...), "--name-only", "-z", base, rev)...)
	if err != nil {
		return nil, err
	}
	changed, listed := map[string]*lineChanges{}, map[string]bool{}
	for _, name := range strings.Split(strings.TrimSuffix(string(inventory), "\x00"), "\x00") {
		if name == "" {
			continue
		}
		listed[name] = true
		if keep(name) {
			changed[name] = &lineChanges{lines: map[int]bool{}, deletedAfter: map[int]bool{}}
			if len(changed) > MaxFiles {
				return nil, refuse("tests-bound-exceeded", "more than %d changed test files", MaxFiles)
			}
		}
	}
	if len(changed) == 0 {
		return changed, nil
	}
	out, err := r.Git(maxDiffBytes, nil, append(append([]string{}, diffArgs...), "--unified=0", base, rev)...)
	if err != nil {
		return nil, err
	}
	var current *lineChanges
	remaining := 0
	for _, line := range strings.Split(string(out), "\n") {
		if remaining > 0 {
			if strings.HasPrefix(line, "\\") {
				continue
			}
			remaining--
			continue
		}
		switch {
		case strings.HasPrefix(line, "diff --git "):
			current = nil
		case strings.HasPrefix(line, "+++ "):
			name, ok := patchPath(strings.TrimPrefix(line, "+++ "))
			if !ok || !listed[name] {
				return nil, refuse("tests-source-unavailable", "a changed path cannot be read from the diff")
			}
			current = changed[name]
		case strings.HasPrefix(line, "@@ "):
			oldCount, start, count, ok := hunkCounts(line)
			if !ok {
				return nil, refuse("tests-source-unavailable", "a diff hunk cannot be read")
			}
			remaining = oldCount + count
			if current == nil {
				continue
			}
			if count == 0 {
				current.deletedAfter[start] = true
			}
			for n := start; n < start+count; n++ {
				current.lines[n] = true
			}
		}
	}
	if remaining > 0 {
		return nil, refuse("tests-source-unavailable", "a diff hunk cannot be read")
	}
	return changed, nil
}

// patchPath decodes a `+++` header path: Git appends a tab after a name holding a space and
// C-quotes a name holding a quote, backslash or control byte.
func patchPath(header string) (string, bool) {
	header, tabbed := strings.CutSuffix(header, "\t")
	if strings.HasPrefix(header, "\"") {
		unquoted, err := strconv.Unquote(header)
		if err != nil {
			return "", false
		}
		header = unquoted
	}
	if strings.Contains(header, " ") != tabbed {
		return "", false
	}
	name, ok := strings.CutPrefix(header, "b/")
	return name, ok && name != ""
}

// hunkCounts reads `@@ -a[,b] +c[,d] @@`: the old-side line count and the new-side start line and
// line count.
func hunkCounts(line string) (int, int, int, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 || !strings.HasPrefix(fields[1], "-") || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, 0, false
	}
	_, oldCount, ok := hunkRange(strings.TrimPrefix(fields[1], "-"))
	if !ok {
		return 0, 0, 0, false
	}
	start, count, ok := hunkRange(strings.TrimPrefix(fields[2], "+"))
	return oldCount, start, count, ok
}

func hunkRange(text string) (int, int, bool) {
	startText, countText, hasCount := strings.Cut(text, ",")
	start, err := strconv.Atoi(startText)
	if err != nil || start < 0 {
		return 0, 0, false
	}
	count := 1
	if hasCount {
		if count, err = strconv.Atoi(countText); err != nil || count < 0 || count > 1<<24 {
			return 0, 0, false
		}
	}
	return start, count, true
}

// readOne reads one committed regular file at rev, or reports it absent.
func readOne(r Repo, rev, name string, limit int) ([]byte, bool, error) {
	data, _, found, err := readOneBlob(r, rev, name, limit)
	return data, found, err
}

// readOneBlob is readOne that also returns the file's Git blob ID (TSA-V0-414).
func readOneBlob(r Repo, rev, name string, limit int) ([]byte, string, bool, error) {
	if name == "" || strings.ContainsAny(name, "\x00\n\r") {
		return nil, "", false, errors.New("invalid path")
	}
	out, err := r.Git(1<<16, nil, "ls-tree", "-z", "-l", "--full-tree", rev, "--", name)
	if err != nil {
		return nil, "", false, err
	}
	meta, listed, ok := strings.Cut(strings.TrimSuffix(string(out), "\x00"), "\t")
	if !ok || listed != name {
		return nil, "", false, nil
	}
	fields := strings.Fields(meta)
	if len(fields) != 4 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
		return nil, "", false, refuse("tests-source-unavailable", "%s is not a regular committed file", name)
	}
	size, err := strconv.Atoi(fields[3])
	if err != nil || size > limit {
		return nil, "", false, refuse("tests-bound-exceeded", "%s exceeds %d bytes", name, limit)
	}
	data, _, err := r.Blobs([]entry{{Path: name, Oid: fields[2], Size: size}}, maxTotalBytes)
	if err != nil {
		return nil, "", false, err
	}
	return data[fields[2]], fields[2], true, nil
}

// webExtensions is testdiscovery.WebExtensions, kept under its old name.
var webExtensions = testdiscovery.WebExtensions

// IsTestFile is testdiscovery.IsTestFile, kept under its old name.
func IsTestFile(name string) bool { return testdiscovery.IsTestFile(name) }

// IsPageObjectFile reports a JavaScript or TypeScript page object: a file under a directory named
// pages, page-objects, pageobjects or pom, or whose base name ends in `.page` or `Page`.
func IsPageObjectFile(name string) bool {
	extension := path.Ext(name)
	if !webExtensions[extension] {
		return false
	}
	stem := strings.TrimSuffix(path.Base(name), extension)
	if strings.HasSuffix(stem, ".page") || strings.HasSuffix(stem, "Page") {
		return true
	}
	for _, segment := range strings.Split(path.Dir(name), "/") {
		switch strings.ToLower(segment) {
		case "pages", "page-objects", "pageobjects", "pom":
			return true
		}
	}
	return false
}
