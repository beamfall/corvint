package affected

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

var (
	// ErrStatusUnavailable reports that the worktree status could not be
	// observed at all.
	ErrStatusUnavailable = errors.New("affected: worktree status is unavailable")
	// ErrStatusOverflow reports a status larger than the fixed bound. A
	// truncated status would understate the dirty set, so it fails closed.
	ErrStatusOverflow = errors.New("affected: worktree status exceeds the byte bound")
	// ErrStatusMalformed reports a status Corvint could not decode exactly.
	ErrStatusMalformed = errors.New("affected: worktree status is malformed")
)

const (
	// MaxStatusBytes bounds one status capture, including the overflow byte.
	MaxStatusBytes = 8 << 20
	// StatusDeadline is the fixed capture deadline.
	StatusDeadline = 10 * time.Second
)

// DirtyPaths captures the exact repository-relative path set Git reports as
// changed in the worktree at root.
//
// The capture fails closed: overflow, deadline, a Git failure, or an
// undecodable record returns an error rather than a short list that would
// silently narrow later selection. Ignored paths are deliberately not
// enumerated; they are unobserved, not proven absent.
//
// This function reads Git's report only. It never opens a worktree file.
func DirtyPaths(ctx context.Context, gitExecutable, root string) ([]string, error) {
	if gitExecutable == "" || root == "" {
		return nil, fmt.Errorf("%w: empty git executable or root", ErrStatusUnavailable)
	}
	deadline, cancel := context.WithTimeout(ctx, StatusDeadline)
	defer cancel()
	raw, err := gitstatus.Status(deadline, root, MaxStatusBytes, boundedGitRunner(gitExecutable),
		"status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=no")
	if cemcode.CodeOf(err) == cemcode.GitOutputExceeded || len(raw) >= MaxStatusBytes {
		return nil, ErrStatusOverflow
	}
	if err != nil {
		return nil, errors.Join(ErrStatusUnavailable, err)
	}
	return DecodeStatus(raw)
}

// boundedGitRunner returns the one Git invocation shape this package uses:
// hermetic configuration, no locks, no lazy fetch, a stdout bound, and the
// fixed per-operation deadline.
func boundedGitRunner(gitExecutable string) gitstatus.Runner {
	budget := gitrun.NewBudget(8, StatusDeadline)
	return func(ctx context.Context, directory string, limit int, args ...string) ([]byte, error) {
		prefix := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
			"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false", "-C", directory}
		environment := []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
			"GIT_NO_REPLACE_OBJECTS=1", "GIT_CEILING_DIRECTORIES=" + filepath.Dir(directory),
			"LANG=C", "LC_ALL=C",
		}
		return gitrun.Run(ctx, budget, gitrun.Options{Binary: gitExecutable, Dir: directory, Env: environment,
			StdoutLimit: min(limit, MaxStatusBytes), PerOpTimeout: StatusDeadline}, append(prefix, args...)...)
	}
}

// RangePaths captures the exact repository-relative path set whose content
// differs between the committed tree at base and the committed tree at HEAD.
//
// It is the committed counterpart of DirtyPaths and shares its bounds and its
// fail-closed rule: overflow, deadline, a Git failure, or an undecodable name
// returns an error, never a short list. Renames are not detected, so a moved
// file enters the set under both its old and new path, exactly as the status
// decoder admits both sides of a rename record. The diff compares two trees;
// it never reads the worktree.
func RangePaths(ctx context.Context, gitExecutable, root, base string) ([]string, error) {
	if gitExecutable == "" || root == "" || base == "" {
		return nil, fmt.Errorf("%w: empty git executable, root, or base", ErrStatusUnavailable)
	}
	deadline, cancel := context.WithTimeout(ctx, StatusDeadline)
	defer cancel()
	raw, err := boundedGitRunner(gitExecutable)(deadline, root, MaxStatusBytes,
		"diff", "--name-only", "-z", "--no-renames", "--no-color", base, "HEAD", "--")
	if cemcode.CodeOf(err) == cemcode.GitOutputExceeded || len(raw) >= MaxStatusBytes {
		return nil, ErrStatusOverflow
	}
	if err != nil {
		return nil, errors.Join(ErrStatusUnavailable, err)
	}
	return DecodeNameList(raw)
}

// DecodeNameList parses a NUL-delimited path list such as
// `git diff --name-only -z` emits, admitting only valid relative paths.
func DecodeNameList(raw []byte) ([]string, error) {
	fields := bytes.Split(raw, []byte{0})
	if len(fields) != 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		value := string(field)
		if !ValidRelativePath(value) {
			return nil, fmt.Errorf("%w: path %q", ErrStatusMalformed, value)
		}
		paths = append(paths, value)
	}
	return NormalizePaths(paths), nil
}

// Observation is a dirty path set some other component already observed, in
// the same repository, from the same Git status bytes this package would have
// captured itself.
//
// It exists so a caller that holds an authority observation does not observe
// the worktree a second time. A worktree is mutable: two captures are two
// different facts, and a plan built from the second cannot be attributed to
// the identity recorded from the first. A nil *Observation means no such
// observation is available, which is distinct from an observation whose path
// set is empty because the worktree is clean.
type Observation struct {
	Paths []string
}

// DirtyPathsFor returns the dirty set selection consumes, preferring an
// existing observation over capturing one.
//
// The observation is normalized, not re-derived: it is already the decoded
// output of a status capture that the observing authority bound its identity
// to. When none is offered, this falls back to DirtyPaths and its own
// fail-closed capture, so the selector still works standalone.
func DirtyPathsFor(ctx context.Context, gitExecutable, root string, observation *Observation) ([]string, error) {
	if observation == nil {
		return DirtyPaths(ctx, gitExecutable, root)
	}
	return NormalizePaths(observation.Paths), nil
}

// DecodeStatus parses the NUL-delimited porcelain v1 stream.
//
// Each record is two status codes, a space, and the path. A rename or copy
// record is followed by a second NUL-terminated field holding the original
// path; both paths enter the dirty set, because a rename changes the unit that
// lost the file as well as the one that gained it.
//
// It is exported because an authority that already captured the same status
// bytes must decode them the same way this package would. Sharing the decoder
// is what makes a published dirty set and a locally captured one the same
// object rather than two implementations that can drift apart on renames.
func DecodeStatus(raw []byte) ([]string, error) {
	fields := bytes.Split(raw, []byte{0})
	if len(fields) != 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	paths := make([]string, 0, len(fields))
	for index := 0; index < len(fields); index++ {
		record := string(fields[index])
		if len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("%w: record %q", ErrStatusMalformed, record)
		}
		codes, value := record[:2], record[3:]
		value = nestedRepositoryPath(codes, value)
		if !ValidRelativePath(value) {
			return nil, fmt.Errorf("%w: path %q", ErrStatusMalformed, value)
		}
		paths = append(paths, value)
		if !strings.ContainsAny(codes, "RC") {
			continue
		}
		index++
		if index >= len(fields) {
			return nil, fmt.Errorf("%w: rename record %q has no origin", ErrStatusMalformed, record)
		}
		origin := string(fields[index])
		if !ValidRelativePath(origin) {
			return nil, fmt.Errorf("%w: origin %q", ErrStatusMalformed, origin)
		}
		paths = append(paths, origin)
	}
	return NormalizePaths(paths), nil
}

// nestedRepositoryPath admits the one directory record Git emits under
// --untracked-files=all: a nested repository or linked worktree, listed as an
// untracked path with a trailing slash. The directory enters the dirty set as
// a path no plugin owns, so selection widens to UNKNOWN scope over it rather
// than the whole capture failing as malformed.
func nestedRepositoryPath(codes, value string) string {
	if codes == "??" && strings.HasSuffix(value, "/") {
		return strings.TrimSuffix(value, "/")
	}
	return value
}

// Overlay is an accepted unsaved-editor-buffer set: repository-relative paths
// whose in-editor bytes differ from the worktree.
//
// Corvint admits an overlay as a selection input only. This package never writes
// overlay bytes into the user's worktree; materializing them for execution is
// the runtime provider's job, under its own isolation.
type Overlay struct {
	Paths []string
}

// Union merges a Git-observed dirty set with an accepted overlay into the
// single sorted path set selection consumes.
//
// Selection is identity-free at this layer: an overlaid path is dirty whether
// or not Git already reported it, so a keystroke in an otherwise clean file
// selects exactly what saving that file would have selected.
func Union(dirty []string, overlay Overlay) []string {
	return NormalizePaths(append(append([]string(nil), dirty...), overlay.Paths...))
}

// DirectoriesOf returns the sorted set of directories containing the paths.
// It is the cheap invalidation key for plugins whose unit is a directory.
func DirectoriesOf(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	directories := make([]string, 0, len(paths))
	for _, value := range paths {
		directory := path.Dir(value)
		if seen[directory] {
			continue
		}
		seen[directory] = true
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	return directories
}
