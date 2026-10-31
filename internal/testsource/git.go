// Package testsource is the static, read-only analysis of JavaScript and TypeScript test sources
// at a committed revision: `corvint tests lint` and `corvint tests duplicates`
// (docs/specs/test-source-analysis-v0.md, TSA-V0). It reads committed Git objects only, never the
// working tree or the network, and every read is bounded.
package testsource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	// MaxFiles bounds the test files one command reads (TSA-V0-012).
	MaxFiles = 5000
	// MaxFileBytes bounds one test file.
	MaxFileBytes = 1 << 20
	// maxTotalBytes bounds every blob one command reads together.
	maxTotalBytes = 64 << 20
	// maxDiffBytes bounds one `git diff` output.
	maxDiffBytes = 64 << 20
)

// Error is a refusal with a stable code (TSA-V0-012).
type Error struct {
	Code, Message string
	cause         error
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Unwrap names the process refusal behind a platform-refused Git read (V1-2487).
func (e *Error) Unwrap() error { return e.cause }

// runGit runs one Git read; tests replace it to force a process outcome.
var runGit = procgroup.Run

// platformRefusal reports a Git read refused by the platform, not by its input (PRS-V1-021,
// V1-2487); it wraps the procgroup refusal so the platform-refusal list names it.
func platformRefusal() error {
	return &Error{Code: "tests-source-unavailable", Message: "Git reads are unsupported on this platform: process-platform-unsupported",
		cause: procgroup.ErrPlatformUnsupported}
}

func refuse(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Repo reads committed objects of one repository.
type Repo struct {
	ctx  context.Context
	root string
}

// OpenRepo binds a repository root.
func OpenRepo(ctx context.Context, root string) Repo { return Repo{ctx: ctx, root: root} }

func (r Repo) git(limit int, stdin []byte, args ...string) ([]byte, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, refuse("tests-source-unavailable", "Git is unavailable")
	}
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.quotePath=false",
		"-c", "credential.helper=", "-C", r.root}, args...)
	o := runGit(r.ctx, procgroup.Spec{Argv: argv, Dir: r.root, Env: gokernel.SanitizedGitEnvironment(), Stdin: stdin,
		Timeout: 120 * time.Second, OutputLimit: limit})
	if errors.Is(o.Err, procgroup.ErrPlatformUnsupported) {
		return nil, platformRefusal()
	}
	if o.Err != nil || o.ExitStatus != 0 {
		return nil, refuse("tests-source-unavailable", "a Git read failed")
	}
	return o.Stdout, nil
}

// Resolve names the full commit a revision denotes.
func (r Repo) Resolve(revision string) (string, error) {
	if revision == "" || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\x00\n\r") {
		return "", refuse("tests-invalid-revision", "the revision is not a commit")
	}
	out, err := r.git(4096, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if errors.Is(err, procgroup.ErrPlatformUnsupported) {
		return "", err
	}
	if err != nil {
		return "", refuse("tests-invalid-revision", "the revision is not a commit")
	}
	return strings.TrimSpace(string(out)), nil
}

// entry is one committed regular file.
type entry struct {
	path, oid string
	size      int
}

// tree lists the committed regular files at rev that keep reports true (symlinks and submodules
// are never followed). More than limit kept files refuses.
func (r Repo) tree(rev string, keep func(string) bool, limit int) ([]entry, error) {
	out, err := r.git(maxDiffBytes, nil, "ls-tree", "-r", "-z", "-l", "--full-tree", rev)
	if err != nil {
		return nil, err
	}
	entries := []entry{}
	for _, line := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if len(line) == 0 {
			continue
		}
		meta, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 {
			return nil, refuse("tests-source-unavailable", "the tree is unreadable")
		}
		if fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") || !keep(name) {
			continue
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, refuse("tests-source-unavailable", "the tree is unreadable")
		}
		entries = append(entries, entry{path: name, oid: fields[2], size: size})
		if len(entries) > limit {
			return nil, refuse("tests-bound-exceeded", "more than %d test files in all", MaxFiles)
		}
	}
	return entries, nil
}

// blobs reads every entry through one `git cat-file --batch`, checking each size bound and that the
// entries' unique blobs fit in limit bytes; it returns the bytes they count against that limit.
func (r Repo) blobs(entries []entry, limit int) (map[string][]byte, int, error) {
	out := map[string][]byte{}
	if len(entries) == 0 {
		return out, 0, nil
	}
	sum, request, unique := 0, &bytes.Buffer{}, map[string]bool{}
	for _, e := range entries {
		if e.size > MaxFileBytes {
			return nil, 0, refuse("tests-bound-exceeded", "%s exceeds %d bytes", e.path, MaxFileBytes)
		}
		if !unique[e.oid] {
			unique[e.oid] = true
			sum += e.size + 128
			request.WriteString(e.oid + "\n")
		}
	}
	if sum > limit {
		return nil, 0, refuse("tests-bound-exceeded", "test sources exceed %d bytes", maxTotalBytes)
	}
	raw, err := r.git(sum+1024, request.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, 0, err
	}
	for len(raw) > 0 {
		header, rest, ok := bytes.Cut(raw, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			return nil, 0, refuse("tests-source-unavailable", "a committed source is unavailable")
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size+1 > len(rest) {
			return nil, 0, refuse("tests-source-unavailable", "a committed source is unavailable")
		}
		out[fields[0]] = rest[:size]
		raw = rest[size+1:]
	}
	for oid := range unique {
		if _, ok := out[oid]; !ok {
			return nil, 0, refuse("tests-source-unavailable", "a committed source is unavailable")
		}
	}
	return out, sum, nil
}

// readBudget is what remains of one command's file and byte bounds; every read of the command
// draws on the same budget, so the aggregate never exceeds MaxFiles files or maxTotalBytes bytes
// (TSA-V0-012, TSA-V0-030).
type readBudget struct{ files, bytes int }

func newReadBudget() *readBudget { return &readBudget{files: MaxFiles, bytes: maxTotalBytes} }

// readFiles reads the kept committed files at rev, keyed by path, within a fresh budget.
func (r Repo) readFiles(rev string, keep func(string) bool) (map[string]string, map[string]string, error) {
	return r.readFilesWithin(rev, keep, newReadBudget())
}

// readFilesWithin reads the kept committed files at rev and charges them to budget.
func (r Repo) readFilesWithin(rev string, keep func(string) bool, budget *readBudget) (map[string]string, map[string]string, error) {
	entries, err := r.tree(rev, keep, budget.files)
	if err != nil {
		return nil, nil, err
	}
	data, used, err := r.blobs(entries, budget.bytes)
	if err != nil {
		return nil, nil, err
	}
	budget.files -= len(entries)
	budget.bytes -= used
	texts, oids := map[string]string{}, map[string]string{}
	for _, e := range entries {
		texts[e.path], oids[e.path] = string(data[e.oid]), e.oid
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
func (r Repo) changedLines(base, rev string, keep func(string) bool) (map[string]*lineChanges, error) {
	inventory, err := r.git(maxDiffBytes, nil, append(append([]string{}, diffArgs...), "--name-only", "-z", base, rev)...)
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
	out, err := r.git(maxDiffBytes, nil, append(append([]string{}, diffArgs...), "--unified=0", base, rev)...)
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
func (r Repo) readOne(rev, name string, limit int) ([]byte, bool, error) {
	data, _, found, err := r.readOneBlob(rev, name, limit)
	return data, found, err
}

// readOneBlob is readOne that also returns the file's Git blob ID (TSA-V0-414).
func (r Repo) readOneBlob(rev, name string, limit int) ([]byte, string, bool, error) {
	if name == "" || strings.ContainsAny(name, "\x00\n\r") {
		return nil, "", false, errors.New("invalid path")
	}
	out, err := r.git(1<<16, nil, "ls-tree", "-z", "-l", "--full-tree", rev, "--", name)
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
	data, _, err := r.blobs([]entry{{path: name, oid: fields[2], size: size}}, maxTotalBytes)
	if err != nil {
		return nil, "", false, err
	}
	return data[fields[2]], fields[2], true, nil
}

var webExtensions = map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true, ".mts": true, ".cts": true}

// IsTestFile reports a JavaScript or TypeScript test file: a base name holding `.spec.` or
// `.test.` before its extension, or any such file under a `__tests__` directory, which Jest's
// default testMatch selects (TSA-V0-010, TSA-V0-388). Every JS/TS test-source reader shares it.
func IsTestFile(name string) bool {
	extension := path.Ext(name)
	if !webExtensions[extension] {
		return false
	}
	stem := strings.TrimSuffix(path.Base(name), extension)
	if strings.HasSuffix(stem, ".spec") || strings.HasSuffix(stem, ".test") {
		return true
	}
	for _, segment := range strings.Split(path.Dir(name), "/") {
		if segment == "__tests__" {
			return true
		}
	}
	return false
}

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
