package console

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// maxSpecIndexBytes bounds the generated requirement index and spec index.
const maxSpecIndexBytes = 8 << 20

// Requirement is one row of `docs/specs/REQUIREMENTS.tsv`: a requirement id,
// the spec file that defines it, and the line it is defined on.
type Requirement struct {
	ID    string
	File  string
	Line  int
	Title string
}

// SpecEntry is one spec's row in `docs/specs/INDEX.json`. Intent and delivery
// are the spec's own declared statuses; the console renders them beside the
// clause and never restates them as its own judgement (LAC-V0-018).
type SpecEntry struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	ReqPrefix string `json:"reqPrefix"`
	Intent    string `json:"intent"`
	Delivery  string `json:"delivery"`
	Claim     string `json:"claim"`
}

// SpecLookup resolves a requirement id to its clause and its owning spec's
// declared status. It reads the two generated indexes at request time and
// keeps nothing (LAC-V0-004).
type SpecLookup struct {
	Root string
}

// Clause is the answer for one requirement id.
type Clause struct {
	Requirement Requirement
	Spec        *SpecEntry
	Text        string
	Source      Source
	Err         string
}

// Resolve finds one requirement id. A missing index or a missing id is
// reported as such, never as an empty clause.
func (l SpecLookup) Resolve(id string) *Clause {
	clause := &Clause{
		Source: Source{
			Argv:     []string{"read", filepath.Join(l.Root, "docs/specs/REQUIREMENTS.tsv"), "and", filepath.Join(l.Root, "docs/specs/INDEX.json")},
			Worktree: l.Root,
			Axes:     UnstatedAxes(),
		},
	}
	requirements, err := l.requirements()
	if err != nil {
		clause.Err = err.Error()
		return clause
	}
	requirement, ok := requirements[id]
	if !ok {
		clause.Err = "requirement " + id + " is not in docs/specs/REQUIREMENTS.tsv"
		return clause
	}
	clause.Requirement = requirement

	if specs, err := l.specs(); err != nil {
		clause.Err = err.Error()
	} else {
		for i := range specs {
			if specs[i].Path == requirement.File {
				clause.Spec = &specs[i]
				// The spec states its own intent and delivery; the console
				// passes them through as the spec's declaration.
				clause.Source.Axes.Set(AxisEpistemicClass, "DECLARED")
				break
			}
		}
	}
	clause.Text, err = l.line(requirement.File, requirement.Line)
	if err != nil {
		clause.Err = err.Error()
	}
	return clause
}

func (l SpecLookup) requirements() (map[string]Requirement, error) {
	raw, err := readBounded(l.Root, filepath.Join("docs", "specs", "REQUIREMENTS.tsv"), maxSpecIndexBytes)
	if err != nil {
		return nil, err
	}
	out := map[string]Requirement{}
	for i, line := range strings.Split(string(raw), "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		number, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		out[fields[0]] = Requirement{ID: fields[0], File: fields[1], Line: number, Title: fields[3]}
	}
	return out, nil
}

func (l SpecLookup) specs() ([]SpecEntry, error) {
	raw, err := readBounded(l.Root, filepath.Join("docs", "specs", "INDEX.json"), maxSpecIndexBytes)
	if err != nil {
		return nil, err
	}
	var entries []SpecEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// cleanRelative rejects an absolute path or a path that escapes the console
// root once cleaned, mirroring internal/doccompiler.cleanRelative: the
// requirement's File field comes from a generated TSV row with no validation
// of its own, and the console is the one surface that renders
// repository-derived paths to a browser, so a `..` component must be refused
// rather than joined and opened.
func cleanRelative(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if filepath.IsAbs(path) {
		return "", errors.New("absolute path is not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." {
		return "", errors.New("path must name a contained file")
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the project")
	}
	return filepath.ToSlash(clean), nil
}

// line reads exactly the clause line a requirement is defined on. The file is
// read from the worktree because a requirement index is generated from the
// worktree; the code pane, which must be immutable, reads Git objects instead.
func (l SpecLookup) line(file string, number int) (string, error) {
	clean, err := cleanRelative(file)
	if err != nil {
		return "", err
	}
	raw, err := readBounded(l.Root, filepath.FromSlash(clean), maxSpecIndexBytes)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(raw), "\n")
	// A row naming a line the file does not have is a stale index: a gap,
	// never an empty clause presented as read (LAC-V0-008).
	if number < 1 || number > len(lines) {
		return "", errors.New(clean + " has no line " + strconv.Itoa(number) + "; docs/specs/REQUIREMENTS.tsv is stale")
	}
	return lines[number-1], nil
}

// errNotRegular reports that a path the console was asked to read is a
// symlink, device, pipe, or other non-regular file rather than the plain
// worktree file the caller named. A committed symlink (e.g. to /dev/zero)
// must never be followed: Lstat below sees the link itself, not its target,
// so it refuses before any open or read happens.
var errNotRegular = errors.New("refusing to read a non-regular file (symlink, device, or pipe)")

// readBounded reads name, a path relative to root, that the console was asked
// to render, refusing to follow a symlink to a device or a file outside the
// worktree and refusing to read past limit bytes. os.Root resolves every
// component beneath root, so a symlinked parent directory (a committed
// docs/specs or .corvint link) cannot lead outside it. Lstat sees the final
// component itself: if it names a symlink, this returns errNotRegular without
// ever opening or following it. The open is non-blocking, so a FIFO swapped in
// after the Lstat cannot block it, and the fstat after Open re-checks the same
// property against the file that was actually opened. io.LimitReader caps the
// read itself so a file whose size changes after the stat (or a special file
// that reports size 0 while streaming unbounded bytes, like /dev/zero) cannot
// exhaust memory.
func readBounded(root, name string, limit int64) ([]byte, error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	linkInfo, err := dir.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !linkInfo.Mode().IsRegular() {
		return nil, &os.PathError{Op: "read", Path: name, Err: errNotRegular}
	}
	file, err := dir.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &os.PathError{Op: "read", Path: name, Err: errNotRegular}
	}
	if info.Size() > limit {
		return nil, &os.PathError{Op: "read", Path: name, Err: os.ErrInvalid}
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &os.PathError{Op: "read", Path: name, Err: os.ErrInvalid}
	}
	return data, nil
}
