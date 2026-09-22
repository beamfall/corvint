// Package gitauth is the isolated Git authority kernel for CEM: it validates
// the repository boundary (reciprocal worktree metadata, alternates denial,
// attribute isolation), resolves revisions, reads tree entries and blobs
// through Git object identity, and derives canonical CEM 0.2 patch bytes that
// are independent of repository configuration, attributes, diff drivers,
// object redirection, shallow/partial state, and linked-worktree layout.
package gitauth

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// Bounded metadata limits.
const (
	maxGitfileBytes    = 4096
	maxAttributesBytes = 64 << 10
	// MaxBlobBytes bounds one blob read; MaxTotalBlobBytes bounds one
	// verification's distinct blob bytes.
	MaxBlobBytes      = 64 << 20
	MaxTotalBlobBytes = 128 << 20
	MaxTreeBytes      = 4 << 20 // every tree body read for one verified path lookup
	MaxTreeDepth      = 128     // directory levels the canonical change-set walk descends; well inside gitrun.DefaultOperations
)

func unavailable(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.RepositoryObjectUnavailable, format, args...)
}

// Repository is a validated repository boundary.
type Repository struct {
	Root         string
	GitDir       string // per-worktree Git directory
	CommonDir    string // common Git directory holding the object store
	ObjectFormat string // "sha1" or "sha256"
	budget       *gitrun.Budget
	blobBytes    int64            // distinct blob bytes charged so far
	chargedOids  map[string]bool  // blob OIDs already charged to the budget
	requestMemo  *RequestReadMemo // request-local immutable successes only
	objectView   *Repository      // admitted immutable objects for live currentness only
	objects      *gitrun.Session  // scoped cat-file co-process; nil means one-shot reads
}

// Open validates root as a primary or reciprocally-linked worktree, denies
// alternates and un-isolatable repository attributes, and pins every later Git
// operation to the resolved administrative directory.
//
// Validation order is frozen by the 0.2 spec: worktree/reciprocal validation
// (repository-object-unavailable) precedes alternate denial
// (unsupported-object-alternates), which precedes any object read.
func Open(root string, budget *gitrun.Budget) (*Repository, error) {
	resolved, err := resolveRoot(root)
	if err != nil {
		return nil, err
	}
	repository := &Repository{Root: resolved, budget: budget}
	if err := repository.resolveGitDirs(); err != nil {
		return nil, err
	}
	if err := repository.denyAlternates(); err != nil {
		return nil, err
	}
	if err := repository.denyRepositoryAttributes(); err != nil {
		return nil, err
	}
	return repository, nil
}

func resolveRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", unavailable("root path cannot be made absolute")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", unavailable("root path does not resolve")
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.IsDir() {
		return "", unavailable("root is not a directory")
	}
	return resolved, nil
}

func (r *Repository) resolveGitDirs() error {
	marker := filepath.Join(r.Root, ".git")
	info, err := os.Lstat(marker)
	if err != nil {
		return unavailable("root has no .git marker")
	}
	switch {
	case info.IsDir():
		r.GitDir, r.CommonDir = marker, marker
		return nil
	case info.Mode().IsRegular():
		return r.resolveLinkedWorktree(marker)
	default:
		return unavailable(".git marker is neither a directory nor a regular file")
	}
}

// resolveLinkedWorktree validates the gitfile chain with bounded no-follow
// reads and requires Git's per-worktree metadata to reciprocally identify the
// requested root before its common object store becomes authority.
func (r *Repository) resolveLinkedWorktree(marker string) error {
	content, err := readBoundedMetadata(marker, maxGitfileBytes)
	if err != nil {
		return err
	}
	gitDir, err := parseGitfile(string(content), r.Root)
	if err != nil {
		return err
	}
	gitDirInfo, err := os.Lstat(gitDir)
	if err != nil || !gitDirInfo.IsDir() {
		return unavailable("linked worktree Git directory does not resolve")
	}
	backPointer, err := readBoundedMetadata(filepath.Join(gitDir, "gitdir"), maxGitfileBytes)
	if err != nil {
		return err
	}
	if err := requireReciprocal(strings.TrimSuffix(string(backPointer), "\n"), marker, r.Root); err != nil {
		return err
	}
	commonDir := gitDir
	commonPath := filepath.Join(gitDir, "commondir")
	if _, statErr := os.Lstat(commonPath); statErr == nil {
		pointer, err := readBoundedMetadata(commonPath, maxGitfileBytes)
		if err != nil {
			return err
		}
		commonDir = filepath.Clean(filepath.Join(gitDir, strings.TrimSuffix(string(pointer), "\n")))
	}
	commonInfo, err := os.Lstat(commonDir)
	if err != nil || !commonInfo.IsDir() {
		return unavailable("common Git directory does not resolve")
	}
	r.GitDir, r.CommonDir = gitDir, commonDir
	return nil
}

func parseGitfile(content, root string) (string, error) {
	line := strings.TrimSuffix(content, "\n")
	target, hasPrefix := strings.CutPrefix(line, "gitdir: ")
	if !hasPrefix || target == "" || strings.ContainsAny(target, "\n\x00") {
		return "", unavailable(".git gitfile is malformed")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return filepath.Clean(target), nil
}

// requireReciprocal requires the per-worktree gitdir back-pointer to identify
// the exact requested root's .git marker.
func requireReciprocal(recorded, marker, root string) error {
	if recorded == "" {
		return unavailable("per-worktree gitdir back-pointer is empty")
	}
	recordedInfo, err := os.Stat(filepath.Clean(recorded))
	if err != nil {
		return unavailable("per-worktree gitdir back-pointer does not resolve")
	}
	markerInfo, err := os.Stat(marker)
	if err != nil {
		return unavailable(".git marker does not resolve")
	}
	if !os.SameFile(recordedInfo, markerInfo) {
		return unavailable("worktree metadata does not reciprocally identify this root")
	}
	parentInfo, err := os.Stat(filepath.Dir(filepath.Clean(recorded)))
	if err != nil {
		return unavailable("per-worktree gitdir parent does not resolve")
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return unavailable("root does not resolve")
	}
	if !os.SameFile(parentInfo, rootInfo) {
		return unavailable("worktree metadata does not reciprocally identify this root")
	}
	return nil
}

// denyAlternates rejects ambient and repository object alternates and closes
// the objects/info and objects/pack topology with no-follow checks.
func (r *Repository) denyAlternates() error {
	objects := filepath.Join(r.CommonDir, "objects")
	if err := requireRealDir(objects, true); err != nil {
		return err
	}
	if err := requireRealDir(filepath.Join(objects, "info"), false); err != nil {
		return err
	}
	if err := requireRealDir(filepath.Join(objects, "pack"), false); err != nil {
		return err
	}
	if value := os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES"); value != "" {
		return cemcode.New(cemcode.UnsupportedObjectAlternates,
			"ambient GIT_ALTERNATE_OBJECT_DIRECTORIES is not supported")
	}
	alternates := filepath.Join(objects, "info", "alternates")
	if _, err := os.Lstat(alternates); err == nil {
		return cemcode.New(cemcode.UnsupportedObjectAlternates,
			"repository objects/info/alternates is not supported")
	}
	return nil
}

func requireRealDir(path string, required bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		if required {
			return unavailable("%s is missing", filepath.Base(path))
		}
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return unavailable("%s is not a real directory", filepath.Base(path))
	}
	return nil
}

// denyRepositoryAttributes fails closed on a repository-local attributes file
// with effective content: $GIT_DIR/info/attributes overrides every neutralizable
// attribute source, so canonical bytes cannot be made independent of it. This
// is a documented operational refusal, not protocol invalidity.
func (r *Repository) denyRepositoryAttributes() error {
	for _, directory := range attributeDirs(r.GitDir, r.CommonDir) {
		path := filepath.Join(directory, "info", "attributes")
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			return unavailable("info/attributes is not a regular file")
		}
		content, err := readBoundedMetadata(path, maxAttributesBytes)
		if err != nil {
			return err
		}
		if hasEffectiveAttributeLine(string(content)) {
			return cemcode.New(cemcode.UnsupportedRepositoryAttributes,
				"repository info/attributes carries effective attribute lines; canonical derivation cannot isolate it")
		}
	}
	return nil
}

func attributeDirs(gitDir, commonDir string) []string {
	if gitDir == commonDir {
		return []string{commonDir}
	}
	return []string{commonDir, gitDir}
}

func hasEffectiveAttributeLine(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return true
		}
	}
	return false
}
