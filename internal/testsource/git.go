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
type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Code + ": " + e.Message }

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
	o := procgroup.Run(r.ctx, procgroup.Spec{Argv: argv, Dir: r.root, Env: gokernel.SanitizedGitEnvironment(), Stdin: stdin,
		Timeout: 120 * time.Second, OutputLimit: limit})
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
			return nil, refuse("tests-bound-exceeded", "more than %d test files", limit)
		}
	}
	return entries, nil
}

// blobs reads every entry through one `git cat-file --batch`, checking each size bound.
func (r Repo) blobs(entries []entry) (map[string][]byte, error) {
	out := map[string][]byte{}
	if len(entries) == 0 {
		return out, nil
	}
	sum, request, unique := 0, &bytes.Buffer{}, map[string]bool{}
	for _, e := range entries {
		if e.size > MaxFileBytes {
			return nil, refuse("tests-bound-exceeded", "%s exceeds %d bytes", e.path, MaxFileBytes)
		}
		if !unique[e.oid] {
			unique[e.oid] = true
			sum += e.size + 128
			request.WriteString(e.oid + "\n")
		}
	}
	if sum > maxTotalBytes {
		return nil, refuse("tests-bound-exceeded", "test sources exceed %d bytes", maxTotalBytes)
	}
	raw, err := r.git(sum+1024, request.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	for len(raw) > 0 {
		header, rest, ok := bytes.Cut(raw, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			return nil, refuse("tests-source-unavailable", "a committed source is unavailable")
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size+1 > len(rest) {
			return nil, refuse("tests-source-unavailable", "a committed source is unavailable")
		}
		out[fields[0]] = rest[:size]
		raw = rest[size+1:]
	}
	for oid := range unique {
		if _, ok := out[oid]; !ok {
			return nil, refuse("tests-source-unavailable", "a committed source is unavailable")
		}
	}
	return out, nil
}

// readFiles reads the kept committed files at rev, keyed by path.
func (r Repo) readFiles(rev string, keep func(string) bool) (map[string]string, map[string]string, error) {
	entries, err := r.tree(rev, keep, MaxFiles)
	if err != nil {
		return nil, nil, err
	}
	data, err := r.blobs(entries)
	if err != nil {
		return nil, nil, err
	}
	texts, oids := map[string]string{}, map[string]string{}
	for _, e := range entries {
		texts[e.path], oids[e.path] = string(data[e.oid]), e.oid
	}
	return texts, oids, nil
}

// changedLines returns, for every file added or modified from base to rev that keep accepts, the
// set of new-side line numbers the diff adds or changes. A rename is a delete and an add.
func (r Repo) changedLines(base, rev string, keep func(string) bool) (map[string]map[int]bool, error) {
	out, err := r.git(maxDiffBytes, nil, "diff", "--no-ext-diff", "--no-color", "--no-renames", "--no-textconv",
		"--unified=0", "--diff-filter=AM", base, rev)
	if err != nil {
		return nil, err
	}
	changed, current := map[string]map[int]bool{}, ""
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			current = ""
		case strings.HasPrefix(line, "+++ "):
			name := strings.TrimPrefix(line, "+++ ")
			if name == "/dev/null" {
				continue
			}
			if !strings.HasPrefix(name, "b/") {
				return nil, refuse("tests-source-unavailable", "a changed path cannot be read from the diff")
			}
			current = ""
			if name = strings.TrimPrefix(name, "b/"); keep(name) {
				current = name
				if changed[current] == nil {
					changed[current] = map[int]bool{}
				}
				if len(changed) > MaxFiles {
					return nil, refuse("tests-bound-exceeded", "more than %d changed test files", MaxFiles)
				}
			}
		case strings.HasPrefix(line, "@@ ") && current != "":
			start, count, ok := hunkNewSide(line)
			if !ok {
				return nil, refuse("tests-source-unavailable", "a diff hunk cannot be read")
			}
			for n := start; n < start+count; n++ {
				changed[current][n] = true
			}
		}
	}
	return changed, nil
}

// hunkNewSide reads `@@ -a[,b] +c[,d] @@`: the new-side start line and line count.
func hunkNewSide(line string) (int, int, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, false
	}
	startText, countText, hasCount := strings.Cut(strings.TrimPrefix(fields[2], "+"), ",")
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
	if name == "" || strings.ContainsAny(name, "\x00\n\r") {
		return nil, false, errors.New("invalid path")
	}
	out, err := r.git(1<<16, nil, "ls-tree", "-z", "-l", "--full-tree", rev, "--", name)
	if err != nil {
		return nil, false, err
	}
	meta, listed, ok := strings.Cut(strings.TrimSuffix(string(out), "\x00"), "\t")
	if !ok || listed != name {
		return nil, false, nil
	}
	fields := strings.Fields(meta)
	if len(fields) != 4 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
		return nil, false, refuse("tests-source-unavailable", "%s is not a regular committed file", name)
	}
	size, err := strconv.Atoi(fields[3])
	if err != nil || size > limit {
		return nil, false, refuse("tests-bound-exceeded", "%s exceeds %d bytes", name, limit)
	}
	data, err := r.blobs([]entry{{path: name, oid: fields[2], size: size}})
	if err != nil {
		return nil, false, err
	}
	return data[fields[2]], true, nil
}

var webExtensions = map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true, ".mts": true, ".cts": true}

// IsTestFile reports a JavaScript or TypeScript test file: a base name holding `.spec.` or
// `.test.` before its extension (TSA-V0-010).
func IsTestFile(name string) bool {
	extension := path.Ext(name)
	if !webExtensions[extension] {
		return false
	}
	stem := strings.TrimSuffix(path.Base(name), extension)
	return strings.HasSuffix(stem, ".spec") || strings.HasSuffix(stem, ".test")
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
