package testdiscovery

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
	// MaxTotalBytes bounds every blob one command reads together.
	MaxTotalBytes = 64 << 20
	// MaxDiffBytes bounds one `git diff` output.
	MaxDiffBytes = 64 << 20
)

// Error is a refusal with a stable code (TSA-V0-012).
type Error struct {
	Code, Message string
	cause         error
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Unwrap names the process refusal behind a platform-refused Git read (V1-2487).
func (e *Error) Unwrap() error { return e.cause }

// RunGit runs one Git read; tests replace it to force a process outcome.
var RunGit = procgroup.Run

// platformRefusal reports a Git read refused by the platform, not by its input (PRS-V1-021,
// V1-2487); it wraps the procgroup refusal so the platform-refusal list names it.
func platformRefusal() error {
	return &Error{Code: "tests-source-unavailable", Message: "Git reads are unsupported on this platform: process-platform-unsupported",
		cause: procgroup.ErrPlatformUnsupported}
}

func Refuse(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Repo reads committed objects of one repository.
type Repo struct {
	ctx  context.Context
	Root string
}

// OpenRepo binds a repository root.
func OpenRepo(ctx context.Context, root string) Repo { return Repo{ctx: ctx, Root: root} }

func (r Repo) Git(limit int, stdin []byte, args ...string) ([]byte, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, Refuse("tests-source-unavailable", "Git is unavailable")
	}
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.quotePath=false",
		"-c", "credential.helper=", "-C", r.Root}, args...)
	o := RunGit(r.ctx, procgroup.Spec{Argv: argv, Dir: r.Root, Env: gokernel.SanitizedGitEnvironment(), Stdin: stdin,
		Timeout: 120 * time.Second, OutputLimit: limit})
	if errors.Is(o.Err, procgroup.ErrPlatformUnsupported) {
		return nil, platformRefusal()
	}
	if o.Err != nil || o.ExitStatus != 0 {
		return nil, Refuse("tests-source-unavailable", "a Git read failed")
	}
	return o.Stdout, nil
}

// Resolve names the full commit a revision denotes.
func (r Repo) Resolve(revision string) (string, error) {
	if revision == "" || strings.HasPrefix(revision, "-") || strings.ContainsAny(revision, "\x00\n\r") {
		return "", Refuse("tests-invalid-revision", "the revision is not a commit")
	}
	out, err := r.Git(4096, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if errors.Is(err, procgroup.ErrPlatformUnsupported) {
		return "", err
	}
	if err != nil {
		return "", Refuse("tests-invalid-revision", "the revision is not a commit")
	}
	return strings.TrimSpace(string(out)), nil
}

// Entry is one committed regular file.
type Entry struct {
	Path, Oid string
	Size      int
}

// Tree lists the committed regular files at rev that keep reports true (symlinks and submodules
// are never followed). More than limit kept files refuses.
func (r Repo) Tree(rev string, keep func(string) bool, limit int) ([]Entry, error) {
	out, err := r.Git(MaxDiffBytes, nil, "ls-tree", "-r", "-z", "-l", "--full-tree", rev)
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	for _, line := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if len(line) == 0 {
			continue
		}
		meta, name, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 4 {
			return nil, Refuse("tests-source-unavailable", "the tree is unreadable")
		}
		if fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") || !keep(name) {
			continue
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, Refuse("tests-source-unavailable", "the tree is unreadable")
		}
		entries = append(entries, Entry{Path: name, Oid: fields[2], Size: size})
		if len(entries) > limit {
			return nil, Refuse("tests-bound-exceeded", "more than %d test files in all", MaxFiles)
		}
	}
	return entries, nil
}

// Blobs reads every entry through one `git cat-file --batch`, checking each size bound and that the
// entries' unique blobs fit in limit bytes; it returns the bytes they count against that limit.
func (r Repo) Blobs(entries []Entry, limit int) (map[string][]byte, int, error) {
	out := map[string][]byte{}
	if len(entries) == 0 {
		return out, 0, nil
	}
	sum, request, unique := 0, &bytes.Buffer{}, map[string]bool{}
	for _, e := range entries {
		if e.Size > MaxFileBytes {
			return nil, 0, Refuse("tests-bound-exceeded", "%s exceeds %d bytes", e.Path, MaxFileBytes)
		}
		if !unique[e.Oid] {
			unique[e.Oid] = true
			sum += e.Size + 128
			request.WriteString(e.Oid + "\n")
		}
	}
	if sum > limit {
		return nil, 0, Refuse("tests-bound-exceeded", "test sources exceed %d bytes", MaxTotalBytes)
	}
	raw, err := r.Git(sum+1024, request.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, 0, err
	}
	for len(raw) > 0 {
		header, rest, ok := bytes.Cut(raw, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || fields[1] != "blob" {
			return nil, 0, Refuse("tests-source-unavailable", "a committed source is unavailable")
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size+1 > len(rest) {
			return nil, 0, Refuse("tests-source-unavailable", "a committed source is unavailable")
		}
		out[fields[0]] = rest[:size]
		raw = rest[size+1:]
	}
	for oid := range unique {
		if _, ok := out[oid]; !ok {
			return nil, 0, Refuse("tests-source-unavailable", "a committed source is unavailable")
		}
	}
	return out, sum, nil
}

var WebExtensions = map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true, ".mts": true, ".cts": true}

// IsTestFile reports a JavaScript or TypeScript test file: a base name holding `.spec.` or
// `.test.` before its extension, or any such file under a `__tests__` directory, which Jest's
// default testMatch selects (TSA-V0-010, TSA-V0-388). Every JS/TS test-source reader shares it.
func IsTestFile(name string) bool {
	extension := path.Ext(name)
	if !WebExtensions[extension] {
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
