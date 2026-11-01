package genesis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// GuidanceSnapshot exposes the bounded immutable inventory to advisory composers.
// Mutable observations are limited to admission and the final consistency check.
type GuidanceSnapshot struct {
	Revision string
	Tree     string
	Records  []Record
	Blobs    map[string][]byte
	Omitted  int
	repo     *repository
	output   int
}

func OpenGuidance(ctx context.Context, root string) (*GuidanceSnapshot, error) {
	limits := defaultLimits()
	limits.MaxEntries = 4096
	limits.MaxTreeBytes = 8 << 20
	limits.MaxBlobBytes = 256 << 10
	limits.MaxTotalBlobBytes = 8 << 20
	limits.GitTimeout = 10 * time.Second
	limits.TotalTimeout = 60 * time.Second
	repo, err := openRepository(ctx, root, limits, "HEAD")
	if err != nil {
		return nil, err
	}
	// Reserve the eight-operation opening budget within the invocation total.
	repo.budget = gitrun.NewBudget(2040, 60*time.Second)
	s := &GuidanceSnapshot{repo: repo}
	layout, err := s.run(ctx, 8192, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	common := strings.TrimSpace(string(layout))
	for _, name := range []string{"info/grafts", "shallow"} {
		if _, err := os.Lstat(filepath.Join(common, name)); !os.IsNotExist(err) {
			return nil, fmt.Errorf("unsupported ancestry metadata: %s", name)
		}
	}
	s.Revision, s.Tree, _, err = repo.resolve(ctx, "HEAD")
	if err != nil {
		return nil, err
	}
	if err = s.Check(ctx); err != nil {
		return nil, err
	}
	format := "sha1"
	if len(s.Revision) == 64 {
		format = "sha256"
	}
	entries, total, err := repo.treeEntries(ctx, s.Revision, format)
	if err != nil {
		return nil, err
	}
	s.Omitted = total - len(entries)
	blobs, exhausted, err := repo.readBlobs(ctx, entries, nil)
	if err != nil {
		return nil, err
	}
	s.Blobs = blobs
	// Account for bounded inventory and batch output in the shared output allowance.
	for _, body := range blobs {
		s.output += len(body)
	}
	s.output += 9 << 20
	for _, entry := range entries {
		s.Records = append(s.Records, ClassifyEntry(entry, blobs, exhausted, nil, limits))
	}
	return s, nil
}

func (s *GuidanceSnapshot) run(ctx context.Context, limit int, args ...string) ([]byte, error) {
	if len(args) > 0 && args[0] == "status" {
		return gitstatus.Status(ctx, s.repo.root, limit, s.runAt, args...)
	}
	return s.runAt(ctx, s.repo.root, limit, args...)
}

// Isolated status probes share both the parent runner ceiling and the invocation
// output ledger; a larger requested capacity never expands either allowance.
func (s *GuidanceSnapshot) runAt(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
	limit = min(limit, s.repo.limits.MaxTreeBytes+int(s.repo.limits.MaxTotalBlobBytes))
	remaining := (64 << 20) - s.output
	if limit > remaining {
		limit = remaining
	}
	if limit <= 0 {
		return nil, fmt.Errorf("aggregate Git output budget exhausted")
	}
	private := *s.repo
	private.root = root
	raw, err := private.runRaw(ctx, limit, 0, nil, args...)
	s.output += len(raw)
	return raw, err
}

func (s *GuidanceSnapshot) Check(ctx context.Context) error {
	layout, err := s.run(ctx, 8192, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	for _, name := range []string{"info/grafts", "shallow"} {
		if _, err := os.Lstat(filepath.Join(strings.TrimSpace(string(layout)), name)); !os.IsNotExist(err) {
			return fmt.Errorf("unsupported ancestry metadata: %s", name)
		}
	}
	raw, err := s.run(ctx, 129, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(raw)) != s.Revision {
		return fmt.Errorf("HEAD drift")
	}
	raw, err = s.run(ctx, 8<<20, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(raw) != 0 {
		return dirtyWorktreeError(raw)
	}
	return nil
}

// dirtyStatusPathsShown bounds the paths a dirty-worktree refusal names.
const dirtyStatusPathsShown = 8

// dirtyWorktreeError names, from `git status --porcelain=v1 -z` output, the
// first dirtyStatusPathsShown entries and the total, so the refusal says which
// paths are dirty (RGV-V0-016, V1-2053). When any entry is untracked it also
// states that Git runs without global configuration (sanitizedGitEnvironment,
// decision 0158), so a path ignored only by a global excludes file counts as
// untracked, and names the fix.
func dirtyWorktreeError(raw []byte) error {
	fields := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	shown := make([]string, 0, dirtyStatusPathsShown)
	total, untracked := 0, false
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		status := entry[:2]
		if status[0] == 'R' || status[0] == 'C' {
			i++ // a rename or copy carries its origin path in the next field
		}
		untracked = untracked || status == "??"
		total++
		if len(shown) < dirtyStatusPathsShown {
			shown = append(shown, strings.TrimSpace(status)+" "+strconv.Quote(entry[3:]))
		}
	}
	detail := fmt.Sprintf("dirty worktree or status drift: %d path(s): %s", total, strings.Join(shown, ", "))
	if total > len(shown) {
		detail += fmt.Sprintf(" (+%d more)", total-len(shown))
	}
	if untracked {
		detail += "; Corvint runs Git without global configuration, so a path ignored only by a global excludes file (core.excludesFile or ~/.config/git/ignore) is untracked here: ignore it in a tracked .gitignore or in .git/info/exclude"
	}
	return errors.New(detail)
}

func (s *GuidanceSnapshot) RefTips(ctx context.Context) (string, error) {
	raw, err := s.run(ctx, 256<<10, "for-each-ref", "--sort=refname", "--format=%(refname) %(objectname)", "refs/heads/")
	if err != nil {
		return "", err
	}
	if bytes.Count(raw, []byte("\n")) > 1024 {
		return "", fmt.Errorf("local ref map exceeds 1024")
	}
	return string(raw), nil
}

func (s *GuidanceSnapshot) CurrentRef(ctx context.Context) (string, error) {
	raw, err := s.run(ctx, 4096, "rev-parse", "--symbolic-full-name", "HEAD")
	return strings.TrimSpace(string(raw)), err
}

// MergeBase is safe unguarded (EEP-V0-771): OpenGuidanceSnapshot and Check refuse a common Git
// directory holding info/grafts or shallow, the same files gitancestry.RewrittenAt reads.
func (s *GuidanceSnapshot) MergeBase(ctx context.Context, a, b string) (string, error) {
	raw, err := s.run(ctx, 129, "merge-base", a, b)
	return strings.TrimSpace(string(raw)), err
}

func (s *GuidanceSnapshot) Paths(ctx context.Context, a, b string) ([]string, error) {
	raw, err := s.run(ctx, 4<<20, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", a, b, "--")
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return []string{}, nil
	}
	if raw[len(raw)-1] != 0 {
		return nil, fmt.Errorf("malformed diff paths")
	}
	paths := strings.Split(string(raw[:len(raw)-1]), "\x00")
	if len(paths) > 16384 {
		return nil, fmt.Errorf("target path budget exceeded")
	}
	return paths, nil
}

// Materialize writes only admitted regular immutable blobs in private scratch.
// The caller owns removal; no repository file, symlink or command is executed.
func (s *GuidanceSnapshot) Materialize() (string, error) {
	root, err := os.MkdirTemp("", "corvint-guidance-")
	if err != nil {
		return "", err
	}
	for _, r := range s.Records {
		if r.Classification != "INCLUDED" {
			continue
		}
		name := filepath.Join(root, filepath.FromSlash(*r.Path))
		if err = os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			break
		}
		if err = os.WriteFile(name, s.Blobs[r.OID], 0600); err != nil {
			break
		}
	}
	if err != nil {
		if cleanupErr := os.RemoveAll(root); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("scratch cleanup failed for %q: %w", root, cleanupErr))
		}
		return "", err
	}
	return root, nil
}
