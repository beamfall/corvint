package console

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// maxBlobBytes bounds one rendered file.
const maxBlobBytes = 2 << 20

// Worktree reads committed content from one repository. Every read names the
// immutable Git object it came from; the console never presents working-tree
// bytes as committed content (LAC-V0-019).
type Worktree struct {
	Root    string
	Timeout time.Duration
}

// Blob is one file's committed bytes and the object id they came from.
type Blob struct {
	Path      string
	ObjectID  string
	Revision  string
	Text      string
	Truncated bool
	Dirty     bool
	DirtyNote string
	Source    Source
	Err       string
}

// Revision is the commit the console is reading at, plus whether the
// worktree differs from it.
type Revision struct {
	Commit     string
	Dirty      bool
	DirtyPaths []string
	Err        string
}

// Head resolves the commit and reports whether the worktree is clean. A dirty
// worktree is labelled everywhere it matters rather than quietly ignored.
func (w Worktree) Head(ctx context.Context) Revision {
	commit, err := w.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return Revision{Err: err.Error()}
	}
	status, err := w.git(ctx, "status", "--porcelain")
	if err != nil {
		return Revision{Commit: strings.TrimSpace(commit), Err: err.Error()}
	}
	revision := Revision{Commit: strings.TrimSpace(commit)}
	// Only line ends are trimmed: the leading status column is part of each
	// porcelain record, and trimming it would cut the first path.
	for _, line := range strings.Split(strings.TrimRight(status, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		revision.Dirty = true
		if len(line) > 3 {
			revision.DirtyPaths = append(revision.DirtyPaths, strings.TrimSpace(line[3:]))
		}
	}
	return revision
}

// Read returns one path's content at the given revision, read through the
// object database. The object id is resolved first and the bytes are read
// from that id, so what is rendered is exactly what the id names.
func (w Worktree) Read(ctx context.Context, revision, path string) *Blob {
	blob := &Blob{
		Path: path, Revision: revision,
		Source: Source{
			Argv:     []string{"git", "-C", w.Root, "cat-file", "blob", revision + ":" + path},
			Worktree: w.Root, Revision: revision, ObservedAt: time.Now().UTC(), Axes: UnstatedAxes(),
		},
	}
	id, err := w.git(ctx, "rev-parse", revision+":"+path)
	if err != nil {
		blob.Err = err.Error()
		return blob
	}
	blob.ObjectID = strings.TrimSpace(id)
	// Read by object id, not by path: the id is what makes this immutable.
	content, err := w.git(ctx, "cat-file", "blob", blob.ObjectID)
	if err != nil {
		blob.Err = err.Error()
		return blob
	}
	if len(content) > maxBlobBytes {
		content = content[:maxBlobBytes]
		blob.Truncated = true
	}
	blob.Text = content
	// The committed bytes are what is shown; say so when the worktree differs.
	// A failed check is not a clean path: say the difference was not observed.
	status, statusErr := w.git(ctx, "status", "--porcelain", "--", path)
	switch {
	case statusErr != nil:
		blob.Dirty = true
		blob.DirtyNote = "whether the worktree copy of this path differs from " + blob.ObjectID + " was not observed (" + statusErr.Error() + "); the committed bytes are shown"
	case strings.TrimSpace(status) != "":
		blob.Dirty = true
		blob.DirtyNote = "the worktree copy of this path differs from " + blob.ObjectID + "; the committed bytes are shown"
	}
	// Content read at an immutable object id is observed by the owning
	// verifier: Git itself. That is stated by the source, not derived here.
	blob.Source.Axes.Set(AxisEpistemicClass, "OBSERVED")
	blob.Source.Axes.Set(AxisAuthorityClass, "OWNING_VERIFIER")
	blob.Source.Axes.Set(AxisValidity, "VALID")
	if blob.Truncated {
		blob.Source.Axes.Set(AxisCompleteness, "PARTIAL")
	} else {
		blob.Source.Axes.Set(AxisCompleteness, "COMPLETE")
	}
	return blob
}

func (w Worktree) git(ctx context.Context, args ...string) (string, error) {
	timeout := w.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	runner := func(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
		// --literal-pathspecs: a path this package passes after `--` (Read's
		// dirty check, a real repository path) must match by name, not as a
		// glob a differently-named dirty file can satisfy instead.
		argv := append([]string{"git", "--literal-pathspecs", "--no-pager", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "protocol.allow=never", "-C", root}, args...)
		out, _, err := runTool(ctx, root, gitEnvironment(), w.Timeout, limit, argv...)
		return out, err
	}
	if len(args) > 0 && args[0] == "status" {
		out, err := gitstatus.Status(runCtx, w.Root, maxEnvelopeBytes, runner, args...)
		return string(out), err
	}
	out, err := runner(runCtx, w.Root, maxEnvelopeBytes, args...)
	return string(out), err
}

type gitError struct{ message string }

func (e *gitError) Error() string { return e.message }

// maxListingEntries bounds one directory listing.
const maxListingEntries = 500

// Entry is one file in a tree, named by the immutable object id its bytes
// come from.
type Entry struct {
	Path     string
	Name     string
	ObjectID string
	Bytes    string
}

// Listing is one directory of a tree at one revision.
type Listing struct {
	Dir       string
	Revision  string
	Entries   []Entry
	Truncated bool
	Source    Source
	Err       string
}

// List reads one directory of the tree at revision. It lists the tree, not the
// worktree: an untracked file is not evidence at a revision, and a listing
// that mixed the two would name object ids for some entries and not others
// (LAC-V0-019).
func (w Worktree) List(ctx context.Context, revision, dir string) *Listing {
	listing := &Listing{
		Dir: dir, Revision: revision,
		Source: Source{
			Argv:     []string{"git", "-C", w.Root, "ls-tree", "-l", "-z", revision, dir + "/"},
			Worktree: w.Root, Revision: revision, ObservedAt: time.Now().UTC(), Axes: UnstatedAxes(),
		},
	}
	// -z: without it Git C-quotes a path holding a non-ASCII or special byte,
	// and the quoted spelling names no file the pane could then read.
	out, err := w.git(ctx, "ls-tree", "-l", "-z", revision, dir+"/")
	if err != nil {
		listing.Err = err.Error()
		return listing
	}
	for _, line := range strings.Split(out, "\x00") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entry, ok := parseTreeEntry(line)
		if !ok {
			continue
		}
		if len(listing.Entries) == maxListingEntries {
			listing.Truncated = true
			break
		}
		listing.Entries = append(listing.Entries, entry)
	}
	// A tree read at a revision is observed by Git, the owning verifier of
	// what that revision names. That is the source's standing, not a class
	// this package decided to assign.
	listing.Source.Axes.Set(AxisEpistemicClass, "OBSERVED")
	listing.Source.Axes.Set(AxisAuthorityClass, "OWNING_VERIFIER")
	listing.Source.Axes.Set(AxisValidity, "VALID")
	if listing.Truncated {
		listing.Source.Axes.Set(AxisCompleteness, "PARTIAL")
	} else {
		listing.Source.Axes.Set(AxisCompleteness, "COMPLETE")
	}
	return listing
}

// parseTreeEntry reads one `git ls-tree -l` row: mode, type, object id, size,
// then a tab and the path. Only blobs are listed; a subtree has no bytes to
// render and is not an entry.
func parseTreeEntry(line string) (Entry, bool) {
	head, path, found := strings.Cut(line, "\t")
	if !found {
		return Entry{}, false
	}
	fields := strings.Fields(head)
	if len(fields) != 4 || fields[1] != "blob" {
		return Entry{}, false
	}
	name := path
	if index := strings.LastIndex(path, "/"); index >= 0 {
		name = path[index+1:]
	}
	return Entry{Path: path, Name: name, ObjectID: fields[2], Bytes: fields[3]}, true
}

// names reports whether this listing named the path. A pane reads only what it
// listed, so a listing route cannot be used to read an arbitrary path.
func (l *Listing) names(path string) bool {
	for _, entry := range l.Entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

// RequireToplevel refuses a root that is not its repository's Git toplevel,
// so a plain directory inside a repository is never read as that repository
// (LAC-V0-031, decision 0255).
func RequireToplevel(ctx context.Context, root string, timeout time.Duration) error {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	out, err := (Worktree{Root: resolved, Timeout: timeout}).git(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New(root + " is not a Git repository: " + err.Error())
	}
	if toplevel := strings.TrimSuffix(out, "\n"); toplevel != resolved {
		return errors.New(root + " is not the Git toplevel; the repository root is " + toplevel)
	}
	return nil
}
