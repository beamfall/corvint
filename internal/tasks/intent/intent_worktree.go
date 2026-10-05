package intent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The intent worktree rule (CTW-V0, docs/specs/corvint-tasks-intent-worktree-v0.md)
// chooses which checkout holds the `.taskman/` projection that reads audit and
// writers publish into. Resolution reads files only: it runs no git
// subprocess, opens nothing for writing and never refuses. It is cooperative
// routing, not a security boundary: HEAD files and queue manifests are
// ordinary repository files any local process can rewrite, and the canonical
// journal checks that follow it remain the authority.

// maxLinkedWorktrees bounds the linked-worktree scan (CTW-V0-004).
const maxLinkedWorktrees = 256

// IntentRoot is the worktree whose `.taskman/` is the intent projection
// (CTW-V0-001): the primary worktree unless the intent worktree rule selected
// a linked worktree. A Repository built without resolution uses the primary.
func (r *Repository) IntentRoot() string {
	if r.IntentWorktree == "" {
		return r.PrimaryWorktree
	}
	return r.IntentWorktree
}

// IntentHEAD is the HEAD file of the intent root: `<common>/HEAD` for the
// primary and `<common>/worktrees/<id>/HEAD` for a linked intent worktree.
func (r *Repository) IntentHEAD() string {
	if r.IntentGitDir == "" {
		return filepath.Join(r.CommonDir, "HEAD")
	}
	return filepath.Join(r.IntentGitDir, "HEAD")
}

// IntentLinked reports whether the intent root is a linked worktree.
func (r *Repository) IntentLinked() bool {
	return r.IntentRoot() != r.PrimaryWorktree
}

// IntentRepair is the repair text a refusal with the given code carries under
// CTW-V0-007. It is empty unless resolution recorded a fix or selected a
// linked intent worktree, so the intent-branch primary keeps its exact
// refusals (CTW-V0-009).
func (r *Repository) IntentRepair(detailCode string) string {
	switch detailCode {
	case wire.CodeIntentBranchMismatch:
		return r.IntentFix
	case wire.CodeIntentDiverged:
		if r.IntentFix != "" {
			return r.IntentFix
		}
		if r.IntentLinked() {
			return fmt.Sprintf("the intent projection is %q in the linked intent worktree (CTW-V0-002): commit or carry the .taskman changes from the checkout that holds them, or inspect the difference with \"corvint-tasks reconcile inspect\", then retry", filepath.Join(r.IntentRoot(), Dir))
		}
	}
	return ""
}

// WithIntentRepair returns err with IntentRepair appended to its message when
// err is an INTENT_BRANCH_MISMATCH or INTENT_DIVERGED wire error and a repair
// applies. The code and location are preserved; any other error is returned
// unchanged.
func (r *Repository) WithIntentRepair(err error) error {
	e, ok := err.(*wire.Error)
	if !ok || r == nil {
		return err
	}
	fix := r.IntentRepair(e.Code)
	if fix == "" {
		return err
	}
	return &wire.Error{Code: e.Code, Where: e.Where, Msg: e.Msg + "; " + fix}
}

type intentHint struct {
	queueID, branch string
}

type linkedCandidate struct {
	root, admin string
}

// resolveIntentWorktree applies CTW-V0-002 to CTW-V0-006 to a resolved
// repository. It depends only on repository files, never on the caller's cwd.
func resolveIntentWorktree(r *Repository) {
	r.IntentWorktree, r.IntentGitDir, r.IntentFix = r.PrimaryWorktree, r.CommonDir, ""
	hint, ok := readIntentHint(filepath.Join(r.PrimaryWorktree, Dir, "queue.json"))
	if !ok {
		return
	}
	current, ok := headBranch(filepath.Join(r.CommonDir, "HEAD"))
	if ok && current == hint.branch {
		return
	}
	found, rejected, truncated := linkedCandidates(r.CommonDir, hint)
	if len(found) == 1 {
		r.IntentWorktree, r.IntentGitDir = found[0].root, found[0].admin
		return
	}
	on := "a detached or unreadable HEAD"
	if ok {
		on = fmt.Sprintf("branch %q", current)
	}
	if len(found) > 1 {
		roots := make([]string, len(found))
		for i, c := range found {
			roots[i] = ShellQuote(c.root)
		}
		r.IntentFix = fmt.Sprintf("the primary checkout is on %s, not the intent branch %q, and %d linked worktrees hold it (%s): run \"git worktree remove <path>\" until one remains, then retry (CTW-V0-006)", on, hint.branch, len(found), strings.Join(roots, ", "))
		return
	}
	fix := fmt.Sprintf("the primary checkout is on %s, not the intent branch %q, and no linked worktree holds it", on, hint.branch)
	if truncated {
		fix += fmt.Sprintf("; only the first %d linked worktree entries were examined", maxLinkedWorktrees)
	}
	if len(rejected) > 0 {
		fix += "; not admitted: " + strings.Join(rejected, "; ") + "; run \"git worktree prune\", or \"git worktree repair\" if that worktree moved, and"
	} else {
		fix += ":"
	}
	suggested := r.PrimaryWorktree + "-" + strings.ReplaceAll(hint.branch, "/", "-")
	r.IntentFix = fix + fmt.Sprintf(" run \"git -C %s worktree add %s %s\", or switch the primary checkout to %s, then retry from any checkout (CTW-V0-005)", ShellQuote(r.PrimaryWorktree), ShellQuote(suggested), ShellQuote(hint.branch), hint.branch)
}

// ShellQuote quotes s as one POSIX shell word: it is wrapped in single quotes
// and each embedded single quote closes the word, is backslash-escaped and
// reopens it. Repair commands quote paths and branches this way so `$`,
// backticks and spaces are pasted literally.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// readIntentHint reads the intent branch and queue named by the primary's
// queue manifest (CTW-V0-003). Any failure means no hint.
func readIntentHint(path string) (intentHint, bool) {
	raw, err := readBounded(path, wire.MaxQueueFileBytes)
	if err != nil {
		return intentHint{}, false
	}
	q, err := DecodeQueue(raw)
	if err != nil || q.IntentBranch == "" {
		return intentHint{}, false
	}
	return intentHint{queueID: q.QueueID.Raw, branch: q.IntentBranch}, true
}

// headBranch reads a HEAD file naming a local branch.
func headBranch(path string) (string, bool) {
	raw, err := readBounded(path, 4*wire.KiB)
	if err != nil {
		return "", false
	}
	const prefix = "ref: refs/heads/"
	text := strings.TrimSuffix(string(raw), "\n")
	if !strings.HasPrefix(text, prefix) || strings.ContainsAny(text, "\r\n") {
		return "", false
	}
	return strings.TrimPrefix(text, prefix), true
}

// linkedCandidates scans `<common>/worktrees/*` in name order for linked
// worktrees whose HEAD names the hinted branch (CTW-V0-004). An entry on the
// branch that fails validation is reported, never admitted.
func linkedCandidates(common string, hint intentHint) (found []linkedCandidate, rejected []string, truncated bool) {
	dir := filepath.Join(common, "worktrees")
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		return nil, nil, false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, false
	}
	if len(entries) > maxLinkedWorktrees {
		entries, truncated = entries[:maxLinkedWorktrees], true
	}
	for _, entry := range entries {
		admin := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(admin)
		if err != nil || !info.IsDir() {
			continue
		}
		if branch, ok := headBranch(filepath.Join(admin, "HEAD")); !ok || branch != hint.branch {
			continue
		}
		root, reason := validateLinked(admin, info, hint)
		if reason != "" {
			rejected = append(rejected, fmt.Sprintf("linked worktree entry %q %s", entry.Name(), reason))
			continue
		}
		found = append(found, linkedCandidate{root: root, admin: admin})
	}
	return found, rejected, truncated
}

// validateLinked admits one linked worktree on the hinted branch: its
// registration must point at an existing worktree whose `.git` file points
// back to the same admin directory, and its own queue manifest must name the
// same queue and intent branch (CTW-V0-004). The reason is empty on success.
func validateLinked(admin string, adminInfo os.FileInfo, hint intentHint) (string, string) {
	raw, err := readBounded(filepath.Join(admin, "gitdir"), 4*wire.KiB)
	if err != nil {
		return "", "is stale: its gitdir file is missing or unreadable"
	}
	line := strings.TrimRight(string(raw), "\r\n")
	if line == "" || strings.ContainsAny(line, "\r\n") {
		return "", "is stale: its gitdir file is not a single path"
	}
	if !filepath.IsAbs(line) {
		line = filepath.Join(admin, line)
	}
	line = filepath.Clean(line)
	if filepath.Base(line) != ".git" {
		return "", "is stale: its gitdir file does not name a worktree .git file"
	}
	root := filepath.Dir(line)
	parent, err := filepath.EvalSymlinks(filepath.Dir(root))
	if err != nil {
		return "", "is stale: its worktree directory is missing"
	}
	root = filepath.Join(parent, filepath.Base(root))
	if _, err := wire.ParsePathText("/intentWorktree", root); err != nil {
		return "", "is not admitted: its worktree path is not a supported path"
	}
	if fi, err := os.Lstat(root); err != nil || !fi.IsDir() {
		return "", "is stale: its worktree directory is missing"
	}
	raw, err = readBounded(filepath.Join(root, ".git"), 4*wire.KiB)
	if err != nil {
		return "", "is stale: its worktree .git file is missing or unreadable"
	}
	back := strings.TrimRight(string(raw), "\r\n")
	if !strings.HasPrefix(back, "gitdir: ") || strings.ContainsAny(back, "\r\n") {
		return "", "is stale: its worktree .git file is not a single gitdir: line"
	}
	back = strings.TrimPrefix(back, "gitdir: ")
	if !filepath.IsAbs(back) {
		back = filepath.Join(root, back)
	}
	if fi, err := os.Lstat(filepath.Clean(back)); err != nil || !os.SameFile(fi, adminInfo) {
		return "", "is stale: its worktree .git file points to a different registration"
	}
	if err := checkNoSymlink(filepath.Join(root, Dir)); err != nil {
		return "", "is not admitted: its .taskman path contains a symlink or cannot be observed"
	}
	raw, err = readBounded(filepath.Join(root, Dir, "queue.json"), wire.MaxQueueFileBytes)
	if err != nil {
		return "", "is not admitted: its .taskman/queue.json is missing or unreadable"
	}
	q, err := DecodeQueue(raw)
	if err != nil {
		return "", "is not admitted: its .taskman/queue.json is malformed"
	}
	if q.QueueID.Raw != hint.queueID || q.IntentBranch != hint.branch {
		return "", "is not admitted: its .taskman/queue.json names a different queue or intent branch"
	}
	return root, ""
}
