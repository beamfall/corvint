package postmergeworkflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	connector "github.com/Beamfall/corvint/internal/postmergeconnector"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const maxAuthoredFiles = 64
const maxAuthoredContent = 1 << 20

// validateAuthored applies PMR-V0-940: authored content stays inside the work
// the adapter itself reported. It is a scope bound, not content validation.
func validateAuthored(r Result) error {
	seen := map[string]bool{}
	for _, a := range r.Authored {
		if seen[a.Kind] || len(a.Files) == 0 || len(a.Files) > maxAuthoredFiles {
			return fmt.Errorf("authored-scope-invalid")
		}
		seen[a.Kind] = true
		paths := map[string]bool{}
		for _, file := range a.Files {
			if !validPath(file.Path) || paths[file.Path] || len(file.Content) > maxAuthoredContent {
				return fmt.Errorf("authored-scope-invalid")
			}
			paths[file.Path] = true
			switch a.Kind {
			case "docs":
				if !slices.Contains(r.DocumentationTargets, file.Path) {
					return fmt.Errorf("authored-scope-invalid")
				}
			case "tests":
				if len(r.TestGaps) == 0 || !testPath(file.Path) {
					return fmt.Errorf("authored-scope-invalid")
				}
			default:
				return fmt.Errorf("authored-scope-invalid")
			}
		}
	}
	return nil
}

func testPath(p string) bool {
	base := filepath.Base(p)
	for _, suffix := range []string{"_test.go", ".test.mjs", ".test.js", "_test.sh"} {
		if strings.HasSuffix(base, suffix) && base != suffix {
			return true
		}
	}
	return strings.HasPrefix(p, "testdata/") || strings.Contains(p, "/testdata/")
}

// authorCommits is the harness-owned author stage (PMR-V0-940). It creates one
// commit per authored entry in a private Git directory under scratch that reads
// the product's objects through alternates, and returns that directory and the
// connector drafts for those commits. The product worktree, index, refs and
// object contents are never written. Identity and date are fixed so both runs
// author the same revisions.
func authorCommits(ctx context.Context, root, scratch, draftURL string, change connector.Change, authored []Authored) (string, []connector.Draft, error) {
	if draftURL == "" {
		return "", nil, fmt.Errorf("draft-url-unpinned")
	}
	when, err := time.Parse(time.RFC3339, change.MergedAt)
	if err != nil {
		return "", nil, fmt.Errorf("authored-date-invalid")
	}
	date := "@" + strconv.FormatInt(when.Unix(), 10) + " +0000"
	common, err := authorGit(ctx, root, nil, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", nil, err
	}
	repo := filepath.Join(scratch, "authored")
	if _, err = authorGit(ctx, scratch, nil, nil, "init", "-q", repo); err != nil {
		return "", nil, err
	}
	gitDir := filepath.Join(repo, ".git")
	alternates := filepath.Join(gitDir, "objects", "info", "alternates")
	if err = os.WriteFile(alternates, []byte(filepath.Join(common, "objects")+"\n"), 0600); err != nil {
		return "", nil, fmt.Errorf("authored-repository-unavailable")
	}
	merge := change.Binding.Merge
	drafts := []connector.Draft{}
	for _, a := range authored {
		env := []string{"GIT_DIR=" + gitDir, "GIT_INDEX_FILE=" + filepath.Join(gitDir, "replay-index-"+a.Kind),
			"GIT_AUTHOR_NAME=corvint-replay", "GIT_AUTHOR_EMAIL=corvint-replay@example.invalid", "GIT_AUTHOR_DATE=" + date,
			"GIT_COMMITTER_NAME=corvint-replay", "GIT_COMMITTER_EMAIL=corvint-replay@example.invalid", "GIT_COMMITTER_DATE=" + date}
		if _, err = authorGit(ctx, repo, env, nil, "read-tree", merge); err != nil {
			return "", nil, err
		}
		var info strings.Builder
		for _, file := range a.Files {
			blob, e := authorGit(ctx, repo, env, []byte(file.Content), "hash-object", "-w", "--stdin")
			if e != nil {
				return "", nil, e
			}
			fmt.Fprintf(&info, "100644 %s\t%s\n", blob, file.Path)
		}
		if _, err = authorGit(ctx, repo, env, []byte(info.String()), "update-index", "--add", "--index-info"); err != nil {
			return "", nil, err
		}
		tree, e := authorGit(ctx, repo, env, nil, "write-tree")
		if e != nil {
			return "", nil, e
		}
		commit, e := authorGit(ctx, repo, env, nil, "commit-tree", tree, "-p", merge, "-m", "Post-merge "+a.Kind+" draft for "+change.Binding.Change)
		if e != nil {
			return "", nil, e
		}
		drafts = append(drafts, connector.Draft{Kind: a.Kind, Revision: commit, URL: draftURL})
	}
	return repo, drafts, nil
}

// authorGit runs Git with a fixed configuration-free environment and returns
// its trimmed standard output.
func authorGit(ctx context.Context, dir string, env []string, stdin []byte, args ...string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git-executable-unavailable")
	}
	base := []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_ALLOW_PROTOCOL=", "LANG=C", "LC_ALL=C"}
	argv := append([]string{git, "-c", "credential.helper=", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "core.hooksPath=/dev/null"}, args...)
	o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: append(base, env...), Stdin: stdin, Timeout: 10 * time.Second, ShutdownTimeout: time.Second, InputLimit: MaxBytes + 1, OutputLimit: 4096})
	if o.ExitStatus != 0 || o.Err != nil || o.OutputOverflow || o.TimedOut || o.Cancelled {
		return "", fmt.Errorf("authored-commit-unavailable")
	}
	return strings.TrimSpace(string(o.Stdout)), nil
}
