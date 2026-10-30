package sharedpath

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// SplitProfile is the split-shared result profile (SPC-V0-003).
const SplitProfile = "corvint-shared-path-split/0"

// manualFollowUps are the only policy elements the split cannot produce.
var manualFollowUps = []string{"notes-listing", "run-citation"}

// Split is one mixed commit and, once applied, its two replacements.
type Split struct {
	Commit       string   `json:"commit"`
	SharedPaths  []string `json:"sharedPaths"`
	SharedCommit string   `json:"sharedCommit,omitempty"`
	RestCommit   string   `json:"restCommit,omitempty"`
}

// SplitResult is the split-shared plan or applied rewrite.
type SplitResult struct {
	Profile         string   `json:"profile"`
	Applied         bool     `json:"applied"`
	Mutates         bool     `json:"mutates"`
	Pushed          bool     `json:"pushed"`
	Branch          string   `json:"branch"`
	Base            string   `json:"base"`
	OldHead         string   `json:"oldHead"`
	NewHead         string   `json:"newHead,omitempty"`
	FinalTree       string   `json:"finalTree"`
	Splits          []Split  `json:"splits"`
	Rewritten       int      `json:"rewritten"`
	ManualFollowUps []string `json:"manualFollowUps"`
	Restore         string   `json:"restore,omitempty"`
}

// SplitRequest names the checkout, the local branch to rewrite, the base and,
// for an applied split, the head the caller expects.
type SplitRequest struct {
	Root, Base, Branch, ExpectHead string
	Apply                          bool
	Config                         Config
}

// SplitShared plans, and with Apply performs, the mechanical split: every
// commit of base..branch that changes shared and non-shared paths becomes a
// shared-only commit followed by the rest, the final tree stays identical,
// and the branch moves by a compare-and-swap on the expected head. It never
// pushes, fetches or touches any other ref.
func SplitShared(ctx context.Context, request SplitRequest) (SplitResult, error) {
	g := git{ctx: ctx, root: request.Root}
	if _, code, err := g.run(nil, nil, "check-ref-format", "refs/heads/"+request.Branch); err != nil || code != 0 || request.Branch == "" || strings.HasPrefix(request.Branch, "-") {
		return SplitResult{}, errors.New(CodeInvalidArguments)
	}
	if err := refuseLiveSession(g, request.Config); err != nil {
		return SplitResult{}, err
	}
	symbolic, code, err := g.run(nil, nil, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return SplitResult{}, err
	}
	if code != 0 {
		return SplitResult{}, errors.New(CodeDetachedHead)
	}
	if strings.TrimSpace(string(symbolic)) != "refs/heads/"+request.Branch {
		return SplitResult{}, errors.New(CodeBranchMismatch)
	}
	status, err := g.ok("status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return SplitResult{}, err
	}
	if len(status) != 0 {
		return SplitResult{}, errors.New(CodeDirtyTree)
	}
	head, err := g.commit("refs/heads/" + request.Branch)
	if err != nil {
		return SplitResult{}, err
	}
	if request.Apply && request.ExpectHead != head {
		return SplitResult{}, errors.New(CodeHeadMismatch)
	}
	base, err := g.commit(request.Base)
	if err != nil {
		return SplitResult{}, err
	}
	if _, code, err := g.run(nil, nil, "merge-base", "--is-ancestor", base, head); err != nil || code != 0 {
		return SplitResult{}, errors.New(CodeRangeUnavailable)
	}
	commits, err := g.commits(base, head)
	if err != nil {
		return SplitResult{}, err
	}
	for _, rc := range commits {
		if len(rc.parents) != 1 {
			return SplitResult{}, errors.New(CodeMergeInRange)
		}
	}
	finalTree, err := g.tree(head)
	if err != nil {
		return SplitResult{}, err
	}
	r := SplitResult{Profile: SplitProfile, Branch: request.Branch, Base: base, OldHead: head, FinalTree: finalTree, Splits: []Split{}, ManualFollowUps: manualFollowUps}
	plans := make([][]change, len(commits))
	for i, rc := range commits {
		changes, err := g.changes(rc.sha)
		if err != nil {
			return SplitResult{}, err
		}
		var shared []change
		for _, ch := range changes {
			if request.Config.Class(ch.path) == "shared" {
				shared = append(shared, ch)
			}
		}
		if len(shared) == 0 || len(shared) == len(changes) {
			continue
		}
		plans[i] = shared
		paths := make([]string, len(shared))
		for j, ch := range shared {
			paths[j] = ch.path
		}
		r.Splits = append(r.Splits, Split{Commit: rc.sha, SharedPaths: paths})
	}
	if !request.Apply || len(r.Splits) == 0 {
		if len(r.Splits) == 0 {
			r.ManualFollowUps = []string{}
		}
		return r, nil
	}
	parent, rewriting, split := base, false, 0
	for i, rc := range commits {
		if plans[i] == nil && !rewriting {
			parent = rc.sha
			continue
		}
		rewriting = true
		m, err := g.message(rc.sha)
		if err != nil {
			return SplitResult{}, err
		}
		if plans[i] != nil {
			sharedTree, err := g.overlay(parent, plans[i])
			if err != nil {
				return SplitResult{}, err
			}
			sharedCommit, err := g.commitTree(sharedTree, parent, m, sharedMessage(m, request.Config.Marker, rc.sha))
			if err != nil {
				return SplitResult{}, err
			}
			r.Splits[split].SharedCommit = sharedCommit
			parent = sharedCommit
			r.Rewritten++
		}
		tree, err := g.tree(rc.sha)
		if err != nil {
			return SplitResult{}, err
		}
		next, err := g.commitTree(tree, parent, m, m.body)
		if err != nil {
			return SplitResult{}, err
		}
		if plans[i] != nil {
			r.Splits[split].RestCommit = next
			split++
		}
		parent = next
		r.Rewritten++
	}
	if tree, err := g.tree(parent); err != nil || tree != finalTree {
		return SplitResult{}, errors.New(CodeTreeMismatch)
	}
	if _, code, err := g.run(nil, nil, "update-ref", "-m", "corvint changes split-shared", "refs/heads/"+request.Branch, parent, head); err != nil || code != 0 {
		return SplitResult{}, errors.New(CodeHeadMismatch)
	}
	r.Applied, r.Mutates, r.NewHead = true, true, parent
	r.Restore = "git update-ref refs/heads/" + request.Branch + " " + head + " " + parent
	return r, nil
}

// refuseLiveSession refuses while a configured session marker exists in the
// checkout, or Git itself holds the index lock or has an operation in progress.
func refuseLiveSession(g git, c Config) error {
	cdup, err := g.ok("rev-parse", "--show-cdup")
	if err != nil {
		return err
	}
	gitDir, err := g.ok("rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	root := filepath.Join(g.root, filepath.FromSlash(strings.TrimSpace(string(cdup))))
	dir := strings.TrimSpace(string(gitDir))
	for _, m := range c.SessionMarkers {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(m))); err == nil {
			return errors.New(CodeLiveSession)
		}
	}
	for _, name := range []string{"index.lock", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "BISECT_LOG"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return errors.New(CodeLiveSession)
		}
	}
	return nil
}

func (g git) tree(commit string) (string, error) {
	out, err := g.ok("rev-parse", "--verify", "--end-of-options", commit+"^{tree}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// overlay writes the tree of parent with the shared changes applied, through
// a private temporary index; the checkout's own index is never read or written.
func (g git) overlay(parent string, shared []change) (string, error) {
	dir, err := os.MkdirTemp("", "corvint-split-shared-")
	if err != nil {
		return "", errors.New(CodeGitUnavailable)
	}
	defer os.RemoveAll(dir)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}
	if _, code, err := g.run(nil, env, "read-tree", parent+"^{tree}"); err != nil || code != 0 {
		return "", errors.New(CodeGitUnavailable)
	}
	var info bytes.Buffer
	for _, ch := range shared {
		if ch.deleted {
			info.WriteString("0 " + strings.Repeat("0", len(ch.object)) + "\t" + ch.path + "\x00")
			continue
		}
		info.WriteString(ch.mode + " " + ch.object + "\t" + ch.path + "\x00")
	}
	if _, code, err := g.run(info.Bytes(), env, "update-index", "-z", "--index-info"); err != nil || code != 0 {
		return "", errors.New(CodeGitUnavailable)
	}
	out, code, err := g.run(nil, env, "write-tree")
	if err != nil || code != 0 {
		return "", errors.New(CodeGitUnavailable)
	}
	return strings.TrimSpace(string(out)), nil
}

// commitTree writes a commit keeping the original author, committer, their
// dates and encoding.
func (g git) commitTree(tree, parent string, m message, body []byte) (string, error) {
	env := identity("AUTHOR", m.author)
	env = append(env, identity("COMMITTER", m.committer)...)
	args := []string{}
	if m.encoding != "" {
		args = append(args, "-c", "i18n.commitEncoding="+m.encoding)
	}
	args = append(args, "commit-tree", "--no-gpg-sign", tree, "-p", parent)
	out, code, err := g.run(body, env, args...)
	if err != nil || code != 0 {
		return "", errors.New(CodeGitUnavailable)
	}
	return strings.TrimSpace(string(out)), nil
}

// identity converts an `author`/`committer` header value into Git's
// environment form, keeping the raw timestamp and offset.
func identity(role, value string) []string {
	open, closing := strings.Index(value, " <"), strings.LastIndex(value, "> ")
	if open < 0 || closing < open {
		return nil
	}
	return []string{
		"GIT_" + role + "_NAME=" + value[:open],
		"GIT_" + role + "_EMAIL=" + value[open+2:closing],
		"GIT_" + role + "_DATE=" + value[closing+2:],
	}
}

// sharedMessage is the shared-only commit's message: the marker before the
// original subject, and a line naming the commit it was split from.
func sharedMessage(m message, marker, from string) []byte {
	subject := m.subject()
	if !strings.HasPrefix(subject, marker) {
		subject = strings.TrimSpace(marker) + " " + subject
	}
	return []byte(subject + "\n\nShared-path changes split from " + from + " by corvint changes split-shared.\n")
}
